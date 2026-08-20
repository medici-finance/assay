package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// drives.go — methodology-metrics/45, phase 1: the operator-declared DRIVE
// campaign. A standalone YAML manifest under docs/roadmap/drives/ adds a
// temporal, ATTRIBUTED, ADDITIVE steer to the Next-up score so the fleet
// self-organizes to a declared campaign — without corrupting the score.
//
// The four load-bearing properties this file must never violate (brief-44):
//
//  1. ABSENT ⇒ INERT. No manifest under docs/roadmap/drives/ ⇒ the zero DriveSet,
//     which contributes exactly 0 to every brief. A board generated with no
//     manifest is BYTE-IDENTICAL to the pre-drives board (TestDriveAbsentIsInert).
//
//  2. FAIL-NEUTRAL POLARITY (THE SAFETY BAR). A malformed / unresolvable / expired
//     / over-concurrent manifest ⇒ ZERO boost, the board STILL generates (rc 0), a
//     loud WARN + a "DRIVE NOT APPLIED" banner — NEVER an rc≠0 PROBLEM. This is the
//     one-PROBLEM-freezes-the-whole-board failure mode: a drive manifest must never
//     be able to freeze the board (TestDriveMalformedFailNeutral). Every loader
//     path here returns a DriveSet — none returns an error.
//
//  3. ADDITIVE, NEVER A MULTIPLIER; MAX ON OVERLAP, NEVER SUM; ≤2 CONCURRENT. The
//     term sits BESIDE weightP0/valueWeight/unblocksWeight (nextup.go) with the
//     same F-09 tunable-heuristic discipline. Two drives covering one brief take
//     the MAX of their terms, never the sum; at most 2 drives are ever active.
//
//  4. A STEER, NOT A VALUE CLAIM. The term is EXCLUDED from the gate-score (mm/11)
//     and from every measured metric (bottleneck / velocity / dora / exports): it
//     re-ranks the Next-up board only, exactly like value/blockedCount, and it is
//     applied AFTER eligibility so it never changes what is eligible.

// Drive intensity weights (F-09 TUNABLE HEURISTIC — not truths, cf. the
// weightP0/valueWeight/unblocksWeight block in nextup.go). ADDITIVE terms added
// beside the priority/staleness/value/unblocks score, never a multiplier, so the
// existing tier reasoning is preserved. Sized against that block: `surge` (+2500)
// can jump a routine priority tier (1000) but the critical tier (a hard
// lexicographic order above all scores, methodology-metrics phase 3) still sits
// above every intensity, so no drive can bury a live fire.
const (
	driveFocusWeight = 800
	drivePushWeight  = 1500
	driveSurgeWeight = 2500

	// maxConcurrentDrives caps how many drives may be active at once. More than
	// this is an over-declaration and is fail-neutral (zero boost + WARN), never a
	// guess about which drives "win".
	maxConcurrentDrives = 2

	// driveExpiryHorizonDays bounds how far out `expires` may sit from `starts`. A
	// drive is a short campaign, not a standing policy; a longer window is rejected
	// fail-neutral so a stale manifest cannot steer the fleet indefinitely.
	driveExpiryHorizonDays = 14

	// driveCoverageThreshold is the anti-Goodhart self-tax trigger. A drive
	// covering more than this fraction of the eligible board scales DOWN (concave
	// self-tax, see scaleForCoverage) and emits a NOTICE — tagging the whole board
	// buys progressively less, so a drive cannot win by covering everything.
	driveCoverageThreshold = 0.40

	// driveSlotCap and driveWorkerCap are the ANTI-STARVATION FLOORS (F-09 TUNABLE
	// HEURISTICS, phase 3 — not truths, cf. the weight block above). Because
	// staleness caps at +300 (stalenessCapDays×stalenessPerDay), a buried same-tier
	// brief can never out-age a drive, so starvation is permanent-by-arithmetic
	// WITHOUT reserved capacity.
	//
	//   - driveSlotCap floors the board: drive-boosted picks take at most 15 of the
	//     20 spanOfControl slots via a 2-pass fill (nextup.go), reserving ≥5
	//     non-drive-consumable slots; a drive pick held back past the cap attributes
	//     to the HeldByDriveCap bucket (mirrors HeldByStreamCap). A stream's declared
	//     max-concurrent (mm/13) still ALWAYS wins via effectiveCap = min(cap,
	//     driveStreamCap) in the streamCaps build loop.
	//   - driveWorkerCap is the PAIRED worker-pool floor (≤6 of 8) the worker-desk
	//     skill mirrors. It is declared HERE as the single numeric source of truth;
	//     statusgen does not dispatch workers, so this binary does not read the const
	//     — the skill does. Kept beside driveSlotCap so the two floors move together.
	driveSlotCap   = 15
	driveWorkerCap = 6
)

