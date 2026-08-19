package main

// autonomy.go — the step-3 adoption-ladder gauges (methodology-metrics/41).
//
// The adoption ladder's 3→4 rung is "most agents are kicked off by Claude, not
// the operator". Claiming it is easy; DEMONSTRATING it needs numbers. This
// sub-command (`statusgen --autonomy`) emits four of them, as a SYSTEM, never a
// single blended score:
//
//  1. Autonomy ratio — loop-initiated units / all units, computed TWO ways and
//     emitted SEPARATELY (never blended):
//       (a) CI-computable proxy — merged PRs bucketed by AUTHOR IDENTITY. Assay
//           Apps (assay-*-app[bot]) and workflow/automation bots are
//           loop-initiated; a roster-known individual is human; ANY other User
//           account (the shared org account, an unrecognised login) is
//           AMBIGUOUS and carried as its OWN bucket — the shared account is used
//           by agents AND desks, and guessing which would fabricate the very
//           ratio this axis exists to measure. The ambiguous bucket is NEVER
//           folded into either side of the loop/human split.
//       (b) dispatch-level ratio — from the mm/40 opmetrics day-file's
//           routine-vs-operator dispatch split. Present only on days that
//           file carries the split; otherwise "unmeasured", never zero.
//  2. Token efficiency — subagent tokens per merged PR + no-op dispatch rate
//     (workers exiting "already done"). Read from the mm/40 day-file (dispatch
//     records are machine-local — see mm/40); absent field → "unmeasured today".
//  3. Deterministic-gate share — % of merged PRs whose merge path was guarded by
//     at least one CODE gate that ran at head (statusgen --lint, go-test,
//     upgrade-check, leak-sweep …) vs model-judgment-only (reviewer-App verdicts,
//     which are REVIEWS and so never appear in a status-check rollup). The split
//     is rendered, not collapsed: a deterministic gate is the stronger 3→4 signal.
//
// The fourth axis, REWORK (CHANGES_REQUESTED count + re-review cycles per merged
// PR), is the automatable #766(b) row-5 definition and is wired into the existing
// `--dora` instability family (see dora.go), NOT re-emitted here — it belongs with
// change-failure rate as a DORA instability metric.
//
// THREE STATES, NOT TWO (matching dora.go / opmetrics emit.go). Every axis is
// measured, or it is "unmeasured" with a REASON CODE. It is NEVER zero: a silent
// zero says "fully operator-driven / perfectly efficient / all model-gated" when
// the truth is "the source was not there". Zero and unmeasured are different
// findings and this report distinguishes them.
//
// SELF-CITING (matching daily-harvest/flowmetrics.go). Every axis prints the
// query or artifact it derives from. A gauge whose number cannot be re-derived by
// re-running its own cited source is not evidence.
//
// ANTI-GAMING. These are DIAGNOSTICS, never targets and never per-person
// scorecards; token budgets stay at the WORKFLOW level (#766 addendum point 6).
// The Goodhart clause is carried in the report header, same as --dora / flow.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultAutonomyWindowDays = 28

const autonomyAntiGamingNote = "Autonomy / token / gate-share are DIAGNOSTIC, per-project, for " +
	"continuous improvement — never a target, an individual scorecard, or a cross-team comparison " +
	"(Goodhart's law: a measure that becomes a target ceases to be a good measure). Token budgets " +
	"stay at the WORKFLOW level, never per-person. An axis with no source reads 'unmeasured' — never zero."

// Autonomy authorship bucket names.
const (
	bucketLoop      = "loop"
	bucketAmbiguous = "ambiguous"
	bucketHuman     = "human"
)

// autonomyBuckets is the CI-proxy authorship split. The ambiguous bucket is a
// first-class count, never folded into Loop or Human — see the file header.
type autonomyBuckets struct {
	LoopInitiated int `json:"loop_initiated"`
	Ambiguous     int `json:"ambiguous"`
	Human         int `json:"human"`
	Total         int `json:"total"`
}

