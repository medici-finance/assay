package deskkit

import (
	"errors"
	"testing"
)

// stubAccountFetcher is a test-only AccountFetcher: a map of login -> canned (Account,
// error), so each test drives the exact decision-table cell it names.
type stubAccountFetcher struct {
	accounts map[string]*Account
	errs     map[string]error
}

func (s *stubAccountFetcher) GetAccount(login string) (*Account, error) {
	if err, ok := s.errs[login]; ok {
		return nil, err
	}
	if acct, ok := s.accounts[login]; ok {
		return acct, nil
	}
	return nil, errors.New("stubAccountFetcher: no fixture for " + login)
}

var _ AccountFetcher = (*stubAccountFetcher)(nil)

// TestCheckRosterLivenessClassifiesEveryCase drives the full decision table — Alive,
// Renamed, Reclaimed, Deleted, Unpinned, CouldNotCheck — through one mixed batch, so the
// mixed-batch case (one identity errors, the others still get their own findings) is
// covered in the same run rather than a separate test that could pass in isolation while
// the batch path drops findings.
func TestCheckRosterLivenessClassifiesEveryCase(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "alive-human", PinnedID: 1001, Source: "human"},
		{Login: "renamed-human", PinnedID: 1002, Source: "human"},
		{Login: "reclaimed-bless", PinnedID: 2001, Source: "bless"},
		{Login: "deleted-bot", PinnedID: 3001, Source: "bot"},
		{Login: "unpinned-human", PinnedID: 0, Source: "human"},
		{Login: "transport-error-human", PinnedID: 4001, Source: "human"},
	}

	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			"alive-human":     {Login: "alive-human", ID: 1001},
			"renamed-human":   {Login: "renamed-human-new-name", ID: 1002},
			"reclaimed-bless": {Login: "reclaimed-bless", ID: 9999}, // different id
			"unpinned-human":  {Login: "unpinned-human", ID: 5555},
		},
		errs: map[string]error{
			"deleted-bot":           ErrAccountNotFound,
			"transport-error-human": errors.New("500 internal server error"),
		},
	}

	findings := CheckRosterLiveness(fetcher, identities)
	if len(findings) != len(identities) {
		t.Fatalf("got %d findings, want %d — one identity's error must never drop another's finding",
			len(findings), len(identities))
	}

	want := map[string]LivenessClass{
		"alive-human":           LivenessAlive,
		"renamed-human":         LivenessRenamed,
		"reclaimed-bless":       LivenessReclaimed,
		"deleted-bot":           LivenessDeleted,
		"unpinned-human":        LivenessUnpinned,
		"transport-error-human": LivenessCouldNotCheck,
	}
	got := make(map[string]LivenessClass, len(findings))
	for _, f := range findings {
		got[f.Identity.Login] = f.Class
	}
	for login, wantClass := range want {
		if got[login] != wantClass {
			t.Errorf("login %q classified %q, want %q", login, got[login], wantClass)
		}
	}

	// A transport error on ONE identity must not suppress findings for the others: every
	// other configured identity in the same batch is still present and correctly classified
	// (already checked via `got` above), not just non-empty.
	if len(got) != len(want) {
		t.Fatalf("got findings for %d distinct logins, want %d", len(got), len(want))
	}
}

// TestCheckRosterLivenessNeverReportsAliveOnIDMismatch is the NEGATIVE control: an id
// mismatch (a different account now answering to a trusted login) must never classify
// Alive, whatever the returned login string looks like.
func TestCheckRosterLivenessNeverReportsAliveOnIDMismatch(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "squatted-login", PinnedID: 42, Source: "human"},
	}
	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			// Same login spelling, but a DIFFERENT id — a classic account-reclaim: the old
			// account was deleted and a new one registered under the same name.
			"squatted-login": {Login: "squatted-login", ID: 99999},
		},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Class == LivenessAlive {
		t.Fatalf("an id mismatch (pinned 42, live 99999) classified Alive — the negative control this test exists for")
	}
	if findings[0].Class != LivenessReclaimed {
		t.Fatalf("id mismatch classified %q, want LivenessReclaimed", findings[0].Class)
	}
}

// TestRosterIdentitiesEnumeratesHumansBlessAndBotsOnly confirms RosterIdentities reads
// exactly the three GitHub-scoped maps the brief names, in deterministic order, and skips
// an unconfigured Bless entry (zero value).
func TestRosterIdentitiesEnumeratesHumansBlessAndBotsOnly(t *testing.T) {
	cfg := Config{
		Humans: map[string]int64{"zed": 3, "ada": 1},
		Bless:  Identity{Login: "ada", ID: 1},
		Bots:   map[string]int64{"zzz-app": 900, "aaa-app": 800},
	}
	ids := RosterIdentities(cfg)
	var logins []string
	for _, id := range ids {
		logins = append(logins, id.Login+":"+id.Source)
	}
	want := []string{"ada:human", "zed:human", "ada:bless", "aaa-app:bot", "zzz-app:bot"}
	if len(logins) != len(want) {
		t.Fatalf("got %v, want %v", logins, want)
	}
	for i := range want {
		if logins[i] != want[i] {
			t.Fatalf("got %v, want %v", logins, want)
		}
	}
}

// TestRosterIdentitiesSkipsUnconfiguredBless confirms a zero-value Bless (unconfigured)
// contributes no identity — the brief's "when c.Bless.Login != ”" condition.
func TestRosterIdentitiesSkipsUnconfiguredBless(t *testing.T) {
	cfg := Config{Humans: map[string]int64{"ada": 1}}
	ids := RosterIdentities(cfg)
	for _, id := range ids {
		if id.Source == "bless" {
			t.Fatalf("unconfigured Bless produced an identity: %+v", id)
		}
	}
}

// TestRenderLivenessNoticesQuietOnAlive confirms an Alive finding produces no line, and
// that every other class produces a DISTINGUISHABLE line (different leading shape), so a
// reader (or a grep) can tell the classes apart without parsing the Detail text.
func TestRenderLivenessNoticesQuietOnAlive(t *testing.T) {
	findings := []LivenessFinding{
		{Identity: RosterIdentity{Login: "a"}, Class: LivenessAlive},
		{Identity: RosterIdentity{Login: "b"}, Class: LivenessRenamed, Detail: "d"},
		{Identity: RosterIdentity{Login: "c"}, Class: LivenessReclaimed, Detail: "d"},
		{Identity: RosterIdentity{Login: "d"}, Class: LivenessDeleted, Detail: "d"},
		{Identity: RosterIdentity{Login: "e"}, Class: LivenessUnpinned, Detail: "d"},
		{Identity: RosterIdentity{Login: "f"}, Class: LivenessCouldNotCheck, Detail: "d"},
	}
	lines := RenderLivenessNotices(findings)
	if len(lines) != 5 {
		t.Fatalf("got %d NOTICE lines, want 5 (Alive must produce none): %v", len(lines), lines)
	}
	seenShapes := map[string]bool{}
	for _, l := range lines {
		if len(l) < len("NOTICE: X") {
			t.Fatalf("line too short to carry a distinguishable shape: %q", l)
		}
		seenShapes[l[:len("NOTICE: X")]] = true
	}
	if len(seenShapes) != 5 {
		t.Fatalf("expected 5 distinguishable NOTICE shapes, got %d from: %v", len(seenShapes), lines)
	}
}
