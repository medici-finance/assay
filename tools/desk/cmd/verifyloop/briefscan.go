package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// briefRow is one parsed stream-README table row plus its resolved brief file.
type briefRow struct {
	Stream        string
	Num           string
	Status        string
	Verified      string
	Reviewed      string
	BriefPath     string // repo-relative
	fm            briefFrontmatter
	evidenceEmpty bool
	// couldNotCheck is non-empty when the row's brief file could not be resolved or read
	// (#1309 item 5): the gate and every risk answer are then UNKNOWN, not "model / all no",
	// so the row is bucketed could-not-check and never dispatched.
	couldNotCheck string
	// verifyRows are the brief's `## Verify` table row numbers (example-stream/16). They let a
	// wake receipt that holds only SOME rows be compared against the whole set: rows not held by
	// an unchanged receipt are still runnable, so the brief dispatches for them while the held
	// rows stay held. Empty when the Verify table is absent or unparseable.
	verifyRows []int
}

// briefFrontmatter is the subset of brief-v1 frontmatter the verify adapter needs. It is
// parsed with a small field extractor rather than a YAML dep so the desk module stays
// self-contained (the deliberate "desk mirrors statusgen, does not import it" pattern —
// statusgen is package main and unimportable anyway).
type briefFrontmatter struct {
	Gate        string
	Risk        loopengine.RiskFlags
	Effort      string
	ExecTier    string
	Implementer string // optional `implementer:` field; empty disables the author!=runner guard for this item

	// The three queue-truthfulness markers (see queueclass.go). All optional; an absent
	// marker is the zero value and leaves the brief a normal DISPATCH candidate.
	//
	// BlockedUntil is the `blocked-until:` frontmatter marker: a condition or date whose
	// presence DEFERS the brief out of the dispatchable list until it can be met. Its whole
	// job is a longitudinal brief whose Verify exit criteria are a calendar/accrual window
	// that has not accrued — re-verifying it every run only reproduces the identical
	// "not accrued" result and burns a dispatchable slot.
	BlockedUntil string
	// VerifyLane is the `verify-lane:` marker naming the substrate a brief's Verify rows run
	// against when it is not this repo's offline tree (a live cluster / online / live session).
	// An online-lane value buckets the brief as awaiting-online-lane — an offline verifier run
	// cannot produce its verdict.
	VerifyLane string
	// InRepair is the `in-repair:` marker: an in-flight table-repair pipeline already owns this
	// brief's Verify table (a stale-artifact re-baseline). Its value is the pipeline reference,
	// carried into the "why it waits" note. Presence buckets the brief as in-repair.
	InRepair string

	// DeferredRows is NOT a frontmatter field: it is the per-ROW derivation result (#1309 item
	// 4) for a brief that stays dispatchable — the Verify rows whose Command cell names an
	// online lane or a longitudinal window, listed as "<num>: <why>; …" so the dispatched
	// verifier records exactly those rows as explicitly unrun and runs the rest. Rows are
	// deferred; the brief is deferred only when EVERY row is.
	DeferredRows string
}

// scanAwaiting reads every stream README under <root>/docs/streams/*/README.md, applies the
// statusgen Awaiting filter (status ∈ {implemented, verified}), resolves each row to its
// brief file + frontmatter + Evidence emptiness, and returns typed Items ORDERED tier-1
// (implemented with an EMPTY Evidence section — the real verify work) before tier-2
// (verified free-closes and Evidence-present-but-unpromoted rows), oldest-first within class.
//
// This is the deterministic board read; there is NO code path that produces a verify verdict
// without going through the engine's Dispatch — the inline-verify path is unrepresentable.
func scanAwaiting(root, targetSHA string) ([]loopengine.Item, error) {
	return scanAwaitingIn(deskkit.RootConfig{Path: root}, targetSHA, nil, nil, time.Time{})
}

