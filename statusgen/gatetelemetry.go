package main

// --gate-telemetry: gate-effectiveness telemetry — the trust/risk metric
// families a boolean flow report lacks. Two failure directions bound the
// design: a forged human: token silently suppressing a gate (the override-rate
// family) and a detector left unwirable with the suite green (the catch-rate /
// ceremonial-gate family). The three-state source semantics that keep an
// unread source from reading as a clean zero are documented on gtSourceState
// below.
//
// This is a REPORT, not a boolean gate — same family as --dora/--trend/
// --bottleneck: the report's CONTENT (an override, a ceremonial gate) never
// fails the process, only the INSTRUMENT's own ability to read its sources
// does. Exit codes (pinned by brief-01's Verify table, brief-rules 14):
//
//	0 = ran to completion, every source read and understood (report may still alarm)
//	1 = a source exists but is malformed — an instrument defect, not "nothing to report"
//	2 = usage error (claimed by the flag package / statusgen's own arg errors)
//	3 = could-not-check — at least one metric's source surface could not be read
//	    or could not be understood for this window (distinct from a legitimate
//	    zero and from malformed input)
//
// THE FAILURE DIRECTION THIS TOOL MUST NOT HAVE: a gate-effectiveness
// instrument that reports a reassuring number it
// did not actually measure is worse than one that reports nothing. A zero fire
// count is the ceremonial ALARM condition, so any path that produces a zero
// without having understood its source affirmatively accuses a working gate.
// Three shapes produce that zero and all three are now could-not-check, never 0:
//
//	1. the source file is absent;
//	2. the source file is present but contains no records (a 0-byte append-only
//	   log is a collection failure — log rotation, a collector that created the
//	   file then died, a truncating write — not evidence of a quiet window);
//	3. the source file contains records in a shape this tool does not recognize
//	   (a schema drift between producer and reader).
//
// Shape 3 was not hypothetical. An earlier audit reader expected an invented
// {"event","gate","blockedDefect"} schema that NO producer has ever emitted;
// fed a real desk audit log (the deskkit audit-entry schema, carrying genuine
// deskpost refusals and ready-flips) it printed "gate-class deskpost-refusal:
// fires=0 ... ceremonial-or-untested" and exited 0. The reader below reads the
// producer's ACTUAL schema, and treats an unrecognized line shape as
// could-not-check rather than as an absence of fires.
//
// --root points directly at ONE window's fixture/data directory (unlike the
// repo-tree roots the rest of statusgen reads) containing:
//
//	pr-verdicts.json     — []gtPRVerdict, PR review threads (App verdict + human outcome)
//	defect-findings.json — []gtDefectFinding, FINDINGS register + bug-labeled issues
//	gates.json           — []gtGateClass, per-gate-class fire records + mutation-test marker
//	audit.jsonl          — the desk audit log, deskkit.Entry schema, one JSON
//	                        object per line (its absence is what the missing-audit
//	                        fixture proves)
//
// Collection status (brief-01 Task 2): audit.jsonl has a real producer in this
// repo and is read in its native schema. The three JSON surfaces do NOT yet
// have a producer — they are hand-authored windows. See brief-01 Task 2's scope
// note for the follow-up that owns the GitHub-sourced collector.
import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	gtExitCheckedFailed = 1
	// 2 is claimed by the flag package's usage-error exit and statusgen's own
	// arg-parsing errors (main.go) — never reused here.
	gtExitCouldNotCheck = 3

	// gtSmallN — denominators below this are flagged so a downstream reader
	// cannot quietly turn 1/1 into "100%" (review finding 7).
	gtSmallN = 5
)

// gtSourceState is the three-state condition of ONE input surface
// (docs/three-state-instrument-rule.md). Only gtSourceUsable may produce a
// number; the other three all render could-not-check.
type gtSourceState int

