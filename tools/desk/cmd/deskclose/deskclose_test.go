package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskclose_test.go — the refusal matrix.
//
// Every refusal in this file ships with a POSITIVE CONTROL: the same scenario with the
// one refused property fixed, asserted to reach exit 0 and to perform the writes. A
// check that has never been observed to pass AND to fail is not a check; it is a
// comment that happens to compile. The controls are what make the red runs meaningful.
//
// Identities come from the fixture roster (rosterfixture_test.go): `ada` is the pinned
// blessing authority, `shared-agent` is a TRUSTED automation login that is not the
// authority — the stand-in for the shared bot account the desk, the workers and the
// human all reach GitHub through. Every "who authorized this" test uses it as the
// control, because it reports type=User exactly as a person does.

const (
	testRepo     = "example-org/tracker"
	otherRepo    = "medici-finance/assay"
	blessLogin   = "ada"
	blessID      = 2001
	sharedLogin  = "shared-agent"
	sharedID     = 2002
	appLogin     = "assay-desk-app[bot]"
	appID        = 300000001
	signOffCID   = "5160257214"
	signOffURL   = "https://github.com/" + testRepo + "/issues/297#issuecomment-" + signOffCID
	manifestCID  = "5160364739"
	manifestURL  = "https://github.com/" + testRepo + "/issues/300#issuecomment-" + manifestCID
	mergedPRRef  = "#40"
	mergedPRNum  = 40
	unmergedPR   = 41
	subjectIssue = 55
)

// ---------------------------------------------------------------- stub remote

type stubRemote struct {
	t *testing.T
	// items/pulls/dispositions are keyed "repo#N".
	items   map[string]string
	pulls   map[string]string
	disps   map[string]string
	comment map[string]string // comment id -> JSON

	// failComment / failItem force a transport failure on a specific read, which is
	// how could-not-check is exercised without a network.
	failComment map[string]bool
	failItem    map[string]bool
	dispMissing bool

	calls     [][]string
	dispCalls [][]string
}

func newStub(t *testing.T) *stubRemote {
	t.Helper()
	return &stubRemote{
		t: t, items: map[string]string{}, pulls: map[string]string{},
		disps: map[string]string{}, comment: map[string]string{},
		failComment: map[string]bool{}, failItem: map[string]bool{},
	}
}

// writes returns only the argv of MUTATING calls. Refusal-path assertions compare this
// against an empty slice, so "the tool refused" is proved by the absence of a write
// rather than by the absence of a success message.
func (s *stubRemote) writes() [][]string {
	var out [][]string
	for _, c := range s.calls {
		if len(c) >= 2 && (c[0] == "issue" || c[0] == "pr") && (c[1] == "close" || c[1] == "comment") {
			out = append(out, c)
		}
	}
	return out
}

func (s *stubRemote) closes() int {
	n := 0
	for _, c := range s.writes() {
		if c[1] == "close" {
			n++
		}
	}
	return n
}

// commentJSON renders a GitHub comment payload. html_url is derived from the issue_url
// so it matches the permalink the register/manifest points at — deskclose cites the
// FETCHED html_url in its close comment, never the string it was handed, so the test
// fixture has to supply a real one.
func commentJSON(login string, id int64, typ, body, issueURL string) string {
	html := strings.Replace(issueURL, "https://api.github.com/repos/", "https://github.com/", 1)
	switch {
	case strings.HasSuffix(html, "/297"):
		html += "#issuecomment-" + signOffCID
	case strings.HasSuffix(html, "/300"):
		html += "#issuecomment-" + manifestCID
	}
	return fmt.Sprintf(`{"id":1,"html_url":%q,"issue_url":%q,"body":%q,
	  "user":{"login":%q,"id":%d,"type":%q}}`, html, issueURL, body, login, id, typ)
}

func issueJSON(n int, state string, labels []string, body string) string {
	var ls []string
	for _, l := range labels {
		ls = append(ls, fmt.Sprintf(`{"name":%q}`, l))
	}
	return fmt.Sprintf(`{"number":%d,"title":"stub item","state":%q,"body":%q,"labels":[%s]}`,
		n, state, body, strings.Join(ls, ","))
}

func prIssueJSON(n int, state string) string {
	return fmt.Sprintf(`{"number":%d,"title":"stub pr","state":%q,"body":"","labels":[],"pull_request":{"merged_at":null}}`, n, state)
}

func pullJSON(n int, state string, merged bool) string {
	return fmt.Sprintf(`{"number":%d,"state":%q,"merged":%t}`, n, state, merged)
}

func dispJSON(state, verdict, evidence string) string {
	return fmt.Sprintf(`{"repo":"x","pr":1,"State":%q,"Record":{"Verdict":%q,"Evidence":%q,
	  "RecordedBy":"worker-session","RecordedAt":"2026-08-12"},"Reason":"","dispatchEligible":false}`,
		state, verdict, evidence)
}