// Reference driveWorkerCap so its role as the worker-desk skill's numeric source of
// truth is explicit — statusgen does not dispatch workers, so nothing else reads it.
var _ = driveWorkerCap

// THE ONE SANCTIONED WALL-CLOCK INPUT (brief-44 ratified decision #1, human:ian
// 2026-08-15). statusgen is otherwise byte-stable with NO wall-clock input — the
// Next-up "now" is the most-recent activity in the tree, never time.Now — so
// STATUS.md regenerates identically on any machine at any time. The drive window
// (`starts`/`expires`) is the single, deliberate deviation: whether a drive is
// live depends on the calendar day. The granularity is the UTC DAY (both the
// window bounds and "today" are truncated to UTC midnight) — never finer — so the
// deviation's blast radius is one board-day, and two regenerations on the same
// UTC day still agree. nowFunc() is the clock (overridable in tests); driveDay
// truncates it. This deviation is walled here: nothing else in statusgen reads
// the wall clock for scoring.
func driveDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

var (
	// briefRefRe matches a typed brief reference "<stream>/<NN>" (NN may carry a
	// trailing letter, e.g. 12a), mirroring the depends: grammar.
	briefRefRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[0-9]+[a-z]?$`)
	// fullIssueRefRe matches a FULL cross-repo issue reference "owner/repo#N". A
	// bare "#N" never matches — cross-repo full-ref enforcement (brief-44): a drive
	// spans repos, so a bare number is ambiguous and is rejected fail-neutral.
	fullIssueRefRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+$`)
)

// DriveItem is one resolved member of a drive's item list. Kind is one of
// stream | brief | issue | operator-act | plan-gap. Only stream and brief items
// contribute board boost in phase 1; issue / operator-act / plan-gap items are
// parsed and validated here (the lifecycle/frontier that consumes them is
// methodology-metrics phase 2).
type DriveItem struct {
	Kind     string
	Ref      string // stream name, "stream/NN", or "owner/repo#N"
	Owner    string // "operator" for operator-act rows
	Unblocks string // what an operator-act row unblocks
	Needs    string // "brief" for plan-gap rows
	// Since is the optional operator-act `since:` date (YYYY-MM-DD): the day this
	// act's aging clock started. "" when absent — the phase-2 frontier then ages it
	// from the drive's own `starts` (operatorActSince). Only operator-act rows carry
	// it; the loader validates the format fail-neutral.
	Since string
	// StreamCap is the optional per-stream `drive-stream-cap:` on a STREAM item
	// (phase 3): the most drive-boosted picks this drive may offer from that stream.
	// 0 when absent. effectiveCap = min(perStreamCap-or-MaxConcurrent, StreamCap) in
	// nextup.go — a stream's declared MaxConcurrent (mm/13) ALWAYS wins; this can
	// only restrict a stream's drive draw further, never widen it. Fail-neutral: a
	// negative value rejects the whole drive (mirrors the `since:` validation).
	StreamCap int
}