const (
	// gtSourceUsable — the surface was read and its records were understood.
	// The metric it backs may still be legitimately zero.
	gtSourceUsable gtSourceState = iota
	// gtSourceAbsent — the file does not exist for this window.
	gtSourceAbsent
	// gtSourceEmpty — the file exists but carries no records. For an
	// append-only log this is a collection failure, not a quiet window. A
	// well-formed empty JSON array is NOT this state: `[]` is an affirmative
	// "zero rows", which is a defensible measured zero.
	gtSourceEmpty
	// gtSourceUnrecognized — the file carries records, none in a shape this
	// tool understands. Producer/reader schema drift.
	gtSourceUnrecognized
)

// gtSource pairs a surface's state with the reason the report prints for it.
type gtSource struct {
	state  gtSourceState
	reason string
}

func (s gtSource) usable() bool { return s.state == gtSourceUsable }

// gtPRVerdict is one row of pr-verdicts.json — a PR's App review verdict and
// its terminal human outcome, the source for override-rate (a) and (b).
type gtPRVerdict struct {
	Number     int    `json:"number"`
	AppVerdict string `json:"appVerdict"` // "APPROVED" | "CHANGES_REQUESTED" | ...
	Outcome    string `json:"outcome"`    // "merged" | "human-rejected" | "reworked" | "closed-unmerged"
}

// recognized reports whether the row carries the discriminator every real row
// must have. An array of shapeless objects is schema drift, not a set of PRs.
func (v gtPRVerdict) recognized() bool { return v.Number != 0 }

// approved reports whether the App gate called this PR clean — the scope of
// BOTH override-rate legs (a) and (b). The family definition is "gates that
// pass work an App verdict called clean"; a PR the App never approved is not
// evidence about the App gate.
func (v gtPRVerdict) approved() bool { return v.AppVerdict == "APPROVED" }

// reversed reports whether a human outcome overturned an App APPROVED verdict.
func (v gtPRVerdict) reversed() bool {
	if !v.approved() {
		return false
	}
	switch v.Outcome {
	case "human-rejected", "reworked", "closed-unmerged":
		return true
	default:
		return false
	}
}

// merged reports a terminal merge outcome.
func (v gtPRVerdict) merged() bool { return v.Outcome == "merged" }

// gtDefectFinding is one row of defect-findings.json — a FINDINGS-register
// entry or bug-labeled issue naming a merged PR, the source for override-rate
// (b): merged work an App verdict called clean that a later defect discovery
// named.
type gtDefectFinding struct {
	PR int `json:"pr"`
}

func (f gtDefectFinding) recognized() bool { return f.PR != 0 }

// gtGateFire is one fire of a non-audit-sourced gate class, inline in
// gates.json.
type gtGateFire struct {
	PR            int  `json:"pr"`
	BlockedDefect bool `json:"blockedDefect"`
}

// gtGateClass is one row of gates.json — a gate class (App review verdict,
// security review, statusgen lint PROBLEM, corroborate, deskpost refusals, CI
// red, ...) with its fire record and mutation-test marker (brief-rules 16).
//
// AuditSourced classes carry their fire counts in the desk audit log instead of
// the inline Fires field; they are resolved through gtAuditGateSelectors.
type gtGateClass struct {
	Class          string       `json:"class"`
	MutationTested bool         `json:"mutationTested"`
	AuditSourced   bool         `json:"auditSourced"`
	Fires          []gtGateFire `json:"fires"` // ignored when AuditSourced
}

func (g gtGateClass) recognized() bool { return g.Class != "" }

// --- the desk audit log, in the producer's own schema ---------------------
//
// The deskkit audit-entry schema is the ONLY shape any producer writes. These
// field names and the verb/result vocabulary belong to that producer; changing
// them here without changing it there re-opens exactly the silent divergence
// this reader now refuses to have.

// deskkit result values this reader depends on (deskkit.Result* in audit.go).
const (
	gtResultOK      = "ok"
	gtResultRefused = "refused"
)

