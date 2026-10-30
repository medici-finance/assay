package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func crossLandGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func crossLandRepo(t *testing.T, repo string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	crossLandGit(t, root, "init")
	crossLandGit(t, root, "config", "user.name", "fixture")
	crossLandGit(t, root, "config", "user.email", "fixture@example.invalid")
	crossLandGit(t, root, "remote", "add", "origin", "https://github.com/"+repo+".git")
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		crossLandGit(t, root, "add", name)
	}
	crossLandGit(t, root, "commit", "-m", "fixture")
	crossLandGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	crossLandGit(t, root, "update-ref", "refs/remotes/origin/evidence-branch", "HEAD")
	crossLandGit(t, root, "checkout", "--detach")
	return root
}

// regression: #2485
func TestCrossRepoEvidenceOutcome(t *testing.T) { crossRepoOutcomeCase(t, "source.txt") }

// A cross-repository brief names its deliverable as ../<sibling>/<path>.
// regression: #3483
func TestCrossRepoSiblingOutcome(t *testing.T) { crossRepoOutcomeCase(t, "../source/source.txt") }

// The same run refuses a sibling its binding never named, before any write.
// regression: #3483
func TestCrossRepoSiblingEscape(t *testing.T) { crossRepoOutcomeCase(t, "../other/source.txt") }

// A brief declaring two sibling repositories binds one source per sibling: the
// run's own deliverable and the configured, fetched checkout of the other.
// regression: #4876
func TestCrossRepoTwoSiblings(t *testing.T) {
	crossRepoOutcomeRun(t, []string{"../source/source.txt", "../other/other.txt"}, true)
}

// A second sibling with no bound source stays refused, whatever the first covers.
// regression: #4876
func TestCrossRepoSiblingUnbound(t *testing.T) {
	crossRepoOutcomeRun(t, []string{"../source/source.txt", "../other/other.txt"}, false)
}

func crossRepoOutcomeCase(t *testing.T, decl string) { crossRepoOutcomeRun(t, []string{decl}, false) }

