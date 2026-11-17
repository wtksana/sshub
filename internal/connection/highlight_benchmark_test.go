package connection

import (
	"io"
	"sort"
	"strings"
	"testing"
	"time"
)

func benchmarkHighlightRules(b *testing.B) *HighlightRules {
	patterns := []string{`(?i)性能`, `(?i)message`, `"id":[0-9]+`, `(?i)trace`, `(?i)debug`, `(?i)info`, `(?i)warn(?:ing)?`, `(?i)error`, `(?i)(?:fatal|panic)`, `(?:[0-9]{4}[-/][0-9]{2}[-/][0-9]{2}(?:[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(?:[.,][0-9]{1,9})?(?:Z|[+-][0-9]{2}:?[0-9]{2})?)?|[0-9]{2}:[0-9]{2}:[0-9]{2}(?:[.,][0-9]{1,9})?(?:Z|[+-][0-9]{2}:?[0-9]{2})?)`}
	var input []HighlightRule
	for i, p := range patterns {
		input = append(input, HighlightRule{Pattern: p, Light: [3]uint8{100, 120, 140}, Dark: [3]uint8{140, 160, 180}, WholeWord: i >= 3})
	}
	rules, err := CompileHighlightRules(input)
	if err != nil {
		b.Fatal(err)
	}
	return rules
}

func BenchmarkHighlightMatcher(b *testing.B) {
	rules := benchmarkHighlightRules(b)
	for _, sample := range []struct{ name, text string }{{"representative", `2026-11-17T22:00:00.123+08:00 INFO 性能 message {"id":123,"message":"error","enabled":true}`}, {"no-match", strings.Repeat("unmatched text ", 6)}} {
		b.Run(sample.name, func(b *testing.B) {
			input := []byte(strings.Repeat(sample.text+"\n", highlightChunk/(len(sample.text)+1)))
			h := &HighlightStream{}
			h.SetEnabled(true)
			h.SetRules(rules)
			b.SetBytes(int64(len(input)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				h.budget = newHighlightBudget()
				h.transform(input)
				if h.Degraded() {
					b.Fatal("representative degraded")
				}
				h.out = h.out[:0]
			}
		})
	}
}

type benchmarkPipe struct{ *io.PipeReader }

func (p benchmarkPipe) Write(v []byte) (int, error) { return len(v), nil }

// Run explicitly with -run ^$ -bench BenchmarkHighlightPrompt -benchtime=1x.
// This measures stream delivery, including the 20ms collection deadline, not UI.
func BenchmarkHighlightPrompt(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "off"
		if enabled {
			name = "on"
		}
		b.Run(name, func(b *testing.B) {
			samples := make([]time.Duration, 200)
			for i := range samples {
				r, w := io.Pipe()
				h := NewHighlightStream(benchmarkPipe{r})
				h.SetEnabled(enabled)
				h.SetRules(customRules(b, "prompt"))
				start := time.Now()
				written := make(chan error, 1)
				go func() { _, err := w.Write([]byte("prompt> ")); written <- err }()
				var p [256]byte
				n, err := h.Read(p[:])
				samples[i] = time.Since(start)
				if err != nil || n == 0 {
					b.Fatalf("prompt %d %v", n, err)
				}
				if err := <-written; err != nil {
					b.Fatal(err)
				}
				w.Close()
				h.Close()
			}
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			b.ReportMetric(float64(samples[100].Microseconds()), "p50-us")
			b.ReportMetric(float64(samples[189].Microseconds()), "p95-us")
			b.ReportMetric(float64(samples[199].Microseconds()), "max-us")
		})
	}
}
