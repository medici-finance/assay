package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const workerRoster = "ASSAY_BLESS_LOGIN=example-human:2001\nASSAY_TRUSTED_LOGINS=example-human:2001\n" +
	"ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk:1,worker=example-worker:3\nASSAY_ALLOWED_REPOS=example-org/one:ci:private\n"

// workerFixture builds a worktree on the dispatched branch with a brief under docs/, plants a
// roster binding distinct desk and worker actors, and returns the home and the brief's path.
func workerFixture(t *testing.T) (home, brief string) {
	t.Helper()
	plantRoster(t, workerRoster)
	home = t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", home}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	git("init")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	brief = filepath.Join(home, "docs", "streams", "example", "brief-01-thing.md")
	if err := os.MkdirAll(filepath.Dir(brief), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brief, []byte("# Thing\n\n## Verify\n\n| 1 | true | exit 0 |\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "fixture")
	git("checkout", "-b", "feat/example-01")
	return home, brief
}

func workerInput(home, brief string) WorkerBindingInput {
	return WorkerBindingInput{Repo: "example-org/one", Item: "one--example--01", Kit: "worker", Branch: "feat/example-01",
		Home: home, BriefPath: brief, Model: "gpt-6.1-sol", Tier: "strong"}
}

func TestWorkerAttestationHappyPathReadsBackClosedStampedRecord(t *testing.T) {
	home, brief := workerFixture(t)
	f := &verifierForge{actor: "example-desk[bot]"}
	b, err := PrepareWorkerBinding(f, workerInput(home, brief))
	if err != nil {
		t.Fatal(err)
	}
	if b.Brief != "docs/streams/example/brief-01-thing.md" || strings.Contains(workerBody(b), home) {
		t.Fatalf("brief must be repo-relative and the body must carry no local path: %q", b.Brief)
	}
	if len(b.Base) != 40 || b.Branch != "feat/example-01" || len(b.Run) != 64 {
		t.Fatalf("binding not taken from the worktree: %+v", b)
	}
	r, err := IssueWorkerAttestation(f, b)
	if err != nil {
		t.Fatal(err)
	}
	if r.Issue != 41 || r.Binding != b || f.issue.State != "closed" || f.creates != 1 || f.closes != 1 {
		t.Fatalf("receipt %+v state=%s creates=%d closes=%d", r, f.issue.State, f.creates, f.closes)
	}
	if !strings.HasPrefix(f.issue.Title, WorkerAttestationTitle) {
		t.Fatalf("title %q", f.issue.Title)
	}
	for _, l := range f.issue.Labels {
		if strings.HasPrefix(l, "dispatched-") {
			t.Fatalf("worker attestation wrote a dispatched-* label: %v", f.issue.Labels)
		}
	}
	if got, err := ReadWorkerAttestation(f, ForgeRepo{Owner: "example-org", Name: "one"}, 41); err != nil || got.Binding != b {
		t.Fatalf("readback %+v %v", got, err)
	}
	ref, ok, err := ParseWorkerAttestationLine("Summary.\n\n" + r.PRBinding() + "\n")
	if err != nil || !ok || ref.Issue != 41 || ref.Run != b.Run || ref.Repo != "example-org/one" {
		t.Fatalf("PR line did not round-trip: %+v %v %v", ref, ok, err)
	}
}

func TestWorkerAttestationIssueSourceBindsBodyDigest(t *testing.T) {
	home, _ := workerFixture(t)
	f := &verifierForge{actor: "example-desk[bot]", issue: &Issue{Number: 9, Body: "do the thing"}}
	in := workerInput(home, "")
	in.SourceIssue = 9
	b, err := PrepareWorkerBinding(f, in)
	if err != nil {
		t.Fatal(err)
	}
	if b.SourceIssue != 9 || b.SourceIssueSHA256 != verifierDigest([]byte("do the thing")) || b.Brief != "" {
		t.Fatalf("issue source not bound: %+v", b)
	}
	in.BriefPath = "/elsewhere/brief.md"
	if _, err := PrepareWorkerBinding(f, in); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("two sources must refuse: %v", err)
	}
}

func TestWorkerAttestationRefusesWrongBranchAndSharedActor(t *testing.T) {
	home, brief := workerFixture(t)
	in := workerInput(home, brief)
	in.Branch = "feat/other"
	if _, err := PrepareWorkerBinding(&verifierForge{}, in); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("worktree on another branch admitted: %v", err)
	}
	plantRoster(t, strings.Replace(workerRoster, "worker=example-worker:3", "worker=example-desk:1", 1))
	if _, err := PrepareWorkerBinding(&verifierForge{}, workerInput(home, brief)); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("desk and worker bound to one actor admitted: %v", err)
	}
}

