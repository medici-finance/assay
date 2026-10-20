package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func landedFixture(t *testing.T, wrong bool) (string, string, string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	workerName, workerEmail, err := deskkit.RoleWorktreeCommitIdentity("worker")
	if err != nil {
		t.Fatal(err)
	}
	verifierName, verifierEmail, err := deskkit.RoleWorktreeCommitIdentity("verifier")
	if err != nil {
		t.Fatal(err)
	}
	flGit(t, root, "init", "-q", "-b", "main")
	flGit(t, root, "config", "commit.gpgsign", "false")
	flGit(t, root, "config", "user.name", workerName)
	flGit(t, root, "config", "user.email", workerEmail)
	files := map[string]string{
		"docs/streams/vf/README.md":         "---\nstream: vf\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# VF\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|---|---|---|---|---|---|\n| 01 | [check](brief-01-check.md) | 0 | S | implemented | — | 2026-09-22 human:alex |\n",
		"docs/streams/vf/brief-01-check.md": "---\nbrief: vf/01\ntitle: Check\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v1\nauthored: 2026-09-22 by assay-worker-app[bot]\nsources: [fixture]\n---\n\n# Check\n\n## Verify\n| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n\n## Evidence\n\n## Review\nGate: model.\n",
	}
	write := func() {
		t.Helper()
		for p, b := range files {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, p), []byte(b), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	write()
	flGit(t, root, "add", "docs")
	flGit(t, root, "commit", "-q", "-m", "implementation")
	source := flGit(t, root, "rev-parse", "HEAD")
	flGit(t, root, "update-ref", "refs/remotes/origin/main", source)
	runner := verifierName + " @ " + source[:12]
	files["docs/streams/vf/README.md"] = strings.Replace(files["docs/streams/vf/README.md"], "| implemented | — |", "| verified | 2026-09-22 "+runner+" |", 1)
	evidence := "\n| # | Command | Result | Output | Date | Runner |\n|---|---|---|---|---|---|\n| 1 | `true` | pass exit=0 | sha256:abcdef012345 | 2026-09-22 | " + runner + " (on-behalf-of human:alex) |\n\n**VERIFY: PASS**\n"
	files["docs/streams/vf/brief-01-check.md"] = strings.Replace(files["docs/streams/vf/brief-01-check.md"], "## Evidence\n", "## Evidence\n"+evidence, 1)
	if !wrong {
		flGit(t, root, "config", "user.name", verifierName)
		flGit(t, root, "config", "user.email", verifierEmail)
	}
	write()
	flGit(t, root, "add", "docs")
	flGit(t, root, "commit", "-q", "-m", "verification")
	landed := flGit(t, root, "rev-parse", "HEAD")
	flGit(t, root, "checkout", "-q", "--detach", source)
	write()
	return root, source, landed, files
}

func landedStatusgenOnPath(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	name := "statusgen"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	build := exec.Command("go", "build", "-o", filepath.Join(bin, name), ".")
	build.Dir = "../../../../statusgen"
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build statusgen: %v %s", err, out)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// regression: #3343
func TestLandedHistory(t *testing.T) {
	landedStatusgenOnPath(t)
	for _, baseline := range []string{"old", "equal", "descended"} {
		for _, wrong := range []bool{false, true} {
			for _, target := range []string{"docs/streams/verify-outcomes/vf/01-test.json", "docs/streams/verify-outcomes.jsonl"} {
				t.Run(baseline+"/"+map[bool]string{false: "verifier", true: "worker"}[wrong]+filepath.Ext(target), func(t *testing.T) {
					f, errbuf := setupFake(t)
					for _, line := range strings.Split(fixtureRoster, "\n") {
						if key, value, ok := strings.Cut(line, "="); ok && strings.HasPrefix(key, "ASSAY_") {
							t.Setenv(key, value)
						}
					}
					root, source, head, files := landedFixture(t, wrong)
					if baseline != "old" {
						mainHead := head
						if baseline == "descended" {
							flGit(t, root, "checkout", "-q", "-f", "--detach", head)
							flGit(t, root, "commit", "-q", "--allow-empty", "-m", "later main")
							mainHead = flGit(t, root, "rev-parse", "HEAD")
							flGit(t, root, "checkout", "-q", "--detach", source)
							for p, b := range files {
								if err := os.WriteFile(filepath.Join(root, p), []byte(b), 0644); err != nil {
									t.Fatal(err)
								}
							}
						}
						flGit(t, root, "update-ref", "refs/remotes/origin/main", mainHead)
					}
					f.commitSHAs = map[string]string{"main": head}
					for p, b := range files {
						f.setFile(p, b)
					}
					record := []byte("{\"brief\":\"vf/01\",\"outcome\":\"verified\"}\n")
					landed, got, cleanup, err := landedClosureTree(root, target, nil, record, false, f, deskkit.ForgeRepo{}, "main")
					if err != nil {
						t.Fatal(err)
					}
					defer cleanup()
					if got != head || landed == root {
						t.Fatal("closure did not get exact separate landed checkout")
					}
					if flGit(t, landed, "rev-parse", "HEAD") != head {
						t.Fatal("wrong checkout head")
					}
					// Full lint still needs the fetched baseline for its other
					// transition gates, independently of strict actor acceptance.
					if flGit(t, landed, "rev-parse", "refs/remotes/origin/main") != flGit(t, root, "rev-parse", "refs/remotes/origin/main") {
						t.Fatal("landed lint baseline differs from fetched main")
					}
					old, err := statusgenLintAt(root)
					if baseline == "old" && (err != nil || len(old) == 0) {
						t.Fatalf("frozen history should fail attribution, problems=%v err=%v", old, err)
					}
					verifiedClosureCheckFn = verifiedClosureCheckAt
					err = guardVerifiedOutcomes(root, landed, target, nil, record, f, deskkit.ForgeRepo{}, head)
					if wrong {
						if err == nil {
							t.Fatal("worker-authored closure accepted")
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						introduced, err := lintDiffAt(landed, target, record)
						if err != nil || len(introduced) != 0 {
							t.Fatalf("lint diff=%v err=%v", introduced, err)
						}
						if err := gateVerifiedSidecarLanding(target, landed, nil, record, false); err != nil {
							t.Fatal(err)
						}
					}
					if filepath.Ext(target) == ".json" {
						// Exercise both active command routes with the real landed
						// checkout and strict attribution guard, before any mutation.
						landedClosureFn = landedClosureTree
						outcomeGuardFn = guardVerifiedOutcomes
						for _, direct := range []bool{false, true} {
							raw := "{\"ts\":\"2026-09-22T02:00:00Z\",\"brief\":\"vf/01\",\"outcome\":\"verified\",\"rows_passed\":1,\"rows_total\":1,\"sha\":\"" + source + "\"}\n"
							recordFile := writeRepoFile(t, "record.json", raw)
							args := []string{"example-org/tracker", "main", "--root", root, "--outcome-record", recordFile}
							if direct {
								rel := "docs/streams/verify-outcomes/vf/01-direct.json"
								recordFile = filepath.Join(root, rel)
								if err := os.MkdirAll(filepath.Dir(recordFile), 0755); err != nil {
									t.Fatal(err)
								}
								if err := os.WriteFile(recordFile, []byte(raw), 0644); err != nil {
									t.Fatal(err)
								}
								args = []string{"example-org/tracker", "main", "--root", root, "--evidence-file", rel}
							}
							f.writes = nil
							errbuf.Reset()
							code := run(args)
							if wrong {
								if code != deskkit.ExitRefused || len(f.writes) != 0 {
									t.Fatalf("direct=%v exit=%d writes=%d: %s", direct, code, len(f.writes), errbuf.String())
								}
							} else if code != 0 || len(f.writes) != 1 || f.writes[0].ExpectedHead != head {
								t.Fatalf("direct=%v exit=%d writes=%+v: %s", direct, code, f.writes, errbuf.String())
							}
						}
					}
					cleanup()
					if _, err := os.Stat(landed); !os.IsNotExist(err) {
						t.Fatal("landed checkout leaked")
					}
					if flGit(t, root, "rev-parse", "HEAD") != source {
						t.Fatal("admitted HEAD moved")
					}
					for p, b := range files {
						got, err := os.ReadFile(filepath.Join(root, p))
						if err != nil || string(got) != b {
							t.Fatal("admitted bytes changed")
						}
					}
				})
			}
		}
	}

}

// regression: #3343
func TestLandedRefusals(t *testing.T) {
	for _, mode := range []string{"missing-commit", "destination-moved", "unreadable-tip", "missing-baseline", "plain-outcome"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := setupFake(t)
			root, source, head, _ := landedFixture(t, false)
			f.commitSHAs = map[string]string{"main": head}
			record := []byte("{\"brief\":\"vf/01\",\"outcome\":\"verified\"}\n")
			scratch := t.TempDir()
			t.Setenv("TMPDIR", scratch)
			if mode == "missing-baseline" {
				flGit(t, root, "update-ref", "-d", "refs/remotes/origin/main")
			}
			if mode == "missing-commit" {
				f.commitSHAs["main"] = strings.Repeat("f", 40)
			}
			if mode == "unreadable-tip" {
				f.commitErr = deskkit.Unverifiable("fixture unreadable", nil)
			}
			if mode == "destination-moved" {
				f.setFile("docs/streams/verify-outcomes.jsonl", "new remote\n")
			}
			if mode == "plain-outcome" {
				record = []byte("{\"brief\":\"vf/01\",\"outcome\":\"blocked\"}\n")
			}
			got, pinned, cleanup, err := landedClosureTree(root, "docs/streams/verify-outcomes.jsonl", nil, record, false, f, deskkit.ForgeRepo{}, "main")
			defer cleanup()
			if mode == "plain-outcome" {
				if err != nil || got != root || pinned != "" || f.getCommitCalls != 0 {
					t.Fatalf("nonverified changed behavior: %s %s %v", got, pinned, err)
				}
			} else if err == nil {
				t.Fatal("unsafe target accepted")
			}
			if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
				t.Fatal("failed checkout leaked scratch")
			}
			if flGit(t, root, "rev-parse", "HEAD") != source {
				t.Fatal("source moved on failure")
			}
		})
	}
}

