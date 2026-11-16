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
			line := fmt.Sprintf("\x1b[32m%010d\x1b[0m SSHub 性能 load ", sequence)
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
