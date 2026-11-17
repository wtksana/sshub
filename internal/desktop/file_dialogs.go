package desktop

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"sshub/internal/connection"
	"sshub/internal/transfer"
)

type fileEditor struct {
	tab                      *sessionTab
	attempt                  int
	kind, target, value, err string
	info                     os.FileInfo
}

func (a *app) editFile(tab *sessionTab, kind, target string, info os.FileInfo) {
	e := &fileEditor{tab: tab, attempt: tab.attempt, kind: kind, target: target, info: info}
	if kind == "rename" {
		e.value = path.Base(target)
	}
	if kind == "properties" {
		e.value = fmt.Sprintf("%03o", info.Mode().Perm())
	}
	a.fileEdit = e
}

func (a *app) applyFileEdit(e *fileEditor) {
	if e.tab.attempt != e.attempt || e.tab.client == nil {
		e.err = "原会话已断开，请重新操作"
		return
	}
	if (e.kind == "mkdir" || e.kind == "rename") && !connection.ValidName(e.value) {
		e.err = "请输入名称，不能包含路径分隔符"
		return
	}
	var mode uint64
	if e.kind == "properties" {
		var err error
		mode, err = strconv.ParseUint(e.value, 8, 12)
		if err != nil || mode > 0777 {
			e.err = "请输入 000–777 的八进制权限"
			return
		}
	}
	a.fileEdit = nil
	a.fileOperation(e.tab, func(ctx context.Context, f *connection.Files) error {
		switch e.kind {
		case "mkdir":
			return f.Mkdir(path.Join(e.target, e.value))
		case "rename":
			return f.Rename(e.target, path.Join(path.Dir(e.target), e.value))
		case "delete":
			return f.RemoveTree(ctx, e.target)
		case "properties":
			return f.Chmod(e.target, os.FileMode(mode))
		}
		return nil
	})
}

func (a *app) fileDialogs(c *ui.Context) {
	if e := a.fileEdit; e != nil {
		open := true
		ui.DialogBase(c.Key(fmt.Sprintf("file-editor-%p", e)), &open, func(backdrop, panel ui.Element) {
			t := c.Theme()
			backdrop.Background(ui.RGBA(0, 0, 0, .35))
			dialogStyle(c, panel, 510)
			title := map[string]string{"mkdir": "新建文件夹", "rename": "重命名", "delete": "删除远程文件？", "properties": "属性与权限"}[e.kind]
			ui.Text(c, title).FontSize(typeSize(c, 5)).Bold()
			ui.Text(c, e.tab.profile.Name+" · "+e.target).Wrap()
			if e.kind == "delete" {
				ui.Text(c, "此操作无法撤销。目录将递归删除，符号链接仅删除链接本身。").Wrap()
			} else {
				if e.kind == "properties" {
					ui.Text(c, fmt.Sprintf("%s · %s\n修改时间：%s", e.info.Mode(), bytesText(e.info.Size()), e.info.ModTime().Format(time.DateTime))).Wrap()
				}
				if e.kind != "properties" || e.info.Mode()&os.ModeSymlink == 0 {
					textInput(c.Key("file-edit-value"), &e.value).Label(map[bool]string{true: "八进制权限", false: "名称"}[e.kind == "properties"]).AutoFocus().FillWidth()
				} else {
					ui.Text(c, "符号链接不支持在此修改目标权限").Wrap()
				}
			}
			if e.err != "" {
				ui.Text(c, e.err).TextColor(t.Danger).Wrap()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ui.Button(c.Key("file-edit-cancel"), "取消").OnClick(func() { a.fileEdit = nil })
				if e.kind != "properties" || e.info.Mode()&os.ModeSymlink == 0 {
					ui.PrimaryButton(c.Key("file-edit-confirm"), "确认").OnClick(func() { a.applyFileEdit(e) })
				}
			})
		})
		if !open {
			a.fileEdit = nil
		}
	}
	if p := a.transferConflict; p != nil {
		open := true
		ui.DialogBase(c.Key(fmt.Sprintf("file-conflict-%p", p)), &open, func(backdrop, panel ui.Element) {
			backdrop.Background(ui.RGBA(0, 0, 0, .35))
			dialogStyle(c, panel, 540)
			ui.Text(c, "目标文件已存在").Bold().FontSize(typeSize(c, 5))
			ui.Text(c, p.server+" · "+p.path).Wrap()
			textInput(c.Key("conflict-name"), &p.name).Placeholder("另存名称（仅文件名）").FillWidth()
			ui.Checkbox(c.Key("conflict-apply-all"), &p.applyAll, "覆盖或跳过应用到本批次")
			answer := func(choice transfer.Choice) {
				p.reply <- transfer.Decision{Choice: choice, Name: p.name, ApplyAll: p.applyAll}
				a.transferConflict = nil
			}
			ui.Row(c).Gap(6).Justify(ui.End).Children(func() {
				ui.Button(c.Key("conflict-cancel"), "取消任务").OnClick(func() { a.cancelConflict(p) })
				ui.Button(c.Key("conflict-skip"), "跳过").OnClick(func() { answer(transfer.Skip) })
				ui.Button(c.Key("conflict-save-as"), "另存").Disabled(!connection.ValidName(p.name) || !filepath.IsLocal(p.name) || strings.Contains(p.name, ":")).OnClick(func() { answer(transfer.SaveAs) })
				ui.PrimaryButton(c.Key("conflict-overwrite"), "覆盖").OnClick(func() { answer(transfer.Overwrite) })
			})
		})
		if !open {
			a.cancelConflict(p)
		}
	}
}

func (a *app) cancelConflict(p *fileConflict) {
	for _, task := range a.transfers {
		if task.ctx == p.ctx && task.cancel != nil {
			a.cancelTransfer(task)
		}
	}
	a.transferConflict = nil
}

func (a *app) download(tab *sessionTab, target, localDir string) {
	name := path.Base(target)
	if !connection.ValidName(name) || !filepath.IsLocal(name) || strings.Contains(name, ":") {
		a.actionError = "无法安全地保存该文件名"
		return
	}
	a.enqueue(tab, transfer.Request{Source: target, Destination: filepath.Join(localDir, name)}, &transferBatch{})
}
