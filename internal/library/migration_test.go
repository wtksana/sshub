package library

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func portable(id, name string) PortableProfile {
	return PortableProfile{ID: id, Name: name, Host: "example.com", Port: 22, Username: "test", Auth: "password", Group: "生产"}
}

func bundle(profiles ...PortableProfile) Migration {
	if profiles == nil {
		profiles = []PortableProfile{}
	}
	p := Preferences{Theme: "dark", FontFamily: "Consolas", FontSize: 16}
	return Migration{Format: "sshub", Version: 1, Profiles: profiles, Preferences: &p}
}

func TestPreferencesSurviveCRUDAndRestart(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	if !reflect.DeepEqual(s.Preferences(), DefaultPreferences()) {
		t.Fatal("legacy defaults missing")
	}
	prefs := Preferences{Theme: "dark", FontFamily: "Consolas", FontSize: 17}
	if err := s.SavePreferences(prefs); err != nil {
		t.Fatal(err)
	}
	p, err := s.Upsert(profile("server"))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MarkUsed(p.ID); err != nil {
		t.Fatal(err)
	}
	p.Favorite = true
	if _, err = s.Upsert(p); err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = openTestStore(t, dir)
	if !reflect.DeepEqual(s.Preferences(), prefs) {
		t.Fatal("CRUD lost preferences")
	}
	for _, bad := range []Preferences{{Theme: "unknown", FontFamily: "x", FontSize: 13}, {Theme: "dark", FontSize: 13}, {Theme: "dark", FontFamily: "x\n", FontSize: 13}, {Theme: "dark", FontFamily: "x", FontSize: 33}} {
		if s.SavePreferences(bad) == nil || !reflect.DeepEqual(s.Preferences(), prefs) {
			t.Fatal("invalid prefs changed store")
		}
	}
}

