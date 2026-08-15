package topology

import (
	"fmt"
	"strconv"
	"strings"
)

// yaml.go — a deliberately SMALL, deliberately STRICT reader for the exact YAML
// subset topology.yaml is written in. tools/desk is a dependency-free module
// (see tools/desk/go.mod: no `require` lines at all), and adding gopkg.in/yaml.v3
// to the module that holds the desk's security gates to read one inventory file
// is a supply-chain trade this package declines to make. The same
// documented-duplicate reasoning already governs citrigger_test.go's hand-rolled
// workflow reader.
//
// STRICTNESS IS THE POINT. Where this reader meets a shape it does not model it
// returns an ERROR naming the line, never a partial value. A loader that quietly
// skips what it did not understand turns "I could not read the source" into "the
// source says nothing here", and a drift check built on that reports green for a
// file it never parsed. Every caller must treat an error as COULD-NOT-CHECK.
//
// Modelled subset, and nothing else:
//
//	key: scalar          # trailing comments stripped outside quotes
//	key: "quoted scalar"
//	key: []              # empty inline sequence ONLY
//	key:                 # nested mapping or sequence on following lines
//	key: >-              # folded block scalar (also |, |-, >)
//	- scalar             # sequence of scalars
//	- key: v             # sequence of mappings
//
// NOT modelled, and therefore an error: inline maps/sequences with content,
// anchors, aliases, tags, multiple documents, flow style, tabs for indentation.

// node is the parsed shape of one YAML value: exactly one of the three fields is
// populated. It is unexported because callers consume the typed Topology, not
// this tree.
type node struct {
	scalar string
	mapv   map[string]*node
	seq    []*node
	// order preserves mapping key order for deterministic error messages.
	order []string
	// line is the 1-based source line the value started on, for error text.
	line int
}

func (n *node) isScalar() bool { return n != nil && n.mapv == nil && n.seq == nil }

// lookup returns the child value for key, or nil.
func (n *node) lookup(key string) *node {
	if n == nil || n.mapv == nil {
		return nil
	}
	return n.mapv[key]
}

// str returns a scalar child as a string. ok is false when the key is absent.
// A key present but NOT a scalar is an error, not a silent "".
func (n *node) str(key string) (string, bool, error) {
	c := n.lookup(key)
	if c == nil {
		return "", false, nil
	}
	if !c.isScalar() {
		return "", false, fmt.Errorf("line %d: %q is a collection, expected a scalar", c.line, key)
	}
	return c.scalar, true, nil
}

// boolAt returns a scalar child parsed as a bool. Only the exact YAML 1.2 core
// forms `true`/`false` parse; anything else is an error rather than a guess.
func (n *node) boolAt(key string) (val bool, present bool, err error) {
	c := n.lookup(key)
	if c == nil {
		return false, false, nil
	}
	if !c.isScalar() {
		return false, false, fmt.Errorf("line %d: %q is a collection, expected a boolean", c.line, key)
	}
	switch c.scalar {
	case "true":
		return true, true, nil
	case "false":
		return false, true, nil
	}
	return false, false, fmt.Errorf("line %d: %q = %q is not a boolean (only `true`/`false` parse)", c.line, key, c.scalar)
}

// strSeq returns a child sequence of scalars. ok is false when the key is absent.
func (n *node) strSeq(key string) (out []string, ok bool, err error) {
	c := n.lookup(key)
	if c == nil {
		return nil, false, nil
	}
	if c.isScalar() {
		return nil, false, fmt.Errorf("line %d: %q is a scalar, expected a sequence", c.line, key)
	}
	if c.mapv != nil {
		return nil, false, fmt.Errorf("line %d: %q is a mapping, expected a sequence", c.line, key)
	}
	for _, item := range c.seq {
		if !item.isScalar() {
			return nil, false, fmt.Errorf("line %d: sequence %q holds a collection where a scalar was expected", item.line, key)
		}
		out = append(out, item.scalar)
	}
	return out, true, nil
}

// mapSeq returns a child sequence of mappings.
func (n *node) mapSeq(key string) (out []*node, ok bool, err error) {
	c := n.lookup(key)
	if c == nil {
		return nil, false, nil
	}
	if c.isScalar() {
		return nil, false, fmt.Errorf("line %d: %q is a scalar, expected a sequence of mappings", c.line, key)
	}
	if c.mapv != nil {
		return nil, false, fmt.Errorf("line %d: %q is a mapping, expected a sequence of mappings", c.line, key)
	}
	for _, item := range c.seq {
		if item.mapv == nil {
			return nil, false, fmt.Errorf("line %d: sequence %q holds a non-mapping item", item.line, key)
		}
		out = append(out, item)
	}
	return out, true, nil
}

// srcLine is one physical line with its 1-based number and computed indent.
type srcLine struct {
	num    int
	indent int
	text   string // indentation stripped, trailing whitespace stripped
	raw    string // the line as written, minus the trailing newline
}

