package main

// transcribe-scan — the SAME-REPO scan transcriber's brain (issue-flow rulings
// R-7; scan-lane/01). It is the sole logic behind the `scan-transcribe.yml`
// workflow, which is thin glue: enactment gate, checkout main, `run`, lint,
// commit, push. This file holds the trust predicate, the delta derivation, and
// the clause evaluation; the workflow holds only the transport.
//
// SHIPPED INERT. The lane evaluates NOTHING until R-7's Sign-off line in
// docs/streams/issue-flow/rulings.md resolves, via the API, to a comment by the
// blessing authority (ASSAY_BLESS_LOGIN, login:id, non-User refused) whose body
// names R-7. Until then transcribeEnactmentGate reports INERT and no clause is
// evaluated — the same self-arming-excluded-by-construction posture as the R-6
// verify-transcribe lane. The workflow independently re-resolves the same line
// before the tool ever runs (defense in depth), so an empty or unauthorized
// sign-off keeps the lane inert from either side.
//
// RELATION TO --scan-issues. The existing --scan-issues is the LOCAL scan-carrier
// flow: a session scans in an isolated worktree and the desk merges the delta as
// a PR (a human merge-skim). This lane replaces that human skim with an explicit
// TRUST PREDICATE and re-derivation, committing the same docs-only delta class
// directly from a server-side workflow. It reuses --scan-issues' derivation
// helpers (renderPlaceholder / derivePlaceholderGate / the close-out machinery)
// so a CREATE it lands is byte-identical to what the scanner renders — the
// re-derivation IS the trust (R-7 cl.2a).
//
// SECURITY: issue bodies are attacker-authorable data. Nothing from any issue
// body is EXECUTED or copied into a placeholder beyond the schema's pointer
// fields (renderPlaceholder writes only the repo, number, derived gate, and
// labels). The trust predicate keys on API-read author identity and roster
// membership, never on any text an issue author controls.

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// jsonUnmarshalTrim unmarshals a gh --jq single-object response, tolerating the
// trailing newline gh appends.
func jsonUnmarshalTrim(data []byte, v any) error {
	return json.Unmarshal([]byte(strings.TrimSpace(string(data))), v)
}

// transcribeFloodThreshold is the R-7 cl.6 flood tripwire: a run whose delta
// would CREATE MORE THAN this many placeholders refuses and files one triage
// issue. It matches the inbound-burst monitor threshold — a mass-create is an
// incident to look at, not a batch to land. RETIREs are not capped.
const transcribeFloodThreshold = 25

// authorIdentity is the API-read author triple R-7 cl.1 requires: login AND
// numeric id AND account type. The cheap `gh issue list` JSON carries no id, so
// the lane resolves each candidate author identity explicitly (authorResolver).
type authorIdentity struct {
	Login string
	ID    int64
	Type  string // "User" | "Bot" | "Organization"
}

// authorResolver reads the API-authenticated author identity of one issue. The
// production implementation shells to `gh api`; tests inject a fixture. An error
// is a REFUSAL for that issue (fail closed), never a default-trust.
type authorResolver func(repo string, issue int) (authorIdentity, error)

// ghAuthorResolver is the default authorResolver: `gh api` for one issue's user.
func ghAuthorResolver(repo string, issue int) (authorIdentity, error) {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%d", repo, issue),
		"--jq", "{login: .user.login, id: .user.id, type: .user.type}").Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return authorIdentity{}, fmt.Errorf("gh api author %s#%d: %v %s", repo, issue, err, detail)
	}
	var a struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	}
	if err := jsonUnmarshalTrim(out, &a); err != nil {
		return authorIdentity{}, fmt.Errorf("parsing author for %s#%d: %w", repo, issue, err)
	}
	return authorIdentity{Login: a.Login, ID: a.ID, Type: a.Type}, nil
}

// commentResolver reads the author + body of ONE comment addressed by a GitHub
// comment URL. It backs the enactment gate (resolve the R-7 sign-off line's URL
// to its author). Production shells to `gh api`; tests inject a fixture.
type commentResolver func(url string) (author authorIdentity, body string, err error)

// ghCommentResolver resolves a GitHub issue/PR comment URL to its author + body.
func ghCommentResolver(url string) (authorIdentity, string, error) {
	repo, id, ok := parseCommentURL(url)
	if !ok {
		return authorIdentity{}, "", fmt.Errorf("cannot parse comment URL %q", url)
	}
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/comments/%s", repo, id),
		"--jq", "{login: .user.login, id: .user.id, type: .user.type, body: .body}").Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return authorIdentity{}, "", fmt.Errorf("gh api comment %s: %v %s", url, err, detail)
	}
	var c struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
		Body  string `json:"body"`
	}
	if err := jsonUnmarshalTrim(out, &c); err != nil {
		return authorIdentity{}, "", fmt.Errorf("parsing comment %s: %w", url, err)
	}
	return authorIdentity{Login: c.Login, ID: c.ID, Type: c.Type}, c.Body, nil
}

// parseCommentURL extracts owner/repo and the numeric comment id from a GitHub
// comment permalink, e.g.
//
//	https://github.com/OWNER/REPO/pull/999#issuecomment-5319580160
//	https://github.com/OWNER/REPO/issues/297#issuecomment-5160364739
func parseCommentURL(url string) (repo, commentID string, ok bool) {
	i := strings.Index(url, "github.com/")
	if i < 0 {
		return "", "", false
	}
	rest := url[i+len("github.com/"):]
	marker := "#issuecomment-"
	j := strings.Index(rest, marker)
	if j < 0 {
		return "", "", false
	}
	commentID = rest[j+len(marker):]
	// trailing junk after the id (rare) — keep only the leading digits.
	commentID = leadingDigits(commentID)
	if commentID == "" {
		return "", "", false
	}
	path := rest[:j] // OWNER/REPO/(pull|issues)/NNN
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0] + "/" + parts[1], commentID, true
}

func leadingDigits(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return s[:i]
		}
	}
	return s
}

