package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// disposition.go — deskclose as the CONSUMER of ground-truth/05's disposition record.
//
// That record (`disposition:<verdict>` label + `<!-- desk-disposition v1 -->` marker
// comment carrying Evidence / Recorded-By / Recorded-At) is the one declared source for
// "a worker investigated this PR and concluded it is done with". It RECORDS but never
// CLOSES — its own doc comment names deskclose as the tool that acts on it.
//
// deskclose does not re-derive that verdict and does not parse the marker. It shells
// out to `deskdisposition read --json`, which owns the schema. Two copies of a parser
// is two things to drift, and the drift would be silent: a marker deskclose failed to
// recognise reads as "no record", which on the write side means "close it anyway".
//
// If ground-truth/05 has not landed in the caller's install, `deskdisposition` is
// simply not on PATH — and that is COULD-NOT-CHECK, exit 6, not "no record found".

// dispositionRead mirrors the JSON `deskdisposition read --json` emits. Only the
// fields deskclose acts on are declared; unknown fields are ignored, so the schema can
// grow without breaking this reader.
type dispositionRead struct {
	State  string `json:"State"`
	Reason string `json:"Reason"`
	Record struct {
		Verdict    string `json:"Verdict"`
		Evidence   string `json:"Evidence"`
		RecordedBy string `json:"RecordedBy"`
		RecordedAt string `json:"RecordedAt"`
	} `json:"Record"`
	DispatchEligible bool `json:"dispatchEligible"`
}

// The three instrument states, spelled exactly as ground-truth/05 spells them so a
// deskclose log line greps alongside a deskdisposition one.
const (
	dispCheckedClean  = "checked-clean"
	dispCheckedFailed = "checked-failed"
	dispCouldNotCheck = "could-not-check"
)

// The terminal verdicts. NEEDS-REBASE is deliberately absent: it means live work.
const (
	verdictSuperseded        = "SUPERSEDED"
	verdictResolvedElsewhere = "RESOLVED-ELSEWHERE"
	verdictNeedsRebase       = "NEEDS-REBASE"
)

// readDisposition asks the sibling tool for one PR's record.
func readDisposition(repo string, n int) (dispositionRead, error) {
	raw, err := runDisposition("read", "-R", repo, "--pr", fmt.Sprintf("%d", n), "--json")
	if err != nil {
		return dispositionRead{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read the disposition record for %s#%d via `%s read` — "+
				"an unreadable record is not an absent one, so the close is refused. "+
				"(ground-truth/05 must be installed alongside deskclose.)", repo, n, dispositionBin), err)
	}
	var d dispositionRead
	if jerr := json.Unmarshal([]byte(raw), &d); jerr != nil {
		return dispositionRead{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the disposition record for %s#%d did not parse", repo, n), jerr)
	}
	return d, nil
}

// requireTerminalDisposition gates the close of a PULL REQUEST.
//
// A PR is only closeable by deskclose when a worker has already recorded a terminal
// verdict on it with evidence. deskclose contributes the human authorization and the
// execution; it does not contribute the finding. Cases:
//
//	could-not-check   → exit 6. The record could not be read.
//	checked-clean     → exit 5. No record: nobody has concluded anything about this PR,
//	                    so there is nothing for a human to have authorized.
//	NEEDS-REBASE      → exit 5. The record says this is LIVE WORK, mechanically blocked.
//	                    Closing it would discard work a worker just confirmed real.
//	terminal + evidence that names a DIFFERENT target than the caller declared
//	                  → exit 5. The record and the caller disagree about what settled
//	                    it; a batch closer must not pick a winner.
func requireTerminalDisposition(repo string, n int, declaredTarget string) (dispositionRead, error) {
	d, err := readDisposition(repo, n)
	if err != nil {
		return d, err
	}
	switch d.State {
	case dispCouldNotCheck:
		return d, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the disposition record for %s#%d is unreadable (%s) — refusing",
			repo, n, deskkit.StripControl(d.Reason)), nil)
	case dispCheckedClean:
		return d, deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d carries no disposition record. deskclose executes a finding a worker has "+
				"already recorded; it does not make one. Record it first with "+
				"`%s set -R %s --pr %d --verdict SUPERSEDED --evidence <ref>`.",
			repo, n, dispositionBin, repo, n))
	case dispCheckedFailed:
	default:
		return d, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the disposition record for %s#%d reported an unknown state %q",
			repo, n, deskkit.StripControl(d.State)), nil)
	}

	v := strings.ToUpper(strings.TrimSpace(d.Record.Verdict))
	if v == verdictNeedsRebase {
		return d, deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is recorded %s — the record states this is LIVE WORK that is mechanically "+
				"blocked, not work that is finished with. Closing it discards a finding a worker just made.",
			repo, n, verdictNeedsRebase))
	}
	if v != verdictSuperseded && v != verdictResolvedElsewhere {
		return d, deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d carries disposition %q, which is not a terminal verdict",
			repo, n, deskkit.StripControl(d.Record.Verdict)))
	}
	if strings.TrimSpace(d.Record.Evidence) == "" {
		return d, deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is recorded %s with no Evidence — a terminal verdict with no link to what "+
				"settled it is the unfalsifiable claim the record exists to replace", repo, n, v))
	}
	if declaredTarget != "" && !refsAgree(d.Record.Evidence, declaredTarget) {
		return d, deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is recorded %s with Evidence %s, but the close declares target %s. "+
				"The record and the caller disagree about what settled this item; deskclose does not "+
				"pick a winner between them.",
			repo, n, v, deskkit.StripControl(d.Record.Evidence), deskkit.StripControl(declaredTarget)))
	}
	return d, nil
}

// refNumRe pulls the trailing item number out of any ref shape: `#123`,
// `owner/repo#123`, or a github.com issues/pull permalink.
var refNumRe = regexp.MustCompile(`(?:#|/(?:issues|pull)/)(\d+)\s*$`)

// refsAgree reports whether two item references name the same item. It compares the
// numbers, and the owner/repo when BOTH sides carry one — a bare `#123` is read in the
// context of the other ref rather than being called a mismatch, because that is how
// both a manifest and a marker comment are actually written.
func refsAgree(a, b string) bool {
	an, aok := refNum(a)
	bn, bok := refNum(b)
	if !aok || !bok || an != bn {
		return false
	}
	ar, br := refRepo(a), refRepo(b)
	if ar == "" || br == "" {
		return true
	}
	return strings.EqualFold(ar, br)
}

func refNum(s string) (int, bool) {
	m := refNumRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	n, err := atoiPositive(m[1], "ref")
	return n, err == nil
}

var refRepoRe = regexp.MustCompile(
	`(?:^|github\.com/)([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+?)(?:#|/(?:issues|pull)/)\d+\s*$`)

func refRepo(s string) string {
	m := refRepoRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return ""
	}
	return m[1]
}
