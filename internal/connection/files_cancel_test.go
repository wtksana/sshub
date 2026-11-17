package connection

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type pausedFilesConn struct {
	net.Conn
	paused atomic.Bool
	resume <-chan struct{}
	onRead func()
}

func (c *pausedFilesConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if c.paused.Load() {
		c.onRead()
		<-c.resume
	}
	return n, err
}

// Stop processing encrypted packets without sending EOF, as a network outage does.
func stalledFilesOptions(t *testing.T, phase string) (Options, <-chan struct{}) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	resume, entered := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(resume); listener.Close() })
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		paused := &pausedFilesConn{Conn: raw, resume: resume, onRead: func() {}}
		config := &ssh.ServerConfig{NoClientAuth: true}
		config.AddHostKey(signer)
		server, channels, requests, err := ssh.NewServerConn(paused, config)
		if err != nil {
			t.Error(err)
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		incoming, ok := <-channels
		if !ok {
			return
		}
		stall := func() {
			paused.paused.Store(true)
			close(entered)
			<-resume
		}
		if phase == "open" {
			stall()
			return
		}
		channel, channelRequests, err := incoming.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer channel.Close()
		request, ok := <-channelRequests
		if !ok {
			return
		}
		if phase == "subsystem" {
			stall()
			return
		}
		if err := request.Reply(true, nil); err != nil {
			t.Error(err)
			return
		}
		var init [9]byte
		if _, err := io.ReadFull(channel, init[:]); err != nil {
			t.Error(err)
			return
		}
		if phase == "version" {
			stall()
			return
		}
		// SFTP v3 version response, without extensions.
		if _, err := channel.Write([]byte{0, 0, 0, 5, 2, 0, 0, 0, 3}); err != nil {
			t.Error(err)
			return
		}
		paused.onRead = sync.OnceFunc(func() { close(entered) })
		paused.paused.Store(true)
		<-resume
	}()
	return Options{
		Target: "test@" + listener.Addr().String(), Auth: "password",
		HostKey: func(_ context.Context, _ string, _ net.Addr, key ssh.PublicKey) error {
			if !bytes.Equal(key.Marshal(), signer.PublicKey().Marshal()) {
				return errors.New("unexpected fixture host")
			}
			return nil
		},
	}, entered
}

func TestFileCancellationClosesUnresponsiveTransport(t *testing.T) {
	for _, viaJump := range []bool{false, true} {
		for _, phase := range []string{"open", "subsystem", "version", "request", "close"} {
			t.Run(fmt.Sprintf("jump=%v/%s", viaJump, phase), func(t *testing.T) {
				o, entered := stalledFilesOptions(t, phase)
				if viaJump {
					jump := trustedOptions(t, server(t))
					o.Jump = &jump
				}
				client, err := DialContext(context.Background(), o)
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() {
					files, err := client.OpenFiles(ctx)
					if err == nil {
						if phase == "request" {
							_, _, err = files.Browse(ctx, "/")
						}
						files.Close()
					}
					done <- err
				}()
				select {
				case <-entered:
				case <-time.After(3 * time.Second):
					t.Fatal("fixture did not reach stalled operation")
				}
				cancel()
				select {
				case err := <-done:
					if phase != "request" && phase != "close" && !errors.Is(err, context.Canceled) {
						t.Fatalf("cancellation error lost: %v", err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("file operation remained blocked after cancellation")
				}
				transportDone := make(chan struct{})
				go func() { client.client.Wait(); close(transportDone) }()
				select {
				case <-transportDone:
				case <-time.After(time.Second):
					t.Fatal("stalled transport survived channel cleanup")
				}
			})
		}
	}
}