// install wires the stub into the two exec seams and resets the per-process gate cache.
func (s *stubRemote) install() {
	cachedGrant = nil
	runGH = func(args ...string) (string, error) {
		s.calls = append(s.calls, args)
		if args[0] == "api" {
			path := args[len(args)-1]
			switch {
			case strings.Contains(path, "/issues/comments/"):
				cid := path[strings.LastIndex(path, "/")+1:]
				if s.failComment[cid] {
					return "", errors.New("HTTP 403: rate limited by the API")
				}
				if j, ok := s.comment[cid]; ok {
					return j, nil
				}
				return "", errors.New("HTTP 404: no such comment")
			case strings.Contains(path, "/pulls/"):
				key := pathKey(path, "/pulls/")
				if j, ok := s.pulls[key]; ok {
					return j, nil
				}
				return "", errors.New("HTTP 404: not a pull request")
			default:
				key := pathKey(path, "/issues/")
				if s.failItem[key] {
					return "", errors.New("HTTP 502: bad gateway")
				}
				if j, ok := s.items[key]; ok {
					return j, nil
				}
				return "", errors.New("HTTP 404: no such issue")
			}
		}
		if len(args) >= 2 && args[1] == "close" {
			// Reflect the close so a resumed run sees the item as already closed.
			for k, v := range s.items {
				if strings.HasSuffix(k, "#"+args[2]) {
					s.items[k] = strings.Replace(v, `"state":"open"`, `"state":"closed"`, 1)
				}
			}
		}
		return "", nil
	}
	runDisposition = func(args ...string) (string, error) {
		s.dispCalls = append(s.dispCalls, args)
		if s.dispMissing {
			return "", errors.New(`exec: "deskdisposition": executable file not found in $PATH`)
		}
		var repo, pr string
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "-R" {
				repo = args[i+1]
			}
			if args[i] == "--pr" {
				pr = args[i+1]
			}
		}
		if j, ok := s.disps[repo+"#"+pr]; ok {
			return j, nil
		}
		return dispJSON(dispCheckedClean, "", ""), nil
	}
	allowWrite = func(string, int) error { return nil }
	sleepFn = func(time.Duration) {}
}

// pathKey turns "repos/owner/name/issues/12" into "owner/name#12".
func pathKey(path, sep string) string {
	i := strings.Index(path, sep)
	if i < 0 {
		return path
	}
	repo := strings.TrimPrefix(path[:i], "repos/")
	return repo + "#" + path[i+len(sep):]
}

// signedRulings writes a rulings register whose R-1 carries signOffURL. `extra` is
// appended inside the R-1 section so a test can vary the surrounding text.
func signedRulings(t *testing.T, signOff string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "rulings.md")
	body := "# Rulings\n\n## " + rulingID + " Delegated close lanes\n\n" +
		"**Statement.** Issues close without a fix PR only through the lanes below.\n\n" +
		"**Sign-off:** " + signOff + "\n\n" +
		"## R-2 Label taxonomy\n\n**Sign-off:** https://github.com/x/y/issues/1#issuecomment-2\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// baseWorld is the happy path every refusal test then breaks exactly one property of.
func baseWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s := newStub(t)
	s.comment[signOffCID] = commentJSON(blessLogin, blessID, "User",
		"accepted. lanes B and C as written.", "https://api.github.com/repos/"+testRepo+"/issues/297")
	s.items[testRepo+"#"+fmt.Sprint(subjectIssue)] = issueJSON(subjectIssue, "open", nil, "tracked by #40")
	s.items[testRepo+"#"+fmt.Sprint(mergedPRNum)] = prIssueJSON(mergedPRNum, "closed")
	s.pulls[testRepo+"#"+fmt.Sprint(mergedPRNum)] = pullJSON(mergedPRNum, "closed", true)
	s.items[testRepo+"#"+fmt.Sprint(unmergedPR)] = prIssueJSON(unmergedPR, "closed")
	s.pulls[testRepo+"#"+fmt.Sprint(unmergedPR)] = pullJSON(unmergedPR, "closed", false)
	s.install()
	return s, signedRulings(t, signOffURL)
}

// exec runs the CLI end to end and returns the process exit code, so every assertion
// in this file is against the number an operator and a manifest loop actually see.
func execCLI(args ...string) (int, string) {
	var out bytes.Buffer
	code := run(args, &out)
	return code, out.String()
}

