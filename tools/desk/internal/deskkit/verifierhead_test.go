package deskkit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// verifierhead_test.go — the pre-merge verifier target (#7623). Every refusal
// here is a guard of the security boundary the target loosens: unmerged code
// runs with the verifier's credential only at an opted-in repository, at the
// exact head the reviewer App approved, authored by someone other than the
// witness, and only while that head stays the change's head.

const (
	phRepo     = "example-org/one"
	phReviewer = "example-reviewer[bot]"
	phWorker   = "example-worker[bot]"
)

// headForge is verifierForge plus the change and review reads admission makes.
// moveOn makes the change's head read as moved from the given GetPullRequest
// call onward (1-based), so a head that moves between reads is observable.
type headForge struct {
	*verifierForge
	pr             PullRequest
	reviews        []Review
	prErr, revErr  error
	prReads, moved int
	moveOn         int
}

func (f *headForge) GetPullRequest(_ ForgeRepo, n int) (*PullRequest, error) {
	f.prReads++
	if f.prErr != nil {
		return nil, f.prErr
	}
	pr := f.pr
	pr.Number = n
	if f.moveOn > 0 && f.prReads >= f.moveOn {
		pr.HeadSHA = strings.Repeat("e", 40)
	}
	return &pr, nil
}

func (f *headForge) ReviewsAtHead(ForgeRepo, int) ([]Review, error) {
	if f.revErr != nil {
		return nil, f.revErr
	}
	return f.reviews, nil
}

func phRoster(t *testing.T, premerge string) {
	t.Helper()
	body := "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\n" +
		"ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk:1,verifier=example-verifier:2,reviewer=example-reviewer:3,worker=example-worker:4\n" +
		"ASSAY_ALLOWED_REPOS=example-org/one:ci:private,example-org/two:ci:private\n"
	if premerge != "" {
		body += EnvVerifierPremergeRepos + "=" + premerge + "\n"
	}
	plantRoster(t, body)
}

// phFixture builds the fixture repository, then commits an unmerged change on
// top of origin/main and leaves the home detached at that change's head.
func phFixture(t *testing.T, premerge string) (string, string, *headForge) {
	t.Helper()
	root := t.TempDir()
	verifierFixtureRepo(t, root, nil)
	phRoster(t, premerge)
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("unmerged change"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", "change"}} {
		if out, err := verifierFixtureGitCommand(append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	head, err := verifierGit(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	f := &headForge{
		verifierForge: &verifierForge{actor: "example-desk[bot]"},
		pr:            PullRequest{State: "open", Draft: true, Author: Account{Login: phWorker, Type: "Bot"}, HeadSHA: head},
		reviews:       []Review{{Author: Account{Login: phReviewer, Type: "Bot"}, State: "APPROVED", CommitID: head, SubmittedAt: "2026-11-23T01:00:00Z"}},
	}
	return root, head, f
}

func phPrepare(root, head string, f *headForge) error {
	return PrepareVerifierHeadAttestationAt(root, phRepo, "brief.md", "gpt-6.1-sol", "strong", VerifierHeadTarget{Change: 12, Head: head}, f)
}

func phNoRecord(t *testing.T, root string) {
	t.Helper()
	record, err := verifierRecordPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("a refused admission left a prepared run record")
	}
}

// regression: #7623
// TestPremergeApprovedHeadAdmits: an opted-in repository's change, approved
// by the reviewer App at exactly its current head, prepares, stamps and
// admits rows. Its record names the target, and it never lands Evidence.
func TestPremergeApprovedHeadAdmits(t *testing.T) {
	root, head, f := phFixture(t, phRepo)
	if err := phPrepare(root, head, f); err != nil {
		t.Fatal(err)
	}
	receipt, err := IssueVerifierAttestation(root, f)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Binding.TargetChange != 12 || receipt.Binding.TargetHead != head || receipt.Binding.Source != head {
		t.Fatalf("binding does not name the target: %+v", receipt.Binding)
	}
	if !strings.Contains(f.issue.Body, head) || !strings.Contains(f.issue.Body, "TargetChange") {
		t.Fatal("the run record does not carry the target")
	}
	if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err != nil {
		t.Fatalf("approved head refused at the use point: %v", err)
	}
	_, err = CheckVerifierEvidenceWithForge(root, phRepo, "brief.md", f)
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("a pre-merge run admitted Evidence: %v", err)
	}
}

// regression: #7623
// TestPremergeRefusals: each admission guard refuses before any run record
// exists. Exit 5 is a refusal; exit 6 is could-not-check, never an admission.
func TestPremergeRefusals(t *testing.T) {
	other := strings.Repeat("c", 40)
	human := Account{Login: "example-human", Type: "User"}
	for _, c := range []struct {
		name, premerge string
		mutate         func(f *headForge, head string)
		want           int
	}{
		{"unapproved head", phRepo, func(f *headForge, _ string) { f.reviews = nil }, ExitRefused},
		{"approval at other sha", phRepo, func(f *headForge, _ string) { f.reviews[0].CommitID = other }, ExitRefused},
		{"human approval", phRepo, func(f *headForge, _ string) { f.reviews[0].Author = human }, ExitRefused},
		{"other app approval", phRepo, func(f *headForge, _ string) { f.reviews[0].Author.Login = phWorker }, ExitRefused},
		{"commented only", phRepo, func(f *headForge, _ string) { f.reviews[0].State = "COMMENTED" }, ExitRefused},
		{"changes then approve", phRepo, func(f *headForge, head string) {
			f.reviews = append([]Review{{Author: Account{Login: phReviewer}, State: "CHANGES_REQUESTED", CommitID: head}}, f.reviews...)
		}, ExitRefused},
		{"approve then changes", phRepo, func(f *headForge, head string) {
			f.reviews = append(f.reviews, Review{Author: Account{Login: phReviewer}, State: "CHANGES_REQUESTED", CommitID: head})
		}, ExitRefused},
		{"opted out", "", func(*headForge, string) {}, ExitRefused},
		{"other repo opted in", "example-org/two", func(*headForge, string) {}, ExitRefused},
		{"implementer witness", phRepo, func(f *headForge, _ string) { f.pr.Author.Login = "example-verifier[bot]" }, ExitRefused},
		{"reviewer authored", phRepo, func(f *headForge, _ string) { f.pr.Author.Login = phReviewer }, ExitRefused},
		{"dispatcher authored", phRepo, func(f *headForge, _ string) { f.pr.Author.Login = "example-desk[bot]" }, ExitRefused},
		{"moved head", phRepo, func(f *headForge, _ string) { f.pr.HeadSHA = other }, ExitRefused},
		{"moved during reviews", phRepo, func(f *headForge, _ string) { f.moveOn = 2 }, ExitRefused},
		{"closed change", phRepo, func(f *headForge, _ string) { f.pr.State = "closed" }, ExitRefused},
		{"change unreadable", phRepo, func(f *headForge, _ string) { f.prErr = errors.New("offline") }, ExitUnverifiable},
		{"reviews unreadable", phRepo, func(f *headForge, _ string) { f.revErr = errors.New("offline") }, ExitUnverifiable},
		{"author unreadable", phRepo, func(f *headForge, _ string) { f.pr.Author.Login = "" }, ExitUnverifiable},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, head, f := phFixture(t, c.premerge)
			c.mutate(f, head)
			err := phPrepare(root, head, f)
			if err == nil || ExitCodeOf(err) != c.want {
				t.Fatalf("%s: err=%v exit=%d, want exit %d", c.name, err, ExitCodeOf(err), c.want)
			}
			phNoRecord(t, root)
			if f.creates != 0 {
				t.Fatal("a refused admission filed a run record")
			}
		})
	}
}

