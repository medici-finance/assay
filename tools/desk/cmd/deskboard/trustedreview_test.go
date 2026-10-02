package main

// #2028 — a PR authored by a trusted or blessed human is reviewed like any other PR.
//
// The retired #177 HUMAN-OWNED arm routed a trust-gate-admitted PR OUT of the review
// dispatch line on author identity alone: a trusted human's own open PR with no desk review
// at head got a NO-OP row instead of NEEDS-REVIEW, and was kept out of the UNREVIEWED
// neglect count. #2028 removes that arm. The trust gate (deskkit.TrustedAuthor, then a
// current blessing) is now the ONLY authorship filter on the board: a PR that clears it is
// dispatched like any other, and a PR that does not is quarantined-visible in EXTERNAL /
// UNBLESSED with no ACTION row, exactly as before.
//
// The tests below pin both halves:
//
//   - the promotion: a trusted human, a trusted shared login, a role App and a BLESSED
//     outsider each get NEEDS-REVIEW (RE-REVIEW on an advanced head) and count toward the
//     UNREVIEWED alarm;
//   - the fail-closed floor: no UNBLESSED author — whatever its spelling (prefix, suffix,
//     whitespace, `[bot]`, bare App slug, look-alike, empty), whatever free text it puts in
//     its title, and whoever ELSE comments — is ever given an ACTION row, and an unreadable
//     trust read or an unloadable roster promotes nothing.
//
// cmd/deskboard/mutations.json plants each of those bugs; every one must turn a test here red.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reviewablePR is a 12-hour-old non-draft PR #7, CI green, mergeable — the exact shape that
// is NEEDS-REVIEW (and past the UNREVIEWED threshold) when nothing has reviewed it. If an
// author's PR in this shape ever produces an ACTION row, it has been promoted into dispatch.
func reviewablePR(author, title string) string {
	a, _ := json.Marshal(author)
	ti, _ := json.Marshal(title)
	return `{"number":7,"title":` + string(ti) + `,"body":"","isDraft":false,` +
		`"author":{"login":` + string(a) + `},"createdAt":"` +
		time.Now().Add(-12*time.Hour).UTC().Format(time.RFC3339) + `",` +
		`"labels":[],"headRefOid":"aaa111","headRefName":"b","mergeStateStatus":"CLEAN",` +
		`"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`
}

