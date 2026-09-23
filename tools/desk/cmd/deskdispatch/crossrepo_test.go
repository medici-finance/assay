package main

// crossrepo_test.go — the cross-repo dispatch contract (deliverable.go), the phantom precondition's
// ordering (phantom.go), and the rework-after-merge follow-up path.
//
// Every alias, repo and stream below is a synthetic example-* fixture. The tracking checkout and the
// deliverable checkout are two separate temp dirs, exactly as in the field: the brief and the alias
// registry live in the tracking checkout (--claim-root), the worktree is cut from the deliverable
// checkout (--root).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const exampleRegistry = `schema: graph-repos-v1
cell: example-cell
self: example-trk
repos:
  example-trk:    {cell: example-cell, repo: example-org/tracker}
  example-tool:   {cell: example-cell, repo: medici-finance/assay}
  example-con:    {cell: example-cell, repo: example-org/console}
  example-hidden: {cell: example-cell, repo: null, unpublished: true}
`

const exampleBriefRel = "docs/streams/example-stream/brief-05-example.md"

// trackingCheckout builds a tracking checkout carrying the registry (unless registry == ""), the
// consumer scripts, and one brief whose frontmatter carries the given extra lines.
func trackingCheckout(t *testing.T, registry string, frontmatter ...string) string {
	t.Helper()
	trk := t.TempDir()
	plantScripts(t, trk)
	if registry != "" {
		p := filepath.Join(trk, filepath.FromSlash(registryRel))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(registry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bp := filepath.Join(trk, filepath.FromSlash(exampleBriefRel))
	if err := os.MkdirAll(filepath.Dir(bp), 0o700); err != nil {
		t.Fatal(err)
	}
	gate := "gate: model\n"
	for _, l := range frontmatter {
		if strings.HasPrefix(l, "gate:") {
			gate = "" // the caller's own gate line wins
		}
	}
	body := "---\ntitle: example\n" + gate + strings.Join(frontmatter, "\n") + "\n---\n\n# Example brief\n"
	if err := os.WriteFile(bp, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return trk
}

// recordMint rebinds the claim step's token mint to record the repo each mint was taken for.
func recordMint(t *testing.T, home string, order *[]string) *[]string {
	t.Helper()
	var repos []string
	old := mintTokenFn
	mintTokenFn = func(role, repo string) (string, string, error) {
		repos = append(repos, repo)
		if order != nil {
			*order = append(*order, "mint")
		}
		return stubMintedToken, stubMintedTokenPath(t, home), nil
	}
	t.Cleanup(func() { mintTokenFn = old })
	return &repos
}

// recordAdmission rebinds the admission gate's enable probe to record that it was consulted. It
// answers "off", so a consulted gate is inert and the dispatch proceeds as it would by default.
func recordAdmission(t *testing.T, order *[]string) {
	t.Helper()
	old := admissionEnabledFn
	admissionEnabledFn = func() (bool, string) {
		*order = append(*order, "admission")
		return false, "test"
	}
	t.Cleanup(func() { admissionEnabledFn = old })
}

func claimCalls(s *stub) [][]string {
	var out [][]string
	for _, c := range s.calls {
		if len(c) > 1 && strings.HasSuffix(c[0], "dispatch-claim.sh") && c[1] == "acquire" {
			out = append(out, c)
		}
	}
	return out
}

func readPrompt(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("prompt file: %v", err)
	}
	return string(b)
}

// TestCrossRepoDeliverableRepoResolvesWorktreeClaimAndToken — the happy path. A brief tracked in the
// tracking checkout declares `deliverable_repo: example-tool`; the alias resolves through the
// registry to the deliverable repo, which IS --root's origin. The claim and the token are taken for
// the RESOLVED repo, the claim key carries the tracking alias, and the prompt points deskpr at the
// tracking checkout.
func TestCrossRepoDeliverableRepoResolvesWorktreeClaimAndToken(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	trk := trackingCheckout(t, exampleRegistry, "deliverable_repo: example-tool")
	s.replies = happyReplies("/private/tmp/worker-home")
	mints := recordMint(t, home, nil)

	promptFile := filepath.Join(t.TempDir(), "p.md")
	err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk,
		"--brief", exampleBriefRel, "--kit", "worker", "--prompt-file", promptFile, "--quiet"})
	if err != nil {
		t.Fatalf("a correctly-rooted cross-repo dispatch must succeed: %v", err)
	}
	cc := claimCalls(s)
	if len(cc) != 1 {
		t.Fatalf("want exactly one claim acquire, got %v", cc)
	}
	if got := strings.Join(cc[0][1:], " "); got != "acquire example-trk--example-stream--05 --repo medici-finance/assay" {
		t.Errorf("claim argv = %q — want the TRACKING alias in the key and the RESOLVED repo as --repo", got)
	}
	if len(*mints) != 1 || (*mints)[0] != "medici-finance/assay" {
		t.Errorf("token minted for %v — want exactly the resolved deliverable repo", *mints)
	}
	if !s.ran("deskwt add example-stream-05") {
		t.Error("no worktree was cut from the deliverable checkout")
	}
	prompt := readPrompt(t, promptFile)
	absTrk, _ := filepath.Abs(trk)
	for _, want := range []string{
		"**Target repo:** `medici-finance/assay`",
		"**Tracked in:** `example-org/tracker`",
		"`deskpr create --root " + absTrk + "`",
		"CROSS-REPO delivery",
		"alias `example-tool`",
		"**Checkout base:** `" + root + "`",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}

// TestCrossRepoMismatchHardFailsBeforeAnything — the class this contract exists to end. The brief's
// deliverable alias resolves to one repo; --root is a checkout of another. HARD FAIL: exit 5, the
// message names the alias and both repos, and NOTHING durable happened — no mint, no claim, no
// worktree.
func TestCrossRepoMismatchHardFailsBeforeAnything(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	trk := trackingCheckout(t, exampleRegistry, "deliverable_repo: example-con")
	s.replies = happyReplies("/private/tmp/worker-home") // --root's origin is medici-finance/assay
	mints := recordMint(t, home, nil)

	err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk,
		"--brief", exampleBriefRel, "--kit", "worker", "--quiet"})
	if err == nil {
		t.Fatal("a deliverable repo that is not --root's origin must HARD-FAIL, never warn and proceed")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("mismatch must be exit 5, got %d: %v", deskkit.ExitCodeOf(err), err)
	}
	for _, want := range []string{"HARD FAIL", `"example-con"`, "example-org/console", "medici-finance/assay"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q: %v", want, err)
		}
	}
	if len(claimCalls(s)) != 0 || s.ran("deskwt add") || len(*mints) != 0 {
		t.Errorf("a hard fail must precede mint, claim and worktree; calls=%v mints=%v", s.calls, *mints)
	}
}

// The --repo flag is a second witness: a --repo naming a different repo than the alias resolves to
// is the same hard fail.
func TestCrossRepoRepoFlagMismatchHardFails(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	trk := trackingCheckout(t, exampleRegistry, "deliverable_repo: example-tool")
	s.replies = happyReplies("/private/tmp/worker-home")

	err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk, "--repo", "example-org/console",
		"--brief", exampleBriefRel, "--quiet"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || err == nil || !strings.Contains(err.Error(), "HARD FAIL") {
		t.Fatalf("--repo disagreeing with the resolved alias must hard-fail (exit 5): %v", err)
	}
	if len(claimCalls(s)) != 0 || s.ran("deskwt add") {
		t.Error("the hard fail must precede the claim and the worktree")
	}
}

// TestCrossRepoUnregisteredAliasIsRefusedByName — an alias the registry does not define is a
// REFUSAL naming the missing alias, whether it came from the brief or the item-key prefix. It is
// never resolved by resemblance to a known repo name.
func TestCrossRepoUnregisteredAliasIsRefusedByName(t *testing.T) {
	for _, tc := range []struct {
		name, item string
		front      []string
	}{
		{"deliverable_repo", "example-stream/05", []string{"deliverable_repo: example-nope"}},
		{"item-key prefix", "example-nope:example-stream/05", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			trk := trackingCheckout(t, exampleRegistry, tc.front...)
			s.replies = happyReplies("/private/tmp/worker-home")
			args := []string{tc.item, "--root", root, "--claim-root", trk, "--quiet"}
			if tc.front != nil {
				args = append(args, "--brief", exampleBriefRel)
			}
			err := cmdDispatch(args)
			if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Fatalf("an unregistered alias must be REFUSED (exit 5), got %v", err)
			}
			if !strings.Contains(err.Error(), `"example-nope"`) || !strings.Contains(err.Error(), "not registered") {
				t.Errorf("the refusal must name the missing alias: %v", err)
			}
			if len(s.calls) != 0 {
				t.Errorf("an unregistered alias reached a child process: %v", s.calls)
			}
		})
	}
}

// An alias the registry reserves but does not publish, and a missing registry, are
// COULD-NOT-CHECK (exit 6) — never a guessed repo, never a pass.
func TestCrossRepoUnresolvableAliasIsCouldNotCheck(t *testing.T) {
	for _, tc := range []struct {
		name, registry string
	}{
		{"unpublished alias", exampleRegistry},
		{"no registry at the claim root", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			trk := trackingCheckout(t, tc.registry, "deliverable_repo: example-hidden")
			s.replies = happyReplies("/private/tmp/worker-home")
			err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk,
				"--brief", exampleBriefRel, "--quiet"})
			if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
				t.Fatalf("want could-not-check (exit 6), got %v", err)
			}
			if len(claimCalls(s)) != 0 || s.ran("deskwt add") {
				t.Error("could-not-check must precede the claim and the worktree")
			}
		})
	}
}

// TestDeliverableRepoHomedInResolvesThroughTheRegistry — `homed-in: <owner>/<name>` is accepted only
// when that repo is REGISTERED under an alias; an unregistered one is refused, not trusted.
func TestDeliverableRepoHomedInResolvesThroughTheRegistry(t *testing.T) {
	t.Run("registered", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		trk := trackingCheckout(t, exampleRegistry, "homed-in: medici-finance/assay")
		s.replies = happyReplies("/private/tmp/worker-home")
		if err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk,
			"--brief", exampleBriefRel, "--quiet", "--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
			t.Fatalf("a registered homed-in repo must resolve: %v", err)
		}
	})
	t.Run("unregistered", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		trk := trackingCheckout(t, exampleRegistry, "homed-in: example-org/unlisted")
		s.replies = happyReplies("/private/tmp/worker-home")
		err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk,
			"--brief", exampleBriefRel, "--quiet"})
		if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "example-org/unlisted") {
			t.Fatalf("an unregistered homed-in repo must be refused by name: %v", err)
		}
	})
	t.Run("conflicts with deliverable_repo", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		trk := trackingCheckout(t, exampleRegistry, "deliverable_repo: example-tool", "homed-in: example-org/console")
		s.replies = happyReplies("/private/tmp/worker-home")
		err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk,
			"--brief", exampleBriefRel, "--quiet"})
		if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("two different declared deliverable repos must be refused: %v", err)
		}
	})
}

// TestDeliverableRepoItemKeyPrefix — `<alias>:<stream>/<NN>`. The alias names the TRACKING repo; with
// no deliverable declared the deliverable is that same repo, so dispatching it from a checkout of
// another repo is the misroute shape and hard-fails, while a matching checkout dispatches with the
// alias in the claim key.
func TestDeliverableRepoItemKeyPrefix(t *testing.T) {
	t.Run("matching checkout", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		trk := trackingCheckout(t, exampleRegistry)
		s.replies = happyReplies("/private/tmp/worker-home")
		if err := cmdDispatch([]string{"example-tool:example-stream/05", "--root", root, "--claim-root", trk,
			"--quiet", "--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
			t.Fatalf("an alias-prefixed item on its own repo's checkout must dispatch: %v", err)
		}
		cc := claimCalls(s)
		if len(cc) != 1 || cc[0][2] != "example-tool--example-stream--05" {
			t.Errorf("claim key must carry the item's alias: %v", cc)
		}
		if !s.ran("deskwt add example-stream-05") {
			t.Error("the worktree name must derive from the key WITHOUT the alias prefix")
		}
	})
	t.Run("tracked elsewhere, dispatched here", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		trk := trackingCheckout(t, exampleRegistry)
		s.replies = happyReplies("/private/tmp/worker-home")
		err := cmdDispatch([]string{"example-trk:example-stream/05", "--root", root, "--claim-root", trk, "--quiet"})
		if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "example-org/tracker") {
			t.Fatalf("a tracking-repo item with no deliverable declared, dispatched from another repo's "+
				"checkout, must hard-fail naming the tracking repo: %v", err)
		}
		if len(claimCalls(s)) != 0 || s.ran("deskwt add") {
			t.Error("the hard fail must precede the claim and the worktree")
		}
	})
}

// An item that declares no alias anywhere keeps the legacy path: no registry is needed or read.
func TestCrossRepoNoAliasIsTheLegacyPath(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root) // no registry anywhere
	s.replies = happyReplies("/private/tmp/worker-home")
	if err := cmdDispatch([]string{"example-stream/05", "--root", root, "--quiet",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
		t.Fatalf("an item with no alias must dispatch exactly as before: %v", err)
	}
}

// TestPhantomPreconditionPrecedesAdmissionMintAndClaim — the phantom check is a DISPATCHER
// precondition: it runs before the admission (slot) gate, the token mint and the durable claim, so a
// phantom never costs a slot. The positive control (no PR) proves the recorders are live: there the
// admission gate and the mint ARE reached, AFTER the phantom read.
func TestPhantomPreconditionPrecedesAdmissionMintAndClaim(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prs     []deskkit.PRRef
		refused bool
	}{
		{"phantom", []deskkit.PRRef{{Number: 377, State: "OPEN", Body: "Brief: example-stream/05"}}, true},
		{"control: no PR", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies("/private/tmp/worker-home")
			var order []string
			recordMint(t, home, &order)
			recordAdmission(t, &order)
			withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
				order = append(order, "phantom")
				return tc.prs, nil
			})
			err := cmdDispatch([]string{"example-stream/05", "--root", root, "--repo", allowedRepo, "--kit", "worker",
				"--quiet", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			if tc.refused {
				if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
					t.Fatalf("phantom must be refused (exit 5): %v", err)
				}
				if strings.Join(order, ",") != "phantom" {
					t.Errorf("order = %v — nothing may run after a phantom refusal (no admission, no mint)", order)
				}
				if len(claimCalls(s)) != 0 || s.ran("deskwt add") {
					t.Error("a phantom reached the claim or the worktree")
				}
				return
			}
			if err != nil {
				t.Fatalf("control dispatch failed: %v", err)
			}
			if strings.Join(order, ",") != "phantom,mint,admission" {
				t.Errorf("order = %v — want the phantom read FIRST, then the mint and the admission gate", order)
			}
		})
	}
}

// TestPhantomPreconditionReadsTheDeliverableRepo — for a cross-repo item the phantom check reads the
// RESOLVED deliverable repo's PRs (where the worker's PR lands), and the deliverable-repo check runs
// before it: a mismatched checkout never reaches a forge read at all.
func TestPhantomPreconditionReadsTheDeliverableRepo(t *testing.T) {
	t.Run("reads the resolved repo", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		trk := trackingCheckout(t, exampleRegistry, "deliverable_repo: example-tool")
		s.replies = happyReplies("/private/tmp/worker-home")
		var read []string
		withRepresentedPRs(t, func(repo string) ([]deskkit.PRRef, error) {
			read = append(read, repo)
			return nil, nil
		})
		if err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk, "--brief", exampleBriefRel,
			"--quiet", "--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
			t.Fatal(err)
		}
		if len(read) != 1 || read[0] != "medici-finance/assay" {
			t.Errorf("phantom check read %v — want the resolved deliverable repo", read)
		}
	})
	t.Run("mismatch precedes any forge read", func(t *testing.T) {
		s := &stub{}
		_, root := s.install(t)
		trk := trackingCheckout(t, exampleRegistry, "deliverable_repo: example-con")
		s.replies = happyReplies("/private/tmp/worker-home")
		withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
			t.Error("the phantom transport was read for a dispatch the deliverable check must already refuse")
			return nil, nil
		})
		if err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk, "--brief", exampleBriefRel,
			"--quiet"}); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("want the hard fail, got %v", err)
		}
	})
}

// TestReworkAfterMergeIsFollowUp — a row awaiting implementer rework whose brief's PR is already
// MERGED dispatches as a FOLLOW-UP on a NEW branch, never under the merged branch's name, and the
// prompt says so.
func TestReworkAfterMergeIsFollowUp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{
			{Number: 360, State: "MERGED", Body: "Brief: example-stream/05"},
			{Number: 372, State: "MERGED", Body: "Brief: example-stream/05"},
		}, nil
	})
	promptFile := filepath.Join(t.TempDir(), "p.md")
	if err := cmdDispatch([]string{"example-stream/05", "--root", root, "--repo", allowedRepo, "--kit", "worker",
		"--rework", "--quiet", "--prompt-file", promptFile}); err != nil {
		t.Fatalf("a rework row with a merged PR must dispatch as a follow-up: %v", err)
	}
	if !s.ran("--branch feat/example-stream-05-followup-372 ") {
		t.Errorf("worktree not cut on the follow-up branch of the NEWEST merged PR; calls: %v", s.calls)
	}
	for _, c := range s.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "deskwt add") && strings.Contains(j, "--branch feat/example-stream-05 ") {
			t.Errorf("the worktree was cut under the MERGED branch's own name: %s", j)
		}
	}
	prompt := readPrompt(t, promptFile)
	for _, want := range []string{"FOLLOW-UP", "`medici-finance/assay#372`", "never a resume",
		"**Branch:** `feat/example-stream-05-followup-372`"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("follow-up prompt is missing %q", want)
		}
	}
}

