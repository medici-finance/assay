package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6274. One dispatch, one admission, one real Verify run. Both
// publication commands consume that run's evidence against a stateful local forge.
func TestDispatchPublicationHandoff(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX verifier row and local test process")
	}
	statusgen := buildStatusgen(t)
	bin := filepath.Dir(statusgen)
	dispatch := filepath.Join(bin, "dispatch.test")
	build := exec.Command("go", "test", "-c", "-o", dispatch, ".")
	build.Dir = "../deskdispatch"
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("build dispatcher: %v %s", err, out)
	}
	checker := "#!/bin/sh\nexec \"$HANDOFF_DISPATCH_TEST\" -test.run='^TestCrossRepoChecker$'\n"
	if err := os.WriteFile(filepath.Join(bin, "deskdispatch"), []byte(checker), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HANDOFF_DISPATCH_TEST", dispatch)
	for _, layout := range []string{"split", "same"} {
		t.Run(layout, func(t *testing.T) { qualifyPublication(t, bin, dispatch, layout == "split") })
	}
}

func qualifyPublication(t *testing.T, bin, dispatch string, split bool) {
	const tracker, brief = "example-org/tracker", vgBrief
	for _, line := range strings.Split(fixtureRoster, "\n") {
		if key, val, ok := strings.Cut(line, "="); ok && strings.HasPrefix(key, "ASSAY_") {
			t.Setenv(key, val)
		}
	}
	t.Setenv(deskkit.EnvAllowedRepos, tracker+":ci:private,example-org/source:ci:private")
	t.Setenv(deskkit.EnvRepoForges, tracker+"=github,example-org/source=github")
	t.Setenv("DESK_LOOP", "verify-desk")
	t.Setenv("DESK_SESSION", "publication-fixture")
	t.Setenv("DESK_HUMAN", "alex")
	t.Setenv("VERIFIER_MAIN_OK", "1")
	deskkit.SetToolClass(deskkit.ClassCI)
	t.Cleanup(func() { deskkit.SetToolClass(deskkit.ClassWrite) })
	delivery, front := tracker, ""
	if split {
		delivery, front = "example-org/source", "deliverable_repo: source\n"
	}
	body := strings.Replace(vgBriefText(""), "---\n", "---\n"+front, 1)
	body = strings.Replace(body, "| 1 | `true` | exit 0 |", "| 1 | `test -f delivery-marker.txt` | exit 0 |\n| 2 | `test -f required-change.txt` | exit 0 |", 1)
	body = strings.Replace(body, "## Verify", "## Context\nfiles:\n- `source.txt`\n\n## Verify", 1)
	files := map[string]string{brief: body, "docs/streams/vg/README.md": vgReadme("implemented"), "docs/streams/graph-repos.yaml": "schema: graph-repos-v1\nself: track\nrepos:\n  track:\n    repo: " + tracker + "\n  source:\n    repo: example-org/source\n", "source.txt": "tracking fixture\n"}
	if !split {
		files["delivery-marker.txt"] = "execution home\n"
	}
	tracking := crossLandRepo(t, tracker, files)
	crossLandGit(t, tracking, "update-ref", "refs/fixture-decisions/4", "HEAD")
	source := tracking
	if split {
		source = crossLandRepo(t, delivery, map[string]string{brief: "# Different brief, same path\n", "source.txt": "delivery fixture\n", "delivery-marker.txt": "execution home\n"})
	}
	sourceHead := crossLandGit(t, source, "rev-parse", "HEAD")
	trackingHead := crossLandGit(t, tracking, "rev-parse", "HEAD")
	api := &attestationAPI{files: map[string]string{brief: body}, head: trackingHead}
	var prReads []string
	changedFiles := []map[string]string{{"filename": brief, "status": "modified"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		enc := json.NewEncoder(w)
		if r.Method == "GET" {
			switch {
			case path == "/repos/"+tracker+"/commits/main":
				_ = enc.Encode(map[string]any{"sha": api.head, "files": []any{}})
				return
			case path == "/repos/"+tracker || path == "/repos/"+delivery:
				_ = enc.Encode(map[string]any{"private": true, "visibility": "private", "default_branch": "main"})
				return
			case path == "/repos/"+tracker+"/issues/7":
				_ = enc.Encode(map[string]any{"number": 7, "state": "open", "title": "missing change"})
				return
			case strings.HasSuffix(path, "/pulls/9"):
				prReads = append(prReads, path)
				sha := sourceHead
				if path != "/repos/"+delivery+"/pulls/9" {
					sha = trackingHead
				}
				_ = enc.Encode(map[string]any{"number": 9, "html_url": prURL(strings.TrimSuffix(strings.TrimPrefix(path, "/repos/"), "/pulls/9"), 9), "state": "closed", "merged": true, "merged_at": "2026-10-01T00:00:00Z", "merge_commit_sha": sha, "base": map[string]any{"ref": "main"}})
				return
			case strings.Contains(path, "/compare/"):
				base, head, _ := strings.Cut(strings.Split(path, "/compare/")[1], "...")
				if path == "/repos/"+delivery+"/compare/"+sourceHead+"...main" && split || base == head {
					_ = enc.Encode(map[string]any{"status": "identical", "total_commits": 0, "commits": []any{}, "files": []any{}})
					return
				}
				if base != trackingHead && base != sourceHead || head != api.head {
					t.Errorf("unknown fixture comparison %s", path)
					http.NotFound(w, r)
					return
				}
				_ = enc.Encode(map[string]any{"status": "ahead", "ahead_by": 1, "total_commits": 1, "commits": []any{map[string]any{"sha": api.head}}, "files": changedFiles})
				return
			}
		}
		if strings.HasPrefix(path, "/repos/") && !strings.HasPrefix(path, "/repos/"+tracker+"/") {
			t.Errorf("unexpected %s on %s", r.Method, path)
			http.NotFound(w, r)
			return
		}
		api.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) { return "fixture-token", srv.URL, nil })
	t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(githubCustodyMint) })
	promptDir := filepath.Join(t.TempDir(), "operator's review desk")
	if err := os.Mkdir(promptDir, 0700); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(promptDir, "prompt.md")
	call := exec.Command(dispatch, "-test.run=^TestPublicationHandoffDispatcher$", "--", srv.URL, source, tracking, delivery, brief, prompt)
	if out, err := call.CombinedOutput(); err != nil {
		t.Fatalf("dispatch: %v %s", err, out)
	}
	if got := crossLandGit(t, source, "for-each-ref", "--format=%(objectname)", "refs/fixture-claims/"); got != sourceHead {
		t.Fatalf("claim points to %q; want %s", got, sourceHead)
	}
	if split && crossLandGit(t, tracking, "for-each-ref", "refs/fixture-claims/") != "" {
		t.Fatal("delivery claim created in tracking repository")
	}
	command, err := os.ReadFile(prompt + ".command")
	if err != nil {
		t.Fatal(err)
	}
	// Capture the literal arguments a verifier's shell would pass to deskevidence.
	// The function stands in for the writer until the run has produced its rows.
	capture := exec.Command("sh", "-c", "deskevidence() { printf '%s\\000' deskevidence \"$@\"; }\n"+string(command))
	fields, err := capture.Output()
	if err != nil {
		t.Fatalf("cannot read the emitted outcome arguments: %v", err)
	}
	args := strings.Split(strings.TrimSuffix(string(fields), "\x00"), "\x00")
	if len(args) != 8 || args[0] != "deskevidence" || args[2] != "main" || args[3] != "--outcome-record" || args[5] != "--root" || args[6] != source || args[7] != "--acceptance-forge" {
		t.Fatalf("unexpected emitted command: %s", command)
	}
	receipt, err := deskkit.RecoverVerifierAttestation(source)
	if err != nil {
		t.Fatal(err)
	}
	briefRoot := source
	if split {
		briefRoot = receipt.BriefRoot
		t.Cleanup(func() { os.RemoveAll(filepath.Dir(briefRoot)) })
		if receipt.Binding.SourceRepo != delivery || receipt.Binding.TrackingSource != trackingHead {
			t.Fatalf("lost tracking/delivery binding: %+v", receipt.Binding)
		}
	}
	if receipt.Binding.Source != sourceHead || receipt.Binding.Repo != tracker {
		t.Fatalf("wrong admission: %+v", receipt.Binding)
	}
	t.Setenv("ASSAY_CROSS_CHECKER", "1")
	t.Setenv("ASSAY_CROSS_API", srv.URL)
	t.Setenv("ASSAY_CROSS_ROOT", source)
	t.Setenv("ASSAY_CROSS_BRIEF", brief)
	verify := exec.Command(filepath.Join(bin, "statusgen"), "verifyrun", "--brief", brief)
	verify.Dir = source
	verifyOut, verifyErr := verify.CombinedOutput()
	observed, err := os.ReadFile(filepath.Join(briefRoot, brief))
	if err != nil || !strings.Contains(string(observed), receipt.EvidenceBinding()) {
		t.Fatalf("no bound Verify evidence: %v %s %v\n%s", verifyErr, verifyOut, err, observed)
	}
	_, rows, ok := strings.Cut(string(observed), "## Evidence\n")
	rows, _, _ = strings.Cut(rows, "\n## Review\n")
	if !ok || !strings.Contains(rows, "pass") || !strings.Contains(rows, "fail") {
		t.Fatalf("row witness: %s", observed)
	}
	fragment := filepath.Join(t.TempDir(), "rows.md")
	writeFixtureFile(t, fragment, rows)
	remote := filepath.Join(t.TempDir(), "remote")
	crossLandGit(t, tracking, "clone", "--no-hardlinks", tracking, remote)
	crossLandGit(t, remote, "config", "user.name", "assay-verifier-app[bot]")
	crossLandGit(t, remote, "config", "user.email", verifierBotEmail)
	api.commit = func(file, content string) string {
		writeFixtureFile(t, filepath.Join(remote, file), content)
		crossLandGit(t, remote, "add", file)
		crossLandGit(t, remote, "commit", "-m", "record verification")
		sha := crossLandGit(t, remote, "rev-parse", "HEAD")
		crossLandGit(t, briefRoot, "fetch", remote, "HEAD")
		api.head = sha
		return sha
	}
	oldMint := mintTokenFn
	t.Cleanup(func() { mintTokenFn = oldMint; ghToken = "" })
	mintTokenFn = func(string) (deskkit.RoleCredential, error) {
		ghToken = "fixture-token"
		return deskkit.RoleCredential{Token: ghToken, Resolution: deskkit.ForgeResolution{Kind: deskkit.ForgeGitHub}}, nil
	}
	oldBudget := evidenceWriteBudget
	t.Cleanup(func() { evidenceWriteBudget = oldBudget })
	evidenceWriteBudget = func(_ string, repo string, _ int) error {
		if repo != tracker {
			t.Fatalf("write budget for %s", repo)
		}
		return nil
	}
	var messages bytes.Buffer
	oldStderr := stderr
	stderr = &messages
	t.Cleanup(func() { stderr = oldStderr })
	land := func(argv ...string) int { messages.Reset(); return run(argv) }
	code := land(tracker, "main", "--root", source, "--brief-path", brief, "--evidence-file", fragment)
	digests := readSetDigests([]byte(rows))
	if len(digests) == 0 {
		if code != 0 || len(api.puts) != 1 || api.puts[0].File != brief {
			t.Fatalf("single-file Evidence: exit %d, writes %v: %s", code, api.puts, &messages)
		}
	} else {
		// The Contents API cannot introduce a row and its read-set blobs in
		// one commit. The fixture represents the reviewed Evidence PR as a
		// local branch, merged into the same tracking history the writer reads.
		if code != deskkit.ExitRefused || !strings.Contains(messages.String(), "read-set") || len(api.puts) != 0 {
			t.Fatalf("unpaired Evidence: exit %d, writes %v: %s", code, api.puts, &messages)
		}
		admitted, err := deskkit.CheckVerifierEvidence(source, tracker, brief)
		if err != nil || admitted.EvidenceBinding() != receipt.EvidenceBinding() {
			t.Fatalf("paired Evidence did not retain the admitted run: %v", err)
		}
		if err := admitted.CheckEvidenceContent(brief, observed, nil); err != nil {
			t.Fatal(err)
		}
		changedFiles = append(changedFiles, mergeHandoffEvidencePR(t, api, remote, source, briefRoot, brief, observed, digests)...)
	}
	if !strings.Contains(api.files[brief], receipt.EvidenceBinding()) {
		t.Fatal("tracking history lost the dispatched Evidence binding")
	}
	t.Logf("tracking Evidence landed with %d recorder files", len(digests))
	beforeOutcome := len(api.puts)
	sha := crossLandGit(t, remote, "rev-parse", "HEAD")
	writeFixtureFile(t, filepath.Join(briefRoot, brief), api.files[brief])
	record := map[string]any{
		"ts": "2000-01-01T00:02:00Z", "started": "2000-01-01T00:00:00Z", "finished": "2000-01-01T00:01:00Z",
		"brief": "vg/01", "outcome": "verify-fail", "verifier_tier": "strong", "rows_passed": 1, "rows_total": 2,
		"sha": sha, "blocker_kind": "implementation", "blocker_ref": "#7", "prs": []any{jpr(delivery, 9, sourceHead)},
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	args[4] = strings.Replace(args[4], "<file>", "handoff", 1)
	writeFixtureFile(t, args[4], string(raw))
	if n := land(args[1:]...); n != 0 {
		t.Fatalf("emitted outcome to %s exited %d: %s\nunexpected=%v", args[1], n, &messages, api.unexpected)
	}
	if len(api.puts) != beforeOutcome+1 || api.branch != "main" || len(prReads) == 0 {
		t.Fatalf("missing publication or delivery read: writes=%v branch=%s reads=%v", api.puts, api.branch, prReads)
	}
	for _, read := range prReads {
		if read != "/repos/"+delivery+"/pulls/9" {
			t.Fatalf("delivery PR read from %s", read)
		}
	}
	if args[1] != tracker || !strings.HasPrefix(api.puts[beforeOutcome].File, "docs/streams/verify-outcomes/vg/01-") {
		t.Fatalf("writes=%+v branch=%s PR reads=%v", api.puts, api.branch, prReads)
	}
	var landed deskkit.WakeReceipt
	if err := json.Unmarshal([]byte(api.puts[beforeOutcome].Content), &landed); err != nil || landed.SHA != sha {
		t.Fatalf("outcome lost Evidence SHA: %v %+v", err, landed)
	}
	sourceKey := deskkit.InputKeyRepoSourcePrefix + delivery
	if split && landed.Inputs[sourceKey] != sourceHead {
		t.Fatalf("outcome lost delivery revision: %+v", landed.Inputs)
	}
	missing := append([]string(nil), args[1:]...)
	missing[5] = filepath.Join(t.TempDir(), "missing")
	if n := land(missing...); n != deskkit.ExitRefused || !strings.Contains(messages.String(), "no pre-work verifier attestation") {
		t.Fatalf("missing root: got %d %s", n, &messages)
	}
	crossLandGit(t, source, "commit", "--allow-empty", "-m", "different revision")
	if n := land(args[1:]...); n != deskkit.ExitRefused || !strings.Contains(messages.String(), "attested detached source commit") {
		t.Fatalf("moved revision: got %d %s", n, &messages)
	}
	if len(api.puts) != beforeOutcome+1 {
		t.Fatalf("refused publication wrote: %+v", api.puts)
	}
}

