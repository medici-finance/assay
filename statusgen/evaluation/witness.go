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
)

// witness.go — witness schema v2 (statusgen/36): the content-bound witness
// reader and the one digest owner.
//
// A v1 witness names the commit it ran at, and coverage relates that commit to
// the item's revision through history (coverage.go). A squash merge removes the
// commit, so a valid v1 witness is orphaned. A v2 witness instead cites a
// record (docs/streams/witnesses/<stream>/<NN>/<record-id>.json) that carries
// the change request that delivered it (provenance) and a digest of the content
// the row read (staleness). judgeWitnessV2 compares those digests with the
// item's revision R through the Repo port. It reads no object at the witnessed
// commit or at any base, and never calls classifyRevisionDetail,
// witnessTreeApplies or depFingerprintSame.
//
// v2 detects staleness, not forgery: the writer computes every digest and
// self-reports runner class, network, tools and outcome. A record's
// authenticity rests on the controls a v1 cell has (who authored the Evidence
// change, its review, and its verified landing, step 2), so no v1 identity,
// acceptance-text or dirty-tree check is weakened here.
//
// The judgment's steps, in order (the first that fails names the gap):
//
//  1. the record exists, parses, and its id matches the cell;
//  2. its change request is bound, agrees with the cell, and is verified by a
//     recognised landing (landingForms) on the default branch;
//  3. the record names the citing brief and row, and the citing Evidence row's
//     Result, output-digest, Date and Command cells match it (replay);
//  4. the record's runner is the cell's identity;
//  5. Command, Expect and Class are unchanged (and an acceptance revision's
//     Expect, when supplied); then a non-pass outcome reports itself and stops;
//  6. the environment rule (WitnessPolicy) and WI-2 environment dependencies;
//  7. the recorded key sets equal the required set at R, and the literal-path
//     lower bound holds;
//  8. every dirty path lies inside the required set;
//  9. nothing is absent or a symlink, then every digest matches;
//  10. the recorded tools agree with the pins at R.

// Witness schema numbers, as ParseRunnerCell reports them.
const (
	WitnessSchemaUnknown = 0
	WitnessSchemaV1      = 1
	WitnessSchemaV2      = 2
)

const (
	// WitnessRecordDir is where v2 records live, one directory per brief.
	WitnessRecordDir = "docs/streams/witnesses/"
	// WitnessPinFile holds the tool and harness pins, an implicit input of
	// every v2 claim.
	WitnessPinFile = "plugins/assay/paired-versions.yaml"
	// ObligationEnv is the `+env` obligation token: the row depends on the
	// network, a code-hosting service or a cluster, so a digest cannot capture
	// it and it is never released by one.
	ObligationEnv = "env"
	// WitnessScopeDeclared and WitnessScopeWholeTree are a record's scopes.
	WitnessScopeDeclared  = "declared"
	WitnessScopeWholeTree = "whole-tree"
	// WitnessAbsent is the digest value of a required path missing where the
	// run read it.
	WitnessAbsent = "absent"
	// WitnessRunnerContainer is the runner class of the pinned network-less
	// container runner.
	WitnessRunnerContainer = "harness-container"
)

// The v2 gap names. A v2 claim's reason always starts with one of these (a
// release excepted).
const (
	GapSchemaUnknown           = "witness-schema-unknown"
	GapRecordMissing           = "witness-record-missing"
	GapRecordMismatch          = "witness-record-mismatch"
	GapCRUnbound               = "witness-cr-unbound"
	GapCRMismatch              = "witness-cr-mismatch"
	GapCRUnverified            = "witness-cr-unverified"
	GapRecordReplayed          = "witness-record-replayed"
	GapCommandCellMismatch     = "witness-command-cell-mismatch"
	GapRunnerMismatch          = "witness-runner-mismatch"
	GapCommandChanged          = "witness-command-changed"
	GapExpectChanged           = "witness-expect-changed"
	GapClassChanged            = "witness-class-changed"
	GapOutcomeFail             = "witness-outcome-fail"
	GapOutcomeCouldNotRun      = "witness-outcome-could-not-run"
	GapOutcomeUnparseable      = "witness-outcome-unparseable"
	GapProseLed                = "witness-prose-led"
	GapRerunOwed               = "witness-rerun-owed"
	GapEnvironmentChanged      = "witness-environment-changed"
	GapInputUndeclaredAtRecord = "witness-input-undeclared-at-record"
	GapInputSetChanged         = "witness-input-set-changed"
	GapInputUndeclared         = "witness-input-undeclared"
	GapDirtyOutsideInputs      = "witness-dirty-outside-inputs"
	GapInputAbsent             = "witness-input-absent"
	GapInputSymlink            = "witness-input-symlink"
	GapInputChanged            = "witness-input-changed"
	GapToolChanged             = "witness-tool-changed"
	GapUnestablished           = "witness-unestablished"
)

// gapResult maps a gap to the claim result it reports (Results and unreadable
// states): a changed input, environment or tool is wrong-revision; a changed
// acceptance definition and an unparseable outcome are error; a recorded
// failure is fail; every other gap is could-not-check.
func gapResult(gap string) string {
	switch gap {
	case GapInputChanged, GapInputSetChanged, GapInputUndeclaredAtRecord, GapEnvironmentChanged, GapToolChanged:
		return ResultWrongRevision
	case GapCommandCellMismatch, GapCommandChanged, GapExpectChanged, GapClassChanged, GapOutcomeUnparseable:
		return ResultError
	case GapOutcomeFail:
		return ResultFail
	}
	return ResultCouldNotCheck
}

// WitnessGap returns the gap name a v2 claim's reason starts with, or "".
func WitnessGap(reason string) string {
	if !strings.HasPrefix(reason, "witness-") {
		return ""
	}
	end := strings.IndexAny(reason, ": ")
	if end < 0 {
		return reason
	}
	return reason[:end]
}

// WitnessPolicy is the Human decision's environment rule: which runs count as
// environment-free. The zero value is the driver's ruling on the decision
// (option 1, 2026-10-25); the other options stay selectable so a later ruling
// is a one-value change and every option keeps its tests.
type WitnessPolicy int

const (
	// WitnessPolicyDefault is the ruled option, option 1.
	WitnessPolicyDefault WitnessPolicy = 0
	// WitnessPolicyReadableHost is option 1: the container runner, or a host or
	// back-filled run whose row is not `+env` and whose readable command names
	// no network tool. An unreadable host command is rerun-owed.
	WitnessPolicyReadableHost WitnessPolicy = 1
	// WitnessPolicyContainerOnly is option 2 (not ruled): only the pinned
	// network-less container runner counts.
	WitnessPolicyContainerOnly WitnessPolicy = 2
	// WitnessPolicyUnmarked is option 3: any run not marked `+env`.
	WitnessPolicyUnmarked WitnessPolicy = 3
	// WitnessPolicyWholeTreeOnly is option 4: option 2, and a declared-scope
	// record never releases.
	WitnessPolicyWholeTreeOnly WitnessPolicy = 4
)

func (p WitnessPolicy) effective() WitnessPolicy {
	if p == WitnessPolicyDefault {
		return WitnessPolicyReadableHost
	}
	return p
}

// ---------------------------------------------------------------------------
// The Runner cell
// ---------------------------------------------------------------------------

// RunnerCell is a witness row's Runner cell, parsed.
type RunnerCell struct {
	// Identity is the executing identity: the text before the first `@`, as
	// every identity read takes it.
	Identity string
	// Schema is WitnessSchemaV1, WitnessSchemaV2 or WitnessSchemaUnknown.
	Schema int
	// Tree is the token after `@` exactly as the v1 reader lifts it
	// (`@\s*([^\s(]+)`); for a v2 cell it is `[w2]`.
	Tree string
	// Marker is the bracketed schema marker, "" for v1.
	Marker string
	// CR and RecordID are a v2 cell's `cr:` and `rec:` values.
	CR       string
	RecordID string
	// Malformed is set when a v2 cell carries a token other than one `cr:` and
	// one `rec:` before its parentheticals.
	Malformed bool
}

var runnerTreeRe = regexp.MustCompile(`@\s*([^\s(]+)`)

// ParseRunnerCell parses a Runner cell. A cell with no bracketed marker after
// `@` is v1 and reads exactly as before; `[w2]` is v2; any other bracketed
// marker (`[w3]`, `[w]`, `[w2x]`, ...) is an unknown schema.
func ParseRunnerCell(cell string) RunnerCell {
	c := RunnerCell{Schema: WitnessSchemaV1}
	at := strings.IndexByte(cell, '@')
	if at < 0 {
		c.Identity = strings.TrimSpace(cell)
		return c
	}
	c.Identity = strings.TrimSpace(cell[:at])
	if m := runnerTreeRe.FindStringSubmatch(cell); m != nil {
		c.Tree = m[1]
	}
	if !strings.HasPrefix(c.Tree, "[") {
		return c
	}
	c.Marker = c.Tree
	if c.Tree != "[w2]" {
		c.Schema = WitnessSchemaUnknown
		return c
	}
	c.Schema = WitnessSchemaV2
	rest := strings.TrimSpace(cell[at+1:])
	rest = strings.TrimPrefix(rest, "[w2]")
	if i := strings.IndexByte(rest, '('); i >= 0 {
		rest = rest[:i]
	}
	for _, f := range strings.Fields(rest) {
		switch {
		case strings.HasPrefix(f, "cr:") && c.CR == "":
			c.CR = f[len("cr:"):]
		case strings.HasPrefix(f, "rec:") && c.RecordID == "":
			c.RecordID = f[len("rec:"):]
		default:
			c.Malformed = true
		}
	}
	return c
}