// gtVerbReadyReversal is the verb a ready-flip reversal WOULD carry. No desk
// tool emits it today — which is why override-rate (c)'s numerator reports
// could-not-check rather than 0 when none is present (review finding 4).
const gtVerbReadyReversal = "ready-reversal"

// gtAuditEntry is one line of audit.jsonl. Only the fields this report needs
// are declared; deskkit.Entry carries more.
type gtAuditEntry struct {
	Tool   string `json:"tool"`
	Verb   string `json:"verb"`
	PR     *int   `json:"pr"`
	Result string `json:"result"`
}

// recognized reports whether the line is a desk audit record at all. Every
// deskkit.Entry sets both fields — Log() refuses an entry with no Result. A
// line lacking them is some other file's content, and must never be counted as
// "no fires".
func (e gtAuditEntry) recognized() bool { return e.Tool != "" && e.Result != "" }

func (e gtAuditEntry) isReadyFlip() bool {
	return e.Tool == "deskpost" && e.Verb == "ready" && e.Result == gtResultOK
}

func (e gtAuditEntry) isReadyFlipReversal() bool {
	return e.Tool == "deskpost" && e.Verb == gtVerbReadyReversal
}

// gtAuditGateSelectors maps an audit-sourced gate class to the predicate that
// recognizes one of its fires in the desk audit log.
//
// A class marked auditSourced whose name is NOT in this table reports
// could-not-check — never "0 fires". An unknown selector matches nothing, and
// matching nothing is indistinguishable from a gate that never fired, which is
// the ceremonial ALARM. A detector must not be able to accuse a gate it never
// looked for.
var gtAuditGateSelectors = map[string]func(gtAuditEntry) bool{
	"deskpost-refusal": func(e gtAuditEntry) bool {
		return e.Result == gtResultRefused
	},
	"app-review-request-changes": func(e gtAuditEntry) bool {
		return e.Tool == "deskpost" &&
			(e.Verb == "review:request-changes" || e.Verb == "review:correctness:request-changes")
	},
	"security-review-request-changes": func(e gtAuditEntry) bool {
		return e.Tool == "deskpost" && e.Verb == "review:security:request-changes"
	},
}

