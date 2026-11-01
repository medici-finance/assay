package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	pathpkg "path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// readset.go — a witness's RECORDED read set (build-less-brittle/15).
//
// A Verify row run under verifyrun's recorder (statusgen readset_linux.go)
// leaves a file naming every in-tree path the row's processes read, stat-ed,
// listed or looked up and found absent. The witness row's Runner cell names
// that file by the sha256 of its content: `<runner> @ <tree> (reads
// sha256:<64 hex>)`. Where the recorder could not observe the whole row the
// cell instead says `(reads could-not-check: <reason>)` and no file exists.
//
// A counted read set replaces the scope a witness speaks for: the witness is
// stale exactly when one of its recorded entries changed between its tree and
// the item's (readSetStale). Nothing here widens a release that today's scope
// refuses on its own ground: the Command and Expect guards still bind, a
// `+dirty` or `+unknown` witness keeps the conservative scope, a manifest's
// scope is unioned in, and the landing check still runs. A read set that does
// not count (judgeReadSet) leaves today's scope and reason untouched.
//
// The go floor is the independent lower layer. For every build-family `go-run`
// entry the caller supplies the files `go list -deps -test` names for the
// recorded invocation at the witness's tree (GoFloor). A recorded set that
// lacks one of them is refused, and a build-family entry with no floor
// observation is could-not-check, so a caller that skips the floor fails
// closed.

// ReadSetDir is the directory under a stream (`docs/streams/<stream>/`) that
// holds its read-set files, each named by the sha256 of its content.
const ReadSetDir = "reads"

// The exact refusal strings (brief 15, Task step 5 and 6).
const (
	ReasonReadSetUnreadable = "the witness's recorded read set cannot be read or does not match its digest"
	ReasonReadSetHistory    = "the row read repository history (.git), so only a witness at the item's exact revision speaks for it"
	ReasonFloorOmits        = "the recorded read set omits a file the go toolchain compiles for this row"
	ReasonFloorMissing      = "the read set records a go build but no compile-input floor was supplied"
	ReasonReadSetBounds     = "the witness's recorded read set or its introduction could not be observed within the reader's bounds"
)

// Read-set entry kinds. A path is slash-separated, relative to the
// repository's toplevel; "." names the toplevel directory itself.
const (
	EntryRead    = "read"    // a file opened for reading, mapped or executed
	EntryStat    = "stat"    // a tracked file stat-ed or probed; judged by content
	EntryList    = "list"    // a directory listed: its direct-children names are an input
	EntryAbsent  = "absent"  // a path looked up and found absent
	EntryDir     = "dir"     // a directory stat-ed or probed: its existence is an input
	EntryHistory = "history" // repository history (.git) was read; no path
	EntryGoRun   = "go-run"  // one exec of the go tool; JSON GoRun
)

// ReadSetHeader is a read-set file's header.
type ReadSetHeader struct {
	Commit  string // the full commit the row ran on
	Command string // "sha256:<64 hex>" of the row's Command text (ReadSetCommandDigest)
	Brief   string // the brief's repository path
	Row     string // the Verify row id
}

// ReadSetObservation is what the caller observed about the read-set file a
// witness's Runner token names. Found is false when the file is absent or
// cannot be read. The Intro fields are the introduction observation: the
// commit that introduced this witness row (identity key: WitnessRowKey) into
// the brief file, read from ancestry (its brief file holds the row and no
// parent's does; a row more than one commit introduced has none), and, there,
// the row's Runner cell and whether the read-set file existed in that commit.
// Problem is non-empty when the caller refused to observe the file or the
// introduction within its own bounds (an over-size file or history object):
// the claim is then could-not-check, never today's scope.
type ReadSetObservation struct {
	Content     []byte
	Found       bool
	IntroFound  bool
	IntroRunner string
	IntroFile   bool
	Problem     string
}

