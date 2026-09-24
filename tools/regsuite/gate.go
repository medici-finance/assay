package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// gateOptions is the parsed form of `regsuite gate`'s flags.
type gateOptions struct {
	Head     string
	Base     string
	Modules  []string // optional narrowing; empty = auto-discover the union of head and base
	Selector string   // execution -run selector; empty = regressionListPattern
}

// execution is one top-level regression test's outcome from a HEAD run.
type execution struct {
	Name    string
	Outcome string // "pass", "fail", or "skip"
}

// moduleResult carries everything gathered for one module (a repo-relative
// path) before the pure verdict function looks at it.
type moduleResult struct {
	Path       string
	HeadListed []string
	BaseListed []string
	Executed   []execution // head only — the brief never executes the base tree
	BuildError string      // non-empty => this module is could-not-check
}

// runGate is the whole orchestration: discover modules, shell out to `go`
// per module, and hand the gathered facts to the pure verdict function
// (verdict.go). It never exits the process and never panics on a bad
// --head/--base — every failure mode it can hit becomes a could-not-check
// verdict (exit 2), per Ground rules / Task step 2: "Never exit 0."
func runGate(opts gateOptions) result {
	var problems []string
	if !dirReadable(opts.Head) {
		problems = append(problems, fmt.Sprintf("--head %q is not a readable directory", opts.Head))
	}
	if !dirReadable(opts.Base) {
		problems = append(problems, fmt.Sprintf("--base %q is not a readable directory", opts.Base))
	}
	if len(problems) > 0 {
		return result{ExitCode: 2, Report: "could-not-check\n  " + strings.Join(problems, "\n  ") + "\n"}
	}

	selector := opts.Selector
	if selector == "" {
		selector = regressionListPattern
	}

	var paths []string
	if len(opts.Modules) > 0 {
		seen := map[string]bool{}
		for _, m := range opts.Modules {
			m = filepath.ToSlash(filepath.Clean(m))
			if !seen[m] {
				seen[m] = true
				paths = append(paths, m)
			}
		}
	} else {
		headMods, err := discoverModules(opts.Head)
		if err != nil {
			return result{ExitCode: 2, Report: fmt.Sprintf("could-not-check\n  discovering modules under --head: %v\n", err)}
		}
		baseMods, err := discoverModules(opts.Base)
		if err != nil {
			return result{ExitCode: 2, Report: fmt.Sprintf("could-not-check\n  discovering modules under --base: %v\n", err)}
		}
		seen := map[string]bool{}
		for _, m := range headMods {
			seen[m] = true
		}
		for _, m := range baseMods {
			seen[m] = true
		}
		for m := range seen {
			paths = append(paths, m)
		}
	}
	sort.Strings(paths)

	mods := make([]moduleResult, 0, len(paths))
	for _, p := range paths {
		mods = append(mods, resolveModule(opts.Head, opts.Base, p, selector))
	}

	return computeVerdict(mods)
}

// resolveModule gathers listed/executed facts for one module path from both
// trees. A module absent from one tree (a brand-new module, or one this
// change deleted outright) is simply reported as zero-listed on that side —
// the count-drop check over the totals still catches a deleted module's
// tests disappearing.
func resolveModule(headRoot, baseRoot, relpath, selector string) moduleResult {
	res := moduleResult{Path: relpath}

	headDir := filepath.Join(headRoot, relpath)
	if dirHasGoMod(headDir) {
		listed, err := goListRegression(headDir)
		if err != nil {
			res.BuildError = err.Error()
			return res
		}
		res.HeadListed = listed

		executed, err := goRunRegression(headDir, selector)
		if err != nil {
			res.BuildError = err.Error()
			return res
		}
		res.Executed = executed
	}

	baseDir := filepath.Join(baseRoot, relpath)
	if dirHasGoMod(baseDir) {
		listed, err := goListRegression(baseDir)
		if err != nil {
			res.BuildError = err.Error()
			return res
		}
		res.BaseListed = listed
	}

	return res
}

// testNameLine matches a bare top-level test name as `go test -list` prints
// it — one identifier per line, distinct from the "ok  <pkg>  <dur>" and
// "?   <pkg>  [no test files]" summary lines the command also prints for
// ./... over multiple packages.
var testNameLine = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)

// goListRegression runs `go test -list '^TestRegression_' ./...` in dir and
// returns the matched top-level names. This is also this tool's build gate:
// `go test -list` compiles the package (including its _test.go files) before
// listing, so a module that fails to compile — including a broken test file,
// which a bare `go build ./...` would never notice, since `go build` skips
// _test.go files entirely — fails here with a non-nil error, which the
// caller reports as could-not-check.
func goListRegression(dir string) ([]string, error) {
	cmd := exec.Command("go", "test", "-list", regressionListPattern, "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go test -list %s ./... failed in %s: %v\n%s", regressionListPattern, dir, err, out)
	}
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if testNameLine.MatchString(line) {
			names = append(names, line)
		}
	}
	sort.Strings(names)
	return names, nil
}

// testEvent is the subset of `go test -json`'s TestEvent this tool reads.
type testEvent struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
}

// goRunRegression runs `go test -count=1 -json -run <selector> ./...` in dir
// and returns the top-level (non-subtest) pass/fail/skip outcomes for names
// carrying RegressionTestPrefix. A selector that matches nothing exits 0 and
// produces package-level JSON events only (no Test field) — exactly the
// vacuous pass this whole gate exists to catch — so an empty, error-free
// result here is a normal, expected outcome that the verdict layer, not this
// function, decides what to do with.
func goRunRegression(dir, selector string) ([]execution, error) {
	cmd := exec.Command("go", "test", "-count=1", "-json", "-run", selector, "./...")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var execs []execution
	sc := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var ev testEvent
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			// go test -json emits one JSON object per line; a line that
			// fails to parse means the toolchain emitted something this
			// tool does not understand. Treat it as could-not-check rather
			// than silently dropping a line that might have carried a
			// fail event.
			return nil, fmt.Errorf("go test -json -run %s ./... in %s: unparsable output line: %v", selector, dir, err)
		}
		if ev.Test == "" || strings.Contains(ev.Test, "/") {
			continue // package-level event, or a subtest — top-level only
		}
		if !strings.HasPrefix(ev.Test, RegressionTestPrefix) {
			continue
		}
		switch ev.Action {
		case "pass", "fail", "skip":
			execs = append(execs, execution{Name: ev.Test, Outcome: ev.Action})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("go test -json -run %s ./... in %s: reading output: %w", selector, dir, err)
	}

	if runErr != nil {
		// A non-zero exit is EXPECTED when a regression test genuinely
		// fails — that is the ordinary "fail" verdict path, driven by the
		// fail event already collected above, not a could-not-check. Only
		// treat the non-zero exit as could-not-check when nothing collected
		// explains it (a panic, a toolchain fault, a build break `-list`
		// somehow missed).
		hasFail := false
		for _, e := range execs {
			if e.Outcome == "fail" {
				hasFail = true
				break
			}
		}
		if !hasFail {
			return nil, fmt.Errorf("go test -count=1 -run %s ./... failed in %s with no test-level failure to explain it: %v\n%s", selector, dir, runErr, stderr.String())
		}
	}

	return execs, nil
}
