package desktop

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"sshub/internal/connection"
)

type fileBrowser struct {
	path, input, err string
	entries          []os.FileInfo
	loading          bool
	generation       int
	cancel           context.CancelFunc
}

func (a *app) browse(tab *workspace, target string) {
	if tab.client == nil || tab.transport == nil {
		return
	}
	if tab.files.cancel != nil {
		tab.files.cancel()
	}
	ctx, cancel := context.WithTimeout(a.workCtx, 20*time.Second)
	tab.files.cancel, tab.files.loading, tab.files.err = cancel, true, ""
	tab.files.generation++
	generation, attempt := tab.files.generation, tab.attempt
	transport := tab.transport
	if !transport.retain() {
		cancel()
		return
	}
	go func() {
		defer transport.release()
		defer cancel()
		files, err := transport.client.OpenFiles(ctx)
		var dir string
		var entries []os.FileInfo
		if err == nil {
			dir, entries, err = files.Browse(ctx, target)
			files.Close()
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		a.dispatch(func() {
			if tab.attempt != attempt || tab.files.generation != generation {
				return
			}
			tab.files.loading = false
			if err != nil {
				tab.files.err = fmt.Sprintf("无法打开 %s：%v", target, err)
				tab.files.input = tab.files.path
				return
			}
			tab.files.path, tab.files.input, tab.files.entries = dir, dir, entries
		})
	}()
}

func (a *app) fileOperation(tab *workspace, action func(context.Context, *connection.Files) error) {
	if tab.transport == nil {
		a.actionError = "请先连接该服务器"
		return
	}
	transport, attempt := tab.transport, tab.attempt
	if !transport.retain() {
		return
	}
	ctx, cancel := context.WithTimeout(a.workCtx, 30*time.Second)
	go func() {
		defer cancel()
		defer transport.release()
		files, err := transport.client.OpenFiles(ctx)
		if err == nil {
			err = action(ctx, files)
			files.Close()
		}
		a.dispatch(func() {
			if err != nil {
				a.actionError = "文件操作失败：" + err.Error()
				return
			}
			if tab.attempt == attempt && tab.client != nil {
				a.browse(tab, tab.files.path)
			}
		})
	}()
}

func (a *app) files(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Clip().Padding(10).Gap(7).Children(func() {
		tab := a.activeWorkspace()
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			ui.Text(c, "文件").Bold().Grow(1)
			if tab != nil {
				ui.MenuButton(c.Key("files-menu"), "…", func(m *ui.Menu) { a.directoryMenu(m, tab) }).Tooltip("文件操作")
			}
		})
		status := "未连接服务器"
		if tab != nil {
			status = tab.profile.Name + " · " + tab.status
		}
		ui.Text(c.Key("file-server"), status).FontSize(12).TextColor(t.TextMuted).Wrap()
		if tab != nil && tab.client != nil {
			input := ui.TextInput(c.Key(fmt.Sprintf("remote-path-%d", tab.key)), &tab.files.input).Placeholder("远程路径，回车跳转").FillWidth()
			if input.Submitted() {
				target := tab.files.input
				if !path.IsAbs(target) {
					target = path.Join(tab.files.path, target)
				}
				a.browse(tab, target)
			}
			if tab.files.loading {
				ui.Text(c, "正在读取目录…").FontSize(12).TextColor(t.TextMuted)
			}
			if tab.files.err != "" {
				ui.Text(c.Key("files-error"), tab.files.err).TextColor(t.Danger).FontSize(12).Wrap()
			}
			zone := ui.Scroll(c.Key("remote-files")).Grow(1).FillWidth().Gap(2).ContextMenu(func(m *ui.Menu) { a.directoryMenu(m, tab) }).Children(func() {
				if tab.files.path != "" {
					ui.Button(c.Key("remote-parent"), "../").FillWidth().OnClick(func() { a.browse(tab, path.Dir(tab.files.path)) })
				}
				for i, entry := range tab.files.entries {
					name := entry.Name()
					if !connection.ValidName(name) {
						continue
					}
					label := name
					if entry.IsDir() {
						label += "/"
					}
					if entry.Mode()&os.ModeSymlink != 0 {
						label += " ↗"
					}
					target := path.Join(tab.files.path, name)
					row := ui.Button(c.Key(fmt.Sprintf("remote-entry-%d", i)), label).FillWidth().Tooltip(entry.Mode().String() + " · " + bytesText(entry.Size())).ContextMenu(func(m *ui.Menu) { a.entryMenu(m, tab, target, entry) })
					if row.DoubleClicked() && entry.IsDir() {
						a.browse(tab, target)
					}
				}
				if !tab.files.loading && tab.files.err == "" && len(tab.files.entries) == 0 {
					ui.Text(c, "空目录").TextColor(t.TextMuted)
				}
			})
			if dropped := zone.DroppedFiles(); len(dropped) > 0 && tab.files.path != "" {
				a.upload(tab, tab.files.path, dropped)
			}
		} else {
			ui.Text(c, "连接后浏览远程文件").TextColor(t.TextMuted).FontSize(12)
			ui.Spacer(c)
		}
		ui.Divider(c)
		a.transferView(c)
	})
}

