package desktop

import (
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

type migrationEditor struct {
	export, busy, applyPreferences bool
	path, err, confirmPath         string
	plan                           *library.ImportPlan
	choices, names                 []string
}

func (a *app) remapImportedLayout(replaced map[string]string) {
	// Only saved references change. Live sessions and transfer retries keep
	// their original immutable target and credential identity.
	for _, pane := range a.layout.Panes() {
		for i := range pane.Tabs {
			if id := replaced[pane.Tabs[i].ProfileID]; id != "" {
				pane.Tabs[i].ProfileID = id
			}
		}
	}
	if id := replaced[a.selected]; id != "" {
		a.selected = id
	}
}

func (a *app) pickMigration(e *migrationEditor) {
	e.busy = true
	go func() {
		var path string
		var err error
		filters := []mygo.FileFilter{{Name: "SSHub 配置", Extensions: []string{"json"}}}
		if e.export {
			path, err = mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: "导出 SSHub 配置", DefaultPath: "sshub-config.json", Filters: filters})
		} else {
			var paths []string
			paths, err = mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "导入 SSHub 配置", Filters: filters})
			if len(paths) > 0 {
				path = paths[0]
			}
		}
		a.dispatch(func() {
			e.busy = false
			if err != nil {
				e.err = "无法选择文件：" + err.Error()
				return
			}
			if path != "" {
				e.path, e.confirmPath, e.err = path, "", ""
				e.plan = nil
			}
		})
	}()
}

func (a *app) previewMigration(e *migrationEditor) {
	path := strings.TrimSpace(e.path)
	if path == "" {
		e.err = "请选择迁移文件"
		return
	}
	e.busy, e.err, e.plan = true, "", nil
	go func() {
		var plan *library.ImportPlan
		f, err := os.Open(path)
		if err == nil {
			var m *library.Migration
			m, err = library.DecodeMigration(f)
			f.Close()
			if err == nil {
				plan, err = a.store.PreviewImport(*m)
			}
		}
		a.dispatch(func() {
			e.busy = false
			if err != nil {
				e.err = err.Error()
				return
			}
			e.plan = plan
			e.choices, e.names = make([]string, len(plan.Rows)), make([]string, len(plan.Rows))
			for i, row := range plan.Rows {
				e.names[i] = row.Profile.Name + "（导入）"
				if row.Conflict {
					e.choices[i] = "请选择"
				} else {
					e.choices[i] = "新增"
				}
			}
		})
	}()
}

func (a *app) commitMigration(e *migrationEditor) {
	decisions := make([]library.ImportDecision, len(e.choices))
	for i, choice := range e.choices {
		mode := map[string]string{"新增": "add", "跳过": "skip", "另存": "rename", "替换": "replace"}[choice]
		if mode == "" {
			e.err = "请处理全部重名冲突"
			return
		}
		decisions[i] = library.ImportDecision{Mode: mode, Name: e.names[i]}
	}
	plan, apply := e.plan, e.applyPreferences
	e.busy, e.err = true, ""
	go func() {
		result, err := a.store.CommitImport(plan, decisions, apply)
		a.dispatch(func() {
			e.busy = false
			if err != nil {
				e.err = err.Error()
				return
			}
			a.remapImportedLayout(result.ReplacedIDs)
			a.refresh()
			if apply {
				a.applyPreferences(a.store.Preferences(), false)
			}
			a.migration = nil
			a.notice = fmt.Sprintf("已导入 %d 个连接；凭据需重新输入，私钥路径需核对。现有标签保留原配置，使用新配置请打开新工作区。", result.Count)
		})
	}()
}

func (a *app) exportMigration(e *migrationEditor, overwrite bool) {
	path := strings.TrimSpace(e.path)
	if overwrite {
		path = e.confirmPath
	}
	if path == "" {
		e.err = "请选择导出位置"
		return
	}
	e.busy, e.err = true, ""
	go func() {
		var err error
		needsConfirmation := false
		if !overwrite {
			_, statErr := os.Stat(path)
			needsConfirmation = statErr == nil
			if statErr != nil && !os.IsNotExist(statErr) {
				err = statErr
			}
		}
		if err == nil && !needsConfirmation {
			err = a.store.ExportTo(path, overwrite)
		}
		a.dispatch(func() {
			e.busy = false
			if err != nil {
				e.err = err.Error()
				return
			}
			if needsConfirmation {
				e.confirmPath = path
				return
			}
			a.migration = nil
			a.notice = "配置已导出到：" + path
		})
	}()
}

