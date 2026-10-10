package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These drive the BUILT binary through `desk`, `models reset`, `show` and `check`, so the boot
// hook, the verb's routing in main.go and each path's DRY_RUN handling are under test, not only
// the reset function.

// agedPinCell is a plain house cell whose worker-desk pin is a DEFAULT recorded 30 days ago.
func agedPinCell(t *testing.T, lines ...string) (*policyFixture, string) {
	t.Helper()
	f := newPolicyFixture(t, "2.1.278")
	f.plainCell(t, append([]string{"DESK_MODEL_worker_desk=fable"}, lines...)...)
	writeAgedRecord(t, f)
	return f, filepath.Join(f.cellDir, "cell.env")
}

func writeAgedRecord(t *testing.T, f *policyFixture) {
	t.Helper()
	at := time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	savePinFile(f.cellDir, pinFile{Pins: map[string]pinRecord{
		"DESK_MODEL_worker_desk": {Kind: pinKindDefault, Value: "fable", At: at},
	}})
}

func readFileT(t *testing.T, p string) string {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

const resetLine = "[model-reset] worker-desk: fable -> sonnet"

// regression: #2425 C5
func TestBinaryBootResetDryRun(t *testing.T) {
	f, envfile := agedPinCell(t)
	before, rec := readFileT(t, envfile), readFileT(t, filepath.Join(f.cellDir, modelPinsFile))
	r := f.dryRunDesk(t, "worker-desk")
	if r.code != 0 {
		t.Fatalf("dry-run desk: %+v", r)
	}
	if !strings.Contains(r.stdout, resetLine) || !strings.Contains(r.stdout, dryRunMark+"\n") {
		t.Errorf("boot did not plan the marked repin:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "role=worker-desk model=sonnet") {
		t.Errorf("the plan does not run the repinned model:\n%s", r.stdout)
	}
	if readFileT(t, envfile) != before || readFileT(t, filepath.Join(f.cellDir, modelPinsFile)) != rec {
		t.Error("a dry-run boot wrote cell.env or the record")
	}
}

// regression: #2425 C5
func TestBinaryModelsResetVerb(t *testing.T) {
	f, envfile := agedPinCell(t)
	before := readFileT(t, envfile)
	r := f.run(t, []string{"DRY_RUN=1"}, "models", "reset", "example")
	if r.code != 0 || !strings.Contains(r.stdout, resetLine) || !strings.Contains(r.stdout, dryRunMark) {
		t.Fatalf("dry-run models reset: %+v", r)
	}
	if readFileT(t, envfile) != before {
		t.Fatal("DRY_RUN=1 models reset wrote cell.env")
	}
	r = f.run(t, nil, "models", "reset", "example")
	if r.code != 0 || !strings.Contains(r.stdout, resetLine) || strings.Contains(r.stdout, dryRunMark) ||
		!strings.Contains(r.stdout, "[model-reset] backup written: ") {
		t.Fatalf("live models reset: %+v", r)
	}
	if !strings.Contains(readFileT(t, envfile), "DESK_MODEL_worker_desk=sonnet\n") {
		t.Errorf("live reset did not write cell.env:\n%s", readFileT(t, envfile))
	}
	r = f.run(t, nil, "models", "reset", "example")
	if r.code != 0 || !strings.Contains(r.stdout, "nothing to repin (ttl 7d)") {
		t.Errorf("second run: %+v", r)
	}
}

// regression: #2425 C3
func TestBinaryModelTTLFlagWins(t *testing.T) {
	// The reset is off in cell.env; --model-ttl turns it on for one run.
	f, _ := agedPinCell(t, "CELL_MODEL_TTL_DAYS=0")
	if r := f.run(t, nil, "models", "reset", "example"); r.code != 0 || strings.Contains(r.stdout, resetLine) {
		t.Fatalf("ttl 0 still repinned: %+v", r)
	}
	if r := f.run(t, nil, "models", "reset", "example", "--model-ttl", "7d"); r.code != 0 || !strings.Contains(r.stdout, resetLine) {
		t.Fatalf("--model-ttl ignored: %+v", r)
	}
	// A hand-edited malformed TTL refuses a plain run, and the flag recovers it.
	f, _ = agedPinCell(t, "CELL_MODEL_TTL_DAYS=soon")
	if r := f.run(t, nil, "models", "reset", "example"); r.code == 0 || !strings.Contains(r.stderr, modelTTLKey) {
		t.Errorf("malformed cell TTL accepted: %+v", r)
	}
	if r := f.run(t, nil, "models", "reset", "example", "--model-ttl", "7d"); r.code != 0 || !strings.Contains(r.stdout, resetLine) {
		t.Errorf("--model-ttl did not recover a malformed cell TTL: %+v", r)
	}
	// `set` refuses the value in the first place.
	if r := f.run(t, nil, "set", "example", "CELL_MODEL_TTL_DAYS=NaN"); r.code == 0 {
		t.Errorf("set accepted CELL_MODEL_TTL_DAYS=NaN: %+v", r)
	}
}

// regression: #2425 C4
func TestBinaryShowCheckPending(t *testing.T) {
	f, envfile := agedPinCell(t)
	before := readFileT(t, envfile)
	r := f.run(t, nil, "show", "example")
	if r.code != 0 || !strings.Contains(r.stdout, "[show] "+resetLine) ||
		!strings.Contains(r.stdout, "[show] model worker-desk=sonnet (pending model-reset of cell.env DESK_MODEL_worker_desk)") {
		t.Errorf("show does not report what the next boot runs:\n%s\n%s", r.stdout, r.stderr)
	}
	r = f.run(t, []string{f.hermeticPath()}, "check", "example")
	if !strings.Contains(r.stdout, "warn  model reset pending: "+resetLine) ||
		!strings.Contains(r.stdout, "role=worker-desk harness=claude model=sonnet") {
		t.Errorf("check does not report what the next boot runs:\n%s", r.stdout)
	}
	if readFileT(t, envfile) != before {
		t.Error("show or check wrote cell.env")
	}
}
