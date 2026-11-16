package connection

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestSSHAndSFTPAgainstLocalServer(t *testing.T) {
	options, sizes := fixture(t)
	c, err := Dial(options)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	shell, err := c.Shell()
	if err != nil {
		t.Fatal(err)
	}
	defer shell.Close()
	if err := shell.Resize(132, 43); err != nil {
		t.Fatal(err)
	}
	select {
	case size := <-sizes:
		if size != [2]uint32{132, 43} {
			t.Fatalf("remote dimensions: %v", size)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("remote received no resize")
	}
	message := []byte("SSH echo 中文\n")
	if _, err := shell.Write(message); err != nil {
		t.Fatal(err)
	}
	read := make(chan error, 1)
	go func() {
		buf := make([]byte, len(message))
		_, err := io.ReadFull(shell, buf)
		if err == nil && string(buf) != string(message) {
			err = fmt.Errorf("echo differs: %q", buf)
		}
		read <- err
	}()
	select {
	case err := <-read:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("echo timed out")
	}
	names, err := c.ListHome()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(names, "fixture.txt") {
		t.Fatalf("SFTP fixture missing: %v", names)
	}
}

func TestSSHRejectsUnknownAndChangedHostKeys(t *testing.T) {
	o, _ := fixture(t)
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint("changed=", changed), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "known_hosts")
			var contents string
			if changed {
				_, key, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				signer, err := ssh.NewSignerFromKey(key)
				if err != nil {
					t.Fatal(err)
				}
				_, address, _ := strings.Cut(o.Target, "@")
				contents = knownhosts.Line([]string{address}, signer.PublicKey()) + "\n"
			}
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			copyOf := o
			copyOf.KnownHosts = path
			client, err := Dial(copyOf)
			if err == nil {
				client.Close()
				t.Fatal("untrusted host unexpectedly connected")
			}
		})
	}
}

func fixture(t *testing.T) (Options, <-chan [2]uint32) {
	t.Helper()
	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	_, userKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	userSigner, err := ssh.NewSignerFromKey(userKey)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if string(key.Marshal()) != string(userSigner.PublicKey().Marshal()) {
			return nil, fmt.Errorf("unauthorized")
		}
		return nil, nil
	}}
	config.AddHostKey(hostSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.txt"), []byte("sftp fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(userKey, "")
	if err != nil {
		t.Fatal(err)
	}
	identity := filepath.Join(dir, "identity")
	if err := os.WriteFile(identity, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}
	known := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(known, []byte(knownhosts.Line([]string{listener.Addr().String()}, hostSigner.PublicKey())+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sizes := make(chan [2]uint32, 4)
	go func() {
		for {
			tcp, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				conn, channels, requests, err := ssh.NewServerConn(tcp, config)
				if err != nil {
					tcp.Close()
					return
				}
				defer conn.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						incoming.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, reqs, err := incoming.Accept()
					if err != nil {
						continue
					}
					go func() {
						defer channel.Close()
						for req := range reqs {
							switch req.Type {
							case "pty-req":
								req.Reply(true, nil)
							case "window-change":
								var size struct{ Columns, Rows, Width, Height uint32 }
								if ssh.Unmarshal(req.Payload, &size) == nil {
									sizes <- [2]uint32{size.Columns, size.Rows}
								}
							case "shell":
								req.Reply(true, nil)
								go io.Copy(channel, channel)
							case "subsystem":
								var subsystem struct{ Name string }
								ssh.Unmarshal(req.Payload, &subsystem)
								if subsystem.Name != "sftp" {
									req.Reply(false, nil)
									continue
								}
								req.Reply(true, nil)
								server, err := sftp.NewServer(channel, sftp.ReadOnly(), sftp.WithServerWorkingDirectory(dir))
								if err == nil {
									server.Serve()
									server.Close()
								}
								return
							default:
								req.Reply(false, nil)
							}
						}
					}()
				}
			}()
		}
	}()
	return Options{Target: "test@" + listener.Addr().String(), Identity: identity, KnownHosts: known}, sizes
}
