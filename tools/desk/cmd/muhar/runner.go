package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// liveGroups tracks every suite and build command muhar has started and not
// yet reaped. Each one runs in its own process group (setProcessGroup), so a
// terminal Ctrl-C no longer reaches it on its own; the interrupt handler in
// main kills every tracked group before muhar exits, so an interrupted run
// leaves no suite behind.
var liveGroups = &processRegistry{procs: map[*exec.Cmd]struct{}{}}

type processRegistry struct {
	mu     sync.Mutex
	procs  map[*exec.Cmd]struct{}
	closed bool
}

// start starts cmd and tracks it. Once killAll has run, a command that starts
// afterwards is killed at once, so a worker racing the interrupt handler
// cannot leave a suite running after muhar exits.
func (r *processRegistry) start(cmd *exec.Cmd) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := cmd.Start(); err != nil {
		return err
	}
	if r.closed {
		_ = killProcessGroup(cmd)
	}
	r.procs[cmd] = struct{}{}
	return nil
}

func (r *processRegistry) done(cmd *exec.Cmd) {
	r.mu.Lock()
	delete(r.procs, cmd)
	r.mu.Unlock()
}

// killAll kills every tracked process group and marks the registry closed.
func (r *processRegistry) killAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	for cmd := range r.procs {
		_ = killProcessGroup(cmd)
	}
}

func runShell(root, command string, env []string, timeout time.Duration) CommandResult {
	return runBounded(root, "sh", []string{"-c", command}, env, timeout) //nolint:gosec // operator-supplied spec command
}

func runGoArgs(root string, args []string, env []string, timeout time.Duration) CommandResult {
	return runBounded(root, "go", args, env, timeout)
}

// runBounded runs one command in its own process group, killed as a group at
// the deadline, and classifies the exit: TimedOut for the deadline or a go
// test binary's own timeout panic, Failed for any other non-zero exit, Err
// when the command could not run at all.
func runBounded(root, name string, args []string, env []string, timeout time.Duration) CommandResult {
	ctx := context.Background()
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	setProcessGroup(cmd)
	if root != "" {
		cmd.Dir = root
	}
	cmd.Env = env
	seen := &timeoutMarker{}
	cmd.Stdout = io.MultiWriter(os.Stderr, seen)
	cmd.Stderr = io.MultiWriter(os.Stderr, seen)
	if err := liveGroups.start(cmd); err != nil {
		return CommandResult{Err: err}
	}
	err := cmd.Wait()
	liveGroups.done(cmd)
	if ctx.Err() == context.DeadlineExceeded {
		return CommandResult{TimedOut: true}
	}
	if err == nil {
		return CommandResult{}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if seen.found() {
			return CommandResult{TimedOut: true}
		}
		return CommandResult{Failed: true}
	}
	return CommandResult{Err: err}
}

// timeoutMarker watches a command's output for go test's own timeout panic
// without keeping the output: it holds only a short tail, enough to match a
// marker split across two writes.
type timeoutMarker struct {
	mu   sync.Mutex
	tail []byte
	hit  bool
}

const goTestTimeoutMarker = "test timed out after"

func (t *timeoutMarker) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.hit {
		buf := append(t.tail, p...)
		if bytes.Contains(buf, []byte(goTestTimeoutMarker)) {
			t.hit = true
		}
		keep := len(goTestTimeoutMarker) - 1
		if len(buf) > keep {
			buf = buf[len(buf)-keep:]
		}
		t.tail = append(t.tail[:0], buf...)
	}
	return len(p), nil
}

func (t *timeoutMarker) found() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.hit
}

// buildMutation proves a mutant compiles before its suite judges it. An
// explicit spec.build runs for every mutation, whatever the file; without one
// only a .go mutation is built, with a command derived from spec.test.
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

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// goTestFlagKind says what goBuildFromTest does with one `go test` flag.
type goTestFlagKind int

const (
	flagUnknown   goTestFlagKind = iota
	flagDropBool                 // a test-run flag with no value: dropped
	flagDropValue                // a test-run flag that takes a value: dropped with it
	flagKeepBool                 // a build flag with no value: carried over
	flagKeepValue                // a build flag that takes a value: carried over with it
)