// GoFloor is the compile-input floor for one `go-run` entry at one witness
// commit: every in-tree file `go list -deps -test -json` names for the
// recorded invocation. Problem is non-empty when it could not be computed.
type GoFloor struct {
	Files   []string
	Problem string
}

// GoFloorSource answers the compile-input floor for one `go-run` entry
// line, verbatim, at one full witness commit. ok is false when it has none.
type GoFloorSource interface {
	GoFloor(commit, entry string) (floor GoFloor, ok bool)
}

// GoFloorMap is a GoFloorSource over floors computed up front, keyed by
// GoFloorKey.
type GoFloorMap map[string]GoFloor

// GoFloor implements GoFloorSource.
func (m GoFloorMap) GoFloor(commit, entry string) (GoFloor, bool) {
	f, ok := m[GoFloorKey(commit, entry)]
	return f, ok
}

// GoFloorKey keys a GoFloorMap: the witness's full commit and the read set's
// `go-run` entry line, verbatim.
func GoFloorKey(commit, entry string) string { return commit + "\x00" + entry }

// GoRun is a `go-run` entry's JSON body: one exec of the go tool.
type GoRun struct {
	// Dir is the exec's working directory relative to the toplevel ("." at it).
	Dir        string   `json:"dir"`
	Args       []string `json:"args"`
	GOFLAGS    string   `json:"goflags,omitempty"`
	GOWORK     string   `json:"gowork,omitempty"`
	GOOS       string   `json:"goos,omitempty"`
	GOARCH     string   `json:"goarch,omitempty"`
	CGOEnabled string   `json:"cgo_enabled,omitempty"`
}

// GoRunEntry renders g as a read-set entry line. The body is the JSON
// encoding/json gives the struct, written without it (the purity boundary):
// fields in declaration order, an empty optional field left out, a nil Args
// as null, and strings escaped as encoding/json escapes them.
func GoRunEntry(g GoRun) string {
	var b strings.Builder
	b.WriteString(EntryGoRun + ` {"dir":`)
	writeGoJSONString(&b, g.Dir)
	b.WriteString(`,"args":`)
	if g.Args == nil {
		b.WriteString("null")
	} else {
		b.WriteByte('[')
		for i, a := range g.Args {
			if i > 0 {
				b.WriteByte(',')
			}
			writeGoJSONString(&b, a)
		}
		b.WriteByte(']')
	}
	for _, f := range []struct{ key, val string }{
		{"goflags", g.GOFLAGS}, {"gowork", g.GOWORK}, {"goos", g.GOOS}, {"goarch", g.GOARCH}, {"cgo_enabled", g.CGOEnabled},
	} {
		if f.val != "" {
			b.WriteString(`,"` + f.key + `":`)
			writeGoJSONString(&b, f.val)
		}
	}
	b.WriteByte('}')
	return b.String()
}

