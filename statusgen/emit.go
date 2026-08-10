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

// blockerSegment classifies an awaiting brief by who owns the blocker
// (methodology-metrics/34). The desk-actionable segment is the residual —
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
// done briefs across all streams (methodology-metrics/34: segmentation of
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

// debtNotice returns a non-empty NOTICE string when the desk-actionable
// Awaiting queue exceeds the threshold or the total done count — the
// queue the desk can actually move is the constraint and should be drained
// before dispatching new implementation work (methodology-metrics/10 +
// methodology-metrics/34: retargeted at the desk-actionable slice).
func debtNotice(streams []*Stream) string {
	_, desk, _, _, done := debtCounts(streams)
	if desk > verificationDebtThreshold || desk > done {
		return fmt.Sprintf("verification debt: %d desk-actionable awaiting vs %d done — the queue is the constraint; drain before dispatching new implementation work (methodology-metrics/10)", desk, done)
	}
	return ""
}

// segmentGroup is a sorted group of gate-score rows belonging to one blocker
// segment (methodology-metrics/34).
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

// emit renders STATUS.md. ages maps "<stream>/<NN>" → rendered awaiting age
// (methodology-metrics/17); nil or missing ids render "—". intake carries the
// untriaged-intake alarm counts for the intake-debt board line (issue-loop/07);
// a zero-value IntakeAlarmResult (no entries parsed) renders the zero state.
// briefTouch holds per-brief last-transition times from the historian for
// gate-score staleness; nil means fall back to stream LastTouch. repo is the
// owning repo declared by this root's `repo:` frontmatter (assay-selfcontain/01)
// — rendered as a banner so a multi-repo reader can tell two boards apart at a
// glance; "" (nobody declared one) renders nothing, keeping single-repo output
// byte-identical to the pre-multi-root generator.
func emit(streams []*Stream, findings []Finding, nu NextUp, ages map[string]string, intake IntakeAlarmResult, briefTouch map[string]time.Time, repo string) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	w("<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.")
	w("     Regenerate: go run ./tools/statusgen -->")
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
	// Degraded claim filtering leads the section (assay-toolkit#305). It is
	// FIRST, before the counts, because every number below it is a superset when
	// it is present — a reader who takes the eligible count at face value is
	// exactly the failure this banner exists to stop.
	if b := nu.Claims.Banner(); b != "" {
		w("%s", b)
		w("")
	}
	// Overflow is an alarm (SCADA / EEMUA-191): when the eligible backlog exceeds
	// what the span-of-control cap shows, say so explicitly — never silently
	// truncate (methodology-metrics/06).
	if nu.Overflow() {
		unfiltered := ""
		if !nu.Claims.Known {
			unfiltered = ", UNFILTERED — see the degraded notice above"
		}
		w("_Next-up: %d of %d eligible%s — %d held back (span-of-control cap %d). Overflow is itself an alarm (EEMUA-191): clear WIP before pulling more._",
			len(nu.Picks), nu.Eligible, unfiltered, nu.HeldBack(), nu.Span)
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
	w("_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner (methodology-metrics/34): the desk-actionable headline counts only the queue the desk can actually drain._")
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
				// Age in current awaiting status (mm/17, #282) — from the
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
