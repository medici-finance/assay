package main

// requirements.go — the REQUIREMENTS register (spec/registers-v1.md §6) and the
// reserved `satisfies:` brief citation (spec/brief-v1.md §3.2–3.3).
//
// A REQUIREMENT records what someone asked the product to do, with the criteria
// that would settle whether the ask was met. It is the first link of the chain
// ask → work → evidence → release; before it, acceptance criteria existed only
// inside one brief's Verify table, where nothing could rank or roll them up.
//
// What this file does, and deliberately does NOT do:
//
//   - It PARSES the entry directory (docs/streams/requirements/) the way
//     registerentries.go parses findings, with the same three-state read: an
//     absent register is a legitimate empty, an UNREADABLE one is an error, and
//     the two are never collapsed.
//   - It VALIDATES each entry's shape: slug-form id, the required fields, the
//     ordered `impact` axis (registers-v1 §3.5 — a missing or out-of-set value is
//     a hard PROBLEM naming the offending value), the ordered `status` lifecycle,
//     at least one acceptance criterion, typed `satisfied-by` brief ids, and the
//     one internal-consistency rule (a `satisfied` entry may not claim
//     satisfaction anonymously).
//   - It validates the GRAMMAR of a brief's `satisfies:` refs and emits the
//     reserved-not-gating NOTICE, in the same posture briefv2.go takes for the
//     dependency-graph keys: parsed, type-checked, shape-validated, inert.
//   - It does NOT implement traceability. A requirement no brief cites, a brief
//     that cites nothing, and a citation naming a requirement that does not exist
//     are all LEGAL here and change no exit code. Those checks cost a linter
//     release and a re-pin in every consumer, and they are a separate change
//     (registers-v1 §6.5). Nothing in this file may be read as enforcing them.
//   - It does NOT generate docs/streams/REQUIREMENTS.md. The spec specifies that
//     view; the reference implementation does not write it yet, and that gap is
//     recorded as a divergence in spec/README.md rather than left to be
//     discovered.
//
// Nothing here reads the network or the git index: pure over the tree, the same
// offline envelope every other --lint check keeps.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// requirementsDirName is the entry directory under docs/streams/ (registers-v1
// §2.1). It is also a reserved register name in load.go's stream discovery — a
// register directory is not a stream and must never be walked as one.
const requirementsDirName = "requirements"

// requirementEntry is one REQUIREMENTS entry file's frontmatter (registers-v1
// §6.2). Acceptance is a list because each criterion is a separate sentence a
// Verify row could be written against; folding them into one paragraph is what
// makes acceptance criteria unrollable in the first place.
type requirementEntry struct {
	ID          string   `yaml:"id"`
	Date        string   `yaml:"date"`
	Title       string   `yaml:"title"`
	Impact      string   `yaml:"impact"`
	AskedBy     string   `yaml:"asked-by"`
	Acceptance  []string `yaml:"acceptance"`
	Status      string   `yaml:"status"`
	SatisfiedBy []string `yaml:"satisfied-by,omitempty"`
	Body        string   `yaml:"-"`
	File        string   `yaml:"-"` // basename, for messages
}

// requirementImpactOrder is the ordered severity axis registers-v1 §3.5
// REQUIRES of any new register schema, lowest first. It is a slice rather than a
// set precisely because the order is the point: a validator can sort on it, and
// the rank function below is what a later ranked rollup reads.
var requirementImpactOrder = []string{"minor", "major", "critical"}

// requirementImpactRank returns the 0-based rank of an impact level and whether
// the value is in the vocabulary at all. Higher rank = higher impact.
func requirementImpactRank(impact string) (int, bool) {
	for i, v := range requirementImpactOrder {
		if v == impact {
			return i, true
		}
	}
	return -1, false
}

// requirementStatusOrder is the requirement's own ordered lifecycle
// (registers-v1 §6.2): proposed → accepted → satisfied, with `withdrawn` as the
// terminal tombstone state reachable from any of them. Withdrawal is a status
// flip, never a file deletion (§3.3).
var requirementStatusOrder = []string{"proposed", "accepted", "satisfied", "withdrawn"}

const requirementStatusSatisfied = "satisfied"

func validRequirementStatus(status string) bool {
	for _, v := range requirementStatusOrder {
		if v == status {
			return true
		}
	}
	return false
}