func TestMain(m *testing.M) {
	cleanup, err := installFixtureRoster()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot install the fixture roster:", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// ---------------------------------------------------------------- Verify row 1

// TestModes covers Verify row 1: each mode in the closed set closes with the right
// state_reason against the stub, and ANY other mode string exits 5.
func TestModes(t *testing.T) {
	t.Run("superseded closes not planned", func(t *testing.T) {
		s, rul := baseWorld(t)
		code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
			"--by", mergedPRRef, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
	})

	t.Run("review-request closes completed", func(t *testing.T) {
		s, rul := baseWorld(t)
		code, out := execCLI(modeReviewRequest, "-R", testRepo, fmt.Sprint(subjectIssue), "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonCompleted)
	})

	t.Run("duplicate closes not planned once the lane is granted", func(t *testing.T) {
		s, _ := baseWorld(t)
		// The positive control for the duplicate refusal: the SAME scenario with the
		// sign-off text explicitly superseding the two-role procedure.
		s.comment[signOffCID] = commentJSON(blessLogin, blessID, "User",
			"accepted, and this "+supersedeMarker+" — the desk may run the duplicate lane.",
			"https://api.github.com/repos/"+testRepo+"/issues/297")
		rul := signedRulings(t, signOffURL)
		s.items[testRepo+"#60"] = issueJSON(60, "open", nil, "")
		code, out := execCLI(modeDuplicate, "-R", testRepo, fmt.Sprint(subjectIssue),
			"--of", "#60", "--mined", "nothing-unique", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
		if !strings.Contains(commentBody(t, s), "nothing-unique") {
			t.Fatal("the --mined assertion must be echoed into the close comment")
		}
	})

	t.Run("manifest applies its rows", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "", []string{
			fmt.Sprintf("  - issue: %d\n    mode: superseded\n    target: \"#%d\"", subjectIssue, mergedPRNum),
		})
		authorizeStub(t, s, mf)
		code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
	})

	// The refusal: anything outside the closed set.
	for _, bogus := range []string{"close", "wontfix", "stale", "duplicate-of", "reject", ""} {
		t.Run("unknown mode "+bogus+" refused", func(t *testing.T) {
			s, _ := baseWorld(t)
			code, _ := execCLI(bogus, "-R", testRepo, "1")
			if bogus == "" {
				// An empty first token is still an unknown mode, not a help request.
				code, _ = execCLI("", "-R", testRepo, "1")
			}
			if code != deskkit.ExitRefused {
				t.Fatalf("mode %q: want exit 5, got %d", bogus, code)
			}
			if got := s.writes(); len(got) != 0 {
				t.Fatalf("an unknown mode performed writes: %v", got)
			}
		})
	}
}

func assertClosedWith(t *testing.T, s *stubRemote, reason string) {
	t.Helper()
	w := s.writes()
	if len(w) != 2 {
		t.Fatalf("want exactly two writes (comment then close), got %d: %v", len(w), w)
	}
	if w[0][1] != "comment" || w[1][1] != "close" {
		t.Fatalf("the comment must precede the close, got %v", w)
	}
	if !contains(w[1], reason) {
		t.Fatalf("want state_reason %q in the close argv, got %v", reason, w[1])
	}
}

func commentBody(t *testing.T, s *stubRemote) string {
	t.Helper()
	for _, c := range s.writes() {
		if c[1] == "comment" {
			return c[len(c)-1]
		}
	}
	t.Fatal("no comment was written")
	return ""
}

// ---------------------------------------------------------------- Verify row 2

// TestManifestAuthorization covers Verify row 2. The four arms are the whole reason
// this tool is allowed to exist.
func TestManifestAuthorization(t *testing.T) {
	rowsBlock := []string{
		fmt.Sprintf("  - issue: %d\n    mode: superseded\n    target: \"#%d\"", subjectIssue, mergedPRNum),
	}

	t.Run("the pinned human authority proceeds", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "", rowsBlock)
		authorizeStub(t, s, mf)
		code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if s.closes() != 1 {
			t.Fatalf("want 1 close, got %d", s.closes())
		}
	})

	// The shared automation account. It is in the trust roster, it reports type=User,
	// and the desk, the workers and the human all reach GitHub through it — which is
	// precisely why authorship by it is not a human authorization.
	t.Run("the shared automation account is refused despite type User", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "", rowsBlock)
		authorizeStubAs(t, s, mf, sharedLogin, sharedID, "User")
		code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("a refused authorization closed %d items", s.closes())
		}
		if !deskkit.TrustedAuthor(sharedLogin) {
			t.Fatal("the control is only meaningful if this login IS otherwise trusted")
		}
	})

	t.Run("an App is refused", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "", rowsBlock)
		authorizeStubAs(t, s, mf, appLogin, appID, "Bot")
		code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("an App authorization closed %d items", s.closes())
		}
	})

	// An App that lies about its type still fails, because the type check is only the
	// first of two independent conditions.
	t.Run("an App claiming type User is still refused", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "", rowsBlock)
		authorizeStubAs(t, s, mf, appLogin, appID, "User")
		code, _ := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatalf("closed %d items", s.closes())
		}
	})

	// The authority's login carrying somebody else's numeric id.
	t.Run("the authority login with a wrong id is refused", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "", rowsBlock)
		authorizeStubAs(t, s, mf, blessLogin, 999999, "User")
		code, _ := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
	})

	t.Run("an unfetchable authorization is exit 6 with zero closes", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "", rowsBlock)
		authorizeStub(t, s, mf)
		s.failComment[manifestCID] = true
		code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6 (could-not-check), got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("could-not-check closed %d items — could-not-check is not authorization", s.closes())
		}
	})

	// The digest binding: an approval for one row set does not authorize another.
	t.Run("a row added after approval invalidates the authorization", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "", rowsBlock)
		authorizeStub(t, s, mf) // approves THIS digest
		s.items[testRepo+"#70"] = issueJSON(70, "open", nil, "")
		mf2 := writeManifest(t, manifestURL, "", append(append([]string{}, rowsBlock...),
			"  - issue: 70\n    mode: superseded\n    target: \"#40\""))
		code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf2, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 on a smuggled row, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("a smuggled row closed %d items", s.closes())
		}
	})
}

