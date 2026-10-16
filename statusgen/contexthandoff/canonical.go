package contexthandoff

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// Canonical returns the canonical serialization of a decoded handoff: the
// typed record (so every brief reference is already the four-segment form)
// written as JSON with object keys in byte order, no insignificant whitespace,
// and a fixed string escaping. Two inputs that differ only in key order,
// whitespace, string escaping or elided-versus-full brief references produce
// identical canonical bytes. The record holds no numbers, so number
// formatting never enters the form.
func (h *Handoff) Canonical() []byte { return canonical(h) }

// Digest is "sha256:" and the lower-case hex sha256 of Canonical().
func (h *Handoff) Digest() string { return digest(h.Canonical()) }

// Canonical returns the canonical serialization of a decoded manifest, by the
// same rules as Handoff.Canonical.
func (m *Manifest) Canonical() []byte { return canonical(m) }

// Digest is "sha256:" and the lower-case hex sha256 of Canonical().
func (m *Manifest) Digest() string { return digest(m.Canonical()) }

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func canonical(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		// The records are plain strings, booleans, slices and pointers to
		// them; marshalling cannot fail.
		panic("contexthandoff: marshal decoded record: " + err.Error())
	}
	tree, perr := parseStrict(raw)
	if perr != nil {
		panic("contexthandoff: re-read marshalled record: " + perr.Error())
	}
	var buf bytes.Buffer
	writeCanonical(&buf, tree)
	return buf.Bytes()
}

func writeCanonical(buf *bytes.Buffer, v any) {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		writeString(buf, t)
	case json.Number:
		buf.WriteString(t.String())
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonical(buf, e)
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeString(buf, k)
			buf.WriteByte(':')
			writeCanonical(buf, t[k])
		}
		buf.WriteByte('}')
	}
}

// writeString escapes only what JSON requires: the quote, the backslash and
// control characters below U+0020 (the five with short forms use them, the
// rest \u00xx in lower-case hex). Every other character is written as UTF-8.
func writeString(buf *bytes.Buffer, s string) {
	const hexdigits = "0123456789abcdef"
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				buf.WriteString(`\u00`)
				buf.WriteByte(hexdigits[r>>4])
				buf.WriteByte(hexdigits[r&0xf])
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}
