package connection

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sshub/internal/testssh"
)

func server(t *testing.T) *testssh.Server {
	t.Helper()
	s, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func trustedOptions(t *testing.T, s *testssh.Server) Options {
	t.Helper()
	trust := NewTrustStore(t.TempDir())
	return Options{Target: "test@" + s.Address(), Auth: "password", Secret: func(context.Context, string) (string, error) { return testssh.Password, nil },
		HostKey: func(ctx context.Context, host string, remote net.Addr, key ssh.PublicKey) error {
			return trust.Verify(ctx, host, remote, key, func(context.Context, HostIdentity) (bool, error) { return true, nil })
		},
	}
}

func TestPasswordAndPrivateKeyAuthentication(t *testing.T) {
	s := server(t)
	for _, mode := range []string{"password", "private_key", "encrypted", "bad-password", "bad-passphrase", "missing-key"} {
		t.Run(mode, func(t *testing.T) {
			o := trustedOptions(t, s)
			secret := testssh.Password
			switch mode {
			case "private_key":
				o.Auth, o.Identity = "private_key", s.Identity
			case "encrypted", "bad-passphrase":
				o.Auth, o.Identity, secret = "private_key", s.EncryptedIdentity, testssh.Passphrase
			case "missing-key":
				o.Auth, o.Identity = "private_key", filepath.Join(t.TempDir(), "missing")
			}
			if strings.HasPrefix(mode, "bad-") {
				secret = "wrong-secret-must-not-leak"
			}
			o.Secret = func(_ context.Context, kind string) (string, error) {
				if o.Auth == "private_key" && kind != "passphrase" {
					t.Errorf("wrong prompt kind: %s", kind)
				}
				return secret, nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, err := DialContext(ctx, o)
			if strings.HasPrefix(mode, "bad-") || mode == "missing-key" {
				if err == nil {
					client.Close()
					t.Fatal("invalid credentials accepted")
				}
				if strings.Contains(err.Error(), secret) {
					t.Fatal("secret leaked in error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			client.Close()
			client.Close()
		})
	}
}

func TestTrustConfirmationRotationAndForget(t *testing.T) {
	s := server(t)
	dir := t.TempDir()
	trust := NewTrustStore(dir)
	remote, _ := net.ResolveTCPAddr("tcp", s.Address())
	ctx := context.Background()
	calls := 0
	confirm := func(_ context.Context, host HostIdentity) (bool, error) {
		calls++
		if host.Fingerprint != ssh.FingerprintSHA256(s.Host.PublicKey()) {
			t.Fatal("wrong fingerprint")
		}
		return true, nil
	}
	if err := trust.Verify(ctx, s.Address(), remote, s.Host.PublicKey(), func(context.Context, HostIdentity) (bool, error) { return false, nil }); err == nil {
		t.Fatal("rejected host trusted")
	}
	if hosts, _ := trust.Hosts(); len(hosts) != 0 {
		t.Fatal("rejected host persisted")
	}
	for i := 0; i < 2; i++ {
		if err := trust.Verify(ctx, s.Address(), remote, s.Host.PublicKey(), confirm); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("known host prompted again")
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	rotated, _ := ssh.NewSignerFromKey(key)
	if err := trust.Verify(ctx, s.Address(), remote, rotated.PublicKey(), confirm); err == nil || !strings.Contains(err.Error(), "变化") {
		t.Fatalf("rotation allowed: %v", err)
	}
	if calls != 1 {
		t.Fatal("rotated key offered trust prompt")
	}
	// Hosts returns the normalized OpenSSH host:port representation.
	hosts, err := trust.Hosts()
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range hosts {
		if err := trust.Forget(host.Address); err != nil {
			t.Fatal(err)
		}
	}
	if hosts, _ := trust.Hosts(); len(hosts) != 0 {
		t.Fatal("forget failed")
	}
	canceled, cancel := context.WithCancel(ctx)
	if err := trust.Verify(canceled, s.Address(), remote, s.Host.PublicKey(), func(context.Context, HostIdentity) (bool, error) { cancel(); return true, nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled trust: %v", err)
	}
	if hosts, _ := trust.Hosts(); len(hosts) != 0 {
		t.Fatal("canceled host persisted")
	}
	path := filepath.Join(dir, "known_hosts")
	bad := []byte("invalid record\n")
	os.WriteFile(path, bad, 0600)
	if err := trust.Verify(ctx, s.Address(), remote, s.Host.PublicKey(), confirm); err == nil {
		t.Fatal("corrupt trust accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(bad) {
		t.Fatal("corrupt trust overwritten")
	}
}

func TestCancelHandshakeAndPrompt(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		raw, err := listener.Accept()
		if err == nil {
			accepted <- raw
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	o := Options{Target: "test@" + listener.Addr().String(), Auth: "password", HostKey: func(context.Context, string, net.Addr, ssh.PublicKey) error { return nil }}
	go func() { _, err := DialContext(ctx, o); result <- err }()
	select {
	case raw := <-accepted:
		defer raw.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("no connection")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("handshake cancellation hung")
	}
	s := server(t)
	o = trustedOptions(t, s)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	o.HostKey = func(ctx context.Context, _ string, _ net.Addr, _ ssh.PublicKey) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}
	go func() { _, err := DialContext(ctx, o); result <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("prompt never reached")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("prompt cancellation hung")
	}
}

func TestJumpVerifiesBothHosts(t *testing.T) {
	jump, target := server(t), server(t)
	o, j := trustedOptions(t, target), trustedOptions(t, jump)
	seen := map[string]bool{}
	check := func(ctx context.Context, host string, remote net.Addr, key ssh.PublicKey) error {
		seen[host] = true
		var want ssh.PublicKey
		if host == jump.Address() {
			want = jump.Host.PublicKey()
		} else {
			want = target.Host.PublicKey()
		}
		if string(want.Marshal()) != string(key.Marshal()) {
			return errors.New("incorrect host identity")
		}
		return nil
	}
	o.HostKey, j.HostKey = check, check
	o.Jump = &j
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialContext(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !seen[jump.Address()] || !seen[target.Address()] {
		t.Fatalf("host not verified: %v", seen)
	}
	shell, err := c.Shell()
	if err != nil {
		t.Fatal(err)
	}
	shell.Close()
	o.HostKey = func(context.Context, string, net.Addr, ssh.PublicKey) error { return errors.New("reject target") }
	if c, err := DialContext(ctx, o); err == nil {
		c.Close()
		t.Fatal("jump bypassed target verification")
	}
	j.Jump = &Options{}
	if c, err := DialContext(ctx, o); err == nil {
		c.Close()
		t.Fatal("nested jump accepted")
	}
}
