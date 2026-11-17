package connection

import (
	"strings"
	"testing"
)

func TestHighlightRulePrioritySnapshotAndBounds(t *testing.T) {
	red, blue := [3]uint8{255, 0, 0}, [3]uint8{0, 0, 255}
	input := []HighlightRule{{Pattern: "ERROR", Light: red, Dark: blue, WholeWord: true}, {Pattern: ".+", Light: blue, Dark: red}}
	rules, err := CompileHighlightRules(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0].Pattern = ".+"
	got, ok := rules.append(nil, []byte("a ERROR b"), false, true, false)
	want := "\x1b[38;2;0;0;255ma \x1b[39m\x1b[38;2;255;0;0mERROR\x1b[39m\x1b[38;2;0;0;255m b\x1b[39m"
	if !ok || string(got) != want {
		t.Fatalf("priority or immutable input: %q", got)
	}
	for _, pattern := range []string{"a", "(?:)"} {
		rules, err = CompileHighlightRules([]HighlightRule{{Pattern: pattern, Light: red}})
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Repeat("a ", 2000)
		got, ok = rules.append(nil, []byte(text), false, true, false)
		if ok || string(got) != text {
			t.Fatal("match budget did not preserve plain text")
		}
	}
	if _, err = CompileHighlightRules([]HighlightRule{{Pattern: "["}}); err == nil {
		t.Fatal("invalid pattern accepted")
	}
}

func TestHighlightCancelledControlKeepsRemoteForeground(t *testing.T) {
	for _, input := range []string{
		"\x1b[12\x1b[31mERROR\nINFO\n\x1b[39mINFO\n",
		"\x1b[31m\x1b[12\x18ERROR\nINFO\n\x1b[39mINFO\n",
		"\x1b[31m\x1b]title\x1aERROR\nINFO\n\x1b[39mINFO\n",
	} {
		want := strings.TrimSuffix(input, "INFO\n") + addLevel("INFO") + "\n"
		for n := 1; n <= len(input); n++ {
			got, _ := transformHighlight(input, n)
			if got != want {
				t.Fatalf("chunk %d: %q", n, got)
			}
		}
	}
}
