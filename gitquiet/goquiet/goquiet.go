// Package goquiet stops the go command's telemetry child in a scratch home that a test
// hands to cmd/go. It is test support, like its parent package: production tools do
// not import it.
//
// THE DEFECT IT CLOSES. Unless telemetry is off, a go command starts a telemetry child
// it does not wait for, at most once a day per telemetry directory. The directory is
// os.UserConfigDir()/go/telemetry, so it moves with HOME (or, on Unix other than
// darwin, with XDG_CONFIG_HOME when that is set). In a fresh scratch home the daily
// token is always free, so the first go command a test runs there starts the child.
// The child outlives the command the test waited on and keeps writing in the telemetry
// directory, so it races the removal of the scratch home, and when it wins the test
// fails with
//
//	TempDir RemoveAll cleanup: unlinkat …: directory not empty
//
// after every assertion held: the same shape as the git maintenance child its parent
// package stops.
//
// THE FIX. `go telemetry off`, run first in the scratch home, writes the mode file and
// starts no child; every later go command there reads mode off and starts none either.
// Off does that for one environment; Cmd does it for the environment of the go command
// a test is about to run; Home returns a t.TempDir() home already treated, and checks at
// cleanup that no go command wrote telemetry there.
//
// Off acts only on a scratch telemetry directory: one that differs from the directory
// this process resolved when it started. A test environment whose config directory is
// still the real one (an inherited XDG_CONFIG_HOME, or Windows, where the directory is
// %AppData% and does not follow HOME) has no scratch telemetry directory to race, and
// Off leaves the developer's own telemetry setting alone.
package goquiet

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// host is the config directory this process resolved before any test moved HOME; Off
// never changes the telemetry mode under it.
var host = func() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Clean(d)
}()

// lookup returns the value a child started with env sees for key: the last entry
// wins, as os/exec keeps the last of duplicate keys.
func lookup(env []string, key string) string {
	v := ""
	for _, kv := range env {
		k, val, ok := strings.Cut(kv, "=")
		if ok && (k == key || runtime.GOOS == "windows" && strings.EqualFold(k, key)) {
			v = val
		}
	}
	return v
}

// configDir applies os.UserConfigDir's rules to env instead of this process's
// environment. It returns "" where os.UserConfigDir would return an error.
func configDir(env []string) string {
	var dir string
	switch runtime.GOOS {
	case "windows":
		dir = lookup(env, "AppData")
	case "darwin", "ios":
		if h := lookup(env, "HOME"); h != "" {
			dir = h + "/Library/Application Support"
		}
	case "plan9":
		if h := lookup(env, "home"); h != "" {
			dir = h + "/lib"
		}
	default:
		dir = lookup(env, "XDG_CONFIG_HOME")
		if dir == "" {
			if h := lookup(env, "HOME"); h != "" {
				dir = h + "/.config"
			}
		} else if !filepath.IsAbs(dir) {
			dir = ""
		}
	}
	if dir == "" {
		return ""
	}
	return filepath.Clean(dir)
}

// TelemetryDir returns the scratch telemetry directory a go command started with env
// would use, or "" when env leaves it at this process's own directory (or names none).
// A nil env means this process's environment.
func TelemetryDir(env []string) string {
	if env == nil {
		env = os.Environ()
	}
	dir := configDir(env)
	if dir == "" || !filepath.IsAbs(dir) || dir == host {
		return ""
	}
	return filepath.Join(dir, "go", "telemetry")
}

// Off turns go telemetry off in the scratch telemetry directory that env gives a go
// command, by running `go telemetry off` with env, so a go command the test starts
// there afterwards starts no telemetry child. A nil env means this process's
// environment: call Off(nil) right after os.Setenv("HOME", …) in a TestMain fixture.
// Off does nothing when env has no scratch telemetry directory, when the mode there
// is already off, or when there is no go command to run.
func Off(env []string) error {
	if env == nil {
		env = os.Environ()
	}
	dir := TelemetryDir(env)
	if dir == "" {
		return nil
	}
	if b, err := os.ReadFile(filepath.Join(dir, "mode")); err == nil && strings.HasPrefix(string(b), "off") {
		return nil
	}
	gobin, err := exec.LookPath("go")
	if err != nil {
		return nil
	}
	cmd := exec.Command(gobin, "telemetry", "off")
	// GOTOOLCHAIN=local: a go.mod in a parent of the working directory never makes
	// this one command download or switch toolchains.
	cmd.Env = append(append([]string(nil), env...), "GOTOOLCHAIN=local", "GOFLAGS=")
	cmd.Dir = os.TempDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go telemetry off in %s: %v: %s", dir, err, out)
	}
	return nil
}

// Cmd calls Off with the environment cmd will run with (cmd.Env, or this process's when
// that is nil) and returns cmd, so a test wraps the go command it is about to run:
//
//	out, err := goquiet.Cmd(exec.Command("go", "build", "-o", bin, ".")).CombinedOutput()
//
// Call it after cmd.Env is final. When Off fails, cmd.Err carries the failure, so
// cmd's Run, Output, CombinedOutput or Start returns it and the go command never runs.
func Cmd(cmd *exec.Cmd) *exec.Cmd {
	if cmd.Err == nil {
		if err := Off(cmd.Env); err != nil {
			cmd.Err = err
		}
	}
	return cmd
}

// Wrote lists, relative to the scratch telemetry directory env gives a go command,
// every file there except the mode file Off writes. A counter file or an upload token
// there means a go command ran with telemetry on, and so started the child Off
// prevents. Wrote returns nothing when env has no scratch telemetry directory.
func Wrote(env []string) ([]string, error) {
	dir := TelemetryDir(env)
	if dir == "" {
		return nil, nil
	}
	var wrote []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if rel, _ := filepath.Rel(dir, path); rel != "mode" {
			wrote = append(wrote, filepath.ToSlash(rel))
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	return wrote, err
}

// Home returns a new t.TempDir() for a test to use as HOME, with go telemetry already
// off in it for this process's environment with HOME replaced, and registers a
// cleanup that fails t if a go command wrote telemetry there anyway. The check runs
// before the directory is removed.
func Home(t testing.TB) string {
	t.Helper()
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home)
	if err := Off(env); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		wrote, err := Wrote(env)
		if err != nil {
			t.Error(err)
		}
		if len(wrote) > 0 {
			t.Errorf("a go command ran with telemetry on in scratch home %s, so it started a telemetry child it does not wait for; it wrote %q", home, wrote)
		}
	})
	return home
}
