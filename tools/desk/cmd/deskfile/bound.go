package main

// bound.go — workflow-bound filings (`deskfile new --for-pr <owner/repo#N> --brief <id>`).
//
// A blocker or question issue that a change request's own review requires is part of that
// work, not a process-error report, so it is not counted by the per-session new-issue rate
// (checkSessionBudget), nor by the fixed 20-an-hour repo-wide ceiling that guards unbound
// filings. It is bounded instead by three counts, each of which stands without the others:
//
//  1. a per-role, per-PR cap on bound filings (the issue.file.bound_per_pr knob, counted over
//     issue.file.window; 0 = off), counted here by checkBoundBudget;
//  2. the per-PR outward-write tier, deskkit.AllowWrite("deskfile", <PR repo>, N);
//  3. the PR-bound repo-wide backstop, deskkit.AllowPRBoundWrite(<repo>, limit), a larger tier
//     whose limit is the write.bound.repo_hour knob (default 60, never off) and which counts
//     only deskfile's PR-bound writes: bound filings, their back-links and `attach --kind mr`
//     replies (audit verbs new-bound and attach-pr).
//
// The last two are always on, so they are the only bound while the cap is off. Unbound filings
// keep the fixed ceiling (deskkit.AllowWriteRepoWide) and do not see the bound class in it.
//
// The counted role is the one whose App token performs the write — deskkit.SessionTokenRole,
// which maps the session's desk loop (DESK_LOOP) to a role, the same custody that mints the
// token. It is never taken from --raised-by, --to or a per-role cap override. It is the first
// field after the PR in the audit line's Detail prefix and is read back from there.
//
// The cap's resolved value and its source layer, and the backstop's, are recorded on every bound
// line (boundConfigNote), so a limit raised or switched off from the environment stays visible.
//
// What the binding does NOT do: --for-pr does not bind the filer to the PR's author, its
// reviewer or the holder of its dispatch claim. Any role may bind a filing to any open PR in
// the repository set. The binding records which work item a filing belongs to and refuses a
// filing that names a brief the PR does not carry; it is not an authorization check. The
// counts above are the bound.

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/config"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"gopkg.in/yaml.v3"
)

// boundVerb is the audit verb of a bound filing. checkSessionBudget counts Verb "new" only,
// so a bound filing never consumes the per-session rate.
const boundVerb = deskkit.PRBoundVerbNew

// attachPRVerb is the audit verb of an attach to a change request, and of a bound filing's
// back-link. It is counted by the PR-bound backstop, not the fixed repo-wide ceiling.
const attachPRVerb = deskkit.PRBoundVerbAttach

// boundCapKnob and boundWindowKnob are the deskcore/config knobs the per-PR cap reads.
const (
	boundCapKnob    = "issue.file.bound_per_pr"
	boundWindowKnob = "issue.file.window"
	// boundTierKnob is the PR-bound repo-wide backstop (deskkit.AllowPRBoundWrite).
	boundTierKnob = "write.bound.repo_hour"
)

// briefNone is the brief id recorded for a filing bound to a PR that carries no brief.
const briefNone = "none"

var forPRRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#([1-9][0-9]{0,9})$`)

// boundPrefixRe reads back the Detail prefix boundPrefix writes. The role is the field
// right after the PR reference, ahead of the brief id, so it is read before any value that a
// brief file supplies. (?s) so a multi-line remainder still matches.
var boundPrefixRe = regexp.MustCompile(`(?s)^for-pr (\S+)#([0-9]+) role ([a-z0-9][a-z0-9._-]*) brief (\S+) \| (.*)$`)