// crossRepoOutcomeRun lands Evidence and then an outcome for a brief declaring
// decls. withOther configures a fetched example-org/other checkout as a
// sibling root; without it every ../other/ declaration is an unbound sibling.
func crossRepoOutcomeRun(t *testing.T, decls []string, withOther bool) {
	decl := strings.Join(decls, "`\n- `")
	escape := strings.Contains(decl, "../other/") && !withOther
	for _, line := range strings.Split(fixtureRoster, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && strings.HasPrefix(k, "ASSAY_") {
			t.Setenv(k, v)
		}
	}
	t.Setenv(deskkit.EnvAllowedRepos, "example-org/source:ci:public,example-org/tracker:ci:private,example-org/other:ci:public")
	t.Setenv(deskkit.EnvRepoForges, "example-org/source=github,example-org/tracker=github,example-org/other=github")
	other := ""
	if withOther {
		other = crossLandRepo(t, "example-org/other", map[string]string{"other.txt": "sibling\n"})
		t.Setenv(deskkit.RootsEnv, "example-org/other="+other)
	}
	t.Setenv("DESK_LOOP", "verify-desk")
	t.Setenv("DESK_HUMAN", "alex")
	deskkit.SetToolClass(deskkit.ClassCI)
	t.Cleanup(func() { deskkit.SetToolClass(deskkit.ClassWrite) })
	const brief = "docs/streams/sample/brief-01-check.md"
	body := "---\nhomed-in: example-org/source\n---\n# Tracking canary\n\n## Context\nfiles:\n- `" + decl + "`\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `test -f source.txt` | exit 0 |\n\n## Evidence\n\nPending.\n"
	source := crossLandRepo(t, "example-org/source", map[string]string{"source.txt": "observed\n"})
	tracking := crossLandRepo(t, "example-org/tracker", map[string]string{brief: body, "docs/streams/graph-repos.yaml": "schema: graph-repos-v1\nrepos:\n  source:\n    repo: example-org/source\n"})
	api := &attestationAPI{files: map[string]string{brief: body}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/repos/example-org/tracker" {
			_ = json.NewEncoder(w).Encode(map[string]any{"private": true, "visibility": "private", "default_branch": "main"})
			return
		}
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/issues/7") {
			_ = json.NewEncoder(w).Encode(map[string]any{"number": 7, "state": "open", "title": "fixture blocker"})
			return
		}
		api.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) { return "fixture-token", srv.URL, nil })
	t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(nil) })
	if err := deskkit.PrepareVerifierAttestation(source, "example-org/source", filepath.Join(tracking, brief), "gpt-6-astra", "strong"); err != nil {
		t.Fatal(err)
	}
	receipt, err := deskkit.RecoverVerifierAttestation(source)
	if err != nil {
		t.Fatal(err)
	}
	br := receipt.BriefRoot
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(br)) })
	// The fake forge commits in a separate real repository and supplies its
	// objects to the retained tracking home without moving the admitted HEAD.
	remote := filepath.Join(t.TempDir(), "remote")
	crossLandGit(t, tracking, "clone", "--no-hardlinks", tracking, remote)
	crossLandGit(t, remote, "config", "user.name", "assay-verifier-app[bot]")
	crossLandGit(t, remote, "config", "user.email", "300000005+assay-verifier-app[bot]@users.noreply.github.com")
	api.commit = func(file, content string) string {
		p := filepath.Join(remote, file)
		os.MkdirAll(filepath.Dir(p), 0700)
		os.WriteFile(p, []byte(content), 0600)
		crossLandGit(t, remote, "add", file)
		crossLandGit(t, remote, "commit", "-m", "Evidence fixture")
		sha := crossLandGit(t, remote, "rev-parse", "HEAD")
		crossLandGit(t, br, "fetch", remote, "HEAD")
		return sha
	}
	oldMint := mintTokenFn
	mintTokenFn = func(string) error { return nil }
	t.Cleanup(func() { mintTokenFn = oldMint })
	oldBudget := evidenceWriteBudget
	budgetCalls := 0
	evidenceWriteBudget = func(tool, repo string, pr int) error {
		budgetCalls++
		if tool != "deskevidence" || repo != "example-org/tracker" {
			t.Fatal("wrong write budget")
		}
		return nil
	}
	t.Cleanup(func() { evidenceWriteBudget = oldBudget })
	// This test pins cross-repository outcome binding, not the decision-gate
	// hold; the hold has its own suites (decisiongate_test.go), so the fake
	// forge here does not serve the compare/tip reads it needs.
	oldDecisionGate := decisionGateDiffFn
	decisionGateDiffFn = func(string, string, []byte, bool, []byte, []boardEdit) ([]string, error) { return nil, nil }
	t.Cleanup(func() { decisionGateDiffFn = oldDecisionGate })
	oldBranchBoard := decisionGateBranchBoardFn
	decisionGateBranchBoardFn = func(deskkit.Forge, deskkit.ForgeRepo, string, string, string) ([]boardEdit, error) { return nil, nil }
	t.Cleanup(func() { decisionGateBranchBoardFn = oldBranchBoard })
	oldLint := lintDiffFn
	lintDiffFn = func(root, path string, content []byte) ([]string, error) {
		if root != br {
			t.Fatalf("lint read source instead of tracking: %s", root)
		}
		return nil, nil
	}
	t.Cleanup(func() { lintDiffFn = oldLint })
	rows := "| 1 | command | 1 | missing behavior |\n" + receipt.EvidenceBinding() + "\n"
	fragment := filepath.Join(t.TempDir(), "rows.md")
	os.WriteFile(fragment, []byte(rows), 0600)
	land := func(repo string, args ...string) error {
		return cmdEvidence(append([]string{repo, "evidence-branch"}, args...), &auditCtx{})
	}
	if err := land("example-org/source", "--root", source, "--brief-path", brief, "--evidence-file", fragment); err == nil {
		t.Fatal("Evidence admitted to deliverable repository")
	}
	if len(api.puts) != 0 || budgetCalls != 0 {
		t.Fatal("refused Evidence wrote or spent budget")
	}
	if err := land("example-org/tracker", "--root", source, "--brief-path", brief, "--evidence-file", fragment); err != nil {
		t.Fatal(err)
	}
	if len(api.puts) != 1 || !strings.Contains(api.puts[0].Content, receipt.EvidenceBinding()) {
		t.Fatal("Evidence landing lost binding")
	}
	evidenceSHA := crossLandGit(t, remote, "rev-parse", "HEAD")
	// Refresh only Evidence bytes; neither immutable source HEAD moves.
	os.WriteFile(filepath.Join(br, brief), []byte(api.files[brief]), 0600)
	raw := withRunFields(t, fmt.Sprintf(`{"ts":"2026-10-12T00:00:00Z","brief":"sample/01","outcome":"verify-fail","rows_passed":0,"rows_total":1,"sha":%q,"blocker_kind":"implementation","blocker_ref":"#7"}`, evidenceSHA))
	record := filepath.Join(t.TempDir(), "record.json")
	os.WriteFile(record, []byte(raw), 0600)
	if escape {
		if err := land("example-org/tracker", "--root", source, "--outcome-record", record); deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "../other/") {
			t.Fatalf("unbound sibling admitted: %v", err)
		}
		if len(api.puts) != 1 {
			t.Fatal("unbound sibling wrote an outcome")
		}
		return
	}
	if err := land("example-org/tracker", "--root", source, "--outcome-record", record); err != nil {
		t.Fatal(err)
	}
	if len(api.puts) != 2 {
		t.Fatalf("outcome writes %d", len(api.puts))
	}
	var wr deskkit.WakeReceipt
	if err := json.Unmarshal([]byte(api.puts[1].Content), &wr); err != nil {
		t.Fatal(err)
	}
	if wr.Inputs[deskkit.InputKeyRepoSourcePrefix+"example-org/source"] != receipt.Binding.Source || wr.SHA != evidenceSHA {
		t.Fatal("outcome lost observed source or Evidence commit")
	}
	if withOther && wr.Inputs[deskkit.InputKeyRepoSourcePrefix+"example-org/other"] != crossLandGit(t, other, "rev-parse", "HEAD") {
		t.Fatalf("second sibling not pinned to its own source: %v", wr.Inputs)
	}
	if wr.Inputs["file:"+brief] != deskkit.Sha256Hex([]byte(api.files[brief])) {
		t.Fatal("wrong tracking brief input")
	}
	for k := range wr.Inputs {
		if strings.Contains(k, "..") {
			t.Fatalf("sibling path became a tracking input %q", k)
		}
	}
	mutations := []string{"missing source", "wrong source", "wrong repo"}
	if withOther {
		mutations = append(mutations, "missing sibling", "wrong sibling")
	}
	for _, mutate := range mutations {
		bad := wr
		bad.Inputs = map[string]string{}
		for k, v := range wr.Inputs {
			bad.Inputs[k] = v
		}
		switch mutate {
		case "missing source":
			delete(bad.Inputs, deskkit.InputKeyRepoSourcePrefix+"example-org/source")
		case "wrong source":
			bad.Inputs[deskkit.InputKeyRepoSourcePrefix+"example-org/source"] = strings.Repeat("f", 40)
		case "wrong repo":
			delete(bad.Inputs, deskkit.InputKeyRepoSourcePrefix+"example-org/source")
			bad.Inputs[deskkit.InputKeyRepoSourcePrefix+"example-org/tracker"] = receipt.Binding.Source
		case "missing sibling":
			delete(bad.Inputs, deskkit.InputKeyRepoSourcePrefix+"example-org/other")
		case "wrong sibling":
			bad.Inputs[deskkit.InputKeyRepoSourcePrefix+"example-org/other"] = strings.Repeat("f", 40)
		}
		fg, fr, err := forgeFor("example-org", "tracker")
		if err != nil {
			t.Fatal(err)
		}
		if err := validateOutcomeObservationBinding(br, fg, fr, "evidence-branch", receipt, bad); err == nil {
			t.Fatalf("forged receipt accepted: %s", mutate)
		}
	}
	// Identical retry retains the original record and consumes no new budget.
	beforeBudget := budgetCalls
	if err := land("example-org/tracker", "--root", source, "--outcome-record", record); err != nil {
		t.Fatal(err)
	}
	if len(api.puts) != 2 || budgetCalls != beforeBudget {
		t.Fatal("retry rewrote or rebudgeted outcome")
	}
	// Explicit recheck keeps its own predicate while obeying the same admitted
	// Evidence and write-budget boundary.
	explicit := wr
	explicit.TS = "2026-10-12T00:01:00Z"
	explicit.WakePredicate, explicit.RecheckReason = deskkit.WakeExplicitRecheck, "operator requested a repeat"
	explicitRaw, _ := json.Marshal(explicit)
	os.WriteFile(record, []byte(withRunFields(t, string(explicitRaw))), 0600)
	evidenceWriteBudget = func(string, string, int) error { return deskkit.Refused("fixture budget exhausted") }
	if err := land("example-org/tracker", "--root", source, "--outcome-record", record); err == nil || !strings.Contains(err.Error(), "fixture budget exhausted") {
		t.Fatalf("write-budget refusal lost: %v", err)
	}
	if len(api.puts) != 2 {
		t.Fatal("budget refusal wrote outcome")
	}
	evidenceWriteBudget = func(string, string, int) error { budgetCalls++; return nil }
	if err := land("example-org/tracker", "--root", source, "--outcome-record", record); err != nil {
		t.Fatal(err)
	}
	if len(api.puts) != 3 || !strings.Contains(api.puts[2].Content, deskkit.WakeExplicitRecheck) {
		t.Fatal("explicit recheck landing failed")
	}
	os.WriteFile(filepath.Join(source, "source.txt"), []byte("unobserved change\n"), 0600)
	if err := land("example-org/tracker", "--root", source, "--outcome-record", record); err == nil {
		t.Fatal("changed deliverable admitted even on retry")
	}
	if len(api.puts) != 3 {
		t.Fatal("changed deliverable wrote outcome")
	}
	os.WriteFile(filepath.Join(source, "source.txt"), []byte("observed\n"), 0600)
	if len(api.unexpected) != 0 {
		t.Fatalf("unexpected forge reads %v", api.unexpected)
	}
	if crossLandGit(t, source, "status", "--porcelain") != "" {
		t.Fatal("deliverable modified")
	}
	if crossLandGit(t, br, "rev-parse", "HEAD") != receipt.Binding.TrackingSource {
		t.Fatal("tracking admission moved")
	}
}

