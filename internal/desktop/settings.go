package desktop

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

type settingsEditor struct {
	preferences       library.Preferences
	size, uiSize, err string
	resetTabs, busy   bool
}

func (a *app) setDefaultFontSize(id int) {
	if _, saved := a.layout.Locate(id); saved != nil {
		saved.FontSize = float32(a.preferences.FontSize)
	}
}

func (a *app) applyPreferences(p library.Preferences, resetTabs bool) {
	a.preferences = p.Clone()
	rules := compileHighlightPreferences(p)
	for _, tab := range a.tabs {
		if tab.highlight != nil {
			tab.highlight.SetRules(rules)
		}
		if resetTabs {
			a.setDefaultFontSize(tab.key)
		}
		if _, saved := a.layout.Locate(tab.key); saved != nil && tab.term != nil {
			tab.term.SetFont(terminal.Font{Family: p.FontFamily, Size: saved.FontSize})
		}
	}
}

func (a *app) saveSettings(e *settingsEditor) {
	p := e.preferences.Clone()
	size, err := strconv.Atoi(strings.TrimSpace(e.size))
	if err != nil {
		e.err = "终端默认字号必须为 8–32 的整数"
		return
	}
	uiSize, err := strconv.Atoi(strings.TrimSpace(e.uiSize))
	if err != nil || uiSize < 10 || uiSize > 20 {
		e.err = "界面字号必须为 10–20 的整数"
		return
	}
	p.UIFontSize, p.UIFontFamily = uiSize, strings.TrimSpace(p.UIFontFamily)
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
	e.preferences.ScrollbackMiB = e.preferences.EffectiveScrollbackMiB()
	if e.preferences.LogHighlight == nil {
		config := e.preferences.EffectiveLogHighlight()
		e.preferences.LogHighlight = &config
	}
	open := true
	ui.DialogBase(c.Key("settings-dialog"), &open, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(ui.RGBA(0, 0, 0, .35))
		dialogStyle(c, panel, 560)
		ui.Text(c, "设置").FontSize(typeSize(c, 5)).Bold()
		ui.Scroll(c.Key("settings-fields")).FillWidth().MaxHeight(450).Gap(16).Disabled(e.busy).Children(func() {
			formField(c, "外观", func() {
				ui.Row(c).Gap(12).Children(func() {
					for _, choice := range []struct{ value, label string }{{"system", "跟随系统"}, {"light", "GitHub Light"}, {"dark", "GitHub Dark"}} {
						ui.Radio(c.Key("theme-"+choice.value), &e.preferences.Theme, choice.value, choice.label)
					}
				})
			})
			ui.Divider(c)
			ui.Text(c, "界面").Bold()
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					formField(c, "字体", func() {
						textInput(c.Key("settings-ui-font"), &e.preferences.UIFontFamily).Placeholder("系统默认字体").FillWidth().MaxLength(256)
					})
				})
				ui.Column(c).Width(max(130, c.Theme().FontSize*9)).Children(func() {
					formField(c, "字号（10–20）", func() { textInput(c.Key("settings-ui-size"), &e.uiSize).FillWidth().MaxLength(3) })
				})
			})
			ui.Divider(c)
			ui.Text(c, "终端").Bold()
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					formField(c, "终端字体", func() { textInput(c.Key("settings-font"), &e.preferences.FontFamily).FillWidth().MaxLength(256) })
				})
				ui.Column(c).Width(max(130, c.Theme().FontSize*9)).Children(func() {
					formField(c, "字号（8–32）", func() { textInput(c.Key("settings-size"), &e.size).FillWidth().MaxLength(3) })
				})
			})
			ui.Text(c, "使用本机已安装字体；缺失字符由系统回退显示。").FontSize(typeSize(c, -2)).TextColor(t.TextMuted).Wrap()
			ui.Checkbox(c.Key("settings-reset-tabs"), &e.resetTabs, "同时将现有标签字号设为默认")
			formField(c, "历史缓冲", func() {
				ui.Row(c).Gap(12).Children(func() {
					for _, capacity := range []int{2, 8, 32, 64} {
						ui.Radio(c.Key("settings-scrollback-"+strconv.Itoa(capacity)), &e.preferences.ScrollbackMiB, capacity, strconv.Itoa(capacity)+" MiB")
					}
				})
			})
			ui.Text(c, "仅新建或重新连接的终端生效；现有终端不变。按容量保留最近输出，非固定行数；实际内存占用可能更多。").FontSize(typeSize(c, -2)).TextColor(t.TextMuted).Wrap()
			ui.Divider(c)
			ui.Text(c, "会话").Bold()
			ui.Checkbox(c.Key("settings-skip-close-confirmation"), &e.preferences.SkipCloseConfirmation, "关闭终端会话时不再提示")
			highlightSettingsView(c, e.preferences.LogHighlight)
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
