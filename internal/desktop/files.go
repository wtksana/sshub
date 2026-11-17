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
	ordered          []os.FileInfo
	sort             fileSort
	loading          bool
	tableWidth       float32
	generation       int
	cancel           context.CancelFunc
}

// Only the visible active session initializes its directory browser. Explicit
// refresh remains available after an error; drawing must never cause retry loops.
func (a *app) ensureFiles(tab *sessionTab) {
	if !a.hideRight && tab != nil && tab.client != nil && tab.files.path == "" && !tab.files.loading && tab.files.err == "" {
		a.browse(tab, "")
	}
}

func (a *app) browse(tab *sessionTab, target string) {
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
		tab.files.loading = false
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
			tab.files.applySort()
		})
	}()
}

func (a *app) fileOperation(tab *sessionTab, action func(context.Context, *connection.Files) error) {
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
	ui.Column(c).Fill().Clip().Background(t.Surface).Padding(12).Gap(8).Children(func() {
		tab := a.activeTab()
		a.ensureFiles(tab)
		status := "未连接服务器"
		if tab != nil {
			status = tab.profile.Name + " · " + tab.status
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "文件").Bold().Shrink(0)
			ui.Text(c.Key("file-server"), status).FontSize(typeSize(c, -1)).TextColor(t.TextMuted).SingleLine().Tooltip(status).Grow(1).MinWidth(0)
			if tab != nil {
				if summary := tab.files.summary(); summary != "" {
					ui.Text(c.Key("files-summary"), summary).FontSize(typeSize(c, -2)).TextColor(t.TextMuted).Shrink(0)
				}
				iconButton(c.Key("files-menu"), iconMore, "文件操作").Menu(func(m *ui.Menu) { a.directoryMenu(m, tab) })
			}
		})
		if tab != nil && tab.client != nil {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				iconButton(c.Key("remote-parent"), iconParent, "上级目录").Disabled(tab.files.path == "").OnClick(func() { a.browse(tab, path.Dir(tab.files.path)) })
				input := textInput(c.Key(fmt.Sprintf("remote-path-%d", tab.key)), &tab.files.input).Placeholder("远程路径，回车跳转").Label("远程路径").Grow(1).MinWidth(0)
				if input.Submitted() {
					target := tab.files.input
					if !path.IsAbs(target) {
						target = path.Join(tab.files.path, target)
					}
					a.browse(tab, target)
				}
			})
			if tab.files.err != "" {
				ui.Text(c.Key("files-error"), tab.files.err).TextColor(t.Danger).FontSize(typeSize(c, -1)).Wrap()
			}
			modifiedWidth, sizeWidth := typeSize(c, -2)*9.5+8, typeSize(c, -2)*5+8
			table := ui.ScrollHorizontal(c.Key("remote-table-scroll")).Grow(1).FillWidth()
			// Horizontal scroll measures its children with unbounded width. Use
			// the last viewport to fill available space; settle once after resize.
			width := max(80+modifiedWidth+sizeWidth+32, table.Bounds().W)
			if width != tab.files.tableWidth {
				tab.files.tableWidth = width
				c.Invalidate()
			}
			table.Children(func() {
				ui.Column(c).FillHeight().Width(width).Shrink(0).Background(t.Background).Radius(6).Clip().Children(func() {
					a.fileHeaders(c, &tab.files, modifiedWidth, sizeWidth)
					entries := tab.files.displayEntries()
					zone := ui.List(c.Key("remote-files"), nil, len(entries), func(i int) {
						entry := entries[i]
						name := entry.Name()
						if !connection.ValidName(name) {
							return
						}
						label, icon := name, iconFile
						if entry.IsDir() {
							label += "/"
							icon = iconFolder
						}
						if entry.Mode()&os.ModeSymlink != 0 {
							icon = iconSymlink
						}
						target := path.Join(tab.files.path, name)
						row := ui.ButtonBase(c.Key(fmt.Sprintf("remote-entry-%d", i))).Label(label).FillWidth().MinHeight(max(34, t.FontSize+18)).Padding(6, 8).Gap(8).Radius(4).Justify(ui.Start).Tooltip(name + "\n" + entry.Mode().String() + " · " + bytesText(entry.Size()) + "\n" + fileModifiedText(entry)).ContextMenu(func(m *ui.Menu) { a.entryMenu(m, tab, target, entry) }).Children(func() {
							ui.Row(c).Grow(1).Basis(0).MinWidth(80).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icon).Size(16, 16).Shrink(0).TextColor(t.TextMuted)
								ui.Text(c, label).SingleLine().Grow(1).MinWidth(0)
							})
							ui.Text(c, fileModifiedText(entry)).Width(modifiedWidth).Shrink(0).SingleLine().FontSize(typeSize(c, -2)).TextColor(t.TextMuted).Tooltip(entry.ModTime().Local().Format("2006-01-02 15:04:05 MST"))
							size := "—"
							if !entry.IsDir() {
								size = bytesText(entry.Size())
							}
							ui.Text(c, size).Width(sizeWidth).Shrink(0).TextAlign(ui.End).SingleLine().FontSize(typeSize(c, -2)).TextColor(t.TextMuted)
						})
						if row.Hovered() {
							row.Background(t.SurfaceHover)
						}
						if row.DoubleClicked() && entry.IsDir() {
							a.browse(tab, target)
						}
					}).Grow(1).FillWidth().Background(t.Background).Radius(6).ContextMenu(func(m *ui.Menu) { a.directoryMenu(m, tab) }).Children(func() {
						if !tab.files.loading && tab.files.err == "" && len(entries) == 0 {
							ui.Column(c).FillWidth().Padding(20, 12).Gap(6).Children(func() {
								ui.Text(c, "空目录").TextColor(t.TextMuted)
								ui.Text(c, "拖入文件即可上传").FontSize(typeSize(c, -2)).TextColor(t.TextMuted)
							})
						}
					})
					if dropped := zone.DroppedFiles(); len(dropped) > 0 && tab.files.path != "" {
						a.upload(tab, tab.files.path, dropped)
					}
				})
			})
		} else {
			ui.Text(c, "连接后浏览远程文件").TextColor(t.TextMuted).FontSize(typeSize(c, -1))
			ui.Spacer(c)
		}
		ui.Divider(c)
		a.transferView(c)
	})
}

func (a *app) directoryMenu(m *ui.Menu, tab *sessionTab) {
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

func (a *app) entryMenu(m *ui.Menu, tab *sessionTab, target string, info os.FileInfo) {
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

func (a *app) chooseUpload(tab *sessionTab, directory bool) {
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

func (a *app) chooseDownload(tab *sessionTab, target string) {
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
