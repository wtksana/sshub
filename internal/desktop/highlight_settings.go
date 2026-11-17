package desktop

import (
	"fmt"
	"slices"
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
		if !library.ValidHighlightColor(s) {
			return [3]uint8{}
		}
		n, _ := strconv.ParseUint(s[1:], 16, 24)
		return [3]uint8{uint8(n >> 16), uint8(n >> 8), uint8(n)}
	}
	for _, custom := range config.Custom {
		if custom.Enabled {
			rules = append(rules, connection.HighlightRule{Pattern: custom.Expression(), Light: color(custom.Color), Dark: color(custom.Color)})
		}
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
		// Persisted preferences are validated before publication; fail closed if
		// a future caller passes an invalid draft directly.
		compiled, _ = connection.CompileHighlightRules(nil)
	}
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
	ui.Text(c, "自定义规则按从上到下优先于预设。最多 32 条，每条匹配内容最多 1024 个 UTF-8 字节；过于复杂的表达式不能保存。输出过长或匹配过密时会提示并跳过部分高亮，原文完整保留。").FontSize(typeSize(c, -2)).TextColor(c.Theme().TextMuted).Wrap()
	ui.Text(c, "正则使用 Go 语法，不支持回溯引用和前后查找；零宽匹配不着色。JSON 按普通文本匹配。").FontSize(typeSize(c, -2)).TextColor(c.Theme().TextMuted).Wrap()
	for i := range p.Custom {
		rule := &p.Custom[i]
		key := fmt.Sprintf("settings-custom-%d", i)
		ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
			ui.Checkbox(c.Key(key+"-enabled"), &rule.Enabled, fmt.Sprintf("自定义规则 %d", i+1))
			ui.Row(c).Gap(12).Children(func() {
				ui.Radio(c.Key(key+"-keyword"), &rule.Kind, "keyword", "关键字")
				ui.Radio(c.Key(key+"-regex"), &rule.Kind, "regex", "正则")
				ui.Checkbox(c.Key(key+"-case"), &rule.CaseSensitive, "区分大小写")
			})
			textInput(c.Key(key+"-pattern"), &rule.Pattern).Label(fmt.Sprintf("规则 %d 匹配内容", i+1)).Placeholder("匹配内容").FillWidth().MaxLength(1024)
			highlightColorEditor(c, key+"-color", "高亮", &rule.Color)
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c.Key(key+"-up"), "上移").Disabled(i == 0).OnClick(func() { p.Custom[i], p.Custom[i-1] = p.Custom[i-1], p.Custom[i] })
				ui.Button(c.Key(key+"-down"), "下移").Disabled(i == len(p.Custom)-1).OnClick(func() { p.Custom[i], p.Custom[i+1] = p.Custom[i+1], p.Custom[i] })
				ui.Button(c.Key(key+"-delete"), "删除规则").OnClick(func() { p.Custom = slices.Delete(p.Custom, i, i+1) })
			})
		})
	}
	ui.Button(c.Key("settings-custom-add"), "新增高亮规则").Disabled(len(p.Custom) >= library.MaxCustomHighlightRules).OnClick(func() {
		p.Custom = append(p.Custom, library.CustomHighlightRule{Kind: "keyword", Enabled: true, Color: "#D29922"})
	})
	ui.Text(c, "内置预设").Bold()
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
