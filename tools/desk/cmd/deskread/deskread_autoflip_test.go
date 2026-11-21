package main

// deskread_autoflip_test.go — the two kinds statusgen's GitLab auto-flip reads through
// (forge-neutral brief 18, #7202): commit-changes (commit → change numbers) and change-trust (a
// change's body-edit time). Same recording fake as the brief-33 rows.

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const testSHA = "0123456789abcdef0123456789abcdef01234567"

func (f *recForge) ListCommitChanges(repo deskkit.ForgeRepo, sha string) ([]int, error) {
	f.rec(recCall{Op: "ListCommitChanges", Repo: repo.Slug(), Detail: sha})
	if err := failFor(repo, 0); err != nil {
		return nil, err
	}
	if repo.Name == "empty" {
		return nil, nil
	}
	return []int{12, 9}, nil
}

func (f *recForge) PRTrustEvents(repo deskkit.ForgeRepo, number int) (*deskkit.TrustPayload, error) {
	f.rec(recCall{Op: "PRTrustEvents", Repo: repo.Slug(), Number: number})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	return &deskkit.TrustPayload{
		BodyEdited: time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC), Complete: true, BodyHistoryKnown: true,
		Events: []deskkit.ContentEvent{{Author: "reviewer", AuthorID: 7, CreatedAt: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)}},
	}, nil
}

// regression: #7202
func TestDeskreadCommitChanges(t *testing.T) {
	t.Run("numbers in any state", func(t *testing.T) {
		f := stubRec(t)
		env, code, errs := runEnvelope(t, "commit-changes", "--repo", "example-org/alpha", "--repo", "example-org/locked", "--sha", testSHA)
		if code != deskkit.ExitOK || len(env.Repos) != 1 || len(env.Partial) != 1 {
			t.Fatalf("exit=%d repos=%+v partial=%+v stderr=%s", code, env.Repos, env.Partial, errs)
		}
		if got := env.Repos[0].CommitChanges; len(got) != 2 || got[0] != 12 || got[1] != 9 {
			t.Errorf("commitChanges = %v, want [12 9] as served", got)
		}
		if c := f.recorded(); len(c) != 2 || c[0].Op != "ListCommitChanges" || c[0].Detail != testSHA {
			t.Errorf("calls = %+v, want ListCommitChanges with the stated sha", c)
		}
	})

	t.Run("an empty list is an answer", func(t *testing.T) {
		stubRec(t)
		out, code, _ := rawOut("commit-changes", "--repo", "example-org/empty", "--sha", testSHA)
		if code != deskkit.ExitOK || !strings.Contains(out, `"commitChanges": []`) {
			t.Errorf("exit=%d out=%s, want an explicit empty list", code, out)
		}
	})

	t.Run("other kinds never carry the key", func(t *testing.T) {
		stubRec(t)
		out, _, _ := rawOut("default-branch", "--repo", "example-org/alpha")
		if strings.Contains(out, "commitChanges") {
			t.Errorf("default-branch carries commitChanges: %s", out)
		}
	})
}

// regression: #7202
func TestDeskreadChangeTrust(t *testing.T) {
	f := stubRec(t)
	env, code, errs := runItemEnvelope(t, "change-trust", "--change", "example-org/alpha#7", "--change", "example-org/alpha#404")
	if code != deskkit.ExitOK || len(env.Items) != 1 || len(env.Partial) != 1 {
		t.Fatalf("exit=%d env=%+v stderr=%s", code, env, errs)
	}
	tr := env.Items[0].Trust
	if tr == nil || tr.BodyEditedAt != "2026-09-04T00:00:00Z" || !tr.Complete || !tr.BodyHistoryKnown || len(tr.Events) != 1 {
		t.Errorf("trust = %+v, want the change's body-edit time and events", tr)
	}
	for _, c := range f.recorded() {
		if c.Op != "PRTrustEvents" {
			t.Errorf("call %+v, want only PRTrustEvents — never the issue read", c)
		}
	}
}

// regression: #7202
func TestDeskreadAutoflipKindRefusals(t *testing.T) {
	for _, args := range [][]string{
		{"commit-changes", "--repo", "example-org/alpha"},
		{"commit-changes", "--repo", "example-org/alpha", "--sha", "0123abc"},
		{"commit-changes", "--repo", "example-org/alpha", "--sha", strings.Repeat("g", 40)},
		{"commit-changes", "--repo", "example-org/alpha", "--sha", testSHA, "--sha", testSHA},
		{"commit-changes", "--change", "example-org/alpha#7", "--sha", testSHA},
		{"default-branch", "--repo", "example-org/alpha", "--sha", testSHA},
		{"change-trust", "--issue", "example-org/alpha#7"},
		{"change-trust", "--change", "example-org/alpha#7", "--sha", testSHA},
		{"change-trust", "--change", "example-org/alpha#7", "--ci-workflow-token"},
	} {
		f := stubRec(t)
		out, code, _ := rawOut(args...)
		if code != deskkit.ExitRefused || out != "" || len(f.recorded()) != 0 {
			t.Errorf("%v → exit=%d stdout=%q calls=%+v, want exit 5, nothing written, zero calls", args, code, out, f.recorded())
		}
	}
}