// Drive is one parsed, ACTIVE, in-window drive campaign.
type Drive struct {
	Slug       string
	DeclaredBy string
	Starts     string
	Expires    string
	Intensity  string
	Why        string
	State      string
	Items      []DriveItem
	streamSet  map[string]bool // streams named directly (boosts every brief in them)
	briefSet   map[string]bool // "stream/NN" named directly (boosts exactly that brief)
	streamCaps map[string]int  // per-stream drive-stream-cap (phase 3); absent stream ⇒ no cap
}

// covers reports whether this drive names streamName (whole-stream membership) or
// the specific brief id ("stream/NN").
func (d Drive) covers(streamName, briefID string) bool {
	return d.streamSet[streamName] || d.briefSet[briefID]
}

// DriveSet is the drive state in effect for one run: the active drives (0..2),
// plus fail-neutral warnings and the not-applied reason for the banner. It always
// carries a valid state — the loader never returns an error, so a broken manifest
// degrades to {NotApplied} rather than failing the board.
type DriveSet struct {
	Active     []Drive
	Warnings   []string // fail-neutral WARN lines — surfaced as NOTICEs (exit 0), never PROBLEMs
	NotApplied bool     // a manifest existed but no boost was applied (malformed/expired/over-concurrent)
	Reason     string   // one-line reason for the DRIVE NOT APPLIED banner
}

// applied reports whether at least one drive is contributing a boost this run.
func (ds DriveSet) applied() bool { return len(ds.Active) > 0 }

// driveManifest is the raw YAML shape of a docs/roadmap/drives/<slug>.yaml file.
type driveManifest struct {
	Schema     string          `yaml:"schema"`
	DeclaredBy string          `yaml:"declared-by"`
	Starts     string          `yaml:"starts"`
	Expires    string          `yaml:"expires"`
	Intensity  string          `yaml:"intensity"`
	Why        string          `yaml:"why"`
	State      string          `yaml:"state"`
	Items      []driveItemYAML `yaml:"items"`
}

type driveItemYAML struct {
	Stream         string `yaml:"stream"`
	Brief          string `yaml:"brief"`
	Issue          string `yaml:"issue"`
	Owner          string `yaml:"owner"`
	Unblocks       string `yaml:"unblocks"`
	Needs          string `yaml:"needs"`
	Since          string `yaml:"since"`            // operator-act only: aging-clock start (YYYY-MM-DD)
	DriveStreamCap int    `yaml:"drive-stream-cap"` // stream item only: per-stream drive draw cap (phase 3)
}

// driveIntensityWeight maps an intensity enum to its additive weight. The second
// return reports whether the enum is recognized — an unknown intensity is a
// malformed manifest (fail-neutral), not a silent zero.
func driveIntensityWeight(intensity string) (int, bool) {
	switch intensity {
	case "focus":
		return driveFocusWeight, true
	case "push":
		return drivePushWeight, true
	case "surge":
		return driveSurgeWeight, true
	}
	return 0, false
}

