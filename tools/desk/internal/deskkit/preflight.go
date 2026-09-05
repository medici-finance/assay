package deskkit

// preflight.go — the desk's OPERATING-ENVELOPE check, run once at boot, before
// any work is claimed.
//
// Roughly one in five of the verifier desk's open issues is not a finding about
// the work; it is the desk discovering its own envelope MID-PASS:
//
//   - #794 — the App pem lives in a directory the minter never looks in, so a
//     fresh shell cannot mint once the ~1h token cache lapses.
//   - #571 — the App's installation is missing the scope the role's duty needs
//     (PR comment), discovered at comment time.
//   - #823 — no permitted outward write transport at LANDING time: the pass is
//     complete, correct and unlandable.
//   - #638 — weeks of commits under an email whose numeric prefix is the App id
//     instead of the bot USER id, so nothing is account-linked.
//   - #679 — a sibling checkout a queued brief's rows need is simply not there.
//
// Each of those cost a whole pass and produced an issue ABOUT THE DESK. A
// boot-time preflight converts every one of them into a single loud line before
// anything is claimed.
//
// Three properties are the whole point and each is pinned by a test:
//
//  1. THREE STATE, ZERO VALUE RED. Every check answers checked-clean,
//     checked-failed, or could-not-check. The zero value of CheckState is
//     could-not-check, so a check that returns without looking can never read
//     green by omission.
//  2. A PREFLIGHT FAILURE IS COULD-NOT-RUN FOR THE WHOLE PASS. The desk prints
//     ONE line and stops. It does not claim work, burn a pass, or file an issue
//     about its own envelope — the envelope issues above already exist.
//  3. A PROBE REJECTION IS A STOP (AGENTS.md, "Scope rejections"). There is no
//     retry-under-another-identity path in this file, and
//     TestPreflightProbeRejectionIsNotRetried pins that the probe is invoked at
//     most once.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CheckState is one check's three-state answer.
//
// The zero value is deliberately CouldNotCheck, not CheckedClean: a check that
// returns its result struct without ever looking must read RED. "Green by
// omission" is how a preflight becomes a rubber stamp.
type CheckState int

const (
	// CouldNotCheck — the check could not look. NOT a pass. This is the zero value.
	CouldNotCheck CheckState = iota
	// CheckedClean — the check looked and the envelope is intact.
	CheckedClean
	// CheckedFailed — the check looked and the envelope is broken.
	CheckedFailed
)

// String renders the state in the fixed vocabulary the desk reports in
// ("checked-clean" / "checked-failed" / "could-not-check"). An out-of-range
// value renders as could-not-check rather than as an unknown token, keeping the
// fail-closed reading of a corrupted value.
func (s CheckState) String() string {
	switch s {
	case CheckedClean:
		return "checked-clean"
	case CheckedFailed:
		return "checked-failed"
	default:
		return "could-not-check"
	}
}

// Green reports whether this state permits work to proceed. ONLY CheckedClean does.
func (s CheckState) Green() bool { return s == CheckedClean }

// Check names of the five envelope checks. They are exported constants because
// the summary line, the tests and the consumers all refer to the same names —
// a check renamed in one place and not another is a check that silently stops
// being reported on.
const (
	CheckColdMint       = "token-mint-cold"
	CheckAppScopes      = "app-scopes-vs-duties"
	CheckWriteTransport = "write-transport"
	CheckCommitIdentity = "commit-identity"
	CheckSiblings       = "sibling-checkouts"
)

// Check is one envelope check's result.
//
// Remediation is a NAMED fix, not a diagnosis: the reader must be able to act on
// it without opening an issue. It is required whenever State is not
// CheckedClean; NewCheck refuses to build a non-green result without one, so the
// "it's broken, good luck" result shape does not exist.
type Check struct {
	Name        string
	State       CheckState
	Detail      string
	Remediation string
	// Refs are the issue numbers this check exists because of, rendered into the
	// summary so the reader can see the failure already has a home and does NOT
	// need a new issue filed about it.
	Refs string
}

// clean builds a green result. A green check carries no remediation by construction.
func clean(name, detail, refs string) Check {
	return Check{Name: name, State: CheckedClean, Detail: detail, Refs: refs}
}

// failed builds a checked-failed result: the check LOOKED and the envelope is broken.
func failed(name, detail, remediation, refs string) Check {
	return Check{Name: name, State: CheckedFailed, Detail: detail, Remediation: remediation, Refs: refs}
}

// unchecked builds a could-not-check result: the check could not look. It is
// NOT a pass, and it carries the remediation that would let it look next time.
func unchecked(name, detail, remediation, refs string) Check {
	return Check{Name: name, State: CouldNotCheck, Detail: detail, Remediation: remediation, Refs: refs}
}

// PreflightReport is the whole envelope answer for one role.
type PreflightReport struct {
	Role   string
	Checks []Check
}

// Green reports whether EVERY check is checked-clean. An empty report is NOT
// green: a preflight that ran no checks proved nothing.
func (r PreflightReport) Green() bool {
	if len(r.Checks) == 0 {
		return false
	}
	for _, c := range r.Checks {
		if !c.State.Green() {
			return false
		}
	}
	return true
}