// briefIDRe is the strict grammar of a recorded brief id: lower-case segments of letters,
// digits, '.', '_' and '-', joined by ':' or '/'. No whitespace and no '|' can reach the
// audit prefix: a brief id may come from a brief file in the change request's own diff, so
// it is text the PR author controls.
var briefIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(?:[:/][a-z0-9][a-z0-9._-]*)+$`)

// roleRe is the grammar of a counted role.
var roleRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// prRef is a parsed --for-pr reference.
type prRef struct {
	Repo string
	N    int
}

func (p prRef) String() string { return p.Repo + "#" + strconv.Itoa(p.N) }

// parseForPR parses `<owner>/<repo>#<N>`. Anything else is refused (exit 5).
func parseForPR(s string) (prRef, error) {
	m := forPRRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || !deskkit.ValidRepoSlug(m[1]) {
		return prRef{}, deskkit.Refused(fmt.Sprintf(
			"refused: --for-pr %q is not <owner>/<repo>#<N> (for example example-org/example-repo#12)", s))
	}
	n, err := strconv.Atoi(m[2])
	if err != nil || n <= 0 {
		return prRef{}, deskkit.Refused(fmt.Sprintf("refused: --for-pr %q has no positive change number", s))
	}
	return prRef{Repo: m[1], N: n}, nil
}

// boundPrefix is the Detail prefix of every audit line of a bound filing. It leads the line,
// ahead of createSentMarker, and boundCharged strips it before applying chargedNewEntry.
// Every field is checked against its grammar here, whatever its origin, so a value that could
// change how the prefix reads back is refused rather than written.
func boundPrefix(pr prRef, brief, role string) (string, error) {
	if !forPRRe.MatchString(pr.String()) {
		return "", deskkit.Refused(fmt.Sprintf("refused: --for-pr %q cannot be recorded", pr.String()))
	}
	if brief != briefNone && !briefIDRe.MatchString(brief) {
		return "", deskkit.Refused(fmt.Sprintf("refused: brief id %q is not well formed "+
			"(lower-case segments joined by ':' or '/', no spaces or '|')", brief))
	}
	if !roleRe.MatchString(role) {
		return "", deskkit.Unverifiable(fmt.Sprintf("deskfile new --for-pr: counted role %q is not well formed", role), nil)
	}
	return fmt.Sprintf("for-pr %s role %s brief %s | ", pr, role, brief), nil
}

// boundRole resolves the counted role: the App role this session's write token is minted
// for. Anything that stops it resolving holds (exit 6) before any create — a bound filing
// is never counted under a guessed role.
func boundRole() (string, error) {
	role, _, err := deskkit.SessionTokenRole("deskfile")
	if err != nil {
		return "", deskkit.Unverifiable(
			"deskfile new --for-pr: the App role this session writes as cannot be resolved, so the bound "+
				"filing cannot be counted against it — holding before any create", err)
	}
	return role, nil
}

// boundLimits resolves the per-PR cap and its window for role from deskcore/config. The
// layers are the environment and the defaults only: deskfile reads no config file, so the
// driver sets the cap through ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR[__<ROLE>] in the desk's
// environment. A resolution error (an out-of-bounds value, an unknown ASSAY_DESK_* variable)
// holds (exit 6): a cap that cannot be read is never read as off.
func boundLimits(role string) (config.Value, config.Value, error) {
	r, err := config.Resolve(config.EnvFromList(os.Environ()), config.KnobFile{}, config.Table())
	if err != nil {
		return config.Value{}, config.Value{}, deskkit.Unverifiable("deskfile new --for-pr: cannot resolve "+boundCapKnob, err)
	}
	c, err := r.Get(boundCapKnob, role)
	if err != nil {
		return config.Value{}, config.Value{}, deskkit.Unverifiable("deskfile new --for-pr: cannot resolve "+boundCapKnob, err)
	}
	w, err := r.Get(boundWindowKnob, role)
	if err != nil {
		return config.Value{}, config.Value{}, deskkit.Unverifiable("deskfile new --for-pr: cannot resolve "+boundWindowKnob, err)
	}
	return c, w, nil
}

