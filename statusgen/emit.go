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

// awaitPlacement is where one awaiting (implemented/verified) brief renders on
// the Awaiting board: a paused or parked stream's rows are UNBUCKETED (they
// never reach bucketAwaiting and count only in the headline total); every other
// row carries the owned queue bucketAwaiting chose, with its owner and next act.
type awaitPlacement struct {
	paused, parked bool
	bucket         awaitingBucket
	owner, nextAct string
}

// placeAwaiting reads the brief's inputs and buckets it (awaiting_bucket.go).
func placeAwaiting(s *Stream, br *Brief) awaitPlacement {
	if s.Status == "paused" {
		return awaitPlacement{paused: true}
	}
	if s.Status == streamStatusParked {
		return awaitPlacement{parked: true}
	}
	b, rows, ev, oc := awaitInputs(s, br)
	bucket, owner, next := bucketAwaiting(b, rows, ev, oc)
	return awaitPlacement{bucket: bucket, owner: owner, nextAct: next}
}

// awaitingTally counts every awaiting brief by its placement.
type awaitingTally struct {
	buckets        map[awaitingBucket]int
	paused, parked int
}

func tallyAwaiting(streams []*Stream) awaitingTally {
	t := awaitingTally{buckets: map[awaitingBucket]int{}}
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "implemented" && br.Status != "verified" {
				continue
			}
			p := placeAwaiting(s, br)
			switch {
			case p.paused:
				t.paused++
			case p.parked:
				t.parked++
			default:
				t.buckets[p.bucket]++
			}
		}
	}
	return t
}

// debtCounts computes verification-debt depth and composition for the
// Awaiting heading and the debt-alarm NOTICE. awaiting = implemented+verified;
// deskActionable = the desk-actionable bucket (the judgement queue the desk
// drains — never a row another owner moves) PLUS the could-not-check rows: a
// row whose inputs could not be read is not known to be someone else's, so it
// stays in the measure. Dropping it would let one unreadable file lower the
// count and switch the drain-before-instrument gate off, silently (fail open).
// Runner-pending rows leave the measure: the verify runner and CI drain them
// without the desk. done is the total done briefs across all streams.
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
	t := tallyAwaiting(streams)
	deskActionable = t.buckets[bucketDeskActionable] + t.buckets[bucketCouldNotCheck]
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
// Awaiting queue (could-not-check rows included, see debtCounts) exceeds the
// threshold or the total done count — the
// queue the desk can actually move is the constraint and should be drained
// before dispatching new implementation work (retargeted at the
// desk-actionable slice).
func debtNotice(streams []*Stream) string {
	if !debtBreached(streams) {
		return ""
	}
	_, desk, _, _, done := debtCounts(streams)
	counted := fmt.Sprintf("%d desk-actionable", desk)
	if n := tallyAwaiting(streams).buckets[bucketCouldNotCheck]; n > 0 {
		counted = fmt.Sprintf("%d desk-actionable (%d of them could-not-check)", desk, n)
	}
	return fmt.Sprintf("verification debt: %s awaiting vs %d done — the queue is the constraint; drain before dispatching new implementation work", counted, done)
}

// placedGate is one gate-score row with its placement.
type placedGate struct {
	GateScore
	place awaitPlacement
}

// segmentGroup is a sorted group of gate-score rows rendered under one heading.
type segmentGroup struct {
	heading string
	gates   []placedGate
	// always renders the heading (with `_None._`) even when empty.
	always bool
}

