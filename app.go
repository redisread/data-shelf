package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/redisread/data-shelf/internal/shelf"
)

type tab struct {
	doc        shelf.Document
	editor     *editor.State
	version    uint64
	revision   uint64
	dirty      bool
	conflict   bool
	problem    string
	mode       int
	format     shelf.Format
	validation string
}
type app struct {
	fileList   ui.ListState
	store      *shelf.Store
	win        *mygo.Window
	entries    []shelf.Entry
	tabs       []*tab
	active     *tab
	selected   string
	expanded   map[string]bool
	filter     string
	naming     string
	name       string
	nameParent string
	focusName  bool
	comments   bool
	status     string
	busy       bool
	afterSync  func()
	orphanView bool
	orphans    []shelf.Record
	md         markdown.Options
}

func newApp(s *shelf.Store) *app {
	a := &app{store: s, expanded: map[string]bool{".": true}, comments: true, status: "所有内容仅保存在本机"}
	a.refresh()
	return a
}
func (a *app) report(err error) bool {
	if err != nil {
		a.status = err.Error()
		return false
	}
	return true
}
func (a *app) refresh() {
	entries, err := a.store.Tree()
	if a.report(err) {
		a.entries = entries
	}
	a.orphans = a.store.Orphans()
}
func (a *app) folder() string {
	for _, e := range a.entries {
		if e.Path == a.selected {
			if e.Dir {
				return e.Path
			}
			return filepath.Dir(e.Path)
		}
	}
	return "."
}
func (a *app) startName(kind string) {
	a.naming = kind
	a.name = ""
	a.nameParent = a.folder()
	a.focusName = true
	if kind == "rename" {
		a.name = filepath.Base(a.selected)
	}
}
func (a *app) create() {
	kind, name, parent := a.naming, a.name, a.nameParent
	if !a.report(shelf.ValidName(name)) {
		return
	}
	a.whenIdle(func() {
		if kind == "rename" {
			a.move(a.selected, filepath.Join(filepath.Dir(a.selected), name))
		} else {
			path, err := a.store.Create(parent, name, kind == "folder")
			if !a.report(err) {
				return
			}
			a.expanded[parent] = true
			a.selected = path
			a.refresh()
			if kind == "file" {
				a.open(path)
			}
		}
		a.naming = ""
	})
}
func (a *app) open(path string) {
	for _, t := range a.tabs {
		if t.doc.Path == path {
			a.active = t
			a.selected = path
			a.orphanView = false
			return
		}
	}
	d, err := a.store.Load(path)
	if !a.report(err) {
		return
	}
	b := editor.NewBuffer(d.Text)
	b.SetLanguage(editor.LanguageForText(path, d.Text))
	t := &tab{doc: d, editor: &editor.State{Buffer: b, Label: "文件内容", Placeholder: "粘贴日志或开始记录…", NoAutoClose: true}, format: shelf.Detect(path, d.Text)}
	t.doc.Text = b.String()

	t.version = b.Version()
	a.validate(t)
	a.tabs = append(a.tabs, t)
	a.active = t
	a.selected = path
	a.orphanView = false
}
func (a *app) validate(t *tab) {
	t.format = shelf.Detect(t.doc.Path, t.doc.Text)
	t.validation = ""
	if t.format == shelf.JSON && strings.TrimSpace(t.doc.Text) != "" {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(t.doc.Text), &raw); err != nil {
			t.validation = "JSON 格式错误：" + err.Error()
		}
	}
}
func (a *app) capture(t *tab) {
	if t.version == t.editor.Buffer.Version() {
		return
	}
	text := t.editor.Buffer.String()
	t.doc.Notes = shelf.Reanchor(t.doc.Text, text, t.doc.Notes)
	t.doc.Text = text
	t.version = t.editor.Buffer.Version()
	a.changed(t)
	a.validate(t)
}
func (a *app) changed(t *tab) { t.dirty = true; t.revision++ }
func (a *app) formatJSON() {
	t := a.active
	if t == nil {
		return
	}
	a.capture(t)
	text, err := shelf.FormatJSON(t.doc.Text)
	if !a.report(err) {
		return
	}
	if len(text) > shelf.MaxFileSize {
		a.status = "格式化后超过 10 MB，已保留原文"
		return
	}
	t.doc.Notes = shelf.ReanchorFormatted(t.doc.Text, text, t.doc.Notes)
	t.doc.Text = text
	t.editor.Buffer.Set(text)
	t.version = t.editor.Buffer.Version()
	a.changed(t)
	a.validate(t)
}
func (a *app) addNote() {
	t := a.active
	if t == nil {
		return
	}
	a.capture(t)
	sel := t.editor.Selection()
	n, err := shelf.NewAnnotation(t.doc.Text, sel.Start(), sel.End(), "")
	if !a.report(err) {
		return
	}
	t.doc.Notes = append(t.doc.Notes, n)
	a.changed(t)
	a.comments = true
}
func (a *app) whenIdle(fn func()) {
	if a.busy {
		a.afterSync = fn
		a.status = "正在保存，随后执行操作…"
		return
	}
	fn()
}
func (a *app) flush() bool {
	for _, t := range a.tabs {
		a.capture(t)
		if t.conflict {
			a.status = "请先处理冲突，内容仍保留在编辑器中"
			return false
		}
		if !t.dirty {
			continue
		}
		saved, err := a.store.Save(snapshotDoc(t))
		t.doc.Hash = saved.Hash
		if !a.report(err) {
			t.problem = err.Error()
			if errors.Is(err, shelf.ErrConflict) {
				t.conflict = true
			}
			return false
		}
		t.dirty = false
		t.problem = ""
		t.editor.Buffer.MarkSaved()
	}
	return true
}
func (a *app) move(from, to string) {
	if !a.flush() {
		return
	}
	if !a.report(a.store.Move(from, to)) {
		return
	}
	for _, t := range a.tabs {
		if t.doc.Path == from || strings.HasPrefix(t.doc.Path, from+string(filepath.Separator)) {
			t.doc.Path = to + strings.TrimPrefix(t.doc.Path, from)
			a.validate(t)
			t.editor.Buffer.SetLanguage(editor.LanguageForText(t.doc.Path, t.doc.Text))
		}
	}
	a.selected = to
	a.expanded[filepath.Dir(to)] = true
	a.refresh()
}
func (a *app) convert(ext string) {
	if a.active == nil {
		return
	}
	t := a.active
	a.whenIdle(func() { base := strings.TrimSuffix(t.doc.Path, filepath.Ext(t.doc.Path)); a.move(t.doc.Path, base+ext) })
}
func (a *app) closeTab(t *tab) {
	a.whenIdle(func() {
		if !a.flush() {
			return
		}
		for i, item := range a.tabs {
			if item == t {
				a.tabs = append(a.tabs[:i], a.tabs[i+1:]...)
				break
			}
		}
		if a.active == t {
			a.active = nil
			if len(a.tabs) > 0 {
				a.active = a.tabs[len(a.tabs)-1]
			}
		}
	})
}
func (a *app) trash(path string) {
	a.whenIdle(func() {
		if !a.flush() {
			return
		}
		if !a.report(mygo.Shell.TrashItem(filepath.Join(a.store.Path, path))) {
			return
		}
		var keep []*tab
		for _, t := range a.tabs {
			if t.doc.Path == path || strings.HasPrefix(t.doc.Path, path+string(filepath.Separator)) {
				if a.active == t {
					a.active = nil
				}
			} else {
				keep = append(keep, t)
			}
		}
		a.tabs = keep
		a.selected = ""
		a.refresh()
		a.status = "已移到废纸篓；评论保留在「未关联评论」中"
	})
}
func (a *app) importPaths(paths []string, parent string) {
	a.whenIdle(func() {
		for _, p := range paths {
			dest, err := a.store.Import(p, parent)
			if !a.report(err) {
				break
			}
			a.status = "已复制导入：" + dest
		}
		a.expanded[parent] = true
		a.refresh()
	})
}
func (a *app) chooseImport(folder bool) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "复制导入资料", Multiple: true, Directory: folder})
	if a.report(err) {
		a.importPaths(paths, a.folder())
	}
}
func (a *app) changeRoot() {
	a.whenIdle(func() {
		if !a.flush() {
			return
		}
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "选择资料目录", Directory: true})
		if !a.report(err) || len(paths) == 0 {
			return
		}
		s, err := shelf.Open(paths[0])
		if !a.report(err) {
			return
		}
		if !a.report(saveSettings(s.Path)) {
			s.Close()
			return
		}
		a.store.Close()
		a.store = s
		a.tabs = nil
		a.active = nil
		a.selected = ""
		a.expanded = map[string]bool{".": true}
		a.refresh()
	})
}

