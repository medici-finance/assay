package evaluation

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// witness_json.go — the witness record's JSON, read and written without
// encoding/json. The library may import text, collection, hashing and
// formatting packages only (the purity boundary, statusgen/15), and
// encoding/json is reflection-driven, so the record grammar is parsed here.
//
// The reader is strict: one JSON value, no duplicate object keys, valid UTF-8,
// nothing but whitespace after the value. The canonical form a record id is
// the SHA-256 of is compact JSON with object keys sorted, strings escaped as
// encoding/json escapes them with HTML escaping off (`"`, `\`, control
// characters, U+2028 and U+2029), and numbers kept as their literal text.

// jsonNumber is a JSON number kept as its literal text.
type jsonNumber string

var jsonNumberRe = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

type jsonReader struct {
	s   string
	pos int
}

// decodeJSON parses raw into map[string]any, []any, string, bool, jsonNumber
// or nil.
func decodeJSON(raw []byte) (any, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("not valid UTF-8")
	}
	r := &jsonReader{s: string(raw)}
	v, err := r.value(0)
	if err != nil {
		return nil, err
	}
	r.space()
	if r.pos != len(r.s) {
		return nil, errors.New("trailing data after the value")
	}
	return v, nil
}

func (r *jsonReader) space() {
	for r.pos < len(r.s) {
		switch r.s[r.pos] {
		case ' ', '\t', '\n', '\r':
			r.pos++
		default:
			return
		}
	}
}

func (r *jsonReader) errf(format string, a ...any) error {
	return fmt.Errorf("offset %d: %s", r.pos, fmt.Sprintf(format, a...))
}

func (r *jsonReader) value(depth int) (any, error) {
	if depth > 32 {
		return nil, r.errf("nested too deeply")
	}
	r.space()
	if r.pos >= len(r.s) {
		return nil, r.errf("unexpected end of input")
	}
	switch c := r.s[r.pos]; {
	case c == '{':
		return r.object(depth)
	case c == '[':
		return r.array(depth)
	case c == '"':
		return r.str()
	case strings.HasPrefix(r.s[r.pos:], "true"):
		r.pos += 4
		return true, nil
	case strings.HasPrefix(r.s[r.pos:], "false"):
		r.pos += 5
		return false, nil
	case strings.HasPrefix(r.s[r.pos:], "null"):
		r.pos += 4
		return nil, nil
	case c == '-' || (c >= '0' && c <= '9'):
		start := r.pos
		for r.pos < len(r.s) && strings.IndexByte("+-.eE0123456789", r.s[r.pos]) >= 0 {
			r.pos++
		}
		n := r.s[start:r.pos]
		if !jsonNumberRe.MatchString(n) {
			return nil, r.errf("malformed number %q", n)
		}
		return jsonNumber(n), nil
	default:
		return nil, r.errf("unexpected character %q", c)
	}
}

func (r *jsonReader) object(depth int) (any, error) {
	r.pos++ // {
	obj := map[string]any{}
	r.space()
	if r.pos < len(r.s) && r.s[r.pos] == '}' {
		r.pos++
		return obj, nil
	}
	for {
		r.space()
		if r.pos >= len(r.s) || r.s[r.pos] != '"' {
			return nil, r.errf("an object key must be a string")
		}
		k, err := r.str()
		if err != nil {
			return nil, err
		}
		if _, dup := obj[k]; dup {
			return nil, r.errf("duplicate key %q", k)
		}
		r.space()
		if r.pos >= len(r.s) || r.s[r.pos] != ':' {
			return nil, r.errf("expected ':' after key %q", k)
		}
		r.pos++
		v, err := r.value(depth + 1)
		if err != nil {
			return nil, err
		}
		obj[k] = v
		r.space()
		if r.pos >= len(r.s) {
			return nil, r.errf("unterminated object")
		}
		switch r.s[r.pos] {
		case ',':
			r.pos++
		case '}':
			r.pos++
			return obj, nil
		default:
			return nil, r.errf("expected ',' or '}' in an object")
		}
	}
}

func (r *jsonReader) array(depth int) (any, error) {
	r.pos++ // [
	arr := []any{}
	r.space()
	if r.pos < len(r.s) && r.s[r.pos] == ']' {
		r.pos++
		return arr, nil
	}
	for {
		v, err := r.value(depth + 1)
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
		r.space()
		if r.pos >= len(r.s) {
			return nil, r.errf("unterminated array")
		}
		switch r.s[r.pos] {
		case ',':
			r.pos++
		case ']':
			r.pos++
			return arr, nil
		default:
			return nil, r.errf("expected ',' or ']' in an array")
		}
	}
}

