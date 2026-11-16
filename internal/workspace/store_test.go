package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveRestoreLayoutAndPreferences(t *testing.T) {
	dir := t.TempDir()
	store, s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Add("server-one")
	s.Split("horizontal")
	id, _ := s.Add("server-two")
	_, tab := s.Locate(id)
	tab.FontSize = 18
	s.Root.Ratio = 63
	s.HideRight = true
	s.LeftSize = 300
	s.ContentPercent = 75
	s.WindowWidth, s.WindowHeight = 1440, 900
	data, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(data); err != nil {
		t.Fatal(err)
	}
	_, restored, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Encode(restored)
	if !bytes.Equal(got, data) {
		t.Fatal("restored state differs")
	}
	for _, forbidden := range []string{"password", "passphrase", "identity_path", "已连接", "output", "HostKey"} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatalf("layout contains non-restorable data: %s", forbidden)
		}
	}
}

func TestBadFilesRemainUnchangedDuringThisRun(t *testing.T) {
	for _, bad := range []string{"{broken", "null", `{"version":2}`, `{"version":1,"password":"must-not-copy"}`, "{} {}"} {
		dir := t.TempDir()
		path := filepath.Join(dir, "workspace.json")
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		store, fallback, err := Load(dir)
		if err == nil {
			t.Fatal("bad file accepted")
		}
		if err := fallback.Validate(); err != nil {
			t.Fatal("no usable fallback")
		}
		data, _ := Encode(fallback)
		if err := store.Save(data); err == nil {
			t.Fatal("bad file overwritten")
		}
		got, _ := os.ReadFile(path)
		if string(got) != bad {
			t.Fatal("bad file changed")
		}
	}
}

func TestExternalChangesAndFailedWritePreserveSavedLayout(t *testing.T) {
	dir := t.TempDir()
	store, s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := Encode(s)
	if err := store.Save(data); err != nil {
		t.Fatal(err)
	}
	s.Add("server")
	next, _ := Encode(s)
	path := filepath.Join(dir, "workspace.json")
	external := []byte("external edit")
	os.WriteFile(path, external, 0600)
	if err := store.Save(next); err == nil {
		t.Fatal("external edit overwritten")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, external) {
		t.Fatal("external edit changed")
	}
	other := t.TempDir()
	store, s, err = Load(other)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = Encode(s)
	if err := store.Save(data); err != nil {
		t.Fatal(err)
	}
	if err := store.Save([]byte(`{"version":2}`)); err == nil {
		t.Fatal("invalid state saved")
	}
	got, _ = os.ReadFile(filepath.Join(other, "workspace.json"))
	if !bytes.Equal(got, data) {
		t.Fatal("rejected save modified previous layout")
	}
}
