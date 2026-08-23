// Package gittest is the shared behaviour-golden harness for the desktools-go-git
// migration (brief 01).
//
// The migration is a seam-swap: ~90 call sites move from the per-tool git-binary seams
// to in-process go-git. A swap is correct when the OUTCOME is preserved — resolved
// SHAs, the ref set, tree/blob contents, returned values, the error class — not when
// the argv is. The existing argv-asserting tests convert to goldens built on this
// harness: same repo fixtures, assertions on outcomes.
//
// The harness deliberately uses only the stdlib plus the local `git` binary to build
// fixtures and read outcomes (go-git itself enters in brief 02) — that is what lets
// the SAME golden pass before and after a seam swap.
package gittest

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// update rewrites the golden files on disk instead of asserting against them.
// Run `go test ./internal/gittest/ -update` only when an outcome change is intended.
var update = flag.Bool("update", false, "rewrite golden files")

// Outcome is the behaviour snapshot: repo state after the operation plus the
// operation's own return. It contains no argv — a seam swap that preserves this
// outcome is a correct swap by definition.
type Outcome struct {
	Refs  map[string]string `json:"refs"`  // refname -> resolved SHA
	Files map[string]string `json:"files"` // path -> blob content at HEAD's tree
	Val   string            `json:"val"`   // operation's returned value
	Err   string            `json:"err"`   // error class ("none", or the error text)
}

// Fixture is a scratch repository built from the stdlib + the local git binary.
type Fixture struct {
	Dir string
}

// Git runs `git <args...>` in the fixture dir and returns trimmed stdout.
// Commits are made deterministic (fixed author/committer identity AND dates) so the
// SHAs a golden snapshots are stable across runs — a real SHA, but a reproducible one.
func (f *Fixture) Git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = f.Dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		"GIT_AUTHOR_DATE=2001-02-03T04:05:06Z", "GIT_COMMITTER_DATE=2001-02-03T04:05:06Z",
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return strings.TrimSpace(string(out)), &Error{Op: args[0], Detail: strings.TrimSpace(string(ee.Stderr))}
		}
		return strings.TrimSpace(string(out)), err
	}
	return strings.TrimSpace(string(out)), nil
}

// Error is the fixture's error class carrier: Op + Detail, no argv beyond the verb.
type Error struct {
	Op     string
	Detail string
}

func (e *Error) Error() string { return "git " + e.Op + ": " + e.Detail }

// NewFixture creates a scratch repo with a seeded commit (main, one file).
func NewFixture(t *testing.T) *Fixture {
	t.Helper()
	f := &Fixture{Dir: t.TempDir()}
	mustGit(t, f, "init", "-q", "-b", "main")
	for _, kv := range [][2]string{
		{"user.name", "test"},
		{"user.email", "test@example.invalid"},
	} {
		mustGit(t, f, "config", kv[0], kv[1])
	}
	f.CommitFile(t, "seed.txt", "seed\n", "seed commit")
	return f
}

func mustGit(t *testing.T, f *Fixture, args ...string) {
	t.Helper()
	if _, err := f.Git(args...); err != nil {
		t.Fatalf("fixture setup git %s: %v", args[0], err)
	}
}

// CommitFile writes path with content and commits it, returning the new HEAD sha.
func (f *Fixture) CommitFile(t *testing.T, path, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.Dir, path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, f, "add", path)
	out, err := f.Git("commit", "-q", "-m", msg)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	sha, err := f.Git("rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	_ = out
	return sha
}

// Outcome snapshots the fixture state plus the operation's return.
func (f *Fixture) Outcome(val string, err error) Outcome {
	o := Outcome{Refs: map[string]string{}, Files: map[string]string{}, Val: val, Err: "none"}
	if err != nil {
		o.Err = err.Error()
	}
	if refs, rerr := f.Git("for-each-ref", "--format=%(refname) %(objectname)"); rerr == nil {
		for _, line := range strings.Split(refs, "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				o.Refs[parts[0]] = parts[1]
			}
		}
	}
	if list, lerr := f.Git("ls-tree", "-r", "--name-only", "HEAD"); lerr == nil {
		for _, p := range strings.Split(list, "\n") {
			if p == "" {
				continue
			}
			if blob, berr := f.Git("cat-file", "blob", "HEAD:"+p); berr == nil {
				o.Files[p] = blob
			} else {
				o.Files[p] = "<unreadable>"
			}
		}
	}
	return o
}

// Record runs op against a fresh fixture, snapshots the outcome, and compares it to
// the golden file testdata/<name>.golden.json. With -update it rewrites the golden.
// Migration briefs copy this shape per op family: same fixture, outcome assertion.
func Record(t *testing.T, name string, op func(f *Fixture) (string, error)) {
	t.Helper()
	f := NewFixture(t)
	val, err := op(f)
	got := f.Outcome(val, err)
	buf, merr := json.MarshalIndent(got, "", "  ")
	if merr != nil {
		t.Fatalf("marshal outcome: %v", merr)
	}
	buf = append(buf, '\n')
	path := filepath.Join("testdata", name+".golden.json")
	if *update {
		if werr := os.MkdirAll("testdata", 0o755); werr != nil {
			t.Fatalf("mkdir testdata: %v", werr)
		}
		if werr := os.WriteFile(path, buf, 0o644); werr != nil {
			t.Fatalf("write golden: %v", werr)
		}
		return
	}
	want, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("no golden for %q — run `go test ./internal/gittest/ -update` to create it (%v)", name, rerr)
	}
	if !bytes.Equal(buf, want) {
		t.Errorf("golden mismatch for %q\n--- got ---\n%s\n--- want ---\n%s", name, buf, want)
	}
}

// SortedKeys is a convenience for migration tests asserting on ref/file sets.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
