package transfer_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"sshub/internal/connection"
	"sshub/internal/testssh"
	"sshub/internal/transfer"
)

func fixture(t *testing.T) (*testssh.Server, *connection.Client) {
	t.Helper()
	s, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := connection.DialContext(ctx, connection.Options{Target: "test@" + s.Address(), Auth: "private_key", Identity: s.Identity,
		HostKey: func(_ context.Context, _ string, _ net.Addr, key ssh.PublicKey) error {
			if !bytes.Equal(key.Marshal(), s.Host.PublicKey().Marshal()) {
				return errors.New("unexpected host")
			}
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return s, c
}
func run(t *testing.T, c *connection.Client, request transfer.Request) (transfer.Progress, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f, err := c.OpenFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	return transfer.Run(ctx, f.Client, request)
}
func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func check(t *testing.T, name, want string) {
	t.Helper()
	got, err := os.ReadFile(name)
	if err != nil || string(got) != want {
		t.Fatalf("%s: %q, %v", name, got, err)
	}
}

func TestDirectoryRoundTripAndConflictDecisions(t *testing.T) {
	s, c := fixture(t)
	src := filepath.Join(t.TempDir(), "中文 文件夹")
	write(t, filepath.Join(src, "子目录", "文本 a.txt"), "你好 SFTP\n")
	write(t, filepath.Join(src, "b.txt"), "second")
	if err := os.Mkdir(filepath.Join(src, "空目录"), 0700); err != nil {
		t.Fatal(err)
	}
	request := transfer.Request{Upload: true, Source: src, Destination: "/中文 文件夹"}
	p, err := run(t, c, request)
	if err != nil || p.Files != 2 {
		t.Fatalf("upload: %+v %v", p, err)
	}
	check(t, filepath.Join(s.Root, "中文 文件夹", "子目录", "文本 a.txt"), "你好 SFTP\n")
	calls := 0
	request.Conflict = func(context.Context, string) (transfer.Decision, error) {
		calls++
		return transfer.Decision{Choice: transfer.Skip, ApplyAll: true}, nil
	}
	p, err = run(t, c, request)
	if err != nil || p.Skipped != 2 || calls != 1 {
		t.Fatalf("skip batch: %+v calls=%d err=%v", p, calls, err)
	}
	request.Conflict = func(context.Context, string) (transfer.Decision, error) {
		return transfer.Decision{Choice: transfer.Overwrite, ApplyAll: true}, nil
	}
	if _, err = run(t, c, request); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "下载")
	p, err = run(t, c, transfer.Request{Source: "/中文 文件夹", Destination: dst})
	if err != nil || p.Files != 2 {
		t.Fatalf("download: %+v %v", p, err)
	}
	check(t, filepath.Join(dst, "子目录", "文本 a.txt"), "你好 SFTP\n")
	request = transfer.Request{Upload: true, Source: filepath.Join(src, "b.txt"), Destination: "/中文 文件夹/b.txt", Conflict: func(context.Context, string) (transfer.Decision, error) {
		return transfer.Decision{Choice: transfer.SaveAs, Name: "另存.txt"}, nil
	}}
	if _, err = run(t, c, request); err != nil {
		t.Fatal(err)
	}
	check(t, filepath.Join(s.Root, "中文 文件夹", "另存.txt"), "second")
}

func TestRemoteDiskFullPreservesDestinationAndCleansPartial(t *testing.T) {
	s, c := fixture(t)
	s.WriteLimit = 64 << 10
	src := filepath.Join(t.TempDir(), "large.txt")
	write(t, src, strings.Repeat("x", 256<<10))
	dest := filepath.Join(s.Root, "original.txt")
	write(t, dest, "original")
	p, err := run(t, c, transfer.Request{Upload: true, Source: src, Destination: "/original.txt", Conflict: func(context.Context, string) (transfer.Decision, error) {
		return transfer.Decision{Choice: transfer.Overwrite}, nil
	}})
	if err == nil || p.Files != 0 {
		t.Fatal("disk full reported success", p, err)
	}
	check(t, dest, "original")
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "original.txt" {
		t.Fatal("failed upload left partial files", entries)
	}
}

func TestCancelPreservesDestinationAndRetryStartsFromZero(t *testing.T) {
	s, c := fixture(t)
	src := filepath.Join(t.TempDir(), "large.bin")
	content := strings.Repeat("0123456789", 100000)
	write(t, src, content)
	write(t, filepath.Join(s.Root, "large.bin"), "original")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, err := c.OpenFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	request := transfer.Request{Upload: true, Source: src, Destination: "/large.bin", Conflict: func(context.Context, string) (transfer.Decision, error) {
		return transfer.Decision{Choice: transfer.Overwrite}, nil
	}, Progress: func(p transfer.Progress) {
		if p.Bytes > 0 {
			cancel()
		}
	}}
	p, err := transfer.Run(ctx, f.Client, request)
	f.Close()
	if !errors.Is(err, context.Canceled) || p.Files != 0 || p.Bytes == 0 {
		t.Fatalf("cancel: %+v %v", p, err)
	}
	check(t, filepath.Join(s.Root, "large.bin"), "original")
	request.Progress = nil
	p, err = run(t, c, request)
	if err != nil || p.Bytes != int64(len(content)) || p.Files != 1 {
		t.Fatalf("retry: %+v %v", p, err)
	}
	check(t, filepath.Join(s.Root, "large.bin"), content)
}

func TestDownloadCancelCleansPartialAndPreservesOldFile(t *testing.T) {
	s, c := fixture(t)
	write(t, filepath.Join(s.Root, "file.bin"), strings.Repeat("a", 1024*1024))
	dst := filepath.Join(t.TempDir(), "file.bin")
	write(t, dst, "old")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, err := c.OpenFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := transfer.Run(ctx, f.Client, transfer.Request{Source: "/file.bin", Destination: dst, Conflict: func(context.Context, string) (transfer.Decision, error) {
		return transfer.Decision{Choice: transfer.Overwrite}, nil
	}, Progress: func(p transfer.Progress) {
		if p.Bytes > 0 {
			cancel()
		}
	}})
	if !errors.Is(err, context.Canceled) || p.Files != 0 {
		t.Fatalf("%+v %v", p, err)
	}
	check(t, dst, "old")
	partials, _ := filepath.Glob(filepath.Join(filepath.Dir(dst), ".sshub-*.partial"))
	if len(partials) != 0 {
		t.Fatalf("local leftovers: %v", partials)
	}
}

func TestDeniedAndDisconnectedTransfersDoNotPublish(t *testing.T) {
	s, c := fixture(t)
	src := filepath.Join(t.TempDir(), "data")
	write(t, src, strings.Repeat("b", 1024*1024))
	if _, err := run(t, c, transfer.Request{Upload: true, Source: src, Destination: "/denied/data"}); err == nil {
		t.Fatal("permission failure expected")
	}
	request := transfer.Request{Upload: true, Source: src, Destination: "/data", Progress: func(p transfer.Progress) {
		if p.Bytes > 0 {
			s.Disconnect()
		}
	}}
	p, err := run(t, c, request)
	if err == nil || p.Files != 0 {
		t.Fatalf("disconnect: %+v %v", p, err)
	}
	if _, err := os.Stat(filepath.Join(s.Root, "data")); !os.IsNotExist(err) {
		t.Fatalf("published incomplete file: %v", err)
	}
}

func TestCanceledFileChannelKeepsShellAndOtherChannelsAlive(t *testing.T) {
	_, c := fixture(t)
	shell, err := c.Shell()
	if err != nil {
		t.Fatal(err)
	}
	defer shell.Close()
	ctx, cancel := context.WithCancel(context.Background())
	f, err := c.OpenFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	f.Close()
	other, err := c.OpenFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, _, err := other.Browse(context.Background(), "/"); err != nil {
		t.Fatal(err)
	}
	if _, err := shell.Write([]byte("channel-alive\n")); err != nil {
		t.Fatal(err)
	}
	ready := make(chan string, 1)
	go func() { b := make([]byte, 256); n, _ := shell.Read(b); ready <- string(b[:n]) }()
	select {
	case out := <-ready:
		if out == "" {
			t.Fatal("shell closed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shell blocked")
	}
}

func TestSymlinksNeverTraversedAndDeleteKeepsTarget(t *testing.T) {
	s, c := fixture(t)
	write(t, filepath.Join(s.Root, "keep", "safe.txt"), "safe")
	if err := os.Mkdir(filepath.Join(s.Root, "remove"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(s.Root, "keep"), filepath.Join(s.Root, "remove", "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "download")
	if _, err := run(t, c, transfer.Request{Source: "/remove", Destination: dst}); err == nil {
		t.Fatal("must reject recursive symlink transfer")
	}
	f, err := c.OpenFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.RemoveTree(context.Background(), "/remove"); err != nil {
		t.Fatal(err)
	}
	check(t, filepath.Join(s.Root, "keep", "safe.txt"), "safe")
	if err := f.RemoveTree(context.Background(), "/"); err == nil {
		t.Fatal("root deletion accepted")
	}
}

func TestLocalDestinationSymlinkEscapeRejected(t *testing.T) {
	s, c := fixture(t)
	write(t, filepath.Join(s.Root, "dir", "file.txt"), "remote")
	local, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(local, "dir")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := run(t, c, transfer.Request{Source: "/dir", Destination: filepath.Join(local, "dir")}); err == nil {
		t.Fatal("must reject symlink destination")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("escaped destination root")
	}
}
