package main

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// mintorder_test.go — the landing mints its token BEFORE pre-work admission reads the forge.
//
// Admission reads the dispatcher's attestation record from the forge, and this tool's custody
// step refuses to hand the forge any credential until the verifier token has been minted. A
// landing that runs admission first therefore refuses on every call, whatever it was asked to
// land. No other test saw that: each one either stubs admission, or replaces the custody step
// with one that always has a token. The tests here keep BOTH real.

// fakeVerifierToken is the only credential a landing may present. It is what the stubbed
// `desktoken` mint hands back, and the only seam through which a landing obtains one.
const fakeVerifierToken = "fake-verifier-token"

// fixture repository and the brief the fixture run is attested for.
const (
	fixtureRepo  = "example-org/tracker"
	fixtureBrief = "docs/streams/x/brief-01-source.md"
	fixtureIndex = "docs/streams/x/README.md"
	fixturePlan  = "# Source\n\n## Verify\n\n| 1 | `true` | exit 0 |\n\n## Evidence\n\nPending.\n"
)

// landingFixture is a dispatched, attested verifier home plus the offline forge that holds its
// attestation record and the fixture repository's branch content.
type landingFixture struct {
	api     *attestationAPI
	home    string
	receipt deskkit.VerifierReceipt
	errBuf  *bytes.Buffer
	mints   int
	mintErr error
}

// newLandingFixture builds the fixture with every part of the landing's identity path REAL:
// the shipped admission binding, the shared admission reader, the real forge resolver and this
// tool's own custody step. Exactly two things are faked, both at the outer edge — the
// `desktoken` mint (a counting stub that sets the fixed fake token) and the API host (an
// offline server, through the test-only base override).
func newLandingFixture(t *testing.T) *landingFixture {
	t.Helper()
	_, errBuf := setupFake(t)
	lf := &landingFixture{api: &attestationAPI{}, errBuf: errBuf}
	srv := httptest.NewServer(lf.api)
	t.Cleanup(srv.Close)

	// Forge selection belongs to the fixture, not the enclosing checkout's origin.
	pinFixtureForge(t, os.Getenv("HOME"), fixtureRepo)

	lf.home = t.TempDir()
	lf.git(t, "init", "-q")
	lf.git(t, "config", "user.name", "fixture")
	lf.git(t, "config", "user.email", "fixture@example.invalid")
	writeFixtureFile(t, filepath.Join(lf.home, fixtureBrief), fixturePlan)
	writeFixtureFile(t, filepath.Join(lf.home, fixtureIndex), "# x\n")
	writeFixtureFile(t, filepath.Join(lf.home, "source.txt"), "attested source\n")
	lf.git(t, "add", fixtureBrief, fixtureIndex, "source.txt")
	lf.git(t, "commit", "-q", "-m", "fixture")
	lf.git(t, "update-ref", "refs/remotes/origin/main", "HEAD")
	lf.git(t, "checkout", "-q", "--detach")

	// The dispatcher attests the run in its OWN process, under its own custody, before any
	// landing exists. That stand-in custody is installed for this set-up step only; the
	// cleanup and the line after the attestation both put this tool's real custody step back,
	// so no later test in the package is left on a custody step other than the shipped one.
	deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) {
		return "fake-dispatcher-token", srv.URL, nil
	})
	t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(githubCustodyMint) })
	if err := deskkit.PrepareVerifierAttestation(lf.home, fixtureRepo, fixtureBrief, "gpt-6-astra", "strong"); err != nil {
		t.Fatal(err)
	}
	receipt, err := deskkit.RecoverVerifierAttestation(lf.home)
	if err != nil {
		t.Fatalf("dispatcher attestation: %v (unexpected API calls %v)", err, lf.api.unexpected)
	}
	lf.receipt = receipt
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)

	// From here on the landing runs as shipped.
	verifierEvidenceAdmissionFn = productionAdmission
	forgeForFn = forgeFor
	oldBase := forgeAPIBase
	forgeAPIBase = srv.URL
	t.Cleanup(func() { forgeAPIBase = oldBase })
	mintTokenFn = func(string) error {
		lf.mints++
		if lf.mintErr != nil {
			return lf.mintErr
		}
		ghToken = fakeVerifierToken
		return nil
	}

	lf.api.files = map[string]string{fixtureBrief: fixturePlan}
	return lf
}

func (lf *landingFixture) git(t *testing.T, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", lf.home}, args...)...)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s %v", args, out, err)
	}
}

func writeFixtureFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// land runs one landing as a FRESH process would: with no token minted yet. Carrying the
// previous landing's token over is exactly what would hide a mint that happens too late, so
// every landing starts from the empty state the shipped binary starts from. It returns the
// exit code and the requests that landing made.
func (lf *landingFixture) land(t *testing.T, args ...string) (int, []apiCall) {
	t.Helper()
	ghToken = ""
	lf.errBuf.Reset()
	lf.mints = 0
	before := len(lf.api.calls)
	code := run(append([]string{fixtureRepo, "main"}, args...))
	return code, append([]apiCall(nil), lf.api.calls[before:]...)
}

// assertOneMintedIdentity fails unless every request carried the minted verifier token.
func assertOneMintedIdentity(t *testing.T, calls []apiCall) {
	t.Helper()
	if len(calls) == 0 {
		t.Fatal("the landing made no forge request at all")
	}
	for _, c := range calls {
		if c.Authorization != "token "+fakeVerifierToken {
			t.Fatalf("%s %s reached the forge as %q, not as the minted verifier token", c.Method, c.Path, c.Authorization)
		}
	}
}

// assertAdmissionReadsFirst fails unless the admission record was read, and read before the
// landing's first Contents-API request — the remote read every later step takes as input.
func assertAdmissionReadsFirst(t *testing.T, calls []apiCall) {
	t.Helper()
	record, contents := -1, -1
	for i, c := range calls {
		if record < 0 && strings.HasSuffix(c.Path, "/issues/77") {
			record = i
		}
		if contents < 0 && strings.Contains(c.Path, contentsPrefix) {
			contents = i
		}
	}
	if record < 0 {
		t.Fatalf("admission never read the attestation record: %+v", calls)
	}
	if contents >= 0 && contents < record {
		t.Fatalf("the landing read the branch (request %d) before admission read its record (request %d): %+v", contents, record, calls)
	}
}

func contentsCalls(calls []apiCall) (n int) {
	for _, c := range calls {
		if strings.Contains(c.Path, contentsPrefix) {
			n++
		}
	}
	return n
}

