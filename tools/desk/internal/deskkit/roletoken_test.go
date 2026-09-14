package deskkit

// roletoken_test.go — the two refusals the identity layer owes its callers, plus the
// property that neither of them can ever print the credential they are complaining about.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- loop identity ---------------------------------------------------------------

func TestRequireLoopIdentityRefusesWhenTheLoopIsUnset(t *testing.T) {
	t.Setenv(loopEnv, "")
	err := RequireLoopIdentity("deskflip")
	if err == nil {
		t.Fatal("an unset loop identity was accepted — a STOP.<loop> flag would then never match this session")
	}
	if got := ExitCodeOf(err); got != ExitRefused {
		t.Fatalf("exit = %d, want %d (refused): a missing identity is a settled state, not an unreadable one", got, ExitRefused)
	}
	for _, want := range []string{"deskflip", loopEnv, "export"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q — an operator cannot act on it: %v", want, err)
		}
	}
}

func TestRequireLoopIdentityIsCouldNotCheckOnAnUnknownLoop(t *testing.T) {
	t.Setenv(loopEnv, "not-a-loop-anyone-knows")
	err := RequireLoopIdentity("deskpost")
	if err == nil {
		t.Fatal("an unrecognised loop name was accepted as an identity")
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable): a name nothing recognises says nothing about "+
			"whether a stop is held", got, ExitUnverifiable)
	}
}

func TestRequireLoopIdentityAcceptsEveryKnownLoop(t *testing.T) {
	for _, loop := range KnownLoopNames() {
		t.Setenv(loopEnv, loop)
		if err := RequireLoopIdentity("deskpr"); err != nil {
			t.Errorf("loop %q rejected: %v", loop, err)
		}
	}
}

func TestSessionTokenRoleResolvesTheAppRoleFromTheLoop(t *testing.T) {
	for loop, want := range LoopTokenRoles() {
		t.Setenv(loopEnv, loop)
		role, gotLoop, err := SessionTokenRole("deskboard")
		if err != nil {
			t.Errorf("loop %q: %v", loop, err)
			continue
		}
		if role != want || gotLoop != loop {
			t.Errorf("loop %q resolved to role %q loop %q, want role %q", loop, role, gotLoop, want)
		}
	}
}

func TestSessionTokenRoleRefusesWithNoLoopIdentity(t *testing.T) {
	t.Setenv(loopEnv, "")
	if _, _, err := SessionTokenRole("deskboard"); err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("SessionTokenRole with no identity: err=%v exit=%d, want a refusal", err, ExitCodeOf(err))
	}
}

// A CANONICAL loop the kill switch knows but that carries no App role must REFUSE, not fall
// back to some default identity. Today every canonical loop has a role, so this reports
// could-not-check rather than passing silently — a vacuous green here would hide the arm
// entirely the first time a role-less loop is added.
func TestSessionTokenRoleRefusesALoopWithNoAppRole(t *testing.T) {
	var orphan string
	roles := LoopTokenRoles()
	for _, loop := range KnownLoopNames() {
		// A retired spelling resolves to its canonical loop, which may well carry a role;
		// the orphan we are after is a CANONICAL loop with none.
		canonical, known := CanonicalLoopName(loop)
		if !known {
			continue
		}
		if _, ok := roles[canonical]; !ok {
			orphan = canonical
			break
		}
	}
	if orphan == "" {
		t.Skip("could-not-check: every known loop now carries an App role, so there is no orphan to exercise")
	}
	t.Setenv(loopEnv, orphan)
	_, _, err := SessionTokenRole("deskboard")
	if err == nil {
		t.Fatalf("loop %q has no App role but resolved one anyway", orphan)
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable)", got, ExitUnverifiable)
	}
}

// TestNoLoopActsAsCellIssues — desk-console/31, Task 3 / Verify row 5. cell-issues is the
// write-issues App's role, mintable by NAME only (`desktoken cell-issues`); it must never be
// a loop's default identity, or every session of that loop's window would silently start
// filing under the write App instead of its own. Pinned two ways: no loop in the table maps
// to it, and asking the table for it as a LOOP name (not a role) resolves nothing.
func TestNoLoopActsAsCellIssues(t *testing.T) {
	for loop, role := range LoopTokenRoles() {
		if role == "cell-issues" {
			t.Fatalf("loop %q resolves to role cell-issues — no loop may act as the write-issues "+
				"App by default; only an explicit by-name selection may", loop)
		}
	}
	if role, ok := TokenRoleForLoop("cell-issues"); ok {
		t.Fatalf(`TokenRoleForLoop("cell-issues") = (%q, true), want ok=false — cell-issues is an `+
			"App role, never a loop name", role)
	}
}

func TestLoopTokenRolesHandsBackACopy(t *testing.T) {
	got := LoopTokenRoles()
	got["the-desk"] = "tampered"
	if again := LoopTokenRoles(); again["the-desk"] == "tampered" {
		t.Fatal("LoopTokenRoles returned the shared map — a caller can rewrite every session's identity")
	}
}

// --- token lookup ----------------------------------------------------------------

// stubMinter replaces the token minter for one test. It returns whatever path the caller
// asks it to, so the file-read half runs against a real file on disk.
func stubMinter(t *testing.T, path, stderr string, err error) *[]string {
	t.Helper()
	var seen []string
	prev := tokenMinter
	t.Cleanup(func() { tokenMinter = prev })
	tokenMinter = func(role, owner string) (string, string, error) {
		seen = append(seen, role+" "+owner)
		return path, stderr, err
	}
	return &seen
}