// writeManifest renders a manifest file. digest is optional (the declared sha256).
func writeManifest(t *testing.T, authURL, digest string, rows []string) string {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "authorized-by: %s\n", authURL)
	if digest != "" {
		fmt.Fprintf(&b, "manifest-sha256: %s\n", digest)
	}
	b.WriteString("rows:\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
	p := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(p, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// authorizeStub posts the approval comment the manifest's digest requires, authored by
// the pinned blessing authority.
func authorizeStub(t *testing.T, s *stubRemote, manifestPath string) {
	t.Helper()
	authorizeStubAs(t, s, manifestPath, blessLogin, blessID, "User")
}

func authorizeStubAs(t *testing.T, s *stubRemote, manifestPath, login string, id int64, typ string) {
	t.Helper()
	m, err := parseManifest(manifestPath)
	if err != nil {
		t.Fatalf("the fixture manifest must parse: %v", err)
	}
	body := fmt.Sprintf("%s sha256:%s", manifestApprovalPhrase, m.Digest())
	s.comment[manifestCID] = commentJSON(login, id, typ, body,
		"https://api.github.com/repos/"+testRepo+"/issues/300")
}

// ---------------------------------------------------------------- Verify row 3

// TestMergedNotClosed covers Verify row 3: closed-unmerged never satisfies lanes 2/3.
// Both arms of the control are here — the same PR merged reaches exit 0.
func TestMergedNotClosed(t *testing.T) {
	t.Run("superseded by a closed-unmerged PR is refused", func(t *testing.T) {
		s, rul := baseWorld(t)
		code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
			"--by", fmt.Sprintf("#%d", unmergedPR), "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("closed %d items against an unmerged PR", s.closes())
		}
	})

	t.Run("review-request whose PR is closed-unmerged is refused", func(t *testing.T) {
		s, rul := baseWorld(t)
		s.items[testRepo+"#"+fmt.Sprint(subjectIssue)] =
			issueJSON(subjectIssue, "open", nil, fmt.Sprintf("review request for #%d", unmergedPR))
		code, out := execCLI(modeReviewRequest, "-R", testRepo, fmt.Sprint(subjectIssue), "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("closed %d items", s.closes())
		}
	})

	t.Run("control: the same PR merged closes", func(t *testing.T) {
		s, rul := baseWorld(t)
		s.pulls[testRepo+"#"+fmt.Sprint(unmergedPR)] = pullJSON(unmergedPR, "closed", true)
		code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
			"--by", fmt.Sprintf("#%d", unmergedPR), "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if s.closes() != 1 {
			t.Fatalf("want 1 close, got %d", s.closes())
		}
	})

	t.Run("an unreadable PR state is could-not-check, not merged", func(t *testing.T) {
		s, rul := baseWorld(t)
		delete(s.pulls, testRepo+"#"+fmt.Sprint(mergedPRNum))
		code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
			"--by", mergedPRRef, "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d\n%s", code, out)
		}
		if s.closes() != 0 {
			t.Fatalf("closed %d items on an unread merge state", s.closes())
		}
	})

	t.Run("a review-request body naming no PR is refused, never guessed", func(t *testing.T) {
		s, rul := baseWorld(t)
		s.items[testRepo+"#"+fmt.Sprint(subjectIssue)] = issueJSON(subjectIssue, "open", nil, "no refs here")
		code, _ := execCLI(modeReviewRequest, "-R", testRepo, fmt.Sprint(subjectIssue), "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("closed on an absent PR ref")
		}
	})

	t.Run("a review-request body naming two PRs is refused as ambiguous", func(t *testing.T) {
		s, rul := baseWorld(t)
		s.items[testRepo+"#"+fmt.Sprint(subjectIssue)] =
			issueJSON(subjectIssue, "open", nil, "see #40 and also #41")
		code, _ := execCLI(modeReviewRequest, "-R", testRepo, fmt.Sprint(subjectIssue), "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("closed on an ambiguous PR ref")
		}
	})
}

// ---------------------------------------------------------------- Verify row 4

// TestDecisionLabelRefused covers Verify row 4: a decision-labelled item exits 5 in
// EVERY mode, including as a manifest row, and the refusal precedes every write.
func TestDecisionLabelRefused(t *testing.T) {
	for _, label := range decisionLabels {
		t.Run(label+"/superseded", func(t *testing.T) {
			s, rul := baseWorld(t)
			s.items[testRepo+"#"+fmt.Sprint(subjectIssue)] =
				issueJSON(subjectIssue, "open", []string{label}, "tracked by #40")
			code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
				"--by", mergedPRRef, "--rulings", rul)
			if code != deskkit.ExitRefused {
				t.Fatalf("want exit 5, got %d\n%s", code, out)
			}
			if got := s.writes(); len(got) != 0 {
				t.Fatalf("a decision-labelled item was written to: %v", got)
			}
		})

		t.Run(label+"/review-request", func(t *testing.T) {
			s, rul := baseWorld(t)
			s.items[testRepo+"#"+fmt.Sprint(subjectIssue)] =
				issueJSON(subjectIssue, "open", []string{label}, "tracked by #40")
			code, _ := execCLI(modeReviewRequest, "-R", testRepo, fmt.Sprint(subjectIssue), "--rulings", rul)
			if code != deskkit.ExitRefused {
				t.Fatalf("want exit 5, got %d", code)
			}
			if got := s.writes(); len(got) != 0 {
				t.Fatalf("written to: %v", got)
			}
		})

		t.Run(label+"/duplicate", func(t *testing.T) {
			s, _ := baseWorld(t)
			s.comment[signOffCID] = commentJSON(blessLogin, blessID, "User",
				"accepted, and this "+supersedeMarker,
				"https://api.github.com/repos/"+testRepo+"/issues/297")
			rul := signedRulings(t, signOffURL)
			s.items[testRepo+"#"+fmt.Sprint(subjectIssue)] =
				issueJSON(subjectIssue, "open", []string{label}, "")
			s.items[testRepo+"#60"] = issueJSON(60, "open", nil, "")
			code, _ := execCLI(modeDuplicate, "-R", testRepo, fmt.Sprint(subjectIssue),
				"--of", "#60", "--mined", "nothing-unique", "--rulings", rul)
			if code != deskkit.ExitRefused {
				t.Fatalf("want exit 5, got %d", code)
			}
			if got := s.writes(); len(got) != 0 {
				t.Fatalf("written to: %v", got)
			}
		})

		// The manifest arm is the one that matters most: an authorized batch must not
		// launder a decision item past the gate just because a human approved the file.
		t.Run(label+"/manifest row", func(t *testing.T) {
			s, rul := baseWorld(t)
			s.items[testRepo+"#71"] = issueJSON(71, "open", []string{label}, "")
			mf := writeManifest(t, manifestURL, "", []string{
				"  - issue: 71\n    mode: superseded\n    target: \"#40\"",
			})
			authorizeStub(t, s, mf)
			code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
			if code != deskkit.ExitRefused {
				t.Fatalf("want exit 5, got %d\n%s", code, out)
			}
			if s.closes() != 0 {
				t.Fatalf("an authorized manifest closed %d decision items", s.closes())
			}
			if !strings.Contains(out, "resume with:") {
				t.Fatal("a hard stop must print a resume point")
			}
		})
	}

	t.Run("control: the same item without the label closes", func(t *testing.T) {
		s, rul := baseWorld(t)
		code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
			"--by", mergedPRRef, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if s.closes() != 1 {
			t.Fatalf("the control must actually close, or the refusals above prove nothing: %d", s.closes())
		}
	})

	t.Run("an unreadable label set is could-not-check", func(t *testing.T) {
		s, rul := baseWorld(t)
		s.failItem[testRepo+"#"+fmt.Sprint(subjectIssue)] = true
		code, _ := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
			"--by", mergedPRRef, "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("closed an item whose labels could not be read")
		}
	})
}