// witnessSchemaOf is the schema a Witness observation carries: from its Runner
// cell when the caller supplied one, else from its tree token alone.
func witnessSchemaOf(w Witness) RunnerCell {
	if w.Runner != "" {
		return ParseRunnerCell(w.Runner)
	}
	c := RunnerCell{Schema: WitnessSchemaV1, Tree: w.Tree}
	if strings.HasPrefix(w.Tree, "[") {
		c.Marker = w.Tree
		c.Schema = WitnessSchemaUnknown
		if w.Tree == "[w2]" {
			c.Schema = WitnessSchemaV2
		}
	}
	return c
}

// ---------------------------------------------------------------------------
// The record
// ---------------------------------------------------------------------------

// WitnessInformational holds the record's informational fields: inside the
// id, never read by the judgment.
type WitnessInformational struct {
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

// WitnessRecord is one v2 witness record. Every field is inside the record id.
type WitnessRecord struct {
	Schema        int                  `json:"schema"`
	Brief         string               `json:"brief"`
	Row           string               `json:"row"`
	CommandSHA256 string               `json:"command_sha256"`
	ExpectSHA256  string               `json:"expect_sha256"`
	ClassSHA256   string               `json:"class_sha256"`
	Runner        string               `json:"runner"`
	RunnerSource  string               `json:"runner_source"`
	RunnerClass   string               `json:"runner_class"`
	Network       string               `json:"network"`
	Tools         map[string]string    `json:"tools"`
	ToolsSource   string               `json:"tools_source"`
	Origin        string               `json:"origin"`
	SourceCR      string               `json:"source_cr,omitempty"`
	SourceCell    string               `json:"source_cell,omitempty"`
	Result        string               `json:"result"`
	Exit          string               `json:"exit"`
	OutputSHA256  string               `json:"output_sha256"`
	Date          string               `json:"date"`
	CR            string               `json:"cr"`
	Scope         string               `json:"scope"`
	Inputs        map[string]string    `json:"inputs"`
	Implicit      map[string]string    `json:"implicit"`
	Attributes    map[string]string    `json:"attributes"`
	TreeDigest    string               `json:"tree_digest"`
	DirtyPaths    []string             `json:"dirty_paths"`
	Informational WitnessInformational `json:"informational"`
}

var (
	witnessRecordKeys = []string{
		"attributes", "brief", "class_sha256", "command_sha256", "cr", "date", "dirty_paths",
		"exit", "expect_sha256", "implicit", "informational", "inputs", "network", "origin",
		"output_sha256", "result", "row", "runner", "runner_class", "runner_source", "schema",
		"scope", "tools", "tools_source", "tree_digest",
	}
	witnessRecordBackfillKeys = []string{"source_cell", "source_cr"}
	hex64Re                   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	objectIDRe                = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// EncodeWitnessRecord returns a record's canonical bytes and its id. It is the
// one encoder: the writer (statusgen/37) and the test fixtures use it, and the
// reader recomputes the same id from the file.
func EncodeWitnessRecord(rec WitnessRecord) (canonical []byte, id string, err error) {
	if rec.Tools == nil {
		rec.Tools = map[string]string{}
	}
	if rec.Inputs == nil {
		rec.Inputs = map[string]string{}
	}
	if rec.Implicit == nil {
		rec.Implicit = map[string]string{}
	}
	if rec.Attributes == nil {
		rec.Attributes = map[string]string{}
	}
	if rec.DirtyPaths == nil {
		rec.DirtyPaths = []string{}
	}
	text, err := encodeCanonicalJSON(recordToObject(rec))
	if err != nil {
		return nil, "", err
	}
	_, canonical, id, err = parseWitnessRecord([]byte(text))
	return canonical, id, err
}

// ParseWitnessRecord parses a record file and returns the record and the id
// recomputed from its canonical form. Any unknown key, missing key, duplicate
// key, wrong type or malformed digest is an error: the record is then a
// mismatch.
func ParseWitnessRecord(raw []byte) (WitnessRecord, string, error) {
	rec, _, id, err := parseWitnessRecord(raw)
	return rec, id, err
}

func parseWitnessRecord(raw []byte) (rec WitnessRecord, canonical []byte, id string, err error) {
	generic, err := decodeJSON(raw)
	if err != nil {
		return rec, nil, "", fmt.Errorf("the record is not JSON: %v", err)
	}
	obj, ok := generic.(map[string]any)
	if !ok {
		return rec, nil, "", errors.New("the record is not a JSON object")
	}
	allowed := map[string]bool{}
	for _, k := range witnessRecordKeys {
		allowed[k] = true
		if _, ok := obj[k]; !ok {
			return rec, nil, "", fmt.Errorf("the record has no %q", k)
		}
	}
	for _, k := range witnessRecordBackfillKeys {
		allowed[k] = true
	}
	for k := range obj {
		if !allowed[k] {
			return rec, nil, "", fmt.Errorf("the record carries the unknown field %q", k)
		}
	}
	text, err := encodeCanonicalJSON(obj)
	if err != nil {
		return rec, nil, "", err
	}
	if rec, err = recordFromObject(obj); err != nil {
		return WitnessRecord{}, nil, "", err
	}
	if err := rec.validate(); err != nil {
		return WitnessRecord{}, nil, "", err
	}
	canonical = []byte(text)
	return rec, canonical, sha256Hex(canonical), nil
}

// validate checks the record's closed value sets and digest shapes.
func (r WitnessRecord) validate() error {
	if r.Schema != WitnessSchemaV2 {
		return fmt.Errorf("the record's schema is %d, not 2", r.Schema)
	}
	for name, v := range map[string]string{"command_sha256": r.CommandSHA256, "expect_sha256": r.ExpectSHA256, "class_sha256": r.ClassSHA256} {
		if !hex64Re.MatchString(v) {
			return fmt.Errorf("%s is not a 64-hex digest", name)
		}
	}
	switch r.Origin {
	case "run":
	case "backfill":
		if r.SourceCR == "" || r.SourceCell == "" {
			return errors.New("a back-filled record carries no source_cr or source_cell")
		}
	default:
		return fmt.Errorf("origin %q is neither run nor backfill", r.Origin)
	}
	switch r.Scope {
	case WitnessScopeDeclared:
	case WitnessScopeWholeTree:
		if !hex64Re.MatchString(r.TreeDigest) {
			return errors.New("a whole-tree record carries no 64-hex tree_digest")
		}
	default:
		return fmt.Errorf("scope %q is neither declared nor whole-tree", r.Scope)
	}
	for _, m := range []map[string]string{r.Inputs, r.Implicit} {
		for p, v := range m {
			if err := witnessPathOK(p); err != nil {
				return err
			}
			if v != WitnessAbsent && !objectIDRe.MatchString(v) {
				return fmt.Errorf("the digest of %s is neither a blob id nor %q", p, WitnessAbsent)
			}
		}
	}
	for p, v := range r.Attributes {
		if err := witnessPathOK(p); err != nil {
			return err
		}
		if !hex64Re.MatchString(v) {
			return fmt.Errorf("the attribute digest of %s is not 64-hex", p)
		}
	}
	for _, p := range r.DirtyPaths {
		if err := witnessPathOK(p); err != nil {
			return err
		}
	}
	return nil
}

// witnessPathOK refuses a path that is not repo-relative and slash-separated:
// a backslash is refused, never normalised silently.
func witnessPathOK(p string) error {
	switch {
	case p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\"):
		return fmt.Errorf("the path %q is not repo-relative and slash-separated", p)
	case pathpkg.Clean(p) != p || p == "." || strings.HasPrefix(p, "../"):
		return fmt.Errorf("the path %q is not a clean repo-relative path", p)
	}
	return nil
}

// WitnessRecordPath is the record's path for brief id "<stream>/<NN>".
func WitnessRecordPath(briefID, recordID string) string {
	return WitnessRecordDir + briefID + "/" + recordID + ".json"
}

// WitnessTextDigest is the SHA-256 of a Command, Expect or Class cell after
// NormalizeCommandText — the normaliser v1's Command check uses, so step 3 and
// step 5 compare text the way v1 does.
func WitnessTextDigest(s string) string {
	return sha256Hex([]byte(NormalizeCommandText(s)))
}

// informationalRevision is the revision a v2 claim reports: the record's
// informational.commit when it is a full object id, else empty.
func informationalRevision(rec WitnessRecord) string {
	if objectIDRe.MatchString(rec.Informational.Commit) {
		return rec.Informational.Commit
	}
	return ""
}

// ---------------------------------------------------------------------------
// Declared inputs (`inputs:`)
// ---------------------------------------------------------------------------

var (
	inputsLabelRe   = regexp.MustCompile(`^inputs:(.*)$`)
	inputsPathRe    = regexp.MustCompile(`^[A-Za-z0-9._@+-]+(/[A-Za-z0-9._@+-]+)*/?$`)
	inputsBacktickR = regexp.MustCompile("`([^`]*)`")
)

// ParseInputsDeclaration reads the `inputs:` field of a brief's `## Context`
// section. It reads a strict, literal-only subset of the `files:` grammar
// (docs/declared-input-scope.md): an inline value or `- `/`* ` list items,
// each entry backticked or bare, separated by commas, semicolons or spaces; an
// em dash ends a value's entries (commentary follows). A directory ends with
// `/`. A glob, brace, `**`, backslash, absolute path or `..` makes the whole
// declaration unparseable, and so does any other token that is not a plain
// path: found is then false and problem says why, which means scope
// whole-tree. A bookkeeping entry (STATUS.md, docs/streams/**) is dropped
// from the set and reported in problem. found is false, problem empty, when
// the field is absent.
func ParseInputsDeclaration(context string) (entries []string, found bool, problem string) {
	lines := strings.Split(strings.ReplaceAll(context, "\r\n", "\n"), "\n")
	label := -1
	inline := ""
	for i, l := range lines {
		if m := inputsLabelRe.FindStringSubmatch(l); m != nil {
			label, inline = i, strings.TrimSpace(m[1])
			break
		}
	}
	if label < 0 {
		return nil, false, ""
	}
	var values []string
	if inline != "" {
		values = append(values, inline)
	}
	for _, l := range lines[label+1:] {
		t := strings.TrimSpace(l)
		if t == "" {
			break
		}
		if !strings.HasPrefix(t, "- ") && !strings.HasPrefix(t, "* ") {
			if l != t && len(values) > 0 { // an indented continuation of the last value
				values[len(values)-1] += " " + t
				continue
			}
			break
		}
		if inline != "" {
			return nil, false, "`inputs:` has both an inline value and list items"
		}
		values = append(values, t[2:])
	}
	if len(values) == 0 {
		return nil, false, "`inputs:` names no entry"
	}
	seen := map[string]bool{}
	var bookkeeping []string
	for _, v := range values {
		if i := strings.Index(v, "—"); i >= 0 {
			v = v[:i]
		}
		if strings.Count(v, "`")%2 != 0 {
			return nil, false, "`inputs:` has an unclosed backtick"
		}
		var toks []string
		for _, m := range inputsBacktickR.FindAllStringSubmatch(v, -1) {
			toks = append(toks, m[1])
		}
		outside := inputsBacktickR.ReplaceAllString(v, " ")
		toks = append(toks, strings.FieldsFunc(outside, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\t'
		})...)
		for _, tok := range toks {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			tok = strings.TrimPrefix(tok, "./")
			if !inputsPathRe.MatchString(tok) {
				return nil, false, fmt.Sprintf("`inputs:` entry %q is not a literal repo-relative path (globs, braces, backslashes, absolute paths and prose are not read)", tok)
			}
			for _, seg := range strings.Split(strings.TrimSuffix(tok, "/"), "/") {
				if seg == ".." || seg == "." {
					return nil, false, fmt.Sprintf("`inputs:` entry %q climbs out of or names its own directory", tok)
				}
			}
			if isBoardBookkeepingPath(tok) || isBoardBookkeepingPath(strings.TrimSuffix(tok, "/")) {
				bookkeeping = append(bookkeeping, tok)
				continue
			}
			if !seen[tok] {
				seen[tok] = true
				entries = append(entries, tok)
			}
		}
	}
	sort.Strings(entries)
	if len(bookkeeping) > 0 {
		problem = "`inputs:` names bookkeeping path(s) " + strings.Join(bookkeeping, ", ") + " (STATUS.md and docs/streams/** are never inputs)"
	}
	return entries, len(entries) > 0, problem
}

// ---------------------------------------------------------------------------
// The required set and the digests (the one digest owner)
// ---------------------------------------------------------------------------

// TreeBlob is one entry of a recursive tree listing with its object id.
type TreeBlob struct {
	Mode string
	OID  string
	Path string
}

// WitnessSpec is what the required set is derived from.
type WitnessSpec struct {
	// Declared are the parsed `inputs:` entries; WholeTree is set when the
	// declaration is absent or unparseable (Declared is then ignored).
	Declared  []string
	WholeTree bool
	// Extra are the always-required implicit paths besides the pin file: the
	// row's WI-2 policy and build paths.
	Extra []string
}

// WitnessContent is the canonical v2 content of a revision for one row: the
// record fields a writer records and the reader recomputes.
type WitnessContent struct {
	Scope      string            `json:"scope"`
	Inputs     map[string]string `json:"inputs"`
	Implicit   map[string]string `json:"implicit"`
	Attributes map[string]string `json:"attributes"`
	TreeDigest string            `json:"tree_digest"`
	DirtyPaths []string          `json:"dirty_paths,omitempty"`
}

// treeIndex is a listing indexed for the required-set derivation.
type treeIndex struct {
	byPath map[string]TreeBlob
	paths  []string // sorted
}

func newTreeIndex(listing []TreeBlob) *treeIndex {
	ix := &treeIndex{byPath: make(map[string]TreeBlob, len(listing))}
	for _, e := range listing {
		ix.byPath[e.Path] = e
		ix.paths = append(ix.paths, e.Path)
	}
	sort.Strings(ix.paths)
	return ix
}

// under lists every path under directory prefix d (d ends with "/").
func (ix *treeIndex) under(d string) []string {
	i := sort.SearchStrings(ix.paths, d)
	var out []string
	for ; i < len(ix.paths) && strings.HasPrefix(ix.paths[i], d); i++ {
		out = append(out, ix.paths[i])
	}
	return out
}

// isDir reports whether some path lies under d.
func (ix *treeIndex) isDir(d string) bool {
	d = strings.TrimSuffix(d, "/") + "/"
	i := sort.SearchStrings(ix.paths, d)
	return i < len(ix.paths) && strings.HasPrefix(ix.paths[i], d)
}

// moduleFileNames are the module and lock files on an input's path to the
// root that are implicit inputs where they exist (requirements*.txt aside).
var moduleFileNames = []string{
	"go.mod", "go.sum", "go.work", "go.work.sum", "package.json", "package-lock.json",
	"pnpm-lock.yaml", "yarn.lock", "Cargo.toml", "Cargo.lock", "pyproject.toml", "uv.lock", "poetry.lock",
}

// requiredSet is the required set of one row at one revision.
type requiredSet struct {
	inputs   []string // sorted
	implicit []string // sorted
	declared []string // the declared entries the set came from (declared scope)
}

func (rs requiredSet) contains(p string) bool {
	return containsSorted(rs.inputs, p) || containsSorted(rs.implicit, p)
}

func containsSorted(s []string, p string) bool {
	i := sort.SearchStrings(s, p)
	return i < len(s) && s[i] == p
}

// requiredWitnessSet derives the required set at a revision from its listing.
func requiredWitnessSet(spec WitnessSpec, ix *treeIndex) requiredSet {
	var rs requiredSet
	inputs := map[string]bool{}
	if !spec.WholeTree {
		rs.declared = append([]string(nil), spec.Declared...)
		for _, d := range spec.Declared {
			if strings.HasSuffix(d, "/") {
				for _, p := range ix.under(d) {
					if !isBoardBookkeepingPath(p) {
						inputs[p] = true
					}
				}
				continue
			}
			inputs[d] = true // required whether or not it exists
		}
	}
	implicit := map[string]bool{WitnessPinFile: true}
	for _, p := range spec.Extra {
		implicit[p] = true
	}
	dirs := map[string]bool{}
	for p := range inputs {
		for d := pathpkg.Dir(p); ; d = pathpkg.Dir(d) {
			if dirs[d] {
				break
			}
			dirs[d] = true
			if d == "." {
				break
			}
		}
	}
	for d := range dirs {
		prefix := ""
		if d != "." {
			prefix = d + "/"
		}
		for _, name := range moduleFileNames {
			if _, ok := ix.byPath[prefix+name]; ok {
				implicit[prefix+name] = true
			}
		}
		for _, p := range ix.under(prefix) {
			name := strings.TrimPrefix(p, prefix)
			if !strings.Contains(name, "/") && strings.HasPrefix(name, "requirements") && strings.HasSuffix(name, ".txt") {
				implicit[p] = true
			}
		}
	}
	rs.inputs = sortedKeys(inputs)
	rs.implicit = sortedKeys(implicit)
	return rs
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// AttributeDigest is the SHA-256 of a path's resolved attribute set: its
// sorted `<attr>: <value>` lines, each newline-terminated.
func AttributeDigest(lines []string) string {
	s := append([]string(nil), lines...)
	sort.Strings(s)
	var b strings.Builder
	for _, l := range s {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return sha256Hex([]byte(b.String()))
}

// TreeDigest is the SHA-256 of the sorted `<mode> <blob id> <path>` lines of
// every non-bookkeeping entry of a listing.
func TreeDigest(listing []TreeBlob) string {
	kept := make([]TreeBlob, 0, len(listing))
	for _, e := range listing {
		if !isBoardBookkeepingPath(e.Path) {
			kept = append(kept, e)
		}
	}
	// Sort by path (then mode and id), never by a substring of the rendered
	// line: a path holding a space must not make the digest depend on input order.
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].Path != kept[j].Path {
			return kept[i].Path < kept[j].Path
		}
		if kept[i].Mode != kept[j].Mode {
			return kept[i].Mode < kept[j].Mode
		}
		return kept[i].OID < kept[j].OID
	})
	var b strings.Builder
	for _, e := range kept {
		b.WriteString(e.Mode + " " + e.OID + " " + e.Path + "\n")
	}
	return sha256Hex([]byte(b.String()))
}

