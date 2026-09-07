package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ---- human-acceptance / human-ruling CITATION corroboration -----------------------
//
// This is the SECOND consumer of the corroborate machinery, alongside the
// human:<name> STAMP check above (corroborate.go). It closes a different laundering
// surface than the stamp gate, and it is deliberately homed here rather than in a
// new tool or in the offline --lint core:
//
//   - It reuses HumanLogin (name -> GitHub login), the gh comment/review shapes, and
//     the same "the named human must have ACTED" corroboration idea. A parallel
//     implementation is how the two drift apart.
//   - The corroboration target is a LIVE, possibly CROSS-REPO read of the CITED
//     issue/PR, so it belongs on the network-capable --corroborate verb and NOT in
//     the offline, deterministic --lint gate (every --lint check is side-effect-free
//     and must never gain a hard network dependency — see main.go).
//
// WHAT IT CATCHES. A worker writes an authority-bearing claim into a durable
// artifact — tracked prose (a runbook, a brief, a register) or a commit message —
// that a named human ACCEPTED / RULED ON something, e.g.
//
//	"<name> accepted this contradiction with the runbook on #1583"
//	"per <name>'s ruling ('Option (a) is the way')"     (in a commit message)
//
// when NO artifact authored by that human exists on the cited issue/PR (0 comments,
// 0 reviews). That launders a non-existent human authorisation into the permanent
// record: a later reader — human or agent — has no cheap way to tell an invented
// acceptance from a real one, and the natural move is to trust the citation. This is
// the human-gate forgery class one surface over from the human:<name> register cell
// the stamp gate already guards.
//
// WHAT IT IS NOT. It does NOT re-scan human:<name> STAMPS in Verified/Reviewed cells
// — that is the stamp gate's job (corroborateStamps / humanStampProblems), and
// duplicating it here would be two gates disagreeing about one token. It only reads
// FREE-PROSE acceptance/ruling claims.
//
// THE DETECTION IS ANCHORED ON CONFIGURED HUMANS, on purpose. A candidate name is
// kept only when it resolves through HumanLogin — i.e. it is a name an adopter has
// declared to be a human whose acceptance carries authority (ASSAY_HUMAN_LOGIN_MAP,
// the same outside-every-ref config surface as the trust roster). This is what keeps
// the check neutral (it hardcodes no real name) and its false-positive rate low (an
// ordinary sentence like "the compiler approved the change" names no configured
// human, so it is ignored). The residual, stated plainly: a citation of a name the
// adopter has NOT mapped escapes — but an unmapped name is not a recognised
// authority in the first place, so citing it launders no RECOGNISED authorisation.
// Like the rest of this surface, an EMPTY map is the strict-but-inert direction:
// with no configured humans there are no names to anchor on, so nothing is detected
// (the mechanism ships public; the names are private adopter config).

// citedHumanLogin resolves a name AS WRITTEN IN A CITATION to the GitHub login whose
// artifacts corroborate it. A prose citation may name a human either by the
// configured NAME (the ASSAY_HUMAN_LOGIN_MAP key, e.g. "alice") or directly by their
// GitHub LOGIN (the map value, e.g. "alice-gh"); both must resolve, because both are
// how a real authority gets cited and the check must not miss the login form. It
// resolves in that order:
//
//   - name is a configured key      -> its mapped login (HumanLogin);
//   - name is itself a mapped login -> that login (the citation named the account).
//
// An UNSET map resolves nothing (the strict-but-inert direction). This is a widening
// of recognition ONLY: like HumanLogin it is consulted to ACCEPT a candidate as a
// citation of a real human, never to grant authority — the corroboration still
// requires that human's artifact on the cited issue/PR.
func citedHumanLogin(name string) (login string, ok bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if l, found := HumanLogin(n); found {
		return l, true
	}
	for _, l := range scanEffectiveConfig().HumanLogins {
		if strings.EqualFold(l, n) {
			return l, true
		}
	}
	return "", false
}

// citationAcceptanceRe matches "<name> <acceptance-verb>" — a named actor asserted to
// have accepted/approved/ruled/decided. The captured group 1 is the candidate name;
// it is only treated as a citation once HumanLogin resolves it to a login.
//
// The name class is a single identifier-like token so the match cannot run across a
// clause boundary; a leading word boundary keeps "inhuman approved" from capturing
// "inhuman" as a name (it would simply fail the HumanLogin lookup, but not matching
// it at all is cheaper and clearer).
var citationAcceptanceRe = regexp.MustCompile(`(?i)\b([A-Za-z][A-Za-z0-9_-]*)\s+(accepted|approved|ruled|decided|acknowledged|signed[-\s]?off)\b`)