// ---------------------------------------------------------------- Verify row 4b

// TestManifestRateLimitResume covers Verify row 4b. A rate-limit refusal mid-manifest
// must WAIT and retry the SAME row: the run is not aborted, and no row is skipped.
func TestManifestRateLimitResume(t *testing.T) {
	s, rul := baseWorld(t)
	for _, n := range []int{81, 82, 83} {
		s.items[fmt.Sprintf("%s#%d", testRepo, n)] = issueJSON(n, "open", nil, "")
	}
	mf := writeManifest(t, manifestURL, "", []string{
		"  - issue: 81\n    mode: superseded\n    target: \"#40\"",
		"  - issue: 82\n    mode: superseded\n    target: \"#40\"",
		"  - issue: 83\n    mode: superseded\n    target: \"#40\"",
	})
	authorizeStub(t, s, mf)

	// Refuse the FIRST write attempt on row 82, once. Everything else is allowed.
	refusedOnce := false
	allowWrite = func(repo string, n int) error {
		if n == 82 && !refusedOnce {
			refusedOnce = true
			return deskkit.RateLimitedAfter("write budget spent", 7*time.Minute)
		}
		return nil
	}
	var slept []time.Duration
	sleepFn = func(d time.Duration) { slept = append(slept, d) }

	code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("a rate limit must never end the run: want exit 0, got %d\n%s", code, out)
	}
	if !refusedOnce {
		t.Fatal("the rate-limit control never fired — the test proves nothing")
	}
	if len(slept) != 1 || slept[0] != 7*time.Minute {
		t.Fatalf("want exactly one wait of the stated retry-after, got %v", slept)
	}
	if !strings.Contains(out, "not skipping it") {
		t.Fatalf("the wait must say it is retrying the same row:\n%s", out)
	}
	// Every row applied EXACTLY once.
	if s.closes() != 3 {
		t.Fatalf("want 3 closes (no row skipped, none doubled), got %d\n%s", s.closes(), out)
	}
	seen := map[string]int{}
	for _, c := range s.writes() {
		if c[1] == "close" {
			seen[c[2]]++
		}
	}
	for _, n := range []string{"81", "82", "83"} {
		if seen[n] != 1 {
			t.Fatalf("row %s closed %d times, want exactly 1", n, seen[n])
		}
	}
}

// TestManifestRateLimitBudgetPauses proves the other arm: when the wait would exceed
// --max-wait the run exits with the rate-limit code and a resume point at the row it
// did NOT apply. Deferred, never skipped.
func TestManifestRateLimitBudgetPauses(t *testing.T) {
	s, rul := baseWorld(t)
	for _, n := range []int{81, 82} {
		s.items[fmt.Sprintf("%s#%d", testRepo, n)] = issueJSON(n, "open", nil, "")
	}
	mf := writeManifest(t, manifestURL, "", []string{
		"  - issue: 81\n    mode: superseded\n    target: \"#40\"",
		"  - issue: 82\n    mode: superseded\n    target: \"#40\"",
	})
	authorizeStub(t, s, mf)
	allowWrite = func(repo string, n int) error {
		if n == 82 {
			return deskkit.RateLimitedAfter("write budget spent", time.Hour)
		}
		return nil
	}
	code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul, "--max-wait", "1m")
	if code != deskkit.ExitRateLimited {
		t.Fatalf("want exit 4, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "--resume-from 82") {
		t.Fatalf("the pause must name the unapplied row:\n%s", out)
	}
	if s.closes() != 1 {
		t.Fatalf("row 81 should have closed and 82 deferred, got %d closes", s.closes())
	}
}

// TestManifestResumeIsIdempotent proves the resumed run treats an already-closed row as
// a no-op rather than closing it twice or refusing the batch.
func TestManifestResumeIsIdempotent(t *testing.T) {
	s, rul := baseWorld(t)
	s.items[testRepo+"#81"] = issueJSON(81, "closed", nil, "")
	s.items[testRepo+"#82"] = issueJSON(82, "open", nil, "")
	mf := writeManifest(t, manifestURL, "", []string{
		"  - issue: 81\n    mode: superseded\n    target: \"#40\"",
		"  - issue: 82\n    mode: superseded\n    target: \"#40\"",
	})
	authorizeStub(t, s, mf)
	code, out := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "noop\t"+testRepo+"#81") {
		t.Fatalf("an already-closed row must be an idempotent no-op:\n%s", out)
	}
	if s.closes() != 1 {
		t.Fatalf("want exactly one close (row 82), got %d", s.closes())
	}
}

