package desktop

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"sshub/internal/connection"
	"sshub/internal/library"
	layout "sshub/internal/workspace"
)

type app struct {
	dir                                  string
	store                                *library.Store
	preferences                          library.Preferences
	settings                             *settingsEditor
	migration                            *migrationEditor
	profiles                             []library.Profile
	loadError, actionError               string
	query, scope, group                  string
	selected                             string
	leftSize, contentPercent             float32
	hideLeft, hideRight                  bool
	editing, deleting                    bool
	editGeneration                       int
	draft                                editor
	deleteID, deleteName                 string
	tabs                                 []*workspace
	layout                               *layout.State
	layoutStore                          *layout.Store
	layoutQueue                          chan []byte
	layoutDone                           chan struct{}
	layoutSnapshot                       string
	layoutReadOnly                       bool
	layoutActions                        []func()
	terminalFocus                        int
	revealedTabs                         map[int]int
	focusedTerminal, lastFocusedTerminal int
	win                                  *mygo.Window
	closing                              atomic.Bool
	trust                                *connection.TrustStore
	prompts                              chan *connectionPrompt
	prompt                               *connectionPrompt
	promptGeneration                     int
	closeTabID                           int
	confirmCloseTab                      bool
	confirmQuit, allowQuit               bool
	manageHosts                          bool
	hosts                                []connection.HostIdentity
	forgetHost                           string
	confirmForgetHost                    bool
	notice                               string
	transfers                            []*transferTask
	nextTransfer                         int
	transferConflict                     *fileConflict
	fileEdit                             *fileEditor
	transferQueue                        chan *transferTask
	transferDone                         chan struct{}
	workCtx                              context.Context
	workCancel                           context.CancelFunc
}

func newApp(dir string) *app {
	a := &app{dir: dir, preferences: library.DefaultPreferences(), leftSize: 260, contentPercent: 66, group: "全部分组", prompts: make(chan *connectionPrompt, 16), revealedTabs: map[int]int{}}
	a.workCtx, a.workCancel = context.WithCancel(context.Background())
	a.load()
	a.transferQueue = make(chan *transferTask, 256)
	a.transferDone = make(chan struct{})
	go a.runTransfers()
	return a
}

func (a *app) load() {
	if a.store != nil {
		return
	}
	if a.dir == "" {
		dir, err := library.DefaultDir()
		if err != nil {
			a.loadError = "无法确定配置目录：" + err.Error()
			return
		}
		a.dir = dir
	}
	s, err := library.Open(a.dir)
	if err != nil {
		a.loadError = err.Error()
		return
	}
	a.store, a.loadError = s, ""
	a.preferences = s.Preferences()
	a.trust = connection.NewTrustStore(a.dir)
	a.refresh()
	a.restoreLayout()
}

func (a *app) close() {
	if a.closing.Swap(true) {
		return
	}
	a.flushLayout()
	a.workCancel()
	for _, task := range a.transfers {
		if task.cancel != nil {
			task.cancel()
		}
	}
	close(a.transferQueue)
	for _, tab := range a.tabs {
		a.release(tab)
		if tab.term != nil {
			tab.term.Close()
		}
	}
	select {
	case <-a.transferDone:
	case <-time.After(3 * time.Second):
	}
	if a.store != nil {
		a.store.Close()
	}
}

func (a *app) refresh() { a.profiles = a.store.Profiles() }

func (a *app) selectedProfile() (library.Profile, bool) {
	i := slices.IndexFunc(a.profiles, func(p library.Profile) bool { return p.ID == a.selected })
	if i < 0 {
		return library.Profile{}, false
	}
	return a.profiles[i], true
}

func (a *app) openWorkspace(p library.Profile) *workspace {
	if err := a.store.MarkUsed(p.ID); err != nil {
		a.actionError = err.Error()
		return nil
	}
	a.refresh()
	id, err := a.layout.Add(p.ID)
	if err != nil {
		a.actionError = err.Error()
		return nil
	}
	tab := a.newWorkspace(id, p)
	a.setDefaultFontSize(id)
	a.tabs = append(a.tabs, tab)
	a.actionError = ""
	return tab
}

func (a *app) newWorkspace(id int, p library.Profile) *workspace {
	tab := &workspace{key: id, profile: p, status: "待连接"}
	if p.JumpID != "" {
		if i := slices.IndexFunc(a.profiles, func(v library.Profile) bool { return v.ID == p.JumpID }); i >= 0 {
			jump := a.profiles[i]
			tab.jump = &jump
		}
	}
	return tab
}

func (a *app) favorite(p library.Profile) {
	p.Favorite = !p.Favorite
	if _, err := a.store.Upsert(p); err != nil {
		a.actionError = err.Error()
		return
	}
	a.actionError = ""
	a.refresh()
}