func (r *jsonReader) str() (string, error) {
	r.pos++ // opening quote
	var b strings.Builder
	for {
		if r.pos >= len(r.s) {
			return "", r.errf("unterminated string")
		}
		c := r.s[r.pos]
		switch {
		case c == '"':
			r.pos++
			return b.String(), nil
		case c < 0x20:
			return "", r.errf("an unescaped control character in a string")
		case c != '\\':
			b.WriteByte(c)
			r.pos++
			continue
		}
		r.pos++ // backslash
		if r.pos >= len(r.s) {
			return "", r.errf("unterminated escape")
		}
		e := r.s[r.pos]
		r.pos++
		switch e {
		case '"', '\\', '/':
			b.WriteByte(e)
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			cp, err := r.hex4()
			if err != nil {
				return "", err
			}
			if cp >= 0xD800 && cp < 0xDC00 {
				if !strings.HasPrefix(r.s[r.pos:], `\u`) {
					return "", r.errf("an unpaired surrogate")
				}
				r.pos += 2
				lo, err := r.hex4()
				if err != nil {
					return "", err
				}
				if lo < 0xDC00 || lo > 0xDFFF {
					return "", r.errf("an unpaired surrogate")
				}
				cp = 0x10000 + (cp-0xD800)<<10 + (lo - 0xDC00)
			} else if cp >= 0xDC00 && cp <= 0xDFFF {
				return "", r.errf("an unpaired surrogate")
			}
			b.WriteRune(rune(cp))
		default:
			return "", r.errf("an unknown escape \\%c", e)
		}
	}
}

func (r *jsonReader) hex4() (int, error) {
	if r.pos+4 > len(r.s) {
		return 0, r.errf("a short \\u escape")
	}
	n, err := strconv.ParseUint(r.s[r.pos:r.pos+4], 16, 32)
	if err != nil {
		return 0, r.errf("a malformed \\u escape")
	}
	r.pos += 4
	return int(n), nil
}

