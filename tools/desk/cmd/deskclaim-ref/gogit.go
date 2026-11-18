package main

// gogit.go — the forge-ref store, one implementation of deskkit.ClaimStore: an in-process git-smart-HTTP transport over go-git
// (internal/gitcore), the ONLY forge access this binary makes. There is no `gh`, `glab`, or any
// CLI here and no external `git` process — a dispatch claim is placed, advanced, stolen, read
// and released as library calls that speak the git wire protocol directly. That is what closes
// the forge-surface violation the ban (internal/forgeban) exists to catch, and it is why this
// path runs native on Windows: it is a plain executable making HTTPS calls, with no shell, no
// shebang, and no bash association to depend on.
//
// The forge (github vs gitlab), the host, the owner/name and the credential are all resolved
// WITHOUT minting anything: the forge kind comes from the roster's ASSAY_REPO_FORGES or the
// origin host (deskkit.ForgeKindFromSlugAndHost, the token-free resolver — never ForgeFor/
// ResolveForge, which read App-token custody), and the token comes from --token-file or the
// GH_TOKEN/GITHUB_TOKEN/GITLAB_TOKEN environment, exactly the ambient credential the bash script
// and the old gh-CLI port used. So this changes the TRANSPORT, not WHO the claim is placed as.
//
// CLOCK — a documented behaviour change from the gh-CLI port. That port omitted the tagger so
// GitHub stamped tagger.date server-side, giving one clock across machines. go-git mints the tag
// locally, so the tagger date is now the CLIENT clock. This is sound: the claim's mutual
// exclusion rests on the SERVER-SIDE compare-and-swap (an explicit-old ref update the forge
// accepts or rejects), not on the timestamp — and the script's own header states clock skew "is
// not an authorization boundary". The timestamp drives only the TTL age display and the
// stale-reclaim heuristic, both tolerant of ordinary skew.

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// storeTimeout bounds every forge round trip so a hung remote fails closed (as
// could-not-check) rather than wedging a dispatcher.
const storeTimeout = 30 * time.Second

// gogitStore is the live forge seam. url is the full HTTPS git URL built from a
// roster-validated slug + host; auth is the git-basic credential (token in memory only).
//
// host and lastErr exist for ATTRIBUTION: a fail-closed (exit 6) transport failure used to
// collapse to a bare "unverifiable" naming neither the host dialed nor the auth/DNS cause, so an
// operator on a self-hosted GitLab debugged the claim namespace, token scopes and ref
// permissions — all of which were fine — while the real fault was the SaaS host the tool silently
// dialed (#727). Each transport failure records its cause here, and so does a server's refusal
// of a claim write (its report-status text, #1631); the verb layer reads TransportCause() to
// append "<host>: <error>" to the message.
type gogitStore struct {
	url     string
	auth    transport.AuthMethod
	host    string // the host this store dials, for fail-closed attribution
	lastErr error  // the most recent transport error or server write refusal, for attribution
}

// gogitStore is the forge-ref implementation of the deskkit claim-store seam.
var _ deskkit.ClaimStore = (*gogitStore)(nil)

