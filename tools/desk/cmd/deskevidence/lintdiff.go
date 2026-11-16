package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lintdiff.go — the statusgen PROBLEM-diff guard behind deskevidence's landing gate
// (deskevidence: refuse a landing that adds a statusgen PROBLEM or lands outside
// docs/streams/). Same-day main-red episodes traced to deskevidence landing an Evidence
// row that reddened `statusgen --lint` — a bare sibling-path row, missing the `../<repo>/`
// prefix the lint requires — with no check catching it before the commit landed. The fix
// asks statusgen itself, twice: once against the landing worktree as it stands, once with
// the pending write staged, and refuses only what the second run adds.
//
// Two layers, two seams, matching this file's existing style (mintTokenFn, forgeForFn,
// publicRepoGateFn in deskevidence.go):
//
//   - lintDiffFn (production: lintDiffAt) is what cmdEvidence calls. It owns staging the
//     pending write into the worktree, running the before/after lint, diffing the PROBLEM
//     sets, and reverting the stage — the whole check as cmdEvidence sees it.
//   - statusgenLintFn (production: statusgenLintAt) is the one thing lintDiffAt calls
//     twice: a single `statusgen --root <dir> --lint` shell, returning the PROBLEM: lines
//     printed. Exposed as its own seam so lintDiffAt's staging/diff/revert mechanics can be
//     tested against a real temp directory without needing a real statusgen on PATH.
//
// Mirrors deskpreflight's own statusgen --lint shell (cmd/deskpreflight/main.go,
// runStatusgenLint): same neutral cwd (os.TempDir()), same "not on PATH is
// could-not-check, never a silent pass" rule (C-4 of the three-state instrument rule), same
// "a nonzero exit is a normal 'problems found' run, not a runner failure" reading — reused
// rather than reinvented, just applied twice and diffed instead of once and reported.

// lintDiffFn is the seam cmdEvidence calls for the whole statusgen PROBLEM-diff check.
// Tests stub it (see setupFake in deskevidence_test.go) so calling cmdEvidence never shells
// a real statusgen or touches a real tree merely by landing an Evidence row; a handful of
// dedicated tests exercise lintDiffAt directly, against a real t.TempDir() root, with
// statusgenLintFn stubbed one level down instead.
var lintDiffFn = lintDiffAt

// statusgenLintFn is the seam for ONE statusgen --lint shell; lintDiffAt calls it twice
// (before and after staging the pending write). Production: statusgenLintAt.
var statusgenLintFn = statusgenLintAt

// lintDiffAt runs statusgenLintFn(root) to get the PROBLEM set the landing worktree
// reports BEFORE this write (the pre-landing base), stages newContent at
// <root>/<targetRepoPath> (creating parent directories as needed), runs statusgenLintFn(root)
// again, and returns exactly the PROBLEM lines present after the stage but absent before it
// — what THIS landing would introduce. A PROBLEM already standing in the tree before the
// write is real, but it is not this landing's to block: that is the whole point of diffing
// against the pre-landing base rather than refusing on any red statusgen reports.
//
// Because the underlying statusgen PROBLEM: lines are returned VERBATIM (never
// re-summarised), a sibling-path PROBLEM introduced by this landing carries statusgen's own
// "prefix it ../<repo>/%s" hint unchanged — deskevidence does not need to know that hint's
// wording, only pass the line through.
//
// The stage is ALWAYS reverted before return — a deferred restore, on every path (a clean
// diff, an introduced-PROBLEM refusal, or a could-not-check from the second lint run) —
// because deskevidence's real write rides the GitHub Contents API, never this local
// mutation: the landing worktree must come back exactly as this function found it. Same
// discipline --root's own #1709 fix established for deskevidence: a tool that leaves a
// worktree silently different from what a human or another session expects has already
// caused one class of main-red.
func lintDiffAt(root, targetRepoPath string, newContent []byte, options childOptions) (introduced []string, err error) {
	before, err := statusgenLintFn(root, options)
	if err != nil {
		return nil, err
	}

	abs := filepath.Join(root, filepath.FromSlash(targetRepoPath))
	origContent, rerr := os.ReadFile(abs)
	origExists := rerr == nil
	if rerr != nil && !os.IsNotExist(rerr) {
		return nil, deskkit.Unverifiable("cannot read local "+abs+" to stage the statusgen PROBLEM-diff check", rerr)
	}

	if merr := os.MkdirAll(filepath.Dir(abs), 0o755); merr != nil {
		return nil, deskkit.Unverifiable("cannot create local directories under "+root+" to stage the statusgen PROBLEM-diff check", merr)
	}
	if werr := os.WriteFile(abs, newContent, 0o644); werr != nil {
		return nil, deskkit.Unverifiable("cannot stage the statusgen PROBLEM-diff check at "+abs, werr)
	}
	defer func() {
		if origExists {
			_ = os.WriteFile(abs, origContent, 0o644)
		} else {
			_ = os.Remove(abs)
		}
	}()

	after, aerr := statusgenLintFn(root, options)
	if aerr != nil {
		return nil, aerr
	}

	introduced, excused := attributeIntroduced(before, after, landingScopeOf(targetRepoPath))
	printExcused(lintDiffStderr, targetRepoPath, excused)
	if len(introduced) > 0 {
		printBeforeSet(targetRepoPath, before, introduced)
	}
	return introduced, nil
}

