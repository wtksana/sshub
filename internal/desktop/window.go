package desktop

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Run(dir string) error {
	a := newApp(dir)
	a.diagnostics = openDiagnostics()
	defer a.close()
	mygo.App.WhenReady(func() {
		width, height := 1280, 800
		if a.layout != nil && a.layout.WindowWidth != 0 && a.layout.WindowHeight != 0 {
			width, height = a.layout.WindowWidth, a.layout.WindowHeight
		}
		w := mygo.NewWindow(mygo.WindowOptions{
			Title: "SSHub", Width: width, Height: height, MinWidth: 900, MinHeight: 620,
			TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 34, HideWindowButtons: true,
			Hidden: true, Content: ui.View(a.view),
		})
		a.win = w
		rememberSize := func() {
			if a.layout != nil && !w.IsMaximized() && !w.IsMinimized() {
				width, height := w.Size()
				a.layout.WindowWidth = max(900, min(16384, width))
				a.layout.WindowHeight = max(620, min(16384, height))
			}
		}
		w.OnResize(rememberSize)
		w.OnReadyToShow(func() {
			w.Show()
			if a.diagnostics != nil {
				a.diagnostics.ready = time.Now()
				a.diagnostics.last = time.Time{}
				a.sampleDiagnostics()
			}
		})
		w.OnClose(func(e *mygo.CloseEvent) {
			rememberSize()
			if !a.allowQuit {
				if a.hasTransfers() {
					e.PreventDefault()
					w.Update(func() { a.confirmQuit = true })
					return
				}
				for _, tab := range a.tabs {
					if busy(tab) {
						e.PreventDefault()
						w.Update(func() { a.confirmQuit = true })
						return
					}
				}
			}
		})
	})
	return mygo.App.Run()
}
