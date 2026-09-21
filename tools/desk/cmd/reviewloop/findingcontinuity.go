package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// findingcontinuity.go — the reactor's CONSUMER of the persistent review-finding record.
//
// reviewloop already reacts to board rows. This is the review-side of the persistent
// review-finding brief: given the durable forge records of ONE pull request's review thread
// (its reviewer verdicts and worker replies, each carrying a typed finding block), derive
// the outstanding findings, the disputed responses and the per-class round counts, and — at
// the existing cap — the single arbiter packet. The derivation lives in deskkit
// (reviewfinding.go); this file is the reactor's read-side: it turns an injected records
// payload into that ledger and RENDERS it, and it produces the COMPACT record the desk
// injects into the reviewer and worker prompts so a replacement agent resumes from the
// ledger rather than rereading the whole thread.
//
// Like the rest of this reactor it reads a FILE, makes no API calls, and is three-state by
// construction: a records payload it cannot positively interpret is could-not-check (exit
// 6), never an empty finding set. "No findings" and "could not read the findings" are the
// two answers this brief exists to keep apart.

// FindingRecord is one durable forge event as the records payload carries it. The body is
// the raw forge review/reply body — the typed finding block is PARSED OUT of it exactly as
// it will be in production, so the fixture path and the live path share one parser and a
// body that parses here parses there.
type FindingRecord struct {
	Seq     int    `json:"seq"`
	Kind    string `json:"kind"`    // "review" | "reply"
	Role    string `json:"role"`    // "reviewer" | "worker" — from the authenticated event
	Actor   string `json:"actor"`   // authenticated login
	Head    string `json:"head"`    // head SHA the record was authored against
	Verdict string `json:"verdict"` // review records: approve | request-changes | comment
	Body    string `json:"body"`    // the raw forge body carrying the finding block (if any)
}

// FindingRecordsReport is the injected records payload for one PR's review thread.
type FindingRecordsReport struct {
	Repo        string          `json:"repo"`
	PR          int             `json:"pr"`
	CurrentHead string          `json:"currentHead"`
	Records     []FindingRecord `json:"records"`
}

// ReadRecords decodes a records payload and lifts each record into a deskkit.ForgeRecord,
// parsing the typed finding block out of its body. It is three-state:
//
//   - a body with NO block is a legacy record and folds in as one (nil block) — it asserts
//     nothing about the ledger, and is NOT read as a clean finding set.
//   - a body whose block is present but MALFORMED poisons the whole read (exit 6): a reactor
//     handed a corrupt record cannot derive a trustworthy ledger, so it fails closed rather
//     than silently dropping the record — the same direction board.go takes on an unknown
//     ACTION.
//   - a record missing its authenticated role or head is carried through as-is; the
//     derivation reports it as could-not-check (it never clears a finding), so the read does
//     not have to re-implement that judgement.
func ReadRecords(data []byte) (*FindingRecordsReport, []deskkit.ForgeRecord, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil, deskkit.Unverifiable("reviewloop: the --records payload is empty — BLIND, not an empty finding set", nil)
	}
	var rep FindingRecordsReport
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rep); err != nil {
		return nil, nil, deskkit.Unverifiable("reviewloop: cannot decode the --records JSON — BLIND, not an empty finding set", err)
	}
	out := make([]deskkit.ForgeRecord, 0, len(rep.Records))
	for _, r := range rep.Records {
		block, present, perr := deskkit.ParseFindingBlock(r.Body)
		if perr != nil {
			return nil, nil, deskkit.Unverifiable(fmt.Sprintf(
				"reviewloop: record seq %d carries a malformed finding block — a corrupt record poisons the derivation, fail closed: %v",
				r.Seq, perr), perr)
		}
		if !present {
			block = nil
		}
		out = append(out, deskkit.ForgeRecord{
			Seq:     r.Seq,
			Kind:    deskkit.RecordKind(r.Kind),
			Role:    deskkit.ActorRole(r.Role),
			Actor:   r.Actor,
			Head:    r.Head,
			Verdict: deskkit.Verdict(r.Verdict),
			Block:   block,
		})
	}
	return &rep, out, nil
}

// DeriveContinuity folds records into the finding ledger. It is a thin, named seam over
// deskkit.DeriveLedger so this package (and its tests) name one entry point for the
// derivation, exactly as ReadBoard is the one entry to board state.
func DeriveContinuity(records []deskkit.ForgeRecord) *deskkit.FindingLedger {
	return deskkit.DeriveLedger(records)
}

