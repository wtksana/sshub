package connection

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func customRules(t testing.TB, patterns ...string) *HighlightRules {
	t.Helper()
	var input []HighlightRule
	for i, p := range patterns {
		input = append(input, HighlightRule{Pattern: p, Light: [3]uint8{uint8(i + 1), 2, 3}})
	}
	r, err := CompileHighlightRules(input)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

var insertedForeground = regexp.MustCompile(`\x1b\[(?:38;2;[0-9]+;[0-9]+;[0-9]+|39)m`)

type customChunkReader struct {
	io.Reader
	chunk int
}

func (r customChunkReader) Read(p []byte) (int, error) {
	return r.Reader.Read(p[:min(len(p), r.chunk)])
}

func TestCustomPartialOverlapAndZeroWidth(t *testing.T) {
	r := customRules(t, "bc", "abcd", "^")
	got, ok := r.append(nil, []byte("abcd"), false, true, false)
	want := "\x1b[38;2;2;2;3ma\x1b[39m\x1b[38;2;1;2;3mbc\x1b[39m\x1b[38;2;2;2;3md\x1b[39m"
	if !ok || string(got) != want {
		t.Fatalf("partial priority %q", got)
	}
	r = customRules(t, "^", "$")
	got, ok = r.append(nil, []byte("中文"), false, true, false)
	if !ok || string(got) != "中文" {
		t.Fatalf("zero-width inserted color %q", got)
	}
}

func TestCustomUTF8EveryFlushBoundaryPreservesEncoding(t *testing.T) {
	input := []byte("中文🙂 INFO {\"id\":123}\n")
	r := customRules(t, ".")
	for split := 1; split < len(input); split++ {
		h := &HighlightStream{budget: newHighlightBudget()}
		h.SetEnabled(true)
		h.SetRules(r)
		h.transform(input[:split])
		h.flush(false)
		h.transform(input[split:])
		h.flush(true)
		plain := insertedForeground.ReplaceAll(h.out, nil)
		if !bytes.Equal(plain, input) || !utf8.Valid(h.out) {
			t.Fatalf("split %d corrupt: %q", split, h.out)
		}
	}
}

func TestCustomCycleBudgetsAcrossShortLines(t *testing.T) {
	for _, pattern := range []string{"a", "^", `(?:a|aa){30}b`} {
		r := customRules(t, pattern)
		input := strings.Repeat("a\n", highlightChunk/2)
		if pattern == `(?:a|aa){30}b` {
			r = customRules(t, pattern, pattern, pattern, pattern, pattern)
			input = strings.Repeat("aaaaaaaa\n", highlightChunk/9)
		}
		h := &HighlightStream{budget: newHighlightBudget()}
		h.SetEnabled(true)
		h.SetRules(r)
		h.transform([]byte(input))
		h.flush(true)
		if !h.Degraded() || h.budget.extra < 0 || h.budget.work < 0 || len(h.out)-len(input) > 48<<10 || string(insertedForeground.ReplaceAll(h.out, nil)) != input {
			t.Fatalf("cycle not bounded %q: budget=%+v out=%d", pattern, h.budget, len(h.out))
		}
	}
	r := customRules(t, `(?:a|aa){30}b`)
	text := []byte(strings.Repeat("a", highlightChunk))
	got, ok := r.append(nil, text, false, true, false)
	if ok || !bytes.Equal(got, text) {
		t.Fatal("fragment work budget not enforced")
	}
}

func TestCustomStreamReadPreservesEveryChunk(t *testing.T) {
	input := "\x1b[1;44m中文INFO\x1b[31mINFO\x1b[39m {\"id\":12}\n"
	for chunk := 1; chunk < len(input); chunk++ {
		s := &highlightSource{Reader: customChunkReader{strings.NewReader(input), chunk}}
		h := NewHighlightStream(s)
		h.SetEnabled(true)
		h.SetRules(customRules(t, "INFO", `"id":[0-9]+`))
		got, err := io.ReadAll(h)
		h.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "\x1b[31mINFO\x1b[39m") || !strings.Contains(string(got), "\x1b[1;44m中文\x1b[38;2;1;2;3mINFO\x1b[39m") {
			t.Fatalf("chunk %d lost original style: %q", chunk, got)
		}
	}
}

func TestHighlightRestoredForegroundPersistsAcrossLines(t *testing.T) {
	for _, pair := range [][2]string{{"\x1b7", "\x1b8"}, {"\x1b[s", "\x1b[u"}} {
		input := "\x1b[31m" + pair[0] + "\x1b[39m" + pair[1] + "ERROR\nINFO\n\x1b[39mINFO\n"
		want := strings.TrimSuffix(input, "INFO\n") + addLevel("INFO") + "\n"
		for n := 1; n < len(input); n++ {
			got, _ := transformHighlight(input, n)
			if got != want {
				t.Fatalf("restored foreground chunk %d: %q", n, got)
			}
		}
	}
	input := "\x1b[31m\x1b[?1049h\x1b[39mINFO\n\x1b[?1049lERROR\nINFO\n\x1b[39mINFO\n"
	want := strings.TrimSuffix(input, "INFO\n") + addLevel("INFO") + "\n"
	for n := 1; n < len(input); n++ {
		got, _ := transformHighlight(input, n)
		if got != want {
			t.Fatalf("alternate restore chunk %d: %q", n, got)
		}
	}
}
