package desktop

import (
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) view(c *ui.Context) {
	a.focusedTerminal = 0
	t := *c.Theme()
	t.Spacing, t.Radius, t.FontSize = 4, 3, 13
	if runtime.GOOS == "windows" {
		t.Font = "Microsoft YaHei UI"
	}
	if !t.Dark {
		t.Background, t.Text, t.TextMuted, t.Border = ui.Hex("#f5f7fa"), ui.Hex("#263244"), ui.Hex("#65748a"), ui.Hex("#dce2ea")
	}
	c.SetTheme(t.WithAccent(ui.Hex("#2763b3")))
	c.DisableAnimations()
	c.SetMenuPresentation(ui.PopoverMenus)
	ui.Column(c).Fill().Children(func() {
		a.titlebar(c)
		ui.Divider(c)
		if a.loadError != "" {
			ui.Column(c).Padding(24).Gap(12).Children(func() {
				ui.Text(c, "无法读取连接库").FontSize(18).Bold()
				ui.Text(c, a.loadError).TextColor(t.Danger).Wrap()
				ui.Text(c, "现有文件未被覆盖。请检查配置文件或关闭占用该目录的实例，然后重试。").Wrap()
				ui.Text(c, a.dir).TextColor(t.TextMuted).Wrap()
				ui.Button(c.Key("retry-load"), "重试读取").OnClick(a.load)
			})
			return
		}
		if a.actionError != "" {
			ui.Row(c).Padding(8, 14).Gap(10).Children(func() {
				ui.Text(c, a.actionError).TextColor(t.Danger).Wrap().Grow(1)
				ui.Button(c.Key("dismiss-error"), "关闭提示").OnClick(func() { a.actionError = "" })
			})
		}
		if a.notice != "" {
			ui.Row(c).Padding(8, 14).Gap(10).Children(func() {
				ui.Text(c, a.notice).Wrap().Grow(1)
				ui.Button(c.Key("dismiss-notice"), "关闭提示").OnClick(func() { a.notice = "" })
			})
		}
		main := func() {
			if a.hideRight {
				a.center(c)
			} else {
				ui.SplitPercent(c.Key("files-divider"), &a.contentPercent, func() { a.center(c) }, func() { a.files(c) }).SplitReset(66).Fill()
			}
		}
		if a.hideLeft {
			ui.Column(c).Grow(1).FillWidth().Children(main)
		} else {
			ui.Split(c.Key("library-divider"), &a.leftSize, func() { a.sidebar(c) }, main).SplitReset(260).Grow(1).FillWidth()
		}
	})
	if a.focusedTerminal != a.lastFocusedTerminal {
		if a.focusedTerminal != 0 {
			a.layout.Activate(a.focusedTerminal)
			c.After(time.Millisecond)
		}
		a.lastFocusedTerminal = a.focusedTerminal
	}
	// Menu choices run while a pane is drawn. Change its tree only after all
	// siblings have been drawn, so a merge cannot invalidate that traversal.
	if len(a.layoutActions) > 0 {
		actions := a.layoutActions
		a.layoutActions = nil
		for _, action := range actions {
			action()
		}
		c.After(time.Millisecond)
	}
	if c.Shortcut(ui.Ctrl, ui.KeyB) {
		a.hideLeft = !a.hideLeft
	}
	if c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyE) {
		a.hideRight = !a.hideRight
	}
	a.editorView(c)
	a.captureLayout()
	a.sessionDialogs(c)
	if !a.confirmQuit && !a.confirmCloseTab {
		a.fileDialogs(c)
	}
	a.promptView(c)
	if ui.AlertDialog(c.Key("delete-confirm"), &a.deleting, "删除连接配置？", "将删除“"+a.deleteName+"”的已保存配置，已打开的工作区不受影响。", "取消", "删除配置") == 1 {
		if err := a.store.Delete(a.deleteID); err != nil {
			a.actionError = err.Error()
		} else {
			a.refresh()
			if a.selected == a.deleteID {
				a.selected = ""
			}
		}
	}
}
