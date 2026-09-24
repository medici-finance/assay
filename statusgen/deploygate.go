package main

// deploygate.go — the deploy transition and the DEPLOYS register (sdlc/06,
// docs/deploy-model.md).
//
// Assay's lifecycle (spec/lifecycle-v1.md) stops at `done`: a brief reaching
// `done` says a change was reviewed and merged, and says nothing about whether
// it reached a place where users are. This file adds the machine-checkable half
// of the deploy model docs/deploy-model.md specifies: a DEPLOYS register holding
// two entry kinds —
//
//   - a DEPLOY record: a gated transition that MUST NOT fire until the brief it
//     carries is `verified` (or later), naming the per-environment authority and
//     the rollback obligation;
//   - a RUNBOOK record: a typed recovery artefact with drill rows, whose
//     Evidence is filled by whoever ran the drill. An undrilled runbook is
//     reported could-not-check, never silently "ready" and never a hard fail —
//     a runbook nobody has rehearsed is unproven, not broken.
//
// Both kinds share one register directory (docs/streams/deploys/) and one
// three-state read, mirroring designgate.go's DECISIONS register exactly:
// an ABSENT directory is a legitimate empty, an UNREADABLE one is a
// could-not-check, and the two are never collapsed into each other.
//
// What this file deliberately does NOT do: it does not talk to any cluster or
// deploy target (offline envelope, docs/deploy-model.md "Ground rules"), it does
// not invent a sixth brief-lifecycle state (deploy is a transition on its OWN
// typed record, spec/lifecycle-v1.md §9), and it does not model environments
// themselves as a register — docs/deploy-model.md supplies the SHAPE and a
// worked example; an adopter's own environment names are not Assay's to
// enumerate.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// deploysDirName is the DEPLOYS entry directory under docs/streams/. Registered
// in load.go's reservedRegisterNames so stream discovery never walks it as a
// stream.
const deploysDirName = "deploys"

const (
	deployKindDeploy  = "deploy"
	deployKindRunbook = "runbook"
	// deployRollbackNoneAccepted is the explicit "no reverse path exists"
	// token (docs/deploy-model.md "Rollback"): a deploy that genuinely cannot be
	// reversed states that as an accepted consequence with a named approver,
	// rather than omitting the field.
	deployRollbackNoneAccepted = "none-accepted"
)

// deployEntry is one DEPLOYS record — either kind. Fields not used by a given
// kind are simply empty; the two kinds share a directory and a reader so the
// register has one three-state read instead of two.
type deployEntry struct {
	ID          string `yaml:"id"`
	Kind        string `yaml:"kind"`
	Date        string `yaml:"date"`
	Title       string `yaml:"title"`
	Environment string `yaml:"environment"` // deploy only
	Brief       string `yaml:"brief"`       // deploy only: "<stream>/<NN>"
	Authority   string `yaml:"authority"`   // deploy only: "human:<name>"
	Rollback    string `yaml:"rollback"`    // deploy only

	RollbackApprover string `yaml:"rollback-approver"` // deploy only, when rollback: none-accepted
	// BlockedBy reuses the brief-v1 `blocked-by: env` marker (spec/lifecycle-v1.md
	// §9.2) for a deploy waiting on an environment that does not yet exist,
	// rather than minting a second marker.
	BlockedBy string `yaml:"blocked-by"` // deploy only, optional

	Trigger string `yaml:"trigger"` // runbook only
	Cadence string `yaml:"cadence"` // runbook only

	Body string `yaml:"-"`
	File string `yaml:"-"` // basename, for messages
}

