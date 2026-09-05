package main

// designgate.go — the design-approval gate (sdlc/05) and the DECISIONS register
// (design-decision records) it dereferences.
//
// The lifecycle before this file had no design stage: a brief went from authored
// to in-progress with nothing recording what was decided or why, so a wrong
// DESIGN was caught only after it had been built, by the review gate reading the
// finished diff. This file adds one earlier control, on the todo → in-progress
// edge (spec/lifecycle-v1.md §4.4): a RISK-GATED brief may not sit at
// in-progress-or-later without an approved design-decision record.
//
// What this file does, and deliberately does NOT do:
//
//   - It PARSES the DECISIONS entry directory (docs/streams/decisions/) with the
//     same three-state read requirements.go uses: an ABSENT register is a
//     legitimate empty, an UNREADABLE one is a could-not-check, and the two are
//     never collapsed.
//   - It VALIDATES each record's shape: DR-<slug> id, the required fields, the
//     ordered `consequence` severity axis (registers-v1 §3.5), a human `decided-by`
//     stamp, at least one enumerated alternative and one accepted consequence.
//   - It ENFORCES the gate: a risk-gated brief authored strictly after the cutover
//     whose README-row status is in-progress-or-later must carry a `design:`
//     reference that dereferences to a record in the register. Absence, or a
//     dangling reference, is a PROBLEM naming the brief.
//   - It is SCOPED, on purpose: `gate: model` all-risks-`no` briefs are untouched
//     (the gate is not a blanket new obligation on every brief in every corpus),
//     and every brief authored on or before the cutover is grandfathered so a pin
//     bump reds nothing already in flight (sdlc/05 `## Human decision`, item 4).
//   - It does NOT mechanically prove the approver differs from the brief's author.
//     It proves an approved record with a human approver EXISTS and dereferences;
//     author≠approver needs a brief-author→human mapping briefs do not carry, and
//     it is named as a boundary here rather than silently claimed — the same
//     attribution-not-identity limit lifecycle-v1.md §7.1.2 declares.
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

// decisionsDirName is the DECISIONS entry directory under docs/streams/
// (registers-v1 §2.1). Registered in load.go's reservedRegisterNames so stream
// discovery never walks a register as a stream.
const decisionsDirName = "decisions"

// designGateCutover is the grandfather boundary (sdlc/05 `## Human decision`,
// item 4). The gate binds only a brief `authored:` STRICTLY AFTER this date, so
// every brief already in any corpus on the day the rule ships is exempt and a pin
// bump cannot retroactively red in-flight work. The obligation attaches only to
// risk-gated briefs authored after the rule lands — which is the whole feature.
// A named constant, not a magic literal, because it is exactly the lifecycle-cost
// decision the human ratifies at the gate.
const designGateCutover = "2026-09-05"

// decisionEntry is one DECISIONS record's frontmatter (a design-decision record).
// Alternatives is a list because enumerating the paths NOT taken — and why each
// was ruled out — is the record's reason for existing; folding them into one
// paragraph is what makes a "decision record" a rationalisation after the fact.
type decisionEntry struct {
	ID           string   `yaml:"id"`
	Date         string   `yaml:"date"`
	Title        string   `yaml:"title"`
	Consequence  string   `yaml:"consequence"`
	DecidedBy    string   `yaml:"decided-by"`
	Alternatives []string `yaml:"alternatives"`
	Accepted     []string `yaml:"accepted"`
	Body         string   `yaml:"-"`
	File         string   `yaml:"-"` // basename, for messages
}

// decisionConsequenceOrder is the ordered severity axis registers-v1 §3.5
// REQUIRES of any new register schema, lowest first — the consequence-if-this-
// design-is-wrong. A slice because the order is the point: a validator sorts on it.
var decisionConsequenceOrder = []string{"minor", "major", "critical"}

func decisionConsequenceValid(c string) bool {
	for _, v := range decisionConsequenceOrder {
		if v == c {
			return true
		}
	}
	return false
}

