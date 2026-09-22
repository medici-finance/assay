package main

// reconcile.go — the core of `deskreconcile`.
//
// deskreconcile is the desk-side writer for the board-reconcile gap tracked as the
// scheduled-reconcile half of derived-board/04 (issue #1175). A merged brief's Status
// cell stays `todo` forever unless something flips it, because:
//
//   - `statusgen regen --readmes` PRESERVES the lifecycle columns (Status / Verified /
//     Reviewed) on every render — it never derives them (statusgen/readmetable.go header),
//   - the ONLY writer of a Status cell is `statusgen reconcile --backfill --apply`, under
//     the narrow, safe conditions its own header states — only todo|in-progress ->
//     implemented, only with a real merged-PR witness, Status cell only
//     (statusgen/reconcileapply.go header), and
//   - no CI workflow invokes it (there is no `readmes`/`reconcile` hit in
//     .github/workflows/*.yml), while the workflow change that WOULD (#1175 / PR #428) is
//     blocked because no App may push workflow files.
//
// deskreconcile removes the workflow dependency: it runs that one writer from a desk verb
// the desk/worker App CAN run, and carries the result as ONE draft PR on a FIXED branch,
// so a scheduled invocation never opens a second PR.
//
// It does the whole cycle in an ISOLATED worktree cut from the fetched remote head — the
// same three non-negotiables scanloop's scan-carrier lane learned (isolate, sync fresh,
// commit-and-carry) — and every command goes through the Exec seam so the sequence is
// unit-testable without a checkout, a remote, or a token.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// FlipRow is one Status-cell flip that `statusgen reconcile --apply` reported in its
// `applied` array — the same shape statusgen/reconcileapply.go's appliedRow emits.
type FlipRow struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Path    string `json:"readme"`
	Witness string `json:"witness"`
}

// reconcileOutput is the slice of statusgen's --json output deskreconcile reads: only the
// rows the run actually WROTE. Everything else statusgen emits is ignored here — the
// authoritative signal that something changed is the worktree diff, not this list (see
// changedReadmes); this list is only how the flips are NAMED in the report.
type reconcileOutput struct {
	Applied []FlipRow `json:"applied"`
}

// Exec is the process seam. Every command deskreconcile runs — git, statusgen, deskpr —
// goes through it, so the command SHAPES are testable without a real checkout, remote or
// token. It returns the command's combined output.
type Exec func(dir, name string, args ...string) (string, error)

// Options configures one reconcile run.
type Options struct {
	// Root is the target repo checkout deskreconcile fetches origin/main into. It is the
	// item's own repo; deskreconcile never writes into it (all writes land in Worktree).
	Root string
	// Worktree is the isolated linked worktree the whole cycle runs in. It must be an
	// ABSOLUTE path outside Root — never nested inside the checkout being reconciled.
	Worktree string
	// Branch is the FIXED carry branch. Production always passes board/reconcile; a fixed
	// branch is what makes "exactly one PR" idempotent across scheduled runs.
	Branch string
	// Base is the fetched remote head to cut a fresh run from, spelled in FULL
	// (refs/remotes/origin/main) so a stray local branch named origin/main cannot shadow it.
	Base string
	// Now stamps the commit message (chore(board): reconcile <date>).
	Now time.Time
	// DryRun runs fetch + reconcile against origin/main and reports the rows it WOULD flip,
	// then discards the worktree. It commits nothing, pushes nothing, and opens no PR.
	DryRun bool
	// Issue is the tracking issue the fresh-create PR body carries as its `Issue: #<N>`
	// trailer. Required for a real (non-dry-run) create; unused on the coalesce/update path
	// (that reuses the existing PR's body) and on the dry-run path.
	Issue int
	// Repo is the owner/name statusgen reconcile reads PRs from to WITNESS a flip. A merged
	// PR witness is the only thing that lets a cell move to `implemented`, and that witness
	// is an online PR read — offline (no --repo) reconcile flips nothing by construction.
	// Empty means "derive it from the target checkout's origin remote".
	Repo string
	// TokenFile, when set, is forwarded to statusgen as --token-file (the API token for the
	// PR read). Empty defers to statusgen's own GITHUB_TOKEN fallback, which the subprocess
	// inherits from this process's environment.
	TokenFile string

	// exec is the process seam; nil means RealExec.
	exec Exec
}

