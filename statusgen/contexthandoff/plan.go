package contexthandoff

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// plan.go reads the plan facts the resolver needs out of a source's files:
// brief identity from the path (directory enumeration, independent of README
// rows), typed edges from frontmatter, lifecycle from the stream README, and
// the protected history regions of a brief.

var (
	briefPathRe   = regexp.MustCompile(`^docs/streams/([a-z0-9][a-z0-9-]*)/brief-([0-9]{2,4}[a-z]?)-[^/]*\.md$`)
	readmePathRe  = regexp.MustCompile(`^docs/streams/([a-z0-9][a-z0-9-]*)/README\.md$`)
	streamNNRefRe = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)/([0-9]{2,4}[a-z]?)$`)
	isoDateRe     = regexp.MustCompile(`\b[0-9]{4}-[0-9]{2}-[0-9]{2}\b`)
	longDateRe    = regexp.MustCompile(`(?i)\b([0-9]{1,2} (january|february|march|april|may|june|july|august|september|october|november|december) [0-9]{4}|(january|february|march|april|may|june|july|august|september|october|november|december) [0-9]{1,2},? [0-9]{4})\b`)
	htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// brief is one enumerated brief file.
type brief struct {
	ID        string // four-segment identity
	Source    string // source id
	Path      string // path inside the source
	Raw       []byte
	Depends   []string // four-segment identities
	Unblocks  []string
	Consumers []string // raw consumers entries
	Lifecycle string
	Evidence  bool // the ## Evidence section is populated
}

// delivered is a brief whose history is closed: delivered lifecycle and a
// populated Evidence section. New obligations on it route elsewhere.
func (b *brief) delivered() bool {
	switch b.Lifecycle {
	case "implemented", "verified", "done":
		return b.Evidence
	}
	return false
}

// unfinished is a brief that can still take a new obligation.
func (b *brief) unfinished() bool {
	switch b.Lifecycle {
	case "todo", "in-progress", "blocked":
		return true
	}
	return false
}

func (b *brief) locator(anchor string) string {
	l := b.Source + ":" + escapePath(b.Path)
	if anchor != "" {
		l += "#" + anchor
	}
	return l
}

type frontmatter struct {
	Depends   []string `yaml:"depends"`
	Unblocks  []string `yaml:"unblocks"`
	Consumers []string `yaml:"consumers"`
}

// splitFrontmatter returns the frontmatter text, the line index where the body
// starts, and the body. A file with no frontmatter has an empty one.
func splitFrontmatter(raw []byte) (fm string, bodyLine int, body []byte) {
	lines := splitLines(raw)
	if len(lines) == 0 || lines[0] != "---" {
		return "", 0, raw
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return strings.Join(lines[1:i], "\n"), i + 1, []byte(strings.Join(lines[i+1:], "\n"))
		}
	}
	return "", 0, raw
}

// parseBrief reads one brief. A frontmatter the minimal reader cannot decode
// yields no typed edges; the brief still counts as enumerated, and prose
// discovery still reads its body.
func parseBrief(src *SnapshotSource, p string, raw []byte, stream, nn string) *brief {
	b := &brief{
		ID:     QID{Cell: src.Cell, Repo: src.Repo, Stream: stream, NN: nn}.String(),
		Source: src.ID, Path: p, Raw: raw, Lifecycle: "unknown",
	}
	fmText, _, _ := splitFrontmatter(raw)
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err == nil {
		ctx := QID{Cell: src.Cell, Repo: src.Repo}
		b.Depends = resolveRefs(fm.Depends, ctx)
		b.Unblocks = resolveRefs(fm.Unblocks, ctx)
		b.Consumers = fm.Consumers
	}
	b.Evidence = evidencePopulated(raw)
	return b
}

// resolveRefs expands plan references against the source's cell and repo: the
// brief-v1 <stream>/<NN> form and the qualified forms. Anything else (a
// cross-repository alias, prose) yields no typed edge.
func resolveRefs(refs []string, ctx QID) []string {
	var out []string
	for _, r := range refs {
		if m := streamNNRefRe.FindStringSubmatch(r); m != nil {
			out = append(out, QID{Cell: ctx.Cell, Repo: ctx.Repo, Stream: m[1], NN: m[2]}.String())
			continue
		}
		if q, err := ResolveQualifiedID(r, &ctx); err == nil {
			out = append(out, q.String())
		}
	}
	sort.Strings(out)
	return out
}

// readmeLifecycles reads a stream README's status table: the row number in the
// first column, the lifecycle in the column headed "Status".
func readmeLifecycles(raw []byte) map[string]string {
	out := map[string]string{}
	status := -1
	for _, line := range splitLines(raw) {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			status = -1
			continue
		}
		cells := tableCells(line)
		if status < 0 {
			for i, c := range cells {
				if strings.EqualFold(c, "status") {
					status = i
				}
			}
			continue
		}
		if len(cells) <= status || !idNumberRe.MatchString(cells[0]) {
			continue
		}
		out[cells[0]] = lifecycleOf(cells[status])
	}
	return out
}

func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func lifecycleOf(cell string) string {
	v := strings.ToLower(strings.Trim(cell, "*_` "))
	switch v {
	case "todo", "in-progress", "implemented", "verified", "done", "blocked", "withdrawn":
		return v
	}
	return "unknown"
}

