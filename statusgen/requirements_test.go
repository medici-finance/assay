package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- fixtures ----------

// requirementFixture renders a REQUIREMENTS entry file body from a field map, so
// each test states only the field it is exercising and every other field stays
// valid. A field whose value is the empty string is OMITTED from the frontmatter
// entirely — that is how the missing-field cases (notably a missing `impact`) are
// built, and a missing severity axis has to be testable separately from a wrong
// one (registers-v1 §3.5).
type requirementFixture struct {
	ID          string
	Date        string
	Title       string
	Impact      string
	AskedBy     string
	Acceptance  []string // nil = omit the key entirely
	Status      string
	SatisfiedBy []string
}

func validRequirementFixture() requirementFixture {
	return requirementFixture{
		ID:         "REQ-evidence-visible",
		Date:       "2026-09-04",
		Title:      "A stranger can check a published claim",
		Impact:     "critical",
		AskedBy:    "project driver",
		Acceptance: []string{"A reader without JavaScript sees the current numbers."},
		Status:     "proposed",
	}
}

func (f requirementFixture) render() string {
	var b strings.Builder
	b.WriteString("---\n")
	line := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%s: %q\n", k, v)
		}
	}
	line("id", f.ID)
	line("date", f.Date)
	line("title", f.Title)
	line("impact", f.Impact)
	line("asked-by", f.AskedBy)
	if f.Acceptance != nil {
		b.WriteString("acceptance:\n")
		for _, a := range f.Acceptance {
			fmt.Fprintf(&b, "  - %q\n", a)
		}
	}
	line("status", f.Status)
	if f.SatisfiedBy != nil {
		b.WriteString("satisfied-by: [")
		for i, s := range f.SatisfiedBy {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", s)
		}
		b.WriteString("]\n")
	}
	b.WriteString("---\n\nThe ask, in the asker's terms.\n")
	return b.String()
}

// requirementRoot writes the given entries into a fresh tracking root and returns
// the root path. The file name is derived from the id so a duplicate-id fixture
// can still be written as two distinct files.
func requirementRoot(t *testing.T, entries ...requirementFixture) string {
	t.Helper()
	root := t.TempDir()
	for i, e := range entries {
		name := fmt.Sprintf("entry-%d.md", i)
		writeFile(t, root, "docs/streams/requirements/"+name, e.render())
	}
	return root
}

func joined(ss []string) string { return strings.Join(ss, "\n") }

// mentioning counts the messages that name a given field. The fixtures below
// drive checkBriefFiles over a stream with no README table, which legitimately
// raises its own unrelated PROBLEM — counting only the messages about this key
// keeps each assertion about the key it is testing.
func mentioning(msgs []string, field string) int {
	n := 0
	for _, m := range msgs {
		if strings.Contains(m, field) {
			n++
		}
	}
	return n
}

func containsAll(hay []string, needles ...string) bool {
	s := joined(hay)
	for _, n := range needles {
		if !strings.Contains(s, n) {
			return false
		}
	}
	return true
}

// ---------- the ordered severity axis (registers-v1 §3.5) ----------

// TestRequirementImpactAxisIsOrdered pins the property §3.5 actually requires:
// not that a vocabulary exists, but that it is an axis a validator can SORT on.
// A set-membership check would pass with the levels in any order; this fails if
// the ranks stop ascending.
func TestRequirementImpactAxisIsOrdered(t *testing.T) {
	minor, ok1 := requirementImpactRank("minor")
	major, ok2 := requirementImpactRank("major")
	critical, ok3 := requirementImpactRank("critical")
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("all three levels must be in the vocabulary: %v %v %v", ok1, ok2, ok3)
	}
	if !(minor < major && major < critical) {
		t.Errorf("impact must be an ORDERED axis minor < major < critical; got ranks %d, %d, %d", minor, major, critical)
	}
	if _, ok := requirementImpactRank("urgent"); ok {
		t.Errorf("an out-of-vocabulary level must not rank")
	}
	if _, ok := requirementImpactRank(""); ok {
		t.Errorf("an empty impact must not rank — absence is not a level")
	}
}

// TestRequirementImpactOutOfSetIsFlagged: a present-but-unrecognized impact is a
// hard PROBLEM, and the message NAMES the offending value — a flag that says only
// "bad impact" sends the author back to the spec to guess which entry it meant.
func TestRequirementImpactOutOfSetIsFlagged(t *testing.T) {
	f := validRequirementFixture()
	f.Impact = "urgent"
	problems := requirementRegisterProblems(requirementRoot(t, f))
	if !containsAll(problems, "impact", `"urgent"`) {
		t.Errorf("an out-of-set impact must be flagged by value; got:\n%s", joined(problems))
	}
}

