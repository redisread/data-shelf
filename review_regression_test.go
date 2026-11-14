package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/redisread/data-shelf/internal/shelf"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewErrorVisible(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "a", false)
	a.open("a")
	a.report(os.ErrExist)
	tt := ui.NewTester(a.view, 1280, 820)
	if !tt.HasText(os.ErrExist.Error()) {
		t.Fatalf("error hidden, texts=%v", tt.Texts())
	}
}
func TestReviewDeletedParentRecovery(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "dir", true)
	a.store.Create("dir", "a.md", false)
	a.open("dir/a.md")
	a.active.editor.Buffer.Set("unsaved")
	a.capture(a.active)
	os.Rename(filepath.Join(a.store.Path, "dir"), filepath.Join(a.store.Path, "moved"))
	a.active.conflict = true
	a.resolve(a.active, true)
	if a.active.conflict {
		t.Fatalf("cannot recover: %s", a.status)
	}
}
func TestReviewRepeatedSelection(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "a.md", false)
	a.open("a.md")
	a.active.editor.Buffer.Set("same\nsame\n")
	a.capture(a.active)
	a.active.editor.Select(0, 4)
	a.addNote()
	a.active.editor.Buffer.Edit(editor.Edit{Start: 0, End: 5, Text: ""})
	a.capture(a.active)
	if !a.active.doc.Notes[0].Detached {
		t.Fatalf("deleted first selection rebound to second: %+v", a.active.doc.Notes)
	}
}
func TestReviewCommentDoesNotRewriteCRLF(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "a.md", false)
	original := "line one\r\nline two\r\n"
	os.WriteFile(filepath.Join(a.store.Path, "a.md"), []byte(original), 0600)
	a.open("a.md")
	a.active.editor.Select(0, 4)
	a.addNote()
	if !a.flush() {
		t.Fatal(a.status)
	}
	b, _ := os.ReadFile(filepath.Join(a.store.Path, "a.md"))
	if string(b) != original {
		t.Fatalf("comment rewrote content: %q", b)
	}
}
func TestReviewOrphanNotOverwrittenByMove(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "old.md", false)
	a.open("old.md")
	a.active.editor.Buffer.Set("old")
	a.capture(a.active)
	a.active.editor.Select(0, 3)
	a.addNote()
	a.flush()
	os.Rename(filepath.Join(a.store.Path, "old.md"), filepath.Join(a.store.Path, "external.md"))
	a.tabs = nil
	a.active = nil
	a.store.Create(".", "new.md", false)
	a.open("new.md")
	a.active.editor.Buffer.Set("new")
	a.capture(a.active)
	a.flush()
	a.move("new.md", "old.md")
	s := a.store
	s.Close()
	s2, e := shelf.Open(s.Path)
	if e != nil {
		t.Fatal(e)
	}
	defer s2.Close()
	d, e := s2.Load("old.md")
	if e != nil {
		t.Fatal(e)
	}
	orphans := s2.Orphans()
	if len(d.Notes) == 0 && len(orphans) == 0 {
		t.Fatal("old orphan comments permanently overwritten")
	}
}

func TestMixedLineEndingsAndLaterLineAnnotation(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "mixed.md", false)
	original := "first\r\nsecond\nthird\r\n"
	os.WriteFile(filepath.Join(a.store.Path, "mixed.md"), []byte(original), 0600)
	a.open("mixed.md")
	at := strings.Index(a.active.doc.Text, "third")
	a.active.editor.Select(at, at+5)
	a.addNote()
	if !a.flush() {
		t.Fatal(a.status)
	}
	d, e := a.store.Load("mixed.md")
	if e != nil || len(d.Notes) != 1 || d.Notes[0].Detached || d.Text != original {
		t.Fatalf("mixed endings %+v %v", d, e)
	}
}

func TestSecondDirectoryReuseKeepsOrphanVisible(t *testing.T) {
	a := testApp(t)
	a.store.Create(".", "old", false)
	a.open("old")
	a.active.editor.Buffer.Set("note")
	a.capture(a.active)
	a.active.editor.Select(0, 4)
	a.addNote()
	a.flush()
	os.Rename(filepath.Join(a.store.Path, "old"), filepath.Join(a.store.Path, "moved"))
	a.store.Create(".", "old", true)
	if len(a.store.Orphans()) == 0 {
		t.Fatal("old file comment hidden by directory path reuse")
	}
}
