package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// verificationDebtThreshold is the default depth at which the Awaiting-
// verification/review queue triggers a NOTICE (non-fatal alarm). The NOTICE
// also fires when the queue depth exceeds the total done count, whichever
// is lower — the ratio signals the queue is the constraint regardless of
// absolute size.
const verificationDebtThreshold = 10

var trackOrder = []string{"product", "platform", "ecosystem", ""}

func trackHeading(t string) string {
	switch t {
	case "product":
		return "Product"
	case "platform":
		return "Platform"
	case "ecosystem":
		return "Ecosystem"
	}
	return "Other"
}

func doneCount(s *Stream) int {
	n := 0
	for _, b := range s.Briefs {
		if b.Status == "done" {
			n++
		}
	}
	return n
}

// blockerSegment classifies an awaiting brief by who owns the blocker.
// The desk-actionable segment is the residual —
// the queue the desk can actually drain.
type blockerSegment int

const (
	segmentDeskActionable blockerSegment = iota
	segmentHumanGate
	segmentRework
	segmentPaused
	segmentEnvBlocked
)

func (s blockerSegment) heading() string {
	switch s {
	case segmentDeskActionable:
		return "Desk-actionable"
	case segmentHumanGate:
		return "Awaiting human gate"
	case segmentRework:
		return "Awaiting implementer rework"
	case segmentPaused:
		return "Paused stream"
	case segmentEnvBlocked:
		return "Env-blocked"
	}
	return ""
}

// classifyAwaiting returns the blocker-owner segment for an awaiting
// (implemented/verified) brief. Order matters — the first matching condition
// wins: a paused stream trumps everything (nobody is working it), then
// env-blocked (no agent can move it), then the Evidence verdict.
//
// The verdict arm reads the LAST verdict recorded, never "any FAIL ever seen"
// (see lastVerifyVerdict): Evidence accumulates, and a brief that failed, was
// reworked and passed is not awaiting rework. That also settles what used to
// be a branch-order question — a brief cannot be simultaneously in rework and
// through its human gate, because only one verdict is the current one. FAIL →
// the implementer owns it. PASS on a `gate: human` brief → human:<name> owns it. Every
// other row is the residual the desk can actually drain, which is what the
// headline counts.
func classifyAwaiting(s *Stream, br *Brief) blockerSegment {
	if s.Status == "paused" {
		return segmentPaused
	}
	if br.BlockedBy == "env" {
		return segmentEnvBlocked
	}
	switch lastVerifyVerdict(br.Evidence) {
	case verdictFail:
		return segmentRework
	case verdictPass:
		if br.Gate == "human" {
			return segmentHumanGate
		}
	}
	return segmentDeskActionable
}

// debtCounts computes verification-debt depth and composition for the
// Awaiting heading and the debt-alarm NOTICE. awaiting = implemented+verified;
// deskActionable = the subset the desk can actually drain (excludes paused,
// human-gated-with-VERIFY:PASS, rework, and env-blocked); done is the total
// done briefs across all streams (segmentation of
// the Awaiting board by blocker owner).
func debtCounts(streams []*Stream) (awaiting, deskActionable, implemented, verified, done int) {
	for _, s := range streams {
		for _, br := range s.Briefs {
			switch br.Status {
			case "implemented":
				implemented++
			case "verified":
				verified++
			case "done":
				done++
			}
		}
	}
	awaiting = implemented + verified
	// Compute desk-actionable: the residual after excluding every non-desk
	// segment. Must be computed separately (the classification loops over
	// the same streams + briefs but the five-class logic lives in one
	// function — debtCounts mirrors it by counting the residual).
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "implemented" && br.Status != "verified" {
				continue
			}
			if classifyAwaiting(s, br) == segmentDeskActionable {
				deskActionable++
			}
		}
	}
	return
}

// debtBreached reports whether the verification-debt alarm condition holds:
// the desk-actionable Awaiting queue exceeds the fixed threshold or the total
// done count. Factored out of debtNotice so the drain-before-instrument
// eligibility gate (nextup.go) and the mm/10 NOTICE read the SAME predicate and
// can never disagree about what "over threshold" means — a board that held a
// metric brief back while printing no debt NOTICE (or the reverse) would be
// unexplainable to the reader looking at it.
func debtBreached(streams []*Stream) bool {
	_, desk, _, _, done := debtCounts(streams)
	return desk > verificationDebtThreshold || desk > done
}

// debtNotice returns a non-empty NOTICE string when the desk-actionable
// Awaiting queue exceeds the threshold or the total done count — the
// queue the desk can actually move is the constraint and should be drained
// before dispatching new implementation work (retargeted at the
// desk-actionable slice).
func debtNotice(streams []*Stream) string {
	if !debtBreached(streams) {
		return ""
	}
	_, desk, _, _, done := debtCounts(streams)
	return fmt.Sprintf("verification debt: %d desk-actionable awaiting vs %d done — the queue is the constraint; drain before dispatching new implementation work", desk, done)
}