func TestMigrationExportAllowlistAndPrivateKeyReview(t *testing.T) {
	s := openTestStore(t, t.TempDir())
	p := profile("key")
	p.Auth = "private_key"
	p.IdentityPath = `C:\keys\id_ed25519`
	p, err := s.Upsert(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MarkUsed(p.ID); err != nil {
		t.Fatal(err)
	}
	data, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 4 {
		t.Fatalf("unexpected export fields: %s", data)
	}
	var profiles []map[string]json.RawMessage
	json.Unmarshal(raw["profiles"], &profiles)
	allowed := map[string]bool{"id": true, "name": true, "host": true, "port": true, "username": true, "auth": true, "identity_path": true, "group": true, "favorite": true, "jump_id": true}
	for key := range profiles[0] {
		if !allowed[key] {
			t.Fatalf("unapproved field: %s", key)
		}
	}
	m, err := DecodeMigration(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	dst := openTestStore(t, t.TempDir())
	plan, err := dst.PreviewImport(*m)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Rows[0].Profile.IdentityNeedsReview {
		t.Fatal("private key not gated")
	}
	if _, err = dst.CommitImport(plan, []ImportDecision{{Mode: "add"}}, true); err != nil {
		t.Fatal(err)
	}
	got := dst.Profiles()[0]
	if got.ID == p.ID || !got.IdentityNeedsReview || !got.LastUsed.IsZero() || got.IdentityPath != p.IdentityPath {
		t.Fatalf("unsafe migration: %+v", got)
	}
}

func TestMigrationRejectsMalformedAndUnknownFields(t *testing.T) {
	valid, _ := json.Marshal(bundle(portable("a", "a")))
	for _, input := range []string{"null", "{}", string(valid) + " {}", string(valid) + " garbage", strings.Replace(string(valid), `"format":"sshub"`, `"password":"secret","format":"sshub"`, 1), strings.Replace(string(valid), `"auth":"password"`, `"auth":"password","passphrase":"secret"`, 1), strings.Repeat(" ", maxConfigBytes+1)} {
		if _, err := DecodeMigration(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted invalid JSON: %.100s", input)
		}
	}
	dup := portable("b", " A ")
	badJump := portable("b", "b")
	badJump.JumpID = "missing"
	for _, m := range []Migration{bundle(portable("a", "a"), dup), bundle(portable("a", "a"), portable("a", "b")), bundle(badJump), {Format: "sshub", Version: 2, Profiles: []PortableProfile{}}} {
		data, _ := json.Marshal(m)
		if _, err := DecodeMigration(bytes.NewReader(data)); err == nil {
			t.Fatal("accepted invalid bundle")
		}
	}
}

func TestMigrationConflictUsesUnicodeCaseFolding(t *testing.T) {
	s := openTestStore(t, t.TempDir())
	if _, err := s.Upsert(profile("Σ")); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PreviewImport(bundle(portable("a", "ς")))
	if err != nil || !plan.Rows[0].Conflict {
		t.Fatal("Unicode conflict not recognized", err)
	}
	if _, err = s.PreviewImport(bundle(portable("a", "Σ"), portable("b", "ς"))); err == nil {
		t.Fatal("Unicode duplicate accepted")
	}
}

func TestImportConflictChoicesAndJumpRemapping(t *testing.T) {
	for _, mode := range []string{"skip", "rename", "replace"} {
		t.Run(mode, func(t *testing.T) {
			s := openTestStore(t, t.TempDir())
			jump, err := s.Upsert(profile("jump"))
			if err != nil {
				t.Fatal(err)
			}
			local := profile("local")
			local.JumpID = jump.ID
			local, err = s.Upsert(local)
			if err != nil {
				t.Fatal(err)
			}
			incomingJump := portable("j", "jump")
			incomingJump.Host = "new.example"
			target := portable("t", "target")
			target.JumpID = "j"
			m := bundle(target, incomingJump)
			before := s.Profiles()
			plan, err := s.PreviewImport(m)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.Profiles(), before) {
				t.Fatal("preview wrote profiles")
			}
			if !plan.Rows[1].Conflict {
				t.Fatal("conflict missing")
			}
			// Callers cannot alter the canonical preview by changing display data.
			plan.Rows[0].Profile.Host = "evil.example"
			plan.Preferences.FontSize = 32
			result, err := s.CommitImport(plan, []ImportDecision{{Mode: "add"}, {Mode: mode, Name: "jump copy"}}, true)
			if err != nil {
				t.Fatal(err)
			}
			if (mode == "skip" && result.Count != 1) || (mode != "skip" && result.Count != 2) {
				t.Fatal(result.Count)
			}
			byName := map[string]Profile{}
			for _, p := range s.Profiles() {
				byName[p.Name] = p
			}
			if byName["target"].Host != "example.com" || s.Preferences().FontSize != 16 {
				t.Fatal("mutable preview compromised commit")
			}
			switch mode {
			case "skip":
				if byName["target"].JumpID != jump.ID || byName["jump"].Host != jump.Host {
					t.Fatal("skip did not retain local jump")
				}
			case "rename":
				if byName["target"].JumpID != byName["jump copy"].ID || byName["local"].JumpID != jump.ID {
					t.Fatal("rename changed local references")
				}
			case "replace":
				newID := byName["jump"].ID
				if newID == jump.ID || newID == incomingJump.ID || byName["target"].JumpID != newID || byName["local"].JumpID != newID {
					t.Fatal("replacement references not remapped")
				}
				if local.JumpID != jump.ID {
					t.Fatal("old value snapshot changed")
				}
			}
		})
	}
}

func TestImportFailuresAreAtomic(t *testing.T) {
	for _, scenario := range []string{"unresolved", "rename collision", "skipped dependency", "stale", "external", "closed", "cross store", "invalid resulting graph"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			s := openTestStore(t, dir)
			local, err := s.Upsert(profile("jump"))
			if err != nil {
				t.Fatal(err)
			}
			j := portable("j", "jump")
			target := portable("t", "target")
			target.JumpID = "j"
			m := bundle(j, target)
			decisions := []ImportDecision{{Mode: "replace"}, {Mode: "add"}}
			switch scenario {
			case "unresolved":
				decisions[0].Mode = ""
			case "rename collision":
				decisions[0] = ImportDecision{Mode: "rename", Name: "jump"}
			case "skipped dependency":
				m.Profiles[0].Name = "missing"
				decisions[0].Mode = "skip"
			case "invalid resulting graph":
				localTarget := profile("local target")
				localTarget.JumpID = local.ID
				if _, err = s.Upsert(localTarget); err != nil {
					t.Fatal(err)
				}
				// Replacing a local jump with a jump-dependent profile would create two hops.
				m.Profiles[0].JumpID = "t"
				m.Profiles[1].JumpID = ""
			}
			plan, err := s.PreviewImport(m)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "stale":
				if err = s.SavePreferences(DefaultPreferences()); err != nil {
					t.Fatal(err)
				}
			case "external":
				if err = os.WriteFile(s.path, []byte("external"), 0600); err != nil {
					t.Fatal(err)
				}
			case "closed":
				s.Close()
			case "cross store":
				s = openTestStore(t, t.TempDir())
			}
			before, _ := os.ReadFile(s.path)
			profiles, prefs := s.Profiles(), s.Preferences()
			if _, err = s.CommitImport(plan, decisions, true); err == nil {
				t.Fatal("expected failure")
			}
			after, _ := os.ReadFile(s.path)
			if !bytes.Equal(before, after) || !reflect.DeepEqual(profiles, s.Profiles()) || !reflect.DeepEqual(prefs, s.Preferences()) {
				t.Fatal("failed import changed data")
			}
		})
	}
}

func TestExportDestinationProtectionAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	if _, err := s.Upsert(profile("server")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"connections.json", "workspace.json", "known_hosts", "library.lock"} {
		if err := s.ExportTo(filepath.Join(dir, name), true); err == nil {
			t.Fatal("overwrote managed file", name)
		}
	}
	dst := filepath.Join(t.TempDir(), "export.json")
	if err := s.ExportTo(dst, false); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(dst)
	if err := s.ExportTo(dst, false); err == nil {
		t.Fatal("silent overwrite")
	}
	if after, _ := os.ReadFile(dst); !bytes.Equal(before, after) {
		t.Fatal("changed unconfirmed file")
	}
	if err := s.ExportTo(dst, true); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias.json")
	if err := os.Link(s.path, alias); err != nil {
		t.Fatal(err)
	}
	if err := s.ExportTo(alias, true); err == nil {
		t.Fatal("overwrote managed hard link")
	}
}
