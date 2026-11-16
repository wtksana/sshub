package desktop

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

func (a *app) view(c *ui.Context) {
	t := *c.Theme()
	t.Spacing, t.Radius, t.FontSize = 4, 5, 13
	if runtime.GOOS == "windows" {
		t.Font = "Microsoft YaHei UI"
	}
	if !t.Dark {
		t.Background, t.Text, t.TextMuted, t.Border = ui.Hex("#f5f7fa"), ui.Hex("#263244"), ui.Hex("#65748a"), ui.Hex("#dce2ea")
	}
	c.SetTheme(t.WithAccent(ui.Hex("#2763b3")))
	c.DisableAnimations()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(10, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "SSHub").FontSize(18).Bold()
			ui.Text(c, "服务器工作区").TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Button(c.Key("manage-hosts"), "已信任主机").OnClick(a.showHosts)
			ui.Button(c.Key("toggle-connections"), "连接库").OnClick(func() { a.hideLeft = !a.hideLeft })
			ui.Button(c.Key("toggle-files"), "文件面板").OnClick(func() { a.hideRight = !a.hideRight })
		})
		ui.Divider(c)
		if a.loadError != "" {
			ui.Column(c).Padding(24).Gap(12).Children(func() {
				ui.Text(c, "无法读取连接库").FontSize(18).Bold()
				ui.Text(c, a.loadError).TextColor(t.Danger).Wrap()
				ui.Text(c, "现有文件未被覆盖。请检查配置文件或关闭占用该目录的实例，然后重试。").Wrap()
				ui.Text(c, a.dir).TextColor(t.TextMuted).Wrap()
				ui.Button(c.Key("retry-load"), "重试读取").OnClick(a.load)
			})
			return
		}
		if a.actionError != "" {
			ui.Row(c).Padding(8, 14).Gap(10).Children(func() {
				ui.Text(c, a.actionError).TextColor(t.Danger).Wrap().Grow(1)
				ui.Button(c.Key("dismiss-error"), "关闭提示").OnClick(func() { a.actionError = "" })
			})
		}
		if a.notice != "" {
			ui.Row(c).Padding(8, 14).Gap(10).Children(func() {
				ui.Text(c, a.notice).Wrap().Grow(1)
				ui.Button(c.Key("dismiss-notice"), "关闭提示").OnClick(func() { a.notice = "" })
			})
		}
		main := func() {
			if a.hideRight {
				a.center(c)
			} else {
				ui.SplitPercent(c.Key("files-divider"), &a.contentPercent, func() { a.center(c) }, func() { a.files(c) }).SplitReset(66).Fill()
			}
		}
		if a.hideLeft {
			ui.Column(c).Grow(1).FillWidth().Children(main)
		} else {
			ui.Split(c.Key("library-divider"), &a.leftSize, func() { a.sidebar(c) }, main).SplitReset(260).Grow(1).FillWidth()
		}
	})
	a.editorView(c)
	a.sessionDialogs(c)
	a.promptView(c)
	if ui.AlertDialog(c.Key("delete-confirm"), &a.deleting, "删除连接配置？", "将删除“"+a.deleteName+"”的已保存配置，已打开的工作区不受影响。", "取消", "删除配置") == 1 {
		if err := a.store.Delete(a.deleteID); err != nil {
			a.actionError = err.Error()
		} else {
			a.refresh()
			if a.selected == a.deleteID {
				a.selected = ""
			}
		}
	}
}