// TestRequirementImpactMissingIsFlagged: §3.5's rule is that the axis is present
// from the first version — an entry that simply omits it must fail exactly as
// loudly as one that misspells it. Omission is the failure mode that would let a
// default-everything backfill in later.
func TestRequirementImpactMissingIsFlagged(t *testing.T) {
	f := validRequirementFixture()
	f.Impact = "" // omitted from the frontmatter entirely
	problems := requirementRegisterProblems(requirementRoot(t, f))
	if !containsAll(problems, "impact") {
		t.Errorf("a missing impact must be flagged (registers-v1 §3.5); got:\n%s", joined(problems))
	}
}

// ---------- entry shape ----------

func TestRequirementValidEntryIsClean(t *testing.T) {
	root := requirementRoot(t, validRequirementFixture())
	if problems := requirementRegisterProblems(root); len(problems) != 0 {
		t.Errorf("a well-formed entry must raise no PROBLEM; got:\n%s", joined(problems))
	}
}

// TestRequirementReservedNoticeIsEmitted: the reservation must be VISIBLE. A key
// that is parsed but never mentioned cannot be told apart from one that is
// ignored, which is the whole reason briefv2.go emits its reserved-edge notices.
func TestRequirementReservedNoticeIsEmitted(t *testing.T) {
	root := requirementRoot(t, validRequirementFixture())
	notices := requirementRegisterNotices(root)
	if !containsAll(notices, "reserved, not gating", "1 requirement") {
		t.Errorf("the register must announce itself as reserved and not gating; got:\n%s", joined(notices))
	}
}

func TestRequirementIDMustBeSlugForm(t *testing.T) {
	for _, id := range []string{"REQ-01", "REQ-short", "R-evidence-visible", "REQ_evidence_visible", ""} {
		f := validRequirementFixture()
		f.ID = id
		problems := requirementRegisterProblems(requirementRoot(t, f))
		if len(problems) == 0 {
			t.Errorf("id %q must be rejected (REQ-<slug>, 10-20 slug chars)", id)
		}
	}
	f := validRequirementFixture()
	f.ID = "REQ-coverage-boundary"
	if problems := requirementRegisterProblems(requirementRoot(t, f)); len(problems) != 0 {
		t.Errorf("a slug-form id must be accepted; got:\n%s", joined(problems))
	}
}

func TestRequirementDuplicateIDIsFlagged(t *testing.T) {
	a, b := validRequirementFixture(), validRequirementFixture()
	b.Title = "A different ask under the same id"
	problems := requirementRegisterProblems(requirementRoot(t, a, b))
	if !containsAll(problems, "already used by") {
		t.Errorf("two entries sharing an id must be flagged (registers-v1 §3.2); got:\n%s", joined(problems))
	}
}

func TestRequirementStatusMustBeInLifecycle(t *testing.T) {
	f := validRequirementFixture()
	f.Status = "in-progress"
	problems := requirementRegisterProblems(requirementRoot(t, f))
	if !containsAll(problems, "status", `"in-progress"`) {
		t.Errorf("an out-of-lifecycle status must be flagged by value; got:\n%s", joined(problems))
	}
	for _, s := range requirementStatusOrder {
		ok := validRequirementFixture()
		ok.Status = s
		if s == requirementStatusSatisfied {
			ok.SatisfiedBy = []string{"sdlc/02"}
		}
		if problems := requirementRegisterProblems(requirementRoot(t, ok)); len(problems) != 0 {
			t.Errorf("status %q is a lifecycle value and must be accepted; got:\n%s", s, joined(problems))
		}
	}
}

// TestRequirementAcceptanceMustCarryACriterion: an ask nobody can settle is not a
// requirement. A list of blanks counts as empty — otherwise the presence rule is
// satisfiable by typing two quotes.
func TestRequirementAcceptanceMustCarryACriterion(t *testing.T) {
	for _, acc := range [][]string{{}, {""}, {"   "}} {
		f := validRequirementFixture()
		f.Acceptance = acc
		problems := requirementRegisterProblems(requirementRoot(t, f))
		if !containsAll(problems, "acceptance") {
			t.Errorf("acceptance %q must be rejected as empty; got:\n%s", acc, joined(problems))
		}
	}
}

func TestRequirementAskedByAndTitleAndDateRequired(t *testing.T) {
	cases := map[string]func(*requirementFixture){
		"asked-by": func(f *requirementFixture) { f.AskedBy = "" },
		"title":    func(f *requirementFixture) { f.Title = "" },
		"date":     func(f *requirementFixture) { f.Date = "" },
	}
	for field, mutate := range cases {
		f := validRequirementFixture()
		mutate(&f)
		problems := requirementRegisterProblems(requirementRoot(t, f))
		if !containsAll(problems, field) {
			t.Errorf("a missing %s must be flagged; got:\n%s", field, joined(problems))
		}
	}
	f := validRequirementFixture()
	f.Date = "04-09-2026"
	if problems := requirementRegisterProblems(requirementRoot(t, f)); !containsAll(problems, "date") {
		t.Errorf("a non-ISO date must be flagged; got:\n%s", joined(problems))
	}
}

