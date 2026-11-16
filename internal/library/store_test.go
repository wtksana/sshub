package library

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func profile(name string) Profile {
	return Profile{Name: name, Host: "example.com", Port: 22, Username: "root", Auth: "agent", Group: "生产"}
}

func openTestStore(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCRUDAndRecentSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	p, err := s.Upsert(profile("  香港服务器  "))
	if err != nil || p.ID == "" || p.Name != "香港服务器" {
		t.Fatalf("create: %+v, %v", p, err)
	}
	if err := s.MarkUsed(p.ID); err != nil {
		t.Fatal(err)
	}
	p.Host, p.Favorite, p.Group = "::1", true, "测试"
	if _, err := s.Upsert(p); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTestStore(t, dir)
	got := s.Profiles()
	if len(got) != 1 || got[0].Host != "::1" || !got[0].Favorite || got[0].Group != "测试" || got[0].LastUsed.IsZero() {
		t.Fatalf("restart lost data: %+v", got)
	}
	got[0].Name = "must not mutate store"
	if s.Profiles()[0].Name != p.Name {
		t.Fatal("caller mutated store through snapshot")
	}
	if err := s.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if got := openTestStore(t, dir).Profiles(); len(got) != 0 {
		t.Fatalf("deleted connection reappeared: %+v", got)
	}
}

func TestInvalidFilesArePreservedAndLockIsReleased(t *testing.T) {
	for _, input := range []string{
		`{broken`, ``, `null`, `{"version":2,"profiles":[]}`, `{"version":1,"profiles":[]} {}`,
		`{"version":1,"profiles":[],"future_feature":true}`,
		`{"version":1,"profiles":[{"id":"x","name":"bad","host":"a","port":0,"username":"r","auth":"agent"}]}`,
		`{"version":1,"profiles":[{"id":"x","name":"a","host":"a","port":22,"username":"r","auth":"agent"},{"id":"x","name":"b","host":"b","port":22,"username":"r","auth":"agent"}]}`,
	} {
		t.Run(input, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "connections.json")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if s, err := Open(dir); err == nil {
				s.Close()
				t.Fatal("invalid file was accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != input {
				t.Fatalf("invalid file was changed: %q, %v", got, err)
			}
			if err := os.WriteFile(path, []byte(`{"version":1,"profiles":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			openTestStore(t, dir)
		})
	}
}

func TestLockPreventsSecondWriterAndReopens(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	if second, err := Open(dir); err == nil {
		second.Close()
		t.Fatal("second writer acquired the same directory")
	}
	s.Close()
	openTestStore(t, dir)
}

func TestExternalChangeRejectsSaveWithoutChangingMemory(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	p, err := s.Upsert(profile("original"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "connections.json")
	original, _ := os.ReadFile(path)
	external := []byte(`{"version":1,"profiles":[]}`)
	if err := os.WriteFile(path, external, 0600); err != nil {
		t.Fatal(err)
	}
	p.Name = "changed"
	if _, err := s.Upsert(p); err == nil {
		t.Fatal("external change was overwritten")
	}
	if err := s.Delete(p.ID); err == nil {
		t.Fatal("delete ignored external change")
	}
	if err := s.MarkUsed(p.ID); err == nil {
		t.Fatal("recent update ignored external change")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, external) || s.Profiles()[0].Name != "original" || !s.Profiles()[0].LastUsed.IsZero() {
		t.Fatal("failed save changed disk or in-memory state")
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Upsert(p); err != nil {
		t.Fatal("retry after restoring file:", err)
	}
}

func TestSearchAndFiltersWith100Profiles(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	var doc Document
	doc.Version = 1
	for i := 0; i < 100; i++ {
		p := profile(fmt.Sprintf("server-%03d", i))
		p.ID, p.Favorite = fmt.Sprint(i), i%10 == 0
		if i%2 == 0 {
			p.Group = "测试"
		}
		doc.Profiles = append(doc.Profiles, p)
	}
	data, _ := json.Marshal(doc)
	s.Close()
	if err := os.WriteFile(filepath.Join(dir, "connections.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	s = openTestStore(t, dir)
	for _, id := range []string{"20", "40"} {
		if err := s.MarkUsed(id); err != nil {
			t.Fatal(err)
		}
	}
	all := s.Profiles()
	if len(Filter(all, "SERVER-042", "", "")) != 1 || len(Filter(all, "", "测试", "")) != 50 || len(Filter(all, "", "", "favorites")) != 10 {
		t.Fatal("search/group/favorite filtering failed")
	}
	if got := Filter(all, "", "", "recent"); len(got) != 2 || got[0].ID != "40" || got[1].ID != "20" {
		t.Fatalf("recent order: %+v", got)
	}
	if got := Groups(all); len(got) != 2 {
		t.Fatalf("groups: %v", got)
	}
}

func TestValidationRejectsBadFieldsAndDuplicateName(t *testing.T) {
	s := openTestStore(t, t.TempDir())
	if _, err := s.Upsert(profile("server")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Upsert(profile("SERVER")); err == nil {
		t.Fatal("duplicate name accepted")
	}
	for _, mutate := range []func(*Profile){
		func(p *Profile) { p.Name = "" }, func(p *Profile) { p.Host = "ssh://example.com" },
		func(p *Profile) { p.Host = "example.com:22" }, func(p *Profile) { p.Username = "bad user" },
		func(p *Profile) { p.Port = 65536 }, func(p *Profile) { p.Name = "a\nb" },
		func(p *Profile) { p.Auth = "private_key" }, func(p *Profile) { p.Auth = "invalid" },
	} {
		p := profile("new")
		mutate(&p)
		if _, err := s.Upsert(p); err == nil {
			t.Fatalf("invalid profile accepted: %+v", p)
		}
	}
	if len(s.Profiles()) != 1 {
		t.Fatal("invalid save changed state")
	}
}

func TestClosedStoreCannotSave(t *testing.T) {
	s := openTestStore(t, t.TempDir())
	s.Close()
	if _, err := s.Upsert(profile("new")); err == nil {
		t.Fatal("closed store accepted a write")
	}
}
