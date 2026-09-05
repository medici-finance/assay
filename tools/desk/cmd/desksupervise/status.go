package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// status.go — `desksupervise status [--json] [--stops] [--repo O/N] [--root DIR] [--now RFC3339]
// [--claims-fixture FILE] [--observations-fixture FILE] [--stops-fixture FILE]`.
//
// status renders the observer's per-claim runtime state as ONE structured record: a human
// table (default) and a schema-validated JSON document (--json). It answers the questions
// the desks otherwise guess at — what is running, how long each timer has to run, which
// stops are armed, and which probe sources could not be read this tick. It reuses `tick`'s
// classification (loopengine.ClassifyLiveness — the SAME taxonomy tick's sweep runs) WITHOUT
// acting: status never releases a claim, files an issue, or journals. It is a pure read.
//
// The three-state rule (docs/three-state-instrument-rule.md) binds every field:
//   - a claim whose probes could-not-check renders liveness=COULD-NOT-CHECK, its timers n/a
//     (never 0s — a blind claim has no computed remaining), and its blind source is listed in
//     aggregates.blind_sources; a snapshot carrying one exits 6 (the reading is incomplete),
//     never 0, exactly as `tick` does.
//   - `tokens` is could-not-check BY DESIGN at this layer: a dispatched worker's token usage
//     is held by the harness, not by desk tools, and no read path exists. The field is present
//     so a future harness binding can fill it; it is NEVER rendered as zero.

// statusSchemaID is the published schema marker the JSON document carries and the console
// keys on. The schema file lives at repo-root schemas/desksupervise-status-v1.json.
const statusSchemaID = "desksupervise-status-v1"

// n/a is the single string every unreadable / not-applicable field renders — never a zero,
// never an empty string (which a consumer could mistake for a missing key).
const notApplicable = "n/a"

// StatusTimers holds the three remaining-time timers, each a Go duration string ("5m0s") or
// "n/a" when the timer does not apply to this claim's phase (or the claim is blind).
type StatusTimers struct {
	ScheduleToStartRemaining string `json:"schedule_to_start_remaining"`
	HeartbeatRemaining       string `json:"heartbeat_remaining"`
	WallCapRemaining         string `json:"wall_cap_remaining"`
}

// StatusStop is a stop armed for one claim/item — distinct from a global STOP flag, which
// halts the whole desk via deskkit.Guard() before status ever runs. null when no stop is armed.
type StatusStop struct {
	ArmedAt string `json:"armed_at"`
	Reason  string `json:"reason"`
}

// StatusClaim is one in-flight claim's runtime snapshot row.
type StatusClaim struct {
	Key            string       `json:"key"`
	Repo           string       `json:"repo"`
	Item           string       `json:"item"`
	State          string       `json:"state"` // claimed | dispatched
	Holder         string       `json:"holder"`
	ClaimedAt      string       `json:"claimed_at"`
	DispatchedAt   string       `json:"dispatched_at"`
	LastObservedAt string       `json:"last_observed_at"`
	ObservedVia    string       `json:"observed_via"`
	Liveness       string       `json:"liveness"`
	Timers         StatusTimers `json:"timers"`
	Stop           *StatusStop  `json:"stop"`
	Tokens         string       `json:"tokens"`
}

// StatusAggregates rolls the per-claim rows up into the counts a console banners.
type StatusAggregates struct {
	ByLiveness   map[string]int `json:"by_liveness"`
	ByState      map[string]int `json:"by_state"`
	ArmedStops   int            `json:"armed_stops"`
	BlindSources []string       `json:"blind_sources"`
}

// StatusSnapshot is the whole runtime record — the shape schemas/desksupervise-status-v1.json
// describes and a console consumes.
type StatusSnapshot struct {
	Schema     string           `json:"schema"`
	Now        string           `json:"now"`
	Claims     []StatusClaim    `json:"claims"`
	Aggregates StatusAggregates `json:"aggregates"`
}

// claimObs is status's richer observation outcome: the fixture/live source captures the BLIND
// REASON (which probe source could not be read) so aggregates.blind_sources can name it, where
// sweep's resolvedObservation only carries a bool.
type claimObs struct {
	observed    bool
	obs         loopengine.Observation
	blindReason string
}

// statusObsSource resolves one claim's cross-probe reading for the snapshot. A returned error
// aborts the WHOLE snapshot as could-not-check (a malformed fixture / unparseable dispatchedAt
// is a config error in status's own input, not a liveness question — guessing would silently
// misclassify).
type statusObsSource func(claim claimRecord) (claimObs, error)

