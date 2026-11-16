package testssh

import (
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

// The fixture's SFTP namespace is rooted independently from identities and
// application data. os.Root also rejects symlink escapes on supported systems.
type rootedFiles struct {
	root  *os.Root
	delay time.Duration
}

func fixturePath(name string) (string, error) {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if name == "denied" || strings.HasPrefix(name, "denied/") {
		return "", os.ErrPermission
	}
	if name == "" {
		name = "."
	}
	return name, nil
}

func (f *rootedFiles) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	name, err := fixturePath(r.Filepath)
	if err != nil {
		return nil, err
	}
	return f.root.Open(name)
}
func (f *rootedFiles) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	name, err := fixturePath(r.Filepath)
	if err != nil {
		return nil, err
	}
	p := r.Pflags()
	flags := os.O_WRONLY
	if p.Read {
		flags = os.O_RDWR
	}
	if p.Creat {
		flags |= os.O_CREATE
	}
	if p.Trunc {
		flags |= os.O_TRUNC
	}
	if p.Excl {
		flags |= os.O_EXCL
	}
	file, err := f.root.OpenFile(name, flags, 0600)
	if err != nil {
		return nil, err
	}
	return &fixtureWriter{File: file, delay: f.delay}, nil
}

type fixtureWriter struct {
	*os.File
	delay time.Duration
}

func (f *fixtureWriter) WriteAt(p []byte, off int64) (int, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.File.WriteAt(p, off)
}
func (f *rootedFiles) Filecmd(r *sftp.Request) error {
	name, err := fixturePath(r.Filepath)
	if err != nil {
		return err
	}
	switch r.Method {
	case "Mkdir":
		return f.root.Mkdir(name, 0755)
	case "Remove", "Rmdir":
		return f.root.Remove(name)
	case "Rename":
		target, err := fixturePath(r.Target)
		if err != nil {
			return err
		}
		if _, err = f.root.Lstat(target); err == nil {
			return os.ErrExist
		}
		return f.root.Rename(name, target)
	case "Setstat":
		if r.AttrFlags().Permissions {
			return f.root.Chmod(name, os.FileMode(r.Attributes().Mode).Perm())
		}
		return os.ErrInvalid
	case "Symlink":
		target, err := fixturePath(r.Target)
		if err != nil {
			return err
		}
		return f.root.Symlink(name, target)
	}
	return os.ErrInvalid
}
func (f *rootedFiles) PosixRename(r *sftp.Request) error {
	name, err := fixturePath(r.Filepath)
	if err != nil {
		return err
	}
	target, err := fixturePath(r.Target)
	if err != nil {
		return err
	}
	return f.root.Rename(name, target)
}

type fileList []os.FileInfo

func (f fileList) ListAt(dst []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(f)) {
		return 0, io.EOF
	}
	n := copy(dst, f[offset:])
	if n < len(dst) {
		return n, io.EOF
	}
	return n, nil
}
func (f *rootedFiles) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	name, err := fixturePath(r.Filepath)
	if err != nil {
		return nil, err
	}
	if r.Method == "List" {
		dir, err := f.root.Open(name)
		if err != nil {
			return nil, err
		}
		defer dir.Close()
		entries, err := dir.Readdir(-1)
		return fileList(entries), err
	}
	info, err := f.root.Stat(name)
	if err != nil {
		return nil, err
	}
	return fileList{info}, nil
}
func (f *rootedFiles) Lstat(r *sftp.Request) (sftp.ListerAt, error) {
	name, err := fixturePath(r.Filepath)
	if err != nil {
		return nil, err
	}
	info, err := f.root.Lstat(name)
	if err != nil {
		return nil, err
	}
	return fileList{info}, nil
}
