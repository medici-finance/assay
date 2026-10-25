package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This file holds the BINARY-level legs of the service contract's three custody
// properties — the ones that are true of `cmd/deskd` as a whole and cannot be
// asserted from inside a library package:
//
//	P1  the service constructs no at-rest credential store (the audit's
//	    counter-example: custody.New at main.go:106 and --authorize-operator at
//	    :112 on origin/main d218068).
//	P2  the free-text human write path (internal/comment, POST /comment/{id})
//	    is not reachable from this binary — a compile-time exclusion, so the
//	    route cannot be re-enabled by a flag.
//	P3  a non-loopback --addr with token validation OFF is a refused start.

const modulePath = "github.com/medici-finance/assay/deskd"

// TestServiceConstructsNoCredentialStore is P1 at the binary boundary: the
// service's main package must contain no at-rest credential construction and no
// authorize-and-store flag. Verify row 12 of the brief greps for exactly this.
func TestServiceConstructsNoCredentialStore(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	src := string(raw)
	// The Verify-row-12 grep set: the at-rest construction, its flags, and the
	// device flow whose only purpose was to fill the at-rest file.
	for _, banned := range []string{"custody.New(", "custody.Store", "operator-dir", "authorize-operator", "RequestDeviceCode", "PollForToken", "os.WriteFile"} {
		if strings.Contains(src, banned) {
			t.Errorf("cmd/deskd/main.go still contains %q — the service holds no human credential at rest; the credential arrives with the named-action request (custody rule (b), the service contract P1)", banned)
		}
	}
	// The guarantee must be STATED at startup, not merely true: the operator
	// reading the banner is the person the gate asks to confirm it.
	if !strings.Contains(src, "holds no human credential at rest") {
		t.Error("the startup banner does not state that the service holds no human credential at rest — the service contract task 4 requires it to say so")
	}
}

// TestServiceExcludesFreeTextCommentPath is P2 as a build property. The
// route-boundary review's deciding counter-example for P2 was POST /comment/{id}
// (internal/comment/handlers.go:40, mounted cmd/deskd/main.go:321): a
// human-authored, free-text platform write outside the enumerated allow-list
// {answer, ack}.
//
// Un-mounting the route would satisfy a route-table walk while leaving the
// handler one line away from being re-mounted. Excluding the PACKAGE from the
// service binary's transitive import graph is the stronger property — the same
// reasoning internal/mcp's authority test uses for the credential — and it is
// the one this test pins. internal/comment itself is retained for the client
// host (the service contract), where a free-text comment is the person's own write
// on their own machine, not a service-side human write.
func TestServiceExcludesFreeTextCommentPath(t *testing.T) {
	deps := goListDeps(t, ".")
	const commentPkg = modulePath + "/internal/comment"
	if deps[commentPkg] {
		t.Fatalf("cmd/deskd imports %s — the free-text human write POST /comment/{id} is OUTSIDE the enumerated named-action allow-list {answer, ack}; "+
			"the service must not be able to serve it (the service contract P2, route-boundary review)", commentPkg)
	}
	// Positive control: the service DOES reach the answer path, which is where
	// the two allow-listed named actions execute. Without it a misspelt package
	// path would make the exclusion above pass vacuously.
	const answerPkg = modulePath + "/internal/answer"
	if !deps[answerPkg] {
		t.Fatalf("cmd/deskd does not import %s — either the named actions left the service or the checked path is wrong, "+
			"which would make the exclusion check vacuous", answerPkg)
	}
}

// TestRefusedStartOffLoopback is P3: the service refuses to bind a non-loopback
// address unless token validation is switched on. On origin/main d218068 there
// was no check at all between the --addr flag (main.go:93) and
// srv.ListenAndServe() (main.go:376): `deskd --addr 0.0.0.0:8787` bound, and
// nothing refused it.
//
// The refusal must NAME the flag that would fix it (brief Verify row 5), and it
// must happen before any listener exists — so the test asserts a non-zero exit
// within a second, not merely an error somewhere in the log.
func TestRefusedStartOffLoopback(t *testing.T) {
	bin := buildDeskd(t)
	cmd := exec.Command(bin, "--addr", "0.0.0.0:0", "--auth", "loopback", "--config", "config/cells.yaml")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "DESKD_GITHUB_TOKEN=not-a-real-token")
	out, err := runWithTimeout(t, cmd, 10*time.Second)
	if err == nil {
		t.Fatalf("deskd --addr 0.0.0.0:0 --auth=loopback started (exit 0) — a non-loopback bind with token validation OFF must be a refused start (the service contract P3).\n%s", out)
	}
	if !strings.Contains(out, "--auth") {
		t.Fatalf("the refusal does not name --auth, so an operator cannot tell what would fix it:\n%s", out)
	}
	if !strings.Contains(out, "0.0.0.0") {
		t.Fatalf("the refusal does not name the address it refused:\n%s", out)
	}
}

// TestLoopbackBindStillStarts is the paired positive control for the refusal
// above: the default loopback bind is unaffected, so P3 cannot be satisfied by
// a check that refuses everything.
func TestLoopbackBindStillStarts(t *testing.T) {
	bin := buildDeskd(t)
	cmd := exec.Command(bin, "--addr", "127.0.0.1:0", "--auth", "loopback", "--config", "config/cells.yaml", "--once")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "DESKD_GITHUB_TOKEN=not-a-real-token", "DESKD_API_UNREACHABLE=1")
	out, _ := runWithTimeout(t, cmd, 30*time.Second)
	// --once polls and exits; the poll fails without network, which is a
	// degraded RESULT (exit 1) not a refused start. What must NOT appear is the
	// bind refusal — the loopback address is allowed.
	if strings.Contains(out, "refusing to bind") {
		t.Fatalf("a loopback bind was refused — the P3 check must refuse only NON-loopback binds:\n%s", out)
	}
}

func buildDeskd(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "deskd")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/deskd")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/deskd: %v\n%s", err, out)
	}
	return bin
}

// runWithTimeout runs cmd, returning its combined output and its exit error.
// A process still running at the deadline is killed and reported as "did not
// exit" — a refused start that hangs is not a refusal.
func runWithTimeout(t *testing.T, cmd *exec.Cmd, d time.Duration) (string, error) {
	t.Helper()
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", cmd.Path, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return buf.String(), err
	case <-time.After(d):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("%s did not exit within %s — a refused start must exit, not hang:\n%s", cmd.Path, d, buf.String())
		return buf.String(), nil
	}
}

func goListDeps(t *testing.T, dir string) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps in %s failed: %v\n%s", dir, err, out)
	}
	deps := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		deps[strings.TrimSpace(line)] = true
	}
	return deps
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find the module root")
		}
		dir = parent
	}
}
