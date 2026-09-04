package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// --scan-issues is a self-contained sub-command (like --verify-issues): it reads
// OPEN GitHub issues across a fixed repo set, EMITS a placeholder-v1 file
// (schema) for every unhandled one, and RETIRES placeholders whose
// issue is now closed (close-out). It WRITES files but never pushes,
// never mutates any GitHub issue, and never touches STATUS.md — the desk reviews
// the batch and commits. The offline --lint gate never calls this:
// it is dispatched before run() in main() and shells out to gh, so lint gains no
// network dependency (registers-in-git principle).

// scanHomeRepo is the repo whose placeholders use the bare `issue-<NN>.md`
// filename. Foreign-repo issues get a `<repo-name>-issue-<NN>.md` prefix so their
// stems stay collision-free (placeholderNameRe accepts both).
//
// It reads the adopter's HOME repo from the roster (ASSAY_HOME_REPO, via
// verifyRepoSlug) rather than compiling in one organisation's slug. Unset yields
// "" — no repo then matches, so every repo gets the prefixed filename (harmless;
// idempotency keys on repo#issue, not on the filename).
func scanHomeRepo() string { return verifyRepoSlug() }

// scanStreamName is the stream directory placeholders land in
// ("placeholders land here"). New files are written to docs/streams/<scanStreamName>/.
const scanStreamName = "issue-loop"

// scanRepos is the repo set --scan-issues reads OPEN issues from: the FULL
// owned-repo set the inbound (intake) desk is the front door for, not just the
// code repos. It is adopter configuration read from the roster (ASSAY_SCAN_REPOS)
// rather than compiled in — the owned-repo roster this externalises out of source
// It mirrors the intake-desk SKILL's boot-step-1
// list, which names this set as its code authority: the two are meant to be one
// authority, so a repo added there is added to the roster in the same change.
//
// It is a DEDICATED roster variable, deliberately NOT ASSAY_ALLOWED_REPOS (the
// desk's write-authorisation boundary). The two sets differ on purpose:
//
//   - The scanner covers repos the desk is the front door for that are NOT write
//     targets (e.g. site repos) — they carry inbound issues but nothing writes to
//     them, so they are outside the write boundary.
//   - The scanner deliberately HOLDS OUT a write-boundary repo whose PRs cannot yet
//     be risk-classed: a placeholder from it reaches Next-up and a worker acts on
//     it, so an un-risk-classable public infra repo would get LESS scrutiny in the
//     scanner than a private app repo. That holdout narrows the READ surface only;
//     the repo stays in ASSAY_ALLOWED_REPOS for the desk's WRITE path.
//
// Reusing ASSAY_ALLOWED_REPOS would silently conflate the two. An UNSET scan set is
// CLOSED: the scanner reads nothing rather than falling back to the write boundary
// (planScan is additionally trust-gated — quarantine unless blessed, fail closed —
// so a PUBLIC repo in the set is safe; see TestScanIssuesTrustGateEnforced).
func scanRepos() []string { return scanEffectiveConfig().ScanRepos }

// scanExcludedLabelSet returns the system-STATE labels that disqualify an open
// issue from becoming a placeholder (issue-loop stream README: "System-emitted
// labels are excluded from scanning"). Each names a closeable state the desk
// machinery emits, not a unit of work — a placeholder for one would be pure
// Next-up noise.
//
// The set is DECLARED ONCE, in `topology.yaml` (labels.system_state), where each
// entry carries its rationale; this reads statusgen's derivation of that source
// (topologyvalues.go). A genuinely new system-state label is added to the
// DECLARED SOURCE — never inferred at run time, and never added here alone.
// ground-truth/04 retired the hand table that used to sit at this spot precisely
// because its twin in tools/desk/cmd/issueboard/board.go had already drifted
// from it (#829: issueboard was missing `review-request`, so the board asked for
// a placeholder on exactly the class of issue this scanner excludes).
func scanExcludedLabelSet() map[string]bool { return labelSetOf(topologySystemStateLabels) }

// ghIssue is the subset of `gh issue list --json …` output the scanner reads.
// gh returns issues only (never PRs) and, with --state open, only open ones.
type ghIssue struct {
	Number int       `json:"number"`
	Title  string    `json:"title"`
	Author ghAuthor  `json:"author"` // trust-gate input; gh CLI renders Apps as "app/<slug>"
	Labels []ghLabel `json:"labels"`
	URL    string    `json:"url"`
}

type ghLabel struct {
	Name string `json:"name"`
}

// issueLister returns the open issues of a repo. The production implementation
// shells out to gh; tests inject a fixture so the scanner is exercised offline.
type issueLister func(repo string) ([]ghIssue, error)

// scanRepoSkip records a scanned repo whose OPEN-issue read failed (rate limit,
// 404/unresolvable, auth) so the scan can distinguish a genuinely clean board
// from one it never managed to read. A skipped repo is a could-not-check, not a
// clean read: its close-out sweep is deliberately NOT run (an empty open set
// would otherwise retire every placeholder as if its issue had closed), and its
// presence forces runScanIssues off the exit-0 clean-board path onto the
// could-not-check exit (assay#186).
type scanRepoSkip struct {
	Repo   string
	Reason string
}