// fixtureStatusObs is the offline observationSource --observations-fixture builds for status.
// A claim key ABSENT from the fixture is "never observed, checked cleanly" (not blind); a key
// with couldNotCheck=true is BLIND and carries its error as the blind reason.
func fixtureStatusObs(byKey map[string]observationRecord) statusObsSource {
	return func(c claimRecord) (claimObs, error) {
		rec, ok := byKey[c.Key]
		if !ok {
			return claimObs{observed: true}, nil // cleanly silent
		}
		if rec.CouldNotCheck {
			reason := strings.TrimSpace(rec.Error)
			if reason == "" {
				reason = "could-not-check"
			}
			return claimObs{observed: false, blindReason: reason}, nil
		}
		r, err := rec.resolve()
		if err != nil {
			return claimObs{}, deskkit.Unverifiable("claim "+c.Key+": "+err.Error(), err)
		}
		return claimObs{observed: true, obs: r.obs}, nil
	}
}

// liveStatusObs is the production observationSource: run loopengine.HouseProbes() against an
// Item built from the claim's own fields, looking no earlier than the claim's dispatchedAt. A
// probe error is BLIND (could-not-check) with the error text as the reason — never presumed
// "no life", exactly the conservative rule liveness.go documents.
func liveStatusObs(probes *loopengine.ObservableProbes) statusObsSource {
	return func(c claimRecord) (claimObs, error) {
		since, err := time.Parse(time.RFC3339, c.DispatchedAt)
		if err != nil {
			return claimObs{}, deskkit.Unverifiable("claim "+c.Key+": dispatchedAt "+c.DispatchedAt+" is not RFC3339", err)
		}
		it := loopengine.Item{
			ID: c.Item,
			Payload: map[string]string{
				loopengine.PayloadSessionTag: c.Owner,
				loopengine.PayloadRepo:       c.Repo,
				loopengine.PayloadBranch:     c.Branch,
			},
		}
		if c.PR > 0 {
			it.Payload[loopengine.PayloadPR] = strconv.Itoa(c.PR)
		}
		obs, perr := probes.Latest(it, since)
		if perr != nil {
			return claimObs{observed: false, blindReason: perr.Error()}, nil
		}
		return claimObs{observed: true, obs: obs}, nil
	}
}

