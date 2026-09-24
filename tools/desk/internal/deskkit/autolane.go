package deskkit

// autolane.go — the AUTO-APPROVE LANE's decision logic: config, category admit, the
// gate-time demotion score, the kill signal, and the audit-log latches.
//
// WHAT THE LANE IS. A narrow lane in which the reviewer App may merge a class of PRs with no
// per-merge human act. It is opened by CATEGORY — every changed path sits inside an area a
// named human opted in, in operator config, and nothing about the change trips a tripwire —
// and it is kept open only while a SCORE, recomputed at every gate the PR passes, finds no
// demotion signal. The score can only EJECT; it never admits. An ejection is one-way for the
// PR (the audit-log latch below), and the lane as a whole disarms itself when its trailing
// first-pass yield drops under a floor.
//
// INERT BY CONSTRUCTION. Every piece of this file fails CLOSED:
//
//   - the four ASSAY_AUTOAPPROVE_* keys absent is the SHIPPED state, and it means the lane is
//     CLOSED. No default value for any of them opens it; a subset set without the rest is a
//     refusal, never a partial lane;
//   - a malformed value, an untrusted opt-in login, an area that overlaps a declared surface,
//     touches a risk-classed path, or can reach a stream brief file refuses the lane;
//   - every forge read the admit or the score consumes is three-state, and an unreadable
//     read FIRES — it is never read as "no signal";
//   - the kill signal's input absent or unreadable is a HOLD, never healthy.
//
// And none of it acts on its own: cmd/deskautolane is the only caller, and that verb's write
// path is additionally gated on a signed ruling line resolved to the blessing authority
// (the enactment gate), which ships UNSIGNED.
//
// WHAT THIS FILE DOES NOT DO. It performs no network read and no write. Every function here
// is a pure judgement over inputs the caller read, so the fail-closed properties above are
// testable without a forge and cannot depend on which forge served the inputs.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The lane's fixed names. Callers key on them — a label, an audit verb, a marker — so each
// is spelled once.
const (
	// AutoLaneLabel is the admission label. A PR carries it only while it is in the lane,
	// and only an application by the reviewer App counts: the same label applied by any
	// other identity reads as NOT admitted (a self-applied admission is no admission).
	AutoLaneLabel = "auto-lane"
	// AutoLaneRulingID is the rulings-register entry whose Sign-off line is the lane's
	// enactment gate.
	AutoLaneRulingID = "R-8"
	// AutoLaneToolName is the tool key the lane's audit lines carry.
	AutoLaneToolName = "deskautolane"
	// The audit verbs. autolane:merge result=ok lines are what the daily cap counts;
	// autolane:eject lines are the per-PR ejection latch.
	AutoLaneVerbEject  = "autolane:eject"
	AutoLaneVerbMerge  = "autolane:merge"
	AutoLaneVerbConfig = "autolane:config"
	AutoLaneVerbCheck  = "autolane:check"
	AutoLaneVerbAdmit  = "autolane:admit"
	// AutoLaneEjectMarker makes the ejection comment idempotent: a PR carries at most one.
	AutoLaneEjectMarker = "<!-- deskautolane:eject -->"
	// AutoLaneKillSignalClass is the class key the kill signal reads from the harvested
	// per-class first-pass-yield file.
	AutoLaneKillSignalClass = "auto-lane"
	// AutoLaneKillSignalMinMerges is the sample floor below which the first-pass yield is
	// could-not-check (the lane then runs on the per-PR ejector and the daily cap alone).
	AutoLaneKillSignalMinMerges = 10
)

// The demotion signals, version 1. Each is 0/1; the score is their count.
const (
	SignalReviewRework     = "review-rework"
	SignalCINonsuccess     = "ci-nonsuccess"
	SignalPushAfterRequest = "push-after-request"
	SignalSizeLarge        = "size-large"
	SignalModelUnstamped   = "model-unstamped"
	SignalUnreadable       = "unreadable"
)

// AutoLaneSignals is the ordered signal set. Its length bounds the eject line: a line at or
// above it could never be exceeded, which would switch the ejector off.
var AutoLaneSignals = []string{
	SignalReviewRework, SignalCINonsuccess, SignalPushAfterRequest,
	SignalSizeLarge, SignalModelUnstamped, SignalUnreadable,
}

// The category tripwires. These are category judgments, never scored: any one of them means
// the PR is not in the lane, whatever the score reads.
const (
	TripAuthorNotWorker = "author-not-worker-app"
	TripNoTrailer       = "no-trailer"
	TripPathOutsideArea = "path-outside-area"
	TripStreamBrief     = "stream-brief-file"
	TripRiskClassed     = "risk-classed"
	TripSurfaceCore     = "surface-core"
	TripSurfaceAbsent   = "surface-absent"
	TripSecurityFail    = "security-review-fail"
	TripNeverAdmit      = "never-admit-path"
	TripUnreadable      = "unreadable"
)

// AutoLaneRulingsPath is the default repo-relative path of the rulings register whose R-8
// Sign-off line is the enactment gate. It sits on a never-admit path (autoLaneNeverAdmitGlobs),
// so no lane merge can ever edit the line that enacts the lane.
const AutoLaneRulingsPath = "docs/streams/issue-flow/rulings.md"

// AutoLaneRulingsGlob is where a rulings register may sit: the enactment gate refuses a
// register path outside it, and it heads the never-admit set.
const AutoLaneRulingsGlob = "docs/streams/**/rulings.md"

// streamBriefGlobs is the compiled never-admit set: a stream brief file is methodology
// state a human signs, and no opt-in may reach it. It is compiled, not configured, so no
// operator entry can widen past it — an area whose glob could match one is refused at load,
// and a changed path matching one trips the category at every gate.
var streamBriefGlobs = []string{
	"docs/streams/**/brief-*.md",
}