// TestRequirementSatisfiedStatusNeedsSatisfiedBy is the entry's one
// internal-consistency rule: a satisfaction CLAIM must say what is claimed to
// satisfy it. It is not a traceability check — nothing here asks whether the
// named brief exists, let alone whether it met the criteria (registers-v1 §6.4).
func TestRequirementSatisfiedStatusNeedsSatisfiedBy(t *testing.T) {
	f := validRequirementFixture()
	f.Status = requirementStatusSatisfied
	problems := requirementRegisterProblems(requirementRoot(t, f))
	if !containsAll(problems, "satisfied-by") {
		t.Errorf("a satisfied entry naming no brief must be flagged; got:\n%s", joined(problems))
	}

	// A satisfied-by naming a brief that does not exist anywhere is LEGAL here:
	// existence is traceability, and traceability is reserved. If this ever
	// starts failing, the reserved boundary has been crossed.
	ok := validRequirementFixture()
	ok.Status = requirementStatusSatisfied
	ok.SatisfiedBy = []string{"nosuchstream/99"}
	if problems := requirementRegisterProblems(requirementRoot(t, ok)); len(problems) != 0 {
		t.Errorf("a citation of a non-existent brief must NOT be flagged at this version (reserved, not gating); got:\n%s", joined(problems))
	}
}

func TestRequirementSatisfiedByMustBeTypedBriefID(t *testing.T) {
	f := validRequirementFixture()
	f.Status = requirementStatusSatisfied
	f.SatisfiedBy = []string{"the traceability brief"}
	problems := requirementRegisterProblems(requirementRoot(t, f))
	if !containsAll(problems, "satisfied-by") {
		t.Errorf("a prose name must be rejected where a typed brief id is required; got:\n%s", joined(problems))
	}
}

// TestRequirementAbsentRegisterIsClean: a root with no register at all is a
// legitimate empty, not a fault — every adopter starts there.
func TestRequirementAbsentRegisterIsClean(t *testing.T) {
	root := t.TempDir()
	if problems := requirementRegisterProblems(root); len(problems) != 0 {
		t.Errorf("an absent register must raise nothing; got:\n%s", joined(problems))
	}
	if notices := requirementRegisterNotices(root); len(notices) != 0 {
		t.Errorf("an absent register must emit no notice; got:\n%s", joined(notices))
	}
}

// TestRequirementUnreadableRegisterIsNotEmpty is the three-state rule: an
// unreadable register is a could-not-check and must NOT be rounded down to "no
// entries". Skipped when the test process can read anything regardless (root).
func TestRequirementUnreadableRegisterIsNotEmpty(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a 0o000 directory is still readable, so the could-not-check cannot be staged")
	}
	root := requirementRoot(t, validRequirementFixture())
	dir := filepath.Join(root, "docs", "streams", "requirements")
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	problems := requirementRegisterProblems(root)
	if !containsAll(problems, "unreadable") {
		t.Errorf("an unreadable register must surface as could-not-check, never as an empty one; got:\n%s", joined(problems))
	}
}

// ---------- the reserved citation grammar (registers-v1 §6.5) ----------

func TestRequirementRefGrammar(t *testing.T) {
	reg := &graphRepos{Cell: "house", Aliases: map[string]graphRepoEntry{"toolkit": {Cell: "house", Repo: "owner/toolkit"}}}
	cases := []struct {
		ref  string
		want bool
	}{
		{"REQ-evidence-visible", true},
		{"toolkit:REQ-evidence-visible", true},
		{"unknownalias:REQ-evidence-visible", false}, // the registry is a CLOSED set
		{"REQ_bad slug", false},
		{"REQ-short", false},
		{"F-evidence-visible", false},
		{"sdlc/01", false},
		{"", false},
	}
	for _, c := range cases {
		ok, reason := validRequirementRef(c.ref, reg)
		if ok != c.want {
			t.Errorf("validRequirementRef(%q) = %v (%s), want %v", c.ref, ok, reason, c.want)
		}
	}
	// With no registry loaded, the in-repo form still resolves and the
	// cross-repo form cannot — an alias with nothing to resolve against is
	// unresolved, never assumed.
	if ok, _ := validRequirementRef("REQ-evidence-visible", nil); !ok {
		t.Errorf("the in-repo form needs no registry")
	}
	if ok, _ := validRequirementRef("toolkit:REQ-evidence-visible", nil); ok {
		t.Errorf("a cross-repo ref must NOT resolve with no registry present")
	}
}

