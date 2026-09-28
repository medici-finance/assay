package deskkit

// End-to-end tests for the ambient-identity check, driven through the REAL
// ambientLoginProbe against a stub `gh` placed alone on PATH.
//
// Why end-to-end and not an injected AmbientLogin: the defect these tests pin was
// a probe/check CONTRACT drift. The check's "no ambient identity — safe" branch
// was keyed on a probe return ("" with no error) that the real probe never
// produced — a `gh` that is not logged in, or an App token that answers 403 on
// /user, both came back as errors and were read as could-not-check. Every
// injected-probe test of that branch passed, because the injection handed the
// check a value reality never does. Driving the real probe through a stub `gh`
// proves each branch of the check is REACHABLE from what `gh` actually prints.
//
// The five cases, per the ambient-identity contract:
//
//  1. `gh` not logged in, or the ambient credential answers 401/403 on /user
//     (an App/integration token) → no usable ambient human identity → the
//     identity half passes and the verdict is decided by the transport half.
//  2. a human login is ambient (the blessing login or any other) → a
//     NON-BLOCKING warning naming the login; it never reddens the envelope.
//  3. a bot/App slug is ambient → checked-failed (blocking).
//  4. `gh` absent, or a probe that genuinely cannot run → could-not-check.
//  5. no remediation recommends logging in as the blessing login; every one
//     leads with clearing the ambient credential for desk shells.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// stubGH writes an executable `gh` into a fresh directory, makes that directory
// the WHOLE of PATH, and returns it. body is the script after the shebang; the
// shebang is absolute so the script needs nothing from PATH.
func stubGH(t *testing.T, shebang, body string) {
	t.Helper()
	dir := t.TempDir()
	script := shebang + "\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// ghSays stubs a `gh` that writes stdout/stderr and exits with code — the shape
// of one real `gh api user -q .login` outcome.
func ghSays(t *testing.T, stdout, stderr string, code int) {
	t.Helper()
	var b strings.Builder
	if stdout != "" {
		b.WriteString("printf '%s\\n' '" + stdout + "'\n")
	}
	if stderr != "" {
		b.WriteString("printf '%s\\n' '" + stderr + "' >&2\n")
	}
	b.WriteString("exit " + strconv.Itoa(code))
	stubGH(t, "#!/bin/sh", b.String())
}

// runRealAmbient runs the full preflight with every probe green EXCEPT that the
// ambient identity is read by the real probe (against whatever `gh` the test
// stubbed onto PATH).
func runRealAmbient(t *testing.T, p PreflightProbes) (PreflightReport, Check) {
	t.Helper()
	p.AmbientLogin = ambientLoginProbe
	rep := runPF(t, p)
	return rep, pfCheck(t, rep, CheckAmbientID)
}

// The literal stderr shapes `gh api user` prints for each outcome.
const (
	ghNotLoggedInStderr = "To get started with GitHub CLI, please run:  gh auth login"
	ghIntegration403    = "gh: Resource not accessible by integration (HTTP 403)"
	ghBadCreds401       = "gh: Bad credentials (HTTP 401)"
	ghRateLimit403      = "gh: API rate limit exceeded for user ID 1. (HTTP 403)"
	ghServer502         = "gh: HTTP 502: Bad Gateway (https://api.github.com/user)"
)

func TestAmbientIdentityNoUsableHumanPasses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stderr string
		code   int
	}{
		{"gh not logged in (exit 4)", ghNotLoggedInStderr, 4},
		{"App/integration token answers 403", ghIntegration403, 1},
		{"credential answers 401", ghBadCreds401, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withRoster(t, goldenRoster())
			ghSays(t, "", tc.stderr, tc.code)
			rep, c := runRealAmbient(t, okProbes())
			if c.State != CheckedClean {
				t.Fatalf("%s: ambient-identity = %s, want checked-clean — no usable ambient human identity is the SAFE state (%s)",
					tc.name, c.State, c.Detail)
			}
			if err := rep.Err(); err != nil {
				t.Fatalf("%s: the envelope is red: %v", tc.name, err)
			}
			if c.Notice != "" {
				t.Fatalf("%s: no ambient human means nothing to warn about, got notice %q", tc.name, c.Notice)
			}
		})
	}
}

// The transport half is independent of the identity half and stays exactly as
// strict: with no ambient human, a helper chain that is not solely the App token
// is still checked-failed.
func TestAmbientIdentityNoUsableHumanStillChecksTransport(t *testing.T) {
	withRoster(t, goldenRoster())
	ghSays(t, "", ghIntegration403, 1)
	p := okProbes()
	p.CredHelperMatchesApp = func(Landing, string) (bool, string, error) {
		return false, "osxkeychain, not the App token", nil
	}
	_, c := runRealAmbient(t, p)
	if c.State != CheckedFailed {
		t.Fatalf("no-ambient-human + helper mismatch = %s, want checked-failed (%s)", c.State, c.Detail)
	}
}