// boundTier resolves the PR-bound repo-wide backstop limit from deskcore/config (environment
// and default only, like boundLimits). The knob is not per role and its minimum is 1, so a
// resolution error or a value that is not a positive count holds (exit 6): the backstop is
// never read as off.
func boundTier() (config.Value, error) {
	r, err := config.Resolve(config.EnvFromList(os.Environ()), config.KnobFile{}, config.Table())
	if err != nil {
		return config.Value{}, deskkit.Unverifiable("deskfile: cannot resolve "+boundTierKnob, err)
	}
	v, err := r.Get(boundTierKnob, "")
	if err != nil {
		return config.Value{}, deskkit.Unverifiable("deskfile: cannot resolve "+boundTierKnob, err)
	}
	if v.Value < 1 {
		return config.Value{}, deskkit.Unverifiable(fmt.Sprintf("deskfile: %s resolved to %d — the backstop is never off", boundTierKnob, v.Value), nil)
	}
	return v, nil
}

// allowBoundTier resolves the backstop and applies it to repo. It returns the resolved value so
// the caller can record it, and the gate's refusal (exit 4) when the repo is at the limit.
func allowBoundTier(repo string) (config.Value, error) {
	v, err := boundTier()
	if err != nil {
		return config.Value{}, err
	}
	return v, deskkit.AllowPRBoundWrite(repo, int(v.Value))
}

// boundConfigNote is the cap's provenance, recorded on every bound audit line once the cap
// resolves: the value in force and the layer it came from (the environment variable that set
// it, or the default), for the cap and its window, then the PR-bound backstop. An override,
// including one that turns the cap off, is therefore always on the line it governs.
func boundConfigNote(c, w, tier config.Value) string {
	return "bound-config: " + c.String() + "; " + w.String() + "; " + tier.String()
}

// tierConfigNote is the provenance of the PR-bound backstop alone, recorded on an
// attach-to-a-change line, which has no per-PR cap.
func tierConfigNote(tier config.Value) string {
	return "bound-config: " + tier.String()
}

// boundCharged reports whether a bound audit entry charges the per-PR cap, and which PR and
// role it charges. It strips the bound prefix and applies chargedNewEntry's rules to the
// remainder, so createSentMarker is read in the same place as on an unbound line. An entry
// whose prefix cannot be read is an error: it is never read as uncharged.
func boundCharged(e deskkit.Entry) (pr prRef, role string, charged bool, err error) {
	m := boundPrefixRe.FindStringSubmatch(e.Detail)
	if m == nil {
		return prRef{}, "", false, fmt.Errorf("bound audit entry has an unreadable prefix")
	}
	n, aerr := strconv.Atoi(m[2])
	if aerr != nil {
		return prRef{}, "", false, aerr
	}
	rest := e
	rest.Detail = m[5]
	return prRef{Repo: m[1], N: n}, m[3], chargedNewEntry(rest), nil
}

