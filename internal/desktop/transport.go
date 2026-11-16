package desktop

import (
	"sshub/internal/connection"
	"sync"
)

// A terminal and each transfer own one reference. Network cleanup runs off
// the UI thread; the final owner closes the SSH transport and its jump host.
type sharedTransport struct {
	mu     sync.Mutex
	client *connection.Client
	refs   int
}

func newTransport(client *connection.Client) *sharedTransport {
	return &sharedTransport{client: client, refs: 1}
}

func (s *sharedTransport) retain() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refs == 0 {
		return false
	}
	s.refs++
	return true
}

func (s *sharedTransport) release() {
	s.mu.Lock()
	s.refs--
	last := s.refs == 0
	s.mu.Unlock()
	if last {
		go s.client.Close()
	}
}