var (
	// deployIDRe / runbookIDRe are the two typed-id shapes, the same slug
	// grammar the DECISIONS register's DR- id uses (registers-v1 §3.4).
	deployIDRe  = regexp.MustCompile(`^DEPLOY-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	runbookIDRe = regexp.MustCompile(`^RUNBOOK-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	// deployDateRe is the shared ISO-8601 date shape.
	deployDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// deployBriefRefRe is a DEPLOY record's `brief:` reference: "<stream>/<NN>",
	// the same id shape expectedBriefID derives from a brief's own filename.
	deployBriefRefRe = regexp.MustCompile(`^[a-z][a-z0-9-]*/[0-9]{2}[a-z]?$`)
)

// parseDeploysDir reads every DEPLOYS record from docs/streams/deploys/, sorted
// by id. Three-state read, identical in shape to parseDecisionsDir: an ABSENT
// register (os.IsNotExist) is a legitimate empty and returns (nil, nil); an
// UNREADABLE one returns a non-nil error, never collapsed into "no records".
func parseDeploysDir(root string) ([]deployEntry, error) {
	dir := filepath.Join(root, "docs", "streams", deploysDirName)
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []deployEntry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == "README.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s/%s: %w", deploysDirName, f.Name(), err)
		}
		fm, body, ferr := splitFrontmatter(string(raw))
		if ferr != nil {
			return nil, fmt.Errorf("parsing %s/%s: %w", deploysDirName, f.Name(), ferr)
		}
		var e deployEntry
		if uerr := yaml.Unmarshal([]byte(fm), &e); uerr != nil {
			return nil, fmt.Errorf("parsing %s/%s: %w", deploysDirName, f.Name(), uerr)
		}
		e.Body = body
		e.File = f.Name()
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

// deployEntryLabel renders a record's path for messages.
func deployEntryLabel(e deployEntry) string {
	if e.File != "" {
		return "docs/streams/" + deploysDirName + "/" + e.File
	}
	return "deploys register"
}

// deployRegisterProblems validates every DEPLOYS record's own shape — id form,
// required fields per kind, the rollback grammar, and the drill-rows presence
// requirement on a runbook. It does NOT consult the brief corpus; the
// brief-status precondition is deployTransitionProblems's job, exactly the way
// designgate.go splits register-shape from gate-enforcement.
func deployRegisterProblems(root string) []string {
	entries, err := parseDeploysDir(root)
	if err != nil {
		return []string{fmt.Sprintf("deploys register unreadable: %v", err)}
	}
	if len(entries) == 0 {
		return nil
	}
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	idOwner := map[string]string{}
	for _, e := range entries {
		p := deployEntryLabel(e)

		switch e.Kind {
		case deployKindDeploy:
			if e.ID == "" {
				add("%s: id is required — a DEPLOY record with no typed id cannot be cited by an audit pack", p)
			} else if !deployIDRe.MatchString(e.ID) {
				add("%s: invalid id %q — a DEPLOY record's id is DEPLOY-<slug> (slug 10-20 chars of [a-z0-9-], starting and ending alphanumeric)", p, e.ID)
			}
		case deployKindRunbook:
			if e.ID == "" {
				add("%s: id is required — a RUNBOOK record with no typed id cannot be dispatched a drill", p)
			} else if !runbookIDRe.MatchString(e.ID) {
				add("%s: invalid id %q — a RUNBOOK record's id is RUNBOOK-<slug> (slug 10-20 chars of [a-z0-9-], starting and ending alphanumeric)", p, e.ID)
			}
		default:
			add(`%s: kind %q is not one of "deploy", "runbook"`, p, e.Kind)
			continue // the rest of the checks are kind-specific and would misfire
		}

		if e.ID != "" {
			if prev, dup := idOwner[e.ID]; dup {
				add("%s: id %q is already used by %s — two register entries MUST NOT share an id", p, e.ID, prev)
			} else {
				idOwner[e.ID] = p
			}
		}
		if !deployDateRe.MatchString(strings.TrimSpace(e.Date)) {
			add("%s: date %q is not an ISO-8601 YYYY-MM-DD date", p, e.Date)
		}
		if strings.TrimSpace(e.Title) == "" {
			add("%s: title is required", p)
		}

		switch e.Kind {
		case deployKindDeploy:
			if strings.TrimSpace(e.Environment) == "" {
				add("%s: environment is required — a deploy record must name the environment it targets (docs/deploy-model.md \"Environments\")", p)
			}
			if strings.TrimSpace(e.Brief) == "" {
				add("%s: brief is required — a deploy record with no brief reference carries no precondition to check", p)
			} else if !deployBriefRefRe.MatchString(strings.TrimSpace(e.Brief)) {
				add("%s: brief %q is not a valid brief reference (want <stream>/<NN>)", p, e.Brief)
			}
			if !hasHumanAuthority(e.Authority) {
				add(`%s: authority %q must name a human ("human:<name>") — the deploy authority per environment is a recorded human act, not a model self-sign-off (docs/deploy-model.md "The deploy transition")`, p, e.Authority)
			}
			rb := strings.TrimSpace(e.Rollback)
			if rb == "" {
				add("%s: rollback is required — a deploy declaration with no reverse path does not pass the gate (docs/deploy-model.md \"Rollback\")", p)
			} else if strings.EqualFold(rb, deployRollbackNoneAccepted) {
				// EqualFold, not ==: the sentinel is case-INSENSITIVE. A byte-sensitive
				// compare fails OPEN — "None-Accepted"/"NONE-ACCEPTED" would fall through
				// as an ordinary stated reverse path and silently skip the
				// rollback-approver-must-name-a-human check, defeating the one gate this
				// record shape exists to enforce.
				if !hasHumanAuthority(e.RollbackApprover) {
					add(`%s: rollback: %s requires rollback-approver to name a human ("human:<name>") — where the reverse path genuinely does not exist, that is an accepted consequence with a NAMED approver, never an omission`, p, deployRollbackNoneAccepted)
				}
			}
			if e.BlockedBy != "" && !validBlockedBy[e.BlockedBy] {
				add("%s: invalid blocked-by %q (want env)", p, e.BlockedBy)
			}
		case deployKindRunbook:
			if strings.TrimSpace(e.Trigger) == "" {
				add("%s: trigger is required — a runbook with no stated trigger cannot be dispatched", p)
			}
			if strings.TrimSpace(e.Cadence) == "" {
				add("%s: cadence is required — an undrilled runbook is invisible without a cadence to miss (docs/deploy-model.md \"Runbooks\")", p)
			}
			total, _ := drillRowCounts(e.Body)
			if total == 0 {
				add("%s: no drill rows found under \"## Drill rows\" — a runbook with no drill rows carries no verification that the recovery worked", p)
			}
		}
	}
	return problems
}