// boardRun runs `actions` against one PR on repo and returns the exit code and the parsed
// report (zero report when the run failed).
func boardRun(t *testing.T, repo, prJSON string) (int, actionsReport, string) {
	t.Helper()
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", "["+prJSON+"]")
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"README.md"}]`)
	t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
	var out, errb bytes.Buffer
	code := run([]string{"actions"}, &out, &errb)
	var rep actionsReport
	if code == deskkit.ExitOK {
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
		}
	}
	return code, rep, errb.String()
}

// TestTrustedHumanNeedsReview — Verify: a trusted human (the bless authority + mapped
// human `ada`) with no desk review at head is NEEDS-REVIEW, dispatchable, and COUNTS toward
// the UNREVIEWED neglect alarm, exactly like the role-App, trusted-shared-login and
// blessed-outsider controls in the identical state. Red against the #177 code, where `ada`
// alone got HUMAN-OWNED and was excluded from the count.
func TestTrustedHumanNeedsReview(t *testing.T) {
	cases := []struct {
		name, repo, author string
		blessed            bool
	}{
		{"trusted human, private repo", privRepoFixture, "ada", false},
		{"trusted human, public repo", pubRepoFixture, "ada", false},
		{"trusted human, case variant", privRepoFixture, "Ada", false},
		{"trusted shared login", privRepoFixture, "shared-agent", false},
		{"role App", privRepoFixture, "app/assay-worker-app", false},
		{"blessed outsider, public repo", pubRepoFixture, "some-fork-account", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installFakeGH(t)
			if tc.blessed {
				t.Setenv("DESKBOARD_GH_GRAPHQL_JSON",
					gqlPRPayload("", nil, []string{gqlPRReview("ada", "User", 2001, "2026-07-21T10:00:00Z")}))
			}
			code, rep, stderr := boardRun(t, tc.repo, reviewablePR(tc.author, "a change"))
			if code != deskkit.ExitOK {
				t.Fatalf("run(actions) = exit %d, stderr=%s", code, stderr)
			}
			row := findRow(t, rep, 7)
			if row.Action != actNeedsReview {
				t.Fatalf("author %q: action = %s, want %s — a PR that cleared the trust gate is "+
					"reviewed like any other; authorship is not a reason to skip it (#2028)",
					tc.author, row.Action, actNeedsReview)
			}
			if rep.Header.UnreviewedCount != 1 {
				t.Errorf("author %q: a 12h-old unreviewed PR the desk now owns must count toward "+
					"UNREVIEWED; count=%d prs=%v", tc.author, rep.Header.UnreviewedCount, rep.Header.UnreviewedPRs)
			}
			if len(rep.External) != 0 {
				t.Errorf("author %q: a trusted/blessed author must not be in EXTERNAL; got %+v", tc.author, rep.External)
			}
		})
	}
}

// TestTrustedHumanReReview — Verify: a trusted human's PR reviewed at an older head whose
// own files then changed is RE-REVIEW — the delta re-review, never a skip.
func TestTrustedHumanReReview(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON", approvedReview("old000", "## Review\n\nVerdict: approve\n"))
	t.Setenv("DESKBOARD_GH_COMPARE_JSON", `{"files":[{"filename":"README.md"}]}`)
	code, rep, stderr := boardRun(t, privRepoFixture, reviewablePR("ada", "a change"))
	if code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, stderr)
	}
	if row := findRow(t, rep, 7); row.Action != actReReview {
		t.Fatalf("action = %s, want %s — a trusted human's advanced head is re-reviewed (#2028)\nnote: %s",
			row.Action, actReReview, row.Note)
	}
}

// TestUnblessedNeverDispatched — the fail-closed floor (#2028): no author outside the
// configured roster, without a CURRENT blessing from the configured blessing authority, is
// EVER given an ACTION row — on a public or a private repo — whatever its spelling. Each
// variant is one of the identity-confusion shapes a widened trust read would admit: a
// prefix/suffix of a trusted login, surrounding whitespace, a `[bot]` suffix on a human
// login, a role App's BARE slug (the username-squat shape — only the rendered `app/<slug>`
// and `<slug>[bot]` forms are trusted), a look-alike, and the empty login. The board's
// typed author carries no display-name field; the free text an outsider DOES control is
// the title, so a title naming a trusted login is pinned too. And a blessing that is not
// from the blessing authority (a trusted non-authority login, or the authority's login
// carrying the wrong numeric id) admits nothing.
func TestUnblessedNeverDispatched(t *testing.T) {
	type variant struct {
		name, author, title string
		gql                 string // trust-events fixture; "" = an empty thread
	}
	variants := []variant{
		{name: "unlisted fork account", author: "some-fork-account"},
		{name: "prefix of trusted login", author: "ada-evil"},
		{name: "suffix of trusted login", author: "evil-ada"},
		{name: "truncated trusted login", author: "ad"},
		{name: "leading whitespace", author: " ada"},
		{name: "trailing whitespace", author: "ada "},
		{name: "human login with [bot]", author: "ada[bot]"},
		{name: "app/ prefix on human", author: "app/ada"},
		{name: "bare App slug squat", author: "assay-worker-app"},
		{name: "bare reviewer slug squat", author: "assay-reviewer-app"},
		{name: "App slug suffix variant", author: "app/assay-worker-app-evil"},
		{name: "look-alike (Cyrillic a)", author: "аda"},
		{name: "empty author", author: ""},
		{name: "title names a trusted login", author: "some-fork-account", title: "ada"},
		{name: "blessed by a non-authority", author: "some-fork-account",
			gql: gqlPRPayload("", nil, []string{gqlPRReview("shared-agent", "User", 2002, "2026-07-21T10:00:00Z")})},
		{name: "authority login, wrong id", author: "some-fork-account",
			gql: gqlPRPayload("", nil, []string{gqlPRReview("ada", "User", 6666, "2026-07-21T10:00:00Z")})},
		{name: "blessed then edited", author: "some-fork-account",
			gql: gqlPRPayload("2026-07-22T10:00:00Z", nil, []string{gqlPRReview("ada", "User", 2001, "2026-07-21T10:00:00Z")})},
		// An overflowed thread: the authority's blessing is on page 1, but a later page was
		// never read — a revoking event could sit there, so the read is AMBIGUOUS and admits nothing.
		{name: "blessed, thread overflowed", author: "some-fork-account",
			gql: strings.Replace(gqlPRPayload("", nil, []string{gqlPRReview("ada", "User", 2001, "2026-07-21T10:00:00Z")}),
				`"reviews":{"pageInfo":{"hasNextPage":false}`, `"reviews":{"pageInfo":{"hasNextPage":true}`, 1)},
	}
	for _, repo := range []string{pubRepoFixture, privRepoFixture} {
		for _, v := range variants {
			t.Run(repo+"/"+v.name, func(t *testing.T) {
				installFakeGH(t)
				if v.gql != "" {
					t.Setenv("DESKBOARD_GH_GRAPHQL_JSON", v.gql)
				}
				title := v.title
				if title == "" {
					title = "a change"
				}
				code, rep, stderr := boardRun(t, repo, reviewablePR(v.author, title))
				if code != deskkit.ExitOK {
					t.Fatalf("run(actions) = exit %d, stderr=%s", code, stderr)
				}
				for _, r := range rep.Rows {
					if r.Number == 7 {
						t.Fatalf("UNBLESSED author %q was promoted to an ACTION row (%s) — only the "+
							"trust gate admits a PR to dispatch, and it must not admit this one (#2028)",
							v.author, r.Action)
					}
				}
				if rep.Header.UnreviewedCount != 0 {
					t.Errorf("an unblessed PR must not count toward UNREVIEWED; count=%d", rep.Header.UnreviewedCount)
				}
				quarantined := false
				for _, e := range rep.External {
					if e.Number == 7 {
						quarantined = true
					}
				}
				if !quarantined {
					t.Errorf("UNBLESSED author %q must stay quarantined-VISIBLE in EXTERNAL; got %+v", v.author, rep.External)
				}
			})
		}
	}
}

// TestTrustReadFailFailsClosed — an unreadable trust read is never a guess: when the
// blessing read for an untrusted author fails, the sweep fails (non-zero exit) and no
// ACTION row is produced for that PR.
func TestTrustReadFailFailsClosed(t *testing.T) {
	installFakeGH(t)
	installFakeTrustFail(t)
	code, rep, _ := boardRun(t, pubRepoFixture, reviewablePR("some-fork-account", "a change"))
	if code == deskkit.ExitOK {
		for _, r := range rep.Rows {
			if r.Number == 7 {
				t.Fatalf("an unreadable trust read promoted the PR to %s — must fail CLOSED", r.Action)
			}
		}
		t.Fatalf("an unreadable trust read must fail the sweep (could-not-check), not exit 0; rep=%+v", rep)
	}
}

// TestBadRosterPromotesNothing — a roster that cannot be read as configured (here: a
// malformed entry beside an otherwise-valid ada line, so the parse records a Problem) is
// "unconfigured", and an unconfigured roster trusts NOBODY: even the would-be bless
// authority's own PR is not promoted into dispatch.
func TestBadRosterPromotesNothing(t *testing.T) {
	installFakeGH(t)
	installBoardRoster(t, badTrustRoster)
	if deskkit.EffectiveConfig().Configured() {
		t.Fatal("precondition: the malformed roster must load as UNCONFIGURED")
	}
	// Inner layer, pinned directly: the board's outer refusal (below) would otherwise mask a
	// trust gate that fails OPEN on a half-read roster — ada IS in the parsed login set.
	for _, login := range []string{"ada", "shared-agent", "app/assay-worker-app"} {
		if deskkit.TrustedAuthor(login) {
			t.Errorf("TrustedAuthor(%q) = true on an UNCONFIGURED roster — the gate must fail CLOSED", login)
		}
	}
	code, rep, _ := boardRun(t, privRepoFixture, reviewablePR("ada", "a change"))
	if code != deskkit.ExitOK {
		return // outer layer: refusing to run at all is fail-closed too
	}
	for _, r := range rep.Rows {
		if r.Number == 7 && (r.Action == actNeedsReview || r.Action == actReReview) {
			t.Fatalf("an unreadable roster promoted ada's PR to %s — must fail CLOSED (#2028)", r.Action)
		}
	}
}

// badTrustRoster is the fixture roster with ONE malformed ASSAY_TRUSTED_LOGINS entry: every
// line the board needs is otherwise present (ada is still parsed into the login set), so
// only the Configured() check stands between the half-read roster and a trusted ada.
var badTrustRoster = strings.Replace(fixtureRoster,
	"ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002",
	"ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002,not a login:x", 1)

// installFakeTrustFail makes the board's trust-events read fail could-not-check.
func installFakeTrustFail(t *testing.T) {
	t.Helper()
	t.Setenv("DESKBOARD_GH_GRAPHQL_JSON", `{not json`)
}

// TestPrsVerbQuarantinesUnblessed — the `prs` verb carries its own copy of the trust gate
// (sweepPRsRepo); it must quarantine the same unblessed authors and fail closed on an
// unreadable trust read, so a consumer reading `prs` instead of `actions` cannot see an
// unblessed PR as an ordinary one.
func TestPrsVerbQuarantinesUnblessed(t *testing.T) {
	for _, author := range []string{"some-fork-account", "ada[bot]", "assay-worker-app", "ada-evil", ""} {
		t.Run(author, func(t *testing.T) {
			installFakeGH(t)
			t.Setenv("DESKBOARD_GH_PR_REPO", pubRepoFixture)
			t.Setenv("DESKBOARD_GH_PRLIST_JSON", "["+reviewablePR(author, "a change")+"]")
			var out, errb bytes.Buffer
			if code := run([]string{"prs"}, &out, &errb); code != deskkit.ExitOK {
				t.Fatalf("run(prs) = exit %d, stderr=%s", code, errb.String())
			}
			var rep prsReport
			if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
				t.Fatalf("parsing prs JSON: %v\n%s", err, out.String())
			}
			for _, r := range rep.PRs {
				if r.Number == 7 {
					t.Fatalf("UNBLESSED author %q got a `prs` row — it must stay in EXTERNAL", author)
				}
			}
			if len(rep.External) != 1 || rep.External[0].Number != 7 {
				t.Fatalf("UNBLESSED author %q must be quarantined-visible; external=%+v", author, rep.External)
			}
		})
	}
	t.Run("trust read fails", func(t *testing.T) {
		installFakeGH(t)
		installFakeTrustFail(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", pubRepoFixture)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON", "["+reviewablePR("some-fork-account", "a change")+"]")
		var out, errb bytes.Buffer
		if code := run([]string{"prs"}, &out, &errb); code == deskkit.ExitOK {
			t.Fatalf("an unreadable trust read must fail the `prs` sweep, not exit 0:\n%s", out.String())
		}
	})
}