// ghIssueLister is the default issueLister: `gh issue list` for one repo. A gh
// failure is returned as an error so planScan can degrade it to a per-repo NOTICE
// ("gh failure on one repo → skip that repo … don't abort").
func ghIssueLister(repo string) ([]ghIssue, error) {
	out, err := exec.Command("gh", "issue", "list",
		"--repo", repo, "--state", "open", "--limit", "1000",
		"--json", "number,title,author,labels,url").Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("gh issue list --repo %s: %v %s", repo, err, detail)
	}
	var issues []ghIssue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing gh output for %s: %w", repo, err)
	}
	return issues, nil
}

// labelNames flattens gh's label objects to their names, dropping blanks.
func labelNames(labels []ghLabel) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if n := strings.TrimSpace(l.Name); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// hasExcludedLabel reports whether any label is in the system-state set
// (case-insensitive).
func hasExcludedLabel(labels []string) bool {
	set := scanExcludedLabelSet()
	for _, l := range labels {
		if set[strings.ToLower(strings.TrimSpace(l))] {
			return true
		}
	}
	return false
}

// placeholderStem is the filename stem (and Brief.Num) for a repo+issue: the bare
// `issue-<NN>` for the home repo, `<repo-name>-issue-<NN>` for a foreign repo. The
// stem is unique per repo+issue even when two repos share an issue number.
func placeholderStem(repo string, issue int) string {
	if home := scanHomeRepo(); home != "" && repo == home {
		return fmt.Sprintf("issue-%d", issue)
	}
	name := repo
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		name = repo[i+1:]
	}
	return fmt.Sprintf("%s-issue-%d", name, issue)
}

// placeholderFileName is placeholderStem + ".md" — the file the scanner writes and
// the idempotency marker it checks for.
func placeholderFileName(repo string, issue int) string {
	return placeholderStem(repo, issue) + ".md"
}

// yamlFlowScalar renders one YAML flow-sequence item, quoting it only when a plain
// scalar would be ambiguous in flow context (commas/brackets/colons/etc. or
// surrounding space). Keeps the common case (`bug`) unquoted, matching the
// hand-written placeholder shape, while staying safe for exotic label names.
func yamlFlowScalar(s string) string {
	if s == "" || s != strings.TrimSpace(s) || strings.ContainsAny(s, ",:[]{}#&*!|>'\"%@`\n\t") {
		return strconv.Quote(s)
	}
	return s
}

// yamlFlowList renders a []string as a YAML flow sequence, e.g. `[bug, security]`.
func yamlFlowList(items []string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = yamlFlowScalar(it)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// renderPlaceholder builds a placeholder-v1 file body for one issue. The gate is
// written EXPLICITLY: derivePlaceholderGate needs the issue TITLE (the risk
// keyword/label trigger), which the offline parser cannot see — so the scanner,
// which has the title, computes the gate once and records it. A human may later
// override the stored `gate:`. effort is left to derivation (always M today).
func renderPlaceholder(repo string, issue int, gate string, labels []string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("schema: placeholder-v1\n")
	fmt.Fprintf(&b, "brief: %s/%s\n", scanStreamName, placeholderStem(repo, issue))
	fmt.Fprintf(&b, "issue: %d\n", issue)
	fmt.Fprintf(&b, "repo: %s\n", repo)
	b.WriteString("wave: 0\n")
	b.WriteString("status: todo\n")
	fmt.Fprintf(&b, "gate: %s\n", gate)
	if len(labels) > 0 {
		fmt.Fprintf(&b, "labels: %s\n", yamlFlowList(labels))
	}
	b.WriteString("---\n")
	fmt.Fprintf(&b, "See issue #%d — the issue body is the spec.\n", issue)
	return b.String()
}

// scanPlan is one placeholder the scan would create (dry-run) or created.
type scanPlan struct {
	Repo   string
	Issue  int
	Path   string // absolute target path
	Rel    string // path relative to root, for display
	Gate   string
	Labels []string
	// Content is the rendered placeholder-v1 file body.
	Content string
}

// closeOutPlan is one placeholder the scan would retire or reactivate. It is
// computed in the same sweep as creation: a closed issue retires
// its placeholder (status: done + resolved: issue-close); a reopened issue
// re-activates its placeholder (status: todo, resolved removed). An OPEN issue
// that gains an excluded label retires the same way but with a
// distinct resolved marker (resolved: label-excluded) — reactivate is the same
// action either way, since the placeholder's current status is the only signal
// that matters once the issue is open and no longer excluded.
type closeOutPlan struct {
	Repo   string
	Issue  int
	Path   string // absolute path of the existing placeholder file
	Rel    string // path relative to root, for display
	Action string // "retire", "retire-label", "sweep", or "reactivate"
	// Dest is the absolute path the placeholder MOVES to when the action
	// archives or reactivates it (D3): retire/retire-label/sweep
	// move a root placeholder into `<stream>/done/`; a reactivate of an archived
	// placeholder moves it back to the stream root. "" means write in place (no
	// move) — e.g. reactivating a placeholder that is still at the stream root.
	Dest string
}

// archiveDirName is the per-stream subfolder a retired placeholder is moved into
// (D3). ROOT-only discovery globs mean `<stream>/done/` is out of
// the active workset with zero exclusion code, while linkcheck's recursive walk
// keeps it lint-valid.
const archiveDirName = "done"

// isArchivedPath reports whether p sits directly inside a per-stream `done/`
// archive (its immediate parent directory is named `done`).
func isArchivedPath(p string) bool {
	return filepath.Base(filepath.Dir(p)) == archiveDirName
}

// archivedPath returns p relocated into its stream's `done/` archive.
func archivedPath(p string) string {
	return filepath.Join(filepath.Dir(p), archiveDirName, filepath.Base(p))
}

// unarchivedPath returns an archived p relocated back to the stream root (strips
// the trailing `done/` path element).
func unarchivedPath(p string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(p)), filepath.Base(p))
}