// printBeforeSet writes the before-landing PROBLEM set to lintDiffStderr beside the lines
// counted as introduced, so a refusal shows the whole delta: an introduced line whose
// brief also has a differently worded line before is visible as a change, not only as an
// addition (#6581).
func printBeforeSet(targetRepoPath string, before, introduced []string) {
	var b strings.Builder
	fmt.Fprintf(&b, "deskevidence: lint delta for %s: %d PROBLEM line(s) before this landing, %d introduced\n",
		targetRepoPath, len(before), len(introduced))
	if len(before) == 0 {
		b.WriteString("  before: (none)\n")
	}
	for _, p := range before {
		b.WriteString("  before: " + p + "\n")
	}
	for _, p := range introduced {
		b.WriteString("  introduced: " + p + "\n")
	}
	fmt.Fprint(lintDiffStderr, b.String())
}

// lintDiffStderr receives the NOTICE lines for after-only could-not-check PROBLEMs that
// lintDiffAt does not count (#6581). A seam so tests can read them.
var lintDiffStderr io.Writer = os.Stderr

// A could-not-check PROBLEM says statusgen could not read something (an origin's authority,
// a forge it was not given) — not that the tree is wrong. Its wording follows the reader's
// environment, so the before and after runs need not print the same line for the same
// unchanged brief: #6581 saw one brief's could-not-check in the after run only, and three
// briefs' "--acceptance-forge is required" in the after run of an outcome record for a
// fourth. So every new-PROBLEM gate (the Evidence and outcome-record landings through
// lintDiffAt, and the flip) attributes after-only lines by brief key, with the parser the
// closure gate's splitClosureProblems uses (outcome.go): an after-only could-not-check on a
// brief OUTSIDE what the write touches is reported, not refused. Everything else still
// refuses — any line in scope (the write's own brief, or every brief of a stream whose
// README it writes), any finding that is not a could-not-check wherever it lands, every
// line when the write names no brief or stream, and every line while either run reports a
// stream whose frontmatter name differs from its directory (statusgen keys lines by the
// frontmatter name, the scope by the directory, so they could not be matched).

// landingScope is what a write touches, in statusgen's keys: whole briefs, and whole
// streams (a README is an input of every brief on its board).
type landingScope struct {
	briefs, streams map[string]bool
}

func (s landingScope) named() bool { return len(s.briefs)+len(s.streams) > 0 }

func (s landingScope) owns(key string) bool {
	stream, _, _ := strings.Cut(key, "/")
	return s.briefs[key] || s.streams[stream]
}

func (s *landingScope) addBrief(key string) {
	if k := briefKey(key); k != "" {
		if s.briefs == nil {
			s.briefs = map[string]bool{}
		}
		s.briefs[k] = true
	}
}

func (s *landingScope) addStream(stream string) {
	if k := briefKey(stream + "/0"); k != "" {
		if s.streams == nil {
			s.streams = map[string]bool{}
		}
		s.streams[stream] = true
	}
}

var (
	briefFileName   = regexp.MustCompile(`^brief-([0-9]+[a-z]?)-[^/]*\.md$`)
	outcomeFileName = regexp.MustCompile(`^([0-9]+[a-z]?)-[^/]*\.json$`)
)

// landingScopeOf names what a landing at targetRepoPath touches: the brief of a brief file
// docs/streams/<stream>/brief-<num>-*.md or of a per-file outcome record
// <deskkit.OutcomeRecordsDir>/<stream>/<num>-*.json, or the whole stream of a board
// docs/streams/<stream>/README.md. Any other path names nothing, and nothing is excused.
func landingScopeOf(targetRepoPath string) landingScope {
	var s landingScope
	cleaned := path.Clean(targetRepoPath)
	dir, file := path.Dir(cleaned), path.Base(cleaned)
	stream := path.Base(dir)
	switch {
	case path.Dir(dir) == deskkit.OutcomeRecordsDir:
		if m := outcomeFileName.FindStringSubmatch(file); m != nil {
			s.addBrief(stream + "/" + m[1])
		}
	case path.Dir(dir) != "docs/streams" || stream == path.Base(deskkit.OutcomeRecordsDir):
	case file == "README.md":
		s.addStream(stream)
	default:
		if m := briefFileName.FindStringSubmatch(file); m != nil {
			s.addBrief(stream + "/" + m[1])
		}
	}
	return s
}

// couldNotCheckRe anchors the could-not-check marker: after the brief key, a chain of plain
// lowercase label segments ("protected table: acceptance transition: ") and then the
// marker. A finding that merely quotes ": could-not-check " out of brief text carries
// quotes, digits or "#" ahead of it, and does not match.
var couldNotCheckRe = regexp.MustCompile(`^(?:[a-z][a-z -]*: )*could-not-check `)

