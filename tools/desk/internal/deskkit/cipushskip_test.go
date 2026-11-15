package deskkit

// Guards for the one per-push skip in ci.yml: the push-to-main leg ignores a
// commit whose only changed file is STATUS.md (the board regen's own commit).
// The pull_request leg must stay unfiltered, because build-test and changelog
// are required checks there, and the workflows that must see every main
// commit (the two leak sweeps, plugin-drift's rot leg, verify-gate-open) must
// keep an unfiltered push leg.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ciPathsIgnoreGlobs enumerates a paths-ignore filter's coverage in the
// negation form anyGlobMatches models: every path, minus each ignored pattern.
// paths-ignore skips a run only when EVERY changed file is ignored, so a
// change to one read file starts the run exactly when that file is not
// ignored itself.
func ciPathsIgnoreGlobs(ignored []string) []string {
	globs := []string{"**"}
	for _, p := range ignored {
		globs = append(globs, "!"+p)
	}
	return globs
}

func TestCIPushSkipsOnlyStatusMD(t *testing.T) {
	const root = "../../../.."
	raw, err := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	ci := string(raw)
	ignored, ok := ciOnEventKey(t, ci, "ci.yml", "push", "paths-ignore")
	if !ok || !reflect.DeepEqual(ignored, []string{"STATUS.md"}) {
		t.Errorf("ci.yml on.push.paths-ignore = %q (present %v), want exactly [STATUS.md]", ignored, ok)
	}
	if _, ok := ciOnEventKey(t, ci, "ci.yml", "push", "paths"); ok {
		t.Error("ci.yml on.push gained a paths: filter")
	}
	for _, key := range []string{"paths", "paths-ignore", "branches-ignore"} {
		if _, ok := ciOnEventKey(t, ci, "ci.yml", "pull_request", key); ok {
			t.Errorf("ci.yml on.pull_request has %s: build-test and changelog are required on every pull request", key)
		}
	}
	// Every CI-registry read of ci.yml must survive the push ignore list.
	rows := 0
	for _, e := range ciCrossModuleRegistry() {
		if e.workflow != ".github/workflows/ci.yml" {
			continue
		}
		rows++
		var walk []string
		for _, r := range e.reads {
			if !strings.ContainsAny(r, "*?") {
				walk = append(walk, r)
				continue
			}
			// A glob read cannot be walked; it is hidden when an ignored path
			// falls inside it.
			for _, p := range ignored {
				if anyGlobMatches(t, []string{r}, p) {
					t.Errorf("%s: glob read %s covers ignored path %s", e.test, r, p)
				}
			}
		}
		for _, fault := range ciReadCoverage(t, root, ciPathsIgnoreGlobs(ignored), walk) {
			t.Errorf("%s: read hidden by ci.yml on.push.paths-ignore: %s", e.test, fault)
		}
	}
	if rows == 0 {
		t.Fatal("no CI-registry row names ci.yml: the coverage loop checked nothing")
	}
	// The workflows that must see every main commit keep an unfiltered push leg.
	for _, wf := range []string{"leaksweep-control.yml", "leaksweep-pattern.yml", "plugin-drift.yml", "verify-gate-open.yml"} {
		b, err := os.ReadFile(filepath.Join(root, ".github/workflows", wf))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := ciOnEventKey(t, string(b), wf, "push", "paths-ignore"); ok {
			t.Errorf("%s on.push gained paths-ignore", wf)
		}
		if globs, ok := ciOnEventKey(t, string(b), wf, "push", "paths"); ok && !reflect.DeepEqual(globs, []string{"**"}) {
			t.Errorf("%s on.push.paths = %q, want none or [**]", wf, globs)
		}
	}
}

// TestCIPushSkipCoverageGuardFires is the negative control: an ignore list
// that swallows a registry read (docs/**), or the history log a docs/streams
// reader nominally covers, must be reported.
func TestCIPushSkipCoverageGuardFires(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"docs/streams/s/README.md", "docs/streams/.history.jsonl", "STATUS.md"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reads := []string{"docs/streams"}
	if f := ciReadCoverage(t, root, ciPathsIgnoreGlobs([]string{"STATUS.md"}), reads); len(f) != 0 {
		t.Errorf("STATUS.md-only ignore hid a docs/streams read: %v", f)
	}
	for _, bad := range [][]string{{"docs/**"}, {"STATUS.md", "docs/streams/.history.jsonl"}} {
		f := ciReadCoverage(t, root, ciPathsIgnoreGlobs(bad), reads)
		if len(f) == 0 || !strings.Contains(strings.Join(f, " "), "uncovered read: docs/streams/") {
			t.Errorf("ignore %q hid a docs/streams read and the guard reported %v", bad, f)
		}
	}
}
