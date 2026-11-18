package desktop

import (
	"cmp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

type profileListItem struct {
	group   string
	profile library.Profile
}

func (i profileListItem) key() string {
	if i.profile.ID != "" {
		return "profile:" + i.profile.ID
	}
	return "group:" + i.group
}

func (a *app) profileRows(results []library.Profile) []profileListItem {
	groups := make(map[string][]library.Profile)
	for _, p := range results {
		groups[p.Group] = append(groups[p.Group], p)
	}
	names := library.Groups(results)
	if len(groups[""]) != 0 || len(names) != 0 && a.group == "全部分组" && a.query == "" && a.scope == "" {
		names = append(names, "")
	}
	// New and imported profiles append to the stored list. Edits keep their slot.
	added := make(map[string]int, len(a.profiles))
	for i, p := range a.profiles {
		added[p.ID] = i
	}
	rows := make([]profileListItem, 0, len(names)+len(results))
	for _, group := range names {
		mode := a.layout.GroupSort[group]
		profiles := groups[group]
		slices.SortStableFunc(profiles, func(p, q library.Profile) int {
			switch mode {
			case "recent":
				if n := q.LastUsed.Compare(p.LastUsed); n != 0 {
					return n
				}
			case "added":
				return cmp.Compare(added[q.ID], added[p.ID])
			}
			return strings.Compare(strings.ToLower(p.Name), strings.ToLower(q.Name))
		})
		rows = append(rows, profileListItem{group: group})
		for _, p := range profiles {
			rows = append(rows, profileListItem{group: group, profile: p})
		}
	}
	return rows
}

func (a *app) profileGroupHeader(c *ui.Context, group string) {
	label := group
	if group == "" {
		label = "未分组"
	}
	t := c.Theme()
	ui.Column(c).FillWidth().Padding(8, 0, 2, 0).Gap(3).Children(func() {
		ui.Divider(c)
		header := ui.Row(c).FillWidth().Padding(2, 4, 2, 8).AlignItems(ui.Center).Radius(4)
		if a.profileDrop(header, group) {
			header.Background(t.Selection)
		}
		header.Children(func() {
			ui.Text(c, label).Bold().SingleLine().Grow(1).MinWidth(0).TextColor(t.TextMuted).Tooltip(label)
			iconButton(c.Key("group-sort-"+group), iconMore, label+"排序").Size(24, 24).Menu(func(m *ui.Menu) {
				for _, sort := range []struct{ mode, label string }{{"", "按名称"}, {"recent", "按最近使用"}, {"added", "按添加时间"}} {
					if m.Item(sort.label).ID("group-sort-" + sort.mode).Radio(a.layout.GroupSort[group] == sort.mode).Chosen() {
						if sort.mode == "" {
							delete(a.layout.GroupSort, group)
						} else {
							if a.layout.GroupSort == nil {
								a.layout.GroupSort = make(map[string]string)
							}
							a.layout.GroupSort[group] = sort.mode
						}
					}
				}
			})
		})
	})
}

type profileDrag struct{ id, group string }

func (a *app) profileDrop(e ui.Element, group string) bool {
	if moved, ok := ui.Drop[profileDrag](e); ok && moved.group != group {
		if err := a.setProfileGroup(moved.id, group); err != nil {
			a.actionError = err.Error()
		} else {
			a.selected = moved.id
		}
	}
	dragging, over := ui.DragOver[profileDrag](e)
	return over && dragging.group != group
}
