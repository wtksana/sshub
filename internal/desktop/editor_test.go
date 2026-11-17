package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

func TestJumpSelectionKeepsIDsAcrossSameNamesAndGroups(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	var jumps []library.Profile
	for _, group := range []string{"生产", "开发"} {
		p, err := a.store.Upsert(library.Profile{Name: "gateway", Group: group, Host: group + ".invalid", Port: 22, Username: "test", Auth: "agent"})
		if err != nil {
			t.Fatal(err)
		}
		jumps = append(jumps, p)
	}
	target, err := a.store.Upsert(library.Profile{Name: "target", Host: "target.invalid", Port: 22, Username: "test", Auth: "agent", JumpID: jumps[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	a.refresh()
	a.beginEdit(target)
	a.saveDraft()
	saved, ok := a.selectedProfile()
	if !ok || a.draft.err != "" || saved.JumpID != jumps[0].ID {
		t.Fatal("saving without edits changed the jump", a.draft.err)
	}
	a.beginEdit(saved)
	view := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(8).Children(func() { a.jumpSelect(c) })
	}, 600, 400)
	for _, choice := range []struct{ label, id string }{{"经由：gateway · 开发", jumps[1].ID}, {"不使用跳板机", ""}} {
		label := "经由：gateway · 生产"
		if a.draft.profile.JumpID == jumps[1].ID {
			label = "经由：gateway · 开发"
		}
		if err := view.Click(label); err != nil {
			t.Fatal(err)
		}
		if err := view.Click(choice.label); err != nil {
			t.Fatal(err)
		}
		if a.draft.profile.JumpID != choice.id {
			t.Fatal("dropdown selected the wrong jump")
		}
		a.saveDraft()
		saved, _ = a.selectedProfile()
		if a.draft.err != "" || saved.JumpID != choice.id {
			t.Fatal("saving lost the selected jump", a.draft.err)
		}
	}
}

func TestJumpNameCannotBeConfusedWithDirectConnection(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	jump, err := a.store.Upsert(library.Profile{Name: "不使用跳板机", Host: "jump.invalid", Port: 22, Username: "test", Auth: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := a.store.Upsert(library.Profile{Name: "target", Host: "target.invalid", Port: 22, Username: "test", Auth: "agent", JumpID: jump.ID})
	if err != nil {
		t.Fatal(err)
	}
	a.refresh()
	a.beginEdit(target)
	a.saveDraft()
	if a.draft.err != "" {
		t.Fatal(a.draft.err)
	}
	p, ok := a.selectedProfile()
	if !ok || p.JumpID != jump.ID {
		t.Fatal("editing silently removed jump")
	}
	a.beginEdit(p)
	a.draft.profile.JumpID = ""
	a.saveDraft()
	p, ok = a.selectedProfile()
	if !ok || p.JumpID != "" {
		t.Fatal("explicit direct selection did not remove jump")
	}
}

func TestNewAndCopiedConnectionAuthentication(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	a.group = "分组：当前筛选"
	a.beginEdit(library.Profile{})
	if a.draft.auth != "密码" || a.draft.profile.Auth != "password" || a.draft.port != "22" || a.draft.profile.Group != "当前筛选" {
		t.Fatal("new connection did not default to password/current group")
	}
	source, err := a.store.Upsert(library.Profile{Name: "生产", Host: "server.invalid", Port: 2222, Username: "root", Auth: "private_key", IdentityPath: `C:\secret\key`, Favorite: true, IdentityNeedsReview: true})
	if err != nil {
		t.Fatal(err)
	}
	source.LastUsed = time.Now()
	a.refresh()
	a.beginCopy(source)
	copy := a.draft.profile
	if copy.ID != "" || copy.Auth != "password" || a.draft.auth != "密码" || copy.IdentityPath != "" || copy.IdentityNeedsReview || !copy.LastUsed.IsZero() {
		t.Fatalf("copy inherited authentication identity: %+v", copy)
	}
	if copy.Host != source.Host || copy.Port != 2222 || a.draft.port != "2222" || copy.Username != source.Username || copy.Group != "" || !copy.Favorite {
		t.Fatal("copy lost connection details or inherited filter group")
	}
	a.saveDraft()
	if a.editing || a.draft.err != "" {
		t.Fatal(a.draft.err)
	}
	saved, ok := a.selectedProfile()
	if !ok || saved.ID == source.ID || a.secretKey(saved, "password") == a.secretKey(source, "password") {
		t.Fatal("copy reused source credential identity")
	}
	for _, p := range a.store.Profiles() {
		if p.ID == source.ID && (p.Auth != "private_key" || p.IdentityPath != source.IdentityPath) {
			t.Fatal("copy changed source")
		}
	}
	a.beginCopy(source)
	if !strings.HasSuffix(a.draft.profile.Name, "副本 2") {
		t.Fatal("copy name collides")
	}
	before := len(a.profiles)
	a.editing = false
	if len(a.store.Profiles()) != before {
		t.Fatal("cancel created connection")
	}
}

func TestQuickGroupPreservesProfileAndRejectsConflicts(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	p, err := a.store.Upsert(library.Profile{Name: "server", Host: "server.invalid", Port: 22, Username: "test", Auth: "private_key", IdentityPath: `C:\key`, Group: "生产", Favorite: true})
	if err != nil {
		t.Fatal(err)
	}
	a.refresh()
	tab := a.openTab(p)
	// Group changes must use the latest stored data, while an open tab retains its snapshot.
	p.Host = "updated.invalid"
	if _, err = a.store.Upsert(p); err != nil {
		t.Fatal(err)
	}
	if err = a.setProfileGroup(p.ID, "  新组  "); err != nil {
		t.Fatal(err)
	}
	saved := a.profiles[0]
	if saved.Group != "新组" || saved.Host != p.Host || saved.Auth != p.Auth || saved.IdentityPath != p.IdentityPath || !saved.Favorite || saved.LastUsed.IsZero() {
		t.Fatal("group assignment lost profile data")
	}
	if tab.profile.Group != "生产" || tab.profile.Host != "server.invalid" {
		t.Fatal("group change modified active session")
	}
	if _, err = a.store.Upsert(library.Profile{Name: p.Name, Host: "other.invalid", Port: 22, Username: "test", Auth: "password", Group: "冲突"}); err != nil {
		t.Fatal(err)
	}
	if err = a.setProfileGroup(p.ID, "冲突"); err == nil {
		t.Fatal("duplicate name/group accepted")
	}
	if a.store.Profiles()[0].Group != "新组" {
		t.Fatal("failed move changed group")
	}
	if err = a.setProfileGroup(p.ID, ""); err != nil || a.profiles[0].Group != "" {
		t.Fatal("ungroup failed", err)
	}
	if err = a.setProfileGroup("deleted", "新组"); err == nil {
		t.Fatal("deleted profile silently recreated")
	}
}
