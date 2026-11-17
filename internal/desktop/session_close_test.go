package desktop

import (
	"errors"
	"testing"

	"sshub/internal/library"
)

func TestSessionClosePreferenceAndTransfers(t *testing.T) {
	for _, tc := range []struct {
		name, status            string
		skip, transfer, confirm bool
	}{
		{"legacy connected", "已连接", false, false, true},
		{"legacy connecting", "连接中", false, false, true},
		{"skip connected", "已连接", true, false, false},
		{"skip connecting", "连接中", true, false, false},
		{"disconnected", "已断开", false, false, false},
		{"active transfer", "已连接", true, true, true},
		{"detached transfer", "已断开", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t.TempDir())
			defer a.close()
			p, err := a.store.Upsert(library.Profile{Name: "server", Host: "example.com", Port: 22, Username: "test", Auth: "password"})
			if err != nil {
				t.Fatal(err)
			}
			tab := a.openTab(p)
			tab.status = tc.status
			a.preferences.SkipCloseConfirmation = tc.skip
			if tc.transfer {
				a.transfers = []*transferTask{{tabID: tab.key, cancel: func() {}}}
			}
			a.closeTabRemember, a.closeTabError = true, "old error"
			a.requestClose(tab)
			if a.confirmCloseTab != tc.confirm || (a.tabByID(tab.key) != nil) != tc.confirm {
				t.Fatal("incorrect close/confirmation decision")
			}
			if tc.confirm && (a.closeTabRemember || a.closeTabError != "") {
				t.Fatal("new confirmation retained canceled choice")
			}
			if !tc.confirm && a.layout.Current().Active == tab.key {
				t.Fatal("closed tab retained in layout")
			}
		})
	}
}

func TestSessionCloseSaveFailureKeepsSession(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	p, err := a.store.Upsert(library.Profile{Name: "server", Host: "example.com", Port: 22, Username: "test", Auth: "password"})
	if err != nil {
		t.Fatal(err)
	}
	tab := a.openTab(p)
	tab.status = "已连接"
	a.requestClose(tab)
	a.closeTabSaving = true
	a.finishSessionClose(tab.key, errors.New("disk full"))
	if a.closeTabSaving || a.closeTabError == "" || !a.confirmCloseTab || a.preferences.SkipCloseConfirmation || a.tabByID(tab.key) == nil {
		t.Fatal("save failure closed session or changed preference")
	}
	// The user can still close this session without changing the saved preference.
	a.acceptSessionClose()
	if a.confirmCloseTab || a.tabByID(tab.key) != nil || a.preferences.SkipCloseConfirmation {
		t.Fatal("one-time close did not preserve confirmation preference")
	}
}
