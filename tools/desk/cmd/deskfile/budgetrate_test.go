package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// This file covers assay#1204: the new-issue filing cap is a per-WINDOW RATE (not a
// per-session tally), env-fixable via ASSAY_DESKFILE_NEW_RATE / ASSAY_DESKFILE_NEW_WINDOW,
// with a --force-file --reason override that STILL files past a spent rate and leaves the
// audit trail intact. The tests use literal fixture counts (not the policy constant) so the
// same file compiles both before the fix (fail-first evidence) and after it.

// seedNewAuditAt appends n charged `new` ok entries dated `at` (so a window test can place
// them inside or outside a custom window). Mirrors seedNewAudit, which is now()-only.
func seedNewAuditAt(t *testing.T, n int, repo, session string, at time.Time) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	defer f.Close()
	for i := 0; i < n; i++ {
		num := 200 + i
		e := deskkit.Entry{
			TS:         at.UTC().Format(time.RFC3339),
			Tool:       "deskfile",
			Verb:       "new",
			Repo:       repo,
			PR:         &num,
			Result:     deskkit.ResultOK,
			Detail:     "created",
			SessionTag: session,
		}
		b, _ := json.Marshal(e)
		if _, werr := f.Write(append(b, '\n')); werr != nil {
			t.Fatalf("seed audit: %v", werr)
		}
	}
}

// chargedNewCount counts the audit `new` entries that consume the rate for (repo, session).
func chargedNewCount(t *testing.T, repo, session string) int {
	t.Helper()
	n := 0
	for _, e := range readAudit(t) {
		if e.Tool == "deskfile" && e.Verb == "new" && e.Repo == repo && e.SessionTag == session && chargedNewEntry(e) {
			n++
		}
	}
	return n
}

// (a) env RATE honored: ASSAY_DESKFILE_NEW_RATE lowers the cap, so a filing that would be
// within the shipped default of 3 is refused. RED on unfixed code (env ignored → 2nd new
// under a rate of 1 is admitted).
func TestNewRateEnvHonored(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("ASSAY_DESKFILE_NEW_RATE", "1")
	seedNewAudit(t, 1, allowedRepo, "test") // one already filed; rate of 1 is now spent
	body := bodyFileWith(t, "one over the env rate of 1")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "second filing under an env rate of one", "--body-file", body})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("2nd new under ASSAY_DESKFILE_NEW_RATE=1 rc = %d, want %d (env rate honored); out=%s",
			rc, deskkit.ExitRateLimited, out)
	}
	assertNoIssueCreate(t, *calls)
}

// (a) env RATE honored the other way: a HIGHER rate admits a 4th filing the shipped default
// of 3 would refuse. RED on unfixed code (env ignored → 4th refused).
func TestNewRateEnvRaisesCap(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("ASSAY_DESKFILE_NEW_RATE", "5")
	seedNewAudit(t, 3, allowedRepo, "test") // over the shipped default, under a rate of 5
	body := bodyFileWith(t, "the fourth filing, allowed by a raised env rate")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "fourth filing under an env rate of five", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("4th new under ASSAY_DESKFILE_NEW_RATE=5 rc = %d, want 0 (raised env rate); out=%s", rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
}

// (F1, security lane) an env-RAISED rate must leave a trace on the audit line: without it an
// entry filed under a raised rate is byte-identical to one filed under the shipped default, so
// the env path would launder over-filing as ordinary activity and defeat the anti-evasion
// property. RED on the code before the F1 fix (the env values leave no mark on Detail).
func TestNewRateEnvRaisedIsTracedInAudit(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("ASSAY_DESKFILE_NEW_RATE", "100") // differs from the shipped default of 3
	body := bodyFileWith(t, "a filing under a raised env rate")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "filing that must record its raised rate", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("new under a raised env rate rc = %d, want 0; out=%s", rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
	e := lastAudit(t)
	if !strings.Contains(e.Detail, "rate-config: 100 per") {
		t.Fatalf("audit detail must record the effective raised rate (rate-config: 100 per ... (env)); got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "(env)") {
		t.Fatalf("audit detail must mark the raised rate as env-sourced; got %q", e.Detail)
	}
}

// (F1) the shipped default must NOT tack a rate-config trace onto every line — the marker's
// signal is that the pace was RAISED from the default, so a default-config filing carries none.
// Green before and after (a guard on the marker's signal, not fail-first).
func TestNewRateDefaultLeavesNoRateConfigTrace(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	body := bodyFileWith(t, "a filing under the shipped default rate")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "default-rate filing carries no rate-config", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("new under the default rate rc = %d, want 0; out=%s", rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
	if e := lastAudit(t); strings.Contains(e.Detail, "rate-config:") {
		t.Fatalf("a default-config filing must carry no rate-config trace; got %q", e.Detail)
	}
}

// (a) unset → shipped fallback: with no env override the cap is the shipped default (3), so
// the 4th filing is refused. Green before and after — it guards that the fallback is the
// shipped default, not "no cap".
func TestNewRateUnsetUsesShippedDefault(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	seedNewAudit(t, 3, allowedRepo, "test")
	body := bodyFileWith(t, "the fourth filing with no env override")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "fourth filing, shipped default cap", "--body-file", body})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("4th new with no env rc = %d, want %d (shipped default cap)", rc, deskkit.ExitRateLimited)
	}
	assertNoIssueCreate(t, *calls)
}

// (a) unparseable → fallback + NOTICE, and it must NOT silently disable the cap. RED on
// unfixed code (no NOTICE is ever printed).
func TestNewRateUnparseableFallsBackWithNotice(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("ASSAY_DESKFILE_NEW_RATE", "banana")
	seedNewAudit(t, 3, allowedRepo, "test") // at the shipped-default cap the fallback must apply
	body := bodyFileWith(t, "a filing while the rate env is garbage")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "filing under an unparseable rate env", "--body-file", body})
	// The cap must NOT be silently disabled: at 3 charged the fallback default refuses the 4th.
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("4th new under ASSAY_DESKFILE_NEW_RATE=banana rc = %d, want %d — an unparseable "+
			"value must fall back to the shipped cap, not disable it; out=%s", rc, deskkit.ExitRateLimited, out)
	}
	if !strings.Contains(out, "NOTICE") || !strings.Contains(out, "banana") {
		t.Fatalf("an unparseable ASSAY_DESKFILE_NEW_RATE must print a NOTICE naming the bad value; out=%s", out)
	}
	assertNoIssueCreate(t, *calls)
}