// Without --rework a MERGED PR is refused as DELIVERED — never told to "resume", which a merged PR
// cannot be. An OPEN PR is still a resume, rework or not.
func TestReworkMergedWithoutFlagIsDeliveredNotResume(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 372, State: "MERGED", Body: "Brief: example-stream/05"}}, nil
	})
	_, err := phantomCheck(dispatchOpts{item: "example-stream/05", kit: "worker"}, allowedRepo)
	if err == nil || !strings.Contains(err.Error(), "DELIVERED") || strings.Contains(err.Error(), "Resume the PR") {
		t.Fatalf("a merged PR must refuse as DELIVERED and point at --rework, never at a resume: %v", err)
	}
	if !strings.Contains(err.Error(), "--rework") {
		t.Errorf("the refusal must name the --rework path: %v", err)
	}

	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{
			{Number: 372, State: "MERGED", Body: "Brief: example-stream/05"},
			{Number: 380, State: "OPEN", Body: "Brief: example-stream/05"},
		}, nil
	})
	f, err := phantomCheck(dispatchOpts{item: "example-stream/05", kit: "worker", rework: true}, allowedRepo)
	if err == nil || f.pr != 0 || !strings.Contains(err.Error(), "--pr 380") {
		t.Fatalf("an OPEN PR is resumed, even on a rework row: follow-up=%v err=%v", f, err)
	}
}