// transcribeEnactmentGate reports whether the lane is ARMED — i.e. whether R-7's
// Sign-off line resolves to a bless-authority comment naming R-7. It is the
// tool-side half of the self-arming-excluded-by-construction gate (the workflow
// re-checks the same line before invoking the tool). Fail closed: an empty
// sign-off line, an unparseable URL, an unreadable comment, or an author/body
// that fails any check leaves the lane INERT.
//
// The EMPTY-sign-off branch is fully offline (it reads only rulings.md), which is
// the state the lane ships in and the state row-3 of the Verify table exercises;
// resolve is called only when a URL is actually present.
func transcribeEnactmentGate(root string, resolve commentResolver) (armed bool, reason string) {
	rulings := filepath.Join(root, "docs", "streams", "issue-flow", "rulings.md")
	data, err := os.ReadFile(rulings)
	if err != nil {
		return false, fmt.Sprintf("cannot read %s: %v", rulings, err)
	}
	line, ok := findR7SignoffLine(string(data))
	if !ok {
		return false, "R-7 sign-off line not found in rulings.md"
	}
	url := firstHTTPSURL(line)
	if url == "" {
		return false, "R-7 sign-off is empty — the lane evaluates no clause until an authorized acceptance URL lands on it"
	}
	if resolve == nil {
		return false, "R-7 sign-off carries a URL but no resolver is available to verify it (fail closed)"
	}
	author, body, err := resolve(url)
	if err != nil {
		return false, fmt.Sprintf("R-7 sign-off URL %s could not be resolved: %v", url, err)
	}
	c := scanEffectiveConfig()
	if !c.Configured() {
		return false, "roster unconfigured — no blessing authority to verify the sign-off against"
	}
	if !strings.EqualFold(author.Login, c.Bless.Login) || author.ID == 0 || author.ID != c.Bless.ID {
		return false, fmt.Sprintf("R-7 sign-off author %s:%d is not the blessing authority %s:%d",
			author.Login, author.ID, c.Bless.Login, c.Bless.ID)
	}
	if author.Type != "User" {
		return false, fmt.Sprintf("R-7 sign-off author is type %q, not User — a non-User identity cannot arm the lane", author.Type)
	}
	if !strings.Contains(body, "R-7") {
		return false, "R-7 sign-off comment body does not name R-7"
	}
	return true, "armed: R-7 sign-off resolves to the blessing authority"
}

// findR7SignoffLine returns the first `**Sign-off:**` line that appears at or
// after the `## R-7` heading. Scoping to the R-7 section keeps a filled R-6
// sign-off (which appears earlier) from arming the scan lane.
func findR7SignoffLine(md string) (string, bool) {
	lines := strings.Split(md, "\n")
	inR7 := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## R-7") {
			inR7 = true
			continue
		}
		// A later top-level ruling heading ends the R-7 section.
		if inR7 && strings.HasPrefix(t, "## R-") && !strings.HasPrefix(t, "## R-7") {
			return "", false
		}
		if inR7 && strings.HasPrefix(t, "**Sign-off:**") {
			return t, true
		}
	}
	return "", false
}

// firstHTTPSURL returns the first https:// token in s (whitespace/paren/angle
// delimited), or "".
func firstHTTPSURL(s string) string {
	i := strings.Index(s, "https://")
	if i < 0 {
		return ""
	}
	rest := s[i:]
	end := strings.IndexFunc(rest, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ')' || r == '>' || r == ']' || r == '"' || r == '\'' || r == '\n' || r == '\r'
	})
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// clauseSkip records ONE issue the lane refused to board, naming the R-7 clause
// it failed. Ordinary per-issue trust refusals (clause 1) are logged but never
// filed as issues (the quarantine lane already holds them, R-7 cl.1 / intake-desk
// EXTERNAL-UNBLESSED lane); only incident-class refusals (flood, lint) file.
type clauseSkip struct {
	Issue  int
	Clause string
	Reason string
}

// transcribeDelta is the fully-derived, not-yet-applied same-repo delta plus the
// per-issue skip log.
type transcribeDelta struct {
	Creates   []scanPlan
	CloseOuts []closeOutPlan
	Unblocks  []unblockPlan
	Skips     []clauseSkip
	Notices   []string
}