// citationPossessiveRe matches "<name>'s <ruling-noun>" — "per <name>'s ruling",
// "<name>'s decision". Group 1 is the candidate name; group 2 the noun (kept for the
// report). It accepts a straight or curly apostrophe and an optional trailing "s"
// nowhere — the possessive marker is required so a bare "<name> ruling" (which the
// verb pattern already covers as a different sense) is not double-matched here.
var citationPossessiveRe = regexp.MustCompile(`(?i)\b([A-Za-z][A-Za-z0-9_-]*)['’]s\s+(ruling|decision|approval|sign[-\s]?off|acceptance)\b`)

// citedRefRe finds an explicit owner/repo#N cross-repo reference on a line.
var citedRefRe = regexp.MustCompile(`\b([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#(\d+)\b`)

// citedBareRe finds a bare #N reference (same-repo) not preceded by the "/" of an
// owner/repo#N form. The leading boundary class excludes "/" so it does not
// re-capture the number half of an owner/repo#N that citedRefRe already took.
var citedBareRe = regexp.MustCompile(`(?:^|[^/\w])#(\d+)\b`)

// citedURLRe finds a GitHub issue/PR URL and pulls out owner, repo and number.
var citedURLRe = regexp.MustCompile(`https?://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/(?:issues|pull)/(\d+)`)

// citation is one detected human-acceptance / human-ruling citation.
type citation struct {
	Name   string // configured human name (lowercased); resolves via HumanLogin
	Marker string // the acceptance word / ruling noun that matched, for the report
	Repo   string // cited artifact repo "owner/repo"; "" means "the PR's own repo"
	Number int    // cited issue/PR number; 0 means none cited (unlinked)
	HasRef bool   // whether an explicit #N / URL reference was found
	Source string // where it was found (a file path, or "commit <shortsha>")
	Line   string // the citation text (truncated) for context
}

// citedKey is the map key a citation's corroboration data is looked up under. It is
// the fully-qualified "owner/repo#N". An unlinked citation (no ref) has no key.
func (c citation) citedKey(prRepo string) string {
	repo := c.Repo
	if repo == "" {
		repo = prRepo
	}
	return fmt.Sprintf("%s#%d", repo, c.Number)
}

// extractCitedRef returns the first artifact reference on a line, preferring the
// most-specific form: an explicit owner/repo#N, then a full GitHub URL, then a bare
// #N (resolved against the PR's own repo by the caller). ok is false when the line
// carries no reference at all — an UNLINKED acceptance claim.
func extractCitedRef(line string) (repo string, number int, ok bool) {
	if m := citedRefRe.FindStringSubmatch(line); m != nil {
		n, _ := strconv.Atoi(m[2])
		return m[1], n, true
	}
	if m := citedURLRe.FindStringSubmatch(line); m != nil {
		n, _ := strconv.Atoi(m[3])
		return m[1] + "/" + m[2], n, true
	}
	if m := citedBareRe.FindStringSubmatch(line); m != nil {
		n, _ := strconv.Atoi(m[1])
		return "", n, true // same-repo; caller substitutes the PR repo
	}
	return "", 0, false
}

// detectCitations scans one text source (a file's added lines already concatenated,
// or a commit message) for human-acceptance / human-ruling citations naming a
// CONFIGURED human. Only names HumanLogin resolves are kept — see the file header.
func detectCitations(source, text string) []citation {
	var out []citation
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		type hit struct{ name, marker string }
		var hits []hit
		for _, m := range citationAcceptanceRe.FindAllStringSubmatch(line, -1) {
			hits = append(hits, hit{strings.ToLower(m[1]), strings.ToLower(m[2])})
		}
		for _, m := range citationPossessiveRe.FindAllStringSubmatch(line, -1) {
			hits = append(hits, hit{strings.ToLower(m[1]), strings.ToLower(m[2])})
		}
		if len(hits) == 0 {
			continue
		}
		repo, number, hasRef := extractCitedRef(line)
		lineCtx := line
		if len(lineCtx) > 120 {
			lineCtx = lineCtx[:120] + "..."
		}
		for _, h := range hits {
			// Anchor: only a configured human's acceptance carries authority worth
			// corroborating. An unresolved name is ordinary prose ("the tool
			// approved…") and is deliberately ignored. The name may be the
			// configured key OR the GitHub login itself (citedHumanLogin).
			if _, known := citedHumanLogin(h.name); !known {
				continue
			}
			out = append(out, citation{
				Name:   h.name,
				Marker: h.marker,
				Repo:   repo,
				Number: number,
				HasRef: hasRef,
				Source: source,
				Line:   lineCtx,
			})
		}
	}
	return dedupeCitations(out)
}