// runGateTelemetry loads one window's data directory and prints the
// gate-effectiveness report to stdout. Returns the process exit code.
func runGateTelemetry(root string) int {
	couldNotCheck := false
	note := func() { couldNotCheck = true }

	verdicts, verdictsSrc, err := loadGtJSON[gtPRVerdict](filepath.Join(root, "pr-verdicts.json"), "pr-verdicts.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: gate-telemetry: pr-verdicts.json:", err)
		return gtExitCheckedFailed
	}
	findings, findingsSrc, err := loadGtJSON[gtDefectFinding](filepath.Join(root, "defect-findings.json"), "defect-findings.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: gate-telemetry: defect-findings.json:", err)
		return gtExitCheckedFailed
	}
	gates, gatesSrc, err := loadGtJSON[gtGateClass](filepath.Join(root, "gates.json"), "gates.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: gate-telemetry: gates.json:", err)
		return gtExitCheckedFailed
	}
	auditEntries, auditSrc, err := loadAuditLog(filepath.Join(root, "audit.jsonl"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: gate-telemetry: audit.jsonl:", err)
		return gtExitCheckedFailed
	}

	fmt.Printf("GATE-TELEMETRY root=%s\n", root)

	// --- override-rate (trust family) ---
	fmt.Println("override-rate:")

	// (a) app-approved-then-human-reversed — pr-verdicts.json.
	if !verdictsSrc.usable() {
		fmt.Printf("  a) app-approved-then-human-reversed: could-not-check (%s)\n", verdictsSrc.reason)
		note()
	} else {
		var num, den int
		var names []string
		for _, v := range verdicts {
			if !v.approved() {
				continue
			}
			den++
			if v.reversed() {
				num++
				names = append(names, fmt.Sprintf("#%d", v.Number))
			}
		}
		sort.Strings(names)
		fmt.Printf("  a) app-approved-then-human-reversed: override-rate %d/%d%s%s\n",
			num, den, formatNames(names), denominatorMarker(den))
	}

	// (b) merged-PR-named-by-defect-finding — defect-findings.json, denominator
	// = App-APPROVED PRs that merged.
	//
	// The denominator is APPROVED-AND-merged, not all-merged (review finding
	// 3). This metric's job is to say how often the App gate passes work that
	// turns out to be defective; a merge the App gate never called clean is not
	// evidence about the App gate, and counting it dilutes the rate — always in
	// the direction of making the gate look better than it is.
	switch {
	case !findingsSrc.usable():
		fmt.Printf("  b) merged-PR-named-by-defect-finding: could-not-check (%s)\n", findingsSrc.reason)
		note()
	case !verdictsSrc.usable():
		// Denominator source unusable too — already counted by leg (a); do not
		// double-count, but this leg is unanswerable either way.
		fmt.Printf("  b) merged-PR-named-by-defect-finding: could-not-check (%s)\n", verdictsSrc.reason)
	default:
		approvedMerged := map[int]bool{}
		inWindow := map[int]bool{}
		for _, v := range verdicts {
			inWindow[v.Number] = true
			if v.approved() && v.merged() {
				approvedMerged[v.Number] = true
			}
		}
		var num, outside int
		var names []string
		seen := map[int]bool{}
		for _, f := range findings {
			if !inWindow[f.PR] {
				outside++
				continue
			}
			if approvedMerged[f.PR] && !seen[f.PR] {
				seen[f.PR] = true
				num++
				names = append(names, fmt.Sprintf("#%d", f.PR))
			}
		}
		sort.Strings(names)
		fmt.Printf("  b) merged-PR-named-by-defect-finding: override-rate %d/%d%s%s\n",
			num, len(approvedMerged), formatNames(names), denominatorMarker(len(approvedMerged)))
		if outside > 0 {
			// Without this line a numerator can shrink silently whenever the
			// window's verdict list is incomplete (review finding 3).
			fmt.Printf("     note: %d defect finding(s) named PRs outside this window's pr-verdicts.json — not counted\n", outside)
		}
	}

	// (c) human-gate-flip-reversal — the desk audit log.
	//
	// Denominator counts DISTINCT PRs that were ready-flipped, not log rows, so
	// the metric does not depend on whether a reversal is appended alongside
	// the original flip or logged in its place (review finding 4: the same real
	// event previously read 1/2 or 1/1 depending on log shape).
	if !auditSrc.usable() {
		fmt.Printf("  c) human-gate-flip-reversal: could-not-check (%s)\n", auditSrc.reason)
		note()
	} else {
		flipped := map[int]bool{}
		reversed := map[int]bool{}
		for _, e := range auditEntries {
			if e.PR == nil {
				continue
			}
			if e.isReadyFlip() {
				flipped[*e.PR] = true
			}
			if e.isReadyFlipReversal() {
				flipped[*e.PR] = true
				reversed[*e.PR] = true
			}
		}
		if len(reversed) == 0 {
			// No desk tool emits a ready-flip reversal record, so "zero
			// reversals seen" cannot be distinguished from "reversals are
			// invisible on this surface". Reporting 0 here would be the tool
			// asserting a clean trust metric it never measured.
			fmt.Printf("  c) human-gate-flip-reversal: could-not-check "+
				"(numerator unobservable: no desk tool emits a %q record; %d ready-flip(s) seen)\n",
				gtVerbReadyReversal, len(flipped))
			note()
		} else {
			fmt.Printf("  c) human-gate-flip-reversal: override-rate %d/%d%s\n",
				len(reversed), len(flipped), denominatorMarker(len(flipped)))
		}
	}

	// --- catch-rate + ceremonial-gate detection (risk family) ---
	if !gatesSrc.usable() {
		fmt.Printf("gate-classes: could-not-check (%s)\n", gatesSrc.reason)
		note()
	} else {
		sorted := make([]gtGateClass, len(gates))
		copy(sorted, gates)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Class < sorted[j].Class })
		for _, g := range sorted {
			if !g.AuditSourced {
				blocked := 0
				for _, f := range g.Fires {
					if f.BlockedDefect {
						blocked++
					}
				}
				printGateClassLine(gtGateReport{
					class: g.Class, fires: len(g.Fires), blocked: blocked,
					catchKnown: true, mutationTested: g.MutationTested,
				})
				continue
			}
			if !auditSrc.usable() {
				fmt.Printf("gate-class %s: could-not-check (%s)\n", g.Class, auditSrc.reason)
				note()
				continue
			}
			match, ok := gtAuditGateSelectors[g.Class]
			if !ok {
				// See gtAuditGateSelectors: an unknown selector matches
				// nothing, and "matched nothing" must never render as "never
				// fired" — that is the ceremonial alarm.
				fmt.Printf("gate-class %s: could-not-check "+
					"(auditSourced, but no audit selector is defined for this class — a fire count of 0 would be unmeasured)\n", g.Class)
				note()
				continue
			}
			fires := 0
			for _, e := range auditEntries {
				if match(e) {
					fires++
				}
			}
			// The desk audit log records that a gate fired, never whether the
			// fire blocked a real defect (deskkit.Entry has no such field, and
			// the brief's proxy — "was the PR subsequently amended before
			// merge" — is not derivable from this surface). Fire count is
			// measured; catch rate is not.
			printGateClassLine(gtGateReport{
				class: g.Class, fires: fires, blocked: 0,
				catchKnown: false, mutationTested: g.MutationTested,
			})
			if fires > 0 {
				note()
			}
		}
	}

	if couldNotCheck {
		return gtExitCouldNotCheck
	}
	return 0
}