// planTranscribeScan derives the three R-7 cl.2 delta classes for the SAME repo
// (the rostered home repo) against the loaded tree:
//
//   - CREATE (cl.2a): a placeholder for each open, non-excluded, unhandled issue
//     whose author passes the clause-1 trust predicate — byte-identical to what
//     the scanner renders (renderPlaceholder), because it IS renderPlaceholder.
//   - RETIRE (cl.2b): an existing placeholder whose issue the API shows closed (or
//     open-but-excluded) is retired/swept, exactly as --scan-issues does.
//   - AWAIT (cl.2c): a blocked placeholder whose issue's comment state confirms the
//     block cleared is unblocked (planUnblock).
//
// It NEVER writes. The clause-1 predicate is: an authorized direct author
// (authorizedByIdentity over the API-read identity), OR a blessing (bless) — the
// blessing path currently rests on the single configured authority, which is a
// member of the authorized set, so it is a fail-CLOSED subset of R-7's
// "blessing by any authorized-set member" (see the note at runTranscribeScan).
// A per-issue trust failure or unreadable author is a clauseSkip, never a
// default-board.
func planTranscribeScan(root string, streams []*Stream, homeRepo string,
	list issueLister, comments commentLister, resolveAuthor authorResolver, bless issueBlessChecker) (transcribeDelta, error) {

	var d transcribeDelta

	issues, err := list(homeRepo)
	if err != nil {
		// Same-repo lane over a single repo: a listing failure is fatal to the run
		// (there is no other repo to degrade to), not a per-repo NOTICE.
		return d, fmt.Errorf("listing issues for %s: %w", homeRepo, err)
	}

	existing := existingPlaceholderIssues(streams)
	for _, s := range streams {
		for _, path := range archivedPlaceholderFilePaths(s) {
			if ph, ok, perr := parsePlaceholderFile(path); perr == nil && ok {
				existing[ph.Repo+"#"+strconv.Itoa(ph.Issue)] = true
			}
		}
	}

	dir := filepath.Join(root, "docs", "streams", scanStreamName)

	openSet := map[int]bool{}
	openExcluded := map[int]bool{}
	for _, iss := range issues {
		openSet[iss.Number] = true
		labels := labelNames(iss.Labels)
		excluded := hasExcludedLabel(labels)
		openExcluded[iss.Number] = excluded
		if excluded {
			// R-7 cl.2: system-state labels (verify-gate / live-verify / needs-decision
			// / review-request …) are excluded by construction — closeable states, not
			// work. Not a trust refusal, so not logged as a clause skip.
			continue
		}
		if existing[homeRepo+"#"+strconv.Itoa(iss.Number)] {
			continue
		}
		// --- R-7 clause 1: trust predicate ---
		ident, aerr := resolveAuthor(homeRepo, iss.Number)
		if aerr != nil {
			// Unreadable author = refusal for THIS issue (fail closed), never trust.
			d.Skips = append(d.Skips, clauseSkip{
				Issue:  iss.Number,
				Clause: "clause-1 (trust)",
				Reason: fmt.Sprintf("author identity unreadable — %v", aerr),
			})
			continue
		}
		trusted := authorizedByIdentity(ident.Login, ident.ID, ident.Type)
		if !trusted {
			blessed, berr := bless(homeRepo, iss.Number)
			if berr != nil {
				d.Skips = append(d.Skips, clauseSkip{
					Issue:  iss.Number,
					Clause: "clause-1 (trust)",
					Reason: fmt.Sprintf("author %q not authorized and blessing unverifiable — %v", ident.Login, berr),
				})
				continue
			}
			if !blessed {
				d.Skips = append(d.Skips, clauseSkip{
					Issue:  iss.Number,
					Clause: "clause-1 (trust)",
					Reason: fmt.Sprintf("author %q (id %d, %s) is not a rostered authorized author and the issue carries no current blessing — left for the quarantine lane",
						ident.Login, ident.ID, ident.Type),
				})
				continue
			}
		}
		path := filepath.Join(dir, placeholderFileName(homeRepo, iss.Number))
		if _, serr := os.Stat(path); serr == nil {
			continue // a file already occupies the target path — never overwrite
		}
		gate := derivePlaceholderGate(labels, iss.Title)
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		d.Creates = append(d.Creates, scanPlan{
			Repo:    homeRepo,
			Issue:   iss.Number,
			Path:    path,
			Rel:     filepath.ToSlash(rel),
			Gate:    gate,
			Labels:  labels,
			Content: renderPlaceholder(homeRepo, iss.Number, gate, labels),
		})
	}

	// --- R-7 cl.2b: RETIRE / reactivate / sweep (home repo only). This mirrors
	// planScan's close-out sweep; it is NOT trust-gated (it touches our own
	// placeholder state, not issue content) and only ever consumes API-confirmed
	// open/closed state. KEEP IN SYNC with planScan's close-out loops. ---
	for _, s := range streams {
		for _, ph := range s.Placeholders {
			if ph.Repo != homeRepo {
				continue
			}
			rel, rerr := filepath.Rel(root, ph.Path)
			if rerr != nil {
				rel = ph.Path
			}
			rel = filepath.ToSlash(rel)
			dest := archivedPath(ph.Path)
			if !openSet[ph.Issue] {
				if ph.Status != "done" {
					d.CloseOuts = append(d.CloseOuts, closeOutPlan{
						Repo: homeRepo, Issue: ph.Issue, Path: ph.Path, Rel: rel, Action: "retire", Dest: dest})
				} else {
					d.CloseOuts = append(d.CloseOuts, closeOutPlan{
						Repo: homeRepo, Issue: ph.Issue, Path: ph.Path, Rel: rel, Action: "sweep", Dest: dest})
				}
				continue
			}
			if openExcluded[ph.Issue] {
				if ph.Status != "done" {
					d.CloseOuts = append(d.CloseOuts, closeOutPlan{
						Repo: homeRepo, Issue: ph.Issue, Path: ph.Path, Rel: rel, Action: "retire-label", Dest: dest})
				} else {
					d.CloseOuts = append(d.CloseOuts, closeOutPlan{
						Repo: homeRepo, Issue: ph.Issue, Path: ph.Path, Rel: rel, Action: "sweep", Dest: dest})
				}
				continue
			}
			if ph.Status == "done" {
				d.CloseOuts = append(d.CloseOuts, closeOutPlan{
					Repo: homeRepo, Issue: ph.Issue, Path: ph.Path, Rel: rel, Action: "reactivate"})
			}
		}
	}
	// Reactivation from the archive (reopened issue): move it back to the root.
	for _, s := range streams {
		for _, path := range archivedPlaceholderFilePaths(s) {
			ph, ok, perr := parsePlaceholderFile(path)
			if perr != nil || !ok || ph.Repo != homeRepo {
				continue
			}
			if !openSet[ph.Issue] || openExcluded[ph.Issue] {
				continue
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				rel = path
			}
			d.CloseOuts = append(d.CloseOuts, closeOutPlan{
				Repo: homeRepo, Issue: ph.Issue, Path: path,
				Rel: filepath.ToSlash(rel), Action: "reactivate", Dest: unarchivedPath(path)})
		}
	}

	// --- R-7 cl.2c: AWAIT flips (blocked placeholder whose comment state cleared).
	// planUnblock is comment-state confirmed and repo-scoped internally; filter to
	// the home repo so the same-repo lane touches only same-repo placeholders. ---
	allUnblocks, unblockNotices := planUnblock(streams, comments)
	for _, u := range allUnblocks {
		if u.Repo == homeRepo {
			d.Unblocks = append(d.Unblocks, u)
		}
	}
	d.Notices = append(d.Notices, unblockNotices...)

	sort.Slice(d.Creates, func(i, j int) bool { return d.Creates[i].Issue < d.Creates[j].Issue })
	sort.Slice(d.CloseOuts, func(i, j int) bool { return d.CloseOuts[i].Issue < d.CloseOuts[j].Issue })
	sort.Slice(d.Skips, func(i, j int) bool { return d.Skips[i].Issue < d.Skips[j].Issue })
	return d, nil
}

