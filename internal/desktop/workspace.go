package desktop

import (
	"fmt"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	layout "sshub/internal/workspace"
)

func (a *app) center(c *ui.Context) {
	ui.Column(c).Fill().Clip().Children(func() { a.paneTree(c, a.layout.Root) })
}

func (a *app) paneTree(c *ui.Context, n *layout.Node) {
	if n.Axis != "" {
		first := func() { a.paneTree(c, n.First) }
		second := func() { a.paneTree(c, n.Second) }
		if n.Axis == "horizontal" {
			ui.SplitPercent(c.Key(fmt.Sprintf("pane-divider-%d", n.ID)), &n.Ratio, first, second).SplitReset(50).Fill()
		} else {
			ui.SplitVerticalPercent(c.Key(fmt.Sprintf("pane-divider-%d", n.ID)), &n.Ratio, first, second).SplitReset(50).Fill()
		}
		return
	}
	a.pane(c, n)
}

func (a *app) pane(c *ui.Context, pane *layout.Node) {
	t := c.Theme()
	box := ui.Column(c.Key(fmt.Sprintf("pane-%d", pane.ID))).Label("终端窗格").Fill().MinWidth(0).MinHeight(0).Clip()
	if a.layout.ActivePane == pane.ID {
		box.Current(ui.CurrentItem)
	}
	box.ObserveInput(func(ui.InputEvent) { a.layout.ActivePane = pane.ID })
	box.ContextMenu(func(m *ui.Menu) { a.paneMenu(m, pane, a.workspaceByID(pane.Active)) })
	box.Children(func() {
		ui.Row(c).FillWidth().Height(max(36, t.FontSize+20)).Background(t.Background).Children(func() {
			ui.ScrollHorizontal(c.Key(fmt.Sprintf("workspace-tabs-%d", pane.ID))).Grow(1).Height(max(36, t.FontSize+20)).Children(func() {
				ui.Row(c).Children(func() {
					for _, saved := range pane.Tabs {
						tab := a.workspaceByID(saved.ID)
						if tab == nil {
							continue
						}
						active := pane.Active == tab.key
						row := ui.Row(c.Key(fmt.Sprintf("tab-row-%d", tab.key))).Height(max(36, t.FontSize+20)).AlignItems(ui.Center)
						if active {
							row.Background(t.Surface)
						}
						row.Children(func() {
							b := ui.ButtonBase(c.Key(fmt.Sprintf("tab-%d", tab.key))).Label(tab.profile.Name+" · "+tab.status).Height(max(34, t.FontSize+18)).Padding(0, 10).AccessSelected(active)
							b.Tooltip(fmt.Sprintf("%s · %s\n%s@%s · 字号 %g\n右键管理会话", tab.profile.Name, tab.status, tab.profile.Username, tab.profile.Host, saved.FontSize))
							b.Children(func() {
								ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
									color := t.TextMuted
									if tab.status == "已连接" {
										color = t.Success
									} else if tab.status == "连接失败" {
										color = t.Danger
									} else if tab.status == "连接中" {
										color = t.Warning
									}
									ui.Icon(c, iconStatus).Size(8, 8).Shrink(0).TextColor(color)
									ui.Text(c, tab.profile.Name).FontSize(typeSize(c, -1)).SingleLine()
								})
							})
							if active && a.revealedTabs[pane.ID] != tab.key {
								b.ScrollIntoView()
								a.revealedTabs[pane.ID] = tab.key
							}
							b.OnClick(func() { a.layout.Activate(tab.key); a.terminalFocus = tab.key })
							b.ContextMenu(func(m *ui.Menu) { a.paneMenu(m, pane, tab) })
							iconButton(c.Key(fmt.Sprintf("close-tab-%d", tab.key)), iconClose, "关闭工作区 "+tab.profile.Name).Size(24, 24).FontSize(14).Radius(0).OnClick(func() { a.requestClose(tab) })
						})
					}
				})
			})
			iconButton(c.Key(fmt.Sprintf("pane-menu-%d", pane.ID)), iconMore, "窗格操作").Tooltip("会话与分屏操作").Radius(0).Menu(func(m *ui.Menu) { a.paneMenu(m, pane, a.workspaceByID(pane.Active)) })
		})
		line := t.Border
		if a.layout.ActivePane == pane.ID {
			line = t.Accent
		}
		ui.Box(c).FillWidth().Height(1).Background(line)
		tab := a.workspaceByID(pane.Active)
		if tab == nil {
			ui.Column(c).Fill().Padding(24).Gap(8).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "打开一个连接").FontSize(typeSize(c, 3)).TextColor(t.TextMuted)
				ui.Text(c, "双击左侧服务器，或右键选择打开工作区").FontSize(typeSize(c, -1)).TextColor(t.TextMuted).Wrap()
			})
			return
		}
		if tab.searchOpen && tab.term != nil {
			a.terminalSearch(c, tab)
		}
		if tab.err != "" {
			ui.Text(c, tab.err).Padding(6).FontSize(typeSize(c, -2)).TextColor(t.Danger).Wrap()
		}
		if tab.term != nil {
			e := terminal.View(c.Key(fmt.Sprintf("terminal-%d", tab.key)), tab.term).Grow(1).FillWidth().MinHeight(0)
			if e.Focused() {
				a.focusedTerminal = tab.key
			}
			if a.terminalFocus == tab.key {
				e.Focus()
				a.terminalFocus = 0
			}
		} else {
			ui.Column(c).Fill().Padding(24).Gap(8).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				ui.Text(c.Key(fmt.Sprintf("session-status-%d", tab.key)), tab.profile.Name+" · "+tab.status).TextColor(t.TextMuted)
				ui.Text(c, "右键标签连接 · Ctrl+Shift+Enter").FontSize(typeSize(c, -1)).TextColor(t.TextMuted)
			})
		}
	})
	if box.Shortcut(ui.Ctrl|ui.Shift, ui.KeyF) {
		a.layout.Activate(pane.Active)
		a.toggleSearch()
	}
	if box.Shortcut(ui.Ctrl|ui.Shift, ui.KeyEnter) {
		if tab := a.workspaceByID(pane.Active); tab != nil {
			a.layout.Activate(tab.key)
			a.connect(tab)
		}
	}
}

func (a *app) splitPane(axis string) {
	if err := a.layout.Split(axis); err != nil {
		a.actionError = err.Error()
	}
}

func (a *app) moveTab() {
	tab := a.activeWorkspace()
	if tab == nil {
		return
	}
	panes := a.layout.Panes()
	for i, pane := range panes {
		if pane.ID == a.layout.ActivePane {
			if err := a.layout.Move(tab.key, panes[(i+1)%len(panes)].ID); err != nil {
				a.actionError = err.Error()
			}
			a.terminalFocus = tab.key
			return
		}
	}
}