// newForgeStore resolves the forge, host, and credential for repo (an "owner/name" slug) and
// returns a deskkit.ClaimStore bound to them, or an error (which the caller maps to exit 6,
// could-not-check). tokenFile, when non-empty, is the 0600 file the token is read from instead
// of the environment.
func newForgeStore(repo, tokenFile string, readOrigin func() (string, error)) (deskkit.ClaimStore, error) {
	owner, name, ok := splitSlug(repo)
	if !ok {
		return nil, fmt.Errorf("repo %q is not an owner/name slug", repo)
	}
	origin, err := readOrigin()
	if err != nil {
		return nil, fmt.Errorf("cannot determine claim forge host from origin: %w", err)
	}
	originHost, err := deskkit.HostOfRemote(origin)
	if err != nil || !deskkit.IsBareGitHost(originHost) {
		// The origin can contain userinfo, including when its URL is malformed.
		return nil, fmt.Errorf("claim origin has no valid ASCII forge hostname")
	}
	if strings.Contains(origin, "://") {
		u, err := url.Parse(origin)
		if err != nil {
			return nil, fmt.Errorf("claim origin URL is malformed")
		}
		if u.Scheme == "https" && u.Port() != "" && u.Port() != "443" {
			return nil, fmt.Errorf("claim store requires the default HTTPS port")
		}
	}
	if deskkit.IsSSHTransport(origin) {
		originHost, err = resolveHostAlias(originHost)
		if err != nil {
			return nil, err
		}
	}
	kind, host, err := deskkit.ForgeKindFromSlugAndHost(repo, originHost)
	if err != nil {
		return nil, err
	}
	if host == "" {
		return nil, fmt.Errorf("no host known for forge %q serving %s", kind, repo)
	}
	token, err := resolveToken(tokenFile)
	if err != nil {
		return nil, err
	}
	username := gitcore.GitHubGitUsername
	if kind == deskkit.ForgeGitLab {
		username = gitcore.GitLabGitUsername
	}
	return &gogitStore{
		url:  "https://" + host + "/" + owner + "/" + name + ".git",
		auth: gitcore.BasicAuthAs(username, token),
		host: host,
	}, nil
}

// fail records err as the store's most recent cause, so the verb layer can attribute a
// fail-closed exit to the host dialed and the underlying error rather than emitting a bare
// "unverifiable" (#727). err is a transport-layer failure (callers then return the
// ClaimWriteUnverifiable/ClaimReadUnverifiable sentinel) or the server's refusal of a write
// (mintAndPush then returns ClaimWriteRejected, #1631). It returns nothing.
func (g *gogitStore) fail(err error) { g.lastErr = err }

// TransportCause reports "<host>: <error>" for the store's most recently recorded cause — a
// transport failure or a server refusal of a write — or "" when none has been recorded. The verb
// layer appends it to a fail-closed message so the operator sees WHERE the tool dialed and WHY it
// failed or was refused.
func (g *gogitStore) TransportCause() string {
	if g.lastErr == nil {
		return ""
	}
	return g.host + ": " + g.lastErr.Error()
}

func (g *gogitStore) refName(id string) plumbing.ReferenceName {
	return plumbing.ReferenceName(refPrefix + "/" + id)
}