// gtGateReport is one gate class's resolved numbers, plus whether the catch
// rate was observable at all on the surface that supplied the fires.
type gtGateReport struct {
	class          string
	fires          int
	blocked        int
	catchKnown     bool
	mutationTested bool
}

// printGateClassLine prints one gate-class row, including ceremonial-gate
// detection (brief-rules 16 cross-reference).
//
// Two ceremony shapes are reported, not one (review finding 6): a gate that
// never fires, and a gate that fires constantly and blocks nothing. The second
// is arguably the more expensive — it spends the fleet's attention every time —
// and was previously invisible.
func printGateClassLine(r gtGateReport) {
	switch {
	case r.fires == 0 && !r.mutationTested:
		fmt.Printf("gate-class %s: fires=0 mutationTested=false ceremonial-or-untested\n", r.class)
	case r.fires == 0 && r.mutationTested:
		// mutationTested is an OPERATOR ASSERTION read straight out of
		// gates.json — nothing corroborates it against a real mutation-test
		// artifact (review finding 5). Setting one boolean moves a gate out of
		// the alarm, so the line says where the claim came from; a reader must
		// not mistake it for proof.
		fmt.Printf("gate-class %s: fires=0 mutationTested=true proven-able-to-fire "+
			"(operator-asserted in gates.json; NOT corroborated against a mutation-test artifact)\n", r.class)
	case !r.catchKnown:
		fmt.Printf("gate-class %s: fires=%d mutationTested=%t catch-rate=could-not-check "+
			"(this surface records that the gate fired, not whether the fire blocked a real defect)\n",
			r.class, r.fires, r.mutationTested)
	case r.blocked == 0:
		fmt.Printf("gate-class %s: fires=%d blocked=0 mutationTested=%t catch-rate=0/%d fires-without-catching%s\n",
			r.class, r.fires, r.mutationTested, r.fires, denominatorMarker(r.fires))
	default:
		fmt.Printf("gate-class %s: fires=%d blocked=%d mutationTested=%t catch-rate=%d/%d%s\n",
			r.class, r.fires, r.blocked, r.mutationTested, r.blocked, r.fires, denominatorMarker(r.fires))
	}
}