// ---------------------------------------------------------------- ruling gate

// TestUnsignedRulingRefusesEveryMode is the gate that makes the whole tool inert until
// a human has actually granted the lanes. It is the state of the register TODAY.
func TestUnsignedRulingRefusesEveryMode(t *testing.T) {
	s, _ := baseWorld(t)
	unsigned := signedRulings(t, "_(empty — the blessing authority fills this with an acceptance/amendment URL)_")
	mf := writeManifest(t, manifestURL, "", []string{
		"  - issue: 55\n    mode: superseded\n    target: \"#40\"",
	})
	authorizeStub(t, s, mf)

	cases := [][]string{
		{modeSuperseded, "-R", testRepo, "55", "--by", "#40", "--rulings", unsigned},
		{modeReviewRequest, "-R", testRepo, "55", "--rulings", unsigned},
		{modeDuplicate, "-R", testRepo, "55", "--of", "#60", "--mined", "x", "--rulings", unsigned},
		{modeManifest, "-R", testRepo, "--file", mf, "--rulings", unsigned},
	}
	for _, args := range cases {
		t.Run(args[0], func(t *testing.T) {
			s.calls = nil
			cachedGrant = nil
			code, out := execCLI(args...)
			if code != deskkit.ExitRefused {
				t.Fatalf("want exit 5 on an unsigned ruling, got %d\n%s", code, out)
			}
			if len(s.writes()) != 0 {
				t.Fatalf("wrote with no signed ruling: %v", s.writes())
			}
		})
	}
}

// TestRulingSignOffMustBeHuman: the file states the claim; the FETCH is what makes it
// authority. A sign-off URL resolving to an App or to the shared automation account is
// no more authority than an empty line.
func TestRulingSignOffMustBeHuman(t *testing.T) {
	for _, tc := range []struct {
		name  string
		login string
		id    int64
		typ   string
	}{
		{"app", appLogin, appID, "Bot"},
		{"shared automation account", sharedLogin, sharedID, "User"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rul := baseWorld(t)
			s.comment[signOffCID] = commentJSON(tc.login, tc.id, tc.typ, "accepted",
				"https://api.github.com/repos/"+testRepo+"/issues/297")
			code, _ := execCLI(modeSuperseded, "-R", testRepo, "55", "--by", mergedPRRef, "--rulings", rul)
			if code != deskkit.ExitRefused {
				t.Fatalf("want exit 5, got %d", code)
			}
			if s.closes() != 0 {
				t.Fatal("closed on a non-human sign-off")
			}
		})
	}

	t.Run("an unfetchable sign-off is could-not-check", func(t *testing.T) {
		s, rul := baseWorld(t)
		s.failComment[signOffCID] = true
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "55", "--by", mergedPRRef, "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("could-not-check is not authorization")
		}
	})

	t.Run("a missing rulings file is could-not-check, not unsigned", func(t *testing.T) {
		s, _ := baseWorld(t)
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "55", "--by", mergedPRRef,
			"--rulings", filepath.Join(t.TempDir(), "absent.md"))
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("closed with no register at all")
		}
	})

	// A later ruling's signed sign-off must not be read as R-1's.
	t.Run("a sibling ruling's sign-off does not sign R-1", func(t *testing.T) {
		s, _ := baseWorld(t)
		dir := t.TempDir()
		p := filepath.Join(dir, "rulings.md")
		body := "## " + rulingID + " lanes\n\n**Sign-off:** _(empty)_\n\n" +
			"## R-2 taxonomy\n\n**Sign-off:** " + signOffURL + "\n"
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "55", "--by", mergedPRRef, "--rulings", p)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("a sibling ruling's signature closed an item")
		}
	})
}

// TestDuplicateLaneRefusedByDefault: R-1 withdraws the duplicate lane from the desk.
// Accepting R-1 as written must not widen the desk's authority by silence.
func TestDuplicateLaneRefusedByDefault(t *testing.T) {
	s, rul := baseWorld(t)
	s.items[testRepo+"#60"] = issueJSON(60, "open", nil, "")
	code, out := execCLI(modeDuplicate, "-R", testRepo, "55", "--of", "#60",
		"--mined", "nothing-unique", "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d\n%s", code, out)
	}
	if s.closes() != 0 {
		t.Fatal("the duplicate lane closed an item without an explicit supersession")
	}

	t.Run("--mined is mandatory", func(t *testing.T) {
		s, rul := baseWorld(t)
		code, _ := execCLI(modeDuplicate, "-R", testRepo, "55", "--of", "#60", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if len(s.writes()) != 0 {
			t.Fatal("wrote without --mined")
		}
	})
}

// ---------------------------------------------------------------- repo scope

func TestDisallowedRepoRefused(t *testing.T) {
	s, rul := baseWorld(t)
	for _, repo := range []string{"someone-else/private-thing", "example-org", ""} {
		code, _ := execCLI(modeSuperseded, "-R", repo, "55", "--by", mergedPRRef, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("repo %q: want exit 5, got %d", repo, code)
		}
	}
	if len(s.writes()) != 0 {
		t.Fatalf("a disallowed repo was written to: %v", s.writes())
	}

	// Control: an allowed repo reaches the gate.
	if !deskkit.IsAllowedRepo(testRepo) || !deskkit.IsAllowedRepo(otherRepo) {
		t.Fatal("the control repos must be in the fixture roster's allowed set")
	}

	// A cross-repo TARGET is scoped too: the target's repo must also be allowed.
	t.Run("a cross-repo target outside the set is refused", func(t *testing.T) {
		s, rul := baseWorld(t)
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "55",
			"--by", "someone-else/elsewhere#9", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("closed against a target outside the repo set")
		}
	})
}

