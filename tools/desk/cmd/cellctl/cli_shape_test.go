package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// The command-line SHAPE the pre-migration parser accepted: an optional `--cells-root <abs>`
// first, then the verb, then the verb's fixed positionals, and only then flags. The recorded
// decision accepts four differences from that shape and no more, so every other line it refused
// must still be refused. These tests pin the refusals; cli_nonhelp_parity_test.go pins them
// against the recorded transcripts of the old binary.

// TestScratchRunFlagsBeforeVerb is the verb-not-first sibling of TestScratchRunArgvBoundary. A
// value-taking `scratch` flag written BEFORE the verb used to be an unknown verb (exit 3). If the
// parser accepted it, its separated value would move where the supervised command starts, and
// cellctl would parse the command's own words. Every such line must be refused with nothing run.
func TestScratchRunFlagsBeforeVerb(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the argv probe is a POSIX shell script")
	}
	w := newCLIWorld(t, "")
	srcA, srcB := filepath.Join(w.root, "src-a"), filepath.Join(w.root, "src-b")
	commitFixtureRepo(t, srcA, "a\n")
	commitFixtureRepo(t, srcB, "b\n")
	other := filepath.Join(w.root, "other-registry")
	out := filepath.Join(w.root, "probe.out")
	probe := "#!/bin/sh\n{ printf 'source=%s\\n' \"$ASSAY_SOURCE_ROOT\"; for a in \"$@\"; do printf 'arg=%s\\n' \"$a\"; done; } > " + out + "\n"
	if err := os.WriteFile(filepath.Join(w.binDir, "argv-probe"), []byte(probe), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(pre []string, rest ...string) []string {
		return append(append(append([]string{}, pre...), "scratch", "example", "run"), rest...)
	}
	cases := []struct {
		name string
		args []string
	}{
		{"source", run([]string{"--source", srcA}, "--session", "s", "--task", "t", "argv-probe", "--source", srcB, "--snapshot", "x")},
		{"session-help", run([]string{"--session", "s"}, "--source", srcA, "--task", "t", "argv-probe", "--help")},
		{"session-short-help", run([]string{"--session", "s"}, "--source", srcA, "--task", "t", "argv-probe", "-h")},
		{"source-selector", run([]string{"--source", srcA}, "--session", "s", "--task", "t", "argv-probe", "--cells-root", other, "x")},
		{"task", run([]string{"--task", "t"}, "--source", srcA, "--session", "s", "argv-probe", "--task", "hijack", "--apply", "x")},
		{"id", run([]string{"--id", "i"}, "--source", srcA, "--session", "s", "--task", "t", "argv-probe", "--source", srcB, "x")},
		{"receipt", run([]string{"--receipt", "r"}, "--source", srcA, "--session", "s", "--task", "t", "argv-probe", "--source", srcB, "x")},
		{"single-dash-source", run([]string{"-source", srcA}, "--session", "s", "--task", "t", "argv-probe", "--source", srcB, "x")},
		{"selector-then-source", run([]string{"--cells-root", w.cellsRoot, "--source", srcA}, "--session", "s", "--task", "t", "argv-probe", "--source", srcB, "x")},
		{"equals-source", run([]string{"--source=" + srcA}, "--session", "s", "--task", "t", "argv-probe", "--source", srcB, "x")},
		{"bool-snapshot", run([]string{"--snapshot"}, "--source", srcA, "--session", "s", "--task", "t", "argv-probe", "x")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Remove(out)
			r := w.run(t, nil, tc.args...)
			if r.Code != 3 {
				t.Errorf("cellctl %v: exit %d, want the usage refusal 3\nstdout:\n%s\nstderr:\n%s", tc.args, r.Code, r.Stdout, r.Stderr)
			}
			if raw, err := os.ReadFile(out); err == nil {
				t.Errorf("cellctl %v: the supervised command ran; it saw\n%s", tc.args, raw)
			}
			if strings.Contains(r.Stdout, "Usage:") || strings.Contains(r.Stderr, echoFragment) {
				t.Errorf("cellctl %v: printed help or the roster echo for a refused line\nstdout:\n%s\nstderr:\n%s", tc.args, r.Stdout, r.Stderr)
			}
		})
	}
}

