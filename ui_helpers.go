package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/redisread/data-shelf/internal/shelf"
)

// One 16px outline set, colored by the containing control's state.
func glyph(path string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
}

var (
	iconFolder     = glyph(`<path d="M2 4V3h4l2 2h6v8H2z"/>`)
	iconFolderOpen = glyph(`<path d="M2 7V3h4l2 2h5v2M2 7h12l-2 6H2z"/>`)
	iconFile       = glyph(`<path d="M4 2h5l3 3v9H4zM9 2v4h3M6 9h4M6 11h3"/>`)
	iconJSON       = glyph(`<path d="M6 2H4v4L2 8l2 2v4h2M10 2h2v4l2 2-2 2v4h-2"/>`)
	iconMarkdown   = glyph(`<path d="M2 11V5l3 3 3-3v6M10 8l2.5 3 2.5-3M12.5 4v7"/>`)
	iconChevron    = glyph(`<path d="m6 4 4 4-4 4"/>`)
	iconPlus       = glyph(`<path d="M8 3v10M3 8h10"/>`)
	iconFolderPlus = glyph(`<path d="M2 4V3h4l2 2h6v8H2zM8 7v4M6 9h4"/>`)
	iconClose      = glyph(`<path d="m5 5 6 6M11 5l-6 6"/>`)
)

func fileGlyph(format shelf.Format) *ui.SVG {
	switch format {
	case shelf.JSON:
		return iconJSON
	case shelf.Markdown:
		return iconMarkdown
	default:
		return iconFile
	}
}

// Navigation rows intentionally respond instantly; they do not scale on press.
func navigationButton(c *ui.Context, label string, selected bool) ui.Element {
	t := c.Theme()
	e := ui.ButtonBase(c).Label(label).Justify(ui.Start).AlignItems(ui.Center).Gap(8).MinHeight(32).Padding(6, 10).Radius(6).AccessSelected(selected)
	color := t.Text
	switch {
	case selected:
		e.Background(t.Accent)
		color = t.AccentText
	case e.Pressed():
		e.Background(t.SurfacePressed)
	case e.Hovered():
		e.Background(t.SurfaceHover)
	}
	return e.TextColor(color)
}
func smallIconButton(c *ui.Context, label string, icon *ui.SVG) ui.Element {
	e := navigationButton(c, label, false).Size(28, 28).MinHeight(28).Padding(6).Justify(ui.Center).Tooltip(label)
	e.Children(func() { ui.Icon(c, icon).Size(16, 16) })
	return e
}
