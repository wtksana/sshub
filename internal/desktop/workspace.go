package desktop

import (
	"fmt"
	"net"
	"strconv"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
	layout "sshub/internal/workspace"
)

func (a *app) center(c *ui.Context) {
	ui.Column(c).Fill().Clip().Children(func() {
		a.profileDetails(c)
		ui.Divider(c)
		ui.Row(c).Padding(5, 8).Gap(5).Wrap().AlignItems(ui.Center).Children(func() {
			ui.Button(c.Key("split-horizontal"), "左右分屏").Disabled(len(a.layout.Panes()) >= 4).OnClick(func() { a.splitPane("horizontal") })
			ui.Button(c.Key("split-vertical"), "上下分屏").Disabled(len(a.layout.Panes()) >= 4).OnClick(func() { a.splitPane("vertical") })
			ui.Button(c.Key("merge-pane"), "合并窗格").Disabled(len(a.layout.Panes()) < 2).OnClick(func() { a.layout.Merge() })
			ui.Button(c.Key("move-tab"), "移至下一窗格").Disabled(len(a.layout.Panes()) < 2 || a.activeWorkspace() == nil).OnClick(a.moveTab)
			ui.Spacer(c)
			ui.Textf(c, "%d 个窗格 / %d 个会话", len(a.layout.Panes()), len(a.tabs)).FontSize(11).TextColor(c.Theme().TextMuted)
		})
		a.paneTree(c, a.layout.Root)
	})
}

func (a *app) profileDetails(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().Padding(10, 12).Gap(5).Children(func() {
		if p, ok := a.selectedProfile(); ok {
			ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.Name).Bold()
				ui.Text(c, p.Username+"@"+net.JoinHostPort(p.Host, strconv.Itoa(p.Port))).FontSize(11).TextColor(t.TextMuted)
				group := p.Group
				if group == "" {
					group = "未分组"
				}
				ui.Text(c, "分组："+group).FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, "认证："+authLabel(p.Auth)).FontSize(11).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(6).Wrap().Children(func() {
				ui.PrimaryButton(c.Key("open-workspace"), "打开工作区").OnClick(func() { a.openWorkspace(p) })
				ui.Button(c.Key("edit-profile"), "编辑").OnClick(func() { a.beginEdit(p) })
				label := "收藏"
				if p.Favorite {
					label = "取消收藏"
				}
				ui.Button(c.Key("favorite-profile"), label).OnClick(func() { a.favorite(p) })
				ui.Button(c.Key("delete-profile"), "删除").TextColor(t.Danger).OnClick(func() { a.deleteID, a.deleteName, a.deleting = p.ID, p.Name, true })
			})
		} else {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "选择一台服务器").Bold()
				ui.Text(c, "从连接库打开至当前窗格").FontSize(11).TextColor(t.TextMuted)
				ui.PrimaryButton(c.Key("welcome-new-profile"), "新建连接").OnClick(func() { a.beginEdit(library.Profile{}) })
			})
		}
	})
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
	color := t.Border
	if a.layout.ActivePane == pane.ID {
		color = t.Accent
	}
	box := ui.Column(c.Key(fmt.Sprintf("pane-%d", pane.ID))).Fill().MinWidth(0).MinHeight(0).Clip().Border(1, color)
	box.ObserveInput(func(ui.InputEvent) { a.layout.ActivePane = pane.ID })
	box.Children(func() {
		ui.Row(c).FillWidth().Gap(3).Padding(3).Children(func() {
			index := 1
			for i, p := range a.layout.Panes() {
				if p.ID == pane.ID {
					index = i + 1
				}
			}
			ui.Button(c.Key(fmt.Sprintf("activate-pane-%d", pane.ID)), fmt.Sprintf("窗格 %d", index)).FontSize(10).Padding(3, 5).OnClick(func() { a.layout.ActivePane = pane.ID })
			ui.ScrollHorizontal(c.Key(fmt.Sprintf("workspace-tabs-%d", pane.ID))).Grow(1).Height(32).Children(func() {
				ui.Row(c).Gap(2).Children(func() {
					for _, saved := range pane.Tabs {
						tab := a.workspaceByID(saved.ID)
						if tab == nil {
							continue
						}
						ui.Row(c.Key(tab.key)).Gap(1).Children(func() {
							b := ui.Button(c.Key(fmt.Sprintf("tab-%d", tab.key)), tab.profile.Name).FontSize(11).Padding(4, 6)
							if pane.Active == tab.key {
								b.Background(t.Accent).TextColor(t.AccentText)
								if a.revealedTabs[pane.ID] != tab.key {
									b.ScrollIntoView()
									a.revealedTabs[pane.ID] = tab.key
								}
							}
							b.OnClick(func() { a.layout.Activate(tab.key); a.terminalFocus = tab.key })
							ui.Button(c.Key(fmt.Sprintf("close-tab-%d", tab.key)), "×").FontSize(11).Padding(4).Label("关闭工作区 " + tab.profile.Name).OnClick(func() { a.requestClose(tab) })
						})
					}
				})
			})
		})
		ui.Divider(c)
		tab := a.workspaceByID(pane.Active)
		if tab == nil {
			ui.Column(c).FillWidth().Grow(1).Padding(16).Gap(8).Children(func() {
				ui.Text(c, "空终端窗格").Bold()
				ui.Text(c, "从连接库打开连接，或将其他窗格的标签移到这里。").TextColor(t.TextMuted).Wrap()
			})
			return
		}
		a.sessionToolbar(c, pane, tab)
		if tab.searchOpen && tab.term != nil {
			a.terminalSearch(c, tab)
		}
		if tab.err != "" {
			ui.Text(c, tab.err).Padding(6).FontSize(11).TextColor(t.Danger).Wrap()
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
			ui.Column(c).Grow(1).FillWidth().Padding(12).Children(func() {
				ui.Text(c, "点击连接，核对服务器身份后进入终端。").TextColor(t.TextMuted).Wrap()
			})
		}
	})
	if box.Shortcut(ui.Ctrl|ui.Shift, ui.KeyF) {
		a.layout.Activate(pane.Active)
		a.toggleSearch()
	}
}

