package askassay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRoot walks up from the package directory to the directory holding
// go.mod. It is used by the checks that read the tree they guard.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find the module root above the package directory")
	return ""
}

// packageSources returns this package's non-test Go source, with line comments
// stripped, keyed by file name. Comments are stripped because this file's own
// prose names the very tokens it forbids.
func packageSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		out[n] = stripLineComments(string(b))
	}
	if len(out) == 0 {
		t.Fatal("no non-test source found — this scan would pass vacuously")
	}
	return out
}

func stripLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		code := line
		if i := strings.Index(code, "//"); i >= 0 {
			code = code[:i]
		}
		b.WriteString(code)
		b.WriteByte('\n')
	}
	return b.String()
}

// TestExactlyOneSubprocessCallSite — read-only is structural, not
// conventional. A second exec site is a second place a write can be added, and
// it would not go past the guard.
func TestExactlyOneSubprocessCallSite(t *testing.T) {
	srcs := packageSources(t)
	for name, src := range srcs {
		for _, form := range []string{"syscall.Exec", "os.StartProcess", "syscall.ForkExec"} {
			if strings.Contains(src, form) {
				t.Errorf("%s reaches a process through %s, bypassing the guarded runner", name, form)
			}
		}
	}
	// exec.CommandContext( is not a substring of exec.Command( — the two forms
	// are counted separately and summed.
	var construct int
	var where []string
	for name, src := range srcs {
		n := strings.Count(src, "exec.CommandContext(") + strings.Count(src, "exec.Command(")
		construct += n
		for i := 0; i < n; i++ {
			where = append(where, name)
		}
	}
	if construct != 1 {
		t.Fatalf("this package holds %d subprocess construction sites (%v), want exactly 1 — every probe must funnel through the guarded runner", construct, where)
	}
	if where[0] != "probe.go" {
		t.Errorf("the single subprocess site is in %s, want probe.go — the file whose header documents the read-only contract", where[0])
	}
	// The one site must be unexported and reached only through Runner.Run.
	if !strings.Contains(srcs["probe.go"], "func execRead(") {
		t.Error("probe.go does not define execRead — the named, unexported single call site")
	}
	if strings.Contains(srcs["probe.go"], "func ExecRead(") {
		t.Error("the subprocess call site is EXPORTED — an external caller could run an argv without passing the guard")
	}
}

// TestPackageHoldsNoWritePath — the pane must not be one commit away from
// writing. This is the brief's original Verify row 2, re-pointed at the
// surface that exists: no write verb, no credential-minting verb, and no
// steering surface is named anywhere in this package's shipped source.
func TestPackageHoldsNoWritePath(t *testing.T) {
	// Assembled so this test's own source does not contain the literals it
	// forbids — a scanner that trips on itself is a scanner nobody keeps.
	forbidden := []struct{ token, why string }{
		{"gh pr " + "create", "opens a PR"},
		{"gh pr " + "merge", "merges"},
		{"gh pr " + "review", "mints a review verdict"},
		{"gh pr " + "ready", "flips a draft"},
		{"gh issue " + "create", "files an issue"},
		{"gh issue " + "comment", "posts"},
		{"gh issue " + "close", "closes"},
		{"gh " + "workflow run", "triggers a workflow"},
		{"git " + "push", "writes to the remote"},
		{"git " + "commit", "writes to the tree"},
		{"desk" + "post", "posts as an App"},
		{"desk" + "reply", "posts as an App"},
		{"desk" + "token", "mints a credential"},
		{"desk" + "claim", "takes a work claim"},
		{"desk" + "pr", "opens a PR"},
		{"kry" + "ton", "is the human-authority surface a read-only pane must not hold"},
	}
	for name, src := range packageSources(t) {
		lower := strings.ToLower(src)
		for _, f := range forbidden {
			if strings.Contains(lower, strings.ToLower(f.token)) {
				t.Errorf("%s names %q in shipped source — it %s, and this pane is read-only", name, f.token, f.why)
			}
		}
	}
}

