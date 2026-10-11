package deskkit

// Tests for the SHARED callout plumbing. They pin the contract every caller's
// fail-closed rule is written against: Run returns an error for every way the
// question can go unanswered, and it returns within its deadline.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fixtureCallout(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the callout fixtures are POSIX shell scripts")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "callout")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return p
}

func TestCalloutRunHappyPath(t *testing.T) {
	c := Callout{Path: fixtureCallout(t, "read line\necho \"saw:$line\"\necho oops >&2\n")}
	res, err := c.Run("hello\n")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "saw:hello" {
		t.Fatalf("stdout = %q — stdin did not reach the callout", res.Stdout)
	}
	if res.Stderr != "oops" {
		t.Fatalf("stderr = %q — a caller cannot quote the callout's own diagnostic", res.Stderr)
	}
}

func TestCalloutRunPassesArgsWithoutShell(t *testing.T) {
	// The argument contains shell metacharacters. exec runs the binary directly, so
	// they must arrive VERBATIM — a callout that received them word-split or expanded
	// would mean the guard's own inputs are shell-interpreted somewhere.
	c := Callout{Path: fixtureCallout(t, "printf '%s' \"$1\"\n")}
	arg := "a b; rm -rf /; $(echo pwn) *"
	res, err := c.Run("", arg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != arg {
		t.Fatalf("argument arrived as %q, want %q — something interpreted it", res.Stdout, arg)
	}
}

// TestCalloutRunErrorsWhenUnanswered is the contract every fail-closed
// caller depends on: `err != nil` is the WHOLE condition, so there is no failure mode
// a caller can forget to enumerate.
func TestCalloutRunErrorsWhenUnanswered(t *testing.T) {
	notExec := filepath.Join(t.TempDir(), "not-exec")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\necho allow\n"), 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	groupWritable := fixtureCallout(t, "echo allow\n")
	if err := os.Chmod(groupWritable, 0o775); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	writableDir := t.TempDir()
	if err := os.Chmod(writableDir, 0o777); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	inWritableDir := filepath.Join(writableDir, "callout")
	if err := os.WriteFile(inWritableDir, []byte("#!/bin/sh\necho allow\n"), 0o755); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	for name, c := range map[string]Callout{
		"empty path":         {Path: ""},
		"relative path":      {Path: "relative/callout"},
		"missing file":       {Path: filepath.Join(t.TempDir(), "nope")},
		"a directory":        {Path: t.TempDir()},
		"not executable":     {Path: notExec},
		"group-writable":     {Path: groupWritable},
		"group-writable dir": {Path: inWritableDir},
		"non-zero exit":      {Path: fixtureCallout(t, "exit 9\n")},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Run(""); err == nil {
				t.Fatalf("%s did not error — a fail-closed caller would treat it as an answer", name)
			}
		})
	}
}

// TestCalloutRunTimeoutIsReal is the WaitDelay regression.
//
// Because Run pipes stdout/stderr through writers, Wait blocks until the write end
// closes — and killing the callout on the deadline does NOT close it when the callout
// left a child holding the pipe. Without cmd.WaitDelay a 150ms deadline against a
// callout whose child slept 30s returned in 30s: the timeout was advertised and not
// enforced, which for a fail-closed caller means the guard hangs rather than refuses.
func TestCalloutRunTimeoutIsReal(t *testing.T) {
	c := Callout{Path: fixtureCallout(t, "sleep 30\necho allow\n"), Timeout: 150 * time.Millisecond}
	start := time.Now()
	_, err := c.Run("")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a callout that never answered returned no error")
	}
	if !strings.Contains(err.Error(), "did not answer within") {
		t.Fatalf("the error does not name the timeout: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Run took %s for a 150ms deadline — the deadline is not enforced against a "+
			"callout whose child holds the output pipe", elapsed)
	}
}

// TestCalloutRunBoundsOutput — a callout that streams is not answering the question,
// and how much memory the guard spends on it must not be the adopter's to choose.
func TestCalloutRunBoundsOutput(t *testing.T) {
	c := Callout{Path: fixtureCallout(t, "i=0\nwhile [ $i -lt 4000 ]; do printf '0123456789012345678901234567890123456789\\n'; i=$((i+1)); done\n")}
	res, err := c.Run("")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Stdout) > maxCalloutOutput {
		t.Fatalf("stdout = %d bytes, want at most %d", len(res.Stdout), maxCalloutOutput)
	}
}

