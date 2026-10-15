package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func crossDispatchConfig(t *testing.T, visibility string) {
	t.Helper()
	for _, line := range strings.Split(fixtureRoster, "\n") {
		if key, value, ok := strings.Cut(line, "="); ok && strings.HasPrefix(key, "ASSAY_") {
			t.Setenv(key, value)
		}
	}
	t.Setenv(deskkit.EnvAllowedRepos, "example-org/source:ci:"+visibility+",example-org/tracking:ci:private")
	t.Setenv(deskkit.EnvRepoForges, "example-org/source=github,example-org/tracking=github")
	// Existing CI-class transport, only inside this test process. No HOME or
	// machine trust file is replaced; production main remains ClassWrite.
	deskkit.SetToolClass(deskkit.ClassCI)
	t.Cleanup(func() { deskkit.SetToolClass(deskkit.ClassWrite) })
}
func crossDispatchGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func crossDispatchRepo(t *testing.T, repo string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	crossDispatchGit(t, root, "init")
	crossDispatchGit(t, root, "config", "user.name", "fixture")
	crossDispatchGit(t, root, "config", "user.email", "fixture@example.invalid")
	crossDispatchGit(t, root, "remote", "add", "origin", "https://github.com/"+repo+".git")
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		crossDispatchGit(t, root, "add", name)
	}
	crossDispatchGit(t, root, "commit", "-m", "fixture")
	crossDispatchGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	crossDispatchGit(t, root, "checkout", "--detach")
	return root
}

// TestCrossRepoChecker is the actual shared admission reader in a separate
// process. It replaces only local roster transport and forge custody; the
// statusgen executor receives no caller-invented JSON receipt.
func TestCrossRepoChecker(t *testing.T) {
	if os.Getenv("ASSAY_CROSS_CHECKER") != "1" {
		return
	}
	deskkit.SetToolClass(deskkit.ClassCI)
	deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) {
		return ghStampToken, os.Getenv("ASSAY_CROSS_API"), nil
	})
	r, err := deskkit.CheckVerifierAttestation(os.Getenv("ASSAY_CROSS_ROOT"), os.Getenv("ASSAY_CROSS_BRIEF"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		os.Exit(6)
	}
	os.Exit(0)
}