// The follow-up guard refuses the exact at-fault shape: an explicit --branch equal to the merged
// work's conventional branch. And --rework is refused where it is meaningless.
func TestReworkRefusesTheMergedBranchAndBadPairings(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 372, State: "MERGED", Body: "Brief: example-stream/05"}}, nil
	})
	err := cmdDispatch([]string{"example-stream/05", "--root", root, "--repo", allowedRepo, "--rework",
		"--branch", "feat/example-stream-05", "--quiet"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || err == nil {
		t.Fatalf("re-cutting the merged branch's name must be refused: %v", err)
	}
	if len(claimCalls(s)) != 0 || s.ran("deskwt add") {
		t.Error("the refusal must precede the claim and the worktree")
	}
	for _, args := range [][]string{
		{"--rework", "--pr", "12"},
		{"--rework", "--kit", "review"},
	} {
		err := cmdDispatch(append([]string{"example-stream/05", "--root", root, "--repo", allowedRepo, "--quiet"}, args...))
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Errorf("%v must be refused pre-claim: %v", args, err)
		}
	}
}

// SEC-1 — every registry value that can reach the claim key, the claim/token repo or the prompt is
// validated at load, before any child process: `self`, each alias key, and each `repo:`. A value
// carrying a leading dash or an embedded newline is REFUSED (exit 5) and never interpolated.
func TestCrossRepoRegistryValuesAreValidatedBeforeUse(t *testing.T) {
	for _, tc := range []struct{ name, registry string }{
		{"self with a leading dash and a newline", strings.Replace(exampleRegistry,
			"self: example-trk", `self: "-x\n## PROBE-LINE"`, 1)},
		{"self that is not a registered alias", strings.Replace(exampleRegistry,
			"self: example-trk", "self: example-ghost", 1)},
		{"repo value carrying a newline", strings.Replace(exampleRegistry,
			"repo: example-org/tracker}", `repo: "example-org/tracker\n## PROBE-LINE"}`, 1)},
		{"repo value that is not owner/name", strings.Replace(exampleRegistry,
			"repo: example-org/tracker}", `repo: "https://example.com/x/y"}`, 1)},
		{"alias key outside the grammar", strings.Replace(exampleRegistry,
			"  example-con:", "  Example Con:", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			trk := trackingCheckout(t, tc.registry, "deliverable_repo: example-tool")
			s.replies = happyReplies("/private/tmp/worker-home")
			promptFile := filepath.Join(t.TempDir(), "p.md")
			err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk,
				"--brief", exampleBriefRel, "--quiet", "--prompt-file", promptFile})
			if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Fatalf("a malformed registry value must be REFUSED (exit 5), got %v", err)
			}
			if len(s.calls) != 0 {
				t.Errorf("a malformed registry reached a child process: %v", s.calls)
			}
			if _, statErr := os.Stat(promptFile); statErr == nil {
				t.Error("a prompt was emitted from a malformed registry")
			}
		})
	}
}