// CompactRecord is the compact finding summary the desk injects into a reviewer or worker
// prompt so a replacement agent resumes from the ledger. It is deterministic (sorted) and
// carries exactly what continuity needs: each outstanding finding's ID, class, state and
// blocker kind; the per-class round count and the cap; and the shared repairs. It never
// carries prose to re-litigate — a successor reads what is disputed and what would resolve
// it, not a retelling of the thread.
func CompactRecord(l *deskkit.FindingLedger) string {
	var b strings.Builder
	fmt.Fprintf(&b, "review-finding ledger (derived; survives agent replacement and restart):\n")
	open := l.OpenBlocking()
	if len(open) == 0 {
		fmt.Fprintf(&b, "  outstanding blocking findings: none\n")
	}
	for _, id := range open {
		f := l.Findings[id]
		fmt.Fprintf(&b, "  - %s [class=%s state=%s blocker=%s rounds=%d/%d]", id, f.Class, f.State, f.Blocker, l.Rounds[f.Class], deskkit.RoundCap)
		if f.Blocker == deskkit.BlockerExternalPrereq && f.SharedRepair != "" {
			fmt.Fprintf(&b, " sharedRepair=%s", f.SharedRepair)
		}
		fmt.Fprintf(&b, "\n")
	}
	classes := make([]string, 0, len(l.Rounds))
	for c := range l.Rounds {
		classes = append(classes, c)
	}
	sort.Strings(classes)
	for _, c := range classes {
		held := ""
		if l.Held[c] {
			held = " HELD (awaiting-arbitration)"
		}
		fmt.Fprintf(&b, "  class %q: %d/%d rounds%s\n", c, l.Rounds[c], deskkit.RoundCap, held)
	}
	return b.String()
}

// RenderFindings prints the derived finding-continuity section of a plan run. It states the
// outstanding findings, the per-class rounds against the cap, the arbiter packets, and every
// could-not-check reason — the reactor's three-state discipline applied to the finding
// ledger the same way the idle gate applies it to the board.
func RenderFindings(w io.Writer, rep *FindingRecordsReport, l *deskkit.FindingLedger) {
	fmt.Fprintf(w, "\nREVIEW-FINDING CONTINUITY (%s#%d, current head %s) — derived from durable forge records, survives agent replacement:\n",
		rep.Repo, rep.PR, short(firstNonEmptyHead(rep.CurrentHead)))

	open := l.OpenBlocking()
	fmt.Fprintf(w, "  outstanding blocking findings: %d\n", len(open))
	for _, id := range open {
		f := l.Findings[id]
		note := ""
		if f.Blocker == deskkit.BlockerExternalPrereq {
			note = " (shared prerequisite — a shared repair, not a per-PR content defect; ready-flip still requires the applicable checks)"
			if f.SharedRepair != "" {
				note = " (shared repair " + f.SharedRepair + " — not a per-PR content defect; ready-flip still requires the applicable checks)"
			}
		}
		fmt.Fprintf(w, "    - %-10s class=%-16s state=%-22s rounds=%d/%d%s\n",
			id, f.Class, f.State, l.Rounds[f.Class], deskkit.RoundCap, note)
	}

	if repairs := l.SharedRepairs(); len(repairs) > 0 {
		fmt.Fprintf(w, "  shared repairs cited (one repair, however many PRs cite it): %s\n", strings.Join(repairs, ", "))
	}

	if len(l.Arbiter) > 0 {
		fmt.Fprintf(w, "  ARBITER PACKETS (one per class at the %d-round cap — filed to the human decision lane, never an auto-overrule):\n", deskkit.RoundCap)
		for _, p := range l.Arbiter {
			fmt.Fprintf(w, "    - class=%s rounds=%d findings=%s\n      %s\n", p.Class, p.Rounds, strings.Join(p.FindingIDs, ","), p.Summary)
		}
	}

	if len(l.Blind) > 0 {
		fmt.Fprintf(w, "  COULD-NOT-CHECK (reported as itself — never rounded to a resolution or a clean set):\n")
		for _, r := range l.Blind {
			fmt.Fprintf(w, "    - %s\n", r)
		}
	}
}

func firstNonEmptyHead(h string) string {
	if strings.TrimSpace(h) == "" {
		return HeadUnresolved
	}
	return h
}
