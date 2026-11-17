package connection

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type highlightSource struct {
	io.Reader
	written    bytes.Buffer
	cols, rows int
	closed     bool
}

func (s *highlightSource) Write(p []byte) (int, error) { return s.written.Write(p) }
func (s *highlightSource) Close() error                { s.closed = true; return nil }
func (s *highlightSource) Resize(cols, rows int) error { s.cols, s.rows = cols, rows; return nil }

func addLevel(word string) string { return levelColor(word, false) + word + "\x1b[39m" }

func transformHighlight(input string, chunk int) (string, *HighlightStream) {
	h := &HighlightStream{}
	h.SetEnabled(true)
	for len(input) > 0 {
		n := min(chunk, len(input))
		h.transform([]byte(input[:n]))
		input = input[n:]
	}
	h.flush(true)
	return string(h.out), h
}

func TestHighlightLevelsAndArbitraryChunks(t *testing.T) {
	input := "中文 trace DEBUG Info warn WARNING error fatal PANIC errors _ERROR 3ERROR ERROR3 ERROR_ 中文ERROR中文\n"
	want := "中文 " + addLevel("trace") + " " + addLevel("DEBUG") + " " + addLevel("Info") + " " + addLevel("warn") + " " + addLevel("WARNING") + " " + addLevel("error") + " " + addLevel("fatal") + " " + addLevel("PANIC") + " errors _ERROR 3ERROR ERROR3 ERROR_ 中文" + addLevel("ERROR") + "中文\n"
	for n := 1; n <= len(input); n++ {
		got, _ := transformHighlight(input, n)
		if got != want {
			t.Fatalf("chunk %d: got %q, want %q", n, got, want)
		}
	}
}

func TestHighlightPreservesANSIAndControlStrings(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"\x1b[31mERROR\x1b[39m INFO\n", "\x1b[31mERROR\x1b[39m " + addLevel("INFO") + "\n"},
		{"\x1b[1;44mINFO\n", "\x1b[1;44m" + addLevel("INFO") + "\n"},
		{"\x1b[38;2;0;39;0mERROR\x1b[0m INFO\n", "\x1b[38;2;0;39;0mERROR\x1b[0m " + addLevel("INFO") + "\n"},
		{"\x1b[48;5;31mINFO\n", "\x1b[48;5;31m" + addLevel("INFO") + "\n"},
		{"\x1b[38:2::0:0:0mERROR\x1b[39m INFO\n", "\x1b[38:2::0:0:0mERROR\x1b[39m " + addLevel("INFO") + "\n"},
		{"\x1b]0;ERROR\x07INFO\n", "\x1b]0;ERROR\x07" + addLevel("INFO") + "\n"},
		{"\x1bPERROR\x1b\\INFO\n", "\x1bPERROR\x1b\\" + addLevel("INFO") + "\n"},
		{"\x1b[?1049hINFO\n\x1b[?1049lINFO\n", "\x1b[?1049hINFO\n\x1b[?1049l" + addLevel("INFO") + "\n"},
		{"\x1b[999zERROR\nINFO\n", "\x1b[999zERROR\n" + addLevel("INFO") + "\n"},
		{"ERROR \x1b[38;2;1", addLevel("ERROR") + " \x1b[38;2;1"},
	} {
		for n := 1; n <= len(tc.input); n++ {
			got, _ := transformHighlight(tc.input, n)
			if got != tc.want {
				t.Fatalf("input %q, chunk %d: got %q, want %q", tc.input, n, got, tc.want)
			}
		}
	}
}

