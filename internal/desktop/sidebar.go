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
	ui.Column(c).Fill().Clip().Background(t.Surface).Padding(12).Gap(8).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "连接").Bold().Grow(1)
			iconButton(c.Key("new-profile"), iconPlus, "新建连接").OnClick(func() { a.beginEdit(library.Profile{}) })
			iconButton(c.Key("connection-filter"), iconFilter, "筛选连接").Menu(a.filterMenu)
		})
		textInput(c.Key("search-profiles"), &a.query).Label("搜索连接").Placeholder("搜索名称、地址、用户名…").FillWidth()
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
		ui.Textf(c, "%d / %d 条连接%s", len(results), len(a.profiles), suffix).FontSize(typeSize(c, -2)).TextColor(t.TextMuted).SingleLine().Tooltip(filter)
		rows := a.profileRows(results)
		ui.List(c.Key("profile-list"), nil, len(rows)).ItemKey(func(i int) any {
			return rows[i].key()
		}).Rows(func(row ui.ListRow) {
			item := rows[row.Index]
			if item.profile.ID == "" {
				a.profileGroupHeader(row.Context, item.group)
			} else {
				a.profileRow(row.Context, item.profile)
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

func (a *app) profileRow(c *ui.Context, p library.Profile) {
	t := c.Theme()
	row := ui.ButtonBase(c.Key("profile-"+p.ID)).Drag(profileDrag{p.ID, p.Group}).Label("选择连接 "+p.Name).AccessSelected(a.selected == p.ID).FillWidth().Padding(7, 8).Radius(6)
	if a.profileDrop(row, p.Group) || a.selected == p.ID {
		row.Background(t.Selection)
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				if p.Favorite {
					ui.Icon(c, iconStar).Size(14, 14).Shrink(0).TextColor(t.TextMuted)
				}
				ui.Text(c, p.Name).Bold().SingleLine()
			})
			ui.Text(c, p.Username+"@"+net.JoinHostPort(p.Host, strconv.Itoa(p.Port))).FontSize(typeSize(c, -2)).SingleLine()
		})
	}).OnClick(func() { a.selected = p.ID })
	row.Tooltip(p.Name + "\n" + p.Username + "@" + net.JoinHostPort(p.Host, strconv.Itoa(p.Port)) + "\n认证：" + authLabel(p.Auth) + "\n分组：" + p.Group + "\n双击连接 · 拖动改组 · 右键管理")
	row.ContextMenu(func(m *ui.Menu) { a.profileMenu(m, p) })
	if row.DoubleClicked() {
		a.selected = p.ID
		if tab := a.openTab(p); tab != nil {
			a.connect(tab)
		}
	}
}
