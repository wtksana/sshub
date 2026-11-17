package testssh

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// stream sends 20 fixed 101-byte UTF-8/ANSI lines every 20 ms. Blocking SSH
// writes provide back pressure; measurements must check actual bytes received.
func (s *Server) stream(channel ssh.Channel, done <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var sequence uint64
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
		}
		if !s.LoadEnabled.Load() {
			continue
		}
		var batch strings.Builder
		for range 20 {
			sequence++
			level := [...]string{"INFO", "WARN", "ERROR"}[sequence%3]
			line := fmt.Sprintf("\x1b[32m%010d\x1b[0m %s 2026-11-17T12:00:00Z {\"message\":\"性能\",\"id\":42} ", sequence, level)
			line += strings.Repeat("x", 99-len(line)) + "\r\n"
			batch.WriteString(line)
		}
		n, err := channel.Write([]byte(batch.String()))
		s.SentBytes.Add(uint64(n))
		if err != nil {
			return
		}
	}
}
