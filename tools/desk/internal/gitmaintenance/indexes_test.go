package gitmaintenance

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Legacy command fixtures lacked the process-owner's unknown-cleanup result;
// these witnesses distinguish it from failure after confirmed tree cleanup.
// regression: #2513
func TestIndexCustodyResults(t *testing.T) {
	for _, kind := range []string{"success", "uncertain", "failure", "warning", "exit-code"} {
		t.Run(kind, func(t *testing.T) {
			repo := fixtureRepo(t)
			requireNativeHooks(t, repo)
			marker := filepath.Join(repo, ".git", activeName)
			old := indexProcess
			t.Cleanup(func() { indexProcess = old })
			calls := 0
			indexProcess = func(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer) (int, bool, error) {
				calls++
				if _, err := os.Stat(marker); err != nil {
					t.Errorf("child lacks durable marker: %v", err)
				}
				if dir != repo {
					t.Errorf("wrong worktree scope %q", dir)
				}
				joined := strings.Join(argv, " ")
				if calls == 1 && !strings.HasSuffix(joined, "pack-refs --all") {
					t.Errorf("changed first task: %s", joined)
				}
				if calls == 2 && !strings.Contains(joined, "commit-graph write --reachable --split --no-progress --max-new-filters=0") {
					t.Errorf("changed second task: %s", joined)
				}
				switch kind {
				case "uncertain":
					return 0, true, nil
				case "failure":
					return 1, false, errors.New("fixture failure")
				case "warning":
					_, _ = io.WriteString(stderr, "fixture warning")
				case "exit-code":
					return 7, false, nil
				}
				return 0, false, nil
			}
			err := RunIndexes(context.Background(), repo, nil)
			if kind == "success" {
				if err != nil || calls != 2 {
					t.Fatalf("fixed tasks: calls=%d err=%v", calls, err)
				}
			} else if err == nil || calls != 1 {
				t.Fatalf("uncertainty accepted: calls=%d err=%v", calls, err)
			}
			_, statErr := os.Stat(marker)
			if kind == "uncertain" {
				if statErr != nil || !errors.Is(err, errCleanupUnknown) {
					t.Fatalf("unknown cleanup lost marker/sentinel: %v %v", statErr, err)
				}
				if err := CheckCustody(context.Background(), repo); err == nil {
					t.Fatal("unknown child admitted another coordinator")
				}
			} else if !os.IsNotExist(statErr) {
				t.Fatalf("confirmed cleanup retained marker: %v", statErr)
			}
		})
	}
}

// regression: #2513
func TestIndexAmbientEnv(t *testing.T) {
	got := indexEnv([]string{"HOME=legitimate", "git_dir=hostile", "GIT_CONFIG_COUNT=1", "ASSAY_FIXTURE=preserved"})
	want := []string{"HOME=legitimate", "ASSAY_FIXTURE=preserved", "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment: %v", got)
	}
}
