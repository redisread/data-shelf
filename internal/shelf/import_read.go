package shelf

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

func readImport(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxFileSize {
		return nil, errors.New("文件超过 10 MB")
	}
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return nil, errors.New("导入仅支持 UTF-8 文本")
	}
	return b, nil
}
