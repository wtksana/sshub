package desktop

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sshub/internal/connection"
	"sshub/internal/library"
	"sshub/internal/testssh"
	layout "sshub/internal/workspace"
)

func TestQueuedUploadsKeepServerAndPathAfterFocusAndProfileChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a := &app{workCtx: ctx, transferQueue: make(chan *transferTask, 2), layout: layout.New()}
	roots := make([]string, 2)
	for i := range 2 {
		s, err := testssh.Start(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(s.Close)
		roots[i] = s.Root
		client, err := connection.DialContext(ctx, connection.Options{
			Target: "test@" + s.Address(), Auth: "private_key", Identity: s.Identity,
			HostKey: func(_ context.Context, _ string, _ net.Addr, key ssh.PublicKey) error {
				if !bytes.Equal(key.Marshal(), s.Host.PublicKey().Marshal()) {
					return errors.New("unexpected fixture host")
				}
				return nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		transport := newTransport(client)
		t.Cleanup(transport.release)
		id, err := a.layout.Add(s.Address())
		if err != nil {
			t.Fatal(err)
		}
		a.tabs = append(a.tabs, &workspace{key: id, client: client, transport: transport,
			profile: library.Profile{ID: s.Address(), Name: s.Address()}, files: fileBrowser{path: "/"}})
	}
	source := filepath.Join(t.TempDir(), "中文 上传.txt")
	if err := os.WriteFile(source, []byte("bound to original server and directory"), 0600); err != nil {
		t.Fatal(err)
	}
	first, second := a.tabs[0], a.tabs[1]
	a.layout.Activate(first.key)
	a.upload(first, "/", []string{source})
	first.profile = second.profile
	first.files.path = "/changed-after-enqueue"
	a.layout.Activate(second.key)
	if a.activeWorkspace() != second {
		t.Fatal("focus did not switch")
	}
	task := <-a.transferQueue
	defer task.transport.release()
	a.runTransfer(task)
	want, _ := os.ReadFile(source)
	got, err := os.ReadFile(filepath.Join(roots[0], filepath.Base(source)))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("original server lost queued upload: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(roots[1], filepath.Base(source))); !os.IsNotExist(err) {
		t.Fatal("focus switch redirected upload to another server", err)
	}
	if task.server == second.profile.Name || task.profile.ID == second.profile.ID || task.request.Destination != "/中文 上传.txt" {
		t.Fatal("queued metadata followed live profile or path")
	}
}