// checkBoundBudget applies the per-role, per-PR cap on bound filings. It returns nil when
// the cap is off or one more filing is within it, RateLimited (exit 4, with retry-after) at
// the cap, and Unverifiable (exit 6) on an unreadable ledger. It never reads the session tag:
// the count is keyed on the PR (repository plus number) and the writing role only, and never
// on the PR head, which a push would reset.
func checkBoundBudget(pr prRef, role string, now time.Time, limit int64, window time.Duration) error {
	if limit <= 0 {
		return nil
	}
	entries, err := deskkit.LoadEntries()
	if err != nil {
		return err // already an Unverifiable *DeskError (exit 6)
	}
	cutoff := now.Add(-window)
	var charged []time.Time
	for _, e := range entries {
		if e.Tool != "deskfile" || e.Verb != boundVerb {
			continue
		}
		ts, perr := time.Parse(time.RFC3339, e.TS)
		if perr != nil {
			return deskkit.Unverifiable(fmt.Sprintf(
				"deskfile bound-filing cap: audit entry has an unparseable ts %q — run `deskaudit recover`", e.TS), perr)
		}
		if ts.Before(cutoff) {
			continue
		}
		epr, erole, ok, berr := boundCharged(e)
		if berr != nil {
			return deskkit.Unverifiable("deskfile bound-filing cap: a "+boundVerb+" audit entry in the window cannot be read — run `deskaudit recover`", berr)
		}
		if !ok || erole != role || epr.N != pr.N || !strings.EqualFold(epr.Repo, pr.Repo) {
			continue
		}
		charged = append(charged, ts)
	}
	if int64(len(charged)) < limit {
		return nil
	}
	sortTimesAscending(charged)
	freeAt := charged[0].Add(window).Add(time.Second)
	retryAfter := freeAt.Sub(now)
	if retryAfter <= 0 {
		retryAfter = time.Second
	}
	return deskkit.RateLimitedAfter(fmt.Sprintf(
		"refused: bound-filing cap reached (%d filings bound to %s by role %s within the last %s; %s = %d) — "+
			"retry-after: %ds (free at %s). Attach further observations to an issue already filed for this "+
			"change request instead.",
		len(charged), pr, role, window, boundCapKnob, limit,
		int(retryAfter/time.Second), freeAt.UTC().Format(time.RFC3339)), retryAfter)
}

// boundBinding is a verified --for-pr binding.
type boundBinding struct {
	pr    prRef
	brief string // canonical qualified brief id, or briefNone
	// cfg is the bound-config note the filing's own line carries; the back-link repeats it, so
	// the backstop's value and source sit on the line of every write it counted.
	cfg string
}

// verifyBinding checks, through the forge, that pr is open and that briefArg (empty when
// --brief was not given) is that PR's brief. Refusals are exit 5; anything that could not be
// read is exit 6 and is never treated as bound.
func verifyBinding(pr prRef, briefArg string) (boundBinding, error) {
	fg, fr, _, ferr := forgeForFn(pr.Repo, false)
	if ferr != nil {
		return boundBinding{}, ferr
	}
	got, err := fg.GetPullRequest(fr, pr.N)
	if err != nil || got == nil {
		return boundBinding{}, deskkit.Unverifiable("cannot read change request "+pr.String()+" — holding rather than bind to it", err)
	}
	if !strings.EqualFold(got.State, "open") {
		return boundBinding{}, deskkit.Refused(fmt.Sprintf(
			"refused: %s is %s, not open — a filing can be bound only to an open change request", pr, got.State))
	}
	trailers, terr := deskkit.ParseTrailers([]byte(got.Body))
	if terr != nil {
		return boundBinding{}, deskkit.Refused("refused: " + pr.String() + "'s link trailers cannot be read: " + terr.Error())
	}
	files, cerr := deskkit.CompleteChangedFiles(fg, fr, pr.N)
	if cerr != nil {
		return boundBinding{}, deskkit.Unverifiable("cannot read the complete diff of "+pr.String(), cerr)
	}

	if strings.TrimSpace(briefArg) == "" {
		if prHasBrief(trailers, files) {
			return boundBinding{}, deskkit.Refused(fmt.Sprintf(
				"refused: %s carries a brief, so --for-pr needs --brief <id> naming it", pr))
		}
		return boundBinding{pr: pr, brief: briefNone}, nil
	}

	stream, nn, ok := deskkit.SplitBriefTrailer(briefArg)
	if !ok || strings.ContainsAny(briefArg, " \t\n") {
		return boundBinding{}, deskkit.Refused(fmt.Sprintf(
			"refused: --brief %q is not a brief id (<stream>/<NN>, <stream>:<NN> or a qualified colon form)", briefArg))
	}
	id := strings.ToLower(stream + "/" + nn)

	// The tracking repository: the alias repo a Cross-repo-brief trailer names, else the PR's own.
	trackRepo, xrAlias := pr.Repo, ""
	for _, t := range trailers {
		if t.Kind != deskkit.TrailerCrossRepoBrief {
			continue
		}
		alias, _, _, xok := deskkit.SplitCrossRepoBriefTrailer(t.Value)
		if !xok {
			return boundBinding{}, deskkit.Refused("refused: " + pr.String() + "'s Cross-repo-brief trailer is malformed")
		}
		repo, rerr := resolveAlias(fg, fr, got.BaseRef, alias)
		if rerr != nil {
			return boundBinding{}, rerr
		}
		trackRepo, xrAlias = repo, alias
	}

	tfg, tfr := fg, fr
	if !strings.EqualFold(trackRepo, pr.Repo) {
		if !deskkit.IsAllowedRepo(trackRepo) {
			return boundBinding{}, deskkit.Refused("refused: tracking repository " + trackRepo + " is not in the desk-tools repo set")
		}
		var terr2 error
		tfg, tfr, _, terr2 = forgeForFn(trackRepo, false)
		if terr2 != nil {
			return boundBinding{}, terr2
		}
	}
	var diff []deskkit.ChangedFile
	if strings.EqualFold(trackRepo, pr.Repo) {
		diff = files
	}
	file, fm, rerr := resolveBriefFile(tfg, tfr, stream, nn, diff, got.HeadSHA)
	if rerr != nil {
		return boundBinding{}, rerr
	}
	qualified, qerr := qualifiedBriefID(briefArg, id, fm)
	if qerr != nil {
		return boundBinding{}, qerr
	}
	if !isPRsBrief(trailers, xrAlias, id, file, diff) {
		return boundBinding{}, deskkit.Refused(fmt.Sprintf(
			"refused: %s is not %s's brief — no Brief: or Cross-repo-brief: trailer names it and the diff does "+
				"not touch %s. Bind the filing to the change request that carries this brief.", id, pr, file))
	}
	return boundBinding{pr: pr, brief: qualified}, nil
}

