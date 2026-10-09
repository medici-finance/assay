// Package testledger reports which test functions left a tree between two revisions, and
// whether a `Retires-test:` trailer said why (docs/contracts.md, R-retires-test).
//
// WHY. Fail-first evidence proves a regression test can go red on the day it lands; nothing
// held the test in place afterwards. Test functions were deleted, and renamed under Verify
// rows that then ran zero tests (#1306), with no commit naming the function or the reason.
// The ratchet has three parts: a `// regression: #<N>` doc-comment tag that says what a test
// pins, a `Retires-test: <Name> — <why>` commit trailer that says why a test leaves, and this
// report, which lists every departure for the reviewer to judge: an untrailed one as such, a
// trailed one with each trailer's commit and reason. A trailer never hides a departure, so the
// report is `clean` only when nothing departed.
//
// DEFINITIONS (over every `*_test.go` outside testdata/, vendor/, node_modules/ and dot or
// underscore directories):
//
//	test     — a top-level func with no receiver and one parameter, named Test*, Fuzz* or
//	           Benchmark* under go test's naming rule; TestMain is not a test.
//	package  — the slash path of the file's directory, relative to the tree root.
//	tag      — the refs of every doc-comment line `// regression: <ref>[, <ref>…]`, joined
//	           ", ", where a ref is `#<N>`, `F-<slug>` (slug: lower-case letters and digits
//	           in hyphen-separated runs) or `class #<N>`. A line with anything else after the
//	           prefix is prose that happens to wrap there, and is not a tag.
//	hash     — the func body's token stream (comments and layout dropped), hashed.
//	deleted  — in base, not in head (same package, same name).
//	renamed  — a deletion paired with an addition in the same package whose hash, or else
//	           non-empty tag, matches, and the match is unique on both sides.
//	dropped  — kept (same package, same name) with a tag ref in base missing from head: the
//	           tag deleted or edited away, made unparseable, or parted from the func.
//	covered  — a departure named by a trailer's old name; a rename-form trailer covers only
//	           when its new name is the rename's new name, or for a deletion a test added in
//	           the same package. Trailers carry no package, so a bare name covers a same-named
//	           test in any package: the line shows the trailer's commit and reason to judge.
//
// BLIND SPOTS. Go tests only. A test kept by name, and by tag, with its body gutted, skipped
// or build-constrained away is not a departure. A move to another package reads as a deletion
// plus an addition. A Verify row is tied to a test only when it spells the whole name, so a
// prefix or regex `-run` selector that once matched the test is not listed. The report is
// read by the code that runs it: a reviewer runs it from a current-main checkout, never the
// tree under review, and names each side by full commit id (a name that is also a directory
// where it runs is could-not-check).
//
// The report is a rubric, never a gate: nothing here decides that a departure is wrong.
// Every function is pure; the git reads live in ledger_test.go (TestReportTestLedger).
package testledger

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TagPrefix opens the doc-comment line that tags a regression test.
const TagPrefix = "// regression: "

// TestFunc is one test function in a tree.
type TestFunc struct {
	Pkg  string // slash directory path relative to the tree root ("." at the root)
	Name string
	Tag  string // the tag's refs joined ", "; empty when untagged
	Hash string // normalised body hash
	File string // slash path of the declaring file
}

// Rename pairs a deleted test with the added test that replaced it.
type Rename struct {
	Old, New TestFunc
	By       string // "body identical" or "same tag"
}

// Report is the difference between two trees' test functions.
type Report struct {
	Deleted []TestFunc
	Renamed []Rename
	Added   []TestFunc
	Dropped []Rename // kept tests whose head tag lacks a ref the base tag had; By is empty
}

// Retirement is one parsed `Retires-test:` trailer.
type Retirement struct {
	Commit  string
	Test    string
	NewName string // set by the rename form
	Why     string
}

// Row is one Verify-table row that names a test.
type Row struct {
	File string // slash path of the brief
	Line int    // 1-based line of the row
	ID   string // the row's first cell
}

// Tests returns every test function in fsys, sorted by package then name.
func Tests(fsys fs.FS) ([]TestFunc, error) {
	var out []TestFunc
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		src, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		fns, err := fileTests(p, src)
		if err != nil {
			return err
		}
		out = append(out, fns...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return key(out[i]) < key(out[j]) })
	return out, nil
}