func (a *app) directoryMenu(m *ui.Menu, tab *workspace) {
	enabled := tab.client != nil && tab.files.path != ""
	if m.Item("刷新").ID("files-refresh").Disabled(tab.client == nil).Chosen() {
		a.browse(tab, tab.files.path)
	}
	if m.Item("主目录").ID("files-home").Disabled(tab.client == nil).Chosen() {
		a.browse(tab, "")
	}
	m.Separator()
	if m.Item("上传文件…").ID("files-upload").Disabled(!enabled).Chosen() {
		a.chooseUpload(tab, false)
	}
	if m.Item("上传文件夹…").ID("files-upload-folder").Disabled(!enabled).Chosen() {
		a.chooseUpload(tab, true)
	}
	if m.Item("新建文件夹…").ID("files-mkdir").Disabled(!enabled).Chosen() {
		a.editFile(tab, "mkdir", tab.files.path, nil)
	}
}

func (a *app) entryMenu(m *ui.Menu, tab *workspace, target string, info os.FileInfo) {
	if m.Item("下载到…").ID("file-download").Disabled(info.Mode()&os.ModeSymlink != 0).Chosen() {
		a.chooseDownload(tab, target)
	}
	if m.Item("重命名…").ID("file-rename").Chosen() {
		a.editFile(tab, "rename", target, info)
	}
	if m.Item("属性与权限…").ID("file-properties").Chosen() {
		a.editFile(tab, "properties", target, info)
	}
	m.Separator()
	if m.Item("删除…").ID("file-delete").Danger().Chosen() {
		a.editFile(tab, "delete", target, info)
	}
}

func (a *app) chooseUpload(tab *workspace, directory bool) {
	dir, attempt := tab.files.path, tab.attempt
	go func() {
		files, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "选择上传内容", Multiple: true, Directory: directory})
		a.dispatch(func() {
			if err != nil {
				a.actionError = err.Error()
				return
			}
			if len(files) == 0 {
				return
			}
			if tab.attempt != attempt || tab.client == nil {
				a.actionError = "原会话已断开，请重新选择上传"
				return
			}
			a.upload(tab, dir, files)
		})
	}()
}

func (a *app) chooseDownload(tab *workspace, target string) {
	attempt := tab.attempt
	go func() {
		dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "选择下载目录", Directory: true, CreateDirectories: true})
		a.dispatch(func() {
			if err != nil {
				a.actionError = err.Error()
				return
			}
			if len(dirs) == 0 {
				return
			}
			if tab.attempt != attempt || tab.client == nil {
				a.actionError = "原会话已断开，请重新选择下载"
				return
			}
			a.download(tab, target, dirs[0])
		})
	}()
}
