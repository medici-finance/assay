package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

type verifierForge struct {
	Forge
	issue                                *Issue
	events                               []LabelEvent
	creates, closes, writes              int
	actor                                string
	edited, unreadable, drop, loseCreate bool
	missingHistory                       bool
}

func (f *verifierForge) FileIssue(_ ForgeRepo, in IssueInput) (*IssueRef, error) {
	f.creates++
	f.issue = &Issue{Number: 41, Title: in.Title, Body: in.Body, State: "open", Author: Account{Login: "example-desk[bot]"}, URL: "https://example.invalid/attestation/41"}
	if f.loseCreate {
		return nil, errors.New("response lost")
	}
	return &IssueRef{Number: 41, URL: f.issue.URL}, nil
}
func (f *verifierForge) GetIssueTyped(_ ForgeRepo, n int, k TargetKind) (*Issue, error) {
	if k != TargetIssue {
		panic("wrong target")
	}
	if f.unreadable {
		return nil, errors.New("offline")
	}
	return f.issue, nil
}
func (f *verifierForge) IssueTrustEvents(ForgeRepo, int) (*TrustPayload, error) {
	r := &TrustPayload{Complete: true, BodyHistoryKnown: !f.missingHistory}
	if f.edited {
		r.BodyEdited = time.Now()
	}
	return r, nil
}
func (f *verifierForge) ListIssueLabelEvents(ForgeRepo, int) ([]LabelEvent, error) {
	return f.events, nil
}
func (f *verifierForge) ApplyLabels(_ ForgeRepo, _ int, in LabelChange) (*LabelOutcome, error) {
	if in.Target != TargetIssue {
		panic("wrong target")
	}
	f.writes++
	if f.drop {
		return nil, nil
	}
	for _, remove := range in.Remove {
		var labels []string
		for _, l := range f.issue.Labels {
			if l != remove {
				labels = append(labels, l)
			}
		}
		f.issue.Labels = labels
		f.events = append(f.events, LabelEvent{Name: remove, AppliedBy: f.actor, Removed: true})
	}
	for _, add := range in.Add {
		found := false
		for _, l := range f.issue.Labels {
			if l == add.Name {
				found = true
			}
		}
		if !found {
			f.issue.Labels = append(f.issue.Labels, add.Name)
			f.events = append(f.events, LabelEvent{Name: add.Name, AppliedBy: f.actor})
		}
	}
	return &LabelOutcome{}, nil
}
func (f *verifierForge) CloseIssueTyped(_ ForgeRepo, _ int, k TargetKind, _ string) error {
	if k != TargetIssue {
		panic("wrong target")
	}
	f.closes++
	f.issue.State = "closed"
	return nil
}
func (f *verifierForge) SearchIssues(ForgeRepo, SearchIssuesInput) ([]IssueSearchResult, error) {
	if f.issue == nil {
		return nil, nil
	}
	return []IssueSearchResult{{Number: 41, Title: f.issue.Title, URL: f.issue.URL}}, nil
}
func verifierFixture(t *testing.T, model string) (string, *verifierForge) {
	t.Helper()
	return verifierFixtureWith(t, model, "brief.md", nil)
}

