package connection

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Files owns one SFTP channel. An unresponsive channel may close its transport.
type Files struct {
	*sftp.Client
	closeChannel func() error
	stop         func() bool
}

func (c *Client) OpenFiles(ctx context.Context) (*Files, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Opening a channel itself cannot be canceled through the SSH channel API.
	setupDone := make(chan struct{})
	stopSetup := context.AfterFunc(ctx, func() {
		timer := time.AfterFunc(time.Second, c.Close)
		defer timer.Stop()
		<-setupDone
	})
	defer stopSetup()
	defer close(setupDone)
	channel, requests, err := c.client.OpenChannel("session", nil)
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, err
	}
	closed := make(chan struct{})
	go func() { ssh.DiscardRequests(requests); close(closed) }()
	go io.Copy(io.Discard, channel.Stderr())
	closeChannel := sync.OnceValue(func() error {
		// Close only sends a request; wait for the peer, then force a dead transport closed.
		timer := time.AfterFunc(time.Second, c.Close)
		defer timer.Stop()
		err := channel.Close()
		<-closed
		return err
	})
	stop := context.AfterFunc(ctx, func() { closeChannel() })
	fail := func(err error) (*Files, error) {
		stop()
		closeChannel()
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, err
	}
	ok, err := channel.SendRequest("subsystem", true, ssh.Marshal(struct{ Name string }{"sftp"}))
	if err != nil {
		return fail(err)
	}
	if !ok {
		return fail(errors.New("服务器拒绝启动 SFTP 子系统"))
	}
	client, err := sftp.NewClientPipe(channel, channel)
	if err != nil {
		return fail(err)
	}
	if ctx.Err() != nil {
		stop()
		closeChannel()
		client.Close()
		return nil, ctx.Err()
	}
	return &Files{Client: client, closeChannel: closeChannel, stop: stop}, nil
}

func (f *Files) Close() error {
	f.stop()
	f.closeChannel()
	return f.Client.Close()
}

func (f *Files) Browse(ctx context.Context, dir string) (string, []os.FileInfo, error) {
	if dir == "" {
		dir = "."
	}
	canonical, err := f.RealPath(dir)
	if err != nil {
		return "", nil, err
	}
	entries, err := f.ReadDirContext(ctx, canonical)
	slices.SortFunc(entries, func(a, b os.FileInfo) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
	})
	return canonical, entries, err
}

func ValidName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
}

// RemoveTree inspects each entry with Lstat. Links are removed as links.
// As with other path-based SFTP operations, concurrent server-side renames
// cannot be made race-free by the SFTP v3 protocol.
func (f *Files) RemoveTree(ctx context.Context, target string) error {
	target = path.Clean(target)
	if target == "/" || target == "." {
		return errors.New("不能删除根目录或当前目录")
	}
	return f.remove(ctx, target, 0)
}

func (f *Files) remove(ctx context.Context, target string, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 128 {
		return errors.New("目录嵌套过深")
	}
	info, err := f.Lstat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return f.Remove(target)
	}
	entries, err := f.ReadDirContext(ctx, target)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !ValidName(entry.Name()) {
			return fmt.Errorf("无效远程文件名：%q", entry.Name())
		}
		if err := f.remove(ctx, path.Join(target, entry.Name()), depth+1); err != nil {
			return err
		}
	}
	return f.RemoveDirectory(target)
}
