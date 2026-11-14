package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/redisread/data-shelf/internal/shelf"
)

func (a *app) workspace(c *ui.Context) {
	theme := c.Theme()
	if a.orphanView {
		a.orphanPanel(c)
		return
	}
	if len(a.tabs) > 0 {
		ui.ScrollHorizontal(c).MinHeight(46).MaxHeight(46).FillWidth().KeepScrollbars(true).Background(theme.Surface).Children(func() {
			ui.Row(c).Gap(4).Padding(6, 8).Children(func() {
				for _, t := range a.tabs {
					group := ui.Row(c).Key(t).Gap(2).Padding(0, 4).Radius(6)
					if a.active == t {
						group.Background(theme.Background).BorderWidth(0, 0, 2, 0).BorderColor(theme.Accent)
					}
					group.Children(func() {
						button := navigationButton(c, filepath.Base(t.doc.Path), false).MaxWidth(230).Tooltip(t.doc.Path)
						button.Children(func() {
							ui.Icon(c, fileGlyph(t.format)).Size(14, 14).Shrink(0)
							ui.Text(c, filepath.Base(t.doc.Path)).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
							if t.dirty {
								ui.Text(c, "•").TextColor(theme.Accent)
							}
						})
						if button.Clicked() {
							a.active = t
							a.selected = t.doc.Path
						}
						if smallIconButton(c, "关闭 "+filepath.Base(t.doc.Path), iconClose).Clicked() {
							a.closeTab(t)
						}
					})
				}
			})
		})
	}

	if a.active == nil {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(15).Children(func() {
			ui.Text(c, "{ }").FontSize(48).TextColor(theme.Accent)
			ui.Text(c, "给每一次排查，留一份记录").FontSize(24).Bold()
			ui.Text(c, "创建文件，粘贴日志，选中文字留下评论。").TextColor(theme.TextMuted)
			ui.Text(c, "JSON · Markdown · 本地保存").FontSize(12).TextColor(theme.TextMuted)
			if ui.PrimaryButton(c, "新建第一份记录").Clicked() {
				a.startName("file")
			}
			ui.Text(c, "⌘N 新建文件    ⌘⇧N 新建文件夹").FontSize(11).TextColor(theme.TextMuted)
		})
		return
	}
	t := a.active
	ui.Row(c).Padding(10, 16, 0, 16).Children(func() {
		ui.Text(c, t.doc.Path).SingleLine().Ellipsis("…").FontSize(11).TextColor(theme.TextMuted).Grow(1).MinWidth(0).Tooltip(t.doc.Path).Focusable().Label("当前文件路径")
	})
	ui.Toolbar(c, func() {
		if t.format == shelf.JSON {
			if ui.Button(c, "格式化 JSON").Tooltip("⌘⇧F").Clicked() {
				a.formatJSON()
			}
		}
		if t.format == shelf.Markdown {
			ui.Segmented(c, &t.mode, "源码", "预览", "分栏")
		}
		format := string(t.format)
		if ui.Select(c, &format, []string{"JSON", "Markdown", "文本"}).Label("文件格式").Width(115).Changed() {
			ext := map[string]string{"JSON": ".json", "Markdown": ".md", "文本": ".txt"}[format]
			a.convert(ext)
		}
		ui.Box(c).Grow(1)
		if ui.Button(c, "查找").Clicked() {
			t.editor.Find("")
			t.mode = 0
		}
		if ui.Button(c, "＋ 评论").Tooltip("选中文字后添加 · ⌘⇧M").Clicked() {
			a.addNote()
		}
		if ui.Button(c, "评论栏").Clicked() {
			a.comments = !a.comments
		}
	}).FillWidth().Padding(8, 12).Gap(8).Wrap().BorderWidth(0, 0, 1, 0).BorderColor(theme.Border)
	if t.conflict {
		ui.Column(c).Padding(10, 14).Gap(7).Background(theme.Surface).Children(func() {
			ui.Text(c, "检测到外部修改，已暂停自动保存").Bold()
			ui.Text(c, "两种操作都会保留本地副本和外部原文件。").FontSize(12)
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "查看本地副本").Clicked() {
					a.resolve(t, true)
				}
				if ui.Button(c, "加载外部版本").Clicked() {
					a.resolve(t, false)
				}
			})
		})
	}
	if t.validation != "" {
		ui.Text(c, t.validation).FontSize(12).Padding(8, 12).TextColor(theme.TextMuted)
	}
	t.editor.TextBackgrounds = nil
	for _, n := range t.doc.Notes {
		if !n.Detached {
			t.editor.TextBackgrounds = append(t.editor.TextBackgrounds, editor.TextBackground{Start: n.Start, End: n.End, Color: ui.RGBA(235, 178, 52, 0.25)})
		}
	}
	ui.Row(c).Grow(1).FillWidth().AlignItems(ui.Stretch).Children(func() {
		if t.format != shelf.Markdown || t.mode != 1 {
			c.Key(t)
			editor.View(c, t.editor).Grow(1).FillHeight()
		}
		if t.format == shelf.Markdown && t.mode != 0 {
			ui.Scroll(c).Key("preview-" + t.doc.Path).Grow(1).FillHeight().BorderStartWidth(1).BorderColor(theme.Border).Children(func() { markdown.View(c, t.doc.Text, &a.md).FillWidth().Padding(24) })
		}
	})
}
func (a *app) commentsView(c *ui.Context, compact bool) {
	t := a.active
	theme := c.Theme()
	panel := ui.Column(c).Shrink(0).Background(theme.Surface).BorderColor(theme.Border)
	if compact {
		panel.FillWidth().Height(230).BorderWidth(1, 0, 0, 0)
	} else {
		panel.Width(286).FillHeight().BorderStartWidth(1)
	}
	panel.Children(func() {
		ui.Row(c).Padding(16).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("评论 · %d", len(t.doc.Notes))).FontSize(13).Bold().Grow(1)
			if ui.Button(c, "收起").Clicked() {
				a.comments = false
			}
		})
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			if len(t.doc.Notes) == 0 {
				ui.Column(c).Padding(18).Gap(10).Children(func() {
					ui.Text(c, "让线索留在原文旁").Bold()
					ui.Text(c, "在源码中选中一段文字，\n点击「＋ 评论」。").FontSize(12).TextColor(theme.TextMuted)
				})
			}
			remove := -1
			for i := range t.doc.Notes {
				n := &t.doc.Notes[i]
				ui.Column(c).Key(n.ID).Margin(0, 12, 12, 12).Padding(12).Gap(8).Background(theme.Background).Radius(20).Children(func() {
					quote := n.Quote
					r := []rune(quote)
					if len(r) > 110 {
						quote = string(r[:110]) + "…"
					}
					ui.Text(c, quote).FontSize(12).TextColor(theme.TextMuted).Tooltip(n.Quote).Focusable().Label("评论原文：" + n.Quote)
					if n.Detached {
						ui.Text(c, "待重新定位").FontSize(11).TextColor(theme.Accent)
					}
					inputHeight := float32(90)
					if compact {
						inputHeight = 64
					}
					input := ui.TextArea(c, &n.Comment).Label("评论 " + n.ID).Placeholder("写下你的发现…").FillWidth().Height(inputHeight).Radius(8)
					if input.Changed() {
						a.changed(t)
					}
					ui.Row(c).Gap(8).Wrap().Children(func() {
						if !n.Detached {
							if ui.Button(c, "定位原文").Clicked() {
								t.mode = 0
								t.editor.Select(n.Start, n.End)
								t.editor.ScrollTo(t.editor.Buffer.Pos(n.Start).Line)
							}
						} else {
							if ui.Button(c, "关联当前选区").Clicked() {
								sel := t.editor.Selection()
								replacement, err := shelf.NewAnnotation(t.doc.Text, sel.Start(), sel.End(), n.Comment)
								if a.report(err) {
									replacement.ID = n.ID
									*n = replacement
									a.changed(t)
								}
							}
						}
						ui.Box(c).Grow(1)
						if ui.Button(c, "删除").Label("删除评论 " + n.ID).Clicked() {
							remove = i
						}
					})
				})
			}
			if remove >= 0 {
				t.doc.Notes = append(t.doc.Notes[:remove], t.doc.Notes[remove+1:]...)
				a.changed(t)
			}
		})
	})
}
func (a *app) orphanPanel(c *ui.Context) {
	ui.Row(c).Padding(16).Gap(10).Children(func() {
		ui.Text(c, "未关联评论").FontSize(20).Bold().Grow(1)
		if ui.Button(c, "返回编辑").Clicked() {
			a.orphanView = false
		}
	})
	ui.Text(c, "原文件已移走或路径被复用，评论仍保留。先打开目标文件，即可复制评论并重新定位。").Padding(0, 16, 12, 16).TextColor(c.Theme().TextMuted)
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		for i, r := range a.orphans {
			ui.Column(c).Key(fmt.Sprintf("%d:%s", i, r.Path)).Margin(8, 16).Padding(16).Gap(8).Background(c.Theme().Surface).Radius(8).Children(func() {
				ui.Text(c, r.Path).Bold()

				if a.active != nil {
					if ui.Button(c, "复制评论到当前文件").Clicked() {
						a.active.doc.Notes = append(a.active.doc.Notes, shelf.CopyAnnotations(r.Notes)...)
						a.changed(a.active)
						a.comments = true
						a.orphanView = false
						a.status = "已复制为待定位评论；选择原文后点击「关联当前选区」"
					}
				}
				for _, n := range r.Notes {
					ui.Text(c, strings.TrimSpace(n.Quote)).FontSize(12)
					ui.Text(c, n.Comment).TextColor(c.Theme().TextMuted)
				}
			})
		}
	})
}