// TestLandingMintsBeforeAdmission lands Evidence and an outcome record through the real
// admission and the real custody step, each landing starting with no token minted.
//
// Red before the fix: admission ran ahead of the mint, so its forge read hit the custody
// step's empty-token refusal and EVERY landing — dry-run or not — exited 5 with "refusing to
// reach the forge with no minted verifier token".
func TestLandingMintsBeforeAdmission(t *testing.T) {
	lf := newLandingFixture(t)
	// The desk writes the verifier's returned rows to its own scratch, outside the home.
	fragment := filepath.Join(t.TempDir(), "evidence.md")
	writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
	evidence := []string{"--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment}

	t.Run("dry-run-writes-nothing", func(t *testing.T) {
		code, calls := lf.land(t, append(evidence, "--dry-run")...)
		if code != deskkit.ExitOK {
			t.Fatalf("dry-run landing exit = %d: %s", code, lf.errBuf)
		}
		if lf.mints != 1 {
			t.Fatalf("dry-run minted %d time(s), want exactly 1", lf.mints)
		}
		assertOneMintedIdentity(t, calls)
		assertAdmissionReadsFirst(t, calls)
		if contentsCalls(calls) == 0 {
			t.Fatalf("dry-run never read the branch, so it exercised no remote gate: %+v", calls)
		}
		if len(lf.api.puts) != 0 {
			t.Fatalf("dry-run wrote: %+v", lf.api.puts)
		}
	})

	t.Run("evidence-lands", func(t *testing.T) {
		code, calls := lf.land(t, evidence...)
		if code != deskkit.ExitOK {
			t.Fatalf("Evidence landing exit = %d: %s", code, lf.errBuf)
		}
		if lf.mints != 1 {
			t.Fatalf("landing minted %d time(s), want exactly 1", lf.mints)
		}
		assertOneMintedIdentity(t, calls)
		assertAdmissionReadsFirst(t, calls)
		if len(lf.api.puts) != 1 || lf.api.puts[0].File != fixtureBrief ||
			!strings.Contains(lf.api.puts[0].Message, lf.receipt.EvidenceBinding()) {
			t.Fatalf("landing lost its target or run binding: %+v", lf.api.puts)
		}
	})

	t.Run("outcome-record-lands", func(t *testing.T) {
		// The documented refresh: the landed brief replaces the home's copy; HEAD stays.
		writeFixtureFile(t, filepath.Join(lf.home, fixtureBrief), lf.api.files[fixtureBrief])
		record := filepath.Join(t.TempDir(), "outcome.json")
		writeFixtureFile(t, record, `{"brief":"x/01","ts":"2026-10-06T00:00:00Z","verdict":"verify-fail","digest":"0123456789abcdef"}`)
		before := len(lf.api.puts)
		code, calls := lf.land(t, "--root", lf.home, "--outcome-record", record)
		if code != deskkit.ExitOK {
			t.Fatalf("outcome-record landing exit = %d: %s", code, lf.errBuf)
		}
		if lf.mints != 1 {
			t.Fatalf("landing minted %d time(s), want exactly 1", lf.mints)
		}
		assertOneMintedIdentity(t, calls)
		assertAdmissionReadsFirst(t, calls)
		if len(lf.api.puts) != before+1 ||
			!strings.HasPrefix(lf.api.puts[before].File, "docs/streams/verify-outcomes/x/") ||
			!strings.Contains(lf.api.puts[before].Message, lf.receipt.EvidenceBinding()) {
			t.Fatalf("outcome record lost its target or run binding: %+v", lf.api.puts[before:])
		}
	})

	if len(lf.api.unexpected) != 0 {
		t.Fatalf("unexpected forge calls: %v", lf.api.unexpected)
	}
}

// TestAdmissionStillPrecedesEveryLandingRead pins what moving the mint must NOT change:
// admission is still the first thing that touches the forge, and a landing it refuses reads
// nothing from the branch, does not read its local evidence file, and writes nothing — whether
// admission refuses on what it read from the forge or on the home alone.
func TestAdmissionStillPrecedesEveryLandingRead(t *testing.T) {
	lf := newLandingFixture(t)
	fragment := filepath.Join(t.TempDir(), "evidence.md")
	writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
	record := filepath.Join(t.TempDir(), "outcome.json")
	writeFixtureFile(t, record, `{"brief":"x/01","ts":"2026-10-06T00:00:00Z","verdict":"verify-fail","digest":"0123456789abcdef"}`)
	shapes := map[string][]string{
		"evidence":       {"--brief-path", fixtureBrief, "--evidence-file", fragment},
		"outcome-record": {"--outcome-record", record},
	}

	for name, shape := range shapes {
		t.Run(name+"/record-altered-on-the-forge", func(t *testing.T) {
			lf.api.body += "\naltered after dispatch"
			defer func() { lf.api.body = strings.TrimSuffix(lf.api.body, "\naltered after dispatch") }()
			code, calls := lf.land(t, append([]string{"--root", lf.home}, shape...)...)
			if code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d: %s", code, deskkit.ExitRefused, lf.errBuf)
			}
			if !strings.Contains(lf.errBuf.String(), "not the exact dispatcher-authored run record") {
				t.Fatalf("refused for another reason: %s", lf.errBuf)
			}
			// The refusal came from admission's own forge read, made as the minted verifier.
			assertOneMintedIdentity(t, calls)
			if n := contentsCalls(calls); n != 0 {
				t.Fatalf("a landing admission refused still made %d branch request(s): %+v", n, calls)
			}
		})
		t.Run(name+"/root-is-not-the-attested-home", func(t *testing.T) {
			code, calls := lf.land(t, append([]string{"--root", t.TempDir()}, shape...)...)
			if code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d: %s", code, deskkit.ExitRefused, lf.errBuf)
			}
			if !strings.Contains(lf.errBuf.String(), "--root naming the dispatched verifier home") {
				t.Fatalf("refused for another reason: %s", lf.errBuf)
			}
			if len(calls) != 0 {
				t.Fatalf("a landing with no attestation at --root reached the forge: %+v", calls)
			}
			// Admission runs as a whole after the mint (the declared desk-decided cost), so a
			// landing it refuses on the home alone has already minted, exactly once.
			if lf.mints != 1 {
				t.Fatalf("a landing refused on the home alone minted %d time(s), want 1", lf.mints)
			}
		})
		t.Run(name+"/mint-fails", func(t *testing.T) {
			lf.mintErr = deskkit.Unverifiable("desktoken verifier --repo "+fixtureRepo+": mint refused", errors.New("mint refused"))
			defer func() { lf.mintErr = nil }()
			code, calls := lf.land(t, append([]string{"--root", lf.home}, shape...)...)
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("exit = %d, want %d: %s", code, deskkit.ExitUnverifiable, lf.errBuf)
			}
			if len(calls) != 0 {
				t.Fatalf("a landing whose mint failed reached the forge: %+v", calls)
			}
		})
	}

	// Admission also precedes the landing's LOCAL read of the evidence file: a fragment that
	// does not exist is never opened when admission refuses. Run the local read first and this
	// landing exits on the unreadable file instead of on admission's refusal.
	t.Run("evidence/admission-before-local-read", func(t *testing.T) {
		lf.api.body += "\naltered after dispatch"
		defer func() { lf.api.body = strings.TrimSuffix(lf.api.body, "\naltered after dispatch") }()
		missing := filepath.Join(t.TempDir(), "never-written.md")
		code, calls := lf.land(t, "--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", missing)
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d: %s", code, deskkit.ExitRefused, lf.errBuf)
		}
		if !strings.Contains(lf.errBuf.String(), "not the exact dispatcher-authored run record") {
			t.Fatalf("refused for another reason — the local read ran before admission: %s", lf.errBuf)
		}
		if n := contentsCalls(calls); n != 0 {
			t.Fatalf("a landing admission refused still made %d branch request(s): %+v", n, calls)
		}
	})

	if len(lf.api.puts) != 0 {
		t.Fatalf("a refused landing wrote: %+v", lf.api.puts)
	}
	for _, c := range lf.api.calls {
		if c.Authorization == "" {
			t.Fatalf("%s %s reached the forge with no credential", c.Method, c.Path)
		}
	}
	if len(lf.api.unexpected) != 0 {
		t.Fatalf("unexpected forge calls: %v", lf.api.unexpected)
	}
}