// existingPlaceholderIssues returns the set of `repo#issue` keys that already have
// a placeholder ANYWHERE across the loaded streams (requires attachPlaceholders to
// have run). The file's existence is the idempotency marker: a
// second scan creates nothing for an issue that already has one.
func existingPlaceholderIssues(streams []*Stream) map[string]bool {
	keys := map[string]bool{}
	for _, s := range streams {
		for _, ph := range s.Placeholders {
			keys[ph.Repo+"#"+strconv.Itoa(ph.Issue)] = true
		}
	}
	return keys
}

// planScan computes the placeholders to create AND the existing placeholders to
// retire/reactivate for the fixed repo set. It never writes. A gh failure on one
// repo becomes a NOTICE and the scan proceeds with the rest.
// Issues are skipped when they carry an excluded label, when a placeholder already
// exists for the issue (parsed set), or when the target file is already on disk
// (guards an unparseable/foreign file from being overwritten).
//
// Close-out runs in the same sweep: an existing placeholder whose
// issue is now CLOSED (not in the open list) is retired (status: done + resolved:
// issue-close); a retired placeholder whose issue reopened (back in the open list)
// is reactivated (status: todo, resolved removed). A placeholder already at the
// target status is skipped — the action is idempotent.
//
// an existing placeholder whose issue is still OPEN but has GAINED an
// excluded label (the system-state set — e.g. review-request/needs-decision) is
// retired the same way, but with a distinct resolved: label-excluded marker
// (status: done + resolved: label-excluded) so it reads apart from an
// issue-close retire. When the label is later removed while the issue stays
// open, the placeholder is reactivated by the SAME path as a reopen (status:
// todo, resolved removed) — reactivation only ever looks at "is the issue open
// and not excluded, and is the placeholder currently done", never at which
// resolved marker put it there.
//
// Output is sorted (repo, then issue) for deterministic dry-run and write order.
func planScan(root string, streams []*Stream, list issueLister, bless issueBlessChecker) (plans []scanPlan, closeOuts []closeOutPlan, notices []string, skipped []scanRepoSkip) {
	existing := existingPlaceholderIssues(streams)
	// (D3): archived placeholders (already moved into
	// `<stream>/done/`) count as EXISTING — a reopened issue reactivates its
	// archived placeholder (below), it must never get a second one created at the
	// stream root. ROOT-only discovery leaves them out of existingPlaceholderIssues,
	// so seed them here explicitly.
	for _, s := range streams {
		for _, path := range archivedPlaceholderFilePaths(s) {
			if ph, ok, err := parsePlaceholderFile(path); err == nil && ok {
				existing[ph.Repo+"#"+strconv.Itoa(ph.Issue)] = true
			}
		}
	}
	dir := filepath.Join(root, "docs", "streams", scanStreamName)
	for _, repo := range scanRepos() {
		issues, err := list(repo)
		if err != nil {
			// A read failure is a could-not-check for THIS repo, not a clean
			// empty read. Record it structurally (skipped) as well as a NOTICE so
			// runScanIssues can refuse the exit-0 clean-board path and name the
			// unread repo. The `continue` also skips this repo's close-out sweep
			// below, which is essential: an empty open set would otherwise retire
			// every one of its placeholders as if the issue had closed.
			skipped = append(skipped, scanRepoSkip{Repo: repo, Reason: err.Error()})
			notices = append(notices, fmt.Sprintf("scan-issues: %s skipped — %v", repo, err))
			continue
		}
		// Build the set of OPEN issue numbers for this repo — used for both
		// creation (is it already handled?) and close-out (is it now closed?).
		// openExcluded additionally records, per open issue, whether it CURRENTLY
		// carries an excluded label — the close-out sweep below needs this to
		// retire a placeholder whose open issue gained one, and to
		// reactivate one whose issue lost it while staying open.
		openSet := map[int]bool{}
		openExcluded := map[int]bool{}
		for _, iss := range issues {
			openSet[iss.Number] = true
			labels := labelNames(iss.Labels)
			excluded := hasExcludedLabel(labels)
			openExcluded[iss.Number] = excluded
			if excluded {
				continue
			}
			if existing[repo+"#"+strconv.Itoa(iss.Number)] {
				continue
			}
			// Trust gate (trustgate.go):
			// a placeholder is a durable desk work item — NEVER create one for an
			// untrusted-author issue without a current blessing. The bounded
			// blessing read runs only for untrusted authors; a failed read fails
			// CLOSED (no placeholder, loud NOTICE). Quarantined issues surface as
			// NOTICEs here and on issueboard's EXTERNAL / UNBLESSED lane; one authority
			// comment admits them on the next sweep. Close-out below is NOT gated:
			// retiring/reactivating touches our local placeholder state, not the
			// issue's content.
			if !trustedAuthor(iss.Author.Login) {
				blessed, berr := bless(repo, iss.Number)
				if berr != nil {
					notices = append(notices, fmt.Sprintf(
						"scan-issues: %s#%d NOT created — trust gate unverifiable: %v", repo, iss.Number, berr))
					continue
				}
				if !blessed {
					notices = append(notices, fmt.Sprintf(
						"scan-issues: %s#%d quarantined (trust gate): author %q is not a trusted desk identity "+
							"and no current blessing exists — a comment from the configured blessing authority admits it",
						repo, iss.Number, iss.Author.Login))
					continue
				}
			}
			path := filepath.Join(dir, placeholderFileName(repo, iss.Number))
			if _, err := os.Stat(path); err == nil {
				continue // a file already occupies the target path — never overwrite
			}
			gate := derivePlaceholderGate(labels, iss.Title)
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			plans = append(plans, scanPlan{
				Repo:    repo,
				Issue:   iss.Number,
				Path:    path,
				Rel:     filepath.ToSlash(rel),
				Gate:    gate,
				Labels:  labels,
				Content: renderPlaceholder(repo, iss.Number, gate, labels),
			})
		}

		// Close-out: check every existing ROOT placeholder for this repo against
		// the open set. A placeholder whose issue is NOT open → retire (mark done
		// + archive to done/). A retired placeholder whose issue IS open →
		// reactivate (mark todo). (D3): retire/retire-label/sweep
		// additionally MOVE the file into `<stream>/done/`; and ANY status:done
		// placeholder still sitting at the stream ROOT is swept into done/ in the
		// same run (drains the ghost backlog; self-heals thereafter).
		for _, s := range streams {
			for _, ph := range s.Placeholders {
				if ph.Repo != repo {
					continue
				}
				rel, err := filepath.Rel(root, ph.Path)
				if err != nil {
					rel = ph.Path
				}
				rel = filepath.ToSlash(rel)
				dest := archivedPath(ph.Path)
				if !openSet[ph.Issue] {
					// Issue is NOT open → closed.
					if ph.Status != "done" {
						// Retire: flip to done + archive.
						closeOuts = append(closeOuts, closeOutPlan{
							Repo: repo, Issue: ph.Issue, Path: ph.Path, Rel: rel,
							Action: "retire", Dest: dest,
						})
					} else {
						// Already done but still at the ROOT → sweep the ghost
						// into done/ (frontmatter unchanged).
						closeOuts = append(closeOuts, closeOutPlan{
							Repo: repo, Issue: ph.Issue, Path: ph.Path, Rel: rel,
							Action: "sweep", Dest: dest,
						})
					}
					continue
				}
				// Issue is still open.
				if openExcluded[ph.Issue] {
					// Open but has GAINED an excluded label — retire it the same
					// way as a close-out, but with a distinct resolved marker
					// (applyCloseOut writes resolved: label-excluded) so a later
					// label removal can reactivate like a reopen does.
					if ph.Status != "done" {
						closeOuts = append(closeOuts, closeOutPlan{
							Repo: repo, Issue: ph.Issue, Path: ph.Path, Rel: rel,
							Action: "retire-label", Dest: dest,
						})
					} else {
						// Already done + still excluded, but sitting at the ROOT
						// → sweep it into done/ too (frontmatter unchanged).
						closeOuts = append(closeOuts, closeOutPlan{
							Repo: repo, Issue: ph.Issue, Path: ph.Path, Rel: rel,
							Action: "sweep", Dest: dest,
						})
					}
					continue
				}
				// Open, and no (longer) excluded label — if the placeholder was
				// retired (status: done, via either an issue-close or a
				// label-exclusion retire) but is STILL at the root, re-activate it
				// in place. Reactivation doesn't care WHICH resolved marker put it
				// at done.
				if ph.Status == "done" {
					closeOuts = append(closeOuts, closeOutPlan{
						Repo: repo, Issue: ph.Issue, Path: ph.Path, Rel: rel,
						Action: "reactivate", // Dest "" — already at the root
					})
				}
			}
		}

		// Reactivation from the archive (D3): a placeholder that
		// was archived to `<stream>/done/` and whose issue has REOPENED (open and
		// not excluded) is reactivated AND moved back to the stream root. Archived
		// placeholders whose issue stays closed/excluded are left in place.
		for _, s := range streams {
			for _, path := range archivedPlaceholderFilePaths(s) {
				ph, ok, err := parsePlaceholderFile(path)
				if err != nil || !ok || ph.Repo != repo {
					continue
				}
				if !openSet[ph.Issue] || openExcluded[ph.Issue] {
					continue
				}
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				closeOuts = append(closeOuts, closeOutPlan{
					Repo: repo, Issue: ph.Issue, Path: path,
					Rel: filepath.ToSlash(rel), Action: "reactivate",
					Dest: unarchivedPath(path),
				})
			}
		}
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Repo != plans[j].Repo {
			return plans[i].Repo < plans[j].Repo
		}
		return plans[i].Issue < plans[j].Issue
	})
	sort.Slice(closeOuts, func(i, j int) bool {
		if closeOuts[i].Repo != closeOuts[j].Repo {
			return closeOuts[i].Repo < closeOuts[j].Repo
		}
		return closeOuts[i].Issue < closeOuts[j].Issue
	})
	sort.Slice(skipped, func(i, j int) bool { return skipped[i].Repo < skipped[j].Repo })
	return plans, closeOuts, notices, skipped
}