// failForge fails one named step of the record's lifecycle.
type failForge struct {
	*verifierForge
	closeErr bool
}

func (f *failForge) CloseIssueTyped(r ForgeRepo, n int, k TargetKind, reason string) error {
	if f.closeErr {
		return errors.New("close refused")
	}
	return f.verifierForge.CloseIssueTyped(r, n, k, reason)
}

// swapForge replaces the record, at close, with another run's complete record: every check on
// the record itself passes, so only the readback-equals-issued check can refuse.
type swapForge struct {
	*verifierForge
	to WorkerBinding
}

func (f *swapForge) CloseIssueTyped(r ForgeRepo, n int, k TargetKind, reason string) error {
	if err := f.verifierForge.CloseIssueTyped(r, n, k, reason); err != nil {
		return err
	}
	f.issue.Title, f.issue.Body = WorkerAttestationTitle+f.to.Run, workerBody(f.to)
	return nil
}

func TestWorkerAttestationReadbackOfAnotherRunRefuses(t *testing.T) {
	home, brief := workerFixture(t)
	f := &swapForge{verifierForge: &verifierForge{actor: "example-desk[bot]"}}
	b, err := PrepareWorkerBinding(f, workerInput(home, brief))
	if err != nil {
		t.Fatal(err)
	}
	f.to = b
	f.to.Run = strings.Repeat("d", 64)
	r, err := IssueWorkerAttestation(f, b)
	if ExitCodeOf(err) != ExitRefused || err == nil || !strings.Contains(err.Error(), "readback binds a different run") {
		t.Fatalf("readback of another run admitted: %+v %v", r, err)
	}
	if got, rerr := ReadWorkerAttestation(f, ForgeRepo{Owner: "example-org", Name: "one"}, 41); rerr != nil || got.Binding != f.to {
		t.Fatalf("fixture: the swapped record must itself be complete, so only the equality check refuses: %v", rerr)
	}
}

func TestWorkerAttestationEveryFailureIsNonSuccess(t *testing.T) {
	home, brief := workerFixture(t)
	for name, tc := range map[string]struct {
		forge func() Forge
		code  int
	}{
		"create lost":     {func() Forge { return &verifierForge{actor: "example-desk[bot]", loseCreate: true} }, ExitUnverifiable},
		"stamp dropped":   {func() Forge { return &verifierForge{actor: "example-desk[bot]", drop: true} }, ExitUnverifiable},
		"foreign applier": {func() Forge { return &verifierForge{actor: "example-worker[bot]"} }, ExitUnverifiable},
		"edited record":   {func() Forge { return &verifierForge{actor: "example-desk[bot]", edited: true} }, ExitRefused},
		"history unknown": {func() Forge { return &verifierForge{actor: "example-desk[bot]", missingHistory: true} }, ExitRefused},
		"close failed": {func() Forge {
			return &failForge{verifierForge: &verifierForge{actor: "example-desk[bot]"}, closeErr: true}
		}, ExitUnverifiable},
		"record unreadable": {func() Forge { return &verifierForge{actor: "example-desk[bot]", unreadable: true} }, ExitUnverifiable},
	} {
		t.Run(name, func(t *testing.T) {
			f := tc.forge()
			b, err := PrepareWorkerBinding(f, workerInput(home, brief))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := IssueWorkerAttestation(f, b); err == nil || ExitCodeOf(err) != tc.code {
				t.Fatalf("want exit %d, got %v", tc.code, err)
			}
			if vf, ok := f.(*verifierForge); ok && vf.issue != nil && vf.issue.State == "closed" && name != "record unreadable" {
				t.Fatal("a failed attestation was closed as complete")
			}
		})
	}
}