// dedupeCitations collapses citations that name the same human, cite the same
// (repo, number) and share a source — multiple restatements on nearby lines count
// once. The line text of the first occurrence is kept for context.
func dedupeCitations(in []citation) []citation {
	seen := map[string]bool{}
	var out []citation
	for _, c := range in {
		key := strings.Join([]string{c.Name, c.Repo, strconv.Itoa(c.Number), c.Source}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// citationsInDiff extracts every citation from the ADDED lines of a unified diff,
// grouped per file so a citation's Source is the file it was added to. Fixture-corpus
// paths are skipped exactly as the stamp scan skips them (isExcludedFixturePath).
func citationsInDiff(root, diff string) []citation {
	lines := strings.Split(diff, "\n")
	added := map[string][]string{}
	curFile := ""
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, "diff --git ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 4 {
				curFile = strings.TrimPrefix(fields[3], "b/")
			}
			continue
		}
		if strings.HasPrefix(trimmed, "+++ ") {
			curFile = strings.TrimPrefix(trimmed, "+++ b/")
			continue
		}
		if !strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "+++") {
			continue
		}
		if isExcludedFixturePath(root, curFile) {
			continue
		}
		added[curFile] = append(added[curFile], strings.TrimPrefix(trimmed, "+"))
	}
	// Deterministic file order for a stable report.
	files := make([]string, 0, len(added))
	for f := range added {
		files = append(files, f)
	}
	sort.Strings(files)
	var out []citation
	for _, f := range files {
		out = append(out, detectCitations(f, strings.Join(added[f], "\n"))...)
	}
	return out
}

// ---- corroboration core -----------------------------------------------------------

// citedArtifact is the pre-fetched data for ONE cited issue/PR: every comment and
// (for a PR) every review on it. corroborateCitations consumes it; the runtime
// fetches it (fetchCitedArtifact) so the core stays offline-testable.
type citedArtifact struct {
	Comments []ghComment
	Reviews  []ghReview
}

// authoredBy reports whether login authored at least one comment or review on the
// artifact — the "≥1 hit" corroboration a ruling/acceptance citation needs. Unlike
// the STAMP gate, a citation is corroborated by the human having ACTED on the cited
// artifact at all, not by an explicit approval phrase: an author who wrote the ruling
// the citation quotes has, at minimum, a comment or review there.
func (a *citedArtifact) authoredBy(login string) (evidence string, ok bool) {
	if a == nil {
		return "", false
	}
	for _, r := range a.Reviews {
		if strings.EqualFold(r.Author.Login, login) {
			return fmt.Sprintf("review by %s (%s)", login, r.State), true
		}
	}
	for _, c := range a.Comments {
		if strings.EqualFold(c.Author.Login, login) {
			if c.URL != "" {
				return fmt.Sprintf("comment by %s: %s", login, c.URL), true
			}
			return fmt.Sprintf("comment by %s", login), true
		}
	}
	return "", false
}

type citationResult struct {
	Citation citation
	Verdict  verdict
	Login    string
	Evidence string
}