// runTranscribeScan is the --transcribe-scan entrypoint (the workflow's "run"
// step). It is the same-repo lane: it derives and, unless dryRun, APPLIES the
// R-7 cl.2 delta to the tree. It NEVER commits, pushes, or mutates any GitHub
// issue — the workflow commits the tree the tool wrote, and the workflow (not the
// tool) files the flood/lint triage issue. Returns a process exit code:
//
//	0  ran (armed + applied/derived) OR INERT (evaluated no clause) — both neutral
//	2  REFUSED: roster unconfigured, or (local write path) a primary-checkout root
//	3  flood tripwire: the delta would CREATE more than the threshold (cl.6) —
//	   nothing is written; the workflow files ONE triage issue and commits nothing
//
// dryRun is the CI-testable "--check" surface: it derives and reports the
// would-be delta and the per-issue skip log without touching the filesystem.
//
// The blessing path (clause 1) rests today on the single configured blessing
// authority (a member of the authorized-author set), which is a fail-CLOSED
// subset of R-7's "blessing by any member of the authorized set": it may fail to
// board an issue a non-blessing-authority authorized author blessed, but never boards one
// R-7 would forbid. Widening blessing to the full authorized set is a follow-up;
// direct-author authorization over the full rostered set is implemented here.
func runTranscribeScan(root string, dryRun bool,
	list issueLister, comments commentLister, resolveAuthor authorResolver,
	bless issueBlessChecker, resolveSignoff commentResolver) int {

	// P1: unset roster is CLOSED — identical posture to --scan-issues. This lane
	// WRITES durable work items from issues arbitrary external users can author, and
	// the roster is exactly what gates that write.
	if err := scanRosterUnconfiguredError(); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan REFUSED:", err)
		return 2
	}

	// Isolation guard (local write path only). In CI the checkout is an ephemeral,
	// dedicated tree the workflow exists to write and commit; the guard exists to
	// stop a LOCAL run from dirtying a live session's shared checkout. --dry-run
	// writes nothing, so it is always allowed.
	if !dryRun && !scanInCI() {
		if reason := scanIsolationRefusal(root); reason != "" {
			fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan REFUSED:", reason)
			return 2
		}
	}

	// --- Enactment gate (R-7): evaluate NOTHING until armed. ---
	armed, reason := transcribeEnactmentGate(root, resolveSignoff)
	if !armed {
		fmt.Println("transcribe-scan: INERT —", reason)
		fmt.Println("transcribe-scan: evaluating no clause; the lane is disarmed until R-7 is signed")
		return 0
	}
	fmt.Println("transcribe-scan:", reason)

	homeRepo := scanHomeRepo()
	if homeRepo == "" {
		fmt.Println("transcribe-scan: no home repo configured (ASSAY_HOME_REPO) — same-repo lane has nothing to do")
		return 0
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan:", err)
		return 1
	}
	attachPlaceholders(streams)

	delta, err := planTranscribeScan(root, streams, homeRepo, list, comments, resolveAuthor, bless)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan:", err)
		return 1
	}

	// Surface the per-issue skip log (clause named) in the run output — the
	// workflow lifts it into the run summary. Ordinary trust refusals are NOT
	// filed as issues (the quarantine lane holds them).
	for _, sk := range delta.Skips {
		fmt.Printf("transcribe-scan: SKIP %s#%d — %s: %s\n", homeRepo, sk.Issue, sk.Clause, sk.Reason)
	}
	for _, n := range delta.Notices {
		fmt.Println("transcribe-scan: NOTICE —", n)
	}

	// --- R-7 cl.6: flood tripwire. Refuse the WHOLE run and let the workflow file
	// one triage issue; nothing is written. RETIREs/AWAITs are not counted. ---
	if len(delta.Creates) > transcribeFloodThreshold {
		fmt.Printf("transcribe-scan: FLOOD — clause-6 tripwire: %d CREATEs exceed the threshold of %d; refusing the run and filing a triage issue (nothing written)\n",
			len(delta.Creates), transcribeFloodThreshold)
		return 3
	}

	// Report the derived delta.
	for _, p := range delta.Creates {
		fmt.Printf("transcribe-scan: CREATE %s  (%s#%d, gate:%s)\n", p.Rel, p.Repo, p.Issue, p.Gate)
	}
	for _, c := range delta.CloseOuts {
		fmt.Printf("transcribe-scan: %s %s  (%s#%d, %s)\n",
			strings.ToUpper(c.Action), c.Rel, c.Repo, c.Issue, closeOutReason(c.Action))
	}
	for _, u := range delta.Unblocks {
		fmt.Printf("transcribe-scan: AWAIT-FLIP %s  (%s#%d, block cleared)\n", u.Path, u.Repo, u.Issue)
	}

	if dryRun {
		if len(delta.Creates) == 0 && len(delta.CloseOuts) == 0 && len(delta.Unblocks) == 0 {
			fmt.Println("transcribe-scan: no changes — nothing to create, retire, or flip")
		}
		return 0
	}

	// --- Apply (write path). Close-outs before creates (a retired placeholder's
	// target path stays occupied, so a create never collides). AWAIT flips edit in
	// place. NO commit, NO push, NO GitHub mutation. ---
	for _, u := range delta.Unblocks {
		if aerr := applyUnblock(u); aerr != nil {
			fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan:", aerr)
			return 1
		}
		fmt.Printf("transcribe-scan: flipped await %s\n", u.Path)
	}
	for _, c := range delta.CloseOuts {
		if aerr := applyCloseOut(c); aerr != nil {
			fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan:", aerr)
			return 1
		}
		fmt.Printf("transcribe-scan: %s %s\n", closeOutPastTense(c.Action), c.Rel)
	}
	if len(delta.Creates) > 0 {
		if merr := os.MkdirAll(filepath.Join(root, "docs", "streams", scanStreamName), 0o755); merr != nil {
			fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan:", merr)
			return 1
		}
		for _, p := range delta.Creates {
			if werr := os.WriteFile(p.Path, []byte(p.Content), 0o644); werr != nil {
				fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan:", werr)
				return 1
			}
			fmt.Printf("transcribe-scan: created %s\n", p.Rel)
		}
	}
	return 0
}