// regression: #7623
// TestPremergeShapeRefusals: an incomplete or abbreviated target, a home not
// at the target head, and a cross-repository brief are all refused.
func TestPremergeShapeRefusals(t *testing.T) {
	root, head, f := phFixture(t, phRepo)
	for _, short := range []string{head[:7], head[:12], strings.ToUpper(head), head + "0", "refs/pull/12/head"} {
		if (VerifierHeadTarget{Change: 12, Head: short}).Problem() == "" {
			t.Errorf("target head %q accepted; only a full lowercase 40-hex commit is a target", short)
		}
	}
	for name, tg := range map[string]VerifierHeadTarget{
		"no change":  {Head: head},
		"short head": {Change: 12, Head: head[:12]},
		"upper head": {Change: 12, Head: strings.ToUpper(head)},
		"other head": {Change: 12, Head: strings.Repeat("c", 40)},
		"zero":       {},
	} {
		err := PrepareVerifierHeadAttestationAt(root, phRepo, "brief.md", "gpt-6.1-sol", "strong", tg, f)
		if ExitCodeOf(err) != ExitRefused {
			t.Fatalf("%s: err=%v, want refused", name, err)
		}
	}
	// The forge's change sits at an approved commit the home is NOT at.
	parent, err := verifierGit(root, "rev-parse", "HEAD^")
	if err != nil {
		t.Fatal(err)
	}
	f.pr.HeadSHA, f.reviews[0].CommitID = parent, parent
	err = PrepareVerifierHeadAttestationAt(root, phRepo, "brief.md", "gpt-6.1-sol", "strong", VerifierHeadTarget{Change: 12, Head: parent}, f)
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("a home not at the target head: %v", err)
	}
	cross := filepath.Join(t.TempDir(), "brief.md")
	err = PrepareVerifierHeadAttestationAt(root, phRepo, cross, "gpt-6.1-sol", "strong", VerifierHeadTarget{Change: 12, Head: head}, f)
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("cross-repository pre-merge run: %v", err)
	}
	phNoRecord(t, root)
}