// regression: #2485
func TestCrossRepoDispatchExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX row executor; core association checks are portable")
	}
	bin := filepath.Join(t.TempDir(), "statusgen")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "../../../../statusgen"
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build statusgen: %v %s", err, out)
	}
	for _, visibility := range []string{"public", "private"} {
		t.Run(visibility, func(t *testing.T) {
			crossDispatchConfig(t, visibility)
			source := crossDispatchRepo(t, "example-org/source", map[string]string{"source.txt": "deliverable\n"})
			output := t.TempDir()
			t.Setenv("VERIFY_OUTPUT", output)
			const brief = "docs/streams/sample/brief-01-check.md"
			body := "---\ndeliverable_repo: source\n---\n# Restricted fixture canary\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `test -f source.txt && touch \"$VERIFY_OUTPUT/row\"` | exit 0 |\n\n## Evidence\n\nPrior-evidence-canary.\n"
			tracking := crossDispatchRepo(t, "example-org/tracking", map[string]string{brief: body, "docs/streams/graph-repos.yaml": "schema: graph-repos-v1\nself: track\nrepos:\n  track:\n    repo: example-org/tracking\n  source:\n    repo: example-org/source\n"})
			gh := installGHStamp(t)
			verifierGHFixture(t, gh)
			o := dispatchOpts{root: source, claimRoot: tracking, repo: "example-org/source", brief: filepath.Join(tracking, brief), model: "gpt-6-astra", tier: "strong", kit: "verifier", item: "sample/01", promptFile: filepath.Join(t.TempDir(), "prompt.md")}
			// Exercise the full dispatch pipeline. Only allocation/claim child
			// commands and the local append sink are fixtures; association,
			// attestation, stamp readback, prompt and packet building are real.
			isolateClaimTool(t, t.TempDir())
			buildPacketFn = buildDispatchPacket
			plantScripts(t, tracking)
			t.Setenv("DESK_LOOP", "verify-desk")
			t.Setenv("DESK_SESSION", "cross-repo-fixture")
			oldExec := execCommand
			var childCalls []string
			execCommand = func(name string, args ...string) *exec.Cmd {
				joined := name + " " + strings.Join(args, " ")
				childCalls = append(childCalls, joined)
				if name == "git" {
					return exec.Command(name, args...)
				}
				if strings.Contains(joined, "deskwt add") {
					return exec.Command("/bin/sh", "-c", "printf '%s\n' "+"'"+strings.ReplaceAll(source, "'", "'\\''")+"'")
				}
				if strings.Contains(joined, "dispatch-claim.sh") {
					return exec.Command("/bin/sh", "-c", "exit 0")
				}
				t.Errorf("unexpected dispatch child: %s", joined)
				return exec.Command("/bin/sh", "-c", "exit 99")
			}
			t.Cleanup(func() { execCommand = oldExec })
			oldAppend := appendDispatchRecord
			var records []deskkit.DispatchRecord
			appendDispatchRecord = func(r *deskkit.DispatchRecord) error { records = append(records, *r); return nil }
			t.Cleanup(func() { appendDispatchRecord = oldAppend })
			o.run = &dispatchRun{}
			if err := dispatch(o); err != nil {
				t.Fatalf("cross-repository dispatch: %v", err)
			}
			if len(records) != 1 || len(childCalls) == 0 {
				t.Fatal("dispatch did not acquire and record its run")
			}
			prompt, err := os.ReadFile(emittedPathFor(o.promptFile))
			if err != nil || !strings.Contains(string(prompt), "Packet:") || strings.Contains(string(prompt), "stamp PENDING") {
				t.Fatalf("dispatch did not emit admitted packet prompt: %v", err)
			}
			br, rel, rev, err := deskkit.VerifierBriefLocation(source)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(filepath.Dir(br)) })
			o.brief, o.briefShown = filepath.Join(br, rel), filepath.Join(br, rel)
			plan := dispatchPlan{claimKey: "sample--01", verifierBriefRoot: br, verifierBriefRevision: rev}
			packetPath := buildDispatchPacket(o, plan, o.repo, source)
			if packetPath == "" {
				t.Fatal("no cross-repository packet")
			}
			packet, err := os.ReadFile(packetPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{crossDispatchGit(t, source, "rev-parse", "HEAD"), rev, "Restricted fixture canary"} {
				if !strings.Contains(string(packet), want) {
					t.Fatalf("packet missing %q", want)
				}
			}
			if strings.Contains(string(packet), "Prior-evidence-canary") {
				t.Fatal("packet copied earlier Evidence")
			}
			receipt, err := deskkit.CheckVerifierAttestation(source, brief)
			if err != nil {
				t.Fatal(err)
			}
			if receipt.Binding.Repo != "example-org/tracking" || receipt.Binding.SourceRepo != "example-org/source" {
				t.Fatal("wrong repository binding")
			}
			for _, call := range gh.minted {
				if call.repo != "example-org/tracking" {
					t.Fatalf("attestation custody reached deliverable: %+v", call)
				}
			}
			helperDir := t.TempDir()
			helper := filepath.Join(helperDir, "deskdispatch")
			script := "#!/bin/sh\nexec '" + strings.ReplaceAll(os.Args[0], "'", "'\\''") + "' -test.run='^TestCrossRepoChecker$'\n"
			if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", helperDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("ASSAY_CROSS_CHECKER", "1")
			t.Setenv("ASSAY_CROSS_API", gh.srv.URL)
			t.Setenv("ASSAY_CROSS_ROOT", source)
			t.Setenv("ASSAY_CROSS_BRIEF", brief)
			t.Setenv("DESK_LOOP", "verify-desk")
			execute := func() ([]byte, error) {
				c := exec.Command(bin, "verifyrun", "--root", source, "--brief", brief)
				c.Dir = source
				return c.CombinedOutput()
			}
			gh.mu.Lock()
			labels := gh.labels
			gh.labels = nil
			gh.mu.Unlock()
			if out, err := execute(); err == nil {
				t.Fatalf("missing stamp ran rows: %s", out)
			}
			if _, err := os.Stat(filepath.Join(output, "row")); !os.IsNotExist(err) {
				t.Fatal("unadmitted row executed")
			}
			gh.mu.Lock()
			gh.labels = labels
			gh.mu.Unlock()
			if out, err := execute(); err != nil {
				t.Fatalf("admitted Verify: %v %s", err, out)
			}
			if _, err := os.Stat(filepath.Join(output, "row")); err != nil {
				t.Fatal("admitted row did not execute in deliverable home")
			}
			landed, err := os.ReadFile(filepath.Join(br, brief))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(landed), receipt.EvidenceBinding()) {
				t.Fatal("Evidence omitted two-source binding")
			}
			if _, err := deskkit.CheckVerifierEvidence(source, "example-org/tracking", brief); err != nil {
				t.Fatal(err)
			}
			if _, err := deskkit.CheckVerifierEvidence(source, "example-org/source", brief); err == nil {
				t.Fatal("private Evidence admitted to deliverable")
			}
			if _, err := os.Stat(filepath.Join(source, brief)); !os.IsNotExist(err) {
				t.Fatal("tracking brief copied into deliverable")
			}
			original, _ := os.ReadFile(filepath.Join(tracking, brief))
			if string(original) != body {
				t.Fatal("shared tracking checkout edited")
			}
			if diff := crossDispatchGit(t, source, "status", "--porcelain"); diff != "" {
				t.Fatalf("deliverable changed: %s", diff)
			}
		})
	}
}

