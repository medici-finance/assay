package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// CLASS GUARD — a CI run of `verifyrun --in-container` aimed at the COMMITTED tree.
//
// The defect (#1433, windows-port/10): the staged CI leg's fail-closed step ran the
// wrapper against the committed harness pin (`--root .`) and read ANY non-zero exit
// as "the wrapper refused the placeholder". Once the real registry digest was
// pinned, the wrapper stopped refusing and the step went on reporting a refusal
// that never happened: on a Docker runner it would pull the image and run the
// brief's whole Verify table, and a daemon error or an inner failure printed the
// same "fail-closed OK". The class is any CI invocation of the in-container
// launcher whose input is the committed tree rather than a fixed scratch/fixture
// root: its meaning then shifts silently whenever the committed pin (or the brief
// it runs) changes. Every such invocation in a workflow must name an explicit
// non-committed `--root`.
//
// The guard scans every workflow file, staged and live, and fails naming each
// invocation whose `--root` is absent (verifyrun then defaults to the working
// directory, i.e. the checkout) or is the checkout itself. A planted instance —
// the pre-fix step, verbatim in shape — is the positive control, so a matcher that
// silently stops matching fails instead of reporting clean.

// vicCommittedRoots are the `--root` spellings that name the checkout itself.
var vicCommittedRoots = map[string]bool{
	".": true, "./": true, `"."`: true, `'.'`: true, `"./"`: true,
	"$GITHUB_WORKSPACE": true, "${GITHUB_WORKSPACE}": true,
	`"$GITHUB_WORKSPACE"`: true, `"${GITHUB_WORKSPACE}"`: true,
	"${{ github.workspace }}": true, `"${{ github.workspace }}"`: true,
}

var vicContinuationRe = regexp.MustCompile(`\\\r?\n[ \t]*`)
var vicRootArgRe = regexp.MustCompile(`--root(?:=|[ \t]+)("[^"]*"|'[^']*'|\$\{\{[^}]*\}\}|[^ \t]+)`)

// vicCommittedRootSites returns one line per in-container invocation in body
// whose root is absent or the checkout itself.
func vicCommittedRootSites(name, body string) []string {
	joined := vicContinuationRe.ReplaceAllString(body, " ")
	var bad []string
	for i, line := range strings.Split(joined, "\n") {
		// A YAML/shell comment line names the launcher in prose; it runs nothing.
		if !strings.Contains(line, "verifyrun --in-container") || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		// Only the text after the launcher belongs to this invocation.
		rest := line[strings.Index(line, "verifyrun --in-container"):]
		m := vicRootArgRe.FindStringSubmatch(rest)
		switch {
		case m == nil:
			bad = append(bad, name+": joined line "+strconv.Itoa(i+1)+": in-container invocation has no --root (defaults to the checkout)")
		case vicCommittedRoots[m[1]]:
			bad = append(bad, name+": joined line "+strconv.Itoa(i+1)+": in-container invocation runs against the committed tree (--root "+m[1]+")")
		}
	}
	return bad
}

// vicPlantedStep is the pre-fix fail-closed step, the positive control.
const vicPlantedStep = `      - name: Fail-closed control
        run: |
          if "${RUNNER_TEMP}/statusgen" verifyrun --in-container --root . \
               --brief docs/streams/windows-port/brief-10-verify-in-container.md; then
            exit 1
          fi
`

func TestInContainerCIRoots(t *testing.T) {
	if got := vicCommittedRootSites("planted.yml", vicPlantedStep); len(got) != 1 {
		t.Fatalf("positive control: the planted pre-fix step must be flagged exactly once, got %d: %v", len(got), got)
	}
	if got := vicCommittedRootSites("clean.yml", `x verifyrun --in-container --root "${fix}" --brief b.md`); len(got) != 0 {
		t.Fatalf("a scratch --root must not be flagged, got %v", got)
	}

	repo := filepath.Join("..")
	var files []string
	for _, dir := range []string{"ci/staged-workflows", ".github/workflows"} {
		m, err := filepath.Glob(filepath.Join(repo, dir, "*.yml"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	staged := filepath.Join(repo, "ci", "staged-workflows", "windows-ci-leg.yml")
	if _, err := os.Stat(staged); err != nil {
		if _, gerr := os.Stat(filepath.Join(repo, ".github", "workflows")); gerr == nil {
			t.Fatalf("full checkout but %s is unreadable (%v) — the guard would check nothing", staged, err)
		}
		t.Skipf("%s absent in this subset tree", staged)
	}
	sort.Strings(files)
	seen := 0
	var bad []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		for _, line := range strings.Split(vicContinuationRe.ReplaceAllString(body, " "), "\n") {
			if strings.Contains(line, "verifyrun --in-container") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				seen++
			}
		}
		bad = append(bad, vicCommittedRootSites(filepath.ToSlash(f), body)...)
	}
	if seen == 0 {
		t.Fatalf("no in-container invocation found in %d workflow files — the guard checked nothing", len(files))
	}
	for _, b := range bad {
		t.Errorf("in-container CI run against the committed tree: %s — point it at a scratch/fixture root", b)
	}
}

// TestFailClosedStepToken couples the staged leg's fail-closed step to the
// launcher's own refusal line: the step must assert the refusal TOKEN (never
// merely a non-zero exit), and the launcher must still print that token when it
// refuses a placeholder, so the two cannot drift apart silently.
func TestFailClosedStepToken(t *testing.T) {
	const token = "refusing to run"
	staged := filepath.Join("..", "ci", "staged-workflows", "windows-ci-leg.yml")
	raw, err := os.ReadFile(staged)
	if err != nil {
		if _, gerr := os.Stat(filepath.Join("..", ".github", "workflows")); gerr == nil {
			t.Fatalf("full checkout but %s is unreadable: %v", staged, err)
		}
		t.Skipf("%s absent in this subset tree", staged)
	}
	if !strings.Contains(string(raw), "grep -q '"+token+"'") {
		t.Errorf("%s: the fail-closed step no longer asserts the launcher's %q line — a non-zero exit alone is not a refusal", staged, token)
	}

	root := t.TempDir()
	writePairedVersions(t, root, "  digest: PENDING-HARVEST-x\n")
	writeFileWP10(t, filepath.Join(root, "b.md"), "# b\n")
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer errf.Close()
	code := runInContainer(filepath.Join(root, "b.md"), root, "", false, false, false, "", os.Stdout, errf)
	out, _ := os.ReadFile(errf.Name())
	if code != verifyrunExitCouldNot || !strings.Contains(string(out), token) {
		t.Fatalf("launcher refusal must exit %d and print %q; got exit %d, stderr:\n%s", verifyrunExitCouldNot, token, code, out)
	}
}
