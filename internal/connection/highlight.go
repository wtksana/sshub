package connection

import (
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	highlightDelay        = 20 * time.Millisecond
	highlightChunk        = 16 << 10
	highlightLineLimit    = 64 << 10
	highlightControlLimit = 256
)

// HighlightStream adds foreground escapes to new output, never to user input.
// Read has the same single-reader contract as an SSH shell.
type HighlightStream struct {
	source                             io.ReadWriteCloser
	enabled, dark                      atomic.Bool
	degraded                           atomic.Bool
	rules                              atomic.Pointer[HighlightRules]
	reads                              chan highlightRead
	slots                              chan struct{}
	received                           chan struct{}
	done                               chan struct{}
	once                               sync.Once
	closeErr                           error
	out, pending, control              []byte
	err                                error
	deadline                           time.Time
	chunkAt                            time.Time
	state                              byte
	foreground, skipLine, previousWord bool
	savedForeground                    bool
	savedForegroundKnown               bool
	alt                                [3]bool
	altForeground                      [3]bool
	lineBytes                          int
	budget                             highlightBudget
}

type highlightRead struct {
	data []byte
	err  error
	at   time.Time
}

func NewHighlightStream(source io.ReadWriteCloser) *HighlightStream {
	h := &HighlightStream{source: source, reads: make(chan highlightRead, 2), slots: make(chan struct{}, 2), received: make(chan struct{}), done: make(chan struct{})}
	h.slots <- struct{}{}
	h.slots <- struct{}{}
	go h.receive()
	return h
}

func (h *HighlightStream) Enabled() bool           { return h.enabled.Load() }
func (h *HighlightStream) SetEnabled(enabled bool) { h.enabled.Store(enabled) }
func (h *HighlightStream) SetDark(dark bool)       { h.dark.Store(dark) }

// Degraded reports whether a bounded fallback has occurred in this session.
func (h *HighlightStream) Degraded() bool              { return h.degraded.Load() }
func (h *HighlightStream) Write(p []byte) (int, error) { return h.source.Write(p) }
func (h *HighlightStream) Resize(cols, rows int) error {
	if r, ok := h.source.(interface{ Resize(int, int) error }); ok {
		return r.Resize(cols, rows)
	}
	return nil
}
func (h *HighlightStream) Close() error {
	h.once.Do(func() { close(h.done); h.closeErr = h.source.Close() })
	return h.closeErr
}

func (h *HighlightStream) receive() {
	defer close(h.received)
	for {
		select {
		case <-h.slots:
		case <-h.done:
			return
		}
		select {
		case <-h.done:
			return
		default:
		}
		p := make([]byte, highlightChunk)
		n, err := h.source.Read(p)
		// Reserve a queue slot before Read so Close can retain every completed
		// read, including the one unblocked by closing the underlying shell.
		h.reads <- highlightRead{p[:n], err, time.Now()}
		if err != nil {
			return
		}
	}
}

func (h *HighlightStream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(h.out) == 0 {
		h.budget = newHighlightBudget()
		if h.err != nil {
			return 0, h.err
		}
		var timeout <-chan time.Time
		var timer *time.Timer
		if !h.deadline.IsZero() {
			if time.Until(h.deadline) <= 0 {
				h.flush(false)
				continue
			}
			timer = time.NewTimer(time.Until(h.deadline))
			timeout = timer.C
		}
		select {
		case r := <-h.reads:
			h.slots <- struct{}{}
			h.chunkAt = r.at
			h.transform(r.data)
			if r.err != nil {
				h.flush(true)
				h.err = r.err
			}
		case <-timeout:
			h.flush(false)
		case <-h.done:
			<-h.received
			for len(h.reads) > 0 {
				r := <-h.reads
				h.chunkAt = r.at
				h.transform(r.data)
			}
			h.flush(false)
			h.err = io.EOF
		}
		if timer != nil {
			timer.Stop()
		}
	}
	n := copy(p, h.out)
	h.out = h.out[n:]
	return n, nil
}

func asciiWord(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
}