func skipDir(name string) bool {
	switch name {
	case "testdata", "vendor", "node_modules":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func key(f TestFunc) string { return f.Pkg + "\x00" + f.Name }

func fileTests(p string, src []byte) ([]TestFunc, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, p, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	var out []TestFunc
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil || !isTestName(fn.Name.Name) {
			continue
		}
		if ps := fn.Type.Params.List; len(ps) != 1 || len(ps[0].Names) > 1 {
			continue
		}
		body := src[fset.Position(fn.Body.Pos()).Offset:fset.Position(fn.Body.End()).Offset]
		out = append(out, TestFunc{
			Pkg:  path.Dir(p),
			Name: fn.Name.Name,
			Tag:  tag(fn.Doc),
			Hash: bodyHash(body),
			File: p,
		})
	}
	return out, nil
}

func isTestName(name string) bool {
	if name == "TestMain" {
		return false
	}
	for _, prefix := range []string{"Test", "Fuzz", "Benchmark"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		if rest == "" {
			return true
		}
		r, _ := utf8.DecodeRuneInString(rest)
		return !unicode.IsLower(r)
	}
	return false
}

func tag(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	var refs []string
	for _, c := range doc.List {
		v, ok := strings.CutPrefix(c.Text, TagPrefix)
		if !ok {
			continue
		}
		line := strings.Split(v, ",")
		for i := range line {
			line[i] = strings.TrimSpace(line[i])
		}
		if allRefs(line) {
			refs = append(refs, line...)
		}
	}
	return strings.Join(refs, ", ")
}

// allRefs reports whether every element is a tag ref: `#<N>`, `F-<slug>` or `class #<N>`.
func allRefs(refs []string) bool {
	for _, r := range refs {
		if !isRef(r) {
			return false
		}
	}
	return len(refs) > 0
}

func isRef(r string) bool {
	if n, ok := strings.CutPrefix(r, "class #"); ok {
		return isNumber(n)
	}
	if slug, ok := strings.CutPrefix(r, "F-"); ok {
		return isSlug(slug)
	}
	n, ok := strings.CutPrefix(r, "#")
	return ok && isNumber(n)
}

// isNumber: an issue number, digits with no leading zero.
func isNumber(s string) bool {
	if s == "" || s[0] == '0' {
		return false
	}
	return strings.Trim(s, "0123456789") == ""
}

// isSlug: lower-case letters and digits in non-empty runs separated by single hyphens.
func isSlug(s string) bool {
	for _, run := range strings.Split(s, "-") {
		if run == "" || strings.ContainsFunc(run, notSlugRune) {
			return false
		}
	}
	return true
}

func notSlugRune(r rune) bool {
	return !('a' <= r && r <= 'z' || '0' <= r && r <= '9')
}

// bodyHash hashes the body's tokens, so a comment, a blank line or a gofmt re-indent leaves
// the hash unchanged and a changed statement changes it.
func bodyHash(body []byte) string {
	fset := token.NewFileSet()
	var s scanner.Scanner
	s.Init(fset.AddFile("", fset.Base(), len(body)), body, nil, 0)
	h := sha256.New()
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		switch {
		case tok == token.SEMICOLON:
			lit = ";"
		case lit == "":
			lit = tok.String()
		}
		fmt.Fprintf(h, "%s\x00", lit)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Diff compares two trees' tests. Renames pair by body hash first, then by tag; a pairing
// is made only when exactly one deletion and one addition in the package share the key, so
// two tests that happen to share a tag stay a deletion and an addition.
func Diff(base, head []TestFunc) Report {
	inHead, inHeadFunc := map[string]bool{}, map[string]TestFunc{}
	for _, f := range head {
		inHead[key(f)], inHeadFunc[key(f)] = true, f
	}
	inBase := map[string]bool{}
	var deleted, added []TestFunc
	for _, f := range base {
		inBase[key(f)] = true
		if !inHead[key(f)] {
			deleted = append(deleted, f)
		}
	}
	for _, f := range head {
		if !inBase[key(f)] {
			added = append(added, f)
		}
	}
	var r Report
	for _, f := range base {
		if h, ok := inHeadFunc[key(f)]; ok && lostRef(f.Tag, h.Tag) {
			r.Dropped = append(r.Dropped, Rename{Old: f, New: h})
		}
	}
	deleted, added = pair(deleted, added, func(f TestFunc) string { return f.Hash }, "body identical", &r)
	deleted, added = pair(deleted, added, func(f TestFunc) string { return f.Tag }, "same tag", &r)
	r.Deleted, r.Added = deleted, added
	sort.Slice(r.Renamed, func(i, j int) bool { return key(r.Renamed[i].Old) < key(r.Renamed[j].Old) })
	return r
}

// lostRef reports whether a ref of the base tag is missing from the head tag.
func lostRef(baseTag, headTag string) bool {
	if baseTag == "" {
		return false
	}
	have := map[string]bool{}
	for _, r := range strings.Split(headTag, ", ") {
		have[r] = true
	}
	for _, r := range strings.Split(baseTag, ", ") {
		if !have[r] {
			return true
		}
	}
	return false
}

func pair(deleted, added []TestFunc, k func(TestFunc) string, by string, r *Report) ([]TestFunc, []TestFunc) {
	count := func(fs []TestFunc) map[string][]int {
		m := map[string][]int{}
		for i, f := range fs {
			if v := k(f); v != "" {
				m[f.Pkg+"\x00"+v] = append(m[f.Pkg+"\x00"+v], i)
			}
		}
		return m
	}
	dk, ak := count(deleted), count(added)
	usedD, usedA := map[int]bool{}, map[int]bool{}
	for kv, ds := range dk {
		if as := ak[kv]; len(ds) == 1 && len(as) == 1 {
			r.Renamed = append(r.Renamed, Rename{Old: deleted[ds[0]], New: added[as[0]], By: by})
			usedD[ds[0]], usedA[as[0]] = true, true
		}
	}
	return without(deleted, usedD), without(added, usedA)
}

func without(fs []TestFunc, used map[int]bool) []TestFunc {
	var out []TestFunc
	for i, f := range fs {
		if !used[i] {
			out = append(out, f)
		}
	}
	return out
}

// ParseTrailers reads the output of
// `git log --format='%H%n%(trailers:key=Retires-test,valueonly,unfold)%n--'`: blocks ended
// by a `--` line, each a commit hash followed by one trailer value per line. A value that
// does not parse is dropped, so a malformed trailer never excuses a departure.
func ParseTrailers(log string) []Retirement {
	var out []Retirement
	commit := ""
	sc := bufio.NewScanner(strings.NewReader(log))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "--":
			commit = ""
		case line == "":
		case commit == "":
			commit = line
		default:
			if r, ok := ParseRetirement(line); ok {
				r.Commit = commit
				out = append(out, r)
			}
		}
	}
	return out
}

