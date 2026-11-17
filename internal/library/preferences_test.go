package library

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSeparateFontsRoundTripAndLegacyDefaults(t *testing.T) {
	var legacy Preferences
	if err := json.Unmarshal([]byte(`{"theme":"dark","font_family":"Consolas","font_size":18}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePreferences(legacy); err != nil || legacy.EffectiveUIFontSize() != 13 || legacy.FontSize != 18 || legacy.SkipCloseConfirmation {
		t.Fatalf("legacy preferences: %+v, %v", legacy, err)
	}
	prefs := legacy
	prefs.UIFontFamily, prefs.UIFontSize = "Microsoft YaHei UI", 16
	prefs.SkipCloseConfirmation = true
	dir := t.TempDir()
	store := openTestStore(t, dir)
	if err := store.SavePreferences(prefs); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store = openTestStore(t, dir)
	if store.Preferences() != prefs {
		t.Fatal("separate fonts did not survive restart")
	}
	data, err := store.Export()
	if err != nil {
		t.Fatal(err)
	}
	incoming, err := DecodeMigration(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	other := openTestStore(t, t.TempDir())
	plan, err := other.PreviewImport(*incoming)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = other.CommitImport(plan, nil, true); err != nil {
		t.Fatal(err)
	}
	if other.Preferences() != prefs {
		t.Fatal("migration lost separate fonts")
	}
	for _, bad := range []Preferences{
		{Theme: "light", FontFamily: "Consolas", FontSize: 13, UIFontSize: 9},
		{Theme: "light", FontFamily: "Consolas", FontSize: 13, UIFontSize: 21},
		{Theme: "light", FontFamily: "Consolas", FontSize: 13, UIFontFamily: "bad\nfont"},
	} {
		if store.SavePreferences(bad) == nil || store.Preferences() != prefs {
			t.Fatal("invalid UI font settings changed saved preferences")
		}
	}
}
