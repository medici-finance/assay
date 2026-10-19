package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type pendingForge struct {
	deskkit.Forge
	pr                                          *deskkit.PullRequest
	change                                      deskkit.ChangeRef
	files                                       []deskkit.ChangedFile
	blobs                                       map[string][]byte
	issue                                       *deskkit.Issue
	labels                                      []deskkit.LabelEvent
	edited, incomplete, readFail, missing, move bool
	lists                                       int
	// extra are further open changes, listed BEFORE change; any per-change read of one fails, and prReads
	// counts per-change reads so a test can prove which changes were never read.
	extra   []deskkit.ChangeRef
	prReads map[int]int
}

func (f *pendingForge) RepoDefaultBranch(deskkit.ForgeRepo) (string, error) { return "main", nil }

func (f *pendingForge) ListChanges(deskkit.ForgeRepo, deskkit.ChangeStates) (*deskkit.ChangeList, error) {
	f.lists++
	if f.readFail {
		return nil, errors.New("offline list")
	}
	return &deskkit.ChangeList{Changes: append(append([]deskkit.ChangeRef{}, f.extra...), f.change), Incomplete: f.incomplete}, nil
}
func (f *pendingForge) GetPullRequest(_ deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	if f.prReads == nil {
		f.prReads = map[int]int{}
	}
	f.prReads[n]++
	if n != f.pr.Number {
		return nil, errors.New("offline unreadable change")
	}
	p := *f.pr
	if f.move {
		p.HeadSHA = strings.Repeat("d", 40)
	}
	return &p, nil
}
func (f *pendingForge) ListChangedFiles(_ deskkit.ForgeRepo, n int) ([]deskkit.ChangedFile, error) {
	if n != f.pr.Number {
		return nil, errors.New("offline unreadable change files")
	}
	return f.files, nil
}
func (f *pendingForge) ReadFile(_ deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	b, ok := f.blobs[in.Ref+":"+in.File]
	return &deskkit.FileContent{Exists: ok && !f.missing, Content: b, Kind: deskkit.FileKindRegular}, nil
}
func (f *pendingForge) GetIssueTyped(deskkit.ForgeRepo, int, deskkit.TargetKind) (*deskkit.Issue, error) {
	return f.issue, nil
}
func (f *pendingForge) GetIssue(deskkit.ForgeRepo, int) (*deskkit.Issue, error) {
	return &deskkit.Issue{State: "open"}, nil
}
func (f *pendingForge) IssueTrustEvents(deskkit.ForgeRepo, int) (*deskkit.TrustPayload, error) {
	p := &deskkit.TrustPayload{Complete: true, BodyHistoryKnown: true}
	if f.edited {
		p.BodyEdited = time.Now()
	}
	return p, nil
}
func (f *pendingForge) ListIssueLabelEvents(deskkit.ForgeRepo, int) ([]deskkit.LabelEvent, error) {
	return f.labels, nil
}