// scanReadSummary renders the per-run "repos read vs skipped" line printed on
// EVERY --scan-issues run, even a genuinely empty one — so a caller can always
// see how many repos the run actually managed to read. When repos went unread it
// names each with its reason, so the summary alone distinguishes a partial scan
// from a clean board (assay#186).
func scanReadSummary(total int, skipped []scanRepoSkip) string {
	read := total - len(skipped)
	if len(skipped) == 0 {
		return fmt.Sprintf("scan-issues: read %d of %d configured repos (0 skipped)", read, total)
	}
	parts := make([]string, 0, len(skipped))
	for _, s := range skipped {
		parts = append(parts, fmt.Sprintf("%s (%s)", s.Repo, s.Reason))
	}
	return fmt.Sprintf("scan-issues: read %d of %d configured repos — COULD-NOT-CHECK: %d skipped: %s",
		read, total, len(skipped), strings.Join(parts, "; "))
}

// runScanIssues is the --scan-issues entrypoint. It loads streams (for
// idempotency and existing placeholders), unblocks any blocked placeholders whose
// issue received a human answer, plans new placeholder creation AND
// close-out of resolved placeholders in one sweep, and either lists
// everything (--dry-run) or writes the files. It never pushes, never mutates
// GitHub, and never touches STATUS.md. Returns a process exit code.
func runScanIssues(root string, dryRun bool, list issueLister, comments commentLister, bless issueBlessChecker) int {
	// P3: the effective-configuration echo now runs ONCE from main, for every mode
	// (it used to fire only here, so --lint — the mode the consumer's CI runs —
	// printed nothing about the roster it judged with).

	// P1: unset is CLOSED. --scan-issues WRITES durable work items from issues on
	// repos arbitrary external users can author, and the trust roster is exactly
	// what gates that write. With no roster configured there is nothing to gate
	// with, so the scan REFUSES rather than running ungated. It must never
	// degrade to "no roster ⇒ no filtering".
	if err := scanRosterUnconfiguredError(); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen --scan-issues REFUSED:", err)
		return 2
	}

	// Isolation guard (scanguard.go, 2026-08-13 incident): a PRIMARY checkout
	// as the scan root is refused before ANY write happens — including the
	// un-block edits below, which run in --dry-run mode too. The sanctioned
	// root is an isolated linked worktree cut fresh from origin/main; the
	// refusal message carries the recipe and the human override.
	if reason := scanIsolationRefusal(root); reason != "" {
		fmt.Fprintln(os.Stderr, "statusgen --scan-issues REFUSED:", reason)
		return 2
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	attachPlaceholders(streams)

	// Un-block detection: for each blocked placeholder, check if the newest
	// non-bot comment on the issue is newer than blockedAt — if so, the block has
	// cleared. planUnblock only COMPUTES the set (mirrors planScan): the write —
	// stripping the blocked:/blockedAt: lines — is deferred to applyUnblock in the
	// write path below, so --dry-run performs ZERO filesystem writes (#852). A
	// per-repo comment-fetch failure becomes a NOTICE; planning continues with the
	// remaining repos.
	unblockPlans, unblockNotices := planUnblock(streams, comments)
	notices := append([]string(nil), unblockNotices...)

	plans, closeOuts, scanNotices, skipped := planScan(root, streams, list, bless)
	notices = append(notices, scanNotices...)
	emitNotices(notices) // per-repo gh failures ALSO surface as NOTICE

	// A run that could not read one or more scanned repos (rate limit,
	// 404/unresolvable, auth) is a could-not-check, not a clean read of an empty
	// world. Print the read/skipped summary on EVERY run (even a genuinely empty
	// one), suppress the clean-board line whenever a repo went unread, and exit on
	// statusgen's could-not-check code (2) so a cron/desk can tell partial from
	// clean rather than reporting a clean intake lane through a rate-limit window
	// (assay#186).
	fmt.Println(scanReadSummary(len(scanRepos()), skipped))
	partial := len(skipped) > 0

	if dryRun {
		for _, u := range unblockPlans {
			fmt.Printf("would unblock %s  (%s#%d, newest non-bot comment %s > blockedAt %s)\n",
				u.Path, u.Repo, u.Issue,
				u.Newest.Format(time.RFC3339), u.BlockedAt.Format(time.RFC3339))
		}
		for _, p := range plans {
			fmt.Printf("would create %s  (%s#%d, gate:%s)\n", p.Rel, p.Repo, p.Issue, p.Gate)
		}
		for _, c := range closeOuts {
			fmt.Printf("would %s %s  (%s#%d, %s)\n", c.Action, c.Rel, c.Repo, c.Issue, closeOutReason(c.Action))
		}
		if len(unblockPlans) == 0 && len(plans) == 0 && len(closeOuts) == 0 && !partial {
			fmt.Println("scan-issues: no changes — nothing to create or retire")
		}
		if partial {
			return 2
		}
		return 0
	}

	// Write path (non-dry-run): apply the unblocks the dry-run branch only
	// reported. A write failure degrades to a NOTICE; the loop continues.
	for _, u := range unblockPlans {
		if err := applyUnblock(u); err != nil {
			emitNotices([]string{fmt.Sprintf("scan-issues: failed to unblock %s — %v", u.Path, err)})
			continue
		}
		fmt.Printf("unblocked %s (newest non-bot comment %s > blockedAt %s)\n",
			u.Path, u.Newest.Format(time.RFC3339), u.BlockedAt.Format(time.RFC3339))
	}

	// Execute close-outs BEFORE creation — a retired placeholder's target path
	// stays occupied, so creation will never collide; but a reactivated one
	// goes back to todo, which is the correct base state.
	for _, c := range closeOuts {
		if err := applyCloseOut(c); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Printf("%s %s\n", closeOutPastTense(c.Action), c.Rel)
	}

	if len(plans) == 0 {
		if len(closeOuts) == 0 && !partial {
			fmt.Println("scan-issues: no unhandled open issues — nothing to create or retire")
		}
		if partial {
			return 2
		}
		return 0
	}

	dir := filepath.Join(root, "docs", "streams", scanStreamName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	for _, p := range plans {
		if err := os.WriteFile(p.Path, []byte(p.Content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Printf("created %s\n", p.Rel)
	}
	// The successfully-read repos' work is applied above; a partial scan still
	// returns the could-not-check code so the run is never mistaken for a
	// complete, authoritative sweep (assay#186).
	if partial {
		return 2
	}
	return 0
}

// closeOutResolvedMarker returns the "resolved:" value applyCloseOut writes for
// a retire action — "issue-close" for a plain closed-issue retire, "label-
// excluded" for an open-issue-gained-excluded-label retire. Empty
// for "reactivate" (no resolved marker is written).
func closeOutResolvedMarker(action string) string {
	switch action {
	case "retire":
		return "issue-close"
	case "retire-label":
		return "label-excluded"
	default:
		return ""
	}
}

// closeOutReason renders the human-readable reason shown in --dry-run output
// for a closeOutPlan action.
func closeOutReason(action string) string {
	switch action {
	case "retire":
		return "issue closed"
	case "retire-label":
		return "gained excluded label"
	case "sweep":
		return "already retired — archived to done/"
	case "reactivate":
		return "issue open, no longer excluded"
	default:
		return action
	}
}

// closeOutPastTense renders the past-tense verb printed after applyCloseOut
// executes a closeOutPlan action.
func closeOutPastTense(action string) string {
	switch action {
	case "retire":
		return "retired"
	case "retire-label":
		return "retired (label-excluded)"
	case "sweep":
		return "archived"
	case "reactivate":
		return "reactivated"
	default:
		return action
	}
}

// applyCloseOut modifies a placeholder file in-place for a closeOutPlan action.
// For "retire": changes status to "done" and adds/updates a "resolved: issue-close"
// frontmatter field. For "retire-label" (issue still open but gained an
// excluded label): same status change, but the resolved marker is "label-excluded"
// instead, so a later label removal is distinguishable from an issue-close and can
// reactivate the same way a reopen does. For "reactivate": changes status to
// "todo" and removes any "resolved:" field (regardless of which retire path wrote
// it). The body prose is left intact.
// Editing is line-based within the frontmatter block (between --- markers),
// matching the pattern established by unblockPlaceholderFile.
func applyCloseOut(c closeOutPlan) error {
	data, err := os.ReadFile(c.Path)
	if err != nil {
		return fmt.Errorf("%s: %w", c.Rel, err)
	}

	// "sweep" (D3) moves an already-retired ghost into done/
	// WITHOUT touching frontmatter — the file is already status: done + resolved.
	if c.Action == "sweep" {
		return writeCloseOut(c, data)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	var out []string
	inFrontmatter := false
	frontmatterClosed := false
	hasStatus := false
	hasResolved := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !frontmatterClosed {
			if trimmed == "---" {
				if !inFrontmatter {
					inFrontmatter = true
				} else {
					frontmatterClosed = true
				}
				out = append(out, line)
				continue
			}
			if inFrontmatter {
				if strings.HasPrefix(trimmed, "status:") {
					hasStatus = true
					switch c.Action {
					case "retire", "retire-label":
						out = append(out, "status: done")
					case "reactivate":
						out = append(out, "status: todo")
					}
					continue
				}
				if strings.HasPrefix(trimmed, "resolved:") {
					hasResolved = true
					if marker := closeOutResolvedMarker(c.Action); marker != "" {
						out = append(out, "resolved: "+marker)
					}
					// reactivate: drop the resolved line
					continue
				}
				// For retire: if we're at the closing --- (next iteration
				// handled above) and haven't added resolved yet, we'll
				// insert it just before the closing ---. Track position.
			}
		}
		out = append(out, line)
	}

	// If status was missing (malformed file), don't inject one — the file
	// will be caught by checkPlaceholderFiles on the next lint run.
	if !hasStatus {
		return fmt.Errorf("%s: no status field found in frontmatter", c.Rel)
	}

	// For retire/retire-label: inject a resolved field if one wasn't already
	// present. Insert it just before the closing --- of the frontmatter.
	if marker := closeOutResolvedMarker(c.Action); marker != "" && !hasResolved {
		// Find the closing --- (second one) and insert before it.
		var result []string
		fmCloseCount := 0
		for _, line := range out {
			if strings.TrimSpace(line) == "---" {
				fmCloseCount++
				if fmCloseCount == 2 {
					result = append(result, "resolved: "+marker)
				}
			}
			result = append(result, line)
		}
		out = result
	}

	return writeCloseOut(c, []byte(strings.Join(out, "\n")))
}

// writeCloseOut persists a closeOutPlan's transformed body. When c.Dest is set
// and differs from c.Path the placeholder is MOVED there (D3):
// the destination directory is created, the body written, and the source removed
// — a delete+add `git add -A` records as a rename, preserving history. An empty
// or identical Dest writes in place.
func writeCloseOut(c closeOutPlan, body []byte) error {
	dest := c.Dest
	if dest == "" {
		dest = c.Path
	}
	if dest != c.Path {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("%s: %w", c.Rel, err)
		}
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		return fmt.Errorf("%s: %w", c.Rel, err)
	}
	if dest != c.Path {
		if err := os.Remove(c.Path); err != nil {
			return fmt.Errorf("%s: %w", c.Rel, err)
		}
	}
	return nil
}

// -- comment types and un-block machinery --

// issueComment is the subset of `gh api repos/.../issues/.../comments` output the
// un-block scanner reads. It needs the author login + type (User/Bot) for the
// non-bot gate, createdAt for the temporal comparison against blockedAt, and body
// for the desk-automation marker check (bot-vs-human gate Blocker 2).
type issueComment struct {
	User      issueCommentUser `json:"user"`
	CreatedAt string           `json:"created_at"`
	Body      string           `json:"body"`
}

type issueCommentUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`   // numeric database id — recycled-login defense on the unblock gate
	Type  string `json:"type"` // "User" or "Bot"
}

// commentLister fetches issue comments for one repo+issue. The production
// implementation shells out to gh; tests inject a fixture.
type commentLister func(repo string, issue int) ([]issueComment, error)

// issueCommentLister is the default commentLister: `gh api` for one issue's comments.
// A gh failure is returned as an error so the caller can degrade it to a NOTICE.
// --paginate fetches all pages (GitHub defaults to 30/page ascending; the resume
// signal is always on the last page, so page-1-only silently drops it — Blocker 1).
// --paginate composes with --jq (applied per page), so the
// existing line-per-object parse loop is unchanged.
func issueCommentLister(repo string, issue int) ([]issueComment, error) {
	endpoint := fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", repo, issue)
	out, err := exec.Command("gh", "api", "--paginate", endpoint,
		"--jq", ".[] | {user: {login: .user.login, id: .user.id, type: .user.type}, created_at: .created_at, body: .body}").Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("gh api %s: %v %s", endpoint, err, detail)
	}
	// gh --jq emits one JSON object per line (not a JSON array).
	var comments []issueComment
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c issueComment
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("parsing gh comment for %s#%d: %w", repo, issue, err)
		}
		comments = append(comments, c)
	}
	return comments, nil
}

// isBotComment reports whether a comment author is a bot, GitHub App actor, or
// desk automation. The identity principle: only a human answer un-blocks the
// placeholder; desk/bot comments never count.
//
// Three gates:
//  1. GitHub's own type:Bot actors (a reviewer App, etc.)
//  2. The [bot] login suffix convention (some App actors masquerade as User)
//  3. Body marker: comments that carry the <!-- desk-automation --> HTML comment
//     are the loop's own automation (the desk, fanout workers, status comments)
//     posting as a shared human/agent account (type:User) — the login alone
//     can't distinguish them from a real human's answers (Blocker 2).
func isBotComment(user issueCommentUser, body string) bool {
	if user.Type == "Bot" {
		return true
	}
	// GitHub Apps that act as users (some review/CI tools) are also bots
	// via the "[bot]" suffix convention.
	if strings.HasSuffix(user.Login, "[bot]") {
		return true
	}
	// Comments stamped with the desk-automation marker are the loop's own
	// automation — the desk, fanout workers, and status-posters all share
	// one human/agent account (type:User), so login alone can't
	// distinguish the answer from the automation asking the question.
	if strings.Contains(body, "<!-- desk-automation -->") {
		return true
	}
	return false
}

// newestNonBotCommentTime returns the createdAt of the most-recent non-bot
// comment, or the zero time if there are no non-bot comments. T1: only the
// CONFIGURED blessing authority (a numeric-id-pinned human) can be "the human
// answer" that un-blocks a placeholder — ANY other non-bot login must NOT.
// With the roster unconfigured there is no such login, so nothing un-blocks.
func newestNonBotCommentTime(comments []issueComment) (time.Time, bool) {
	var newest time.Time
	found := false
	for _, c := range comments {
		if isBotComment(c.User, c.Body) {
			continue
		}
		// T1: only the configured blessing authority (login + numeric-id-pinned
		// human) can un-block. A shared agent account, and any other non-bot
		// login, must NOT — an agent or external commenter's answer must not push
		// a parked item back onto the worklist. Login alone is insufficient:
		// GitHub logins are recyclable, so a renamed/re-registered login must NOT
		// unblock — the comment author's numeric id must ALSO match the pinned
		// bless id. isBlessAuthorityID enforces login + id together (matching the
		// discipline every other trust seam uses, e.g. authorizedByIdentity), and
		// fails closed when the roster is unconfigured or the id is absent/wrong.
		if !isBlessAuthorityID(c.User.Login, c.User.ID) {
			continue
		}
		t, err := time.Parse(time.RFC3339, c.CreatedAt)
		if err != nil {
			continue // malformed timestamp → skip
		}
		if !found || t.After(newest) {
			newest = t
			found = true
		}
	}
	return newest, found
}

// unblockPlan is one blocked placeholder whose block has CLEARED — the newest
// non-bot comment on its issue is newer than blockedAt. planUnblock computes the
// set WITHOUT writing (mirroring planScan/scanPlan); applyUnblock performs the
// write. Separating the two lets --dry-run report the unblocks without mutating
// disk (#852): the un-block lane used to write before the dry-run branch.
type unblockPlan struct {
	Repo      string
	Issue     int
	Path      string    // absolute path of the blocked placeholder file
	Newest    time.Time // newest qualifying non-bot comment time (for the message)
	BlockedAt time.Time // parsed blockedAt (for the message)
}

// planUnblock scans all blocked placeholders and COMPUTES which have cleared (the
// newest non-bot comment on the issue is newer than blockedAt). It NEVER writes —
// the write half lives in applyUnblock, so --dry-run can report the plan without
// touching disk. Returns NOTICEs for unparseable blockedAt and per-repo comment
// fetch failures (one repo is degraded, not the whole scan).
func planUnblock(streams []*Stream, list commentLister) (plans []unblockPlan, notices []string) {
	for _, s := range streams {
		for _, ph := range s.Placeholders {
			if ph.Blocked == "" {
				continue
			}
			blockedAt, err := time.Parse(time.RFC3339, ph.BlockedAt)
			if err != nil {
				// blockedAt unparseable — can't determine temporal order;
				// leave the block in place (safe default). Emit a NOTICE
				// so the desk can fix the file manually.
				notices = append(notices, fmt.Sprintf(
					"scan-issues: cannot parse blockedAt for %s#%d (%q) — leaving blocked; fix the placeholder file",
					ph.Repo, ph.Issue, ph.BlockedAt))
				continue
			}
			comments, err := list(ph.Repo, ph.Issue)
			if err != nil {
				notices = append(notices, fmt.Sprintf(
					"scan-issues: unblock check for %s#%d skipped — %v",
					ph.Repo, ph.Issue, err))
				continue
			}
			newest, ok := newestNonBotCommentTime(comments)
			if !ok || !newest.After(blockedAt) {
				continue // no unblocking comment yet
			}
			plans = append(plans, unblockPlan{
				Repo: ph.Repo, Issue: ph.Issue, Path: ph.Path,
				Newest: newest, BlockedAt: blockedAt,
			})
		}
	}
	return plans, notices
}

// applyUnblock performs the write half of the un-block lane: it strips the
// blocked:/blockedAt: lines from the placeholder file. Kept out of planUnblock so
// --dry-run never reaches an os.WriteFile.
func applyUnblock(p unblockPlan) error {
	return unblockPlaceholderFile(p.Path)
}

// unblockPlaceholders scans all blocked placeholders, clears the block when the
// newest non-bot comment on the issue is newer than blockedAt, and WRITES the
// cleared file. It composes planUnblock (compute) + applyUnblock (write); the
// scan entrypoint calls those two halves directly so it can skip the write under
// --dry-run. Returns NOTICEs for fetch failures (one repo is degraded, not the
// whole scan). root is unused (accepted for the historical signature).
func unblockPlaceholders(root string, streams []*Stream, list commentLister) []string {
	plans, notices := planUnblock(streams, list)
	for _, p := range plans {
		if err := applyUnblock(p); err != nil {
			notices = append(notices, fmt.Sprintf(
				"scan-issues: failed to unblock %s — %v", p.Path, err))
			continue
		}
		fmt.Printf("unblocked %s (newest non-bot comment %s > blockedAt %s)\n",
			p.Path, p.Newest.Format(time.RFC3339), p.BlockedAt.Format(time.RFC3339))
	}
	return notices
}

// unblockPlaceholderFile removes the blocked: and blockedAt: lines from the
// frontmatter block of a placeholder-v1 file, leaving all other fields (and the
// body prose) intact. Scoped to the frontmatter block (between --- markers) so
// body prose that happens to start with "blocked:" or "blockedAt:" is not
// stripped (non-blocking nit).
func unblockPlaceholderFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	var out []string
	inFrontmatter := false
	frontmatterClosed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !frontmatterClosed {
			if trimmed == "---" {
				if !inFrontmatter {
					inFrontmatter = true
				} else {
					frontmatterClosed = true
				}
				out = append(out, line)
				continue
			}
			if inFrontmatter && (strings.HasPrefix(trimmed, "blocked:") || strings.HasPrefix(trimmed, "blockedAt:")) {
				continue
			}
		}
		out = append(out, line)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}
