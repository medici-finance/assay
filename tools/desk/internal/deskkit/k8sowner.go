package deskkit

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// k8sOwnerIndex answers ONE question for the decrypted-k8s-secret rule (k8sLine): does a
// YAML parse PROVE that the data/stringData mapping on a given line belongs to an object
// that is not a Secret? Only a yes narrows the rule. Every other answer — the text does
// not parse, the line sits in a diff hunk that is not a whole new file, the owner has no
// kind or more than one, a Secret encloses it — is could-not-check, and the mapping is read
// exactly as it was before ownership was considered.
//
// Why it exists. The rule arms on a `kind: Secret` ANYWHERE in the text, and deskpr hands
// it a whole multi-file branch diff as one string, so one correctly sops-encrypted Secret
// beside any ConfigMap refused on the ConfigMap's plaintext `data:` with no way through
// but an audited override.
//
// Why a parse, not line segmentation. An earlier draft split the text at `---`, fence and
// hunk-header lines and compared key columns. Inside one YAML document those lines can be
// block-scalar, quoted-scalar or flow-collection content, and in a List the next item's
// `kind:` sits at exactly the column of the previous item's `data:`; both admitted
// decrypted Secret values the pre-fix rule refused. Ownership here is the parent mapping
// of the parsed key node, nothing else.
//
// A text is read in every way it could be meant, and each applicable READING must prove
// the owner independently:
//
//   - a DIFF reading, when any line is a unified-diff hunk header. Each header covers the
//     lines its range counts. Only a new-file hunk (`@@ -0,0 +1,N @@`) is parsed, as the
//     whole file it is — with its `+` markers removed when every line still carries one
//     (a diff quoted in a body), as-is on deskpr's own stripDiffMetaLines surface. A
//     modification or deletion hunk is a fragment of a file and never proves an owner;
//     its reach is taken as old+new count, an over-estimate, so it can only cover more.
//     Every covering hunk must prove the owner, and a line no hunk covers (other than a
//     blank or git's no-newline note) makes the whole diff reading unprovable.
//   - a WHOLE-TEXT reading, when the entire text parses as a YAML stream (`---` and `...`
//     separators are the parser's, not a regex's).
//   - a FENCE reading of markdown code fences, paired the way CommonMark pairs them
//     (indent of at most three, same character, closer at least as long), each block
//     parsed on its own. Used only when the text is not a diff.
//
// One more condition binds every reading: every line that declares a Secret kind must
// be, in that same reading, the `kind` key of a parsed Secret mapping. A chunk boundary
// that falls inside an open scalar or flow collection leaves the chunk unparseable, so
// a Secret split across chunks to borrow another chunk's `kind:` switches the narrowing
// off for the whole text instead of passing its data.
//
// The cost of erring this way, stated rather than hidden: a mapping in a modification
// hunk always refuses beside a Secret elsewhere; so does a ConfigMap whose new-file hunk
// lies inside the over-estimated reach of a modification hunk just above it; and editing
// an EXISTING Secret (its `kind:` line in a modification hunk) beside a new ConfigMap
// keeps the pre-fix refusal for the whole diff.
type k8sOwnerIndex struct {
	readings []*k8sReading
}

// k8sReading is one way of reading the text: a set of chunks, each parsed on its own.
type k8sReading struct {
	chunks []*k8sChunk
	valid  bool
}

// k8sChunk is a run of lines [start,end) parsed as one YAML stream. ok is false when it
// did not parse or was never a candidate for parsing (a modification hunk).
type k8sChunk struct {
	start, end int
	ok         bool
	keys       map[int][]k8sKeyRec // absolute 0-based line -> scalar keys starting on it
}

// k8sKeyRec is one scalar mapping key as the parser saw it.
type k8sKeyRec struct {
	name string
	// otherOwner: the key's own mapping has exactly one `kind`, a plain identifier that
	// is not Secret; no merge key or complex key could add another; and no enclosing
	// mapping is a Secret.
	otherOwner bool
	// secretKind: the key is `kind` and its value is the string Secret.
	secretKind bool
}