// pendingSetup builds the fixture; an optional implementer is declared in brief 01's front
// matter BEFORE the receipt binds the brief, so the binding stays exact.
func pendingSetup(t *testing.T, implementer ...string) (*VerifyLoop, *pendingForge, deskkit.VerifierReceipt, string) {
	t.Helper()
	plantAllowedRoster(t, "example-org/tracker")
	roster := "ASSAY_BLESS_LOGIN=ada:2001\nASSAY_TRUSTED_LOGINS=ada:2001\nASSAY_TRUSTED_BOT_SLUGS=desk=example-desk:300000004,verifier=example-verifier:300000005,worker=example-worker:300000006\nASSAY_ALLOWED_REPOS=example-org/tracker:ci:private\n"
	if e := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env"), []byte(roster), 0600); e != nil {
		t.Fatal(e)
	}
	deskkit.ReloadConfig()
	brief := strings.Replace(planBrief("model", "no", "no", "no", "no"), "## Verify", "## Context\n\nfiles:\n- `input.go`\n- `planned.go`\n\n## Verify", 1)
	if len(implementer) > 0 {
		brief = strings.Replace(brief, "brief: x\n", "brief: x\nimplementer: "+implementer[0]+"\n", 1)
	}
	root := planFixtureRoot(t, map[string]string{"01": brief, "02": planBrief("model", "no", "no", "no", "no")})
	if e := os.WriteFile(filepath.Join(root, "input.go"), []byte("package input\n"), 0600); e != nil {
		t.Fatal(e)
	}
	gitT(t, root, "init", "-q")
	gitT(t, root, "add", "docs/streams", "input.go")
	gitT(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "source")
	sha := gitT(t, root, "rev-parse", "HEAD")
	path := "docs/streams/example-stream/brief-01-x.md"
	receipt := deskkit.VerifierReceipt{Issue: 41, Binding: deskkit.VerifierBinding{Version: 1, Repo: "example-org/tracker", Source: sha, Brief: path, Run: strings.Repeat("a", 64), HomeSHA256: strings.Repeat("b", 64), BriefSHA256: deskkit.Sha256Hex([]byte(brief)), PlanSHA256: deskkit.Sha256Hex([]byte(strings.TrimSpace(strings.Split(brief, "## Evidence")[0]))), Model: "gpt-6.1-sol", Tier: "strong", Checkout: "autocrlf=false eol=native"}}
	evidence := []byte(brief + "\n" + receipt.EvidenceBinding() + "\n\n| 1 | `go test` | FAIL | exit 1 | 2026-10-16 | example-verifier[bot] |\n")
	raw, _ := json.MarshalIndent(receipt.Binding, "", "  ")
	labels, _ := deskkit.ModelStampLabels("gpt-6.1-sol", "strong")
	var events []deskkit.LabelEvent
	for _, label := range labels {
		events = append(events, deskkit.LabelEvent{Name: label, AppliedBy: "example-desk[bot]"})
	}
	head := strings.Repeat("c", 40)
	f := &pendingForge{pr: &deskkit.PullRequest{Number: 51, State: "open", BaseRef: "main", CrossRepo: deskkit.CrossRepoSame, HeadSHA: head, ChangedFiles: 1, Author: deskkit.Account{Login: "example-verifier[bot]", ID: 300000005}}, change: deskkit.ChangeRef{Number: 51, Author: deskkit.Account{Login: "example-verifier[bot]", ID: 300000005}, HeadSHA: head, CrossRepo: deskkit.CrossRepoSame, HeadRepo: "example-org/tracker"}, files: []deskkit.ChangedFile{{Filename: path, Status: "modified"}}, blobs: map[string][]byte{head + ":" + path: evidence}, issue: &deskkit.Issue{Number: 41, Title: deskkit.VerifierAttestationTitle + receipt.Binding.Run, Body: "Dispatcher-owned pre-work attestation. This record contains NO verification result.\n\n" + string(raw) + "\n", State: "closed", Author: deskkit.Account{Login: "example-desk[bot]"}, Labels: labels}, labels: events}
	v := &VerifyLoop{Root: root, RootRepo: "example-org/tracker", Issues: &forgeIssueSource{forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) { return f, nil }}, Now: fixedClock("2026-10-16T12:00:00Z")}
	return v, f, receipt, path
}
func pendingRecord(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, briefPath string, modify func(*deskkit.WakeReceipt)) {
	t.Helper()
	landed := f.blobs[f.pr.HeadSHA+":"+briefPath]
	record := deskkit.WakeReceipt{Brief: "example-stream/01", Repo: r.Binding.Repo, Verifier: "example-verifier[bot]", SHA: strings.Repeat("e", 40), TS: "2026-10-16T10:00:00Z", Outcome: "verify-fail", BlockerKind: deskkit.BlockerImplementation, BlockerRef: "#99"}
	raw, _ := json.Marshal(record)
	in := deskkit.OutcomeReceiptInput{BriefPath: briefPath, LandedBrief: landed, ToolVersion: deskkit.ReleaseTagOrDev(), Repo: r.Binding.Repo}
	if r.Binding.Version == 2 {
		in.SourceRepo, in.Source = r.Binding.SourceRepo, r.Binding.Source
	}
	out, e := deskkit.BuildOutcomeRecord(raw, in, pendingTree{root: v.Root, sha: r.BriefSource()})
	if e != nil {
		t.Fatal(e)
	}
	record, _ = deskkit.ParseWakeReceipt(out)
	if modify != nil {
		modify(&record)
	}
	out, _ = json.Marshal(record)
	name, e := deskkit.RecordName(out)
	if e != nil {
		t.Fatal(e)
	}
	f.blobs[record.SHA+":"+briefPath] = landed
	f.blobs[f.pr.HeadSHA+":"+name] = out
	f.files = append(f.files, deskkit.ChangedFile{Filename: name, Status: "added"})
	f.pr.ChangedFiles = len(f.files)
}

