package desktop

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Run(dir string) error {
	a := newApp(dir)
	defer a.close()
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "SSHub", Width: 1280, Height: 800, MinWidth: 900, MinHeight: 620, Hidden: true, Content: ui.View(a.view)})
		a.win = w
		w.OnReadyToShow(w.Show)
		w.OnClose(func(e *mygo.CloseEvent) {
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