// resolveBriefStatus looks up "<stream>/<NN>" in the loaded stream set and
// returns the referenced brief's board-row Status. found is false when the
// stream or the brief number does not exist — a dangling reference.
func resolveBriefStatus(streams []*Stream, ref string) (status string, found bool) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", false
	}
	streamName, num := parts[0], parts[1]
	for _, s := range streams {
		if s.Name != streamName {
			continue
		}
		for _, b := range s.Briefs {
			if b.Num == num {
				return b.Status, true
			}
		}
	}
	return "", false
}

// deployTransitionStatuses are the brief-lifecycle positions a DEPLOY record's
// precondition is satisfied at (spec/lifecycle-v1.md §9, docs/deploy-model.md
// "The deploy transition"): the brief must be `verified` — a NON-implementer
// has independently re-run its Verify table — or `done`, which subsumes
// `verified` plus the recorded review. `implemented` is deliberately excluded:
// that is the implementer's own self-report, which is exactly the state a
// deploy gate exists to refuse.
var deployTransitionStatuses = map[string]bool{
	"verified": true,
	"done":     true,
}

// deployTransitionProblems enforces the deploy transition's precondition over
// the DEPLOYS register: a DEPLOY record whose `brief:` reference is not at
// least `verified` is a hard PROBLEM naming the precondition (docs/deploy-
// model.md "The deploy transition", item 3). Register-shape problems
// (deployRegisterProblems) are folded in first, mirroring designGateProblems.
func deployTransitionProblems(root string, streams []*Stream) []string {
	problems := deployRegisterProblems(root)

	entries, err := parseDeploysDir(root)
	if err != nil {
		// Unreadable register: already surfaced as a PROBLEM above. Do not
		// additionally red every deploy record that references it — the honest
		// three-state answer is "cannot confirm the precondition", carried as a
		// NOTICE in deployTransitionNotices.
		return problems
	}

	for _, e := range entries {
		if e.Kind != deployKindDeploy {
			continue
		}
		ref := strings.TrimSpace(e.Brief)
		if ref == "" || !deployBriefRefRe.MatchString(ref) {
			continue // already flagged by deployRegisterProblems
		}
		p := deployEntryLabel(e)
		status, found := resolveBriefStatus(streams, ref)
		if !found {
			problems = append(problems, fmt.Sprintf("%s: brief %q dereferences to no brief in the loaded stream set — a dangling brief reference gates nothing (docs/deploy-model.md \"The deploy transition\")", p, ref))
			continue
		}
		if !deployTransitionStatuses[status] {
			problems = append(problems, fmt.Sprintf("%s: brief %q is %q, not verified — a deploy MUST NOT fire until the brief it carries has been independently re-verified (docs/deploy-model.md \"The deploy transition\", precondition 1)", p, ref, status))
		}
	}
	return problems
}

