//go:build unix

package main

// readpipe_test.go — review finding SEC-7 (class read-audit-claim-contradicted): a read verb's
// one audit line must survive its reader closing the output pipe early. Writing to a closed
// pipe on standard output kills a Go process by SIGPIPE, and deferred calls do not run then,
// so a read whose line was written by its deferred finish after the output loop left no line
// when piped into `head`. The in-memory outcome table (readledger_test.go) cannot see this,
// so each case here runs the verb in a CHILD process whose standard output is a real pipe,
// closes the read end early, proves the child died on the broken pipe, and then reads the
// child's ledger for exactly one ok line under the read key.

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	pipeChildEnv = "DESKRUN_PIPE_CHILD" // the verb the child runs
	pipeHomeEnv  = "DESKRUN_PIPE_HOME"  // the HOME (and so the ledger) the child writes under
)

// TestReadPipeChild is the child half; it does nothing unless the parent set pipeChildEnv.
func TestReadPipeChild(t *testing.T) {
	verb := os.Getenv(pipeChildEnv)
	if verb == "" {
		t.Skip("child half of TestReadCutOffRecorded")
	}
	w := plantWorld(t, deskkit.ForgeGitHub)
	home := os.Getenv(pipeHomeEnv)
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	// A log far larger than any pipe buffer, so the reader's early close lands mid-output.
	w.fake.logParts = []deskkit.RunLogPart{{Name: "build", Text: strings.Repeat("compiling a long step\n", 1<<16)}}
	args := []string{"example-org/tracker", "501"}
	switch verb {
	case "log":
		_ = cmdLog(args, os.Stdout)
	case "status":
		_ = cmdStatus(args, os.Stdout)
	}
	// Reaching here means the write never hit the closed pipe; the parent reports that.
}

// TestReadCutOffRecorded — `deskrun log … | head -c 4096` and a `status` whose reader is
// already gone each leave exactly one ok line under the read key.
func TestReadCutOffRecorded(t *testing.T) {
	if os.Getenv(pipeChildEnv) != "" {
		t.Skip("parent half")
	}
	for _, tc := range []struct {
		verb string
		read int64 // bytes the reader takes before closing; 0 closes before the child starts
	}{{"log", 4096}, {"status", 0}} {
		t.Run(tc.verb, func(t *testing.T) {
			home := t.TempDir()
			r, wr, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestReadPipeChild$", "-test.count=1")
			cmd.Env = append(os.Environ(), pipeChildEnv+"="+tc.verb, pipeHomeEnv+"="+home)
			cmd.Stdout = wr
			if tc.read == 0 {
				r.Close()
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			wr.Close()
			if tc.read > 0 {
				if n, err := io.CopyN(io.Discard, r, tc.read); n != tc.read {
					t.Fatalf("the reader got %d of %d bytes before closing: %v", n, tc.read, err)
				}
				r.Close()
			}
			err = cmd.Wait()
			ws, _ := cmd.ProcessState.Sys().(syscall.WaitStatus)
			if !ws.Signaled() || ws.Signal() != syscall.SIGPIPE {
				t.Fatalf("control: the child must die on the broken pipe, got %v (%v) — the case did not happen", err, ws)
			}
			t.Setenv("HOME", home)
			inWrite, recorded := readLines(t)
			if len(inWrite) > 0 || len(recorded) != 1 || recorded[0] != deskkit.ResultOK {
				t.Fatalf("%s cut off by its reader: read-key lines %v, write-bucket lines %v — want exactly one %q",
					tc.verb, recorded, inWrite, deskkit.ResultOK)
			}
		})
	}
}
