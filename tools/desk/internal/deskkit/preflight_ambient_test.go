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
//  4. `gh` absent, or a probe that genuinely cannot run → could-not-check; so
//     is a "not logged in" answer with a stored credential still readable
//     through `gh auth token` (the empty-GH_CONFIG_DIR / OS-keyring shape).
//  5. no remediation recommends logging in as the blessing login; every one
//     leads with clearing the ambient credential for desk shells.
//
// FAIL-FIRST: the cases that changed behaviour (1, 2, 5, and the no-blessing-
// login case) are red on the pre-#1798 tree; the preserved cases (3, 4) are
// proven load-bearing by internal/deskkit/preflight-ambient-mutations.json
// (`go run ./cmd/muhar -spec internal/deskkit/preflight-ambient-mutations.json`
// from tools/desk), which also flips each no-identity classification — the
// 403, the not-logged-in, and the 401 — back to could-not-check and requires
// this suite to catch it.

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
	if strings.Contains(low, "in as "+fixtureBlessLogin) || strings.Contains(low, "identity to "+fixtureBlessLogin) ||
		strings.Contains(low, "confirm the ambient login is "+fixtureBlessLogin) {
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

// TestAmbientLoginProbeClassifiesErrorShapes pins the probe itself: which failed
// `gh api user` answers are "no usable ambient human identity" (wrapping
// ErrNoAmbientIdentity) and which are a plain could-not-look error.
func TestAmbientLoginProbeClassifiesErrorShapes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stderr     string
		code       int
		noIdentity bool
	}{
		{"not logged in: exit 4 + login hint", ghNotLoggedInStderr, 4, true},
		{"not logged in: exit 4, no stderr", "", 4, true},
		{"integration token: HTTP 403", ghIntegration403, 1, true},
		{"bad credentials: HTTP 401", ghBadCreds401, 1, true},
		{"rate-limit HTTP 403", ghRateLimit403, 1, false},
		{"server error", ghServer502, 1, false},
		{"unexplained non-zero exit", "gh: something else went wrong", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ghSays(t, "", tc.stderr, tc.code)
			login, err := ambientLoginProbe()
			if err == nil || login != "" {
				t.Fatalf("probe = (%q, %v), want an error", login, err)
			}
			if got := errors.Is(err, ErrNoAmbientIdentity); got != tc.noIdentity {
				t.Fatalf("errors.Is(ErrNoAmbientIdentity) = %v, want %v (err: %v)", got, tc.noIdentity, err)
			}
		})
	}
	t.Run("a login on stdout", func(t *testing.T) {
		ghSays(t, "mallory", "", 0)
		if login, err := ambientLoginProbe(); err != nil || login != "mallory" {
			t.Fatalf("probe = (%q, %v), want (mallory, nil)", login, err)
		}
	})
	t.Run("exec failure is not a no-identity answer", func(t *testing.T) {
		stubGH(t, "#!/nonexistent/interpreter", "")
		_, err := ambientLoginProbe()
		if err == nil || errors.Is(err, ErrNoAmbientIdentity) {
			t.Fatalf("exec failure = %v, want a plain could-not-look error", err)
		}
	})
}

// TestAmbientIdentityTimeoutIsCouldNotCheck — a `gh` that never answers is a
// probe that could not look, never a no-identity answer.
func TestAmbientIdentityTimeoutIsCouldNotCheck(t *testing.T) {
	withRoster(t, goldenRoster())
	stubGH(t, "#!/bin/sh", "/bin/sleep 5\nexit 4")
	old := ambientProbeTimeout
	ambientProbeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { ambientProbeTimeout = old })
	_, c := runRealAmbient(t, okProbes())
	if c.State != CouldNotCheck {
		t.Fatalf("timed-out probe = %s, want could-not-check (%s)", c.State, c.Detail)
	}
	if !strings.Contains(c.Detail, "did not answer") {
		t.Fatalf("the timeout should say so: %q", c.Detail)
	}
}

// ghSplit stubs a `gh` that answers `gh api user` and `gh auth token` with
// separate scripts, so a test can model a gh that reports "not logged in" for
// the API call while a stored credential is still readable locally.
func ghSplit(t *testing.T, apiUser, authToken string) {
	t.Helper()
	stubGH(t, "#!/bin/sh", "case \"$1 $2\" in\n"+
		"'api user') "+apiUser+" ;;\n"+
		"'auth token') "+authToken+" ;;\n"+
		"*) echo \"unexpected gh call: $*\" >&2; exit 2 ;;\n"+
		"esac")
}

// stubStoredCredential is what the stub prints for `gh auth token` when a
// stored (OS-keyring) credential is readable. It is not a real credential.
const stubStoredCredential = "stub-stored-credential-value"

const (
	ghAPINotLoggedIn   = "printf '%s\\n' '" + ghNotLoggedInStderr + "' >&2; exit 4"
	ghAuthTokenStored  = "printf '%s\\n' '" + stubStoredCredential + "'; exit 0"
	ghAuthTokenMissing = "printf '%s\\n' 'no oauth token found for github.com' >&2; exit 1"
)

