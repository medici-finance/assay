package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestRule51Class pins the class token for token.
func TestRule51Class(t *testing.T) {
	in := []string{
		"statusgen --root . --lint",
		"statusgen --lint --root .",
		"statusgen --lint",
		"statusgen --root=. --lint",
		"statusgen  --root .  --lint",
		"cd statusgen && go run . --root .. --lint",
		"cd statusgen && go run . --lint --root=..",
	}
	out := []string{
		"statusgen --root . --lint --diff-base refs/remotes/origin/main",
		"statusgen --root . --lint --diff-base $(git merge-base refs/remotes/origin/main HEAD)",
		"statusgen --lint --diff-base=HEAD~1",
		"statusgen --root . --lint --acceptance-forge",
		"statusgen --root . --lint; echo rc=$?",
		"statusgen --root . --lint > out.txt",
		"statusgen --root . --lint 2>&1 \\| grep -q 'LINT: PASS'",
		"statusgen --root .. --lint",
		"statusgen --root /tmp/x --lint",
		"statusgen --root . --root . --lint",
		"statusgen --root . --lint --lint",
		"statusgen --root .",
		"statusgen patterns --lint --root .",
		"statusgen --root . --lint --json",
		"statusgen --root . --lint-audit",
		"cd statusgen && go run . --lint",
		"cd statusgen && go run . --root .. --lint; echo $?",
		"cd statusgen && go run . --root . --lint",
		"/tmp/sg --root . --lint",
		"STATUSGEN=1 statusgen --root . --lint",
	}
	for _, c := range in {
		if !rule51Class(c, "exit 0") {
			t.Errorf("%q: want in the rule-51 class", c)
		}
	}
	for _, c := range out {
		if rule51Class(c, "exit 0") {
			t.Errorf("%q: want outside the rule-51 class", c)
		}
	}
	for _, e := range []string{"exit 1", "≥ `3`", "output is `NOTICE: x`"} {
		if rule51Class("statusgen --root . --lint", e) {
			t.Errorf("Expect %q: want outside the rule-51 class (not a clean lint)", e)
		}
	}
	for _, e := range []string{"", "exit 0", "output is `LINT: PASS`", "exit 0 and `LINT: PASS`"} {
		if !rule51Class("statusgen --root . --lint", e) {
			t.Errorf("Expect %q: want in the rule-51 class", e)
		}
	}
}

// rule51Stub writes a `statusgen` that fails offline and, given
// --acceptance-forge, exits forgeRC printing forgeOut, but only when the run
// credential's path reached it.
func rule51Stub(t *testing.T, forgeRC string, forgeOut string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX stub")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"case \" $* \" in *' --acceptance-forge '*)\n" +
		"  test -n \"$" + runCredentialEnv + "\" || { echo 'no credential'; exit 7; }\n" +
		"  printf '%s\\n' '" + forgeOut + "'; exit " + forgeRC + ";;\n" +
		"esac\n" +
		"echo 'PROBLEM: x/01: could-not-check human authority: --acceptance-forge is required'; echo 'LINT: FAIL 1 problem(s)'; exit 1\n"
	p := filepath.Join(dir, "statusgen")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	old := rule51Executable
	rule51Executable = func() (string, error) { return p, nil }
	t.Cleanup(func() { rule51Executable = old })
	oldV := statusgenVersion
	statusgenVersion = "v1.0.99"
	t.Cleanup(func() { statusgenVersion = oldV })
	return dir
}

