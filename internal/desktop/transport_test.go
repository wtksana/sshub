package desktop

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sshub/internal/connection"
	"sshub/internal/testssh"
)

func TestClosingTerminalKeepsTransferTransportUntilLastOwner(t *testing.T) {
	s, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := connection.DialContext(ctx, connection.Options{Target: "test@" + s.Address(), Auth: "private_key", Identity: s.Identity, HostKey: func(_ context.Context, _ string, _ net.Addr, key ssh.PublicKey) error {
		if !bytes.Equal(key.Marshal(), s.Host.PublicKey().Marshal()) {
			return errors.New("unexpected host")
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	shell, err := c.Shell()
	if err != nil {
		t.Fatal(err)
	}
	transport := newTransport(c)
	if !transport.retain() {
		t.Fatal("retain transfer")
	}
	tab := &workspace{client: c, transport: transport, shell: shell, status: "已连接"}
	a := &app{}
	a.release(tab)
	f, err := c.OpenFiles(ctx)
	if err != nil {
		t.Fatalf("terminal close interrupted transfer: %v", err)
	}
	if err := f.Mkdir("/after-terminal-close"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	transport.release()
	if transport.retain() {
		t.Fatal("closed transport was reused")
	}
	deadline := time.Now().Add(time.Second)
	for {
		probe, err := c.OpenFiles(ctx)
		if err != nil {
			break
		}
		probe.Close()
		if time.Now().After(deadline) {
			t.Fatal("final transfer did not close transport")
		}
	}
}