// goTestFlags classifies the flags `go test` accepts (by name, one leading dash).
// A flag absent from this table makes goBuildFromTest fail closed: guessing
// whether it takes a value is what let `-covermode atomic` swallow the mutant's
// package, or a `-bench X` make the build compile the wrong one.
var goTestFlags = map[string]goTestFlagKind{
	// go test's own test-run flags.
	"-bench": flagDropValue, "-benchtime": flagDropValue, "-count": flagDropValue,
	"-cpu": flagDropValue, "-exec": flagDropValue, "-fuzz": flagDropValue,
	"-fuzzminimizetime": flagDropValue, "-fuzztime": flagDropValue, "-list": flagDropValue,
	"-o": flagDropValue, "-outputdir": flagDropValue, "-parallel": flagDropValue,
	"-run": flagDropValue, "-shuffle": flagDropValue, "-skip": flagDropValue,
	"-timeout": flagDropValue, "-vet": flagDropValue,
	"-blockprofile": flagDropValue, "-blockprofilerate": flagDropValue,
	"-coverprofile": flagDropValue, "-cpuprofile": flagDropValue, "-memprofile": flagDropValue,
	"-memprofilerate": flagDropValue, "-mutexprofile": flagDropValue,
	"-mutexprofilefraction": flagDropValue, "-trace": flagDropValue,
	"-benchmem": flagDropBool, "-failfast": flagDropBool, "-fullpath": flagDropBool,
	"-json": flagDropBool, "-short": flagDropBool, "-v": flagDropBool,
	"-n": flagDropBool, "-x": flagDropBool, "-work": flagDropBool, "-i": flagDropBool,
	// build flags shared with go build.
	"-asmflags": flagKeepValue, "-buildmode": flagKeepValue, "-compiler": flagKeepValue,
	"-covermode": flagKeepValue, "-coverpkg": flagKeepValue, "-gccgoflags": flagKeepValue,
	"-gcflags": flagKeepValue, "-installsuffix": flagKeepValue, "-ldflags": flagKeepValue,
	"-mod": flagKeepValue, "-modfile": flagKeepValue, "-overlay": flagKeepValue,
	"-p": flagKeepValue, "-pgo": flagKeepValue, "-pkgdir": flagKeepValue,
	"-tags": flagKeepValue, "-toolexec": flagKeepValue,
	"-a": flagKeepBool, "-asan": flagKeepBool, "-cover": flagKeepBool,
	"-linkshared": flagKeepBool, "-modcacherw": flagKeepBool, "-msan": flagKeepBool,
	"-race": flagKeepBool, "-trimpath": flagKeepBool, "-buildvcs": flagKeepBool,
}

// goBuildFromTest reuses a plain Go test command's environment and build flags,
// removes its test filter, and suppresses test execution. It returns false —
// and the caller builds the mutated package alone — for anything it cannot
// model exactly: a non-`go test` command, an environment prefix that is not a
// plain NAME=value, `-C`, or a flag missing from goTestFlags.
func goBuildFromTest(test, pkg string) (string, bool) {
	toks, err := shellWords(test)
	if err != nil {
		return "", false
	}
	idx := 0
	var assigns []string
	for idx < len(toks) && strings.Contains(toks[idx], "=") && !strings.HasPrefix(toks[idx], "-") {
		name, value, _ := strings.Cut(toks[idx], "=")
		if !envName.MatchString(name) {
			return "", false
		}
		// The name stays unquoted: a quoted word is not an assignment to sh,
		// which would run `'NAME=value'` as a command.
		assigns = append(assigns, name+"="+shellQuote(value))
		idx++
	}
	if idx+1 >= len(toks) || toks[idx] != "go" || toks[idx+1] != "test" {
		return "", false
	}
	out := []string{"go", "test", "-vet=off", "-c", "-o", os.DevNull}
	args := toks[idx+2:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-args" || arg == "--args" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			// Package patterns are deliberately replaced with the package that
			// contains this mutant. `go test -c` must build one package and never
			// execute its init functions or tests as a side effect of validation.
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		if strings.HasPrefix(name, "--") {
			name = name[1:]
		}
		switch goTestFlags[name] {
		case flagDropBool:
		case flagDropValue:
			if !hasValue {
				i++
			}
		case flagKeepBool:
			out = append(out, arg)
		case flagKeepValue:
			if hasValue {
				out = append(out, name+"="+value)
				continue
			}
			if i+1 >= len(args) {
				return "", false
			}
			out = append(out, name, args[i+1])
			i++
		default:
			return "", false
		}
	}
	out = append(out, pkg)
	quoted := assigns
	for _, token := range out {
		quoted = append(quoted, shellQuote(token))
	}
	return strings.Join(quoted, " "), true
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
