package desktop

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

type fileSort struct {
	column    string
	direction int
}

func (f *fileBrowser) summary() string {
	if f.loading {
		return "正在读取目录…"
	}
	if f.path != "" {
		return fmt.Sprintf("%d 项", len(f.entries))
	}
	return ""
}

func (f *fileBrowser) toggleSort(column string) {
	if f.sort.column != column {
		f.sort = fileSort{column, 1}
	} else if f.sort.direction == 1 {
		f.sort.direction = -1
	} else {
		f.sort = fileSort{}
	}
	f.applySort()
}

func (f *fileBrowser) displayEntries() []os.FileInfo {
	if f.sort.direction == 0 {
		return f.entries
	}
	return f.ordered
}

// Keep the server browse's default ordering intact so the third click restores
// it exactly. Rebuild only when a listing or sort changes, never on every frame.
func (f *fileBrowser) applySort() {
	f.ordered = nil
	if f.sort.direction == 0 {
		return
	}
	f.ordered = slices.Clone(f.entries)
	slices.SortStableFunc(f.ordered, func(a, b os.FileInfo) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		order := 0
		switch f.sort.column {
		case "name":
			order = strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
		case "modified":
			order = a.ModTime().Compare(b.ModTime())
		case "size":
			if !a.IsDir() {
				order = cmp.Compare(a.Size(), b.Size())
			}
		}
		return order * f.sort.direction
	})
}

func fileModifiedText(entry os.FileInfo) string {
	if entry.ModTime().IsZero() {
		return "—"
	}
	return entry.ModTime().Local().Format("2006-01-02 15:04")
}

func (a *app) fileHeaders(c *ui.Context, files *fileBrowser, modifiedWidth, sizeWidth float32) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(0, 8).Gap(8).MinHeight(32).Children(func() {
		for _, col := range []struct {
			key, label string
			width      float32
		}{{"name", "名称", 0}, {"modified", "修改时间", modifiedWidth}, {"size", "大小", sizeWidth}} {
			direction, next := "默认排列", "升序"
			order := ui.SortNone
			if files.sort.column == col.key {
				if files.sort.direction == 1 {
					direction, next, order = "升序", "降序", ui.SortAscending
				} else {
					direction, next, order = "降序", "默认排列", ui.SortDescending
				}
			}
			b := ui.ButtonBase(c.Key("files-sort-" + col.key)).Role(ui.RoleColumnHeader).Label(col.label + " · " + direction).Sort(order).Tooltip("点击按" + col.label + next + "；目录优先").Height(32).Padding(0).Justify(ui.Start).Gap(4).Radius(4).FontSize(typeSize(c, -2)).TextColor(t.TextMuted)
			if col.width == 0 {
				b.Grow(1).MinWidth(80)
			} else {
				b.Width(col.width).Shrink(0)
			}
			if col.key == "size" {
				b.Justify(ui.End)
			}
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.Children(func() {
				if col.key == "name" {
					ui.Box(c).Width(20).Shrink(0)
				}
				ui.Text(c, col.label).SingleLine()
				if order != ui.SortNone {
					icon := iconPrevious
					if order == ui.SortDescending {
						icon = iconNext
					}
					ui.Icon(c, icon).Size(12, 12).Shrink(0).TextColor(t.Focus)
				}
			}).OnClick(func() { files.toggleSort(col.key) })
		}
	})
	ui.Divider(c)
}