// ComputeWitnessContent computes the canonical v2 content of one row at one
// revision, from that revision's listing and resolved attributes. It is the
// only place a digest is computed: the writer records it, `statusgen
// witnessdigest` prints it, and the reader compares with the same derivation.
func ComputeWitnessContent(spec WitnessSpec, listing []TreeBlob, attrs map[string][]string) WitnessContent {
	ix := newTreeIndex(listing)
	rs := requiredWitnessSet(spec, ix)
	c := WitnessContent{Scope: WitnessScopeDeclared, Inputs: map[string]string{}, Implicit: map[string]string{}, Attributes: map[string]string{}}
	if spec.WholeTree {
		c.Scope = WitnessScopeWholeTree
		c.TreeDigest = TreeDigest(listing)
	}
	blob := func(p string) string {
		if e, ok := ix.byPath[p]; ok {
			return e.OID
		}
		return WitnessAbsent
	}
	for _, p := range rs.inputs {
		c.Inputs[p] = blob(p)
		c.Attributes[p] = AttributeDigest(attrs[p])
	}
	for _, p := range rs.implicit {
		c.Implicit[p] = blob(p)
	}
	return c
}

// WorktreeListing overlays working-tree blob ids (HashWorktree's answer, clean
// filters and end-of-line attributes applied) on a committed listing, so
// ComputeWitnessContent digests the working tree exactly as it digests a
// commit. dirty lists, sorted, the paths whose working-tree id differs from
// the committed one or that the commit does not hold.
func WorktreeListing(committed []TreeBlob, worktree map[string]string) (listing []TreeBlob, dirty []string) {
	seen := map[string]bool{}
	for _, e := range committed {
		seen[e.Path] = true
		if oid, ok := worktree[e.Path]; ok && oid != e.OID {
			e.OID = oid
			dirty = append(dirty, e.Path)
		}
		listing = append(listing, e)
	}
	for p, oid := range worktree {
		if !seen[p] {
			listing = append(listing, TreeBlob{Mode: "100644", OID: oid, Path: p})
			dirty = append(dirty, p)
		}
	}
	sort.Strings(dirty)
	sort.Slice(listing, func(i, j int) bool { return listing[i].Path < listing[j].Path })
	return listing, dirty
}

