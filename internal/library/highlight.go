package library

import (
	"errors"
	"slices"
	"strconv"
)

type LogHighlightPreferences struct {
	Builtins []BuiltinHighlightRule `json:"builtins"`
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
