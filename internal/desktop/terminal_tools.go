package desktop

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func (a *app) zoom(tab *workspace, steps int, reset bool) {
	_, saved := a.layout.Locate(tab.key)
	if saved == nil {
		return
	}
	size := saved.FontSize + float32(steps)
	if reset {
		size = float32(a.preferences.FontSize)
	}
	if size < 8 {
		size = 8
	}
	if size > 32 {
		size = 32
	}
	saved.FontSize = size
	if tab.term != nil {
		tab.term.SetFont(terminal.Font{Family: a.preferences.FontFamily, Size: size})
	}
}

func (a *app) toggleSearch() {
	tab := a.activeWorkspace()
	if tab == nil || tab.term == nil {
		return
	}
	tab.searchOpen = !tab.searchOpen
	if !tab.searchOpen {
		tab.term.EndSearch()
		tab.searchApplied = ""
		a.terminalFocus = tab.key
	} else {
		tab.searchFocus = true
		tab.term.Search(tab.search)
		tab.searchApplied = tab.search
	}
}

func (a *app) terminalSearch(c *ui.Context, tab *workspace) {
	key := func(s string) *ui.Context { return c.Key(fmt.Sprintf("%s-%d", s, tab.key)) }
	ui.Row(c).FillWidth().Padding(4, 7).Gap(4).Wrap().Children(func() {
		input := ui.TextInput(key("terminal-query"), &tab.search).Label("搜索终端输出").Placeholder("搜索输出…").Grow(1).MinWidth(80)
		if tab.searchFocus {
			input.Focus()
			tab.searchFocus = false
		}
		if tab.search != tab.searchApplied {
			tab.term.Search(tab.search)
			tab.searchApplied = tab.search
		}
		status := tab.term.SearchStatus()
		label := fmt.Sprintf("%d 处匹配", status.Total)
		if status.Searching {
			label = "搜索中…"
			c.After(16 * time.Millisecond)
		}
		ui.Text(key("terminal-matches"), label).FontSize(10)
		ui.Button(key("search-previous"), "上一处").FontSize(10).Padding(4).OnClick(tab.term.SearchPrevious)
		ui.Button(key("search-next"), "下一处").FontSize(10).Padding(4).OnClick(tab.term.SearchNext)
		ui.Button(key("search-close"), "关闭搜索").FontSize(10).Padding(4).OnClick(func() {
			tab.searchOpen = false
			tab.term.EndSearch()
			tab.searchApplied = ""
			a.terminalFocus = tab.key
		})
	})
	if c.Shortcut(0, ui.KeyEscape) {
		tab.searchOpen = false
		tab.term.EndSearch()
		tab.searchApplied = ""
		a.terminalFocus = tab.key
	}
}