// Blocking returns the non-green checks, in check order.
func (r PreflightReport) Blocking() []Check {
	var out []Check
	for _, c := range r.Checks {
		if !c.State.Green() {
			out = append(out, c)
		}
	}
	return out
}

// SummaryLine is the ONE line a red preflight prints. It is one line by
// construction — every embedded newline and carriage return is collapsed to a
// space — because the contract is "report one line and stop", and a probe's
// multi-line stderr pasted into a boot message is how one line becomes forty.
func (r PreflightReport) SummaryLine() string {
	green := 0
	for _, c := range r.Checks {
		if c.State.Green() {
			green++
		}
	}
	verdict := "RED"
	if r.Green() {
		verdict = "GREEN"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "preflight role=%s %s %d/%d checked-clean", r.Role, verdict, green, len(r.Checks))
	for _, c := range r.Blocking() {
		fmt.Fprintf(&b, " · %s=%s: %s → fix: %s", c.Name, c.State, c.Detail, c.Remediation)
		if c.Refs != "" {
			fmt.Fprintf(&b, " [%s]", c.Refs)
		}
	}
	return oneLine(b.String())
}

// oneLine collapses any newline/CR/tab run into a single space so a captured
// stderr can never turn the summary into a multi-line boot dump.
func oneLine(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// Err maps a non-green report to the canonical COULD-NOT-RUN refusal.
//
// Both non-green states map to ExitUnverifiable (6) on purpose. From the pass's
// point of view there is no difference between "the envelope is broken" and "we
// could not tell whether the envelope is broken": in both cases the pass did not
// run and must not be recorded as having run. Collapsing checked-failed to a
// different, softer code is how a red envelope becomes a retried one.
func (r PreflightReport) Err() error {
	if r.Green() {
		return nil
	}
	return Unverifiable(r.SummaryLine(), nil)
}

// --- probes ----------------------------------------------------------------

// ProbeVerdict is what a READ-ONLY write-transport probe learned.
//
// ProbeRejected is a STOP (AGENTS.md, "Scope rejections"): the caller reports it
// and halts. It never re-runs the probe under a different identity, and this
// package supplies no way to.
type ProbeVerdict int

const (
	// ProbeInconclusive — the probe could not reach a verdict (no git, no remote,
	// transport error). could-not-check, never a pass.
	ProbeInconclusive ProbeVerdict = iota
	// ProbePermitted — the transport authenticated and the landing ref would be accepted.
	ProbePermitted
	// ProbeRejected — the transport is denied. A STOP.
	ProbeRejected
)

// Landing is the role's landing path: where this pass's output would be written.
type Landing struct {
	// Dir is the working tree the probe runs in ("" → the process's cwd).
	Dir string
	// Remote is the git remote name the landing targets.
	Remote string
	// Branch is the landing branch. Empty means "this worktree's current branch",
	// resolved at probe time; a DETACHED worktree therefore probes inconclusive
	// rather than guessing a branch.
	Branch string
}

// PreflightProbes are the injectable edges of the five checks. A nil field is
// filled with the real, environment-backed probe — tests supply their own so the
// suite is hermetic (no network, no credentials, no git remote).
type PreflightProbes struct {
	// ColdMint mints the role's token in a FRESH process with a scrubbed
	// environment and returns the TOKEN FILE PATH (never the token value). repo
	// is the owner/name slug whose INSTALLATION the token is minted against;
	// empty means the minter applies its own default.
	ColdMint func(role, repo string) (tokenPath string, err error)
	// GrantedScopes returns the permission map GitHub granted the installation
	// the token was minted for. tokenPath is what ColdMint returned; an empty
	// tokenPath means the mint produced nothing to read scopes from.
	GrantedScopes func(role, tokenPath string) (map[string]string, error)
	// WriteTransport performs ONE read-only probe of the landing path.
	// Called at most once per preflight — see the ProbeRejected doc.
	WriteTransport func(l Landing) (ProbeVerdict, string, error)
	// CommitEmail returns the commit author email this worktree would commit under.
	CommitEmail func(dir string) (string, error)
	// AppIDFor returns the role's GitHub App id — the value that must NOT appear
	// as the commit email's numeric prefix (#638).
	AppIDFor func(role string) (string, error)
	// QueuedSiblings returns the sibling checkouts the QUEUED briefs declare.
	QueuedSiblings func(root string) ([]SiblingReq, error)
	// DirExists reports whether a directory is present and readable.
	DirExists func(path string) (bool, error)
}

// SiblingReq is one sibling checkout a queued brief declares it needs, with the
// brief that declared it so a missing checkout names its claimant.
type SiblingReq struct {
	Brief string
	// Rel is the declared path, relative to the repo root (e.g. "../tracker").
	Rel string
}

// PreflightRequest is a fully-specified preflight. Preflight(role) is the
// convenience wrapper for the common case.
type PreflightRequest struct {
	Role string
	Root string
	// Repo is the owner/name slug the role operates on. Empty means "derive it
	// from the landing remote" — an App token is minted against an INSTALLATION,
	// and a probe that lets the minter fall back to its built-in default owner
	// tests an installation the pass will never use.
	Repo    string
	Landing Landing
	Probes  PreflightProbes
}

// Preflight runs the five envelope checks for a role against the current
// working tree and environment, in the order a pass would hit them.
//
// It is the ONE entry point desks call at boot:
//
//	rep := deskkit.Preflight("verifier")
//	if err := rep.Err(); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(deskkit.ExitCodeOf(err)) }
//
// Nothing is claimed, nothing is filed, and the process stops.
func Preflight(role string) PreflightReport {
	return PreflightRequest{Role: role, Root: "."}.Run()
}

// Run executes the five checks. It never returns an error: every failure mode is
// a Check with a state and a remediation, because an error return is the one
// shape a caller can drop on the floor and still proceed.
func (req PreflightRequest) Run() PreflightReport {
	role := strings.ToLower(strings.TrimSpace(req.Role))
	root := req.Root
	if root == "" {
		root = "."
	}
	l := req.Landing
	if l.Remote == "" {
		l.Remote = "origin"
	}
	if l.Dir == "" {
		l.Dir = root
	}
	p := req.Probes.withDefaults()

	rep := PreflightReport{Role: role}
	if role == "" {
		rep.Checks = []Check{unchecked(CheckColdMint, "no role given",
			"call preflight with the desk role this session acts as (reviewer/verifier/worker/desk/issue-loop/intake-loop)", "")}
		return rep
	}

	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		repo = deriveRepoSlug(l.Dir, l.Remote)
	}

	tokenPath, mint := checkColdMint(p, role, repo)
	rep.Checks = append(rep.Checks,
		mint,
		checkAppScopes(p, role, tokenPath),
		checkWriteTransport(p, l),
		checkCommitIdentity(p, role, l.Dir),
		checkSiblings(p, root),
	)
	return rep
}