// corroborateCitations resolves each citation's name to a GitHub login and checks the
// pre-fetched cited artifact for ≥1 comment/review by that login. fetched is keyed by
// citedKey(prRepo). A citation with no reference (unlinked), or one whose cited
// artifact carries no action by the named human, is MISSING-CORROBORATION.
//
// The fetched map distinguishes three states for a referenced citation, so a fetch
// FAILURE is never rounded down to a fabricated absence:
//
//   - key present, artifact NON-nil  -> the artifact was read (a genuine 404 arrives
//     here as a non-nil EMPTY artifact): authoredBy decides CORROBORATED vs MISSING;
//   - key present, artifact nil       -> the live fetch FAILED (network/auth/rate-limit
//     /transient 5xx): COULD-NOT-CHECK — the instrument observed nothing, so it
//     condemns nothing;
//   - key ABSENT                      -> the fetch was never attempted for this ref:
//     likewise COULD-NOT-CHECK (fail-safe, never a fabricated MISSING).
func corroborateCitations(cits []citation, fetched map[string]*citedArtifact, prRepo string) []citationResult {
	var results []citationResult
	for _, c := range cits {
		login, known := citedHumanLogin(c.Name)
		if !known {
			// Should not happen (detectCitations already filtered), but keep the
			// gate fail-closed rather than silently dropping.
			results = append(results, citationResult{
				Citation: c, Verdict: verdictMissing,
				Evidence: fmt.Sprintf("name %q has no mapping in %s — no GitHub login to check", c.Name, scanEnvHumanLoginMap),
			})
			continue
		}
		if !c.HasRef {
			results = append(results, citationResult{
				Citation: c, Verdict: verdictMissing, Login: login,
				Evidence: fmt.Sprintf("acceptance/ruling by %s is asserted with NO cited issue/PR — "+
					"cite the artifact (owner/repo#N or its URL) so the claim can be verified", login),
			})
			continue
		}
		art, present := fetched[c.citedKey(prRepo)]
		if !present || art == nil {
			// The cited artifact could not be fetched (transient/auth/rate-limit, or
			// no fetch attempted). Do NOT emit MISSING: the check never observed the
			// artifact, so it cannot assert the human failed to act on it. Demote to
			// COULD-NOT-CHECK — fail-open for a fault the instrument never resolved,
			// while a genuinely empty/404 artifact (a non-nil empty struct) still
			// falls through to MISSING below.
			results = append(results, citationResult{
				Citation: c, Verdict: verdictCitationUncheckable, Login: login,
				Evidence: fmt.Sprintf("could not fetch the cited %s (network, token, rate-limit "+
					"or transient failure) — cannot confirm or deny the acceptance/ruling; re-run --corroborate",
					c.citedKey(prRepo)),
			})
			continue
		}
		if ev, ok := art.authoredBy(login); ok {
			results = append(results, citationResult{
				Citation: c, Verdict: verdictCorroborated, Login: login, Evidence: ev,
			})
			continue
		}
		results = append(results, citationResult{
			Citation: c, Verdict: verdictMissing, Login: login,
			Evidence: fmt.Sprintf("no comment or review by %s on the cited %s — "+
				"the claimed acceptance/ruling has no artifact behind it", login, c.citedKey(prRepo)),
		})
	}
	return results
}

// ---- runtime gathering + live fetch (untested plumbing, like fetchPRData) ---------

// commitMessagesSince returns the commit messages on HEAD not reachable from base,
// each tagged with its short SHA as the citation source. Best-effort: a git failure
// yields no messages rather than an error, so a citation scan degrades to diff-only.
func commitMessagesSince(root, base string) []struct{ source, text string } {
	out := []struct{ source, text string }{}
	cmd := exec.Command("git", "-C", root, "log", "--no-merges",
		"--format=%h%x00%B%x1e", base+"..HEAD")
	raw, err := cmd.Output()
	if err != nil {
		return out
	}
	for _, rec := range strings.Split(string(raw), "\x1e") {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		sha, body, found := strings.Cut(rec, "\x00")
		if !found {
			continue
		}
		out = append(out, struct{ source, text string }{"commit " + sha, body})
	}
	return out
}

// gatherCitations collects citations from a PR diff plus, when a base ref resolves,
// the branch's own commit messages.
func gatherCitations(root, diff string) []citation {
	cits := citationsInDiff(root, diff)
	if base, resolved := registerLandedBase(root); resolved {
		for _, m := range commitMessagesSince(root, base) {
			cits = append(cits, detectCitations(m.source, m.text)...)
		}
	}
	return dedupeCitations(cits)
}

// ghErrIsNotFound reports whether a gh api failure was an HTTP 404 — the cited
// artifact genuinely does not exist — as opposed to a transient/auth/rate-limit
// failure. .Output() populates *exec.ExitError.Stderr (cmd.Stderr is nil here), where
// gh writes "gh: Not Found (HTTP 404)". A 404 is an OBSERVED absence (a bogus ref); any
// other failure is could-not-check.
func ghErrIsNotFound(err error) bool {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		s := strings.ToLower(string(ee.Stderr))
		return strings.Contains(s, "http 404") || strings.Contains(s, "not found")
	}
	return false
}

