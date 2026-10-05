package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Defect class: a mutation spec's `old` anchor goes stale. A refactor rewrites the
// guard a mutation targets (moves it, re-indents it, wraps it), the spec still names
// the old text, and muhar can no longer plant the mutation — COULD_NOT_MUTATE. The
// guard is then silently untested. truth-suite only runs on push to main and on a
// cron, and most specs are in no workflow matrix at all, so nothing on a PR sees it:
// the deskkit spec went red on main this way, ten more anchors across six other specs
// had already gone stale unseen, and one spec had no control at all.
//
// The guard below runs on every PR (it is a plain `go test` in tools/desk): it finds
// every muhar spec in the repo and dry-runs each control and mutation edit against the
// file it names, with the same applyEdit the harness uses. It never runs a suite, so
// it costs file reads only; whether a planted mutation is CAUGHT stays truth-suite's.

// staleAnchors walks root for muhar specs and returns how many it found plus one
// problem line per edit that cannot land. A relative spec `file` resolves against the
// directory of the nearest go.mod above the spec — the module dir muhar is run from.
func staleAnchors(root string) (int, []string, error) {
	var specs int
	var problems []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var s spec
		if json.Unmarshal(data, &s) != nil || !looksLikeSpec(s) {
			return nil
		}
		specs++
		rel, _ := filepath.Rel(root, path)
		modDir, ok := moduleDir(root, filepath.Dir(path))
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: no go.mod above the spec to resolve its files against", rel))
			return nil
		}
		edits := s.Mutations
		if s.Control.File == "" {
			// muhar plants the control before any mutation; without one every run of
			// the spec is reported HARNESS BROKEN, so no mutation in it is ever tested.
			problems = append(problems, fmt.Sprintf("%s: no positive control — muhar discards every run of this spec", rel))
		} else {
			edits = append([]Mutation{s.Control}, edits...)
		}
		for _, m := range edits {
			file := m.File
			if !filepath.IsAbs(file) {
				file = filepath.Join(modDir, file)
			}
			src, err := os.ReadFile(file)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: %q: cannot read %s: %v", rel, m.Name, m.File, err))
				continue
			}
			if _, err := applyEdit(string(src), m); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", rel, err))
			}
		}
		return nil
	})
	sort.Strings(problems)
	return specs, problems, err
}

// looksLikeSpec is true for JSON that decodes into a muhar spec with at least one
// edit naming a file. Other JSON (fixtures, configs, other harnesses' arrays) is not.
func looksLikeSpec(s spec) bool {
	if s.Control.File != "" {
		return true
	}
	for _, m := range s.Mutations {
		if m.File != "" {
			return true
		}
	}
	return false
}

// moduleDir returns the nearest directory at or above dir (and inside root) holding
// a go.mod.
func moduleDir(root, dir string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, true
		}
		if dir == root || filepath.Dir(dir) == dir {
			return "", false
		}
		dir = filepath.Dir(dir)
	}
}

// repoRoot is the checkout root: the nearest ancestor of the cwd holding
// tools/desk/go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "tools", "desk", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no ancestor of the cwd holds tools/desk/go.mod — cannot find the repo root")
		}
		dir = parent
	}
}

// TestSpecAnchorsLand is the class guard: every control and mutation in every muhar
// spec in the repo must still land on the file it names.
func TestSpecAnchorsLand(t *testing.T) {
	root := repoRoot(t)
	specs, problems, err := staleAnchors(root)
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	// Zero specs means the walk or the shape test broke, not that the repo is clean.
	if specs < 10 {
		t.Fatalf("found %d muhar specs under the repo root — the discovery is broken", specs)
	}
	for _, p := range problems {
		t.Errorf("stale mutation anchor — %s", p)
	}
	if len(problems) > 0 {
		t.Logf("%d of the edits across %d specs cannot land; re-anchor `old` to the guard's current text (never delete the mutation)", len(problems), specs)
	}
}

// TestAnchorCheckFlagsStale is the guard's own positive control: a spec whose
// mutation names text the source no longer holds MUST be reported, a spec that
// lands must not, and non-spec JSON is ignored.
func TestAnchorCheckFlagsStale(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "mod")
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(mod, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/mod\n")
	write("pkg/guard.go", "package pkg\n\nfunc ok(n int) bool {\n\tif n < 0 {\n\t\treturn false\n\t}\n\treturn true\n}\n")
	write("pkg/good-mutations.json", `{"test":"true",
	  "control":{"name":"ctl","file":"pkg/guard.go","old":"\treturn true\n","new":"\treturn false\n"},
	  "mutations":[{"name":"guard off","file":"pkg/guard.go","old":"if n < 0 {","new":"if false {"}]}`)
	write("pkg/stale-mutations.json", `{"test":"true",
	  "control":{"name":"ctl","file":"pkg/guard.go","old":"\treturn true\n","new":"\treturn false\n"},
	  "mutations":[{"name":"guard off (pre-refactor text)","file":"pkg/guard.go","old":"if n <= -1 {","new":"if false {"}]}`)
	write("pkg/config.json", `{"mutations":"not a spec","other":[1,2,3]}`)
	write("pkg/array.json", `[{"test":"x","file":"pkg/guard.go","old":"a","new":"b"}]`)

	specs, problems, err := staleAnchors(root)
	if err != nil {
		t.Fatalf("staleAnchors: %v", err)
	}
	if specs != 2 {
		t.Errorf("found %d specs, want 2 (good + stale; config and array JSON are not specs)", specs)
	}
	if len(problems) != 1 {
		t.Fatalf("problems = %q, want exactly the stale spec's one edit", problems)
	}
	want := filepath.Join("mod", "pkg", "stale-mutations.json")
	if !strings.HasPrefix(problems[0], want+": ") || !strings.Contains(problems[0], "guard off (pre-refactor text)") {
		t.Errorf("problem = %q, want it to name %s and the stale entry", problems[0], want)
	}

	// A spec outside any Go module is reported, not skipped.
	if err := os.WriteFile(filepath.Join(root, "orphan-mutations.json"), []byte(`{"control":{"name":"c","file":"x.go","old":"a","new":"b"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, problems, err = staleAnchors(root); err != nil || len(problems) != 2 || !anyHas(problems, "orphan-mutations.json: no go.mod above the spec") {
		t.Fatalf("with an orphan spec: problems = %q, err = %v; want the orphan plus the stale edit", problems, err)
	}
	if err := os.Remove(filepath.Join(root, "orphan-mutations.json")); err != nil {
		t.Fatal(err)
	}

	// A spec with mutations but no control can never run healthily, so it is reported.
	write("pkg/nocontrol-mutations.json", `{"test":"true","mutations":[{"name":"guard off","file":"pkg/guard.go","old":"if n < 0 {","new":"if false {"}]}`)
	if _, problems, err = staleAnchors(root); err != nil || len(problems) != 2 || !anyHas(problems, "nocontrol-mutations.json: no positive control") {
		t.Fatalf("with a control-less spec: problems = %q, err = %v; want it named plus the stale edit", problems, err)
	}
}

func anyHas(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