// scanAwaitingRoots is the MULTI-ROOT board read: one scanAwaitingIn per configured root, in
// the order given (deskkit.ConfiguredRoots sorts by repo), then ONE global tier ordering so a
// tier-1 brief on the last root is never crowded out by tier-2 free-closes on the first. Within
// a class the per-root order (root order, then stream, then brief-num) is preserved — the sort
// is stable. A root whose streams cannot be read is an error naming the root, never a silent
// omission: the whole point of the multi-root plan is that a repo's queue cannot vanish.
func scanAwaitingRoots(roots []deskkit.RootConfig, targetSHA string, reader deskkit.WakeInputs, issues deskkit.IssueStateSource, now time.Time) ([]loopengine.Item, error) {
	var all []loopengine.Item
	for _, r := range roots {
		items, err := scanAwaitingIn(r, targetSHA, reader, issues, now)
		if err != nil {
			return nil, fmt.Errorf("root %s (%s): %w", r.Repo, r.Path, err)
		}
		all = append(all, items...)
	}
	sort.SliceStable(all, func(i, j int) bool { return itemWorkClass(all[i]) < itemWorkClass(all[j]) })
	return all, nil
}

// itemWorkClass is workClass read back off a scanned Item's payload (status + Evidence
// emptiness), so the cross-root ordering uses exactly the per-root tier rule.
func itemWorkClass(it loopengine.Item) int {
	if strings.ToLower(payloadValue(it, "status")) == "implemented" && payloadValue(it, "evidence_empty") == "yes" {
		return 0
	}
	return 1
}