// Preserve the local version as a new ordinary file before accepting the
// externally changed original. Neither side of a conflict is overwritten.
func (a *app) resolve(t *tab, showCopy bool) {
	a.whenIdle(func() {
		a.capture(t)
		ext := filepath.Ext(t.doc.Path)
		name := strings.TrimSuffix(filepath.Base(t.doc.Path), ext) + " (本地副本 " + time.Now().Format("20060102-150405") + ")" + ext
		parent := filepath.Dir(t.doc.Path)
		path, err := a.store.Create(parent, name, false)
		if errors.Is(err, os.ErrNotExist) {
			path, err = a.store.Create(".", name, false)
		}
		if !a.report(err) {
			return
		}
		copyDoc := snapshotDoc(t)
		copyDoc.Path = path
		copyDoc.Hash = shelf.Hash("")
		if _, err = a.store.Save(copyDoc); !a.report(err) {
			return
		}
		disk, err := a.store.Load(t.doc.Path)
		if err == nil {
			t.doc = disk
			t.editor.Buffer.Reset(disk.Text)
			t.doc.Text = t.editor.Buffer.String()
			t.version = t.editor.Buffer.Version()
			t.dirty = false
			t.conflict = false
			t.problem = ""
			a.validate(t)
		} else {
			a.status = "原文件已移走，本地内容已另存为副本"
			for i, x := range a.tabs {
				if x == t {
					a.tabs = append(a.tabs[:i], a.tabs[i+1:]...)
					break
				}
			}
			a.active = nil
			showCopy = true
		}
		a.refresh()
		if showCopy {
			a.open(path)
		}
		a.status = "已保留外部原文件及本地副本"
	})
}

