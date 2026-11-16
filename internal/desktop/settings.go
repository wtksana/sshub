package desktop

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

type settingsEditor struct {
	preferences     library.Preferences
	size, err       string
	resetTabs, busy bool
}

func (a *app) setDefaultFontSize(id int) {
	if _, saved := a.layout.Locate(id); saved != nil {
		saved.FontSize = float32(a.preferences.FontSize)
	}
}

func (a *app) applyPreferences(p library.Preferences, resetTabs bool) {
	a.preferences = p
	for _, tab := range a.tabs {
		if resetTabs {
			a.setDefaultFontSize(tab.key)
		}
		if _, saved := a.layout.Locate(tab.key); saved != nil && tab.term != nil {
			tab.term.SetFont(terminal.Font{Family: p.FontFamily, Size: saved.FontSize})
		}
	}
}

func (a *app) saveSettings(e *settingsEditor) {
	p := e.preferences
	size, err := strconv.Atoi(strings.TrimSpace(e.size))
	if err != nil {
		e.err = "终端默认字号必须为 8–32 的整数"
		return
	}
	p.FontSize, p.FontFamily = size, strings.TrimSpace(p.FontFamily)
	if err := library.ValidatePreferences(p); err != nil {
		e.err = err.Error()
		return
	}
	e.busy, e.err = true, ""
	reset := e.resetTabs
	go func() {
		err := a.store.SavePreferences(p)
		a.dispatch(func() {
			e.busy = false
			if err != nil {
				e.err = err.Error()
				return
			}
			a.applyPreferences(p, reset)
			a.settings = nil
			a.notice = "设置已保存"
		})
	}()
}

func (a *app) settingsView(c *ui.Context) {
	e := a.settings
	if e == nil {
		return
	}
	open := true
	ui.DialogBase(c.Key("settings-dialog"), &open, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(ui.RGBA(0, 0, 0, .35))
		panel.Width(500).MaxWidthPercent(94).MaxHeightPercent(94).Padding(20).Gap(12).Radius(6).Background(t.Background).Border(1, t.Border)
		ui.Text(c, "设置").FontSize(18).Bold()
		ui.Column(c).FillWidth().Gap(12).Disabled(e.busy).Children(func() {
			ui.Field(c, "外观", func() {
				ui.Row(c).Gap(12).Children(func() {
					for _, choice := range []struct{ value, label string }{{"system", "跟随系统"}, {"light", "浅色"}, {"dark", "深色"}} {
						ui.Radio(c.Key("theme-"+choice.value), &e.preferences.Theme, choice.value, choice.label)
					}
				})
			})
			ui.Field(c, "终端字体", func() { ui.TextInput(c.Key("settings-font"), &e.preferences.FontFamily).FillWidth().MaxLength(256) })
			ui.Text(c, "使用本机已安装的字体；找不到的字符由系统字体回退显示。").FontSize(11).TextColor(t.TextMuted).Wrap()
			ui.Field(c, "默认字号（8–32）", func() { ui.TextInput(c.Key("settings-size"), &e.size).FillWidth().MaxLength(3) })
			ui.Checkbox(c.Key("settings-reset-tabs"), &e.resetTabs, "同时将现有标签字号设为默认")
		})
		if e.err != "" {
			ui.Text(c, e.err).TextColor(t.Danger).Wrap()
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("settings-cancel"), "取消").Disabled(e.busy).OnClick(func() { a.settings = nil })
			ui.PrimaryButton(c.Key("settings-save"), "保存设置").Disabled(e.busy).OnClick(func() { a.saveSettings(e) })
		})
	})
	if !open && !e.busy {
		a.settings = nil
	}
}