// verifierFixtureWith commits the base fixture plus extra files, then prepares
// the brief at briefPath (one of them) from the detached origin/main commit.
func verifierFixtureWith(t *testing.T, model, briefPath string, extra map[string]string) (string, *verifierForge) {
	t.Helper()
	plantRoster(t, "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\nASSAY_TRUSTED_BOT_SLUGS=desk=example-desk:1,verifier=example-verifier:2\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n")
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	git("init")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "brief.md"), []byte("# Fixture\n\n## Verify\n\n| 1 | true | exit 0 |\n| 2 | true | exit 0 |\n\n## Evidence\n\nPending.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"README.md": "implemented", "source.txt": "attested source", ".gitignore": "ignored/\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range extra {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", "-A")
	git("commit", "-m", "fixture")
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	git("checkout", "--detach")
	if err := PrepareVerifierAttestation(root, "example-org/one", briefPath, model, "strong"); err != nil {
		t.Fatal(err)
	}
	return root, &verifierForge{actor: "example-desk[bot]"}
}
func TestVerifierAttestationRoundTrip(t *testing.T) {
	for _, model := range []string{"gpt-6-astra", "gpt-6.1-sol", "gpt-6-1-sol"} {
		t.Run(model, func(t *testing.T) {
			root, f := verifierFixture(t, model)
			if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil {
				t.Fatal("pending admitted")
			}
			receipt, err := IssueVerifierAttestation(root, f)
			if err != nil {
				t.Fatal(err)
			}
			if receipt.Binding.Model != model || receipt.Binding.Tier != "strong" || receipt.Issue != 41 {
				t.Fatalf("wrong receipt %+v", receipt)
			}
			writes := f.writes
			again, err := IssueVerifierAttestation(root, f)
			if err != nil || again != receipt {
				t.Fatalf("idempotent receipt %+v %v", again, err)
			}
			if f.creates != 1 || f.closes != 1 || f.writes != writes {
				t.Fatal("idempotence created durable churn")
			}
			path := filepath.Join(root, "brief.md")
			b, _ := os.ReadFile(path)
			b = []byte(strings.Replace(string(b), "Pending.", "Two observed rows.\n\n"+receipt.EvidenceBinding(), 1))
			os.WriteFile(path, b, 0600)
			if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err != nil {
				t.Fatalf("legitimate Evidence edit deadlocked: %v", err)
			}
			if _, err := CheckVerifierEvidenceWithForge(root, "example-org/one", "brief.md", f); err != nil {
				t.Fatalf("legitimate Evidence edit deadlocked at landing: %v", err)
			}
			if err := receipt.CheckEvidenceContent("brief.md", b, nil); err != nil {
				t.Fatal(err)
			}
			b = []byte(strings.Replace(string(b), "| 1 | true", "| 1 | false", 1))
			os.WriteFile(path, b, 0600)
			if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil {
				t.Fatal("changed Verify row admitted")
			}
			if err := receipt.CheckEvidenceContent("brief.md", b, nil); err == nil {
				t.Fatal("changed Verify command allowed at Evidence landing")
			}
		})
	}
}
func TestVerifierAttestationFailures(t *testing.T) {
	for _, what := range []string{"wrong-author", "self-stamp", "missing-history", "edited", "unreadable", "dropped-write", "wrong-run", "wrong-model", "wrong-source", "different-home", "different-brief"} {
		t.Run(what, func(t *testing.T) {
			root, f := verifierFixture(t, "gpt-6.1-sol")
			if what == "dropped-write" {
				f.drop = true
				if _, err := IssueVerifierAttestation(root, f); err == nil {
					t.Fatal("dropped stamp succeeded")
				}
				return
			}
			if _, err := IssueVerifierAttestation(root, f); err != nil {
				t.Fatal(err)
			}
			brief := "brief.md"
			switch what {
			case "wrong-author":
				f.issue.Author.Login = "example-verifier[bot]"
			case "self-stamp":
				for i := range f.events {
					f.events[i].AppliedBy = "example-verifier[bot]"
				}
			case "missing-history":
				f.missingHistory = true
			case "edited":
				f.edited = true
			case "unreadable":
				f.unreadable = true
			case "wrong-run":
				f.issue.Body = strings.Replace(f.issue.Body, "Run", "OtherRun", 1)
			case "wrong-model":
				f.issue.Labels = []string{"dispatched-model:gpt-6-astra", "dispatched-tier:strong"}
			case "wrong-source":
				r, _ := verifierLoad(root)
				r.Binding.Source = strings.Repeat("a", 40)
				verifierSave(root, r)
			case "different-home":
				r, _ := verifierLoad(root)
				r.Home = t.TempDir()
				verifierSave(root, r)
			case "different-brief":
				brief = "other.md"
			}
			if _, err := CheckVerifierAttestationWithForge(root, brief, f); err == nil {
				t.Fatalf("%s admitted", what)
			}
		})
	}
}
func TestVerifierAttestationRecovery(t *testing.T) {
	root, f := verifierFixture(t, "gpt-6-astra")
	f.loseCreate = true
	if _, err := IssueVerifierAttestation(root, f); err == nil {
		t.Fatal("lost response succeeded")
	}
	if _, err := IssueVerifierAttestation(root, f); err != nil {
		t.Fatal(err)
	}
	if f.creates != 1 {
		t.Fatal("recovery created duplicate")
	}
}
func TestVerifierAttestationUnknownCreateRefusesDuplicate(t *testing.T) {
	root, f := verifierFixture(t, "gpt-6-astra")
	r, _ := verifierLoad(root)
	r.Attempted = true
	verifierSave(root, r)
	if _, err := IssueVerifierAttestation(root, f); err == nil {
		t.Fatal("unknown creation admitted")
	}
	if f.creates != 0 {
		t.Fatal("unknown creation retried")
	}
}

func TestVerifierAttestationCopiedHomeCannotRebind(t *testing.T) {
	root, f := verifierFixture(t, "gpt-6.1-sol")
	if _, err := IssueVerifierAttestation(root, f); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if out, err := exec.Command("git", "clone", "--no-hardlinks", root, other).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	if out, err := exec.Command("git", "-C", other, "checkout", "--detach").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	r, _ := verifierLoad(root)
	canonical, err := filepath.EvalSymlinks(other)
	if err != nil {
		t.Fatal(err)
	}
	r.Home = canonical
	if err := verifierSave(other, r); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckVerifierAttestationWithForge(other, "brief.md", f); err == nil {
		t.Fatal("copied local record admitted another home")
	}
	r.Binding.HomeSHA256 = verifierDigest([]byte(canonical))
	if err := verifierSave(other, r); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckVerifierAttestationWithForge(other, "brief.md", f); err == nil {
		t.Fatal("locally rewritten binding replaced immutable dispatcher binding")
	}
}
func TestVerifierAttestationDryPlanRequiresExistingBinding(t *testing.T) {
	root, _ := verifierFixture(t, "gpt-6-astra")
	if err := PlanVerifierAttestation(root, "brief.md"); err != nil {
		t.Fatal(err)
	}
	if err := PlanVerifierAttestation(root, "different.md"); err == nil {
		t.Fatal("wrong brief preview succeeded")
	}
	if err := PlanVerifierAttestation(t.TempDir(), ""); err == nil {
		t.Fatal("missing run preview succeeded")
	}
}

func attestGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s %v", args, out, err)
	}
}