// withDefaults fills every nil probe with the real environment-backed one.
func (p PreflightProbes) withDefaults() PreflightProbes {
	if p.ColdMint == nil {
		p.ColdMint = coldMintProbe
	}
	if p.GrantedScopes == nil {
		p.GrantedScopes = grantedScopesProbe
	}
	if p.WriteTransport == nil {
		p.WriteTransport = writeTransportProbe
	}
	if p.CommitEmail == nil {
		p.CommitEmail = commitEmailProbe
	}
	if p.AppIDFor == nil {
		p.AppIDFor = AppID
	}
	if p.QueuedSiblings == nil {
		p.QueuedSiblings = QueuedSiblings
	}
	if p.DirExists == nil {
		p.DirExists = dirExistsProbe
	}
	return p
}

// --- check 1: token mint, cold ---------------------------------------------

// checkColdMint proves the role can obtain a credential from a FRESH process
// with no inherited token, pem override or App-id override — the exact state a
// long session lands in ~1h after boot when the cached token lapses (#794), and
// the state a worker starts in when the tool is not even on PATH (#567).
func checkColdMint(p PreflightProbes, role, repo string) (string, Check) {
	tokenPath, err := p.ColdMint(role, repo)
	if err != nil {
		st := CheckedFailed
		if IsUnverifiable(err) && strings.Contains(err.Error(), "could not run") {
			st = CouldNotCheck
		}
		c := Check{
			Name:   CheckColdMint,
			State:  st,
			Detail: oneLine(err.Error()),
			Remediation: "put the desk-tools bin dir on PATH and point " + EnvConfigHome +
				" at the directory holding <role>-app.pem and apps.env, then re-run preflight",
			Refs: "#794 #567",
		}
		return "", c
	}
	if strings.TrimSpace(tokenPath) == "" {
		return "", unchecked(CheckColdMint, "the mint reported success but named no token file",
			"re-run `desktoken "+role+"` by hand and read its stdout — it must print the token cache PATH", "#794")
	}
	return tokenPath, clean(CheckColdMint, "cold mint ok ("+tokenPath+")", "#794 #567")
}

// coldMintProbe is the real cold mint: `desktoken <role>` in a fresh process
// with a SCRUBBED environment.
//
// "Cold" is the load-bearing word. Inheriting this process's environment would
// carry <ROLE>_TOKEN / <ROLE>_PEM / <ROLE>_APP_ID / GH_TOKEN forward and the
// probe would pass on an ambient credential that a fresh shell will not have —
// which is precisely the failure #794 describes: everything works until the warm
// cache lapses. Only HOME, PATH, the config-home knob, the proxy/TLS variables
// the network call needs, and TMPDIR survive.
//
// The probe MINTS (it does not use --ttl): --ttl fails on a cold machine that
// has no cache yet, which is the normal state at boot, so it would report red
// for a perfectly healthy envelope. Minting writes only the 0600 token cache —
// it is the same call the desk makes at boot anyway, and no repo is touched.
func coldMintProbe(role, repo string) (string, error) {
	bin, err := exec.LookPath("desktoken")
	if err != nil {
		return "", Refused("desktoken is not on PATH: " + err.Error())
	}
	args := []string{role}
	if strings.TrimSpace(repo) != "" {
		args = append(args, "--repo", repo)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = scrubbedEnv()
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if asExitError(err, &ee) {
			msg := oneLine(string(ee.Stderr))
			if msg == "" {
				msg = "desktoken " + role + " exited " + strconv.Itoa(ee.ExitCode())
			}
			return "", Unverifiable("cold mint refused: "+msg, nil)
		}
		// Could not even run the binary (timeout, exec failure): could-not-check.
		return "", Unverifiable("could not run desktoken "+role+": "+oneLine(err.Error()), nil)
	}
	return strings.TrimSpace(lastLine(string(out))), nil
}