// (a) unparseable WINDOW → fallback + NOTICE. RED on unfixed code (no NOTICE).
func TestNewWindowUnparseableFallsBackWithNotice(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("ASSAY_DESKFILE_NEW_WINDOW", "not-a-duration")
	seedNewAudit(t, 3, allowedRepo, "test")
	body := bodyFileWith(t, "a filing while the window env is garbage")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "filing under an unparseable window env", "--body-file", body})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("4th new under a garbage window env rc = %d, want %d (fallback window still counts); out=%s",
			rc, deskkit.ExitRateLimited, out)
	}
	if !strings.Contains(out, "NOTICE") || !strings.Contains(out, "not-a-duration") {
		t.Fatalf("an unparseable ASSAY_DESKFILE_NEW_WINDOW must print a NOTICE naming the bad value; out=%s", out)
	}
	assertNoIssueCreate(t, *calls)
}

// (a) env WINDOW honored: a wider window than the shipped 24h counts filings the default
// would have aged out. Seed 3 filings 48h old and widen the window to 720h — the rate is
// then spent. RED on unfixed code (24h window ages the 48h-old filings out → admitted).
func TestNewWindowEnvHonored(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("ASSAY_DESKFILE_NEW_WINDOW", "720h") // 30 days
	seedNewAuditAt(t, 3, allowedRepo, "test", time.Now().Add(-48*time.Hour))
	body := bodyFileWith(t, "a filing 48h after three, inside a 720h window")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "filing inside a widened window", "--body-file", body})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("new under ASSAY_DESKFILE_NEW_WINDOW=720h with three 48h-old filings rc = %d, "+
			"want %d (widened window still counts them); out=%s", rc, deskkit.ExitRateLimited, out)
	}
	assertNoIssueCreate(t, *calls)
}

// (b) the override files past a spent rate AND records the override + reason on the audit
// line. RED on unfixed code (--force-file is an unknown flag → refused exit 5).
func TestForceFileFilesPastSpentRateAndAudits(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	seedNewAudit(t, 3, allowedRepo, "test") // rate spent at the shipped default
	body := bodyFileWith(t, "an urgent filing after the rate is spent")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "urgent filing past the spent rate", "--body-file", body,
		"--force-file", "--reason", "human asked for this filed now; rate is spent"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--force-file past a spent rate rc = %d, want 0 (the override still files); out=%s", rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create` under --force-file; gh calls: %v", ghCalls(*calls))
	}
	e := lastAudit(t)
	if !strings.Contains(e.Detail, "force-file") {
		t.Fatalf("audit detail must record the override; got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "human asked for this filed now") {
		t.Fatalf("audit detail must record the reason; got %q", e.Detail)
	}
	// Identity: the audit line is tagged with the filing session (the trail names who overrode).
	if e.SessionTag != "test" {
		t.Fatalf("audit sessionTag = %q, want \"test\" (the override records the identity)", e.SessionTag)
	}
}