// loadDrives reads every docs/roadmap/drives/<slug>.yaml under root and returns
// the DriveSet in effect at `now`. It NEVER returns an error: an absent directory
// is the inert baseline; any broken manifest degrades to a fail-neutral WARN +
// banner. `now` is the sanctioned wall-clock (see driveDay) — passed in so tests
// pin the board-day.
func loadDrives(root string, streams []*Stream, now time.Time) DriveSet {
	dir := filepath.Join(root, "docs", "roadmap", "drives")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// ABSENT ⇒ INERT. The common case (no drive declared) and the byte-
			// identical baseline path: silent, no warning, no banner.
			return DriveSet{}
		}
		// UNREADABLE (permission/ENOTDIR/…) is fail-neutral, not a PROBLEM: a drive
		// manifest — or the directory holding it — must never freeze the board.
		return DriveSet{
			NotApplied: true,
			Reason:     fmt.Sprintf("docs/roadmap/drives unreadable: %v", err),
			Warnings:   []string{fmt.Sprintf("DRIVE NOT APPLIED (WARN): docs/roadmap/drives could not be read (%v) — zero boost, board still generated (fail-neutral)", err)},
		}
	}
	// Known-ref index over the loaded board. An item naming a stream, or a
	// "stream/NN" brief, that resolves to NOTHING here is UNRESOLVABLE — the safety
	// bar names "unresolvable" explicitly and it must be fail-neutral (WARN +
	// banner), never silently inert. The likeliest real mistake is an un-zero-
	// padded ref (`education/7`) that passes the ref-shape regex but matches no
	// brief; without this resolution it would boost nothing and say nothing.
	knownStreams := map[string]bool{}
	knownBriefs := map[string]bool{}
	for _, s := range streams {
		knownStreams[s.Name] = true
		for _, b := range s.Briefs {
			knownBriefs[s.Name+"/"+b.Num] = true
		}
	}
	var ds DriveSet
	var active []Drive
	var rejected []string // slugs rejected fail-neutral, for the banner reason
	// Any regular file under the drives dir that the loader does NOT read as a
	// manifest — a ".yml" typo, or a manifest tucked in a subdirectory — is
	// otherwise silently inert: the operator's declared campaign does nothing and
	// says nothing. Warn on each, fail-neutral.
	for _, w := range strayManifestWarnings(dir) {
		ds.Warnings = append(ds.Warnings, w)
		ds.NotApplied = true
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			// Non-manifest top-level files and subdirectories are surfaced by
			// strayManifestWarnings above, not silently skipped.
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".yaml")
		d, warn, silent := parseDrive(root, filepath.Join(dir, e.Name()), slug, now, knownStreams, knownBriefs)
		switch {
		case warn != "":
			// Malformed / expired / unresolvable ⇒ fail-neutral WARN + banner.
			ds.Warnings = append(ds.Warnings, warn)
			ds.NotApplied = true
			rejected = append(rejected, slug)
		case silent:
			// Legitimately inactive (state labelled non-active, or a future not-yet-
			// started window): zero boost, and NO warning/banner — a scheduled or
			// retired drive is a decision, not a fault.
		case d != nil:
			active = append(active, *d)
		}
	}
	// Deterministic order so overlap/coverage math and the banner are stable.
	sort.Slice(active, func(i, j int) bool { return active[i].Slug < active[j].Slug })
	// ≤2 concurrent. Over-declaration is fail-neutral: apply NONE — guessing which
	// two "win" would silently drop a campaign the operator declared.
	if len(active) > maxConcurrentDrives {
		slugs := make([]string, len(active))
		for i, d := range active {
			slugs[i] = d.Slug
		}
		ds.NotApplied = true
		ds.Reason = fmt.Sprintf("%d drives active (%s) exceed the ≤%d-concurrent cap", len(active), strings.Join(slugs, ", "), maxConcurrentDrives)
		ds.Warnings = append(ds.Warnings, fmt.Sprintf("DRIVE NOT APPLIED (WARN): %s — zero boost, board still generated (fail-neutral)", ds.Reason))
		return ds
	}
	ds.Active = active
	if len(active) > 0 {
		// At least one drive applied: the applied drives ARE the headline, so the
		// DRIVE NOT APPLIED banner is cleared — a sibling rejected/stray file still
		// keeps its own WARN line in ds.Warnings (surfaced as a NOTICE). Without
		// this the board renders two directly opposed headline banners two lines
		// apart, which is the worst outcome for a feature whose premise is honesty.
		ds.NotApplied = false
	}
	if ds.NotApplied && ds.Reason == "" {
		// Name the rejected manifest(s) in the banner reason — the WARN lines
		// already do, and a generic "a drive manifest was rejected" hides which.
		if len(rejected) > 0 {
			ds.Reason = fmt.Sprintf("drive manifest(s) rejected: %s", strings.Join(rejected, ", "))
		} else {
			ds.Reason = "a drive manifest was rejected"
		}
	}
	return ds
}

