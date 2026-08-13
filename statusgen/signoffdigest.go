package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The human-gate sign-off digest (methodology-metrics/38).
//
// mm/12 already opens ONE issue PER BRIEF awaiting a human sign-off. That is the
// per-brief surface and it stays exactly as it is. What it does not give the
// human is a QUEUE: N scattered cards, no age on any of them, nothing that says
// which one has been waiting longest. This file adds the roll-up — one daily
// digest, oldest-first, each row carrying its recorded Evidence link — plus the
// per-stream age-at-gate metric that puts the same number on the board.
//
// Two properties are load-bearing and are why this file is small:
//
//  1. DERIVE, NEVER HAND-MAINTAIN. Membership comes from calling verifyIssues()
//     — mm/12's own eligibility predicate — not from a second copy of the rule.
//     There is no stored list of "what is waiting"; every run recomputes it. A
//     change to the eligibility rule moves the digest with it, or fails a test.
//
//  2. THREE STATES, NEVER TWO. "Nothing is waiting" and "I could not read my
//     inputs" are DIFFERENT FACTS and must read differently. A digest that
//     renders an unreadable repo as an empty list is worse than no digest: it
//     actively reports an all-clear it never established. Ages degrade
//     SEPARATELY from membership — an unreadable historian withdraws the
//     oldest-first CLAIM rather than hiding the queue.
//
// Surfacing only. The digest reports; it never closes, flips, or nudges. Only a
// human's close advances a brief (the closer allowlist is unchanged).
const (
	// signoffAwaiting — the inputs were read and briefs are waiting.
	signoffAwaiting = "awaiting"
	// signoffClear — the inputs were read and nothing is waiting. A POSITIVE
	// finding, established by a successful read.
	signoffClear = "clear"
	// signoffCouldNotCheck — the inputs could not be read. Establishes NOTHING
	// about the queue, and must never render as signoffClear.
	signoffCouldNotCheck = "could-not-check"
)

// signoffDigestMarker is the hidden idempotency key, so a workflow can find and
// UPDATE the day's digest instead of opening a new one every morning.
const signoffDigestMarker = "<!-- signoff-digest -->"

// signoffEntry is one brief awaiting a human sign-off.
type signoffEntry struct {
	Brief  string // "<stream>/<NN>"
	Title  string
	Stream string
	Status string // README-row status: implemented | verified
	// Age is the RENDERED age at the gate, or "—" when the historian has no
	// record. Never "0" for unknown — an invented zero reads as "just arrived",
	// the opposite of the truth for a brief older than the log.
	Age string
	// EnteredAt is the unrendered instant the brief entered its current status.
	// Zero = unknown; the sort keeps those LAST rather than treating the zero
	// time as infinitely old.
	EnteredAt time.Time
	// Evidence links into the brief's recorded Evidence section, so the human can
	// act from the digest alone rather than hunting the brief file.
	Evidence string
}

// signoffDigest is one day's rendered queue.
type signoffDigest struct {
	Date    string
	State   string // signoffAwaiting | signoffClear | signoffCouldNotCheck
	Reason  string // why the inputs could not be read (could-not-check only)
	Entries []signoffEntry
	// AgesKnown is false when the historian could not be read. Membership is
	// still trustworthy; the ORDERING is not, and the render says so out loud.
	AgesKnown bool
}

// streamGateAge is one row of the per-stream age-at-gate metric: how long the
// longest-waiting gate:human brief in a stream has sat in its current awaiting
// status. Age "—" with an empty Brief means the stream HAS something at the gate
// but the historian has no record of when it arrived — unknown, never zero.
type streamGateAge struct {
	Stream string
	Brief  string
	Age    string
}

// awaitingGateStatuses are the statuses that count as "sitting at the gate".
var awaitingGateStatuses = map[string]bool{"implemented": true, "verified": true}

