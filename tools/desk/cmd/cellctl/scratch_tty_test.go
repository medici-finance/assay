//go:build darwin || linux

package main

import (
	"context"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"golang.org/x/term"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestScratchTTYFixture(t *testing.T) {
	mode := os.Getenv("SCRATCH_TTY_FIXTURE")
	if mode == "" {
		return
	}
	if mode == "child" {
		for _, f := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
			if !term.IsTerminal(int(f.Fd())) {
				os.Exit(91)
			}
		}
		fmt.Fprintln(os.Stdout, "interactive child has three terminal descriptors")
		os.Exit(0)
	}
	dir := os.Getenv("SCRATCH_TTY_DIR")
	c := &Cell{Name: "fixture", Dir: dir}
	lease, err := cellcadence.Acquire(c.cadenceDir("worker-desk"))
	if err != nil {
		os.Exit(92)
	}
	c.cadenceLease = lease
	defer lease.Close()
	defer func() {
		if value := recover(); value != nil {
			if code, ok := value.(exitCode); ok {
				os.WriteFile(filepath.Join(dir, "result"), []byte(fmt.Sprint(code.code)), 0600)
				return
			}
			panic(value)
		}
	}()
	env := envSet(os.Environ(), "SCRATCH_TTY_FIXTURE", "child")
	env = envSet(env, "ASSAY_SOURCE_REVISION", "fixture-revision")
	c.runInteractiveHarness("worker-desk", []string{os.Args[0], "-test.run=^TestScratchTTYFixture$"}, env, dir)
}

func TestScratchTerminal(t *testing.T) {
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("real PTY requires script")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "terminal.log")
	args := []string{"-q", log, os.Args[0], "-test.run=^TestScratchTTYFixture$"}
	if runtime.GOOS == "linux" {
		command := "'" + strings.ReplaceAll(os.Args[0], "'", "'\\''") + "' -test.run=^TestScratchTTYFixture$"
		args = []string{"-q", "-e", "-c", command, log}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, args...)
	cmd.Env = append(os.Environ(), "SCRATCH_TTY_FIXTURE=owner", "SCRATCH_TTY_DIR="+dir)
	output, runErr := cmd.CombinedOutput()
	result, err := os.ReadFile(filepath.Join(dir, "result"))
	if runErr != nil || err != nil || string(result) != "0" {
		t.Fatalf("interactive terminal contract: result=%q error=%v run=%v output=%s", result, err, runErr, output)
	}
}