// requirementBrief renders a minimal valid brief-v1 file, optionally carrying a
// `satisfies:` line spelled exactly as given (so a malformed list can be tested).
func requirementBrief(t *testing.T, dir, num, satisfiesLine string) string {
	t.Helper()
	stream := filepath.Base(dir)
	body := fmt.Sprintf(`---
brief: %s/%s
title: fixture brief %s
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-04 by fixture
sources: ["fixture"]
%s---

# Brief %s

## Context
files:
- none

facts:
- k: v
`, stream, num, num, satisfiesLine, num)
	path := filepath.Join(dir, "brief-"+num+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRequirementSatisfiesParsesUnderBriefV1 pins the schema decision: the key
// rides the EXISTING brief-v1 schema. Brief-schema evolution fails closed, so
// minting a new schema value for one optional key would refuse the whole tree on
// every pinned consumer that had not upgraded yet.
func TestRequirementSatisfiesParsesUnderBriefV1(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs", "streams", "sdlc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := requirementBrief(t, dir, "01", "satisfies: [\"REQ-evidence-visible\", \"REQ-coverage-boundary\"]\n")
	bf, ok, err := parseBriefFile(path)
	if err != nil || !ok {
		t.Fatalf("brief must parse: ok=%v err=%v", ok, err)
	}
	if bf.Schema != "brief-v1" {
		t.Errorf("the key must ride brief-v1, got schema %q", bf.Schema)
	}
	if len(bf.Satisfies) != 2 || bf.Satisfies[0] != "REQ-evidence-visible" {
		t.Errorf("satisfies must parse into the brief, got %v", bf.Satisfies)
	}
}

func TestRequirementSatisfiesWrongTypeIsParseError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs", "streams", "sdlc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := requirementBrief(t, dir, "01", "satisfies: REQ-evidence-visible\n")
	if _, _, err := parseBriefFile(path); err == nil {
		t.Errorf("a scalar satisfies: must be a parse error, not a silently-accepted string")
	}
}

// TestRequirementSatisfiesAbsenceIsNeverFlagged and its malformed-ref twin are
// the two halves of "reserved, not gating": absence costs nothing, a present ref
// is shape-checked, and carrying the key produces a NOTICE rather than silence.
func TestRequirementSatisfiesAbsenceIsNeverFlagged(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "sdlc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	requirementBrief(t, dir, "01", "")
	s := &Stream{Name: "sdlc", Dir: dir, Root: root}
	problems, notices := checkBriefFiles([]*Stream{s}, []*Stream{s})
	if mentioning(problems, "satisfies") != 0 {
		t.Errorf("a brief with no satisfies: must raise no satisfies PROBLEM; got:\n%s", joined(problems))
	}
	if strings.Contains(joined(notices), "satisfies") {
		t.Errorf("absence must not even be mentioned; got:\n%s", joined(notices))
	}
}

func TestRequirementSatisfiesMalformedRefIsFlagged(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "sdlc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	requirementBrief(t, dir, "01", "satisfies: [\"REQ_bad slug\"]\n")
	s := &Stream{Name: "sdlc", Dir: dir, Root: root}
	problems, _ := checkBriefFiles([]*Stream{s}, []*Stream{s})
	if !containsAll(problems, "satisfies", "REQ_bad slug") {
		t.Errorf("a malformed requirement ref must be flagged by value; got:\n%s", joined(problems))
	}
}

func TestRequirementSatisfiesPresentEmitsReservedNotice(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "sdlc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	requirementBrief(t, dir, "01", "satisfies: [\"REQ-evidence-visible\"]\n")
	s := &Stream{Name: "sdlc", Dir: dir, Root: root}
	problems, notices := checkBriefFiles([]*Stream{s}, []*Stream{s})
	if mentioning(problems, "satisfies") != 0 {
		t.Errorf("a well-formed citation must raise no satisfies PROBLEM; got:\n%s", joined(problems))
	}
	if !containsAll(notices, "satisfies", "reserved, not gating") {
		t.Errorf("carrying the key must produce the reserved notice; got:\n%s", joined(notices))
	}
}

// TestRequirementRegisterIsNotAStream: the entry directory must be invisible to
// stream discovery. Without the reserved-name entry, docs/streams/requirements/
// would be walked as a stream and fail for having no README — a check the
// register itself would have caused.
func TestRequirementRegisterIsNotAStream(t *testing.T) {
	if !reservedRegisterNames[requirementsDirName] {
		t.Fatalf("%q must be a reserved register name, not a stream directory", requirementsDirName)
	}
	root := requirementRoot(t, validRequirementFixture())
	writeFile(t, root, "docs/streams/sdlc/README.md", "---\nstream: sdlc\nserves: assay\nstatus: active\n---\n\n# sdlc\n")
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatalf("loadStreams must not fail over the register directory: %v", err)
	}
	for _, s := range streams {
		if s.Name == requirementsDirName {
			t.Errorf("the register directory must not load as a stream")
		}
	}
}