func (g *gogitStore) Read(id string) (deskkit.ClaimStoreRecord, deskkit.ClaimReadStatus) {
	refs, err := gitcore.List(gitcore.ListOpts{URL: g.url, Auth: g.auth})
	if err != nil {
		g.fail(err)
		return deskkit.ClaimStoreRecord{}, deskkit.ClaimReadUnverifiable
	}
	sha, found := findRef(refs, refPrefix+"/"+id)
	if !found {
		// A clean advertisement with no matching ref is FREE (the brief's rule); only a
		// transport/auth/not-found error above is could-not-check.
		return deskkit.ClaimStoreRecord{}, deskkit.ClaimReadFree
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	payload, perr := gitcore.FetchTagPayload(ctx, g.url, g.auth, plumbing.NewHash(sha))
	if perr != nil {
		g.fail(perr)
		return deskkit.ClaimStoreRecord{}, deskkit.ClaimReadUnverifiable
	}
	return deskkit.ClaimStoreRecord{
		Version: sha,
		Msg:     strings.TrimRight(payload.Message, "\n"),
		Date:    payload.When.UTC().Format(time.RFC3339),
	}, deskkit.ClaimReadHeld
}

func (g *gogitStore) CreateIfAbsent(id, msg string) deskkit.ClaimWriteOutcome {
	return g.mintAndPush(id, msg, plumbing.ZeroHash)
}

func (g *gogitStore) UpdateFrom(id, oldSHA, msg string) deskkit.ClaimWriteOutcome {
	return g.mintAndPush(id, msg, plumbing.NewHash(oldSHA))
}

func (g *gogitStore) mintAndPush(id, msg string, old plumbing.Hash) deskkit.ClaimWriteOutcome {
	objs, tagSHA, err := gitcore.MintClaimTag(id, msg, time.Now())
	if err != nil {
		g.fail(err)
		return deskkit.ClaimWriteUnverifiable
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	res, reason, perr := gitcore.PushRefUpdateDetail(ctx, gitcore.RefUpdate{
		URL: g.url, Auth: g.auth, Ref: g.refName(id), Old: old, New: tagSHA, Objects: objs, Trace: claimTrace,
	})
	if perr != nil {
		g.fail(perr)
		return deskkit.ClaimWriteUnverifiable
	}
	if res == gitcore.RefUpdateRejected {
		// Keep the server's own refusal text for attribution: when the verb then finds no
		// holder, "rejected but no claim exists" must say WHY the server refused.
		g.fail(fmt.Errorf("server refused %s: %s", g.refName(id), reason))
		return deskkit.ClaimWriteRejected
	}
	return deskkit.ClaimWriteApplied
}

func (g *gogitStore) Remove(id string) (deskkit.ClaimWriteOutcome, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	res, err := gitcore.DeleteRefTrace(ctx, g.url, g.auth, g.refName(id), claimTrace)
	if err != nil {
		g.fail(err)
		return deskkit.ClaimWriteUnverifiable, false
	}
	return deskkit.ClaimWriteApplied, res == gitcore.DeleteDone
}

func (g *gogitStore) List() ([]string, deskkit.ClaimReadStatus) {
	refs, err := gitcore.List(gitcore.ListOpts{URL: g.url, Auth: g.auth})
	if err != nil {
		g.fail(err)
		return nil, deskkit.ClaimReadUnverifiable
	}
	var ids []string
	for _, r := range refs {
		name := r.Name().String()
		if strings.HasSuffix(name, "^{}") {
			continue // the peeled companion of an annotated tag — the tag ref carries the sha
		}
		if id, ok := strings.CutPrefix(name, refPrefix+"/"); ok {
			ids = append(ids, id)
		}
	}
	return ids, deskkit.ClaimReadHeld
}

func (g *gogitStore) BranchExists(branch string) (bool, bool) {
	if branch == "" || branch == "-" {
		return false, true
	}
	refs, err := gitcore.List(gitcore.ListOpts{URL: g.url, Auth: g.auth})
	if err != nil {
		g.fail(err)
		return false, false
	}
	_, found := findRef(refs, "refs/heads/"+branch)
	return found, true
}

// findRef returns the sha of the exact ref name (skipping the peeled `^{}` companions), so an
// annotated tag resolves to its TAG OBJECT sha — the value the CAS `old` must carry — not its
// peeled target.
func findRef(refs []*plumbing.Reference, name string) (string, bool) {
	for _, r := range refs {
		if r.Name().String() == name {
			return r.Hash().String(), true
		}
	}
	return "", false
}

// --- repo / token resolution ------------------------------------------------

// An explicit repo chooses which claim namespace to use. The forge store still
// needs the checked origin to determine which instance serves that namespace.
func resolveRepo(repoFlag string, readOrigin func() (string, error)) (string, error) {
	if s := strings.TrimSpace(repoFlag); s != "" {
		return s, nil
	}
	origin, err := readOrigin()
	if err != nil {
		return "", err
	}
	return deskkit.RemoteRepoSlug(origin)
}

func checkedOriginURL(dir string) (string, error) {
	top, err := gitcore.Toplevel(dir)
	if err != nil {
		return "", err
	}
	repo, err := gitcore.Open(top)
	if err != nil {
		return "", err
	}
	return deskkit.SlugRemoteURL(repo, "origin")
}

// sshConfigHostname resolves an OpenSSH `Host` alias to the real hostname its config maps it
// to, by shelling `ssh -G <alias>` and reading the `hostname` line from its (fully-resolved,
// lower-cased) output — exactly the lookup the `ssh` child real `git` invokes performs
// implicitly for an scp-like origin. It is a package var so a test can stub it without a real
// `~/.ssh/config` or an `ssh` binary on PATH. Empty output or a non-zero exit is reported as an
// error; this function never guesses.
//
// The `--` before alias is load-bearing, not decorative: alias is the host segment parsed out
// of a git remote URL — attacker-influenceable input (a crafted `origin`, or a claim resolved
// against a repo/checkout this process does not fully control). Without `--`, a value shaped
// like a flag (`-oProxyCommand=...` is the textbook ssh argv-injection payload) would be parsed
// by `ssh` as an OPTION rather than a positional hostname: exec.Command invokes `ssh` directly
// with no shell, so there is no shell-quoting fault here, but `ssh` still does its own argv flag
// parsing on whatever argument lands in that position. `--` is OpenSSH's own end-of-options
// marker, so everything after it is forced positional regardless of its shape. resolveHostAlias
// below additionally refuses a leading-`-` alias before ever reaching this function, as a second,
// independent layer that holds even if a future edit here ever dropped the `--`.
var sshConfigHostname = func(alias string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", sshGConfigArgv(alias)...)
	outb, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ssh -G -- %s: %w", alias, err)
	}
	sc := bufio.NewScanner(strings.NewReader(string(outb)))
	for sc.Scan() {
		line := strings.Trim(sc.Text(), " \t\r\n")
		if h, ok := strings.CutPrefix(line, "hostname "); ok {
			if h = strings.Trim(h, " \t\r\n"); h != "" {
				return h, nil
			}
		}
	}
	return "", fmt.Errorf("ssh -G -- %s printed no hostname line", alias)
}

// sshGConfigArgv builds the literal argv `ssh -G` runs with — separated out from
// sshConfigHostname so the argv SHAPE is assertable without executing a real `ssh` process
// (TestSSHGConfigArgvCarriesEndOfOptionsMarker). alias always lands as argv[2], strictly after
// the `--` end-of-options marker at argv[1], so `ssh`'s own getopt-style argument parser can
// never treat it as a flag regardless of what it starts with — this is what makes the leading-
// dash refusal in resolveHostAlias a second, redundant layer rather than the only one.
func sshGConfigArgv(alias string) []string {
	return []string{"-G", "--", alias}
}

// resolveHostAlias resolves host to a dialable forge hostname when it looks like an SSH config
// `Host` alias rather than a DNS name — issue #1371: an scp-like origin (`git@alias:owner/name`)
// with an alias mapping to a real forge host and a per-identity key. HostOfRemote takes the
// literal host segment of such a URL, and ForgeKindFromSlugAndHost (by design, #727) never
// substitutes a SaaS default for it, so an unresolved alias would otherwise be dialed verbatim
// as if it were a DNS name and fail with a bare "no such host".
//
// The test here is deliberately narrow: an alias is never a DNS name, so it never contains a
// dot (`git@github.com:...` has one; `git@work-github:...` does not). host == "" or already
// containing a dot is returned unchanged — this function has nothing to add to either case, and
// leaves them to the caller / ForgeKindFromSlugAndHost as before.
//
// A host beginning with `-` is refused HERE, before sshConfigHostname (or any exec.Command) is
// ever reached: the host segment comes from parsing a git remote URL, so it is untrusted, and a
// leading `-` is what an ssh argv-injection payload (`-oProxyCommand=...`) needs to be mistaken
// for a flag. sshConfigHostname independently passes `--` to `ssh -G` so this can never reach an
// option position even without this check; the two are deliberately redundant layers, not one
// control restated (a dash-leading alias is nonsensical as a `Host` name in any case, so refusing
// it costs nothing real).
//
// Resolution shells `ssh -G <host>` and takes its `hostname` line — the same source real `git`
// consults via the `ssh` child it launches for an scp-like origin, so this honours whatever the
// operator already has in `~/.ssh/config` without this tool ever reading that file itself. When
// the resolver is unavailable or yields nothing, this keeps today's fail-closed refusal — but
// the error NAMES the alias and the resolution attempted, per the issue's fix shape, rather than
// collapsing to a bare "no such host" once the caller later tries to dial the alias directly.
func resolveHostAlias(host string) (string, error) {
	h := strings.TrimSpace(host)
	if h == "" || strings.ContainsAny(h, ".:") {
		return h, nil
	}
	if strings.HasPrefix(h, "-") {
		return "", fmt.Errorf(
			"origin host %q looks like an SSH config Host alias, but it starts with %q, which is never a "+
				"legitimate Host name and is refused outright rather than risked as an argument to `ssh -G` "+
				"(an alias shaped like an ssh flag, e.g. -oProxyCommand=..., is an argv-injection attempt, not "+
				"a real Host block). Point origin at the real forge host directly.",
			h, "-")
	}
	resolved, err := sshConfigHostname(h)
	if err != nil {
		return "", fmt.Errorf(
			"origin host %q looks like an SSH config Host alias (no dot, so it cannot be a DNS name), but "+
				"it could not be resolved to a real hostname: tried `ssh -G -- %s` and got: %s. Add a Host "+
				"block for %s in ~/.ssh/config with a HostName, or point origin at the real forge host "+
				"directly.",
			h, h, err.Error(), h)
	}
	if !deskkit.IsBareGitHost(resolved) {
		return "", fmt.Errorf("SSH alias did not resolve to a bare forge hostname")
	}
	return deskkit.NormalizeForgeHost(resolved)
}

func splitSlug(slug string) (owner, name string, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(slug), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// resolveToken reads the forge credential from --token-file (a 0600 file) when given, else the
// first non-empty of GH_TOKEN / GITHUB_TOKEN / GITLAB_TOKEN — the same ambient credential the
// script and the gh-CLI port used. It never mints and never falls back to a CLI's cached login.
func resolveToken(tokenFile string) (string, error) {
	if tf := strings.TrimSpace(tokenFile); tf != "" {
		if err := verifyTokenFileMode(tf); err != nil {
			return "", err
		}
		b, err := os.ReadFile(tf)
		if err != nil {
			return "", fmt.Errorf("cannot read --token-file %s: %w", tf, err)
		}
		tok := strings.TrimSpace(string(b))
		if tok == "" {
			return "", fmt.Errorf("--token-file %s is empty", tf)
		}
		return tok, nil
	}
	for _, env := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("no forge token: pass --token-file <0600 file> or set GH_TOKEN/GITHUB_TOKEN/GITLAB_TOKEN")
}

// verifyTokenFileMode enforces that a --token-file is a regular file locked to its owner (0600),
// so a world- or group-readable credential is refused rather than silently trusted. The
// permission-bit test is skipped on Windows, where os.FileMode's bits are synthetic.
func verifyTokenFileMode(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat --token-file %s: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("--token-file %s is not a regular file (mode %s)", path, fi.Mode())
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("--token-file %s is group/world accessible (mode %s); it must be 0600", path, fi.Mode().Perm())
	}
	return nil
}

// claimTrace emits only the bounded protocol receipt, never payloads or credentials.
func claimTrace(r gitcore.RefReceipt) {
	if !deskkit.TraceEnabled() {
		return
	}
	fmt.Fprintln(errOut, deskkit.Scrub(fmt.Sprintf("deskclaim-ref trace: ref=%q old=%s new=%s advertised=%s pack_sha256=%s pack_bytes=%d pack_objects=%d caps=%q phase=%s verdict=%s http_status=%d request_id=%q", traceRef(r.Ref), r.Old, r.New, r.Advertised, r.PackSHA256, r.PackBytes, r.PackObjects, r.Capabilities, r.Phase, r.Verdict, r.HTTPStatus, r.RequestID)))
}

func traceRef(ref string) string {
	ref = deskkit.Scrub(ref)
	if len(ref) > 256 {
		return ref[:256] + "..."
	}
	return ref
}
