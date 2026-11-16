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
	ui.Column(c).Fill().Clip().Padding(12).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "连接库").Bold().Grow(1)
			ui.PrimaryButton(c.Key("new-profile"), "新建").OnClick(func() { a.beginEdit(library.Profile{}) })
		})
		ui.TextInput(c.Key("search-profiles"), &a.query).Label("搜索连接").Placeholder("搜索名称、地址、用户名…").FillWidth()
		ui.Row(c).Gap(4).Children(func() {
			for _, scope := range []struct{ value, label string }{{"", "全部"}, {"favorites", "收藏"}, {"recent", "最近"}} {
				button := ui.Button(c.Key("scope-"+scope.value), scope.label).Grow(1).Basis(0)
				if a.scope == scope.value {
					button.Background(t.Accent).TextColor(t.AccentText)
				}
				button.OnClick(func() { a.scope = scope.value })
			}
		})
		groups := []string{"全部分组", "未分组"}
		for _, name := range library.Groups(a.profiles) {
			groups = append(groups, "分组："+name)
		}
		if !slices.Contains(groups, a.group) {
			a.group = "全部分组"
		}
		ui.Select(c.Key("group-filter"), &a.group, groups).Label("筛选分组").FillWidth()
		group := strings.TrimPrefix(a.group, "分组：")
		if a.group == "全部分组" || a.group == "未分组" {
			group = ""
		}
		results := library.Filter(a.profiles, a.query, group, a.scope)
		if a.group == "未分组" {
			results = slices.DeleteFunc(results, func(p library.Profile) bool { return p.Group != "" })
		}
		ui.Textf(c, "%d / %d 条连接", len(results), len(a.profiles)).FontSize(11).TextColor(t.TextMuted)
		ui.List(c.Key("profile-list"), nil, len(results), func(i int) {
			p := results[i]
			row := ui.ButtonBase(c.Key("profile-"+p.ID)).Label("选择连接 "+p.Name).FillWidth().Padding(9, 8).Radius(4)
			if a.selected == p.ID {
				row.Background(t.Accent).TextColor(t.AccentText)
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
		}).Grow(1).FillWidth().Children(func() {
			if len(results) == 0 {
				message := "没有匹配的连接。试试其他搜索或分组。"
				if len(a.profiles) == 0 {
					message = "还没有连接，点击“新建”添加服务器。"
				}
				ui.Text(c, message).Padding(8).TextColor(t.TextMuted).Wrap()
			}
		})
		ui.Text(c, "配置保存在本机").FontSize(11).TextColor(t.TextMuted).Tooltip(a.dir)
	})
}
