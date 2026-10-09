package main

// intakeexits.go — `statusgen --intake-exits --json`: the intake register's half of the
// intake-exit-v1 record (desk-supervision/33).
//
// The schema is docs/streams/desk-supervision/intake-exit-v1.md. A second writer of the same record
// lives in a different Go module (tools/desk/cmd/scanloop/exitrecord.go); this module cannot import
// it, so the key set and the closed role set are spelled here as literals and pinned to the schema
// document by TestIntakeExitSchema_MatchesDoc in BOTH modules.
//
// The export is derived on demand from the committed register and writes nothing. It emits one
// record per triaged entry that MAPS to one of the five exits AND carries a complete, valid triage
// stamp (`triaged`, `triaged-by`, `triager-tier`), then one summary object. An entry that does not
// map, or is not stamped, is COUNTED and never guessed: who decided, and when, cannot be recovered
// from an unstamped entry, and inventing it would put a fabricated identity into a public record.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

const intakeExitSchema = "intake-exit-v1"

// intakeExitRecord is one intake-exit-v1 line. The JSON keys and their order are the schema
// document's "Fields" table; every key is always written.
type intakeExitRecord struct {
	Schema      string `json:"schema"`
	Source      string `json:"source"`
	Item        string `json:"item"`
	Repo        string `json:"repo"`
	Exit        string `json:"exit"`
	Detail      string `json:"detail"`
	Artifact    string `json:"artifact"`
	DecidedBy   string `json:"decided_by"`
	TriagerRole string `json:"triager_role"`
	TriagerTier string `json:"triager_tier"`
	Opened      string `json:"opened"`
	Triaged     string `json:"triaged"`
	Kind        string `json:"kind"`
	Trust       string `json:"trust"`
	SessionTag  string `json:"session_tag"`
	DispatchRef string `json:"dispatch_ref"`
}

// intakeExitSummary is the export's last line: every triaged entry lands in exactly one count.
type intakeExitSummary struct {
	Stamped   int `json:"stamped"`
	Unstamped int `json:"unstamped"`
	Unmapped  int `json:"unmapped"`
}

// intakeExitRoles is the CLOSED role set a stamp's `triaged-by` (and a record's `triager_role`) is
// drawn from: the five canonical desk loop names plus `driver`. Membership is checked against this
// list, never a pattern — a pattern that admits a role slug also admits a person's login, and the
// register is committed to git.
var intakeExitRoles = []string{"the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"}

// intakeExitTiers are the tiers a register stamp may carry. A stamp is always a judgment.
var intakeExitTiers = []string{"any", "strong"}

var (
	intakeStreamRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	intakeBriefRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[0-9]{2,4}$`)
	intakeFindRe   = regexp.MustCompile(`^F-[A-Za-z0-9][A-Za-z0-9-]*$`)
	// intakeIssueRe is the `issue #NN` scoped-to form (and a `decision-issue` value, which may
	// also be the bare number).
	intakeIssueRe = regexp.MustCompile(`^(?i:issue\s*)?#?([1-9][0-9]*)$`)
)

