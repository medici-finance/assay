//go:build darwin || linux

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// regression: #2793
func TestPlanNoAuditReads(t *testing.T) {
	if count := os.Getenv("REVIEW_PLAN_PIPE"); count != "" {
		n, err := strconv.Atoi(count)
		if err != nil {
			t.Fatal(err)
		}
		dir := historyDir(t)
		// An attempted history read blocks. Zero ledger loads must hold as keys grow.
		if err := syscall.Mkfifo(filepath.Join(dir, "audit.jsonl"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := cmdPlan(planFixture(t, n), io.Discard); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, count := range []string{"0", "1", "33"} {
		t.Run(count, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPlanNoAuditReads$")
			cmd.Env = append(os.Environ(), "REVIEW_PLAN_PIPE="+count)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s keys: planner attempted a history read or failed: %v %s", count, err, out)
			}
		})
	}
}