type syncItem struct {
	target   *tab
	doc      shelf.Document
	revision uint64
	save     bool
}
type syncResult struct {
	item syncItem
	doc  shelf.Document
	err  error
}

func (a *app) sync() {
	if a.busy || a.win == nil {
		return
	}
	a.busy = true
	s := a.store
	var items []syncItem
	for _, t := range a.tabs {
		a.capture(t)
		if !t.conflict {
			d := snapshotDoc(t)
			d.Notes = append([]shelf.Annotation(nil), d.Notes...)
			items = append(items, syncItem{t, d, t.revision, t.dirty})
		}
	}
	go func() {
		results := make([]syncResult, 0, len(items))
		for _, item := range items {
			var d shelf.Document
			var err error
			if item.save {
				d, err = s.Save(item.doc)
			} else {
				d, err = s.Load(item.doc.Path)
			}
			results = append(results, syncResult{item, d, err})
		}
		entries, treeErr := s.Tree()
		orphans := s.Orphans()
		a.win.Update(func() {
			a.busy = false
			if a.store != s {
				return
			}
			if treeErr == nil {
				a.entries = entries
				a.orphans = orphans
			} else {
				a.report(treeErr)
			}
			for _, r := range results {
				t := r.item.target
				if t.doc.Path != r.item.doc.Path {
					continue
				}
				a.capture(t)
				if r.err != nil {
					if r.item.save && r.doc.Hash != "" {
						t.doc.Hash = r.doc.Hash
					}
					t.problem = r.err.Error()
					if errors.Is(r.err, shelf.ErrConflict) || errors.Is(r.err, os.ErrNotExist) {
						t.conflict = true
					}
					continue
				}
				if r.item.save {
					t.doc.Hash = r.doc.Hash
					if t.revision == r.item.revision {
						t.dirty = false
						t.problem = ""
						t.editor.Buffer.MarkSaved()
						if filepath.Ext(t.doc.Path) == "" && t.format == shelf.JSON {
							old := t.doc.Path
							if err := s.Move(old, old+".json"); err == nil {
								t.doc.Path = old + ".json"
								if a.selected == old {
									a.selected = t.doc.Path
								}
								t.editor.Buffer.SetLanguage(editor.LanguageFor("file.json"))
							} else {
								t.problem = "自动添加 .json 扩展名失败：" + err.Error()
							}
						}
					}
				} else if r.doc.Hash != t.doc.Hash {
					if t.dirty || t.revision != r.item.revision {
						t.conflict = true
						t.problem = shelf.ErrConflict.Error()
					} else {
						t.doc.Notes = shelf.Reanchor(t.doc.Text, shelf.Normalize(r.doc.Text), t.doc.Notes)
						t.doc.Text = shelf.Normalize(r.doc.Text)
						t.doc.Hash = r.doc.Hash
						t.editor.Buffer.Reset(r.doc.Text)
						t.version = t.editor.Buffer.Version()
						t.dirty = len(t.doc.Notes) > 0
						a.validate(t)
					}
				}
			}
			if fn := a.afterSync; fn != nil {
				a.afterSync = nil
				fn()
			}
		})
	}()
}