func TestAttestActorSeparation(t *testing.T) {
	root, f := verifierFixture(t, "gpt-6-astra")
	if _, err := IssueVerifierAttestation(root, f); err != nil {
		t.Fatal(err)
	}
	plantRoster(t, "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\nASSAY_TRUSTED_BOT_SLUGS=desk=example-desk:1,verifier=example-desk:1\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n")
	if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil {
		t.Fatal("same desk and verifier actor admitted")
	}
	if err := PrepareVerifierAttestation(root, "example-org/one", "brief.md", "gpt-6-astra", "strong"); err == nil || !strings.Contains(err.Error(), "distinct configured") {
		t.Fatalf("same actor not refused at preparation: %v", err)
	}
}

// The same reader is used before execution and before Evidence landing. Each
// fixture alters the real dispatched home, leaving the immutable record intact.
func TestAttestSourceClosure(t *testing.T) {
	for _, mode := range []string{"tracked", "staged", "branch", "commit", "untracked", "ignored", "second-site", "deleted", "assume-unchanged", "skip-worktree", "replace-ref", "clean-filter", "smudge-filter", "attr-tree-filter", "info-attributes-crlf", "info-attributes-encoding", "local-autocrlf", "local-eol", "attributes-file", "attr-tree", "stream-index", "index-removed", "index-swapped", "index-added", "index-attributes", "index-redirect", "core-worktree", "worktree-config-worktree", "core-worktree-link", "core-bare"} {
		t.Run(mode, func(t *testing.T) {
			var extra map[string]string
			switch {
			case strings.HasSuffix(mode, "-filter"):
				// The filter attribute is attested; only the driver is local.
				extra = map[string]string{".gitattributes": "source.txt filter=fixture\n"}
			case mode == "local-eol":
				// The attested attributes make the file text; only core.eol is local.
				extra = map[string]string{".gitattributes": ".gitignore text\n"}
			case mode == "index-attributes":
				extra = map[string]string{".gitattributes": "sub/data.txt filter=fixture\n", "sub/data.txt": "attested line\n"}
			}
			root, f := verifierFixtureWith(t, "gpt-6-astra", "brief.md", extra)
			receipt, err := IssueVerifierAttestation(root, f)
			if err != nil {
				t.Fatal(err)
			}
			name := "source.txt"
			want := "source files changed"
			switch mode {
			case "index-removed", "index-swapped", "index-added", "index-attributes", "index-redirect":
				name = indexChange(t, root, mode)
				want = "source files changed since verifier dispatch: " + name
			case "core-worktree", "worktree-config-worktree", "core-worktree-link", "core-bare":
				want = workTreeChange(t, root, mode)
			case "attr-tree-filter":
				// An unattested attribute tree unsets the attested filter for the
				// attribute query alone; the driver then renders the planted bytes.
				attrTreeUnsetsFilter(t, root, name)
			case "local-autocrlf", "local-eol", "attributes-file", "attr-tree":
				name = convertTrackedChange(t, root, mode)
				want = "source files changed since verifier dispatch: " + name
			case "branch":
				attestGit(t, root, "checkout", "-b", "fixture-branch")
				want = "attested detached source"
			case "commit":
				attestGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "other commit")
				want = "attested detached source"
			case "deleted":
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			case "info-attributes-crlf", "info-attributes-encoding":
				// Each mode makes the home's own, unattested attributes or config
				// convert a tracked file on checkout; the planted bytes are what
				// git renders under that conversion, so git diff stays silent.
				name = convertTrackedChange(t, root, mode)
				want = "info/attributes"
			case "stream-index":
				// The stream index is a source file a Verify row may read.
				name = "README.md"
				if err := os.WriteFile(filepath.Join(root, name), []byte("rewritten"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "source files changed since verifier dispatch: " + name
			case "assume-unchanged", "skip-worktree", "replace-ref", "clean-filter", "smudge-filter":
				// Each mode changes a tracked input and hides the change from
				// git's own worktree comparison; the plant is asserted to land.
				hideTrackedChange(t, root, name, mode)
				if mode == "replace-ref" {
					want = "replacement objects"
				}
			default:
				switch mode {
				case "untracked":
					name = "extra.txt"
				case "ignored":
					name = "ignored/extra.txt"
				case "second-site":
					name = "nested/second.txt"
				}
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, name), []byte("fixture change"), 0600); err != nil {
					t.Fatal(err)
				}
				if mode == "staged" {
					attestGit(t, root, "add", name)
				}
				if mode == "untracked" || mode == "ignored" || mode == "second-site" {
					want = "unattested worktree file: " + name
				}
			}
			switch mode {
			case "tracked", "assume-unchanged", "skip-worktree", "clean-filter", "smudge-filter", "stream-index", "local-autocrlf", "local-eol", "attributes-file":
				if !gitHasAttrSource(t, root) {
					// A git without --attr-source cannot tell a converted file
					// from a changed one, so either refuses as this git's limit.
					want = "cannot be admitted on this git: " + name + " differs from its blob"
				}
			}
			// Check once before execution, then again with allowed Evidence edits.
			for _, phase := range []string{"execution", "evidence"} {
				check := func() error {
					_, err := CheckVerifierAttestationWithForge(root, "brief.md", f)
					return err
				}
				if phase == "evidence" {
					check = func() error {
						_, err := CheckVerifierEvidenceWithForge(root, "example-org/one", "brief.md", f)
						return err
					}
					path := filepath.Join(root, "brief.md")
					b, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(strings.Replace(string(b), "Pending.", receipt.EvidenceBinding(), 1)), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := check(); err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("%s %s: want refusal %q, got %v", phase, mode, want, err)
				}
			}
		})
	}
}