// scanAwaitingIn scans ONE root. With r.Repo empty this is the single-root read exactly as
// before (bare `<stream>/<NN>` IDs, no provenance). With r.Repo set — the multi-root plan — every
// item's ID is `<owner>/<repo>:<stream>/<NN>` and its payload carries `repo` and `root`, so the
// root is named on every printed item and two roots carrying a same-named stream cannot alias.
// issues reads a held receipt's blocker issue; nil reads every blocker as could-not-check (plan
// --no-forge), so a hold is never released — or confirmed — without a forge read.
func scanAwaitingIn(r deskkit.RootConfig, targetSHA string, reader deskkit.WakeInputs, issues deskkit.IssueStateSource, now time.Time) ([]loopengine.Item, error) {
	root := r.Path
	streamsDir := filepath.Join(root, "docs", "streams")
	entries, err := os.ReadDir(streamsDir)
	if err != nil {
		return nil, err
	}
	// Wake evaluation reads external observation only through an already-authorized reader and a
	// clock — both injectable in tests. The default is the OFFLINE, probe-free reader over this
	// root's tree (content hashes + the tool version; it never observes an external action).
	if reader == nil {
		reader = deskkit.NewRootRevisionReader(root, deskkit.ReleaseTagOrDev())
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	outcomes, receipts, futureOutcomes, oerr := readVerifyOutcomeRecords(root)
	if oerr != nil {
		return nil, oerr
	}
	var rows []briefRow
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		readme := filepath.Join(streamsDir, e.Name(), "README.md")
		raw, err := os.ReadFile(readme)
		if err != nil {
			continue // a stream dir without a README is not a fatal scan error
		}
		stream, tableRows := parseStreamTable(string(raw), e.Name())
		for _, row := range tableRows {
			st := strings.ToLower(row.Status)
			if st != "implemented" && st != "verified" {
				continue // Awaiting filter
			}
			row.Stream = stream
			row.BriefPath, row.fm, row.evidenceEmpty, row.couldNotCheck, row.verifyRows = resolveBrief(root, streamsDir, e.Name(), row.Num)
			rows = append(rows, row)
		}
	}

	// Order: tier-1 (implemented + empty Evidence) first, then tier-2; oldest-first within
	// class. Age from the statusgen historian is the live signal; for a deterministic
	// reference read we order within class by (stream, brief-num), a stable proxy.
	sort.SliceStable(rows, func(i, j int) bool {
		ci, cj := workClass(rows[i]), workClass(rows[j])
		if ci != cj {
			return ci < cj
		}
		if rows[i].Stream != rows[j].Stream {
			return rows[i].Stream < rows[j].Stream
		}
		return rows[i].Num < rows[j].Num
	})

	items := make([]loopengine.Item, 0, len(rows))
	for _, br := range rows {
		id := br.Stream + "/" + br.Num
		payload := map[string]string{
			"status":         br.Status,
			"verified":       br.Verified,
			"reviewed":       br.Reviewed,
			"blocked_until":  br.fm.BlockedUntil,
			"verify_lane":    br.fm.VerifyLane,
			"in_repair":      br.fm.InRepair,
			"evidence_empty": yesNo(br.evidenceEmpty),
			"deferred_rows":  br.fm.DeferredRows,
		}
		if br.couldNotCheck != "" {
			payload["could_not_check"] = br.couldNotCheck
		} else if futureOutcomes[br.Stream+"/"+br.Num] {
			// SR-1803-2: a verify-outcome record for this brief carries a `ts` more than
			// deskkit.MaxClockSkew ahead of now. deskkit.LatestPerBrief already refused to let it
			// win the newest-ts comparison; report the brief as could-not-check rather than
			// silently trusting whichever OTHER record (if any) was left after excluding it.
			payload["could_not_check"] = "verify-outcome record for " + br.Stream + "/" + br.Num +
				" carries a ts more than the clock-skew tolerance ahead of now — could-not-check, never let win (#1803 SR-1803-2)"
		}
		if oc, ok := outcomes[br.Stream+"/"+br.Num]; ok {
			payload["sidecar_outcome"] = oc.Outcome
			payload["sidecar_ts"] = oc.TS
			payload["sidecar_sha"] = oc.SHA
		}
		// WAKE (example-stream/16): a failed/blocked verification's latest receipt decides
		// whether re-running is worth a slot. Evaluated here (where the reader + clock live) and
		// carried onto the payload as strings, so classifyItem reads it exactly like the other
		// queue-truthfulness markers. A verified receipt is the stuck-flip lane's, not this one.
		if rec, ok := receipts[br.Stream+"/"+br.Num]; ok && rec.IsFailedOrBlocked() {
			defaultRepo := rec.Repo
			if defaultRepo == "" {
				defaultRepo = r.Repo
			}
			deriveWakePayload(payload, rec, br.verifyRows, reader, issues, defaultRepo, now)
		}
		if r.Repo != "" {
			id = r.Repo + ":" + id
			payload["repo"] = r.Repo
			payload["root"] = root
		}
		items = append(items, loopengine.Item{
			ID:          id,
			BriefPath:   br.BriefPath,
			TargetSHA:   targetSHA,
			Risk:        br.fm.Risk,
			Gate:        br.fm.Gate,
			Effort:      br.fm.Effort,
			ExecTier:    br.fm.ExecTier,
			Implementer: br.fm.Implementer,
			Payload:     payload,
		})
	}
	return items, nil
}

// outcomeRow is the subset of a verify-outcome record the stuck-flip bucket keys on.
type outcomeRow struct {
	TS      string `json:"ts"`
	Brief   string `json:"brief"`
	Outcome string `json:"outcome"`
	SHA     string `json:"sha"`
}