// strayManifestWarnings returns a fail-neutral WARN for every regular file under
// the drives directory that the loader does NOT read as a manifest: a non-*.yaml
// top-level file (e.g. a ".yml" typo) or ANY file in a subdirectory. Only
// top-level *.yaml files are manifests; without this such a file is silently
// inert, so an operator's declared campaign would do nothing and say nothing.
// Dotfiles (editor/OS cruft like .DS_Store) are ignored so they don't spuriously
// redden the board.
func strayManifestWarnings(dir string) []string {
	var warns []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable entry is the caller's fail-neutral concern
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return nil
		}
		if strings.Contains(rel, string(os.PathSeparator)) {
			warns = append(warns, fmt.Sprintf("DRIVE NOT APPLIED (WARN): manifest %q sits in a subdirectory under docs/roadmap/drives — only top-level *.yaml manifests are read; move it up — zero boost, board still generated (fail-neutral)", rel))
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yaml") {
			warns = append(warns, fmt.Sprintf("DRIVE NOT APPLIED (WARN): file %q under docs/roadmap/drives is not a *.yaml manifest (a .yml typo is the likeliest cause) — it is ignored; rename to *.yaml — zero boost, board still generated (fail-neutral)", rel))
		}
		return nil
	})
	sort.Strings(warns)
	return warns
}