// hideTrackedChange writes inert bytes into a tracked file, then applies one
// mechanism that makes git's worktree comparison report no difference. It
// asserts both that the bytes changed and that git diff is now silent, so a
// passing admission check could only mean the change was hidden.
func hideTrackedChange(t *testing.T, root, name, mode string) {
	t.Helper()
	path := filepath.Join(root, name)
	switch mode {
	case "assume-unchanged", "skip-worktree":
		attestGit(t, root, "update-index", "--"+mode, name)
		if err := os.WriteFile(path, []byte("attested SOURCE"), 0600); err != nil {
			t.Fatal(err)
		}
	case "replace-ref":
		source := strings.TrimSpace(attestOut(t, root, "rev-parse", "HEAD"))
		if err := os.WriteFile(path, []byte("attested SOURCE"), 0600); err != nil {
			t.Fatal(err)
		}
		attestGit(t, root, "add", name)
		tree := strings.TrimSpace(attestOut(t, root, "write-tree"))
		other := strings.TrimSpace(attestOut(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree, "-p", source, "-m", "replacement"))
		attestGit(t, root, "reset", "-q")
		attestGit(t, root, "replace", source, other)
	case "clean-filter", "smudge-filter":
		attestGit(t, root, "config", "filter.fixture.clean", "tr A-Z a-z")
		if mode == "smudge-filter" {
			// The driver renders the attested blob as the planted bytes, so the
			// checkout form itself is no longer the attested content.
			attestGit(t, root, "config", "filter.fixture.smudge", "sed s/source/SOURCE/")
		}
		if err := os.WriteFile(path, []byte("attested SOURCE"), 0600); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown hiding mode %s", mode)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "attested SOURCE" {
		t.Fatalf("%s: plant did not land: %q %v", mode, b, err)
	}
	if diff := attestOut(t, root, "diff", "--name-only", "HEAD", "--"); diff != "" {
		t.Fatalf("%s: change not hidden from git diff: %q", mode, diff)
	}
}

