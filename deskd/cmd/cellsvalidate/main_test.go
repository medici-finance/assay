package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This is the BOOT half of Verify row 8. config/schema's own tests prove the
// rule; this proves the operator meets it — that `cellsvalidate` actually EXITS
// NON-ZERO on a config naming a model outside the §13.8 ladder, rather than
// validating it and printing OK. A rule that the binary does not enforce is a
// rule nobody is stopped by.
//
// It runs the real binary via `go run`, because the exit code IS the contract:
// calling schema.Load in-process would test the library again and prove nothing
// about the command.

const cellPreamble = `cells:
  - name: assay
    repos: [example-org/console]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [glm-5.3]
        app: assay-reviewer-app
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
`

func writeCells(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cells.yaml")
	if err := os.WriteFile(path, []byte(cellPreamble+body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// runValidator runs cmd/cellsvalidate over a fixture and returns its exit code
// and combined output.
func runValidator(t *testing.T, path string) (int, string) {
	t.Helper()
	cmd := exec.Command("go", "run", ".", path)
	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if err != nil {
		if !asExitError(err, &ee) {
			t.Fatalf("go run failed to start: %v\n%s", err, out)
		}
		code = ee.ExitCode()
	}
	return code, string(out)
}

func asExitError(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}

// TestAnswerRecordsBackendConfigRefusedOutsideLadder is Verify row 8's boot
// half: a configuration naming a model outside the ladder is REFUSED at boot,
// so no metered backend can be configured in.
func TestAnswerRecordsBackendConfigRefusedOutsideLadder(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs the validator binary")
	}
	for _, m := range []string{"gpt-4o", "claude-opus-4-8", "gemini-2.5-pro", "glm-5.3"} {
		path := writeCells(t, "    chat:\n      models: ["+m+"]\n      key_env: MODEL_BACKEND_KEY\n")
		code, out := runValidator(t, path)
		if code == 0 {
			t.Fatalf("cellsvalidate exited 0 on a config naming the off-ladder model %q — the boot gate does not "+
				"stop a metered backend:\n%s", m, out)
		}
		if !strings.Contains(out, "ladder") {
			t.Fatalf("%q: the operator is refused without being taught the rule:\n%s", m, out)
		}
	}
}

// TestAnswerRecordsBackendConfigAcceptsTheLadder is the paired positive
// control. A validator that exits non-zero on everything proves nothing about
// the refusal above.
func TestAnswerRecordsBackendConfigAcceptsTheLadder(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs the validator binary")
	}
	path := writeCells(t, "    chat:\n      models: [deepseek-chat, glm-5.2]\n      key_env: ASSAY_MODEL_BACKEND_KEY\n")
	code, out := runValidator(t, path)
	if code != 0 {
		t.Fatalf("cellsvalidate refused a valid on-ladder chat block (exit %d):\n%s", code, out)
	}
	if !strings.Contains(out, "OK") {
		t.Fatalf("validator did not report OK:\n%s", out)
	}
}

// TestAnswerRecordsBackendConfigRefusesAPastedKey: the file NAMES a variable and
// can never hold a key. An operator who pastes one is stopped by the
// validator's character rule, before anyone has to notice what the value is.
func TestAnswerRecordsBackendConfigRefusesAPastedKey(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs the validator binary")
	}
	path := writeCells(t, "    chat:\n      models: [deepseek-chat]\n      key_env: \"sk-not-a-variable-name\"\n")
	code, out := runValidator(t, path)
	if code == 0 {
		t.Fatalf("cellsvalidate accepted a key VALUE in key_env:\n%s", out)
	}
	// The refusal must not echo the offending value back into a place it could
	// be scraped from — it names the field and the rule.
	if !strings.Contains(out, "key_env") {
		t.Fatalf("the refusal does not name the field:\n%s", out)
	}
}