// remoteSlugRe pulls owner/name out of a git remote URL in any of the shapes a
// desk checkout uses (https, ssh, scp-style, and an ssh HOST ALIAS — the alias
// form is the one a naive parser drops, and dropping it silently sends the mint
// at the minter's default owner instead of the one the pass will land on).
var remoteSlugRe = regexp.MustCompile(`(?:[:/])([A-Za-z0-9._-]+)/([A-Za-z0-9._-]+?)(?:\.git)?$`)

// deriveRepoSlug reads the landing remote's URL and extracts owner/name. It
// returns "" when it cannot — an empty slug lets the minter apply its own
// default, and the cold-mint check reports whatever that produces rather than
// this function inventing a repo.
func deriveRepoSlug(dir, remote string) string {
	out, err := gitOut(orDot(dir), "remote", "get-url", remote)
	if err != nil {
		return ""
	}
	m := remoteSlugRe.FindStringSubmatch(strings.TrimSpace(out))
	if m == nil {
		return ""
	}
	return m[1] + "/" + m[2]
}

// scrubbedEnv is the ALLOWLIST a cold probe runs under. An allowlist, not a
// denylist: a new credential env var added elsewhere must not silently start
// warming this probe.
func scrubbedEnv() []string {
	keep := []string{"HOME", "PATH", EnvConfigHome, "TMPDIR",
		"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy",
		"SSL_CERT_FILE", "SSL_CERT_DIR"}
	var env []string
	for _, k := range keep {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// --- check 2: App scopes vs the role's rostered duties ----------------------

// Duty is one thing the role is rostered to DO, bound to the GitHub App
// permission that lets it. The duty list is the mechanism this repo ships; the
// role→App binding is the policy the roster supplies (ASSAY_TRUSTED_BOT_SLUGS).
type Duty struct {
	Permission string
	Level      string
	Why        string
}

// requiredDuties are the three duties every desk role's App must be able to
// perform. They are exactly the three #571 names: a verifier that can open an
// issue but goes silent on PR threads has a scope gap that only shows up at
// comment time, three quarters of the way through a pass.
var requiredDuties = []Duty{
	{Permission: "pull_requests", Level: "write", Why: "PR comment / review (#571)"},
	{Permission: "issues", Level: "write", Why: "file and comment on issues"},
	{Permission: "contents", Level: "write", Why: "land commits (Evidence, status)"},
}

// checkAppScopes compares the installation's GRANTED permissions against the
// role's duties. A missing scope is checked-failed; an unreadable grant is
// could-not-check — reading a permission LISTING as a grant would be the exact
// mistake AGENTS.md forbids in the other direction, so an ABSENT listing is
// certainly not one.
func checkAppScopes(p PreflightProbes, role, tokenPath string) Check {
	const refs = "#571"
	if tokenPath == "" {
		return unchecked(CheckAppScopes, "no token was minted, so no grant could be read",
			"fix the "+CheckColdMint+" check first — scopes are read from the mint's recorded grant", refs)
	}
	// After any installation permission change the cached token still carries the OLD grant
	// for the rest of the ~50-min reuse window, and the .perms sidecar this check reads is
	// only rewritten on a FRESH mint — so every remediation below says `--fresh`, not a plain
	// re-mint (which would be a no-op that re-reads the same stale grant).
	granted, err := p.GrantedScopes(role, tokenPath)
	if err != nil {
		return unchecked(CheckAppScopes, oneLine(err.Error())+grantSidecarAge(tokenPath),
			"re-mint FRESH with `desktoken "+role+" --fresh` so the grant sidecar is (re)written next to the "+
				"token cache; after any installation permission change the cached token still carries the old "+
				"grant — re-mint fresh", refs)
	}
	if len(granted) == 0 {
		return unchecked(CheckAppScopes, "the recorded grant is empty"+grantSidecarAge(tokenPath),
			"re-mint FRESH with `desktoken "+role+" --fresh`; if the grant is still empty the installation grants "+
				"nothing and needs its permissions set (after any installation permission change the cached token "+
				"still carries the old grant — re-mint fresh)", refs)
	}
	var missing []string
	for _, d := range requiredDuties {
		if !levelSatisfies(granted[d.Permission], d.Level) {
			missing = append(missing, fmt.Sprintf("%s=%s (want %s, for %s)",
				d.Permission, orNone(granted[d.Permission]), d.Level, d.Why))
		}
	}
	if len(missing) > 0 {
		return failed(CheckAppScopes, "granted scopes do not cover the role's duties: "+strings.Join(missing, "; ")+grantSidecarAge(tokenPath),
			"add the missing permission to the "+role+" App's INSTALLATION (both installs) and accept the "+
				"permission-update prompt, THEN re-mint FRESH with `desktoken "+role+" --fresh` — after any "+
				"installation permission change the cached token still carries the old grant, so a plain re-mint is "+
				"a no-op; this check compares the granted scopes against the role's rostered duty set", refs)
	}
	return clean(CheckAppScopes, fmt.Sprintf("all %d rostered duties covered", len(requiredDuties))+grantSidecarAge(tokenPath), refs)
}

// grantSidecarAge returns a short " (grant recorded <age> ago)" suffix for the app-scopes
// detail line, computed from the .perms sidecar's mtime, or "" if it cannot be read. The age
// is the diagnostic that tells a stale-grant remediation from a fresh one: a grant recorded
// long ago against a since-changed installation is exactly the #571 trap `--fresh` closes.
func grantSidecarAge(tokenPath string) string {
	if tokenPath == "" {
		return ""
	}
	fi, err := os.Stat(tokenPath + ".perms")
	if err != nil {
		return ""
	}
	return fmt.Sprintf(" (grant sidecar recorded %s ago)", roundAge(time.Since(fi.ModTime())))
}

// roundAge renders a duration to whole minutes (or seconds under a minute) for a human
// detail line — the sidecar age only needs coarse resolution to flag a stale grant.
func roundAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// levelSatisfies reports whether a granted permission level covers a required
// one. GitHub's App levels are ordered read < write < admin.
func levelSatisfies(granted, want string) bool {
	rank := map[string]int{"": 0, "read": 1, "write": 2, "admin": 3}
	g, ok := rank[strings.ToLower(strings.TrimSpace(granted))]
	if !ok {
		return false
	}
	w, ok := rank[strings.ToLower(strings.TrimSpace(want))]
	if !ok {
		return false
	}
	return g >= w
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(not granted)"
	}
	return s
}

// grantedScopesProbe reads the permission grant desktoken records next to the
// token cache at mint time (`<tokenPath>.perms`).
//
// The grant is only ever visible in the access-token RESPONSE, so the minter is
// the only component that can observe it. Recording it there and reading it here
// keeps the JWT-signing logic in one place instead of duplicating it into this
// package to ask GitHub the same question a second time.
func grantedScopesProbe(_ string, tokenPath string) (map[string]string, error) {
	path := tokenPath + ".perms"
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no recorded grant at %s: %w", path, err)
	}
	return parsePermsJSON(string(raw))
}