// prHasBrief reports whether a change request carries a brief: a Brief:, Cross-repo-brief:
// or Authors: trailer, or a diff that touches any brief file. An Authors: change request (one
// that authors briefs) therefore needs --brief; isPRsBrief then admits only a brief whose
// file its diff touches, since an authoring change carries each authored brief's file.
// Several Brief:, Cross-repo-brief: or Authors: trailers, or a mix of kinds, never reach
// here: deskkit.ParseTrailers refuses them (exit 5).
func prHasBrief(trailers []deskkit.Trailer, files []deskkit.ChangedFile) bool {
	for _, t := range trailers {
		switch t.Kind {
		case deskkit.TrailerBrief, deskkit.TrailerCrossRepoBrief, deskkit.TrailerAuthors:
			return true
		}
	}
	for _, f := range files {
		for _, p := range []string{f.Filename, f.PreviousFilename} {
			dir, base := path.Split(p)
			if strings.HasPrefix(dir, "docs/streams/") && strings.HasPrefix(base, "brief-") && strings.HasSuffix(base, ".md") {
				return true
			}
		}
	}
	return false
}

// isPRsBrief is the membership test: a Brief: trailer names id, a Cross-repo-brief: trailer
// names it (its alias already chose the tracking repository), or the diff touches its file.
func isPRsBrief(trailers []deskkit.Trailer, xrAlias, id, file string, diff []deskkit.ChangedFile) bool {
	for _, t := range trailers {
		switch t.Kind {
		case deskkit.TrailerBrief:
			if deskkit.CanonicalBriefID(t.Value) == id {
				return true
			}
		case deskkit.TrailerCrossRepoBrief:
			alias, s, n, ok := deskkit.SplitCrossRepoBriefTrailer(t.Value)
			if ok && alias == xrAlias && strings.ToLower(s+"/"+n) == id {
				return true
			}
		}
	}
	for _, f := range diff {
		if f.Filename == file || f.PreviousFilename == file {
			return true
		}
	}
	return false
}