// ---------------------------------------------------------------- disposition record

// TestDispositionRecordConsumed proves deskclose consumes ground-truth/05's record
// rather than re-deriving a verdict — and that every unhappy state of that read is a
// refusal.
func TestDispositionRecordConsumed(t *testing.T) {
	// A PR target. The subject item is itself a PR, which is the case the five
	// confirmed-superseded PRs on this desk present.
	setup := func(t *testing.T) (*stubRemote, string) {
		s, rul := baseWorld(t)
		s.items[testRepo+"#90"] = prIssueJSON(90, "open")
		s.pulls[testRepo+"#90"] = pullJSON(90, "open", false)
		return s, rul
	}

	t.Run("a terminal record with matching evidence closes", func(t *testing.T) {
		s, rul := setup(t)
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedFailed, verdictSuperseded, testRepo+"#40")
		code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if len(s.dispCalls) == 0 {
			t.Fatal("deskclose must READ the record, not re-derive the verdict")
		}
		if !strings.Contains(commentBody(t, s), verdictSuperseded) {
			t.Fatal("the close comment must cite the disposition record it acted on")
		}
	})

	t.Run("NEEDS-REBASE is live work and is refused", func(t *testing.T) {
		s, rul := setup(t)
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedFailed, verdictNeedsRebase, testRepo+"#40")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("closed a PR a worker recorded as live work")
		}
	})

	t.Run("no record at all is refused", func(t *testing.T) {
		s, rul := setup(t)
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedClean, "", "")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("closed a PR nobody has recorded a finding on")
		}
	})

	t.Run("could-not-check is exit 6", func(t *testing.T) {
		s, rul := setup(t)
		s.disps[testRepo+"#90"] = dispJSON(dispCouldNotCheck, "", "")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d", code)
		}
	})

	t.Run("a missing deskdisposition binary is could-not-check, not 'no record'", func(t *testing.T) {
		s, rul := setup(t)
		s.dispMissing = true
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("want exit 6, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("closed with the record reader absent")
		}
	})

	t.Run("evidence naming a different target is refused", func(t *testing.T) {
		s, rul := setup(t)
		s.disps[testRepo+"#90"] = dispJSON(dispCheckedFailed, verdictSuperseded, testRepo+"#999")
		code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		if s.closes() != 0 {
			t.Fatal("deskclose picked a winner between the record and the caller")
		}
	})
}

// ---------------------------------------------------------------- no escape hatch

// TestNoForceEscapeHatch is a source-level assertion, because the property is about
// what does NOT exist. A future contributor reaching for --force trips this.
func TestNoForceEscapeHatch(t *testing.T) {
	banned := []string{
		`"force"`, `"yes"`, `"no-verify"`, `"skip-authorization"`, `"i-know-what-im-doing"`,
		"DESKCLOSE_FORCE", "DESKCLOSE_SKIP", "DESKCLOSE_AUTHORIZED",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, rerr := os.ReadFile(e.Name())
		if rerr != nil {
			t.Fatal(rerr)
		}
		checked++
		src := string(raw)
		for _, b := range banned {
			if strings.Contains(src, b) {
				t.Errorf("%s declares %s — deskclose has no override for its authorization gate. "+
					"The escape hatch for an unauthorized batch is the human authorizing it.", e.Name(), b)
			}
		}
		// os.Getenv anywhere in this package would be a control surface outside the
		// gate. The roster is read by deskkit, from the config home, by design.
		if strings.Contains(src, "os.Getenv") {
			t.Errorf("%s reads the environment — deskclose takes no environment override", e.Name())
		}
	}
	if checked < 5 {
		t.Fatalf("only %d source files scanned; the guard is not covering the package", checked)
	}
}

// TestRefusalPathsNeverWrite sweeps every refusal in the matrix and asserts the shared
// property: zero mutating argv is ever constructed. It is the belt to the per-case
// braces above, and it fails if a new lane is added without its gate.
func TestRefusalPathsNeverWrite(t *testing.T) {
	type scenario struct {
		name string
		args []string
		want int
		prep func(*stubRemote, *string)
	}
	scenarios := []scenario{
		{"unknown mode", []string{"wontfix", "-R", testRepo, "55"}, deskkit.ExitRefused, nil},
		{"disallowed repo", []string{modeSuperseded, "-R", "nope/nope", "55", "--by", "#40"}, deskkit.ExitRefused, nil},
		{"decision label", []string{modeSuperseded, "-R", testRepo, "55", "--by", "#40"}, deskkit.ExitRefused,
			func(s *stubRemote, _ *string) {
				s.items[testRepo+"#55"] = issueJSON(55, "open", []string{"needs-decision"}, "")
			}},
		{"unmerged PR", []string{modeSuperseded, "-R", testRepo, "55", "--by", "#41"}, deskkit.ExitRefused, nil},
		{"duplicate lane", []string{modeDuplicate, "-R", testRepo, "55", "--of", "#40", "--mined", "x"},
			deskkit.ExitRefused, nil},
		{"unfetchable ruling", []string{modeSuperseded, "-R", testRepo, "55", "--by", "#40"},
			deskkit.ExitUnverifiable, func(s *stubRemote, _ *string) { s.failComment[signOffCID] = true }},
		{"unsigned ruling", []string{modeSuperseded, "-R", testRepo, "55", "--by", "#40"},
			deskkit.ExitRefused, func(_ *stubRemote, rul *string) { *rul = "" }},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			s, rul := baseWorld(t)
			if sc.prep != nil {
				sc.prep(s, &rul)
			}
			if rul == "" {
				rul = signedRulings(t, "_(empty)_")
			}
			code, _ := execCLI(append(append([]string{}, sc.args...), "--rulings", rul)...)
			if code != sc.want {
				t.Fatalf("want exit %d, got %d", sc.want, code)
			}
			if got := s.writes(); len(got) != 0 {
				t.Fatalf("a refusal path performed writes: %v", got)
			}
		})
	}
}

