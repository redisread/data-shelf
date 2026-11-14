package shelf

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Import copies into a staging area and publishes only a complete import.
func (s *Store) Import(source, parent string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	source, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	if source == s.Path || strings.HasPrefix(s.Path, source+string(filepath.Separator)) {
		return "", errors.New("不能导入资料目录或其父目录")
	}
	info, err := os.Lstat(source)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("不导入符号链接")
	}
	name := filepath.Base(source)
	if err := ValidName(name); err != nil {
		return "", err
	}
	dest := filepath.Join(parent, name)
	if err := s.check(dest); err != nil {
		return "", err
	}
	if _, err := s.root.Lstat(dest); err == nil {
		return "", fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	stage := metaDir + "/import-" + newID()
	defer s.root.RemoveAll(stage)
	copyFile := func(src, target string) error {
		f, e := os.Open(src)
		if e != nil {
			return e
		}
		defer f.Close()
		stat, e := f.Stat()
		if e != nil {
			return e
		}
		if !stat.Mode().IsRegular() || stat.Size() > MaxFileSize {
			return errors.New("导入仅支持 10 MB 内的普通文件")
		}
		data, e := readImport(f)
		if e != nil {
			return e
		}
		return s.atomic(target, data)
	}
	if info.IsDir() {
		if err = s.root.Mkdir(stage, 0700); err != nil {
			return "", err
		}
		count := 0
		err = filepath.WalkDir(source, func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if path == source {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			count++
			if count > 10000 {
				return errors.New("单次导入最多 10000 个文件和文件夹")
			}
			if d.Type()&os.ModeSymlink != 0 {
				return errors.New("文件夹包含符号链接，未导入")
			}
			rel, e := filepath.Rel(source, path)
			if e != nil {
				return e
			}
			if e = validPath(rel); e != nil {
				return e
			}
			target := filepath.Join(stage, rel)
			if d.IsDir() {
				return s.root.Mkdir(target, 0700)
			}
			return copyFile(path, target)
		})
	} else {
		err = copyFile(source, stage)
	}
	if err != nil {
		return "", err
	}
	if err = s.root.Rename(stage, dest); err != nil {
		return "", err
	}
	m := s.copyMeta()
	if retire(&m, dest) {
		if err = s.persist(m); err != nil {
			return "", errors.Join(err, s.root.Rename(dest, stage))
		}
	}
	return dest, nil
}
