package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLINonHelpParity compares the cellctl binary under test with transcripts captured from
// the PRE-migration Go binary (the commit recorded in the golden file), through the fixture
// world's DRY_RUN and file-only cells. It is deliberately not the shell parity script: that
// script defaults BOTH sides to the shell oracle, so a bare run proves nothing about the Go
// migration. Help output is excluded by design (help is generated from the command tree now);
// every case here is a non-help command whose stdout, stderr and exit code are contracts.
//
// Recording (only when the old binary is available) is explicit:
//
//	CELLCTL_GOLDEN_RECORD=/path/to/pre-migration/cellctl CELLCTL_GOLDEN_BASE=<sha> \
//	  go test ./cmd/cellctl -run '^TestCLINonHelpParity$'
//
// Cases whose behavior was deliberately changed are NOT here; they are listed with their reason
// in the compatibility table (docs/cellctl.md) and pinned by TestCLILegacyForms and
// TestCLICompatChanges.

const goldenPath = "testdata/cli-nonhelp-golden.json"

type goldenStep struct {
	env  []string
	args []string
}

type goldenCase struct {
	name   string
	policy bool // install the shipped example model policy first
	extra  string
	setup  func(t *testing.T, w *cliWorld)
	steps  []goldenStep
}

func st(args ...string) goldenStep { return goldenStep{args: args} }
func stEnv(env string, args ...string) goldenStep {
	return goldenStep{env: strings.Fields(env), args: args}
}

var dry = "DRY_RUN=1"

