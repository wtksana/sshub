// Package testssh provides a loopback-only SSH fixture for protocol and UI tests.
package testssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Server struct {
	listener                    net.Listener
	mu                          sync.Mutex
	connections                 map[net.Conn]bool
	closed                      bool
	Host                        ssh.Signer
	UserKey                     ed25519.PrivateKey
	Identity, EncryptedIdentity string
	Sizes                       chan [2]uint32
	Root                        string
	WriteDelay                  time.Duration
	WriteLimit                  int64
	LoadEnabled                 atomic.Bool
	ShellCount                  atomic.Int64
	SFTPRequests                atomic.Uint64
	SentBytes                   atomic.Uint64
}

const Password = "fixture-password-中文"
const Passphrase = "fixture-passphrase"

func Start(dir string) (*Server, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	host, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, err
	}
	_, user, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	s := &Server{Host: host, UserKey: user, Sizes: make(chan [2]uint32, 128), connections: map[net.Conn]bool{}}
	s.Root = filepath.Join(dir, "files")
	if err := os.MkdirAll(s.Root, 0700); err != nil {
		return nil, err
	}
	for _, encrypted := range []bool{false, true} {
		var block *pem.Block
		name := "identity"
		if encrypted {
			name = "identity-encrypted"
			block, err = ssh.MarshalPrivateKeyWithPassphrase(user, "fixture", []byte(Passphrase))
		} else {
			block, err = ssh.MarshalPrivateKey(user, "fixture")
		}
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dir, name)
		if err = os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
			return nil, err
		}
		if encrypted {
			s.EncryptedIdentity = path
		} else {
			s.Identity = path
		}
	}
	s.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	public, err := ssh.NewSignerFromKey(user)
	if err != nil {
		s.Close()
		return nil, err
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if string(password) != Password {
				return nil, errors.New("unauthorized")
			}
			return nil, nil
		},
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) != string(public.PublicKey().Marshal()) {
				return nil, errors.New("unauthorized")
			}
			return nil, nil
		},
	}
	config.AddHostKey(host)
	go func() {
		for {
			raw, err := s.listener.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				raw.Close()
				return
			}
			s.connections[raw] = true
			s.mu.Unlock()
			go func() {
				defer func() { raw.Close(); s.mu.Lock(); delete(s.connections, raw); s.mu.Unlock() }()
				conn, channels, requests, err := ssh.NewServerConn(raw, config)
				if err != nil {
					return
				}
				defer conn.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					go s.channel(incoming)
				}
			}()
		}
	}()
	return s, nil
}

func (s *Server) Address() string { return s.listener.Addr().String() }
func (s *Server) Close() {
	s.listener.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for raw := range s.connections {
		raw.Close()
	}
}

// Disconnect simulates a dropped server connection while keeping its listener.
func (s *Server) Disconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for raw := range s.connections {
		raw.Close()
	}
}

func (s *Server) channel(incoming ssh.NewChannel) {
	if incoming.ChannelType() == "direct-tcpip" {
		var request struct {
			Host       string
			Port       uint32
			Origin     string
			OriginPort uint32
		}
		if ssh.Unmarshal(incoming.ExtraData(), &request) != nil || request.Host != "127.0.0.1" {
			incoming.Reject(ssh.Prohibited, "loopback only")
			return
		}
		raw, err := net.Dial("tcp", net.JoinHostPort(request.Host, fmt.Sprint(request.Port)))
		if err != nil {
			incoming.Reject(ssh.ConnectionFailed, "unavailable")
			return
		}
		channel, reqs, err := incoming.Accept()
		if err != nil {
			raw.Close()
			return
		}
		defer raw.Close()
		defer channel.Close()
		go ssh.DiscardRequests(reqs)
		go func() { io.Copy(raw, channel); raw.Close() }()
		io.Copy(channel, raw)
		return
	}
	if incoming.ChannelType() != "session" {
		incoming.Reject(ssh.UnknownChannelType, "unsupported")
		return
	}
	channel, reqs, err := incoming.Accept()
	if err != nil {
		return
	}
	defer channel.Close()
	done := make(chan struct{})
	defer close(done)
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			req.Reply(true, nil)
		case "window-change":
			var size struct{ Columns, Rows, Width, Height uint32 }
			if ssh.Unmarshal(req.Payload, &size) == nil {
				select {
				case s.Sizes <- [2]uint32{size.Columns, size.Rows}:
				default:
				}
			}
		case "shell":
			req.Reply(true, nil)
			s.ShellCount.Add(1)
			go func() {
				defer s.ShellCount.Add(-1)
				io.WriteString(channel, "\r\nSSHub local SSH fixture ready\r\n")
				io.Copy(channel, channel)
			}()
			go s.stream(channel, done)
		case "subsystem":
			var subsystem struct{ Name string }
			if ssh.Unmarshal(req.Payload, &subsystem) != nil || subsystem.Name != "sftp" {
				req.Reply(false, nil)
				continue
			}
			s.SFTPRequests.Add(1)
			root, err := os.OpenRoot(s.Root)
			if err != nil {
				req.Reply(false, nil)
				continue
			}
			req.Reply(true, nil)
			h := &rootedFiles{root: root, delay: s.WriteDelay, limit: s.WriteLimit}
			server := sftp.NewRequestServer(channel, sftp.Handlers{FileGet: h, FilePut: h, FileCmd: h, FileList: h})
			go ssh.DiscardRequests(reqs)
			server.Serve()
			server.Close()
			root.Close()
			return
		default:
			req.Reply(false, nil)
		}
	}
}