func (h *HighlightStream) flush(boundary bool) {
	p := h.pending
	h.pending = nil
	h.deadline = time.Time{}
	if len(p) == 0 {
		return
	}
	if !h.Enabled() || h.foreground || h.skipLine || h.alt[0] || h.alt[1] || h.alt[2] {
		h.out = append(h.out, p...)
	} else if rules := h.rules.Load(); rules != nil {
		var ok bool
		h.out, ok = rules.appendBudget(h.out, p, h.previousWord, boundary, h.dark.Load(), &h.budget)
		if !ok {
			h.degraded.Store(true)
		}
	} else {
		// ASCII matches never split a UTF-8 encoding, including truncated input.
		for i := 0; i < len(p); {
			j := i
			if asciiWord(p[i]) {
				for j < len(p) && asciiWord(p[j]) {
					j++
				}
				color := levelColor(string(p[i:j]), h.dark.Load())
				if color != "" && (i > 0 || !h.previousWord) && (j < len(p) || boundary) {
					h.out = append(h.out, color...)
					h.out = append(h.out, p[i:j]...)
					h.out = append(h.out, "\x1b[39m"...)
				} else {
					h.out = append(h.out, p[i:j]...)
				}
			} else {
				j++
				h.out = append(h.out, p[i])
			}
			i = j
		}
	}
	h.previousWord = asciiWord(p[len(p)-1])
}

func levelColor(word string, dark bool) string {
	light, night := "", ""
	switch strings.ToUpper(word) {
	case "TRACE":
		light, night = "87;96;106", "139;148;158"
	case "DEBUG":
		light, night = "130;80;223", "188;140;255"
	case "INFO":
		light, night = "26;127;55", "63;185;80"
	case "WARN", "WARNING":
		light, night = "154;103;0", "210;153;34"
	case "ERROR", "FATAL", "PANIC":
		light, night = "207;34;46", "248;81;73"
	default:
		return ""
	}
	if dark {
		light = night
	}
	return "\x1b[38;2;" + light + "m"
}

func (h *HighlightStream) transform(p []byte) {
	for _, b := range p {
		if h.state != 0 {
			h.out = append(h.out, b)
			h.controlByte(b)
			continue
		}
		if b == 0x1b {
			h.flush(false)
			h.out = append(h.out, b)
			h.state, h.control = 'e', nil
			continue
		}
		if b < 0x20 || b == 0x7f {
			h.flush(true)
			h.out = append(h.out, b)
			if b == '\n' {
				h.lineBytes, h.skipLine, h.previousWord = 0, false, false
			}
			continue
		}
		if len(h.pending) == 0 {
			h.deadline = h.chunkAt.Add(highlightDelay)
			if h.chunkAt.IsZero() {
				h.deadline = time.Now().Add(highlightDelay)
			}
		}
		h.pending = append(h.pending, b)
		h.lineBytes++
		if h.lineBytes > highlightLineLimit {
			h.skipLine = true
			h.degraded.Store(true)
		}
		if len(h.pending) >= highlightChunk {
			// A forced flush may miss a word crossing this boundary.
			h.degraded.Store(true)
			h.flush(false)
		}
	}
	if !h.Enabled() || h.foreground || h.skipLine || h.alt[0] || h.alt[1] || h.alt[2] {
		h.flush(false)
	}
	// Newlines and controls already flush. Short prompt fragments wait for
	// the original deadline even while the next underlying Read blocks.
}