// goldenCases is the retained non-help surface. Names are stable keys in the golden file.
func goldenCases() []goldenCase {
	return []goldenCase{
		{name: "ls", steps: []goldenStep{st("ls")}},
		{name: "version-flag", steps: []goldenStep{st("--version")}},
		{name: "version-verb", steps: []goldenStep{st("version")}},
		{name: "unknown-verb", steps: []goldenStep{st("frobnicate")}},
		{name: "missing-cell", steps: []goldenStep{st("desk"), st("show"), st("status"), st("up"), st("down"), st("set"), st("cadence")}},
		{name: "unknown-cell", steps: []goldenStep{st("show", "nope"), st("status", "nope"), stEnv(dry, "desk", "nope", "worker-desk")}},
		{name: "status", steps: []goldenStep{st("status", "example"), st("status", "scrub")}},
		{name: "deskd-needs-attended", steps: []goldenStep{st("deskd", "example")}},
		{name: "show", steps: []goldenStep{
			st("show", "example"),
			st("show", "example", "--model", "opus"),
			st("show", "example", "--harness", "codex", "--cockpit", "tmux"),
			st("show", "example", "--provider", "kimi"),
			st("show", "example", "--kind", "house"),
			st("show", "scrub"),
		}},
		{name: "show-refusals", steps: []goldenStep{
			st("show", "example", "--harness", "gemini"),
			st("show", "example", "--cockpit", "vim"),
			st("show", "example", "--kind", "bogus"),
			st("show", "example", "--model"),
			st("show", "example", "stray"),
		}},
		{name: "desk-house", extra: "DESK_MODEL_worker_desk=sonnet\nDESK_MODEL_the_desk=fable\nDESK_MODEL_DEFAULT=opus\n", steps: []goldenStep{
			stEnv(dry, "desk", "example", "worker-desk"),
			stEnv(dry, "desk", "example", "the-desk"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", "haiku"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", "-dash-leading"),
			stEnv(dry+" DESK_MODEL_OVERRIDE=envmodel", "desk", "example", "worker-desk"),
			stEnv(dry+" DESK_MODEL_OVERRIDE=envmodel", "desk", "example", "worker-desk", "--model", "flagmodel"),
			stEnv(dry, "desk", "example", "worker-desk", "--provider", "kimi"),
			stEnv(dry, "desk", "example", "worker-desk", "--provider", "glm", "--model", "glm-x"),
			stEnv(dry, "desk", "example", "worker-desk", "--harness", "codex"),
			stEnv(dry, "desk", "example", "worker-desk", "--cockpit", "tmux"),
			stEnv(dry, "desk", "example", "worker-desk", "--cadence", "5m", "--tick-budget", "2m"),
			stEnv(dry, "desk", "example", "worker-desk", "--cadence", "off"),
			stEnv(dry, "desk", "example", "worker-desk", "<ROOT>/claude-config"),
			stEnv(dry, "desk", "example", "worker-desk", "--kind", "house"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", "one", "--model", "two"),
		}},
		{name: "desk-refusals", steps: []goldenStep{
			st("desk", "example"),
			stEnv(dry, "desk", "example", "no-such-desk"),
			stEnv(dry, "desk", "example", "worker-desk", "--harness", "gemini"),
			stEnv(dry, "desk", "example", "worker-desk", "--cockpit", "vim"),
			stEnv(dry, "desk", "example", "worker-desk", "--kind", "bogus"),
			stEnv(dry, "desk", "example", "worker-desk", "--set"),
			stEnv(dry, "desk", "example", "worker-desk", "--model"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", ""),
			stEnv(dry, "desk", "example", "worker-desk", "--provider", "nosuchprovider"),
			stEnv(dry, "desk", "scrub", "worker-desk", "--provider", "kimi"),
			stEnv(dry, "desk", "scrub", "worker-desk", "<ROOT>/claude-config"),
			stEnv(dry, "desk", "scrub", "not-enabled-role"),
		}},
		{name: "desk-set-round-trip", steps: []goldenStep{
			stEnv(dry, "desk", "example", "worker-desk", "--model", "persisted", "--set"),
			st("show", "example"),
			stEnv(dry, "desk", "example", "worker-desk"),
			stEnv(dry, "desk", "example", "worker-desk", "--harness", "codex", "--model", "cx", "--set"),
			st("show", "example"),
		}},
		{name: "desk-scrubbed", steps: []goldenStep{
			stEnv(dry, "desk", "scrub", "the-desk"),
			stEnv(dry, "desk", "scrub", "worker-desk", "--model", "sonnet"),
			stEnv(dry, "desk", "scrub", "worker-desk", "--harness", "codex"),
		}},
		{name: "desk-policy", policy: true, steps: []goldenStep{
			stEnv(dry, "desk", "example", "pr-review-desk"),
			stEnv(dry, "desk", "example", "worker-desk"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", "glm-5.3-flash[1m]"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", "x", "--set"),
			stEnv(dry, "desk", "example", "intake-desk"),
			st("show", "example"),
		}},
		{name: "up", steps: []goldenStep{
			stEnv(dry, "up", "example"),
			stEnv(dry, "up", "example", "--no-the-desk"),
			stEnv(dry, "up", "example", "--no-attach", "--model", "haiku"),
			stEnv(dry, "up", "example", "--cockpit", "tmux"),
			stEnv(dry, "up", "example", "--provider", "kimi", "--harness", "codex"),
			stEnv(dry, "up", "example", "--automate", "daily"),
			stEnv(dry, "up", "example", "--harness", "gemini"),
			stEnv(dry, "up", "example", "--cockpit", "vim"),
			stEnv(dry, "up", "example", "--set"),
			stEnv(dry, "up", "scrub"),
		}},
		{name: "down", steps: []goldenStep{
			stEnv(dry, "down", "example"),
			stEnv(dry, "down", "example", "--keep-deskd"),
			stEnv(dry, "down", "example", "--cockpit", "tmux"),
			stEnv(dry, "down", "example", "--cockpit", "vim"),
			stEnv(dry, "down", "scrub", "--keep-deskd"),
		}},
		{name: "set", steps: []goldenStep{
			st("set", "example", "DESK_MODEL_DEFAULT=opus"),
			st("set", "example", "DESK_MODEL_DEFAULT=sonnet", "CELL_COCKPIT=tmux"),
			st("show", "example"),
			st("set", "example", "worker-desk", "--model", "haiku"),
			st("set", "example", "worker-desk", "--harness", "codex", "--model", "gpt-x"),
			st("set", "example", "--cockpit", "tmux", "--provider", "kimi"),
			st("set", "example", "--harness", "codex"),
			st("show", "example"),
			st("set", "example", "NOT_A_KNOWN_KEY=1"),
			st("set", "example", "NOT_A_KNOWN_KEY=1", "--force"),
			st("show", "example"),
		}},
		{name: "set-refusals", steps: []goldenStep{
			st("set", "example"),
			st("set", "example", "worker-desk"),
			st("set", "example", "worker-desk", "--model", "m", "A=b"),
			st("set", "example", "worker-desk", "the-desk", "--model", "m"),
			st("set", "example", "--model", "m"),
			st("set", "example", "notakv"),
			st("set", "example", "--kind", "bogus"),
			st("set", "example", "--cockpit", "vim"),
			st("set", "example", "worker-desk", "--harness", "gemini", "--model", "m"),
			st("set", "example", "--harness"),
		}},
		{name: "new-house", steps: []goldenStep{
			st("new", "fresh", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo"),
			st("show", "fresh"),
			st("new", "fresh", "--kind", "house", "--repo", "<ROOT>/repo"),
			st("ls"),
		}},
		{name: "new-refusals", steps: []goldenStep{
			st("new"),
			st("new", "x", "--kind", "bogus"),
			st("new", "x", "extra"),
			st("new", "x", "--kind", "scrubbed"),
			st("new", "x", "--kind", "scrubbed", "--repo", "<ROOT>/repo"),
			st("new", "x", "--kind", "scrubbed", "--repo", "<ROOT>/repo", "--repo-slug", "bad slug"),
			st("new", "x", "--kind", "container"),
			st("new", "x", "--container-config", "<ROOT>/none.json"),
			st("new", "x", "--forge", "bogus"),
		}},
		{name: "check", steps: []goldenStep{st("check", "example"), st("check", "example", "<ROOT>/claude-config"), st("check", "scrub")}},
		{name: "cadence", steps: []goldenStep{
			st("cadence", "example", "status"),
			st("cadence", "example", "stop", "worker-desk"),
			st("cadence", "example", "status", "worker-desk"),
			st("cadence", "example", "resume", "worker-desk"),
			st("cadence", "example", "recover", "worker-desk"),
			st("cadence", "example", "recover", "worker-desk", "--confirm-stopped"),
			st("cadence", "example", "status", "no-role"),
			st("cadence", "example", "bogus"),
			st("cadence", "example", "stop", "worker-desk", "--confirm-stopped"),
			st("cadence", "example"),
			st("cadence", "scrub", "status"),
		}},
		{name: "cache", steps: []goldenStep{
			st("cache", "example", "status"),
			st("cache", "example"),
			st("cache", "example", "recover", "--confirm-stopped"),
		}},
		{name: "cache-enabled", extra: "CELL_GO_CACHE=on\nCELL_GO_CACHE_ROOT=<ROOT>/gocache\n", steps: []goldenStep{
			st("cache", "example"),
			st("cache", "example", "bogus"),
			st("cache", "example", "recover"),
			st("cache", "example", "status"),
			st("cache", "example", "clean"),
			st("cache", "example", "recover", "--confirm-stopped"),
		}},
		{name: "comms", steps: []goldenStep{
			st("comms", "example", "check"),
			st("comms", "example"),
			st("comms", "example", "bogus"),
			st("comms", "example", "recover"),
		}},
		{name: "scratch", steps: []goldenStep{
			st("scratch", "example"),
			st("scratch", "example", "bogus"),
			st("scratch", "example", "sweep"),
			st("scratch", "example", "sweep", "extra"),
			st("scratch", "example", "inventory"),
			st("scratch", "example", "inventory", "--path", "<ROOT>/nowhere"),
			st("scratch", "example", "ack"),
			st("scratch", "example", "run"),
			st("scratch", "example", "run", "--source", "<ROOT>/repo"),
			st("scratch", "example", "run", "--source", "<ROOT>/repo", "--", "true"),
			st("scratch", "example", "sweep", "--max-age", "bogus"),
			st("scratch", "example", "sweep", "--max-bytes", "-1"),
		}},
		{name: "providers", steps: []goldenStep{
			st("providers"),
			st("providers", "init"),
			st("providers", "init"),
			st("providers", "bogus"),
		}},
		{name: "internal-entrypoints", steps: []goldenStep{
			st("container-run"),
			st("container-run", "example", "worker-desk", "claude", "m"),
			st("cache-run"),
			st("model-policy"),
			st("model-policy", "hook", "relative", "worker-desk", "anthropic", "", "claude", "abc"),
			st("model-policy", "hook", "<ROOT>/cells/example", "worker-desk", "anthropic", "", "claude", ""),
			st("model-policy", "hook", "<ROOT>/cells/example", "worker-desk", "anthropic", "", "claude", "abc"),
			st("cache-run", "true"),
			st("cache-run", "sh", "-c", "exit 7"),
		}},
		{name: "cells-root", steps: []goldenStep{
			st("--cells-root", "<ROOT>/cells", "ls"),
			st("--cells-root", "relative", "ls"),
			st("--cells-root", "<ROOT>/cells"),
			st("--cells-root"),
			st("--cells-root", "<ROOT>/other", "ls"),
			st("--cells-root", "<ROOT>/cells", "show", "example"),
		}},
	}
}

type goldenFile struct {
	BaseSHA string              `json:"base_sha"`
	Note    string              `json:"note"`
	Cases   map[string][]cliRun `json:"cases"`
}

// runGoldenCase executes one case against bin in a fresh world and returns normalized results.
func runGoldenCase(t *testing.T, bin string, c goldenCase) []cliRun {
	t.Helper()
	extra := c.extra
	w := newCLIWorld(t, "")
	extra = strings.ReplaceAll(extra, "<ROOT>", w.root)
	if extra != "" {
		w.writeHouseEnv(t, extra)
	}
	if c.policy {
		w.installPolicy(t)
	}
	if c.setup != nil {
		c.setup(t, w)
	}
	var out []cliRun
	for _, s := range c.steps {
		args := make([]string, len(s.args))
		for i, a := range s.args {
			args[i] = strings.ReplaceAll(a, "<ROOT>", w.root)
		}
		env := make([]string, len(s.env))
		for i, e := range s.env {
			env[i] = strings.ReplaceAll(e, "<ROOT>", w.root)
		}
		r := w.runBin(t, bin, env, args...)
		r.Args = append([]string(nil), s.args...)
		if len(s.env) > 0 {
			r.Args = append([]string{"env:" + strings.Join(s.env, ",")}, r.Args...)
		}
		r.Stdout, r.Stderr = w.norm(r.Stdout), w.norm(r.Stderr)
		out = append(out, r)
	}
	return out
}

// deliberateDiff is one reviewed difference from the pre-migration transcript: the new exit code
// and the exact new stderr (stdout is still required to match). The reason is the compatibility
// report's row (docs/cellctl-cli-compat.md); an entry that stops differing is itself a failure.
type deliberateDiff struct {
	code   int
	stderr string
	why    string
}

const (
	whyNoEcho    = "help, --version, version and parse failures print no roster echo: nothing acts, so there is no effective configuration to show"
	whyParseText = "a malformed command line is now refused by the parser with its own wording and the usage exit (3)"
)

var deliberateDiffs = map[string]deliberateDiff{
	"version-flag/0":  {0, "", whyNoEcho},
	"version-verb/0":  {0, "", whyNoEcho},
	"unknown-verb/0":  {3, "cellctl: unknown command \"frobnicate\" for \"cellctl\"\nRun 'cellctl --help' for usage.\n", whyParseText},
	"show-refusals/3": {3, "cellctl: flag needs an argument: --model\nRun 'cellctl --help' for usage.\n", whyParseText},
	"desk-refusals/6": {3, "cellctl: flag needs an argument: --model\nRun 'cellctl --help' for usage.\n", whyParseText},
	"set-refusals/9":  {3, "cellctl: flag needs an argument: --harness\nRun 'cellctl --help' for usage.\n", whyParseText},
	"scratch/10":      {3, "cellctl: invalid argument \"bogus\" for \"--max-age\" flag: time: invalid duration \"bogus\"\nRun 'cellctl --help' for usage.\n", whyParseText + "; scratch's old flag-package exit 2 becomes 3"},
	"cells-root/3":    {3, "cellctl: flag needs an argument: --cells-root\nRun 'cellctl --help' for usage.\n", whyParseText},
}

func TestCLINonHelpParity(t *testing.T) {
	cases := goldenCases()
	if rec := os.Getenv("CELLCTL_GOLDEN_RECORD"); rec != "" {
		g := goldenFile{
			BaseSHA: os.Getenv("CELLCTL_GOLDEN_BASE"),
			Note:    "Transcripts of the pre-migration Go cellctl at base_sha over the fixture world (cli_world_test.go). Regenerate only from that binary.",
			Cases:   map[string][]cliRun{},
		}
		for _, c := range cases {
			g.Cases[c.name] = runGoldenCase(t, rec, c)
		}
		raw, err := json.MarshalIndent(g, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skipf("recorded %d cases from %s; not a comparison", len(cases), rec)
	}
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden transcripts missing: %v", err)
	}
	var g goldenFile
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	if g.BaseSHA == "" || len(g.Cases) == 0 {
		t.Fatal("golden file has no base_sha or no cases")
	}
	for _, c := range cases {
		want, ok := g.Cases[c.name]
		if !ok {
			t.Errorf("case %s has no recorded transcript", c.name)
			continue
		}
		got := runGoldenCase(t, cellctlBinary(t), c)
		got = append([]cliRun(nil), got...)
		want = append([]cliRun(nil), want...)
		for i := range got {
			got[i].Stdout, got[i].Stderr = hostNorm(got[i].Stdout), hostNorm(got[i].Stderr)
		}
		for i := range want {
			want[i].Stdout, want[i].Stderr = hostNorm(want[i].Stdout), hostNorm(want[i].Stderr)
		}
		if len(got) != len(want) {
			t.Errorf("case %s: %d steps recorded, %d run", c.name, len(want), len(got))
			continue
		}
		for i := range want {
			key := fmt.Sprintf("%s/%d", c.name, i)
			if d, ok := deliberateDiffs[key]; ok {
				same := got[i].Code == want[i].Code && got[i].Stdout == want[i].Stdout && got[i].Stderr == want[i].Stderr
				if same {
					t.Errorf("%s is listed as a deliberate difference but matches the pre-migration transcript: drop the entry", key)
				}
				if got[i].Code != d.code || got[i].Stdout != want[i].Stdout || got[i].Stderr != d.stderr {
					t.Errorf("%s %v: the deliberate difference (%s) no longer holds\n  exit   want %d got %d\n  stdout want %q got %q\n  stderr want %q\n         got  %q",
						key, want[i].Args, d.why, d.code, got[i].Code, want[i].Stdout, got[i].Stdout, d.stderr, got[i].Stderr)
				}
				continue
			}
			if got[i].Code != want[i].Code || got[i].Stdout != want[i].Stdout || got[i].Stderr != want[i].Stderr {
				t.Errorf("case %s step %d %v diverges from the pre-migration transcript (%s)\n"+
					"  exit   want %d got %d\n  stdout want %q\n         got  %q\n  stderr want %q\n         got  %q",
					c.name, i, want[i].Args, g.BaseSHA, want[i].Code, got[i].Code,
					want[i].Stdout, got[i].Stdout, want[i].Stderr, got[i].Stderr)
			}
		}
	}
	if len(g.Cases) != len(cases) {
		t.Errorf("golden file holds %d cases, the table has %d: regenerate or remove the stale ones", len(g.Cases), len(cases))
	}
}