// convertTrackedChange makes an unattested conversion apply to .gitignore, then
// writes the file as git renders the attested blob under it. It asserts the
// bytes differ from the attested blob and that git diff reports no change.
func convertTrackedChange(t *testing.T, root, mode string) string {
	t.Helper()
	name := ".gitignore"
	writeAttrs := func(path, body string) {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	info := strings.TrimSpace(attestOut(t, root, "rev-parse", "--path-format=absolute", "--git-path", "info/attributes"))
	switch mode {
	case "info-attributes-crlf":
		writeAttrs(info, name+" text eol=crlf\n")
	case "info-attributes-encoding":
		writeAttrs(info, name+" working-tree-encoding=UTF-16\n")
	case "local-autocrlf":
		attestGit(t, root, "config", "core.autocrlf", "true")
	case "local-eol":
		attestGit(t, root, "config", "core.eol", "crlf")
	case "attributes-file":
		file := filepath.Join(t.TempDir(), "attributes")
		writeAttrs(file, name+" text eol=crlf\n")
		attestGit(t, root, "config", "core.attributesFile", file)
	case "attr-tree":
		// An unattested tree named as the attribute source.
		blob := strings.TrimSpace(attestIn(t, root, name+" text eol=crlf\n", "hash-object", "-w", "--stdin"))
		tree := strings.TrimSpace(attestIn(t, root, "100644 blob "+blob+"\t.gitattributes\n", "mktree"))
		attestGit(t, root, "config", "attr.tree", tree)
		if out := strings.TrimSpace(attestOut(t, root, "check-attr", "eol", "--", name)); !strings.HasSuffix(out, ": crlf") {
			// attr.tree arrived in git 2.46; an older git never reads it, so this
			// route does not exist there. Every other mode runs on any git.
			t.Skip("this git does not read attr.tree")
		}
	default:
		t.Fatalf("unknown conversion mode %s", mode)
	}
	attested := attestOut(t, root, "cat-file", "blob", "HEAD:"+name)
	planted := attestOut(t, root, "cat-file", "--filters", "--path="+name, "HEAD:"+name)
	if planted == attested {
		t.Fatalf("%s: conversion did not change the checkout form", mode)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(planted), 0600); err != nil {
		t.Fatal(err)
	}
	if diff := attestOut(t, root, "diff", "--name-only", "HEAD", "--"); diff != "" {
		t.Fatalf("%s: change not hidden from git diff: %q", mode, diff)
	}
	return name
}

// workTreeChange leaves the home's files and index as attested and changes only
// which work tree git resolves for the home, then asserts the plant: what a
// Verify row's own git reads no longer follows from the home's files. It
// returns the refusal the mode must produce.
func workTreeChange(t *testing.T, root, mode string) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "core-worktree", "worktree-config-worktree":
		// A copy of the home with one file changed, named as the work tree.
		other := t.TempDir()
		for _, name := range []string{"brief.md", "README.md", ".gitignore", "source.txt"} {
			b, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			if name == "source.txt" {
				b = []byte("TAMPERED source")
			}
			if err := os.WriteFile(filepath.Join(other, name), b, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if mode == "worktree-config-worktree" {
			attestGit(t, root, "config", "extensions.worktreeConfig", "true")
			attestGit(t, root, "config", "--worktree", "core.worktree", other)
		} else {
			attestGit(t, root, "config", "core.worktree", other)
		}
		if got := attestOut(t, root, "grep", "-l", "TAMPERED"); got != "source.txt\n" {
			t.Fatalf("%s: redirected work tree not visible to git grep: %q", mode, got)
		}
		if b, err := os.ReadFile(filepath.Join(root, "source.txt")); err != nil || string(b) != "attested source" {
			t.Fatalf("%s: home changed: %q %v", mode, b, err)
		}
		// git's resolved view refuses first; the configured work tree would
		// also refuse, so the assertion pins which signal answers.
		return "git work tree is not the home itself"
	case "core-worktree-link":
		// A configured work tree that resolves to the home today: git's own
		// view matches, but the link can be re-pointed after admission.
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(home, link); err != nil {
			t.Fatal(err)
		}
		attestGit(t, root, "config", "core.worktree", link)
		if got := strings.TrimSpace(attestOut(t, root, "rev-parse", "--show-toplevel")); got != home {
			t.Fatalf("core-worktree-link: work tree %q does not resolve to the home %q", got, home)
		}
		return "configures its git work tree"
	case "core-bare":
		// No work tree at all: every row's git then fails, and a negated row
		// (! git grep -q X) reads that failure as a pass.
		attestGit(t, root, "config", "extensions.worktreeConfig", "true")
		attestGit(t, root, "config", "--worktree", "core.bare", "true")
		cmd := exec.Command("git", "-C", root, "grep", "-q", "attested source")
		if err := cmd.Run(); err == nil || cmd.ProcessState.ExitCode() == 1 {
			t.Fatalf("core-bare: row git still reads a work tree: %v", err)
		}
		return "no git work tree of its own"
	}
	t.Fatalf("unknown work tree mode %s", mode)
	return ""
}

// TestVerifierEnvStrip pins the environment strip one variable at a time: each
// inherited GIT_* variable that can redirect the repository, work tree, index,
// object store, config, attributes or replace-ref base is absent from every
// admission git read, whatever its case, and the two admission settings are
// present.
func TestVerifierEnvStrip(t *testing.T) {
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_REPLACE_REF_BASE", "GIT_CONFIG", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_ATTR_SOURCE", "GIT_NAMESPACE", "GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM", "Git_Dir"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "fixture-value")
			env := verifierEnv()
			for _, kv := range env {
				if key, _, _ := strings.Cut(kv, "="); strings.EqualFold(key, name) {
					t.Fatalf("%s reaches admission git reads", name)
				}
			}
			for _, want := range []string{"GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1"} {
				if !slices.Contains(env, want) {
					t.Fatalf("admission environment lacks %s", want)
				}
			}
		})
	}
}

// gitHasAttrSource reports whether this git takes --attr-source (git 2.41 and
// later). An older git cannot render a checkout from the attested attributes,
// so admission refuses any file whose bytes differ from its blob there.
func gitHasAttrSource(t *testing.T, root string) bool {
	t.Helper()
	return exec.Command("git", "-C", root, "--attr-source=HEAD", "version").Run() == nil
}