// evidencePopulated reports whether the ## Evidence section carries anything
// beyond blank lines and HTML comments.
func evidencePopulated(raw []byte) bool {
	r, ok := evidenceSection(splitLines(raw))
	if !ok {
		return false
	}
	text := strings.Join(splitLines(raw)[r.start+1:r.end], "\n")
	return strings.TrimSpace(htmlCommentRe.ReplaceAllString(text, "")) != ""
}

// region is a protected span of lines, [start, end), 0-based.
type region struct {
	kind   string // history kind: evidence or source-account
	anchor string
	start  int
	end    int
	text   string
}

// evidenceSection finds the ## Evidence section: the heading line through the
// last non-blank line before the next level-2 heading.
func evidenceSection(lines []string) (region, bool) {
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			return sectionAt(lines, i), true
		}
	}
	return region{}, false
}

func sectionAt(lines []string, i int) region {
	end := len(lines)
	for j := i + 1; j < len(lines); j++ {
		if strings.HasPrefix(lines[j], "## ") {
			end = j
			break
		}
	}
	for end > i+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return region{start: i, end: end}
}

// protectedRegions lists a brief's history: a populated Evidence section,
// every level-2 section whose heading carries a date (a dated amendment or
// source account), and every dated frontmatter line or freshness-checked line.
// A patch may add after a region; it may not remove or rewrite inside one.
func protectedRegions(raw []byte) []region {
	lines := splitLines(raw)
	_, bodyLine, _ := splitFrontmatter(raw)
	var out []region
	add := func(r region) {
		r.text = strings.Join(lines[r.start:r.end], "\n")
		out = append(out, r)
	}
	if r, ok := evidenceSection(lines); ok && evidencePopulated(raw) {
		r.kind, r.anchor = "evidence", "evidence"
		add(r)
	}
	for i := bodyLine; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "## ") && strings.TrimSpace(l) != "## Evidence" && hasDate(l) {
			r := sectionAt(lines, i)
			r.kind, r.anchor = "source-account", slug(strings.TrimPrefix(l, "## "))
			add(r)
		}
	}
	for i, l := range lines {
		inFrontmatter := bodyLine > 0 && i > 0 && i < bodyLine-1
		if (inFrontmatter && hasDate(l)) || strings.Contains(l, "freshness-checked") {
			if covered(out, i) {
				continue
			}
			add(region{kind: "source-account", anchor: "line-" + shortHash(l), start: i, end: i + 1})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].start < out[b].start })
	return out
}

func covered(rs []region, line int) bool {
	for _, r := range rs {
		if line >= r.start && line < r.end {
			return true
		}
	}
	return false
}

func hasDate(s string) bool { return isoDateRe.MatchString(s) || longDateRe.MatchString(s) }

// slug makes a heading into a locator anchor: lower-case ASCII letters and
// digits joined by single hyphens.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	if b.Len() == 0 {
		return "section"
	}
	return b.String()
}

// splitLines splits on "\n" and drops the empty tail a final newline leaves.
func splitLines(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	s := string(raw)
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// BlobRevision is a file's git blob identity: the revision a claim or a
// candidate patch names, reproducible from the bytes alone.
func BlobRevision(raw []byte) string {
	h := sha1.New()
	h.Write([]byte("blob " + strconv.Itoa(len(raw)) + "\x00"))
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}

// PatchDigest is a candidate patch's digest, as a Patch binding records it.
func PatchDigest(raw []byte) string { return digest(raw) }

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func shortHash(s string) string { return sha256Hex([]byte(s))[:12] }

// escapePath makes a path a locator: whitespace and the locator punctuation
// (%, @, #) are percent-escaped, so a locator never carries whitespace and
// always splits back unambiguously.
func escapePath(p string) string {
	var b bytes.Buffer
	for _, r := range p {
		if r == '%' || r == '@' || r == '#' || unicode.IsSpace(r) || r == 0xfeff || r < 0x20 || r == 0x7f {
			for _, c := range []byte(string(r)) {
				b.WriteString("%" + strings.ToUpper(hex.EncodeToString([]byte{c})))
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// hasTerm reports whether text mentions term with token boundaries, ignoring
// case: a letter, digit, hyphen or underscore on either side is not a
// boundary, so "ledger-export" does not match "ledger-exporter".
func hasTerm(text, term string) bool {
	t := strings.ToLower(text)
	k := strings.ToLower(term)
	for i := 0; ; {
		j := strings.Index(t[i:], k)
		if j < 0 {
			return false
		}
		at := i + j
		end := at + len(k)
		if (at == 0 || !wordByte(t[at-1])) && (end == len(t) || !wordByte(t[end])) {
			return true
		}
		i = at + 1
	}
}

func wordByte(c byte) bool {
	return c == '-' || c == '_' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c >= 0x80
}
