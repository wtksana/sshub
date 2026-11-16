//go:build windows

package connection

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh/agent"
)

func TestWindowsAgentNamedPipe(t *testing.T) {
	s := server(t)
	pipe := fmt.Sprintf(`\\.\pipe\sshub-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(pipe, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: s.UserKey}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer raw.Close(); agent.ServeAgent(keyring, raw) }()
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", pipe)
	o := trustedOptions(t, s)
	o.Auth = "agent"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialContext(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	t.Setenv("SSH_AUTH_SOCK", pipe+"-missing")
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if c, err := DialContext(ctx, o); err == nil {
		c.Close()
		t.Fatal("missing agent connected")
	}
}
