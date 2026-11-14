package shelf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
)

const metaDir = ".data-shelf"

var ErrConflict = errors.New("文件已在外部修改，请选择保留哪一份内容")

type Record struct {
	Path  string       `json:"path"`
	Hash  string       `json:"hash"`
	Notes []Annotation `json:"notes,omitempty"`
}
type metadata struct {
	Version int               `json:"version"`
	Records map[string]Record `json:"records"`
}
type Store struct {
	mu   sync.Mutex
	root *os.Root
	lock *os.File
	Path string
	meta metadata
}
type Entry struct {
	Path string
	Name string
	Dir  bool
	Size int64
}
type Document struct {
	Path, Text, Hash string
	Notes            []Annotation
}

func Hash(text string) string { v := sha256.Sum256([]byte(text)); return hex.EncodeToString(v[:]) }

func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	s := &Store{root: root, Path: abs, meta: metadata{Version: 1, Records: map[string]Record{}}}
	fail := func(e error) (*Store, error) { s.Close(); return nil, e }
	if err = root.Mkdir(metaDir, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fail(err)
	}
	info, err := root.Lstat(metaDir)
	if err != nil {
		return fail(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fail(errors.New("评论目录必须为真实文件夹"))
	}
	lock, err := root.OpenFile(metaDir+"/lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return fail(err)
	}
	s.lock = lock
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fail(errors.New("此资料目录已由另一个 data-shelf 窗口打开"))
	}
	data, err := root.ReadFile(metaDir + "/metadata.json")
	if err == nil {
		if err = json.Unmarshal(data, &s.meta); err != nil {
			return fail(fmt.Errorf("评论数据损坏，已停止写入以保护原始数据: %w", err))
		}
		if s.meta.Version != 1 || s.meta.Records == nil {
			return fail(errors.New("不支持的评论数据版本"))
		}
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fail(err)
	}
	return s, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock != nil {
		s.lock.Close()
		s.lock = nil
	}
	return s.root.Close()
}
func validPath(path string) error {
	if path == "." {
		return nil
	}
	if !filepath.IsLocal(path) || filepath.Clean(path) != path {
		return errors.New("路径必须位于资料目录内")
	}
	for _, p := range strings.Split(path, string(filepath.Separator)) {
		if strings.HasPrefix(p, ".") {
			return errors.New("不支持隐藏文件或保留路径")
		}
	}
	return nil
}
func ValidName(name string) error {
	if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) || filepath.Base(name) != name || strings.ContainsAny(name, "/\\:\x00\n\r") || strings.HasPrefix(name, ".") {
		return errors.New("请输入有效名称，不包含路径分隔符或前后空格")
	}
	return nil
}
func (s *Store) check(path string) error {
	if err := validPath(path); err != nil {
		return err
	}
	parts := strings.Split(path, string(filepath.Separator))
	for i := range parts {
		p := filepath.Join(parts[:i+1]...)
		info, err := s.root.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("不跟随符号链接")
		}
	}
	return nil
}
func (s *Store) Tree() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var entries []Entry
	err := fs.WalkDir(s.root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		entries = append(entries, Entry{path, d.Name(), d.IsDir(), info.Size()})
		return nil
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, err
}
func (s *Store) read(path string) (string, error) {
	if err := s.check(path); err != nil {
		return "", err
	}
	f, err := s.root.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("请选择普通文件")
	}
	if info.Size() > MaxFileSize {
		return "", errors.New("文件超过 10 MB，未加载")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return "", err
	}
	if len(b) > MaxFileSize {
		return "", errors.New("文件超过 10 MB，未加载")
	}
	if !utf8.Valid(b) || strings.IndexByte(string(b), 0) >= 0 {
		return "", errors.New("仅支持 UTF-8 文本文件")
	}
	return string(b), nil
}
func (s *Store) Load(path string) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text, err := s.read(path)
	if err != nil {
		return Document{}, err
	}
	r := s.meta.Records[path]
	notes := append([]Annotation(nil), r.Notes...)
	hash := Hash(text)
	for i := range notes {
		n := &notes[i]
		if r.Hash != hash || n.Start < 0 || n.End > len(text) || n.Start >= n.End || text[n.Start:n.End] != n.Quote {
			n.Detached = true
		}
	}
	return Document{path, text, hash, notes}, nil
}
func (s *Store) atomic(path string, data []byte) error {
	tmp := filepath.Join(filepath.Dir(path), ".shelf-"+newID())
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove(tmp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return s.root.Rename(tmp, path)
}
func (s *Store) persist(m metadata) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err = s.atomic(metaDir+"/metadata.json", b); err != nil {
		return err
	}
	s.meta = m
	return nil
}
func (s *Store) copyMeta() metadata {
	m := metadata{Version: 1, Records: map[string]Record{}}
	for k, v := range s.meta.Records {
		v.Notes = append([]Annotation(nil), v.Notes...)
		m.Records[k] = v
	}
	return m
}
func (s *Store) Save(d Document) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(d.Text) > MaxFileSize {
		return d, errors.New("内容超过 10 MB，未保存")
	}
	if !utf8.ValidString(d.Text) {
		return d, errors.New("内容不是 UTF-8 文本")
	}
	current, err := s.read(d.Path)
	if err != nil {
		return d, err
	}
	if Hash(current) != d.Hash {
		return d, ErrConflict
	}
	if current != d.Text {
		if err = s.atomic(d.Path, []byte(d.Text)); err != nil {
			return d, err
		}
	}
	d.Hash = Hash(d.Text)
	m := s.copyMeta()
	m.Records[d.Path] = Record{d.Path, d.Hash, append([]Annotation(nil), d.Notes...)}
	if err = s.persist(m); err != nil {
		return d, fmt.Errorf("正文已保存，但评论保存失败: %w", err)
	}
	return d, nil
}
func (s *Store) Create(parent, name string, dir bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ValidName(name); err != nil {
		return "", err
	}
	path := filepath.Join(parent, name)
	if err := s.check(path); err != nil {
		return "", err
	}
	if dir {
		return path, s.root.Mkdir(path, 0700)
	}
	f, err := s.root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	return path, f.Close()
}
func (s *Store) Move(from, to string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if from == to {
		return nil
	}
	if from == "." || to == "." || strings.HasPrefix(to, from+string(filepath.Separator)) {
		return errors.New("不能将文件夹移动到自身内部")
	}
	if err := s.check(from); err != nil {
		return err
	}
	if err := s.check(to); err != nil {
		return err
	}
	if _, err := s.root.Lstat(to); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := s.root.Rename(from, to); err != nil {
		return err
	}
	m := s.copyMeta()
	for k, r := range s.meta.Records {
		if k == from || strings.HasPrefix(k, from+string(filepath.Separator)) {
			delete(m.Records, k)
			r.Path = to + strings.TrimPrefix(k, from)
			m.Records[r.Path] = r
		}
	}
	if err := s.persist(m); err != nil {
		rollback := s.root.Rename(to, from)
		return errors.Join(err, rollback)
	}
	return nil
}

// Orphans remain discoverable when Finder moves/removes annotated files.
func (s *Store) Orphans() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Record
	for path, r := range s.meta.Records {
		if len(r.Notes) == 0 {
			continue
		}
		if _, err := s.root.Stat(path); errors.Is(err, fs.ErrNotExist) {
			r.Notes = append([]Annotation(nil), r.Notes...)
			out = append(out, r)
		}
	}
	return out
}
