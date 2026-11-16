package desktop

import (
	"fmt"
	"net"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

type workspace struct {
	key     int
	profile library.Profile
}

func (a *app) center(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Clip().Children(func() {
		ui.ScrollHorizontal(c.Key("workspace-tabs")).Height(43).FillWidth().Children(func() {
			ui.Row(c).Gap(4).Padding(5, 8).Children(func() {
				if len(a.tabs) == 0 {
					ui.Text(c, "工作区").Padding(5).TextColor(t.TextMuted)
				}
				for _, tab := range a.tabs {
					ui.Row(c.Key(tab.key)).Gap(2).Children(func() {
						b := ui.Button(c.Key(fmt.Sprintf("tab-%d", tab.key)), tab.profile.Name)
						if a.activeTab == tab.key {
							b.Background(t.Accent).TextColor(t.AccentText)
						}
						b.OnClick(func() { a.activeTab = tab.key })
						ui.Button(c.Key(fmt.Sprintf("close-tab-%d", tab.key)), "×").Label("关闭工作区 " + tab.profile.Name).OnClick(func() {
							a.tabs = slices.DeleteFunc(a.tabs, func(v workspace) bool { return v.key == tab.key })
							if a.activeTab == tab.key {
								a.activeTab = 0
								if len(a.tabs) > 0 {
									a.activeTab = a.tabs[len(a.tabs)-1].key
								}
							}
						})
					})
				}
			})
		})
		ui.Divider(c)
		ui.Scroll(c.Key("profile-details")).Grow(1).FillWidth().Padding(24).Gap(14).Children(func() {
			if p, ok := a.selectedProfile(); ok {
				ui.Text(c, p.Name).FontSize(21).Bold().Wrap()
				ui.Text(c, p.Username+"@"+net.JoinHostPort(p.Host, strconv.Itoa(p.Port))).TextColor(t.TextMuted).Wrap()
				group := p.Group
				if group == "" {
					group = "未分组"
				}
				ui.Text(c, "分组："+group).Wrap()
				ui.Text(c, "认证："+authLabel(p.Auth)).Wrap()
				ui.Row(c).Gap(8).Wrap().Children(func() {
					ui.PrimaryButton(c.Key("open-workspace"), "打开工作区").OnClick(func() { a.openWorkspace(p) })
					ui.Button(c.Key("edit-profile"), "编辑").OnClick(func() { a.beginEdit(p) })
					label := "收藏"
					if p.Favorite {
						label = "取消收藏"
					}
					ui.Button(c.Key("favorite-profile"), label).OnClick(func() { a.favorite(p) })
					ui.Button(c.Key("delete-profile"), "删除").TextColor(t.Danger).OnClick(func() {
						a.deleteID, a.deleteName, a.deleting = p.ID, p.Name, true
					})
				})
			} else {
				ui.Text(c, "选择一台服务器").FontSize(21).Bold()
				ui.Text(c, "在连接库中选择连接，或添加第一台服务器。").TextColor(t.TextMuted).Wrap()
				ui.PrimaryButton(c.Key("welcome-new-profile"), "新建连接").OnClick(func() { a.beginEdit(library.Profile{}) })
			}
			ui.Divider(c)
			for _, tab := range a.tabs {
				if tab.key == a.activeTab {
					ui.Text(c, tab.profile.Name+" · 待连接").Bold().Wrap()
					ui.Text(c, "尚未建立 SSH 连接，未执行远程命令。").TextColor(t.TextMuted).Wrap()
					ui.Button(c.Key("connect-pending"), "连接").Disabled(true).Tooltip("当前版本提供连接管理，SSH 连接功能尚未接入")
				}
			}
		})
	})
}