// encodeCanonicalJSON renders a decoded value in the canonical form.
func encodeCanonicalJSON(v any) (string, error) {
	var b strings.Builder
	if err := writeCanonical(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

func writeCanonical(b *strings.Builder, v any) error {
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case jsonNumber:
		b.WriteString(string(t))
	case string:
		writeJSONString(b, t)
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeCanonical(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSONString(b, k)
			b.WriteByte(':')
			if err := writeCanonical(b, t[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("a value of type %T has no JSON form", v)
	}
	return nil
}

func writeJSONString(b *strings.Builder, s string) {
	const hexDigits = "0123456789abcdef"
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x2028 || r == 0x2029:
			b.WriteString(`\u`)
			b.WriteByte(hexDigits[(r>>12)&0xf])
			b.WriteByte(hexDigits[(r>>8)&0xf])
			b.WriteByte(hexDigits[(r>>4)&0xf])
			b.WriteByte(hexDigits[r&0xf])
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}

// ---------------------------------------------------------------------------
// The record's fields
// ---------------------------------------------------------------------------

func jsonStringField(obj map[string]any, k string) (string, error) {
	s, ok := obj[k].(string)
	if !ok {
		return "", fmt.Errorf("a record field has the wrong type: %q is not a string", k)
	}
	return s, nil
}

func jsonStringMapField(obj map[string]any, k string) (map[string]string, error) {
	m, ok := obj[k].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("a record field has the wrong type: %q is not an object", k)
	}
	out := make(map[string]string, len(m))
	for mk, mv := range m {
		s, ok := mv.(string)
		if !ok {
			return nil, fmt.Errorf("a record field has the wrong type: %q.%q is not a string", k, mk)
		}
		out[mk] = s
	}
	return out, nil
}

// recordFromObject reads the record's typed fields from its decoded object;
// the key set has already been checked.
func recordFromObject(obj map[string]any) (WitnessRecord, error) {
	var rec WitnessRecord
	n, ok := obj["schema"].(jsonNumber)
	if !ok {
		return rec, errors.New("a record field has the wrong type: \"schema\" is not a number")
	}
	schema, err := strconv.Atoi(string(n))
	if err != nil {
		return rec, fmt.Errorf("a record field has the wrong type: schema %s is not an integer", n)
	}
	rec.Schema = schema
	for _, f := range []struct {
		key string
		dst *string
	}{
		{"brief", &rec.Brief}, {"row", &rec.Row}, {"command_sha256", &rec.CommandSHA256},
		{"expect_sha256", &rec.ExpectSHA256}, {"class_sha256", &rec.ClassSHA256}, {"runner", &rec.Runner},
		{"runner_source", &rec.RunnerSource}, {"runner_class", &rec.RunnerClass}, {"network", &rec.Network},
		{"tools_source", &rec.ToolsSource}, {"origin", &rec.Origin}, {"result", &rec.Result}, {"exit", &rec.Exit},
		{"output_sha256", &rec.OutputSHA256}, {"date", &rec.Date}, {"cr", &rec.CR}, {"scope", &rec.Scope},
		{"tree_digest", &rec.TreeDigest},
	} {
		if *f.dst, err = jsonStringField(obj, f.key); err != nil {
			return rec, err
		}
	}
	for _, k := range witnessRecordBackfillKeys {
		if _, has := obj[k]; !has {
			continue
		}
		s, err := jsonStringField(obj, k)
		if err != nil {
			return rec, err
		}
		if k == "source_cr" {
			rec.SourceCR = s
		} else {
			rec.SourceCell = s
		}
	}
	for _, f := range []struct {
		key string
		dst *map[string]string
	}{{"tools", &rec.Tools}, {"inputs", &rec.Inputs}, {"implicit", &rec.Implicit}, {"attributes", &rec.Attributes}} {
		if *f.dst, err = jsonStringMapField(obj, f.key); err != nil {
			return rec, err
		}
	}
	list, ok := obj["dirty_paths"].([]any)
	if !ok {
		return rec, errors.New("a record field has the wrong type: \"dirty_paths\" is not an array")
	}
	rec.DirtyPaths = []string{}
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			return rec, errors.New("a record field has the wrong type: a dirty path is not a string")
		}
		rec.DirtyPaths = append(rec.DirtyPaths, s)
	}
	info, ok := obj["informational"].(map[string]any)
	if !ok {
		return rec, errors.New("a record field has the wrong type: \"informational\" is not an object")
	}
	for k := range info {
		if k != "commit" && k != "dirty" {
			return rec, fmt.Errorf("the record carries the unknown field informational.%q", k)
		}
	}
	if rec.Informational.Commit, err = jsonStringField(info, "commit"); err != nil {
		return rec, err
	}
	dirty, ok := info["dirty"].(bool)
	if !ok {
		return rec, errors.New("a record field has the wrong type: \"informational.dirty\" is not a boolean")
	}
	rec.Informational.Dirty = dirty
	return rec, nil
}

// recordToObject is recordFromObject's inverse: the decoded form of a record
// (source_cr and source_cell omitted when empty).
func recordToObject(rec WitnessRecord) map[string]any {
	strMap := func(m map[string]string) map[string]any {
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	dirty := make([]any, 0, len(rec.DirtyPaths))
	for _, p := range rec.DirtyPaths {
		dirty = append(dirty, p)
	}
	obj := map[string]any{
		"schema": jsonNumber(strconv.Itoa(rec.Schema)), "brief": rec.Brief, "row": rec.Row,
		"command_sha256": rec.CommandSHA256, "expect_sha256": rec.ExpectSHA256, "class_sha256": rec.ClassSHA256,
		"runner": rec.Runner, "runner_source": rec.RunnerSource, "runner_class": rec.RunnerClass,
		"network": rec.Network, "tools": strMap(rec.Tools), "tools_source": rec.ToolsSource, "origin": rec.Origin,
		"result": rec.Result, "exit": rec.Exit, "output_sha256": rec.OutputSHA256, "date": rec.Date, "cr": rec.CR,
		"scope": rec.Scope, "inputs": strMap(rec.Inputs), "implicit": strMap(rec.Implicit),
		"attributes": strMap(rec.Attributes), "tree_digest": rec.TreeDigest, "dirty_paths": dirty,
		"informational": map[string]any{"commit": rec.Informational.Commit, "dirty": rec.Informational.Dirty},
	}
	if rec.SourceCR != "" {
		obj["source_cr"] = rec.SourceCR
	}
	if rec.SourceCell != "" {
		obj["source_cell"] = rec.SourceCell
	}
	return obj
}
