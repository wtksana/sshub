package desktop

import (
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Clip().Padding(8).Gap(6).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "连接库").Bold().Grow(1)
			ui.Button(c.Key("new-profile"), "+").Label("新建连接").Tooltip("新建连接").Padding(3, 7).Background(ui.Transparent).BorderWidth(0).OnClick(func() { a.beginEdit(library.Profile{}) })
			ui.Button(c.Key("connection-filter"), "⋯").Label("筛选连接").Tooltip("筛选连接").Padding(3, 7).Background(ui.Transparent).BorderWidth(0).Menu(a.filterMenu)
		})
		ui.TextInput(c.Key("search-profiles"), &a.query).Label("搜索连接").Placeholder("搜索名称、地址、用户名…").FillWidth()
		if a.group != "全部分组" && a.group != "未分组" && !slices.Contains(library.Groups(a.profiles), strings.TrimPrefix(a.group, "分组：")) {
			a.group = "全部分组"
		}
		group := strings.TrimPrefix(a.group, "分组：")
		if a.group == "全部分组" || a.group == "未分组" {
			group = ""
		}
		results := library.Filter(a.profiles, a.query, group, a.scope)
		if a.group == "未分组" {
			results = slices.DeleteFunc(results, func(p library.Profile) bool { return p.Group != "" })
		}
		filter := "全部连接"
		if a.scope == "favorites" {
			filter = "收藏"
		} else if a.scope == "recent" {
			filter = "最近使用"
		}
		if a.group != "全部分组" {
			filter += " / " + a.group
		}
		suffix := ""
		if filter != "全部连接" {
			suffix = " · " + filter
		}
		ui.Textf(c, "%d / %d 条连接%s", len(results), len(a.profiles), suffix).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(filter)
		ui.List(c.Key("profile-list"), nil, len(results), func(i int) {
			p := results[i]
			row := ui.ButtonBase(c.Key("profile-"+p.ID)).Label("选择连接 "+p.Name).AccessSelected(a.selected == p.ID).FillWidth().Padding(7, 8).Radius(2)
			if a.selected == p.ID {
				row.Background(ui.Hex("#e6edf6"))
			}
			row.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					name := p.Name
					if p.Favorite {
						name = "★ " + name
					}
					ui.Text(c, name).Bold().SingleLine()
					ui.Text(c, p.Username+"@"+net.JoinHostPort(p.Host, strconv.Itoa(p.Port))).FontSize(11).SingleLine()
				})
			}).OnClick(func() { a.selected = p.ID })
			row.Tooltip(p.Name + "\n" + p.Username + "@" + net.JoinHostPort(p.Host, strconv.Itoa(p.Port)) + "\n认证：" + authLabel(p.Auth) + "\n分组：" + p.Group + "\n双击连接 · 右键管理")
			row.ContextMenu(func(m *ui.Menu) { a.profileMenu(m, p) })
			if row.DoubleClicked() {
				a.selected = p.ID
				if tab := a.openWorkspace(p); tab != nil {
					a.connect(tab)
				}
			}
		}).Grow(1).FillWidth().Children(func() {
			if len(results) == 0 {
				message := "没有匹配的连接。试试其他搜索或分组。"
				if len(a.profiles) == 0 {
					message = "还没有连接，点击“新建”添加服务器。"
				}
				ui.Text(c, message).Padding(8).TextColor(t.TextMuted).Wrap()
			}
		})
	})
}
