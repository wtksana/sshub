package desktop

import (
	"context"
	"errors"
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"golang.org/x/crypto/ssh"
	"sshub/internal/connection"
	"sshub/internal/library"
	"sshub/internal/transfer"
)

type sessionTab struct {
	key                     int
	profile                 library.Profile
	jump                    *library.Profile
	status                  string
	err                     string
	attempt                 int
	cancel                  context.CancelFunc
	client                  *connection.Client
	transport               *sharedTransport
	shell                   *connection.Shell
	files                   fileBrowser
	retryRequests           []transfer.Request
	term                    *terminal.Terminal
	observed                *observedShell
	highlight               *connection.HighlightStream
	highlightNotified       bool
	searchOpen, searchFocus bool
	search, searchApplied   string
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

func (a *app) activeTab() *sessionTab {
	if a.layout == nil || a.layout.Current() == nil {
		return nil
	}
	return a.tabByID(a.layout.Current().Active)
}

func (a *app) tabByID(id int) *sessionTab {
	for _, tab := range a.tabs {
		if tab.key == id {
			return tab
		}
	}
	return nil
}

func busy(tab *sessionTab) bool { return tab.status == "连接中" || tab.status == "已连接" }

func (a *app) release(tab *sessionTab) {
	tab.attempt++
	if tab.files.cancel != nil {
		tab.files.cancel()
		tab.files.cancel = nil
	}
	tab.files = fileBrowser{generation: tab.files.generation + 1}
	if tab.cancel != nil {
		tab.cancel()
		tab.cancel = nil
	}
	if tab.shell != nil {
		shell := tab.shell
		tab.shell = nil
		go shell.Close()
	}
	if tab.highlight != nil {
		highlight := tab.highlight
		tab.highlight = nil
		go highlight.Close()
	}
	if tab.transport != nil {
		tab.transport.release()
		tab.transport = nil
	}
	tab.client = nil
}

func (a *app) disconnect(tab *sessionTab) {
	tab.retryRequests = nil
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
	a.tabs = slices.DeleteFunc(a.tabs, func(v *sessionTab) bool { return v.key == key })
	a.layout.Close(key)
}

func (a *app) requestClose(tab *sessionTab) {
	if (busy(tab) && !a.preferences.SkipCloseConfirmation) || a.hasTabTransfers(tab.key) {
		a.closeTabID, a.confirmCloseTab = tab.key, true
		a.closeTabRemember, a.closeTabError = false, ""
		return
	}
	a.removeTab(tab.key)
}

func (a *app) connect(tab *sessionTab) {
	if busy(tab) {
		return
	}
	if tab.profile.IdentityNeedsReview || (tab.jump != nil && tab.jump.IdentityNeedsReview) {
		tab.status, tab.err = "连接失败", "导入的私钥路径尚未核对，请编辑目标或跳板机连接并确认本机私钥路径，然后从连接列表重新连接"
		tab.retryRequests = nil
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
	tab.searchOpen, tab.searchApplied = false, ""
	attempt := tab.attempt
	fontSize := float32(a.preferences.FontSize)
	fontFamily := a.preferences.FontFamily
	scrollback := a.preferences.EffectiveScrollbackMiB() << 20
	if _, saved := a.layout.Locate(tab.key); saved != nil {
		fontSize = saved.FontSize
	}
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
		observed := a.observeShell(shell)
		highlight := connection.NewHighlightStream(observed)
		term, err := terminal.New(terminal.Options{
			Theme: githubTerminalLight, DarkTheme: githubTerminalDark,
			Conn: highlight, Font: terminal.Font{Family: fontFamily, Size: fontSize}, Scrollback: scrollback, ImageLimit: -1,
			OnZoom: func(z terminal.Zoom) bool { a.zoom(tab, z.Steps, z.Reset); return true },
			OnKey: func(mods ui.Modifiers, key ui.Key) bool {
				if key == ui.KeyH && mods == ui.Ctrl|ui.Shift && tab.highlight != nil {
					tab.highlight.SetEnabled(!tab.highlight.Enabled())
					return true
				}
				if key == ui.KeyContextMenu && mods == 0 || key == ui.KeyF10 && mods == ui.Shift {
					a.showTerminalMenu(tab)
					return true
				}
				return false
			},
			OnSearch: func(terminal.SearchStatus) { a.dispatch(func() {}) },
			// Status is shown in the toolbar; an injected exit line can overwrite a partial remote line.
			NoExitMessage: true,
			OnExit: func(int) {
				a.dispatch(func() {
					if tab.attempt == attempt && tab.status == "已连接" {
						a.release(tab)
						tab.status = "已断开"
					}
				})
			},
		})
		if err != nil {
			highlight.Close()
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
			stop()
			cancel()
			tab.cancel = nil
			tab.transport, tab.shell = newTransport(client), shell
			tab.term, tab.client, tab.status = term, client, "已连接"
			tab.observed, _ = observed.(*observedShell)
			tab.highlight, tab.highlightNotified = highlight, false
			a.zoom(tab, 0, false)
			select {
			case <-term.Done():
				a.release(tab)
				tab.status = "已断开"
			default:
				for _, request := range tab.retryRequests {
					a.enqueue(tab, request, &transferBatch{})
				}
				tab.retryRequests = nil
			}
		})
	}()
}

func (a *app) connectionFailed(tab *sessionTab, attempt int, err error) {
	a.dispatch(func() {
		if tab.attempt != attempt {
			return
		}
		tab.status = "连接失败"
		tab.retryRequests = nil
		if errors.Is(err, context.Canceled) {
			tab.status = "已取消"
		}
		tab.err = err.Error()
	})
}
