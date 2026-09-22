package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBashQuoteMatchesBash is the one test that cannot be written from memory: it asks BASH what
// `printf %q` produces and requires bashQuote to agree, byte for byte. The [plan] argv line is
// built with %q in the oracle and diffed character by character by the parity harness, so a
// quoting difference is a divergence.
func TestBashQuoteMatchesBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on PATH")
	}
	cases := []string{
		"", "claude", "--model", "/assay:the-desk",
		`Invoke the "assay:the-desk" skill now.`,
		"HOME=/tmp/a b", "PATH=/a:/b", "it's", "a$b", "~lead", "#lead", "tail~", "x#y",
		"semi;colon", "pipe|bar", "star*", "back\\slash", "tab\there", "nl\nthere",
		"CELL_ROOTS=example-org/example-repo=/tmp/r",
	}
	for _, in := range cases {
		// The script text is a CONSTANT and the case travels as $1 — argv, never interpolated
		// into the script — so no input here can become shell syntax.
		out, err := exec.Command("bash", "-c", `printf '%q' "$1"`, "bash", in).Output()
		if err != nil {
			t.Fatalf("bash printf %%q failed for %q: %v", in, err)
		}
		if got, want := bashQuote(in), string(out); got != want {
			t.Errorf("bashQuote(%q) = %q, bash says %q", in, got, want)
		}
	}
}

func TestPrintPlanGrammar(t *testing.T) {
	dir := t.TempDir()
	c := &Cell{
		Env:  &Env{vals: map[string]string{}, set: map[string]bool{}},
		Name: "demo",
		Dir:  dir,
	}
	env := ComposedEnv{Sorted: []string{"AAA=1", "KUBECONFIG=/dev/null", "ZZZ=2"}}
	out := captureStdout(t, func() {
		c.printPlan(env, "/some/wt", []string{"claude", "--model", "fable", "/assay:the-desk"})
	})
	want := strings.Join([]string{
		"[plan] env AAA=1",
		"[plan] env KUBECONFIG=/dev/null",
		"[plan] env ZZZ=2",
		"[plan] argv claude --model fable /assay:the-desk",
		"[plan] cwd /some/wt",
		"[plan] lock " + filepath.Join(dir, "run", "lock.d"),
		"",
	}, "\n")
	if out != want {
		t.Errorf("plan grammar drifted:\ngot:\n%s\nwant:\n%s", out, want)
	}
}

// TestParityMutateInertWithoutTag is row 13's assertion at the unit level. It deliberately does
// NOT name the injector's environment variable: row 14 requires EVERY file in this package that
// names CELLCTL_PARITY_MUTATE to carry `//go:build parity`, tests included, so the env-driven
// half of this proof lives in parity_on_test.go behind that tag and this half asserts the only
// thing a tagless build can be asked: the hook is a constant false, whatever the environment
// says.
func TestParityMutateInertWithoutTag(t *testing.T) {
	if parityBuild {
		t.Skip("this build carries the parity tag; parity_on_test.go covers it")
	}
	for _, kv := range []string{"KUBECONFIG=/dev/null", "HOME=/x", "", "PATH=/a"} {
		if parityDropsPlanEnv(kv) {
			t.Errorf("a release build dropped %q from the plan — the injector must not exist here", kv)
		}
	}
}

func TestHarnessArgv(t *testing.T) {
	got := harnessArgv("claude", "the-desk", "fable", "demo-the-desk", "/wt")
	want := []string{"claude", "--name", "demo-the-desk", "--model", "fable", "/assay:the-desk"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("claude argv = %q, want %q", got, want)
	}
	got = harnessArgv("codex", "worker-desk", "gpt-5.6-terra", "demo-worker-codex", "/wt")
	want = []string{"codex", "--sandbox", "danger-full-access", "-C", "/wt", "-m", "gpt-5.6-terra",
		`Invoke the "assay:worker-desk" skill now.`}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("codex argv = %q, want %q", got, want)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			b.Write(buf[:n])
			if rerr != nil {
				break
			}
		}
		done <- b.String()
	}()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}