// parsePermsJSON reads the flat {"key":"value"} permission object desktoken
// writes. It is deliberately a tiny hand parser rather than encoding/json into
// map[string]any so a nested or unexpected shape is an ERROR (could-not-check)
// instead of a silently empty map that would read as "nothing granted".
func parsePermsJSON(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("recorded grant is not a JSON object")
	}
	out := map[string]string{}
	body := strings.TrimSpace(s[1 : len(s)-1])
	if body == "" {
		return out, nil
	}
	for _, pair := range strings.Split(body, ",") {
		k, v, ok := strings.Cut(pair, ":")
		if !ok {
			return nil, fmt.Errorf("recorded grant has a malformed entry %q", strings.TrimSpace(pair))
		}
		k = strings.Trim(strings.TrimSpace(k), `"`)
		v = strings.Trim(strings.TrimSpace(v), `"`)
		if k == "" {
			return nil, fmt.Errorf("recorded grant has an empty permission name")
		}
		out[k] = v
	}
	return out, nil
}

// --- check 3: write transport ----------------------------------------------

// checkWriteTransport probes the role's LANDING path before the pass starts,
// rather than discovering at landing time that no outward write transport is
// permitted (#823 — a pass verified two briefs to a clean PASS and could land
// neither).
//
// The probe is READ-ONLY by construction (see writeTransportProbe) and a
// rejection is a STOP: this function reports it and returns. There is no branch
// that tries a second credential — AGENTS.md, "A scope rejection is a STOP —
// never re-push the change under a different identity."
func checkWriteTransport(p PreflightProbes, l Landing) Check {
	const refs = "#823"
	verdict, detail, err := p.WriteTransport(l)
	if err != nil {
		return unchecked(CheckWriteTransport, oneLine(err.Error()),
			"run the landing probe by hand (`git -C "+orDot(l.Dir)+" push --dry-run "+l.Remote+" HEAD`) and read the transport error", refs)
	}
	switch verdict {
	case ProbePermitted:
		return clean(CheckWriteTransport, "landing transport permitted for "+l.Remote+" "+orCurrent(l.Branch)+
			" ("+oneLine(detail)+")", refs)
	case ProbeRejected:
		return failed(CheckWriteTransport, "landing transport REJECTED for "+l.Remote+" "+orCurrent(l.Branch)+
			": "+oneLine(detail),
			"STOP — do not retry under another identity. Obtain the write permission for THIS identity "+
				"(or run in a permission mode that admits the desk's landing verbs) and re-run preflight", refs)
	default:
		return unchecked(CheckWriteTransport, "probe inconclusive: "+oneLine(detail),
			"ensure a git remote named "+l.Remote+" exists and this worktree is on a branch (a detached HEAD has no landing ref)", refs)
	}
}

