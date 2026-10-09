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
// Cases whose behavior was deliberately changed are recorded here too and listed in
// deliberateDiffs with their reason; each is written up in the compatibility table
// (docs/cellctl-cli-compat.md), which TestCLICompatDoc requires, and a listed difference that
// stops differing fails this test, so the table cannot go stale in either direction.

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
		{name: "completion-entrypoints", steps: []goldenStep{
			st("__complete", "desk", "x"),
			st("__completeNoDesc", "desk"),
			st("--cells-root", "<ROOT>/cells", "__complete", "x"),
		}},
		{name: "flags-before-positionals", steps: []goldenStep{
			st("scratch", "example", "--apply", "sweep"),
			st("cadence", "example", "--confirm-stopped", "recover", "worker-desk"),
			stEnv(dry, "desk", "example", "--model", "early", "worker-desk"),
			st("show", "--harness", "codex", "example"),
		}},
		{name: "single-dash-outside-scratch", steps: []goldenStep{
			stEnv(dry, "desk", "example", "worker-desk", "-model", "dash"),
			st("show", "example", "-harness", "codex"),
			st("-version"),
			st("version", "extra"),
		}},
		{name: "cells-root-spellings", steps: []goldenStep{
			st("--cells-root=<ROOT>/cells", "ls"),
			st("-cells-root", "<ROOT>/cells", "ls"),
			st("--cells-root=<ROOT>/cells", "cache-run", "true"),
			st("--cells-root=<ROOT>/cells", "model-policy", "hook", "relative", "worker-desk", "anthropic", "", "claude", "abc"),
			st("-cells-root", "relative", "model-policy", "hook", "a", "b", "c", "d", "e", "f"),
		}},
		{name: "refusal-order", steps: []goldenStep{
			stEnv(dry, "desk", "example", "badrole", "--model", ""),
		}},
		// The old parser read the first --kind of desk, up and show by scanning the whole line
		// before its flag loop, so a --kind in another flag's value position was still the kind,
		// and a later --kind was skipped unread even with no value after it.
		{name: "kind-whole-line", steps: []goldenStep{
			stEnv(dry, "desk", "example", "worker-desk", "--kind", "house", "--kind"),
			stEnv(dry, "up", "example", "--kind", "house", "--kind"),
			st("show", "example", "--kind", "house", "--kind"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", "--kind"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", "--kind", "bogus"),
			stEnv(dry, "desk", "example", "worker-desk", "--model", "--kind", "scrubbed", "<ROOT>/claude-config"),
			stEnv(dry, "desk", "scrub", "worker-desk", "--model", "--kind", "house", "<ROOT>/claude-config"),
			stEnv(dry, "up", "example", "--model", "--kind"),
			stEnv(dry, "up", "example", "--provider", "--kind"),
			stEnv(dry, "up", "scrub", "--provider", "--kind", "house", "<ROOT>/claude-config"),
			st("show", "example", "--model", "--kind"),
			st("show", "example", "--provider", "--kind"),
			st("show", "example", "--provider", "--kind", "house"),
			st("show", "scrub", "--kind", "house", "--kind", "--kind"),
		}},
		// new read most values with a helper that took a missing trailing value as empty.
		{name: "new-trailing-value", steps: []goldenStep{
			st("new", "f1", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo", "--forge"),
			st("new", "f2", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo", "--port"),
			st("show", "f2"),
			st("new", "f3", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo", "--roles"),
			st("new", "f4", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo", "--orgs"),
			st("new", "f5", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo", "--cells-yaml"),
			st("new", "f6", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo", "--launcher"),
			st("new", "f7", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo", "--repo-slug"),
			st("new", "f8", "--kind", "house", "--repo", "<ROOT>/repo", "--roots"),
			st("new", "f9", "--forge"),
			st("new", "f10", "--kind"),
			st("new", "f11", "--kind", "house", "--repo", "<ROOT>/repo", "--roots", "o/r=<ROOT>/repo", "--repo", "--forge"),
			st("ls"),
		}},
		{name: "scratch-terminator", steps: []goldenStep{
			st("scratch", "--", "example", "sweep"),
			st("scratch", "--", "example"),
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
//
// A line the old binary refused and the new one runs has output of its own, so such an entry
// names fragments instead: stdoutHas replaces the stdout-must-match rule, and stderrHas replaces
// the exact-stderr rule (the roster echo is in it, and is world-specific).
type deliberateDiff struct {
	code      int
	stderr    string
	why       string
	stdoutHas []string
	stderrHas []string
}

const (
	whyNoEcho     = "help, --version, version and parse failures print no roster echo: nothing acts, so there is no effective configuration to show"
	whyParseText  = "a malformed command line is now refused by the parser with its own wording and the usage exit (3)"
	whyShape      = "refused as before (exit 3, nothing run), by the shape check in the parser's words and without the roster echo"
	whyHookBlocks = "a refused line that names the model-policy hook exits with the hook's blocking 2, where the legacy unknown-verb exit 3 was non-blocking"
	echoFragment  = "assay-config:"
)

func unknownCmd(name string) string {
	return shapeErr("unknown command \"" + name + "\" for \"cellctl\"")
}

func shapeErr(msg string) string { return "cellctl: " + msg + "\nRun 'cellctl --help' for usage.\n" }

var deliberateDiffs = map[string]deliberateDiff{
	"version-flag/0":  {code: 0, stderr: "", why: whyNoEcho},
	"version-verb/0":  {code: 0, stderr: "", why: whyNoEcho},
	"unknown-verb/0":  {code: 3, stderr: "cellctl: unknown command \"frobnicate\" for \"cellctl\"\nRun 'cellctl --help' for usage.\n", why: whyParseText},
	"show-refusals/3": {code: 3, stderr: "cellctl: flag needs an argument: --model\nRun 'cellctl --help' for usage.\n", why: whyParseText},
	"desk-refusals/6": {code: 3, stderr: "cellctl: flag needs an argument: --model\nRun 'cellctl --help' for usage.\n", why: whyParseText},
	"set-refusals/9":  {code: 3, stderr: "cellctl: flag needs an argument: --harness\nRun 'cellctl --help' for usage.\n", why: whyParseText},
	"scratch/10":      {code: 3, stderr: "cellctl: invalid argument \"bogus\" for \"--max-age\" flag: time: invalid duration \"bogus\"\nRun 'cellctl --help' for usage.\n", why: whyParseText + "; scratch's old flag-package exit 2 becomes 3"},
	"cells-root/3":    {code: 3, stderr: "cellctl: flag needs an argument: --cells-root\nRun 'cellctl --help' for usage.\n", why: whyParseText},

	// The hidden completion entrypoints refuse like any unknown command.
	"completion-entrypoints/0": {code: 3, stderr: unknownCmd("__complete"), why: whyParseText},
	"completion-entrypoints/1": {code: 3, stderr: unknownCmd("__completeNoDesc"), why: whyParseText},
	"completion-entrypoints/2": {code: 3, stderr: unknownCmd("__complete"), why: whyParseText},

	// Shapes the legacy parser refused and the shape check (shape.go) refuses again, before
	// anything parses: the same exit 3 with nothing run, in the parser's words and without the
	// roster echo. The verb-position lines are unknown verbs (decision entry 2); the wording of the
	// others is the change the compatibility report's "Refused shapes" section sets out.
	"flags-before-positionals/0": {code: 3, stderr: shapeErr(`scratch: flag "--apply" comes before the command's positional arguments; flags follow them`), why: whyShape},
	"flags-before-positionals/1": {code: 3, stderr: shapeErr(`cadence: flag "--confirm-stopped" comes before the command's positional arguments; flags follow them`), why: whyShape},
	"flags-before-positionals/2": {code: 3, stderr: shapeErr(`desk: flag "--model" comes before the command's positional arguments; flags follow them`), why: whyShape},
	"flags-before-positionals/3": {code: 3, stderr: shapeErr(`show: flag "--harness" comes before the command's positional arguments; flags follow them`), why: whyShape},

	"single-dash-outside-scratch/0": {code: 3, stderr: shapeErr(`desk: single-dash flag "-model" (only scratch takes that spelling; write --model)`), why: whyShape},
	"single-dash-outside-scratch/1": {code: 3, stderr: shapeErr(`show: single-dash flag "-harness" (only scratch takes that spelling; write --harness)`), why: whyShape},
	"single-dash-outside-scratch/2": {code: 3, stderr: unknownCmd("-version"), why: whyParseText},
	"single-dash-outside-scratch/3": {code: 3, stderr: shapeErr(`unknown command "extra" for "cellctl version"`), why: whyParseText},

	"cells-root-spellings/0": {code: 3, stderr: unknownCmd("--cells-root=<ROOT>/cells"), why: whyParseText},
	"cells-root-spellings/1": {code: 3, stderr: unknownCmd("-cells-root"), why: whyParseText},
	"cells-root-spellings/2": {code: 3, stderr: unknownCmd("--cells-root=<ROOT>/cells"), why: whyParseText},
	"cells-root-spellings/3": {code: 2, stderr: unknownCmd("--cells-root=<ROOT>/cells"), why: whyParseText + "; " + whyHookBlocks},
	"cells-root-spellings/4": {code: 2, stderr: unknownCmd("-cells-root"), why: whyParseText + "; " + whyHookBlocks},
}

// diffHolds reports whether a listed difference still has exactly the recorded new shape.
func diffHolds(d deliberateDiff, got, old cliRun) bool {
	if got.Code != d.code {
		return false
	}
	if d.stdoutHas == nil {
		if got.Stdout != old.Stdout {
			return false
		}
	} else {
		for _, f := range d.stdoutHas {
			if !strings.Contains(got.Stdout, f) {
				return false
			}
		}
	}
	if d.stderrHas == nil {
		return got.Stderr == d.stderr
	}
	for _, f := range d.stderrHas {
		if !strings.Contains(got.Stderr, f) {
			return false
		}
	}
	return true
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
				if !diffHolds(d, got[i], want[i]) {
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
