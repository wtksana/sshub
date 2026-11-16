package desktop

import (
	"slices"

	"sshub/internal/library"
)

type app struct {
	dir                      string
	store                    *library.Store
	profiles                 []library.Profile
	loadError, actionError   string
	query, scope, group      string
	selected                 string
	leftSize, contentPercent float32
	hideLeft, hideRight      bool
	editing, deleting        bool
	editGeneration           int
	draft                    editor
	deleteID, deleteName     string
	tabs                     []workspace
	nextTab, activeTab       int
}

func newApp(dir string) *app {
	a := &app{dir: dir, leftSize: 260, contentPercent: 66, group: "全部分组"}
	a.load()
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
	a.refresh()
}

func (a *app) close() {
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

func (a *app) openWorkspace(p library.Profile) {
	if err := a.store.MarkUsed(p.ID); err != nil {
		a.actionError = err.Error()
		return
	}
	a.refresh()
	a.nextTab++
	a.tabs = append(a.tabs, workspace{key: a.nextTab, profile: p})
	a.activeTab, a.actionError = a.nextTab, ""
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