// writeGoJSONString writes s as encoding/json's Marshal does: HTML escaping
// on, `\b` and `\f` short forms, U+2028 and U+2029 escaped, and each invalid
// UTF-8 byte replaced by the rune U+FFFD.
func writeGoJSONString(b *strings.Builder, s string) {
	const hexDigits = "0123456789abcdef"
	b.WriteByte('"')
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			switch {
			case c == '"' || c == '\\':
				b.WriteByte('\\')
				b.WriteByte(c)
			case c == '\b':
				b.WriteString(`\b`)
			case c == '\f':
				b.WriteString(`\f`)
			case c == '\n':
				b.WriteString(`\n`)
			case c == '\r':
				b.WriteString(`\r`)
			case c == '\t':
				b.WriteString(`\t`)
			case c < 0x20 || c == '<' || c == '>' || c == '&':
				b.WriteString(`\u00`)
				b.WriteByte(hexDigits[c>>4])
				b.WriteByte(hexDigits[c&0xf])
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteRune(utf8.RuneError)
		case r == '\u2028' || r == '\u2029':
			b.WriteString(`\u202`)
			b.WriteByte(hexDigits[r&0xf])
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
}

// ParseGoRun reads a `go-run` entry line. The body must be one JSON object
// whose keys are GoRun's own field names, each a string (args: an array of
// strings) or null; any other key, type or spelling is malformed, so an entry
// this reader cannot fully read never narrows a scope.
func ParseGoRun(entry string) (GoRun, bool) {
	body, ok := strings.CutPrefix(entry, EntryGoRun+" ")
	if !ok {
		return GoRun{}, false
	}
	v, err := decodeJSON([]byte(body))
	obj, isObj := v.(map[string]any)
	if err != nil || !isObj {
		return GoRun{}, false
	}
	var g GoRun
	fields := map[string]*string{"dir": &g.Dir, "goflags": &g.GOFLAGS, "gowork": &g.GOWORK, "goos": &g.GOOS, "goarch": &g.GOARCH, "cgo_enabled": &g.CGOEnabled}
	for k, val := range obj {
		if k == "args" {
			switch t := val.(type) {
			case nil:
			case []any:
				g.Args = make([]string, 0, len(t))
				for _, e := range t {
					s, ok := e.(string)
					if !ok {
						return GoRun{}, false
					}
					g.Args = append(g.Args, s)
				}
			default:
				return GoRun{}, false
			}
			continue
		}
		dst, known := fields[k]
		if !known {
			return GoRun{}, false
		}
		switch t := val.(type) {
		case nil:
		case string:
			*dst = t
		default:
			return GoRun{}, false
		}
	}
	return g, true
}

// Subcommand is the go tool subcommand the exec ran: Args[1], or the one after
// a leading `-C <dir>` / `-C=<dir>`.
func (g GoRun) Subcommand() string {
	a := g.Args
	if len(a) < 2 {
		return ""
	}
	i := 1
	switch {
	case a[i] == "-C" || a[i] == "--C":
		i += 2
	case strings.HasPrefix(a[i], "-C=") || strings.HasPrefix(a[i], "--C="):
		i++
	}
	if i >= len(a) {
		return ""
	}
	return a[i]
}

// BuildFamily reports whether the exec compiles or loads packages: build,
// test, vet, run, install or list. Only these get a floor.
func (g GoRun) BuildFamily() bool {
	switch g.Subcommand() {
	case "build", "test", "vet", "run", "install", "list":
		return true
	}
	return false
}

// readsTokenRe lifts the Runner cell's read-set token. A could-not-check
// reason may carry one level of parentheses (`recorder is Linux-only
// (darwin)`).
var readsTokenRe = regexp.MustCompile(`\s*\(reads (sha256:([0-9a-f]{64})|could-not-check: ((?:[^()]|\([^()]*\))*))\)`)

// ReadSetToken returns a Runner cell's read-set token: digest is the 64-hex
// file name of a `sha256:` token, couldNot the reason of a `could-not-check:`
// token; ok is false when the cell carries none.
func ReadSetToken(runner string) (digest, couldNot string, ok bool) {
	m := readsTokenRe.FindStringSubmatch(runner)
	if m == nil {
		return "", "", false
	}
	if m[2] != "" {
		return m[2], "", true
	}
	return "", m[3], true
}

// StripReadSetToken removes the read-set token from a Runner cell, so the
// rest (runner, tree and any other parenthetical) reads as before.
func StripReadSetToken(runner string) string {
	return strings.TrimSpace(readsTokenRe.ReplaceAllString(runner, ""))
}

// ReadSetCellToken renders the token's parenthetical for a Runner cell from a
// digest ("" when none) or a could-not-check reason. A reason's table pipes,
// line breaks and unbalanced parentheses are neutralised so the cell parses.
func ReadSetCellToken(digest, couldNot string) string {
	if digest != "" {
		return "(reads sha256:" + digest + ")"
	}
	return "(reads could-not-check: " + sanitizeReason(couldNot) + ")"
}

// maxFloorRuns bounds the build-family go invocations one read set may
// record: each needs its own floor, one go tool run.
var maxFloorRuns = 16

// reasonText is recorded or tree-supplied text made safe for a reason: no
// control byte, no bidirectional format character or Unicode line separator,
// no cell separator, no backtick, at most one parenthesis level.
func reasonText(s string) string {
	return sanitizeReason(strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || (r >= 0x7f && r < 0xa0): // C0, DEL and C1 controls
			return ' '
		case r == 0x061c || r == 0x200e || r == 0x200f, // arabic letter, left-to-right and right-to-left marks
			r >= 0x202a && r <= 0x202e, // embeddings, overrides and their pop
			r >= 0x2066 && r <= 0x2069, // isolates and their pop
			r == 0x2028 || r == 0x2029: // line and paragraph separators
			return ' '
		}
		return r
	}, s))
}

