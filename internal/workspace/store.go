package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const maxBytes = 1 << 20

// Store uses the application's existing data-directory lock. Only its writer
// goroutine may call Save; load failures keep the store read-only for this run.
type Store struct {
	path    string
	source  []byte
	blocked bool
}

func Load(dir string) (*Store, *State, error) {
	s := &Store{path: filepath.Join(dir, "workspace.json")}
	data, err := read(s.path)
	if err != nil {
		s.blocked = true
		return s, New(), err
	}
	s.source = data
	if data == nil {
		return s, New(), nil
	}
	state, err := decode(data)
	if err != nil {
		s.blocked = true
		return s, New(), err
	}
	return s, state, nil
}

func Encode(state *State) ([]byte, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	return append(data, '\n'), err
}

func decode(data []byte) (*State, error) {
	var state *State
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&state); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("工作区布局包含多余内容")
	}
	if err := state.Validate(); err != nil {
		return nil, err
	}
	return state, nil
}

func read(path string) ([]byte, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, errors.New("工作区布局超过 1 MiB，原文件已保留")
	}
	return data, nil
}

func (s *Store) Save(data []byte) error {
	if s.blocked {
		return errors.New("工作区布局未能读取，本次不会覆盖原文件")
	}
	if len(data) > maxBytes {
		return errors.New("工作区布局超过 1 MiB")
	}
	if _, err := decode(data); err != nil {
		return err
	}
	if bytes.Equal(data, s.source) {
		return nil
	}
	current, err := read(s.path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, s.source) {
		s.blocked = true
		return errors.New("工作区布局已被外部修改，未覆盖；请重启应用")
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".workspace-*")
	if err != nil {
		return err
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
		return err
	}
	if err := os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	s.source = append([]byte(nil), data...)
	return nil
}
