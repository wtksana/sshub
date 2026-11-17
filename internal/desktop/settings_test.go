package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

func TestDefaultFontAndPerTabZoom(t *testing.T) {
	dir := t.TempDir()
	a := newApp(dir)
	p, err := a.store.Upsert(library.Profile{Name: "server", Host: "example.com", Port: 22, Username: "test", Auth: "password"})
	if err != nil {
		t.Fatal(err)
	}
	prefs := library.Preferences{Theme: "dark", FontFamily: "Consolas", FontSize: 16}
	if err = a.store.SavePreferences(prefs); err != nil {
		t.Fatal(err)
	}
	a.applyPreferences(prefs, false)
	one := a.openTab(p)
	a.zoom(one, 2, false)
	two := a.openTab(p)
	prefs.FontSize = 15
	prefs.UIFontSize, prefs.UIFontFamily = 20, "Microsoft YaHei UI"
	if err = a.store.SavePreferences(prefs); err != nil {
		t.Fatal(err)
	}
	a.applyPreferences(prefs, false)
	_, first := a.layout.Locate(one.key)
	_, second := a.layout.Locate(two.key)
	if first.FontSize != 18 || second.FontSize != 16 {
		t.Fatal("default change reset per-tab zoom")
	}
	a.zoom(one, 0, true)
	if first.FontSize != 15 {
		t.Fatal("reset ignored configured default")
	}
	a.applyPreferences(prefs, true)
	if second.FontSize != 15 {
		t.Fatal("explicit reset not applied")
	}
	a.close()
	b := newApp(dir)
	defer b.close()
	if b.preferences != prefs || len(b.tabs) != 2 {
		t.Fatal("settings or tabs not restored")
	}
}

func TestScrollbackSettingsDraftCanBeCancelled(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	a.settings = &settingsEditor{preferences: a.preferences, size: "13", uiSize: "13"}
	view := ui.NewTester(a.settingsView, 800, 800)
	if !view.HasText("历史缓冲") || !view.HasText("仅新建或重新连接的终端生效；现有终端不变。按容量保留最近输出，非固定行数；实际内存占用可能更多。") {
		t.Fatal("scrollback setting or application scope missing", view.Texts())
	}
	if err := view.Click("64 MiB"); err != nil {
		t.Fatal(err)
	}
	if a.settings.preferences.ScrollbackMiB != 64 || a.preferences.EffectiveScrollbackMiB() != 2 {
		t.Fatal("editing changed active scrollback setting")
	}
	if err := view.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if a.settings != nil || a.preferences.EffectiveScrollbackMiB() != 2 || a.store.Preferences().EffectiveScrollbackMiB() != 2 {
		t.Fatal("cancel applied scrollback preference")
	}
}

func TestImportedPrivateKeysBlockedUntilReviewed(t *testing.T) {
	for _, viaJump := range []bool{false, true} {
		a := newApp(t.TempDir())
		p := library.Profile{ID: "key", Name: "key", Host: "127.0.0.1", Port: 1, Username: "test", Auth: "private_key", IdentityPath: `C:\keys\key`, IdentityNeedsReview: true}
		tab := &sessionTab{profile: p, status: "待连接"}
		if viaJump {
			tab.profile = library.Profile{Auth: "password"}
			tab.jump = &p
		}
		a.connect(tab)
		if tab.status != "连接失败" || tab.err == "" || tab.cancel != nil || tab.attempt != 0 {
			t.Fatal("unreviewed key attempted connection")
		}
		p.ID = ""
		saved, err := a.store.Upsert(p)
		if err != nil {
			t.Fatal(err)
		}
		a.refresh()
		a.beginEdit(saved)
		a.saveDraft()
		if !a.editing || a.draft.err == "" || !a.store.Profiles()[0].IdentityNeedsReview {
			t.Fatal("saved without confirmation")
		}
		a.draft.identityReviewed = true
		a.saveDraft()
		if a.editing || a.store.Profiles()[0].IdentityNeedsReview {
			t.Fatal("review not persisted")
		}
		if !saved.IdentityNeedsReview {
			t.Fatal("editing mutated old snapshot")
		}
		a.close()
	}
}

func TestImportReplacementPreservesSessionsAndCredentialIsolation(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	p, err := a.store.Upsert(library.Profile{Name: "server", Host: "example.com", Port: 22, Username: "test", Auth: "password"})
	if err != nil {
		t.Fatal(err)
	}
	tab := a.openTab(p)
	m := library.Migration{Format: "sshub", Version: 1, Profiles: []library.PortableProfile{{ID: p.ID, Name: p.Name, Host: p.Host, Port: p.Port, Username: p.Username, Auth: p.Auth}}}
	plan, err := a.store.PreviewImport(m)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.store.CommitImport(plan, []library.ImportDecision{{Mode: "replace"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	a.remapImportedLayout(result.ReplacedIDs)
	a.refresh()
	newProfile := a.profiles[0]
	if tab.profile.ID != p.ID || newProfile.ID == p.ID || a.secretKey(p, "password") == a.secretKey(newProfile, "password") {
		t.Fatal("replacement reused credential identity or modified session")
	}
	_, saved := a.layout.Locate(tab.key)
	if saved.ProfileID != newProfile.ID {
		t.Fatal("layout retains obsolete profile reference")
	}
}

func TestFilePanelDefaultsAndReleaseReset(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	if !a.hideRight {
		t.Fatal("new workspace should start without SFTP panel")
	}
	tab := &sessionTab{files: fileBrowser{path: "/old", input: "/old", err: "old failure", loading: true, generation: 3}}
	a.release(tab)
	if tab.files.path != "" || tab.files.err != "" || tab.files.loading || tab.files.generation <= 3 {
		t.Fatal("reconnection retained directory state")
	}
}

func TestSettingsRejectZeroUISize(t *testing.T) {
	a := newApp(t.TempDir())
	defer a.close()
	e := &settingsEditor{preferences: library.DefaultPreferences(), size: "13", uiSize: "0"}
	a.saveSettings(e)
	if e.err == "" || e.busy {
		t.Fatal("explicit zero UI size accepted as a legacy default")
	}
}
