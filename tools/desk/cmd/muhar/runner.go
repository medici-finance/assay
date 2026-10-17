package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runShell(root, command string, env []string, timeout time.Duration) CommandResult {
	ctx := context.Background()
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // operator-supplied spec command
	setProcessGroup(cmd)
	if root != "" {
		cmd.Dir = root
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stderr, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return CommandResult{TimedOut: true}
	}
	if err == nil {
		return CommandResult{}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if strings.Contains(stdout.String()+stderr.String(), "test timed out after") {
			return CommandResult{TimedOut: true}
		}
		return CommandResult{Failed: true}
	}
	return CommandResult{Err: err}
}

func runGoArgs(root string, args []string, env []string, timeout time.Duration) CommandResult {
	ctx := context.Background()
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	setProcessGroup(cmd)
	if root != "" {
		cmd.Dir = root
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stderr, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return CommandResult{TimedOut: true}
	}
	if err == nil {
		return CommandResult{}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if strings.Contains(stdout.String()+stderr.String(), "test timed out after") {
			return CommandResult{TimedOut: true}
		}
		return CommandResult{Failed: true}
	}
	return CommandResult{Err: err}
}

func buildMutation(root string, m Mutation, specBuild, specTest string, env []string, timeout time.Duration) CommandResult {
	if specBuild != "" {
		return runShell(root, specBuild, env, timeout)
	}
	if !strings.HasSuffix(m.File, ".go") {
		return CommandResult{}
	}
	base := root
	if base == "" {
		var err error
		base, err = os.Getwd()
		if err != nil {
			return CommandResult{Err: err}
		}
	}
	path := resolve(root, m.File)
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	rel, err := filepath.Rel(base, filepath.Dir(path))
	if err != nil {
		return CommandResult{Err: err}
	}
	pkg := "."
	if rel != "." {
		pkg = "./" + filepath.ToSlash(rel)
	}
	cmd, ok := goBuildFromTest(specTest, pkg)
	if !ok {
		cmd = "go test -vet=off -c -o " + shellQuote(os.DevNull) + " " + shellQuote(pkg)
	}
	return runShell(root, cmd, env, timeout)
}

// goBuildFromTest reuses a plain Go test command's environment and build flags,
// removes its test filter, and suppresses test execution. Complex shell suites
// use the mutated package fallback or an explicit spec.build command.
func goBuildFromTest(test, pkg string) (string, bool) {
	toks, err := shellWords(test)
	if err != nil {
		return "", false
	}
	idx := 0
	for idx < len(toks) && strings.Contains(toks[idx], "=") && !strings.HasPrefix(toks[idx], "-") {
		idx++
	}
	if idx+1 >= len(toks) || toks[idx] != "go" || toks[idx+1] != "test" {
		return "", false
	}
	out := append([]string(nil), toks[:idx+2]...)
	out = append(out, "-vet=off", "-c", "-o", os.DevNull)
	testValueFlags := map[string]bool{
		"-count": true, "-exec": true, "-list": true, "-parallel": true,
		"-skip": true, "-cpu": true, "-coverprofile": true, "-outputdir": true,
		"-blockprofile": true, "-cpuprofile": true, "-memprofile": true,
		"-mutexprofile": true, "-trace": true, "-fuzz": true, "-fuzztime": true,
		"-benchtime": true, "-shuffle": true,
	}
	buildValueFlags := map[string]bool{
		"-tags": true, "-toolexec": true, "-mod": true, "-modfile": true,
		"-overlay": true, "-coverpkg": true, "-ldflags": true, "-gcflags": true,
		"-asmflags": true, "-pkgdir": true, "-p": true,
	}
	args := toks[idx+2:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-args" || args[i] == "--args" {
			break
		}
		if args[i] == "-run" || args[i] == "--run" || args[i] == "-timeout" || args[i] == "--timeout" {
			i++
			continue
		}
		if args[i] == "-vet" || args[i] == "--vet" {
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-run=") || strings.HasPrefix(args[i], "--run=") || strings.HasPrefix(args[i], "-timeout=") || strings.HasPrefix(args[i], "--timeout=") {
			continue
		}
		if strings.HasPrefix(args[i], "-vet=") || strings.HasPrefix(args[i], "--vet=") {
			continue
		}
		if testValueFlags[args[i]] {
			i++
			continue
		}
		if buildValueFlags[args[i]] {
			if i+1 >= len(args) {
				return "", false
			}
			out = append(out, args[i], args[i+1])
			i++
			continue
		}
		// Package patterns are deliberately replaced with the package that
		// contains this mutant. `go test -c` must build one package and never
		// execute its init functions or tests as a side effect of validation.
		if !strings.HasPrefix(args[i], "-") {
			continue
		}
		out = append(out, args[i])
	}
	out = append(out, pkg)
	var quoted []string
	for _, token := range out {
		quoted = append(quoted, shellQuote(token))
	}
	return strings.Join(quoted, " "), true
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