var (
	reK8sHunkRange = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)
	reK8sKindName  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
	// reK8sSecretDecl is looser than reK8sSecretKind on purpose: any spelling that
	// declares a Secret kind on a line (flow style, a doubled marker) must be accounted
	// for by a parse before any mapping is narrowed.
	reK8sSecretDecl = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])"?kind"?\s*:\s*["']?Secret(?:[^A-Za-z0-9_]|$)`)
	reK8sFenceOpen  = regexp.MustCompile("^( {0,3})(`{3,}|~{3,})(.*)$")
)

func newK8sOwnerIndex(lines []string) *k8sOwnerIndex {
	idx := &k8sOwnerIndex{}
	diff := false
	for _, ln := range lines {
		if reK8sHunkRange.MatchString(ln) {
			diff = true
			break
		}
	}
	if diff {
		idx.readings = append(idx.readings, k8sDiffReading(lines))
	} else if r := k8sFenceReading(lines); r != nil {
		idx.readings = append(idx.readings, r)
	}
	if c := parseK8sChunk(lines, 0, len(lines)); c.ok {
		idx.readings = append(idx.readings, &k8sReading{chunks: []*k8sChunk{c}, valid: true})
	}
	for _, r := range idx.readings {
		if r.valid {
			r.valid = r.secretsAccounted(lines)
		}
	}
	return idx
}

// ownedByOtherKind reports whether every applicable reading proves that the `name` key
// (data or stringData) starting on line i belongs to a non-Secret object.
func (idx *k8sOwnerIndex) ownedByOtherKind(i int, name string) bool {
	if len(idx.readings) == 0 {
		return false
	}
	for _, r := range idx.readings {
		ok := r.valid && r.proves(i, func(recs []k8sKeyRec) bool {
			found := false
			for _, k := range recs {
				if k.name != name {
					continue
				}
				if !k.otherOwner {
					return false
				}
				found = true
			}
			return found
		})
		if !ok {
			return false
		}
	}
	return true
}

// proves applies test to the keys on line i in every chunk covering it. No covering
// chunk, or one that did not parse, is could-not-check.
func (r *k8sReading) proves(i int, test func([]k8sKeyRec) bool) bool {
	covered := false
	for _, c := range r.chunks {
		if i < c.start || i >= c.end {
			continue
		}
		covered = true
		if !c.ok || !test(c.keys[i]) {
			return false
		}
	}
	return covered
}

// secretsAccounted reports whether every Secret-kind declaration line is, in this
// reading, the kind key of a parsed Secret mapping.
func (r *k8sReading) secretsAccounted(lines []string) bool {
	for i, ln := range lines {
		if !reK8sSecretDecl.MatchString(ln) {
			continue
		}
		if !r.proves(i, func(recs []k8sKeyRec) bool {
			for _, k := range recs {
				if k.secretKind {
					return true
				}
			}
			return false
		}) {
			return false
		}
	}
	return true
}

// k8sDiffReading reads a unified diff hunk by hunk (see k8sOwnerIndex).
func k8sDiffReading(lines []string) *k8sReading {
	r := &k8sReading{valid: true}
	accounted := make([]bool, len(lines))
	for h, ln := range lines {
		m := reK8sHunkRange.FindStringSubmatch(strings.TrimRight(ln, "\r"))
		if m == nil {
			continue
		}
		oldStart, e1 := strconv.Atoi(m[1])
		oldN, e2 := hunkCount(m[2])
		newN, e3 := hunkCount(m[4])
		if e1 != nil || e2 != nil || e3 != nil {
			continue // left unaccounted: the reading becomes unprovable below
		}
		accounted[h] = true
		newFile := oldStart == 0 && oldN == 0
		reach := oldN + newN
		if newFile {
			reach = newN
		}
		end := h + 1 + reach
		truncated := end > len(lines) || end < h+1
		if truncated {
			end = len(lines)
		}
		c := &k8sChunk{start: h + 1, end: end}
		if newFile && !truncated {
			body := append([]string(nil), lines[h+1:end]...)
			marked := len(body) > 0
			for _, b := range body {
				if !strings.HasPrefix(b, "+") {
					marked = false
					break
				}
			}
			if marked {
				for j := range body {
					body[j] = body[j][1:]
				}
			}
			c = parseK8sChunk(body, 0, len(body))
			c.start, c.end = h+1, end
			c.keys = shiftK8sKeys(c.keys, h+1)
		}
		for j := h + 1; j < end; j++ {
			accounted[j] = true
		}
		r.chunks = append(r.chunks, c)
	}
	for i, ln := range lines {
		if accounted[i] {
			continue
		}
		if t := strings.TrimSpace(ln); t != "" && t != `\ No newline at end of file` {
			r.valid = false
			break
		}
	}
	return r
}

