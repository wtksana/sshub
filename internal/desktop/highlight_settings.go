package desktop

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"sshub/internal/connection"
	"sshub/internal/library"
)

// Date shapes are deliberately bounded; this is highlighting, not calendar validation.
const highlightDatePattern = `(?:[0-9]{4}[-/][0-9]{2}[-/][0-9]{2}(?:[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(?:[.,][0-9]{1,9})?(?:Z|[+-][0-9]{2}:?[0-9]{2})?)?|[0-9]{2}:[0-9]{2}:[0-9]{2}(?:[.,][0-9]{1,9})?(?:Z|[+-][0-9]{2}:?[0-9]{2})?)`

func compileHighlightPreferences(p library.Preferences) *connection.HighlightRules {
	config := p.EffectiveLogHighlight()
	var rules []connection.HighlightRule
	color := func(s string) [3]uint8 {
		n, _ := strconv.ParseUint(s[1:], 16, 24)
		return [3]uint8{uint8(n >> 16), uint8(n >> 8), uint8(n)}
	}
	// Preset order is fixed even when an imported file uses another list order.
	for _, id := range []string{"trace", "debug", "info", "warn", "error", "fatal", "date"} {
		for _, preset := range config.Builtins {
			if preset.ID != id || !preset.Enabled {
				continue
			}
			pattern := "(?i)" + id
			switch id {
			case "warn":
				pattern = `(?i)warn(?:ing)?`
			case "fatal":
				pattern = `(?i)(?:fatal|panic)`
			case "date":
				pattern = highlightDatePattern
			}
			rules = append(rules, connection.HighlightRule{Pattern: pattern, Light: color(preset.LightColor), Dark: color(preset.DarkColor), WholeWord: true})
		}
	}
	compiled, err := connection.CompileHighlightRules(rules)
	if err != nil {
		panic(err)
	} // All patterns above are fixed program constants.
	return compiled
}

func (a *app) beginSettings() {
	p := a.preferences.Clone()
	config := p.EffectiveLogHighlight()
	p.LogHighlight = &config
	a.settings = &settingsEditor{preferences: p, size: fmt.Sprint(p.FontSize), uiSize: fmt.Sprint(p.EffectiveUIFontSize())}
}

func highlightPresetLabel(id string) string {
	switch id {
	case "warn":
		return "WARN / WARNING"
	case "fatal":
		return "FATAL / PANIC"
	case "date":
		return "日期 / 时间"
	default:
		return strings.ToUpper(id)
	}
}

func highlightColorEditor(c *ui.Context, key, label string, value *string) {
	formField(c, label, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			color := ui.RGB(0, 0, 0)
			if library.ValidHighlightColor(*value) {
				color = ui.Hex(*value)
			}
			if ui.ColorWell(c.Key(key+"-picker"), &color).Label(label).Changed() {
				*value = fmt.Sprintf("#%02X%02X%02X", color.R, color.G, color.B)
			}
			textInput(c.Key(key), value).Label(label + "颜色 #RRGGBB").Grow(1).Basis(0).MaxLength(9)
		})
	})
}

func highlightSettingsView(c *ui.Context, p *library.LogHighlightPreferences) {
	ui.Divider(c)
	ui.Text(c, "日志高亮规则").Bold()
	ui.Text(c, "右键开启当前终端的日志高亮；规则全局共用，仅影响新增输出，历史颜色保留。颜色格式：#RRGGBB。").FontSize(typeSize(c, -2)).TextColor(c.Theme().TextMuted).Wrap()
	for i := range p.Builtins {
		preset := &p.Builtins[i]
		key := "settings-highlight-" + preset.ID
		ui.Checkbox(c.Key(key+"-enabled"), &preset.Enabled, highlightPresetLabel(preset.ID))
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Children(func() { highlightColorEditor(c, key+"-light", "浅色主题", &preset.LightColor) })
			ui.Column(c).Grow(1).Basis(0).Children(func() { highlightColorEditor(c, key+"-dark", "深色主题", &preset.DarkColor) })
		})
	}
}