// readVerifyOutcomeRecords is verifyloop's ONE reader of verify-outcome records
// (#882): every record under docs/streams/verify-outcomes/ AND every line of any
// legacy docs/streams/verify-outcomes*.jsonl, reduced to the single latest-by-timestamp record
// per brief (deskkit.LatestPerBrief — never by line/file position), then decoded into the two
// shapes the rest of this file already consumes (outcomeRow for the stuck-flip bucket,
// deskkit.WakeReceipt for the wake evaluator) so nothing downstream of this function changed.
// An absent records directory and an absent legacy log together are an empty set (no brief is
// then stuck-flip — the pre-#1309 behaviour); an UNREADABLE record is an error that propagates
// as a could-not-check scan failure, never a silently skipped record (common-clause C4).
// future reports, by brief key, whether that brief's records included one deskkit.LatestPerBrief
// excluded for carrying a `ts` more than deskkit.MaxClockSkew ahead of now (#1803 SR-1803-2) — the
// caller reports such a brief as could-not-check rather than silently trusting whatever record (if
// any) was left.
func readVerifyOutcomeRecords(root string) (outcomes map[string]outcomeRow, receipts map[string]deskkit.WakeReceipt, future map[string]bool, err error) {
	records, rerr := deskkit.ReadVerifyOutcomes(root)
	if rerr != nil {
		return nil, nil, nil, rerr
	}
	outcomes = map[string]outcomeRow{}
	receipts = map[string]deskkit.WakeReceipt{}
	latest, future := deskkit.LatestPerBrief(records)
	for brief, rec := range latest {
		var row outcomeRow
		if json.Unmarshal(rec.Raw, &row) == nil && row.Brief != "" {
			outcomes[brief] = row
		}
		if wrec, ok := deskkit.ParseWakeReceipt(rec.Raw); ok {
			receipts[brief] = wrec
		}
	}
	return outcomes, receipts, future, nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// workClass returns 0 for tier-1 (the real verify work: implemented with an EMPTY Evidence
// section) and 1 for tier-2 (verified free-closes + Evidence-present-but-unpromoted). Tier-1
// must never be crowded out by tier-2.
func workClass(r briefRow) int {
	if strings.ToLower(r.Status) == "implemented" && r.evidenceEmpty {
		return 0
	}
	return 1
}

var sepRe = regexp.MustCompile(`^\s*\|?\s*-{2,}`)

// parseStreamTable extracts the stream name (frontmatter `stream:`) and the brief-table rows
// from a stream README. It mirrors statusgen/parse.go's column-by-header approach: it finds
// the pipe table whose header carries both "brief" and "status", then reads rows by column
// name so column order/extra columns do not matter.
func parseStreamTable(content, dirName string) (string, []briefRow) {
	stream := dirName
	if m := regexp.MustCompile(`(?m)^stream:\s*(\S+)`).FindStringSubmatch(content); m != nil {
		stream = m[1]
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		idx := headerIndex(line)
		if _, ok := idx["#"]; !ok {
			continue
		}
		if _, ok := idx["status"]; !ok {
			continue
		}
		var rows []briefRow
		for _, row := range lines[i+2:] { // skip the |---| separator
			if !strings.HasPrefix(strings.TrimSpace(row), "|") {
				break
			}
			if sepRe.MatchString(row) {
				continue
			}
			cells := splitRow(row)
			get := func(name string) string {
				if j, ok := idx[name]; ok && j < len(cells) {
					return strings.TrimSpace(cells[j])
				}
				return ""
			}
			num := get("#")
			if num == "" {
				continue
			}
			rows = append(rows, briefRow{
				Num:      num,
				Status:   get("status"),
				Verified: normalizeMark(get("verified")),
				Reviewed: normalizeMark(get("reviewed")),
			})
		}
		return stream, rows
	}
	return stream, nil
}

func headerIndex(line string) map[string]int {
	idx := map[string]int{}
	for j, c := range splitRow(line) {
		idx[strings.ToLower(strings.TrimSpace(c))] = j
	}
	return idx
}

func splitRow(line string) []string {
	return strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
}

func normalizeMark(s string) string {
	switch strings.TrimSpace(s) {
	case "—", "-", "–", "":
		return ""
	}
	return s
}

// resolveBrief finds the brief file for a row (docs/streams/<dir>/brief-<num>-*.md), parses
// the frontmatter subset, and reports whether its ## Evidence section is empty.
//
// FAIL CLOSED (#1309 item 5). A row whose brief file is not found, or cannot be read, returns
// a non-empty couldNotCheck reason and the ZERO frontmatter — and the caller must treat that
// row as could-not-check, never dispatchable. Before this the zero value flowed straight into
// classification: gate "" and every risk flag false read as a risk-clear model-gated brief, so
// an unresolvable row was routed to DISPATCH with its human gate erased.
func resolveBrief(root, streamsDir, dir, num string) (relPath string, fm briefFrontmatter, evidenceEmpty bool, couldNotCheck string, verifyRows []int) {
	pattern := filepath.Join(streamsDir, dir, "brief-"+num+"-*.md")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return "", briefFrontmatter{}, true, "brief file not found: " + relTo(root, pattern), nil
	}
	path := matches[0]
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", briefFrontmatter{}, true, "brief file unreadable: " + relTo(root, path) + " (" + err.Error() + ")", nil
	}
	rel := relTo(root, path)
	fm = parseFrontmatter(string(raw))
	deriveContentSignals(&fm, extractVerify(string(raw)))
	evidenceEmpty = !evidenceHasContent(extractEvidence(string(raw)))
	for _, vr := range parseVerifyRows(string(raw)) {
		verifyRows = append(verifyRows, vr.Num)
	}
	return rel, fm, evidenceEmpty, "", verifyRows
}