func pendingCross(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) (deskkit.VerifierReceipt, string) {
	t.Helper()
	rosterPath := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
	roster, _ := os.ReadFile(rosterPath)
	os.WriteFile(rosterPath, []byte(strings.Replace(string(roster), "example-org/tracker:ci:private", "example-org/tracker:ci:private,example-org/source:ci:private", 1)), 0600)
	deskkit.ReloadConfig()
	brief, _ := os.ReadFile(filepath.Join(v.Root, p))
	brief = []byte(strings.Replace(string(brief), "brief: x", "brief: x\ndeliverable_repo: source", 1))
	os.WriteFile(filepath.Join(v.Root, p), brief, 0600)
	os.WriteFile(filepath.Join(v.Root, "docs/streams/graph-repos.yaml"), []byte("schema: graph-repos-v1\nself: tracking\nrepos:\n  tracking: {repo: example-org/tracker}\n  source: {repo: example-org/source}\n"), 0600)
	gitT(t, v.Root, "add", p, "docs/streams/graph-repos.yaml")
	gitT(t, v.Root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "tracking association")
	gitT(t, v.Root, "remote", "add", "origin", "https://github.com/example-org/tracker.git")
	gitT(t, v.Root, "update-ref", "refs/remotes/origin/main", "HEAD")
	source := t.TempDir()
	gitT(t, source, "init", "-q")
	commitIn(t, source, "source.txt")
	gitT(t, source, "remote", "add", "origin", "https://github.com/example-org/source.git")
	gitT(t, source, "update-ref", "refs/remotes/origin/main", "HEAD")
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+v.Root+",example-org/source="+source)
	r.Binding.Version, r.Binding.SourceRepo = 2, "example-org/source"
	r.Binding.TrackingSource = gitT(t, v.Root, "rev-parse", "HEAD")
	r.Binding.Source = gitT(t, source, "rev-parse", "HEAD")
	r.Binding.TrackingHomeSHA256, r.Binding.TrackingCheckout = strings.Repeat("b", 64), r.Binding.Checkout
	r.Binding.BriefSHA256 = deskkit.Sha256Hex(brief)
	r.Binding.PlanSHA256 = deskkit.Sha256Hex([]byte(strings.TrimSpace(strings.Split(string(brief), "## Evidence")[0])))
	raw, _ := json.MarshalIndent(r.Binding, "", "  ")
	f.issue.Body = "Dispatcher-owned pre-work attestation. This record contains NO verification result.\n\n" + string(raw) + "\n"
	f.blobs[f.pr.HeadSHA+":"+p] = []byte(string(brief) + "\n" + r.EvidenceBinding() + "\n\n| 1 | check | FAIL | observed |\n")
	return r, source
}

