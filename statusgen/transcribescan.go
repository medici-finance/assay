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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