// regression: #3343
func TestLandedRouting(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "record", true: "direct-record"}[legacy], func(t *testing.T) {
			f, errbuf := setupFake(t)
			root := t.TempDir()
			landed := t.TempDir()
			head := strings.Repeat("b", 40)
			record := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verified","rows_passed":5,"rows_total":5,"sha":"0000002"}` + "\n"
			file := writeRepoFile(t, "record.json", record)
			args := []string{"example-org/tracker", "main", "--root", root, "--outcome-record", file}
			if legacy {
				target := "docs/streams/verify-outcomes/example-stream/14-direct.json"
				if err := os.MkdirAll(filepath.Join(root, "docs/streams/verify-outcomes/example-stream"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, target), []byte(record), 0644); err != nil {
					t.Fatal(err)
				}
				args = []string{"example-org/tracker", "main", "--root", root, "--evidence-file", target}
			}
			cleaned := false
			landedClosureFn = func(r, target string, before, after []byte, exists bool, fg deskkit.Forge, fr deskkit.ForgeRepo, branch string) (string, string, func(), error) {
				if r != root {
					t.Fatalf("admission root changed: %s", r)
				}
				return landed, head, func() { cleaned = true }, nil
			}
			outcomeGuardFn = func(r, l, target string, before, after []byte, fg deskkit.Forge, fr deskkit.ForgeRepo, ref string) error {
				if r != root || l != landed || ref != head {
					t.Fatal("closure guard lost source/landed separation")
				}
				return nil
			}
			lintDiffFn = func(r, target string, content []byte) ([]string, error) {
				if r != landed {
					t.Fatal("lint staged on frozen home")
				}
				return nil, nil
			}
			verifiedClosureCheckFn = func(r, brief string) (closureVerdict, string, error) {
				if r != landed {
					t.Fatal("sidecar gate used frozen home")
				}
				return closureAccepted, "", nil
			}
			decisionGateBranchBoardFn = func(fg deskkit.Forge, fr deskkit.ForgeRepo, r, ref, target string) ([]boardEdit, error) {
				if r != landed || ref != head {
					t.Fatal("decision gate lost pinned tree")
				}
				return nil, nil
			}
			if code := run(args); code != 0 {
				t.Fatalf("exit %d: %s", code, errbuf.String())
			}
			if !cleaned || len(f.writes) != 1 || f.writes[0].ExpectedHead != head {
				t.Fatalf("write lost exact head or cleanup: %+v", f.writes)
			}
		})
	}
}
