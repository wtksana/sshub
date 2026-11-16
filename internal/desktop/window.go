package desktop

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Run(dir string) error {
	a := newApp(dir)
	defer a.close()
	mygo.App.WhenReady(func() {
		width, height := 1280, 800
		if a.layout != nil && a.layout.WindowWidth != 0 && a.layout.WindowHeight != 0 {
			width, height = a.layout.WindowWidth, a.layout.WindowHeight
		}
		w := mygo.NewWindow(mygo.WindowOptions{Title: "SSHub", Width: width, Height: height, MinWidth: 900, MinHeight: 620, Hidden: true, Content: ui.View(a.view)})
		a.win = w
		w.OnReadyToShow(w.Show)
		w.OnClose(func(e *mygo.CloseEvent) {
			if a.layout != nil {
				width, height := w.Size()
				a.layout.WindowWidth = max(900, min(16384, width))
				a.layout.WindowHeight = max(620, min(16384, height))
			}
			if !a.allowQuit {
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