// Result is what a run produced, for the caller to print.
type Result struct {
	// Steps is the exact command sequence the run executed — the audit surface, one line
	// per step.
	Steps []string
	// Flipped names the Status cells statusgen wrote (from its --json `applied`).
	Flipped []FlipRow
	// Changed is the README files deskreconcile committed (dry-run: would commit).
	Changed []string
	// NoOp is true when the reconcile changed nothing — the run committed and pushed
	// nothing, opened no PR, and left no worktree behind.
	NoOp bool
	// DryRun echoes Options.DryRun so the caller can phrase the report as a preview.
	DryRun bool
}

const (
	// defaultBranch is the fixed carry branch (#1175).
	defaultBranch = "board/reconcile"
	// prTitle is the fixed PR title (#1175); its stability is what makes the PR findable.
	prTitle = "chore(board): reconcile"
	// streamReadmePrefix / streamReadmeSuffix bound the ONLY paths a reconcile is allowed to
	// have changed — a stream README. A changed path outside this shape is a reconcile that
	// touched something it must not, and the run refuses rather than commit it.
	streamReadmePrefix = "docs/streams/"
	streamReadmeSuffix = "/README.md"
)

// Run executes one reconcile cycle and returns what it did. The caller prints the Result;
// Run itself writes nothing to stdout.
func Run(o Options) (Result, error) {
	var res Result
	res.DryRun = o.DryRun
	if err := assertIsolatedWorktree(o.Root, o.Worktree); err != nil {
		return res, err
	}
	branch := o.Branch
	if branch == "" {
		branch = defaultBranch
	}
	base := o.Base
	if base == "" {
		base = "refs/remotes/origin/main"
	}
	if !o.DryRun && o.Issue <= 0 {
		// The real create path needs a work-item trailer (deskpr create refuses without
		// one). Refuse HERE — before a fetch, a worktree, or a token — with the remedy,
		// rather than letting deskpr refuse deep in the sequence after side effects.
		return res, deskkit.Refused("refused: --issue is required for a real reconcile run — it becomes the PR body's `Issue: #<N>` trailer; pass --dry-run to preview without it")
	}

	run := o.exec
	if run == nil {
		run = RealExec
	}
	step := func(dir, name string, args ...string) (string, error) {
		res.Steps = append(res.Steps, renderStep(dir, name, args))
		return run(dir, name, args...)
	}

	// Resolve the repo statusgen witnesses against — an explicit --repo, else the target
	// checkout's own origin. Refuse rather than run witness-blind: a reconcile with no repo
	// to read PRs from can only ever flip nothing, so a silent empty --repo would look like
	// a clean "nothing to reconcile" when it never looked.
	repo := strings.TrimSpace(o.Repo)
	if repo == "" {
		originURL, oerr := run(o.Root, "git", "config", "--get", "remote.origin.url")
		if oerr != nil {
			return res, deskkit.Unverifiable("cannot read the target checkout's origin remote to resolve --repo", oerr)
		}
		repo = ownerNameFromRemote(originURL)
		if repo == "" {
			return res, deskkit.Refused("refused: could not resolve owner/name from origin remote " + strings.TrimSpace(originURL) + " — pass --repo owner/name")
		}
	}

	// 1. sync fresh.
	if _, err := step(o.Root, "git", "fetch", "origin"); err != nil {
		return res, err
	}

	// On a real run, coalesce onto the existing carry branch when one is already open, so the
	// push is a fast-forward (force-push is denied) and the one PR accumulates. On a dry-run
	// there is nothing to push, so it always cuts a fresh preview from origin/main.
	coalesce := false
	if !o.DryRun {
		lsOut, err := step(o.Root, "git", "ls-remote", "--heads", "origin", branch)
		if err != nil {
			return res, err
		}
		coalesce = strings.TrimSpace(lsOut) != ""
	}

	// 2. isolate. --detach + checkout -B keeps the FIXED local branch resettable across runs
	//    (a plain `worktree add -b` fails the second time the branch already exists).
	startPoint := base
	if coalesce {
		startPoint = "refs/remotes/origin/" + branch
	}
	if _, err := step(o.Root, "git", "worktree", "add", "--detach", o.Worktree, startPoint); err != nil {
		return res, err
	}
	// From here the worktree exists; remove it on every exit path.
	removeWorktree := func() {
		_, _ = step(o.Root, "git", "worktree", "remove", "--force", o.Worktree)
	}
	if _, err := step(o.Worktree, "git", "checkout", "-B", branch); err != nil {
		removeWorktree()
		return res, err
	}
	if coalesce {
		// Never reconcile a base behind the default branch; merge the fetched head in first.
		if _, err := step(o.Worktree, "git", "merge", "--no-edit", base); err != nil {
			removeWorktree()
			return res, err
		}
	}

	// 3. run the ONE writer. --json so the flips can be named; the worktree diff is the
	//    authoritative "did anything change". --repo lets it witness merged PRs.
	sgArgs := []string{"reconcile", "--backfill", "--apply", "--root", ".", "--json", "--repo", repo}
	if o.TokenFile != "" {
		sgArgs = append(sgArgs, "--token-file", o.TokenFile)
	}
	sgOut, err := step(o.Worktree, "statusgen", sgArgs...)
	if err != nil {
		removeWorktree()
		return res, err
	}
	flipped, perr := parseApplied(sgOut)
	if perr != nil {
		removeWorktree()
		return res, deskkit.Unverifiable("cannot parse statusgen reconcile --json output", perr)
	}
	res.Flipped = flipped

	// 4. what changed on disk. This — not the JSON — is what gets committed, and it is
	//    constrained to stream READMEs: a reconcile that dirtied anything else is refused.
	changed, cerr := changedReadmes(o.Worktree, step)
	if cerr != nil {
		removeWorktree()
		return res, cerr
	}
	res.Changed = changed

	if len(changed) == 0 {
		// Nothing to reconcile: no commit, no push, no PR.
		res.NoOp = true
		removeWorktree()
		return res, nil
	}

	if o.DryRun {
		// Preview only: the writes live in the throwaway worktree and are discarded with it.
		removeWorktree()
		return res, nil
	}

	// 5. commit ONLY the changed READMEs, as ONE commit with a stable message.
	addArgs := append([]string{"add", "--"}, changed...)
	if _, err := step(o.Worktree, "git", addArgs...); err != nil {
		removeWorktree()
		return res, err
	}
	msg := prTitle + " " + o.Now.Format("2006-01-02")
	if _, err := step(o.Worktree, "git", "commit", "-m", msg); err != nil {
		removeWorktree()
		return res, err
	}

	// 6. carry it as exactly ONE draft PR on the fixed branch. On an already-open PR the
	//    follow-up push verb keeps the same PR; otherwise the create verb opens one (and is
	//    itself idempotent — a noop if a PR already raced onto the branch).
	if coalesce {
		if _, err := step(o.Worktree, "deskpr", "update"); err != nil {
			removeWorktree()
			return res, err
		}
	} else {
		bodyPath, berr := writePRBody(o.Worktree, o.Issue, flipped)
		if berr != nil {
			removeWorktree()
			return res, deskkit.Unverifiable("cannot write the PR body file", berr)
		}
		createOut, err := step(o.Worktree, "deskpr", "create", "--title", prTitle, "--body-file", bodyPath)
		if err != nil {
			removeWorktree()
			return res, err
		}
		// deskpr create is idempotent: if a PR already existed on the branch it NOOPS
		// WITHOUT pushing this commit (it checks before pushing). Push the follow-up
		// through the update verb so the just-made commit reaches the open PR.
		if strings.Contains(createOut, "noop: open PR already exists") {
			if _, err := step(o.Worktree, "deskpr", "update"); err != nil {
				removeWorktree()
				return res, err
			}
		}
	}

	removeWorktree()
	return res, nil
}