// regression: #2485
func TestCrossRepoDeliveryJoin(t *testing.T) {
	receipt := deskkit.VerifierReceipt{Binding: deskkit.VerifierBinding{Version: 2, SourceRepo: jOtherRepo, Source: jSHA}}
	facts := joinFacts{Entries: []joinEntry{{Repo: jOtherRepo, MergeSHA: jMerge}}}
	for _, status := range []string{"identical", "ahead", "behind", "diverged"} {
		f := &fakeForge{compareStatus: map[string]string{jOtherRepo + ":" + jMerge + "..." + jSHA: status}}
		err := checkAdmittedDeliveryPRs(f, receipt, facts)
		if (err == nil) != (status == "identical" || status == "ahead") {
			t.Fatalf("%s: %v", status, err)
		}
		if len(f.joinReads) != 1 || !strings.Contains(f.joinReads[0], "..."+jSHA) {
			t.Fatal("delivery join did not compare admitted source")
		}
	}
	f := &fakeForge{compareErr: fmt.Errorf("unavailable")}
	if err := checkAdmittedDeliveryPRs(f, receipt, facts); err == nil {
		t.Fatal("unreadable delivery history accepted")
	}
	receipt.Binding.Version = 1
	f = &fakeForge{}
	if err := checkAdmittedDeliveryPRs(f, receipt, facts); err != nil || len(f.joinReads) != 0 {
		t.Fatal("same-repo join changed")
	}
}

