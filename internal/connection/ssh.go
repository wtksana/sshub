package connection

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Options struct{ Target, Identity, KnownHosts string }
type Client struct {
	client *ssh.Client
	agent  net.Conn
}

func Dial(o Options) (*Client, error) {
	user, address, ok := strings.Cut(o.Target, "@")
	if !ok || user == "" || address == "" {
		return nil, fmt.Errorf("target must be user@host:port")
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		if strings.Contains(address, ":") {
			return nil, fmt.Errorf("include port and bracket IPv6: %w", err)
		}
		address = net.JoinHostPort(address, "22")
	}
	if o.KnownHosts == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		o.KnownHosts = filepath.Join(home, ".ssh", "known_hosts")
	}
	check, err := knownhosts.New(o.KnownHosts)
	if err != nil {
		return nil, fmt.Errorf("known_hosts: %w", err)
	}
	c := &Client{}
	var auth ssh.AuthMethod
	if o.Identity != "" {
		key, err := os.ReadFile(o.Identity)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse private key (encrypted keys must be unlocked by an agent): %w", err)
		}
		auth = ssh.PublicKeys(signer)
	} else {
		socket := os.Getenv("SSH_AUTH_SOCK")
		if socket == "" {
			return nil, fmt.Errorf("set Identity or SSH_AUTH_SOCK for public key authentication")
		}
		c.agent, err = net.DialTimeout("unix", socket, 5*time.Second)
		if err != nil {
			return nil, err
		}
		auth = ssh.PublicKeysCallback(agent.NewClient(c.agent).Signers)
	}
	conn, err := net.DialTimeout("tcp", address, 8*time.Second)
	if err != nil {
		c.Close()
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: check})
	if err != nil {
		conn.Close()
		c.Close()
		return nil, err
	}
	conn.SetDeadline(time.Time{})
	c.client = ssh.NewClient(sshConn, chans, reqs)
	return c, nil
}

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
func (c *Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
	if c.agent != nil {
		c.agent.Close()
	}
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