// parseYAML parses the modelled subset into a root mapping node.
func parseYAML(data []byte) (*node, error) {
	raw := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := make([]srcLine, 0, len(raw))
	for i, l := range raw {
		if strings.ContainsRune(l, '\t') && strings.TrimSpace(l) != "" {
			return nil, fmt.Errorf("line %d: TAB in indentation — this reader models spaces only", i+1)
		}
		lines = append(lines, srcLine{
			num:    i + 1,
			indent: len(l) - len(strings.TrimLeft(l, " ")),
			text:   strings.TrimRight(strings.TrimLeft(l, " "), " "),
			raw:    l,
		})
	}
	p := &parser{lines: lines}
	// Skip leading comments/blanks, then parse the document body at indent 0.
	p.skipIgnorable()
	if p.done() {
		return nil, fmt.Errorf("topology source is empty — a reader that parsed nothing knows nothing")
	}
	root, err := p.parseBlock(0)
	if err != nil {
		return nil, err
	}
	p.skipIgnorable()
	if !p.done() {
		l := p.lines[p.i]
		return nil, fmt.Errorf("line %d: unconsumed content %q (dedent below the document root)", l.num, l.text)
	}
	if root.mapv == nil {
		return nil, fmt.Errorf("topology source root must be a mapping")
	}
	return root, nil
}

type parser struct {
	lines []srcLine
	i     int
}

func (p *parser) done() bool { return p.i >= len(p.lines) }

// skipIgnorable advances past blank lines and whole-line comments.
func (p *parser) skipIgnorable() {
	for !p.done() {
		t := p.lines[p.i].text
		if t == "" || strings.HasPrefix(t, "#") {
			p.i++
			continue
		}
		return
	}
}

// parseBlock parses every construct at exactly `indent`, returning a mapping or
// a sequence node.
func (p *parser) parseBlock(indent int) (*node, error) {
	p.skipIgnorable()
	if p.done() {
		return nil, fmt.Errorf("unexpected end of source while reading a block at indent %d", indent)
	}
	if strings.HasPrefix(p.lines[p.i].text, "- ") || p.lines[p.i].text == "-" {
		return p.parseSeq(indent)
	}
	return p.parseMap(indent)
}

func (p *parser) parseSeq(indent int) (*node, error) {
	out := &node{seq: []*node{}, line: p.lines[p.i].num}
	for {
		p.skipIgnorable()
		if p.done() {
			return out, nil
		}
		l := p.lines[p.i]
		if l.indent < indent {
			return out, nil
		}
		if l.indent > indent {
			return nil, fmt.Errorf("line %d: unexpected indent %d inside a sequence at indent %d", l.num, l.indent, indent)
		}
		if !strings.HasPrefix(l.text, "- ") && l.text != "-" {
			return out, nil
		}
		rest := strings.TrimSpace(strings.TrimPrefix(l.text, "-"))
		if rest == "" {
			return nil, fmt.Errorf("line %d: bare `-` (a sequence item on its own line) is not modelled", l.num)
		}
		// A sequence item is either `- scalar` or `- key: value` (a mapping whose
		// first key shares the dash's line). Distinguish on a top-level colon.
		if k, v, isKV := splitKey(rest); isKV {
			// Rewrite the item's first line as a mapping entry at indent+2 and
			// let parseMap consume it together with the item's later keys.
			item := &node{mapv: map[string]*node{}, line: l.num}
			p.i++
			child, err := p.parseMapEntry(indent+2, k, v, l.num)
			if err != nil {
				return nil, err
			}
			item.mapv[k] = child
			item.order = append(item.order, k)
			if err := p.parseMapInto(item, indent+2); err != nil {
				return nil, err
			}
			out.seq = append(out.seq, item)
			continue
		}
		s, err := scalarValue(stripComment(rest), l.num)
		if err != nil {
			return nil, err
		}
		out.seq = append(out.seq, &node{scalar: s, line: l.num})
		p.i++
	}
}

func (p *parser) parseMap(indent int) (*node, error) {
	out := &node{mapv: map[string]*node{}, line: p.lines[p.i].num}
	if err := p.parseMapInto(out, indent); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *parser) parseMapInto(out *node, indent int) error {
	for {
		p.skipIgnorable()
		if p.done() {
			return nil
		}
		l := p.lines[p.i]
		if l.indent < indent {
			return nil
		}
		if l.indent > indent {
			return fmt.Errorf("line %d: unexpected indent %d inside a mapping at indent %d", l.num, l.indent, indent)
		}
		if strings.HasPrefix(l.text, "- ") || l.text == "-" {
			// A sequence item at the mapping's own indent ends this mapping.
			return nil
		}
		k, v, isKV := splitKey(l.text)
		if !isKV {
			return fmt.Errorf("line %d: %q is neither `key: value` nor a sequence item", l.num, l.text)
		}
		if _, dup := out.mapv[k]; dup {
			return fmt.Errorf("line %d: duplicate key %q — a duplicate is a silent overwrite, refused", l.num, k)
		}
		p.i++
		child, err := p.parseMapEntry(indent+2, k, v, l.num)
		if err != nil {
			return err
		}
		out.mapv[k] = child
		out.order = append(out.order, k)
	}
}

