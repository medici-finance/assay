package main

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// regression: #4116
func TestDomainSnapshotClosure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "python3", "-B", "-I", "domain/mutate_test.py").CombinedOutput()
	if err != nil {
		t.Fatalf("domain snapshot regression: %v\n%s", err, out)
	}
}
