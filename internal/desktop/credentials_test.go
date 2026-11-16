//go:build windows

package desktop

import (
	"context"
	"testing"
	"time"

	"sshub/internal/library"
)

func TestCredentialsAreIsolatedAndRememberRequiresConsent(t *testing.T) {
	a := &app{dir: t.TempDir(), prompts: make(chan *connectionPrompt, 4)}
	p := library.Profile{ID: "fixture", Host: "127.0.0.1", Port: 22, Username: "test", Auth: "password"}
	key := a.secretKey(p, "password")
	t.Cleanup(func() { a.vault().Delete(key) })
	other := &app{dir: t.TempDir()}
	if other.secretKey(p, "password") == key {
		t.Fatal("data directories share credentials")
	}
	changed := p
	changed.Host = "other.invalid"
	if a.secretKey(changed, "password") == key {
		t.Fatal("edited server reuses old credentials")
	}
	if a.secretKey(p, "passphrase") == key {
		t.Fatal("password and passphrase share credentials")
	}
	for _, remember := range []bool{false, true} {
		saved := map[string]string{}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result := make(chan error, 1)
		go func() {
			value, err := a.requestSecret(ctx, p, "password", saved)
			if err == nil && value != "fixture-secret" {
				t.Error("prompt value lost")
			}
			result <- err
		}()
		select {
		case prompt := <-a.prompts:
			if prompt.remember {
				t.Fatal("remember defaults to enabled")
			}
			prompt.reply <- promptReply{accepted: true, value: "fixture-secret", remember: remember}
		case <-ctx.Done():
			t.Fatal("secret prompt not requested")
		}
		if err := <-result; err != nil {
			t.Fatal(err)
		}
		cancel()
		if _, err := a.vault().Get(key); !secretMissing(err) {
			t.Fatalf("prompt persisted a secret before successful connection: %v", err)
		}
		if remember && saved[key] != "fixture-secret" {
			t.Fatal("explicit consent lost")
		}
		if !remember && len(saved) != 0 {
			t.Fatal("secret retained without consent")
		}
	}
	if err := a.vault().Set(key, []byte("remembered")); err != nil {
		t.Fatal(err)
	}
	value, err := a.requestSecret(context.Background(), p, "password", map[string]string{})
	if err != nil || value != "remembered" {
		t.Fatalf("saved credential unavailable: %v", err)
	}
	if err := a.vault().Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, err := a.vault().Get(key); !secretMissing(err) {
		t.Fatal("credential not removed")
	}
}