// SEC advisory — once the resolved repo matches --root's origin case-insensitively, the claim and
// the mint use the ORIGIN's canonical spelling, never the registry's.
func TestCrossRepoUsesTheCanonicalOriginSlug(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	reg := strings.Replace(exampleRegistry, "repo: medici-finance/assay}", "repo: Medici-Finance/Assay}", 1)
	trk := trackingCheckout(t, reg, "deliverable_repo: example-tool")
	s.replies = happyReplies("/private/tmp/worker-home")
	mints := recordMint(t, home, nil)
	if err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk, "--brief", exampleBriefRel,
		"--quiet", "--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
		t.Fatal(err)
	}
	cc := claimCalls(s)
	if len(cc) != 1 || cc[0][len(cc[0])-1] != "medici-finance/assay" {
		t.Errorf("claim --repo must be the origin's canonical slug: %v", cc)
	}
	if len(*mints) != 1 || (*mints)[0] != "medici-finance/assay" {
		t.Errorf("mint repo must be the origin's canonical slug: %v", *mints)
	}
}

// COR-1 — a brief found only under --claim-root (the tracking checkout) is the SAME file every reader
// uses: a `gate: human` there fires the decision gate from that file, and the prompt names it where
// it actually is. Before the fix the deliverable was resolved from it while its human gate was not
// seen, so the dispatch went ahead with nothing in front of the human.
func TestCrossRepoGateHumanBriefUnderClaimRootFiresTheDecisionGate(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	trk := trackingCheckout(t, exampleRegistry, "gate: human", "deliverable_repo: example-tool")
	s.replies = append(happyReplies("/private/tmp/worker-home"),
		reply{match: "decision-issue.sh ensure", stdout: "created: decision issue #9"})
	t.Setenv("GH_TOKEN", "example-explicit-export")
	promptFile := filepath.Join(t.TempDir(), "p.md")
	if err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk,
		"--brief", exampleBriefRel, "--quiet", "--prompt-file", promptFile}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	absBrief, _ := filepath.Abs(filepath.Join(trk, filepath.FromSlash(exampleBriefRel)))
	if !s.ran("decision-issue.sh ensure " + absBrief + " ") {
		t.Errorf("the decision gate was not ensured from the tracking checkout's brief %s; calls: %v", absBrief, s.calls)
	}
	prompt := readPrompt(t, promptFile)
	for _, want := range []string{"Human-gated item", "**Specification:** `" + absBrief + "` (in the tracking checkout"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}