// deriveWakePayload evaluates one failed/blocked receipt against the hold table
// (docs/verify-wake.md; first match wins) and writes the wake markers classifyItem reads:
//
//   - an explicit-recheck receipt (the latest record, so newer than any hold) → fire,
//     "recheck: <reason>"; the blocker is not read.
//   - a receipt that is absent from the table's shape — legacy, incomplete, or a
//     relevant-input-changed receipt without a blocker_ref → no wake_state, wake_reason
//     "classification pass, writes a receipt": one ordinary pass, never a fabricated hold.
//   - complete, inputs changed → fire, "inputs changed: <path>".
//   - complete, inputs could-not-check → wake_state=could-not-check (held, surfaced).
//   - complete, inputs unchanged, blocker closed → fire, "blocker closed: <ref>".
//   - complete, inputs unchanged, blocker open → hold (a WAIT row naming the ref and the next
//     actor). If the receipt holds only some Verify rows, the rest are still runnable: fire with
//     wake_held_rows naming the held rows to record as explicitly unrun (mirrors deferred_rows).
//   - complete, inputs unchanged, blocker could-not-check → wake_state=could-not-check.
//
// Receipts on the two older predicates (referenced-action-done, deadline-reached) keep the
// verify-wake-v1 evaluator they were written for.
func deriveWakePayload(payload map[string]string, rec deskkit.WakeReceipt, verifyRows []int, reader deskkit.WakeInputs,
	issues deskkit.IssueStateSource, defaultRepo string, now time.Time) {
	fire := func(reason string) {
		payload["wake_state"] = "fire"
		payload["wake_reason"] = reason
	}
	cnc := func(reason string) {
		payload["wake_state"] = "could-not-check"
		payload["wake_reason"] = reason
	}
	switch {
	case rec.Complete() && rec.WakePredicate == deskkit.WakeExplicitRecheck:
		fire("recheck: " + strings.TrimSpace(rec.RecheckReason))
		return
	case rec.Complete() && (rec.WakePredicate == deskkit.WakeReferencedActionDone || rec.WakePredicate == deskkit.WakeDeadlineReached):
		deriveLegacyWake(payload, rec, verifyRows, reader, now)
		return
	case !rec.Complete() || strings.TrimSpace(rec.BlockerRef) == "":
		payload["wake_reason"] = "classification pass, writes a receipt"
		return
	}
	ref := strings.TrimSpace(rec.BlockerRef)
	inState, inWhy := deskkit.Unchanged(rec, reader)
	switch inState {
	case deskkit.InputsChanged:
		fire("inputs changed: " + strings.TrimPrefix(inWhy, "changed "))
		return
	case deskkit.InputsCouldNotCheck:
		cnc("inputs " + inWhy + " — held, never rounded to unchanged; next: " + rec.NextActor())
		return
	}
	blState, blWhy := deskkit.ReadBlocker(ref, defaultRepo, issues)
	switch blState {
	case deskkit.BlockerClosed:
		fire("blocker closed: " + ref)
		return
	case deskkit.BlockerCouldNotCheck:
		cnc("blocker " + ref + " could-not-check: " + blWhy + " — held, never read as closed; next: " + rec.NextActor())
		return
	}
	reason := "blocker " + ref + " open (" + rec.BlockerKind + "); next: " + rec.NextActor() +
		"; wakes when " + ref + " closes or a declared input changes"
	holdOrPartial(payload, rec, verifyRows, reason)
}

