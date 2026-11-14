package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/redisread/data-shelf/internal/shelf"
)

func testApp(t *testing.T) *app {
	t.Helper()
	s, e := shelf.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return newApp(s)
}
func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
}
func TestCreateEditFormatAnnotateAndReload(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "新建文件夹")
	tt.Type("支付接口")
	tt.Key(0, ui.KeyEnter)
	if a.selected != "支付接口" {
		t.Fatalf("folder: %s %v", a.selected, tt.Texts())
	}
	click(t, tt, "新建文件")
	tt.Type("异常.json")
	tt.Key(0, ui.KeyEnter)
	if a.active == nil {
		t.Fatal(tt.Texts())
	}
	click(t, tt, "文件内容")
	tt.Type(`{"id":12345678901234567890,"message":"支付失败"}`)
	tt.Frame()
	click(t, tt, "格式化 JSON")
	if !strings.Contains(a.active.doc.Text, "\n") {
		t.Fatal("not formatted")
	}
	at := strings.Index(a.active.doc.Text, "支付失败")
	a.active.editor.Select(at, at+len("支付失败"))
	tt.Frame()
	click(t, tt, "＋ 评论")
	if len(a.active.doc.Notes) != 1 {
		t.Fatal("no note")
	}
	note := a.active.doc.Notes[0]
	click(t, tt, "评论 "+note.ID)
	tt.Type("检查上游响应")
	tt.Frame()
	if !a.flush() {
		t.Fatal(a.status)
	}
	doc, err := a.store.Load("支付接口/异常.json")
	if err != nil || len(doc.Notes) != 1 || doc.Notes[0].Comment != "检查上游响应" || doc.Notes[0].Detached {
		t.Fatalf("reload %+v %v", doc, err)
	}
	tt.SetDark(true)
	tt.Frame()
	if os.Getenv("DATA_SHELF_SCREENSHOTS") != "" {
		f, e := os.Create("/tmp/data-shelf-dark.png")
		if e != nil {
			t.Fatal(e)
		}
		png.Encode(f, tt.Image())
		f.Close()
		tt.SetDark(false)
		tt.Frame()
		f, e = os.Create("/tmp/data-shelf-light.png")
		if e != nil {
			t.Fatal(e)
		}
		png.Encode(f, tt.Image())
		f.Close()
	}
}
func TestConflictKeepsBothVersions(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "a.json", false)
	a.open("a.json")
	a.active.editor.Buffer.Set(`{"local":1}`)
	a.capture(a.active)
	os.WriteFile(filepath.Join(a.store.Path, "a.json"), []byte(`{"external":2}`), 0600)
	if a.flush() {
		t.Fatal("missed conflict")
	}
	a.resolve(a.active, true)
	if a.active == nil || !strings.Contains(a.active.doc.Path, "本地副本") {
		t.Fatal("copy not opened")
	}
	original, e := a.store.Load("a.json")
	if e != nil || original.Text != `{"external":2}` {
		t.Fatal(original, e)
	}
	if a.active.doc.Text != `{"local":1}` {
		t.Fatal("local lost")
	}
}
func TestMarkdownAndRenamePreserveComments(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "note.md", false)
	a.open("note.md")
	a.active.editor.Buffer.Set("# 排查记录\n\n| 状态 | 描述 |\n| --- | --- |\n| 失败 | 超时 |\n\n```json\n{}\n```\n")
	a.capture(a.active)
	a.active.editor.Select(2, 14)
	a.addNote()
	a.move("note.md", "renamed.md")
	tt := ui.NewTester(a.view, 1280, 820)
	click(t, tt, "分栏")
	if a.active.mode != 2 || len(a.active.doc.Notes) != 1 {
		t.Fatal("lost note or mode")
	}
	click(t, tt, "预览")
	if a.active.mode != 1 {
		t.Fatal("no preview")
	}
}

func TestThousandsOfFilesRemainVirtualized(t *testing.T) {
	a := testApp(t)
	a.entries = nil
	for i := 0; i < 5000; i++ {
		a.entries = append(a.entries, shelf.Entry{Path: fmt.Sprintf("log-%04d.json", i), Name: fmt.Sprintf("log-%04d.json", i)})
	}
	tt := ui.NewTester(a.view, 1280, 820)
	if len(tt.Texts()) > 100 {
		t.Fatalf("rendered all files: %d", len(tt.Texts()))
	}
	tt.SetSize(1040, 620)
	tt.SetDark(true)
}
