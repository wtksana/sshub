package desktop

import "github.com/egoist/mygo/ui"

func (a *app) showHosts() {
	if a.trust == nil {
		return
	}
	hosts, err := a.trust.Hosts()
	if err != nil {
		a.actionError = err.Error()
		return
	}
	a.hosts, a.manageHosts = hosts, true
}

func (a *app) sessionDialogs(c *ui.Context) {
	if a.hasTabTransfers(a.closeTabID) {
		choice := ui.AlertDialog(c.Key("close-session-confirm"), &a.confirmCloseTab, "关闭终端会话？", "该会话有未完成传输。可以只关闭终端并继续传输，或取消传输后关闭。", "返回", "关闭终端并继续传输", "取消传输并关闭")
		if choice > 0 {
			if choice == 2 {
				a.cancelTabTransfers(a.closeTabID)
			}
			a.removeTab(a.closeTabID)
		}
	} else {
		a.closeSessionDialog(c)
	}
	quitMessage, quitLabel := "退出将断开所有终端会话。", "断开并退出"
	if a.hasTransfers() {
		quitMessage, quitLabel = "还有未完成传输。退出将取消全部传输并断开终端；未完成文件不会发布为目标文件。", "取消传输并退出"
	}
	if ui.AlertDialog(c.Key("quit-confirm"), &a.confirmQuit, "退出 SSHub？", quitMessage, "返回", quitLabel) == 1 {
		a.allowQuit = true
		a.win.Close()
	}
	ui.DialogBase(c.Key("trusted-hosts"), &a.manageHosts, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		dialogStyle(c, panel, 650)
		ui.Text(c, "已信任主机").FontSize(typeSize(c, 5)).Bold()
		ui.Scroll(c).MaxHeight(400).Gap(12).Children(func() {
			if len(a.hosts) == 0 {
				ui.Text(c, "暂无主机信任记录")
			}
			for i, host := range a.hosts {
				ui.Column(c.Key(i)).Gap(5).Children(func() {
					ui.Text(c, host.Address+" · "+host.Algorithm).Bold().Wrap()
					ui.Text(c, host.Fingerprint).FontSize(typeSize(c, -1)).Wrap()
					ui.Button(c.Key("forget-host-"+host.Address), "移除信任").OnClick(func() {
						a.forgetHost, a.confirmForgetHost, a.manageHosts = host.Address, true, false
					})
				})
			}
		})
		ui.Button(c.Key("close-hosts"), "关闭").OnClick(func() { a.manageHosts = false })
	})
	if ui.AlertDialog(c.Key("forget-host-confirm"), &a.confirmForgetHost, "移除主机信任？", "将移除 "+a.forgetHost+" 的信任记录。下次连接需重新核对指纹，当前会话不受影响。", "取消", "移除信任记录") == 1 {
		if err := a.trust.Forget(a.forgetHost); err != nil {
			a.actionError = err.Error()
		} else {
			a.notice = "已移除主机信任记录"
		}
	}
}

func (a *app) closeSessionDialog(c *ui.Context) {
	ui.DialogBase(c.Key("close-session-confirm"), &a.confirmCloseTab, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .35))
		dialogStyle(c, panel, 400)
		ui.Text(c, "关闭终端会话？").Bold().FontSize(typeSize(c, 2))
		ui.Text(c, "该标签仍在连接或已连接，关闭后将断开并清除终端输出。").TextColor(c.Theme().TextMuted).Wrap()
		ui.Checkbox(c.Key("close-session-remember"), &a.closeTabRemember, "以后不再提示").Disabled(a.closeTabSaving)
		if a.closeTabError != "" {
			ui.Text(c, a.closeTabError).TextColor(c.Theme().Danger).Wrap()
		}
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Disabled(a.closeTabSaving).Children(func() {
			ui.Button(c.Key("close-session-cancel"), "取消").OnClick(func() { a.confirmCloseTab = false })
			ui.PrimaryButton(c.Key("close-session-accept"), "关闭会话").OnClick(a.acceptSessionClose)
		})
	})
	if a.closeTabSaving {
		a.confirmCloseTab = true
	}
}

func (a *app) acceptSessionClose() {
	if a.closeTabSaving {
		return
	}
	if !a.closeTabRemember {
		a.confirmCloseTab = false
		a.removeTab(a.closeTabID)
		return
	}
	key, p := a.closeTabID, a.preferences
	p.SkipCloseConfirmation = true
	a.closeTabSaving, a.closeTabError = true, ""
	go func() {
		err := a.store.SavePreferences(p)
		a.dispatch(func() { a.finishSessionClose(key, err) })
	}()
}

func (a *app) finishSessionClose(key int, err error) {
	a.closeTabSaving = false
	if err != nil {
		a.closeTabError = "无法保存提示偏好：" + err.Error()
		return
	}
	a.preferences.SkipCloseConfirmation = true
	a.confirmCloseTab = false
	a.removeTab(key)
}