// parseApplied reads statusgen reconcile --json and returns its `applied` rows. An empty
// or applied-less document is a legitimate zero, never an error — that is the no-flip run.
func parseApplied(jsonOut string) ([]FlipRow, error) {
	trimmed := strings.TrimSpace(jsonOut)
	if trimmed == "" {
		return nil, nil
	}
	var out reconcileOutput
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, err
	}
	return out.Applied, nil
}

// changedReadmes returns the stream-README paths the reconcile modified in the worktree,
// sorted and deduplicated. It REFUSES if the reconcile changed any path that is not a
// stream README: `statusgen reconcile --apply` only ever writes a stream README's Status
// cell, so anything else in the diff is not a reconcile result and must not be committed
// under the reconcile message.
func changedReadmes(worktree string, step func(dir, name string, args ...string) (string, error)) ([]string, error) {
	out, err := step(worktree, "git", "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var readmes, foreign []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Porcelain v1: "XY <path>" (XY is exactly two status columns + a space).
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		// A rename is "old -> new"; take the destination path.
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		path = strings.Trim(path, `"`)
		if isStreamReadme(path) {
			readmes = append(readmes, path)
		} else {
			foreign = append(foreign, path)
		}
	}
	if len(foreign) > 0 {
		sort.Strings(foreign)
		return nil, deskkit.Refused("refused: reconcile changed non-README paths — a reconcile writes ONLY stream README Status cells, so these are not reconcile output and will not be committed: " + strings.Join(foreign, ", "))
	}
	sort.Strings(readmes)
	return dedupe(readmes), nil
}

