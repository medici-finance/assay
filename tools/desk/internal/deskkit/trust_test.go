package deskkit

import (
	"testing"
	"time"
)

func TestTrustedAuthor(t *testing.T) {
	cases := []struct {
		login string
		want  bool
	}{
		// Humans.
		{"ada", true},
		{"shared-agent", true},
		// GitHub logins are case-insensitively unique, so case variants are the SAME
		// account rendered differently — trusted.
		{"Ada", true},
		{"SHARED-AGENT", true},

		// The six desk Apps, REST rendering (<slug>[bot]).
		{"assay-desk-app[bot]", true},
		{"assay-reviewer-app[bot]", true},
		{"assay-verifier-app[bot]", true},
		{"assay-worker-app[bot]", true},
		{"assay-issue-loop-app[bot]", true},
		{"assay-intake-loop-app[bot]", true},
		// gh CLI rendering (app/<slug>).
		{"app/assay-desk-app", true},
		{"app/assay-reviewer-app", true},
		{"app/assay-verifier-app", true},
		{"app/assay-worker-app", true},
		{"app/assay-issue-loop-app", true},
		{"app/assay-intake-loop-app", true},
		{"App/Assay-Desk-App", true}, // case-insensitive on the rendered form

		// Fail closed: empty / unknown.
		{"", false},
		{"external-user", false},
		{"dependabot[bot]", false},
		{"github-actions[bot]", false},

		// Spoofing-shaped: the BARE slug is a registrable username namespace — never
		// trusted without the [bot] / app/ rendering only GitHub itself can produce.
		{"assay-desk-app", false},
		{"assay-reviewer-app", false},
		// Near-miss lookalikes.
		{"ada2", false},
		{"not-ada", false},
		{"ada[bot]", false},
		{"app/ada", false},
		{"assay-desk-app[bot]x", false},
		{"xapp/assay-desk-app", false},
		{" ada", false}, // no trimming — exact rendered form only
		{"assay-desk-app [bot]", false},
	}
	for _, c := range cases {
		t.Run(c.login, func(t *testing.T) {
			if got := TrustedAuthor(c.login); got != c.want {
				t.Fatalf("TrustedAuthor(%q) = %v, want %v", c.login, got, c.want)
			}
		})
	}
}

func TestIsBlessAuthority(t *testing.T) {
	cases := []struct {
		login string
		want  bool
	}{
		{"ada", true},
		{"Ada", true}, // same GitHub account, different rendering
		{"", false},
		{"ada2", false},
		{"shared-agent", false},        // trusted author, but NOT the blessing authority
		{"assay-desk-app[bot]", false}, // desk identities cannot bless
	}
	for _, c := range cases {
		if got := IsBlessAuthority(c.login); got != c.want {
			t.Errorf("IsBlessAuthority(%q) = %v, want %v", c.login, got, c.want)
		}
	}
}

func TestItemTrusted(t *testing.T) {
	cases := []struct {
		name           string
		author         string
		commentAuthors []string
		want           bool
	}{
		{"trusted author, no comments", "ada", nil, true},
		{"desk App author (REST form)", "assay-worker-app[bot]", nil, true},
		{"desk App author (gh CLI form)", "app/assay-worker-app", nil, true},
		{"untrusted author, no comments -> fail closed", "external-user", nil, false},
		{"empty author, no comments -> fail closed", "", nil, false},
		{"untrusted author, ada commented -> blessed", "external-user", []string{"someone", "ada"}, true},
		{"empty author, ada commented -> blessed", "", []string{"ada"}, true},
		{"untrusted author, only untrusted commenters", "external-user", []string{"someone", "someone-else"}, false},

		// A desk identity commenting does NOT bless an external item — only ada's
		// comment authorship admits it.
		{"untrusted author, desk App commented -> still untrusted",
			"external-user", []string{"assay-desk-app[bot]", "app/assay-reviewer-app"}, false},
		{"untrusted author, shared-agent commented -> still untrusted",
			"external-user", []string{"shared-agent"}, false},

		// Spoofing-shaped: an untrusted user whose comment MENTIONS ada is not a
		// blessing — only comment AUTHORSHIP counts, and the author login here is a
		// lookalike, not ada.
		{"comment author is a ada lookalike, not ada",
			"external-user", []string{"ada-fan", "notada", "ada2", "ada "}, false},
		{"empty comment author entries fail closed", "external-user", []string{"", ""}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ItemTrusted(c.author, c.commentAuthors); got != c.want {
				t.Fatalf("ItemTrusted(%q, %v) = %v, want %v", c.author, c.commentAuthors, got, c.want)
			}
		})
	}
}