// ---------------------------------------------------------------------------
// The literal-path lower bound
// ---------------------------------------------------------------------------

// commandLiteralPaths lists the repo-relative paths a Command names literally
// (a `cd` target, a package path, a file argument), as candidates; readable is
// false when the library cannot read the command, which adds no bound. The
// caller keeps only candidates that exist at the revision.
func commandLiteralPaths(command, shell string) (paths []string, readable bool) {
	if shell != "" && shell != ShellSh {
		return nil, false
	}
	words, err := lexRow(UnescapePipes(command))
	if err != nil {
		return nil, false
	}
	cwd := ""
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for i := 0; i < len(words); i++ {
		w := words[i]
		if w.op != "" {
			continue
		}
		if w.text == "cd" && i+2 < len(words) && words[i+1].op == "" && words[i+2].op == "&&" && (i == 0 || words[i-1].op != "") {
			if d, ok := literalPath(cwd, words[i+1].text); ok {
				cwd = d
				add(d)
			}
			i++
			continue
		}
		tok := strings.TrimSuffix(strings.TrimSuffix(w.text, "..."), "/")
		if tok == "" || tok == "." {
			continue
		}
		if p, ok := literalPath(cwd, tok); ok {
			add(p)
		}
	}
	return paths, true
}

// literalPath resolves one command word against the row's working directory
// for the literal-path bound. Unlike the input derivation's rowPath it follows
// `..`: a word that climbs out of its `cd` directory still names a repository
// path, and skipping it would let a Command read outside `inputs:` unbounded.
// Words that are flags, absolute, globbed, or resolve outside the repository
// (or to its root or .git) name no candidate.
func literalPath(cwd, w string) (string, bool) {
	if w == "" || w == "-" || strings.HasPrefix(w, "-") || strings.HasPrefix(w, "/") || strings.ContainsAny(w, "\\:*?[]=$") {
		return "", false
	}
	p := pathpkg.Clean(pathpkg.Join(cwd, w))
	if p == "." || p == ".." || strings.HasPrefix(p, "../") || p == ".git" || strings.HasPrefix(p, ".git/") {
		return "", false
	}
	return p, true
}

// insideDeclared reports whether a path the command names lies inside the
// declared set: a file is a declared file or lies under a declared directory;
// a directory is, or lies under, a declared directory, or holds a declared
// entry.
func insideDeclared(declared []string, p string, isDir bool) bool {
	for _, d := range declared {
		if strings.HasSuffix(d, "/") {
			if strings.HasPrefix(p+"/", d) || (isDir && strings.HasPrefix(d, p+"/")) {
				return true
			}
			continue
		}
		if d == p || (isDir && strings.HasPrefix(d, p+"/")) {
			return true
		}
	}
	return false
}

// InputsBoundViolations lists the paths a Verify row's Command names literally
// that exist (per exists) and lie outside the declared `inputs:` set — the
// literal-path lower bound `--lint` reports as a PROBLEM and the reader
// refuses as witness-input-undeclared. A command the library cannot read adds
// no bound.
func InputsBoundViolations(declared []string, r VerifyRow, exists func(p string) (exists, isDir bool)) []string {
	cands, readable := commandLiteralPaths(r.Command, r.Shell)
	if !readable {
		return nil
	}
	var out []string
	for _, p := range cands {
		ok, dir := exists(p)
		if !ok || isBoardBookkeepingPath(p) {
			continue
		}
		if !insideDeclared(declared, p, dir) {
			out = append(out, p)
		}
	}
	return out
}

// networkTools are the commands option 1 reads as using the network.
var networkTools = map[string]bool{
	"curl": true, "wget": true, "gh": true, "glab": true, "kubectl": true, "helm": true,
	"ssh": true, "scp": true, "rsync": true, "nc": true, "ncat": true, "telnet": true, "ftp": true,
	"sftp": true, "dig": true, "nslookup": true, "ping": true, "docker": true, "podman": true,
	"npm": true, "npx": true, "pnpm": true, "yarn": true, "pip": true, "pip3": true, "cargo": true,
	"aws": true, "gcloud": true, "az": true, "terraform": true, "vault": true, "bao": true,
}

var networkGitVerbs = map[string]bool{"fetch": true, "pull": true, "push": true, "clone": true, "ls-remote": true, "submodule": true, "remote": true}

// opaqueRunners are build-runner commands whose recipe is a file the library
// does not read: a command that runs one is unreadable, never "no network".
var opaqueRunners = map[string]bool{
	"make": true, "gmake": true, "just": true, "task": true, "mage": true, "ninja": true,
	"bazel": true, "bazelisk": true, "gradle": true, "gradlew": true, "mvn": true, "mvnw": true,
	"eval": true,
}

// shellInterpreters run the string after -c as a command.
var shellInterpreters = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "ash": true}

// wrapperSpec describes how a wrapper command's own words are skipped to reach
// the command it runs: the number of leading operands consumed after the
// wrapper's flags, and which flags take a value.
type wrapperSpec struct {
	valueFlags map[string]bool // flags that consume the next word
	operands   int             // positional operands before the wrapped command
	assigns    bool            // NAME=value words are skipped (env)
}

var commandWrappers = map[string]wrapperSpec{
	"env":        {valueFlags: map[string]bool{"-u": true, "-C": true, "-S": true, "--unset": true, "--chdir": true}, assigns: true},
	"timeout":    {valueFlags: map[string]bool{"-k": true, "-s": true, "--kill-after": true, "--signal": true}, operands: 1},
	"nice":       {valueFlags: map[string]bool{"-n": true, "--adjustment": true}},
	"nohup":      {},
	"sudo":       {valueFlags: map[string]bool{"-u": true, "-g": true, "-h": true, "-p": true, "-C": true, "-D": true, "-R": true, "-T": true, "-r": true, "-t": true, "-U": true}},
	"doas":       {valueFlags: map[string]bool{"-u": true, "-C": true}},
	"xargs":      {valueFlags: map[string]bool{"-I": true, "-n": true, "-P": true, "-d": true, "-E": true, "-L": true, "-s": true, "-a": true, "-J": true}},
	"command":    {},
	"exec":       {valueFlags: map[string]bool{"-a": true}},
	"time":       {valueFlags: map[string]bool{"-f": true, "-o": true}},
	"stdbuf":     {valueFlags: map[string]bool{"-i": true, "-o": true, "-e": true}},
	"setsid":     {},
	"ionice":     {valueFlags: map[string]bool{"-c": true, "-n": true, "-p": true}},
	"chrt":       {operands: 1},
	"caffeinate": {valueFlags: map[string]bool{"-t": true, "-w": true}},
}

