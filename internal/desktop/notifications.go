package desktop

import (
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) notifications(c *ui.Context) {
	for _, pending := range []struct {
		message *string
		kind    string
	}{
		{&a.actionError, "error"},
		{&a.notice, "info"},
	} {
		if *pending.message != "" {
			c.AddToast(ui.Toast{ID: pending.kind + ":" + *pending.message, Title: *pending.message, Type: pending.kind, Timeout: 3 * time.Second})
			*pending.message = ""
		}
	}
	t := c.Theme()
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Top(34).Padding(12).Justify(ui.Start).AlignItems(ui.End).Gap(8)
		for _, notification := range toasts {
			color := t.Accent
			if notification.Type == "error" {
				color = t.Danger
			}
			toast := ui.ToastBase(c, notification)
			toast.Root.Row().Width(360).MaxWidthPercent(100).Padding(12).Gap(10).
				Radius(8).Background(t.Background).Border(1, t.Border).Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, 0.18))
			toast.Root.Children(func() {
				ui.Icon(c, iconStatus).TextColor(color).Shrink(0)
				ui.Text(c, notification.Title).Wrap().Grow(1).Shrink(1)
				close := toast.CloseButton().Label("关闭通知").
					Size(24, 24).Shrink(0).Padding(0).Radius(6).TextColor(t.TextMuted)
				if close.Hovered() {
					close.Background(t.SurfaceHover)
				}
				close.Children(func() { ui.Icon(c, iconClose) })
			})
		}
	})
}
