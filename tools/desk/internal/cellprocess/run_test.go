package cellprocess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

type fixtureResult struct {
	Args                  []string
	Dir, Value, Inherited string
	StdinBytes            int
}

func TestProcessFixture(t *testing.T) {
	if os.Getenv("CELLPROCESS_FIXTURE") != "1" {
		return
	}
	i := 0
	for i < len(os.Args) && os.Args[i] != "--" {
		i++
	}
	args := os.Args[i+1:]
	switch args[0] {
	case "record":
		dir, _ := os.Getwd()
		stdin, _ := io.ReadAll(os.Stdin)
		_ = json.NewEncoder(os.Stdout).Encode(fixtureResult{Args: args[2:], Dir: dir, Value: os.Getenv("FIXTURE_VALUE"), Inherited: os.Getenv("CELLPROCESS_PARENT_ONLY"), StdinBytes: len(stdin)})
		_, _ = os.Stderr.WriteString("fixture stderr\n")
		code, _ := strconv.Atoi(args[1])
		os.Exit(code)
	case "sleep":
		time.Sleep(time.Hour)
	case "heartbeat":
		_ = os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())), 0600)
		for {
			f, err := os.OpenFile(args[2], os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				os.Exit(92)
			}
			_, _ = f.WriteString(".")
			_ = f.Close()
			time.Sleep(20 * time.Millisecond)
		}
	case "spawn-exit", "spawn-wait":
		child := exec.Command(os.Args[0], "-test.run=^TestProcessFixture$", "--", "heartbeat", args[1], args[2])
		child.Env, child.Stdout, child.Stderr = os.Environ(), os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(93)
		}
		until := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(args[1]); err == nil {
				break
			}
			if time.Now().After(until) {
				_ = child.Process.Kill()
				_ = child.Wait()
				os.Exit(94)
			}
			time.Sleep(5 * time.Millisecond)
		}
		if args[0] == "spawn-wait" {
			_ = child.Wait()
		}
		os.Exit(0)
	default:
		os.Exit(95)
	}
	os.Exit(0)
}

func fixture(t *testing.T, args ...string) ([]string, []string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv := append([]string{exe, "-test.run=^TestProcessFixture$", "--"}, args...)
	return argv, []string{"CELLPROCESS_FIXTURE=1", "FIXTURE_VALUE=exact value ' & % Ω", "SYSTEMROOT=" + os.Getenv("SYSTEMROOT")}
}

func TestRunPreservesLaunchAndExit(t *testing.T) {
	t.Setenv("CELLPROCESS_PARENT_ONLY", "must-not-leak")
	dir := filepath.Join(t.TempDir(), "work ' & % Ω")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// The argument that looks like shell syntax remains an ordinary argument.
	argv, env := fixture(t, "record", "23", "a b", "' & % Ω", "$(touch not-a-command)", "")
	var stdout, stderr bytes.Buffer
	code, uncertain, err := Run(context.Background(), argv, env, dir, &stdout, &stderr)
	if code != 23 || uncertain || err == nil {
		t.Fatalf("result=%d uncertain=%v err=%v", code, uncertain, err)
	}
	var got fixtureResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, argv[5:]) || got.Value != "exact value ' & % Ω" || got.Inherited != "" || got.StdinBytes != 0 {
		t.Fatalf("child launch changed: %+v", got)
	}
	wantDir, _ := filepath.EvalSymlinks(dir)
	gotDir, _ := filepath.EvalSymlinks(got.Dir)
	if gotDir != wantDir || stderr.String() != "fixture stderr\n" {
		t.Fatalf("cwd/stderr changed: %q / %q", got.Dir, stderr.String())
	}
}

func TestRunDeadline(t *testing.T) {
	argv, env := fixture(t, "sleep")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, uncertain, err := Run(ctx, argv, env, t.TempDir(), io.Discard, io.Discard)
	if uncertain || !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 5*time.Second {
		t.Fatalf("deadline not cleaned: uncertain=%v err=%v elapsed=%s", uncertain, err, time.Since(started))
	}
}

type blindTree struct{ processTree }

func (blindTree) empty(*os.Process) (bool, error) {
	return false, errors.New("fixture cannot observe cleanup")
}

func TestRunCleanupUncertaintyIsReported(t *testing.T) {
	argv, env := fixture(t, "record", "0")
	code, uncertain, err := run(context.Background(), argv, env, t.TempDir(), io.Discard, io.Discard, func(cmd *exec.Cmd) (processTree, error) {
		tree, err := newProcessTree(cmd)
		return blindTree{tree}, err
	})
	if code != 0 || !uncertain || err == nil {
		t.Fatalf("blind cleanup reported clean: %d %v %v", code, uncertain, err)
	}
}

type deniedKillTree struct{ processTree }

func (deniedKillTree) kill(*os.Process) error { return errors.New("fixture termination denied") }

func TestRunTerminationFailureIsUncertain(t *testing.T) {
	argv, env := fixture(t, "sleep")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, uncertain, err := run(ctx, argv, env, t.TempDir(), io.Discard, io.Discard, func(cmd *exec.Cmd) (processTree, error) {
		tree, err := newProcessTree(cmd)
		return deniedKillTree{tree}, err
	})
	// WaitDelay's direct-child fallback reaps this fixture, but a failed tree
	// termination is still uncertainty and must never license the next tick.
	if !uncertain || !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 5*time.Second {
		t.Fatalf("termination failure not bounded/uncertain: %v %v %s", uncertain, err, time.Since(started))
	}
}

func TestRunRefusesBeforeLaunch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	argv, env := fixture(t, "sleep")
	if _, uncertain, err := Run(ctx, argv, env, "", io.Discard, io.Discard); uncertain || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled start: %v %v", uncertain, err)
	}
	if _, uncertain, err := Run(context.Background(), nil, nil, "", nil, nil); uncertain || err == nil {
		t.Fatalf("invalid start: %v %v", uncertain, err)
	}
	if _, uncertain, err := Run(context.Background(), []string{filepath.Join(t.TempDir(), "missing")}, nil, "", nil, nil); uncertain || err == nil {
		t.Fatalf("missing executable: %v %v", uncertain, err)
	}
}
