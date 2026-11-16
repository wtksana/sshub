package desktop

import (
	"sshub/internal/library"
	"testing"
)

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
	a.draft.jumpLabel = "不使用跳板机"
	a.saveDraft()
	p, ok = a.selectedProfile()
	if !ok || p.JumpID != "" {
		t.Fatal("explicit direct selection did not remove jump")
	}
}