// parseMapEntry resolves the VALUE of `key: v` whose key line has already been
// consumed. childIndent is the indent nested content must sit at (or deeper, for
// block scalars).
func (p *parser) parseMapEntry(childIndent int, key, v string, keyLine int) (*node, error) {
	switch {
	case v == "":
		// Nested block. It may legitimately be absent (a key with no children is
		// an error here — every mapping key in this schema either has a scalar or
		// a block).
		p.skipIgnorable()
		if p.done() {
			return nil, fmt.Errorf("line %d: key %q has no value and no nested block", keyLine, key)
		}
		next := p.lines[p.i]
		// A sequence may sit at the KEY's indent (YAML permits it) or deeper.
		if next.indent < childIndent-2 {
			return nil, fmt.Errorf("line %d: key %q has no value and no nested block", keyLine, key)
		}
		seqIndent := next.indent
		if strings.HasPrefix(next.text, "- ") || next.text == "-" {
			return p.parseSeq(seqIndent)
		}
		if next.indent != childIndent {
			return nil, fmt.Errorf("line %d: nested block under %q is indented %d, want %d",
				next.num, key, next.indent, childIndent)
		}
		return p.parseMap(childIndent)
	case v == "[]":
		return &node{seq: []*node{}, line: keyLine}, nil
	case v == ">-" || v == ">" || v == "|" || v == "|-" || v == "|+" || v == ">+":
		return p.parseBlockScalar(v, childIndent, key, keyLine)
	default:
		if strings.HasPrefix(v, "[") || strings.HasPrefix(v, "{") {
			return nil, fmt.Errorf("line %d: flow-style value %q under %q is not modelled", keyLine, v, key)
		}
		s, err := scalarValue(v, keyLine)
		if err != nil {
			return nil, err
		}
		return &node{scalar: s, line: keyLine}, nil
	}
}

// parseBlockScalar consumes an indented block scalar. Folded styles (`>`) join
// lines with a single space; literal styles (`|`) join with newlines. Chomping
// is normalised: this reader always strips trailing whitespace, because every
// consumer of these fields compares trimmed prose.
func (p *parser) parseBlockScalar(style string, childIndent int, key string, keyLine int) (*node, error) {
	folded := strings.HasPrefix(style, ">")
	var parts []string
	for !p.done() {
		l := p.lines[p.i]
		if l.text == "" {
			// A blank line inside a block scalar is a paragraph break; a blank
			// line after it is just spacing. Look ahead to tell them apart.
			if p.blockContinues(childIndent) {
				parts = append(parts, "")
				p.i++
				continue
			}
			break
		}
		if l.indent < childIndent {
			break
		}
		parts = append(parts, strings.Repeat(" ", l.indent-childIndent)+l.text)
		p.i++
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("line %d: block scalar under %q has no content", keyLine, key)
	}
	sep := "\n"
	if folded {
		sep = " "
	}
	return &node{scalar: strings.TrimSpace(strings.Join(parts, sep)), line: keyLine}, nil
}

// blockContinues reports whether, past the blank line at p.i, more block-scalar
// content sits at or beyond childIndent.
func (p *parser) blockContinues(childIndent int) bool {
	for j := p.i; j < len(p.lines); j++ {
		if p.lines[j].text == "" {
			continue
		}
		return p.lines[j].indent >= childIndent
	}
	return false
}

// splitKey splits `key: value` on the FIRST colon that is followed by a space or
// ends the line and that is not inside quotes. isKV is false when the text holds
// no such colon.
func splitKey(text string) (key, value string, isKV bool) {
	inSingle, inDouble := false, false
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if inSingle || inDouble {
				continue
			}
			if i == len(text)-1 {
				return strings.TrimSpace(text[:i]), "", true
			}
			if text[i+1] == ' ' {
				return strings.TrimSpace(text[:i]), stripComment(strings.TrimSpace(text[i+1:])), true
			}
		}
	}
	return "", "", false
}

// stripComment removes a trailing ` #…` comment that is not inside quotes.
func stripComment(v string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if inSingle || inDouble {
				continue
			}
			if i == 0 || v[i-1] == ' ' {
				return strings.TrimSpace(v[:i])
			}
		}
	}
	return strings.TrimSpace(v)
}

// scalarValue unquotes a scalar. Only fully-quoted forms are unquoted; a partial
// quote is an error rather than a value with stray quotes in it.
func scalarValue(v string, line int) (string, error) {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		out, err := strconv.Unquote(v)
		if err != nil {
			return "", fmt.Errorf("line %d: %q is not a valid double-quoted scalar: %v", line, v, err)
		}
		return out, nil
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'"), nil
	}
	if strings.HasPrefix(v, "\"") || strings.HasPrefix(v, "'") {
		return "", fmt.Errorf("line %d: unterminated quote in scalar %q", line, v)
	}
	return v, nil
}
