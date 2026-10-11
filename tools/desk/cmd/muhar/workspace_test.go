package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func needGo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not on PATH")
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// replaceTargets maps each replace directive's Old.Path to its New.Path.
func replaceTargets(t *testing.T, gomod string) map[string]string {
	t.Helper()
	reps, err := readReplaces(gomod)
	if err != nil {
		t.Fatalf("read replaces: %v", err)
	}
	out := map[string]string{}
	for _, r := range reps {
		out[r.Old.Path] = r.New.Path
	}
	return out
}

// The reported shape, end to end: a module whose go.mod carries a `../`
// relative replace, swept in parallel. Each worker's copy sits in a fresh temp
// dir, so the unrelinked replace resolves to nothing, the baseline goes red in
// the copy and the whole run is discarded — the truth-suite's [setup failed].
//
// regression: #2550
func TestParallelCopyRelativeReplace(t *testing.T) {
	needGo(t)
	base := t.TempDir()
	writeFiles(t, base, map[string]string{
		"lib/go.mod": "module example.com/lib\n\ngo 1.21\n",
		"lib/lib.go": "package lib\n\nfunc Answer() int { return 42 }\n",
		"app/go.mod": "module example.com/app\n\ngo 1.21\n\nrequire example.com/lib v0.0.0\n\nreplace example.com/lib => ../lib\n",
		"app/app.go": "package app\n\nimport \"example.com/lib\"\n\nfunc Value() int { return lib.Answer() }\n",
		"app/app_test.go": "package app\n\nimport \"testing\"\n\n" +
			"func TestValue(t *testing.T) {\n\tif Value() != 42 {\n\t\tt.Fatal(Value())\n\t}\n}\n",
	})
	app := filepath.Join(base, "app")
	origMod, err := os.ReadFile(filepath.Join(app, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	h := &Harness{
		Root: app,
		Jobs: 2,
		Test: func(root string) bool {
			cmd := exec.Command("go", "test", "./...", "-count=1")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOPROXY=off", "GOTOOLCHAIN=local")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Logf("suite in %s: %s", root, out)
			}
			return err != nil
		},
		Control: Mutation{Name: "control", File: "app.go", Old: "return lib.Answer()", New: "return lib.Answer() + 1"},
		Mutations: []Mutation{
			{Name: "zero", File: "app.go", Old: "return lib.Answer()", New: "return 0"},
			{Name: "negate", File: "app.go", Old: "return lib.Answer()", New: "return -lib.Answer()"},
		},
	}
	h.Workspace = newWorkspace(app)

	rep := h.Run()
	if rep.Broken {
		t.Fatalf("parallel sweep of a module with a ../ replace was discarded: %s", rep.BrokenReason)
	}
	for _, r := range rep.Results {
		if r.Result != Caught {
			t.Fatalf("mutation %q: got %s (%v), want CAUGHT", r.Mutation.Name, r.Result, r.Err)
		}
	}
	if after, _ := os.ReadFile(filepath.Join(app, "go.mod")); string(after) != string(origMod) {
		t.Fatalf("the SOURCE go.mod changed — only the copy may be rewritten:\n%s", after)
	}
}

// The class, not the instance: EVERY module root in the copy is relinked (a
// nested module is the planted second instance), and only the replacements
// that escape the copied tree move. In-tree, versioned and absolute targets,
// and fixture go.mod files under testdata/, stay exactly as written.
//
// regression: #2550
func TestRelinkEscapingReplaces(t *testing.T) {
	needGo(t)
	base := t.TempDir()
	abs := filepath.Join(t.TempDir(), "elsewhere")
	src := filepath.Join(base, "src")
	fixture := "module example.com/fixture\n\ngo 1.21\n\nreplace example.com/f => ../../../nowhere\n"
	writeFiles(t, src, map[string]string{
		"go.mod": "module example.com/root\n\ngo 1.21\n\nreplace (\n" +
			"\texample.com/a => ../outside\n" +
			"\texample.com/b => ./inner\n" +
			"\texample.com/c v1.0.0 => example.com/fork v1.2.0\n" +
			"\texample.com/d => " + abs + "\n" +
			"\texample.com/g v0.3.0 => ../pinned\n" +
			")\n",
		"inner/go.mod":         "module example.com/b\n\ngo 1.21\n",
		"nested/go.mod":        "module example.com/nested\n\ngo 1.21\n\nreplace example.com/e => ../../outside2\n",
		"testdata/fix/go.mod":  fixture,
		"_hidden/go.mod":       fixture,
		"plain/go.mod":         "module example.com/plain\n\ngo 1.21\n",
		"plain/doc.go":         "package plain\n",
		"nested/sub/readme.md": "not a module\n",
	})

	dst, cleanup, err := newWorkspace(src)(0)
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	defer cleanup()

	root := replaceTargets(t, filepath.Join(dst, "go.mod"))
	want := map[string]string{
		"example.com/a": filepath.Join(base, "outside"),
		"example.com/b": "./inner",
		"example.com/c": "example.com/fork",
		"example.com/d": abs,
		"example.com/g": filepath.Join(base, "pinned"),
	}
	for old, w := range want {
		if root[old] != w {
			t.Errorf("copy's root go.mod: replace %s => %q, want %q", old, root[old], w)
		}
	}
	if got := replaceTargets(t, filepath.Join(dst, "nested", "go.mod"))["example.com/e"]; got != filepath.Join(base, "outside2") {
		t.Errorf("nested module not relinked: replace example.com/e => %q, want %q", got, filepath.Join(base, "outside2"))
	}
	for _, f := range []string{"testdata/fix/go.mod", "_hidden/go.mod"} {
		if b, _ := os.ReadFile(filepath.Join(dst, filepath.FromSlash(f))); string(b) != fixture {
			t.Errorf("%s is ignored by the go command and must be copied verbatim, got:\n%s", f, b)
		}
	}
	if got := replaceTargets(t, filepath.Join(src, "go.mod"))["example.com/a"]; got != "../outside" {
		t.Errorf("the SOURCE go.mod changed: replace example.com/a => %q", got)
	}
}