// signoffBriefMeta is per-brief display data (title, path, row status) looked up
// for briefs verifyIssues has ALREADY selected. Deliberately a lookup and not a
// filter: nothing here may add or remove a brief from the digest.
type signoffBriefMeta struct {
	stream string
	path   string
	title  string
	status string
}

// indexBriefMeta walks the loaded streams once and maps brief id → display data.
func indexBriefMeta(streams []*Stream) map[string]signoffBriefMeta {
	index := make(map[string]signoffBriefMeta)
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			m := signoffBriefMeta{stream: s.Name, path: path, title: bf.Title}
			if _, num, okName := expectedBriefID(path); okName {
				if row := findRow(s, num); row != nil {
					m.status = row.Status
				}
			}
			index[bf.Brief] = m
		}
	}
	return index
}

// evidenceLink renders a link to a brief's Evidence section. Absolute
// (github.com/<slug>/blob/main/…) when the roster names a home repo, since the
// digest is read in a GitHub issue where repo-relative links do not resolve;
// repo-relative otherwise, which still tells the reader where to look. Same
// offline-only derivation renderVerifyBody uses — no network, no git remote.
func evidenceLink(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	if slug := verifyRepoSlug(); slug != "" {
		return "https://github.com/" + slug + "/blob/main/" + rel + "#evidence"
	}
	return rel + "#evidence"
}

// buildSignoffDigest computes the day's digest.
//
// entered is the historian's per-brief "entered its current status" map (from
// awaitingEnteredAt); agesKnown reports whether that map came from a historian
// that was actually READ. The two are separate arguments on purpose: a nil map
// from a SUCCESSFUL read means "no brief has a recorded transition", while a nil
// map from a FAILED read means "I do not know" — and only the second may
// withdraw the oldest-first claim.
func buildSignoffDigest(root string, streams []*Stream, entered map[string]time.Time, agesKnown bool, now time.Time) signoffDigest {
	d := signoffDigest{
		Date:      now.Format("2006-01-02"),
		State:     signoffClear,
		AgesKnown: agesKnown,
	}

	// Membership is mm/12's predicate, called live. Passing an EMPTY
	// existing-marker set is deliberate: the digest lists everything AT the gate,
	// not just what has no card yet — an already-carded brief that has waited ten
	// days is precisely what this roll-up exists to surface.
	eligible := verifyIssues(root, streams, map[string]bool{})
	if len(eligible) == 0 {
		return d
	}
	d.State = signoffAwaiting

	index := indexBriefMeta(streams)
	for _, iss := range eligible {
		m := index[iss.Brief]
		e := signoffEntry{
			Brief:  iss.Brief,
			Title:  m.title,
			Stream: m.stream,
			Status: m.status,
			Age:    "—",
		}
		if e.Title == "" {
			e.Title = iss.Title
		}
		if m.path != "" {
			e.Evidence = evidenceLink(root, m.path)
		}
		if agesKnown {
			if ts, ok := entered[iss.Brief]; ok && !ts.IsZero() {
				e.EnteredAt = ts
				e.Age = renderAge(now.Sub(ts))
			}
		}
		d.Entries = append(d.Entries, e)
	}

	// Oldest first — the whole product. Briefs with NO recorded arrival sort
	// LAST, in brief-id order: a zero timestamp would otherwise sort them oldest
	// and invent an age nobody recorded.
	sort.SliceStable(d.Entries, func(i, j int) bool {
		a, b := d.Entries[i], d.Entries[j]
		aKnown, bKnown := !a.EnteredAt.IsZero(), !b.EnteredAt.IsZero()
		if aKnown != bKnown {
			return aKnown
		}
		if aKnown && !a.EnteredAt.Equal(b.EnteredAt) {
			return a.EnteredAt.Before(b.EnteredAt)
		}
		return a.Brief < b.Brief
	})
	return d
}