// indexChange alters the home's index and nothing on disk except what a mode
// names, then asserts the plant: what a Verify row reading the index sees
// (git grep, git ls-files) no longer follows from the attested commit.
func indexChange(t *testing.T, root, mode string) string {
	t.Helper()
	blob := func(body string) string {
		return strings.TrimSpace(attestIn(t, root, body, "hash-object", "-w", "--stdin"))
	}
	listed := func(name string) bool { return attestOut(t, root, "ls-files", "--", name) != "" }
	switch mode {
	case "index-removed", "index-redirect":
		attestGit(t, root, "rm", "-q", "--cached", "source.txt")
		if listed("source.txt") || exec.Command("git", "-C", root, "grep", "-q", "attested source").Run() == nil {
			t.Fatalf("%s: removal not visible to a row reading the index", mode)
		}
		if b, err := os.ReadFile(filepath.Join(root, "source.txt")); err != nil || string(b) != "attested source" {
			t.Fatalf("%s: worktree changed: %q %v", mode, b, err)
		}
		if mode == "index-redirect" {
			// An attested-looking index elsewhere, named by the caller's
			// environment; the home's own index is the one a row reads.
			clean := filepath.Join(t.TempDir(), "index")
			cmd := exec.Command("git", "-C", root, "read-tree", "HEAD")
			cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+clean)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("read-tree: %s %v", out, err)
			}
			t.Setenv("GIT_INDEX_FILE", clean)
		}
		return "source.txt"
	case "index-swapped":
		attestGit(t, root, "update-index", "--cacheinfo", "100644,"+blob("attested SOURCE")+",source.txt")
		if attestOut(t, root, "grep", "--cached", "-l", "SOURCE") != "source.txt\n" {
			t.Fatal("index-swapped: swapped blob not visible to git grep --cached")
		}
		return "source.txt"
	case "index-added":
		attestGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+blob("planted")+",planted.txt")
		if _, err := os.Lstat(filepath.Join(root, "planted.txt")); !listed("planted.txt") || err == nil {
			t.Fatalf("index-added: plant did not land in the index only (%v)", err)
		}
		return "planted.txt"
	case "index-attributes":
		// An index-only attributes file unsets the attested filter for a git
		// that reads attributes from the index; the driver's output is planted.
		attestGit(t, root, "config", "filter.fixture.smudge", "sed s/attested/TAMPERED/")
		attestGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+blob("data.txt -filter\n")+",sub/.gitattributes")
		if err := os.WriteFile(filepath.Join(root, "sub", "data.txt"), []byte("TAMPERED line\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(root, "sub", ".gitattributes")); !listed("sub/.gitattributes") || err == nil {
			t.Fatalf("index-attributes: plant did not land in the index only (%v)", err)
		}
		return "sub/.gitattributes"
	}
	t.Fatalf("unknown index mode %s", mode)
	return ""
}