func inClosedSet(v string, set []string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// clipStamp bounds an offending value quoted back in a PROBLEM, so a long pasted string is not
// echoed into lint output.
func clipStamp(v string) string {
	const max = len("pr-review-desk") // the closed role set's longest member
	r := []rune(v)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return v
}

// parseStampTime reads a `triaged` value: a date (YYYY-MM-DD) or RFC3339.
func parseStampTime(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

// intakeStampProblems lints an entry's triage stamp. Each key is optional (forward-only, no
// backfill); a key that is present must be valid. The returned strings are complete PROBLEM
// messages naming the entry and the rule.
func intakeStampProblems(e intakeEntry) []string {
	var out []string
	if v := strings.TrimSpace(e.Triaged); v != "" {
		if _, ok := parseStampTime(v); !ok {
			out = append(out, fmt.Sprintf(
				"intake register: %s: triaged %q is neither a date (YYYY-MM-DD) nor RFC3339 (desk-supervision/33)",
				e.ID, clipStamp(v)))
		}
	}
	if v := strings.TrimSpace(e.TriagedBy); v != "" && !inClosedSet(v, intakeExitRoles) {
		out = append(out, fmt.Sprintf(
			"intake register: %s: triaged-by must be one of the closed role set (%s), never a person's login — got %q (desk-supervision/33)",
			e.ID, strings.Join(intakeExitRoles, ", "), clipStamp(v)))
	}
	if v := strings.TrimSpace(e.TriagerTier); v != "" && !inClosedSet(v, intakeExitTiers) {
		out = append(out, fmt.Sprintf(
			"intake register: %s: triager-tier must be any or strong — got %q (desk-supervision/33)",
			e.ID, clipStamp(v)))
	}
	return out
}

// intakeStamped reports whether an entry carries a complete, valid stamp.
func intakeStamped(e intakeEntry) bool {
	return strings.TrimSpace(e.Triaged) != "" && strings.TrimSpace(e.TriagedBy) != "" &&
		strings.TrimSpace(e.TriagerTier) != "" && len(intakeStampProblems(e)) == 0
}

// splitDisposition separates a disposition's leading word from the rest. A per-entry file carries
// the bare word (`scoped`, with the target in `scoped-to`); the monolithic view carries the raw
// line (`scoped → x`, `decision-needed → issue #NN`, `rejected — why`).
func splitDisposition(raw string) (word, rest string) {
	s := strings.TrimSpace(raw)
	i := strings.IndexFunc(s, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') })
	if i < 0 {
		return strings.ToLower(s), ""
	}
	word = strings.ToLower(s[:i])
	rest = strings.TrimSpace(s[i:])
	for _, arrow := range []string{"→", "->"} {
		if strings.HasPrefix(rest, arrow) {
			return word, strings.TrimSpace(strings.TrimPrefix(rest, arrow))
		}
	}
	return word, "" // `— why` and any other trailer is prose, never a target
}

// mapIntakeExit maps a triaged entry's disposition to (exit, detail, artifact). ok is false when
// the disposition has no mapping — it is counted `unmapped`, never guessed. spec/registers-v1.md
// §5.4 is this table.
func mapIntakeExit(e intakeEntry) (exit, detail, artifact string, ok bool) {
	word, inline := splitDisposition(e.Disposition)
	target := strings.TrimSpace(e.ScopedTo)
	if target == "" {
		target = inline
	}
	switch word {
	case "scoped":
		switch {
		case intakeFindRe.MatchString(target):
			return "finding", "", target, true
		case intakeIssueRe.MatchString(target) && strings.Contains(target, "#"):
			return "bug", "", "#" + intakeIssueRe.FindStringSubmatch(target)[1], true
		case intakeBriefRe.MatchString(target), intakeStreamRe.MatchString(target):
			return "placeholder", "", target, true
		}
	case "adopted": // legacy spelling: it became work in a stream
		if intakeBriefRe.MatchString(target) || intakeStreamRe.MatchString(target) {
			return "placeholder", "", target, true
		}
	case "decision-needed":
		ref := strings.TrimSpace(e.DecisionIssue)
		if ref == "" {
			ref = inline
		}
		if m := intakeIssueRe.FindStringSubmatch(ref); m != nil {
			return "needs-decision", "", "#" + m[1], true
		}
	case "watching", "rejected":
		return "rejected-watching", word, "", true
	}
	return "", "", "", false
}

// buildIntakeExits derives the records and the summary from the register's entries.
func buildIntakeExits(entries []intakeEntry, repo string) ([]intakeExitRecord, intakeExitSummary) {
	var recs []intakeExitRecord
	var sum intakeExitSummary
	for _, e := range entries {
		if isUntriagedDisposition(e.Disposition) {
			continue
		}
		exit, detail, artifact, ok := mapIntakeExit(e)
		if !ok {
			sum.Unmapped++
			continue
		}
		if !intakeStamped(e) {
			sum.Unstamped++
			continue
		}
		triaged, _ := parseStampTime(e.Triaged)
		rec := intakeExitRecord{
			Schema:      intakeExitSchema,
			Source:      "intake",
			Item:        strings.TrimSpace(e.ID),
			Repo:        repo,
			Exit:        exit,
			Detail:      detail,
			Artifact:    artifact,
			DecidedBy:   "judgment",
			TriagerRole: strings.TrimSpace(e.TriagedBy),
			TriagerTier: strings.TrimSpace(e.TriagerTier),
			Triaged:     triaged.Format(time.RFC3339),
		}
		// opened is the front-door arrival: the entry's date. Never later than triaged.
		if opened, err := time.Parse("2006-01-02", strings.TrimSpace(e.Date)); err == nil && !opened.After(triaged) {
			rec.Opened = opened.UTC().Format(time.RFC3339)
		}
		recs = append(recs, rec)
		sum.Stamped++
	}
	return recs, sum
}

// runIntakeExits is the --intake-exits sub-command. A register that cannot be read (neither the
// per-entry directory nor the index exists, or either is unreadable) is could-not-check, exit 6 —
// never an empty export.
func runIntakeExits(root string, asJSON bool) int {
	return writeIntakeExits(root, asJSON, os.Stdout, os.Stderr)
}

func writeIntakeExits(root string, asJSON bool, w, errw io.Writer) int {
	entries, err := loadIntake(root)
	if err != nil {
		fmt.Fprintf(errw, "statusgen --intake-exits: could-not-check: %v\n", err)
		return 6
	}
	repo := trackedGitHubRepo(root)
	if strings.Count(repo, "/") != 1 {
		repo = "" // a nested group path is not an owner/repo; the schema says empty, never a guess
	}
	recs, sum := buildIntakeExits(entries, repo)
	if !asJSON {
		fmt.Fprintf(w, "intake exits: %d stamped (exported), %d unstamped, %d unmapped — --json emits the intake-exit-v1 records\n",
			sum.Stamped, sum.Unstamped, sum.Unmapped)
		return 0
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, r := range recs {
		if err := enc.Encode(r); err != nil {
			fmt.Fprintln(errw, "statusgen:", err)
			return 1
		}
	}
	if err := enc.Encode(sum); err != nil {
		fmt.Fprintln(errw, "statusgen:", err)
		return 1
	}
	return 0
}