func TestHighlightPrivateCSIAndRestartedEscapesPreserveForeground(t *testing.T) {
	for _, sequence := range []string{"\x1b[>4;0m", "\x1b[?0m", "\x1b[=0m", "\x1b[<0m", "\x1b[0 m", "\x1b[0$m"} {
		input := "\x1b[31m" + sequence + "ERROR\nINFO\n\x1b[39mINFO\n"
		want := "\x1b[31m" + sequence + "ERROR\nINFO\n\x1b[39m" + addLevel("INFO") + "\n"
		for chunk := 1; chunk <= len(input); chunk++ {
			got, _ := transformHighlight(input, chunk)
			if got != want {
				t.Fatalf("sequence %q chunk %d: %q", sequence, chunk, got)
			}
		}
	}
	for _, prefix := range []string{"\x1b\x1b", "\x1b\x1b\x1b", "\x1b[12\x1b", "\x1b(\x1b"} {
		input := prefix + "[31mERROR\nINFO\n\x1b[0;1mINFO\n"
		want := prefix + "[31mERROR\nINFO\n\x1b[0;1m" + addLevel("INFO") + "\n"
		for chunk := 1; chunk <= len(input); chunk++ {
			got, _ := transformHighlight(input, chunk)
			if got != want {
				t.Fatalf("prefix %q chunk %d: %q", prefix, chunk, got)
			}
		}
	}
	input := "\x1b[>4;0mERROR\nINFO\n"
	want := "\x1b[>4;0mERROR\n" + addLevel("INFO") + "\n"
	if got, _ := transformHighlight(input, 1); got != want {
		t.Fatalf("private CSI invented a foreground color: %q", got)
	}
}

func TestHighlightToggleTracksRemoteColorAndTheme(t *testing.T) {
	h := &HighlightStream{}
	h.transform([]byte("\x1b[31m"))
	h.SetEnabled(true)
	h.transform([]byte("ERROR\n\x1b[39mINFO\n"))
	h.SetEnabled(false)
	h.transform([]byte("ERROR\n"))
	h.SetEnabled(true)
	h.SetDark(true)
	h.transform([]byte("INFO\n"))
	want := "\x1b[31mERROR\n\x1b[39m" + addLevel("INFO") + "\nERROR\n" + levelColor("INFO", true) + "INFO\x1b[39m\n"
	if string(h.out) != want {
		t.Fatalf("got %q, want %q", h.out, want)
	}
}

func TestHighlightDisabledStreamsRemainIndependent(t *testing.T) {
	a, b := &HighlightStream{}, &HighlightStream{}
	a.SetEnabled(true)
	a.transform([]byte("INFO\n"))
	b.transform([]byte("INFO\n"))
	if string(a.out) != addLevel("INFO")+"\n" || string(b.out) != "INFO\n" {
		t.Fatal("stream enablement leaked between sessions")
	}
	b.transform([]byte("prompt$ "))
	if len(b.pending) != 0 || !strings.HasSuffix(string(b.out), "prompt$ ") {
		t.Fatal("disabled highlighting delayed prompt output")
	}
}

type errorHighlightReader struct {
	err  error
	sent bool
}

func (r *errorHighlightReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, r.err
	}
	r.sent = true
	return copy(p, "中文 ERROR 尾部"), r.err
}

func TestHighlightDrainsDataBeforeReadError(t *testing.T) {
	wantErr := errors.New("remote failed")
	for _, readErr := range []error{io.EOF, wantErr} {
		s := &highlightSource{Reader: &errorHighlightReader{err: readErr}}
		h := NewHighlightStream(s)
		h.SetEnabled(true)
		var got []byte
		for {
			p := make([]byte, 1)
			n, err := h.Read(p)
			got = append(got, p[:n]...)
			if err != nil {
				if !errors.Is(err, readErr) {
					t.Fatal(err)
				}
				break
			}
		}
		h.Close()
		if string(got) != "中文 "+addLevel("ERROR")+" 尾部" {
			t.Fatalf("got %q", got)
		}
	}
}

type pipeHighlightSource struct {
	*io.PipeReader
	w    *io.PipeWriter
	once sync.Once
}

func newPipeHighlightSource() *pipeHighlightSource {
	r, w := io.Pipe()
	return &pipeHighlightSource{PipeReader: r, w: w}
}
func (s *pipeHighlightSource) Write(p []byte) (int, error) { return len(p), nil }
func (s *pipeHighlightSource) Close() error {
	s.once.Do(func() { s.PipeReader.Close(); s.w.Close() })
	return nil
}

