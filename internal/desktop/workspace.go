package desktop

import (
	"fmt"
	"net"
	"strconv"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

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
							a.requestClose(tab)
						})
					})
				}
			})
		})
		ui.Divider(c)
		details := ui.Scroll(c.Key("profile-details")).FillWidth().Padding(18).Gap(10)
		if a.activeWorkspace() != nil {
			details.MaxHeight(220)
		} else {
			details.Grow(1)
		}
		details.Children(func() {
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
		})
		if tab := a.activeWorkspace(); tab != nil {
			ui.Divider(c)
			ui.Row(c).Padding(8, 12).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Text(c, tab.profile.Name+" · "+tab.status).Bold().Wrap()
				ui.Spacer(c)
				switch tab.status {
				case "连接中":
					ui.Button(c.Key("cancel-connect"), "取消连接").OnClick(func() { a.disconnect(tab); tab.status = "已取消" })
				case "已连接":
					ui.Button(c.Key("disconnect-session"), "断开").OnClick(func() { a.disconnect(tab) })
				default:
					ui.PrimaryButton(c.Key("connect-pending"), "连接").OnClick(func() { a.connect(tab) })
				}
				ui.Button(c.Key("forget-credentials"), "忘记凭据").Disabled(tab.status == "连接中").OnClick(func() { a.forgetCredentials(tab.profile) })
			})
			if tab.err != "" {
				ui.Text(c, tab.err).Padding(10).TextColor(t.Danger).Wrap()
			}
			if tab.term != nil {
				terminal.View(c.Key(tab.key), tab.term).Grow(1).FillWidth()
			} else {
				ui.Column(c).Grow(1).FillWidth().Padding(20).Children(func() {
					ui.Text(c, "连接后在此显示终端；密码和主机指纹会单独确认。").TextColor(t.TextMuted).Wrap()
				})
			}
		}
	})
}
