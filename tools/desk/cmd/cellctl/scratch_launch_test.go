package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
)

func init() {
	if os.Getenv("CELL_SCRATCH_FIXTURE") != "1" {
		return
	}
	work := os.Getenv("TMPDIR")
	if work == "" || work != os.Getenv("TMP") || work != os.Getenv("TEMP") {
		os.Exit(81)
	}
	if err := os.WriteFile(filepath.Join(work, "build-output"), []byte(strings.Repeat("x", 1024*1024)), 0600); err != nil {
		os.Exit(82)
	}
	// The canonical artifact is outside disposable scratch. Read it back before ack.
	destination := os.Getenv("SCRATCH_EVIDENCE")
	if err := os.WriteFile(destination, []byte("verified task outcome"), 0600); err != nil {
		os.Exit(83)
	}
	if b, err := os.ReadFile(destination); err != nil || string(b) != "verified task outcome" {
		os.Exit(84)
	}
	if os.Getenv("SCRATCH_NO_ACK") != "1" {
		s, err := cellscratch.Open(os.Getenv("ASSAY_SCRATCH_ROOT"))
		if err != nil {
			os.Exit(85)
		}
		if err = s.Acknowledge(os.Getenv("ASSAY_SCRATCH_ID"), destination, false); err != nil {
			fmt.Fprintln(os.Stderr, "ack failed:", err)
			os.Exit(86)
		}
		s.Close()
	}
	if os.Getenv("SCRATCH_HANG") == "1" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if os.Getenv("SCRATCH_FAIL") == "1" {
		fmt.Fprintln(os.Stderr, "selected failure diagnostic")
		os.Exit(17)
	}
	final := "tick role=" + os.Getenv("DESK_LOOP") + " outcome=ok swept=1 acted=1 filed=0 duration=1\n"
	for i, a := range os.Args {
		if a == "--output-last-message" && i+1 < len(os.Args) {
			if err := os.WriteFile(os.Args[i+1], []byte(final), 0600); err != nil {
				os.Exit(87)
			}
			os.Exit(0)
		}
	}
	fmt.Print(final)
	os.Exit(0)
}
func TestScratchLaunchRoles(t *testing.T) {
	for _, role := range []string{"worker-desk", "pr-review-desk", "verify-desk"} {
		for _, harness := range harnessValues {
			t.Run(role+"/"+harness, func(t *testing.T) {
				c := &Cell{Name: "fixture", Dir: t.TempDir(), Config: t.TempDir()}
				os.MkdirAll(c.cadenceDir(role), 0700)
				wt := t.TempDir()
				evidence := filepath.Join(t.TempDir(), "result")
				env := append(os.Environ(), "CELL_SCRATCH_FIXTURE=1", "ASSAY_SOURCE_REVISION=fixture-revision", "DESK_LOOP="+role, "SCRATCH_EVIDENCE="+evidence)
				for n := 0; n < 3; n++ {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					result := c.executeCadencePass(ctx, role, harness, []string{os.Args[0], "--fixture-prompt"}, env, wt)
					cancel()
					if result.Err != nil || result.ExitCode != 0 || result.Outcome != "ok" {
						t.Fatalf("result: %+v", result)
					}
					entries, err := os.ReadDir(filepath.Join(c.Dir, "run", "scratch"))
					if err != nil {
						t.Fatal(err)
					}
					for _, e := range entries {
						if strings.HasPrefix(e.Name(), "task-") {
							t.Fatal("successful role scratch grows with pass count", e.Name())
						}
					}
					if _, err = os.Stat(evidence); err != nil {
						t.Fatal("canonical evidence removed", err)
					}
				}
			})
		}
	}
}
func TestScratchLaunchFailure(t *testing.T) {
	for _, noAck := range []bool{false, true} {
		t.Run(fmt.Sprint(noAck), func(t *testing.T) {
			c := &Cell{Name: "fixture", Dir: t.TempDir(), Config: t.TempDir()}
			wt := t.TempDir()
			env := append(os.Environ(), "CELL_SCRATCH_FIXTURE=1", "ASSAY_SOURCE_REVISION=fixture-revision", "DESK_LOOP=worker-desk", "SCRATCH_EVIDENCE="+filepath.Join(t.TempDir(), "result"))
			if noAck {
				env = append(env, "SCRATCH_NO_ACK=1")
			} else {
				env = append(env, "SCRATCH_FAIL=1")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result := c.executeCadencePass(ctx, "worker-desk", "claude", []string{os.Args[0], "fixture"}, env, wt)
			if !noAck && (result.ExitCode != 17 || result.Err == nil) {
				t.Fatal("failure result lost", result)
			}
			root := filepath.Join(c.Dir, "run", "scratch")
			entries, _ := os.ReadDir(root)
			var found bool
			for _, e := range entries {
				if !strings.HasPrefix(e.Name(), "task-") {
					continue
				}
				found = true
				b, err := os.ReadFile(filepath.Join(root, e.Name(), "record.json"))
				if err != nil {
					t.Fatal(err)
				}
				var r cellscratch.Record
				if err = json.Unmarshal(b, &r); err != nil {
					t.Fatal(err)
				}
				_, workErr := os.Stat(filepath.Join(root, e.Name(), "work"))
				if noAck {
					if r.Evidence != "pending" || workErr != nil {
						t.Fatal("pending evidence discarded", r, workErr)
					}
				} else {
					if r.ExitCode != 17 || !os.IsNotExist(workErr) {
						t.Fatal("failure tree not compacted", r, workErr)
					}
					b, err = os.ReadFile(filepath.Join(root, e.Name(), "diagnostic.txt"))
					if err != nil || !strings.Contains(string(b), "selected failure diagnostic") {
						t.Fatal("selected diagnostics lost", err)
					}
				}
			}
			if !found {
				t.Fatal("protected/failure record missing")
			}
		})
	}
}
func TestScratchCodexPropagation(t *testing.T) {
	args := scratchArgv("codex", []string{"codex", "exec", "prompt"}, []string{"TMPDIR=/owned", "ASSAY_SCRATCH_ID=task-id", "TOKEN=secret"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "shell_environment_policy.set.TMPDIR=") || !strings.Contains(joined, "shell_environment_policy.set.ASSAY_SCRATCH_ID=") || strings.Contains(joined, "secret") || args[len(args)-1] != "prompt" {
		t.Fatal(args)
	}
}

func TestScratchLaunchBudget(t *testing.T) {
	c := &Cell{Name: "fixture", Dir: t.TempDir(), Env: &Env{vals: map[string]string{"CELL_SCRATCH_MAX_BYTES": "0"}}}
	env := append(os.Environ(), "CELL_SCRATCH_FIXTURE=1", "ASSAY_SOURCE_REVISION=fixture-revision", "DESK_LOOP=worker-desk", "SCRATCH_FAIL=1", "SCRATCH_EVIDENCE="+filepath.Join(t.TempDir(), "result"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := c.executeCadencePass(ctx, "worker-desk", "claude", []string{os.Args[0], "fixture"}, env, t.TempDir())
	if result.ExitCode != 17 {
		t.Fatal("retention changed task result", result)
	}
	entries, err := os.ReadDir(filepath.Join(c.Dir, "run", "scratch"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "task-") {
			t.Fatal("cell budget not applied", e.Name())
		}
	}
}
func TestScratchLaunchCancel(t *testing.T) {
	c := &Cell{Name: "fixture", Dir: t.TempDir()}
	env := append(os.Environ(), "CELL_SCRATCH_FIXTURE=1", "ASSAY_SOURCE_REVISION=fixture-revision", "DESK_LOOP=worker-desk", "SCRATCH_HANG=1", "SCRATCH_EVIDENCE="+filepath.Join(t.TempDir(), "result"))
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	result := c.executeCadencePass(ctx, "worker-desk", "claude", []string{os.Args[0], "fixture"}, env, t.TempDir())
	if result.Err == nil || result.ExitCode == 0 || result.Uncertain {
		t.Fatal("cancellation result/cleanup", result)
	}
	root := filepath.Join(c.Dir, "run", "scratch")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "task-") {
			continue
		}
		found = true
		if _, err = os.Stat(filepath.Join(root, e.Name(), "work")); !os.IsNotExist(err) {
			t.Fatal("cancelled disposable workspace retained", err)
		}
	}
	if !found {
		t.Fatal("cancelled failure diagnostics missing")
	}
}
