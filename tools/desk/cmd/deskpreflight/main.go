// Command deskpreflight is the ONE bundled worker-side pre-PR gate.
//
// It runs, in one command, the hygiene checks a worker is otherwise asked to
// remember one at a time before opening a draft PR — and it REFUSES (exits
// nonzero) unless every one of them is clean. The point is packaging, not any
// single check: moving these findings from review time to author time.
//
// Two properties are the whole design and each is pinned by a test:
//
//  1. A CHECK THAT COULD NOT RUN IS A FAILURE. Every check answers one of three
//     states — checked-clean, checked-failed, could-not-check — and only
//     checked-clean is a pass. A missing underlying tool, an unreadable tree, a
//     probe that errors: all are could-not-check, and could-not-check exits
//     nonzero. "Unchecked counts as failing" — a check can never silently
//     no-op its way to green.
//
//  2. READ-ONLY BY DESIGN. deskpreflight computes and reports. It never
//     formats, fixes, stages, writes, or mutates anything — not the tree it
//     scans, not the index, not a config. The worker reads the findings and
//     acts. Every check below shells an existing tool or a stock command in a
//     read-only mode, or asks the filesystem for metadata (os.Lstat); none of
//     them has a write path.
//
// It is NOT a merge gate and NOT CI. It is the local step a worker runs before
// `deskpr create`; the repo's CI gates are unchanged and remain the enforcement
// layer. deskpreflight only moves the cheapest findings earlier.
//
// The bundled checks (v1):
//
//	conflict-markers   grep for VCS conflict markers in the git work tree under --root
//	junk-oversize      junk-named or oversize files in the git work tree under --root
//	go-fmt-vet         gofmt -l + go vet, scoped to the touched Go files under --root
//	statusgen-lint     statusgen --root <abs> --lint, from a neutral cwd
//	leak-sweep         the installed leak-sweep, WHERE PRESENT (see runLeakSweep)
//
// SCOPE. The tree the first two checks look at is the WORKING TREE AS GIT SEES
// IT — `git ls-files --cached --others --exclude-standard` — never a filesystem
// walk (see gitWorkTreeFiles). A gate for "what is about to ride into a PR"
// must see exactly what `git add` could stage: walking the filesystem instead
// made both checks report git's own 43 MB packfiles under `.git/`, dangling
// blobs under `.git/lost-found/`, ignored build output, and this tool's own
// committed conflict-marker fixture, so `deskpreflight` could not reach exit 0
// in any real repository. Where git cannot answer, both checks are
// could-not-check — never an empty pass.
//
// USAGE:
//
//	deskpreflight [--root DIR]
//	deskpreflight --version
//
// --root defaults to ".". Run it from a NEUTRAL cwd so statusgen scans the
// absolute --root and nothing else.
//
// EXIT CODES (deskkit contract, internal/deskkit/exitcodes.go):
//
//	0  every check checked-clean            → PREFLIGHT: PASS
//	5  a check checked-failed (no unchecked) → PREFLIGHT: FAIL n
//	6  any check could-not-check            → PREFLIGHT: COULD-NOT-CHECK n
//
// could-not-check takes precedence over checked-failed in the headline: from the
// gate's seat "we could not tell" is at least as blocking as "we found a
// defect", and both are nonzero.
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskpreflight — the one bundled worker-side pre-PR gate.

USAGE:
  deskpreflight [--root DIR]
  deskpreflight --version

Runs every pre-PR hygiene check in one command and REFUSES unless all pass.
A check that could not run (missing tool, unreadable tree) is a FAILURE, never a
silent pass. Read-only: it reports, it never formats/fixes/stages/mutates.

Checks: conflict-markers, junk-oversize, go-fmt-vet, statusgen-lint,
        leak-sweep (where the tool is installed).