// ParseRetirement parses one trailer value: `<TestName> — <why>`, or for a rename
// `<Old> — renamed <New>; <why>`. ` -- ` is accepted for the em dash. The name must be an
// identifier and the reason non-empty.
func ParseRetirement(v string) (Retirement, bool) {
	name, why, ok := strings.Cut(v, " — ")
	if !ok {
		name, why, ok = strings.Cut(v, " -- ")
	}
	name, why = strings.TrimSpace(name), strings.TrimSpace(why)
	if !ok || !isIdent(name) || why == "" {
		return Retirement{}, false
	}
	r := Retirement{Test: name, Why: why}
	if rest, ok := strings.CutPrefix(why, "renamed "); ok {
		nn, reason, ok := strings.Cut(rest, ";")
		nn, reason = strings.TrimSpace(nn), strings.TrimSpace(reason)
		if !ok || !isIdent(nn) || reason == "" {
			return Retirement{}, false
		}
		r.NewName, r.Why = nn, reason
	}
	return r, true
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r))) {
			return false
		}
	}
	return true
}

// RowsNaming returns the Verify-table rows in docs/streams/**/brief-*.md that name the
// test: the name appears as a whole identifier in the row (a `-run` selector or a bare name).
func RowsNaming(fsys fs.FS, name string) ([]Row, error) {
	var out []Row
	err := fs.WalkDir(fsys, "docs/streams", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasPrefix(d.Name(), "brief-") || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		inVerify := false
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "## ") {
				inVerify = strings.HasPrefix(line, "## Verify")
				continue
			}
			if !inVerify || !strings.HasPrefix(line, "|") || !namesIdent(line, name) {
				continue
			}
			id := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(line, "|"), "|", 2)[0])
			out = append(out, Row{File: p, Line: i + 1, ID: id})
		}
		return nil
	})
	if err != nil {
		if _, statErr := fs.Stat(fsys, "docs/streams"); statErr != nil {
			return nil, nil // a tree without docs/streams has no rows to name
		}
		return nil, err
	}
	return out, nil
}

