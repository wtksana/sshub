package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

type linkFileInfo struct{ os.FileInfo }

func (linkFileInfo) Mode() os.FileMode { return os.ModeSymlink | 0777 }

func TestFileDialogsAllowLinkRenameButNotLinkPermissions(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, kind, label string
		info              os.FileInfo
		editable          bool
	}{
		{"link rename", "rename", "名称", linkFileInfo{info}, true},
		{"link permissions", "properties", "八进制权限", linkFileInfo{info}, false},
		{"file permissions", "properties", "八进制权限", info, true},
		{"new directory", "mkdir", "名称", nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := &app{}
			a.editFile(&sessionTab{}, test.kind, "/file", test.info)
			view := ui.NewTester(a.fileDialogs, 800, 600)
			if _, exists := view.Find(test.label); exists != test.editable {
				t.Fatal("unexpected input visibility", view.Texts())
			}
			if _, exists := view.Find("确认"); exists != test.editable {
				t.Fatal("unexpected confirm button visibility")
			}
			if view.HasText("符号链接不支持在此修改目标权限") == test.editable {
				t.Fatal("permissions warning shown for the wrong operation")
			}
		})
	}
}
