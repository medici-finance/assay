package main

// evidenceactor_gitlab1477_test.go — the OFFLINE fallback for a GitLab verifier whose commit
// carries the account DISPLAY name while the roster binds the USERNAME (#1477).
//
// THE DISCRIMINATING SCENARIO (issue #1477): a GitLab service account has a username AND a
// separate display name, and every commit GitLab writes through the API carries the DISPLAY
// name in author_name. The roster binds the account by username (verifier=gitlab:<username>:<id>),
// the field the commit does NOT carry, so the Evidence-actor gate's name-vs-username compare
// could never succeed and `implemented -> verified` was permanently blocked on GitLab. The
// offline fallback declares the display name in ASSAY_GITLAB_DISPLAY_NAMES so the gate has a
// value to compare against.
//
// The tests below are FAIL-FIRST against the pre-fix classifyGitLab (which matched the username
// only): the display-name commit read as actorRejected, and the roster's declared display name
// was never consulted. The negative controls keep this from degrading into "everything backed":
// a DIFFERENT service account, and a display name that matches nothing, both still reject.

import (
	"strings"
	"testing"
)

const (
	// gl1477DisplayName mirrors the issue's real value: spaces AND parentheses, exactly the
	// shape the comma/space list splitter would shred and the exact-username compare could
	// never match.
	gl1477DisplayName = "Example Verifier (fleet bot)"
)

// TestEvidenceActorGitLabDisplayNameOffline is the issue's exact scenario: a service-account
// commit whose author NAME is the account's DISPLAY name (not its username) reads as BACKED
// once the roster declares that display name — and stays REJECTED when it is not declared.
func TestEvidenceActorGitLabDisplayNameOffline(t *testing.T) {
	// The bound GitLab verifier, WITH a declared display name (the offline fallback armed).
	p := evidenceActorPolicy{
		Verifier:            actorRef{Login: glVerifierName},
		VerifierForge:       forgeGitLab,
		VerifierDisplayName: gl1477DisplayName,
	}

	// FAIL-FIRST: pre-fix, classifyGitLab compared the display name against the username only,
	// so this returned actorRejected. With the fallback it is BACKED.
	if v, reason := p.classify(gl1477DisplayName, glVerifierEmail); v != actorVerifier {
		t.Fatalf("a service-account commit carrying the DECLARED display name must be BACKED, got %v (%s)", v, reason)
	} else if !strings.Contains(reason, "display name") {
		t.Errorf("the accept reason must name the display-name match so a reader can tell it from the username match; got: %s", reason)
	}

	// The exact-username match still works (ACCEPT 1 unchanged) — a deployment whose admin set
	// the display name equal to the username.
	if v, _ := p.classify(glVerifierName, glVerifierEmail); v != actorVerifier {
		t.Errorf("the exact-username match must still back a row, got %v", v)
	}

	// NEGATIVE CONTROL 1: a DIFFERENT service account under a name matching neither the
	// username nor the declared display name stays unbacked (not "everything backed").
	if v, _ := p.classify("Some Other Bot", glWorkerEmail); v != actorRejected {
		t.Errorf("a service-account commit matching neither the username nor the display name must be unbacked, got %v", v)
	}

	// NEGATIVE CONTROL 2: with NO display name declared, the same display-name commit rejects,
	// AND the rejection names the display-name-vs-username gap and the remedy (suggestion 3).
	noDN := evidenceActorPolicy{Verifier: actorRef{Login: glVerifierName}, VerifierForge: forgeGitLab}
	v, reason := noDN.classify(gl1477DisplayName, glVerifierEmail)
	if v != actorRejected {
		t.Fatalf("with no declared display name the display-name commit must reject, got %v", v)
	}
	for _, want := range []string{"DISPLAY name", "USERNAME", scanEnvGitLabDisplayNames} {
		if !strings.Contains(reason, want) {
			t.Errorf("the rejection must name the display-name-vs-username gap and the remedy (%q); got: %s", want, reason)
		}
	}
}

// TestEvidenceActorGitLabHumanNoreply is the human half of #1477: a roster-known human who
// verified on a GitLab deployment commits under GitLab's PRIVATE commit noreply address
// (`<id>-<username>@users.noreply.<host>`), which the pre-fix GitLab path did not recognise —
// so a GitLab human verifier was rejected exactly as the service account was.
func TestEvidenceActorGitLabHumanNoreply(t *testing.T) {
	p := evidenceActorPolicy{
		Verifier:      actorRef{Login: glVerifierName},
		VerifierForge: forgeGitLab,
		Humans:        []actorRef{{Login: "ada", ID: 100001}},
	}

	// FAIL-FIRST: pre-fix, only the GitHub noreply form was honoured, so this rejected.
	if v, reason := p.classify("Ada Lovelace", "100001-ada@users.noreply.gitlab.com"); v != actorHuman {
		t.Fatalf("a roster-known human on GitLab's private commit address must back a row, got %v (%s)", v, reason)
	}

	// A self-hosted instance's custom hostname matches too (the host is general).
	if v, _ := p.classify("Ada", "100001-ada@users.noreply.gitlab.example.com"); v != actorHuman {
		t.Errorf("a self-hosted GitLab private commit address must also back a row, got %v", v)
	}

	// NEGATIVE CONTROL: the id-pinned match refuses a re-registered username on a DIFFERENT id.
	if v, _ := p.classify("Not Ada", "999999-ada@users.noreply.gitlab.com"); v != actorRejected {
		t.Errorf("the id must pin the human match — a different id under the same username must reject, got %v", v)
	}

	// The GitLab private address must NOT be confused with the service-account form, and a
	// human's private address must not be read as the verifier.
	if v, _ := p.classify(glVerifierName, "100001-ada@users.noreply.gitlab.com"); v == actorVerifier {
		t.Errorf("a human's private commit address must never be accepted as the verifier service account")
	}
}

