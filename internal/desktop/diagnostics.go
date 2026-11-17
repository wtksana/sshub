package desktop

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"sshub/internal/connection"
)

// Diagnostics are explicitly enabled by a developer-supplied output path.
// Records contain timings, counters and runtime totals, never targets or text.
type diagnostics struct {
	queue chan diagnosticSample
	done  chan struct{}
	last  time.Time
	ready time.Time
}

type diagnosticSample struct {
	At            time.Time `json:"at"`
	ReadyAt       time.Time `json:"ready_at"`
	Connected     int       `json:"connected"`
	Highlighted   int       `json:"highlighted"`
	ReceivedBytes []uint64  `json:"received_bytes"`
	HeapBytes     uint64    `json:"heap_bytes"`
	Goroutines    int       `json:"goroutines"`
}

func openDiagnostics() *diagnostics {
	path := os.Getenv("SSHUB_DIAGNOSTICS")
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		log.Print("无法创建诊断输出：", err)
		return nil
	}
	d := &diagnostics{queue: make(chan diagnosticSample, 1), done: make(chan struct{})}
	go func() {
		defer close(d.done)
		defer f.Close()
		encoder := json.NewEncoder(f)
		for sample := range d.queue {
			if err := encoder.Encode(sample); err != nil {
				log.Print("诊断写入失败：", err)
				return
			}
		}
	}()
	return d
}

func (a *app) sampleDiagnostics() {
	d := a.diagnostics
	if d == nil || time.Since(d.last) < time.Second {
		return
	}
	d.last = time.Now()
	s := diagnosticSample{At: d.last, ReadyAt: d.ready, ReceivedBytes: []uint64{}, Goroutines: runtime.NumGoroutine()}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	s.HeapBytes = mem.HeapAlloc
	for _, tab := range a.tabs {
		if tab.status == "已连接" {
			s.Connected++
			if tab.highlight != nil && tab.highlight.Enabled() {
				s.Highlighted++
			}
		}
		if tab.observed != nil {
			s.ReceivedBytes = append(s.ReceivedBytes, tab.observed.received.Load())
		}
	}
	select {
	case d.queue <- s:
	default:
	}
}

type observedShell struct {
	*connection.Shell
	received atomic.Uint64
}

func (s *observedShell) Read(p []byte) (int, error) {
	n, err := s.Shell.Read(p)
	s.received.Add(uint64(n))
	return n, err
}

func (a *app) observeShell(shell *connection.Shell) io.ReadWriteCloser {
	if a.diagnostics == nil {
		return shell
	}
	observed := &observedShell{Shell: shell}
	// The caller attaches the wrapper only when the attempt reaches the UI.
	return observed
}