// TestTrustedLoginsComplete pins the full compiled-in set — 2 humans + 6 App slugs in
// both rendered forms = 14 logins. A drift here (slug rename, App added/removed) must
// be a conscious edit to BOTH trust.go and this expectation.
func TestTrustedLoginsComplete(t *testing.T) {
	want := []string{
		"ada",
		"app/assay-desk-app",
		"app/assay-intake-loop-app",
		"app/assay-issue-loop-app",
		"app/assay-reviewer-app",
		"app/assay-verifier-app",
		"app/assay-worker-app",
		"assay-desk-app[bot]",
		"assay-intake-loop-app[bot]",
		"assay-issue-loop-app[bot]",
		"assay-reviewer-app[bot]",
		"assay-verifier-app[bot]",
		"assay-worker-app[bot]",
		"shared-agent",
	}
	got := TrustedLogins()
	if len(got) != len(want) {
		t.Fatalf("TrustedLogins() has %d entries, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("TrustedLogins()[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
	for _, l := range want {
		if !TrustedAuthor(l) {
			t.Errorf("TrustedAuthor(%q) = false for a listed trusted login", l)
		}
	}
}

// ts is a compact RFC3339 helper for the bless-then-edit tables.
func ts(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad test timestamp %q: %v", s, err)
	}
	return v
}

// TestBlessed covers the bless-then-edit hole: a ada comment
// only blesses content that existed when he commented.
func TestBlessed(t *testing.T) {
	t1 := "2026-07-20T10:00:00Z" // author's comment
	t2 := "2026-07-21T10:00:00Z" // ada's blessing
	t3 := "2026-07-22T10:00:00Z" // after the blessing

	cases := []struct {
		name       string
		bodyEdited string // "" = never edited
		events     []ContentEvent
		want       bool
	}{
		{"no ada comment -> fail closed", "", []ContentEvent{
			{Author: "external-user", CreatedAt: ts(t, t1)},
		}, false},
		// NOTE: production GraphQL always supplies databaseId for a User-typed author
		// (PRTrustQuery/IssueTrustQuery request `...on User{databaseId}` on every node)
		// — ada's events below carry AuthorID: fixtureBlessID to match that reality.
		// trustedContentAuthor (the untrusted-content-loop check) requires a numeric id
		// match for human logins; an id-less "ada" event is untrusted content and
		// self-voids on the tie (see "id-less ada event cannot bless" below).
		{"ada commented, body never edited -> blessed", "", []ContentEvent{
			{Author: "external-user", CreatedAt: ts(t, t1)},
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
		}, true},
		{"body edited BEFORE the blessing -> blessed (ada saw the edit)", t1, []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
		}, true},
		{"body edited AFTER the blessing -> re-quarantined", t3, []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
		}, false},
		// T3, the TIE. GitHub timestamps are second-granular, so a body edit landing in
		// the same second as the blessing is indistinguishable from one landing just
		// after it — and Blessed uses `!bodyEditedAt.Before(bless)` rather than
		// `.After(bless)` precisely so the tie VOIDS. statusgen's copy of this rule
		// (evalIssueBlessing) is pinned by TestEvalIssueBlessingTie; this copy was not,
		// and it is the copy with the wider blast radius — deskkit.Blessed is the live
		// gate for deskboard's quarantine, issueboard, and deskpost's pre-flip trust
		// check, while the statusgen copy guards only `--scan-issues`. Relaxing `!Before`
		// to `.After` here survived the entire suite.
		{"body edited IN THE SAME SECOND as the blessing -> re-quarantined (T3 tie)", t2, []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
		}, false},
		// blessed-then-labeled: a label move touches REST updated_at but NOT
		// lastEditedAt — callers pass zero bodyEdited, so the item stays trusted.
		{"blessed then labeled (no content edit) -> stays blessed", "", []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
		}, true},
		{"untrusted comment EDITED after the blessing -> re-quarantined", "", []ContentEvent{
			{Author: "external-user", CreatedAt: ts(t, t1), EditedAt: ts(t, t3)},
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
		}, false},
		{"untrusted comment ADDED after the blessing -> re-quarantined", "", []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
			{Author: "someone-else", CreatedAt: ts(t, t3)},
		}, false},
		{"TRUSTED comment after the blessing -> stays blessed", "", []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
			{Author: "assay-desk-app[bot]", CreatedAt: ts(t, t3)},
			{Author: "shared-agent", AuthorID: 2002, CreatedAt: ts(t, t3)},
		}, true},
		{"ada edits his OWN comment after blessing -> stays blessed", "", []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2), EditedAt: ts(t, t3)},
		}, true},
		{"fresh ada comment AFTER the edit re-blesses", t3, []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, "2026-07-23T10:00:00Z")},
		}, true},
		// id-aware blessing: a "ada" login with the WRONG numeric id is not ada
		// (login-recycling defense) — no blessing.
		{"ada login with wrong numeric id cannot bless", "", []ContentEvent{
			{Author: "ada", AuthorID: 999999, CreatedAt: ts(t, t2)},
		}, false},
		{"ada login with the pinned numeric id blesses", "", []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
		}, true},
		// A trusted-LOGIN event with a mismatched id is UNTRUSTED content: after the
		// blessing it re-quarantines like any other untrusted event.
		{"desk-App login with wrong id after blessing re-quarantines", "", []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
			{Author: "assay-desk-app[bot]", AuthorID: 424242, CreatedAt: ts(t, t3)},
		}, false},
		// trustedContentAuthor divergence closed (#315 review, "deskkit
		// KEEP-IN-SYNC divergence"): an id-less "ada" event used to be treated as
		// trusted content by the general-purpose TrustedAuthorID (id==0 login-only
		// fallback) and would NOT re-quarantine if it arrived after a real blessing.
		// statusgen's analogous check (trustedAuthorID, post-T10) already required a
		// numeric id match for human logins; this proves the two copies now agree.
		{"id-less ada event after a real blessing re-quarantines (was trusted content pre-fix)", "", []ContentEvent{
			{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, t2)},
			{Author: "ada", CreatedAt: ts(t, t3)}, // AuthorID omitted -> 0
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var bodyEdited time.Time
			if c.bodyEdited != "" {
				bodyEdited = ts(t, c.bodyEdited)
			}
			if got := Blessed(bodyEdited, c.events); got != c.want {
				t.Fatalf("Blessed(%v, %+v) = %v, want %v", bodyEdited, c.events, got, c.want)
			}
		})
	}
}