// TestTheProbeSeamIsNotReplaceableFromOutside — a guard a caller can swap out
// is a flag, and the gap between a flag and a grant is one commit.
func TestTheProbeSeamIsNotReplaceableFromOutside(t *testing.T) {
	src := packageSources(t)["probe.go"]
	if !strings.Contains(src, "run func(ctx context.Context, dir string, argv []string) ([]byte, error)") {
		t.Error("probe.go's subprocess seam is not the expected unexported field — if it was renamed, re-verify that it is still unexported")
	}
	if strings.Contains(src, "Run func(") || strings.Contains(src, "Exec func(") || strings.Contains(src, "Runner func(") {
		t.Error("probe.go exposes an EXPORTED function field on the runner — an external caller could substitute the subprocess seam and route a write through it")
	}
	// The guard must be called by Run, unconditionally and first.
	runBody := src
	if i := strings.Index(runBody, "func (r Runner) Run("); i >= 0 {
		runBody = runBody[i:]
	} else {
		t.Fatal("probe.go does not define Runner.Run")
	}
	guardAt := strings.Index(runBody, "GuardReadOnly(argv)")
	seamAt := strings.Index(runBody, "r.run(")
	if guardAt < 0 {
		t.Fatal("Runner.Run does not call GuardReadOnly")
	}
	if seamAt >= 0 && guardAt > seamAt {
		t.Error("Runner.Run reaches the subprocess seam before the guard")
	}
}

// TestAllowListsAreClosedNotConfigurable — the read-only surface must not be
// widenable at run time. An env-var or flag that adds a verb is exactly the
// dormant write path a sibling decision refused to ship.
func TestAllowListsAreClosedNotConfigurable(t *testing.T) {
	for name, src := range packageSources(t) {
		for _, form := range []string{"os.Getenv", "os.LookupEnv", "flag.String", "flag.Bool", "os.Environ"} {
			if strings.Contains(src, form) {
				t.Errorf("%s reads run-time configuration via %s — the read-only allow-list must be closed in source, not widenable at run time", name, form)
			}
		}
	}
	// And exported mutation of the allow-list maps must not exist.
	for _, sym := range []string{"func AddReadVerb", "func RegisterProbe", "func SetAllowList", "var ReadOnlyBinaries", "var GhReadVerbs"} {
		for name, src := range packageSources(t) {
			if strings.Contains(src, sym) {
				t.Errorf("%s exports %s — the allow-list is mutable from outside the package", name, sym)
			}
		}
	}
}

// TestScannersReportABentTree is the positive control for the two source
// scanners above: fed source that violates each rule, they must report it.
// Without this, a scanner that silently matches nothing passes forever.
func TestScannersReportABentTree(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		forbids string
	}{
		{"a second exec site", "func x() { exec.Command(\"gh\") }", "exec.Command("},
		{"a write verb", "const c = \"gh pr merge 1\"", "gh pr merge"},
		{"an env-widened allow-list", "func x() { _ = os.Getenv(\"EXTRA_VERBS\") }", "os.Getenv"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stripped := stripLineComments(c.src)
			if !strings.Contains(strings.ToLower(stripped), strings.ToLower(c.forbids)) {
				t.Fatalf("POSITIVE CONTROL FAILED: the scanner's matcher did not see %q in bent source %q", c.forbids, c.src)
			}
		})
	}
	// And the comment-stripping must not be what makes the scan pass: a
	// violation inside a comment is not a violation, but one in code is.
	if strings.Contains(stripLineComments("// exec.Command(\"gh\")"), "exec.Command(") {
		t.Error("comment stripping failed — a commented example would be reported as a call site")
	}
	if !strings.Contains(stripLineComments("x := exec.Command(\"gh\") // a comment"), "exec.Command(") {
		t.Error("comment stripping removed real code — a real call site would go unreported")
	}
}
