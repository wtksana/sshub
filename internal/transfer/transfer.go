// Package transfer copies a captured file tree over SFTP. It never owns the
// parent SSH connection; the caller owns its lifetime and queue policy.
package transfer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"sshub/internal/connection"
)

type Choice int

const (
	Overwrite Choice = iota + 1
	Skip
	SaveAs
)

type Decision struct {
	Choice   Choice
	Name     string
	ApplyAll bool
}

type Progress struct {
	Path                       string
	Destination                string
	Bytes, Total               int64
	CurrentBytes, CurrentTotal int64
	Files, Skipped             int
}

type Request struct {
	Upload              bool
	Source, Destination string
	Conflict            func(context.Context, string) (Decision, error)
	Progress            func(Progress)
}

// Run writes a fresh sibling temporary file, closes it successfully, then
// publishes it. Retry repeats the request from the beginning, without resume.
func Run(ctx context.Context, remote *sftp.Client, request Request) (Progress, error) {
	local := request.Source
	if !request.Upload {
		local = request.Destination
	}
	local, err := filepath.Abs(local)
	if err != nil {
		return Progress{}, err
	}
	root, err := os.OpenRoot(filepath.Dir(local))
	if err != nil {
		return Progress{}, err
	}
	defer root.Close()
	r := &runner{ctx: ctx, remote: remote, local: root, request: request}
	src, dst := filepath.Base(local), request.Destination
	if !request.Upload {
		src, dst = request.Source, filepath.Base(local)
	}
	err = r.copy(src, dst, 0)
	if ctx.Err() != nil && !errors.Is(err, ctx.Err()) {
		err = errors.Join(ctx.Err(), err)
	}
	return r.progress, err
}

type runner struct {
	ctx      context.Context
	remote   *sftp.Client
	local    *os.Root
	request  Request
	progress Progress
	policy   Choice
}

func (r *runner) stat(uploadSide bool, name string) (os.FileInfo, error) {
	if uploadSide {
		return r.local.Lstat(name)
	}
	return r.remote.Lstat(name)
}

func (r *runner) copy(src, dst string, depth int) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if depth > 128 {
		return errors.New("目录嵌套过深")
	}
	info, err := r.stat(r.request.Upload, src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("未传输符号链接：%s", src)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return fmt.Errorf("不支持的文件类型：%s", src)
	}
	existing, err := r.stat(!r.request.Upload, dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if existing.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("目标为符号链接，未写入：%s", dst)
		}
		if info.IsDir() != existing.IsDir() {
			return fmt.Errorf("目标类型不同：%s", dst)
		}
		if !info.IsDir() {
			d := Decision{Choice: r.policy}
			if d.Choice == 0 {
				if r.request.Conflict == nil {
					return fmt.Errorf("目标已存在：%s", dst)
				}
				d, err = r.request.Conflict(r.ctx, dst)
				if err != nil {
					return err
				}
				if d.ApplyAll && d.Choice != SaveAs {
					r.policy = d.Choice
				}
			}
			switch d.Choice {
			case Skip:
				r.progress.Skipped++
				r.notify()
				return nil
			case SaveAs:
				if !connection.ValidName(d.Name) || !filepath.IsLocal(d.Name) || strings.Contains(d.Name, ":") {
					return errors.New("另存名称无效")
				}
				return r.copy(src, path.Join(path.Dir(dst), d.Name), depth+1)
			case Overwrite:
			default:
				return errors.New("无效的冲突处理方式")
			}
		}
	}
	if info.IsDir() {
		if existing == nil {
			if r.request.Upload {
				err = r.remote.Mkdir(dst)
			} else {
				err = r.local.Mkdir(dst, 0755)
			}
			if err != nil {
				return err
			}
		}
		var entries []os.FileInfo
		if r.request.Upload {
			f, openErr := r.local.Open(src)
			if openErr != nil {
				return openErr
			}
			entries, err = f.Readdir(-1)
			f.Close()
		} else {
			entries, err = r.remote.ReadDirContext(r.ctx, src)
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if !connection.ValidName(name) || !filepath.IsLocal(name) || strings.Contains(name, ":") {
				return fmt.Errorf("不安全的文件名：%q", name)
			}
			if err := r.copy(path.Join(src, name), path.Join(dst, name), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return r.file(src, dst, info.Size(), existing)
}

func (r *runner) notify() {
	if r.request.Progress != nil {
		r.request.Progress(r.progress)
	}
}

func (r *runner) file(src, dst string, size int64, existing os.FileInfo) (err error) {
	r.progress.Path, r.progress.Total = src, r.progress.Total+size
	r.progress.Destination = dst
	r.progress.CurrentBytes, r.progress.CurrentTotal = 0, size
	r.notify()
	var input io.ReadCloser
	var output io.WriteCloser
	if r.request.Upload {
		input, err = r.local.Open(src)
	} else {
		input, err = r.remote.Open(src)
	}
	if err != nil {
		return err
	}
	defer input.Close()
	tmp := path.Join(path.Dir(dst), ".sshub-"+rand.Text()+".partial")
	if r.request.Upload {
		output, err = r.remote.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	} else {
		output, err = r.local.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	}
	if err != nil {
		return err
	}
	published := false
	defer func() {
		output.Close()
		if !published {
			// A dead channel may prevent remote cleanup. The queue explicitly reports
			// this path so the incomplete object can be found and removed later.
			var cleanup error
			if r.request.Upload {
				cleanup = r.remote.Remove(tmp)
			} else {
				cleanup = r.local.Remove(tmp)
			}
			if cleanup != nil && !os.IsNotExist(cleanup) {
				err = fmt.Errorf("%w；未完成临时文件可能保留：%s（%v）", err, tmp, cleanup)
			}
		}
	}()
	if r.request.Upload {
		// Keep partially received content private, then preserve an overwritten
		// file's permissions when publishing the replacement.
		if err = r.remote.Chmod(tmp, 0600); err != nil {
			return err
		}
	}
	buffer := make([]byte, 128*1024)
	var copied int64
	for {
		if err = r.ctx.Err(); err != nil {
			return err
		}
		n, readErr := input.Read(buffer)
		if n > 0 {
			written, writeErr := output.Write(buffer[:n])
			copied += int64(written)
			r.progress.CurrentBytes = copied
			r.progress.Bytes += int64(written)
			r.notify()
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if copied != size {
		return errors.New("源文件大小在传输中发生变化，请重试")
	}
	if err = output.Close(); err != nil {
		return err
	}
	if err = r.ctx.Err(); err != nil {
		return err
	}
	if r.request.Upload {
		if existing != nil {
			if err = r.remote.Chmod(tmp, existing.Mode().Perm()); err != nil {
				return err
			}
			err = r.remote.PosixRename(tmp, dst)
		} else {
			err = r.remote.Rename(tmp, dst)
		}
	} else {
		// Re-check before publishing; do not replace a newly appeared destination
		// without a decision. os.Root keeps the entire write within this directory.
		if existing == nil {
			if _, e := r.local.Lstat(dst); e == nil {
				return errors.New("目标在传输过程中出现，请重试并确认覆盖")
			} else if !os.IsNotExist(e) {
				return e
			}
		}
		if existing != nil {
			if err = r.local.Chmod(tmp, existing.Mode().Perm()); err != nil {
				return err
			}
		}
		err = r.local.Rename(tmp, dst)
	}
	if err != nil {
		return err
	}
	published = true
	r.progress.Files++
	r.notify()
	return nil
}