// sanitizeReason keeps a reason inside one table cell and within the one
// level of parentheses readsTokenRe accepts.
func sanitizeReason(s string) string {
	s = strings.NewReplacer("|", "/", "\n", " ", "\r", " ", "`", "'").Replace(s)
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			if depth == 0 {
				depth++
				b.WriteRune(r)
			} else {
				b.WriteRune('[')
			}
			continue
		case ')':
			if depth == 1 {
				depth--
				b.WriteRune(r)
			} else {
				b.WriteRune(']')
			}
			continue
		}
		b.WriteRune(r)
	}
	if depth == 1 {
		b.WriteRune(')')
	}
	return strings.TrimSpace(b.String())
}

// ReadSetCommandDigest is the header's command digest of a Verify row's
// Command text as authored (whitespace-normalised, as the Command guard
// compares it).
func ReadSetCommandDigest(command string) string {
	sum := sha256.Sum256([]byte(NormalizeCommandText(command)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ReadSetDigest is the file name of a read set's content.
func ReadSetDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// ReadSetPath is the repository path of a stream's read-set file.
func ReadSetPath(streamDir, digest string) string {
	return pathpkg.Join(streamDir, ReadSetDir, digest)
}

// WitnessRowKey is a witness row's identity key for the introduction binding:
// its `#`, Command and Date cells and its Runner cell without the read-set
// token, so neither the token nor a result or output edit makes a new row.
func WitnessRowKey(id, command, date, runner string) string {
	return strings.Join([]string{strings.TrimSpace(id), NormalizeCommandText(command), strings.TrimSpace(date), NormalizeCommandText(StripReadSetToken(runner))}, "\x00")
}

// FormatReadSet renders a read-set file: the header, then the sorted, de-
// duplicated entries, newline-terminated.
func FormatReadSet(h ReadSetHeader, entries []string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "witness-commit %s\ncommand %s\nbrief %s\nrow %s\n", h.Commit, h.Command, h.Brief, h.Row)
	seen := map[string]bool{}
	sorted := append([]string(nil), entries...)
	sort.Strings(sorted)
	for _, e := range sorted {
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		b.WriteString(e)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

var (
	hex40Re = regexp.MustCompile(`^[0-9a-f]{40}$`)
	cmdSum  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// ParseReadSet reads a read-set file. Any line it does not know is an error:
// a file this reader cannot fully read never narrows a scope.
func ParseReadSet(content []byte) (ReadSetHeader, []string, error) {
	var h ReadSetHeader
	text := string(content)
	if !strings.HasSuffix(text, "\n") {
		return h, nil, errors.New("not newline-terminated")
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) < 4 {
		return h, nil, errors.New("short header")
	}
	fields := []*string{&h.Commit, &h.Command, &h.Brief, &h.Row}
	for i, name := range []string{"witness-commit ", "command ", "brief ", "row "} {
		v, ok := strings.CutPrefix(lines[i], name)
		if !ok || v == "" {
			return h, nil, fmt.Errorf("header line %d is not %q", i+1, strings.TrimSpace(name))
		}
		*fields[i] = v
	}
	if !hex40Re.MatchString(h.Commit) || !cmdSum.MatchString(h.Command) {
		return h, nil, errors.New("malformed header")
	}
	var entries []string
	for _, l := range lines[4:] {
		kind, rest, _ := strings.Cut(l, " ")
		switch kind {
		case EntryHistory:
			if rest != "" {
				return h, nil, fmt.Errorf("malformed entry %q", l)
			}
		case EntryGoRun:
			if _, ok := ParseGoRun(l); !ok {
				return h, nil, fmt.Errorf("malformed entry %q", l)
			}
		case EntryRead, EntryStat, EntryList, EntryAbsent, EntryDir:
			if !readSetPathOK(rest) {
				return h, nil, fmt.Errorf("malformed path in entry %q", l)
			}
		default:
			return h, nil, fmt.Errorf("unknown entry %q", l)
		}
		entries = append(entries, l)
	}
	return h, entries, nil
}

// readSetPathOK reports whether p is a clean, local, slash-separated path.
func readSetPathOK(p string) bool {
	if p == "." {
		return true
	}
	return p != "" && !strings.HasPrefix(p, "/") && pathpkg.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}

// readSet is a counted read set, its paths with the exempt ones subtracted.
// Each kind of entry is a set, so a path recorded twice is judged once.
type readSet struct {
	files   map[string]bool // read and stat entries
	all     map[string]bool // every read and stat path, exempt ones included (the floor's comparison)
	lists   map[string]bool
	absent  map[string]bool
	dirs    map[string]bool
	history bool
	goRuns  []string
	paths   int // distinct non-exempt path entries
	exempt  int // distinct recorded path entries subtracted as bookkeeping
}

// exempt reports whether a change to repo path p can never stale a witness of
// this brief: the brief's own file (bound by the Command and Expect guards),
// the board's bookkeeping surface (docs/streams/**, STATUS.md) and the paths
// verify and regen write. The conservative scope leaves out exactly these,
// and the read-set scope subtracts them through this one function.
func (sc witnessScope) exempt(p string) bool {
	return p == sc.briefRel || isBoardBookkeepingPath(p) || isVerifyWrittenPath(p)
}

// judgeReadSet decides whether witness w's read set counts for row r. It
// returns the counted set, or nil (today's scope stands); couldNot is the
// reason when the token names a file that is absent or whose digest differs,
// or when the caller could not observe the file or its introduction within
// its bounds, which resolves the claim could-not-check whatever its scope
// would say.
func (x *evalCtx) judgeReadSet(sc witnessScope, r VerifyRow, w Witness) (rs *readSet, couldNot string) {
	digest, _, ok := ReadSetToken(w.Runner)
	if !ok || digest == "" {
		return nil, ""
	}
	o := w.ReadSet
	if o != nil && o.Problem != "" {
		return nil, ReasonReadSetBounds + " (" + reasonText(o.Problem) + ")"
	}
	if o == nil || !o.Found || ReadSetDigest(o.Content) != digest {
		return nil, ReasonReadSetUnreadable
	}
	if why := x.readSetOverdue(); why != "" {
		return nil, ReasonReadSetBounds + " (" + reasonText(why) + ")"
	}
	h, entries, err := ParseReadSet(o.Content)
	if err != nil {
		return nil, ""
	}
	full, why := x.resolveCommitToken(witnessBaseRevision(w.Tree))
	if why != "" || h.Commit != full {
		return nil, ""
	}
	if h.Command != ReadSetCommandDigest(r.Command) || h.Brief != sc.briefRel || h.Row != r.ID {
		return nil, ""
	}
	// The introduction binding: the row arrived carrying this token, with its
	// file, in one commit. A token or file a later commit added is ignored.
	if !o.IntroFound || !o.IntroFile {
		return nil, ""
	}
	if d, _, ok := ReadSetToken(o.IntroRunner); !ok || d != digest {
		return nil, ""
	}
	rs = &readSet{files: map[string]bool{}, all: map[string]bool{}, lists: map[string]bool{}, absent: map[string]bool{}, dirs: map[string]bool{}}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e] {
			continue // a repeated entry is judged once
		}
		seen[e] = true
		kind, p, _ := strings.Cut(e, " ")
		switch kind {
		case EntryHistory:
			rs.history = true
			continue
		case EntryGoRun:
			rs.goRuns = append(rs.goRuns, e)
			continue
		case EntryRead, EntryStat:
			rs.all[p] = true
		}
		if sc.exempt(p) {
			rs.exempt++
			continue
		}
		rs.paths++
		switch kind {
		case EntryRead, EntryStat:
			rs.files[p] = true
		case EntryList:
			rs.lists[p] = true
		case EntryAbsent:
			rs.absent[p] = true
		case EntryDir:
			rs.dirs[p] = true
		}
	}
	if rs.paths == 0 {
		return nil, ""
	}
	return rs, ""
}

// describe says what a read-set scope covers, for a reason.
func (rs *readSet) describe() string {
	return fmt.Sprintf("its recorded read set: %d in-tree entries, %d recorded reads exempt as the board's bookkeeping", rs.paths, rs.exempt)
}

// underDir reports whether p sits below directory d ("." is the toplevel).
func underDir(d, p string) (child string, ok bool) {
	rest := p
	if d != "." {
		var found bool
		rest, found = strings.CutPrefix(p, d+"/")
		if !found {
			return "", false
		}
	}
	child, _, _ = strings.Cut(rest, "/")
	return child, child != ""
}

// contentAttrs are the attributes that change what a checkout of a path
// holds.
var contentAttrs = []string{"text", "eol", "filter", "working-tree-encoding", "ident"}

// readSetApplies is witnessTreeApplies for a counted read set, at full commit
// ids w and t that share history. used is false when the read set cannot be
// applied here (the root is not the toplevel), and today's scope then judges.
func (x *evalCtx) readSetApplies(sc witnessScope, rs *readSet, w, t, landedNoun string) (rel revisionRelation, detail string, used bool) {
	if prefix, ok := x.rootPrefix(); !ok || prefix != "" {
		return 0, "", false
	}
	if rs.history {
		return revisionMismatch, ReasonReadSetHistory, true
	}
	// The go floor (lower layer): each build-family exec's compiled files must
	// all be in the recorded set. Each floor is one bounded go tool run, so
	// their number is bounded too.
	builds := 0
	for _, e := range rs.goRuns {
		if g, _ := ParseGoRun(e); g.BuildFamily() {
			builds++
		}
	}
	if builds > maxFloorRuns {
		return revisionUnestablished, fmt.Sprintf("the read set records %d go build invocations, over the %d the compile-input floor computes", builds, maxFloorRuns), true
	}
	for _, e := range rs.goRuns {
		g, _ := ParseGoRun(e)
		if !g.BuildFamily() {
			continue
		}
		var fl GoFloor
		ok := false
		if x.goFloors != nil {
			fl, ok = x.goFloors.GoFloor(w, e)
		}
		if !ok {
			return revisionUnestablished, ReasonFloorMissing, true
		}
		if fl.Problem != "" {
			return revisionUnestablished, "the compile-input floor for the row's `" + reasonText(strings.Join(g.Args, " ")) + "` could not be computed (" + reasonText(fl.Problem) + ")", true
		}
		for _, f := range fl.Files {
			if !rs.all[f] {
				return revisionMismatch, fmt.Sprintf("%s (%s is compiled for `%s` and is not recorded)", ReasonFloorOmits, reasonText(f), reasonText(strings.Join(g.Args, " "))), true
			}
		}
	}
	changed, ok := x.repo.DiffNames(w, t)
	if !ok {
		return revisionUnestablished, "the witness's tree and the item's could not be compared (git diff failed)", true
	}
	var eff witnessScope
	union := sc.manifest != nil
	if union {
		eff = sc.atBase(x, w, t)
	}
	attrsTouched := false
	for _, p := range changed {
		if why := x.readSetOverdue(); why != "" {
			return revisionUnestablished, ReasonReadSetBounds + " (" + reasonText(why) + ")", true
		}
		if pathpkg.Base(p) == ".gitattributes" {
			attrsTouched = true
		}
		why, unread := rs.stale(x, p, w, t)
		if unread != "" {
			return revisionUnestablished, unread, true
		}
		if why != "" {
			return revisionMismatch, fmt.Sprintf("%s differs between the witness's tree and the item's and the witness speaks for it (%s; %s)", p, why, rs.describe()), true
		}
		if union && eff.invalidatedBy(p) {
			return revisionMismatch, fmt.Sprintf("%s differs between the witness's tree and the item's and the witness speaks for it (%s)", p, eff.describe()), true
		}
	}
	if attrsTouched {
		if p, why := x.attrsChanged(rs, w, t); why != "" {
			return revisionUnestablished, why, true
		} else if p != "" {
			return revisionMismatch, fmt.Sprintf("%s differs between the witness's tree and the item's and the witness speaks for it (its content attributes changed: a .gitattributes edit alters how it checks out; %s)", p, rs.describe()), true
		}
	}
	landedAt, landed, why := x.witnessLanded(w, t)
	if why != "" {
		return revisionUnestablished, why, true
	}
	if !landed {
		return revisionMismatch, fmt.Sprintf("the witness's tree never landed: no commit on the item's history since the merge base has a tree identical to it outside docs/streams/** and STATUS.md (a branch squash-merged after its base moved, or never merged), so the check did not run against code %s carries", t), true
	}
	derivation := fmt.Sprintf("none of the %d in-tree entries of the witness's recorded read set differs between the witness's tree and the item's (%d recorded reads exempt as the board's bookkeeping)", rs.paths, rs.exempt)
	if union {
		derivation += "; and " + eff.derivation()
	}
	return revisionMatch, fmt.Sprintf("%s; %s landed at %s", derivation, landedNoun, landedAt[:TreeSHALen]), true
}

// stale reports why a change to path p stales the read set ("" when it does
// not): a recorded or stat-ed file changed; a listed directory's direct
// children gained or lost a name; a path recorded absent now exists; a
// directory recorded present is gone, removed or replaced by a file. unread
// is set instead when a tree listing the answer needs could not be read: a
// failed listing is never read as "unchanged" or "gone".
//
// Its work follows p's depth, never the read set's size or the tree's: only
// p's own ancestors are looked up in the read set, and a directory's entry is
// found in a revision's listing by binary search (sortedFiles), the listing
// sorted once per revision and evaluation.
func (rs *readSet) stale(x *evalCtx, p, w, t string) (why, unread string) {
	if rs.files[p] {
		return "the row read or stat-ed it", ""
	}
	// p's ancestors, outermost first: "." then each directory above p.
	ancestors := []string{"."}
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			ancestors = append(ancestors, p[:i])
		}
	}
	for _, a := range append(ancestors[1:], p) {
		if rs.absent[a] {
			return "the row looked up " + a + " and found it absent", ""
		}
	}
	listing := func(rev string) ([]string, string) {
		files := x.sortedFiles(rev)
		if files == nil {
			return nil, "the tree at " + rev[:min(len(rev), TreeSHALen)] + " could not be listed, so whether a recorded directory changed under " + p + " cannot be established"
		}
		return files, ""
	}
	for _, d := range ancestors {
		if !rs.lists[d] {
			continue
		}
		c, _ := underDir(d, p)
		entry := c
		if d != "." {
			entry = d + "/" + c
		}
		wf, bad := listing(w)
		if bad != "" {
			return "", bad
		}
		tf, bad := listing(t)
		if bad != "" {
			return "", bad
		}
		if holds(wf, entry) != holds(tf, entry) {
			return "the row listed the directory " + d + ", whose entry " + c + " was added or removed", ""
		}
	}
	for _, d := range ancestors {
		if !rs.dirs[d] {
			continue
		}
		tf, bad := listing(t)
		if bad != "" {
			return "", bad
		}
		if d == "." && len(tf) == 0 || d != "." && !holdsBelow(tf, d) {
			return "the row stat-ed the directory " + d + ", which no longer exists", ""
		}
	}
	return "", ""
}

// readSetOverdue is why the caller's read-set time limit has run out
// (CoverageInput.ReadSetDeadline), or "".
func (x *evalCtx) readSetOverdue() string {
	if x.readSetDeadline == nil {
		return ""
	}
	return x.readSetDeadline()
}

// sortedFiles is rev's file listing sorted, sorted once per revision in an
// evaluation; nil when the listing could not be read.
func (x *evalCtx) sortedFiles(rev string) []string {
	if got, ok := x.rsSorted[rev]; ok {
		return got
	}
	files := x.treeFilesAt(rev)
	if files != nil && !sort.StringsAreSorted(files) {
		files = append([]string(nil), files...)
		sort.Strings(files)
	}
	if x.rsSorted == nil {
		x.rsSorted = map[string][]string{}
	}
	x.rsSorted[rev] = files
	return files
}

// holds reports whether a sorted file listing holds path e as a file or as a
// directory with a file below it: a listed directory's entry exists either way.
func holds(sorted []string, e string) bool {
	i := sort.SearchStrings(sorted, e)
	if i < len(sorted) && sorted[i] == e {
		return true
	}
	return holdsBelow(sorted, e)
}

// holdsBelow reports whether a sorted file listing holds a file below
// directory d. A stat-ed directory still exists only this way: d replaced by a
// file of the same name is gone as a directory.
func holdsBelow(sorted []string, d string) bool {
	j := sort.SearchStrings(sorted, d+"/")
	return j < len(sorted) && strings.HasPrefix(sorted[j], d+"/")
}

// attrsChanged compares the content attributes of every recorded file at w
// and t. It returns the first path whose attributes differ, or why they could
// not be read.
func (x *evalCtx) attrsChanged(rs *readSet, w, t string) (path, why string) {
	wr, ok := x.repo.(WitnessRepo)
	if !ok {
		return "", "a .gitattributes file changed and this repository reader cannot resolve attributes, so whether the recorded paths still check out the same cannot be established"
	}
	if why := x.readSetOverdue(); why != "" {
		return "", ReasonReadSetBounds + " (" + reasonText(why) + ")"
	}
	paths := make([]string, 0, len(rs.files))
	for p := range rs.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	was, ok1 := wr.CheckAttr(w, paths)
	now, ok2 := wr.CheckAttr(t, paths)
	if !ok1 || !ok2 {
		return "", "a .gitattributes file changed and the recorded paths' attributes could not be read (git check-attr failed)"
	}
	for _, p := range paths {
		if contentAttrLines(was[p]) != contentAttrLines(now[p]) {
			return p, ""
		}
	}
	return "", ""
}

// contentAttrLines keeps the content attributes of one path's `<attr>:
// <value>` lines, sorted, as one comparable string.
func contentAttrLines(lines []string) string {
	var keep []string
	for _, l := range lines {
		name, _, _ := strings.Cut(l, ":")
		for _, a := range contentAttrs {
			if name == a {
				keep = append(keep, l)
			}
		}
	}
	sort.Strings(keep)
	return strings.Join(keep, "\n")
}