// isStreamReadme reports whether p is a docs/streams/<stream>/README.md path.
func isStreamReadme(p string) bool {
	if !strings.HasPrefix(p, streamReadmePrefix) || !strings.HasSuffix(p, streamReadmeSuffix) {
		return false
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(p, streamReadmePrefix), streamReadmeSuffix)
	// exactly one path component between the prefix and the suffix — a single stream dir.
	return mid != "" && !strings.Contains(mid, "/")
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := in[:1]
	for _, s := range in[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}

// writePRBody writes the fresh-create PR body to a file inside the worktree and returns its
// path. The body is SELF-CONTAINED (public-repo rule): it names no private repo, path, or
// identifier, and carries exactly one `Issue: #<N>` trailer.
func writePRBody(worktree string, issue int, flipped []FlipRow) (string, error) {
	var b strings.Builder
	b.WriteString(prTitle + "\n\n")
	b.WriteString("Automated board reconcile. Flips merged briefs' Status cells " +
		"`todo`/`in-progress` -> `implemented` from a real merged-PR witness, via " +
		"`statusgen reconcile --backfill --apply`. Only stream README Status cells change; " +
		"Verified/Reviewed and every other cell are preserved byte-for-byte.\n\n")
	if len(flipped) == 0 {
		b.WriteString("No rows flipped in this run.\n\n")
	} else {
		b.WriteString("Flipped:\n")
		for _, f := range flipped {
			b.WriteString(fmt.Sprintf("- `%s`: %s -> %s (witness %s)\n", f.ID, f.From, f.To, f.Witness))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Issue: #%d\n", issue)
	path := filepath.Join(worktree, ".reconcile-pr-body")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// assertIsolatedWorktree enforces the isolation floor before anything runs: the worktree
// must be a non-empty ABSOLUTE path, and it must not be nested inside the checkout being
// reconciled (that is a shared-checkout write under another name).
func assertIsolatedWorktree(root, worktree string) error {
	if strings.TrimSpace(worktree) == "" {
		return deskkit.Refused("refused: no worktree path — deskreconcile runs in an isolated linked worktree, never in place")
	}
	if !filepath.IsAbs(worktree) {
		return deskkit.Refused("refused: worktree path " + worktree + " is not absolute — a relative worktree resolves against whatever directory the process happens to be in")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return deskkit.Unverifiable("cannot resolve the target checkout path", err)
	}
	absWT, err := filepath.Abs(worktree)
	if err != nil {
		return deskkit.Unverifiable("cannot resolve the worktree path", err)
	}
	if absWT == absRoot || strings.HasPrefix(absWT, absRoot+string(filepath.Separator)) {
		return deskkit.Refused("refused: the reconcile worktree " + absWT + " is inside the target checkout " + absRoot + " — that is a shared-checkout write under another name")
	}
	return nil
}

// renderStep formats one executed command for the audit surface — one line per step.
func renderStep(dir, name string, args []string) string {
	return "(" + dir + ") " + name + " " + strings.Join(args, " ")
}

// ownerNameFromRemote extracts owner/name from a git remote URL — ssh
// (git@host:owner/name.git), ssh:// (ssh://git@host/owner/name.git) or https
// (https://host/owner/name.git). Returns "" when it cannot, so the caller refuses rather
// than run against a repo it guessed wrong.
func ownerNameFromRemote(remote string) string {
	s := strings.TrimSpace(remote)
	s = strings.TrimSuffix(s, ".git")
	// URL forms (https://host/owner/name, ssh://git@host/owner/name): strip scheme + host,
	// keep the path tail.
	if i := strings.Index(s, "://"); i >= 0 {
		rest := s[i+3:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return twoPathTail(rest[slash+1:])
		}
		return ""
	}
	// scp-like form: git@host:owner/name — take everything after the colon.
	if colon := strings.Index(s, ":"); colon >= 0 {
		return twoPathTail(s[colon+1:])
	}
	return twoPathTail(s)
}

// twoPathTail returns the last two "/"-separated components of p as "owner/name", or "".
func twoPathTail(p string) string {
	p = strings.Trim(p, "/")
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return ""
	}
	owner, name := parts[len(parts)-2], parts[len(parts)-1]
	if owner == "" || name == "" {
		return ""
	}
	return owner + "/" + name
}