// streamDirMismatch reports whether a lint run names a stream whose frontmatter name
// differs from its directory (statusgen/checks.go).
var streamDirMismatch = regexp.MustCompile(`: frontmatter stream "[^"]*" != directory name "`)

// attributeIntroduced is after minus before, by exact line, except that an after-only
// could-not-check on a brief outside scope is returned as excused rather than introduced.
func attributeIntroduced(before, after []string, scope landingScope) (introduced, excused []string) {
	had := make(map[string]bool, len(before))
	attribute := scope.named()
	for _, p := range before {
		had[p] = true
		attribute = attribute && !streamDirMismatch.MatchString(p)
	}
	for _, p := range after {
		attribute = attribute && !streamDirMismatch.MatchString(p)
	}
	for _, p := range after {
		if had[p] {
			continue
		}
		had[p] = true
		if attribute && foreignCouldNotCheck(p, scope) {
			excused = append(excused, p)
			continue
		}
		introduced = append(introduced, p)
	}
	return introduced, excused
}

func foreignCouldNotCheck(problem string, scope landingScope) bool {
	key, rest := problemBriefKey(problem)
	return key != "" && !scope.owns(key) && couldNotCheckRe.MatchString(rest)
}

// printExcused writes one NOTICE per excused line: reported, never silently dropped.
func printExcused(w io.Writer, target string, excused []string) {
	for _, p := range excused {
		fmt.Fprintln(w, "deskevidence: NOTICE: not counted as introduced by "+target+
			" (could-not-check on a brief this write does not touch): "+p)
	}
}

// statusgenLintAt shells `statusgen --root <root> --lint` from a neutral working directory
// (os.TempDir(), exactly as deskpreflight's runStatusgenLint does — cmd/deskpreflight/main.go
// — so statusgen scans exactly root and never the calling process's own cwd) and returns the
// PROBLEM: lines it printed. statusgen not being on PATH is Unverifiable (could-not-check,
// never a silent pass — C-4 of the three-state instrument rule, docs/three-state-instrument-rule.md).
//
// A nonzero exit is read against the PROBLEM: lines it carries:
//
//   - exit 0                    → the tree linted clean; return no problems.
//   - exit nonzero, ≥1 PROBLEM: → the normal "problems found" outcome, not a runner
//     failure; return those PROBLEM lines. Mirrors statusgen's own self-shell in
//     statusgen/difflint.go (productionDiffLintRunner), which draws the same
//     *exec.ExitError-is-not-an-error line.
//   - exit nonzero, 0 PROBLEM:  → statusgen could not EVALUATE this root at all. It printed
//     a structural diagnostic (`statusgen: reading …/docs/streams: no such file or
//     directory`, `statusgen: stream directory x has no README.md`, …) and `LINT: FAIL`,
//     but no board PROBLEM — the lint never ran over an evaluable board. This is
//     could-not-check (Unverifiable), NOT "0 problems / clean": folding it into a clean
//     report is the exact silent-pass #1078 caught, where a landing run against a root that
//     is not a full, current checkout of the target repo (the scratchpad / bare-cwd
//     invocation verify-desk's own skill documents) would sail EVERY landing through,
//     including the sibling-path bug this guard exists to catch. Reporting it Unverifiable
//     makes cmdEvidence refuse the landing (exit 6) rather than trust an un-evaluable lint.
func statusgenLintAt(root string, options childOptions) ([]string, error) {
	if _, err := exec.LookPath("statusgen"); err != nil {
		return nil, deskkit.Unverifiable("statusgen is not on PATH — cannot check this landing for introduced PROBLEMs", err)
	}
	cmd := statusgenAudit(context.Background(), auditLint, root, options)
	cmd.Dir = os.TempDir()
	out, cerr := cmd.CombinedOutput()
	exitedNonZero := false
	if cerr != nil {
		if _, ok := cerr.(*exec.ExitError); !ok {
			return nil, deskkit.Unverifiable("could not run statusgen --lint at "+root, cerr)
		}
		exitedNonZero = true
	}
	var problems []string
	var diag string // first structural/summary line, for the could-not-evaluate message
	for _, ln := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "PROBLEM:") {
			problems = append(problems, t)
			continue
		}
		if diag == "" && (strings.HasPrefix(t, "statusgen:") || strings.HasPrefix(t, "LINT: FAIL")) {
			diag = t
		}
	}
	if exitedNonZero && len(problems) == 0 {
		msg := "statusgen --lint could not evaluate " + root +
			" for the landing PROBLEM-diff — it exited nonzero with no PROBLEM lines, " +
			"meaning the root has no evaluable docs/streams tree (an incomplete or absent " +
			"checkout of the target repo), so a clean report cannot be trusted"
		if diag != "" {
			msg += ": " + diag
		}
		return nil, deskkit.Unverifiable(msg, nil)
	}
	return problems, nil
}
