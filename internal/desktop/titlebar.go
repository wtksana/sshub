package desktop

import "github.com/egoist/mygo/ui"

func (a *app) titlebar(c *ui.Context) {
	ui.Row(c).Height(34).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "SSHub").FontSize(13).Bold().Padding(0, 10).DragWindow()
		ui.Button(c.Key("app-menu"), "☰").Label("应用菜单").Tooltip("设置、配置迁移与视图").Padding(3, 7).Background(ui.Transparent).BorderWidth(0).Menu(a.appMenu)
		ui.Box(c.Key("titlebar-drag")).Label("拖动窗口").Height(34).Grow(1).DragWindow()
		if a.win == nil {
			return
		}
		a.windowButton(c, "window-minimize", "—", "最小化", a.win.Minimize)
		icon, label := "□", "最大化"
		if a.win.IsMaximized() {
			icon, label = "❐", "还原窗口"
		}
		a.windowButton(c, "window-maximize", icon, label, func() {
			if a.win.IsMaximized() {
				a.win.Unmaximize()
			} else {
				a.win.Maximize()
			}
		})
		a.windowButton(c, "window-close", "×", "关闭窗口", a.win.Close)
	})
}

func (a *app) windowButton(c *ui.Context, key, icon, label string, action func()) {
	b := ui.Button(c.Key(key), icon).Label(label).Tooltip(label).Size(46, 34).Padding(0).Radius(0).BorderWidth(0).Background(ui.Transparent)
	if key == "window-maximize" {
		b.MaximizeButton()
	}
	if b.Hovered() {
		if key == "window-close" {
			b.Background(ui.Hex("#c42b1c")).TextColor(ui.Hex("#ffffff"))
		} else {
			b.Background(c.Theme().SurfaceHover)
		}
	}
	b.OnClick(action)
}