// segmentGroup is a sorted group of gate-score rows belonging to one blocker
// segment.
type segmentGroup struct {
	heading string
	gates   []GateScore
}

// buildSegments classifies each gate-score row and groups them by blocker
// owner. Segments are returned in fixed display order: desk-actionable first
// (the headline), then human-gate, rework, paused, env-blocked.
func buildSegments(gates []GateScore) []segmentGroup {
	var desk, human, rework, paused, env []GateScore
	for _, g := range gates {
		seg := classifyAwaiting(g.Stream, &g.Brief)
		switch seg {
		case segmentDeskActionable:
			desk = append(desk, g)
		case segmentHumanGate:
			human = append(human, g)
		case segmentRework:
			rework = append(rework, g)
		case segmentPaused:
			paused = append(paused, g)
		case segmentEnvBlocked:
			env = append(env, g)
		}
	}
	groups := []segmentGroup{
		{heading: segmentDeskActionable.heading(), gates: desk},
		{heading: segmentHumanGate.heading(), gates: human},
		{heading: segmentRework.heading(), gates: rework},
		{heading: segmentPaused.heading(), gates: paused},
		{heading: segmentEnvBlocked.heading(), gates: env},
	}
	// Remove empty groups in-place.
	n := 0
	for _, g := range groups {
		if len(g.gates) > 0 {
			groups[n] = g
			n++
		}
	}
	return groups[:n]
}

