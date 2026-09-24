package main

// parity_test.go — THE ORACLE. The verb's stdout must equal the bash script's stdout on every
// recorded fixture, cycle by cycle, with the same exit code, the same state files left behind, and
// the same repos read in the same order.
//
// The script is run for real (bash + the stub gh + jq); the verb is run in-process against the
// httptest replay of the same answers. Everything is compared byte-for-byte EXCEPT the diagnostic
// inside a failed read's parentheses — `(gh: <gh's stderr>)` from the script, `(forge: <the forge
// client's error>)` from the verb. That text is each tool's own report of the same failure and
// differs by construction; the comparison masks exactly that span and nothing else, and the test
// separately asserts the verb's diagnostic carries the forge's status code, so the mask cannot
// hide an empty report.
//
// A runner without bash or jq cannot run the oracle: the test SKIPS naming which one is missing,
// which a Verify row reads as could-not-check — never as a pass.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// failedReadDiag is the one span the comparison masks: the tool's own quote of a failed read.
var failedReadDiag = regexp.MustCompile(`\((gh|forge): .*?\) — (keeping|no baseline)`)

func maskDiagnostics(s string) string {
	return failedReadDiag.ReplaceAllString(s, "(<diagnostic>) — $2")
}

// requireOracle returns the script path, or skips with the could-not-check reason.
func requireOracle(t *testing.T, script string) string {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("could-not-check: bash is not on PATH — the parity oracle (" + script + ") cannot run on this runner")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("could-not-check: jq is not on PATH — the parity oracle (" + script + ") refuses to run without it")
	}
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "plugins", "assay", "scripts", script))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("the parity oracle is missing at %s: %v — it is kept until windows-port/14 retires it", p, err)
	}
	return p
}

type cycleResult struct {
	stdout string
	code   int
	state  map[string]string
	reads  []string
}

func runOracle(t *testing.T, script string, env []string, args []string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("the oracle did not run: %v", err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errb.String(), code
}

// runParity drives one fixture through both implementations and returns their per-cycle results.
func runParity(t *testing.T, script string, fx fixture) (oracle, verb []cycleResult) {
	t.Helper()
	plantRoster(t, fx.Repos)
	// `pr` reads as the session's App token (it takes no --token-file, like its oracle); the mint is
	// stubbed so no test reaches the real minter.
	stubSessionIdentity(t, "worker", "x-parity-minted-token", nil)
	fake := newForgeFake(t)
	gh := newGHStub(t)

	oracleState := filepath.Join(t.TempDir(), "oracle-state")
	verbState := filepath.Join(t.TempDir(), "verb-state")
	stateEnv := "INBOUND_MONITOR_STATE_DIR"
	limitEnv, limitDef := "INBOUND_MONITOR_LIMIT", 500
	if fx.Kind == "pr" {
		stateEnv = "PR_MONITOR_STATE_DIR"
		limitEnv, limitDef = "PR_MONITOR_LIMIT", 100
	}
	limit := limitDef
	if v, ok := fx.Env[limitEnv]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("fixture %s=%q", limitEnv, v)
		}
		limit = n
	}

	// Identity: every owner is handed a token file, to both, so neither needs a session role.
	var args []string
	if fx.Kind == "inbound" {
		for _, o := range owners(fx.Repos) {
			args = append(args, "--token-file", o+"="+ownerTokenFile(t, "x-parity-token-"+o))
		}
	}
	args = append(args, fx.Repos...)

	// The fixture's knobs, pace 0 (the test's own speed), for both.
	t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
	for k, v := range fx.Env {
		t.Setenv(k, v)
	}

	baseEnv := func(stateDir string) []string {
		var env []string
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, stateEnv+"=") {
				continue
			}
			env = append(env, e)
		}
		return append(env,
			"PATH="+gh.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
			stateEnv+"="+stateDir,
			"PARITY_CYCLE_DIR="+gh.cycleDir,
			"PARITY_READ_LOG="+gh.logPath,
		)
	}

	for i, c := range fx.Cycles {
		// --- the oracle ---
		gh.render(t, fx.Kind, c, limit)
		out, errOut, code := runOracle(t, script, baseEnv(oracleState), args)
		if code == 1 {
			t.Fatalf("cycle %d (%s): the oracle hit a precondition failure: %s", i+1, c.Note, errOut)
		}
		oracle = append(oracle, cycleResult{stdout: out, code: code, state: snapshotState(t, oracleState), reads: gh.takeReads(t)})

		// --- the verb ---
		fake.setCycle(c)
		t.Setenv(stateEnv, verbState)
		var vout, verr bytes.Buffer
		vcode := run(append([]string{fx.Kind}, args...), &vout, &verr)
		if vcode == 1 {
			t.Fatalf("cycle %d (%s): the verb hit a precondition failure: %s", i+1, c.Note, verr.String())
		}
		verb = append(verb, cycleResult{stdout: vout.String(), code: vcode, state: snapshotState(t, verbState), reads: fake.takeReads()})
	}
	return oracle, verb
}