func (h *HighlightStream) controlByte(b byte) {
	if b == 0x18 || b == 0x1a {
		h.state, h.control, h.skipLine = 0, nil, true
		return
	}
	if b == 0x1b && (h.state == 'e' || h.state == 'c' || h.state == 'i') {
		h.state, h.control, h.skipLine = 'e', nil, true
		return
	}
	switch h.state {
	case 'e':
		switch b {
		case '[':
			h.state = 'c'
		case ']':
			h.state = 'o'
		case 'P', 'X', '^', '_':
			h.state = 's'
		case 'c':
			h.state = 0
			h.foreground = false
			h.savedForegroundKnown = false
			h.alt = [3]bool{}
		case '7':
			h.savedForeground, h.savedForegroundKnown, h.state = h.foreground, true, 0
		case '8':
			h.restoreForeground()
			h.state, h.skipLine = 0, true
		default:
			if b >= 0x20 && b <= 0x2f {
				h.state = 'i'
			} else {
				h.state = 0
				h.skipLine = true
			}
		}
	case 'i':
		if b >= 0x30 && b <= 0x7e {
			h.state = 0
			h.skipLine = true
		}
	case 'c':
		if b >= 0x40 && b <= 0x7e {
			if len(h.control) <= highlightControlLimit {
				h.csi(b, string(h.control))
			} else {
				h.foreground = true
				h.skipLine = true
			}
			h.control, h.state = nil, 0
		} else if len(h.control) <= highlightControlLimit {
			h.control = append(h.control, b)
			if len(h.control) > highlightControlLimit {
				h.degraded.Store(true)
			}
		}
	case 'o', 's':
		if b == 0x1b {
			h.state += 1
		} else if b == 7 && h.state == 'o' {
			h.state = 0
		}
		if len(h.control) <= highlightControlLimit {
			h.control = append(h.control, b)
		} else {
			h.degraded.Store(true)
		}
	case 'p', 't':
		if b == '\\' {
			h.state, h.control = 0, nil
		} else if b != 0x1b {
			h.state -= 1
		}
	}
}

func (h *HighlightStream) csi(final byte, params string) {
	if final == 'm' {
		// Private prefixes and intermediates identify other CSI commands, not SGR.
		for _, b := range []byte(params) {
			if (b < '0' || b > '9') && b != ';' && b != ':' {
				h.skipLine = true
				return
			}
		}
		h.sgr(params)
		return
	}
	if params == "" && final == 's' {
		h.savedForeground, h.savedForegroundKnown = h.foreground, true
		return
	}
	if params == "" && final == 'u' {
		h.restoreForeground()
		h.skipLine = true
		return
	}
	if (final == 'h' || final == 'l') && strings.HasPrefix(params, "?") {
		for _, mode := range strings.Split(params[1:], ";") {
			i := -1
			switch mode {
			case "47":
				i = 0
			case "1047":
				i = 1
			case "1049":
				i = 2
			}
			if i >= 0 {
				if final == 'h' && !h.alt[i] {
					h.altForeground[i] = h.foreground
				}
				if final == 'l' {
					h.foreground = h.foreground || !h.alt[i] || h.altForeground[i]
				}
				h.alt[i] = final == 'h'
			}
		}
		return
	}
	// Cursor movement, erase and reports are kept intact; color the next
	// logical line only, rather than guessing what was overwritten.
	h.skipLine = true
}

func (h *HighlightStream) restoreForeground() {
	// An unmatched restore can recover an explicit server color we never saw.
	// shortcut: ambiguous restores skip coloring until explicit reset; revisit
	// only if the terminal exposes its effective foreground state.
	h.foreground = h.foreground || !h.savedForegroundKnown || h.savedForeground
}

func (h *HighlightStream) sgr(params string) {
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if strings.Contains(part, ":") {
			n := strings.Split(part, ":")[0]
			if n == "38" {
				h.foreground = true
			} else if n != "48" && n != "58" {
				h.foreground = true
			}
			continue
		}
		n := 0
		var err error
		if part != "" {
			n, err = strconv.Atoi(part)
		}
		if err != nil {
			h.foreground = true
			continue
		}
		switch {
		case n == 0 || n == 39:
			h.foreground = false
		case n >= 30 && n <= 37 || n >= 90 && n <= 97:
			h.foreground = true
		case n == 38 || n == 48 || n == 58:
			if n == 38 {
				h.foreground = true
			}
			if i+1 < len(parts) && parts[i+1] == "5" {
				i += 2
			} else if i+1 < len(parts) && parts[i+1] == "2" {
				i += 4
			} else {
				h.foreground = true
				return
			}
		}
	}
}