func namesIdent(s, name string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], name)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(name)
		before, _ := utf8.DecodeLastRuneInString(s[:start])
		after, _ := utf8.DecodeRuneInString(s[end:])
		if !identRune(before) && !identRune(after) {
			return true
		}
		i = start + 1
	}
}

func identRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// Lines renders the report: one line per departure, then one line per Verify row naming any
// departed test. A departure no trailer covers is `retired-untrailed` or `renamed-untrailed`;
// a covered one is `retired-trailed` or `renamed-trailed` and carries each covering trailer's
// commit and reason, so a trailer shows a departure to the reviewer and never hides it. A kept
// test that lost a tag ref is `tag-dropped`. at names the commit a departure landed in; rows
// returns the rows naming a test. Every departure yields exactly one line, so an empty report
// (`clean`) means nothing departed.
func Lines(r Report, retired []Retirement, at func(TestFunc) string, rows func(string) []Row) []string {
	var out, rowLines []string
	addRows := func(name string) {
		for _, row := range rows(name) {
			rowLines = append(rowLines, fmt.Sprintf("verify-rows-naming: %s → %s:%s", name, row.File, row.ID))
		}
	}
	addedIn := map[string]bool{}
	for _, f := range r.Added {
		addedIn[f.Pkg+"\x00"+f.Name] = true
	}
	for _, f := range r.Deleted {
		bracket := ""
		if f.Tag != "" {
			bracket = " [regression " + f.Tag + "]"
		}
		by := covering(retired, f.Name, func(nn string) bool { return addedIn[f.Pkg+"\x00"+nn] })
		if by == "" {
			out = append(out, fmt.Sprintf("retired-untrailed: %s.%s%s deleted in %s", f.Pkg, f.Name, bracket, at(f)))
		} else {
			out = append(out, fmt.Sprintf("retired-trailed: %s.%s%s deleted in %s%s", f.Pkg, f.Name, bracket, at(f), by))
		}
		addRows(f.Name)
	}
	for _, rn := range r.Renamed {
		by := covering(retired, rn.Old.Name, func(nn string) bool { return nn == rn.New.Name })
		if by == "" {
			out = append(out, fmt.Sprintf("renamed-untrailed: %s.%s → %s (%s) in %s", rn.Old.Pkg, rn.Old.Name, rn.New.Name, rn.By, at(rn.Old)))
		} else {
			out = append(out, fmt.Sprintf("renamed-trailed: %s.%s → %s (%s) in %s%s", rn.Old.Pkg, rn.Old.Name, rn.New.Name, rn.By, at(rn.Old), by))
		}
		addRows(rn.Old.Name)
	}
	for _, d := range r.Dropped {
		now := "untagged"
		if d.New.Tag != "" {
			now = "[regression " + d.New.Tag + "]"
		}
		out = append(out, fmt.Sprintf("tag-dropped: %s.%s [regression %s] → %s", d.Old.Pkg, d.Old.Name, d.Old.Tag, now))
	}
	return append(out, rowLines...)
}

// covering renders every trailer that covers a departure of the test named name, as
// `; Retires-test in <commit>: [renamed <New>, ]"<why>"`, or "" when none does. A delete-form
// trailer covers by name; a rename-form one only when newName accepts its new name. The
// reason is quoted: it is the committer's text, shown to the reviewer as data.
func covering(retired []Retirement, name string, newName func(string) bool) string {
	var b strings.Builder
	for _, x := range retired {
		if x.Test != name || (x.NewName != "" && !newName(x.NewName)) {
			continue
		}
		c := x.Commit
		if len(c) > 12 {
			c = c[:12]
		}
		renamed := ""
		if x.NewName != "" {
			renamed = "renamed " + x.NewName + ", "
		}
		fmt.Fprintf(&b, "; Retires-test in %s: %s%q", c, renamed, x.Why)
	}
	return b.String()
}
