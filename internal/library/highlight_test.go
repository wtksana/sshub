package library

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func highlightPreferences() Preferences {
	p := DefaultPreferences()
	config := p.EffectiveLogHighlight()
	p.LogHighlight = &config
	return p
}

func TestHighlightPreferencesOwnSnapshotsAndMigration(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	if got := s.Preferences(); got.LogHighlight != nil || !reflect.DeepEqual(got.EffectiveLogHighlight(), DefaultLogHighlightPreferences()) {
		t.Fatal("missing legacy preset defaults")
	}
	p := highlightPreferences()
	for i := range p.LogHighlight.Builtins {
		p.LogHighlight.Builtins[i].Enabled = false
	}
	p.LogHighlight.Builtins[0].LightColor = "#123456"
	p.LogHighlight.Custom = []CustomHighlightRule{{Kind: "keyword", Pattern: "性能", Enabled: true, Color: "#123456"}, {Kind: "regex", Pattern: `"id":[0-9]+`, CaseSensitive: true, Color: "#ABCDEF"}}
	want := p.Clone()
	if err := s.SavePreferences(p); err != nil {
		t.Fatal(err)
	}
	p.LogHighlight.Builtins[0].Enabled = true
	p.LogHighlight.Custom[0].Pattern = "changed"
	copy := s.Preferences()
	copy.LogHighlight.Builtins[0].LightColor = "#654321"
	copy.LogHighlight.Custom[0].Color = "#654321"
	if !reflect.DeepEqual(s.Preferences(), want) {
		t.Fatal("caller can mutate stored presets")
	}
	s.Close()
	s = openTestStore(t, dir)
	if !reflect.DeepEqual(s.Preferences(), want) {
		t.Fatal("all-disabled presets not restored")
	}
	data, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	m, err := DecodeMigration(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	dst := openTestStore(t, t.TempDir())
	plan, err := dst.PreviewImport(*m)
	if err != nil {
		t.Fatal(err)
	}
	m.Preferences.LogHighlight.Builtins[0].LightColor = "#000000"
	plan.Preferences.LogHighlight.Builtins[0].Enabled = true
	plan.Preferences.LogHighlight.Custom[0], plan.Preferences.LogHighlight.Custom[1] = plan.Preferences.LogHighlight.Custom[1], plan.Preferences.LogHighlight.Custom[0]
	plan.Preferences.LogHighlight.Builtins[0], plan.Preferences.LogHighlight.Builtins[1] = plan.Preferences.LogHighlight.Builtins[1], plan.Preferences.LogHighlight.Builtins[0]
	if _, err = dst.CommitImport(plan, nil, true); err != nil {
		t.Fatal(err)
	}
	plan.Preferences.LogHighlight.Builtins[0].LightColor = "#000000"
	if !reflect.DeepEqual(dst.Preferences(), want) {
		t.Fatal("input or public preview mutated canonical import")
	}
}

func TestInvalidHighlightPreferencesNeverApply(t *testing.T) {
	s := openTestStore(t, t.TempDir())
	want := highlightPreferences()
	want.LogHighlight.Custom = []CustomHighlightRule{{Kind: "keyword", Pattern: "old", Enabled: true, Color: "#123456"}}
	if err := s.SavePreferences(want); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*LogHighlightPreferences){
		func(p *LogHighlightPreferences) { p.Builtins = nil },
		func(p *LogHighlightPreferences) { p.Builtins = append(p.Builtins, p.Builtins[0]) },
		func(p *LogHighlightPreferences) { p.Builtins[0].ID = "json" },
		func(p *LogHighlightPreferences) { p.Builtins[0].ID = p.Builtins[1].ID },
		func(p *LogHighlightPreferences) { p.Builtins[0].LightColor = "red" },
		func(p *LogHighlightPreferences) { p.Builtins[0].LightColor = "#12345678" },
		func(p *LogHighlightPreferences) { p.Builtins[0].DarkColor = "#12345G" },
		func(p *LogHighlightPreferences) { p.Builtins[0].DarkColor = "#12+345" },
		func(p *LogHighlightPreferences) {
			p.Custom = []CustomHighlightRule{{Kind: "json", Pattern: "x", Color: "#123456"}}
		},
		func(p *LogHighlightPreferences) {
			p.Custom = []CustomHighlightRule{{Kind: "keyword", Pattern: "", Color: "#123456"}}
		},
		func(p *LogHighlightPreferences) {
			p.Custom = []CustomHighlightRule{{Kind: "regex", Pattern: "[", Color: "#123456"}}
		},
		func(p *LogHighlightPreferences) {
			p.Custom = []CustomHighlightRule{{Kind: "regex", Pattern: `(a)\1`, Color: "#123456"}}
		},
		func(p *LogHighlightPreferences) {
			p.Custom = []CustomHighlightRule{{Kind: "regex", Pattern: `(?=x)`, Color: "#123456"}}
		},
		func(p *LogHighlightPreferences) {
			p.Custom = []CustomHighlightRule{{Kind: "regex", Pattern: `(?:a?){1000}b`, Color: "#123456"}}
		},
		func(p *LogHighlightPreferences) {
			p.Custom = []CustomHighlightRule{{Kind: "keyword", Pattern: "x", Color: "red"}}
		},
		func(p *LogHighlightPreferences) {
			for range 33 {
				p.Custom = append(p.Custom, CustomHighlightRule{Kind: "keyword", Pattern: "x", Color: "#123456"})
			}
		},
		func(p *LogHighlightPreferences) {
			for range 8 {
				p.Custom = append(p.Custom, CustomHighlightRule{Kind: "regex", Pattern: `(?:a|aa){30}b`, Color: "#123456"})
			}
		},
	} {
		bad := want.Clone()
		change(bad.LogHighlight)
		if s.SavePreferences(bad) == nil {
			t.Fatal("invalid preset accepted", bad.LogHighlight)
		}
		data, err := json.Marshal(Migration{Format: "sshub", Version: 1, Profiles: []PortableProfile{}, Preferences: &bad})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeMigration(bytes.NewReader(data)); err == nil {
			t.Fatal("invalid imported preset accepted")
		}
		after, _ := os.ReadFile(s.path)
		if !bytes.Equal(before, after) || !reflect.DeepEqual(s.Preferences(), want) {
			t.Fatal("invalid preset partially applied")
		}
	}
	if err = os.WriteFile(s.path, append(before, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	next := want.Clone()
	next.LogHighlight.Builtins[0].Enabled = false
	next.LogHighlight.Custom[0].Pattern = "new"
	if s.SavePreferences(next) == nil || !reflect.DeepEqual(s.Preferences(), want) {
		t.Fatal("failed write changed active presets")
	}
}
