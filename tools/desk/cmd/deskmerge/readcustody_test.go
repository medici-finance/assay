package main

// readcustody_test.go — desktools-v2/03: deskmerge's two READS (the PR-state read and the R-5
// sign-off read) run on a minted, repo-scoped App token through the real forge resolver.
//
// Every other test in this package swaps forgeFor for a recorded stub, which is right for the
// git-world assertions and useless for these: the property under test IS the resolver path
// (forgeFor -> deskkit.ForgeFor -> the custody minter installed in forge.go -> the GitHub
// backend). So these tests keep the production forgeFor and point only the backend's API
// host at an httptest server.
//
// Two independent layers, one test each:
//
//   - TestReadRefusesUnmintedToken — the transport floor. An unresolvable role, a mint that
//     errors, or a mint that hands back an empty token is refused BEFORE any request leaves
//     the process, while GH_TOKEN / GITHUB_TOKEN hold a decoy. A planted ambient fallback
//     sends a request carrying the decoy and this test goes red.
//   - TestReadInstallationFromRepo — the identity floor. The token is minted for the repo
//     being READ, never for GH_REPO / the environment, and every request carries the minted
//     token. A planted env-derived installation mints for the decoy repo and this goes red.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	ambientDecoyToken = "ambient-decoy-token-0000"
	ambientDecoyRepo  = "example-org/decoy"
	mintedReadToken   = "minted-read-token-0000"
)

// readCustodyServer records every request the backend sends — path and Authorization — and
// answers the PR read with a minimal open, same-repo pull.
type readCustodyServer struct {
	mu    sync.Mutex
	paths []string
	auths []string
}

func (s *readCustodyServer) handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.paths = append(s.paths, r.Method+" "+r.URL.Path)
	s.auths = append(s.auths, r.Header.Get("Authorization"))
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/pulls/7") {
		_, _ = w.Write([]byte(`{"number":7,"state":"open","draft":false,"merged":false,` +
			`"head":{"ref":"feat/x","sha":"1111111111111111111111111111111111111111",` +
			`"repo":{"full_name":"medici-finance/assay"}},` +
			`"base":{"ref":"main","sha":"2222222222222222222222222222222222222222",` +
			`"repo":{"full_name":"medici-finance/assay"}},` +
			`"user":{"login":"someone","id":42}}`))
		return
	}
	http.Error(w, `{"message":"not served by this fixture"}`, http.StatusNotFound)
}

func (s *readCustodyServer) snapshot() (paths, auths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...), append([]string(nil), s.auths...)
}

// readCustodyEnv installs the production read path against srv, with the ambient decoys set.
// It returns nothing: the seams it swaps are restored by t.Cleanup.
func readCustodyEnv(t *testing.T, srv *httptest.Server) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir config home: %v", err)
	}
	// The fixture roster plus a forge entry for the repo under test, so the resolver answers
	// "github" from configuration rather than from whatever origin the test's cwd has.
	roster := fixtureRoster + "ASSAY_REPO_FORGES=" + testRepo + "=github," + ambientDecoyRepo + "=github\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	t.Setenv("HOME", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)

	// The ambient decoys: a present token in both variables gh and go-gh read, and a GH_REPO
	// naming a different repository. None of them may decide anything below.
	t.Setenv("GH_TOKEN", ambientDecoyToken)
	t.Setenv("GITHUB_TOKEN", ambientDecoyToken)
	t.Setenv("GH_REPO", ambientDecoyRepo)

	oldBase, oldMint, oldRole := forgeAPIBase, mintTokenFn, sessionRoleFn
	t.Cleanup(func() { forgeAPIBase, mintTokenFn, sessionRoleFn = oldBase, oldMint, oldRole })
	forgeAPIBase = srv.URL
	sessionRoleFn = func(string) (string, string, error) { return "worker", "worker-desk", nil }
}