// TestLandingUsesOneForgeForAdmissionAndWrite pins the shape of the fix at the seams, for
// both landing shapes: the token is minted once, the forge is resolved once, admission is
// handed THAT forge, and admission returns before the landing's first branch read.
func TestLandingUsesOneForgeForAdmissionAndWrite(t *testing.T) {
	rec := `{"brief":"x/01","ts":"2026-10-06T00:00:00Z","verdict":"verify-fail","digest":"0123456789abcdef"}`
	for name, args := range map[string][]string{
		"evidence":       {"--evidence-file", "docs/streams/x/brief.md"},
		"outcome-record": {"--outcome-record", "record.json"},
	} {
		t.Run(name, func(t *testing.T) {
			f, errBuf := setupFake(t)
			var steps []string
			var resolved, admitted deskkit.Forge
			mintTokenFn = func(string) error {
				steps = append(steps, "mint")
				ghToken = fakeVerifierToken
				return nil
			}
			forgeForFn = func(owner, name string) (deskkit.Forge, deskkit.ForgeRepo, error) {
				steps = append(steps, "forge")
				resolved = deskkit.OutboundChecked(f, "verifier")
				return resolved, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
			}
			verifierEvidenceAdmissionFn = func(_, _, _ string, fg deskkit.Forge) (deskkit.VerifierReceipt, error) {
				steps = append(steps, "admission")
				admitted = fg
				if len(f.hits) != 0 {
					t.Errorf("the landing reached the forge before admission: %v", f.hits)
				}
				return deskkit.VerifierReceipt{}, nil
			}
			root := rootWithFile(t, "docs/streams/x/brief.md", "# Brief\n\n## Evidence\n| 1 | fixture | 0 | ok |\n")
			writeFixtureFile(t, filepath.Join(root, "record.json"), rec)
			f.setFile("docs/streams/x/brief.md", "# Brief\n\n## Evidence\n")

			if code := run(append([]string{fixtureRepo, "main", "--root", root}, args...)); code != deskkit.ExitOK {
				t.Fatalf("landing exit = %d: %s", code, errBuf)
			}
			if got := strings.Join(steps, " "); got != "mint forge admission" {
				t.Fatalf("landing ran %q, want exactly one mint, one forge resolution, then admission", got)
			}
			if admitted == nil || admitted != resolved {
				t.Fatal("admission was not handed the forge the landing resolved")
			}
			if f.putCalls != 1 {
				t.Fatalf("landing made %d write(s), want 1", f.putCalls)
			}
		})
	}
}