// holdOrPartial writes a hold, or — when the receipt holds only some of the brief's Verify rows —
// a fire for the runnable remainder with the held rows named.
func holdOrPartial(payload map[string]string, rec deskkit.WakeReceipt, verifyRows []int, reason string) {
	held := heldRowSet(rec.Rows)
	remaining := runnableRemainder(verifyRows, held)
	if len(rec.Rows) > 0 && len(remaining) > 0 {
		payload["wake_state"] = "fire"
		payload["wake_held_rows"] = joinInts(rec.Rows)
		payload["wake_reason"] = reason + " — holding rows " + joinInts(rec.Rows) +
			"; dispatching runnable row(s) " + joinInts(remaining)
		return
	}
	payload["wake_state"] = "hold"
	payload["wake_reason"] = reason
	if len(rec.Rows) > 0 {
		payload["wake_held_rows"] = joinInts(rec.Rows)
	}
}

// deriveLegacyWake is the verify-wake-v1 evaluator for referenced-action-done and
// deadline-reached receipts, unchanged.
func deriveLegacyWake(payload map[string]string, rec deskkit.WakeReceipt, verifyRows []int, reader deskkit.WakeInputs, now time.Time) {
	dec := rec.EvaluateWake(reader, now)
	switch dec.State {
	case deskkit.WakeCouldNotCheck:
		payload["wake_state"] = "could-not-check"
		payload["wake_reason"] = dec.Reason
	case deskkit.WakeUnclassified:
		// leave unset — one ordinary classification pass
	case deskkit.WakeFire:
		payload["wake_state"] = "fire"
		payload["wake_reason"] = dec.Reason
	case deskkit.WakeHold:
		holdOrPartial(payload, rec, verifyRows, dec.Reason)
	}
}

func heldRowSet(rows []int) map[int]bool {
	s := map[int]bool{}
	for _, r := range rows {
		s[r] = true
	}
	return s
}

// runnableRemainder is the brief's Verify rows that the receipt does NOT hold — the rows still
// runnable this pass. Empty when every row is held (or the row set is unknown).
func runnableRemainder(all []int, held map[int]bool) []int {
	var out []int
	for _, r := range all {
		if !held[r] {
			out = append(out, r)
		}
	}
	return out
}

func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}

// relTo is path relative to root, or path itself when it cannot be made relative.
func relTo(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "" {
		return path
	}
	return rel
}

// deriveContentSignals fills the two queue-truthfulness markers a real brief usually EXPRESSES in
// its Verify rows rather than declaring in frontmatter — an online/cluster verify lane and a
// longitudinal observation/accrual window. An explicit frontmatter marker always wins: derivation
// only runs when the field is empty, so an author's stated condition/lane (and its exact reason
// text) is never overwritten.
//
// PER ROW, FROM THE COMMAND CELL ONLY (#1309 item 4). The signals are read from each Verify
// row's Command cell — never from its Expect prose — because prose EXPLAINS: a row whose
// expectation says "kubectl is refused here" is an offline row, and a row whose expectation
// mentions "the shadow window" while its command is `git ls-files … | wc -l` runs offline today.
// Both were live false positives that deferred a whole brief on one row's wording. And rows are
// deferred, not briefs: when at least one row is runnable offline the brief stays dispatchable
// and DeferredRows names the rows to record as explicitly unrun; only a brief whose EVERY row
// names an online lane / a window is bucketed as a whole. A Verify section with no parseable
// Command column derives nothing (there is no command cell to read) and stays dispatchable.
func deriveContentSignals(fm *briefFrontmatter, verifyText string) {
	rows := parseVerifyRowsIn(verifyText)
	if len(rows) == 0 {
		return
	}
	var notes []string
	runnable := 0
	lane, blocked := "", ""
	for _, r := range rows {
		rl := deriveOnlineLane(r.Command)
		rb := deriveBlockedUntil(r.Command)
		if rl == "" && rb == "" {
			runnable++
			continue
		}
		if rl != "" {
			notes = append(notes, fmt.Sprintf("%d: online lane (%s)", r.Num, rl))
			if lane == "" {
				lane = rl
			}
		} else {
			notes = append(notes, fmt.Sprintf("%d: longitudinal window", r.Num))
		}
		if rb != "" && blocked == "" {
			blocked = rb
		}
	}
	if runnable > 0 {
		fm.DeferredRows = strings.Join(notes, "; ")
		return
	}
	if fm.VerifyLane == "" && lane != "" {
		fm.VerifyLane = lane
	}
	if fm.BlockedUntil == "" && blocked != "" {
		fm.BlockedUntil = blocked
	}
}