// fetchCitedArtifact reads a cited issue/PR's comments and (if it is a PR) reviews via
// the REST API. It returns a NON-nil error ONLY when the fetch could not be completed
// (network, token, rate-limit, transient 5xx) — a state the caller must render as
// COULD-NOT-CHECK, never as a fabricated absence. A genuine HTTP 404 is NOT an error:
// it means the cited artifact does not exist, which is a real MISSING, so it returns a
// non-nil EMPTY artifact and nil error. The reviews endpoint 404s for a plain issue —
// expected, and it just contributes no reviews.
func fetchCitedArtifact(repo string, number int) (*citedArtifact, error) {
	art := &citedArtifact{}
	// Issue comments (covers both issues and PRs). This endpoint is also the
	// existence probe: a 404 here means the cited artifact is absent (a bogus ref).
	out, err := exec.Command("gh", "api", "--paginate", "--slurp",
		fmt.Sprintf("repos/%s/issues/%d/comments", repo, number)).Output()
	if err != nil {
		if ghErrIsNotFound(err) {
			return art, nil // observed 404: empty artifact -> MISSING downstream
		}
		return nil, fmt.Errorf("gh api issues/%d/comments: %w", number, err)
	}
	var cpages [][]struct {
		User    ghAuthor `json:"user"`
		Body    string   `json:"body"`
		HTMLURL string   `json:"html_url"`
	}
	if json.Unmarshal(out, &cpages) == nil {
		for _, p := range cpages {
			for _, c := range p {
				art.Comments = append(art.Comments, ghComment{
					Author: c.User, Body: c.Body, URL: c.HTMLURL,
				})
			}
		}
	}
	// PR reviews (a 404 here is EXPECTED for a plain issue — no reviews, not a
	// failure). Any OTHER error means we could not read the reviews, so we cannot
	// rule out a corroborating APPROVED review: surface it as a fetch failure rather
	// than reporting a possibly-false absence.
	out, err = exec.Command("gh", "api", "--paginate", "--slurp",
		fmt.Sprintf("repos/%s/pulls/%d/reviews", repo, number)).Output()
	if err != nil {
		if !ghErrIsNotFound(err) {
			return nil, fmt.Errorf("gh api pulls/%d/reviews: %w", number, err)
		}
	} else {
		var rpages [][]struct {
			User  ghAuthor `json:"user"`
			Body  string   `json:"body"`
			State string   `json:"state"`
		}
		if json.Unmarshal(out, &rpages) == nil {
			for _, p := range rpages {
				for _, r := range p {
					art.Reviews = append(art.Reviews, ghReview{
						Author: r.User, Body: r.Body, State: r.State,
					})
				}
			}
		}
	}
	return art, nil
}

// checkCitationCorroboration runs the full citation pipeline for one PR: gather
// citations (diff + commit messages) → fetch each distinct cited artifact live →
// corroborate. Returns one result per detected citation.
func checkCitationCorroboration(root, repo, diff string, pr int) []citationResult {
	cits := gatherCitations(root, diff)
	if len(cits) == 0 {
		return nil
	}
	fetched := map[string]*citedArtifact{}
	for _, c := range cits {
		if !c.HasRef {
			continue
		}
		key := c.citedKey(repo)
		if _, done := fetched[key]; done {
			continue
		}
		citedRepo := c.Repo
		if citedRepo == "" {
			citedRepo = repo
		}
		art, err := fetchCitedArtifact(citedRepo, c.Number)
		if err != nil {
			// Record the failure as a nil artifact under the key so corroborateCitations
			// renders COULD-NOT-CHECK (present, nil) rather than a fabricated MISSING.
			// Mirror the stamp lane's stderr diagnostic for a fetch fault (clause 8:
			// demote a stderr to could-not-check, do not swallow it into a verdict).
			fmt.Fprintf(os.Stderr, "statusgen: --corroborate: could not fetch cited %s: %v\n", key, err)
			fetched[key] = nil
			continue
		}
		fetched[key] = art
	}
	return corroborateCitations(cits, fetched, repo)
}