// ---------------------------------------------------------------- manifest parser

// TestManifestParserIsStrict: an unknown key is refused, never dropped. A dropped line
// is a row the human approved and the tool did something else with.
func TestManifestParserIsStrict(t *testing.T) {
	bad := map[string]string{
		"unknown top-level key": "authorized-by: " + manifestURL + "\nclose-all: true\nrows:\n  - issue: 1\n    mode: superseded\n    target: \"#2\"\n",
		"unknown row key":       "authorized-by: " + manifestURL + "\nrows:\n  - issue: 1\n    mode: superseded\n    target: \"#2\"\n    force: true\n",
		"unknown row mode":      "authorized-by: " + manifestURL + "\nrows:\n  - issue: 1\n    mode: wontfix\n    target: \"#2\"\n",
		"nested manifest mode":  "authorized-by: " + manifestURL + "\nrows:\n  - issue: 1\n    mode: manifest\n    target: \"#2\"\n",
		"no authorization":      "rows:\n  - issue: 1\n    mode: superseded\n    target: \"#2\"\n",
		"no rows":               "authorized-by: " + manifestURL + "\nrows:\n",
		"duplicate row unmined": "authorized-by: " + manifestURL + "\nrows:\n  - issue: 1\n    mode: duplicate\n    target: \"#2\"\n",
		"target missing":        "authorized-by: " + manifestURL + "\nrows:\n  - issue: 1\n    mode: superseded\n",
	}
	for name, body := range bad {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "m.yaml")
			if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := parseManifest(p); err == nil {
				t.Fatal("want a refusal, got a parse")
			} else if !deskkit.IsRefused(err) {
				t.Fatalf("want exit 5, got %v", deskkit.ExitCodeOf(err))
			}
		})
	}

	t.Run("control: the accepted grammar parses", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "m.yaml")
		body := "# a comment\nauthorized-by: " + manifestURL + "\nmanifest-sha256: deadbeef\nrows:\n" +
			"  - issue: 1\n    mode: superseded\n    target: \"#2\"\n" +
			"  - issue: 3\n    mode: review-request\n"
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		m, err := parseManifest(p)
		if err != nil {
			t.Fatalf("the accepted grammar must parse: %v", err)
		}
		if len(m.Rows) != 2 || m.Rows[0].Issue != 1 || m.Rows[1].Mode != modeReviewRequest {
			t.Fatalf("parsed wrong: %+v", m.Rows)
		}
	})

	t.Run("a declared digest that disagrees with the rows is refused", func(t *testing.T) {
		s, rul := baseWorld(t)
		mf := writeManifest(t, manifestURL, "0000000000000000000000000000000000000000000000000000000000000000",
			[]string{"  - issue: 55\n    mode: superseded\n    target: \"#40\""})
		authorizeStub(t, s, mf)
		code, _ := execCLI(modeManifest, "-R", testRepo, "--file", mf, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
	})
}

// TestDryRunWritesNothing: the flag that selects the dry-run audit result is the flag
// that suppresses the write, which is deskkit's stated precondition for ResultDryRun.
func TestDryRunWritesNothing(t *testing.T) {
	s, rul := baseWorld(t)
	code, out := execCLI(modeSuperseded, "-R", testRepo, "55", "--by", mergedPRRef,
		"--rulings", rul, "--dry-run")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if got := s.writes(); len(got) != 0 {
		t.Fatalf("--dry-run wrote: %v", got)
	}
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("the dry run must print what it would have done:\n%s", out)
	}
}

// TestPositionalAfterFlags pins the brief's documented argv shape. Go's flag package
// stops at the first non-flag token, so `-R <repo> <N> --mined <s>` would silently drop
// --mined — turning a mandatory assertion into an optional one.
func TestPositionalAfterFlags(t *testing.T) {
	flags, pos := splitPositionals([]string{"-R", testRepo, "55", "--of", "60", "--mined", "nothing-unique"})
	if len(pos) != 1 || pos[0] != "55" {
		t.Fatalf("positional split wrong: %v", pos)
	}
	if !contains(flags, "--mined") || !contains(flags, "nothing-unique") {
		t.Fatalf("flags after the positional were dropped: %v", flags)
	}
	if contains(flags, "55") {
		t.Fatalf("the item number leaked into the flags: %v", flags)
	}
	// `-R owner/repo` must not have its value read as the item number.
	if contains(pos, testRepo) {
		t.Fatalf("the repo was read as a positional: %v", pos)
	}
}

// TestCommentPrecedesClose pins the trail discipline: the comment naming the lane, the
// target and the authorizing ruling lands BEFORE the close, so a wrong batch is
// auditable and reversible rather than silent.
func TestCommentPrecedesClose(t *testing.T) {
	s, rul := baseWorld(t)
	if code, out := execCLI(modeSuperseded, "-R", testRepo, "55", "--by", mergedPRRef,
		"--rulings", rul); code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	body := commentBody(t, s)
	for _, want := range []string{modeSuperseded, mergedPRRef, rulingID, signOffURL} {
		if !strings.Contains(body, want) {
			t.Fatalf("the close comment must name %q:\n%s", want, body)
		}
	}
}
