package library

import (
	"errors"
	"strings"
	"unicode"
)

type Preferences struct {
	Theme      string `json:"theme"`
	FontFamily string `json:"font_family"`
	FontSize   int    `json:"font_size"`
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
	return nil
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