func TestAmbientIdentityHumanIsNonBlockingWarning(t *testing.T) {
	for _, login := range []string{"mallory", fixtureBlessLogin} {
		t.Run(login, func(t *testing.T) {
			withRoster(t, goldenRoster())
			ghSays(t, login, "", 0)
			rep, c := runRealAmbient(t, okProbes())
			if !c.State.Passing() {
				t.Fatalf("human ambient %q = %s, want a passing state — a human login is a warning, never a blocker (%s)",
					login, c.State, c.Detail)
			}
			if err := rep.Err(); err != nil {
				t.Fatalf("human ambient %q reddened the envelope on its own: %v", login, err)
			}
			if !strings.Contains(c.Notice, "WARNING") || !strings.Contains(c.Notice, login) {
				t.Fatalf("human ambient %q: want a WARNING notice naming the login, got %q", login, c.Notice)
			}
			if !strings.Contains(c.Notice, "would act as") {
				t.Fatalf("human ambient %q: the warning must say a fall-through would act as that human: %q", login, c.Notice)
			}
			if !strings.Contains(rep.SummaryLine(), "NOTICE "+CheckAmbientID) {
				t.Fatalf("human ambient %q: the warning is not surfaced on the summary line: %q", login, rep.SummaryLine())
			}
			assertNoBlessLoginAdvice(t, c.Notice)
		})
	}
}

// A human warning does not mask a transport failure.
func TestAmbientIdentityHumanWarningDoesNotMaskTransport(t *testing.T) {
	withRoster(t, goldenRoster())
	ghSays(t, "mallory", "", 0)
	p := okProbes()
	p.CredHelperMatchesApp = func(Landing, string) (bool, string, error) {
		return false, "osxkeychain, not the App token", nil
	}
	_, c := runRealAmbient(t, p)
	if c.State != CheckedFailed {
		t.Fatalf("human ambient + helper mismatch = %s, want checked-failed (%s)", c.State, c.Detail)
	}
}

func TestAmbientIdentityBotSlugIsBlocking(t *testing.T) {
	for _, login := range []string{"assay-worker-app[bot]", "assay-worker-app"} {
		t.Run(login, func(t *testing.T) {
			withRoster(t, goldenRoster())
			ghSays(t, login, "", 0)
			rep, c := runRealAmbient(t, okProbes())
			if c.State != CheckedFailed {
				t.Fatalf("bot ambient %q = %s, want checked-failed (%s)", login, c.State, c.Detail)
			}
			if rep.Err() == nil {
				t.Fatalf("bot ambient %q did not redden the envelope", login)
			}
			assertNoBlessLoginAdvice(t, c.Remediation)
			if !strings.HasPrefix(strings.ToLower(c.Remediation), "clear the ambient") {
				t.Fatalf("bot remediation must lead with clearing the ambient credential: %q", c.Remediation)
			}
		})
	}
}

func TestAmbientIdentityCannotRunIsCouldNotCheck(t *testing.T) {
	t.Run("gh absent from PATH", func(t *testing.T) {
		withRoster(t, goldenRoster())
		t.Setenv("PATH", t.TempDir())
		_, c := runRealAmbient(t, okProbes())
		if c.State != CouldNotCheck {
			t.Fatalf("gh absent = %s, want could-not-check (%s)", c.State, c.Detail)
		}
		assertNoBlessLoginAdvice(t, c.Remediation)
	})
	t.Run("exec failure (gh cannot be started)", func(t *testing.T) {
		withRoster(t, goldenRoster())
		stubGH(t, "#!/nonexistent/interpreter", "")
		_, c := runRealAmbient(t, okProbes())
		if c.State != CouldNotCheck {
			t.Fatalf("exec failure = %s, want could-not-check (%s)", c.State, c.Detail)
		}
		assertNoBlessLoginAdvice(t, c.Remediation)
	})
	// A 403 that is a RATE LIMIT is not "the credential is not a user": a human
	// token can be rate-limited. It stays could-not-check — the 403 classification
	// is deliberately narrow.
	t.Run("rate-limit 403 is not a no-identity answer", func(t *testing.T) {
		withRoster(t, goldenRoster())
		ghSays(t, "", ghRateLimit403, 1)
		_, c := runRealAmbient(t, okProbes())
		if c.State != CouldNotCheck {
			t.Fatalf("rate-limit 403 = %s, want could-not-check (%s)", c.State, c.Detail)
		}
	})
	t.Run("server error is not a no-identity answer", func(t *testing.T) {
		withRoster(t, goldenRoster())
		ghSays(t, "", ghServer502, 1)
		_, c := runRealAmbient(t, okProbes())
		if c.State != CouldNotCheck {
			t.Fatalf("server error = %s, want could-not-check (%s)", c.State, c.Detail)
		}
	})
}

// assertNoBlessLoginAdvice pins case 5: no text the check emits recommends
// logging in as (or switching to) the blessing login.
func assertNoBlessLoginAdvice(t *testing.T, text string) {
	t.Helper()
	low := strings.ToLower(text)
	if strings.Contains(low, "in as "+fixtureBlessLogin) || strings.Contains(low, "to "+fixtureBlessLogin) ||
		strings.Contains(low, "login is "+fixtureBlessLogin) {
		t.Fatalf("text recommends the blessing login as the fix: %q", text)
	}
	if strings.Contains(low, "gh auth login") {
		t.Fatalf("text recommends logging a human in: %q", text)
	}
}

// The verdict no longer reads the blessing login — no ambient state depends on
// it being set — so a roster that names none must not turn the check into a
// blocker it cannot clear.
func TestAmbientIdentityNeedsNoBlessingLogin(t *testing.T) {
	r := goldenRoster()
	delete(r, EnvBlessLogin)
	withRoster(t, r)
	ghSays(t, "", ghNotLoggedInStderr, 4)
	_, c := runRealAmbient(t, okProbes())
	if c.State != CheckedClean {
		t.Fatalf("no blessing login configured + gh not logged in = %s, want checked-clean (%s)", c.State, c.Detail)
	}
}