func rule51TestPlan(t *testing.T) shellPlan {
	t.Helper()
	plan := resolveShellPlan()
	plan.rowScratch = t.TempDir()
	var err error
	plan.rule51HostRoster, _, err = renderRosterCopy()
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func rule51Cred(t *testing.T) *runReadCredential {
	t.Helper()
	c := &runReadCredential{repo: "example-org/example"}
	p := filepath.Join(t.TempDir(), "read-credential")
	if err := os.WriteFile(p, []byte("fixture-read-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.once.Do(func() { c.path, c.secret = p, "fixture-read-value" })
	return c
}

func rule51Run(t *testing.T, plan shellPlan, sandbox string) witness {
	t.Helper()
	rows := briefVerifyRows(r51Verify("statusgen --root . --lint"))
	ws := runWitnessesSandboxed(plan, sandbox, t.TempDir(), rows, "assay-verifier-app[bot]", "", r51Tree, r51Date, time.Minute, false)
	if len(ws) != 1 {
		t.Fatalf("got %d witnesses", len(ws))
	}
	return ws[0]
}

// TestRule51RunStampsAndCloses: an admitted --rule51 run of a row red
// offline and clean under the forge writes a stamped witness that keeps the
// authored command and the offline result, and that witness closes.
func TestRule51RunStampsAndCloses(t *testing.T) {
	rule51Stub(t, "0", "LINT: PASS")
	plan := rule51TestPlan(t)
	plan.rule51, plan.readCred = true, rule51Cred(t)
	w := rule51Run(t, plan, sandboxUnshare)
	row := w.row()
	if w.State != stateFail || w.Exit != 1 || w.Command != "statusgen --root . --lint" {
		t.Fatalf("offline result changed: %+v", w)
	}
	for _, want := range []string{"| fail exit=1 rule51=pass forge-rc=0 ", " pin=v1.0.99 ", " cred=verifier-run-read cred-digest=sha256:", "`statusgen --root . --lint`"} {
		if !strings.Contains(row, want) {
			t.Errorf("witness row lacks %q:\n%s", want, row)
		}
	}
	if strings.Contains(row, "fixture-read-value") {
		t.Fatal("the credential value reached the witness")
	}
	if witnessStateOf(row) != stateFail || witnessCommandOf(row) != "statusgen --root . --lint" {
		t.Errorf("row reads back as %q / %q", witnessStateOf(row), witnessCommandOf(row))
	}
	if err := r51Close(t, r51Verify("statusgen --root . --lint"), witnessHeader+"\n"+row+"\n"); err != nil {
		t.Errorf("the stamped witness did not close: %v", err)
	}
	// Without --rule51 the same run writes the unstamped witness, byte for byte the old one.
	plan.rule51 = false
	if w := rule51Run(t, plan, sandboxUnshare); w.Rule51 != "" || strings.Contains(w.row(), "rule51=") {
		t.Errorf("an ordinary run stamped the row: %s", w.row())
	}
}

// TestRule51RunRefusals: the forge red, no admission, and the container lane
// each stamp something the closure refuses.
func TestRule51RunRefusals(t *testing.T) {
	cases := []struct {
		name, rc, out string
		cred          bool
		sandbox       string
		state         string
	}{
		{"forge red", "1", "LINT: FAIL 1 problem(s)", true, sandboxUnshare, stateFail},
		{"forge authority read incomplete", "1", "PROBLEM: human approval could not be corroborated: HTTP 502", true, sandboxUnshare, stateCouldNotRun},
		{"rc 0 without LINT: PASS", "0", "nothing", true, sandboxUnshare, stateFail},
		{"not admitted", "0", "LINT: PASS", false, sandboxUnshare, stateCouldNotRun},
		{"container lane", "0", "LINT: PASS", true, sandboxContainerNetns, stateCouldNotRun},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rule51Stub(t, c.rc, c.out)
			plan := rule51TestPlan(t)
			plan.rule51 = true
			if c.cred {
				plan.readCred = rule51Cred(t)
			}
			sandbox := c.sandbox
			if sandbox == sandboxContainerNetns {
				// The container lane's precheck would refuse every row on a
				// host; stamp directly to test the lane's own refusal.
				w := witness{ID: "1", Command: "statusgen --root . --lint", State: stateFail, Exit: 1, OutHash: r51OutHash, Date: r51Date, Runner: "assay-verifier-app[bot]", Tree: r51Tree}
				w.Rule51, _ = runRule51(plan, sandbox, t.TempDir(), w, time.Minute)
				c.state = stateCouldNotRun
				checkRule51Refused(t, w, c.state)
				return
			}
			checkRule51Refused(t, rule51Run(t, plan, sandbox), c.state)
		})
	}
}

func checkRule51Refused(t *testing.T, w witness, state string) {
	t.Helper()
	row := w.row()
	if !strings.Contains(row, "rule51="+state+" ") {
		t.Errorf("want rule51=%s in:\n%s", state, row)
	}
	err := r51Close(t, r51Verify("statusgen --root . --lint"), witnessHeader+"\n"+row+"\n")
	if err == nil || !strings.Contains(err.Error(), "rule-51 stamp is not credited") {
		t.Errorf("closure = %v, want a refused rule-51 stamp", err)
	}
}

// TestRule51CreditRequiresRecordedFailure pins the guard that a stamp sits on
// a recorded offline failure. checkWitnesses and the coverage adapter call
// rule51Credit only for a failure, so through them the guard is unreachable;
// here a stamp whose digest does bind (over the failure fields) sits on a pass
// and on a could-not-run Result, and must not be credited, so a new caller
// cannot credit a row whose offline run was not red.
func TestRule51CreditRequiresRecordedFailure(t *testing.T) {
	const cmd = "statusgen --root . --lint"
	s := rule51Stamp{State: statePass, ForgeRC: 0, ForgeOut: "aaaaaaaaaaaa", Pin: r51Pin, CredClass: rule51CredClass, CredSum: "0f0f0f0f0f0f"}
	stamp := "rule51=pass forge-rc=0 forge-out=sha256:aaaaaaaaaaaa pin=" + r51Pin + " cred=" + rule51CredClass +
		" cred-digest=sha256:0f0f0f0f0f0f stamp=sha256:" + rule51Digest(s, "1", cmd, stateFail, "0", r51OutHash, r51Tree, r51Date)
	rows := briefVerifyRows(r51Verify(cmd))
	if len(rows) != 1 {
		t.Fatalf("fixture Verify table parsed to %d rows", len(rows))
	}
	for _, offline := range []string{statePass, stateCouldNotRun} {
		row := "| 1 | `" + cmd + "` | " + offline + " exit=0 " + stamp + " | sha256:" + r51OutHash + " | " + r51Date +
			" | assay-verifier-app[bot] @ " + r51Tree + " |"
		stamped, credited, why := rule51Credit(rows[0], row)
		if !stamped || credited || !strings.Contains(why, "not a recorded failure") {
			t.Errorf("offline %s: stamped=%v credited=%v why=%q, want a refusal naming the offline result", offline, stamped, credited, why)
		}
	}
}