func (a *app) migrationView(c *ui.Context) {
	e := a.migration
	if e == nil {
		return
	}
	open := true
	ui.DialogBase(c.Key("migration-dialog"), &open, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(ui.RGBA(0, 0, 0, .35))
		dialogStyle(c, panel, 670)
		title := "导入配置"
		if e.export {
			title = "导出配置"
		}
		ui.Text(c, title).FontSize(typeSize(c, 5)).Bold()
		ui.Text(c, "包含连接、分组、收藏与偏好；不包含密码、口令、私钥内容、凭据引用、主机信任或工作区布局。").FontSize(typeSize(c, -2)).TextColor(t.TextMuted).Wrap()
		ui.Row(c).Gap(8).FillWidth().Children(func() {
			before := e.path
			textInput(c.Key("migration-path"), &e.path).Label("配置文件路径").Placeholder("JSON 文件路径").Grow(1).Disabled(e.busy || e.plan != nil || e.confirmPath != "")
			if e.path != before {
				e.confirmPath = ""
			}
			ui.Button(c.Key("migration-browse"), "浏览…").Disabled(e.busy || e.plan != nil || e.confirmPath != "").OnClick(func() { a.pickMigration(e) })
		})
		if e.plan != nil {
			ui.Text(c, fmt.Sprintf("预览：%d 个连接，尚未写入", len(e.plan.Rows))).Bold()
			ui.Scroll(c.Key("migration-preview")).MaxHeight(290).FillWidth().Gap(8).Disabled(e.busy).Children(func() {
				for i, row := range e.plan.Rows {
					key := func(s string) *ui.Context { return c.Key(fmt.Sprintf("import-%s-%d", s, i)) }
					ui.Column(key("row")).FillWidth().Gap(4).Padding(8).Border(1, t.Border).Children(func() {
						p := row.Profile
						group := p.Group
						if group == "" {
							group = "未分组"
						}
						ui.Text(c, p.Name+" · "+group).Bold().Wrap()
						ui.Text(c, fmt.Sprintf("%s@%s:%d · %s", p.Username, p.Host, p.Port, authLabel(p.Auth))).FontSize(typeSize(c, -2)).Wrap()
						if p.IdentityNeedsReview {
							ui.Text(c, "私钥路径待核对，确认前不能连接").FontSize(typeSize(c, -2)).Wrap()
						}
						options := []string{"新增", "跳过"}
						if row.Conflict {
							options = []string{"请选择", "跳过", "另存", "替换"}
							ui.Text(c, "同组重名：替换仅更新配置，现有会话保留原配置").FontSize(typeSize(c, -2)).Wrap()
						}
						ui.Row(c).Gap(8).FillWidth().Children(func() {
							ui.Select(key("choice"), &e.choices[i], options).Width(135)
							if e.choices[i] == "另存" {
								textInput(key("name"), &e.names[i]).Label("另存名称").Grow(1)
							}
						})
					})
				}
			})
			if e.plan.Preferences != nil {
				ui.Checkbox(c.Key("import-preferences"), &e.applyPreferences, "同时导入外观、界面和终端默认设置").Disabled(e.busy)
			}
		}
		if e.confirmPath != "" {
			ui.Text(c, "目标文件已存在，确认覆盖？\n"+e.confirmPath).Wrap().TextColor(t.Danger)
		}
		if e.err != "" {
			ui.Text(c, e.err).TextColor(t.Danger).Wrap()
		}
		if e.busy {
			ui.Text(c, "正在处理…").FontSize(typeSize(c, -2))
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("migration-cancel"), "取消").Disabled(e.busy).OnClick(func() { a.migration = nil })
			if e.export {
				if e.confirmPath != "" {
					ui.Button(c.Key("export-reselect"), "更换位置").Disabled(e.busy).OnClick(func() { e.confirmPath = "" })
					ui.PrimaryButton(c.Key("export-overwrite"), "确认覆盖").Disabled(e.busy).OnClick(func() { a.exportMigration(e, true) })
				} else {
					ui.PrimaryButton(c.Key("export-save"), "导出").Disabled(e.busy).OnClick(func() { a.exportMigration(e, false) })
				}
			} else if e.plan == nil {
				ui.PrimaryButton(c.Key("import-preview"), "预览").Disabled(e.busy).OnClick(func() { a.previewMigration(e) })
			} else {
				ui.Button(c.Key("import-back"), "重新选择").Disabled(e.busy).OnClick(func() { e.plan = nil; e.err = "" })
				ui.PrimaryButton(c.Key("import-confirm"), "确认导入").Disabled(e.busy).OnClick(func() { a.commitMigration(e) })
			}
		})
	})
	if !open && !e.busy {
		a.migration = nil
	}
}