// escapesRoot reports whether resolved path p lies outside root. Both sides are
// symlink-resolved so the comparison is between real paths — the containment
// check for a manifest that might be a symlink pointing out of the tree.
func escapesRoot(root, p string) bool {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	rel, err := filepath.Rel(realRoot, p)
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// parseDrive reads and validates one manifest file. It returns:
//   - (drive, "", false)  — a valid, active, in-window drive to apply;
//   - (nil, warn, false)  — a malformed/expired/unresolvable manifest: fail-neutral;
//   - (nil, "", true)     — a legitimately inactive drive (state≠active, or future): silent.
//
// It never returns an error — every failure is a warn string.
func parseDrive(root, path, slug string, now time.Time, knownStreams, knownBriefs map[string]bool) (drive *Drive, warn string, silent bool) {
	warnf := func(format string, a ...any) (*Drive, string, bool) {
		return nil, fmt.Sprintf("DRIVE NOT APPLIED (WARN): drive %q — %s — zero boost, board still generated (fail-neutral)", slug, fmt.Sprintf(format, a...)), false
	}
	// Symlink containment: a manifest committed as a symlink pointing OUTSIDE the
	// repo is otherwise followed and applied — precisely the surface the gate-why
	// calls out as invisible in a diff. Resolve the real path and require it stay
	// under root; an escape is fail-neutral.
	if real, rerr := filepath.EvalSymlinks(path); rerr == nil && escapesRoot(root, real) {
		return warnf("resolves outside the repository (symlink escape) — a drive manifest must live under the repo root")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return warnf("unreadable (%v)", err)
	}
	var m driveManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return warnf("malformed YAML (%v)", err)
	}
	// state gate first. A drive with NO `state:` key is a typo, not a decision:
	// silence would let a mistyped manifest vanish, so it is fail-neutral. A
	// present-but-non-active label (scheduled/retired/…) IS a decision — silent.
	switch strings.TrimSpace(m.State) {
	case "":
		return warnf("no `state:` key — a drive with no declared state is a typo, not a decision; set `state: active` to apply, or a label like `scheduled`/`retired` to intentionally hold it")
	case "active":
		// fall through and validate/apply
	default:
		return nil, "", true
	}
	for _, req := range []struct{ name, val string }{
		{"declared-by", m.DeclaredBy}, {"starts", m.Starts}, {"expires", m.Expires}, {"intensity", m.Intensity},
	} {
		if strings.TrimSpace(req.val) == "" {
			return warnf("required field %q is missing", req.name)
		}
	}
	if _, ok := driveIntensityWeight(m.Intensity); !ok {
		return warnf("intensity %q is not one of focus|push|surge", m.Intensity)
	}
	starts, err := time.Parse("2006-01-02", strings.TrimSpace(m.Starts))
	if err != nil {
		return warnf("starts %q is not a YYYY-MM-DD date", m.Starts)
	}
	expires, err := time.Parse("2006-01-02", strings.TrimSpace(m.Expires))
	if err != nil {
		return warnf("expires %q is not a YYYY-MM-DD date", m.Expires)
	}
	startDay, expiryDay, today := driveDay(starts), driveDay(expires), driveDay(now)
	if expiryDay.Before(startDay) {
		return warnf("expires %s precedes starts %s", m.Expires, m.Starts)
	}
	if expiryDay.After(startDay.AddDate(0, 0, driveExpiryHorizonDays)) {
		return warnf("window %s→%s exceeds the %d-day horizon", m.Starts, m.Expires, driveExpiryHorizonDays)
	}
	// EXPIRED ⇒ fail-neutral WARN + banner (brief-44 names expired explicitly).
	if today.After(expiryDay) {
		return warnf("expired (window %s→%s, today %s)", m.Starts, m.Expires, today.Format("2006-01-02"))
	}
	// Not-yet-started ⇒ silent: a scheduled future campaign is not a fault, and
	// it contributes no boost until its window opens.
	if today.Before(startDay) {
		return nil, "", true
	}
	d := &Drive{
		Slug: slug, DeclaredBy: m.DeclaredBy, Starts: m.Starts, Expires: m.Expires,
		Intensity: m.Intensity, Why: m.Why, State: m.State,
		streamSet: map[string]bool{}, briefSet: map[string]bool{}, streamCaps: map[string]int{},
	}
	for i, it := range m.Items {
		item, ierr := classifyDriveItem(it)
		if ierr != "" {
			return warnf("item %d: %s", i+1, ierr)
		}
		// Resolve stream/brief items against the loaded board. A ref that matches
		// nothing is unresolvable — fail-neutral, not silent. classifyDriveItem
		// validates ref SHAPE; this validates the ref RESOLVES.
		switch item.Kind {
		case "stream":
			if !knownStreams[item.Ref] {
				return warnf("item %d: stream %q resolves to no known stream (a mistyped stream name is unresolvable) — check docs/streams/", i+1, item.Ref)
			}
			d.streamSet[item.Ref] = true
			if item.StreamCap > 0 {
				// Most-restrictive wins if the same stream is named more than once.
				if prev, ok := d.streamCaps[item.Ref]; !ok || item.StreamCap < prev {
					d.streamCaps[item.Ref] = item.StreamCap
				}
			}
		case "brief":
			if !knownBriefs[item.Ref] {
				return warnf("item %d: brief %q resolves to no known brief (a nonexistent or un-zero-padded ref — e.g. `education/7` vs `education/07` — is unresolvable)", i+1, item.Ref)
			}
			d.briefSet[item.Ref] = true
		}
		d.Items = append(d.Items, item)
	}
	return d, "", false
}