// attrTreeUnsetsFilter names an unattested attribute tree that unsets the
// attested filter, configures the driver, and plants its output. Only the
// attribute query is exposed to the tree: rendering itself reads the attested
// attributes and so writes exactly the planted bytes through the driver.
func attrTreeUnsetsFilter(t *testing.T, root, name string) {
	t.Helper()
	attestGit(t, root, "config", "filter.fixture.smudge", "sed s/source/SOURCE/")
	blob := strings.TrimSpace(attestIn(t, root, name+" -filter\n", "hash-object", "-w", "--stdin"))
	tree := strings.TrimSpace(attestIn(t, root, "100644 blob "+blob+"\t.gitattributes\n", "mktree"))
	attestGit(t, root, "config", "attr.tree", tree)
	if out := strings.TrimSpace(attestOut(t, root, "check-attr", "filter", "--", name)); !strings.HasSuffix(out, ": unset") {
		t.Skip("this git does not read attr.tree") // git 2.46 and later do
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte("attested SOURCE"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := attestOut(t, root, "--attr-source=HEAD", "cat-file", "--filters", "--path="+name, "HEAD:"+name); got != "attested SOURCE" {
		t.Fatalf("attr-tree-filter: driver output %q is not the plant", got)
	}
}

func attestIn(t *testing.T, root, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func attestOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// The tree reader must resolve the dispatched commit's real objects even when a
// replacement exists; the separate replace-ref refusal is pinned above.
func TestVerifierTreeIgnoresReplacements(t *testing.T) {
	root, _ := verifierFixture(t, "gpt-6-astra")
	source := strings.TrimSpace(attestOut(t, root, "rev-parse", "HEAD"))
	want, err := verifierTree(root, source)
	if err != nil {
		t.Fatal(err)
	}
	hideTrackedChange(t, root, "source.txt", "replace-ref")
	got, err := verifierTree(root, source)
	if err != nil {
		t.Fatal(err)
	}
	if got["source.txt"] != want["source.txt"] {
		t.Fatalf("replacement object read as the attested source: %v != %v", got["source.txt"], want["source.txt"])
	}
}

func TestAttestRemoteRef(t *testing.T) {
	root, f := verifierFixture(t, "gpt-6-astra")
	receipt, err := IssueVerifierAttestation(root, f)
	if err != nil {
		t.Fatal(err)
	}
	attestGit(t, root, "commit", "--allow-empty", "-m", "stray ref")
	attestGit(t, root, "branch", "origin/main", "HEAD")
	attestGit(t, root, "checkout", "--detach", receipt.Binding.Source)
	path, err := verifierRecordPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := PrepareVerifierAttestation(root, "example-org/one", "brief.md", "gpt-6-astra", "strong"); err != nil {
		t.Fatalf("stray local ref shadowed remote source: %v", err)
	}
}

// Control for the byte comparison: a fresh checkout that converts line endings,
// carries an executable and a link is admitted; changing the mode or a converted
// file's bytes is refused.
func TestVerifierSourceClosureCheckoutForms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modes and links are compared on platforms that carry them")
	}
	src := t.TempDir()
	for name, body := range map[string]string{".gitattributes": "*.ps1 text eol=crlf\n", "run.ps1": "one\ntwo\n", "tool.sh": "#!/bin/sh\n", "plain.txt": "plain\n"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(src, "tool.sh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("plain.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}
	attestGit(t, src, "init", "-q")
	attestGit(t, src, "add", ".")
	attestGit(t, src, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "fixture")
	commit := strings.TrimSpace(attestOut(t, src, "rev-parse", "HEAD"))
	home := filepath.Join(t.TempDir(), "home")
	if out, err := exec.Command("git", "clone", "-q", src, home).CombinedOutput(); err != nil {
		t.Fatalf("clone: %s %v", out, err)
	}
	if b, _ := os.ReadFile(filepath.Join(home, "run.ps1")); string(b) != "one\r\ntwo\r\n" {
		t.Fatalf("checkout did not convert: %q", b)
	}
	co, err := readVerifierCheckout(home)
	if err != nil {
		t.Fatal(err)
	}
	if !gitHasAttrSource(t, home) {
		// A git without --attr-source cannot render a converted file from
		// the attested attributes alone, so the converted file is refused
		// rather than rendered from attributes the home could have moved.
		if err := verifierSourceClosure(home, commit, co, nil); err == nil || !strings.Contains(err.Error(), "run.ps1 differs from its blob (its checkout form") {
			t.Fatalf("converted checkout on a git without --attr-source: %v, want a run.ps1 refusal", err)
		}
		return
	}
	if err := verifierSourceClosure(home, commit, co, nil); err != nil {
		t.Fatalf("clean converted checkout refused: %v", err)
	}
	if err := os.Chmod(filepath.Join(home, "tool.sh"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifierSourceClosure(home, commit, co, nil); err == nil || !strings.Contains(err.Error(), "tool.sh") {
		t.Fatalf("mode change admitted: %v", err)
	}
	if err := os.Chmod(filepath.Join(home, "tool.sh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "run.ps1"), []byte("one\r\nTWO\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifierSourceClosure(home, commit, co, nil); err == nil || !strings.Contains(err.Error(), "run.ps1") {
		t.Fatalf("converted file change admitted: %v", err)
	}
}

// An attestation for one brief admits Evidence only for that brief: the brief
// itself, its stream index (and only its own row there), or an outcome record
// keyed to its <stream>/<NN>.
func TestVerifierEvidenceTargetBinding(t *testing.T) {
	r := VerifierReceipt{Binding: VerifierBinding{Brief: "docs/streams/x/brief-01-attested.md"}}
	for target, ok := range map[string]bool{
		"docs/streams/x/brief-01-attested.md":                       true,
		"./docs/streams/x/brief-01-attested.md":                     true,
		"docs/streams/x/README.md":                                  true,
		"docs/streams/verify-outcomes/x/01-20261006T000000Z-a.json": true,
		"docs/streams/y/brief-02-never-attested.md":                 false,
		"docs/streams/x/brief-02-sibling.md":                        false,
		"docs/streams/y/README.md":                                  false,
		"docs/streams/verify-outcomes/x/02-20261006T000000Z-a.json": false,
		"docs/streams/verify-outcomes/y/01-20261006T000000Z-a.json": false,
		"docs/streams/x/../y/brief-02-never-attested.md":            false,
		"": false,
	} {
		if err := r.CheckEvidenceTarget(target); (err == nil) != ok {
			t.Errorf("target %q: admitted=%v, want %v (%v)", target, err == nil, ok, err)
		}
	}
	for _, c := range []struct {
		rows []string
		ok   bool
	}{{nil, false}, {[]string{"02"}, false}, {[]string{"01", "02"}, false}, {[]string{"01"}, true}} {
		if err := r.CheckEvidenceContent("docs/streams/x/README.md", []byte("index"), c.rows); (err == nil) != c.ok {
			t.Errorf("index rows %v: admitted=%v, want %v (%v)", c.rows, err == nil, c.ok, err)
		}
	}

	root, _ := verifierFixture(t, "gpt-6-astra")
	if _, err := CheckVerifierEvidence(root, "example-org/one", "other.md"); err == nil || !strings.Contains(err.Error(), "not bound to the attested brief") {
		t.Fatalf("Evidence for an unattested brief admitted: %v", err)
	}
}

// The stream index is source before execution, and at Evidence time may carry
// only the attested brief's own row's lifecycle edit. Each case edits the real
// dispatched home; the attested row is 01.
func TestAttestStreamIndex(t *testing.T) {
	const brief = "docs/streams/x/brief-01-attested.md"
	const index = "docs/streams/x/README.md"
	const table = "# Stream x\n\nProse a Verify row may grep.\n\n" + TableMarkerBegin + "\n" +
		"| # | Brief | Status | Verified | Reviewed |\n|---|---|---|---|---|\n" +
		"| 01 | attested | implemented | — | — |\n| 02 | sibling | todo | — | — |\n" + TableMarkerEnd + "\n"
	edit := func(base, from, to string) string {
		if strings.Count(base, from) != 1 {
			t.Fatalf("fixture anchor %q", from)
		}
		return strings.Replace(base, from, to, 1)
	}
	own := edit(table, "| 01 | attested | implemented | — | — |", "| 01 | attested | verified | 2026-10-06 | — |")
	for _, c := range []struct {
		name, content       string
		remove              bool
		execution, evidence bool // admitted at each checkpoint
	}{
		{name: "unchanged", content: table, execution: true, evidence: true},
		{name: "own-row-lifecycle", content: own, evidence: true},
		{name: "rewritten", content: "rewritten\n"},
		{name: "prose", content: edit(table, "Prose a Verify row may grep.", "Prose changed.")},
		{name: "foreign-row", content: edit(table, "| 02 | sibling | todo |", "| 02 | sibling | verified |")},
		{name: "own-row-authoring", content: edit(table, "| 01 | attested | implemented |", "| 01 | renamed | implemented |")},
		{name: "own-row-and-prose", content: edit(own, "Prose a Verify row may grep.", "Prose changed.")},
		{name: "deleted", remove: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, f := verifierFixtureWith(t, "gpt-6-astra", brief, map[string]string{
				brief: "# Brief\n\n## Verify\n\n| 1 | true | exit 0 |\n\n## Evidence\n\nPending.\n",
				index: table,
			})
			if _, err := IssueVerifierAttestation(root, f); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, filepath.FromSlash(index))
			if c.remove {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte(c.content), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := CheckVerifierAttestationWithForge(root, brief, f)
			if (err == nil) != c.execution || (err != nil && !strings.Contains(err.Error(), index)) {
				t.Fatalf("execution: admitted=%v, want %v (%v)", err == nil, c.execution, err)
			}
			for _, target := range []string{brief, index} {
				_, err = CheckVerifierEvidenceWithForge(root, "example-org/one", target, f)
				if (err == nil) != c.evidence || (err != nil && !strings.Contains(err.Error(), index)) {
					t.Fatalf("evidence %s: admitted=%v, want %v (%v)", target, err == nil, c.evidence, err)
				}
			}
		})
	}
}

// The checkout conversion is pinned at dispatch in the immutable record: a home
// checked out with autocrlf is admitted, and changing the home's config after
// dispatch changes nothing admission renders with.
func TestVerifierCheckoutConversionPinned(t *testing.T) {
	plantRoster(t, "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\nASSAY_TRUSTED_BOT_SLUGS=desk=example-desk:1,verifier=example-verifier:2\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n")
	src := t.TempDir()
	for name, body := range map[string]string{
		".gitattributes": "*.md text eol=lf\n",
		"brief.md":       "# Brief\n\n## Verify\n\n| 1 | true | exit 0 |\n\n## Evidence\n\nPending.\n",
		"data.txt":       "one\ntwo\n",
	} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	attestGit(t, src, "init", "-q", "-b", "main")
	attestGit(t, src, "add", ".")
	attestGit(t, src, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "fixture")
	home := filepath.Join(t.TempDir(), "home")
	if out, err := exec.Command("git", "clone", "-q", "-c", "core.autocrlf=true", src, home).CombinedOutput(); err != nil {
		t.Fatalf("clone: %s %v", out, err)
	}
	attestGit(t, home, "checkout", "-q", "--detach")
	if b, _ := os.ReadFile(filepath.Join(home, "data.txt")); string(b) != "one\r\ntwo\r\n" {
		t.Fatalf("checkout did not convert: %q", b)
	}
	if !gitHasAttrSource(t, home) {
		// A git without --attr-source cannot render from the attested
		// attributes, so a converted checkout fails closed there.
		if err := PrepareVerifierAttestation(home, "example-org/one", "brief.md", "gpt-6-astra", "strong"); err == nil || !strings.Contains(err.Error(), "its checkout form") {
			t.Fatalf("converted home on a git without --attr-source: %v", err)
		}
		return
	}
	if err := PrepareVerifierAttestation(home, "example-org/one", "brief.md", "gpt-6-astra", "strong"); err != nil {
		t.Fatalf("converted home refused: %v", err)
	}
	f := &verifierForge{actor: "example-desk[bot]"}
	receipt, err := IssueVerifierAttestation(home, f)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Binding.Checkout != "autocrlf=true eol=native" || !strings.Contains(f.issue.Body, receipt.Binding.Checkout) {
		t.Fatalf("conversion not pinned in the immutable record: %+v", receipt.Binding)
	}
	attestGit(t, home, "config", "core.autocrlf", "false")
	if _, err := CheckVerifierAttestationWithForge(home, "brief.md", f); err != nil {
		t.Fatalf("pinned conversion not used after a config change: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "data.txt"), []byte("one\r\nTWO\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckVerifierAttestationWithForge(home, "brief.md", f); err == nil || !strings.Contains(err.Error(), "data.txt") {
		t.Fatalf("converted file change admitted: %v", err)
	}
	for _, bad := range []string{"", "autocrlf=true", "autocrlf=yes eol=native", "eol=native autocrlf=true", "autocrlf=true eol=native x"} {
		if _, err := parseVerifierCheckout(bad); err == nil {
			t.Errorf("checkout %q accepted", bad)
		}
	}
}

// System attributes cannot be planted from a test, so the switch that turns
// them off is pinned directly, even when the caller's environment enables them.
func TestVerifierEnvIgnoresSystemAttributes(t *testing.T) {
	t.Setenv("GIT_ATTR_NOSYSTEM", "0")
	env := verifierEnv()
	last := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_ATTR_NOSYSTEM=") {
			last = kv
		}
	}
	if last != "GIT_ATTR_NOSYSTEM=1" {
		t.Fatalf("system attributes enabled: %q", last)
	}
}
