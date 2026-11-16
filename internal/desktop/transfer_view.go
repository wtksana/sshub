package desktop

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) transferView(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Children(func() {
		ui.Text(c, "传输队列").Bold().Grow(1)
		ui.MenuButton(c.Key("transfers-menu"), "…", func(m *ui.Menu) {
			if m.Item("清除已结束任务").ID("transfers-clear").Chosen() {
				a.transfers = slices.DeleteFunc(a.transfers, func(task *transferTask) bool { return task.cancel == nil })
			}
			if m.Item("取消全部传输").ID("transfers-cancel-all").Disabled(!a.hasTransfers()).Chosen() {
				for _, task := range a.transfers {
					if task.cancel != nil {
						a.cancelTransfer(task)
					}
				}
			}
		})
	})
	if len(a.transfers) == 0 {
		ui.Text(c, "暂无传输任务").FontSize(12).TextColor(t.TextMuted)
		return
	}
	ui.Scroll(c.Key("transfer-list")).MaxHeight(220).FillWidth().Gap(8).Children(func() {
		for _, task := range a.transfers {
			label := "↓ "
			if task.request.Upload {
				label = "↑ " + filepath.Base(task.request.Source)
			} else {
				label += path.Base(task.request.Source)
			}
			ui.Column(c.Key(fmt.Sprintf("transfer-%d", task.id))).Label("传输任务 " + label).Gap(3).ContextMenu(func(m *ui.Menu) {
				if m.Item("取消传输").ID("transfer-cancel").Disabled(task.cancel == nil).Chosen() {
					a.cancelTransfer(task)
				}
				if m.Item("从头重试（必要时重新连接）").ID("transfer-retry").Disabled(task.cancel != nil || task.status == "已完成").Chosen() {
					a.retryTransfer(task)
				}
			}).Children(func() {
				ui.Text(c, label).FontSize(12).Bold().Wrap()
				ui.Text(c, task.server+" · "+task.status).FontSize(12).Wrap()
				ui.Text(c, task.request.Destination).FontSize(11).TextColor(t.TextMuted).Wrap()
				if task.progress.Files == 1 && task.progress.Destination != "" && task.progress.Destination != task.request.Destination {
					ui.Text(c, "已保存："+task.progress.Destination).FontSize(11).TextColor(t.TextMuted).Wrap()
				}
				if task.progress.Bytes > 0 || task.status == "已完成" {
					summary := fmt.Sprintf("%s · %d 文件 · %d 跳过", bytesText(task.progress.Bytes), task.progress.Files, task.progress.Skipped)
					if task.cancel != nil && !task.started.IsZero() {
						summary += " · " + bytesText(int64(float64(task.progress.Bytes)/max(.1, time.Since(task.started).Seconds()))) + "/s"
					}
					ui.Text(c, summary).FontSize(11).TextColor(t.TextMuted).Wrap()
					if task.cancel != nil && task.progress.CurrentTotal > 0 {
						ui.Text(c, fmt.Sprintf("%s · %.0f%%", path.Base(task.progress.Path), float64(task.progress.CurrentBytes)*100/float64(task.progress.CurrentTotal))).FontSize(11).TextColor(t.TextMuted).Wrap()
					}
				}
				if task.err != "" {
					ui.Text(c, task.err).FontSize(11).TextColor(t.Danger).Wrap()
				}
			})
		}
	})
	if a.hasTransfers() {
		c.After(250 * time.Millisecond)
	}
}