// writeTransportProbe is the real, NON-MUTATING write-transport probe:
// `git push --dry-run`.
//
// --dry-run does everything except send the update: it authenticates, contacts
// the remote's receive-pack and evaluates the ref update, then stops. That is
// exactly the read-only "would this land?" question, and it is NOT the
// permission LISTING that AGENTS.md warns is neither grant nor bar — a listing
// says what the docs claim, this says what the transport does.
//
// It runs ONCE. Every failure path below returns; none re-invokes git with a
// different credential.
func writeTransportProbe(l Landing) (ProbeVerdict, string, error) {
	dir := orDot(l.Dir)
	if _, err := exec.LookPath("git"); err != nil {
		return ProbeInconclusive, "git is not on PATH", nil
	}
	branch := strings.TrimSpace(l.Branch)
	if branch == "" {
		out, err := gitOut(dir, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return ProbeInconclusive, "detached HEAD: no landing branch to probe", nil
		}
		branch = strings.TrimSpace(out)
	}
	if _, err := gitOut(dir, "remote", "get-url", l.Remote); err != nil {
		return ProbeInconclusive, "no remote named " + l.Remote, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "push", "--dry-run", "--porcelain",
		l.Remote, "HEAD:refs/heads/"+branch)
	// GIT_TERMINAL_PROMPT=0: a probe must never block on an interactive
	// credential prompt — an unauthenticated transport has to FAIL, loudly.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	text := oneLine(string(out))
	if err == nil {
		return ProbePermitted, text, nil
	}
	if rejectionRe.MatchString(text) {
		return ProbeRejected, text, nil
	}
	return ProbeInconclusive, text, nil
}

// rejectionRe matches the transport answers that mean DENIED, as opposed to
// "could not reach a verdict". Both the git/GitHub wordings and the harness
// permission-classifier wording (#823) are here: from the desk's seat a
// classifier denial and a 403 are the same fact — this identity has no permitted
// write transport to its landing path.
var rejectionRe = regexp.MustCompile(`(?i)(permission denied|denied to |403|401|authentication failed|not authorized|resource not accessible|protected branch|pre-receive hook declined|refusing to allow|blocked by|permission to .* denied)`)

// --- check 4: commit identity ----------------------------------------------

// noreplyRe splits an App noreply commit email into its numeric prefix and its
// login half: "<bot-user-id>+<app-slug>[bot]@users.noreply.github.com".
var noreplyRe = regexp.MustCompile(`^(\d+)\+(.+)@users\.noreply\.github\.com$`)

// checkCommitIdentity proves this worktree would commit under an email whose
// numeric prefix is the role's BOT USER id — not its App id (#638).
//
// The difference is invisible locally and total remotely: with the bot user id,
// `gh api …/commits/<sha>` answers author.login=<slug>[bot], type=Bot; with the
// App id it answers author.login=null, type=null and the commit is attributed to
// nobody. #638 found weeks of Evidence commits in the second state, and the SKILL
// that prescribed it was itself wrong — which is why this is a CHECK and not a
// doc fix.
func checkCommitIdentity(p PreflightProbes, role, dir string) Check {
	const refs = "#638"
	ident, bound := EffectiveConfig().RoleBotIdentity(role)
	if !bound {
		return unchecked(CheckCommitIdentity, "the roster binds no App to role "+role,
			"add "+role+"=<forge>:<slug-or-login>[:<id>] to "+EnvTrustedBotSlugs+" in "+ConfigHomePath(), refs)
	}

	email, err := p.CommitEmail(dir)
	if err != nil {
		return unchecked(CheckCommitIdentity, oneLine(err.Error()), commitIdentityRemedy(ident, dir), refs)
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return failed(CheckCommitIdentity, "no commit author email is configured", commitIdentityRemedy(ident, dir), refs)
	}

	// The expected commit address is derived from the ENTRY's forge; neither forge
	// falls back to the other's shape. GitHub keeps the #638 exact-match logic; GitLab
	// matches the service-account noreply SHAPE (its group id / suffix are not derivable
	// from the roster — forgeidentity.go).
	if ident.Forge == ForgeGitLab {
		return checkGitLabCommitIdentity(ident, email, dir, refs)
	}
	return checkGitHubCommitIdentity(p, ident, role, email, dir, refs)
}

