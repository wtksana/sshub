package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"sshub/internal/library"
)

func TestRestartRestoresPendingSessionsWithoutSecretsOrOutput(t *testing.T) {
	dir := t.TempDir()
	a := newApp(dir)
	p, err := a.store.Upsert(library.Profile{Name: "server", Host: "127.0.0.1", Port: 22, Username: "test", Auth: "password"})
	if err != nil {
		t.Fatal(err)
	}
	a.openTab(p)
	one := a.activeTab()
	a.openTab(p)
	two := a.activeTab()
	one.status, two.status = "已断开", "连接失败"
	a.splitPane("horizontal")
	panes := a.layout.Panes()
	if err := a.layout.Move(two.key, panes[1].ID); err != nil {
		t.Fatal(err)
	}
	a.zoom(two, 3, false)
	a.hideLeft = true
	a.contentPercent = 71
	a.close()
	b := newApp(dir)
	defer b.close()
	if len(b.tabs) != 2 || len(b.layout.Panes()) != 2 || !b.hideLeft || b.contentPercent != 71 {
		t.Fatal("layout not restored")
	}
	for _, tab := range b.tabs {
		if tab.status != "待连接" || tab.client != nil || tab.term != nil || tab.cancel != nil {
			t.Fatal("restoration resumed a network session")
		}
	}
	if b.tabs[0].key == b.tabs[1].key {
		t.Fatal("same-profile sessions share identity")
	}
	_, saved := b.layout.Locate(two.key)
	if saved == nil || saved.FontSize != 16 {
		t.Fatal("font preference not restored")
	}
	b.removeTab(one.key)
	if len(b.tabs) != 1 || b.tabs[0].key != two.key {
		t.Fatal("closing one session removed its sibling")
	}
}

func TestBadLayoutDoesNotPreventConnectionLibraryOrOverwriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.json")
	bad := []byte("{broken")
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	a := newApp(dir)
	if a.store == nil || a.layout == nil || !a.layoutReadOnly || a.actionError == "" {
		t.Fatal("layout failure was not recoverable")
	}
	a.close()
	got, _ := os.ReadFile(path)
	if string(got) != string(bad) {
		t.Fatal("bad layout was overwritten on exit")
	}
}
