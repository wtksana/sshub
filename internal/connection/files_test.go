package connection_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sshub/internal/connection"
	"sshub/internal/testssh"
)

func TestFileBrowserAndManagement(t *testing.T) {
	s, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := range 1000 {
		if err := os.WriteFile(filepath.Join(s.Root, fmt.Sprintf("file-%04d", i)), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	f, err := c.OpenFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Mkdir("/中文 目录"); err != nil {
		t.Fatal(err)
	}
	dir, entries, err := f.Browse(ctx, "")
	if err != nil || dir != "/" || len(entries) != 1001 || !entries[0].IsDir() {
		t.Fatalf("browse %q (%d entries): %v", dir, len(entries), err)
	}
	if err := f.Rename("/中文 目录", "/新目录"); err != nil {
		t.Fatal(err)
	}
	if err := f.Rename("/file-0000", "/file-0001"); err == nil {
		t.Fatal("rename must not overwrite")
	}
	if err := f.Chmod("/file-0000", 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.Browse(ctx, "/denied"); err == nil {
		t.Fatal("permission failure expected")
	}
	if _, _, err := f.Browse(ctx, "/missing"); !os.IsNotExist(err) {
		t.Fatalf("missing directory: %v", err)
	}
	if err := f.RemoveTree(ctx, "/新目录"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Root, "新目录")); !os.IsNotExist(err) {
		t.Fatalf("directory remains: %v", err)
	}
}