// couldNotCheckSignoffDigest builds the third state: the digest could not read
// its inputs, so it reports that and nothing else. reason is carried verbatim
// into the body — a could-not-check with no reason is a dead end for whoever has
// to fix it.
func couldNotCheckSignoffDigest(reason string, now time.Time) signoffDigest {
	return signoffDigest{
		Date:   now.Format("2006-01-02"),
		State:  signoffCouldNotCheck,
		Reason: reason,
	}
}

// renderSignoffDigest renders the digest as a markdown issue body. The rendered
// ORDER matches d.Entries — a correctly sorted struct rendered in map order
// would still bury the oldest item, which is the failure this brief closes.
func renderSignoffDigest(d signoffDigest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", signoffDigestMarker)
	fmt.Fprintf(&b, "## Human sign-off queue — %s\n\n", d.Date)

	switch d.State {
	case signoffCouldNotCheck:
		fmt.Fprintf(&b, "**%s.** This digest could not read its inputs, so it is reporting **nothing** about the queue.\n\n", signoffCouldNotCheck)
		fmt.Fprintf(&b, "> %s\n\n", d.Reason)
		b.WriteString("An unreadable input and an empty queue are different facts. Treat this as UNKNOWN — briefs may well be waiting. Fix the read and re-run before drawing any conclusion from it.\n")
		return b.String()

	case signoffClear:
		b.WriteString("**Nothing is awaiting a human sign-off.** The brief set was read successfully and no brief is at the gate.\n\n")
		b.WriteString("_Membership is computed live from the same eligibility rule the per-brief cards use — there is no second list to fall out of date._\n")
		return b.String()
	}

	fmt.Fprintf(&b, "**%d brief(s) awaiting a human sign-off.**\n\n", len(d.Entries))
	switch {
	case !d.AgesKnown:
		b.WriteString("> **Ordering degraded: age could-not-check.** The status historian could not be read, so this digest **cannot claim oldest-first** — the rows below are in brief-id order and every age reads `—`. The LIST is still trustworthy; the ORDER is not.\n\n")
	case !anyKnownAge(d.Entries):
		// The historian WAS read; it simply records nothing about any brief
		// below. Distinct from could-not-check and must not borrow its words —
		// the read succeeded, and saying otherwise would send someone to debug a
		// failure that did not happen. But the ordering claim is just as empty,
		// so it is withdrawn all the same.
		b.WriteString("> **Ordering degraded: no age on record.** The status historian was read successfully but holds no transition for any brief below, so this digest **cannot claim oldest-first** — the rows are in brief-id order. The LIST is still trustworthy; the ORDER is not.\n\n")
	default:
		b.WriteString("Oldest first — the top row has waited longest.\n\n")
	}

	b.WriteString("| # | Brief | Age at gate | Status | Title | Evidence |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for i, e := range d.Entries {
		ev := "—"
		if e.Evidence != "" {
			ev = "[Evidence](" + e.Evidence + ")"
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s |\n",
			i+1, e.Brief, e.Age, dashIfEmpty(e.Status), dashIfEmpty(e.Title), ev)
	}

	b.WriteString("\n_`—` in Age means the historian has no recorded transition into the brief's current status (a brief older than the log): the age is UNKNOWN, not zero — those rows sort last rather than claiming to be oldest._\n\n")
	b.WriteString("_Surfacing only. This digest never closes, flips, or nudges anything; only a human's close advances a brief. Membership is the per-brief sign-off surface's own eligibility rule, called live — not a second copy of it._\n")
	return b.String()
}

// anyKnownAge reports whether at least one entry carries a recorded arrival
// time. With none, "oldest first" is a claim about an ordering nobody measured.
func anyKnownAge(entries []signoffEntry) bool {
	for _, e := range entries {
		if !e.EnteredAt.IsZero() {
			return true
		}
	}
	return false
}

