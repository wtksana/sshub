package connection

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Files owns one SFTP channel. Canceling it never closes sibling channels.
type Files struct {
	*sftp.Client
	session *ssh.Session
	stop    func() bool
}

func (c *Client) OpenFiles(ctx context.Context) (*Files, error) {
	type result struct {
		session *ssh.Session
		err     error
	}
	ready := make(chan result)
	go func() {
		s, err := c.client.NewSession()
		select {
		case ready <- result{s, err}:
		case <-ctx.Done():
			if s != nil {
				s.Close()
			}
		}
	}()
	var s *ssh.Session
	select {
	case r := <-ready:
		if r.err != nil {
			return nil, r.err
		}
		s = r.session
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	stop := context.AfterFunc(ctx, func() { s.Close() })
	fail := func(err error) (*Files, error) {
		stop()
		s.Close()
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, err
	}
	in, err := s.StdinPipe()
	if err != nil {
		return fail(err)
	}
	out, err := s.StdoutPipe()
	if err != nil {
		return fail(err)
	}
	if err = s.RequestSubsystem("sftp"); err != nil {
		return fail(err)
	}
	client, err := sftp.NewClientPipe(out, in)
	if err != nil {
		return fail(err)
	}
	return &Files{Client: client, session: s, stop: stop}, nil
}

func (f *Files) Close() error {
	f.stop()
	// Close the SSH channel first so an unresponsive server cannot block cleanup.
	f.session.Close()
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