func (a *app) view(c *ui.Context) {
	for _, t := range a.tabs {
		a.capture(t)
	}
	theme := c.Theme()
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		a.startName("file")
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyN) {
		a.startName("folder")
	}
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		a.whenIdle(func() { a.flush() })
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyF) {
		a.formatJSON()
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyM) {
		a.addNote()
	}
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		if a.status != "" {
			ui.Row(c).FillWidth().Padding(7, 12).Gap(10).Background(theme.Surface).Children(func() {
				ui.Text(c, a.status).FontSize(12).Grow(1)
				if ui.Button(c, "×").Label("关闭提示").Clicked() {
					a.status = ""
				}
			})
		}
		ui.Row(c).Grow(1).FillWidth().AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c)
			ui.Column(c).Grow(1).FillHeight().Children(func() { a.workspace(c) })
			if a.comments && a.active != nil && !a.orphanView {
				a.commentsView(c)
			}
		})
		ui.Row(c).Height(28).PaddingX(14).AlignItems(ui.Center).Background(theme.Surface).BorderWidth(1, 0, 0, 0).BorderColor(theme.Border).Children(func() {
			label := a.status
			if a.active != nil {
				t := a.active
				switch {
				case t.problem != "":
					label = t.problem
				case t.dirty:
					label = "正在自动保存…"
				default:
					label = "已保存到本机"
				}
			}
			ui.Text(c, label).FontSize(11).TextColor(theme.TextMuted).Grow(1)
			if a.active != nil {
				pos := a.active.editor.CursorPos()
				ui.Text(c, fmt.Sprintf("%s  ·  %d 字节  ·  行 %d，列 %d", a.active.format, len(a.active.doc.Text), pos.Line+1, pos.Col+1)).FontSize(11).TextColor(theme.TextMuted)
			}
		})
	})
}
func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	panel := ui.Column(c).Width(248).FillHeight().Shrink(0).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border)
	panel.Children(func() {
		ui.Column(c).Padding(18, 14, 12, 14).Gap(5).Children(func() {
			ui.Text(c, "data-shelf").FontSize(22).Bold()
			ui.Text(c, "日志与笔记，井然有序").FontSize(11).TextColor(t.TextMuted)
		})
		ui.Row(c).PaddingX(12).Gap(6).Children(func() {
			if ui.Button(c, "＋ 文件").Label("新建文件").Grow(1).Clicked() {
				a.startName("file")
			}
			if ui.Button(c, "＋ 文件夹").Label("新建文件夹").Grow(1).Clicked() {
				a.startName("folder")
			}
		})
		ui.TextInput(c, &a.filter).Placeholder("筛选文件名…").Label("筛选文件名").Margin(10, 12).FillWidth()
		if a.naming != "" {
			ui.Column(c).Padding(4, 12, 10, 12).Gap(5).Children(func() {
				title := "新建文件"
				if a.naming == "folder" {
					title = "新建文件夹"
				}
				if a.naming == "rename" {
					title = "重命名"
				}
				ui.Text(c, title+" · Enter 完成").FontSize(11).TextColor(t.TextMuted)
				c.Key("name-" + a.naming)
				input := ui.TextInput(c, &a.name).Label("名称").FillWidth()
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
		root := ui.Button(c, "资料目录").Key("root").FillWidth().BorderWidth(0).TextAlign(ui.Start)
		if root.Clicked() {
			a.selected = ""
		}
		a.dropTarget(root, ".")
		visible := a.visibleEntries()
		a.fileList.Key = func(i int) any { return visible[i].Path }
		ui.List(c, &a.fileList, len(visible), func(i int) {
			a.treeRow(c, visible[i])
		}).Grow(1).FillWidth()
		ui.Column(c).Padding(12).Gap(6).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			if len(a.orphans) > 0 {
				if ui.Button(c, fmt.Sprintf("未关联评论 · %d 份文件", len(a.orphans))).FillWidth().Clicked() {
					a.orphanView = true
				}
			}
			ui.Row(c).Gap(6).Children(func() {
				if ui.Button(c, "导入文件").Grow(1).Clicked() {
					a.chooseImport(false)
				}
				if ui.Button(c, "导入文件夹").Grow(1).Clicked() {
					a.chooseImport(true)
				}
			})
			ui.Row(c).Gap(6).Children(func() {
				if ui.Button(c, "在 Finder 打开").Grow(1).Clicked() {
					a.report(mygo.Shell.OpenPath(a.store.Path))
				}
				if ui.Button(c, "更换目录").Clicked() {
					a.changeRoot()
				}
			})
			ui.Text(c, a.store.Path).FontSize(10).TextColor(t.TextMuted).Tooltip(a.store.Path)
		})
	})
	if files := panel.DroppedFiles(); len(files) > 0 {
		a.importPaths(files, a.folder())
	}
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
	label := e.Name
	if e.Dir {
		if a.expanded[e.Path] || a.filter != "" {
			label = "▾  " + label
		} else {
			label = "▸  " + label
		}
	} else {
		switch shelf.Detect(e.Name, "") {
		case shelf.JSON:
			label = "{}  " + label
		case shelf.Markdown:
			label = "M↓  " + label
		default:
			label = "·  " + label
		}
	}
	row := ui.Button(c, label).Key(e.Path).Label(e.Name).FillWidth().Padding(5, 8, 5, float32(12+depth*14)).BorderWidth(0).TextAlign(ui.Start).Drag(e.Path)
	if a.selected == e.Path {
		row.Background(c.Theme().Accent).TextColor(ui.Hex("#ffffff"))
	}
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
		a.dropTarget(row, e.Path)
	}
}
func (a *app) dropTarget(e ui.Element, parent string) {
	if _, over := ui.DragOver[string](e); over {
		e.Background(cyan())
	}
	if from, ok := ui.Drop[string](e); ok {
		a.whenIdle(func() { a.move(from, filepath.Join(parent, filepath.Base(from))) })
	}
}
func cyan() ui.Color { return ui.Hex("#1f9b99") }

func snapshotDoc(t *tab) shelf.Document {
	d := t.doc
	var out bytes.Buffer
	_, _ = t.editor.Buffer.WriteTo(&out) // bytes.Buffer.Write never fails.
	d.Text = out.String()
	d.Notes = append([]shelf.Annotation(nil), t.doc.Notes...)
	return d
}