// dashIfEmpty renders "" as an em dash so a table cell is never blank.
func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// oldestHumanGateAges computes the per-stream age-at-gate metric: for each
// stream, how long its LONGEST-waiting gate:human brief has sat in its current
// awaiting status. Oldest stream first; a stream whose gate briefs have no
// historian record sorts last with an em-dash age (unknown, never zero). Streams
// with nothing at the human gate are absent entirely.
//
// Render-only — never a Next-up or gate-score input. Until this existed, only
// COUNTS were surfaced at the human gate, never AGES, so a brief could age at
// the gate for a week without any board number moving.
func oldestHumanGateAges(streams []*Stream, entered map[string]time.Time, now time.Time) []streamGateAge {
	type row struct {
		streamGateAge
		at    time.Time
		known bool
	}
	var rows []row
	for _, s := range streams {
		var oldest time.Time
		oldestNum := ""
		atGate := false
		for _, br := range s.Briefs {
			if br.Gate != "human" || !awaitingGateStatuses[br.Status] {
				continue
			}
			// The stream IS at the gate even if the historian cannot date it —
			// recorded before the age lookup so an undated brief still puts its
			// stream on the board wearing a "—", instead of vanishing as if
			// nothing were waiting.
			atGate = true
			ts, ok := entered[s.Name+"/"+br.Num]
			if !ok || ts.IsZero() {
				continue
			}
			if oldestNum == "" || ts.Before(oldest) {
				oldest, oldestNum = ts, br.Num
			}
		}
		if !atGate {
			continue
		}
		r := row{streamGateAge: streamGateAge{Stream: s.Name, Age: "—"}}
		if oldestNum != "" {
			r.Brief = oldestNum
			r.Age = renderAge(now.Sub(oldest))
			r.at, r.known = oldest, true
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.known != b.known {
			return a.known
		}
		if a.known && !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		return a.Stream < b.Stream
	})
	out := make([]streamGateAge, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.streamGateAge)
	}
	return out
}

// runSignoffDigest is the --signoff-digest entrypoint: print the digest body to
// stdout. Returns a process exit code. Never reads or writes STATUS.md.
//
// EXIT DISCIPLINE, and the reason this is a function rather than a bare print: a
// root whose brief set cannot be read exits NON-ZERO with a could-not-check
// body. It never exits 0 with an empty digest. A daily cron that treats an
// unreadable repo as "nothing waiting" is worse than no cron — it manufactures
// an all-clear nobody established. An unreadable HISTORIAN is a different,
// lesser failure: the queue is still readable, so the digest still prints
// (exit 0) with its oldest-first claim explicitly withdrawn.
func runSignoffDigest(root string) int {
	now := nowFunc()
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Print(renderSignoffDigest(couldNotCheckSignoffDigest(err.Error(), now)))
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}

	// The historian read is DELIBERATELY not just `LoadHistory(…) == nil`.
	// LoadHistory returns (nil, nil) for a MISSING log — best-effort by design,
	// so the board still renders — which collapses "the log records nothing" and
	// "there is no log" into the same empty map. Consuming that as "ages known"
	// is exactly the empty-map-means-two-things bug (an empty claim map read as
	// both "nothing claimed" and "remote unreadable"). Stat first, so an ABSENT
	// historian degrades the ordering claim instead of silently backing it.
	histPath := filepath.Join(root, filepath.FromSlash(historyRelPath))
	var entered map[string]time.Time
	agesKnown := false
	if _, serr := os.Stat(histPath); serr == nil {
		if hist, herr := LoadHistory(histPath); herr == nil {
			cur := make(map[string]string)
			for _, s := range streams {
				for _, br := range s.Briefs {
					if awaitingGateStatuses[br.Status] {
						cur[s.Name+"/"+br.Num] = br.Status
					}
				}
			}
			entered = awaitingEnteredAt(hist, cur)
			agesKnown = true
		}
	}

	fmt.Print(renderSignoffDigest(buildSignoffDigest(root, streams, entered, agesKnown, now)))
	return 0
}
