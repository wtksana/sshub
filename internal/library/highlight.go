package library

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"sshub/internal/highlight"
)

type LogHighlightPreferences struct {
	Builtins []BuiltinHighlightRule `json:"builtins"`
	Custom   []CustomHighlightRule  `json:"custom,omitempty"`
}

const MaxCustomHighlightRules = 32

type CustomHighlightRule struct {
	Kind          string `json:"kind"`
	Pattern       string `json:"pattern"`
	Enabled       bool   `json:"enabled"`
	CaseSensitive bool   `json:"case_sensitive"`
	Color         string `json:"color"`
}

func (r CustomHighlightRule) Expression() string {
	p := r.Pattern
	if r.Kind == "keyword" {
		p = regexp.QuoteMeta(p)
	}
	if !r.CaseSensitive {
		p = "(?i)" + p
	}
	return p
}

type BuiltinHighlightRule struct {
	ID         string `json:"id"`
	Enabled    bool   `json:"enabled"`
	LightColor string `json:"light_color"`
	DarkColor  string `json:"dark_color"`
}

func DefaultLogHighlightPreferences() LogHighlightPreferences {
	return LogHighlightPreferences{Builtins: []BuiltinHighlightRule{
		{ID: "trace", Enabled: true, LightColor: "#57606A", DarkColor: "#8B949E"},
		{ID: "debug", Enabled: true, LightColor: "#8250DF", DarkColor: "#BC8CFF"},
		{ID: "info", Enabled: true, LightColor: "#1A7F37", DarkColor: "#3FB950"},
		{ID: "warn", Enabled: true, LightColor: "#9A6700", DarkColor: "#D29922"},
		{ID: "error", Enabled: true, LightColor: "#CF222E", DarkColor: "#F85149"},
		{ID: "fatal", Enabled: true, LightColor: "#CF222E", DarkColor: "#F85149"},
		{ID: "date", Enabled: true, LightColor: "#0969DA", DarkColor: "#58A6FF"},
	}}
}

func (p Preferences) Clone() Preferences {
	if p.LogHighlight != nil {
		copy := *p.LogHighlight
		copy.Builtins = slices.Clone(copy.Builtins)
		copy.Custom = slices.Clone(copy.Custom)
		p.LogHighlight = &copy
	}
	return p
}

func (p Preferences) EffectiveLogHighlight() LogHighlightPreferences {
	if p.LogHighlight == nil {
		return DefaultLogHighlightPreferences()
	}
	return *p.Clone().LogHighlight
}

func ValidHighlightColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(color[1:], 16, 24)
	return err == nil
}

func validateLogHighlight(p LogHighlightPreferences) error {
	if len(p.Custom) > MaxCustomHighlightRules {
		return errors.New("自定义日志高亮最多 32 条规则")
	}
	// Reserve the fixed presets even when disabled, so toggles preserve validity.
	total := 139
	for i, rule := range p.Custom {
		if rule.Kind != "keyword" && rule.Kind != "regex" {
			return fmt.Errorf("自定义规则 %d：未知规则类型", i+1)
		}
		if !ValidHighlightColor(rule.Color) {
			return fmt.Errorf("自定义规则 %d：颜色必须为 #RRGGBB", i+1)
		}
		if rule.Pattern == "" || len(rule.Pattern) > highlight.MaxPatternBytes {
			return fmt.Errorf("自定义规则 %d：匹配内容不能为空，且最多为 1024 个 UTF-8 字节", i+1)
		}
		_, cost, err := highlight.Compile(rule.Expression())
		if err != nil {
			return fmt.Errorf("自定义规则 %d：%w", i+1, err)
		}
		total += cost
		if total > highlight.MaxTotalInstructions {
			return errors.New("日志高亮规则整体过于复杂，请减少规则或简化表达式")
		}
	}
	defaults := DefaultLogHighlightPreferences().Builtins
	if len(p.Builtins) != len(defaults) {
		return errors.New("日志高亮预设必须包含全部七项规则")
	}
	seen := make(map[string]bool, len(defaults))
	for _, rule := range p.Builtins {
		if seen[rule.ID] || !slices.ContainsFunc(defaults, func(v BuiltinHighlightRule) bool { return v.ID == rule.ID }) {
			return errors.New("日志高亮预设包含未知或重复规则")
		}
		seen[rule.ID] = true
		if !ValidHighlightColor(rule.LightColor) || !ValidHighlightColor(rule.DarkColor) {
			return errors.New("日志高亮颜色必须为 #RRGGBB 格式的不透明颜色")
		}
	}
	return nil
}