// TestGitlabHumanIdentityFromEmail pins the address parse, including the disjointness from the
// service-account shape (neither form may be read as the other).
func TestGitlabHumanIdentityFromEmail(t *testing.T) {
	cases := []struct {
		email    string
		wantUser string
		wantID   int64
		wantOK   bool
	}{
		{"100001-ada@users.noreply.gitlab.com", "ada", 100001, true},
		{"42-some.user_name@users.noreply.gitlab.example.com", "some.user_name", 42, true},
		// A service-account address is NOT a human private address.
		{glVerifierEmail, "", 0, false},
		// A GitHub noreply address is not a GitLab private address.
		{"300000005+assay-verifier-app[bot]@users.noreply.github.com", "", 0, false},
		// A plain address pins nobody.
		{"ada@example.com", "", 0, false},
		// A zero id is refused.
		{"0-ada@users.noreply.gitlab.com", "", 0, false},
	}
	for _, c := range cases {
		user, id, ok := gitlabHumanIdentityFromEmail(c.email)
		if ok != c.wantOK || user != c.wantUser || id != c.wantID {
			t.Errorf("gitlabHumanIdentityFromEmail(%q) = (%q,%d,%v), want (%q,%d,%v)",
				c.email, user, id, ok, c.wantUser, c.wantID, c.wantOK)
		}
	}
}

// TestEvidenceActorPolicyCarriesGitLabDisplayName drives the whole roster load path: an
// ASSAY_GITLAB_DISPLAY_NAMES entry for the bound GitLab verifier lands on the policy's
// VerifierDisplayName, and a display-name commit then classifies BACKED end to end.
func TestEvidenceActorPolicyCarriesGitLabDisplayName(t *testing.T) {
	roster := gitlabRoster()
	roster[scanEnvGitLabDisplayNames] = glVerifierName + "=" + gl1477DisplayName
	scanWithRoster(t, roster)

	p := evidenceActorPolicyFromRoster()
	if p.Unavailable != "" {
		t.Fatalf("policy must be available: %s", p.Unavailable)
	}
	if p.VerifierDisplayName != gl1477DisplayName {
		t.Fatalf("the roster's declared display name must reach the policy; got %q", p.VerifierDisplayName)
	}
	if v, _ := p.classify(gl1477DisplayName, glVerifierEmail); v != actorVerifier {
		t.Errorf("end to end: a display-name commit must be BACKED once the roster declares the display name, got %v", v)
	}
}

// TestGitLabDisplayNameOnlyForGitLabVerifier — a GitHub verifier never consults the display-name
// map (its display name is untrusted free text, per the file header), so the map is set only for
// a GitLab binding.
func TestGitLabDisplayNameOnlyForGitLabVerifier(t *testing.T) {
	scanWithRoster(t, map[string]string{
		scanEnvBlessLogin:         "ada:100001",
		scanEnvTrustedLogins:      "ada:100001",
		scanEnvTrustedBotSlugs:    "verifier=github:assay-verifier-app:300000005",
		scanEnvGitLabDisplayNames: "assay-verifier-app=Some Display Name",
	})
	p := evidenceActorPolicyFromRoster()
	if p.VerifierDisplayName != "" {
		t.Errorf("a GitHub verifier must not carry a display name from the GitLab map; got %q", p.VerifierDisplayName)
	}
}

// TestScanSplitDisplayNameEntries pins the space-tolerant splitter: it splits on `;`/newline
// only, so a display name keeps its spaces and parentheses, and a malformed entry is refused by
// the parser (fail-closed).
func TestScanSplitDisplayNameEntries(t *testing.T) {
	got := scanSplitDisplayNameEntries("a=Alice Example (bot); b=Bob\nc=Carol, Jr.")
	want := []string{"a=Alice Example (bot)", "b=Bob", "c=Carol, Jr."}
	if len(got) != len(want) {
		t.Fatalf("split = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("split[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestGitLabDisplayNamesMalformedRefuses — a display-name entry with no `=` (or an empty half)
// collapses the whole configuration, the same fail-closed direction a malformed slug takes,
// rather than silently leaving the offline fallback empty while reporting configured=true.
func TestGitLabDisplayNamesMalformedRefuses(t *testing.T) {
	roster := gitlabRoster()
	roster[scanEnvGitLabDisplayNames] = "no-equals-sign-here"
	scanReloadConfig()
	t.Cleanup(scanReloadConfig)
	scanWithRoster(t, roster)
	c := scanEffectiveConfig()
	if len(c.Problems) == 0 {
		t.Fatal("a malformed ASSAY_GITLAB_DISPLAY_NAMES entry must REFUSE the whole configuration")
	}
	joined := strings.Join(c.Problems, "\n")
	if !strings.Contains(joined, scanEnvGitLabDisplayNames) {
		t.Errorf("the refusal must name the offending key; got: %s", joined)
	}
}
