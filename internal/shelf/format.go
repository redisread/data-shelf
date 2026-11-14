package shelf

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
)

type Format string

const (
	Plain       Format = "文本"
	JSON        Format = "JSON"
	Markdown    Format = "Markdown"
	MaxFileSize        = 10 * 1024 * 1024
)

// Detect respects an explicit extension before inspecting the content.
func Detect(name, text string) Format {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json":
		return JSON
	case ".md", ".markdown":
		return Markdown
	case "":
		if json.Valid([]byte(text)) {
			return JSON
		}
	}
	return Plain
}

// FormatJSON preserves number lexemes, key order and duplicate object keys.
func FormatJSON(text string) (string, error) {
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(text), "", "  "); err != nil {
		return "", err
	}
	return out.String(), nil
}