// COR advisory — a --brief that resolves nowhere is REFUSED before anything runs, never a NOTICE the
// dispatch proceeds past on the legacy path.
func TestBriefThatResolvesNowhereIsRefused(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	err := cmdDispatch([]string{"example-stream/05", "--root", root, "--repo", allowedRepo,
		"--brief", "docs/streams/example-stream/brief-99-missing.md", "--quiet"})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("an unresolvable --brief must be refused (exit 5): %v", err)
	}
	if len(s.calls) != 0 {
		t.Errorf("an unresolvable --brief reached a child process: %v", s.calls)
	}
}

// Contract source — the alias segment of the brief's own brief-v2 id names the TRACKING repo; with no
// deliverable declared, dispatching that brief from another repo's checkout hard-fails.
func TestDeliverableRepoFromBriefIDAlias(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	trk := trackingCheckout(t, exampleRegistry, "brief: example-cell:example-trk:example-stream:05")
	s.replies = happyReplies("/private/tmp/worker-home")
	err := cmdDispatch([]string{"example-stream/05", "--root", root, "--claim-root", trk, "--brief", exampleBriefRel, "--quiet"})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "example-org/tracker") {
		t.Fatalf("a brief-v2 id tracked elsewhere, dispatched here with no deliverable declared, must hard-fail: %v", err)
	}
	if len(claimCalls(s)) != 0 || s.ran("deskwt add") {
		t.Error("the hard fail must precede the claim and the worktree")
	}
}

// COR advisory — a dry run cannot run the phantom check, so a --rework dry run SAYS the branch may
// move to a follow-up rather than presenting the fresh branch as final.
func TestReworkDryRunNamesTheFollowUpPossibility(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	out := captureStdout(t, func() {
		if rc := run([]string{"example-stream/05", "--root", root, "--repo", allowedRepo, "--rework", "--dry-run",
			"--prompt-file", filepath.Join(t.TempDir(), "p.md")}); rc != deskkit.ExitOK {
			t.Fatalf("rc = %d", rc)
		}
	})
	if !strings.Contains(out, "FOLLOW-UP branch feat/example-stream-05-followup-<N>") {
		t.Errorf("the --rework dry-run plan does not name the follow-up possibility:\n%s", out)
	}
}