// mergeHandoffEvidencePR publishes only the verified brief and the recorder
// files its actual witnesses name. Read sets live in the execution worktree's
// Git directory until a single Evidence commit introduces them beside the brief.
func mergeHandoffEvidencePR(t *testing.T, api *attestationAPI, remote, source, briefRoot, brief string, observed []byte, digests []string) []map[string]string {
	t.Helper()
	base := crossLandGit(t, remote, "rev-parse", "HEAD")
	prior, err := gitShowAt(remote, base, brief)
	if err != nil || len(readSetDigests(prior)) != 0 {
		t.Fatalf("parent of paired Evidence already names a read set or is unreadable: %v", err)
	}
	state := crossLandGit(t, source, "rev-parse", "--absolute-git-dir")
	crossLandGit(t, remote, "checkout", "-b", "fixture-evidence")
	writeFixtureFile(t, filepath.Join(remote, brief), string(observed))
	crossLandGit(t, remote, "add", "--", brief)
	blobs := map[string]string{brief: string(observed)}
	var additions []map[string]string
	for _, d := range digests {
		data, err := os.ReadFile(filepath.Join(state, evaluation.ReadSetStateDir, d))
		if err != nil || evaluation.ReadSetDigest(data) != d {
			t.Fatalf("recorder bytes for %s: %v", d, err)
		}
		path := evaluation.ReadSetPath(filepath.ToSlash(filepath.Dir(brief)), d)
		writeFixtureFile(t, filepath.Join(remote, filepath.FromSlash(path)), string(data))
		crossLandGit(t, remote, "add", "--", path)
		blobs[path] = string(data)
		additions = append(additions, map[string]string{"filename": path, "status": "added"})
	}
	crossLandGit(t, remote, "commit", "-m", "Record verifier Evidence and its read sets")
	sha := crossLandGit(t, remote, "rev-parse", "HEAD")
	crossLandGit(t, remote, "checkout", "--detach", base)
	crossLandGit(t, remote, "merge", "--ff-only", "fixture-evidence")
	// Reading the introducing commit back catches an unstaged file or one
	// already present in a parent. The same commit must contain the witness.
	for path, want := range blobs {
		got, err := gitShowAt(remote, sha, path)
		if err != nil || string(got) != want {
			t.Fatalf("merged Evidence blob %s differs: %v", path, err)
		}
	}
	added := strings.Split(crossLandGit(t, remote, "diff", "--diff-filter=A", "--name-only", base, sha), "\n")
	if len(added) != len(digests) {
		t.Fatalf("Evidence commit introduced %v for %d read sets", added, len(digests))
	}
	for _, path := range added {
		if path == brief || blobs[path] == "" {
			t.Fatalf("unexpected path in Evidence commit: %q", path)
		}
	}
	crossLandGit(t, briefRoot, "fetch", remote, sha)
	api.mu.Lock()
	defer api.mu.Unlock()
	for path, data := range blobs {
		api.files[path] = data
	}
	if api.commits == nil {
		api.commits = map[string]bool{}
	}
	api.commits[sha] = true
	api.head = sha
	return additions
}