// regression: #2939
func TestPendingForge(t *testing.T) {
	// author sets one account as the change's author on both the list and the change read.
	author := func(f *pendingForge, login string, id int64) {
		f.change.Author = deskkit.Account{Login: login, ID: id}
		f.pr.Author = f.change.Author
	}
	cases := []struct {
		name, want  string
		change      func(*testing.T, *VerifyLoop, *pendingForge, deskkit.VerifierReceipt, string)
		want02      string                          // the untouched brief 02; "" is dispatch
		implementer string                          // brief 01's declared implementer; "" declares none
		check       func(*testing.T, *pendingForge) // extra assertions after both items classify
	}{
		{name: "exact-source", want: "awaiting-merge", change: nil},
		{name: "unrelated-advance-no-wake", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			commitIn(t, v.Root, "unrelated")
		}},
		{name: "unrelated-advance-with-wake", want: "awaiting-merge", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			pendingRecord(t, v, f, r, p, nil)
			commitIn(t, v.Root, "unrelated")
		}},
		{name: "input-changes", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			pendingRecord(t, v, f, r, p, nil)
			if e := os.WriteFile(filepath.Join(v.Root, "input.go"), []byte("package changed\n"), 0600); e != nil {
				t.Fatal(e)
			}
			gitT(t, v.Root, "add", "input.go")
			gitT(t, v.Root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "changed source input")
		}},
		{name: "new-declared-path-wakes", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			pendingRecord(t, v, f, r, p, nil)
			commitIn(t, v.Root, "planned.go")
		}},
		{name: "plan-changes", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			b, _ := os.ReadFile(filepath.Join(v.Root, p))
			os.WriteFile(filepath.Join(v.Root, p), []byte(strings.Replace(string(b), "go test ./...", "go test ./pkg", 1)), 0600)
		}},
		// A non-verifier change cannot carry admitted Evidence: it establishes no hold and
		// no uncertainty, and is never read past the list (F1/S1 on the PR that added this).
		{name: "untrusted-author", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			author(f, "ada", 2001)
		}},
		// regression: S3 — a different rostered App (login and id both pinned) is not the verifier.
		{name: "other-app-author", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			author(f, "example-worker[bot]", 300000006)
		}},
		{name: "fork-author", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			author(f, "mallory", 9)
			f.change.HeadRepo, f.change.CrossRepo = "example-org/fork", deskkit.CrossRepoFork
		}, check: func(t *testing.T, f *pendingForge) {
			if f.prReads[51] != 0 {
				t.Fatalf("non-verifier change read %d times past the list", f.prReads[51])
			}
		}},
		// The list names the verifier but the change read names another pinned App: the
		// change read's login check is load-bearing, not only the list filter.
		{name: "list-get-author-mismatch", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.pr.Author = deskkit.Account{Login: "example-worker[bot]", ID: 300000006}
		}},
		// regression: S2 — an unreadable non-verifier change touches no item and is never read.
		{name: "unrelated-unreadable-change", want: "awaiting-merge", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.extra = []deskkit.ChangeRef{{Number: 52, Author: deskkit.Account{Login: "mallory", ID: 9}, CrossRepo: deskkit.CrossRepoFork, HeadRepo: "example-org/fork"}}
		}, check: func(t *testing.T, f *pendingForge) {
			if f.prReads[52] != 0 {
				t.Fatalf("unrelated non-verifier change read %d times", f.prReads[52])
			}
		}},
		// An unreadable verifier-authored change may touch any brief: items it could affect
		// are could-not-check, a hold proven by another change stands, and the failure is
		// read once per pass, not once per item.
		{name: "verifier-unreadable-change", want: "awaiting-merge", want02: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.extra = []deskkit.ChangeRef{{Number: 53, Author: deskkit.Account{Login: "example-verifier[bot]", ID: 300000005}, CrossRepo: deskkit.CrossRepoSame, HeadRepo: "example-org/tracker"}}
		}, check: func(t *testing.T, f *pendingForge) {
			if f.prReads[53] != 1 {
				t.Fatalf("failed change read %d times in one pass, want 1", f.prReads[53])
			}
		}},
		{name: "verifier-unbound", want: "could-not-check", want02: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			rosterPath := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
			roster, _ := os.ReadFile(rosterPath)
			os.WriteFile(rosterPath, []byte(strings.Replace(string(roster), ",verifier=example-verifier:300000005", "", 1)), 0600)
			deskkit.ReloadConfig()
		}},
		// regression: A1 — the verifier App authored the Evidence AND is brief 01's declared
		// implementer: no independent verifier, so no hold (could-not-check, never awaiting-merge).
		{name: "implementer-is-verifier", want: "could-not-check", implementer: "example-verifier[bot]"},
		{name: "wrong-id", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.pr.Author.ID = 123
		}},
		{name: "missing-id", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.pr.Author.ID = 0
		}},
		{name: "edited-attestation", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.edited = true
		}},
		{name: "wrong-stamp-actor", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			for i := range f.labels {
				f.labels[i].AppliedBy = "ada"
			}
		}},
		{name: "missing-file", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.missing = true
		}},
		{name: "unreadable-list", want: "could-not-check", want02: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.readFail = true
		}},
		{name: "partial-list", want: "could-not-check", want02: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.incomplete = true
			f.files = nil
			f.pr.ChangedFiles = 0
		}},
		{name: "fork-head", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.change.HeadRepo = "example-org/fork"
		}},
		{name: "unrelated-base", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.pr.BaseRef = "feature"
		}},
		{name: "dirty-source-no-receipt", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			os.WriteFile(filepath.Join(v.Root, "input.go"), []byte("package changed\n"), 0600)
		}},
		{name: "moved-head", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) { f.move = true }},
		{name: "narrowed-inputs", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			pendingRecord(t, v, f, r, p, func(w *deskkit.WakeReceipt) { delete(w.Inputs, "file:input.go") })
		}},
		{name: "recheck-wakes", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			pendingRecord(t, v, f, r, p, func(w *deskkit.WakeReceipt) {
				w.WakePredicate = deskkit.WakeExplicitRecheck
				w.RecheckReason = "operator request"
			})
		}},
		{name: "newer-landed-recheck", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			pendingRecord(t, v, f, r, p, nil)
			writeReceipts(t, v.Root, recheckReceipt("2026-10-16T11:00:00Z", "new operator recheck"))
		}},
		{name: "cross-provenance-missing", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			r.Binding.Version = 2
			r.Binding.SourceRepo = "example-org/unconfigured"
			raw, _ := json.MarshalIndent(r.Binding, "", "  ")
			f.issue.Body = "Dispatcher-owned pre-work attestation. This record contains NO verification result.\n\n" + string(raw) + "\n"
			f.blobs[f.pr.HeadSHA+":"+p] = []byte(strings.Replace(string(f.blobs[f.pr.HeadSHA+":"+p]), deskkit.VerifierEvidenceBindings(f.blobs[f.pr.HeadSHA+":"+p])[0], r.EvidenceBinding(), 1))
		}},
		{name: "cross-current", want: "awaiting-merge", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			pendingCross(t, v, f, r, p)
		}},
		{name: "cross-source-wake", want: "dispatch", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			r, source := pendingCross(t, v, f, r, p)
			pendingRecord(t, v, f, r, p, nil)
			commitIn(t, source, "new-source")
			gitT(t, source, "update-ref", "refs/remotes/origin/main", "HEAD")
		}},
		{name: "cross-tracking-advance", want: "awaiting-merge", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			r, _ = pendingCross(t, v, f, r, p)
			pendingRecord(t, v, f, r, p, nil)
			commitIn(t, v.Root, "unrelated")
			gitT(t, v.Root, "update-ref", "refs/remotes/origin/main", "HEAD")
		}},
		{name: "unreadable-input", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			pendingRecord(t, v, f, r, p, nil)
			os.Remove(filepath.Join(v.Root, "input.go"))
		}},
		{name: "forged-run", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.blobs[f.pr.HeadSHA+":"+p] = []byte(strings.Replace(string(f.blobs[f.pr.HeadSHA+":"+p]), r.Binding.Run, strings.Repeat("f", 64), 1))
		}},
		{name: "fenced-binding", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			f.blobs[f.pr.HeadSHA+":"+p] = []byte(strings.Replace(string(f.blobs[f.pr.HeadSHA+":"+p]), r.EvidenceBinding(), "```\n"+r.EvidenceBinding()+"\n```", 1))
		}},
		{name: "binding-only", want: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			b := string(f.blobs[f.pr.HeadSHA+":"+p])
			f.blobs[f.pr.HeadSHA+":"+p] = []byte(strings.Split(b, "\n\n| 1")[0])
		}},
		{name: "no-forge", want: "could-not-check", want02: "could-not-check", change: func(t *testing.T, v *VerifyLoop, f *pendingForge, r deskkit.VerifierReceipt, p string) {
			v.Issues = offlineScheduling{}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var implementer []string
			if tc.implementer != "" {
				implementer = []string{tc.implementer}
			}
			v, f, r, p := pendingSetup(t, implementer...)
			if tc.change != nil {
				tc.change(t, v, f, r, p)
			}
			items, e := v.SelectQueue()
			if e != nil {
				t.Fatal(e)
			}
			for _, it := range items {
				tier, _ := v.TierPolicy(it)
				d, why := classifyItem(it, tier)
				want := tc.want
				if strings.HasSuffix(it.ID, "/02") {
					want = tc.want02
					if want == "" {
						want = "dispatch"
					}
				}
				if d.String() != want {
					t.Fatalf("%s = %s (%s), want %s", it.ID, d, why, want)
				}
				if d.String() == "awaiting-merge" {
					if _, e := v.Dispatch(it, tier); e == nil {
						t.Fatal("pending Evidence reached dispatch")
					}
				}
			}
			if f.lists > 1 {
				t.Fatalf("list read %d times", f.lists)
			}
			if tc.check != nil {
				tc.check(t, f)
			}
		})
	}
	// The rendered plan is the public queue consumer, not just a private predicate.
	t.Run("plan", func(t *testing.T) {
		v, f, _, _ := pendingSetup(t)
		old := issueSourceFn
		t.Cleanup(func() { issueSourceFn = old })
		issueSourceFn = func() deskkit.IssueStateSource { return v.Issues }
		gitT(t, v.Root, "remote", "add", "origin", "https://github.com/example-org/tracker.git")
		var e error
		out := captureStdout(t, func() { e = cmdPlan([]string{"--root", v.Root}) })
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(out, "awaiting-merge #51") || strings.Contains(out, "=== DISPATCH example-stream/01 ") || !strings.Contains(out, "wait=1 dispatchable=1 could-not-check=0") {
			t.Fatal(out)
		}
		_ = f
	})
}
