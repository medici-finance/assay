package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// An on-behalf-of relay is ATTRIBUTION: it records which human an App identity acted
// for. It is never that human's sign-off. The online corroboration lanes strip the
// relay marker (stripOnBehalfOf), so a relay needs nothing on the PR to corroborate.
// These tests pin the other half: the offline human-AUTHORITY readers must not read the
// same text as the human's own act. If one did, an App could write
// `authorized-by: on-behalf-of human:<name>` and pass both lanes without the human ever
// acting.
//
// The fixture roster maps name `alex` to login `ada` (rosterfixture_test.go), so both
// principal forms a relay can carry are exercised: the neutral name (the public form)
// and the login (the known-private form).

// relayAuthorityValues are the relay-shaped values that must never authorize, and the
// form each one takes.
var relayAuthorityValues = []struct{ why, value string }{
	{"relay naming the neutral name", "on-behalf-of human:alex"},
	{"relay naming the login", "on-behalf-of human:ada"},
	{"trailer-shaped relay", "On-behalf-of: human:alex"},
	{"relay in upper case", "ON-BEHALF-OF human:alex"},
	{"relay inside a parenthetical", "(on-behalf-of human:alex)"},
}

func authorityFrontmatter(key, value string) []byte {
	return []byte("---\nid: F-a\n" + key + ": \"" + value + "\"\n---\n\nBody.\n")
}

// TestOnBehalfOfRelayDoesNotAuthorizeRegisterKeys: a relay in `authorized-by:` or
// `parked-by:` does not authorize a finding gut or a park. A `human:<name>` written
// outside the relay still does. The relay's removal must not swallow a genuine stamp
// that sits beside it.
func TestOnBehalfOfRelayDoesNotAuthorizeRegisterKeys(t *testing.T) {
	readers := []struct {
		key  string
		read func([]byte) bool
	}{
		{"authorized-by", authorizedByVerifiedHuman},
		{"parked-by", parkAuthorizedByVerifiedHuman},
	}
	for _, r := range readers {
		for _, v := range relayAuthorityValues {
			if r.read(authorityFrontmatter(r.key, v.value)) {
				t.Errorf("%s: %q (%s) authorizes — a relay is attribution, never the human's own act",
					r.key, v.value, v.why)
			}
		}
		for _, genuine := range []string{"human:alex", "human:alex (on-behalf-of human:alex)", "on-behalf-of human:ada; human:alex"} {
			if !r.read(authorityFrontmatter(r.key, genuine)) {
				t.Errorf("%s: %q no longer authorizes — the human:<name> written outside the relay must still count",
					r.key, genuine)
			}
		}
	}
}

// TestOnBehalfOfRelayIsNotDeployAuthority: a deploy record's `authority:` and
// `rollback-approver:` must name a human, and a relay does not. This includes the
// login form, which main already let through with nothing online to corroborate.
func TestOnBehalfOfRelayIsNotDeployAuthority(t *testing.T) {
	for _, v := range relayAuthorityValues {
		if hasHumanAuthority(v.value) {
			t.Errorf("hasHumanAuthority(%q) = true (%s) — a relay is not deploy authority", v.value, v.why)
		}
	}
	for _, genuine := range []string{"human:alex", "human:ada", "human:alex (on-behalf-of human:alex)"} {
		if !hasHumanAuthority(genuine) {
			t.Errorf("hasHumanAuthority(%q) = false — a human:<name> outside a relay is still authority", genuine)
		}
	}
}

// TestDeployRegisterRejectsRelayAuthority drives the same rule through the register
// lint end to end. A deploy record whose `authority:` and `rollback-approver:` are
// relays is reported on both keys.
func TestDeployRegisterRejectsRelayAuthority(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", deploysDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	rec := "---\nid: DEPLOY-relay-authority\nkind: deploy\ndate: \"2026-09-23\"\n" +
		"title: \"Relay-shaped authority\"\nenvironment: staging\nbrief: dp/01\n" +
		"authority: \"on-behalf-of human:alex\"\nrollback: none-accepted\n" +
		"rollback-approver: \"on-behalf-of human:ada\"\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(dir, "DEPLOY-relay-authority.md"), []byte(rec), 0o644); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(deployRegisterProblems(root), "\n")
	for _, want := range []string{"authority \"on-behalf-of human:alex\" must name a human", "requires rollback-approver to name a human"} {
		if !strings.Contains(joined, want) {
			t.Errorf("deploy register problems do not report %q; got:\n%s", want, joined)
		}
	}
}

