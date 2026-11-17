package desktop

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *app) terminalContextMenu(c *ui.Context, element ui.Element, tab *sessionTab) {
	// MyGo's terminal has private edit actions. Native menu roles dispatch
	// those actions to the focused terminal through its public input handler.
	element.Children(func() {
		target := ui.Box(c.Key("terminal-context-target")).Absolute().Top(0).Left(0).Fill()
		open := false
		ui.ContextMenuBase(c, target, &open)
		if open {
			element.Focus()
			a.showTerminalMenu(tab)
		}
	})
}

func (a *app) showTerminalMenu(tab *sessionTab) {
	term, highlight := tab.term, tab.highlight
	if a.win == nil || term == nil {
		return
	}
	// Build first, then post the native popup after the frame. It must not
	// enter a nested native loop while MyGo is building the terminal view.
	a.win.Update(func() {
		if a.closing.Load() || tab.term != term {
			return
		}
		mygo.NewMenu([]*mygo.MenuItem{
			{Label: "复制", Role: mygo.RoleCopy, Accelerator: "Ctrl+Shift+C"},
			{Label: "粘贴", Role: mygo.RolePaste, Accelerator: "Ctrl+Shift+V"},
			{Label: "全选", Role: mygo.RoleSelectAll, Accelerator: "Ctrl+Shift+A"},
			mygo.Separator(),
			{Label: "清屏", Click: func(*mygo.MenuItem, *mygo.Window) { term.ClearScreen() }},
			mygo.Separator(),
			{ID: "log-highlighting", Label: "日志高亮", Type: mygo.MenuItemCheckbox, Accelerator: "Ctrl+Shift+H",
				Checked: highlight != nil && highlight.Enabled(), Disabled: highlight == nil,
				Click: func(item *mygo.MenuItem, _ *mygo.Window) {
					if tab.highlight == highlight && highlight != nil {
						highlight.SetEnabled(item.IsChecked())
					}
				}},
		}).Popup(a.win)
	})
}
