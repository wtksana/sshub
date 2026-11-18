package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNotificationLifetimeAndHover(t *testing.T) {
	a := &app{notice: "设置已保存"}
	view := ui.NewTester(a.notifications, 900, 620)
	if !view.HasText("设置已保存") || a.notice != "" {
		t.Fatal("feedback was not published and consumed")
	}
	view.Advance(2 * time.Second)
	r, ok := view.Find("设置已保存")
	if !ok || r.X < 500 || r.Y < 34 || r.Y > 100 {
		t.Fatal("notification is not at the top right", r)
	}
	view.Move(r.X+r.W/2, r.Y+r.H/2)
	view.Advance(4 * time.Second)
	if !view.HasText("设置已保存") {
		t.Fatal("hover did not pause expiry")
	}
	view.Move(10, 500)
	view.Advance(time.Second + time.Millisecond)
	if view.HasText("设置已保存") {
		t.Fatal("notification did not expire after three unhovered seconds")
	}
	a.notice = "设置已保存"
	view.Frame()
	view.Advance(2 * time.Second)
	a.notice = "设置已保存"
	view.Frame()
	view.Advance(2 * time.Second)
	if !view.HasText("设置已保存") || strings.Count(strings.Join(view.Texts(), "\n"), "设置已保存") != 1 {
		t.Fatal("repeated feedback did not reset expiry without duplication")
	}
	if err := view.Click("关闭通知"); err != nil {
		t.Fatal(err)
	}
	if view.HasText("设置已保存") {
		t.Fatal("manual close left notification visible")
	}
}

func TestNotificationsWrapAndKeepContentLayout(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			a := &app{}
			view := ui.NewTester(func(c *ui.Context) {
				a.preferences.Theme, a.preferences.UIFontSize = theme, 20
				colors := githubTheme(false, a.preferences)
				c.SetTheme(&colors)
				ui.Column(c).Fill().Children(func() { ui.Text(c, "终端内容") })
				a.notifications(c)
			}, 320, 620)
			before, _ := view.Find("终端内容")
			a.actionError = "无法读取标签布局，原文件已保留；本次布局不会保存。请将数据目录中的 workspace.json 重命名后重启。"
			a.notice = "设置已保存"
			view.Frame()
			after, _ := view.Find("终端内容")
			if before != after || !view.HasText("设置已保存") {
				t.Fatal("notifications displaced or hid content")
			}
			for _, text := range []string{"设置已保存", "无法读取标签布局，原文件已保留；本次布局不会保存。请将数据目录中的 workspace.json 重命名后重启。"} {
				r, ok := view.Find(text)
				if !ok || r.X < 0 || r.X+r.W > 320 || r.Y+r.H > 620 {
					t.Fatal("notification text escaped narrow window", text, r)
				}
			}
			view.Advance(3*time.Second + time.Millisecond)
			if view.HasText("设置已保存") || view.HasText("关闭通知") {
				t.Fatal("error or info notification outlived timeout")
			}
		})
	}
}
