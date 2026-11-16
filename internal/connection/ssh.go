package connection

import (
	"io"
	"sort"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Shell struct {
	session *ssh.Session
	in      io.WriteCloser
	out     io.Reader
	once    sync.Once
	err     error
}

func (c *Client) Shell() (*Shell, error) {
	s, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Shell, error) { s.Close(); return nil, err }
	in, err := s.StdinPipe()
	if err != nil {
		return fail(err)
	}
	out, err := s.StdoutPipe()
	if err != nil {
		return fail(err)
	}
	// Drain stderr too; an unread SSH stream can block the whole channel.
	s.Stderr = io.Discard
	if err = s.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		return fail(err)
	}
	if err = s.Shell(); err != nil {
		return fail(err)
	}
	return &Shell{session: s, in: in, out: out}, nil
}
func (s *Shell) Read(b []byte) (int, error)  { return s.out.Read(b) }
func (s *Shell) Write(b []byte) (int, error) { return s.in.Write(b) }
func (s *Shell) Resize(cols, rows int) error { return s.session.WindowChange(rows, cols) }
func (s *Shell) Close() error {
	s.once.Do(func() { s.in.Close(); s.err = s.session.Close() })
	return s.err
}
func (c *Client) ListHome() ([]string, error) {
	s, err := sftp.NewClient(c.client)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	files, err := s.ReadDir(".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		name := f.Name()
		if f.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