// loadStopsFixture reads a --stops-fixture file: a JSON object keyed by claim key, each value
// {armed_at, reason}. A read/parse failure is could-not-check (a stops fixture that cannot be
// read is never treated as "no stops").
func loadStopsFixture(path string) (map[string]*StatusStop, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read --stops-fixture "+path, err)
	}
	var raw map[string]struct {
		ArmedAt string `json:"armed_at"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, deskkit.Unverifiable("cannot parse --stops-fixture "+path+" as a JSON object keyed by claim key", err)
	}
	out := map[string]*StatusStop{}
	for k, v := range raw {
		out[k] = &StatusStop{ArmedAt: orNA(v.ArmedAt), Reason: v.Reason}
	}
	return out, nil
}

// buildSnapshot classifies every claim once against pol at now — reusing ClassifyLiveness, the
// exact taxonomy tick's sweep runs — and assembles the structured record WITHOUT acting. When
// stopsOnly is true the rendered claim list is filtered to claims carrying an armed stop; the
// blind reading (and thus the exit-6 signal) is computed over the FULL set regardless of the
// filter, because a could-not-check anywhere means the tick's reading was incomplete.
func buildSnapshot(claims []claimRecord, obsSource statusObsSource, stopsByKey map[string]*StatusStop, stopsOnly bool, pol loopengine.LivenessPolicy, now time.Time) (StatusSnapshot, bool, error) {
	snap := StatusSnapshot{
		Schema: statusSchemaID,
		Now:    now.UTC().Format(time.RFC3339),
		Claims: []StatusClaim{},
		Aggregates: StatusAggregates{
			ByLiveness:   map[string]int{},
			ByState:      map[string]int{},
			BlindSources: []string{},
		},
	}
	anyBlind := false

	for _, c := range claims {
		tier, terr := tierOf(c.Tier)
		if terr != nil {
			return StatusSnapshot{}, false, deskkit.Unverifiable("claim "+c.Key+": "+terr.Error(), terr)
		}
		dispatchedAt, derr := time.Parse(time.RFC3339, c.DispatchedAt)
		if derr != nil {
			return StatusSnapshot{}, false, deskkit.Unverifiable("claim "+c.Key+": dispatchedAt "+c.DispatchedAt+" is not RFC3339", derr)
		}
		co, oerr := obsSource(c)
		if oerr != nil {
			return StatusSnapshot{}, false, oerr
		}

		state := c.State
		if state == "" {
			state = "dispatched"
		}
		row := StatusClaim{
			Key:          c.Key,
			Repo:         c.Repo,
			Item:         c.Item,
			State:        state,
			Holder:       orNA(c.Owner),
			ClaimedAt:    orNA(c.ClaimedAt),
			DispatchedAt: c.DispatchedAt,
			Stop:         stopsByKey[c.Key],
			Tokens:       "could-not-check",
		}

		if !co.observed {
			anyBlind = true
			row.Liveness = "COULD-NOT-CHECK"
			row.LastObservedAt = notApplicable
			row.ObservedVia = notApplicable
			// A blind claim has NO computed remaining — never render 0s for one.
			row.Timers = StatusTimers{ScheduleToStartRemaining: notApplicable, HeartbeatRemaining: notApplicable, WallCapRemaining: notApplicable}
			reason := co.blindReason
			if reason == "" {
				reason = "could-not-check"
			}
			snap.Aggregates.BlindSources = append(snap.Aggregates.BlindSources, fmt.Sprintf("%s (%s)", reason, c.Key))
		} else {
			disp := loopengine.ClassifyLiveness(pol, loopengine.ClaimClock{DispatchedAt: dispatchedAt}, tier, now, co.obs)
			row.Liveness = disp.String()
			row.Timers = computeTimers(pol, tier, dispatchedAt, co.obs.At, now)
			if co.obs.At.IsZero() {
				row.LastObservedAt = notApplicable
				row.ObservedVia = notApplicable
			} else {
				row.LastObservedAt = co.obs.At.UTC().Format(time.RFC3339)
				row.ObservedVia = orNA(co.obs.What)
			}
		}

		snap.Aggregates.ByLiveness[row.Liveness]++
		snap.Aggregates.ByState[state]++
		if row.Stop != nil {
			snap.Aggregates.ArmedStops++
		}

		if stopsOnly && row.Stop == nil {
			continue // --stops: render only claims with an armed stop
		}
		snap.Claims = append(snap.Claims, row)
	}

	sort.Strings(snap.Aggregates.BlindSources)
	return snap, anyBlind, nil
}

// computeTimers derives the three remaining-time timers for a NON-blind claim. In the
// single-observation snapshot model ClassifyLiveness uses, the one observation is both the
// start event and the latest heartbeat, so startedAt == lastObserved == obsAt; a zero obsAt
// means the worker has emitted no sign of life yet (not started). A timer that does not apply
// to the claim's current phase — or whose policy knob is disabled — renders n/a, never 0s.
func computeTimers(pol loopengine.LivenessPolicy, tier loopengine.Tier, dispatchedAt, obsAt, now time.Time) StatusTimers {
	started := !obsAt.IsZero()
	t := StatusTimers{
		ScheduleToStartRemaining: notApplicable,
		HeartbeatRemaining:       notApplicable,
		WallCapRemaining:         notApplicable,
	}
	if !started {
		if pol.ScheduleToStart > 0 {
			t.ScheduleToStartRemaining = (pol.ScheduleToStart - now.Sub(dispatchedAt)).String()
		}
		return t
	}
	if pol.HeartbeatGap > 0 {
		t.HeartbeatRemaining = (pol.HeartbeatGap - now.Sub(obsAt)).String()
	}
	if cap := pol.StartToClose[tier]; cap > 0 {
		t.WallCapRemaining = (cap - now.Sub(obsAt)).String()
	}
	return t
}

// orNA returns s, or the n/a marker when s is empty — so no field renders a bare empty string a
// consumer could mistake for a missing key.
func orNA(s string) string {
	if strings.TrimSpace(s) == "" {
		return notApplicable
	}
	return s
}

// renderStatusJSON marshals the snapshot as indented JSON (the schema-validated contract).
func renderStatusJSON(w io.Writer, snap StatusSnapshot) error {
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return deskkit.Unverifiable("cannot marshal the status snapshot", err)
	}
	_, werr := fmt.Fprintln(w, string(b))
	return werr
}

// renderStatusTable renders the snapshot as a human table (NOT a contract — the JSON is). Each
// claim is one block; the aggregates and any blind sources follow.
func renderStatusTable(w io.Writer, snap StatusSnapshot) {
	fmt.Fprintf(w, "runtime snapshot @ %s (schema %s)\n", snap.Now, snap.Schema)
	fmt.Fprintln(w, strings.Repeat("-", 72))
	for _, c := range snap.Claims {
		fmt.Fprintf(w, "%s  [%s]  %s\n", c.Key, c.State, c.Liveness)
		fmt.Fprintf(w, "  item=%s holder=%s\n", c.Item, c.Holder)
		fmt.Fprintf(w, "  dispatched_at=%s last_observed_at=%s via=%s\n", c.DispatchedAt, c.LastObservedAt, c.ObservedVia)
		fmt.Fprintf(w, "  timers: schedule_to_start=%s heartbeat=%s wall_cap=%s\n",
			c.Timers.ScheduleToStartRemaining, c.Timers.HeartbeatRemaining, c.Timers.WallCapRemaining)
		fmt.Fprintf(w, "  tokens=%s\n", c.Tokens)
		if c.Stop != nil {
			fmt.Fprintf(w, "  STOP armed_at=%s reason=%s\n", c.Stop.ArmedAt, c.Stop.Reason)
		}
	}
	fmt.Fprintln(w, strings.Repeat("-", 72))
	fmt.Fprintf(w, "by_liveness: %s\n", countsLine(snap.Aggregates.ByLiveness))
	fmt.Fprintf(w, "by_state: %s\n", countsLine(snap.Aggregates.ByState))
	fmt.Fprintf(w, "armed_stops: %d\n", snap.Aggregates.ArmedStops)
	if len(snap.Aggregates.BlindSources) > 0 {
		fmt.Fprintf(w, "blind_sources (COULD-NOT-CHECK this tick): %s\n", strings.Join(snap.Aggregates.BlindSources, "; "))
	} else {
		fmt.Fprintln(w, "blind_sources: none")
	}
}

// countsLine renders a count map deterministically (sorted keys) for the human table.
func countsLine(m map[string]int) string {
	if len(m) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, " ")
}

// snapshotFromSweep builds the runtime snapshot from a tick's already-computed sweepResults —
// no re-probe, so a `run --interval` loop writes status.json from the SAME reading its actions
// were taken on (re-probing would double-advance the branch probe's in-process SHA memory and
// disagree with what the tick actually acted on). A blind result carries no per-source reason
// here (sweepResult does not retain it), so its blind_sources entry names the claim key alone;
// the richer `status` verb path (buildSnapshot) keeps the probe reason.
func snapshotFromSweep(results []sweepResult, pol loopengine.LivenessPolicy, now time.Time) StatusSnapshot {
	snap := StatusSnapshot{
		Schema: statusSchemaID,
		Now:    now.UTC().Format(time.RFC3339),
		Claims: []StatusClaim{},
		Aggregates: StatusAggregates{
			ByLiveness:   map[string]int{},
			ByState:      map[string]int{},
			BlindSources: []string{},
		},
	}
	for _, r := range results {
		state := r.Claim.State
		if state == "" {
			state = "dispatched"
		}
		row := StatusClaim{
			Key:          r.Claim.Key,
			Repo:         r.Claim.Repo,
			Item:         r.Claim.Item,
			State:        state,
			Holder:       orNA(r.Claim.Owner),
			ClaimedAt:    orNA(r.Claim.ClaimedAt),
			DispatchedAt: r.Claim.DispatchedAt,
			Tokens:       "could-not-check",
		}
		if r.Blind {
			row.Liveness = "COULD-NOT-CHECK"
			row.LastObservedAt = notApplicable
			row.ObservedVia = notApplicable
			row.Timers = StatusTimers{ScheduleToStartRemaining: notApplicable, HeartbeatRemaining: notApplicable, WallCapRemaining: notApplicable}
			snap.Aggregates.BlindSources = append(snap.Aggregates.BlindSources, fmt.Sprintf("could-not-check (%s)", r.Claim.Key))
		} else {
			row.Liveness = r.Disp.String()
			if tier, terr := tierOf(r.Claim.Tier); terr == nil {
				if dispatchedAt, derr := time.Parse(time.RFC3339, r.Claim.DispatchedAt); derr == nil {
					row.Timers = computeTimers(pol, tier, dispatchedAt, r.Last, now)
				}
			}
			if r.Last.IsZero() {
				row.LastObservedAt = notApplicable
			} else {
				row.LastObservedAt = r.Last.UTC().Format(time.RFC3339)
			}
			row.ObservedVia = orNA(r.Via)
		}
		snap.Aggregates.ByLiveness[row.Liveness]++
		snap.Aggregates.ByState[state]++
		snap.Claims = append(snap.Claims, row)
	}
	sort.Strings(snap.Aggregates.BlindSources)
	return snap
}

// statusJSONPath resolves <StateDir>/supervise/status.json — the file `run --interval` writes
// each tick for a local reader, alongside the other desk-tools state (kill switch, audit log).
func statusJSONPath() (string, error) {
	stateDir, err := deskkit.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "supervise", "status.json"), nil
}

// writeStatusJSON writes the snapshot to path atomically (temp file in the same directory +
// rename), so a local reader never observes a half-written document. The parent directory is
// created if absent.
func writeStatusJSON(path string, snap StatusSnapshot) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal status snapshot: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".status-*.json.tmp")
	if err != nil {
		return fmt.Errorf("cannot create a temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("cannot write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot rename %s to %s: %w", tmpName, path, err)
	}
	return nil
}

// cmdStatus implements the `status` verb. It is a PURE READ — no claim is released, no issue
// filed, no journal line written — so it takes no --dry-run (there is nothing to suppress).
func cmdStatus(args []string) (err error) {
	ac := &auditCtx{verb: "status"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	jsonOut := fs.Bool("json", false, "emit the schema-validated JSON document instead of the human table")
	stopsOnly := fs.Bool("stops", false, "render only claims carrying an armed stop")
	repo := fs.String("repo", "", "owner/name of the repo whose dispatch claims to snapshot (live mode)")
	root := fs.String("root", "", "repo root to read a live claim tool against (default: cwd)")
	nowStr := fs.String("now", "", "RFC3339 clock override — Verify-fixture use only; a live run always uses the real clock")
	claimsFixture := fs.String("claims-fixture", "", "JSON array of claim records — bypasses the live claim tool")
	obsFixture := fs.String("observations-fixture", "", "JSON object keyed by claim key — bypasses the forge and audit file")
	stopsFixture := fs.String("stops-fixture", "", "JSON object keyed by claim key — per-claim armed stops (offline)")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: status takes no positional arguments")
	}
	if (*claimsFixture == "") != (*obsFixture == "") {
		return deskkit.Refused("refused: --claims-fixture and --observations-fixture are given together or not at all")
	}
	ac.repo = *repo

	now := time.Now().UTC()
	if *nowStr != "" {
		parsed, perr := time.Parse(time.RFC3339, *nowStr)
		if perr != nil {
			return deskkit.Refused("refused: --now " + *nowStr + " is not RFC3339")
		}
		now = parsed
	}

	pol, perr := loadPolicy()
	if perr != nil {
		return perr
	}

	var (
		claims    []claimRecord
		obsSource statusObsSource
	)
	if *claimsFixture != "" {
		claims, err = loadClaimsFixture(*claimsFixture)
		if err != nil {
			return err
		}
		byKey, oerr := loadObservationsFixture(*obsFixture)
		if oerr != nil {
			return oerr
		}
		obsSource = fixtureStatusObs(byKey)
	} else {
		dir := *root
		if dir == "" {
			wd, gerr := os.Getwd()
			if gerr != nil {
				return deskkit.Unverifiable("cannot resolve working directory", gerr)
			}
			dir = wd
		}
		liveClaims, cerr := readLiveClaims(dir, *repo, now)
		if cerr != nil {
			return cerr
		}
		claims = liveClaims
		obsSource = liveStatusObs(loopengine.HouseProbes())
	}

	// Per-claim stops come from --stops-fixture offline. KNOWN GAP: no LIVE per-item stop
	// source is implemented yet — a GLOBAL STOP flag already halts this tool via
	// deskkit.Guard() before status runs, so a global halt cannot silently pass unseen; a
	// per-ITEM stop marker is follow-on work. In live mode the per-claim stop stays null.
	stopsByKey := map[string]*StatusStop{}
	if *stopsFixture != "" {
		stopsByKey, err = loadStopsFixture(*stopsFixture)
		if err != nil {
			return err
		}
	}

	snap, anyBlind, berr := buildSnapshot(claims, obsSource, stopsByKey, *stopsOnly, pol, now)
	if berr != nil {
		return berr
	}

	if *jsonOut {
		if werr := renderStatusJSON(os.Stdout, snap); werr != nil {
			return deskkit.Unverifiable("cannot write the status JSON", werr)
		}
	} else {
		renderStatusTable(os.Stdout, snap)
	}

	ac.detail = fmt.Sprintf("%d claim(s) rendered, blind=%t, stops-only=%t, json=%t", len(snap.Claims), anyBlind, *stopsOnly, *jsonOut)
	if len(claims) == 0 {
		ac.successResult = deskkit.ResultNoop
	}
	if anyBlind {
		// could-not-check is never a pass: a snapshot that saw a blind claim exits 6, exactly
		// as `tick` does. The snapshot itself rendered fine; the READING is incomplete.
		return deskkit.Unverifiable(fmt.Sprintf(
			"%d claim(s) were COULD-NOT-CHECK this tick — the snapshot rendered, the reading is incomplete",
			len(snap.Aggregates.BlindSources)), nil)
	}
	return nil
}
