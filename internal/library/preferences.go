package library

import (
	"errors"
	"strings"
	"unicode"
)

type Preferences struct {
	Theme                 string `json:"theme"`
	FontFamily            string `json:"font_family"`
	FontSize              int    `json:"font_size"`
	UIFontFamily          string `json:"ui_font_family,omitempty"`
	UIFontSize            int    `json:"ui_font_size,omitempty"`
	ScrollbackMiB         int    `json:"scrollback_mib,omitempty"`
	SkipCloseConfirmation bool   `json:"skip_close_confirmation,omitempty"`
}

func DefaultPreferences() Preferences {
	return Preferences{Theme: "system", FontFamily: "Cascadia Mono", FontSize: 13}
}

func ValidatePreferences(p Preferences) error {
	if p.Theme != "system" && p.Theme != "light" && p.Theme != "dark" {
		return errors.New("请选择跟随系统、浅色或深色主题")
	}
	if strings.TrimSpace(p.FontFamily) == "" || len(p.FontFamily) > 256 || strings.IndexFunc(p.FontFamily, unicode.IsControl) >= 0 {
		return errors.New("请输入有效的终端字体名称")
	}
	if p.FontSize < 8 || p.FontSize > 32 {
		return errors.New("终端默认字号必须为 8–32")
	}
	if p.UIFontFamily != "" && (strings.TrimSpace(p.UIFontFamily) == "" || len(p.UIFontFamily) > 256 || strings.IndexFunc(p.UIFontFamily, unicode.IsControl) >= 0) {
		return errors.New("请输入有效的界面字体名称")
	}
	if p.UIFontSize != 0 && (p.UIFontSize < 10 || p.UIFontSize > 20) {
		return errors.New("界面字号必须为 10–20")
	}
	if p.ScrollbackMiB != 0 && p.ScrollbackMiB != 2 && p.ScrollbackMiB != 8 && p.ScrollbackMiB != 32 && p.ScrollbackMiB != 64 {
		return errors.New("终端历史缓冲必须为 2、8、32 或 64 MiB")
	}
	return nil
}

func (p Preferences) EffectiveScrollbackMiB() int {
	if p.ScrollbackMiB == 0 {
		return 2
	}
	return p.ScrollbackMiB
}

// UI defaults also apply to preferences saved before separate UI fonts existed.
func (p Preferences) EffectiveUIFontSize() int {
	if p.UIFontSize == 0 {
		return 13
	}
	return p.UIFontSize
}

func (s *Store) Preferences() Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()
	return preferences(s.doc)
}

func preferences(d Document) Preferences {
	if d.Preferences == nil {
		return DefaultPreferences()
	}
	return *d.Preferences
}

func (s *Store) SavePreferences(p Preferences) error {
	if err := ValidatePreferences(p); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.doc
	next.Preferences = &p
	return s.save(next)
}
