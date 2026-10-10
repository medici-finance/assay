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

// TestCLILegacyShapeRefusals: each line here (but the two marked) was refused by the pre-migration
// binary with exit 3 and is outside the recorded decision's entries, so it is refused again:
// usage exit 3, no roster echo, nothing on stdout, and the cell's files untouched.
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
		// (the old parser ignored every word after ls, so these two listed CELLS_ROOT's registry
		// with exit 0; applying the selector there would list another one, so they are refused
		// and the compat doc lists that as decision entry 6)
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
		// a lone `-` where the verb goes was an unknown verb, whatever follows it
		{"-"},
		{"-", "ls"},
		{"-", "-", "ls"},
		{"--cells-root", w.cellsRoot, "-", "ls"},
		{"-", "help", "ls"},
		{"-", "version"},
		{"-", "--help"},
		{"-", "cache-run", "child", "x"},
		{"-", "model-policy", "hook", "a", "b", "c", "d", "e", "f"},
		// a flag ahead of a flag-less verb's cell
		{"check", "--bogus", "example"},
		{"deskd", "-x", "example"},
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

// TestCLILegacyWordsRestored: lines where the parser's reading differed from the legacy one
// without a ruling (review C4 and security S3 on #2391) read as they did before: the verbs that
// compared exact words (cadence, cache, comms) see those words again, the flag-less verbs
// (check, deskd) read words by position, a help flag where the verb goes is the usage whatever
// follows, a relative selector is refused first, the hook's blocking exit is the hook line's
// alone, and a repeated --kind keeps its first value.
func TestCLILegacyWordsRestored(t *testing.T) {
	w := newCLIWorld(t, "")
	w.writeHouseEnv(t, "CELL_GO_CACHE=on\nCELL_GO_CACHE_ROOT="+filepath.Join(w.root, "gocache")+"\n")
	logPath := w.logStub(t)
	type want struct {
		code int
		err  string
	}
	cadenceUnexpected := want{3, "cellctl: cadence: unexpected arguments"}
	commsUsage := want{3, "cellctl: comms <cell> check|run|recover --confirm-stopped"}
	cacheUsage := want{3, "cellctl: cache <cell> status|clean|recover --confirm-stopped (status is dry-run)"}
	before := snapshotTree(t, w.root)
	for _, tc := range []struct {
		args []string
		want want
	}{
		{[]string{"cadence", "example", "recover", "the-desk", "--confirm-stopped", "--confirm-stopped"}, cadenceUnexpected},
		{[]string{"cadence", "example", "recover", "the-desk", "--confirm-stopped=true", "--confirm-stopped"}, cadenceUnexpected},
		{[]string{"cadence", "example", "recover", "the-desk", "--confirm-stopped=false"}, cadenceUnexpected},
		{[]string{"cadence", "example", "recover", "the-desk", "--", "--confirm-stopped"}, cadenceUnexpected},
		{[]string{"cadence", "example", "recover", "the-desk", "--confirm-stopped=1", "--", "x"}, cadenceUnexpected},
		{[]string{"cadence", "example", "recover", "--confirm-stopped", "the-desk"}, want{3, `cellctl: cadence: unknown role "--confirm-stopped"`}},
		{[]string{"cadence", "example", "recover", "--confirm-stopped=true", "the-desk"}, want{3, `cellctl: cadence: unknown role "--confirm-stopped=true"`}},
		{[]string{"comms", "example", "recover", "--confirm-stopped", "--confirm-stopped"}, commsUsage},
		{[]string{"comms", "example", "recover", "--", "--confirm-stopped"}, commsUsage},
		{[]string{"comms", "example", "check", "--confirm-stopped"}, commsUsage},
		{[]string{"cache", "example", "recover", "--confirm-stopped", "--confirm-stopped"}, cacheUsage},
		{[]string{"cache", "example", "status", "--confirm-stopped"}, cacheUsage},
		{[]string{"cache", "example", "recover", "--", "--confirm-stopped"}, cacheUsage},
		{[]string{"check", "example", "cfgdir", "--bogus"}, want{3, "cellctl: CLAUDE_CONFIG_DIR not a directory: cfgdir"}},
		{[]string{"check", "example", "-x"}, want{3, "cellctl: CLAUDE_CONFIG_DIR not a directory: -x"}},
		{[]string{"--cells-root", w.cellsRoot, "check", "example", "cfgdir", "--bogus"}, want{3, "cellctl: CLAUDE_CONFIG_DIR not a directory: cfgdir"}},
		{[]string{"check", "example", "-bogus"}, want{3, "cellctl: CLAUDE_CONFIG_DIR not a directory: -bogus"}},
		{[]string{"check", "example", "cfgdir", "--cells-root", w.cellsRoot}, want{3, "cellctl: CLAUDE_CONFIG_DIR not a directory: cfgdir"}},
		{[]string{"deskd", "example", "--bogus"}, want{3, "CELL_ATTENDED=1"}},
		{[]string{"deskd", "example", "-bogus"}, want{3, "CELL_ATTENDED=1"}},
		{[]string{"deskd", "example", "--cells-root=" + w.cellsRoot}, want{3, "CELL_ATTENDED=1"}},
		{[]string{"--cells-root", "relative", "version"}, want{3, selectorRefusal}},
		{[]string{"--cells-root", "relative", "--help"}, want{3, selectorRefusal}},
		{[]string{"--cells-root", "relative", "help", "ls"}, want{3, selectorRefusal}},
		{[]string{"--cells-root", "relative", "model-policy", "bogus"}, want{3, selectorRefusal}},
		{[]string{"--cells-root", "relative", "model-policy"}, want{3, selectorRefusal}},
		{[]string{"--cells-root", "relative", "-", "model-policy", "hook", "a", "b", "c", "d", "e", "f"}, want{3, selectorRefusal}},
		{[]string{"--cells-root", "relative", "model-policy", "hook", "a", "b", "c", "d", "e", "f"}, want{hookBlockExit, ""}},
		{[]string{"show", "scrub", "--kind", "bogus", "--kind", "scrubbed"}, want{3, "--kind must be one of"}},
		{[]string{"desk", "example", "worker-desk", "--kind", "container", "--kind", "house"}, want{3, "container launcher"}},
		{[]string{"up", "example", "--kind", "container", "--kind", "house"}, want{3, "container launcher"}},
		{[]string{"scratch", "nosuch", "sweep", "-h"}, want{3, "no cell 'nosuch'"}},
	} {
		r := w.run(t, []string{"DRY_RUN=1"}, tc.args...)
		if r.Code != tc.want.code || !strings.Contains(r.Stderr, tc.want.err) {
			t.Errorf("%q: exit %d, want %d with %q\nstdout:\n%s\nstderr:\n%s", tc.args, r.Code, tc.want.code, tc.want.err, r.Stdout, r.Stderr)
		}
	}
	if _, err := os.Stat(logPath); err == nil {
		t.Error("a refused line started a harness")
	}
	if after := snapshotTree(t, w.root); after != before {
		t.Errorf("a refused line changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	// A help flag where the verb goes was the usage, exit 0, whatever followed it.
	for _, args := range [][]string{{"-h", "--bogus"}, {"--help", "--nosuch"}, {"--help", "up", "example"}, {"-h", "cache-run", "child", "x"}} {
		if r := w.run(t, nil, args...); r.Code != 0 || !strings.Contains(r.Stdout, "Usage:") || strings.Contains(r.Stderr, echoFragment) {
			t.Errorf("%q: exit %d, want the usage and exit 0\n%s%s", args, r.Code, r.Stdout, r.Stderr)
		}
	}
	// A help flag after a scratch action was that action's flag: its usage on stderr, exit 2.
	for _, args := range [][]string{{"scratch", "example", "sweep", "-h"}, {"scratch", "example", "sweep", "--apply", "--help"}} {
		if r := w.run(t, nil, args...); r.Code != 2 || r.Stdout != "" || !strings.Contains(r.Stderr, "Usage:") {
			t.Errorf("%q: exit %d, want the action usage on stderr and exit 2\nstdout:\n%s\nstderr:\n%s", args, r.Code, r.Stdout, r.Stderr)
		}
	}
	// The first --kind is the one read; a later one is skipped unread, as the legacy pre-scan did.
	if r := w.run(t, nil, "show", "scrub", "--kind", "scrubbed", "--kind", "bogus"); r.Code != 0 || !strings.Contains(r.Stdout, "CELL_KIND=scrubbed (flag)") {
		t.Errorf("show --kind scrubbed --kind bogus: exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	// The exact confirmation still records, in both spellings (decision entry 4).
	for _, last := range []string{"--confirm-stopped", "--confirm-stopped=true"} {
		if r := w.run(t, nil, "cadence", "example", "recover", "the-desk", last); r.Code != 0 || !strings.Contains(r.Stdout, "the-desk recover recorded") {
			t.Errorf("cadence recover the-desk %s: exit %d\n%s%s", last, r.Code, r.Stdout, r.Stderr)
		}
	}
}

// TestCLIWholeLineScans: the old parser read two words by scanning the verb's whole line before
// its flag loop, so a word in another flag's value position was read twice: the first --kind on
// desk, up and show (applied and validated before the cell loads), and -h/--help on new (usage,
// exit 0, nothing written). The parity transcripts (cases kind-whole-line, new-trailing-value)
// pin the non-help lines against the old binary; help text is not transcribed, so new's help
// lines are pinned here, with the kind lines again for the mutation harness.
func TestCLIWholeLineScans(t *testing.T) {
	w := newCLIWorld(t, "")
	n := []string{"new", "fresh", "--kind", "house", "--repo", w.repoDir, "--roots", "o/r=" + w.repoDir}
	before := snapshotTree(t, w.root)
	for _, args := range [][]string{
		append(append([]string{}, n...), "--orgs", "--help"),
		append(append([]string{}, n...), "--forge", "-h"),
		append(append([]string{}, n...), "--port", "--help"),
		append(append([]string{}, n...), "--roles", "-h"),
		append(append([]string{}, n...), "--", "-h"),
		{"new", "--orgs", "--help"},
		{"new", "--forge", "-h"},
		// A --cells-root after the verb is refused on any other line; on a help line new's whole-line
		// read comes first, as it did before the migration.
		{"new", "nx", "--cells-root", filepath.Join(w.root, "cells"), "--help"},
	} {
		if r := w.run(t, nil, args...); r.Code != 0 || !strings.Contains(r.Stdout, "Usage:") {
			t.Errorf("%q: exit %d, want new's usage and exit 0\nstdout:\n%s\nstderr:\n%s", args, r.Code, r.Stdout, r.Stderr)
		}
	}
	if after := snapshotTree(t, w.root); after != before {
		t.Errorf("a help line on new wrote files:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	dry := []string{"DRY_RUN=1"}
	type want struct {
		code int
		out  string
		err  string
	}
	for _, tc := range []struct {
		args []string
		want want
	}{
		{[]string{"desk", "example", "worker-desk", "--kind", "house", "--kind"}, want{0, "kind=house (override)", ""}},
		{[]string{"up", "example", "--kind", "house", "--kind"}, want{0, "kind=house (override)", ""}},
		{[]string{"show", "example", "--kind", "house", "--kind"}, want{0, "CELL_KIND=house (flag)", ""}},
		{[]string{"desk", "example", "worker-desk", "--model", "--kind"}, want{3, "", "--kind needs a value"}},
		{[]string{"up", "example", "--provider", "--kind"}, want{3, "", "--kind needs a value"}},
		{[]string{"show", "example", "--model", "--kind"}, want{3, "", "--kind needs a value"}},
		{[]string{"desk", "example", "worker-desk", "--model", "--kind", "bogus"}, want{3, "", "--kind must be one of"}},
		{[]string{"desk", "scrub", "worker-desk", "--model", "--kind", "house", w.cfgDir}, want{0, "kind=house (override)", ""}},
		{[]string{"new", "f1", "--kind", "house", "--repo", w.repoDir, "--roots", "o/r=" + w.repoDir, "--forge"}, want{0, "scaffolded", ""}},
		{[]string{"new", "f2", "--forge"}, want{3, "", "--forge must be github or gitlab, got ''"}},
		{[]string{"scratch", "--", "example", "sweep"}, want{3, "", "no cell '--'"}},
	} {
		r := w.run(t, dry, tc.args...)
		if r.Code != tc.want.code || !strings.Contains(r.Stdout, tc.want.out) || !strings.Contains(r.Stderr, tc.want.err) {
			t.Errorf("%q: exit %d, want %d with stdout %q and stderr %q\nstdout:\n%s\nstderr:\n%s", tc.args, r.Code, tc.want.code, tc.want.out, tc.want.err, r.Stdout, r.Stderr)
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
	// An empty word where the verb goes printed the usage, exit 0, before the migration; it still
	// does, and the next word is not run as the verb.
	for _, args := range [][]string{{"", "show", "example"}, {"", "ls"}, {"--cells-root", w.cellsRoot, "", "ls"}} {
		if r := w.run(t, nil, args...); r.Code != 0 || !strings.Contains(r.Stdout, "Usage:") || strings.Contains(r.Stdout, "example\n") || strings.Contains(r.Stderr, echoFragment) {
			t.Errorf("%q: exit %d, want the usage and exit 0 with nothing run\n%s%s", args, r.Code, r.Stdout, r.Stderr)
		}
	}
	// scratch read its flags with the Go flag package, so its single-dash spellings still work.
	for _, args := range [][]string{
		{"scratch", "example", "sweep", "-apply"},
		{"--cells-root", w.cellsRoot, "scratch", "example", "sweep", "-apply=true"},
	} {
		if r := w.run(t, nil, args...); r.Code != 0 || !strings.Contains(r.Stdout, `"apply":true`) {
			t.Errorf("%v: exit %d\n%s%s", args, r.Code, r.Stdout, r.Stderr)
		}
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
		// Directly after the verb it is the verb's first word too, never a selector: cache-run is
		// asked to run a program named --cells-root, and raw-probe does not run.
		_ = os.Remove(out)
		r = w.run(t, nil, "cache-run", "--cells-root", w.cellsRoot, "raw-probe", "x")
		if got := readFileOrEmpty(out); r.Code == 0 || len(got) != 0 {
			t.Errorf("cache-run --cells-root <abs> raw-probe: exit %d, child argv %q: a word after the verb was taken as the selector\n%s", r.Code, got, r.Stderr)
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
