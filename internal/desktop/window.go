package desktop

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const minWindowWidth, minWindowHeight = 900, 620

func startupWindowBounds(width, height int, work mygo.Rectangle) mygo.Rectangle {
	if work.Width <= 0 || work.Height <= 0 {
		return mygo.Rectangle{Width: width, Height: height}
	}
	width, height = min(width, work.Width), min(height, work.Height)
	return mygo.Rectangle{
		X: work.X + (work.Width-width)/2, Y: work.Y + (work.Height-height)/2,
		Width: width, Height: height,
	}
}

func Run(dir string) error {
	a := newApp(dir)
	a.diagnostics = openDiagnostics()
	defer a.close()
	mygo.App.WhenReady(func() {
		width, height := 1280, 800
		if a.layout != nil && a.layout.WindowWidth != 0 && a.layout.WindowHeight != 0 {
			width, height = a.layout.WindowWidth, a.layout.WindowHeight
		}
		bounds := startupWindowBounds(width, height, mygo.Screen.PrimaryDisplay().WorkArea)
		w := mygo.NewWindow(mygo.WindowOptions{
			Title: "SSHub", X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height,
			MinWidth: min(minWindowWidth, bounds.Width), MinHeight: min(minWindowHeight, bounds.Height),
			TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 34, HideWindowButtons: true,
			Hidden: true, Content: ui.View(a.view),
		})
		a.win = w
		rememberSize := func() {
			if a.layout != nil && !w.IsMaximized() && !w.IsMinimized() {
				width, height := w.Size()
				a.layout.WindowWidth = max(minWindowWidth, min(16384, width))
				a.layout.WindowHeight = max(minWindowHeight, min(16384, height))
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
