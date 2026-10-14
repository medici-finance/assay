package gitmaintenance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// regression: #2513
func TestPackDirectoryBudget(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	common := filepath.Join(repo, ".git")
	pack := filepath.Join(common, "objects", "pack")
	// Junk does not contribute to the object count or pack-byte eligibility.
	// Its enumeration must have an independent allocation/work bound.
	for i := 0; i < 4097; i++ {
		if err := os.WriteFile(filepath.Join(pack, fmt.Sprintf("junk-%05d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Existing coordinator lock files make the apply assertion cover task and
	// stamp writes, rather than incidental creation of its persistent locks.
	for _, name := range []string{lockName, tempRefLockName} {
		if err := os.WriteFile(filepath.Join(common, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := diskImage(t, repo)
	t.Run("configuration preflight", func(t *testing.T) {
		_, err := validateConfig(context.Background(), common)
		if err == nil || !strings.Contains(err.Error(), "pack directory exceeds inventory budget") {
			t.Fatalf("oversized pack directory escaped configuration preflight: %v", err)
		}
	})
	t.Run("batch selection", func(t *testing.T) {
		_, err := packBatch(context.Background(), common)
		if err == nil || !strings.Contains(err.Error(), "pack directory exceeds inventory budget") {
			t.Fatalf("oversized pack directory escaped batch selection: %v", err)
		}
	})
	t.Run("inventory and apply", func(t *testing.T) {
		if _, err := Inspect(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "pack directory exceeds inventory budget") {
			t.Fatalf("oversized pack directory escaped inventory: %v", err)
		}
		for _, dry := range []bool{true, false} {
			report, err := Run(context.Background(), repo, Options{DryRun: dry})
			if err == nil || !strings.Contains(err.Error(), "pack directory exceeds inventory budget") || len(report.Tasks) != 0 {
				t.Fatalf("oversized pack directory escaped run (dry=%t): %+v, %v", dry, report, err)
			}
		}
	})
	t.Run("canceled reader", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := readPackEntries(ctx, common); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled directory reader escaped its deadline: %v", err)
		}
	})
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatal("oversized pack directory inspection changed repository state")
	}
}
