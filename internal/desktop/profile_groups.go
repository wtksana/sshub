package desktop

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

type profileGroupEditor struct {
	id, value, err string
}

func (a *app) groupInput(c *ui.Context, key string, value *string) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
		cb := ui.ComboboxBase(c.Key(key), value)
		cb.Input.Label("分组").Placeholder("选择或输入新分组，留空为未分组").Grow(1).MinWidth(0).MinHeight(max(36, t.FontSize+18)).Padding(8, 10).Radius(6).Background(t.Background).Border(1, t.Border).MaxLength(1024)
		if cb.Input.Focused() {
			cb.Input.Border(1, t.Focus)
		}
		iconButton(c.Key(key+"-options"), iconNext, "选择已有分组").OnClick(func() {
			cb.Input.Focus()
			cb.SetOpen(!cb.Open())
		})
		cb.Popup(func(panel ui.Element) {
			panel.Padding(4).Gap(2).MaxHeight(200).Background(t.Background).Border(1, t.Border).Radius(6)
			for _, group := range library.Groups(a.profiles) {
				if cb.Filtering() && !strings.Contains(strings.ToLower(group), strings.ToLower(*value)) {
					continue
				}
				item := cb.Item(group).Padding(6, 10).Radius(4)
				if item.Highlighted() {
					item.Background(t.Selection)
				}
				item.Children(func() { ui.Text(c, group) })
			}
		})
		if chosen, ok := cb.Chosen(); ok {
			*value = chosen
		}
	})
}

func (a *app) profileGroupMenu(m *ui.Menu, p library.Profile) {
	set := func(group string) {
		if err := a.setProfileGroup(p.ID, group); err != nil {
			a.actionError = err.Error()
		}
	}
	if m.Item("未分组").ID("profile-group-none").Radio(p.Group == "").Chosen() {
		set("")
	}
	for i, group := range library.Groups(a.profiles) {
		if m.Item(group).ID(fmt.Sprintf("profile-group-%d", i)).Radio(p.Group == group).Chosen() {
			set(group)
		}
	}
	m.Separator()
	if m.Item("新建分组…").ID("profile-group-new").Chosen() {
		a.groupEdit = &profileGroupEditor{id: p.ID}
	}
}

func (a *app) setProfileGroup(id, group string) error {
	// Read the current profile so changing its group cannot restore stale auth data.
	for _, p := range a.store.Profiles() {
		if p.ID != id {
			continue
		}
		p.Group = group
		if _, err := a.store.Upsert(p); err != nil {
			return err
		}
		a.refresh()
		return nil
	}
	return fmt.Errorf("连接配置已不存在，请重新选择")
}

func (a *app) groupDialog(c *ui.Context) {
	e := a.groupEdit
	if e == nil {
		return
	}
	open := true
	ui.DialogBase(c.Key("profile-group-dialog"), &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .35))
		dialogStyle(c, panel, 400)
		ui.Text(c, "设置连接分组").Bold().FontSize(typeSize(c, 3))
		a.groupInput(c, "quick-group", &e.value)
		if e.err != "" {
			ui.Text(c, e.err).TextColor(c.Theme().Danger).Wrap()
		}
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("quick-group-cancel"), "取消").OnClick(func() { a.groupEdit = nil })
			ui.PrimaryButton(c.Key("quick-group-save"), "保存分组").OnClick(func() {
				if err := a.setProfileGroup(e.id, e.value); err != nil {
					e.err = err.Error()
					return
				}
				a.groupEdit = nil
			})
		})
	})
	if !open {
		a.groupEdit = nil
	}
}