// (b) the override must NOT reset or erase the rate count: the next UNOVERRIDDEN new still
// sees the full history (the seeded three PLUS the forced one). RED on unfixed code
// (--force-file is unknown → the first override call is refused exit 5, not filed).
func TestForceFileDoesNotResetTheRateCount(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	seedNewAudit(t, 3, allowedRepo, "test")
	forceBody := bodyFileWith(t, "the forced filing")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "forced filing that must not reset the count", "--body-file", forceBody,
		"--force-file", "--reason", "urgent, rate spent"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--force-file rc = %d, want 0; out=%s", rc, out)
	}
	// The forced filing added to the count rather than clearing it: four charged now.
	if got := chargedNewCount(t, allowedRepo, "test"); got != 4 {
		t.Fatalf("charged count after force-file = %d, want 4 (override must not reset the trail)", got)
	}
	// A plain new is therefore STILL refused — the override raised the cap for ONE filing,
	// it did not reset the window's count.
	plainBody := bodyFileWith(t, "a plain filing right after the override")
	rc2, out2 := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "plain filing after the override", "--body-file", plainBody})
	if rc2 != deskkit.ExitRateLimited {
		t.Fatalf("plain new after force-file rc = %d, want %d (the count was not reset); out=%s",
			rc2, deskkit.ExitRateLimited, out2)
	}
	// Exactly ONE create happened across both calls — the force-file filing. The plain
	// follow-up filed nothing (the count was not reset by the override).
	if n := countCalls(ghCalls(*calls), "issue", "create"); n != 1 {
		t.Fatalf("issue-create count = %d, want 1 (only the force-file filing); calls: %v", n, ghCalls(*calls))
	}
}

// countCalls counts how many recorded gh ops contain every token in want (the same
// subset-match anyCall uses, but counting rather than existence).
func countCalls(calls [][]string, want ...string) int {
	n := 0
	for _, c := range calls {
		all := true
		for _, w := range want {
			found := false
			for _, a := range c {
				if a == w {
					found = true
					break
				}
			}
			if !found {
				all = false
				break
			}
		}
		if all {
			n++
		}
	}
	return n
}

// (b) the override requires a stated reason — an unreasoned override is the cap removed, not
// an escape hatch. Green before (unknown flag → bad-flags refusal) and after (explicit
// require), both exit 5; kept as a guard on the reason requirement.
func TestForceFileWithoutReasonRefuses(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	body := bodyFileWith(t, "x")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "override without a reason", "--body-file", body, "--force-file"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("--force-file without --reason rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	assertNoMutatingGH(t, *calls)
}

// (c) anti-evasion: neither the override nor a rotated session id may ERASE an accumulated
// count. Session-A files to the cap; session-B (a rotation) then --force-files. Session-A's
// recorded count must be intact afterwards, and the override line must be tagged to the
// session that filed it (session-B) — the trail is append-only and per-session, so a
// rotation leaves a forensic record rather than wiping one. RED on unfixed code
// (--force-file unknown → the session-B call is refused exit 5, files nothing).
func TestForceFileUnderRotatedSessionDoesNotEraseCount(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	seedNewAudit(t, 3, allowedRepo, "session-A")
	t.Setenv("CLAUDE_SESSION_ID", "session-B") // rotate the session id
	body := bodyFileWith(t, "a rotated-session override filing")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "rotated session override filing", "--body-file", body,
		"--force-file", "--reason", "rotated session, urgent"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--force-file under a rotated session rc = %d, want 0; out=%s", rc, out)
	}
	// Session-A's accumulated count is NOT erased by the rotation or the override.
	if got := chargedNewCount(t, allowedRepo, "session-A"); got != 3 {
		t.Fatalf("session-A charged count after a rotated-session override = %d, want 3 "+
			"(the count must not be erasable by rotating the session id or by --force-file)", got)
	}
	// The override line is tagged to the session that actually filed it — the trail names
	// the rotation rather than hiding it.
	e := lastAudit(t)
	if e.SessionTag != "session-B" {
		t.Fatalf("override audit sessionTag = %q, want \"session-B\" (the rotation leaves a trail)", e.SessionTag)
	}
	if !strings.Contains(e.Detail, "force-file") {
		t.Fatalf("override audit detail must record the override; got %q", e.Detail)
	}
}
