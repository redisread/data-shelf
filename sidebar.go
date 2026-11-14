package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/redisread/data-shelf/internal/shelf"
)

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	panel := ui.Column(c).Width(280).FillHeight().Shrink(0).Background(t.Surface).BorderEndWidth(1).BorderColor(t.Border)
	panel.Children(func() {
		ui.Column(c).Padding(20, 16, 16, 16).Gap(16).Children(func() {
			ui.Row(c).Gap(10).Children(func() {
				ui.Icon(c, iconJSON).Size(24, 24).TextColor(t.Accent)
				ui.Text(c, "data-shelf").FontSize(20).Bold()
			})
			ui.Row(c).Gap(8).Children(func() {
				create := ui.PrimaryButton(c, "").Label("新建文件").Grow(1).MinHeight(34).Gap(6)
				create.Children(func() { ui.Icon(c, iconPlus).Size(16, 16); ui.Text(c, "新建文件") })
				if create.Clicked() {
					a.startName("file")
				}
				folder := ui.Button(c, "").Label("新建文件夹").Tooltip("新建文件夹 · ⌘⇧N").Size(36, 34).Padding(8)
				folder.Children(func() { ui.Icon(c, iconFolderPlus).Size(16, 16) })
				if folder.Clicked() {
					a.startName("folder")
				}
			})
			ui.TextInput(c, &a.filter).Placeholder("搜索文件名…").Label("筛选文件名").FillWidth()
		})
		if a.naming != "" {
			a.nameInput(c)
		}
		ui.Row(c).Padding(0, 16, 6, 16).Children(func() {
			ui.Text(c, "文件").FontSize(11).TextColor(t.TextMuted).Grow(1)
			ui.Text(c, fmt.Sprintf("%d 项", len(a.entries))).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Column(c).PaddingX(8).Children(func() {
			root := navigationButton(c, "资料目录", a.selected == "").FillWidth().Tooltip(a.store.Path)
			root.Children(func() {
				ui.Icon(c, iconFolderOpen).Size(16, 16)
				ui.Text(c, "全部文件").FontSize(13).Grow(1).MinWidth(0)
			})
			if root.Clicked() {
				a.selected = ""
			}
			a.dropTarget(c, root, ".")
		})
		visible := a.visibleEntries()
		a.fileList.Key = func(i int) any { return visible[i].Path }
		// Deep trees scroll horizontally instead of consuming all room with indent.
		rowWidth := float32(240)
		for _, e := range visible {
			rowWidth = max(rowWidth, 176+16*float32(strings.Count(e.Path, string(filepath.Separator))))
		}
		ui.List(c, &a.fileList, len(visible), func(i int) { a.treeRow(c, visible[i]) }).Grow(1).MinHeight(0).FillWidth().Padding(4, 8).RowWidth(rowWidth).KeepScrollbars(true).Children(func() {
			if len(visible) == 0 {
				message := "暂无文件，点击上方新建"
				if a.filter != "" {
					message = "没有匹配的文件"
				}
				ui.Text(c, message).FontSize(12).TextColor(t.TextMuted).Padding(16, 8)
			}
		})
		ui.Column(c).Padding(12, 16, 16, 16).Gap(10).Children(func() {
			if len(a.orphans) > 0 {
				if ui.Button(c, fmt.Sprintf("未关联评论 · %d", len(a.orphans))).FillWidth().Clicked() {
					a.orphanView = true
				}
			}
			ui.Row(c).Gap(8).Children(func() {
				ui.MenuButton(c, "导入", func(m *ui.Menu) {
					if m.Item("导入文件…").Chosen() {
						a.chooseImport(false)
					}
					if m.Item("导入文件夹…").Chosen() {
						a.chooseImport(true)
					}
				}).Label("导入资料").Grow(1)
				ui.MenuButton(c, "目录设置", func(m *ui.Menu) {
					if m.Item("在 Finder 中打开").Chosen() {
						a.report(mygo.Shell.OpenPath(a.store.Path))
					}
					if m.Item("更换资料目录…").Chosen() {
						a.changeRoot()
					}
				}).Label("目录设置").Grow(1)
			})
			ui.Text(c, a.store.Path).FontSize(11).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth().Tooltip(a.store.Path).Focusable().Label("资料目录路径：" + a.store.Path)
		})
	})
	if files := panel.DroppedFiles(); len(files) > 0 {
		a.importPaths(files, a.folder())
	}
}
func (a *app) nameInput(c *ui.Context) {
	ui.Column(c).Padding(0, 16, 16, 16).Gap(6).Children(func() {
		title := "新建文件"
		if a.naming == "folder" {
			title = "新建文件夹"
		}
		if a.naming == "rename" {
			title = "重命名"
		}
		ui.Row(c).Children(func() {
			ui.Text(c, title).FontSize(12).Bold().Grow(1)
			if smallIconButton(c, "取消命名", iconClose).Clicked() {
				a.naming = ""
			}
		})
		parent := a.nameParent
		if parent == "." {
			parent = "全部文件"
		}
		ui.Text(c, "位置："+parent).SingleLine().Ellipsis("…").FontSize(11).TextColor(c.Theme().TextMuted).Tooltip(parent)
		c.Key("name-" + a.naming)
		input := ui.TextInput(c, &a.name).Label("名称").Placeholder("输入名称，Enter 创建").FillWidth()
		if a.focusName {
			input.Focus()
			a.focusName = false
		}
		if input.Submitted() {
			a.create()
		}
		if c.Shortcut(0, ui.KeyEscape) {
			a.naming = ""
		}
	})
}
func (a *app) visibleEntries() []shelf.Entry {
	matches := map[string]bool{}
	query := strings.ToLower(a.filter)
	if query != "" {
		for _, e := range a.entries {
			if strings.Contains(strings.ToLower(e.Path), query) {
				for p := e.Path; p != "."; p = filepath.Dir(p) {
					matches[p] = true
				}
			}
		}
	}
	var out []shelf.Entry
	for _, e := range a.entries {
		if query != "" {
			if matches[e.Path] {
				out = append(out, e)
			}
			continue
		}
		visible := true
		for p := filepath.Dir(e.Path); p != "."; p = filepath.Dir(p) {
			if !a.expanded[p] {
				visible = false
				break
			}
		}
		if visible {
			out = append(out, e)
		}
	}
	return out
}
func (a *app) treeRow(c *ui.Context, e shelf.Entry) {
	depth := strings.Count(e.Path, string(filepath.Separator))
	opened := e.Dir && (a.expanded[e.Path] || a.filter != "")
	c.Key(e.Path)
	row := navigationButton(c, e.Name, a.selected == e.Path).FillWidth().PaddingStart(float32(10 + depth*16)).Drag(e.Path).Tooltip(e.Path)
	row.Children(func() {
		if e.Dir {
			arrow := ui.Icon(c, iconChevron).Size(12, 12).Shrink(0)
			if opened {
				arrow.Rotate(90)
			} else if c.Direction() == ui.RightToLeft {
				arrow.Rotate(180)
			}
		} else {
			ui.Box(c).Width(12).Shrink(0)
		}
		icon := fileGlyph(shelf.Detect(e.Name, ""))
		if e.Dir {
			icon = iconFolder
			if opened {
				icon = iconFolderOpen
			}
		}
		ui.Icon(c, icon).Size(16, 16).Shrink(0)
		ui.Text(c, e.Name).Label("文件名：" + e.Path).FontSize(13).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
	})
	if row.Clicked() {
		a.selected = e.Path
		if e.Dir {
			a.expanded[e.Path] = !a.expanded[e.Path]
		} else {
			a.open(e.Path)
		}
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("重命名").Chosen() {
			a.selected = e.Path
			a.startName("rename")
		}
		if e.Dir {
			if m.Item("新建文件").Chosen() {
				a.selected = e.Path
				a.startName("file")
			}
			if m.Item("新建文件夹").Chosen() {
				a.selected = e.Path
				a.startName("folder")
			}
		}
		m.Separator()
		if m.Item("移到废纸篓").Danger().Chosen() {
			a.trash(e.Path)
		}
	})
	if e.Dir {
		a.dropTarget(c, row, e.Path)
	}
}

func (a *app) dropTarget(c *ui.Context, e ui.Element, parent string) {
	if _, over := ui.DragOver[string](e); over {
		e.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
	}
	if from, ok := ui.Drop[string](e); ok {
		a.whenIdle(func() { a.move(from, filepath.Join(parent, filepath.Base(from))) })
	}
}