var (
	// decisionIDRe is the DR- slug form (registers-v1 §3.4), the same 10–20-char
	// [a-z0-9-] slug shape the REQUIREMENTS register uses, with the DR- prefix.
	decisionIDRe = regexp.MustCompile(`^DR-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	// designRefRe is a brief's `design:` reference — the same DR-<slug> grammar.
	designRefRe = regexp.MustCompile(`^DR-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	// decisionDateRe is the shared ISO-8601 date shape.
	decisionDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// leadingDateRe captures the leading YYYY-MM-DD of an `authored:` value like
	// "2026-09-05 by worker-desk (…)".
	leadingDateRe = regexp.MustCompile(`^\s*(\d{4}-\d{2}-\d{2})`)
)

// parseDecisionsDir reads every decision record from docs/streams/decisions/,
// sorted by id. Three-state read (docs/three-state-instrument-rule.md, sub-rule
// 1), identical in shape to parseRequirementsDir: an ABSENT register
// (os.IsNotExist) is a legitimate empty and returns (nil, nil); an UNREADABLE one
// returns a non-nil error, never collapsed into "no records".
func parseDecisionsDir(root string) ([]decisionEntry, error) {
	dir := filepath.Join(root, "docs", "streams", decisionsDirName)
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []decisionEntry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == "README.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s/%s: %w", decisionsDirName, f.Name(), err)
		}
		fm, body, ferr := splitFrontmatter(string(raw))
		if ferr != nil {
			return nil, fmt.Errorf("parsing %s/%s: %w", decisionsDirName, f.Name(), ferr)
		}
		var e decisionEntry
		if uerr := yaml.Unmarshal([]byte(fm), &e); uerr != nil {
			return nil, fmt.Errorf("parsing %s/%s: %w", decisionsDirName, f.Name(), uerr)
		}
		e.Body = strings.TrimSpace(body)
		e.File = f.Name()
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

// decisionRegisterProblems validates the DECISIONS register at root — every rule
// is about one record's own shape or the register-wide duplicate-id rule
// registers-v1 §3.2 requires. Nothing here consults the brief corpus.
func decisionRegisterProblems(root string) []string {
	entries, err := parseDecisionsDir(root)
	if err != nil {
		return []string{fmt.Sprintf("decisions register unreadable: %v", err)}
	}
	if len(entries) == 0 {
		return nil
	}
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	label := func(e decisionEntry) string {
		if e.File != "" {
			return "docs/streams/" + decisionsDirName + "/" + e.File
		}
		return "decisions register"
	}
	idOwner := map[string]string{}
	for _, e := range entries {
		p := label(e)
		if e.ID == "" {
			add("%s: id is required — a design-decision record with no typed id cannot be cited by a brief's design:", p)
		} else if !decisionIDRe.MatchString(e.ID) {
			add("%s: invalid id %q — a design-decision id is DR-<slug> (slug 10-20 chars of [a-z0-9-], starting and ending alphanumeric)", p, e.ID)
		}
		if e.ID != "" {
			if prev, dup := idOwner[e.ID]; dup {
				add("%s: id %q is already used by %s — two register entries MUST NOT share an id", p, e.ID, prev)
			} else {
				idOwner[e.ID] = p
			}
		}
		if !decisionDateRe.MatchString(strings.TrimSpace(e.Date)) {
			add("%s: date %q is not an ISO-8601 YYYY-MM-DD date", p, e.Date)
		}
		if strings.TrimSpace(e.Title) == "" {
			add("%s: title is required", p)
		}
		// The severity axis (registers-v1 §3.5) — a missing value is flagged with
		// the same force as a wrong one.
		if !decisionConsequenceValid(strings.TrimSpace(e.Consequence)) {
			add("%s: consequence %q is not one of the ordered levels %s (registers-v1 §3.5: a new register's severity axis is required on every entry, from its first version)",
				p, e.Consequence, strings.Join(decisionConsequenceOrder, " < "))
		}
		// decided-by is the human approval stamp — the design-approval authority
		// (sdlc/05 `## Human decision`, item 3). It reuses the human:<name>
		// vocabulary of the Verified/Reviewed cells so a model self-sign-off cannot
		// stand in for it.
		if !hasHumanReviewer(e.DecidedBy) {
			add(`%s: decided-by %q must name a human ("human:<name>") — a design decision on a risk-gated brief is a recorded human act, not a model self-sign-off`, p, e.DecidedBy)
		}
		if len(nonEmptyStrings(e.Alternatives)) == 0 {
			add("%s: alternatives must enumerate at least one path not taken — a decision record with no alternatives records an outcome, not a decision", p)
		}
		if len(nonEmptyStrings(e.Accepted)) == 0 {
			add("%s: accepted must state at least one consequence accepted — a decision that accepts nothing hides its cost", p)
		}
	}
	return problems
}

// riskGated reports whether a brief file is risk-gated — gate: human, or any of
// the four risk answers yes. This is the same derivation lifecycle-v1.md §4.3
// binds gate to; a legacy brief with no gate/risk is not risk-gated.
func riskGated(bf *BriefFile) bool {
	if bf.Gate == "human" {
		return true
	}
	for _, v := range bf.Risk {
		if v == "yes" {
			return true
		}
	}
	return false
}