// TestCLILegacyShapeRefusals: each line here was refused by the pre-migration binary with exit 3
// and is outside the recorded decision's four entries, so it is refused again: usage exit 3, no
// roster echo, nothing on stdout, and the cell's files untouched.
func TestCLILegacyShapeRefusals(t *testing.T) {
	w := newCLIWorld(t, "")
	other := filepath.Join(w.root, "other-registry")
	if err := os.MkdirAll(filepath.Join(other, "elsewhere"), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := w.logStub(t)
	before := snapshotTree(t, w.root)
	for _, args := range [][]string{
		// a flag before the verb
		{"--harness", "codex", "show", "example"},
		{"--model", "m", "show", "example"},
		{"--apply=true", "scratch", "example", "sweep"},
		{"--max-age", "1h", "scratch", "example", "sweep"},
		{"--cells-root", w.cellsRoot, "--model", "m", "show", "example"},
		// the selector: once, before the verb, separated form only
		{"--cells-root", w.cellsRoot, "--cells-root", other, "show", "example"},
		{"--cells-root", w.cellsRoot, "show", "example", "--cells-root", other},
		{"--cells-root", w.cellsRoot, "down", "example", "--cells-root", other},
		{"show", "example", "--cells-root", other},
		{"ls", "--cells-root", w.cellsRoot},
		{"ls", "--cells-root=" + w.cellsRoot},
		{"--cells-root=" + w.cellsRoot, "ls"},
		{"-cells-root", w.cellsRoot, "ls"},
		{"-cells-root=" + w.cellsRoot, "ls"},
		// a flag ahead of a fixed positional
		{"scratch", "example", "--apply", "sweep"},
		{"cadence", "example", "--confirm-stopped", "recover", "worker-desk"},
		{"desk", "example", "--model", "early", "worker-desk"},
		{"show", "--harness", "codex", "example"},
		// single-dash long flags outside scratch, and version with company
		{"desk", "example", "worker-desk", "-model", "dash"},
		{"show", "example", "-harness", "codex"},
		{"-version"},
		{"version", "extra"},
		{"--version", "extra"},
		// help names one command or none
		{"help", "__complete"},
		{"help", "completion"},
		{"help", "desk", "extra"},
	} {
		r := w.run(t, []string{"DRY_RUN=1"}, args...)
		if r.Code != 3 || r.Stdout != "" || strings.Contains(r.Stderr, echoFragment) {
			t.Errorf("%v: exit %d, want the usage refusal 3 with no output and no roster echo\nstdout:\n%s\nstderr:\n%s", args, r.Code, r.Stdout, r.Stderr)
		}
	}
	if _, err := os.Stat(logPath); err == nil {
		t.Error("a refused line started a harness")
	}
	if after := snapshotTree(t, w.root); after != before {
		t.Errorf("a refused line changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	// The hook verb fails closed behind every selector spelling, as before: whatever refuses the
	// line, the hook's exit is the blocking 2.
	for _, pre := range [][]string{{"-cells-root", "relative"}, {"--cells-root=relative"}, {"--cells-root", w.cellsRoot, "--cells-root", other}} {
		args := append(append([]string{}, pre...), "model-policy", "hook", "a", "b", "c", "d", "e", "f")
		if r := w.run(t, nil, args...); r.Code != hookBlockExit {
			t.Errorf("%v: exit %d, want the blocking %d", args, r.Code, hookBlockExit)
		}
	}
}

// TestCLILegacyShapeAccepted: the shapes the old parser accepted still run, the four ruled
// entries included, so the refusals above are not a blanket.
func TestCLILegacyShapeAccepted(t *testing.T) {
	w := newCLIWorld(t, "")
	elsewhere := []string{"CELLS_ROOT=" + filepath.Join(w.root, "elsewhere")}
	if r := w.run(t, elsewhere, "--cells-root", w.cellsRoot, "ls"); r.Code != 0 || !strings.Contains(r.Stdout, "example") {
		t.Errorf("--cells-root <abs> ls: exit %d %q\n%s", r.Code, r.Stdout, r.Stderr)
	}
	if r := w.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk", "--model=eq", "--"); r.Code != 0 || !strings.Contains(r.Stdout, "model=eq (override)") {
		t.Errorf("--flag=value and a bare --: exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	if r := w.run(t, nil, "show", "--", "example"); r.Code != 0 || !strings.Contains(r.Stdout, "[show] cell=example") {
		t.Errorf("a bare -- before the positionals: exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	if r := w.run(t, nil, "show", "example", "--harness", "codex", "--cockpit", "tmux"); r.Code != 0 || !strings.Contains(r.Stdout, "CELL_HARNESS=codex (flag)") {
		t.Errorf("flags after the positionals: exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	// An empty first word printed the usage and exited 0 before the migration; it still does.
	if r := w.run(t, nil, "", "show", "example"); r.Code != 0 || !strings.Contains(r.Stdout, "Usage:") {
		t.Errorf("empty first word: exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	// A selector AFTER a raw verb is that verb's own word, as it always was: it reaches the child.
	if runtime.GOOS != "windows" {
		out := filepath.Join(w.root, "raw-probe.out")
		probe := "#!/bin/sh\n{ for a in \"$@\"; do printf 'arg=%s\\n' \"$a\"; done; } > " + out + "\n"
		if err := os.WriteFile(filepath.Join(w.binDir, "raw-probe"), []byte(probe), 0o755); err != nil {
			t.Fatal(err)
		}
		r := w.run(t, nil, "--cells-root", w.cellsRoot, "cache-run", "raw-probe", "--cells-root", "/nope", "x")
		if got := string(readFileOrEmpty(out)); r.Code != 0 || got != "arg=--cells-root\narg=/nope\narg=x\n" {
			t.Errorf("selector after cache-run: exit %d, child argv %q\n%s", r.Code, got, r.Stderr)
		}
	}
}

// leadingArgsGap is the class guard for the shape check: every command of the tree either takes
// its argv verbatim (DisableFlagParsing) or declares how many fixed positionals it reads before
// its flags. A command that does neither would let a flag stand in for its cell or action.
func leadingArgsGap(root *cobra.Command) []string {
	var bad []string
	for _, c := range root.Commands() {
		if c.DisableFlagParsing {
			continue
		}
		if _, ok := c.Annotations[leadingArgsKey]; !ok {
			bad = append(bad, c.Name())
		}
	}
	return bad
}

func TestCLIEveryVerbDeclaresItsPositionals(t *testing.T) {
	if bad := leadingArgsGap(buildRoot()); len(bad) != 0 {
		t.Errorf("verbs with no declared fixed positionals: %v", bad)
	}
	planted := buildRoot()
	planted.AddCommand(&cobra.Command{Use: "plant <cell>", Run: func(*cobra.Command, []string) {}})
	if bad := leadingArgsGap(planted); !reflect.DeepEqual(bad, []string{"plant"}) {
		t.Errorf("planted verb without a declared count: guard reported %v, want [plant]", bad)
	}
}