func TestRoleTokenForOwnerReadsTheMintedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reviewer-token-1")
	if err := os.WriteFile(path, []byte("installation-token-stub-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	seen := stubMinter(t, path, "", nil)

	tok, gotPath, err := RoleTokenForOwner("reviewer", "example-org")
	if err != nil {
		t.Fatalf("RoleTokenForOwner: %v", err)
	}
	if tok != "installation-token-stub-value" {
		t.Fatalf("token = %q — the trailing newline of the cache file must not survive", tok)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if len(*seen) != 1 || (*seen)[0] != "reviewer example-org" {
		t.Fatalf("minter calls = %v, want one call for role reviewer on owner example-org", *seen)
	}
}

func TestRoleTokenForRepoAsksForTheOwnerNotTheSlug(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "worker-token-1")
	if err := os.WriteFile(path, []byte("installation-token-stub-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	seen := stubMinter(t, path, "", nil)
	if _, _, err := RoleTokenForRepo("worker", "example-org/tracker"); err != nil {
		t.Fatalf("RoleTokenForRepo: %v", err)
	}
	if len(*seen) != 1 || (*seen)[0] != "worker example-org" {
		t.Fatalf("minter calls = %v — an installation token is per ACCOUNT, so the owner is what it "+
			"must be asked for", *seen)
	}
}

func TestRoleTokenForOwnerFailsClosedOnAMissingCacheFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-token")
	stubMinter(t, missing, "", nil)
	_, path, err := RoleTokenForOwner("reviewer", "example-org")
	if err == nil {
		t.Fatal("a missing token file returned a token")
	}
	if path != missing || !strings.Contains(err.Error(), missing) || !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("the refusal must name the role AND the path an operator has to fix: %v", err)
	}
}

// The minter now prints the P3 effective-config echo on stderr BEFORE any error,
// so the failure detail this surfaces must be the REAL refusal, not the echo
// header — otherwise every mint failure reads "assay-config: class=..." instead of
// what actually went wrong.
func TestMinterFailureDetailSkipsTheConfigEcho(t *testing.T) {
	stderr := "assay-config: class=write source=file configured=true\n" +
		"assay-config: ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private\n" +
		"no installation found for owner \"example-org\"\n"
	stubMinter(t, "unused", stderr, fmt.Errorf("exit status 6"))
	_, _, err := RoleTokenForOwner("reviewer", "example-org")
	if err == nil {
		t.Fatal("a minter error returned no error")
	}
	if !strings.Contains(err.Error(), "no installation found for owner") {
		t.Fatalf("failure detail must quote the real refusal, not the config echo: %v", err)
	}
	if strings.Contains(err.Error(), "assay-config:") {
		t.Fatalf("failure detail leaked the config echo header: %v", err)
	}
}

// minterErrorDetail falls back to firstLine when stderr is nothing but echo.
func TestMinterErrorDetailFallsBackWhenOnlyEcho(t *testing.T) {
	only := "assay-config: class=write source=file configured=false\n"
	if got := minterErrorDetail(only); got != "assay-config: class=write source=file configured=false" {
		t.Fatalf("with only echo lines minterErrorDetail should fall back to firstLine; got %q", got)
	}
	if got := minterErrorDetail(""); got != "" {
		t.Fatalf("empty stderr should yield empty detail; got %q", got)
	}
}

func TestRoleTokenForOwnerFailsClosedOnAnEmptyCacheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reviewer-token-1")
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stubMinter(t, path, "", nil)
	if _, _, err := RoleTokenForOwner("reviewer", "example-org"); err == nil {
		t.Fatal("an empty token file was accepted — the call would then run unauthenticated")
	}
}

func TestRoleTokenForOwnerRefusesWithNoRoleOrNoOwner(t *testing.T) {
	stubMinter(t, "unused", "", nil)
	if _, _, err := RoleTokenForOwner("", "example-org"); err == nil {
		t.Error("an unnamed role minted a token — a token cannot target an identity nothing names")
	}
	if _, _, err := RoleTokenForOwner("reviewer", ""); err == nil {
		t.Error("an unnamed account minted a token — an installation token is per account")
	}
}

// The credential must never reach a message. This is the property that lets every refusal
// above be printed, logged and pasted into an issue without leaking anything.
func TestNoRefusalEverCarriesTheTokenValue(t *testing.T) {
	const secret = "installation-token-never-printed"
	path := filepath.Join(t.TempDir(), "reviewer-token-1")
	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	// A successful lookup returns the value but must not put it in the path it reports.
	stubMinter(t, path, "", nil)
	tok, gotPath, err := RoleTokenForOwner("reviewer", "example-org")
	if err != nil {
		t.Fatal(err)
	}
	if tok != secret {
		t.Fatalf("token = %q, want the file's contents", tok)
	}
	if strings.Contains(gotPath, secret) {
		t.Fatal("the reported token PATH carries the token VALUE")
	}
	// A file that was read and then found unusable must complain about the PATH, never
	// about its contents.
	if err := os.WriteFile(path, []byte("  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = RoleTokenForOwner("reviewer", "example-org")
	if err == nil {
		t.Fatal("an unusable token file was accepted")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("the refusal does not name the token path an operator has to fix: %v", err)
	}
}

// A mint that fails without naming a path still refuses, and the refusal says which role
// and which account it was for — the two things an operator needs to find the credential.
func TestMintFailureNamesRoleAndAccount(t *testing.T) {
	stubMinter(t, "", "no installation id for role \"reviewer\"", os.ErrPermission)
	_, _, err := RoleTokenForOwner("reviewer", "example-org")
	if err == nil {
		t.Fatal("a failing mint returned no error")
	}
	for _, want := range []string{"reviewer", "example-org"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("mint refusal does not name %q: %v", want, err)
		}
	}
}
