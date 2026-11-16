package desktop

import (
	"context"
	"errors"
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"golang.org/x/crypto/ssh"
	"sshub/internal/connection"
	"sshub/internal/library"
)

type workspace struct {
	key     int
	profile library.Profile
	jump    *library.Profile
	status  string
	err     string
	attempt int
	cancel  context.CancelFunc
	client  *connection.Client
	term    *terminal.Terminal
}

func (a *app) dispatch(fn func()) {
	if !a.closing.Load() && a.win != nil {
		a.win.Update(func() {
			if !a.closing.Load() {
				fn()
			}
		})
	}
}

func (a *app) activeWorkspace() *workspace {
	for _, tab := range a.tabs {
		if tab.key == a.activeTab {
			return tab
		}
	}
	return nil
}

func busy(tab *workspace) bool { return tab.status == "连接中" || tab.status == "已连接" }

func (a *app) release(tab *workspace) {
	tab.attempt++
	if tab.cancel != nil {
		tab.cancel()
		tab.cancel = nil
	}
	if tab.client != nil {
		client := tab.client
		tab.client = nil
		go client.Close()
	}
}

func (a *app) disconnect(tab *workspace) {
	a.release(tab)
	tab.status, tab.err = "已断开", ""
	// Keep the emulator and its output until the user closes or reconnects.
}

func (a *app) removeTab(key int) {
	for _, tab := range a.tabs {
		if tab.key == key {
			a.release(tab)
			if tab.term != nil {
				term := tab.term
				go term.Close()
			}
		}
	}
	a.tabs = slices.DeleteFunc(a.tabs, func(v *workspace) bool { return v.key == key })
	if a.activeTab == key {
		a.activeTab = 0
		if len(a.tabs) > 0 {
			a.activeTab = a.tabs[len(a.tabs)-1].key
		}
	}
}

func (a *app) requestClose(tab *workspace) {
	if busy(tab) {
		a.closeTabID, a.confirmCloseTab = tab.key, true
		return
	}
	a.removeTab(tab.key)
}

func (a *app) connect(tab *workspace) {
	if busy(tab) {
		return
	}
	a.release(tab)
	if tab.term != nil {
		term := tab.term
		tab.term = nil
		go term.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	tab.cancel, tab.status, tab.err = cancel, "连接中", ""
	attempt := tab.attempt
	go func() {
		saved := map[string]string{}
		options := func(p library.Profile) connection.Options {
			return connection.Options{
				Target: p.Username + "@" + net.JoinHostPort(p.Host, strconv.Itoa(p.Port)), Identity: p.IdentityPath, Auth: p.Auth,
				Secret: func(ctx context.Context, kind string) (string, error) { return a.requestSecret(ctx, p, kind, saved) },
				HostKey: func(ctx context.Context, host string, remote net.Addr, key ssh.PublicKey) error {
					return a.trust.Verify(ctx, host, remote, key, a.confirmHost)
				},
			}
		}
		o := options(tab.profile)
		if tab.jump != nil {
			jump := options(*tab.jump)
			o.Jump = &jump
		}
		client, err := connection.DialContext(ctx, o)
		if err != nil {
			a.connectionFailed(tab, attempt, err)
			return
		}
		stop := context.AfterFunc(ctx, client.Close)
		shellCtx, shellCancel := context.WithTimeout(ctx, 15*time.Second)
		stopShell := context.AfterFunc(shellCtx, client.Close)
		shell, err := client.Shell()
		stopShell()
		shellCancel()
		if err != nil || ctx.Err() != nil {
			stop()
			client.Close()
			if err == nil {
				err = ctx.Err()
			}
			a.connectionFailed(tab, attempt, err)
			return
		}
		term, err := terminal.New(terminal.Options{
			Conn: shell, Font: terminal.Font{Family: "Cascadia Mono", Size: 13}, Scrollback: 2 << 20, ImageLimit: -1,
			// Status is shown in the toolbar; an injected exit line can overwrite a partial remote line.
			NoExitMessage: true,
			OnExit: func(int) {
				client.Close()
				a.dispatch(func() {
					if tab.attempt == attempt && tab.status == "已连接" {
						tab.status = "已断开"
					}
				})
			},
		})
		if err != nil {
			stop()
			client.Close()
			a.connectionFailed(tab, attempt, err)
			return
		}
		if ctx.Err() != nil || a.closing.Load() {
			term.Close()
			client.Close()
			return
		}
		// Until the UI takes ownership, cancellation must also dispose the emulator.
		stopPending := context.AfterFunc(ctx, func() { term.Close() })
		for key, value := range saved {
			if err := a.vault().Set(key, []byte(value)); err != nil {
				a.dispatch(func() { a.actionError = "连接成功，但系统凭据保存失败；请下次重新输入。" })
			}
			delete(saved, key)
		}
		a.dispatch(func() {
			stopPending()
			if tab.attempt != attempt || ctx.Err() != nil {
				go term.Close()
				client.Close()
				return
			}
			tab.term, tab.client, tab.status = term, client, "已连接"
			select {
			case <-term.Done():
				tab.status = "已断开"
			default:
			}
		})
	}()
}

func (a *app) connectionFailed(tab *workspace, attempt int, err error) {
	a.dispatch(func() {
		if tab.attempt != attempt {
			return
		}
		tab.status = "连接失败"
		if errors.Is(err, context.Canceled) {
			tab.status = "已取消"
		}
		tab.err = err.Error()
	})
}