// briefLinkRe finds brief file names in a stream README.
var briefLinkRe = regexp.MustCompile(`brief-[0-9]+-[A-Za-z0-9._-]+\.md`)

// resolveBriefFile finds the brief file for stream/nn in the tracking repository: a file in the
// PR's diff (read at the PR head — an authoring or Evidence PR carries it before main does),
// else one the stream README links (read at the default branch). It returns the path and the
// file's frontmatter `brief:` value. No file is a refusal; a failed read holds.
func resolveBriefFile(fg deskkit.Forge, fr deskkit.ForgeRepo, stream, nn string, diff []deskkit.ChangedFile, head string) (string, string, error) {
	try := func(p, ref string) (bool, string, error) {
		fc, err := fg.ReadFile(fr, deskkit.ReadFileInput{File: p, Ref: ref})
		if err != nil {
			return false, "", deskkit.Unverifiable("cannot read "+p+" on "+fr.Slug(), err)
		}
		if fc == nil || !fc.Exists {
			return false, "", nil
		}
		return true, frontmatterBrief(fc.Content), nil
	}
	for _, f := range diff {
		if f.Status == "removed" || !briefFileFor(f.Filename, stream, nn) {
			continue
		}
		ok, fm, err := try(f.Filename, head)
		if err != nil || ok {
			return f.Filename, fm, err
		}
	}
	def, err := fg.RepoDefaultBranch(fr)
	if err != nil {
		return "", "", deskkit.Unverifiable("cannot read the default branch of "+fr.Slug(), err)
	}
	readme := "docs/streams/" + stream + "/README.md"
	fc, err := fg.ReadFile(fr, deskkit.ReadFileInput{File: readme, Ref: def})
	if err != nil {
		return "", "", deskkit.Unverifiable("cannot read "+readme+" on "+fr.Slug(), err)
	}
	if fc != nil && fc.Exists {
		seen := map[string]bool{}
		for _, name := range briefLinkRe.FindAllString(string(fc.Content), -1) {
			p := "docs/streams/" + stream + "/" + name
			if seen[p] || !briefFileFor(p, stream, nn) {
				continue
			}
			seen[p] = true
			ok, fm, err := try(p, def)
			if err != nil || ok {
				return p, fm, err
			}
		}
	}
	return "", "", deskkit.Refused(fmt.Sprintf(
		"refused: brief %s/%s does not resolve to a brief file in %s", stream, nn, fr.Slug()))
}

// briefFileFor reports whether p is docs/streams/<stream>/brief-<nn>-*.md.
func briefFileFor(p, stream, nn string) bool {
	dir, base := path.Split(path.Clean(p))
	if !strings.EqualFold(dir, "docs/streams/"+stream+"/") {
		return false
	}
	prefix := "brief-" + nn + "-"
	return strings.HasPrefix(base, prefix) && strings.HasSuffix(base, ".md") && len(base) > len(prefix)+len(".md")
}