func hunkCount(s string) (int, error) {
	if s == "" {
		return 1, nil
	}
	return strconv.Atoi(s)
}

func shiftK8sKeys(keys map[int][]k8sKeyRec, by int) map[int][]k8sKeyRec {
	out := make(map[int][]k8sKeyRec, len(keys))
	for k, v := range keys {
		out[k+by] = v
	}
	return out
}

// k8sFenceReading pairs markdown code fences the CommonMark way and parses each block.
// It returns nil when the text has no fenced block.
func k8sFenceReading(lines []string) *k8sReading {
	r := &k8sReading{valid: true}
	for i := 0; i < len(lines); i++ {
		m := reK8sFenceOpen.FindStringSubmatch(strings.TrimRight(lines[i], "\r"))
		if m == nil || (m[2][0] == '`' && strings.Contains(m[3], "`")) {
			continue
		}
		indent, fence := len(m[1]), m[2]
		j := i + 1
		for j < len(lines) && !isK8sFenceClose(lines[j], fence) {
			j++
		}
		body := make([]string, 0, j-i-1)
		for _, b := range lines[i+1 : j] {
			n := 0
			for n < indent && n < len(b) && b[n] == ' ' {
				n++
			}
			body = append(body, b[n:])
		}
		c := parseK8sChunk(body, 0, len(body))
		c.start, c.end = i+1, j
		c.keys = shiftK8sKeys(c.keys, i+1)
		r.chunks = append(r.chunks, c)
		i = j
	}
	if len(r.chunks) == 0 {
		return nil
	}
	return r
}

func isK8sFenceClose(line, fence string) bool {
	t := strings.TrimRight(line, " \t\r")
	n := 0
	for n < 3 && n < len(t) && t[n] == ' ' {
		n++
	}
	t = t[n:]
	return len(t) >= len(fence) && strings.Trim(t, fence[:1]) == ""
}

// parseK8sChunk parses lines[start:end] as one YAML stream and indexes every scalar
// mapping key by absolute line.
func parseK8sChunk(lines []string, start, end int) *k8sChunk {
	c := &k8sChunk{start: start, end: end, keys: map[int][]k8sKeyRec{}}
	dec := yaml.NewDecoder(strings.NewReader(strings.Join(lines[start:end], "\n")))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			c.keys = map[int][]k8sKeyRec{}
			return c // ok stays false: could-not-check
		}
		c.index(&doc, start, false)
	}
	c.ok = true
	return c
}

func (c *k8sChunk) index(n *yaml.Node, base int, underSecret bool) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, ch := range n.Content {
			c.index(ch, base, underSecret)
		}
	case yaml.MappingNode:
		kinds, unsure, isSecret, own := 0, false, false, ""
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Kind != yaml.ScalarNode || k.Tag == "!!merge" || k.Value == "<<" {
				unsure = true // a complex or merge key could carry another kind
				continue
			}
			if k.Value != "kind" {
				continue
			}
			kinds++
			if v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
				unsure = true
				continue
			}
			own = v.Value
			if own == "Secret" {
				isSecret = true
			}
		}
		other := kinds == 1 && !unsure && !underSecret && own != "Secret" && reK8sKindName.MatchString(own)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Kind == yaml.ScalarNode {
				line := base + k.Line - 1
				c.keys[line] = append(c.keys[line], k8sKeyRec{
					name:       k.Value,
					otherOwner: other,
					secretKind: k.Value == "kind" && v.Kind == yaml.ScalarNode && v.Value == "Secret",
				})
			} else {
				c.index(k, base, underSecret || isSecret)
			}
			c.index(v, base, underSecret || isSecret)
		}
	}
}