func isEnvAssign(w string) bool {
	i := strings.IndexByte(w, '=')
	if i <= 0 {
		return false
	}
	for j := 0; j < i; j++ {
		c := w[j]
		if !(c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || j > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// commandNamesNetworkTool reads a command for option 1: readable is false when
// the library cannot read it; network is true when a simple command runs a
// network tool (looking through the common wrappers, `sh -c` strings and
// `find -exec`), git with a network verb, a networked go command, or a word is
// a URL. An opaque runner (make, eval, ...) is unreadable.
func commandNamesNetworkTool(command, shell string) (network, readable bool) {
	if shell != "" && shell != ShellSh {
		return false, false
	}
	return networkInCommand(UnescapePipes(command), 0)
}

const maxShellNesting = 3

func networkInCommand(command string, depth int) (network, readable bool) {
	if depth > maxShellNesting {
		return false, false
	}
	words, err := lexRow(command)
	if err != nil {
		return false, false
	}
	readable = true
	var simple []shellWord
	flush := func() {
		if len(simple) == 0 {
			return
		}
		n, r := networkInSimple(simple, depth)
		network = network || n
		readable = readable && r
		simple = nil
	}
	for _, w := range words {
		if w.op != "" {
			flush()
			continue
		}
		simple = append(simple, w)
	}
	flush()
	if network {
		return true, true
	}
	return false, readable
}

// networkInSimple judges one simple command (the words between operators).
func networkInSimple(words []shellWord, depth int) (network, readable bool) {
	for _, w := range words {
		if strings.Contains(w.text, "://") {
			return true, true
		}
	}
	i := 0
	for i < len(words) && isEnvAssign(words[i].text) {
		i++
	}
	// Look through wrappers to the command that actually runs.
	for i < len(words) {
		name := pathpkg.Base(words[i].text)
		spec, wrapped := commandWrappers[name]
		if !wrapped {
			break
		}
		i++
		for i < len(words) {
			t := words[i].text
			if spec.assigns && isEnvAssign(t) {
				i++
				continue
			}
			if strings.HasPrefix(t, "-") && t != "-" {
				i++
				if spec.valueFlags[t] && i < len(words) {
					i++
				}
				continue
			}
			break
		}
		for n := 0; n < spec.operands && i < len(words); n++ {
			i++
		}
		for i < len(words) && spec.assigns && isEnvAssign(words[i].text) {
			i++
		}
	}
	if i >= len(words) {
		return false, true
	}
	name := pathpkg.Base(words[i].text)
	rest := words[i+1:]
	switch {
	case networkTools[name]:
		return true, true
	case opaqueRunners[name]:
		return false, false
	case shellInterpreters[name]:
		for j, a := range rest {
			if len(a.text) >= 2 && a.text[0] == '-' && a.text[1] != '-' && strings.Contains(a.text[1:], "c") {
				if j+1 >= len(rest) {
					return false, false
				}
				return networkInCommand(rest[j+1].text, depth+1)
			}
		}
		return false, true // a script file: judged by its argument words only
	case name == "git":
		for _, n := range rest {
			if networkGitVerbs[n.text] {
				return true, true
			}
		}
	case name == "go":
		for k, a := range rest {
			if strings.HasPrefix(a.text, "-") {
				continue
			}
			switch a.text {
			case "get", "install":
				return true, true
			case "run":
				for _, b := range rest[k+1:] {
					if strings.Contains(b.text, "@") && !strings.HasPrefix(b.text, "-") {
						return true, true
					}
				}
			case "mod":
				if k+1 < len(rest) && (rest[k+1].text == "download" || rest[k+1].text == "tidy" || rest[k+1].text == "vendor") {
					return true, true
				}
			}
			break
		}
	case name == "find":
		for k, a := range rest {
			if (a.text == "-exec" || a.text == "-execdir" || a.text == "-ok" || a.text == "-okdir") && k+1 < len(rest) {
				return networkInSimple(rest[k+1:], depth)
			}
		}
	}
	return false, true
}

// ---------------------------------------------------------------------------
// The landing-form table and step 2
// ---------------------------------------------------------------------------

// landingForm is one recognised change-request landing: its text pattern and
// the commit shape it fixes.
type landingForm struct {
	name    string
	re      *regexp.Regexp
	subject bool // matched against the subject line only
	parents int
	// recordAtSecondParent: the record is present at the second parent and
	// absent at the first (the merge-commit forms).
	recordAtSecondParent bool
}

// landingForms is the one table of recognised landing forms.
var landingForms = []landingForm{
	{name: "github-merge", re: regexp.MustCompile(`^Merge pull request #([0-9]+) from \S`), subject: true, parents: 2, recordAtSecondParent: true},
	{name: "github-squash", re: regexp.MustCompile(`\(#([0-9]+)\)\s*$`), subject: true, parents: 1},
	{name: "gitlab-merge", re: regexp.MustCompile(`(?m)^See merge request \S*!([0-9]+)\s*$`), parents: 2, recordAtSecondParent: true},
}

// matchLandingForm returns the first form whose text the message matches, and
// the change-request number it names.
func matchLandingForm(message string) (landingForm, string, bool) {
	subject, _, _ := strings.Cut(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	for _, f := range landingForms {
		text := message
		if f.subject {
			text = subject
		}
		if m := f.re.FindStringSubmatch(text); m != nil {
			return f, m[1], true
		}
	}
	return landingForm{}, "", false
}

// LandingCR names the change request a commit lands per the landing-form
// table, when its message matches a form and its parent count fits that form's
// shape; "" otherwise. It is the v1 `landing_cr` report; it proves nothing on
// its own.
func LandingCR(message string, parents int) string {
	f, n, ok := matchLandingForm(message)
	if !ok || parents != f.parents {
		return ""
	}
	return n
}

// normalizeCR reduces a change-request id to its number: `#12`, `!12` and `12`
// are the same id. ok is false for anything else.
func normalizeCR(cr string) (string, bool) {
	n := strings.TrimLeft(cr, "#!")
	if n == "" || len(cr)-len(n) > 1 || strings.Trim(n, "0123456789") != "" {
		return "", false
	}
	return n, true
}

// Forge is the optional `--forge` seam: what only the code-hosting service can
// say. A nil Forge is offline.
type Forge interface {
	// DefaultBranch names the default branch, ok false when unknown.
	DefaultBranch() (branch string, ok bool)
	// LandedCommit reports whether change request cr (its number) is merged and
	// the commit it landed as; ok false when the forge could not answer.
	LandedCommit(cr string) (commit string, merged, ok bool)
}

// WitnessAddition is one first-parent commit that added a witness record.
type WitnessAddition struct {
	Path    string
	Commit  string
	Parents []string
	Message string
}

// WitnessRepo is the Repo port's v2 extension: batched, per-revision reads. A
// Repo that does not implement it judges every v2 claim could-not-check
// (witness-unestablished). Every read runs with replace objects disabled.
type WitnessRepo interface {
	// ListTree lists every entry of rev's tree with mode and object id
	// (`git ls-tree -r`).
	ListTree(rev string) ([]TreeBlob, bool)
	// CheckAttr resolves every attribute of the given paths at rev in one
	// batch (`git check-attr -a --source=<rev> --stdin`): path → `<attr>:
	// <value>` lines; a path with no attribute is absent from the map.
	CheckAttr(rev string, paths []string) (map[string][]string, bool)
	// CatObjects reads the named objects in one batch (`git cat-file
	// --batch`). A spec is a commit id or `<rev>:<path>`; a spec that names no
	// object is absent from the map.
	CatObjects(specs []string) (map[string]CatObject, bool)
	// WitnessAdditions lists, newest first, every first-parent commit on rev's
	// history that added a file under WitnessRecordDir
	// (`git log --first-parent --diff-filter=A`).
	WitnessAdditions(rev string) ([]WitnessAddition, bool)
	// FirstParents lists tip's first-parent history.
	FirstParents(tip string) ([]string, bool)
	// ShallowCommits lists the clone's shallow-boundary commits (empty when
	// the clone is complete).
	ShallowCommits() (map[string]bool, bool)
	// ReplacedObjects lists every object a `refs/replace/` ref replaces.
	ReplacedObjects() (map[string]bool, bool)
	// HashWorktree returns the blob id `git add` would store for each working
	// tree path (clean filters and end-of-line attributes applied).
	HashWorktree(paths []string) (map[string]string, bool)
}

// CatObject is one object CatObjects read: its id, type and content.
type CatObject struct {
	OID     string
	Type    string
	Content []byte
}

// v2Ctx is one EvaluateCoverage call's v2 read cache, alongside evalCtx.
type v2Ctx struct {
	listings   map[string]*treeIndex
	listingRaw map[string][]TreeBlob
	attrs      map[string]map[string][]string
	objects    map[string]*CatObject // spec → object, nil when the spec names none
	additions  map[string]map[string]WitnessAddition
	firstPar   map[string]map[string]bool
	pins       map[string]map[string]string
}

func newV2Ctx() *v2Ctx {
	return &v2Ctx{
		listings: map[string]*treeIndex{}, listingRaw: map[string][]TreeBlob{}, attrs: map[string]map[string][]string{},
		objects: map[string]*CatObject{}, additions: map[string]map[string]WitnessAddition{},
		firstPar: map[string]map[string]bool{}, pins: map[string]map[string]string{},
	}
}

func (x *evalCtx) witnessRepo() (WitnessRepo, bool) {
	wr, ok := x.repo.(WitnessRepo)
	return wr, ok
}

func (x *evalCtx) listing(rev string) (*treeIndex, []TreeBlob, bool) {
	if ix, ok := x.v2.listings[rev]; ok {
		return ix, x.v2.listingRaw[rev], ix != nil
	}
	wr, _ := x.witnessRepo()
	raw, ok := wr.ListTree(rev)
	if !ok {
		x.v2.listings[rev] = nil
		return nil, nil, false
	}
	ix := newTreeIndex(raw)
	x.v2.listings[rev], x.v2.listingRaw[rev] = ix, raw
	return ix, raw, true
}

func (x *evalCtx) attrsAt(rev string) (map[string][]string, bool) {
	if a, ok := x.v2.attrs[rev]; ok {
		return a, a != nil
	}
	_, raw, ok := x.listing(rev)
	if !ok {
		x.v2.attrs[rev] = nil
		return nil, false
	}
	paths := make([]string, len(raw))
	for i, e := range raw {
		paths[i] = e.Path
	}
	wr, _ := x.witnessRepo()
	a, ok := wr.CheckAttr(rev, paths)
	if !ok {
		x.v2.attrs[rev] = nil
		return nil, false
	}
	if a == nil {
		a = map[string][]string{}
	}
	x.v2.attrs[rev] = a
	return a, true
}

// fetchObjects reads every spec not yet cached in one batch. A failed read
// caches nothing, so the caller reports it as unestablished.
func (x *evalCtx) fetchObjects(specs []string) bool {
	wr, ok := x.witnessRepo()
	if !ok {
		return false
	}
	var want []string
	seen := map[string]bool{}
	for _, s := range specs {
		if _, cached := x.v2.objects[s]; !cached && !seen[s] {
			seen[s] = true
			want = append(want, s)
		}
	}
	if len(want) == 0 {
		return true
	}
	got, ok := wr.CatObjects(want)
	if !ok {
		return false
	}
	for _, s := range want {
		if o, has := got[s]; has {
			o := o
			x.v2.objects[s] = &o
		} else {
			x.v2.objects[s] = nil
		}
	}
	return true
}

// object reads one spec: from the cache, else one read.
func (x *evalCtx) object(spec string) (obj *CatObject, ok bool) {
	if o, cached := x.v2.objects[spec]; cached {
		return o, true
	}
	if !x.fetchObjects([]string{spec}) {
		return nil, false
	}
	return x.v2.objects[spec], true
}

func revPath(rev, path string) string { return rev + ":" + path }

// recordAt reads one file's content at rev.
func (x *evalCtx) recordAt(rev, path string) (raw []byte, found, ok bool) {
	o, ok := x.object(revPath(rev, path))
	if !ok {
		return nil, false, false
	}
	if o == nil || o.Type != "blob" {
		return nil, false, true
	}
	return o.Content, true, true
}

func (x *evalCtx) additionOf(rev, path string) (WitnessAddition, bool, bool) {
	m, has := x.v2.additions[rev]
	if !has {
		wr, _ := x.witnessRepo()
		list, ok := wr.WitnessAdditions(rev)
		if !ok {
			x.v2.additions[rev] = nil
			return WitnessAddition{}, false, false
		}
		m = map[string]WitnessAddition{}
		for _, a := range list {
			if _, seen := m[a.Path]; !seen { // newest first: the latest addition wins
				m[a.Path] = a
			}
		}
		x.v2.additions[rev] = m
	}
	if m == nil {
		return WitnessAddition{}, false, false
	}
	a, found := m[path]
	return a, found, true
}

func (x *evalCtx) onFirstParents(tip, commit string) (on, ok bool) {
	set, has := x.v2.firstPar[tip]
	if !has {
		wr, _ := x.witnessRepo()
		list, ok := wr.FirstParents(tip)
		if !ok {
			x.v2.firstPar[tip] = nil
			return false, false
		}
		set = map[string]bool{}
		for _, c := range list {
			set[c] = true
		}
		x.v2.firstPar[tip] = set
	}
	if set == nil {
		return false, false
	}
	return set[commit], true
}

// defaultBranchTip resolves the default branch: refs/remotes/origin/HEAD;
// else the branch the forge reports; else refs/remotes/origin/main; else the
// local main.
func (x *evalCtx) defaultBranchTip() (string, bool) {
	candidates := []string{"refs/remotes/origin/HEAD"}
	if x.forge != nil {
		if b, ok := x.forge.DefaultBranch(); ok && b != "" {
			candidates = append(candidates, "refs/remotes/origin/"+b, "refs/heads/"+b)
		}
	}
	candidates = append(candidates, "refs/remotes/origin/main", "refs/heads/main")
	for _, c := range candidates {
		if id, ok := x.repo.ResolveCommit(c); ok && id != "" {
			return id, true
		}
	}
	return "", false
}

func gapf(gap, format string, a ...any) string {
	if format == "" {
		return gap
	}
	return gap + ": " + fmt.Sprintf(format, a...)
}

// checkLanding is step 2: the change request is bound, agrees with the cell,
// and is verified by the first-parent commit on R's history that added the
// record. gap is "" when it holds.
func (x *evalCtx) checkLanding(sc witnessScope, cell RunnerCell, rec WitnessRecord, R, recPath string) (gap, reason string) {
	if cell.CR == "" || cell.CR == "pending" || rec.CR == "" || rec.CR == "pending" {
		return GapCRUnbound, gapf(GapCRUnbound, "the record's change request is %q and the cell's %q — a record binds only to a delivering change request, never to pending", rec.CR, cell.CR)
	}
	if cell.CR != rec.CR {
		return GapCRMismatch, gapf(GapCRMismatch, "the cell names cr:%s, the record cr:%s", cell.CR, rec.CR)
	}
	n, ok := normalizeCR(rec.CR)
	if !ok {
		return GapCRUnbound, gapf(GapCRUnbound, "%q is not a change-request id", rec.CR)
	}
	wr, _ := x.witnessRepo()
	add, found, ok := x.additionOf(R, recPath)
	if !ok {
		return GapUnestablished, gapf(GapUnestablished, "the first-parent log of witness additions could not be read")
	}
	if !found {
		return GapCRUnverified, gapf(GapCRUnverified, "no first-parent commit on the item's history added %s", recPath)
	}
	replaced, ok := wr.ReplacedObjects()
	if !ok {
		return GapUnestablished, gapf(GapUnestablished, "the replace refs could not be listed")
	}
	if replaced[add.Commit] {
		return GapUnestablished, gapf(GapUnestablished, "replace ref: a refs/replace/ ref names the landing commit %s, so its parent shape is not judged", add.Commit)
	}
	shallow, ok := wr.ShallowCommits()
	if !ok {
		return GapUnestablished, gapf(GapUnestablished, "the shallow boundary could not be read")
	}
	if shallow[add.Commit] {
		return GapUnestablished, gapf(GapUnestablished, "shallow — the landing commit %s is at the clone's shallow boundary; fetch the full history", add.Commit)
	}
	if !x.fetchObjects(landingSpecs(add, recPath, sc.briefRel)) {
		return GapUnestablished, gapf(GapUnestablished, "the landing commit's objects could not be read")
	}
	for _, p := range add.Parents {
		if replaced[p] {
			return GapUnestablished, gapf(GapUnestablished, "replace ref: a refs/replace/ ref names the landing commit's parent %s", p)
		}
		if o, _ := x.object(p); o == nil || o.Type != "commit" {
			return GapUnestablished, gapf(GapUnestablished, "shallow — the landing commit's parent %s is not in the object store", p)
		}
	}
	form, named, matched := matchLandingForm(add.Message)
	forgeConfirmed := false
	if x.forge != nil {
		landed, merged, ok := x.forge.LandedCommit(n)
		if !ok {
			return GapUnestablished, gapf(GapUnestablished, "the forge could not say whether change request %s merged", n)
		}
		if !merged || landed != add.Commit {
			return GapCRUnverified, gapf(GapCRUnverified, "the forge reports change request %s landed as %q, not as the adding commit %s", n, landed, add.Commit)
		}
		forgeConfirmed = true
	}
	if matched {
		if named != n {
			return GapCRUnverified, gapf(GapCRUnverified, "the landing commit %s names change request %s, not %s", add.Commit, named, n)
		}
		if len(add.Parents) != form.parents {
			return GapCRUnverified, gapf(GapCRUnverified, "the landing commit %s matches the %s form but has %d parent(s), not %d", add.Commit, form.name, len(add.Parents), form.parents)
		}
		if form.recordAtSecondParent {
			if o, _ := x.object(revPath(add.Parents[0], recPath)); o != nil {
				return GapCRUnverified, gapf(GapCRUnverified, "the merge commit's first parent %s already holds the record", add.Parents[0])
			}
			if o, _ := x.object(revPath(add.Parents[1], recPath)); o == nil {
				return GapCRUnverified, gapf(GapCRUnverified, "the merge commit's second parent %s does not hold the record (the merge itself added it)", add.Parents[1])
			}
		}
	} else if !forgeConfirmed {
		return GapCRUnverified, gapf(GapCRUnverified, "the commit %s that added the record lands in no recognised form (a fast-forward or rebase landing is verifiable only with --forge)", add.Commit)
	}
	if len(add.Parents) == 0 {
		return GapCRUnverified, gapf(GapCRUnverified, "the landing commit %s has no parent", add.Commit)
	}
	before, _ := x.object(revPath(add.Parents[0], sc.briefRel))
	after, _ := x.object(revPath(add.Commit, sc.briefRel))
	if sc.briefRel == "" || before == after || (before != nil && after != nil && before.OID == after.OID) {
		return GapCRUnverified, gapf(GapCRUnverified, "the landing commit %s does not change the citing brief %s", add.Commit, sc.briefRel)
	}
	if forgeConfirmed {
		return "", ""
	}
	tip, ok := x.defaultBranchTip()
	if !ok {
		return GapCRUnverified, gapf(GapCRUnverified, "no default branch resolves (refs/remotes/origin/HEAD, refs/remotes/origin/main, main); run `git remote set-head origin --auto`")
	}
	on, ok := x.onFirstParents(tip, add.Commit)
	if !ok {
		return GapUnestablished, gapf(GapUnestablished, "the default branch's first-parent history could not be read")
	}
	if !on {
		return GapCRUnverified, gapf(GapCRUnverified, "the landing commit %s is not on the default branch's first-parent history", add.Commit)
	}
	return "", ""
}

// ---------------------------------------------------------------------------
// The pins (step 10)
// ---------------------------------------------------------------------------

// parsePins reads the tool pins of paired-versions.yaml line-wise:
// `statusgen.tag`, `desk-tools.tag` and `harness.digest`.
func parsePins(raw []byte) map[string]string {
	pins := map[string]string{}
	section := ""
	for _, l := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		if l[0] != ' ' && l[0] != '\t' {
			section = strings.TrimSuffix(strings.TrimSpace(strings.SplitN(l, "#", 2)[0]), ":")
			continue
		}
		if strings.HasPrefix(l, "    ") || strings.HasPrefix(l, "\t\t") {
			continue // nested deeper than a section's own keys
		}
		k, v, ok := strings.Cut(strings.TrimSpace(l), ":")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(strings.SplitN(v, " #", 2)[0]), `"'`)
		switch {
		case section == "statusgen" && k == "tag":
			pins["statusgen"] = v
		case section == "desk-tools" && k == "tag":
			pins["desk-tools"] = v
		case section == "harness" && k == "digest":
			pins["harness"] = v
		}
	}
	return pins
}

func (x *evalCtx) pinsAt(R string) (map[string]string, bool) {
	if p, ok := x.v2.pins[R]; ok {
		return p, p != nil
	}
	raw, found, ok := x.recordAt(R, WitnessPinFile)
	if !ok || !found {
		x.v2.pins[R] = nil
		return nil, false
	}
	p := parsePins(raw)
	x.v2.pins[R] = p
	return p, true
}

// ---------------------------------------------------------------------------
// judgeWitnessV2
// ---------------------------------------------------------------------------

// v2Target resolves the item's revision to a full commit, or says why it
// cannot be read.
func (x *evalCtx) v2Target(target string) (string, string) {
	if target == "" {
		return "", "no item revision is known"
	}
	if strings.Contains(target, "+") {
		return "", "the item's revision " + target + " carries uncommitted changes"
	}
	prefix, ok := x.rootPrefix()
	if !ok {
		return "", "the repository's toplevel could not be read"
	}
	if prefix != "" {
		return "", "the evaluated tree is below its repository's toplevel (" + prefix + "); v2 records are judged at the toplevel only"
	}
	R, ok := x.repo.ResolveCommit(target)
	if !ok {
		return "", "the item's revision " + target + " does not resolve to a commit"
	}
	return R, ""
}

func hasObligation(r VerifyRow, ob string) bool {
	for _, o := range r.Obligations {
		if o == ob {
			return true
		}
	}
	return false
}

// judgeWitnessV2 judges a v2 cell at the item's revision. It returns
// resolveVerifyClaim's (result, reason, revision) triple; every non-release
// reason starts with its gap name.
func (x *evalCtx) judgeWitnessV2(sc witnessScope, r VerifyRow, w Witness, cell RunnerCell, target, acceptance string) (result, reason, revision string) {
	fail := func(rev, gap, format string, a ...any) (string, string, string) {
		return gapResult(gap), gapf(gap, format, a...), rev
	}
	if _, ok := x.witnessRepo(); !ok {
		return fail("", GapUnestablished, "the repository port cannot list trees, attributes or witness landings")
	}
	R, why := x.v2Target(target)
	if why != "" {
		return fail("", GapUnestablished, "%s", why)
	}

	// Step 1: the record exists, parses, and its id matches the cell.
	if cell.Malformed {
		return fail("", GapRecordMismatch, "the v2 Runner cell carries a token other than one cr: and one rec:")
	}
	if cell.RecordID == "" {
		return fail("", GapRecordMissing, "the v2 cell names no record (rec:)")
	}
	if !hex64Re.MatchString(cell.RecordID) {
		return fail("", GapRecordMismatch, "the cell's rec:%s is not a full 64-hex record id", cell.RecordID)
	}
	recPath := WitnessRecordPath(sc.briefID, cell.RecordID)
	raw, found, ok := x.recordAt(R, recPath)
	if !ok {
		return fail("", GapUnestablished, "the record %s could not be read at %s", recPath, R)
	}
	if !found {
		return fail("", GapRecordMissing, "%s is not in the item's revision", recPath)
	}
	rec, id, err := ParseWitnessRecord(raw)
	if err != nil {
		return fail("", GapRecordMismatch, "%s: %v", recPath, err)
	}
	if id != cell.RecordID {
		return fail("", GapRecordMismatch, "the record's canonical id is %s, the cell names %s", id, cell.RecordID)
	}
	rev := informationalRevision(rec)

	// Step 2: the change request is bound and verified.
	if gap, why := x.checkLanding(sc, cell, rec, R, recPath); gap != "" {
		return gapResult(gap), why, rev
	}

	// Step 3: the record is the citing brief's and row's, and the citing
	// Evidence row's cells match it.
	if rec.Brief != sc.briefID || rec.Row != r.ID {
		return fail(rev, GapRecordReplayed, "the record is for %s row #%s, cited from %s row #%s", rec.Brief, rec.Row, sc.briefID, r.ID)
	}
	if strings.TrimSpace(w.Result) != rec.Result || strings.TrimSpace(w.Output) != rec.OutputSHA256 || strings.TrimSpace(w.Date) != rec.Date {
		return fail(rev, GapRecordReplayed, "the citing row's Result, output digest or Date differs from the record's")
	}
	if WitnessTextDigest(w.Command) != rec.CommandSHA256 {
		return fail(rev, GapCommandCellMismatch, "the citing Evidence row's Command cell is not the command the record ran")
	}

	// Step 4: the executing identity.
	if rec.Runner != cell.Identity {
		return fail(rev, GapRunnerMismatch, "the cell names %q, the record's runner is %q", cell.Identity, rec.Runner)
	}

	// Step 5: the acceptance definition.
	if WitnessTextDigest(r.Command) != rec.CommandSHA256 {
		return fail(rev, GapCommandChanged, "the Verify row's Command changed after the run")
	}
	if WitnessTextDigest(r.Expect) != rec.ExpectSHA256 {
		return fail(rev, GapExpectChanged, "the Verify row's Expect changed after the run")
	}
	if WitnessTextDigest(r.ClassCell) != rec.ClassSHA256 {
		return fail(rev, GapClassChanged, "the Verify row's Class changed after the run")
	}
	if acceptance != "" {
		if !objectIDRe.MatchString(acceptance) {
			return fail(rev, GapUnestablished, "the original acceptance definition is not a full immutable object id")
		}
		histExpect, ok := x.verifyExpectAt(acceptance, sc.briefRel, r.ID)
		if !ok {
			return fail(rev, GapUnestablished, "the original acceptance definition for row #%s could not be read at revision %s", r.ID, acceptance)
		}
		if NormalizeCommandText(histExpect) != NormalizeCommandText(r.Expect) {
			return fail(rev, GapExpectChanged, "the selected original Expect differs from the acceptance definition at revision %s", acceptance)
		}
	}

	// Recorded outcome: a cell that does not record a pass reports it.
	switch w.State {
	case StatePass:
	case StateFail:
		return fail(rev, GapOutcomeFail, "the bound record records a failure")
	case StateCouldNotRun:
		return fail(rev, GapOutcomeCouldNotRun, "the bound record records could-not-run — the row produced no verdict")
	default:
		return fail(rev, GapOutcomeUnparseable, "the witness row's Result cell could not be parsed to a known state")
	}
	if r.ProseLed != "" {
		return fail(rev, GapProseLed, "%s", r.ProseLedNote)
	}

	// Step 6: the environment rule.
	if hasObligation(r, ObligationEnv) {
		return fail(rev, GapRerunOwed, "the row is marked +env; a digest cannot capture the network, a forge or a cluster")
	}
	container := rec.Origin == "run" && rec.RunnerClass == WitnessRunnerContainer && rec.Network == "none"
	switch x.policy.effective() {
	case WitnessPolicyReadableHost:
		if !container {
			network, readable := commandNamesNetworkTool(r.Command, r.Shell)
			if !readable {
				return fail(rev, GapRerunOwed, "a host or back-filled run whose command the library cannot read")
			}
			if network {
				return fail(rev, GapRerunOwed, "a host or back-filled run whose command names a network tool")
			}
		}
	case WitnessPolicyContainerOnly, WitnessPolicyWholeTreeOnly:
		if !container {
			return fail(rev, GapRerunOwed, "only the pinned network-less container runner releases (runner_class %s, network %s, origin %s)", rec.RunnerClass, rec.Network, rec.Origin)
		}
		if x.policy.effective() == WitnessPolicyWholeTreeOnly && rec.Scope == WitnessScopeDeclared {
			return fail(rev, GapRerunOwed, "a declared-scope record does not release until the observed-read check exists")
		}
	case WitnessPolicyUnmarked:
	}
	var extra []string
	for _, d := range sc.deps {
		switch d.Kind {
		case DepEnvironment:
			if d.Name == "" || d.Recorded == "" || d.Current == "" {
				return fail(rev, GapUnestablished, "the %s has no recorded or current value", d.label())
			}
			if d.Recorded != d.Current {
				return fail(rev, GapEnvironmentChanged, "%s", d.Name)
			}
		case DepPolicy, DepBuild:
			p := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(d.Path), "./"), "/")
			if witnessPathOK(p) != nil || strings.ContainsAny(p, "*?[{") {
				return fail(rev, GapUnestablished, "the %s is not a plain repo-relative path", d.label())
			}
			extra = append(extra, p)
		}
	}

	// Step 7: the recorded key sets equal the required set at R.
	ix, listing, ok := x.listing(R)
	if !ok {
		return fail(rev, GapUnestablished, "the item's tree could not be listed")
	}
	spec := WitnessSpec{Declared: sc.v2inputs, WholeTree: !sc.v2inputsFound, Extra: extra}
	wantScope := WitnessScopeDeclared
	if spec.WholeTree {
		wantScope = WitnessScopeWholeTree
	}
	if rec.Scope != wantScope {
		return fail(rev, GapInputSetChanged, "scope — the record is %s, the brief's declaration now gives %s", rec.Scope, wantScope)
	}
	rs := requiredWitnessSet(spec, ix)
	if gap, p := compareKeySets(rs.inputs, rec.Inputs); gap != "" {
		return fail(rev, gap, "%s", p)
	}
	if gap, p := compareKeySets(rs.implicit, rec.Implicit); gap != "" {
		return fail(rev, gap, "%s", p)
	}
	if gap, p := compareKeySets(sortedKeysOf(rec.Inputs), rec.Attributes); gap != "" {
		return fail(rev, gap, "%s (attributes)", p)
	}
	if !spec.WholeTree {
		viol := InputsBoundViolations(spec.Declared, r, func(p string) (bool, bool) {
			if _, ok := ix.byPath[p]; ok {
				return true, false
			}
			return ix.isDir(p), true
		})
		if len(viol) > 0 {
			return fail(rev, GapInputUndeclared, "%s — the Command names it, but `inputs:` does not declare it", viol[0])
		}
	}

	// Step 8: every dirty path lies inside the required set.
	if !spec.WholeTree {
		for _, p := range rec.DirtyPaths {
			if !rs.contains(p) {
				return fail(rev, GapDirtyOutsideInputs, "%s", p)
			}
		}
	}

	// Step 9: nothing absent, nothing a symlink, then every digest matches.
	all := append(append([]string(nil), rs.inputs...), rs.implicit...)
	sort.Strings(all)
	recorded := func(p string) string {
		if v, ok := rec.Inputs[p]; ok {
			return v
		}
		return rec.Implicit[p]
	}
	for _, p := range all {
		if recorded(p) == WitnessAbsent {
			return fail(rev, GapInputAbsent, "%s (recorded absent)", p)
		}
		if _, ok := ix.byPath[p]; !ok {
			return fail(rev, GapInputAbsent, "%s (missing at the item's revision)", p)
		}
	}
	for _, p := range all {
		if ix.byPath[p].Mode == "120000" {
			return fail(rev, GapInputSymlink, "%s — declare the link's target instead", p)
		}
		if ix.byPath[p].Mode == "160000" {
			return fail(rev, GapUnestablished, "%s is a submodule; its content is not in this repository's tree", p)
		}
	}
	for _, p := range all {
		if ix.byPath[p].OID != recorded(p) {
			return fail(rev, GapInputChanged, "%s", p)
		}
	}
	if len(rs.inputs) > 0 {
		attrs, ok := x.attrsAt(R)
		if !ok {
			return fail(rev, GapUnestablished, "the item's attributes could not be resolved")
		}
		for _, p := range rs.inputs {
			if AttributeDigest(attrs[p]) != rec.Attributes[p] {
				return fail(rev, GapInputChanged, "%s (attributes)", p)
			}
		}
	}
	if spec.WholeTree && TreeDigest(listing) != rec.TreeDigest {
		return fail(rev, GapInputChanged, "whole tree — the tree digest differs")
	}

	// Step 10: the recorded tools agree with the pins at R.
	if rec.ToolsSource == "none" || rec.ToolsSource == "" {
		return fail(rev, GapRerunOwed, "the record's tools are unknown (tools_source %q)", rec.ToolsSource)
	}
	pins, ok := x.pinsAt(R)
	if !ok {
		return fail(rev, GapUnestablished, "the pin file could not be read at the item's revision")
	}
	// The compared tools: the statusgen and desk-tools releases for any run,
	// and the harness image digest for a containerised one. Other recorded
	// tools (the Go version) are audit fields with no pin to compare.
	required := []string{"statusgen", "desk-tools"}
	if rec.RunnerClass != "host" {
		required = append(required, "harness")
	}
	for _, k := range required {
		got, ok := rec.Tools[k]
		if !ok || got == "" {
			return fail(rev, GapRerunOwed, "the record does not name its %s", k)
		}
		pin, ok := pins[k]
		if !ok || pin == "" {
			return fail(rev, GapRerunOwed, "the recorded tool %s has no pin at the item's revision", k)
		}
		if pin != got {
			return fail(rev, GapToolChanged, "%s %s, pinned %s at the item's revision", k, got, pin)
		}
	}
	return ResultPass, fmt.Sprintf("v2 witness record %s releases: change request %s verified, and every recorded digest equals the item's revision %s", cell.RecordID[:TreeSHALen], rec.CR, R[:TreeSHALen]), rev
}

func sortedKeysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// compareKeySets compares a required set with a record's keys as SETS (a
// subset test would never compare a key that left the required set): the
// first required path with no recorded key is undeclared-at-record; the first
// recorded key no longer required is set-changed.
func compareKeySets[V any](required []string, recorded map[string]V) (gap, path string) {
	for _, p := range required {
		if _, ok := recorded[p]; !ok {
			return GapInputUndeclaredAtRecord, p
		}
	}
	for _, p := range sortedKeysOf(recorded) {
		if !containsSorted(required, p) {
			return GapInputSetChanged, p
		}
	}
	return "", ""
}

// WitnessClaimSpec returns the spec the required set of one row is derived
// from — for `statusgen witnessdigest`, which shares the derivation with the
// reader.
func WitnessClaimSpec(declared []string, declaredFound bool, man *DependencyManifest, rowID string) WitnessSpec {
	spec := WitnessSpec{Declared: declared, WholeTree: !declaredFound}
	if man != nil {
		for _, d := range man.Deps {
			if (d.Kind == DepPolicy || d.Kind == DepBuild) && d.appliesTo(rowID) {
				spec.Extra = append(spec.Extra, strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(d.Path), "./"), "/"))
			}
		}
	}
	return spec
}

// landingSpecs names every object the landing check of one record reads:
// the landing commit's parents (present in the object store), the record at
// a merge commit's two parents, and the citing brief before and at the
// landing commit (its diff changes the brief).
func landingSpecs(add WitnessAddition, recPath, briefRel string) []string {
	specs := append([]string(nil), add.Parents...)
	if len(add.Parents) == 2 {
		specs = append(specs, revPath(add.Parents[0], recPath), revPath(add.Parents[1], recPath))
	}
	if len(add.Parents) > 0 && briefRel != "" {
		specs = append(specs, revPath(add.Parents[0], briefRel), revPath(add.Commit, briefRel))
	}
	return specs
}