func TestHighlightFlushesPromptWhileUnderlyingReadBlocks(t *testing.T) {
	s := newPipeHighlightSource()
	h := NewHighlightStream(s)
	h.SetEnabled(true)
	defer h.Close()
	go s.w.Write([]byte("主机$ "))
	started := time.Now()
	p := make([]byte, 100)
	n, err := h.Read(p)
	elapsed := time.Since(started)
	if err != nil || string(p[:n]) != "主机$ " {
		t.Fatalf("%q %v", p[:n], err)
	}
	if elapsed < highlightDelay/2 || elapsed > 150*time.Millisecond {
		t.Fatalf("flush latency %s", elapsed)
	}
	t.Logf("prompt fragment flush: %s (20 ms target, test scheduling tolerance 150 ms)", elapsed)
}

func TestHighlightContinuousInputDoesNotResetDeadline(t *testing.T) {
	s := newPipeHighlightSource()
	h := NewHighlightStream(s)
	h.SetEnabled(true)
	defer h.Close()
	go func() {
		for i := 0; i < 50; i++ {
			if _, err := s.w.Write([]byte("x")); err != nil {
				return
			}
			time.Sleep(3 * time.Millisecond)
		}
	}()
	p := make([]byte, 100)
	started := time.Now()
	n, err := h.Read(p)
	if err != nil || n == 0 || n >= 50 || time.Since(started) > 120*time.Millisecond {
		t.Fatalf("n=%d err=%v elapsed=%s", n, err, time.Since(started))
	}
}

func TestHighlightCloseReleasesBlockedReadAndDelegates(t *testing.T) {
	s := newPipeHighlightSource()
	h := NewHighlightStream(s)
	finished := make(chan struct{})
	go func() { defer close(finished); h.Read(make([]byte, 8)) }()
	h.Close()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("Close left Read blocked")
	}
	plain := &highlightSource{Reader: strings.NewReader("")}
	h = NewHighlightStream(plain)
	h.Write([]byte("tail -f file\r"))
	h.Resize(80, 24)
	h.Close()
	if plain.written.String() != "tail -f file\r" || plain.cols != 80 || plain.rows != 24 || !plain.closed {
		t.Fatal("shell operations not delegated")
	}
}

func TestHighlightCloseDrainsQueuedAndInFlightRead(t *testing.T) {
	for range 100 {
		s := newPipeHighlightSource()
		h := NewHighlightStream(s)
		written := make(chan struct{})
		go func() { s.w.Write([]byte("first ")); s.w.Write([]byte("尾部\n")); close(written) }()
		select {
		case <-written:
		case <-time.After(time.Second):
			t.Fatal("reader did not consume both bounded chunks")
		}
		if err := h.Close(); err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(h)
		if err != nil || string(got) != "first 尾部\n" {
			t.Fatalf("Close lost completed reads: %q %v", got, err)
		}
		select {
		case <-h.received:
		default:
			t.Fatal("Close leaked receiving goroutine")
		}
	}
	delegate := &highlightSource{Reader: strings.NewReader("")}
	h := NewHighlightStream(delegate)
	if n, err := h.Write([]byte("\x03tail 输入")); n != len("\x03tail 输入") || err != nil {
		t.Fatal(n, err)
	}
	if err := h.Resize(90, 30); err != nil {
		t.Fatal(err)
	}
	h.Close()
	if delegate.written.String() != "\x03tail 输入" || delegate.cols != 90 || delegate.rows != 30 || !delegate.closed {
		t.Fatal("wrapper changed shell delegation")
	}
}

func TestHighlightFallbackBoundsAndPreservesBytes(t *testing.T) {
	for _, input := range []string{
		strings.Repeat("中文 x ", 100000) + "ERROR\n",
		"\x1b]" + strings.Repeat("ERROR", 100000),
		"\x1b[" + strings.Repeat("1;", 100000),
	} {
		got, h := transformHighlight(input, 47)
		if got != input {
			t.Fatal("fallback changed text")
		}
		if !h.Degraded() || len(h.control) > highlightControlLimit+1 || len(h.pending) > highlightChunk {
			t.Fatal("fallback was not bounded")
		}
	}
}

func TestHighlightTimeoutDoesNotMistakeSplitWord(t *testing.T) {
	h := &HighlightStream{}
	h.SetEnabled(true)
	h.transform([]byte("ERROR"))
	h.flush(false)
	h.transform([]byte("s ERROR\n"))
	if string(h.out) != "ERRORs "+addLevel("ERROR")+"\n" {
		t.Fatalf("got %q", h.out)
	}
}