// autoLaneNeverAdmitGlobs is the rest of the compiled never-admit set: files that steer the
// lane itself or the agents that work under it, so a lane merge that edited one could widen
// the lane or re-instruct its workers with no human act.
//
//   - a rulings register — the lane's own enactment line lives in one;
//   - `.assay-surfaces` — the surface tier the lane's area rule is checked against;
//   - agent-instruction files: CLAUDE.md, AGENTS.md and SKILL.md at any depth, anything
//     under a `.claude/` directory, and `.mcp.json`.
//
// Like streamBriefGlobs it is compiled, never configured. The per-path check at admit is the
// binding one; the load-time overlap test refuses an area only where its sample expansion
// finds a common path (an area such as `docs/notes/**` is not refused merely because a
// CLAUDE.md could one day appear under it — a PR adding one trips at admit instead).
var autoLaneNeverAdmitGlobs = []string{
	AutoLaneRulingsGlob,
	".assay-surfaces",
	"**/.assay-surfaces",
	"CLAUDE.md",
	"**/CLAUDE.md",
	"AGENTS.md",
	"**/AGENTS.md",
	"**/SKILL.md",
	".claude/**",
	"**/.claude/**",
	".mcp.json",
	"**/.mcp.json",
}

// IsStreamBriefPath reports whether path is a stream brief file.
func IsStreamBriefPath(path string) bool {
	for _, g := range streamBriefGlobs {
		if MatchSurfaceGlob(g, path) {
			return true
		}
	}
	return false
}

// IsAutoLaneNeverAdmitPath reports whether path is on the compiled never-admit set beyond the
// stream brief files (autoLaneNeverAdmitGlobs).
func IsAutoLaneNeverAdmitPath(path string) bool {
	for _, g := range autoLaneNeverAdmitGlobs {
		if MatchSurfaceGlob(g, path) {
			return true
		}
	}
	return false
}