// regression: #2485
func TestCrossRepoPromptDestination(t *testing.T) {
	root := crossDispatchRepo(t, "example-org/source", map[string]string{"source.txt": "source\n"})
	outside := t.TempDir()
	link := filepath.Join(outside, "linked.md")
	if err := os.Symlink(filepath.Join(root, "source.txt"), link); err != nil {
		t.Fatal(err)
	}
	directoryLink := filepath.Join(outside, "tree")
	if err := os.Symlink(root, directoryLink); err != nil {
		t.Fatal(err)
	}
	for _, dest := range []string{filepath.Join(root, "prompt.md"), link, filepath.Join(directoryLink, "prompt.md")} {
		if err := verifierPrivatePromptDestination(dest); err == nil {
			t.Fatalf("private prompt allowed into repository: %s", dest)
		}
	}
	if err := verifierPrivatePromptDestination(filepath.Join(outside, "prompt.md")); err != nil {
		t.Fatal(err)
	}
	// --prompt-file is never written; the emitted sibling is, so that is what is checked.
	caller := filepath.Join(outside, "task.md")
	if err := os.Symlink(filepath.Join(root, "source.txt"), emittedPathFor(caller)); err != nil {
		t.Fatal(err)
	}
	if err := verifierPrivatePromptDestination(verifierEmittedDest(caller)); err == nil {
		t.Fatal("emitted prompt allowed through a link into repository")
	}
	hardlink := filepath.Join(outside, "hardlinked.md")
	if err := os.Link(filepath.Join(root, "source.txt"), hardlink); err != nil {
		t.Fatal(err)
	}
	if err := writeVerifierPrompt(hardlink, []byte("private context\n")); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "source.txt")); err != nil || string(b) != "source\n" {
		t.Fatal("prompt followed hardlink into repository")
	}
	if info, err := os.Stat(hardlink); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private prompt permissions are not owner-only")
	}
	if got := crossDispatchGit(t, root, "status", "--porcelain"); got != "" {
		t.Fatal("destination checks wrote repository")
	}
}