// checkGitHubCommitIdentity is the #638 commit-identity logic: the email must carry the
// bot USER id, not the App id (which lands the commit account-UNLINKED), and must name
// the role's own bound App.
func checkGitHubCommitIdentity(p PreflightProbes, ident BotIdentity, role, email, dir, refs string) Check {
	spec := ident.CommitEmailSpec()
	if !spec.Derivable {
		return unchecked(CheckCommitIdentity, "the roster pins no BOT USER id for "+ident.Slug,
			"pin it: "+EnvTrustedBotSlugs+" entry "+role+"=github:"+ident.Slug+":<bot-user-id> (the bot USER id, from `gh api /users/"+ident.Slug+"[bot]`)", refs)
	}
	want := spec.Exact
	if strings.EqualFold(email, want) {
		return clean(CheckCommitIdentity, "commit email carries the bot USER id ("+want+")", refs)
	}
	m := noreplyRe.FindStringSubmatch(email)
	if m == nil {
		return failed(CheckCommitIdentity, "commit email "+email+" is not the App noreply form",
			"git -C "+orDot(dir)+" config user.email "+want, refs)
	}
	prefix, login := m[1], m[2]
	if appID, aerr := p.AppIDFor(role); aerr == nil && strings.TrimSpace(appID) == prefix {
		return failed(CheckCommitIdentity,
			"commit email prefix "+prefix+" is the APP id, not the bot USER id — commits land account-UNLINKED (author.login=null)",
			"git -C "+orDot(dir)+" config user.email "+want, refs)
	}
	if !strings.EqualFold(login, ident.Slug+"[bot]") {
		return failed(CheckCommitIdentity, "commit email names "+login+", but role "+role+" is bound to "+ident.Slug+"[bot]",
			"git -C "+orDot(dir)+" config user.email "+want, refs)
	}
	return failed(CheckCommitIdentity, "commit email prefix "+prefix+" is neither the bot USER id ("+
		strconv.FormatInt(ident.ID, 10)+") nor a recognised id",
		"git -C "+orDot(dir)+" config user.email "+want, refs)
}

// checkGitLabCommitIdentity validates a GitLab service-account commit address by SHAPE.
// The group id and per-account suffix are not in the roster, so the shape is the tightest
// available check — but a GitHub noreply address for a GitLab entry is a hard failure
// (the cross-forge case), never a fall-through that a skipped check would let pass.
func checkGitLabCommitIdentity(ident BotIdentity, email, dir, refs string) Check {
	if ident.CommitEmailSpec().Accepts(email) {
		return clean(CheckCommitIdentity, "commit email is the GitLab service-account noreply form ("+email+
			"); the group id and per-account suffix are not derivable from the roster, so the shape is the "+
			"tightest available check", refs)
	}
	if noreplyRe.MatchString(email) {
		return failed(CheckCommitIdentity,
			"commit email "+email+" is the GitHub noreply form, but role's identity is a GitLab service "+
				"account — a GitHub-shaped address for a GitLab entry lands the commit under no GitLab identity",
			commitIdentityRemedy(ident, dir), refs)
	}
	return failed(CheckCommitIdentity,
		"commit email "+email+" is not the GitLab service-account noreply form "+
			"(service_account_group_<group-id>_<suffix>@noreply.<host>)",
		commitIdentityRemedy(ident, dir), refs)
}

// commitIdentityRemedy renders the forge-appropriate fix for a missing or wrong commit
// author email. GitHub can name the exact address to set; GitLab cannot (the roster does
// not carry the group id / suffix), so it points at the provisioned service-account form.
func commitIdentityRemedy(ident BotIdentity, dir string) string {
	spec := ident.CommitEmailSpec()
	if spec.Forge == ForgeGitHub && spec.Derivable {
		return "git -C " + orDot(dir) + " config user.email " + spec.Exact
	}
	if spec.Forge == ForgeGitLab {
		return "set this worktree's user.email to the " + ident.Slug + " GitLab service-account noreply " +
			"address (service_account_group_<group-id>_<suffix>@noreply.<host>) provisioned for it"
	}
	return "pin the bot USER id in " + EnvTrustedBotSlugs + " for " + ident.Slug +
		", then set this worktree's user.email to the resulting noreply address"
}

// commitEmailProbe reads the effective commit author email: GIT_AUTHOR_EMAIL
// wins (it is what a `git -c`-free commit in this process would actually use),
// then the worktree's git config.
func commitEmailProbe(dir string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("GIT_AUTHOR_EMAIL")); v != "" {
		return v, nil
	}
	out, err := gitOut(orDot(dir), "config", "--get", "user.email")
	if err != nil {
		return "", fmt.Errorf("no commit author email: git config user.email is unset in %s", orDot(dir))
	}
	return strings.TrimSpace(out), nil
}

// --- check 5: sibling checkouts --------------------------------------------