// TestItemTrustedEvents: trusted authorship needs no blessing; everything else runs
// the edit-aware blessing.
func TestItemTrustedEvents(t *testing.T) {
	after := ts(t, "2026-07-22T10:00:00Z")
	bless := []ContentEvent{{Author: "ada", AuthorID: fixtureBlessID, CreatedAt: ts(t, "2026-07-21T10:00:00Z")}}

	if !ItemTrustedEvents("shared-agent", after, nil) {
		t.Error("trusted author must be admitted regardless of edit timestamps")
	}
	if !ItemTrustedEvents("external-user", time.Time{}, bless) {
		t.Error("ada-blessed unedited item must be admitted")
	}
	if ItemTrustedEvents("external-user", after, bless) {
		t.Error("body edited after the blessing must re-quarantine")
	}
	if ItemTrustedEvents("external-user", time.Time{}, nil) {
		t.Error("no events, untrusted author -> fail closed")
	}
}

// TestTrustedAuthorID pins the id-aware identity checks (login recycling defense).
func TestTrustedAuthorID(t *testing.T) {
	cases := []struct {
		name  string
		login string
		id    int64
		want  bool
	}{
		{"ada with pinned id", "ada", 2001, true},
		{"ada without id (surface lacks it)", "ada", 0, true},
		{"ada with WRONG id -> recycled login, untrusted", "ada", 31337, false},
		{"shared-agent with pinned id", "shared-agent", 2002, true},
		{"shared-agent with wrong id", "shared-agent", 1, false},
		{"desk App REST form with pinned bot id", "assay-desk-app[bot]", 300000001, true},
		{"desk App gh CLI form with pinned bot id", "app/assay-desk-app", 300000001, true},
		{"desk App with wrong id", "assay-desk-app[bot]", 300000004, false}, // reviewer's id on desk's login
		{"untrusted login with any id", "external-user", 12345, false},
		{"empty login", "", 2001, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TrustedAuthorID(c.login, c.id); got != c.want {
				t.Fatalf("TrustedAuthorID(%q, %d) = %v, want %v", c.login, c.id, got, c.want)
			}
		})
	}
}