// frontmatterBrief returns the `brief:` value of a brief file's frontmatter, or "".
func frontmatterBrief(b []byte) string {
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, ln := range lines[1:] {
		if strings.TrimSpace(ln) == "---" {
			break
		}
		if v, ok := strings.CutPrefix(ln, "brief:"); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// qualifiedBriefID is the id the audit records: the brief file's own frontmatter id when it
// names the same stream/NN, else the canonical stream/NN. A qualified --brief whose prefix
// disagrees with the frontmatter id names a different brief and is refused.
//
// The brief file may come from the change request's own diff, so its frontmatter id is
// untrusted: one that names this brief but is not well formed (briefIDRe) is refused, and a
// qualified --brief on a brief whose file carries no frontmatter id cannot be checked and is
// refused too.
func qualifiedBriefID(arg, id, fm string) (string, error) {
	fm = strings.ToLower(strings.TrimSpace(fm))
	a := strings.Split(strings.ToLower(strings.TrimSpace(arg)), ":")
	if fm == "" {
		if len(a) > 2 {
			return "", deskkit.Refused(fmt.Sprintf("refused: --brief %q is qualified, but brief %s carries no "+
				"frontmatter brief: id to check the qualifier against — pass %s", arg, id, id))
		}
		return id, nil
	}
	if deskkit.CanonicalBriefID(fm) != id {
		return id, nil
	}
	if !briefIDRe.MatchString(fm) {
		return "", deskkit.Refused(fmt.Sprintf("refused: brief %s's frontmatter id %q is not a well-formed brief id "+
			"(lower-case segments joined by ':' or '/', no spaces or '|')", id, fm))
	}
	f := strings.Split(fm, ":")
	if len(a) > 2 {
		if len(a) > len(f) {
			return "", deskkit.Refused(fmt.Sprintf("refused: --brief %q is qualified beyond the brief's own id %s", arg, fm))
		}
		for i := 1; i <= len(a); i++ {
			if a[len(a)-i] != f[len(f)-i] {
				return "", deskkit.Refused(fmt.Sprintf("refused: --brief %q does not name the brief whose id is %s", arg, fm))
			}
		}
	}
	return fm, nil
}

// resolveAlias maps a Cross-repo-brief alias to its repository through the PR repository's
// graph-repos.yaml at ref. An alias the registry does not carry is refused; one it carries
// without a published repository (or a registry that cannot be read) holds.
func resolveAlias(fg deskkit.Forge, fr deskkit.ForgeRepo, ref, alias string) (string, error) {
	const reg = "docs/streams/graph-repos.yaml"
	fc, err := fg.ReadFile(fr, deskkit.ReadFileInput{File: reg, Ref: ref})
	if err != nil || fc == nil || !fc.Exists {
		return "", deskkit.Unverifiable("cannot read "+reg+" on "+fr.Slug()+" to resolve alias "+alias, err)
	}
	var g struct {
		Repos map[string]struct {
			Repo *string `yaml:"repo"`
		} `yaml:"repos"`
	}
	if yerr := yaml.Unmarshal(fc.Content, &g); yerr != nil {
		return "", deskkit.Unverifiable("cannot parse "+reg+" on "+fr.Slug(), yerr)
	}
	e, ok := g.Repos[alias]
	if !ok {
		return "", deskkit.Refused("refused: Cross-repo-brief alias " + alias + " is not registered in " + reg)
	}
	if e.Repo == nil || !deskkit.ValidRepoSlug(strings.TrimSpace(*e.Repo)) {
		return "", deskkit.Unverifiable("alias "+alias+" has no published repository in "+reg+" — the brief cannot be read from here", nil)
	}
	return strings.TrimSpace(*e.Repo), nil
}

// backLink posts the one comment on the bound change request that names the filed issue, through
// the attach path (postAttach), and writes that write's own attach audit line. A failure is
// returned for the caller to report as unverifiable; the issue is never deleted and the create
// is never retried.
func backLink(b boundBinding, issueURL string) error {
	ac := &auditCtx{verb: attachPRVerb, repo: b.pr.Repo, boundCfg: b.cfg}
	n := b.pr.N
	ac.target = &n
	var err error
	defer func() { ac.finalize(err) }()
	fg, fr, _, ferr := forgeForFn(b.pr.Repo, false)
	if ferr != nil {
		err = ferr
		return err
	}
	body := []byte(fmt.Sprintf("Filed %s for this change request (brief %s).\n", issueURL, b.brief))
	ac.bodyDigest = deskkit.Sha256Hex(body)
	ref, perr := postAttach(fg, fr, b.pr.Repo, b.pr.N, deskkit.TargetChange, body)
	if perr != nil {
		err = perr
		return err
	}
	ac.detail = "back-link " + issueURL + " commented " + deskkit.StripControl(ref.URL) + " kind=" + string(deskkit.TargetChange)
	return nil
}