// prefetchV2Records reads, in one object batch, every record a latest v2
// cell of the input cites, the pin file, and every object those records'
// landing checks read, after one first-parent log of witness additions at the
// item's revision. A corpus with no v2 cell makes no read at all; with v2
// cells, the per-revision reads are one tree listing, one attribute batch and
// one log, and none is per row (TestCoverageGitCallsBounded).
func (x *evalCtx) prefetchV2Records(in CoverageInput) {
	if _, ok := x.witnessRepo(); !ok {
		return
	}
	type cited struct{ path, briefRel string }
	var recs []cited
	for _, b := range in.Briefs {
		for _, r := range b.VerifyRows {
			ws := b.Witnesses[r.ID]
			if len(ws) == 0 {
				continue
			}
			cell := witnessSchemaOf(ws[len(ws)-1])
			if cell.Schema == WitnessSchemaV2 && hex64Re.MatchString(cell.RecordID) {
				recs = append(recs, cited{WitnessRecordPath(b.ID, cell.RecordID), b.BriefRel})
			}
		}
	}
	if len(recs) == 0 {
		return
	}
	R, why := x.v2Target(in.Revision)
	if why != "" {
		return
	}
	specs := []string{revPath(R, WitnessPinFile)}
	for _, c := range recs {
		specs = append(specs, revPath(R, c.path))
		if add, found, ok := x.additionOf(R, c.path); ok && found {
			specs = append(specs, landingSpecs(add, c.path, c.briefRel)...)
		}
	}
	x.fetchObjects(specs)
}

// landingFormsText renders the landing-form table for the policy digest.
func landingFormsText() string {
	var parts []string
	for _, f := range landingForms {
		parts = append(parts, fmt.Sprintf("%s:%s:%d:%t", f.name, f.re.String(), f.parents, f.recordAtSecondParent))
	}
	return strings.Join(parts, ";")
}