func TestCalloutRunEnvIsExactlyWhatTheCallerGives(t *testing.T) {
	t.Setenv("EXAMPLE_CALLER_VAR", "example-value")
	p := fixtureCallout(t, "env | sort\n")
	// Env nil: the callout inherits (the existing callers' behaviour).
	res, err := Callout{Path: p}.Run("")
	if err != nil || !strings.Contains(res.Stdout, "EXAMPLE_CALLER_VAR=example-value") {
		t.Fatalf("nil Env must inherit: err=%v out=%q", err, res.Stdout)
	}
	// Env set: only those variables (plus what the shell itself adds).
	res, err = Callout{Path: p, Env: []string{"PATH=/usr/bin:/bin", "LANG=C"}}.Run("")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Stdout, "EXAMPLE_CALLER_VAR") || !strings.Contains(res.Stdout, "LANG=C") {
		t.Fatalf("Env was not exact: %q", res.Stdout)
	}
}

func TestCalloutRunReportsTruncation(t *testing.T) {
	small := fixtureCallout(t, "echo ok\n")
	if res, err := (Callout{Path: small}).Run(""); err != nil || res.Truncated {
		t.Fatalf("a short answer reported Truncated: err=%v %+v", err, res)
	}
	big := fixtureCallout(t, "head -c 200000 /dev/zero | tr '\\0' a\n")
	res, err := Callout{Path: big}.Run("")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated {
		t.Fatal("an over-cap answer did not report Truncated")
	}
}

// calloutLayoutDir returns a fresh directory with the given mode. t.TempDir is
// 0700 already; the explicit chmod pins the mode the case is about.
func calloutLayoutDir(t *testing.T, mode os.FileMode) string {
	t.Helper()
	d := t.TempDir()
	if err := os.Chmod(d, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	return d
}

// calloutLayoutFile writes an "echo allow" script at dir/name with the given mode.
func calloutLayoutFile(t *testing.T, dir, name string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho allow\n"), 0o700); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	return p
}

func calloutLayoutLink(t *testing.T, target, link string) string {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	return link
}

// calloutMidLink builds <open>/via/callout where via is a link to a safe bin
// directory holding the callout, and <open> has the given mode. It returns the
// configured path, whose middle component (via) is the link.
func calloutMidLink(t *testing.T, mode os.FileMode) string {
	t.Helper()
	bin := calloutLayoutDir(t, 0o755)
	calloutLayoutFile(t, bin, "callout", 0o755)
	open := calloutLayoutDir(t, 0o755)
	calloutLayoutLink(t, bin, filepath.Join(open, "via"))
	if err := os.Chmod(open, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	return filepath.Join(open, "via", "callout")
}

// TestCalloutRefusalNamesRealDir pins that a refusal names the directory that
// really holds the link (its physical, cleaned path), so the `chmod` it suggests
// is applied to the right directory rather than to a spelling such as a/b/..
func TestCalloutRefusalNamesRealDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the callout fixtures are POSIX shell scripts")
	}
	root := calloutLayoutDir(t, 0o755)
	for _, d := range []string{"bin", "real", "real/sub"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	calloutLayoutFile(t, filepath.Join(root, "real"), "target", 0o755)
	// real/sub/callout -> ../target is a link held in real/sub, reached through
	// bin/callout -> ../real/sub/callout. real/sub is the writable one.
	calloutLayoutLink(t, "../target", filepath.Join(root, "real", "sub", "callout"))
	path := calloutLayoutLink(t, "../real/sub/callout", filepath.Join(root, "bin", "callout"))
	if err := os.Chmod(filepath.Join(root, "real", "sub"), 0o777); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	physRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolving the fixture root: %v", err)
	}
	want := fmt.Sprintf("callout directory %q is group-", filepath.Join(physRoot, "real", "sub"))
	_, err = Callout{Path: path}.Run("")
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("refusal = %v; want it to name %s", err, want)
	}
}