// buildSegments places each gate-score row and groups them. Fixed display
// order: the four owned queues and the desk's judgement queue (human gate →
// implementer rework → environment-blocked → runner-pending → desk-actionable),
// each ALWAYS rendered so a reader can tell an empty queue from a missing one;
// then could-not-check, paused and parked, rendered only when non-empty.
func buildSegments(gates []GateScore) []segmentGroup {
	byBucket := map[awaitingBucket][]placedGate{}
	var paused, parked []placedGate
	for _, g := range gates {
		p := placeAwaiting(g.Stream, &g.Brief)
		pg := placedGate{GateScore: g, place: p}
		switch {
		case p.paused:
			paused = append(paused, pg)
		case p.parked:
			parked = append(parked, pg)
		default:
			byBucket[p.bucket] = append(byBucket[p.bucket], pg)
		}
	}
	var groups []segmentGroup
	for _, b := range bucketRenderOrder {
		groups = append(groups, segmentGroup{heading: b.heading(), gates: byBucket[b], always: true})
	}
	for _, g := range []segmentGroup{
		{heading: bucketCouldNotCheck.heading(), gates: byBucket[bucketCouldNotCheck]},
		{heading: "Paused stream", gates: paused},
		{heading: "Parked stream", gates: parked},
	} {
		if len(g.gates) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}

// awaitingHeadline is the roll-up: one count per owner, then the total.
func awaitingHeadline(streams []*Stream) string {
	awaiting, _, _, _, _ := debtCounts(streams)
	t := tallyAwaiting(streams)
	line := fmt.Sprintf("## Awaiting verification / review (%d for the desk · %d for the driver · %d for workers · %d for an operator · %d runner-pending — of %d total",
		t.buckets[bucketDeskActionable], t.buckets[bucketHumanGate], t.buckets[bucketRework],
		t.buckets[bucketEnvBlocked], t.buckets[bucketRunnerPending], awaiting)
	if n := t.buckets[bucketCouldNotCheck]; n > 0 {
		line += fmt.Sprintf("; %d could-not-check", n)
	}
	return line + ")"
}

// tableCell renders free text safely inside a Markdown table cell.
func tableCell(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\\|", "|")
	return strings.ReplaceAll(s, "|", "\\|")
}

// nextActCell renders a next act; an environment-blocked command renders as a
// code span so it reads verbatim.
func nextActCell(p awaitPlacement) string {
	if p.paused || p.parked || p.nextAct == "" {
		return "—"
	}
	if p.bucket == bucketEnvBlocked && p.nextAct != nextActNoEnvCmd && !strings.HasPrefix(p.nextAct, nextActEnvBlocker) && !strings.Contains(p.nextAct, "`") {
		return "`" + tableCell(p.nextAct) + "`"
	}
	return tableCell(p.nextAct)
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
			// Parked streams are shelved — they render under their own `## Parked`
			// heading below, not in the active track roll-up (attention-budget/04).
			if s.Status == streamStatusParked {
				continue
			}
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

	// Parked streams (attention-budget/04): shelved out of the active roll-up and
	// out of Next-up, but their briefs are kept and listed here so a parked stream
	// is visible rather than vanished. Re-activates by a README `status:` flip
	// (itself subject to the cap). Rendered ONLY when a parked stream exists, so a
	// tree with none is byte-identical to the pre-parked board.
	var parkedStreams []*Stream
	for _, s := range streams {
		if s.Status == streamStatusParked {
			parkedStreams = append(parkedStreams, s)
		}
	}
	if len(parkedStreams) > 0 {
		sort.Slice(parkedStreams, func(i, j int) bool { return parkedStreams[i].Name < parkedStreams[j].Name })
		w("")
		w("## Parked")
		w("")
		w("_Shelved streams: excluded from Next-up and every dispatch view, briefs kept. Re-activate by flipping the README `status:` back to `active` (subject to the active-stream cap)._")
		w("")
		w("| Stream | Priority | Briefs | Last touched |")
		w("|---|---|---|---|")
		for _, s := range parkedStreams {
			touched := ""
			if !s.LastTouch.IsZero() {
				touched = s.LastTouch.Format("2006-01-02")
			}
			w("| [%s](docs/streams/%s/README.md) | %s | %d/%d | %s |",
				s.Name, s.Name, s.Priority, doneCount(s), len(s.Briefs), touched)
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
	// A claim set that was read but never DECAYED is the other way these rows
	// mislead (#1111) — not a superset this time but a subset, with real backlog
	// held behind merged/closed corpses. Same placement, same reason: a reader who
	// takes the rows at face value is exactly the failure the banner stops.
	if b := nu.Claims.DecayBanner(); b != "" {
		w("%s", b)
		w("")
	}
	// Drive banners (methodology-metrics/45). The fail-neutral "DRIVE NOT APPLIED"
	// banner leads (a rejected manifest changed nothing, and the reader must know
	// the board is un-steered), then the ACTIVE DRIVE honesty banner, then any
	// anti-Goodhart coverage NOTICE. All absent when no drive is active, keeping a
	// no-manifest board byte-identical.
	if nu.DriveNotApplied != "" {
		w("%s", nu.DriveNotApplied)
		w("")
	}
	if nu.DriveBanner != "" {
		w("%s", nu.DriveBanner)
		w("")
	}
	for _, n := range nu.DriveCoverageNotices {
		w("> **DRIVE COVERAGE — %s**", n)
		w("")
	}
	// The critical tier's main-red arm could not check (drive active, no
	// --main-health input). Present only while a drive is active.
	if nu.MainRedUnknown != "" {
		w("> **COULD NOT CHECK — %s**", nu.MainRedUnknown)
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
	// Homed in another repo (statusgen/12). A brief here is NOT held pending
	// anything on this board — its deliverable simply lives in another repo, so it
	// is not a dispatch candidate here. The board NAMES each one with its target
	// so a cross-repo dispatcher reads the right repo instead of burning a slot to
	// discover the mis-route; silence would let the tracking row keep reading as a
	// fresh local todo.
	if len(nu.HomedElsewhere) > 0 {
		ids := make([]string, 0, len(nu.HomedElsewhere))
		for id := range nu.HomedElsewhere {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		frags := make([]string, 0, len(ids))
		for _, id := range ids {
			frags = append(frags, fmt.Sprintf("%s → %s", id, nu.HomedElsewhere[id]))
		}
		w("> **HOMED IN ANOTHER REPO — %d brief(s) not dispatchable here:** %s. Each declares `homed-in:` — "+
			"its deliverable lives in the named repo, not this one. The tracking row stays so the work is not "+
			"lost, but it is held out of Next-up so no slot is spent re-discovering the move. Dispatch it "+
			"against the repo named after the arrow.",
			len(nu.HomedElsewhere), strings.Join(frags, ", "))
		w("")
	}
	// Merged in a sibling repo (siblingmerge.go). A row
	// here is NOT proven delivered — the finding is a PROMPT to read the
	// merged change Task by Task, never proof — but a change naming it
	// already merged in the named sibling and nobody has recorded what it
	// covered, so it is held out of Next-up rather than let a slot rediscover
	// work that (at least partly) already landed. The board NAMES each one
	// with its sibling so a reader checks the right repo instead of guessing.
	if len(nu.MergedElsewhere) > 0 {
		ids := make([]string, 0, len(nu.MergedElsewhere))
		for id := range nu.MergedElsewhere {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		frags := make([]string, 0, len(ids))
		for _, id := range ids {
			frags = append(frags, fmt.Sprintf("%s → %s", id, nu.MergedElsewhere[id]))
		}
		w("> **Merged in a sibling repo — check before dispatch (%d):** %s. Each has a matching "+
			"sibling-merge-unreconciled finding: a change naming it already merged in the named sibling "+
			"repo. This is a PROMPT to read the merged change Task by Task, never proof of delivery — "+
			"reconcile the row, or record/complete a `delivery:` claim, to release it back onto this board.",
			len(nu.MergedElsewhere), strings.Join(frags, ", "))
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
	// Per-stream held-back decomposition. A bare "N held back" reads as a drained
	// board; naming WHICH streams are at their dispatch cap — and the top holder —
	// tells an operator (or a drain loop) that the backlog is capped, not empty, so
	// they clear a claiming branch/PR on the right stream rather than concluding
	// there is nothing to do. Rendered whenever a per-stream cap actually held work
	// back, independent of the span-overflow line above.
	if nu.HeldByStreamCap > 0 && len(nu.HeldByStreamDetail) > 0 {
		frags, top, total := heldByStreamTop(nu.HeldByStreamDetail, 5)
		more := ""
		if n := len(nu.HeldByStreamDetail) - len(frags); n > 0 {
			more = fmt.Sprintf(", +%d more stream(s)", n)
		}
		w("_Held by per-stream caps: %d brief(s) across %d stream(s) — top: %s. By stream: %s%s. "+
			"A stream at its dispatch cap (perStreamCap %d, a declared max-concurrent, or in-flight claims) offers "+
			"nothing more until a claiming branch or PR clears — this backlog is capped here, not drained._",
			total, len(nu.HeldByStreamDetail), top, strings.Join(frags, ", "), more, perStreamCap)
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
			if p.Brief.HomedIn != "" {
				marker += " [homed→" + p.Brief.HomedIn + "]"
			}
			// Score is rendered DECOMPOSED when a drive steer is present: `base +
			// term (drive:<slug>)`, never a merged number (brief-44 honesty rule).
			// With no drive term the format is the bare base score — byte-identical
			// to the pre-drives column.
			score := fmt.Sprintf("%d", p.Score)
			if p.DriveTerm > 0 {
				score = fmt.Sprintf("%d + %d (drive:%s)", p.Score, p.DriveTerm, p.DriveSlug)
			}
			w("| %s | %s — %s%s | %d | %s |", p.Stream.Name, p.Brief.Num, p.Brief.Title, marker, p.Brief.Wave, score)
		}
	}

	// Drive dashboard (methodology-metrics phase 4, brief-48). Rendered ONLY when
	// a drive is active (absent ⇒ inert — a no-manifest board stays byte-identical
	// to the pre-drives baseline). The operator slice leads, per brief-44's
	// Dashboard order. activeDriveStatuses/activeDriveHeartbeat are set by run()
	// right before emit — the section is a pure render of phase 2's
	// frontier/state, plus the git-derived last-regen heartbeat.
	if len(activeDriveStatuses) > 0 {
		w("%s", driveSections(activeDriveStatuses, activeDriveHeartbeat))
	}

	gates := gateScores(streams, briefTouch)
	segments := buildSegments(gates)

	w("")
	w("## Intake queue")
	w("")
	w("%s", intakeBoardLine(intake))
	w("")
	w("%s", awaitingHeadline(streams))
	w("")
	w("_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board bucketed by owner (docs/board.md): four owned queues — the driver's human gate, workers' implementer rework, an operator's environment-blocked rows, and runner-pending rows CI or the verify runner moves — then the desk's judgement queue. Each row names its owner and its next act; a row whose inputs cannot be read is could-not-check, never a bucket. Paused and parked streams are not bucketed and count only in the total._")
	w("")
	w("%s", unrunLegend)
	w("")

	for i, seg := range segments {
		if i > 0 {
			w("")
		}
		w("### %s (%d)", seg.heading, len(seg.gates))
		w("")
		if len(seg.gates) == 0 {
			w("_None._")
			continue
		}
		w("| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |")
		w("|---|---|---|---|---|---|---|---|---|---|")
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
			if br.HomedIn != "" {
				marker += " [homed→" + br.HomedIn + "]"
			}
			owner := g.place.owner
			if owner == "" {
				owner = "—"
			}
			// Reviewed stays the LAST column: consumers read it as $(NF-1).
			w("| %s | %s%s | %s | %d | %d | %s | %s | %s | %s | %s |", s.Name, br.Num, marker, qualityToken(s, br), g.Score, g.BlockedCount, age, owner, nextActCell(g.place), v, r)
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

	active, paused, parked, done, total := 0, 0, 0, 0, 0
	for _, s := range streams {
		switch s.Status {
		case "active":
			active++
		case "paused":
			paused++
		case streamStatusParked:
			parked++
		}
		done += doneCount(s)
		total += len(s.Briefs)
	}
	w("")
	w("## Totals")
	w("")
	// The parked clause is appended ONLY when a parked stream exists, so a tree
	// with none renders the Totals line byte-identically to the pre-parked board.
	parkedClause := ""
	if parked > 0 {
		parkedClause = fmt.Sprintf(", **%d** parked", parked)
	}
	w("**%d** streams (**%d** active, **%d** paused%s) · **%d/%d** briefs done · completed initiatives: see `docs/archive/`", len(streams), active, paused, parkedClause, done, total)
	return b.String()
}
