package library

import (
	"bytes"
	"encoding/json"
	"os"
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

func TestScrollbackLegacyDefaultsAndRoundTrip(t *testing.T) {
	var legacy Preferences
	if err := json.Unmarshal([]byte(`{"theme":"dark","font_family":"Consolas","font_size":18}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePreferences(legacy); err != nil || legacy.EffectiveScrollbackMiB() != 2 || DefaultPreferences().EffectiveScrollbackMiB() != 2 {
		t.Fatal("legacy scrollback default", legacy, err)
	}
	for _, capacity := range []int{2, 8, 32, 64} {
		prefs := legacy
		prefs.ScrollbackMiB = capacity
		dir := t.TempDir()
		store := openTestStore(t, dir)
		if err := store.SavePreferences(prefs); err != nil {
			t.Fatal(err)
		}
		store.Close()
		store = openTestStore(t, dir)
		if got := store.Preferences(); got != prefs || got.EffectiveScrollbackMiB() != capacity {
			t.Fatal("scrollback lost after restart", got)
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
			t.Fatal("migration lost scrollback capacity")
		}
	}
}

func TestInvalidScrollbackAndExternalWriteLeavePreferencesUnchanged(t *testing.T) {
	store := openTestStore(t, t.TempDir())
	prefs := DefaultPreferences()
	prefs.ScrollbackMiB = 8
	if err := store.SavePreferences(prefs); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, capacity := range []int{-1, 1, 4, 16, 65, 1 << 30} {
		bad := prefs
		bad.ScrollbackMiB = capacity
		if store.SavePreferences(bad) == nil || store.Preferences() != prefs {
			t.Fatal("invalid capacity changed preferences", capacity)
		}
		data, err := json.Marshal(Migration{Format: "sshub", Version: 1, Profiles: []PortableProfile{}, Preferences: &bad})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeMigration(bytes.NewReader(data)); err == nil {
			t.Fatal("invalid imported capacity accepted", capacity)
		}
	}
	after, err := os.ReadFile(store.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("invalid capacity changed disk", err)
	}
	external := append(before, '\n')
	if err = os.WriteFile(store.path, external, 0600); err != nil {
		t.Fatal(err)
	}
	next := prefs
	next.ScrollbackMiB = 64
	if err = store.SavePreferences(next); err == nil || store.Preferences() != prefs {
		t.Fatal("external edit was overwritten", err)
	}
	after, err = os.ReadFile(store.path)
	if err != nil || !bytes.Equal(after, external) {
		t.Fatal("failed save changed external content", err)
	}
}
