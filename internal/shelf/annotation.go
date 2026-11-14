package shelf

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"
)

type Annotation struct {
	ID       string `json:"id"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Quote    string `json:"quote"`
	Comment  string `json:"comment"`
	Detached bool   `json:"detached,omitempty"`
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func NewAnnotation(text string, start, end int, comment string) (Annotation, error) {
	if start < 0 || end > len(text) || start >= end || !utf8.ValidString(text[start:end]) {
		return Annotation{}, errors.New("请先选择一段完整文字")
	}
	return Annotation{ID: newID(), Start: start, End: end, Quote: text[start:end], Comment: comment}, nil
}

// Reanchor only moves an untouched selection through an unambiguous single
// changed region. Overlapping changes detach; old quotes are never guessed at.
func Reanchor(old, text string, notes []Annotation) []Annotation {
	out := append([]Annotation(nil), notes...)
	if old == text {
		return out
	}
	prefix := 0
	for prefix < len(old) && prefix < len(text) && old[prefix] == text[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(text)-prefix && old[len(old)-suffix-1] == text[len(text)-suffix-1] {
		suffix++
	}
	for i := range out {
		n := &out[i]
		// Without an edit-event log, repeated quotes cannot prove identity.
		// Prefer a detached note to silently attaching it to a surviving copy.
		if strings.Index(old, n.Quote) != strings.LastIndex(old, n.Quote) || strings.Index(text, n.Quote) != strings.LastIndex(text, n.Quote) {
			n.Detached = true
		}
		if n.Detached {
			continue
		}
		switch {
		case n.End <= prefix:
		case n.Start >= len(old)-suffix:
			n.Start += len(text) - len(old)
			n.End += len(text) - len(old)
		default:
			n.Detached = true
		}
		if !n.Detached && (n.Start < 0 || n.End > len(text) || text[n.Start:n.End] != n.Quote) {
			n.Detached = true
		}
	}
	return out
}

// Formatting changes only insignificant JSON whitespace. Map the exact token
// bytes to retain annotations across pretty-printing, including string spaces.
func ReanchorFormatted(old, text string, notes []Annotation) []Annotation {
	compact := func(s string) (string, []int) {
		var b strings.Builder
		var positions []int
		quoted, escaped := false, false
		for i := 0; i < len(s); i++ {
			ch := s[i]
			if quoted || !strings.ContainsRune(" \t\n\r", rune(ch)) {
				b.WriteByte(ch)
				positions = append(positions, i)
			}
			if quoted {
				if escaped {
					escaped = false
				} else if ch == '\\' {
					escaped = true
				} else if ch == '"' {
					quoted = false
				}
			} else if ch == '"' {
				quoted = true
			}
		}
		return b.String(), positions
	}
	a, ap := compact(old)
	b, bp := compact(text)
	if a != b {
		return Reanchor(old, text, notes)
	}
	out := append([]Annotation(nil), notes...)
	for i := range out {
		n := &out[i]
		if n.Detached {
			continue
		}
		first, last := -1, -1
		for j, p := range ap {
			if p >= n.Start && p < n.End {
				if first < 0 {
					first = j
				}
				last = j
			}
		}
		if first < 0 {
			n.Detached = true
			continue
		}
		n.Start = bp[first]
		n.End = bp[last] + 1
		n.Quote = text[n.Start:n.End]
	}
	return out
}

// CopyAnnotations preserves archived originals while giving the destination
// independent identities and requiring an explicit new text selection.
func CopyAnnotations(notes []Annotation) []Annotation {
	out := append([]Annotation(nil), notes...)
	for i := range out {
		out[i].ID = newID()
		out[i].Detached = true
	}
	return out
}
