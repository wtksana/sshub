package desktop

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
	layout "sshub/internal/workspace"
)

func (a *app) profileMenu(m *ui.Menu, p library.Profile) {
	if m.Item("连接服务器").ID("connect-profile").Chosen() {
		a.selected = p.ID
		if tab := a.openWorkspace(p); tab != nil {
			a.connect(tab)
		}
	}
	if m.Item("打开工作区").ID("open-workspace").Chosen() {
		a.selected = p.ID
		a.openWorkspace(p)
	}
	m.Separator()
	if m.Item("编辑连接…").ID("edit-profile").Chosen() {
		a.beginEdit(p)
	}
	label := "收藏"
	if p.Favorite {
		label = "取消收藏"
	}
	if m.Item(label).ID("favorite-profile").Chosen() {
		a.favorite(p)
	}
	if m.Item("忘记凭据").ID("forget-profile-credentials").Chosen() {
		a.forgetCredentials(p)
	}
	m.Separator()
	if m.Item("删除连接…").ID("delete-profile").Danger().Chosen() {
		a.deleteID, a.deleteName, a.deleting = p.ID, p.Name, true
	}
}

func (a *app) paneMenu(m *ui.Menu, pane *layout.Node, tab *workspace) {
	activate := func() {
		a.layout.ActivePane = pane.ID
		if tab != nil {
			a.layout.Activate(tab.key)
		}
	}
	if tab != nil {
		switch tab.status {
		case "连接中":
			if m.Item("取消连接").ID("cancel-connect").Chosen() {
				activate()
				a.disconnect(tab)
				tab.status = "已取消"
			}
		case "已连接":
			if m.Item("断开连接").ID("disconnect-session").Chosen() {
				activate()
				a.disconnect(tab)
			}
		default:
			label := "连接"
			if tab.term != nil {
				label = "重新连接"
			}
			if m.Item(label).ID("connect-pending").Chosen() {
				activate()
				a.connect(tab)
			}
		}
		if m.Item("搜索终端").ID("terminal-search").Disabled(tab.term == nil).Chosen() {
			activate()
			a.toggleSearch()
		}
		m.Submenu("字号", func(m *ui.Menu) {
			if _, saved := a.layout.Locate(tab.key); saved != nil {
				m.Item(fmt.Sprintf("当前字号：%g", saved.FontSize)).Disabled(true)
			}
			for _, item := range []struct {
				label, id string
				steps     int
				reset     bool
			}{{"放大字号", "font-larger", 1, false}, {"缩小字号", "font-smaller", -1, false}, {"重置字号", "font-reset", 0, true}} {
				if m.Item(item.label).ID(item.id).Disabled(tab.term == nil).Chosen() {
					activate()
					a.zoom(tab, item.steps, item.reset)
				}
			}
		})
		m.Separator()
	}
	if m.Item("左右分屏").ID("split-horizontal").Disabled(len(a.layout.Panes()) >= 4).Chosen() {
		a.layoutActions = append(a.layoutActions, func() { activate(); a.splitPane("horizontal") })
	}
	if m.Item("上下分屏").ID("split-vertical").Disabled(len(a.layout.Panes()) >= 4).Chosen() {
		a.layoutActions = append(a.layoutActions, func() { activate(); a.splitPane("vertical") })
	}
	if m.Item("移至下一窗格").ID("move-tab").Disabled(tab == nil || len(a.layout.Panes()) < 2).Chosen() {
		a.layoutActions = append(a.layoutActions, func() { activate(); a.moveTab() })
	}
	if m.Item("合并窗格").ID("merge-pane").Disabled(len(a.layout.Panes()) < 2).Chosen() {
		a.layoutActions = append(a.layoutActions, func() { activate(); a.layout.Merge() })
	}
	if tab != nil {
		m.Separator()
		if m.Item("忘记凭据").ID("forget-credentials").Disabled(tab.status == "连接中").Chosen() {
			a.forgetCredentials(tab.profile)
		}
		if m.Item("关闭会话").ID("close-session").Chosen() {
			a.requestClose(tab)
		}
	}
}

func (a *app) appMenu(m *ui.Menu) {
	if m.Item("连接库").ID("toggle-connections").Checked(!a.hideLeft).Chosen() {
		a.hideLeft = !a.hideLeft
	}
	if m.Item("文件面板").ID("toggle-files").Checked(!a.hideRight).Chosen() {
		a.hideRight = !a.hideRight
	}
	m.Separator()
	if m.Item("已信任主机…").ID("manage-hosts").Disabled(a.store == nil).Chosen() {
		a.showHosts()
	}
}

func (a *app) filterMenu(m *ui.Menu) {
	for _, scope := range []struct{ value, label string }{{"", "全部连接"}, {"favorites", "收藏"}, {"recent", "最近使用"}} {
		if m.Item(scope.label).ID("scope-" + scope.value).Radio(a.scope == scope.value).Chosen() {
			a.scope = scope.value
		}
	}
	m.Separator()
	groups := []string{"全部分组", "未分组"}
	for _, name := range library.Groups(a.profiles) {
		groups = append(groups, "分组："+name)
	}
	m.Submenu("分组", func(m *ui.Menu) {
		for _, group := range groups {
			if m.Item(group).Radio(a.group == group).Chosen() {
				a.group = group
			}
		}
	})
}
