package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/redisread/data-shelf/internal/shelf"
)

func TestTrashFromContextMenuPreservesData(t *testing.T) {
	for _, width := range []int{1040, 1280} {
		for _, folder := range []bool{false, true} {
			t.Run(fmt.Sprintf("width=%d/folder=%t", width, folder), func(t *testing.T) {
				a := testApp(t)
				target, path := "note.txt", "note.txt"
				if folder {
					target = "folder"
					path = "folder/note.txt"
					if _, err := a.store.Create(".", target, true); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := a.store.Create(filepath.Dir(path), filepath.Base(path), false); err != nil {
					t.Fatal(err)
				}
				a.open(path)
				a.active.editor.Buffer.Set("unsaved text")
				a.capture(a.active)
				a.active.editor.Select(0, 7)
				a.addNote()
				a.active.doc.Notes[0].Comment = "keep comment"
				a.refresh()
				destination := filepath.Join(t.TempDir(), target)
				a.trashItem = func(source string) error { return os.Rename(source, destination) }
				tt := ui.NewTester(a.view, width, 820)
				if err := tt.RightClick(target); err != nil {
					t.Fatal(err)
				}
				if err := tt.ChooseMenuItem("移到废纸篓"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if a.active != nil || len(a.tabs) != 0 {
					t.Fatal("trashed tab still open")
				}
				if _, err := os.Stat(filepath.Join(a.store.Path, target)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("source still exists: %v", err)
				}
				trashedFile := destination
				if folder {
					trashedFile = filepath.Join(destination, "note.txt")
				}
				data, err := os.ReadFile(trashedFile)
				if err != nil || string(data) != "unsaved text" {
					t.Fatalf("trashed text: %q, %v", data, err)
				}
				root := a.store.Path
				if err := a.store.Close(); err != nil {
					t.Fatal(err)
				}
				reopened, err := shelf.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				orphans := reopened.Orphans()
				if len(orphans) != 1 || len(orphans[0].Notes) != 1 || orphans[0].Notes[0].Comment != "keep comment" {
					t.Fatalf("lost comments after restart: %+v", orphans)
				}
				if err := os.Rename(destination, filepath.Join(root, target)); err != nil {
					t.Fatal(err)
				}
				doc, err := reopened.Load(path)
				if err != nil || doc.Text != "unsaved text" || len(doc.Notes) != 1 || doc.Notes[0].Comment != "keep comment" {
					t.Fatalf("restored document: %+v, %v", doc, err)
				}
			})
		}
	}
}

func TestTrashFailureKeepsDocument(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprintf("conflict=%t", conflict), func(t *testing.T) {
			a := testApp(t)
			if _, err := a.store.Create(".", "note.txt", false); err != nil {
				t.Fatal(err)
			}
			a.open("note.txt")
			original := a.active
			original.editor.Buffer.Set("local text")
			a.capture(original)
			a.active.editor.Select(0, 5)
			a.addNote()
			a.active.doc.Notes[0].Comment = "local comment"
			called := false
			a.trashItem = func(string) error { called = true; return os.ErrPermission }
			expected := "local text"
			if conflict {
				expected = "external text"
				if err := os.WriteFile(filepath.Join(a.store.Path, "note.txt"), []byte(expected), 0600); err != nil {
					t.Fatal(err)
				}
			}
			a.trash("note.txt")
			if called == conflict {
				t.Fatal("unexpected trash call")
			}
			if a.active != original || len(a.tabs) != 1 || original.doc.Text != "local text" || original.doc.Notes[0].Comment != "local comment" {
				t.Fatal("local document lost")
			}
			data, err := os.ReadFile(filepath.Join(a.store.Path, "note.txt"))
			if err != nil || string(data) != expected {
				t.Fatalf("disk data changed: %q %v", data, err)
			}
			if conflict && !original.conflict {
				t.Fatal("conflict not surfaced")
			}
			if a.status == "" {
				t.Fatal("missing error")
			}
		})
	}
}