// classifyDriveItem resolves one raw item row to exactly one kind, or returns a
// non-empty error string. The kinds are mutually exclusive by which field is set;
// an item matching none (or an ambiguous/ill-formed ref) is a malformed manifest.
func classifyDriveItem(it driveItemYAML) (DriveItem, string) {
	switch {
	case strings.TrimSpace(it.Stream) != "":
		// drive-stream-cap is OPTIONAL (0 = absent). A PRESENT value must be a
		// positive integer — a negative cap is nonsense and would silently widen or
		// zero a stream's draw, so it is fail-neutral (rejects the whole drive),
		// mirroring the `since:` validation above.
		if it.DriveStreamCap < 0 {
			return DriveItem{}, fmt.Sprintf("stream item `drive-stream-cap:` %d is negative — it must be a positive per-stream cap", it.DriveStreamCap)
		}
		return DriveItem{Kind: "stream", Ref: strings.TrimSpace(it.Stream), StreamCap: it.DriveStreamCap}, ""
	case strings.TrimSpace(it.Brief) != "":
		ref := strings.TrimSpace(it.Brief)
		if !briefRefRe.MatchString(ref) {
			return DriveItem{}, fmt.Sprintf("brief ref %q is not <stream>/<NN>", ref)
		}
		return DriveItem{Kind: "brief", Ref: ref}, ""
	case strings.TrimSpace(it.Issue) != "":
		ref := strings.TrimSpace(it.Issue)
		if !fullIssueRefRe.MatchString(ref) {
			return DriveItem{}, fmt.Sprintf("issue ref %q must be a FULL owner/repo#N reference (a bare #N is rejected — cross-repo full-ref enforcement)", ref)
		}
		return DriveItem{Kind: "issue", Ref: ref}, ""
	case strings.TrimSpace(it.Owner) == "operator":
		if strings.TrimSpace(it.Unblocks) == "" {
			return DriveItem{}, "operator-act row (owner: operator) must name what it unblocks (unblocks:)"
		}
		since := strings.TrimSpace(it.Since)
		// `since:` is OPTIONAL (aging falls back to the drive's `starts`), but a
		// PRESENT value must be a real YYYY-MM-DD date — a typo'd date would silently
		// mis-age the operator-act and the @operator escalation that keys on it, so it
		// is validated here and a bad value is fail-neutral (rejects the whole drive).
		if since != "" {
			if _, err := time.Parse("2006-01-02", since); err != nil {
				return DriveItem{}, fmt.Sprintf("operator-act `since:` %q is not a YYYY-MM-DD date", it.Since)
			}
		}
		return DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: strings.TrimSpace(it.Unblocks), Since: since}, ""
	case strings.TrimSpace(it.Needs) != "":
		if strings.TrimSpace(it.Needs) != "brief" {
			return DriveItem{}, fmt.Sprintf("plan-gap row needs: %q — only 'brief' is supported", it.Needs)
		}
		return DriveItem{Kind: "plan-gap", Needs: "brief"}, ""
	}
	return DriveItem{}, "row matches no item kind (expected one of: stream, brief, issue, owner: operator + unblocks, needs: brief)"
}

// scaleForCoverage applies the concave anti-Goodhart self-tax. At or below the
// coverage threshold the full weight stands; above it the per-brief weight scales
// by threshold/frac. Past the threshold the TOTAL boost summed across the covered
// briefs is weight×threshold×(eligible count) — CONSTANT — so the marginal boost
// per added covered brief is zero, not negative; what declines is the per-brief
// term. Either way a drive cannot win by covering the whole board. Integer floor
// (a boost is a whole-number score term).
func scaleForCoverage(weight int, frac float64) int {
	if frac <= driveCoverageThreshold || frac <= 0 {
		return weight
	}
	return int(float64(weight) * driveCoverageThreshold / frac)
}

// driveBoost returns the additive drive term for one brief and the slug of the
// drive that supplied it. Overlapping drives take the MAX term, never the sum
// (brief-44). coverage maps each active drive's slug to its eligible-board
// coverage fraction, so the returned term is already self-taxed.
func (ds DriveSet) driveBoost(streamName, briefNum string, coverage map[string]float64) (int, string) {
	id := streamName + "/" + briefNum
	best, slug := 0, ""
	for _, d := range ds.Active {
		if !d.covers(streamName, id) {
			continue
		}
		w, _ := driveIntensityWeight(d.Intensity)
		w = scaleForCoverage(w, coverage[d.Slug])
		if w > best {
			best, slug = w, d.Slug
		}
	}
	return best, slug
}

