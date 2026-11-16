package desktop

import (
	"github.com/egoist/mygo/ui"
)

func (a *app) files(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Clip().Padding(14).Gap(10).Children(func() {
		ui.Text(c, "文件").Bold()
		status := "未连接服务器"
		if tab := a.activeWorkspace(); tab != nil {
			status = tab.profile.Name + " · " + tab.status
		}
		ui.Text(c, status).FontSize(12).TextColor(t.TextMuted).Wrap()
		ui.Divider(c)
		ui.Text(c, "文件浏览即将提供。").TextColor(t.TextMuted).Wrap()
		ui.Spacer(c)
		ui.Divider(c)
		ui.Text(c, "传输队列").Bold()
		ui.Text(c, "暂无传输任务").FontSize(12).TextColor(t.TextMuted)
	})
}