func assertParity(t *testing.T, fx fixture, oracle, verb []cycleResult) {
	t.Helper()
	for i := range fx.Cycles {
		o, v := oracle[i], verb[i]
		label := "cycle " + strconv.Itoa(i+1) + " (" + fx.Cycles[i].Note + ")"
		t.Logf("%s: exit %d, reads %v\n%s", label, v.code, v.reads, v.stdout)
		if maskDiagnostics(o.stdout) != maskDiagnostics(v.stdout) {
			t.Errorf("%s: stdout differs\n--- oracle ---\n%s--- verb ---\n%s", label, o.stdout, v.stdout)
		}
		if o.code != v.code {
			t.Errorf("%s: exit code oracle=%d verb=%d", label, o.code, v.code)
		}
		if !reflect.DeepEqual(o.state, v.state) {
			t.Errorf("%s: state files differ\n--- oracle ---\n%v\n--- verb ---\n%v", label, o.state, v.state)
		}
		if !reflect.DeepEqual(o.reads, v.reads) {
			t.Errorf("%s: repos read differ\n  oracle %v\n  verb   %v", label, o.reads, v.reads)
		}
		// The mask must never hide an empty report: every masked verb diagnostic carries the
		// forge's own status for the failure the fixture recorded.
		for _, m := range regexp.MustCompile(`\(forge: (.*?)\) — `).FindAllStringSubmatch(v.stdout, -1) {
			if !regexp.MustCompile(`HTTP [0-9]{3}`).MatchString(m[1]) {
				t.Errorf("%s: the verb's failed-read diagnostic names no HTTP status: %q", label, m[1])
			}
		}
	}
}

// TestParityInboundMonitor — `deskmonitor inbound` ≡ inbound-monitor.sh on every recorded fixture.
func TestParityInboundMonitor(t *testing.T) {
	script := requireOracle(t, "inbound-monitor.sh")
	fixtures := loadFixtures(t, "inbound")
	names := sortedKeys(fixtures)
	for _, name := range names {
		fx := fixtures[name]
		t.Run(name, func(t *testing.T) {
			oracle, verb := runParity(t, script, fx)
			assertParity(t, fx, oracle, verb)
		})
	}
}

// TestParityPRMonitor — `deskmonitor pr` ≡ pr-monitor.sh on every recorded fixture.
func TestParityPRMonitor(t *testing.T) {
	script := requireOracle(t, "pr-monitor.sh")
	fixtures := loadFixtures(t, "pr")
	for _, name := range sortedKeys(fixtures) {
		fx := fixtures[name]
		t.Run(name, func(t *testing.T) {
			oracle, verb := runParity(t, script, fx)
			assertParity(t, fx, oracle, verb)
		})
	}
}

// TestParityPRMonitor_EmptyBaselineOracleDefect pins the ONE stated divergence (pr.go's header):
// a repo seeded with ZERO open PRs, then two PRs open. The oracle's awk NR==FNR diff reads the
// current set as the baseline and reports both as CLOSED; the verb reports both as OPENED. If the
// oracle is ever fixed, the first assertion fails and this test is retired into the parity corpus.
func TestParityPRMonitor_EmptyBaselineOracleDefect(t *testing.T) {
	script := requireOracle(t, "pr-monitor.sh")
	fx := fixture{
		Kind:  "pr",
		Repos: []string{"example-org/tracker"},
		Cycles: []fixtureCycle{
			{Note: "seed with no open PRs", Reads: map[string]fixtureRead{"example-org/tracker": {}}},
			{Note: "two PRs open", Reads: map[string]fixtureRead{"example-org/tracker": {PRs: []fixturePR{
				{Number: 5, HeadRefOid: "aaa", MergeStateStatus: "CLEAN"},
				{Number: 6, HeadRefOid: "bbb", IsDraft: true, MergeStateStatus: "BLOCKED"},
			}}}},
		},
	}
	oracle, verb := runParity(t, script, fx)
	wantOracle := "PR-EVENT: example-org/tracker#5 closed aaa -> -\nPR-EVENT: example-org/tracker#6 closed bbb -> -\n"
	gotOracle := strings.Join(sortedLines(oracle[1].stdout), "")
	if gotOracle != wantOracle {
		t.Fatalf("the oracle no longer shows the empty-baseline defect (fixed?) — fold this case into the parity corpus.\n got %q", oracle[1].stdout)
	}
	wantVerb := "PR-EVENT: example-org/tracker#5 opened - -> aaa\nPR-EVENT: example-org/tracker#6 opened - -> bbb\n"
	if verb[1].stdout != wantVerb {
		t.Fatalf("verb on an empty baseline:\n got %q\nwant %q", verb[1].stdout, wantVerb)
	}
	// Everything else about the cycle still agrees: exit code, the baseline left behind, the reads.
	if oracle[1].code != verb[1].code || !reflect.DeepEqual(oracle[1].state, verb[1].state) || !reflect.DeepEqual(oracle[1].reads, verb[1].reads) {
		t.Fatalf("beyond the stated divergence the cycle differs: oracle %+v verb %+v", oracle[1], verb[1])
	}
}

func sortedKeys(m map[string]fixture) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedLines(s string) []string {
	var out []string
	for _, l := range strings.SplitAfter(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}
