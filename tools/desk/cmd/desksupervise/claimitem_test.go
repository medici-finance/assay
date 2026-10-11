package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func claimItemBoard(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "docs", "streams", "example-stream")
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	board := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|---|---|---|---|---|---|\n| 03 | Third | 1 | M | in-progress | - | - |\n"
	if err = os.WriteFile(filepath.Join(dir, "README.md"), []byte(board), 0600); err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = wt.Add("docs/streams/example-stream/README.md"); err != nil {
		t.Fatal(err)
	}
	sha, err := wt.Commit("fixture board", &git.CommitOptions{Author: &object.Signature{Name: "Fixture", Email: "fixture@example.invalid", When: time.Now()}})
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.Storer.SetReference(plumbing.NewHashReference("refs/remotes/origin/main", sha)); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestClaimPayloadProductionReaderSelection(t *testing.T) {
	withForgeFixture(t, "org/repo", "github")
	root := claimItemBoard(t)
	now := mustNow(t)
	for _, tc := range []struct {
		key, item     string
		pr, issue     int
		closed, blind bool
	}{
		{key: "repo--example-stream--03", item: "example-stream/03"},
		{key: "repo--pr-7", item: "repo--pr-7", pr: 7},
		{key: "repo--pr-7--security", item: "repo--pr-7--security", pr: 7},
		{key: "repo--issue-8", item: "repo--issue-8", issue: 8},
		{key: "repo--pr-7", item: "repo--pr-7", pr: 7, closed: true},
		{key: "repo--issue-8", item: "repo--issue-8", issue: 8, closed: true},
		{key: "repo--missing-stream--03", item: "missing-stream/03", blind: true},
	} {
		t.Run(fmt.Sprintf("%s-closed%t", tc.key, tc.closed), func(t *testing.T) {
			ref := plumbing.NewHashReference(plumbing.ReferenceName(deskkit.DispatchClaimActiveRefsPrefix+tc.key), plumbing.NewHash("1111111111111111111111111111111111111111"))
			claims, _, err := claimPage([]*plumbing.Reference{ref}, "org/repo", now, 50, "", func(plumbing.Hash) (gitcore.TagPayload, error) {
				return gitcore.TagPayload{Message: "dispatch-claim " + tc.key + " owner=worker state=dispatched branch=topic", When: now.Add(-time.Minute)}, nil
			})
			if err != nil || len(claims) != 1 {
				t.Fatalf("payload: %v %+v", err, claims)
			}
			var prCalls, issueCalls int
			src := liveEligibilitySourceWith(root, func(c claimRecord) (loopengine.ClaimRecord, error) {
				return loopengine.ClaimRecord{Key: c.Key, Holder: c.Owner}, nil
			}, func(repo string, n int) (loopengine.PRState, error) {
				if n == 0 {
					return loopengine.PRState{Kind: deskkit.PROpen}, nil
				}
				prCalls++
				if repo != "org/repo" || n != tc.pr {
					t.Fatalf("wrong PR read %s#%d", repo, n)
				}
				kind := deskkit.PROpen
				if tc.closed {
					kind = deskkit.PRClosed
				}
				return loopengine.PRState{Kind: kind}, nil
			}, func(repo string, n int) (*deskkit.Issue, error) {
				issueCalls++
				if repo != "org/repo" || n != tc.issue {
					t.Fatalf("wrong issue read %s#%d", repo, n)
				}
				state := "open"
				if tc.closed {
					state = "closed"
				}
				return &deskkit.Issue{Number: n, State: state}, nil
			})
			eligible, results, blind, err := reconcile(claims, src, now, true, func(claimRecord) error { t.Fatal("dry-run release"); return nil }, noopArm, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if tc.blind {
				if !blind || len(eligible) != 0 || len(results) != 1 || results[0].Source != "board-row" {
					t.Fatalf("unreadable board admitted: %+v %+v %t", eligible, results, blind)
				}
				return
			}
			if blind {
				t.Fatalf("healthy payload never reached assessment: %+v", results)
			}
			if claims[0].PR != tc.pr {
				t.Fatalf("PR metadata=%d want %d", claims[0].PR, tc.pr)
			}
			if claims[0].Item != tc.item {
				t.Fatalf("item=%q want %q", claims[0].Item, tc.item)
			}
			if tc.closed {
				if len(results) != 1 || !results[0].Verdict.Terminal() {
					t.Fatalf("closed target not terminal: %+v", results)
				}
			} else if len(eligible) != 1 {
				t.Fatalf("healthy payload not eligible: %+v", results)
			}
			if (prCalls > 0) != (tc.pr > 0) || (issueCalls > 0) != (tc.issue > 0) {
				t.Fatalf("reader selection: pr=%d issue=%d", prCalls, issueCalls)
			}
		})
	}
}

func TestClaimTargetBindingAndUncertainty(t *testing.T) {
	dir := withForgeFixture(t, "org/repo", "github")
	file := filepath.Join(dir, "roster.env")
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("ASSAY_REPO_ALIASES=org/repo=rp:\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	deskkit.ReloadConfig()
	for _, key := range []string{"rp--example-stream--03", "repo--example-stream--03", "rp--pr-review--03"} {
		if _, err := decodeClaimTarget(key, "org/repo"); err != nil {
			t.Fatalf("known prefix/form rejected: %s: %v", key, err)
		}
	}
	for _, key := range []string{"foreign--example-stream--03", "repo--../escape--03", "repo--issue-0", "repo--issue-+7", "repo--pr-0", "repo--untyped"} {
		if _, err := decodeClaimTarget(key, "org/repo"); err == nil {
			t.Fatalf("ambiguous/foreign target admitted: %s", key)
		}
	}
	for _, mode := range []string{"unreadable", "wrong-kind", "unknown-state", "held"} {
		c := fixtureClaim("repo--issue-8", nil)
		c.Repo = "org/repo"
		src := liveEligibilitySourceWith(t.TempDir(), func(c claimRecord) (loopengine.ClaimRecord, error) {
			return loopengine.ClaimRecord{Holder: c.Owner}, nil
		}, func(string, int) (loopengine.PRState, error) {
			t.Fatal("issue used PR endpoint")
			return loopengine.PRState{}, nil
		}, func(string, int) (*deskkit.Issue, error) {
			issue := &deskkit.Issue{Number: 8, State: "open"}
			switch mode {
			case "unreadable":
				return nil, fmt.Errorf("offline read failed")
			case "wrong-kind":
				issue.IsPullRequest = true
			case "unknown-state":
				issue.State = "uncertain"
			case "held":
				issue.Labels = []string{"needs-decision"}
			}
			return issue, nil
		})
		verdict, err := src(c)
		if mode == "held" {
			if err != nil || !verdict.Held() {
				t.Fatalf("issue hold ignored: %+v %v", verdict, err)
			}
		} else if source, ok := loopengine.BlindSource(err); !ok || source != "issue" {
			t.Fatalf("%s must remain issue-blind: %+v %v", mode, verdict, err)
		}
	}
}