// authoredAfterCutover reports whether the brief's authored: date is strictly
// after designGateCutover. ok is false when the date cannot be parsed — a
// could-not-check the caller must NOT round up into "subject to the gate": an
// unparseable authored line means the gate does not fire (fail-open on the scope
// question), reported as a NOTICE, never a silent red.
func authoredAfterCutover(authored string) (after, ok bool) {
	m := leadingDateRe.FindStringSubmatch(authored)
	if m == nil {
		return false, false
	}
	// ISO-8601 dates compare correctly as strings (lexicographic == chronological).
	return m[1] > designGateCutover, true
}

// designGateStatuses are the lifecycle positions at or past the todo →
// in-progress transition the gate guards. A todo brief has not yet made the move,
// so it is never gated; every later position means the move already happened and
// (for a post-cutover risk-gated brief) required an approved design record.
var designGateStatuses = map[string]bool{
	"in-progress": true,
	"implemented": true,
	"verified":    true,
	"done":        true,
}

// designGateProblems enforces the design-approval gate over the brief corpus and
// returns hard PROBLEM messages (exit 1). designGateNotices carries the
// three-state could-not-check and the scope-skip NOTICEs. Split the same way
// requirementRegisterProblems/Notices are.
//
// `root` resolves the DECISIONS register; `streams` supplies the brief files and
// their README-row status (the file↔row join attributionProblems uses).
func designGateProblems(root string, streams []*Stream) []string {
	// Register-shape problems are independent of any brief and always apply.
	problems := decisionRegisterProblems(root)

	entries, err := parseDecisionsDir(root)
	if err != nil {
		// Unreadable register: the register-shape call above already surfaced it as
		// a PROBLEM. Do not additionally red every brief that references it — that
		// would double-report one fault and turn a could-not-check into a corpus of
		// reds. The gate degrades to "cannot confirm a record dereferences"
		// (NOTICE, in designGateNotices), which is the honest three-state answer.
		return problems
	}
	known := map[string]bool{}
	for _, e := range entries {
		if e.ID != "" {
			known[e.ID] = true
		}
	}

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, perr := parseBriefFile(path)
			if perr != nil || !ok {
				continue // malformed reported by checkBriefFiles; legacy exempt
			}
			if !riskGated(bf) {
				continue // scoped: gate: model all-risks-no briefs are untouched
			}
			after, dateOK := authoredAfterCutover(bf.Authored)
			if !dateOK || !after {
				continue // grandfathered (authored on/before cutover) or unparseable date
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			var row *Brief
			for i := range s.Briefs {
				if s.Briefs[i].Num == num {
					row = &s.Briefs[i]
					break
				}
			}
			if row == nil || !designGateStatuses[row.Status] {
				continue // no row, or a status before the guarded transition (todo)
			}
			label := fmt.Sprintf("%s/brief-%s", s.Name, num)
			ref := strings.TrimSpace(bf.Design)
			switch {
			case ref == "":
				add := fmt.Sprintf("%s: risk-gated brief at %q has no design: record — a brief authored after %s may not move to in-progress until an approved design-decision record (DR-<slug> under docs/streams/decisions/) is cited (spec/lifecycle-v1.md §4.4)", label, row.Status, designGateCutover)
				problems = append(problems, add)
			case !designRefRe.MatchString(ref):
				problems = append(problems, fmt.Sprintf("%s: design: %q is not a valid design-decision reference (want DR-<slug>, slug 10-20 chars of [a-z0-9-] starting and ending alphanumeric)", label, ref))
			case !known[ref]:
				problems = append(problems, fmt.Sprintf("%s: design: %q dereferences to no record under docs/streams/decisions/ — a dangling design reference gates nothing (spec/lifecycle-v1.md §4.4)", label, ref))
			}
		}
	}
	return problems
}

// designGateNotices carries the advisory half of the gate: the reserved-status
// line whenever records exist, and the three-state could-not-check when the
// register is unreadable (so a run that could not confirm the dereference is
// distinguishable from a clean pass).
func designGateNotices(root string, streams []*Stream) []string {
	var notices []string
	entries, err := parseDecisionsDir(root)
	if err != nil {
		return []string{fmt.Sprintf("design-approval gate COULD-NOT-CHECK: decisions register unreadable (%v) — design: references cannot be dereferenced this run; read as unverified, not clean (spec/lifecycle-v1.md §4.4)", err)}
	}
	if len(entries) > 0 {
		notices = append(notices, fmt.Sprintf("docs/streams/%s: %s parsed — the design-approval gate binds risk-gated briefs authored after %s (spec/lifecycle-v1.md §4.4)", decisionsDirName, decisionCountPhrase(len(entries)), designGateCutover))
	}
	return notices
}

// decisionCountPhrase renders an entry count with correct grammar.
func decisionCountPhrase(n int) string {
	if n == 1 {
		return "1 design-decision record"
	}
	return fmt.Sprintf("%d design-decision records", n)
}
