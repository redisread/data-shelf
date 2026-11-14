package shelf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func createText(t *testing.T, s *Store, path, text string) Document {
	t.Helper()
	_, e := s.Create(".", path, false)
	if e != nil {
		t.Fatal(e)
	}
	d, e := s.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	d.Text = text
	d, e = s.Save(d)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestPersistMoveAndReopen(t *testing.T) {
	s := testStore(t)
	d := createText(t, s, "支付.json", `{"金额":123456789012345678901234567890}`)
	n, e := NewAnnotation(d.Text, 2, 8, "检查字段")
	if e != nil {
		t.Fatal(e)
	}
	d.Notes = []Annotation{n}
	if _, e = s.Save(d); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(".", "接口", true); e != nil {
		t.Fatal(e)
	}
	if e = s.Move(d.Path, "接口/支付.json"); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s2, e := Open(s.Path)
	if e != nil {
		t.Fatal(e)
	}
	defer s2.Close()
	loaded, e := s2.Load("接口/支付.json")
	if e != nil || len(loaded.Notes) != 1 || loaded.Notes[0].Detached || loaded.Text != d.Text {
		t.Fatalf("lost contents: %+v %v", loaded, e)
	}
}
func TestConflictDoesNotOverwrite(t *testing.T) {
	s := testStore(t)
	d := createText(t, s, "a.json", "{}")
	os.WriteFile(filepath.Join(s.Path, d.Path), []byte("external"), 0600)
	d.Text = "local"
	if _, e := s.Save(d); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(s.Path, d.Path))
	if string(b) != "external" {
		t.Fatal("overwrote external change")
	}
}
func TestUnsafePathsAndOverwrite(t *testing.T) {
	s := testStore(t)
	for _, name := range []string{"../x", "/tmp/x", ".data-shelf", "a/b", "", " a"} {
		if _, e := s.Create(".", name, false); e == nil {
			t.Fatal(name)
		}
	}
	createText(t, s, "a", "first")
	if _, e := s.Create(".", "a", false); e == nil {
		t.Fatal("overwrite")
	}
	os.Symlink(t.TempDir(), filepath.Join(s.Path, "link"))
	if _, e := s.Create("link", "x", false); e == nil {
		t.Fatal("followed symlink")
	}
}
func TestExternalEditDetachesNotesAndMoveRetainsOrphans(t *testing.T) {
	s := testStore(t)
	d := createText(t, s, "a", "hello")
	n, _ := NewAnnotation(d.Text, 0, 5, "note")
	d.Notes = []Annotation{n}
	s.Save(d)
	os.WriteFile(filepath.Join(s.Path, "a"), []byte("hello elsewhere"), 0600)
	loaded, e := s.Load("a")
	if e != nil || !loaded.Notes[0].Detached {
		t.Fatal("unverified anchor survived")
	}
	os.Rename(filepath.Join(s.Path, "a"), filepath.Join(s.Path, "b"))
	if len(s.Orphans()) != 1 {
		t.Fatal("lost orphan")
	}
}
func TestImportAtomicAndDoesNotOverwrite(t *testing.T) {
	s := testStore(t)
	source := t.TempDir()
	os.WriteFile(filepath.Join(source, "a.md"), []byte("# Hi"), 0600)
	path, e := s.Import(source, ".")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Load(filepath.Join(path, "a.md")); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Import(source, "."); e == nil {
		t.Fatal("overwrote import")
	}
	bad := t.TempDir()
	os.WriteFile(filepath.Join(bad, "binary"), []byte{0, 255}, 0600)
	if _, e = s.Import(bad, "."); e == nil {
		t.Fatal("accepted binary")
	}
	if _, e = os.Stat(filepath.Join(s.Path, filepath.Base(bad))); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("partial import visible")
	}
}
func TestLargeFileRejected(t *testing.T) {
	s := testStore(t)
	os.WriteFile(filepath.Join(s.Path, "large"), []byte(strings.Repeat("x", MaxFileSize+1)), 0600)
	if _, e := s.Load("large"); e == nil {
		t.Fatal("accepted oversized file")
	}
}
func TestCorruptMetadataPreserved(t *testing.T) {
	path := t.TempDir()
	os.Mkdir(filepath.Join(path, metaDir), 0700)
	f := filepath.Join(path, metaDir, "metadata.json")
	os.WriteFile(f, []byte("broken"), 0600)
	if s, e := Open(path); e == nil {
		s.Close()
		t.Fatal("accepted corrupt metadata")
	}
	b, _ := os.ReadFile(f)
	if string(b) != "broken" {
		t.Fatal("metadata overwritten")
	}
}
func TestJSONPrecisionAndAnchors(t *testing.T) {
	source := `{"id":123456789012345678901234567890,"id":2,"message":"a b"}`
	pretty, e := FormatJSON(source)
	if e != nil || !strings.Contains(pretty, "123456789012345678901234567890") || strings.Count(pretty, `"id"`) != 2 {
		t.Fatal(pretty, e)
	}
	at := strings.Index(source, "a b")
	n, _ := NewAnnotation(source, at, at+3, "why")
	notes := ReanchorFormatted(source, pretty, []Annotation{n})
	if notes[0].Detached || pretty[notes[0].Start:notes[0].End] != "a b" {
		t.Fatal(notes)
	}
	notes = Reanchor(source, "prefix"+source, []Annotation{n})
	if notes[0].Start != at+6 {
		t.Fatal(notes)
	}
	notes = Reanchor(source, strings.Replace(source, "a b", "deleted", 1), []Annotation{n})
	if !notes[0].Detached {
		t.Fatal("attached deleted selection")
	}
}

func TestSingleWriterLock(t *testing.T) {
	s := testStore(t)
	other, e := Open(s.Path)
	if e == nil {
		other.Close()
		t.Fatal("allowed simultaneous metadata writers")
	}
	s.Close()
	other, e = Open(s.Path)
	if e != nil {
		t.Fatal(e)
	}
	other.Close()
}
func BenchmarkFormatTenMB(b *testing.B) {
	text := `{"events":[` + strings.Repeat(`{"status":"ok","id":12345678901234567890},`, 220000) + `{}]}`
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := FormatJSON(text); e != nil {
			b.Fatal(e)
		}
	}
}