// checkSiblings proves the sibling checkouts the QUEUED briefs declare are
// actually present, before a pass claims a brief whose rows cannot run (#679 —
// two rows deferred mid-verify for want of a co-located sibling checkout).
func checkSiblings(p PreflightProbes, root string) Check {
	const refs = "#679"
	reqs, err := p.QueuedSiblings(root)
	if err != nil {
		return unchecked(CheckSiblings, oneLine(err.Error()),
			"run preflight from a repo root that has docs/streams/ (or pass --root), so the queue can be read", refs)
	}
	if len(reqs) == 0 {
		return clean(CheckSiblings, "no queued brief declares an out-of-repo checkout", refs)
	}
	var missing []string
	for _, r := range reqs {
		abs := r.Rel
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, r.Rel)
		}
		ok, derr := p.DirExists(abs)
		if derr != nil {
			return unchecked(CheckSiblings, "cannot stat "+abs+" (declared by "+r.Brief+"): "+oneLine(derr.Error()),
				"make "+abs+" readable, or correct the out-of-repo declaration in "+r.Brief, refs)
		}
		if !ok {
			missing = append(missing, r.Rel+" (declared by "+r.Brief+")")
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return failed(CheckSiblings, "declared sibling checkout(s) absent: "+strings.Join(missing, "; "),
			"clone or `git worktree add` the missing checkout(s) next to "+root+
				" before claiming those briefs — a row that cannot run must not be claimed", refs)
	}
	return clean(CheckSiblings, fmt.Sprintf("%d declared sibling checkout(s) present", len(reqs)), refs)
}

// siblingRe extracts a sibling checkout root from an out-of-repo declaration:
// "../tracker/docs/x.md" → "../tracker". Only the ../<name> head is taken — the check is
// "is the checkout there", not "is that one file there".
var siblingRe = regexp.MustCompile(`\.\./([A-Za-z0-9][A-Za-z0-9._-]*)`)

// awaitingRe matches a stream-README brief row whose Status is in the Awaiting
// queue. Rows outside the queue are NOT scanned: a sibling declared by a brief
// nobody is about to claim is not an envelope failure, and treating it as one
// would make preflight red for work that is not queued.
var awaitingRe = regexp.MustCompile(`(?i)\|\s*(implemented|verified)\s*\|`)

// QueuedSiblings reads the sibling checkouts declared by the briefs currently in
// the Awaiting queue under <root>/docs/streams/.
//
// It reads ONLY the brief schema's `out-of-repo files:` declaration — not the
// whole body. A brief that mentions ../<repo> inside an Evidence table or a
// Verify command has not DECLARED a dependency, and counting those would red the
// preflight on prose.
func QueuedSiblings(root string) ([]SiblingReq, error) {
	streams := filepath.Join(root, "docs", "streams")
	entries, err := os.ReadDir(streams)
	if err != nil {
		return nil, fmt.Errorf("cannot read the brief queue at %s: %w", streams, err)
	}
	seen := map[string]bool{}
	var out []SiblingReq
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		readme, rerr := os.ReadFile(filepath.Join(streams, e.Name(), "README.md"))
		if rerr != nil {
			continue // a stream dir without a README carries no queue
		}
		queued := map[string]bool{}
		for _, line := range strings.Split(string(readme), "\n") {
			if !awaitingRe.MatchString(line) {
				continue
			}
			cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
			if len(cells) == 0 {
				continue
			}
			if num := strings.TrimSpace(cells[0]); num != "" {
				queued[num] = true
			}
		}
		for num := range queued {
			matches, _ := filepath.Glob(filepath.Join(streams, e.Name(), "brief-"+num+"-*.md"))
			for _, m := range matches {
				raw, berr := os.ReadFile(m)
				if berr != nil {
					continue
				}
				rel := m
				if r, rerr := filepath.Rel(root, m); rerr == nil && r != "" {
					rel = r
				}
				for _, sib := range declaredSiblings(string(raw)) {
					key := rel + "\x00" + sib
					if seen[key] {
						continue
					}
					seen[key] = true
					out = append(out, SiblingReq{Brief: rel, Rel: sib})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Brief != out[j].Brief {
			return out[i].Brief < out[j].Brief
		}
		return out[i].Rel < out[j].Rel
	})
	return out, nil
}

// declaredSiblings pulls the ../<name> roots out of a brief's `out-of-repo
// files:` declaration. "none" (the overwhelmingly common value) yields nothing.
func declaredSiblings(brief string) []string {
	var out []string
	seen := map[string]bool{}
	lines := strings.Split(brief, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(line)), "out-of-repo files:") {
			continue
		}
		block := []string{line}
		// A list-form declaration continues onto following "- " bullet lines.
		for _, next := range lines[i+1:] {
			t := strings.TrimSpace(next)
			if !strings.HasPrefix(t, "- ") {
				break
			}
			block = append(block, next)
		}
		joined := strings.Join(block, "\n")
		if strings.Contains(strings.ToLower(joined), "none") && !siblingRe.MatchString(joined) {
			continue
		}
		for _, m := range siblingRe.FindAllStringSubmatch(joined, -1) {
			sib := "../" + m[1]
			if seen[sib] {
				continue
			}
			seen[sib] = true
			out = append(out, sib)
		}
	}
	sort.Strings(out)
	return out
}

func dirExistsProbe(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return fi.IsDir(), nil
}

// --- small shared helpers ---------------------------------------------------

func gitOut(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.CommandContext(ctx, "git", full...).Output()
	return string(out), err
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

func orDot(s string) string {
	if strings.TrimSpace(s) == "" {
		return "."
	}
	return s
}

func orCurrent(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(current branch)"
	}
	return s
}
