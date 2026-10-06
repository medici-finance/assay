package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	git("add", "brief.md", "README.md", "source.txt", ".gitignore")
	git("commit", "-m", "fixture")
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	git("checkout", "--detach")
	if err := PrepareVerifierAttestation(root, "example-org/one", "brief.md", model, "strong"); err != nil {
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
			os.WriteFile(filepath.Join(root, "README.md"), []byte("implemented -> verified"), 0600)
			if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err != nil {
				t.Fatalf("legitimate Evidence/status edit deadlocked: %v", err)
			}
			if err := receipt.CheckEvidenceContent("brief.md", b); err != nil {
				t.Fatal(err)
			}
			b = []byte(strings.Replace(string(b), "| 1 | true", "| 1 | false", 1))
			os.WriteFile(path, b, 0600)
			if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil {
				t.Fatal("changed Verify row admitted")
			}
			if err := receipt.CheckEvidenceContent("brief.md", b); err == nil {
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
	for _, mode := range []string{"tracked", "staged", "branch", "commit", "untracked", "ignored", "second-site"} {
		t.Run(mode, func(t *testing.T) {
			root, f := verifierFixture(t, "gpt-6-astra")
			receipt, err := IssueVerifierAttestation(root, f)
			if err != nil {
				t.Fatal(err)
			}
			name := "source.txt"
			want := "source files changed"
			switch mode {
			case "branch":
				attestGit(t, root, "checkout", "-b", "fixture-branch")
				want = "attested detached source"
			case "commit":
				attestGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "other commit")
				want = "attested detached source"
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
			// Check once before execution, then again with allowed Evidence edits.
			for _, phase := range []string{"execution", "evidence"} {
				if phase == "evidence" {
					path := filepath.Join(root, "brief.md")
					b, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(strings.Replace(string(b), "Pending.", receipt.EvidenceBinding(), 1)), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("%s %s: want refusal %q, got %v", phase, mode, want, err)
				}
			}
		})
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
