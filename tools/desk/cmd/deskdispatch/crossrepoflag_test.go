package main

// crossrepoflag_test.go — the explicit cross-repo dispatch (medici-finance/assay#2748): a brief
// homed (tracked) in one repo whose deliverable PR opens in another, named by --repo under the
// opt-in --cross-repo flag. Every alias, repo and stream is a synthetic example-* fixture; the
// tracking and deliverable checkouts are separate temp dirs, as in crossrepo_test.go.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2748
// TestCrossRepoFlagDispatches — the brief resolves to its HOME repo (by its brief-v2 id or the
// item-key prefix, published or not), --repo names where the PR opens, and --cross-repo says the
// split is intended. The worktree is cut from the --repo checkout, the claim and the mint land in
// --repo under the TRACKING key, and the prompt names the exact Cross-repo-brief: trailer line in
// place of a Brief: one (which would resolve against the wrong board).
func TestCrossRepoFlagDispatches(t *testing.T) {
	for _, c := range []struct {
		name, item, tracked, alias string
		front                      []string
	}{
		{"brief-v2 id", "example-stream/05", "**Tracked in:** `example-org/tracker`", "example-trk", []string{exampleV2ID}},
		{"item-key prefix", "example-trk:example-stream/05", "**Tracked in:** `example-org/tracker`", "example-trk", nil},
		{"unpublished home", "example-hidden:example-stream/05",
			"**Tracked in:** the tracking repo (alias `example-hidden`)", "example-hidden", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t) // --root's origin is medici-finance/assay (alias example-tool)
			trk := trackingCheckout(t, exampleRegistry, c.front...)
			s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
			mints := recordMint(t, home, nil)
			promptFile := filepath.Join(t.TempDir(), "p.md")
			err := cmdDispatch([]string{c.item, "--repo", "medici-finance/assay", "--cross-repo", "--root", root,
				"--claim-root", trk, "--brief", exampleBriefRel, "--kit", "worker", "--prompt-file", promptFile, "--quiet"})
			if err != nil {
				t.Fatalf("an explicit cross-repo dispatch must proceed: %v", err)
			}
			cc := claimCalls(s)
			if len(cc) != 1 {
				t.Fatalf("want exactly one claim acquire, got %v", cc)
			}
			want := "acquire " + c.alias + "--example-stream--05 --repo medici-finance/assay"
			if got := strings.Join(cc[0][1:], " "); got != want {
				t.Errorf("claim argv = %q, want %q (the TRACKING key, claimed in --repo)", got, want)
			}
			if len(*mints) != 1 || (*mints)[0] != "medici-finance/assay" {
				t.Errorf("token minted for %v, want exactly --repo", *mints)
			}
			if !s.ran("deskwt add") {
				t.Error("no worktree was cut from the --repo checkout")
			}
			prompt := readPrompt(t, promptFile)
			for _, w := range []string{
				"**Target repo:** `medici-finance/assay`",
				c.tracked,
				"`Cross-repo-brief: " + c.alias + ":example-stream/05`",
			} {
				if !strings.Contains(prompt, w) {
					t.Errorf("prompt is missing %q", w)
				}
			}
			absTrk, _ := filepath.Abs(trk)
			if strings.Contains(prompt, "`deskpr create --root "+absTrk+"`") {
				t.Error("the cross-repo-brief prompt must not point deskpr at the tracking checkout")
			}
		})
	}
}

