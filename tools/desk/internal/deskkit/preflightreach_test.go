package deskkit

// Goldens for the preflight write-transport check's reachability verdict, which is
// now an authenticated in-process gitcore.List of the landing repo. Every remote here
// is LOCAL: a bare repository on disk (reachable) or an httptest server answering a
// fixed status (rejected / inconclusive). Nothing dials a real forge.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// reachBare builds a bare repository holding one commit on main and returns its path.
func reachBare(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; the local fixture remote needs it to be built")
	}
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	bare := filepath.Join(dir, "remote.git")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", work},
		{"-C", work, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "seed"},
		{"clone", "-q", "--bare", work, bare},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return bare
}

// statusServer answers every request with code, so a List against it fails the way a
// forge denying this identity fails.
func statusServer(t *testing.T, code int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/example-org/tracker.git"
}

// TestPreflightReachabilityLocal — a reachable remote answers the List: PERMITTED.
func TestPreflightReachabilityLocal(t *testing.T) {
	v, detail, err := listReachability(gitcore.ListOpts{URL: reachBare(t)})
	if err != nil || v != ProbePermitted {
		t.Fatalf("List of a reachable local remote = (%v, %q, %v), want ProbePermitted", v, detail, err)
	}
	if !strings.Contains(detail, "refs/heads/main") {
		t.Fatalf("permitted detail %q does not name the advertised ref it saw", detail)
	}
}

// TestPreflightReachability401 / 403 — a forge that refuses this identity is a
// REJECTION (a STOP), never inconclusive and never a pass.
func TestPreflightReachability401(t *testing.T) {
	assertReach(t, statusServer(t, http.StatusUnauthorized), ProbeRejected)
}

func TestPreflightReachability403(t *testing.T) {
	assertReach(t, statusServer(t, http.StatusForbidden), ProbeRejected)
}

// TestPreflightReachability503 — a transport failure that is not a denial is
// could-not-check: no verdict was reached.
func TestPreflightReachability503(t *testing.T) {
	assertReach(t, statusServer(t, http.StatusServiceUnavailable), ProbeInconclusive)
}

func assertReach(t *testing.T, url string, want ProbeVerdict) {
	t.Helper()
	v, detail, err := listReachability(gitcore.ListOpts{URL: url, Auth: gitcore.BasicAuth("fixture-token")})
	if err != nil {
		t.Fatalf("listReachability returned an error (%v); a transport answer is a verdict, not an error", err)
	}
	if v != want {
		t.Fatalf("List of %s = %v (%q), want %v", url, v, detail, want)
	}
	if strings.Contains(detail, "fixture-token") {
		t.Fatalf("the verdict detail leaks the credential: %q", detail)
	}
}

// withEndpoint points the default probe's endpoint resolution at a fixture for one test.
func withEndpoint(t *testing.T, fn func(repo, role, origin string) (ForgeGitEndpoint, error)) {
	t.Helper()
	old := forgeGitEndpointFn
	forgeGitEndpointFn = fn
	t.Cleanup(func() { forgeGitEndpointFn = old })
}

// TestPreflightReachabilityCaller is the NEIGHBOUR row: the preflight CALLER
// (PreflightRequest.Run → checkWriteTransport) with the DEFAULT probe left in place —
// only the endpoint is pointed at a local fixture — still turns the List verdict into
// the reachable / unreachable check state its own callers read, and hands the probe
// the role and repo the pass lands as.
func TestPreflightReachabilityCaller(t *testing.T) {
	withRoster(t, goldenRoster())
	bare := reachBare(t)
	cases := []struct {
		name string
		url  string
		want CheckState
	}{
		{"reachable", bare, CheckedClean},
		{"denied", statusServer(t, http.StatusForbidden), CheckedFailed},
		{"unreachable", statusServer(t, http.StatusBadGateway), CouldNotCheck},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotRepo, gotRole string
			withEndpoint(t, func(repo, role, _ string) (ForgeGitEndpoint, error) {
				gotRepo, gotRole = repo, role
				return ForgeGitEndpoint{Kind: ForgeGitHub, Opts: gitcore.ListOpts{URL: tc.url, Auth: gitcore.BasicAuth("t")}}, nil
			})
			p := okProbes()
			p.WriteTransport = nil // the real default probe runs
			rep := PreflightRequest{Role: pfRole, Root: t.TempDir(), Repo: "example-org/tracker", Probes: p}.Run()
			c := pfCheck(t, rep, CheckWriteTransport)
			if c.State != tc.want {
				t.Fatalf("%s = %s (%s), want %s", c.Name, c.State, c.Detail, tc.want)
			}
			if gotRepo != "example-org/tracker" || gotRole != pfRole {
				t.Fatalf("probe resolved the endpoint for (%q, %q), want (example-org/tracker, %s)", gotRepo, gotRole, pfRole)
			}
		})
	}
}

// TestPreflightReachabilityNoEp — no endpoint (unnamed forge, no credential) is
// could-not-check at the caller, never a pass and never a rejection.
func TestPreflightReachabilityNoEp(t *testing.T) {
	withRoster(t, goldenRoster())
	withEndpoint(t, func(string, string, string) (ForgeGitEndpoint, error) {
		return ForgeGitEndpoint{}, errors.New("forge not named in the roster")
	})
	p := okProbes()
	p.WriteTransport = nil
	rep := PreflightRequest{Role: pfRole, Root: t.TempDir(), Repo: "example-org/tracker", Probes: p}.Run()
	if got := pfCheck(t, rep, CheckWriteTransport).State; got != CouldNotCheck {
		t.Fatalf("%s = %s with no endpoint, want could-not-check", CheckWriteTransport, got)
	}
}