// TestOnBehalfOfRelayDoesNotShelveAFinding: a park whose `parked-by:` is a relay is
// not in authority vocabulary, so it never shelves the finding's standing notice.
func TestOnBehalfOfRelayDoesNotShelveAFinding(t *testing.T) {
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	for _, v := range relayAuthorityValues {
		if parkIsAuthorizedVocab(v.value) {
			t.Errorf("parkIsAuthorizedVocab(%q) = true (%s)", v.value, v.why)
		}
		f := Finding{ParkedUntil: "2026-12-31", ParkedBy: v.value, ParkedReason: "waiting"}
		if got := classifyPark(f, now); got == parkActive {
			t.Errorf("classifyPark with parked-by %q = parkActive — a relay shelved the finding", v.value)
		}
	}
	f := Finding{ParkedUntil: "2026-12-31", ParkedBy: "human:alex", ParkedReason: "waiting"}
	if got := classifyPark(f, now); got != parkActive {
		t.Errorf("classifyPark with parked-by human:alex = %v, want parkActive (control)", got)
	}
}

// TestOnlineExemptLineNeverAuthorizesOffline is the invariant both lanes must hold
// together. If the online stamp lane records NO stamp for an added authority line, the
// line has nothing on the PR to corroborate, so no offline reader may treat it as a
// human's authority. The added lines are the security review's probe rows, plus the
// login-form rows.
func TestOnlineExemptLineNeverAuthorizesOffline(t *testing.T) {
	type reader struct {
		key  string
		auth func(value string) bool
	}
	readers := []reader{
		{"authorized-by", func(v string) bool { return authorizedByVerifiedHuman(authorityFrontmatter("authorized-by", v)) }},
		{"parked-by", func(v string) bool { return parkAuthorizedByVerifiedHuman(authorityFrontmatter("parked-by", v)) }},
		{"authority", hasHumanAuthority},
		{"rollback-approver", hasHumanAuthority},
	}
	values := []string{"on-behalf-of human:alex", "on-behalf-of human:ada", "On-behalf-of: human:alex"}
	for _, r := range readers {
		for _, v := range values {
			diff := "diff --git a/docs/streams/findings/F-a.md b/docs/streams/findings/F-a.md\n" +
				"+++ b/docs/streams/findings/F-a.md\n" +
				"+" + r.key + ": " + v + "\n"
			online := len(stampsInDiff("", diff))
			if online == 0 && r.auth(v) {
				t.Errorf("%s: %s — online lane records 0 stamps and the offline reader grants authority: "+
					"a relay passes both lanes with no human act", r.key, v)
			}
		}
		// Control: a genuine stamp is gated online AND authorizes offline.
		diff := "diff --git a/f.md b/f.md\n+++ b/f.md\n+" + r.key + ": human:alex\n"
		if n := len(stampsInDiff("", diff)); n != 1 || !r.auth("human:alex") {
			t.Errorf("%s: human:alex control — online stamps = %d (want 1), offline authority = %v (want true)",
				r.key, n, r.auth("human:alex"))
		}
	}

	// decided-by: the design-approval key. Its online lane is stampsInDiff PLUS the
	// decision-record lane (decisionRecordsInDiff → addDecisionRecordStamps), which
	// re-reads the record on disk. Its offline reader is the DECISIONS register lint
	// that the design gate dereferences by the frontmatter id, not by the file name.
	// So both a DR-named file and a file whose name is not DR-shaped are probed: the
	// register reads every .md in the directory, and the online lane must see the same
	// set. Values are written unquoted where YAML allows it, which is the shape that
	// leaves the relay marker strippable online.
	decidedByValues := []struct{ why, yamlValue, value string }{
		{"relay naming the neutral name", "on-behalf-of human:alex", "on-behalf-of human:alex"},
		{"relay naming the login", "on-behalf-of human:ada", "on-behalf-of human:ada"},
		{"trailer-shaped relay", "\"On-behalf-of: human:alex\"", "On-behalf-of: human:alex"},
	}
	for _, file := range []string{"DR-relay-probe-rec.md", "relay-probe-rec.md"} {
		for _, v := range decidedByValues {
			online, offline := decidedByLanes(t, file, v.yamlValue)
			if online == 0 && offline {
				t.Errorf("decided-by in docs/streams/decisions/%s: %s (%s) — online lane records 0 stamps and the "+
					"register lint accepts it as the design-approval authority: a relay passes both lanes with no human act",
					file, v.value, v.why)
			}
			if offline {
				t.Errorf("decided-by in docs/streams/decisions/%s: %s (%s) — the register lint accepts a relay as the "+
					"design-approval authority; a relay is attribution, never the human's own act", file, v.value, v.why)
			}
		}
		// Control: a genuine decided-by is gated online AND accepted offline.
		if online, offline := decidedByLanes(t, file, "\"human:alex\""); online == 0 || !offline {
			t.Errorf("decided-by in docs/streams/decisions/%s: human:alex control — online stamps = %d (want >0), "+
				"offline authority = %v (want true)", file, online, offline)
		}
		// The literal placeholder is accepted offline because only the ruling lane can
		// corroborate it, so the online lane must record it whatever the file is named.
		if online, offline := decidedByLanes(t, file, "\"human:<name>\""); online == 0 && offline {
			t.Errorf("decided-by in docs/streams/decisions/%s: the human:<name> placeholder — online lane records 0 "+
				"stamps and the register lint accepts it: nothing gates it", file)
		}
	}
}