// AutonomyAxis is one gauge in the system. Measured=false carries a Reason code
// and a Value of "unmeasured" — never a fabricated zero.
type AutonomyAxis struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Measured bool   `json:"measured"`
	Value    string `json:"value"`
	Reason   string `json:"reason,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Source   string `json:"source"`
}

// AutonomyReport is the whole emitted system. Axes is ordered (ci-proxy autonomy,
// dispatch autonomy, token efficiency, gate share) so the four always present
// together and a consumer can assert "exactly four".
type AutonomyReport struct {
	Since     string          `json:"since"`
	Until     string          `json:"until"`
	Generated string          `json:"generated"`
	Note      string          `json:"note"`
	Buckets   autonomyBuckets `json:"ci_proxy_buckets"`
	Axes      []AutonomyAxis  `json:"axes"`
}

// autonomyAuthor is one merged PR's authorship for the CI-proxy bucket.
type autonomyAuthor struct {
	Login string
	IsBot bool
}

// autonomyGatePR is one merged PR's status-check contexts for the gate-share axis.
type autonomyGatePR struct {
	Number     int
	CheckNames []string
}

// opDayFile is the CONSUMER view of the mm/40 opmetrics day-file. Only the fields
// the CI-side axes read are declared, all as pointers so "absent" is
// distinguishable from "present and zero". The producer's schema (opmetrics/1,
// tools/desk/cmd/opmetrics/emit.go) does not yet carry the dispatch-split or
// token blocks; when a future producer adds them, this reader picks them up with
// no change here — until then both day-file axes honestly report "unmeasured".
type opDayFile struct {
	Schema   string `json:"schema"`
	Dispatch *struct {
		LoopInitiated     *int `json:"loop_initiated"`
		OperatorInitiated *int `json:"operator_initiated"`
	} `json:"dispatch"`
	TokenEfficiency *struct {
		SubagentTokens  *int `json:"subagent_tokens"`
		MergedPRs       *int `json:"merged_prs"`
		NoopDispatches  *int `json:"noop_dispatches"`
		TotalDispatches *int `json:"total_dispatches"`
	} `json:"token_efficiency"`
}

// autonomyInputs is the pure input to computeAutonomy — everything gathered from
// gh + the day-file, kept as data so the ratio math is deterministic and testable
// with no exec/network.
type autonomyInputs struct {
	Since, Until, Now time.Time
	MergedAuthors     []autonomyAuthor
	AuthorsOK         bool
	HumanLogins       map[string]bool
	GateData          []autonomyGatePR
	GateOK            bool
	DayFile           *opDayFile // nil = no day-file found
	DayFileDate       string     // the date of the day-file consumed (for self-citing)
}

// autonomyHumanLogins is the set of roster-known individual GitHub logins that
// classify to the human bucket. Default EMPTY: without a roster of known humans
// we cannot CLAIM any User account is an individual operator, so every non-bot
// non-app author falls to the ambiguous bucket — the never-guess default. A
// caller (or test) that knows the roster populates this.
var autonomyHumanLogins = map[string]bool{}

// deterministicGatePatterns are the lower-cased substrings that mark a status
// check as a CODE gate (ran deterministically at head) rather than a
// model-judgment gate. Reviewer-App verdicts are REVIEWS, not status checks, so
// they never appear in a rollup and need no exclusion here. This is the GENERIC
// built-in default set; a deployment MERGES additional house-specific gate
// names on top of it through EnvDeterministicGatePatterns (see below), so the
// shipped binary carries no house gate name.
var deterministicGatePatterns = []string{
	"lint", "gate", "test", "vet", "build", "upgrade-check",
	"leak-sweep", "leaksweep", "check-", "-check", "consumers", "go-test",
}

// EnvDeterministicGatePatterns names the environment variable carrying
// ADDITIONAL deterministic-gate name substrings, MERGED on top of the built-in
// generic default set (deterministicGatePatterns). Comma- or whitespace-
// separated; each entry is lower-cased and trimmed:
//
//	ASSAY_DETERMINISTIC_GATE_PATTERNS=<your-house-gate>
//
// WHY A CONFIG CALLOUT AND NOT A HARD-CODED PATTERN. A house may run a
// deterministic CI gate whose NAME is house-internal — its own build- or
// contract-specific gate, not part of the generic set. Compiling that name
// into the SHIPPED binary would disclose a house-internal gate name in a public
// copy of statusgen, so the extra pattern(s) are a DEPLOYMENT VALUE the house
// supplies at run time, exactly as the accepted-drift register path works
// (ASSAY_CHANNEL_DRIFT_TARGET) and the other generic knobs
// (ASSAY_SWEEP_WITHHELD_STREAMS, ASSAY_WRITEGUARD_CALLOUT): the shipped binary
// carries the MECHANISM (merge house-supplied gate patterns into the generic
// set) plus the generic default set; the deployment carries the POLICY (which
// extra gate names count as deterministic).
//
// UNSET IS COMPLETE, NOT DEGRADED. Unset means the generic default set ALONE —
// the correct public/adopter posture, carrying no house gate name. A house sets
// ASSAY_DETERMINISTIC_GATE_PATTERNS (roster.env or CI env) to its own gate
// name(s) to preserve their deterministic-gate classification.
//
// FOLLOW-UP: a house VALUE is expected to migrate to a house-owned config
// source (tracked separately); this callout is the interim home for the
// MECHANISM.
//
// NAMESPACE. Generic methodology knob (the MECHANISM of merging house gate
// patterns) so it wears the ASSAY_ prefix; the VALUE (house gate names) is
// deployment-specific and never compiled in.
const EnvDeterministicGatePatterns = "ASSAY_DETERMINISTIC_GATE_PATTERNS"

// additionalDeterministicGatePatterns returns the house-configured extra
// deterministic-gate substrings from EnvDeterministicGatePatterns, each
// lower-cased and trimmed. Empty (the unset case) is the complete
// public/adopter configuration: only the generic built-in set applies and the
// shipped binary carries no house gate name.
func additionalDeterministicGatePatterns() []string {
	raw := os.Getenv(EnvDeterministicGatePatterns)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.ToLower(strings.TrimSpace(f)); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// isAssayApp reports whether a (bot) login is an assay App identity — the
// loop-initiated automation actors (assay-worker-app, assay-reviewer-app, …).
func isAssayApp(login string) bool {
	l := strings.ToLower(strings.TrimSuffix(login, "[bot]"))
	return strings.HasPrefix(l, "assay-") && strings.HasSuffix(l, "-app")
}

// classifyAuthor buckets one merged PR's author. NEVER guesses: an assay App or
// any other bot is loop-initiated; a roster-known individual is human; every
// other User account (the shared org account, an unrecognised login) is
// AMBIGUOUS. The ambiguous bucket is the honest home for "cannot attribute".
func classifyAuthor(login string, isBot bool, humanLogins map[string]bool) string {
	l := strings.ToLower(strings.TrimSuffix(login, "[bot]"))
	switch {
	case isAssayApp(login):
		return bucketLoop
	case isBot:
		return bucketLoop
	case humanLogins[l]:
		return bucketHuman
	default:
		return bucketAmbiguous
	}
}

// bucketAuthors reduces the merged-PR authors to the three-way split.
func bucketAuthors(authors []autonomyAuthor, humanLogins map[string]bool) autonomyBuckets {
	var b autonomyBuckets
	for _, a := range authors {
		switch classifyAuthor(a.Login, a.IsBot, humanLogins) {
		case bucketLoop:
			b.LoopInitiated++
		case bucketHuman:
			b.Human++
		default:
			b.Ambiguous++
		}
		b.Total++
	}
	return b
}

// hasDeterministicGate reports whether any of a PR's status-check names matches a
// deterministic-gate pattern.
func hasDeterministicGate(names []string) bool {
	patterns := deterministicGatePatterns
	if extra := additionalDeterministicGatePatterns(); len(extra) > 0 {
		patterns = append(append([]string(nil), deterministicGatePatterns...), extra...)
	}
	for _, n := range names {
		ln := strings.ToLower(n)
		for _, p := range patterns {
			if strings.Contains(ln, p) {
				return true
			}
		}
	}
	return false
}

// computeAutonomy is the pure computation: autonomyInputs → AutonomyReport. No
// exec, no network, no clock — every value derives from the passed inputs.
func computeAutonomy(in autonomyInputs) AutonomyReport {
	rep := AutonomyReport{
		Since:     in.Since.Format("2006-01-02"),
		Until:     in.Until.Format("2006-01-02"),
		Generated: in.Now.UTC().Format(time.RFC3339),
		Note:      autonomyAntiGamingNote,
	}

	// Axis 1 — autonomy ratio, CI-computable proxy (merged-PR authorship).
	proxy := AutonomyAxis{
		Key: "autonomy_ci_proxy", Name: "Autonomy ratio (CI proxy: merged-PR authorship)",
		Source: "gh pr list --state merged --json number,author,mergedAt (authorship buckets)",
	}
	if in.AuthorsOK {
		b := bucketAuthors(in.MergedAuthors, in.HumanLogins)
		rep.Buckets = b
		if b.Total > 0 {
			proxy.Measured = true
			ratio := float64(b.LoopInitiated) / float64(b.Total)
			proxy.Value = fmt.Sprintf("%.0f%% loop-initiated", ratio*100)
			proxy.Detail = fmt.Sprintf(
				"loop=%d, human=%d, ambiguous=%d (shared/unattributed account — NOT folded into either side), total=%d merged PR(s)",
				b.LoopInitiated, b.Human, b.Ambiguous, b.Total)
		} else {
			proxy.Value = "unmeasured"
			proxy.Reason = "no-merged-prs"
			proxy.Detail = "no merged PRs in the window to bucket by authorship"
		}
	} else {
		proxy.Value = "unmeasured"
		proxy.Reason = "gh-unreadable"
		proxy.Detail = "gh merged-PR authorship could not be read (offline / unauthenticated)"
	}
	rep.Axes = append(rep.Axes, proxy)

	// Axis 2 — autonomy ratio, dispatch-level (mm/40 day-file split).
	disp := AutonomyAxis{
		Key: "autonomy_dispatch", Name: "Autonomy ratio (dispatch-level: routine vs operator)",
		Source: "mm/40 opmetrics day-file — dispatch.loop_initiated / dispatch.operator_initiated",
	}
	if in.DayFile == nil {
		disp.Value = "unmeasured"
		disp.Reason = "no-opmetrics-dayfile"
		disp.Detail = "no opmetrics day-file under docs/reports/daily/ (operator collector did not run — mm/40)"
	} else if in.DayFile.Dispatch == nil || in.DayFile.Dispatch.LoopInitiated == nil || in.DayFile.Dispatch.OperatorInitiated == nil {
		disp.Value = "unmeasured"
		disp.Reason = "dayfile-no-dispatch-split"
		disp.Detail = fmt.Sprintf("opmetrics day-file (%s, schema %s) carries no routine/operator dispatch split", in.DayFileDate, in.DayFile.Schema)
	} else {
		loop := *in.DayFile.Dispatch.LoopInitiated
		op := *in.DayFile.Dispatch.OperatorInitiated
		total := loop + op
		if total == 0 {
			disp.Value = "unmeasured"
			disp.Reason = "dayfile-no-dispatches"
			disp.Detail = fmt.Sprintf("opmetrics day-file (%s) recorded 0 dispatches", in.DayFileDate)
		} else {
			disp.Measured = true
			disp.Value = fmt.Sprintf("%.0f%% loop-initiated", float64(loop)/float64(total)*100)
			disp.Detail = fmt.Sprintf("loop=%d, operator=%d, total=%d dispatch(es) (day-file %s)", loop, op, total, in.DayFileDate)
		}
	}
	rep.Axes = append(rep.Axes, disp)

	// Axis 3 — token efficiency (mm/40 day-file).
	tok := AutonomyAxis{
		Key: "token_efficiency", Name: "Token efficiency (subagent tokens/merged PR + no-op rate)",
		Source: "mm/40 opmetrics day-file — token_efficiency.{subagent_tokens,merged_prs,noop_dispatches,total_dispatches}",
	}
	if in.DayFile == nil {
		tok.Value = "unmeasured"
		tok.Reason = "no-opmetrics-dayfile"
		tok.Detail = "no opmetrics day-file under docs/reports/daily/ — dispatch/token records are machine-local (mm/40)"
	} else if in.DayFile.TokenEfficiency == nil {
		tok.Value = "unmeasured"
		tok.Reason = "dayfile-no-token-block"
		tok.Detail = fmt.Sprintf("opmetrics day-file (%s, schema %s) carries no token block", in.DayFileDate, in.DayFile.Schema)
	} else {
		te := in.DayFile.TokenEfficiency
		var parts []string
		if te.SubagentTokens != nil && te.MergedPRs != nil && *te.MergedPRs > 0 {
			parts = append(parts, fmt.Sprintf("%.0f subagent tokens/merged PR", float64(*te.SubagentTokens)/float64(*te.MergedPRs)))
		}
		if te.NoopDispatches != nil && te.TotalDispatches != nil && *te.TotalDispatches > 0 {
			parts = append(parts, fmt.Sprintf("%.0f%% no-op dispatch rate", float64(*te.NoopDispatches)/float64(*te.TotalDispatches)*100))
		}
		if len(parts) > 0 {
			tok.Measured = true
			tok.Value = strings.Join(parts, "; ")
			tok.Detail = fmt.Sprintf("from opmetrics day-file %s", in.DayFileDate)
		} else {
			tok.Value = "unmeasured"
			tok.Reason = "dayfile-token-block-empty"
			tok.Detail = fmt.Sprintf("opmetrics day-file (%s) token block has neither a tokens/merged-PR nor a no-op-rate numerator+denominator", in.DayFileDate)
		}
	}
	rep.Axes = append(rep.Axes, tok)

	// Axis 4 — deterministic-gate share (merged-PR status-check rollups).
	gate := AutonomyAxis{
		Key: "deterministic_gate_share", Name: "Deterministic-gate share (code gate vs model-judgment-only)",
		Source: "gh pr list --state merged --json number,mergedAt,statusCheckRollup",
	}
	if in.GateOK {
		total := len(in.GateData)
		if total == 0 {
			gate.Value = "unmeasured"
			gate.Reason = "no-merged-prs"
			gate.Detail = "no merged PRs in the window to classify by gate path"
		} else {
			gated := 0
			for _, p := range in.GateData {
				if hasDeterministicGate(p.CheckNames) {
					gated++
				}
			}
			modelOnly := total - gated
			gate.Measured = true
			gate.Value = fmt.Sprintf("%.0f%% deterministic-gated", float64(gated)/float64(total)*100)
			gate.Detail = fmt.Sprintf(
				"%d/%d merged PR(s) had >=1 code gate at head; %d relied on model-judgment gates only (reviewer-App verdicts). The split is shown, not collapsed.",
				gated, total, modelOnly)
		}
	} else {
		gate.Value = "unmeasured"
		gate.Reason = "gh-unreadable"
		gate.Detail = "gh merged-PR status-check rollups could not be read (offline / unauthenticated)"
	}
	rep.Axes = append(rep.Axes, gate)

	return rep
}

// renderAutonomyText renders the system with the anti-gaming header, one block
// per axis, each carrying its measured value (or unmeasured + reason), detail,
// and self-citing source.
func renderAutonomyText(rep AutonomyReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Autonomy / token / gate-share — %s … %s (step-3 adoption-ladder gauges)\n", rep.Since, rep.Until)
	fmt.Fprintf(&b, "%s\n\n", rep.Note)
	for _, a := range rep.Axes {
		fmt.Fprintf(&b, "%s\n  %s", a.Name, a.Value)
		if !a.Measured && a.Reason != "" {
			fmt.Fprintf(&b, "  [reason: %s]", a.Reason)
		}
		b.WriteString("\n")
		if a.Detail != "" {
			fmt.Fprintf(&b, "      %s\n", a.Detail)
		}
		fmt.Fprintf(&b, "      source: %s\n\n", a.Source)
	}
	return b.String()
}

// --- data gathering (exec/network — overridable in tests) ---------------------

// autonomyMergedAuthors lists merged PRs (in window) with author login + bot flag
// for the CI-proxy authorship split. ok=false on any gh failure — degrades to
// "unmeasured", never a fabricated zero (offline discipline, matching --dora).
var autonomyMergedAuthors = func(root string, since, until time.Time) ([]autonomyAuthor, bool) {
	out, err := exec.Command("gh", "pr", "list", "--state", "merged",
		"--limit", "500", "--json", "number,mergedAt,author").Output()
	if err != nil {
		return nil, false
	}
	var raw []struct {
		MergedAt time.Time `json:"mergedAt"`
		Author   struct {
			Login string `json:"login"`
			IsBot bool   `json:"is_bot"`
		} `json:"author"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, false
	}
	var authors []autonomyAuthor
	for _, p := range raw {
		if p.MergedAt.IsZero() || p.MergedAt.Before(since) || p.MergedAt.After(until) {
			continue
		}
		authors = append(authors, autonomyAuthor{Login: p.Author.Login, IsBot: p.Author.IsBot})
	}
	return authors, true
}