func TestIsBlessAuthorityID(t *testing.T) {
	if !IsBlessAuthorityID("ada", 0) {
		t.Error("IsBlessAuthorityID(ada, 0) must be true (no id available)")
	}
	if !IsBlessAuthorityID("Ada", fixtureBlessID) {
		t.Error("IsBlessAuthorityID with the pinned id must be true")
	}
	if IsBlessAuthorityID("ada", 42) {
		t.Error("IsBlessAuthorityID with a wrong id must be false (recycled-login defense)")
	}
	if IsBlessAuthorityID("someone", fixtureBlessID) {
		t.Error("IsBlessAuthorityID with the right id but wrong login must be false")
	}
}

// RoleAppLogin's ok return carries a construction invariant every caller relies on:
// ok==true implies a NON-EMPTY login. The matcher helpers in deskboard/deskpost/
// deskevidence are written against it — their empty-actor guards are belt, and the
// braces here are the load-bearing half. Without this the guard would be unfailable
// (with ok true and want non-empty, no actor login can equal "" anyway), which is
// exactly the shape that lets a defect pass green.
//
// CAN-FAIL: drop the `strings.TrimSpace(slug) == ""` clause from RoleAppLogin and this
// returns ("[bot]", true) — an empty binding rendered as a real identity.
func TestRoleAppLoginNeverReturnsOKWithEmptyLogin(t *testing.T) {
	cfgMu.Lock()
	prevVal, prevOnce := cfgValue, cfgOnce
	cfgValue = Config{
		Source:   "test",
		Bless:    Identity{Login: "ada", ID: 1001},
		Logins:   map[string]bool{"ada": true},
		RoleBots: map[string]string{"reviewer": "", "verifier": "   "},
	}
	cfgOnce = true
	cfgMu.Unlock()
	t.Cleanup(func() {
		cfgMu.Lock()
		cfgValue, cfgOnce = prevVal, prevOnce
		cfgMu.Unlock()
	})

	if !EffectiveConfig().Configured() {
		t.Fatal("precondition: this Config must be CONFIGURED — an unconfigured one refuses " +
			"before the binding is ever consulted, and the test would prove nothing")
	}
	for _, role := range []string{"reviewer", "verifier"} {
		login, ok := RoleAppLogin(role)
		if ok {
			t.Errorf("RoleAppLogin(%q) = (%q, true) for an EMPTY binding — ok must never be true "+
				"with a login the callers would then compare against", role, login)
		}
		if login != "" {
			t.Errorf("RoleAppLogin(%q) returned login %q with ok=false; it must be empty", role, login)
		}
		if RoleBound(role) {
			t.Errorf("RoleBound(%q) must be false for an empty binding", role)
		}
		if err := RequireRole(role); err == nil {
			t.Errorf("RequireRole(%q) must refuse for an empty binding", role)
		}
	}
}
