package library

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxConfigBytes = 8 << 20

type Document struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// Store owns an OS-held lock until Close, including while the UI is idle.
// The lock is released by the OS if the process crashes.
type Store struct {
	mu     sync.Mutex
	path   string
	lock   *os.File
	doc    Document
	source []byte
}

func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "SSHub"), nil
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("创建配置目录：%w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "library.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("打开配置锁：%w", err)
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("配置目录正由另一个实例使用，或无法锁定；请关闭另一个实例后重试：%w", err)
	}
	s := &Store{path: filepath.Join(dir, "connections.json"), lock: f, doc: Document{Version: 1, Profiles: []Profile{}}}
	data, err := readConfig(s.path)
	if err != nil {
		s.Close()
		return nil, err
	}
	if data == nil {
		return s, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	s.doc = Document{}
	if err := dec.Decode(&s.doc); err != nil {
		s.Close()
		return nil, fmt.Errorf("配置文件无效，原文件已保留：%w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		s.Close()
		return nil, errors.New("配置文件包含多余内容，原文件已保留")
	}
	if err := validateDocument(s.doc); err != nil {
		s.Close()
		return nil, fmt.Errorf("配置文件无效，原文件已保留：%w", err)
	}
	s.source = data
	return s, nil
}

func readConfig(path string) ([]byte, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置：%w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取配置：%w", err)
	}
	if len(b) > maxConfigBytes {
		return nil, errors.New("配置文件超过 8 MiB，原文件已保留")
	}
	return b, nil
}

func validateDocument(doc Document) error {
	if doc.Version != 1 {
		return fmt.Errorf("不支持的配置版本 %d，请使用兼容版本的软件", doc.Version)
	}
	ids := map[string]bool{}
	for _, p := range doc.Profiles {
		if p.ID == "" || ids[p.ID] {
			return errors.New("连接配置 ID 缺失或重复")
		}
		ids[p.ID] = true
		if err := Validate(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Profiles() []Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.doc.Profiles)
}

func (s *Store) Upsert(p Profile) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.Name, p.Host, p.Username, p.Group, p.IdentityPath = strings.TrimSpace(p.Name), strings.TrimSpace(p.Host), strings.TrimSpace(p.Username), strings.TrimSpace(p.Group), strings.TrimSpace(p.IdentityPath)
	if err := Validate(p); err != nil {
		return Profile{}, err
	}
	for _, existing := range s.doc.Profiles {
		if existing.ID != p.ID && strings.EqualFold(existing.Name, p.Name) && strings.EqualFold(existing.Group, p.Group) {
			return Profile{}, errors.New("该分组中已有同名连接，请更换名称")
		}
	}
	next := Document{Version: 1, Profiles: slices.Clone(s.doc.Profiles)}
	if p.ID == "" {
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			return Profile{}, err
		}
		p.ID = hex.EncodeToString(id)
		next.Profiles = append(next.Profiles, p)
	} else {
		i := slices.IndexFunc(next.Profiles, func(v Profile) bool { return v.ID == p.ID })
		if i < 0 {
			return Profile{}, errors.New("连接配置已不存在，请重新选择")
		}
		p.LastUsed = next.Profiles[i].LastUsed
		next.Profiles[i] = p
	}
	return p, s.save(next)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.doc.Profiles, func(v Profile) bool { return v.ID == id })
	if i < 0 {
		return errors.New("连接配置已不存在")
	}
	next := Document{Version: 1, Profiles: slices.Delete(slices.Clone(s.doc.Profiles), i, i+1)}
	return s.save(next)
}

func (s *Store) MarkUsed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := Document{Version: 1, Profiles: slices.Clone(s.doc.Profiles)}
	i := slices.IndexFunc(next.Profiles, func(v Profile) bool { return v.ID == id })
	if i < 0 {
		return errors.New("连接配置已不存在")
	}
	next.Profiles[i].LastUsed = time.Now().UTC()
	return s.save(next)
}

func (s *Store) save(next Document) error {
	if s.lock == nil {
		return errors.New("配置存储已关闭")
	}
	current, err := readConfig(s.path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, s.source) {
		return errors.New("配置文件已被外部修改，请重启应用读取新文件；未覆盖原文件")
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > maxConfigBytes {
		return errors.New("配置总大小超过 8 MiB，未保存")
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".connections-*")
	if err != nil {
		return fmt.Errorf("创建保存文件：%w", err)
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("写入配置，原文件已保留：%w", err)
	}
	if err := os.Rename(f.Name(), s.path); err != nil {
		return fmt.Errorf("替换配置，原文件已保留：%w", err)
	}
	s.doc, s.source = next, data
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil
	}
	err := s.lock.Close()
	s.lock = nil
	return err
}