// autonomyGates lists merged PRs (in window) with their status-check rollup
// context names for the gate-share axis. ok=false on any gh failure.
var autonomyGates = func(root string, since, until time.Time) ([]autonomyGatePR, bool) {
	out, err := exec.Command("gh", "pr", "list", "--state", "merged",
		"--limit", "500", "--json", "number,mergedAt,statusCheckRollup").Output()
	if err != nil {
		return nil, false
	}
	var raw []struct {
		Number   int       `json:"number"`
		MergedAt time.Time `json:"mergedAt"`
		// gh returns a heterogeneous array: CheckRun entries carry .name,
		// StatusContext entries carry .context. Capture both.
		StatusCheckRollup []struct {
			Name    string `json:"name"`
			Context string `json:"context"`
		} `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, false
	}
	var out2 []autonomyGatePR
	for _, p := range raw {
		if p.MergedAt.IsZero() || p.MergedAt.Before(since) || p.MergedAt.After(until) {
			continue
		}
		var names []string
		for _, c := range p.StatusCheckRollup {
			if c.Name != "" {
				names = append(names, c.Name)
			}
			if c.Context != "" {
				names = append(names, c.Context)
			}
		}
		out2 = append(out2, autonomyGatePR{Number: p.Number, CheckNames: names})
	}
	return out2, true
}

// loadOpDayFile finds the most recent opmetrics day-file on or before `until`
// under docs/reports/daily/<date>/opmetrics.json and parses it into the consumer
// view. Returns (nil, "") when no day-file exists or none parses — the honest
// "unmeasured" input, never a fabricated empty report.
func loadOpDayFile(root string, until time.Time) (*opDayFile, string) {
	dir := filepath.Join(root, "docs", "reports", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, ""
	}
	var dates []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d, perr := time.Parse("2006-01-02", e.Name())
		if perr != nil || d.After(until) {
			continue
		}
		dates = append(dates, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	for _, d := range dates {
		path := filepath.Join(dir, d, "opmetrics.json")
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}
		var df opDayFile
		if json.Unmarshal(raw, &df) != nil {
			continue
		}
		return &df, d
	}
	return nil, ""
}

// rosterHumanLogins derives the genuine-individual login set from the roster's
// ASSAY_HUMAN_LOGIN_MAP (name→login). Those login VALUES are the humans the
// roster has established acted as individuals — the parser REFUSES a bot/App or
// shared-agent login in that map (rosterconfig.go), so the shared org account is
// never in here and correctly stays in the ambiguous bucket. Best-effort: any
// load failure yields an empty set, and every non-bot author then falls to
// ambiguous (the never-guess degrade).
func rosterHumanLogins() map[string]bool {
	out := map[string]bool{}
	cfg := scanLoadConfig(scanClassReadOnly)
	for _, login := range cfg.HumanLogins {
		if l := strings.ToLower(strings.TrimSpace(login)); l != "" {
			out[l] = true
		}
	}
	return out
}

// gatherAutonomyInputs assembles autonomyInputs from gh + the day-file. gh
// failures degrade to their ok=false flags; a missing day-file degrades both
// day-file axes to "unmeasured".
func gatherAutonomyInputs(root string, since, until, now time.Time) autonomyInputs {
	humans := rosterHumanLogins()
	// An explicit override (a test, or a caller that pre-seeded the set) wins.
	for l := range autonomyHumanLogins {
		humans[l] = true
	}
	in := autonomyInputs{Since: since, Until: until, Now: now, HumanLogins: humans}
	in.MergedAuthors, in.AuthorsOK = autonomyMergedAuthors(root, since, until)
	in.GateData, in.GateOK = autonomyGates(root, since, until)
	in.DayFile, in.DayFileDate = loadOpDayFile(root, until)
	return in
}

// runAutonomy is the --autonomy entrypoint: a self-contained diagnostic
// sub-command, same STATUS.md-free discipline as --dora/--bottleneck. It never
// reads or writes STATUS.md. Exit 0 always on a well-formed invocation — a
// missing source is reported "unmeasured", not an error.
func runAutonomy(root, since string, asJSON bool) int {
	now := nowFunc()
	until := now
	var sinceT time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t.UTC()
	} else {
		sinceT = until.AddDate(0, 0, -defaultAutonomyWindowDays)
	}
	if sinceT.After(until) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is in the future")
		return 1
	}

	rep := computeAutonomy(gatherAutonomyInputs(root, sinceT, until, now))
	if asJSON {
		enc, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Println(string(enc))
		return 0
	}
	fmt.Print(renderAutonomyText(rep))
	return 0
}