// drillRowCounts locates the "## Drill rows" section of a RUNBOOK body and
// returns the total number of data rows and how many of them have an empty
// Evidence cell (undrilled). It is a presence/structure check in the same
// spirit as verifyTableHasRow: it locates the header row naming "Evidence" by
// NAME, then scans the data rows below it. A section with no such header, or no
// data rows, reports total == 0.
func drillRowCounts(body string) (total, undrilled int) {
	section := extractHeadingSection(body, "Drill rows")
	evIdx := -1
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			evIdx = -1
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := splitRowEscaped(line)
		if evIdx < 0 {
			for j, c := range cells {
				if strings.EqualFold(strings.TrimSpace(c), "evidence") {
					evIdx = j
				}
			}
			continue // header row is not a data row
		}
		total++
		if evIdx < 0 || evIdx >= len(cells) || strings.TrimSpace(cells[evIdx]) == "" {
			undrilled++
		}
	}
	return total, undrilled
}

// extractHeadingSection returns the body text between a `## <name>` heading
// (case-insensitive on name) and the next `## ` heading (or end of body). "" if
// the heading is not present. A small, local scan rather than a Markdown parser
// — the same "presence/structure, not full parse" scope brief-file section
// extraction already keeps.
func extractHeadingSection(body, name string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## ") && strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(t, "## ")), name) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return strings.Join(lines[start:], "\n")
}

// deployCountPhrase renders an entry count with correct grammar.
func deployCountPhrase(n int) string {
	if n == 1 {
		return "1 deploys-register record"
	}
	return fmt.Sprintf("%d deploys-register records", n)
}

// deployTransitionNotices carries the advisory half of the deploy transition:
// the reserved-status line whenever records exist, the three-state
// could-not-check when the register itself is unreadable, and — the negative
// path docs/deploy-model.md's Verify row 5 pins — an explicit NOTICE per
// RUNBOOK record that has at least one undrilled drill row. An undrilled
// runbook is reported could-not-check: NOT a pass (nothing proved the recovery
// works) and NOT a hard failure (an unrehearsed drill is not itself a defect).
func deployTransitionNotices(root string, streams []*Stream) []string {
	var notices []string
	entries, err := parseDeploysDir(root)
	if err != nil {
		return []string{fmt.Sprintf("deploy transition COULD-NOT-CHECK: deploys register unreadable (%v) — brief: references cannot be dereferenced this run; read as unverified, not clean (docs/deploy-model.md \"The deploy transition\")", err)}
	}
	if len(entries) > 0 {
		notices = append(notices, fmt.Sprintf("docs/streams/%s: %s parsed — the deploy transition gates a DEPLOY record on its brief being verified (docs/deploy-model.md \"The deploy transition\")", deploysDirName, deployCountPhrase(len(entries))))
	}
	for _, e := range entries {
		if e.Kind != deployKindRunbook {
			continue
		}
		total, undrilled := drillRowCounts(e.Body)
		if total > 0 && undrilled > 0 {
			notices = append(notices, fmt.Sprintf("%s: could-not-check — %d of %d drill row(s) have no Evidence — an undrilled runbook is unproven, not \"ready\" (docs/deploy-model.md \"Runbooks\")", deployEntryLabel(e), undrilled, total))
		}
	}
	return notices
}

// hasHumanAuthority reports whether a deploy record's authority value (`authority:` or
// `rollback-approver:`) names a human, once every on-behalf-of relay has been removed
// (withoutOnBehalfOfRelays). The online corroboration lane strips the relay marker, so a
// relay-only value would otherwise pass the lint with nothing on the PR to corroborate.
// A relay records which human an App acted for. It is not that human's deploy
// authority. The check stays hasHumanReviewer's syntactic test in every other respect.
// README Reviewed cells and a decision record's decided-by keep hasHumanReviewer
// unchanged, because their online lanes read the raw value and gate it.
func hasHumanAuthority(value string) bool {
	return hasHumanReviewer(withoutOnBehalfOfRelays(value))
}