// regression: #2748
// TestCrossRepoFlagNearMisses — --cross-repo widens exactly one shape. Every near-miss keeps an
// exit-5 refusal with nothing claimed, minted or cut: the flag never overrides an explicit
// deliverable, never stands in for a named tracking alias, never admits an unregistered --repo,
// and never skips the --root-origin witness.
func TestCrossRepoFlagNearMisses(t *testing.T) {
	for _, c := range []struct {
		name, item, repo, want string
		front                  []string
		registry               string // "" = exampleRegistry
	}{
		{"no --repo", "example-stream/05", "", "--cross-repo", []string{exampleV2ID}, ""},
		{"no alias at all", "example-stream/05", "medici-finance/assay", "--cross-repo", nil, ""},
		{"explicit deliverable_repo", "example-stream/05", "medici-finance/assay", "deliverable_repo",
			[]string{exampleV2ID, "deliverable_repo: example-con"}, ""},
		{"explicit homed-in", "example-stream/05", "medici-finance/assay", "homed-in",
			[]string{exampleV2ID, "homed-in: example-org/console"}, ""},
		{"unregistered --repo", "example-stream/05", "example-org/nope", "example-org/nope", []string{exampleV2ID}, ""},
		{"--repo is the home", "example-stream/05", "example-org/tracker", "tracking repo", []string{exampleV2ID}, ""},
		{"--root not --repo", "example-stream/05", "example-org/console", "HARD FAIL", []string{exampleV2ID}, ""},
		// --repo IS --root's origin, so only the registry's publish check can refuse it.
		{"--repo not published", "example-stream/05", "medici-finance/assay", "not published", []string{exampleV2ID},
			strings.Replace(exampleRegistry, "  example-tool:   {cell: example-cell, repo: medici-finance/assay}\n", "", 1)},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			reg := c.registry
			if reg == "" {
				reg = exampleRegistry
			}
			trk := trackingCheckout(t, reg, c.front...)
			s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
			mints := recordMint(t, home, nil)
			args := []string{c.item, "--cross-repo", "--root", root, "--claim-root", trk, "--brief", exampleBriefRel, "--quiet"}
			if c.repo != "" {
				args = append(args, "--repo", c.repo)
			}
			err := cmdDispatch(args)
			if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("this near-miss must be an exit-5 refusal naming %q: %v", c.want, err)
			}
			if len(claimCalls(s)) != 0 || s.ran("deskwt add") || len(*mints) != 0 {
				t.Errorf("the refusal must precede mint, claim and worktree; calls=%v mints=%v", s.calls, *mints)
			}
		})
	}
}

// regression: #2748
// TestCrossRepoFlagPhantom — a --cross-repo PR carries no `Brief:` line, so the phantom check must
// also read its `Cross-repo-brief:` link, or a second dispatch spawns a duplicate PR. The match is
// on the WHOLE <alias>:<stream>/<NN> value: the same stream/NN under another alias, or a local
// `Brief:`-less PR, is not this brief's delivery.
func TestCrossRepoFlagPhantom(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		refused bool
	}{
		{"open delivery", "Cross-repo-brief: example-trk:example-stream/05", true},
		{"another home", "Cross-repo-brief: example-con:example-stream/05", false},
		{"issue-only", "Issue: #7", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			trk := trackingCheckout(t, exampleRegistry, exampleV2ID)
			s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
			mints := recordMint(t, home, nil)
			withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
				return []deskkit.PRRef{{Number: 377, State: "OPEN", Body: "x\n" + tc.body + "\n"}}, nil
			})
			err := cmdDispatch([]string{"example-stream/05", "--repo", "medici-finance/assay", "--cross-repo",
				"--root", root, "--claim-root", trk, "--brief", exampleBriefRel, "--kit", "worker",
				"--prompt-file", filepath.Join(t.TempDir(), "p.md"), "--quiet"})
			if !tc.refused {
				if err != nil {
					t.Fatalf("not this brief's delivery, so the dispatch must proceed: %v", err)
				}
				return
			}
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "#377") {
				t.Fatalf("an open Cross-repo-brief: delivery must be a phantom refusal naming it: %v", err)
			}
			if len(claimCalls(s)) != 0 || s.ran("deskwt add") || len(*mints) != 0 {
				t.Errorf("a phantom reached the mint, claim or worktree; calls=%v mints=%v", s.calls, *mints)
			}
		})
	}
}