// ===========================================================================
// R-7 clause 4 — the CROSS-REPO scan-delta verify path (scan-lane-private/02,
// Task 3).
//
// RELATION TO THE SAME-REPO LANE ABOVE. planTranscribeScan re-derives its own
// delta from a live API read of the home repo's open issues — the API access
// itself is the trust primitive. A foreign repo this box cannot always read
// (a private repo like oit or assay-console) has no such API-re-derivation
// available for every entry, so the cross-repo lane substitutes a SIGNATURE:
// the intake loop's existing scan already computes the foreign delta in an
// isolated worktree, signs the canonical payload with the issue-loop role key
// (deskverdict --key issue-loop, Task 1), and files it as ONE scan-delta issue
// on THIS (home) repo. That issue IS the R-7 cl.4 trust primitive.
//
// FIVE INDEPENDENT LAYERS, each naming the clause it refuses under:
//
//  1. clause-4 (author)        — the CONTAINER issue (the scan-delta issue
//     itself) must be authored by the issue-loop App's own API-read identity
//     (login+id+type) — an identity fact, mirrors R-6 clause-1.
//  2. clause-4 (signature)     — the payload block verifies against the
//     issue-loop role's public key (scanDeltaVerifyBody), AND the block must
//     DECLARE role=issue-loop before any signature arithmetic runs (Task 1's
//     declared-role invariant, reused here as its own layer: a
//     verifier-signed artifact is refused on this lane even with a valid sig).
//  3. clause-4 (body-unedited) — the CONTAINER issue's body was not edited
//     since creation (GitHub `last_edited_at`), mirroring R-6's timeline check
//     — an edited container could carry a payload the signature no longer
//     covers byte-for-byte... except it always covers the payload it was
//     computed over; the edit check exists because an editable container
//     could otherwise be used to SUBSTITUTE a stale-but-still-valid signed
//     block after the fact (re-pasting an old payload+signature pair into a
//     newer-looking issue). Refusing ANY edit, not just payload edits, is the
//     simple, auditable rule R-6 already established.
//  4. clause-4 (per-entry API re-check) — for an entry whose Repo this box
//     CAN read via the API, the entry's claimed author identity is
//     re-verified; a contradiction refuses THAT ENTRY (not the whole issue).
//     An entry whose repo is NOT readable (a private foreign repo) rests on
//     the signature + the PRODUCER's own clause-1 check — R-7 cl.4's stated
//     two-tier honesty, never silently dropped and never silently promoted to
//     "verified via API".
//  5. clause-4 (same-repo)     — an entry whose Repo equals homeRepo is
//     refused: same-repo issues have their OWN lane (planTranscribeScan
//     above), and accepting one here would let a signed artifact bypass that
//     lane's independent re-derivation.
//
// The lane stays INERT behind the SAME R-7 enactment gate as the same-repo
// lane (transcribeEnactmentGate) — this file adds NO second arming path.
// ===========================================================================

// scanDeltaSchemaVersion is the payload schema this build speaks. A payload
// naming a version this build does not recognise is refused, never guessed —
// same posture as verdictSchemaVersion.
const scanDeltaSchemaVersion = "scan-delta-v1"

// scanDeltaPubkeyVar carries the issue-loop role's PUBLIC key. A repo/Actions
// VARIABLE, never a secret, and NOT committed to the tree — the second
// role-keyed variable alongside verdictPubkeyVar (verdict-lane/01's landed
// invariant, which this brief EXTENDS to a second role rather than carving an
// exception to). Matches deskkit.IssueLoopPubkeyVar byte-for-byte; the two are
// separate Go modules that share no code (see transcribeverdict.go's package
// doc), so this name is a documented cross-tree duplicate.
const scanDeltaPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY"

// scanDeltaWantRole is the ONE role this lane ever accepts. Matches
// deskkit.VerdictRoleIssueLoop.
const scanDeltaWantRole = "issue-loop"

// scanDeltaRoleRE extracts the declared signing ROLE from a deskverdict-signed
// body's trailer comment (`role=<role>`). Absent entirely (a body signed
// before role-keying existed) means the implicit default "verifier" — the
// SAME default deskkit.extractDeclaredRole applies, so the two independently
// duplicated implementations agree on every input.
var scanDeltaRoleRE = regexp.MustCompile(verdictSigMarker + `\b[^>]*\brole=([a-z-]+)`)

func scanDeltaDeclaredRole(body string) string {
	if m := scanDeltaRoleRE.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return "verifier"
}

// scanDeltaEntry is ONE cross-repo delta item inside a scan-delta payload —
// the producer's canonical per-entry shape (brief fact: "repo, issue number,
// author identity as read, trust basis, delta class, rendered placeholder
// body"). Body is the byte-identical output of the producer's own
// renderPlaceholder call: the trust primitive is the SIGNATURE over this
// content, not a local re-derivation (which is not always possible — the
// entry's repo may be private to this box).
type scanDeltaEntry struct {
	Repo        string `json:"repo"`
	Issue       int    `json:"issue"`
	AuthorLogin string `json:"author_login"`
	AuthorID    int64  `json:"author_id"`
	AuthorType  string `json:"author_type"`
	TrustBasis  string `json:"trust_basis"` // "rostered" | "blessed:<comment-id>"
	Class       string `json:"class"`       // this lane handles "create" only; any other class is could-not-check
	Body        string `json:"body"`        // rendered placeholder-v1 file body
}

// scanDeltaPayload is the canonical cross-repo scan-delta payload — the
// signed block's JSON content.
type scanDeltaPayload struct {
	Schema  string           `json:"schema"`
	TS      string           `json:"ts"`
	Entries []scanDeltaEntry `json:"entries"`
}

