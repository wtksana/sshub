package desktop

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/lucide/*.svg
var iconFiles embed.FS

var (
	iconMenu     = loadIcon("menu")
	iconPlus     = loadIcon("plus")
	iconFilter   = loadIcon("list-filter")
	iconMore     = loadIcon("ellipsis")
	iconMinus    = loadIcon("minus")
	iconMaximize = loadIcon("square")
	iconRestore  = loadIcon("copy")
	iconClose    = loadIcon("x")
	iconStar     = loadIcon("star")
	iconStatus   = loadIcon("circle")
	iconUpload   = loadIcon("upload")
	iconDownload = loadIcon("download")
	iconFolder   = loadIcon("folder")
	iconFile     = loadIcon("file")
	iconSymlink  = loadIcon("file-symlink")
	iconParent   = loadIcon("corner-left-up")
	iconPrevious = loadIcon("chevron-up")
	iconNext     = loadIcon("chevron-down")
)

func loadIcon(name string) *ui.SVG {
	data, err := iconFiles.ReadFile("icons/lucide/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return ui.MustParseSVG(data)
}

func iconButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(28, 28).Padding(0).Radius(2).FontSize(16).TextColor(c.Theme().Text)
	if b.Pressed() {
		b.Background(c.Theme().SurfacePressed)
	} else if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, icon).Shrink(0) })
	return b
}