// ConclusionGreen is the ONE accepted set of green check-run conclusions: exactly SUCCESS,
// NEUTRAL and SKIPPED. Every other conclusion — CANCELLED, TIMED_OUT, ACTION_REQUIRED,
// FAILURE, STALE, and anything this reader does not recognise — is NOT green. deskflip's
// checks-green gate delegates here, so the lane's ci-nonsuccess signal and the ready flip
// judge a run by one implementation.
func ConclusionGreen(conclusion string) bool {
	switch strings.ToUpper(strings.TrimSpace(conclusion)) {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------------------

// AutoLaneArea is one opt-in entry: a repo, ONE glob, and the human who opted it in.
type AutoLaneArea struct {
	Repo  string
	Glob  string
	Login string
}

// String renders the entry as the operator wrote it, for refusal messages.
func (a AutoLaneArea) String() string { return a.Repo + ":" + a.Glob + ":" + a.Login }

// AutoLaneConfig is a loaded, validated lane configuration.
type AutoLaneConfig struct {
	Areas     []AutoLaneArea
	EjectLine int
	FPYFloor  float64
	DailyCap  int
}

// AreasFor returns the entries for repo (case-insensitive slug compare).
func (c AutoLaneConfig) AreasFor(repo string) []AutoLaneArea {
	var out []AutoLaneArea
	for _, a := range c.Areas {
		if strings.EqualFold(a.Repo, repo) {
			out = append(out, a)
		}
	}
	return out
}

// AutoLaneConfigState is the three-state outcome of loading the lane config.
type AutoLaneConfigState int

const (
	// AutoLaneUnconfigured — every lane key is absent. The shipped state: the lane is
	// CLOSED. Not an error, and never an opening.
	AutoLaneUnconfigured AutoLaneConfigState = iota
	// AutoLaneConfigRefused — a key is malformed, a subset is set, or a rule refused an
	// entry. The lane is CLOSED, loudly.
	AutoLaneConfigRefused
	// AutoLaneConfigLoaded — the four keys parsed and every load-time rule held.
	AutoLaneConfigLoaded
)

func (s AutoLaneConfigState) String() string {
	switch s {
	case AutoLaneConfigLoaded:
		return "loaded"
	case AutoLaneConfigRefused:
		return "refused"
	default:
		return "unconfigured"
	}
}

// AutoLaneLoad is the result of ParseAutoLaneConfig. Problem is set exactly when State is
// AutoLaneConfigRefused, and always names the key or entry that refused.
type AutoLaneLoad struct {
	State   AutoLaneConfigState
	Config  AutoLaneConfig
	Problem string
}

// AutoLaneValidator carries the two roster predicates the config load consults. They are
// injected so the parser stays a pure function; DefaultAutoLaneValidator binds the
// roster's own answers.
type AutoLaneValidator struct {
	// RepoAllowed answers "is this repo in the write-authorisation set?".
	RepoAllowed func(repo string) bool
	// LoginMayOptIn answers "is this login the blessing authority or a trusted human?".
	LoginMayOptIn func(login string) bool
}

// DefaultAutoLaneValidator binds the roster: a repo must be in ASSAY_ALLOWED_REPOS, and an
// opt-in login must be the blessing authority or a trusted HUMAN (never an App — an area a
// bot opted in is an area nobody accountable opted in).
func DefaultAutoLaneValidator() AutoLaneValidator {
	return AutoLaneValidator{
		RepoAllowed: IsAllowedRepo,
		LoginMayOptIn: func(login string) bool {
			if looksLikeBot(login) {
				return false
			}
			return IsBlessAuthority(login) || TrustedHumanAuthor(login)
		},
	}
}

// Bounds on the three numbers. The eject line is bounded ABOVE by the signal count: a line
// the score can never exceed switches the ejector off, and that is a lane with no score.
const (
	autoLaneMaxDailyCap = 100
)

var autoLaneRepoSlugRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// LoadAutoLaneConfig reads the lane keys from the process roster and parses them. An
// unconfigured or refused ROSTER closes the lane too: an opt-in that cannot be checked
// against a trust roster is not an opt-in.
func LoadAutoLaneConfig() AutoLaneLoad {
	cfg := EffectiveConfig()
	if len(cfg.AutoLaneRaw) == 0 {
		return AutoLaneLoad{State: AutoLaneUnconfigured}
	}
	if !cfg.Configured() {
		return AutoLaneLoad{State: AutoLaneConfigRefused,
			Problem: "the trust roster is not configured, so no opt-in login can be checked — the lane is closed"}
	}
	return ParseAutoLaneConfig(cfg.AutoLaneRaw, DefaultAutoLaneValidator())
}

// ParseAutoLaneConfig parses and validates the four raw lane values. It is pure: the roster
// predicates arrive through v. See the file header for the fail-closed rules it enforces.
func ParseAutoLaneConfig(raw map[string]string, v AutoLaneValidator) AutoLaneLoad {
	get := func(k string) string { return strings.TrimSpace(raw[k]) }
	set := 0
	for _, k := range autoLaneKeys() {
		if get(k) != "" {
			set++
		}
	}
	if set == 0 {
		return AutoLaneLoad{State: AutoLaneUnconfigured}
	}
	refuse := func(format string, a ...any) AutoLaneLoad {
		return AutoLaneLoad{State: AutoLaneConfigRefused, Problem: fmt.Sprintf(format, a...)}
	}
	if set != len(autoLaneKeys()) {
		var missing []string
		for _, k := range autoLaneKeys() {
			if get(k) == "" {
				missing = append(missing, k)
			}
		}
		return refuse("refused: partial lane config — %s unset. The lane opens only with all four keys set; "+
			"a subset is never read as a partial lane", strings.Join(missing, ", "))
	}

	var c AutoLaneConfig

	line, err := strconv.Atoi(get(EnvAutoApproveEjectLine))
	if err != nil || line < 0 || line >= len(AutoLaneSignals) {
		return refuse("refused: %s=%q must be an integer in [0, %d] — a line at or above the signal count "+
			"could never be exceeded, which switches the ejector off", EnvAutoApproveEjectLine,
			get(EnvAutoApproveEjectLine), len(AutoLaneSignals)-1)
	}
	c.EjectLine = line

	floor, err := strconv.ParseFloat(get(EnvAutoApproveFPYFloor), 64)
	if err != nil || math.IsNaN(floor) || math.IsInf(floor, 0) || floor <= 0 || floor > 1 {
		return refuse("refused: %s=%q must be a decimal in (0, 1] — a floor of 0 never fires, which "+
			"switches the kill signal off", EnvAutoApproveFPYFloor, get(EnvAutoApproveFPYFloor))
	}
	c.FPYFloor = floor

	capN, err := strconv.Atoi(get(EnvAutoApproveDailyCap))
	if err != nil || capN < 1 || capN > autoLaneMaxDailyCap {
		return refuse("refused: %s=%q must be an integer in [1, %d]", EnvAutoApproveDailyCap,
			get(EnvAutoApproveDailyCap), autoLaneMaxDailyCap)
	}
	c.DailyCap = capN

	seen := map[string]bool{}
	for _, entry := range splitList(get(EnvAutoApproveAreas)) {
		a, perr := parseAutoLaneArea(entry)
		if perr != nil {
			return refuse("refused: %s entry %q: %v", EnvAutoApproveAreas, StripControl(entry), perr)
		}
		if v.RepoAllowed == nil || !v.RepoAllowed(a.Repo) {
			return refuse("refused: area repo not allowed: %s entry %q names %s, which is not in %s",
				EnvAutoApproveAreas, a, a.Repo, EnvAllowedRepos)
		}
		if v.LoginMayOptIn == nil || !v.LoginMayOptIn(a.Login) {
			return refuse("refused: opt-in login not trusted: %s entry %q names %s, which is neither the "+
				"blessing authority nor a trusted human — an area only a human may opt in",
				EnvAutoApproveAreas, a, StripControl(a.Login))
		}
		if seen[strings.ToLower(a.String())] {
			continue
		}
		seen[strings.ToLower(a.String())] = true
		c.Areas = append(c.Areas, a)
	}
	if len(c.Areas) == 0 {
		return refuse("refused: %s names no entries", EnvAutoApproveAreas)
	}
	return AutoLaneLoad{State: AutoLaneConfigLoaded, Config: c}
}

// parseAutoLaneArea splits `<owner>/<repo>:<glob>:<login>`. The repo is everything before
// the FIRST colon and the login everything after the LAST, so the glob is the middle and a
// glob can never swallow a login or a repo.
func parseAutoLaneArea(entry string) (AutoLaneArea, error) {
	first := strings.Index(entry, ":")
	last := strings.LastIndex(entry, ":")
	if first < 0 || last == first {
		return AutoLaneArea{}, errors.New("want <owner>/<repo>:<glob>:<login>")
	}
	a := AutoLaneArea{
		Repo:  strings.TrimSpace(entry[:first]),
		Glob:  strings.TrimSpace(entry[first+1 : last]),
		Login: strings.TrimSpace(entry[last+1:]),
	}
	if !autoLaneRepoSlugRe.MatchString(a.Repo) {
		return AutoLaneArea{}, fmt.Errorf("repo %q is not a full owner/name slug (a pattern is never an area)", a.Repo)
	}
	if a.Login == "" || strings.ContainsAny(a.Login, " \t/") {
		return AutoLaneArea{}, fmt.Errorf("login %q is not a login", a.Login)
	}
	if err := validateAutoLaneGlob(a.Glob); err != nil {
		return AutoLaneArea{}, err
	}
	return a, nil
}

// validateAutoLaneGlob admits exactly the `.assay-surfaces` glob subset MatchSurfaceGlob
// implements — `*` within a segment, `**` as a whole segment, everything else literal — and
// refuses anything outside it rather than guessing what it meant. It also refuses a glob
// that names no directory at all (`*`, `**`, `**/x`): an area that starts matching at the
// repo root is an area nobody bounded.
func validateAutoLaneGlob(g string) error {
	if g == "" {
		return errors.New("empty glob")
	}
	if strings.HasPrefix(g, "/") || strings.HasPrefix(g, "!") {
		return fmt.Errorf("glob %q: a leading '/' or '!' is outside the .assay-surfaces subset", g)
	}
	if strings.ContainsAny(g, "?[]{}\\ \t,") {
		return fmt.Errorf("glob %q uses syntax outside the .assay-surfaces subset (only '*' and a whole-segment '**')", g)
	}
	segs := strings.Split(strings.TrimSuffix(g, "/"), "/")
	for _, s := range segs {
		switch {
		case s == "" || s == "." || s == "..":
			return fmt.Errorf("glob %q has an empty or relative segment", g)
		case strings.Contains(s, "**") && s != "**":
			return fmt.Errorf("glob %q: '**' is only valid as a whole segment", g)
		}
	}
	if strings.Contains(segs[0], "*") {
		return fmt.Errorf("glob %q starts matching at the repo root — an area must name a literal first segment", g)
	}
	if strings.Count(g, "**") > 3 {
		return fmt.Errorf("glob %q carries more than three '**' segments", g)
	}
	return nil
}

// GlobSamples expands a glob into representative concrete paths: each `**` becomes zero,
// one and two segments, and each `*` becomes a filler and (where the segment survives) the
// empty string. It is the sample-expansion the load-time overlap test runs, so two globs
// that can match a common path are caught without a repository tree to walk. The expansion
// is bounded; validateAutoLaneGlob caps the `**` count that drives it.
func GlobSamples(g string) []string {
	segs := strings.Split(strings.Trim(g, "/"), "/")
	type alt [][]string
	var alts []alt
	for _, s := range segs {
		switch {
		case s == "**":
			alts = append(alts, alt{{}, {"x"}, {"x", "y"}})
		case strings.Contains(s, "*"):
			filled := strings.ReplaceAll(s, "*", "x")
			emptied := strings.ReplaceAll(s, "*", "")
			a := alt{{filled}}
			if emptied != "" && emptied != filled {
				a = append(a, []string{emptied})
			}
			alts = append(alts, a)
		default:
			alts = append(alts, alt{{s}})
		}
	}
	out := [][]string{{}}
	for _, a := range alts {
		var next [][]string
		for _, prefix := range out {
			for _, choice := range a {
				p := append(append([]string{}, prefix...), choice...)
				next = append(next, p)
			}
		}
		out = next
		if len(out) > 512 {
			out = out[:512]
		}
	}
	seen := map[string]bool{}
	var paths []string
	for _, p := range out {
		s := strings.Join(p, "/")
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		paths = append(paths, s)
	}
	return paths
}

// globsOverlap reports whether two globs can match a common path, by sample expansion in
// BOTH directions, and returns the witness path.
func globsOverlap(a, b string) (string, bool) {
	for _, s := range GlobSamples(a) {
		if MatchSurfaceGlob(b, s) {
			return s, true
		}
	}
	for _, s := range GlobSamples(b) {
		if MatchSurfaceGlob(a, s) {
			return s, true
		}
	}
	return "", false
}

// ValidateAutoLaneRepo runs the load-time rules that need the repo's own declared surfaces:
// every area entry for repo must overlap NO `.assay-surfaces` glob, match NO path the risk
// classifier classes, and be unable to reach a stream brief file. surfacesPresent false (the
// repo declares no `.assay-surfaces`) refuses: the surface tier is three-state, and "no
// declared surfaces" is never read as safe. A repo with no area entry has nothing to
// validate and returns "".
//
// It returns the refusal text, or "" when every entry for repo holds. The rules run in a
// fixed order — surface overlap, risk, brief — so a glob that fails several is refused
// under the first, deterministically.
func ValidateAutoLaneRepo(c AutoLaneConfig, repo string, surfacesPresent bool, surfaceGlobs []string,
	riskClassed func(repo string, paths []string) bool) string {
	if len(c.AreasFor(repo)) == 0 {
		return ""
	}
	if !surfacesPresent {
		return fmt.Sprintf("refused: area repo declares no surfaces: %s has no .assay-surfaces file on its base "+
			"branch, so the surface tier is absent and the lane admits nothing there", repo)
	}
	if p := AutoLaneSurfaceOverlap(c, repo, surfaceGlobs); p != "" {
		return p
	}
	return AutoLaneAreaTripwires(c, repo, riskClassed)
}

// AutoLaneSurfaceOverlap refuses any area entry for repo whose glob can match a path one of
// surfaceGlobs matches. It is ONLY-NARROWING by construction — it can refuse, never admit —
// so it is safe to run against any copy of `.assay-surfaces`, including a local one a caller
// could edit: an edit can only add refusals.
func AutoLaneSurfaceOverlap(c AutoLaneConfig, repo string, surfaceGlobs []string) string {
	for _, a := range c.AreasFor(repo) {
		for _, sg := range surfaceGlobs {
			if w, ok := globsOverlap(a.Glob, sg); ok {
				return fmt.Sprintf("refused: area overlaps declared surface: entry %q overlaps .assay-surfaces glob "+
					"%q (both match %q)", a, sg, w)
			}
		}
	}
	return ""
}

// AutoLaneAreaTripwires refuses any area entry for repo whose glob can match a path the
// risk classifier classes, or a stream brief file. Needs no repository read.
func AutoLaneAreaTripwires(c AutoLaneConfig, repo string, riskClassed func(repo string, paths []string) bool) string {
	areas := c.AreasFor(repo)
	for _, a := range areas {
		for _, s := range GlobSamples(a.Glob) {
			if riskClassed == nil || riskClassed(repo, []string{s}) {
				return fmt.Sprintf("refused: area matches a risk-classed path: entry %q matches %q, which the risk "+
					"classifier classes (or could not classify) for %s", a, s, repo)
			}
		}
	}
	for _, a := range areas {
		for _, bg := range streamBriefGlobs {
			if w, ok := globsOverlap(a.Glob, bg); ok {
				return fmt.Sprintf("refused: area can admit a stream brief file: entry %q matches %q", a, w)
			}
		}
	}
	for _, a := range areas {
		for _, ng := range autoLaneNeverAdmitGlobs {
			if w, ok := globsOverlap(a.Glob, ng); ok {
				return fmt.Sprintf("refused: area can admit a never-admit path: entry %q matches %q "+
					"(a rulings register, .assay-surfaces or an agent-instruction file)", a, w)
			}
		}
		// The rulings register is the one never-admit file whose edit could ENACT the lane, so
		// its load-time test does not rely on sampling alone: every directory the area's own
		// samples reach is also probed with a register file in it.
		for _, s := range append([]string{""}, GlobSamples(a.Glob)...) {
			w := strings.TrimPrefix(s+"/rulings.md", "/")
			if MatchSurfaceGlob(a.Glob, w) && MatchSurfaceGlob(AutoLaneRulingsGlob, w) {
				return fmt.Sprintf("refused: area can admit a never-admit path: entry %q matches %q "+
					"(a rulings register — a lane merge could edit its own enactment line)", a, w)
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------------------------------
// Category admit
// ---------------------------------------------------------------------------------------

// AutoLaneAdmitInput is everything the category judgment reads, already read by the caller.
// Each *Err field is the three-state half of its read: non-nil means the read failed, and a
// failed read TRIPS the category (unreadable), never passes it.
type AutoLaneAdmitInput struct {
	Repo string
	// AuthorLogin is the PR author as the forge rendered it; WorkerAppLogin is the roster's
	// worker-role App login ("" when the role is unbound, which trips unreadable).
	AuthorLogin    string
	WorkerAppLogin string
	Body           string
	// ChangedFiles is every path the change touches at the current head — for a rename,
	// BOTH the old and the new path.
	ChangedFiles []string
	FilesErr     error
	Labels       []string
	// Reviews is the full reviews array (every head); a Security-Review fail anywhere trips.
	Reviews    []Review
	ReviewsErr error
	// SurfacesPresent / SurfaceGlobs are the repo's declared `.assay-surfaces` read from its
	// BASE branch (never from a caller's worktree); SurfacesErr is that read failing.
	SurfacesPresent bool
	SurfaceGlobs    []string
	SurfacesErr     error
	// RiskClassed is the risk classifier (RiskPathTriggered in production).
	RiskClassed func(repo string, paths []string) bool
}

// AutoLaneAdmit is the category verdict. Tripwires names every tripwire that fired, in a
// fixed order; Admitted is true exactly when it is empty.
type AutoLaneAdmit struct {
	Admitted  bool
	Tripwires []string
	Details   []string
}

// AdmitAutoLane is the category judgment. It runs EVERY tripwire and reports all that fired,
// so the ejection names the whole reason rather than the first one found.
func AdmitAutoLane(c AutoLaneConfig, in AutoLaneAdmitInput) AutoLaneAdmit {
	var res AutoLaneAdmit
	fired := map[string]bool{}
	trip := func(name, detail string) {
		if !fired[name] {
			fired[name] = true
			res.Tripwires = append(res.Tripwires, name)
		}
		res.Details = append(res.Details, name+": "+detail)
	}

	if in.WorkerAppLogin == "" {
		trip(TripUnreadable, "the roster binds no worker-role App, so the author cannot be checked")
	} else if !strings.EqualFold(strings.TrimSpace(in.AuthorLogin), strings.TrimSpace(in.WorkerAppLogin)) {
		trip(TripAuthorNotWorker, fmt.Sprintf("author %s is not the roster's worker App", StripControl(in.AuthorLogin)))
	}

	if trs, err := ParseTrailers([]byte(in.Body)); err != nil || len(trs) == 0 {
		trip(TripNoTrailer, "the PR body carries no single Brief: or Issue: trailer")
	}

	files := in.ChangedFiles
	switch {
	case in.FilesErr != nil:
		trip(TripUnreadable, "the changed-file list could not be read")
	case len(files) == 0:
		trip(TripUnreadable, "the change reports no changed files — nothing to judge is never in scope")
	}
	areas := c.AreasFor(in.Repo)
	for _, f := range files {
		if strings.TrimSpace(f) == "" {
			trip(TripUnreadable, "a blank path in the changed-file list")
			continue
		}
		if IsStreamBriefPath(f) {
			trip(TripStreamBrief, f)
		}
		if IsAutoLaneNeverAdmitPath(f) {
			trip(TripNeverAdmit, f)
		}
		inArea := false
		for _, a := range areas {
			if MatchSurfaceGlob(a.Glob, f) {
				inArea = true
				break
			}
		}
		if !inArea {
			trip(TripPathOutsideArea, f)
		}
	}
	if len(files) > 0 && in.FilesErr == nil {
		if in.RiskClassed == nil || in.RiskClassed(in.Repo, files) {
			trip(TripRiskClassed, "the risk classifier classes this diff (or could not classify it)")
		}
	}

	switch {
	case in.SurfacesErr != nil:
		trip(TripUnreadable, "the repo's .assay-surfaces could not be read")
	case !in.SurfacesPresent:
		trip(TripSurfaceAbsent, "the repo declares no .assay-surfaces — no declared surfaces is never read as safe")
	default:
		if st, globs := ClassifySurface(true, in.SurfaceGlobs, files); st == SurfaceCore {
			trip(TripSurfaceCore, strings.Join(globs, ", "))
		}
	}
	for _, l := range in.Labels {
		if strings.EqualFold(strings.TrimSpace(l), SurfaceCoreLabel) {
			trip(TripSurfaceCore, "the PR carries "+SurfaceCoreLabel)
		}
	}

	if in.ReviewsErr != nil {
		trip(TripUnreadable, "the reviews could not be read")
	} else {
		for _, r := range in.Reviews {
			if HasSecurityReviewFail(r.Body) {
				trip(TripSecurityFail, "a Security-Review: fail verdict stands in the reviews array")
				break
			}
		}
	}

	res.Admitted = len(res.Tripwires) == 0
	return res
}

// ---------------------------------------------------------------------------------------
// Score
// ---------------------------------------------------------------------------------------

// AutoLaneScoreInput is everything the demotion score reads, already read by the caller at
// the current head.
type AutoLaneScoreInput struct {
	Head string
	// Reviews is the FULL reviews array, every head, every identity — never a latest-per-
	// reviewer view, which would launder a superseded CHANGES_REQUESTED.
	Reviews    []Review
	ReviewsErr error
	// Checks is the check rollup at Head.
	Checks    *ChecksAtHead
	ChecksErr error
	Labels    []string
	// Model is the attested model-stamp state (AttestedModelStampOf); ModelErr is the label
	// timeline read failing.
	Model    ModelState
	ModelErr error
	// Events is the label timeline (the read ModelErr belongs to) and ReviewerLogin the
	// roster's reviewer-role App login. The size signal reads its label only when the
	// reviewer App — the identity that computes and applies it at verdict time — is the one
	// that applied it: a size label the PR's author set or swapped is no size reading.
	Events        []LabelEvent
	ReviewerLogin string
}

// AutoLaneScore is one recompute's result. Fired lists the fired signals in AutoLaneSignals
// order; Score is their count; Unreadable names every read that failed.
type AutoLaneScore struct {
	Fired      []string
	Score      int
	Unreadable []string
	CIDetail   string
}

// Ejects reports whether the score is above line.
func (s AutoLaneScore) Ejects(line int) bool { return s.Score > line }

// ScoreAutoLane recomputes the demotion score. Every signal reads its input live from what
// the caller passed; an unreadable input fires `unreadable` AND is named, so a failed read
// can never be what makes a score read clean.
func ScoreAutoLane(in AutoLaneScoreInput) AutoLaneScore {
	fired := map[string]bool{}
	var unreadable []string
	var ciDetail string

	if in.ReviewsErr != nil {
		unreadable = append(unreadable, "reviews")
	} else {
		for _, r := range in.Reviews {
			if !strings.EqualFold(strings.TrimSpace(r.State), "CHANGES_REQUESTED") {
				continue
			}
			fired[SignalReviewRework] = true
			// A push after the request: the request was made at a commit that is no longer
			// the head. A request carrying no commit cannot be placed, so it counts.
			if in.Head == "" || strings.TrimSpace(r.CommitID) != in.Head {
				fired[SignalPushAfterRequest] = true
			}
		}
	}

	if in.ChecksErr != nil || in.Checks == nil {
		unreadable = append(unreadable, "checks")
	} else {
		state, detail := EvalChecksAtHead(in.Checks)
		switch state {
		case ChecksGreen:
		case ChecksTruncated, ChecksPending, ChecksEmpty:
			// Not yet decided is could-not-check, never a demotion: a PR whose CI is still
			// running (or has not reported) must not be ejected one-way for it. It does not
			// merge either — `unreadable` fires, and the merge step's checks-green re-read
			// refuses the same state could-not-check.
			unreadable = append(unreadable, "checks")
			ciDetail = detail
		default:
			fired[SignalCINonsuccess] = true
			ciDetail = detail
		}
	}

	// size-large. A size:L label FIRES whoever applied it — the only-narrowing direction.
	// Any other reading must be positively established: exactly one size label, applied by
	// the reviewer App. An absent label (the labeler has not run), several, or one applied by
	// any other identity is could-not-check, so removing or downgrading the label can never
	// read as "not large".
	var sizes []string
	for _, l := range in.Labels {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(strings.ToLower(l), SizeLabelPrefix) {
			sizes = append(sizes, l)
		}
		if strings.EqualFold(l, SizeLabelPrefix+"L") {
			fired[SignalSizeLarge] = true
		}
	}
	if !fired[SignalSizeLarge] {
		switch applier, ok := LabelApplier(in.Labels, in.Events, firstOr(sizes, "")); {
		case len(sizes) != 1, in.ModelErr != nil, !ok,
			strings.TrimSpace(in.ReviewerLogin) == "",
			!strings.EqualFold(strings.TrimSpace(applier), strings.TrimSpace(in.ReviewerLogin)):
			unreadable = append(unreadable, "size-label")
		}
	}

	if in.ModelErr != nil {
		unreadable = append(unreadable, "model-stamp")
	} else if in.Model == ModelUnknown || in.Model == ModelIndeterminate {
		fired[SignalModelUnstamped] = true
	}

	if in.Head == "" {
		unreadable = append(unreadable, "head")
	}
	if len(unreadable) > 0 {
		fired[SignalUnreadable] = true
	}

	var s AutoLaneScore
	for _, name := range AutoLaneSignals {
		if fired[name] {
			s.Fired = append(s.Fired, name)
		}
	}
	s.Score = len(s.Fired)
	s.Unreadable = unreadable
	s.CIDetail = ciDetail
	return s
}

// ChecksState is the reduced state of a check rollup.
type ChecksState int

const (
	ChecksGreen ChecksState = iota
	ChecksNotGreen
	ChecksPending
	ChecksEmpty
	ChecksTruncated
)

type laneRollupEntry struct {
	name, recency string
	run           *CheckRun
	status        *StatusContext
}

// EvalChecksAtHead reduces a rollup to one state, judging ONLY the latest run per check name
// (and the latest state per status context) — LatestRunPerName, the same reduction deskflip
// and deskboard use — so a check re-run several times at one head reads by its own latest
// run, never by the mere presence of an earlier superseded one. Green is exactly: every
// reduced check run COMPLETED with a ConclusionGreen conclusion and every reduced status
// context `success`. A pending run, a non-green conclusion (CANCELLED included), a
// non-success status, an EMPTY rollup (nothing is positively green) or a rollup whose
// advertised totals exceed what it served (TRUNCATED) is not green.
func EvalChecksAtHead(c *ChecksAtHead) (ChecksState, string) {
	if c == nil {
		return ChecksTruncated, "no rollup"
	}
	if c.CheckRunsTotalCount > len(c.CheckRuns) || c.StatusTotalCount > len(c.Statuses) {
		return ChecksTruncated, fmt.Sprintf("the rollup advertises %d run(s)/%d status(es) but served %d/%d",
			c.CheckRunsTotalCount, c.StatusTotalCount, len(c.CheckRuns), len(c.Statuses))
	}
	var entries []laneRollupEntry
	for i := range c.CheckRuns {
		r := &c.CheckRuns[i]
		rec := r.CompletedAt
		if rec == "" {
			rec = r.StartedAt
		}
		entries = append(entries, laneRollupEntry{name: "run:" + r.Name, recency: rec, run: r})
	}
	for i := range c.Statuses {
		s := &c.Statuses[i]
		entries = append(entries, laneRollupEntry{name: "status:" + s.Context, recency: s.CreatedAt, status: s})
	}
	if len(entries) == 0 {
		return ChecksEmpty, "no check run or status reported at this head — nothing is positively green"
	}
	reduced := LatestRunPerName(entries,
		func(e laneRollupEntry) string {
			if e.name == "run:" || e.name == "status:" {
				return ""
			}
			return e.name
		},
		func(e laneRollupEntry) string { return e.recency })
	var bad, pending []string
	for _, e := range reduced {
		switch {
		case e.run != nil:
			if !strings.EqualFold(strings.TrimSpace(e.run.Status), "COMPLETED") {
				pending = append(pending, e.run.Name)
			} else if !ConclusionGreen(e.run.Conclusion) {
				bad = append(bad, e.run.Name+"="+strings.ToLower(e.run.Conclusion))
			}
		case e.status != nil:
			switch strings.ToUpper(strings.TrimSpace(e.status.State)) {
			case "SUCCESS":
			case "PENDING", "EXPECTED":
				pending = append(pending, e.status.Context)
			default:
				bad = append(bad, e.status.Context+"="+strings.ToLower(e.status.State))
			}
		}
	}
	if len(bad) > 0 {
		return ChecksNotGreen, "not green: " + strings.Join(bad, ", ")
	}
	if len(pending) > 0 {
		return ChecksPending, "still pending: " + strings.Join(pending, ", ")
	}
	return ChecksGreen, ""
}

// ---------------------------------------------------------------------------------------
// Kill signal
// ---------------------------------------------------------------------------------------

// KillSignalState is the lane-health reading.
type KillSignalState int

const (
	// KillSignalHold — the lane is disarmed: below the floor with enough merges, or the
	// input could not be read. The ZERO VALUE, so a reading nobody made is a hold.
	KillSignalHold KillSignalState = iota
	// KillSignalEarly — fewer than AutoLaneKillSignalMinMerges lane merges in the window: the
	// first-pass yield is could-not-check and the lane runs on the per-PR ejector and the
	// daily cap alone.
	KillSignalEarly
	// KillSignalHealthy — enough merges, and the first-pass yield is at or above the floor.
	KillSignalHealthy
)

// AutoLaneKillSignal is one reading of the kill signal. Line is the operator-facing
// rendering (`lane: hold (…)` / `lane: early (…)` / `lane: healthy (…)`).
type AutoLaneKillSignal struct {
	State KillSignalState
	N     int
	FPY   float64
	Line  string
}

// Armed reports whether the reading lets the lane run.
func (k AutoLaneKillSignal) Armed() bool { return k.State != KillSignalHold }

// ReadAutoLaneKillSignal reads the harvested per-class first-pass-yield file at path and
// evaluates the lane's class against floor. An empty path, an absent or unreadable file, or
// a file that does not parse is a HOLD (could-not-check) — never healthy.
func ReadAutoLaneKillSignal(path string, floor float64) AutoLaneKillSignal {
	if strings.TrimSpace(path) == "" {
		return killHold("could-not-check")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return killHold("could-not-check")
	}
	return EvalAutoLaneKillSignal(data, floor)
}

func killHold(why string) AutoLaneKillSignal {
	return AutoLaneKillSignal{State: KillSignalHold, Line: "lane: hold (" + why + ")"}
}

// EvalAutoLaneKillSignal evaluates the per-class file's content. The file is a JSON object
// keyed by class; the lane's class carries `n` (lane merges in the window) and
// `firstPassYield` (a number, or the string "could-not-check" when n is 0).
func EvalAutoLaneKillSignal(data []byte, floor float64) AutoLaneKillSignal {
	var classes map[string]json.RawMessage
	if err := json.Unmarshal(data, &classes); err != nil {
		return killHold("could-not-check")
	}
	raw, ok := classes[AutoLaneKillSignalClass]
	if !ok {
		return killHold("could-not-check")
	}
	var cls struct {
		N   *int            `json:"n"`
		FPY json.RawMessage `json:"firstPassYield"`
	}
	if err := json.Unmarshal(raw, &cls); err != nil || cls.N == nil || *cls.N < 0 {
		return killHold("could-not-check")
	}
	n := *cls.N
	if n < AutoLaneKillSignalMinMerges {
		return AutoLaneKillSignal{State: KillSignalEarly, N: n,
			Line: fmt.Sprintf("lane: early (n=%d < %d; fpy could-not-check — the per-PR ejector and the daily cap bound the lane)",
				n, AutoLaneKillSignalMinMerges)}
	}
	var fpy float64
	if err := json.Unmarshal(cls.FPY, &fpy); err != nil || math.IsNaN(fpy) || fpy < 0 || fpy > 1 {
		return killHold(fmt.Sprintf("could-not-check, n=%d", n))
	}
	if fpy < floor {
		return AutoLaneKillSignal{State: KillSignalHold, N: n, FPY: fpy,
			Line: fmt.Sprintf("lane: hold (fpy %.2f < floor %.2f, n=%d)", fpy, floor, n)}
	}
	return AutoLaneKillSignal{State: KillSignalHealthy, N: n, FPY: fpy,
		Line: fmt.Sprintf("lane: healthy (fpy %.2f >= floor %.2f, n=%d)", fpy, floor, n)}
}

// ---------------------------------------------------------------------------------------
// Audit-log latches
// ---------------------------------------------------------------------------------------

func isAutoLaneEntry(e Entry, verb, repo string) bool {
	return CanonicalToolKeyOr(e.Tool) == AutoLaneToolName && e.Verb == verb && strings.EqualFold(e.Repo, repo)
}

// AutoLaneMergesOn counts the lane merges recorded for repo on day's UTC date — the daily
// cap's count. Only `autolane:merge result=ok` lines count; the log is append-only, so no
// defect in a later run can lower it. A line whose timestamp does not parse COUNTS (the
// fail-closed direction for a cap).
func AutoLaneMergesOn(entries []Entry, repo string, day time.Time) int {
	want := day.UTC().Format("2006-01-02")
	n := 0
	for _, e := range entries {
		if !isAutoLaneEntry(e, AutoLaneVerbMerge, repo) || e.Result != ResultOK {
			continue
		}
		ts, err := time.Parse(time.RFC3339, e.TS)
		if err != nil || ts.UTC().Format("2006-01-02") == want {
			n++
		}
	}
	return n
}

// AutoLanePriorEjection reports whether an ejection is recorded for repo#pr — the one-way
// latch. ANY autolane:eject line for the PR counts, whatever its result: an ejection that
// was decided but whose write failed is still a PR the lane decided against, and the latch
// errs toward the human queue.
func AutoLanePriorEjection(entries []Entry, repo string, pr int) bool {
	for _, e := range entries {
		if isAutoLaneEntry(e, AutoLaneVerbEject, repo) && e.PR != nil && *e.PR == pr {
			return true
		}
	}
	return false
}

// AutoLaneEjectedOnForge reports whether the PR's own thread carries the ejection comment —
// the FORGE half of the one-way latch. The audit-log line is local to one host and one HOME;
// the marked comment is visible to every host, so a second host, a fresh HOME or a rotated
// ledger still sees the ejection. Only a comment authored by the reviewer App (the identity
// that performs ejections) counts; reviewerLogin "" matches nothing.
func AutoLaneEjectedOnForge(comments []Comment, reviewerLogin string) bool {
	reviewerLogin = strings.TrimSpace(reviewerLogin)
	if reviewerLogin == "" {
		return false
	}
	for _, c := range comments {
		if strings.Contains(c.Body, AutoLaneEjectMarker) && strings.EqualFold(strings.TrimSpace(c.Author.Login), reviewerLogin) {
			return true
		}
	}
	return false
}

// autoLaneEnactRe is the acceptance the enactment gate requires in the sign-off artifact's
// body: a line reading exactly `Enact: R-8`, alone on its line. A body that merely NAMES the
// ruling, thanks someone, or discusses it is no acceptance.
var autoLaneEnactRe = regexp.MustCompile(`(?mi)^[ \t]*Enact:[ \t]*` + regexp.QuoteMeta(AutoLaneRulingID) + `[ \t\r]*$`)

// autoLaneNegationRe voids an acceptance line: a body that also rejects, negates, revokes or
// withdraws is ambiguous, and an ambiguous artifact is not an authorization.
var autoLaneNegationRe = regexp.MustCompile(`(?i)\b(reject(s|ed|ing)?|not[ \t]+(accepted|approved|enacted)|do[ \t]+not|don't|revok(e|es|ed|ing)|withdraw(s|n)?|declin(e|es|ed)|veto(es|ed)?|rescind(s|ed)?)\b`)

// AutoLaneEnactLine is the exact line the sign-off artifact must carry.
const AutoLaneEnactLine = "Enact: " + AutoLaneRulingID

// AutoLaneAcceptance judges a sign-off artifact's BODY: it enacts only when it carries the
// AutoLaneEnactLine alone on a line AND no rejection or negation anywhere. why names the
// failing half; it is "" exactly when ok.
func AutoLaneAcceptance(body string) (ok bool, why string) {
	if !autoLaneEnactRe.MatchString(body) {
		return false, "the artifact carries no line reading exactly `" + AutoLaneEnactLine + "`"
	}
	if m := autoLaneNegationRe.FindString(body); m != "" {
		return false, fmt.Sprintf("the artifact also carries %q — a rejection or negation voids the acceptance line",
			StripControl(m))
	}
	return true, ""
}

func firstOr(xs []string, def string) string {
	if len(xs) == 0 {
		return def
	}
	return xs[0]
}

// AutoLaneEjectComment renders the ONE ejection comment, carrying its idempotency marker.
func AutoLaneEjectComment(reasons []string, head string) string {
	rs := append([]string{}, reasons...)
	sort.Strings(rs)
	return AutoLaneEjectMarker + "\n" +
		"**Auto-approve lane: ejected.** This PR left the auto-approve lane for the human merge queue at " +
		"`" + head + "`, on: " + strings.Join(rs, ", ") + ".\n\n" +
		"An ejection is one-way: nothing re-admits this PR to the lane. It merges only by a human act."
}

// AutoLaneLabelApplier resolves who applied the admission label standing on the PR: the
// actor of the LAST `labeled` event for it not followed by an `unlabeled`. ok is false when
// the label is on the PR but no event attributes it (could-not-check) or it is not present.
func AutoLaneLabelApplier(present []string, events []LabelEvent) (applier string, ok bool) {
	return LabelApplier(present, events, AutoLaneLabel)
}

// LabelApplier is AutoLaneLabelApplier for any label name: the actor of the LAST `labeled`
// event for name not followed by an `unlabeled`, when name is on the PR. An empty name, a
// label not present, or a present label no event attributes is ("", false).
func LabelApplier(present []string, events []LabelEvent, name string) (applier string, ok bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	has := false
	for _, l := range present {
		if strings.EqualFold(strings.TrimSpace(l), name) {
			has = true
		}
	}
	if !has {
		return "", false
	}
	for _, ev := range events {
		if !strings.EqualFold(strings.TrimSpace(ev.Name), name) {
			continue
		}
		if ev.Removed {
			applier = ""
			continue
		}
		applier = ev.AppliedBy
	}
	return applier, applier != ""
}