// TestCalloutResolvedLayouts pins that the file checks and the directory checks are
// made against the file the configured path RESOLVES to, for the layout where the
// configured name (a symbolic link) and the file it resolves to live in different
// directories — and that every refusal the configured path already got is kept.
func TestCalloutResolvedLayouts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the callout fixtures are POSIX shell scripts")
	}

	type layout struct {
		path    string
		wantErr string // "" = must run; otherwise a substring the refusal names
	}
	cases := map[string]func(t *testing.T) layout{
		// --- must still run -------------------------------------------------------
		"plain path, no link": func(t *testing.T) layout {
			return layout{path: calloutLayoutFile(t, calloutLayoutDir(t, 0o755), "callout", 0o755)}
		},
		"link to a target in another safe dir": func(t *testing.T) layout {
			target := calloutLayoutFile(t, calloutLayoutDir(t, 0o755), "callout", 0o755)
			return layout{path: calloutLayoutLink(t, target, filepath.Join(calloutLayoutDir(t, 0o755), "callout"))}
		},
		"relative link into a sibling safe dir": func(t *testing.T) layout {
			root := calloutLayoutDir(t, 0o755)
			for _, d := range []string{"bin", "real"} {
				if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			}
			calloutLayoutFile(t, filepath.Join(root, "real"), "callout", 0o755)
			return layout{path: calloutLayoutLink(t, "../real/callout", filepath.Join(root, "bin", "callout"))}
		},
		"linked dir mid-path, safe dir": func(t *testing.T) layout {
			return layout{path: calloutMidLink(t, 0o755)}
		},
		"link through linked dir, safe": func(t *testing.T) layout {
			mid := calloutMidLink(t, 0o755)
			return layout{path: calloutLayoutLink(t, mid, filepath.Join(calloutLayoutDir(t, 0o755), "callout"))}
		},

		// --- new: refused because of where the RESOLVED file lives -----------------
		"link to a target in a writable dir": func(t *testing.T) layout {
			target := calloutLayoutFile(t, calloutLayoutDir(t, 0o777), "callout", 0o755)
			return layout{
				path:    calloutLayoutLink(t, target, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "group- or world-writable",
			}
		},
		"link to a target in a group-writable dir": func(t *testing.T) layout {
			target := calloutLayoutFile(t, calloutLayoutDir(t, 0o775), "callout", 0o755)
			return layout{
				path:    calloutLayoutLink(t, target, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "group- or world-writable",
			}
		},
		"relative link into a writable sibling dir": func(t *testing.T) layout {
			root := calloutLayoutDir(t, 0o755)
			if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.Mkdir(filepath.Join(root, "real"), 0o700); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.Chmod(filepath.Join(root, "real"), 0o777); err != nil {
				t.Fatalf("chmod: %v", err)
			}
			calloutLayoutFile(t, filepath.Join(root, "real"), "callout", 0o755)
			return layout{
				path:    calloutLayoutLink(t, "../real/callout", filepath.Join(root, "bin", "callout")),
				wantErr: "group- or world-writable",
			}
		},
		"intermediate link in a writable dir": func(t *testing.T) layout {
			target := calloutLayoutFile(t, calloutLayoutDir(t, 0o755), "callout", 0o755)
			mid := calloutLayoutLink(t, target, filepath.Join(calloutLayoutDir(t, 0o777), "callout"))
			return layout{
				path:    calloutLayoutLink(t, mid, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "group- or world-writable",
			}
		},
		"linked dir mid-path, open dir": func(t *testing.T) layout {
			// open/via/callout with via a link to a safe bin directory: the
			// directory holding via is world-writable, so via can be replaced.
			return layout{path: calloutMidLink(t, 0o777), wantErr: "group- or world-writable"}
		},
		"linked dir mid-path, group dir": func(t *testing.T) layout {
			return layout{path: calloutMidLink(t, 0o775), wantErr: "group- or world-writable"}
		},
		"link through linked dir, open": func(t *testing.T) layout {
			// A final-element link in a safe directory whose TARGET passes
			// through a linked directory held in a world-writable directory.
			mid := calloutMidLink(t, 0o777)
			return layout{
				path:    calloutLayoutLink(t, mid, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "group- or world-writable",
			}
		},

		// --- existing refusals, kept when the configured path is a link ---------------
		"link itself in a writable dir": func(t *testing.T) layout {
			target := calloutLayoutFile(t, calloutLayoutDir(t, 0o755), "callout", 0o755)
			return layout{
				path:    calloutLayoutLink(t, target, filepath.Join(calloutLayoutDir(t, 0o777), "callout")),
				wantErr: "group- or world-writable",
			}
		},
		"dot-dot after a linked dir, lexical dir writable": func(t *testing.T) layout {
			// The configured path spells ".." after a linked directory, so the
			// directory its text names (lexical) and the one that holds the file
			// (physical) differ. The lexical one was always checked and still is.
			real := calloutLayoutDir(t, 0o755)
			if err := os.Mkdir(filepath.Join(real, "sub"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			calloutLayoutFile(t, real, "callout", 0o755)
			lexical := calloutLayoutDir(t, 0o755)
			calloutLayoutLink(t, filepath.Join(real, "sub"), filepath.Join(lexical, "via"))
			if err := os.Chmod(lexical, 0o777); err != nil {
				t.Fatalf("chmod: %v", err)
			}
			return layout{
				path:    lexical + "/via/../callout",
				wantErr: "group- or world-writable",
			}
		},
		"dot-dot, lexical dir holds no link": func(t *testing.T) layout {
			// <lexical>/sub/via/../../callout with via -> <real>/a/b: the
			// directory the text names is <lexical>, which holds no link (via
			// sits in <lexical>/sub), so only the lexical term checks it.
			real := calloutLayoutDir(t, 0o755)
			if err := os.MkdirAll(filepath.Join(real, "a", "b"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			calloutLayoutFile(t, real, "callout", 0o755)
			lexical := calloutLayoutDir(t, 0o755)
			if err := os.Mkdir(filepath.Join(lexical, "sub"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			calloutLayoutLink(t, filepath.Join(real, "a", "b"), filepath.Join(lexical, "sub", "via"))
			if err := os.Chmod(lexical, 0o777); err != nil {
				t.Fatalf("chmod: %v", err)
			}
			return layout{
				path:    lexical + "/sub/via/../../callout",
				wantErr: "group- or world-writable",
			}
		},
		"link to a missing file": func(t *testing.T) layout {
			missing := filepath.Join(calloutLayoutDir(t, 0o755), "nope")
			return layout{
				path:    calloutLayoutLink(t, missing, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "cannot be read",
			}
		},
		"link loop": func(t *testing.T) layout {
			d := calloutLayoutDir(t, 0o755)
			calloutLayoutLink(t, filepath.Join(d, "b"), filepath.Join(d, "a"))
			calloutLayoutLink(t, filepath.Join(d, "a"), filepath.Join(d, "b"))
			return layout{path: filepath.Join(d, "a"), wantErr: "cannot be read"}
		},
		"link to a directory": func(t *testing.T) layout {
			target := calloutLayoutDir(t, 0o755)
			return layout{
				path:    calloutLayoutLink(t, target, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "is a directory",
			}
		},
		"link to a non-regular file": func(t *testing.T) layout {
			return layout{
				path:    calloutLayoutLink(t, os.DevNull, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "not a regular file",
			}
		},
		"link to a non-executable file": func(t *testing.T) layout {
			target := calloutLayoutFile(t, calloutLayoutDir(t, 0o755), "callout", 0o644)
			return layout{
				path:    calloutLayoutLink(t, target, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "not executable",
			}
		},
		"trailing slash on a file": func(t *testing.T) layout {
			return layout{
				path:    calloutLayoutFile(t, calloutLayoutDir(t, 0o755), "callout", 0o755) + "/",
				wantErr: "cannot be read",
			}
		},
		"link to a group-writable file": func(t *testing.T) layout {
			target := calloutLayoutFile(t, calloutLayoutDir(t, 0o755), "callout", 0o775)
			return layout{
				path:    calloutLayoutLink(t, target, filepath.Join(calloutLayoutDir(t, 0o755), "callout")),
				wantErr: "group- or world-writable",
			}
		},
	}

	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			l := mk(t)
			res, err := Callout{Path: l.path}.Run("")
			if l.wantErr == "" {
				if err != nil {
					t.Fatalf("a layout that must run was refused: %v", err)
				}
				if res.Stdout != "allow" {
					t.Fatalf("stdout = %q, want %q", res.Stdout, "allow")
				}
				return
			}
			if err == nil {
				t.Fatalf("ran the callout; want a refusal naming %q", l.wantErr)
			}
			if !strings.Contains(err.Error(), l.wantErr) {
				t.Fatalf("refusal %q does not name %q", err, l.wantErr)
			}
		})
	}
}