// onlineLanePhrases are the substrings in a Verify row that mean the row's substrate is a live
// cluster / online / live-session lane, not this repo's offline tree — an offline verifier run
// cannot produce the verdict. Each maps to a canonical lane value in onlineLaneValues so the
// existing classifyItem online-lane path fires on the derived value exactly as on an authored one.
var onlineLanePhrases = []struct {
	phrase string
	lane   string
}{
	{"kubectl", "cluster"},
	{"live cluster", "cluster"},
	{"against the cluster", "cluster"},
	{"external cluster", "cluster"},
	{"online lane", "online"},
	{"online verify lane", "online"},
	{"pod verify lane", "online"},
	{"pod/online verify lane", "online"},
	// A Verify row (or its Expect) that names an offline→online hand-off — the verdict is
	// produced by an external/online verifier, not this offline run. These are compound,
	// domain-specific phrases (not a bare "hand-off") so an ordinary hand-off mention does not
	// false-bucket an actionable brief.
	{"console-external-verify", "online"},
	{"external-verify", "online"},
	{"external hand-off", "online"},
	{"hand-off to the online", "online"},
	{"hand-off to the pod", "online"},
	{"offline→pod", "online"},
	{"offline->pod", "online"},
	{"live session", "live-session"},
	{"live-session", "live-session"},
}

// deriveOnlineLane returns the canonical lane if a Verify row's COMMAND cell names an
// online/cluster substrate, else "". Case-insensitive.
func deriveOnlineLane(command string) string {
	lc := strings.ToLower(command)
	for _, p := range onlineLanePhrases {
		if strings.Contains(lc, p.phrase) {
			return p.lane
		}
	}
	return ""
}

// longitudinalPhrases are the substrings in a Verify row's exit criteria that mean the brief can
// only pass once a calendar/accrual window elapses — re-verifying it now only reproduces the same
// "window not accrued" non-verdict. Kept narrow (an explicit "<qualifier> window" or accrual
// phrase) so a brief that merely mentions a window in some other sense is not falsely deferred.
var longitudinalPhrases = []string{
	"shadow window",
	"shadow clock",
	"dated capture trees",
	"observation window",
	"accrual window",
	"window accrues",
	"window has accrued",
	"window has elapsed",
	"window elapses",
	"window elapsed",
	"window not yet elapsed",
	"accrual period",
	"observation period",
}

// deriveBlockedUntil returns a human-facing defer reason if a Verify row's COMMAND cell names a
// longitudinal observation/accrual window, else "". Case-insensitive.
func deriveBlockedUntil(command string) string {
	lc := strings.ToLower(command)
	for _, p := range longitudinalPhrases {
		if strings.Contains(lc, p) {
			return "longitudinal: Verify exit criteria depend on an observation/accrual window (derived from the brief's Verify rows) — add an explicit `blocked-until:` marker to state the date/condition"
		}
	}
	return ""
}
