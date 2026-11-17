package desktop

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"sshub/internal/connection"
	"sshub/internal/library"
)

type settingsHighlightSource struct {
	io.Reader
	chunk int
}

func (s settingsHighlightSource) Write(p []byte) (int, error) { return len(p), nil }
func (s settingsHighlightSource) Close() error                { return nil }
func (s settingsHighlightSource) Read(p []byte) (int, error) {
	return s.Reader.Read(p[:min(s.chunk, len(p))])
}

func renderSettingsHighlight(t *testing.T, prefs library.Preferences, input string, chunk int, dark bool) string {
	t.Helper()
	h := connection.NewHighlightStream(settingsHighlightSource{strings.NewReader(input), chunk})
	defer h.Close()
	h.SetEnabled(true)
	h.SetDark(dark)
	h.SetRules(compileHighlightPreferences(prefs))
	got, err := io.ReadAll(h)
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

func TestBuiltinHighlightDatesAndWordBoundaries(t *testing.T) {
	p := library.DefaultPreferences()
	for _, text := range []string{"2026-11-17", "2026/11/17", "12:34:56", "12:34:56.123", "2026-11-17 12:34:56,123", "2026-11-17T12:34:56Z", "2026/11/17T12:34:56.123456789+08:00", "2026-11-17T12:34:56-0800"} {
		for _, dark := range []bool{false, true} {
			color := "9;105;218"
			if dark {
				color = "88;166;255"
			}
			input := "中文[" + text + "]\n"
			want := "中文[\x1b[38;2;" + color + "m" + text + "\x1b[39m]\n"
			for chunk := 1; chunk <= len(input); chunk++ {
				if got := renderSettingsHighlight(t, p, input, chunk, dark); got != want {
					t.Fatalf("date %s chunk %d: %q", text, chunk, got)
				}
			}
		}
	}
	for _, input := range []string{"x2026-11-17y", "_12:34:56_", "112:34:567", "errors _ERROR ERROR3", `{"message":"not a date"}`} {
		if got := renderSettingsHighlight(t, p, input+"\n", 1, false); got != input+"\n" {
			t.Fatalf("unexpected boundary match %q", got)
		}
	}
	input := "\x1b[31m2026-11-17 ERROR\n\x1b[39m INFO\n"
	want := "\x1b[31m2026-11-17 ERROR\n\x1b[39m \x1b[38;2;26;127;55mINFO\x1b[39m\n"
	if got := renderSettingsHighlight(t, p, input, 1, false); got != want {
		t.Fatalf("remote foreground lost: %q", got)
	}
}

func TestBuiltinHighlightDraftCancelAndValidationDoNotApply(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	a.beginSettings()
	view := ui.NewTester(a.settingsView, 900, 700)
	a.settings.preferences.LogHighlight.Builtins[0].LightColor = "invalid"
	view.Frame()
	a.saveSettings(a.settings)
	if a.settings.err == "" || a.settings.busy || a.preferences.LogHighlight != nil {
		t.Fatal("invalid color draft applied or started a save")
	}
	a.settings.preferences.LogHighlight.Builtins[0].LightColor = "#123456"
	a.settings.preferences.LogHighlight.Builtins[0].Enabled = false
	view.Frame()
	if err := view.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if a.settings != nil || a.preferences.LogHighlight != nil || a.store.Preferences().LogHighlight != nil {
		t.Fatal("draft leaked into active preferences")
	}
	a.beginSettings()
	a.settings.preferences.LogHighlight.Builtins[0].LightColor = "#invalid"
	a.saveSettings(a.settings)
	if a.settings.err == "" || a.settings.busy || a.preferences.LogHighlight != nil {
		t.Fatal("invalid color applied")
	}
}

func TestBuiltinHighlightPublishOwnsSnapshotAndPreservesSwitch(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	input := "INFO 2026-11-17\n"
	for _, enabled := range []bool{true, false, true} {
		h := connection.NewHighlightStream(settingsHighlightSource{strings.NewReader(input), 1})
		h.SetEnabled(enabled)
		a.tabs = append(a.tabs, &sessionTab{key: len(a.tabs) + 1, highlight: h})
	}
	p := a.preferences.Clone()
	config := p.EffectiveLogHighlight()
	p.LogHighlight = &config
	for i := range config.Builtins {
		if config.Builtins[i].ID == "info" {
			config.Builtins[i].LightColor = "#010203"
		}
		if config.Builtins[i].ID == "date" {
			config.Builtins[i].Enabled = false
		}
	}
	a.applyPreferences(p, false)
	want := p.Clone()
	p.LogHighlight.Builtins[0].LightColor = "#000000"
	if !reflect.DeepEqual(a.preferences, want) {
		t.Fatal("apply retains caller-owned configuration")
	}
	for i, tab := range a.tabs {
		output, err := io.ReadAll(tab.highlight)
		if err != nil {
			t.Fatal(err)
		}
		expected := input
		if i != 1 {
			expected = "\x1b[38;2;1;2;3mINFO\x1b[39m 2026-11-17\n"
		}
		if string(output) != expected || tab.highlight.Enabled() != (i != 1) {
			t.Fatal(fmt.Sprintf("tab %d changed switch or rules: %q", i, output))
		}
	}
}
