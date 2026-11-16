package desktop

import (
	"github.com/egoist/mygo/ui"
)

func (a *app) files(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Clip().Padding(14).Gap(10).Children(func() {
		ui.Text(c, "文件").Bold()
		ui.Text(c, "未连接服务器").FontSize(12).TextColor(t.TextMuted)
		ui.Divider(c)
		ui.Text(c, "连接服务器后浏览远程文件。").TextColor(t.TextMuted).Wrap()
		ui.Spacer(c)
		ui.Divider(c)
		ui.Text(c, "传输队列").Bold()
		ui.Text(c, "暂无传输任务").FontSize(12).TextColor(t.TextMuted)
	})
}