// A `gh` that says "not logged in" for `gh api user` while `gh auth token` still
// returns a stored credential is NOT the no-identity state. That is the shape an
// empty GH_CONFIG_DIR gives on a machine whose gh login lives in the OS keyring:
// the API call finds no config, but the keyring credential is one local call
// away, and a wrapper that resolves `gh auth token` into GH_TOKEN hands it
// straight back to the next `gh`. The check cannot tell whose credential it is
// without using it, so it reports could-not-check and never a clean pass, and it
// never echoes the credential.
func TestAmbientIdentityStoredCredentialBehindNotLoggedInIsNotClean(t *testing.T) {
	withRoster(t, goldenRoster())
	ghSplit(t, ghAPINotLoggedIn, ghAuthTokenStored)
	rep, c := runRealAmbient(t, okProbes())
	if c.State != CouldNotCheck {
		t.Fatalf("not-logged-in API call + readable stored credential = %s, want could-not-check (%s)", c.State, c.Detail)
	}
	if rep.Err() == nil {
		t.Fatalf("a reachable stored credential must not leave the envelope green")
	}
	if !strings.Contains(c.Detail, "gh auth token") {
		t.Fatalf("the detail should name the local call that still yields a credential: %q", c.Detail)
	}
	for _, text := range []string{c.Detail, c.Remediation, c.Notice, rep.SummaryLine()} {
		if strings.Contains(text, stubStoredCredential) {
			t.Fatalf("the check echoed the stored credential: %q", text)
		}
	}
	if !strings.HasPrefix(strings.ToLower(c.Remediation), "clear the ambient") {
		t.Fatalf("remediation must lead with clearing the ambient credential: %q", c.Remediation)
	}
	assertNoBlessLoginAdvice(t, c.Remediation)
}

// The same not-logged-in answer with NO stored credential behind it stays the
// safe state: the stored-credential look must not turn every not-logged-in gh
// into could-not-check.
func TestAmbientIdentityNotLoggedInWithNoStoredCredentialPasses(t *testing.T) {
	withRoster(t, goldenRoster())
	ghSplit(t, ghAPINotLoggedIn, ghAuthTokenMissing)
	_, c := runRealAmbient(t, okProbes())
	if c.State != CheckedClean {
		t.Fatalf("not logged in + no stored credential = %s, want checked-clean (%s)", c.State, c.Detail)
	}
}

// A `gh auth token` that never answers has not shown there is no stored
// credential, so it is could-not-check, never a clean pass.
func TestAmbientIdentityStoredCredentialLookTimeoutIsCouldNotCheck(t *testing.T) {
	withRoster(t, goldenRoster())
	// The budget is long enough that the immediate `gh api user` answer always
	// lands inside it, even on a loaded runner, so only the `gh auth token` call
	// can time out — the detail assertion below pins which call it was.
	ghSplit(t, ghAPINotLoggedIn, "/bin/sleep 8; exit 1")
	old := ambientProbeTimeout
	ambientProbeTimeout = 2 * time.Second
	t.Cleanup(func() { ambientProbeTimeout = old })
	_, c := runRealAmbient(t, okProbes())
	if c.State != CouldNotCheck {
		t.Fatalf("timed-out stored-credential look = %s, want could-not-check (%s)", c.State, c.Detail)
	}
	if !strings.Contains(c.Detail, "gh auth token did not answer") {
		t.Fatalf("the timeout should name the stored-credential look: %q", c.Detail)
	}
}

// The probe-level pin for the same states: a readable stored credential behind
// "not logged in" is ErrStoredAmbientCredential, never ErrNoAmbientIdentity.
func TestAmbientLoginProbeStoredCredential(t *testing.T) {
	t.Run("stored credential readable", func(t *testing.T) {
		ghSplit(t, ghAPINotLoggedIn, ghAuthTokenStored)
		_, err := ambientLoginProbe()
		if !errors.Is(err, ErrStoredAmbientCredential) || errors.Is(err, ErrNoAmbientIdentity) {
			t.Fatalf("probe err = %v, want ErrStoredAmbientCredential and not ErrNoAmbientIdentity", err)
		}
		if strings.Contains(err.Error(), stubStoredCredential) {
			t.Fatalf("the probe error echoes the stored credential: %v", err)
		}
	})
	t.Run("no stored credential", func(t *testing.T) {
		ghSplit(t, ghAPINotLoggedIn, ghAuthTokenMissing)
		if _, err := ambientLoginProbe(); !errors.Is(err, ErrNoAmbientIdentity) {
			t.Fatalf("probe err = %v, want ErrNoAmbientIdentity", err)
		}
	})
	t.Run("empty stored-credential answer", func(t *testing.T) {
		ghSplit(t, ghAPINotLoggedIn, "exit 0")
		if _, err := ambientLoginProbe(); !errors.Is(err, ErrNoAmbientIdentity) {
			t.Fatalf("probe err = %v, want ErrNoAmbientIdentity", err)
		}
	})
}
