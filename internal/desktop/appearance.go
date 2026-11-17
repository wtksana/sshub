package desktop

import (
	"runtime"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

// GitHub's canvas, foreground and ANSI colors keep chrome and terminals consistent.
func githubTheme(systemDark bool, p library.Preferences) ui.Theme {
	dark := systemDark
	if p.Theme == "light" {
		dark = false
	} else if p.Theme == "dark" {
		dark = true
	}
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
	}
	t.Spacing, t.Radius, t.FontSize = 4, 6, float32(p.EffectiveUIFontSize())
	t.Font = p.UIFontFamily
	if t.Font == "" && runtime.GOOS == "windows" {
		t.Font = "Microsoft YaHei UI"
	}
	if dark {
		t.Background, t.Surface = ui.Hex("#0d1117"), ui.Hex("#161b22")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#21262d"), ui.Hex("#30363d")
		t.Border, t.Text, t.TextMuted = ui.Hex("#30363d"), ui.Hex("#e6edf3"), ui.Hex("#8b949e")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#1f6feb"), ui.Hex("#388bfd"), ui.Hex("#1158c7")
		t.Danger, t.Warning, t.Success = ui.Hex("#f85149"), ui.Hex("#d29922"), ui.Hex("#3fb950")
		t.Selection, t.Focus, t.Scrollbar = ui.Hex("#1c3553"), ui.Hex("#58a6ff"), ui.Hex("#484f58")
	} else {
		t.Background, t.Surface = ui.Hex("#ffffff"), ui.Hex("#f6f8fa")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#eaeef2"), ui.Hex("#d8dee4")
		t.Border, t.Text, t.TextMuted = ui.Hex("#d0d7de"), ui.Hex("#24292f"), ui.Hex("#57606a")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#0969da"), ui.Hex("#0860ca"), ui.Hex("#0757ba")
		t.Danger, t.Warning, t.Success = ui.Hex("#cf222e"), ui.Hex("#9a6700"), ui.Hex("#1a7f37")
		t.Selection, t.Focus, t.Scrollbar = ui.Hex("#ddf4ff"), ui.Hex("#0969da"), ui.Hex("#afb8c1")
	}
	t.AccentText, t.InactiveAccent = ui.Hex("#ffffff"), t.Accent
	t.FocusRingWidth, t.FocusRingOffset = 2, -1
	t.Inverse, t.InverseText = t.Text, t.Background
	return t
}

func typeSize(c *ui.Context, delta float32) float32 { return max(10, c.Theme().FontSize+delta) }

func textInput(c *ui.Context, value *string) ui.Element {
	return ui.TextInput(c, value).MinHeight(max(36, c.Theme().FontSize+18)).Padding(8, 10).Radius(6).Background(c.Theme().Background)
}

func formField(c *ui.Context, label string, content func()) ui.Element {
	// Keep Field's label/focus relationship for keyboard and assistive technology.
	t := c.Theme()
	return ui.Field(c, label, func() {
		ui.Column(c).FillWidth().FontSize(t.FontSize).TextColor(t.Text).Children(content)
	}).FillWidth().Gap(6).FontSize(typeSize(c, -1)).TextColor(t.TextMuted)
}

func dialogStyle(c *ui.Context, panel ui.Element, width float32) {
	panel.Width(width).MaxWidthPercent(94).MaxHeightPercent(94).Padding(24).Gap(12).Radius(10).Background(c.Theme().Background).Border(1, c.Theme().Border)
}

func githubTerminal(dark bool) *terminal.Theme {
	t := githubTheme(dark, library.DefaultPreferences())
	out := &terminal.Theme{Foreground: t.Text, Background: t.Background, Cursor: t.Text, CursorText: t.Background, Selection: t.Selection, SelectionText: t.Text}
	colors := []string{"#24292e", "#d73a49", "#28a745", "#dbab09", "#0366d6", "#5a32a3", "#0598bc", "#6a737d", "#959da5", "#cb2431", "#22863a", "#b08800", "#005cc5", "#5a32a3", "#3192aa", "#d1d5da"}
	if dark {
		colors = []string{"#484f58", "#ff7b72", "#3fb950", "#d29922", "#58a6ff", "#bc8cff", "#39c5cf", "#b1bac4", "#6e7681", "#ffa198", "#56d364", "#e3b341", "#79c0ff", "#d2a8ff", "#56d4dd", "#f0f6fc"}
	}
	for i, color := range colors {
		out.Palette[i] = ui.Hex(color)
	}
	return out
}

var githubTerminalLight, githubTerminalDark = githubTerminal(false), githubTerminal(true)