var (
	// requirementIDRe is the REQ- slug form (registers-v1 §3.4): 10–20 chars of
	// [a-z0-9-] after the prefix, starting and ending alphanumeric. There is no
	// legacy numeric form to grandfather — this register is new, so the counter
	// form it would license never existed here.
	requirementIDRe = regexp.MustCompile(`^REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	// requirementRefInRepoRe is the in-repo citation form: REQ-<slug>.
	requirementRefInRepoRe = regexp.MustCompile(`^REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	// requirementRefCrossRepoRe is the cross-repo form: <alias>:REQ-<slug>, the
	// alias resolved through the EXISTING docs/streams/graph-repos.yaml registry
	// (briefv2.go's loadGraphRepos) — never a second registry.
	requirementRefCrossRepoRe = regexp.MustCompile(`^[a-z0-9]+:REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	// requirementDateRe is the ISO-8601 date shape shared with the other
	// registers' dated fields.
	requirementDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// validRequirementRef reports whether ref is a well-formed requirement citation
// (registers-v1 §6.5) and, for the cross-repo form, whether its alias resolves
// in the graph-repos registry. reason is "" on success.
//
// A shape-valid ref to an UNKNOWN alias is invalid — that closed-set property is
// the reason the grammar exists. A ref to a requirement that does not exist is
// NOT checked here and is not an error: existence is traceability, which is
// reserved (§6.5).
func validRequirementRef(ref string, reg *graphRepos) (ok bool, reason string) {
	switch {
	case requirementRefInRepoRe.MatchString(ref):
		return true, ""
	case requirementRefCrossRepoRe.MatchString(ref):
		alias := ref[:strings.IndexByte(ref, ':')]
		return aliasKnown(alias, reg)
	default:
		return false, fmt.Sprintf("%q is not a valid requirement reference (want REQ-<slug> or <alias>:REQ-<slug>, slug 10-20 chars of [a-z0-9-] starting and ending alphanumeric)", ref)
	}
}

// parseRequirementsDir reads every requirement entry file from
// docs/streams/requirements/, sorted by id.
//
// Three-state read (docs/three-state-instrument-rule.md, sub-rule 1), identical
// in shape to parseFindingsDir and separate for the same reason: an ABSENT
// register (os.IsNotExist) is a legitimate empty and returns (nil, nil), while an
// UNREADABLE one (a permission or I/O error) returns a non-nil error. Collapsing
// the two branches would turn a register nobody could read into a register with
// nothing in it — a could-not-check reported as a clean pass.
func parseRequirementsDir(root string) ([]requirementEntry, error) {
	dir := filepath.Join(root, "docs", "streams", requirementsDirName)
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []requirementEntry
	for _, f := range files {
		// README.md is directory documentation, not an entry (same rule as the
		// findings and intake directories).
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == "README.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s/%s: %w", requirementsDirName, f.Name(), err)
		}
		e, err := parseRequirementFile(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing %s/%s: %w", requirementsDirName, f.Name(), err)
		}
		e.File = f.Name()
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

// parseRequirementFile parses a single requirement entry (YAML frontmatter +
// prose body).
func parseRequirementFile(raw []byte) (*requirementEntry, error) {
	fm, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return nil, err
	}
	var e requirementEntry
	if err := yaml.Unmarshal([]byte(fm), &e); err != nil {
		return nil, err
	}
	e.Body = strings.TrimSpace(body)
	return &e, nil
}

// requirementRegisterProblems validates the REQUIREMENTS register at root and
// returns hard PROBLEM messages (exit 1), each prefixed with the entry file it
// came from.
//
// Every rule below is about ONE entry's own shape or internal consistency.
// Nothing here reaches across entries except the duplicate-id check that
// registers-v1 §3.2 requires of every register, and nothing here consults the
// brief corpus: an entry naming a brief that does not exist is legal at this
// version (§6.5).
func requirementRegisterProblems(root string) []string {
	entries, err := parseRequirementsDir(root)
	if err != nil {
		// An unreadable register is a could-not-check, and a could-not-check on a
		// register the board reads is reported as a PROBLEM rather than skipped —
		// the alternative is a run that silently validated nothing.
		return []string{fmt.Sprintf("requirements register unreadable: %v", err)}
	}
	if len(entries) == 0 {
		return nil
	}
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	label := func(e requirementEntry) string {
		if e.File != "" {
			return "docs/streams/" + requirementsDirName + "/" + e.File
		}
		return "requirements register"
	}

	reg, _, regErr := loadGraphRepos(root)
	if regErr != nil {
		add("%s", regErr.Error()) // already path-prefixed
	}

	idOwner := map[string]string{}
	for _, e := range entries {
		p := label(e)
		if e.ID == "" {
			add("%s: id is required — a requirement with no typed id cannot be cited", p)
		} else if !requirementIDRe.MatchString(e.ID) {
			add("%s: invalid id %q — a requirement id is REQ-<slug> (slug 10-20 chars of [a-z0-9-], starting and ending alphanumeric)", p, e.ID)
		}
		if e.ID != "" {
			if prev, dup := idOwner[e.ID]; dup {
				add("%s: id %q is already used by %s — two register entries MUST NOT share an id", p, e.ID, prev)
			} else {
				idOwner[e.ID] = p
			}
		}
		if !requirementDateRe.MatchString(strings.TrimSpace(e.Date)) {
			add("%s: date %q is not an ISO-8601 YYYY-MM-DD date", p, e.Date)
		}
		if strings.TrimSpace(e.Title) == "" {
			add("%s: title is required", p)
		}
		if strings.TrimSpace(e.AskedBy) == "" {
			add("%s: asked-by is required — a requirement with no asker is an assertion, not an ask", p)
		}
		// The severity axis (registers-v1 §3.5). A MISSING value is flagged with
		// the same force as a wrong one: an axis a register can omit is an axis
		// the register does not have, and the whole point of §3.5 is that it
		// cannot be retrofitted honestly later.
		if _, ok := requirementImpactRank(strings.TrimSpace(e.Impact)); !ok {
			add("%s: impact %q is not one of the ordered levels %s (registers-v1 §3.5: a new register's severity axis is required on every entry, from its first version)",
				p, e.Impact, strings.Join(requirementImpactOrder, " < "))
		}
		if !validRequirementStatus(strings.TrimSpace(e.Status)) {
			add("%s: status %q is not one of the lifecycle values %s", p, e.Status, strings.Join(requirementStatusOrder, " -> "))
		}
		if len(nonEmptyStrings(e.Acceptance)) == 0 {
			add("%s: acceptance must carry at least one criterion — a requirement with no acceptance criteria states an ask nobody can settle", p)
		}
		// satisfied-by entries are typed brief ids (registers-v1 §3.4). The
		// grammar is the graph reference grammar already in the tree; existence of
		// the named brief is NOT checked (that is traceability, and it is
		// reserved).
		for _, ref := range nonEmptyStrings(e.SatisfiedBy) {
			if ok, reason := validGraphRef(ref, reg); !ok {
				add("%s: satisfied-by ref %s", p, reason)
			}
		}
		// Internal consistency, not traceability: an entry may not claim
		// satisfaction without naming what satisfied it. This says nothing about
		// whether the named work actually meets the criteria — see §6.4.
		if strings.TrimSpace(e.Status) == requirementStatusSatisfied && len(nonEmptyStrings(e.SatisfiedBy)) == 0 {
			add("%s: status is %q but satisfied-by names no brief — a satisfaction claim must say what is claimed to satisfy it", p, requirementStatusSatisfied)
		}
	}
	return problems
}

// requirementRegisterNotices returns the advisory lines for the REQUIREMENTS
// register. There is exactly one, and it exists to make the RESERVATION visible:
// the register is parsed and shape-validated, and the traceability it looks like
// it enables is deliberately not wired. A reader of a --lint run must be able to
// tell "parsed and reserved" from "silently ignored", which is the same reason
// briefv2.go emits its reserved-edge notices.
func requirementRegisterNotices(root string) []string {
	entries, err := parseRequirementsDir(root)
	if err != nil || len(entries) == 0 {
		// The unreadable case is already a PROBLEM in requirementRegisterProblems;
		// repeating it as a NOTICE would double-report one fault.
		return nil
	}
	return []string{fmt.Sprintf(
		"docs/streams/%s: %s parsed, %s (reserved, not gating) — requirement traceability (an uncited requirement, a brief citing none, a citation naming no requirement) is not checked at this version",
		requirementsDirName, requirementCountPhrase(len(entries)), "satisfies: citations are shape-validated only")}
}

// requirementCountPhrase renders an entry count with correct grammar, so the
// lint line reads naturally ("1 requirement", "3 requirements").
func requirementCountPhrase(n int) string {
	if n == 1 {
		return "1 requirement"
	}
	return fmt.Sprintf("%d requirements", n)
}

// nonEmptyStrings drops blank and whitespace-only entries from a string list, so
// a list written as `acceptance: [""]` counts as the empty list it is rather than
// satisfying a presence rule with nothing in it.
func nonEmptyStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