// denominatorMarker annotates a fraction whose denominator cannot carry the
// weight a reader will put on it (review finding 7). The report prints raw
// fractions and never percentages, but any consumer that divides them
// reintroduces the "0% over zero samples" failure, so a zero or small N says so
// on the line itself.
func denominatorMarker(den int) string {
	switch {
	case den == 0:
		return " n=0 (no data — not a rate)"
	case den < gtSmallN:
		return fmt.Sprintf(" small-n (n=%d)", den)
	}
	return ""
}

// formatNames renders a space-prefixed, parenthesized name list, or "" when
// empty — kept off the line entirely rather than printing "()" or leaving a
// trailing space for a clean zero.
func formatNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	out := " ("
	for i, n := range names {
		if i > 0 {
			out += " "
		}
		out += n
	}
	return out + ")"
}

// loadGtJSON reads a JSON-array source file and classifies it three ways.
//
//   - absent                          → gtSourceAbsent (could-not-check)
//   - well-formed but no rows (`[]`)  → gtSourceUsable; `[]` is an affirmative
//     "zero rows" someone wrote, which is a defensible measured zero
//   - rows present, none recognized   → gtSourceUnrecognized (could-not-check);
//     producer/reader schema drift must not read as an absence of events
//   - unparseable                     → hard error (checked-failed, exit 1)
func loadGtJSON[T interface{ recognized() bool }](path, label string) ([]T, gtSource, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, gtSource{gtSourceAbsent, label + " missing"}, nil
		}
		return nil, gtSource{gtSourceAbsent, label + " unreadable"}, err
	}
	var rows []T
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, gtSource{}, fmt.Errorf("malformed JSON: %w", err)
	}
	recognized := 0
	for _, r := range rows {
		if r.recognized() {
			recognized++
		}
	}
	if len(rows) > 0 && recognized == 0 {
		return nil, gtSource{gtSourceUnrecognized, fmt.Sprintf(
			"%s: %d row(s), 0 recognized — schema mismatch, not an absence of records", label, len(rows))}, nil
	}
	return rows, gtSource{gtSourceUsable, ""}, nil
}

// loadAuditLog reads audit.jsonl (deskkit.Entry, one JSON object per line) and
// classifies it three ways. Unlike the JSON-array sources, an EMPTY log is
// could-not-check: an append-only log that carries no lines is a collection
// failure (rotation, a collector that created the file then died, a truncating
// write), not evidence that the window was quiet. Rendering it as a measured
// zero is what made a 0-byte file accuse a live gate of being ceremonial while
// exiting 0.
func loadAuditLog(path string) ([]gtAuditEntry, gtSource, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, gtSource{gtSourceAbsent, "audit.jsonl missing"}, nil
		}
		return nil, gtSource{gtSourceAbsent, "audit.jsonl unreadable"}, err
	}
	defer f.Close()

	var entries []gtAuditEntry
	lines, recognized := 0, 0
	sc := bufio.NewScanner(f)
	// deskkit writes long detail fields; its own reader raises the cap the same
	// way (deskkit.LoadEntries). A default-buffer overflow would surface as a
	// scan error below, never as a short read.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			continue
		}
		lines++
		var e gtAuditEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, gtSource{}, fmt.Errorf("malformed line %d: %w", lines, err)
		}
		if !e.recognized() {
			continue
		}
		recognized++
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, gtSource{gtSourceAbsent, "audit.jsonl scan error"}, err
	}
	switch {
	case lines == 0:
		return nil, gtSource{gtSourceEmpty,
			"audit.jsonl present but empty — an append-only log with no lines is a collection failure, not a quiet window"}, nil
	case recognized == 0:
		return nil, gtSource{gtSourceUnrecognized, fmt.Sprintf(
			"audit.jsonl: %d line(s), 0 recognized as deskkit audit records — schema mismatch, not an absence of events", lines)}, nil
	}
	return entries, gtSource{gtSourceUsable, ""}, nil
}
