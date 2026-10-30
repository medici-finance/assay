package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the actual bracket/runner, with an intentionally unusable .git
// pointer like a host linked worktree mounted in a container. All Git objects
// used by the baseline must come from the bundle, not an accessible host repo.
func TestContainerHistoryFailFirst(t *testing.T) {
	fx := newContainerFixture(t)
	clock := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	savedNow := nowFunc
	nowFunc = func() time.Time { return clock }
	t.Cleanup(func() { nowFunc = savedNow })
	body := "# Fixture\n\n## Verify\n\n| # | Command | Expect | Class |\n|---|---|---|---|\n| 1 | `test -f delivered.txt` | risk-bearing exit 0 | check:ci |\n\n## Evidence\n\n## Review\n"
	putFile(t, fx.brief, body)
	runGit(t, fx.home, "add", "docs/brief.md")
	runGit(t, fx.home, "commit", "-m", "original obligation")
	base := gitOutT(t, fx.home, "rev-parse", "HEAD")
	putFile(t, filepath.Join(fx.home, "delivered.txt"), "delivered\n")
	runGit(t, fx.home, "add", "delivered.txt")
	runGit(t, fx.home, "commit", "-m", "delivery")
	head := gitOutT(t, fx.home, "rev-parse", "HEAD")
	runGit(t, fx.home, "update-ref", remoteMainRef, head)
	check := checkJSON(1, fixtureRun, head, "docs/brief.md", "m")
	putFile(t, fx.check, check)
	bundle := filepath.Join(t.TempDir(), "history.pack")
	res, stderr := captureVerifyrun(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt, "--history-out", bundle})
	if res.code != 0 {
		t.Fatalf("open: %d %s", res.code, stderr)
	}
	gitdir := filepath.Join(t.TempDir(), "host.git")
	if err := os.Rename(filepath.Join(fx.home, ".git"), gitdir); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(fx.home, ".git"), "gitdir: /unmounted-host/git/worktrees/verifier\n")
	before, err := homeTreeDigest(fx.home)
	if err != nil {
		t.Fatal(err)
	}
	args := append(fx.runArgs(), "--fail-first", "--base", base)
	res, stderr = captureVerifyrun(t, args)
	if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "names no commit") {
		t.Fatalf("without history, want original refusal: %d %s", res.code, stderr)
	}
	res, stderr = captureVerifyrun(t, append(args, "--history-pack", bundle))
	if res.code != 0 {
		t.Fatalf("with history: %d %s", res.code, stderr)
	}
	witness := getFile(t, fx.witness)
	if !strings.Contains(witness, "pass exit=0 sandbox=container-netns") || !strings.Contains(witness, "base="+base[:treeSHALen]+" red rc=1") {
		t.Fatalf("missing real head pass / baseline red: %s", witness)
	}
	after, err := homeTreeDigest(fx.home)
	if err != nil || after != before {
		t.Fatalf("admitted home changed: before=%s after=%s err=%v", before, after, err)
	}
	if err := os.Remove(filepath.Join(fx.home, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(gitdir, filepath.Join(fx.home, ".git")); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	res, stderr = captureVerifyrun(t, []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness})
	if res.code != 0 {
		t.Fatalf("close: %d %s", res.code, stderr)
	}
}

func TestHistoryRefusesInputs(t *testing.T) {
	fx := newContainerFixture(t)
	head := gitOutT(t, fx.home, "rev-parse", "HEAD")
	bundle := filepath.Join(t.TempDir(), "history.pack")
	if err := exportContainerHistory(fx.home, bundle, head); err != nil {
		t.Fatal(err)
	}
	if err := exportContainerHistory(fx.home, bundle, head); err == nil {
		t.Fatal("overwrote an existing bundle")
	}
	if err := exportContainerHistory(fx.home, filepath.Join(fx.home, "history.pack"), head); err == nil {
		t.Fatal("exported into the admitted home")
	}
	if _, _, cleanup, err := importContainerHistory(fx.home, fx.brief, bundle, fixtureSource); err == nil {
		cleanup()
		t.Fatal("accepted a bundle from a different source")
	}
	putFile(t, bundle, "corrupt bundle\n")
	if _, _, cleanup, err := importContainerHistory(fx.home, fx.brief, bundle, head); err == nil {
		cleanup()
		t.Fatal("accepted a corrupt bundle")
	}
	res, stderr := captureVerifyrun(t, []string{"--brief", fx.brief, "--history-pack", bundle, "--fail-first"})
	if res.code != verifyrunExitUsageError || !strings.Contains(stderr, "only to a container-netns") {
		t.Fatalf("history accepted outside container fail-first: %d %s", res.code, stderr)
	}
}