// regression: #7623
// TestPremergeVoidAtUsePoint: after admission, the change is read again at
// every use point. A head that moved, an approval withdrawn, a change closed
// or merged, or an unreadable forge each stops the rows.
func TestPremergeVoidAtUsePoint(t *testing.T) {
	for _, c := range []struct {
		name   string
		mutate func(f *headForge, head string)
		want   int
	}{
		{"moved head", func(f *headForge, _ string) { f.pr.HeadSHA = strings.Repeat("d", 40) }, ExitRefused},
		{"approval withdrawn", func(f *headForge, head string) {
			f.reviews = append(f.reviews, Review{Author: Account{Login: phReviewer}, State: "CHANGES_REQUESTED", CommitID: head})
		}, ExitRefused},
		{"change merged", func(f *headForge, _ string) { f.pr.State = "closed" }, ExitRefused},
		{"opt-in removed", func(*headForge, string) { phRoster(t, "") }, ExitRefused},
		{"forge unreadable", func(f *headForge, _ string) { f.prErr = errors.New("offline") }, ExitUnverifiable},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, head, f := phFixture(t, phRepo)
			if err := phPrepare(root, head, f); err != nil {
				t.Fatal(err)
			}
			if _, err := IssueVerifierAttestation(root, f); err != nil {
				t.Fatal(err)
			}
			c.mutate(f, head)
			_, err := CheckVerifierAttestationWithForge(root, "brief.md", f)
			if err == nil || ExitCodeOf(err) != c.want {
				t.Fatalf("%s at the use point: err=%v exit=%d, want exit %d", c.name, err, ExitCodeOf(err), c.want)
			}
		})
	}
}

// regression: #7623
// TestPremergeRecordTamper: stripping the target from the local record leaves
// a binding whose rendering is not the stamped run record, and a target
// whose head is not the bound source is refused locally.
func TestPremergeRecordTamper(t *testing.T) {
	for _, what := range []string{"stripped", "retargeted"} {
		t.Run(what, func(t *testing.T) {
			root, head, f := phFixture(t, phRepo)
			if err := phPrepare(root, head, f); err != nil {
				t.Fatal(err)
			}
			if _, err := IssueVerifierAttestation(root, f); err != nil {
				t.Fatal(err)
			}
			r, err := verifierLoad(root)
			if err != nil {
				t.Fatal(err)
			}
			if what == "stripped" {
				r.Binding.TargetChange, r.Binding.TargetHead = 0, ""
			} else {
				r.Binding.TargetHead = strings.Repeat("d", 40)
			}
			if err := verifierSave(root, r); err != nil {
				t.Fatal(err)
			}
			if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil {
				t.Fatalf("%s record admitted", what)
			}
		})
	}
}

// regression: #7623
// TestPremergeMainUnchanged: a merged-main run is unchanged by the target. It
// binds no target, reads no change, and still refuses a home that is not at
// origin/main, even in a repository opted in to pre-merge runs.
func TestPremergeMainUnchanged(t *testing.T) {
	root := t.TempDir()
	verifierFixtureRepo(t, root, nil)
	phRoster(t, phRepo)
	if err := PrepareVerifierAttestation(root, phRepo, "brief.md", "gpt-6.1-sol", "strong"); err != nil {
		t.Fatal(err)
	}
	f := &headForge{verifierForge: &verifierForge{actor: "example-desk[bot]"}, prErr: errors.New("a merged-main run read a change")}
	receipt, err := IssueVerifierAttestation(root, f)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Binding.TargetChange != 0 || receipt.Binding.TargetHead != "" || strings.Contains(f.issue.Body, "Target") {
		t.Fatalf("merged-main binding carries a target: %+v", receipt.Binding)
	}
	if _, err := CheckVerifierAttestationWithForge(root, "brief.md", f); err != nil {
		t.Fatal(err)
	}
	if f.prReads != 0 {
		t.Fatal("a merged-main run read a change")
	}

	moved, head, _ := phFixture(t, phRepo)
	_ = head
	if err := PrepareVerifierAttestation(moved, phRepo, "brief.md", "gpt-6.1-sol", "strong"); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("a merged-main run off origin/main: %v", err)
	}
}

// regression: #7623
// TestPremergeRosterKey: the opt-in is a trust key. A basename, a pattern or a
// repeated repository refuses the whole roster; unset is complete and off.
func TestPremergeRosterKey(t *testing.T) {
	for _, bad := range []string{"one", "example-org/*", "example-org/one,example-org/one", "/one"} {
		var probs []string
		got := parsePremergeRepos(bad, func(f string, a ...any) { probs = append(probs, f) })
		if len(probs) == 0 || len(got) != 0 {
			t.Errorf("%q: problems=%v set=%v, want a refusal and no set", bad, probs, got)
		}
	}
	phRoster(t, "example-org/one,example-org/*")
	if EffectiveConfig().Configured() {
		t.Fatal("a malformed opt-in left the roster configured")
	}
	phRoster(t, "")
	if err := VerifierPremergeOptedIn(phRepo); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("unset opt-in: %v, want refused", err)
	}
	phRoster(t, "Example-Org/One")
	if err := VerifierPremergeOptedIn(phRepo); err != nil {
		t.Fatalf("listed repository: %v", err)
	}
}
