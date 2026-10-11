//go:build unix

package gitmaintenance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// regression: #2513
func TestFailedAttemptCoalesces(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	counter := filepath.Join(shim, "calls")
	script := "#!/bin/sh\ncase \" $* \" in *' commit-graph write '*) printf 'attempt\\n' >> '" + counter + "'; exit 1;; esac\nexec '" + gitPath + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	first, err := Run(context.Background(), repo, Options{})
	if err == nil {
		t.Fatal("positive control: late maintenance task did not fail")
	}
	if len(first.Tasks) < 2 {
		t.Fatal("positive control: failure did not follow an earlier completed task")
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "packed-refs")); err != nil {
		t.Fatal("positive control: earlier task did not run")
	}
	before := diskImage(t, repo)
	second, err := Run(context.Background(), repo, Options{})
	if err != nil || second.Skipped != "minimum attempt interval has not elapsed" || len(second.Tasks) != 0 {
		t.Fatalf("failed task re-entered maintenance: %#v %v", second, err)
	}
	if second.Before.LastAttempt == nil || second.Before.LastSuccess != nil || second.After != nil {
		t.Fatal("failed attempt falsely reported a successful run")
	}
	got, err := os.ReadFile(counter)
	if err != nil || string(got) != "attempt\n" {
		t.Fatalf("late task ran more than once: %q %v", got, err)
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatal("coalesced failed attempt mutated repository")
	}
}