// emit renders STATUS.md. ages maps "<stream>/<NN>" → rendered awaiting age;
// nil or missing ids render "—". gateAges is the per-stream
// oldest-age-at-the-human-gate metric (methodology-metrics/38), already ordered
// oldest-stream-first; nil renders the section's zero state. intake carries the
// untriaged-intake alarm counts for the intake-debt board line;
// a zero-value IntakeAlarmResult (no entries parsed) renders the zero state.
// briefTouch holds per-brief last-transition times from the historian for
// gate-score staleness; nil means fall back to stream LastTouch. repo is the
// owning repo declared by this root's `repo:` frontmatter
// — rendered as a banner so a multi-repo reader can tell two boards apart at a
// glance; "" (nobody declared one) renders nothing, keeping single-repo output
// byte-identical to the pre-multi-root generator.
func emit(streams []*Stream, findings []Finding, nu NextUp, ages map[string]string, gateAges []streamGateAge, intake IntakeAlarmResult, briefTouch map[string]time.Time, repo string) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	w("<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.")
	// The regenerate line is DERIVED from the declared channel set
	// (statusgen/channels.go), not written here. It is the header-hint form,
	// not the situation-aware one: this string is persisted into STATUS.md and
	// byte-compared by --check, so a build-dependent value would report drift
	// between an installed binary and a `go run` CI on a file neither touched.
	w("     Regenerate: %s -->", regenerateHeaderHint)
	w("")
	w("# Project Status")
	w("")
	if repo != "" {
		w("_Repo: `%s` — this board covers the streams in this repo only; sibling repos have their own._", repo)
		w("")
	}
	w("## Roll-up")
	for _, track := range trackOrder {
		var group []*Stream
		for _, s := range streams {
			if s.Track == track {
				group = append(group, s)
			}
		}
		if len(group) == 0 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].Name < group[j].Name })
		w("")
		w("### %s", trackHeading(track))
		w("")
		w("| Stream | Priority | Status | Briefs done | Last touched | Notes |")
		w("|---|---|---|---|---|---|")
		for _, s := range group {
			note := ""
			if s.External != "" {
				note = "→ " + s.External
			}
			if s.Tiering != nil && strings.TrimSpace(*s.Tiering) != "" {
				if note != "" {
					note += " · "
				}
				note += strings.TrimSpace(*s.Tiering)
			}
			touched := ""
			if !s.LastTouch.IsZero() {
				touched = s.LastTouch.Format("2006-01-02")
			}
			w("| [%s](docs/streams/%s/README.md) | %s | %s | %d/%d | %s | %s |",
				s.Name, s.Name, s.Priority, s.Status, doneCount(s), len(s.Briefs), touched, note)
		}
	}

	w("")
	w("## Next up")
	w("")
	// Degraded claim filtering leads the section. It is
	// FIRST, before the counts, because every number below it is a superset when
	// it is present — a reader who takes the eligible count at face value is
	// exactly the failure this banner exists to stop.
	if b := nu.Claims.Banner(); b != "" {
		w("%s", b)
		w("")
	}
	// Could-not-check on a serialized stream. Distinct from the banner above:
	// that one says the whole board is a superset, this one names the streams
	// being WITHHELD because of it. Reported, never silently downgraded to
	// "offer the declared budget anyway".
	if len(nu.SerializedUnknown) > 0 {
		w("> **COULD NOT CHECK — serialized streams held back.** %s declare `max-concurrent`, "+
			"but claim filtering did not run, so what is already in flight is unknowable and the declaration "+
			"cannot be honoured. These streams offer **nothing** on this board rather than risk the parallel "+
			"dispatch they exist to forbid. Regenerate with a reachable `origin` to restore them.",
			strings.Join(nu.SerializedUnknown, ", "))
		w("")
	}
	// Drain-before-instrument. A brief held here has not gone anywhere — it is
	// waiting on a queue that a person can drain — so the board says which brief,
	// which queue, and what clears it. Silence would make the brief look retired.
	if len(nu.MeasuresGated) > 0 {
		w("> **DRAIN BEFORE INSTRUMENT — %d brief(s) held back:** %s. Each declares `measures:` on a queue "+
			"that is currently over its own alarm threshold. Instrumentation is not service: the fix for a "+
			"breached queue is to drain it, not to build another metric about it. They return to this board "+
			"by themselves once the queue is back under threshold — nothing needs re-authoring.",
			len(nu.MeasuresGated), strings.Join(nu.MeasuresGated, ", "))
		w("")
	}
	// Could-not-check on a measured queue. Distinct from the line above: that one
	// says the queue IS breached, this one says nobody could find out. Held back
	// (fail closed) and named — never silently dropped, and never quietly allowed.
	if len(nu.MeasuresUnknown) > 0 {
		w("> **COULD NOT CHECK — %d instrumentation brief(s) held back:** %s. Each declares `measures:` on a "+
			"queue whose depth this board cannot read, so whether the drain-before-instrument gate should "+
			"fire is unknowable. They offer **nothing** here rather than re-permit the dispatch the gate "+
			"exists to stop. `--lint` names the file and the bad queue name; fix the name, or wire the queue.",
			len(nu.MeasuresUnknown), strings.Join(nu.MeasuresUnknown, ", "))
		w("")
	}
	// Overflow is an alarm (SCADA / EEMUA-191): when the eligible backlog exceeds
	// what the caps show, say so explicitly — never silently truncate. The
	// held-back count names WHICH cap fired: it used to blame the span-of-control
	// cap unconditionally, even when the per-stream caps were the whole reason.
	if nu.Overflow() {
		unfiltered := ""
		if !nu.Claims.Known {
			unfiltered = ", UNFILTERED — see the degraded notice above"
		}
		w("_Next-up: %d of %d eligible%s — %d held back (%s). Overflow is itself an alarm (EEMUA-191): clear WIP before pulling more._",
			len(nu.Picks), nu.Eligible, unfiltered, nu.HeldBack(), heldBackReason(nu))
		w("")
	}
	if len(nu.Picks) == 0 {
		w("_Nothing eligible — all active streams are blocked, stale-flagged, or done._")
	} else {
		w("| Stream | Brief | Wave | Score |")
		w("|---|---|---|---|")
		for _, p := range nu.Picks {
			marker := ""
			if p.Brief.ExecTier == "strong" {
				marker = " [exec:strong]"
			}
			w("| %s | %s — %s%s | %d | %d |", p.Stream.Name, p.Brief.Num, p.Brief.Title, marker, p.Brief.Wave, p.Score)
		}
	}

	awaiting, deskActionable, implemented, verified, _ := debtCounts(streams)
	gates := gateScores(streams, briefTouch)
	segments := buildSegments(gates)

	w("")
	w("## Intake queue")
	w("")
	w("%s", intakeBoardLine(intake))
	w("")
	w("## Awaiting verification / review (%d desk-actionable of %d total — %d at implemented, %d verified awaiting review)", deskActionable, awaiting, implemented, verified)
	w("")
	w("_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner: the desk-actionable headline counts only the queue the desk can actually drain._")
	w("")
	w("%s", unrunLegend)
	w("")

	if len(gates) == 0 {
		w("_None._")
	} else {
		for _, seg := range segments {
			w("")
			w("### %s (%d)", seg.heading, len(seg.gates))
			w("")
			w("| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |")
			w("|---|---|---|---|---|---|---|---|")
			for _, g := range seg.gates {
				s, br := g.Stream, &g.Brief
				v, r := br.Verified, br.Reviewed
				if v == "" {
					v = "—"
				}
				if r == "" {
					r = "—"
				}
				// Age in current awaiting status — from the
				// historian; "—" when unknown, never a guess. Render-only.
				age := ages[s.Name+"/"+br.Num]
				if age == "" {
					age = "—"
				}
				marker := ""
				if br.ExecTier == "strong" {
					marker = " [exec:strong]"
				}
				w("| %s | %s%s | %s | %d | %d | %s | %s | %s |", s.Name, br.Num, marker, qualityToken(s, br), g.Score, g.BlockedCount, age, v, r)
			}
		}
	}

	// Age at the human gate (methodology-metrics/38). The awaiting board above
	// surfaces per-row ages; this rolls them up per STREAM so the human gate's
	// queue is as visible as the model gates' — until now only COUNTS were
	// surfaced at the human gate, never AGES, and a brief could age at the gate
	// for a week without any board number moving.
	w("")
	w("## Age at the human gate")
	w("")
	w("_Per stream: how long the longest-waiting `gate: human` brief has sat in its CURRENT awaiting status (implemented/verified), from the historian (`.history.jsonl`). Oldest stream first. Render-only — never a Next-up or gate-score input. `—` means the historian has no recorded transition into that status (a brief older than the log, or a fresh checkout): the age is UNKNOWN, not zero._")
	w("")
	w("_Deliberately WIDER than `--signoff-digest`: this counts every `gate: human` brief sitting at implemented/verified, whereas the digest lists only those the per-brief sign-off surface has judged actionable (a recorded model verify pass behind them). A stream appearing here with no digest row is a brief waiting on its VERIFIER, not on the human — a different queue, and worth seeing separately._")
	w("")
	if len(gateAges) == 0 {
		w("_No brief is awaiting the human gate._")
	} else {
		w("| Stream | Oldest at gate | Brief |")
		w("|---|---|---|")
		for _, g := range gateAges {
			// An empty Brief means the stream IS at the gate but no listed brief
			// has a recorded arrival — render the em dash rather than a blank
			// cell, which reads as a rendering bug instead of a stated unknown.
			brief := g.Brief
			if brief == "" {
				brief = "—"
			}
			w("| %s | %s | %s |", g.Stream, g.Age, brief)
		}
	}

	w("")
	w("## Unresolved findings")
	w("")
	rows := 0
	for _, f := range findings {
		if f.Resolved {
			continue
		}
		if rows == 0 {
			w("| ID | Date | Title | Affects |")
			w("|---|---|---|---|")
		}
		rows++
		w("| %s | %s | %s | %s |", f.ID, f.Date, f.Title, strings.Join(f.Affects, ", "))
	}
	if rows == 0 {
		w("_None._")
	}

	w("")
	w("## Incomplete briefs")
	for _, s := range streams {
		var open []Brief
		for _, br := range s.Briefs {
			if br.Status != "done" {
				open = append(open, br)
			}
		}
		if len(open) == 0 {
			continue
		}
		w("")
		w("### %s (%d open)", s.Name, len(open))
		w("")
		for _, br := range open {
			stale := ""
			if br.StaleRef != "" {
				stale = fmt.Sprintf(" ⚠ %s", br.StaleRef)
			}
			w("- %s %s — %s (wave %d)%s", br.Num, br.Title, br.Status, br.Wave, stale)
		}
	}

	w("")
	w("## Done briefs")
	w("")
	w("_`done*` = unbacked (I-08 point quality): the row's Evidence section is empty and/or its Verified/Reviewed cells aren't dated+attributed per brief-16 — see `--lint` for the full list. Plain `done` is evidence-backed._")
	w("")
	w("%s", unrunLegend)
	for _, s := range streams {
		var doneBriefs []Brief
		for _, br := range s.Briefs {
			if br.Status == "done" {
				doneBriefs = append(doneBriefs, br)
			}
		}
		if len(doneBriefs) == 0 {
			continue
		}
		w("")
		w("### %s (%d done)", s.Name, len(doneBriefs))
		w("")
		for _, br := range doneBriefs {
			line := fmt.Sprintf("- %s %s — %s (wave %d)", br.Num, br.Title, qualityToken(s, &br), br.Wave)
			if _, reasons := rowIsBacked(s, &br); len(reasons) > 0 {
				line += " — unbacked: " + strings.Join(reasons, "; ")
			}
			w("%s", line)
		}
	}

	active, paused, done, total := 0, 0, 0, 0
	for _, s := range streams {
		switch s.Status {
		case "active":
			active++
		case "paused":
			paused++
		}
		done += doneCount(s)
		total += len(s.Briefs)
	}
	w("")
	w("## Totals")
	w("")
	w("**%d** streams (**%d** active, **%d** paused) · **%d/%d** briefs done · completed initiatives: see `docs/archive/`", len(streams), active, paused, done, total)
	return b.String()
}