// A bound sibling's delivery PR answers to that sibling's own source, never
// to the primary source commit.
// regression: #4876
func TestSiblingDeliveryJoin(t *testing.T) {
	for _, line := range strings.Split(fixtureRoster, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && strings.HasPrefix(k, "ASSAY_") {
			t.Setenv(k, v)
		}
	}
	t.Setenv(deskkit.EnvAllowedRepos, "example-org/tracker:ci:private,"+jOtherRepo+":ci:public,example-org/third:ci:public")
	deskkit.SetToolClass(deskkit.ClassCI)
	t.Cleanup(func() { deskkit.SetToolClass(deskkit.ClassWrite) })
	const third, thirdSHA = "example-org/third", "fedcba9876543210fedcba9876543210fedcba98"
	receipt := deskkit.VerifierReceipt{Binding: deskkit.VerifierBinding{Version: 2, SourceRepo: jOtherRepo, Source: jSHA,
		SiblingSources: third + "@" + thirdSHA}}
	facts := joinFacts{Entries: []joinEntry{{Repo: third, MergeSHA: jMerge}}}
	for _, status := range []string{"identical", "behind"} {
		f := &fakeForge{compareStatus: map[string]string{third + ":" + jMerge + "..." + thirdSHA: status, third + ":" + jMerge + "..." + jSHA: "ahead"}}
		err := checkAdmittedDeliveryPRs(f, receipt, facts)
		if (err == nil) != (status == "identical") {
			t.Fatalf("%s: %v", status, err)
		}
		// The refusal names the sibling, its merge and its bound source.
		if err != nil && !strings.Contains(err.Error(), third+" merge "+jMerge+" is not contained in "+third+"@"+thirdSHA) {
			t.Fatalf("refusal does not name the stale sibling: %v", err)
		}
		if len(f.joinReads) != 1 || !strings.Contains(f.joinReads[0], "..."+thirdSHA) {
			t.Fatalf("sibling delivery not compared with its own source: %v", f.joinReads)
		}
	}
}

// regression: #2485
func TestCrossRepoOutcomeGitEnv(t *testing.T) {
	tracking := crossLandRepo(t, "example-org/tracker", map[string]string{"brief.md": "tracked plan\n"})
	sha := crossLandGit(t, tracking, "rev-parse", "HEAD")
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "unrelated.git"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	tree, err := newGitOutcomeTree(tracking, sha)
	if err != nil {
		t.Fatal(err)
	}
	body, err := tree.ReadFile("brief.md")
	if err != nil || string(body) != "tracked plan\n" {
		t.Fatalf("history redirected: %q %v", body, err)
	}
	kind, err := tree.PathKind("brief.md")
	if err != nil || kind != "blob" {
		t.Fatalf("kind redirected: %s %v", kind, err)
	}
}