func TestWorkerAttestationOpenOrUnstampedRecordNeverBinds(t *testing.T) {
	home, brief := workerFixture(t)
	repo := ForgeRepo{Owner: "example-org", Name: "one"}

	// A failed close leaves exactly a stamped-but-open record: issue a complete, stamped record,
	// then reopen it, so the state check is the only guard left that can refuse.
	f := &verifierForge{actor: "example-desk[bot]"}
	b, err := PrepareWorkerBinding(f, workerInput(home, brief))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := IssueWorkerAttestation(f, b); err != nil {
		t.Fatal(err)
	}
	f.issue.State = "open"
	if got, err := ReadWorkerAttestation(f, repo, 41); err == nil || ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "record still open") {
		t.Fatalf("stamped but open record bound: %+v %v", got, err)
	}

	// The same stamped, closed record read through another repository: only the binding's repo
	// check can refuse it.
	f.issue.State = "closed"
	if _, err := ReadWorkerAttestation(f, repo, 41); err != nil {
		t.Fatalf("complete record must read back before the cross-repository case: %v", err)
	}
	other := ForgeRepo{Owner: "example-org", Name: "two"}
	if _, err := ReadWorkerAttestation(f, other, 41); ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "belongs to a different repository") {
		t.Fatalf("record bound in another repository: %v", err)
	}

	// A record whose body binds this run but whose title names another run is not this run's
	// record: only the exact-title check can refuse it.
	f.issue.Title = WorkerAttestationTitle + strings.Repeat("d", 64)
	if _, err := ReadWorkerAttestation(f, repo, 41); ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "not the exact dispatcher-authored run record") {
		t.Fatalf("record titled for another run bound: %v", err)
	}

	// A closed record that was never stamped refuses on the stamp.
	u := &verifierForge{actor: "example-desk[bot]"}
	if _, err := u.FileIssue(repo, IssueInput{Title: WorkerAttestationTitle + b.Run, Body: workerBody(b)}); err != nil {
		t.Fatal(err)
	}
	u.issue.State = "closed"
	if _, err := ReadWorkerAttestation(u, repo, 41); ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "authoring stamp") {
		t.Fatalf("closed but unstamped record bound: %v", err)
	}
}

func TestWorkerChangeBindingRefusesEveryMismatch(t *testing.T) {
	r := WorkerReceipt{Issue: 41, Binding: WorkerBinding{Repo: "example-org/one", Kit: "worker", Branch: "feat/example-01", Model: "gpt-6.1-sol", Tier: "strong"}}
	if err := CheckWorkerChangeBinding(r, "example-org/one", "worker", "GPT-6.1-SOL", "strong", "feat/example-01"); err != nil {
		t.Fatalf("matching selection refused: %v", err)
	}
	for name, args := range map[string][5]string{
		"model":  {"example-org/one", "worker", "gpt-6-astra", "strong", "feat/example-01"},
		"tier":   {"example-org/one", "worker", "gpt-6.1-sol", "any", "feat/example-01"},
		"kit":    {"example-org/one", "worker-objective", "gpt-6.1-sol", "strong", "feat/example-01"},
		"repo":   {"example-org/two", "worker", "gpt-6.1-sol", "strong", "feat/example-01"},
		"branch": {"example-org/one", "worker", "gpt-6.1-sol", "strong", "feat/other"},
	} {
		if err := CheckWorkerChangeBinding(r, args[0], args[1], args[2], args[3], args[4]); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%s mismatch not refused: %v", name, err)
		}
	}
}

func TestParseWorkerAttestationLineRefusesAmbiguity(t *testing.T) {
	run := strings.Repeat("a", 64)
	line := "Worker-Attestation: example-org/one#41 run=" + run
	if _, ok, err := ParseWorkerAttestationLine("no binding here"); ok || err != nil {
		t.Fatalf("absent line: %v %v", ok, err)
	}
	if _, _, err := ParseWorkerAttestationLine(line + "\n" + line); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("duplicate line admitted: %v", err)
	}
	if _, _, err := ParseWorkerAttestationLine("Worker-Attestation: #41"); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("malformed line admitted: %v", err)
	}
}

func TestAttestationRecordListingPolicy(t *testing.T) {
	plantRoster(t, workerRoster)
	if !IsAttestationRecord(WorkerAttestationTitle+"x", "example-desk[bot]") {
		t.Fatal("dispatcher-authored worker record listed as work")
	}
	if IsAttestationRecord(WorkerAttestationTitle+"x", "example-human") || IsAttestationRecord("ordinary", "example-desk[bot]") {
		t.Fatal("look-alike or ordinary issue hidden from intake")
	}
	if !sharedIssueSkip(WorkerAttestationTitle+"x", "example-desk[bot]") {
		t.Fatal("shared listing policy does not skip worker records")
	}
}