// decidedByLanes writes one decision record, named file, whose decided-by is
// yamlValue, into a fresh root. It returns the stamps the online lane records for a
// diff that adds the record, and whether the offline register lint accepts the
// decided-by as a human's design-approval authority.
func decidedByLanes(t *testing.T, file, yamlValue string) (online int, offline bool) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", decisionsDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	decidedBy := "decided-by: " + yamlValue
	rec := "---\nid: DR-relay-probe-rec\ndate: \"2026-09-24\"\ntitle: \"Relay probe\"\nconsequence: minor\n" +
		decidedBy + "\nalternatives:\n  - \"another path\"\naccepted:\n  - \"a cost\"\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(rec), 0o644); err != nil {
		t.Fatal(err)
	}
	rel := "docs/streams/" + decisionsDirName + "/" + file
	diff := "diff --git a/" + rel + " b/" + rel + "\n+++ b/" + rel + "\n+" + decidedBy + "\n"
	stamps := stampsInDiff(root, diff)
	var recs []decisionRecordStamp
	for _, f := range decisionRecordsInDiff(root, diff) {
		recs = append(recs, loadDecisionRecordStamp(root, f))
	}
	stamps = addDecisionRecordStamps(stamps, recs)
	offline = true
	for _, p := range decisionRegisterProblems(root) {
		if strings.Contains(p, "decided-by") {
			offline = false
		}
	}
	return len(stamps), offline
}

// TestReadmeCellRelayCaughtByGainGuard is the invariant for the README Verified and
// Reviewed cells. The online stamp lane strips the relay there as well, so an added
// row carrying `on-behalf-of human:<x>` records no stamp, and hasHumanReviewer reads the
// cell as naming a human. What rejects the relay is the branch lint's human-stamp gain
// guard (humanStampProblems), which reads the raw cell and refuses any human:<x> a
// branch adds, relay included. This test names that dependency: if the gain guard ever
// stopped seeing a relay, a relay would pass both lanes with no human act.
func TestReadmeCellRelayCaughtByGainGuard(t *testing.T) {
	const (
		row01 = "| 01 | [brief-01](brief-01.md) | 1 | S | todo | — | — |"
		row02 = "| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | — |"
	)
	cells := []struct{ cell, from, to string }{
		{"Verified", row01, "| 01 | [brief-01](brief-01.md) | 1 | S | todo | 2026-08-01 opus-verifier %s | — |"},
		{"Reviewed", row02, "| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | 2026-08-01 opus-reviewer %s |"},
	}
	for _, c := range cells {
		for _, v := range relayAuthorityValues {
			line := strings.Replace(c.to, "%s", v.value, 1)
			diff := "diff --git a/docs/streams/test-stream/README.md b/docs/streams/test-stream/README.md\n" +
				"+++ b/docs/streams/test-stream/README.md\n+" + line + "\n"
			online := len(stampsInDiff("", diff))
			root, path := humanStampFixture(t)
			if err := os.WriteFile(path, []byte(strings.Replace(humanStampSampleReadme, c.from, line, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			problems, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
			if online == 0 && len(problems) == 0 {
				t.Errorf("%s cell %q (%s): online lane records 0 stamps and the gain guard reports nothing — "+
					"a relay passes both lanes with no human act", c.cell, v.value, v.why)
			}
		}
	}
}