func TestHistoryBaselineGuards(t *testing.T) {
	fx := newContainerFixture(t)
	beforeBrief := gitOutT(t, fx.home, "rev-list", "--max-parents=0", "HEAD")
	// A later obligation must still reject a baseline predating the brief.
	newBrief := filepath.Join(fx.home, "docs", "later.md")
	putFile(t, newBrief, getFile(t, fx.brief))
	runGit(t, fx.home, "add", "docs/later.md")
	runGit(t, fx.home, "commit", "-m", "later brief")
	head := gitOutT(t, fx.home, "rev-parse", "HEAD")
	bundle := filepath.Join(t.TempDir(), "history.pack")
	if err := exportContainerHistory(fx.home, bundle, head); err != nil {
		t.Fatal(err)
	}
	history, brief, cleanup, err := importContainerHistory(fx.home, newBrief, bundle, head)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := failFirstBaseRev(history, beforeBrief, brief); err == nil {
		t.Fatal("history transport bypassed the brief-at-base guard")
	}
	if _, err := failFirstBaseRev(history, strings.Repeat("e", 40), brief); err == nil {
		t.Fatal("history transport invented a missing baseline")
	}
	if _, err := os.Stat(filepath.Join(history, ".git", "config")); err != nil {
		t.Fatal(err)
	}
	if bare := gitOutT(t, history, "config", "--get", "core.bare"); bare != "false" {
		t.Fatalf("history is not a private working repository: %s", bare)
	}
}

func TestHistoryNoHostConfig(t *testing.T) {
	fx := newContainerFixture(t)
	head := gitOutT(t, fx.home, "rev-parse", "HEAD")
	runGit(t, fx.home, "config", "credential.helper", "sensitive-host-helper")
	runGit(t, fx.home, "config", "remote.origin.url", "https://example.invalid/private-remote")
	putFile(t, filepath.Join(fx.home, ".git", "hooks", "post-checkout"), "host hook must not travel\n")
	bundle := filepath.Join(t.TempDir(), "history.pack")
	if err := exportContainerHistory(fx.home, bundle, head); err != nil {
		t.Fatal(err)
	}
	history, _, cleanup, err := importContainerHistory(fx.home, fx.brief, bundle, head)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	config := getFile(t, filepath.Join(history, ".git", "config"))
	if strings.Contains(config, "sensitive-host-helper") || strings.Contains(config, "private-remote") {
		t.Fatalf("host config crossed history transport: %s", config)
	}
	if _, err := os.Stat(filepath.Join(history, ".git", "hooks", "post-checkout")); !os.IsNotExist(err) {
		t.Fatalf("host hook crossed history transport: %v", err)
	}
	t.Setenv("TMPDIR", fx.home)
	if _, _, cleanup, err := importContainerHistory(fx.home, fx.brief, bundle, head); err == nil {
		cleanup()
		t.Fatal("accepted writable history scratch inside admitted home")
	}
}

func TestHistoryIncompleteBundle(t *testing.T) {
	fx := newContainerFixture(t)
	putFile(t, filepath.Join(fx.home, "later.txt"), "later\n")
	runGit(t, fx.home, "add", "later.txt")
	runGit(t, fx.home, "commit", "-m", "later")
	head := gitOutT(t, fx.home, "rev-parse", "HEAD")
	partial := filepath.Join(t.TempDir(), "partial.pack")
	if err := exportContainerHistory(fx.home, partial, head); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(partial)
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, partial, string(raw[:len(raw)/2]))
	if _, _, cleanup, err := importContainerHistory(fx.home, fx.brief, partial, head); err == nil {
		cleanup()
		t.Fatal("accepted a truncated history pack")
	}
	putFile(t, filepath.Join(fx.home, ".git", "shallow"), head+"\n")
	if err := exportContainerHistory(fx.home, filepath.Join(t.TempDir(), "shallow.bundle"), head); err == nil {
		t.Fatal("exported incomplete shallow history")
	}
}

func TestHistoryImportStopsRows(t *testing.T) {
	for _, kind := range []string{"corrupt", "missing", "inside-home"} {
		t.Run(kind, func(t *testing.T) {
			fx := newContainerFixture(t)
			pack := filepath.Join(t.TempDir(), "input.pack")
			want := "invalid history pack"
			switch kind {
			case "corrupt":
				putFile(t, pack, "corrupt pack")
			case "missing":
				want = "readable regular file"
			case "inside-home":
				pack = filepath.Join(fx.home, "input.pack")
				putFile(t, pack, "corrupt pack")
				want = "outside the admitted home"
			}
			fx.writeBracketReceipt(t, validCheck)
			before := getFile(t, fx.brief)
			args := append(fx.runArgs(), "--fail-first", "--base", fixtureSource, "--history-pack", pack)
			res, stderr := captureVerifyrun(t, args)
			if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "--history-pack:") || !strings.Contains(stderr, want) {
				t.Fatalf("import refusal not preserved: exit=%d stderr=%s", res.code, stderr)
			}
			fx.assertNothingRan(t, before)
		})
	}
}