// streamDriveCap returns the tightest per-stream drive-stream-cap declared for
// streamName across the active drives, and whether any was declared. nextup.go
// folds it into effectiveCap = min(perStreamCap-or-MaxConcurrent, cap): it can only
// RESTRICT a stream's drive draw, never widen it, and a stream's declared
// MaxConcurrent still always wins. Absent (no active drive caps this stream) ⇒
// (0, false) and the stream keeps its ordinary cap — so a no-manifest board is
// unaffected (byte-identical baseline).
func (ds DriveSet) streamDriveCap(streamName string) (int, bool) {
	best, ok := 0, false
	for _, d := range ds.Active {
		c, has := d.streamCaps[streamName]
		if !has {
			continue
		}
		if !ok || c < best {
			best, ok = c, true
		}
	}
	return best, ok
}

// driveCoverage computes each active drive's coverage fraction over the eligible
// pick set (the anti-Goodhart denominator) plus, for any drive over the
// threshold, a NOTICE line. Returned map is keyed by slug.
func (ds DriveSet) driveCoverage(eligible []Pick) (map[string]float64, []string) {
	cov := map[string]float64{}
	var notices []string
	total := float64(len(eligible))
	if total == 0 {
		return cov, nil
	}
	for _, d := range ds.Active {
		n := 0
		for _, p := range eligible {
			if d.covers(p.Stream.Name, p.Stream.Name+"/"+p.Brief.Num) {
				n++
			}
		}
		frac := float64(n) / total
		cov[d.Slug] = frac
		if frac > driveCoverageThreshold {
			notices = append(notices, fmt.Sprintf(
				"drive %q covers %.0f%% of the eligible board (> %.0f%%) — its boost is self-taxed (concave scale-down) so tagging the whole board buys progressively less (anti-Goodhart, methodology-metrics/45)",
				d.Slug, frac*100, driveCoverageThreshold*100))
		}
	}
	return cov, notices
}

// driveBannerText renders the honesty banner shown whenever a boosted pick is on
// the board. A boosted pick WITHOUT this banner is a --lint PROBLEM
// (driveBannerProblems) — the score must never present a merged, unattributed
// number.
func driveBannerText(ds DriveSet) string {
	parts := make([]string, 0, len(ds.Active))
	for _, d := range ds.Active {
		w, _ := driveIntensityWeight(d.Intensity)
		parts = append(parts, fmt.Sprintf("`%s` (%s, +%d, %s→%s)", d.Slug, d.Intensity, w, d.Starts, d.Expires))
	}
	return fmt.Sprintf("> **ACTIVE DRIVE — the Next-up score below carries an operator-declared steer.** %s. "+
		"Each boosted pick shows its score DECOMPOSED as `base + term (drive:<slug>)` — the term is a temporal, "+
		"attributed steer, ADDITIVE and walled off from every measured metric, never a value claim.",
		strings.Join(parts, "; "))
}

// driveNotAppliedText renders the fail-neutral banner: a manifest existed but was
// rejected, so the board generated with ZERO boost. Never accompanied by an rc≠0.
func driveNotAppliedText(ds DriveSet) string {
	reason := ds.Reason
	if reason == "" {
		reason = "a drive manifest could not be applied"
	}
	return fmt.Sprintf("> **DRIVE NOT APPLIED.** %s. The board generated normally with **zero** drive boost — "+
		"a malformed, expired, or over-concurrent drive manifest is fail-neutral by design and can never freeze "+
		"the board (see `--lint` WARN lines).", reason)
}

// driveBannerProblems is the honesty gate (brief-44 Verify row 3): a boosted
// Next-up pick shown WITHOUT the active-drive banner is a PROBLEM. nextUp sets the
// banner whenever a shown pick is boosted, so this never fires on a correct run —
// it pins that invariant against regression.
func driveBannerProblems(nu NextUp) []string {
	boosted := false
	for _, p := range nu.Picks {
		if p.DriveTerm > 0 {
			boosted = true
			break
		}
	}
	if boosted && nu.DriveBanner == "" {
		return []string{"a boosted Next-up pick is shown without the active-drive banner — the drive term must always be displayed decomposed and attributed, never as a merged number (methodology-metrics/45)"}
	}
	return nil
}