func (a *app) sessionToolbar(c *ui.Context, pane *layout.Node, tab *workspace) {
	ui.Column(c).FillWidth().Padding(5, 7).Gap(4).Children(func() {
		ui.Text(c.Key(fmt.Sprintf("session-status-%d", tab.key)), tab.profile.Name+" · "+tab.status).FontSize(11).Bold().Wrap()
		if a.layout.ActivePane != pane.ID {
			return
		}
		ui.Row(c).Gap(4).Wrap().Children(func() {
			switch tab.status {
			case "连接中":
				ui.Button(c.Key("cancel-connect"), "取消连接").FontSize(11).Padding(3, 5).OnClick(func() { a.disconnect(tab); tab.status = "已取消" })
			case "已连接":
				ui.Button(c.Key("disconnect-session"), "断开").FontSize(11).Padding(3, 5).OnClick(func() { a.disconnect(tab) })
			default:
				label := "连接"
				if tab.term != nil {
					label = "重新连接"
				}
				ui.PrimaryButton(c.Key("connect-pending"), label).FontSize(11).Padding(3, 5).OnClick(func() { a.connect(tab) })
			}
			ui.Button(c.Key("forget-credentials"), "忘记凭据").FontSize(11).Padding(3, 5).Disabled(tab.status == "连接中").OnClick(func() { a.forgetCredentials(tab.profile) })
			ui.Button(c.Key("terminal-search"), "搜索").FontSize(11).Padding(3, 5).Disabled(tab.term == nil).OnClick(a.toggleSearch)
			ui.Button(c.Key("font-smaller"), "A−").FontSize(11).Padding(3, 5).Label("缩小终端字号").Disabled(tab.term == nil).OnClick(func() { a.zoom(tab, -1, false) })
			ui.Button(c.Key("font-larger"), "A+").FontSize(11).Padding(3, 5).Label("放大终端字号").Disabled(tab.term == nil).OnClick(func() { a.zoom(tab, 1, false) })
			if _, saved := a.layout.Locate(tab.key); saved != nil {
				ui.Button(c.Key("font-reset"), fmt.Sprintf("%g", saved.FontSize)).FontSize(11).Padding(3, 5).Label(fmt.Sprintf("重置终端字号，当前 %g", saved.FontSize)).Disabled(tab.term == nil).OnClick(func() { a.zoom(tab, 0, true) })
			}
		})
	})
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