// scanDeltaResolvePubkey resolves the issue-loop role's PUBLIC key: an
// explicit --pubkey PEM path, then the ASSAY_ISSUE_LOOP_PUBKEY variable (a
// PEM string OR base64-of-PEM via the SAME decoder verdictResolvePubkey uses,
// generalised by varName — Task 3), else an error — never a silent pass.
// Mirrors verdictResolvePubkey for the second role.
func scanDeltaResolvePubkey(pubkeyPath string) (*rsa.PublicKey, error) {
	if strings.TrimSpace(pubkeyPath) != "" {
		data, err := os.ReadFile(pubkeyPath)
		if err != nil {
			return nil, fmt.Errorf("reading --scan-delta-pubkey %s: %w", pubkeyPath, err)
		}
		return verdictParseRSAPublicKeyPEM(data)
	}
	if v := strings.TrimSpace(os.Getenv(scanDeltaPubkeyVar)); v != "" {
		pemBytes, err := verdictDecodePubkeyVar(scanDeltaPubkeyVar, v)
		if err != nil {
			return nil, err
		}
		return verdictParseRSAPublicKeyPEM(pemBytes)
	}
	return nil, fmt.Errorf("no issue-loop public key: pass --scan-delta-pubkey <file> or set %s (a missing key is could-not-check, never trust)", scanDeltaPubkeyVar)
}

// scanDeltaVerifyBody is verdictVerifyBody generalised for the issue-loop
// role's declared-role invariant (Task 1's second trust layer, reused here):
// it refuses a block whose trailer declares a role OTHER than "issue-loop"
// BEFORE any signature arithmetic runs — a verifier-signed artifact is
// refused on this lane even with an otherwise-valid signature, and even if
// the wrong key were somehow reachable from this lane's pubkey resolution.
func scanDeltaVerifyBody(body string, pub *rsa.PublicKey) (verdictVerifyState, string) {
	rawPayload, sigB64, err := verdictParseBody(body)
	if err != nil {
		return verdictCouldNotCheck, "could not check: " + err.Error()
	}
	if declared := scanDeltaDeclaredRole(body); declared != scanDeltaWantRole {
		return verdictRefused, fmt.Sprintf("refused: signed block declares role %q, but the scan-delta lane requires role %q", declared, scanDeltaWantRole)
	}
	if pub == nil {
		return verdictCouldNotCheck, "could not check: no issue-loop public key configured (" + scanDeltaPubkeyVar + ")"
	}
	canonical, cerr := verdictCanonicalizeJSON(rawPayload)
	if cerr != nil {
		return verdictCouldNotCheck, "could not check: " + cerr.Error()
	}
	sig, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if derr != nil {
		return verdictRefused, "refused: signature is not valid base64: " + derr.Error()
	}
	sum := sha256.Sum256(canonical)
	if verr := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); verr != nil {
		return verdictRefused, "refused: signature does not verify against the issue-loop public key"
	}
	return verdictVerified, "verified: signature matches the canonical scan-delta payload (role=issue-loop)"
}

// scanDeltaIssueLoopIdentity resolves the issue-loop App's identity from the
// roster (role key "issue-loop"), mirroring verdictVerifierIdentity for the
// verifier role. Absent from the roster is NOT trust: clause-4 (author)
// refuses to arm for any issue without it.
func scanDeltaIssueLoopIdentity() (authorIdentity, bool) {
	c := scanEffectiveConfig()
	slug := c.RoleBots["issue-loop"]
	if slug == "" {
		return authorIdentity{}, false
	}
	id := c.Bots[slug]
	if id == 0 {
		return authorIdentity{}, false
	}
	return authorIdentity{Login: slug + "[bot]", ID: id, Type: "Bot"}, true
}

// scanDeltaEntryOutcome is the three-state per-entry result — same discipline
// as VerdictVerifyState: CouldNotCheck is a structural surprise, never a pass
// and never rounded down to Refused.
type scanDeltaEntryOutcome int

const (
	scanDeltaAccepted scanDeltaEntryOutcome = iota
	scanDeltaEntryRefused
	scanDeltaEntryCouldNotCheck
)

// scanDeltaEntryResult names the clause an entry was accepted or refused
// under — never a bare pass/fail with no attribution.
type scanDeltaEntryResult struct {
	Entry   scanDeltaEntry
	Outcome scanDeltaEntryOutcome
	Clause  string
	Reason  string
}

// verifyScanDeltaEntry runs the two PER-ENTRY clause-4 layers — same-repo
// refusal and the API re-check where readable — against one entry. It never
// touches the filesystem or the network beyond resolveAuthor.
func verifyScanDeltaEntry(homeRepo string, e scanDeltaEntry, resolveAuthor authorResolver) scanDeltaEntryResult {
	if e.Repo == "" || e.Issue <= 0 {
		return scanDeltaEntryResult{e, scanDeltaEntryCouldNotCheck, "clause-4 (structure)",
			"entry carries no repo or a non-positive issue number"}
	}
	if e.Class != "create" {
		return scanDeltaEntryResult{e, scanDeltaEntryCouldNotCheck, "clause-4 (class)",
			fmt.Sprintf("unsupported delta class %q — this lane handles \"create\" only", e.Class)}
	}
	// --- clause-4 (same-repo): a same-repo entry has its OWN lane above. ---
	if strings.EqualFold(e.Repo, homeRepo) {
		return scanDeltaEntryResult{e, scanDeltaEntryRefused, "clause-4 (same-repo)",
			fmt.Sprintf("entry targets %s, this transcriber's OWN home repo — same-repo entries have their own lane (R-7 cl.2a) and are refused here so a signed artifact can never bypass that lane's independent re-derivation", e.Repo)}
	}
	if e.TrustBasis != "rostered" && !strings.HasPrefix(e.TrustBasis, "blessed:") {
		return scanDeltaEntryResult{e, scanDeltaEntryCouldNotCheck, "clause-4 (trust-basis)",
			fmt.Sprintf("unrecognized trust basis %q (want \"rostered\" or \"blessed:<comment-id>\")", e.TrustBasis)}
	}
	if e.AuthorLogin == "" || e.AuthorID == 0 || e.AuthorType == "" {
		return scanDeltaEntryResult{e, scanDeltaEntryCouldNotCheck, "clause-4 (per-entry author)",
			"entry carries no author identity triple (login/id/type) to check"}
	}
	// --- clause-4 (per-entry API re-check), where readable. ---
	if resolveAuthor == nil {
		return scanDeltaEntryResult{e, scanDeltaAccepted, "clause-4 (per-entry author, unreadable)",
			fmt.Sprintf("%s#%d: no author resolver available — resting on the signed producer body (R-7 cl.4 two-tier honesty)", e.Repo, e.Issue)}
	}
	ident, aerr := resolveAuthor(e.Repo, e.Issue)
	if aerr != nil {
		// Unreadable (private foreign repo, deleted issue, rate limit, ...): the
		// two-tier honesty this clause is named for — rest on the signature and
		// the PRODUCER's own clause-1 check. NOT a refusal, and the caller
		// surfaces the reason as a NOTICE rather than silently dropping it.
		return scanDeltaEntryResult{e, scanDeltaAccepted, "clause-4 (per-entry author, unreadable)",
			fmt.Sprintf("%s#%d: author identity not API-readable from here (%v) — resting on the signed producer body (R-7 cl.4 two-tier honesty)", e.Repo, e.Issue, aerr)}
	}
	if !strings.EqualFold(ident.Login, e.AuthorLogin) || ident.ID != e.AuthorID || ident.Type != e.AuthorType {
		return scanDeltaEntryResult{e, scanDeltaEntryRefused, "clause-4 (per-entry author contradicted)",
			fmt.Sprintf("%s#%d: entry declares author %s:%d:%s but the API reads %s:%d:%s — contradicted, refused",
				e.Repo, e.Issue, e.AuthorLogin, e.AuthorID, e.AuthorType, ident.Login, ident.ID, ident.Type)}
	}
	return scanDeltaEntryResult{e, scanDeltaAccepted, "clause-4 (per-entry author, confirmed)",
		fmt.Sprintf("%s#%d: author confirmed via API re-check", e.Repo, e.Issue)}
}