func TestReadRefusesUnmintedToken(t *testing.T) {
	cases := []struct {
		name string
		role func(string) (string, string, error)
		mint func(role, repo string) (string, string, error)
	}{
		{
			name: "mint hands back an empty token",
			mint: func(string, string) (string, string, error) { return "", "", nil },
		},
		{
			name: "mint hands back a whitespace token",
			mint: func(string, string) (string, string, error) { return "  \n", "", nil },
		},
		{
			name: "mint errors (no App credential provisioned)",
			mint: func(string, string) (string, string, error) {
				return "", "", errors.New("desktoken: no installation for this role")
			},
		},
		{
			name: "session role unresolvable (no loop identity)",
			role: func(string) (string, string, error) {
				return "", "", deskkit.Unverifiable("no DESK_LOOP set", nil)
			},
			mint: func(string, string) (string, string, error) {
				t.Error("the mint was reached with no session role — the role refusal must come first")
				return mintedReadToken, "", nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &readCustodyServer{}
			srv := httptest.NewServer(http.HandlerFunc(rec.handler))
			t.Cleanup(srv.Close)
			readCustodyEnv(t, srv)
			mintTokenFn = tc.mint
			if tc.role != nil {
				sessionRoleFn = tc.role
			}

			_, perr := fetchPR(testRepo, testPR)
			if perr == nil {
				t.Fatal("the PR-state read succeeded with no minted token — it must refuse, never read as the ambient identity")
			}
			if code := deskkit.ExitCodeOf(perr); code != deskkit.ExitUnverifiable && code != deskkit.ExitRefused {
				t.Errorf("PR-state read refusal exit = %d, want could-not-check (%d) or refused (%d): %v",
					code, deskkit.ExitUnverifiable, deskkit.ExitRefused, perr)
			}
			_, cerr := fetchComment(signOffURL)
			if cerr == nil {
				t.Fatal("the sign-off read succeeded with no minted token — an unread authorization is not an authorization")
			}

			paths, auths := rec.snapshot()
			if len(paths) != 0 {
				t.Errorf("an unminted read still reached the forge %d time(s): %v — the refusal must precede any request", len(paths), paths)
			}
			for _, a := range auths {
				if strings.Contains(a, ambientDecoyToken) {
					t.Errorf("a request carried the AMBIENT token (%q) — the read fell back to the environment's identity", a)
				}
			}
		})
	}
}

func TestReadInstallationFromRepo(t *testing.T) {
	rec := &readCustodyServer{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	t.Cleanup(srv.Close)
	readCustodyEnv(t, srv)

	var mu sync.Mutex
	var minted []string
	mintTokenFn = func(role, repo string) (string, string, error) {
		mu.Lock()
		minted = append(minted, role+" "+repo)
		mu.Unlock()
		return mintedReadToken, "", nil
	}

	pr, err := fetchPR(testRepo, testPR)
	if err != nil {
		t.Fatalf("PR-state read with a minted token: %v", err)
	}
	if pr.Number != testPR || pr.HeadRefOid == "" || pr.CrossRepo != deskkit.CrossRepoSame {
		t.Errorf("PR-state read = %+v, want #%d with a head oid and a same-repo head", pr, testPR)
	}
	// The sign-off read: this fixture does not serve the comment thread, so the read comes
	// back could-not-check — what matters here is WHICH installation it asked for and WHICH
	// token it sent, not the thread.
	if _, cerr := fetchComment(signOffURL); cerr == nil {
		t.Fatal("the sign-off read returned a comment the fixture never served")
	}

	mu.Lock()
	gotMints := append([]string(nil), minted...)
	mu.Unlock()
	if len(gotMints) == 0 {
		t.Fatal("no token was minted — the reads did not go through the custody minter")
	}
	for _, m := range gotMints {
		if m != "worker "+testRepo {
			t.Errorf("minted %q, want %q — the installation must come from the repo being read, never from GH_REPO (%s) or the environment",
				m, "worker "+testRepo, ambientDecoyRepo)
		}
	}

	paths, auths := rec.snapshot()
	if len(paths) == 0 {
		t.Fatal("no request reached the forge — the test proves nothing about which token was sent")
	}
	for i, a := range auths {
		if strings.Contains(a, ambientDecoyToken) {
			t.Errorf("request %s carried the AMBIENT token — the environment decided the identity", paths[i])
		}
		if !strings.Contains(a, mintedReadToken) {
			t.Errorf("request %s carried Authorization %q, want the minted token", paths[i], a)
		}
	}
	for _, p := range paths {
		if strings.Contains(p, "/repos/") && !strings.Contains(p, "/repos/"+testRepo+"/") {
			t.Errorf("request %s left the repo being read (%s)", p, testRepo)
		}
	}
}