// TestPreMintRefusalsNeverMint pins the other side of the line admitLanding draws: every
// refusal that needs only the command line — or, for an outcome record, the record's own
// bytes — fires before the mint, so a call those refuse never mints a token, never resolves
// a forge and never reaches admission. Moving the mint above any one of them turns its
// subtest red.
func TestPreMintRefusalsNeverMint(t *testing.T) {
	dir := t.TempDir()
	badJSON := filepath.Join(dir, "bad.json")
	writeFixtureFile(t, badJSON, "{not json")
	future := filepath.Join(dir, "future.json")
	writeFixtureFile(t, future, `{"brief":"x/01","ts":"2999-01-01T00:00:00Z","verdict":"verify-fail","digest":"0123456789abcdef"}`)
	unnamed := filepath.Join(dir, "unnamed.json")
	writeFixtureFile(t, unnamed, `{"brief":"../01","ts":"2026-10-06T00:00:00Z","verdict":"verify-fail","digest":"0123456789abcdef"}`)

	cases := map[string]struct {
		args   []string
		mainOK string
		want   string // the refusal this case must hit, so it cannot pass by refusing early
	}{
		"repo-not-owner-name": {args: []string{"not-a-repo", "work", "--evidence-file", "docs/streams/x/a.md"}, want: "repo must be owner/name"},
		"repo-not-in-set":     {args: []string{"random-org/random-repo", "work", "--evidence-file", "docs/streams/x/a.md"}, want: "not in the desk-tools repo set"},
		"main-unsanctioned":   {args: []string{fixtureRepo, "main", "--evidence-file", "docs/streams/x/a.md"}, mainOK: "0", want: "is human-gated"},
		"bad-flag":            {args: []string{fixtureRepo, "work", "--no-such-flag"}, want: "bad flags"},
		"record-and-evidence": {args: []string{fixtureRepo, "work", "--outcome-record", badJSON, "--evidence-file", "docs/streams/x/a.md"}, want: "mutually exclusive"},
		"no-evidence-file":    {args: []string{fixtureRepo, "work"}, want: "--evidence-file is required"},
		"generated-status":    {args: []string{fixtureRepo, "work", "--evidence-file", "STATUS.md"}, want: "is generated"},
		"appended-log":        {args: []string{fixtureRepo, "work", "--evidence-file", "docs/streams/verify-outcomes.jsonl"}, want: "shared appended"},
		"record-not-json":     {args: []string{fixtureRepo, "work", "--outcome-record", badJSON}, want: "invalid --outcome-record JSON"},
		"record-future-ts":    {args: []string{fixtureRepo, "work", "--outcome-record", future}, want: "ahead of now"},
		"record-unnameable":   {args: []string{fixtureRepo, "work", "--outcome-record", unnamed}, want: "cannot name --outcome-record"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, errBuf := setupFake(t)
			if tc.mainOK != "" {
				t.Setenv("VERIFIER_MAIN_OK", tc.mainOK)
			}
			var steps []string
			mintTokenFn = func(string) error {
				steps = append(steps, "mint")
				ghToken = fakeVerifierToken
				return nil
			}
			forgeForFn = func(string, string) (deskkit.Forge, deskkit.ForgeRepo, error) {
				steps = append(steps, "forge")
				return nil, deskkit.ForgeRepo{}, errors.New("no forge in this test")
			}
			verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) {
				steps = append(steps, "admission")
				return deskkit.VerifierReceipt{}, nil
			}
			if code := run(tc.args); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d: %s", code, deskkit.ExitRefused, errBuf)
			}
			if len(steps) != 0 {
				t.Fatalf("a call refused on its command line alone still ran %v: %s", steps, errBuf)
			}
			if !strings.Contains(errBuf.String(), tc.want) {
				t.Fatalf("refused for another reason, want %q: %s", tc.want, errBuf)
			}
		})
	}
}

// TestAdmissionRefusesWithNoResolvedForge: admission handed no forge refuses outright. It
// must never pass a nil on to the shared reader, which would resolve a forge of its own —
// outside the landing's mint.
func TestAdmissionRefusesWithNoResolvedForge(t *testing.T) {
	setupFake(t)
	called := false
	verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) {
		called = true
		return deskkit.VerifierReceipt{}, nil
	}
	ac := &auditCtx{}
	_, err := admitVerifierEvidence(t.TempDir(), fixtureRepo, fixtureBrief, nil, ac)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("admission with no forge returned %v, want a refusal", err)
	}
	if called {
		t.Fatal("admission with no forge still reached the shared reader")
	}
	if ac.attestation != "" {
		t.Fatalf("a refused admission recorded a binding: %q", ac.attestation)
	}
}