Exit codes:
  0  PREFLIGHT: PASS             — every check checked-clean
  5  PREFLIGHT: FAIL n           — n checks checked-failed (nothing unchecked)
  6  PREFLIGHT: COULD-NOT-CHECK n — n checks could not run (fail-closed)`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, realTools()))
}

// check is one bundled check's three-state result. detail is a human line; for a
// non-clean state it names WHAT (which file, which tool) so the worker can act
// without opening the tool by hand.
type check struct {
	name   string
	state  deskkit.CheckState
	detail string
}

// tools is the injectable edge of every check: process launching and the one
// filesystem read, isolated behind three functions so the test suite is
// hermetic (no real grep/git/gofmt/go/statusgen/leak-sweep, and no real tree).
type tools struct {
	// lookPath reports whether a tool is on PATH (exec.LookPath).
	lookPath func(name string) (string, error)
	// output runs name+args with the working directory dir and returns combined
	// stdout+stderr, the process exit code, and ran=false when the process could
	// not be STARTED at all (exec error: not on PATH mid-run, permission, etc.).
	// A ran=true with a nonzero code is a process that ran and failed — a
	// different thing from could-not-run, and the checks treat them differently.
	output func(dir, name string, args ...string) (out []byte, code int, ran bool)
	// stat returns metadata for one path. It is os.Lstat: a pure read, and one
	// that describes a symlink rather than following it, so no check ever reads
	// through a link out of the tree it was pointed at. It is the size and
	// file-type oracle for junk-oversize, and the existence filter that keeps a
	// tracked-but-deleted path from turning an ordinary mid-refactor working
	// tree into a could-not-check.
	stat func(path string) (os.FileInfo, error)
}

func realTools() tools {
	return tools{lookPath: exec.LookPath, output: realOutput, stat: os.Lstat}
}

// realOutput runs a command read-only with a bounded timeout. It distinguishes
// "process ran and exited nonzero" (ran=true) from "could not start the process"
// (ran=false) — the second is the could-not-check signal.
func realOutput(dir, name string, args ...string) (out []byte, code int, ran bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err == nil {
		return buf.Bytes(), 0, true
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return buf.Bytes(), ee.ExitCode(), true
	}
	// Could not start the process at all (not on PATH, exec failure, timeout).
	return buf.Bytes(), -1, false
}

func run(args []string, stdout, stderr io.Writer, tl tools) int {
	fs := flag.NewFlagSet("deskpreflight", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		root    = fs.String("root", ".", "root of the tree to check before opening a PR")
		version = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}

	if *version {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskpreflight sourceSHA=%s builtAt=%s releaseTag=%s\n",
			sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		// The root path itself is unresolvable: no check can look. Fail closed.
		fmt.Fprintf(stdout, "  root: %s — cannot resolve --root %q: %v\n",
			deskkit.CouldNotCheck, *root, err)
		fmt.Fprintln(stdout, "PREFLIGHT: COULD-NOT-CHECK 1")
		return deskkit.ExitUnverifiable
	}

	checks := runChecks(absRoot, tl)
	return report(checks, stdout)
}

// runChecks runs every bundled check against absRoot and returns their results
// in a fixed order. leak-sweep is appended ONLY where its tool is installed
// (see runLeakSweep); every other check is always present.
func runChecks(absRoot string, tl tools) []check {
	out := []check{
		runConflictMarkers(absRoot, tl),
		runJunkOversize(absRoot, tl),
		runGoFmtVet(absRoot, tl),
		runStatusgenLint(absRoot, tl),
	}
	if ls, present := runLeakSweep(absRoot, tl); present {
		out = append(out, ls)
	}
	return out
}

// report prints one line per check plus the PREFLIGHT summary, and returns the
// process exit code. could-not-check outranks checked-failed in the headline.
func report(checks []check, stdout io.Writer) int {
	var failed, couldNot int
	for _, c := range checks {
		fmt.Fprintf(stdout, "  %s: %s — %s\n", c.name, c.state, c.detail)
		switch c.state {
		case deskkit.CheckedFailed:
			failed++
		case deskkit.CouldNotCheck:
			couldNot++
		}
	}
	switch {
	case len(checks) == 0:
		// A preflight that ran no checks proved nothing: never a pass.
		fmt.Fprintln(stdout, "PREFLIGHT: COULD-NOT-CHECK 0")
		return deskkit.ExitUnverifiable
	case couldNot > 0:
		fmt.Fprintf(stdout, "PREFLIGHT: COULD-NOT-CHECK %d\n", couldNot)
		return deskkit.ExitUnverifiable
	case failed > 0:
		fmt.Fprintf(stdout, "PREFLIGHT: FAIL %d\n", failed)
		return deskkit.ExitRefused
	default:
		fmt.Fprintln(stdout, "PREFLIGHT: PASS")
		return deskkit.ExitOK
	}
}

// --- check helpers ----------------------------------------------------------

const (
	nameConflictMarkers = "conflict-markers"
	nameJunkOversize    = "junk-oversize"
	nameGoFmtVet        = "go-fmt-vet"
	nameStatusgenLint   = "statusgen-lint"
	nameLeakSweep       = "leak-sweep"
)

func clean(name, detail string) check {
	return check{name: name, state: deskkit.CheckedClean, detail: detail}
}
func failedCheck(name, detail string) check {
	return check{name: name, state: deskkit.CheckedFailed, detail: detail}
}
func couldNotCheck(name, detail string) check {
	return check{name: name, state: deskkit.CouldNotCheck, detail: detail}
}

// --- the scan input: the working tree as git sees it ------------------------

// gitWorkTreeFiles enumerates the WORKING TREE AS GIT SEES IT under absRoot:
// tracked files, plus untracked files that .gitignore does not exclude, and
// nothing else. That set — what `git add` could stage right now — is the only
// tree a pre-PR gate has any business looking at.
//
// `git ls-files --cached --others --exclude-standard -z`, run with the working
// directory at absRoot, is the whole definition, and it gets three exclusions
// right at once. Each was a live false positive while the checks walked the
// filesystem instead:
//
//   - `.git/` is excluded BY CONSTRUCTION — git never lists its own object
//     store — so a 43 MB packfile is no longer reported as an oversize file the
//     worker is about to push, and a dangling blob under `.git/lost-found/` is
//     no longer reported as a conflict marker.
//   - `.gitignore` is HONOURED, so build output no `git add` would ever pick up
//     (a sibling agent worktree's `dist/`, a coverage dump) stops counting.
//   - the listing is bounded to absRoot, so nothing outside the root leaks in.
//
// Paths come back RELATIVE to absRoot, sorted — which is also how the checks
// report them, so a gate's output reads like the paths the worker would type.
//
// If git cannot answer — not on PATH, absRoot is not a work tree, ls-files
// errors — the file set is UNKNOWABLE and every check built on it is
// could-not-check. An enumeration that returned nothing because it could not
// look has cleared nothing.
func gitWorkTreeFiles(absRoot string, tl tools) (paths []string, why string, ok bool) {
	if _, err := tl.lookPath("git"); err != nil {
		return nil, "git is not on PATH, so the working-tree file set is unknowable: " + err.Error(), false
	}
	out, code, ran := tl.output(absRoot, "git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if !ran {
		return nil, "could not run git ls-files under " + absRoot, false
	}
	if code != 0 {
		return nil, fmt.Sprintf("git ls-files exited %d under %s (not a git work tree?): %s",
			code, absRoot, firstLine(out)), false
	}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, "", true
}

// treeFile is one enumerated working-tree file: its path relative to the scan
// root, and the size os.Lstat reported for it.
type treeFile struct {
	rel  string
	size int64
}

// regularFiles narrows an ls-files listing to the paths that are regular files
// on disk right now, carrying each one's size.
//
// The narrowing is not cosmetic. `--cached` lists a path that has been deleted
// in the working tree but not yet staged, and a gitlink (submodule) is listed as
// what stats as a directory. Handing either to grep makes it exit 2, which is
// could-not-check — turning a worker's ordinary mid-refactor state into a
// blocked gate. Skipping them scans strictly less, never more.
func regularFiles(absRoot string, paths []string, tl tools) []treeFile {
	out := make([]treeFile, 0, len(paths))
	for _, rel := range paths {
		fi, err := tl.stat(filepath.Join(absRoot, rel))
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, treeFile{rel: rel, size: fi.Size()})
	}
	return out
}

// --- check 1: conflict markers ----------------------------------------------

// conflictMarkerExclusion is the ONE path shape this check declines to scan,
// carried as a NAMED CONSTANT rather than a flag: an exclusion nobody can add at
// the command line is an exclusion a reviewer audits in exactly one place, and a
// gate whose blind spots are configurable is not a gate.
//
// It exists because this tool's own committed fixture,
// tools/desk/cmd/deskpreflight/testdata/markers/conflict.txt, carries real
// conflict markers BY DESIGN — it is what the marker regex is tested against.
// Without the exclusion the gate failed on a spotless checkout of the repo that
// ships it, and the only way to green it was to damage the fixture: a check
// whose whole job is to refuse, pushing the worker to weaken it.
//
// The check PRINTS this constant on both its clean and its failed line, so its
// output always ships with the shape it cannot see.
const conflictMarkerExclusion = "**/testdata/**"

// grepArgBudget bounds the bytes of path arguments handed to ONE grep
// invocation. macOS caps a process's argv+env near 1 MiB and Linux is larger but
// still finite, so a large repository's file list is split across invocations.
// It is a batching constant, not a scan limit: every path is scanned, across as
// many invocations as it takes.
const grepArgBudget = 96 * 1024

// runConflictMarkers shells grep to find VCS conflict markers left in the work
// tree. A marker line is a run of exactly seven <, =, or > characters at line
// start, bounded by a space or end-of-line — the shape `git` writes and the
// shape that slips into a PR when a merge is resolved by hand and one hunk is
// missed.
//
// grep is handed the explicit file list from gitWorkTreeFiles (minus
// conflictMarkerExclusion) rather than being turned loose on the tree with -R,
// so it can only read what git would stage. Its exit codes carry the three
// states directly: 1 = no match, 0 = match (and grep -H -n names the files),
// >=2 or "could not start" = could-not-check. grep is only READ.
func runConflictMarkers(absRoot string, tl tools) check {
	declined := " (declined: " + conflictMarkerExclusion + ")"
	listed, why, ok := gitWorkTreeFiles(absRoot, tl)
	if !ok {
		return couldNotCheck(nameConflictMarkers, why)
	}
	var scan []string
	for _, f := range regularFiles(absRoot, listed, tl) {
		if underDeclinedPath(f.rel) {
			continue
		}
		scan = append(scan, f.rel)
	}
	if len(scan) == 0 {
		return clean(nameConflictMarkers, "no conflict markers under "+absRoot+declined)
	}
	if _, err := tl.lookPath("grep"); err != nil {
		return couldNotCheck(nameConflictMarkers, "grep is not on PATH: "+err.Error())
	}
	var files []string
	for _, batch := range batchPaths(scan, grepArgBudget) {
		// -I skip binary files, -n show line numbers, -H always prefix the
		// filename (grep omits it for a single-file argument, which would leave
		// the match unparseable), -E extended regex, -- so a path beginning with
		// '-' is read as a path. Anchored, exactly-seven, boundaried.
		args := append([]string{"-I", "-n", "-H", "-E", `^(<{7}|={7}|>{7})( |$)`, "--"}, batch...)
		out, code, ran := tl.output(absRoot, "grep", args...)
		if !ran {
			return couldNotCheck(nameConflictMarkers, "could not run grep over "+absRoot)
		}
		switch code {
		case 1: // no match in this batch
		case 0:
			files = append(files, uniqueLeadingFields(out)...)
		default:
			return couldNotCheck(nameConflictMarkers,
				fmt.Sprintf("grep exited %d over %s: %s", code, absRoot, firstLine(out)))
		}
	}
	if len(files) == 0 {
		return clean(nameConflictMarkers, "no conflict markers under "+absRoot+declined)
	}
	sort.Strings(files)
	return failedCheck(nameConflictMarkers,
		"conflict marker(s) found in: "+strings.Join(files, ", ")+declined)
}

// underDeclinedPath reports whether rel matches conflictMarkerExclusion, i.e.
// lies inside a directory named `testdata`. Only a DIRECTORY component counts —
// that is what the trailing `/**` means — so a file named `testdata` is still
// scanned. The match is on the path RELATIVE to the scan root: pointing --root
// straight at a fixture directory scans it, which is how the tool's own
// real-fixture test still proves the check refuses.
func underDeclinedPath(rel string) bool {
	segs := strings.Split(filepath.ToSlash(rel), "/")
	for i := 0; i < len(segs)-1; i++ {
		if segs[i] == "testdata" {
			return true
		}
	}
	return false
}

// batchPaths splits paths into consecutive groups whose joined byte length stays
// within budget, so no single argv exceeds the platform limit. A single path
// longer than the budget still gets its own batch — dropping it would silently
// shrink the scan.
func batchPaths(paths []string, budget int) [][]string {
	var out [][]string
	var cur []string
	n := 0
	for _, p := range paths {
		if len(cur) > 0 && n+len(p)+1 > budget {
			out = append(out, cur)
			cur, n = nil, 0
		}
		cur = append(cur, p)
		n += len(p) + 1
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// uniqueLeadingFields pulls the "path" out of grep -n lines ("path:lineno:...")
// and returns the sorted unique set — the files that carry markers, not every
// matching line.
func uniqueLeadingFields(out []byte) []string {
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		path := line
		if i := strings.IndexByte(line, ':'); i >= 0 {
			path = line[:i]
		}
		seen[path] = true
	}
	var files []string
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// --- check 2: junk / oversize files -----------------------------------------

// oversizeBytes is the threshold above which a file staged for a PR is treated
// as an accident (a stray binary, a captured log, a vendored blob). 5 MiB.
const oversizeBytes = 5 * 1024 * 1024

// junkNames are the editor/merge droppings flagged by name — the same list the
// check has always carried, matched against each file's base name.
var junkNames = []string{"*.orig", "*.rej", "*.swp", "*~", ".DS_Store", "Thumbs.db"}

// runJunkOversize flags files that should not ride into a PR: editor/merge
// droppings by name, and anything over the size threshold.
//
// The candidates are the working tree as git sees it (gitWorkTreeFiles), never a
// filesystem walk — a walk descended into `.git/objects/pack/` and reported
// git's own multi-megabyte packfiles, which the worker is not "about to commit"
// under any reading, and which no `--root` a desk passes could have avoided.
//
// The name and size tests are done here with os.Lstat rather than by shelling
// `find` over the list: `find` would need every path on one argv (the same limit
// grep is batched around), and the two predicates are a base-name match and an
// integer compare. Lstat is a pure read and does not follow symlinks, so the
// check still cannot reach outside the tree it was pointed at.
func runJunkOversize(absRoot string, tl tools) check {
	listed, why, ok := gitWorkTreeFiles(absRoot, tl)
	if !ok {
		return couldNotCheck(nameJunkOversize, why)
	}
	var hits []string
	for _, f := range regularFiles(absRoot, listed, tl) {
		if isJunkName(filepath.Base(f.rel)) || f.size > oversizeBytes {
			hits = append(hits, f.rel)
		}
	}
	if len(hits) == 0 {
		return clean(nameJunkOversize, fmt.Sprintf(
			"no junk or oversize files among the %d work-tree file(s) under %s", len(listed), absRoot))
	}
	sort.Strings(hits)
	return failedCheck(nameJunkOversize,
		"junk/oversize file(s): "+strings.Join(hits, ", "))
}

// isJunkName reports whether a file's base name matches the junk list.
func isJunkName(base string) bool {
	for _, pat := range junkNames {
		if ok, err := filepath.Match(pat, base); err == nil && ok {
			return true
		}
	}
	return false
}

// --- check 3: gofmt + go vet on the touched Go files ------------------------

// runGoFmtVet runs gofmt -l and go vet, SCOPED to the Go files that git reports
// as touched under --root — not the whole tree, so the gate flags the worker's
// own change and not a pre-existing wart elsewhere in the repo.
//
// The touched set is derived from `git status --porcelain -- .` (run with the
// working directory at --root, so the pathspec bounds it to that subtree). If
// git cannot answer — not installed, --root not in a work tree — the touched set
// is unknowable and the whole check is could-not-check, never an empty pass.
// git/gofmt/go are all run READ-only.
func runGoFmtVet(absRoot string, tl tools) check {
	if _, err := tl.lookPath("git"); err != nil {
		return couldNotCheck(nameGoFmtVet, "git is not on PATH, so the touched Go set is unknowable: "+err.Error())
	}
	topOut, code, ran := tl.output(absRoot, "git", "rev-parse", "--show-toplevel")
	if !ran || code != 0 {
		return couldNotCheck(nameGoFmtVet,
			"git could not resolve a work tree at "+absRoot+" (touched Go set unknowable): "+firstLine(topOut))
	}
	repoRoot := strings.TrimSpace(string(topOut))

	stOut, code, ran := tl.output(absRoot, "git", "status", "--porcelain", "--", ".")
	if !ran || code != 0 {
		return couldNotCheck(nameGoFmtVet,
			"git status failed under "+absRoot+" (touched Go set unknowable): "+firstLine(stOut))
	}
	goFiles := touchedGoFiles(string(stOut), repoRoot)
	if len(goFiles) == 0 {
		return clean(nameGoFmtVet, "no touched Go files under "+absRoot)
	}

	// gofmt -l lists files that are NOT properly formatted; empty output = clean.
	if _, err := tl.lookPath("gofmt"); err != nil {
		return couldNotCheck(nameGoFmtVet, "gofmt is not on PATH: "+err.Error())
	}
	fmtOut, code, ran := tl.output("", "gofmt", append([]string{"-l"}, goFiles...)...)
	if !ran {
		return couldNotCheck(nameGoFmtVet, "could not run gofmt")
	}
	if code != 0 {
		return couldNotCheck(nameGoFmtVet, fmt.Sprintf("gofmt exited %d: %s", code, firstLine(fmtOut)))
	}
	if bad := nonEmptyLines(fmtOut); len(bad) > 0 {
		sort.Strings(bad)
		return failedCheck(nameGoFmtVet, "gofmt: not formatted: "+strings.Join(bad, ", "))
	}

	// go vet the unique package directories the touched files live in.
	if _, err := tl.lookPath("go"); err != nil {
		return couldNotCheck(nameGoFmtVet, "go is not on PATH: "+err.Error())
	}
	for _, dir := range uniqueDirs(goFiles) {
		vetOut, code, ran := tl.output(dir, "go", "vet", ".")
		if !ran {
			return couldNotCheck(nameGoFmtVet, "could not run go vet in "+dir)
		}
		if code != 0 {
			return failedCheck(nameGoFmtVet, "go vet failed in "+dir+": "+firstLine(vetOut))
		}
	}
	return clean(nameGoFmtVet, fmt.Sprintf("gofmt + go vet clean on %d touched Go file(s)", len(goFiles)))
}

// touchedGoFiles parses `git status --porcelain` output into the absolute paths
// of the touched *.go files, resolving each against the repo root. Deleted paths
// are skipped (there is nothing left to format or vet); a rename's NEW path is
// taken.
func touchedGoFiles(porcelain, repoRoot string) []string {
	seen := map[string]bool{}
	var out []string
	sc := bufio.NewScanner(strings.NewReader(porcelain))
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 4 {
			continue
		}
		xy := line[:2]
		rest := line[3:]
		// A deletion in either column has no working-tree file to check.
		if strings.ContainsAny(xy, "D") {
			continue
		}
		path := rest
		// Rename/copy: "orig -> new"; the new path is what exists.
		if i := strings.Index(rest, " -> "); i >= 0 {
			path = rest[i+4:]
		}
		path = strings.Trim(strings.TrimSpace(path), `"`)
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		abs := path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(repoRoot, path)
		}
		if seen[abs] {
			continue
		}
		if fi, err := os.Stat(abs); err != nil || fi.IsDir() {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	sort.Strings(out)
	return out
}

func uniqueDirs(files []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		d := filepath.Dir(f)
		if seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// --- check 4: statusgen --lint ----------------------------------------------

// runStatusgenLint shells statusgen in --lint mode against the ABSOLUTE root,
// from a NEUTRAL working directory (os.TempDir), so statusgen scans exactly
// --root and never picks up the cwd the worker happens to be in. statusgen
// --lint is a pure read; it writes nothing when --lint is set.
//
// statusgen's exit code is the check's state: 0 = clean, a "could not start" is
// could-not-check (statusgen not installed), and any other nonzero is a lint
// failure the worker must resolve.
func runStatusgenLint(absRoot string, tl tools) check {
	if _, err := tl.lookPath("statusgen"); err != nil {
		return couldNotCheck(nameStatusgenLint, "statusgen is not on PATH: "+err.Error())
	}
	out, code, ran := tl.output(os.TempDir(), "statusgen", "--root", absRoot, "--lint")
	if !ran {
		return couldNotCheck(nameStatusgenLint, "could not run statusgen --lint")
	}
	if code == 0 {
		return clean(nameStatusgenLint, "statusgen --lint clean for "+absRoot)
	}
	return failedCheck(nameStatusgenLint,
		fmt.Sprintf("statusgen --lint exited %d for %s: %s", code, absRoot, lintProblemLine(out)))
}

// lintProblemLine surfaces the first PROBLEM: line statusgen printed (its config
// echo is noise); falls back to the first non-empty line.
func lintProblemLine(out []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(t, "PROBLEM:") || strings.HasPrefix(t, "LINT:") {
			return t
		}
	}
	return firstLine(out)
}

// --- check 5: leak-sweep (WHERE PRESENT) ------------------------------------

// runLeakSweep runs the installed pre-publication leak-sweep over the tree, and
// returns present=false when the tool is NOT installed.
//
// This is the one deliberately conditional check (brief facts: "the installed
// leak-sweep check where present"). The leak-sweep is house-internal
// publication infrastructure that is not shipped with this tool and is absent by
// design in the public tree; its absence is a known, accepted condition — an
// optional check that isn't part of the set here — NOT the could-not-check case
// that a missing REQUIRED tool triggers everywhere else. When it IS installed it
// obeys the same three states as every other check.
func runLeakSweep(absRoot string, tl tools) (check, bool) {
	if _, err := tl.lookPath("leaksweep"); err != nil {
		return check{}, false
	}
	out, code, ran := tl.output("", "leaksweep", "run", "--tree", absRoot)
	if !ran {
		return couldNotCheck(nameLeakSweep, "leaksweep is installed but could not run over "+absRoot), true
	}
	if code == 0 {
		return clean(nameLeakSweep, "leaksweep clean for "+absRoot), true
	}
	return failedCheck(nameLeakSweep,
		fmt.Sprintf("leaksweep exited %d for %s: %s", code, absRoot, firstLine(out))), true
}

// --- shared helpers ---------------------------------------------------------

func nonEmptyLines(out []byte) []string {
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			lines = append(lines, t)
		}
	}
	return lines
}

func firstLine(out []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			return t
		}
	}
	return ""
}