// planScanDelta derives the R-7 clause-4 delta from ONE candidate scan-delta
// issue: the container-level checks (author, signature+role, body-unedited),
// then the payload parse, then the PER-ENTRY battery for every entry. It NEVER
// writes. armed must already be true (the caller checks the R-7 enactment
// gate via transcribeEnactmentGate) — this function evaluates no enactment
// logic of its own, mirroring planTranscribeScan's separation of gate from
// derivation. existing is the repo-agnostic "repo#issue" set of placeholders
// already on the board (existingPlaceholderIssues(streams) plus any archived
// ones), so a cross-repo entry never collides with a placeholder any lane
// already created for the same foreign issue.
func planScanDelta(root string, containerIssue int, vi verdictIssue, pub *rsa.PublicKey,
	homeRepo string, issueLoop authorIdentity, resolveAuthor authorResolver,
	existing map[string]bool) (creates []scanPlan, results []scanDeltaEntryResult, notices []string, refuseClause, refuseReason string) {

	// --- clause-4 (author): the CONTAINER issue must be authored by the
	// issue-loop App's own API-read identity — an identity fact, not a crypto
	// fact (single-point-of-failure note, layer 1). ---
	if !strings.EqualFold(vi.Author.Login, issueLoop.Login) || vi.Author.ID == 0 ||
		vi.Author.ID != issueLoop.ID || vi.Author.Type != "Bot" {
		return nil, nil, nil, "clause-4 (author)",
			fmt.Sprintf("issue author %s:%d (%s) is not the issue-loop App %s:%d — an author login alone is spoofable; only the API-read issue-loop identity is trusted",
				vi.Author.Login, vi.Author.ID, vi.Author.Type, issueLoop.Login, issueLoop.ID)
	}

	// --- clause-4 (signature): role-declared + RS256, BEFORE any per-entry
	// work — an unsigned or wrongly-signed container trusts nothing inside it. ---
	state, msg := scanDeltaVerifyBody(vi.Body, pub)
	if state != verdictVerified {
		return nil, nil, nil, "clause-4 (signature)", msg
	}

	// --- clause-4 (body-unedited): mirrors R-6's timeline check — an edited
	// container could substitute a stale-but-still-valid signed block. ---
	if vi.Edited {
		return nil, nil, nil, "clause-4 (body-unedited)",
			"the scan-delta issue body was edited after creation — refused; the signature is trustworthy only for the ORIGINAL body GitHub's last_edited_at reports as unedited"
	}

	rawPayload, _, _ := verdictParseBody(vi.Body) // already succeeded inside scanDeltaVerifyBody
	var payload scanDeltaPayload
	if uerr := json.Unmarshal(rawPayload, &payload); uerr != nil {
		return nil, nil, nil, "clause-4 (payload)", fmt.Sprintf("scan-delta payload does not parse as JSON: %v", uerr)
	}
	if payload.Schema != scanDeltaSchemaVersion {
		return nil, nil, nil, "clause-4 (schema)",
			fmt.Sprintf("unrecognized scan-delta schema %q (want %q) — refused rather than guessed", payload.Schema, scanDeltaSchemaVersion)
	}

	dir := filepath.Join(root, "docs", "streams", scanStreamName)
	for _, e := range payload.Entries {
		res := verifyScanDeltaEntry(homeRepo, e, resolveAuthor)
		results = append(results, res)
		if res.Outcome != scanDeltaAccepted {
			continue
		}
		if strings.Contains(res.Clause, "unreadable") {
			notices = append(notices, res.Reason)
		}
		key := e.Repo + "#" + strconv.Itoa(e.Issue)
		if existing[key] {
			notices = append(notices, fmt.Sprintf("%s: a placeholder already exists on the board — skipped, never duplicated", key))
			continue
		}
		path := filepath.Join(dir, placeholderFileName(e.Repo, e.Issue))
		if _, serr := os.Stat(path); serr == nil {
			notices = append(notices, fmt.Sprintf("%s: a file already occupies %s — skipped, never overwritten", key, path))
			continue
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		creates = append(creates, scanPlan{
			Repo:    e.Repo,
			Issue:   e.Issue,
			Path:    path,
			Rel:     filepath.ToSlash(rel),
			Content: e.Body,
		})
	}
	sort.Slice(creates, func(i, j int) bool {
		if creates[i].Repo != creates[j].Repo {
			return creates[i].Repo < creates[j].Repo
		}
		return creates[i].Issue < creates[j].Issue
	})
	return creates, results, notices, "", ""
}

// runTranscribeScanDelta is the --transcribe-scan-delta entrypoint (the R-7
// clause-4 cross-repo verify path). It ships behind the SAME R-7 enactment
// gate as runTranscribeScan — this task adds NO second arming path — and
// sweeps every open issue on the home repo the same way runTranscribeVerdict
// sweeps verdict issues: a cheap body-shape test (verdictHasPayloadBlock)
// selects candidates, and an issue with no payload block is not a scan-delta
// issue and is skipped silently (no log noise). dryRun is the no-write
// "--check" surface.
func runTranscribeScanDelta(root string, dryRun bool, pubkeyPath string,
	list issueLister, resolveIssue verdictIssueResolver, resolveAuthor authorResolver,
	resolveSignoff commentResolver) int {

	if err := scanRosterUnconfiguredError(); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan-delta REFUSED:", err)
		return 2
	}
	if !dryRun && !scanInCI() {
		if reason := scanIsolationRefusal(root); reason != "" {
			fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan-delta REFUSED:", reason)
			return 2
		}
	}

	armed, reason := transcribeEnactmentGate(root, resolveSignoff)
	if !armed {
		fmt.Println("transcribe-scan-delta: INERT —", reason)
		fmt.Println("transcribe-scan-delta: evaluating no clause; the lane is disarmed until R-7 is signed")
		return 0
	}
	fmt.Println("transcribe-scan-delta:", reason)

	issueLoop, ok := scanDeltaIssueLoopIdentity()
	if !ok {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan-delta REFUSED: no `issue-loop=` App bound in ASSAY_TRUSTED_BOT_SLUGS with a numeric id — there is no issue-loop identity whose scan-delta issues could be trusted.")
		return 2
	}

	homeRepo := scanHomeRepo()
	if homeRepo == "" {
		fmt.Println("transcribe-scan-delta: no home repo configured (ASSAY_HOME_REPO) — nothing to sweep")
		return 0
	}

	pub, perr := scanDeltaResolvePubkey(pubkeyPath)
	if perr != nil {
		// A missing/unreadable issue-loop public key is could-not-check for
		// EVERY candidate issue, not a crash: report it and let clause-4
		// (signature) name the same fact per issue (pub stays nil).
		fmt.Println("transcribe-scan-delta: could-not-check — no usable issue-loop public key:", perr)
	}

	streams, _, serr := loadStreams(root)
	if serr != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan-delta:", serr)
		return 1
	}
	attachPlaceholders(streams)
	existing := existingPlaceholderIssues(streams)
	for _, s := range streams {
		for _, path := range archivedPlaceholderFilePaths(s) {
			if ph, ok, perr := parsePlaceholderFile(path); perr == nil && ok {
				existing[ph.Repo+"#"+strconv.Itoa(ph.Issue)] = true
			}
		}
	}

	issues, lerr := list(homeRepo)
	if lerr != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan-delta:", lerr)
		return 1
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })

	var allCreates []scanPlan
	var allResults []scanDeltaEntryResult
	var notices []string
	seen := 0
	for _, iss := range issues {
		vi, rerr := resolveIssue(homeRepo, iss.Number)
		if rerr != nil {
			notices = append(notices, fmt.Sprintf("%s#%d could not be read: %v", homeRepo, iss.Number, rerr))
			continue
		}
		if !verdictHasPayloadBlock(vi.Body) {
			continue // not a scan-delta issue
		}
		seen++
		creates, results, ns, clause, why := planScanDelta(root, iss.Number, vi, pub, homeRepo, issueLoop, resolveAuthor, existing)
		notices = append(notices, ns...)
		if clause != "" {
			fmt.Printf("transcribe-scan-delta: REFUSE %s#%d — %s: %s\n", homeRepo, iss.Number, clause, why)
			continue
		}
		allResults = append(allResults, results...)
		// --- R-7 cl.6 flood tripwire, applied to THIS payload as a whole
		// (brief fact: "the tripwire applies to the payload as a whole"). ---
		if len(creates) > transcribeFloodThreshold {
			fmt.Printf("transcribe-scan-delta: FLOOD %s#%d — clause-6 tripwire: %d CREATEs exceed the threshold of %d; refusing this issue's delta (nothing written for it)\n",
				homeRepo, iss.Number, len(creates), transcribeFloodThreshold)
			continue
		}
		for _, key := range creates {
			existing[key.Repo+"#"+strconv.Itoa(key.Issue)] = true // dedupe across issues in the same sweep
		}
		allCreates = append(allCreates, creates...)
	}
	fmt.Printf("transcribe-scan-delta: swept %d open issue(s), %d carried a scan-delta payload block\n", len(issues), seen)

	for _, r := range allResults {
		switch r.Outcome {
		case scanDeltaEntryRefused:
			fmt.Printf("transcribe-scan-delta: REFUSE %s#%d — %s: %s\n", r.Entry.Repo, r.Entry.Issue, r.Clause, r.Reason)
		case scanDeltaEntryCouldNotCheck:
			fmt.Printf("transcribe-scan-delta: COULD-NOT-CHECK %s#%d — %s: %s\n", r.Entry.Repo, r.Entry.Issue, r.Clause, r.Reason)
		}
	}
	for _, n := range notices {
		fmt.Println("transcribe-scan-delta: NOTICE —", n)
	}
	for _, c := range allCreates {
		fmt.Printf("transcribe-scan-delta: CREATE %s  (%s#%d)\n", c.Rel, c.Repo, c.Issue)
	}

	if dryRun {
		if len(allCreates) == 0 {
			fmt.Println("transcribe-scan-delta: no changes — nothing to create")
		}
		return 0
	}

	if len(allCreates) > 0 {
		if merr := os.MkdirAll(filepath.Join(root, "docs", "streams", scanStreamName), 0o755); merr != nil {
			fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan-delta:", merr)
			return 1
		}
		for _, p := range allCreates {
			if werr := os.WriteFile(p.Path, []byte(p.Content), 0o644); werr != nil {
				fmt.Fprintln(os.Stderr, "statusgen --transcribe-scan-delta:", werr)
				return 1
			}
			fmt.Printf("transcribe-scan-delta: created %s\n", p.Rel)
		}
	}
	return 0
}
