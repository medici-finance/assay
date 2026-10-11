package gitmaintenance

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// regression: #2513
func TestEffectiveReflogRetention(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	// The inventory sees effective global policy; an invalid global pack setting
	// makes any leak of that same scope into native packing observable.
	global := []byte("[gc]\n reflogExpire = 14.days\n[gc \"refs/heads/Topic/*\"]\n reflogExpireUnreachable = 3.days\n[pack]\n allowPackReuse = not-valid\n")
	globalPath := filepath.Join(os.Getenv("HOME"), ".gitconfig")
	if err := os.WriteFile(globalPath, global, 0600); err != nil {
		t.Fatal(err)
	}
	before := diskImage(t, repo)
	inv, err := Inspect(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"gc.reflogexpire":                               "14.days",
		"gc.reflogexpireunreachable":                    "30 days (Git default)",
		"gc.refs/heads/Topic/*.reflogexpireunreachable": "3.days",
	}
	if !reflect.DeepEqual(inv.ReflogRecovery, want) {
		t.Fatalf("effective retention omitted or changed global policy: got %v want %v", inv.ReflogRecovery, want)
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatal("retention inspection changed repository state")
	}
	reflog := fixtureGit(t, repo, "reflog", "show", "--format=%H %gs", "HEAD")
	report, err := Run(context.Background(), repo, Options{})
	if err != nil {
		t.Fatalf("task environment inherited global packing poison: %v", err)
	}
	if len(report.Tasks) != 4 || report.After == nil || !reflect.DeepEqual(report.After.ReflogRecovery, want) {
		t.Fatalf("maintenance did not retain effective policy in its report: %#v", report)
	}
	for _, task := range report.Tasks {
		if task.Status != "completed" {
			t.Fatalf("task did not complete under scrubbed scope: %#v", task)
		}
	}
	if got := fixtureGit(t, repo, "reflog", "show", "--format=%H %gs", "HEAD"); got != reflog {
		t.Fatal("task execution changed reflog recovery")
	}
	if got, err := os.ReadFile(globalPath); err != nil || string(got) != string(global) {
		t.Fatal("inventory or task execution rewrote global policy")
	}
	fixtureGit(t, repo, "config", "gc.reflogExpire", "7.days")
	fixtureGit(t, repo, "config", "gc.refs/heads/Topic/*.reflogExpireUnreachable", "2.days")
	overridden, err := Inspect(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	want["gc.reflogexpire"] = "7.days"
	want["gc.refs/heads/Topic/*.reflogexpireunreachable"] = "2.days"
	if !reflect.DeepEqual(overridden.ReflogRecovery, want) {
		t.Fatalf("effective retention did not apply local precedence: got %v want %v", overridden.ReflogRecovery, want)
	}
}
