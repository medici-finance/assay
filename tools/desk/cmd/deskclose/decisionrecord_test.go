package main

// decisionrecord_test.go — the human-decided lane's human-decision-v1 record
// (the brief's Verify rows 2, 5 and 6). Each test runs under its own HOME, so the
// local copy (decision-records.jsonl) and the audit log it reads back are the test's own.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	recT0 = "2026-10-01T09:00:00Z" // the ask opened
	recT1 = "2026-10-03T10:30:15Z" // the ruling
	// T1 − T0 = 2d 1h 30m 15s.
	recLatency = 2*86400 + 3600 + 30*60 + 15

	recOptA    = "Ship the small fix"
	recOptB    = "Rewrite the parser"
	recOptC    = "Do nothing for now"
	recAppSlug = "assay-desk-app"
	recAppID   = 300000001
)

var recBlockRe = regexp.MustCompile(`<!-- human-decision-v1 (\{[^\n]*\}) -->`)

// recIssueBody is the fixture ask: three options, B marked recommended.
func recIssueBody() string {
	return "<!-- needs-decision: demo-stream/07 -->\n## Context\nWhich parser fix.\n\n## Options\n" +
		"A. " + recOptA + "\nB. " + recOptB + " (recommended)\nC. " + recOptC + "\n"
}

// recWorld is triageWorld under a private HOME, with the fixture ask at triageIssue
// (created_at = createdAt; "" leaves the forge's creation time absent).
func recWorld(t *testing.T, createdAt string) (*stubRemote, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	s, rul := triageWorld(t)
	s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = fmt.Sprintf(
		`{"number":%d,"title":"stub item","state":"open","body":%q,"labels":[{"name":"human-decided"}],"created_at":%q}`,
		triageIssue, recIssueBody(), createdAt)
	return s, rul, home
}

// plantAtComment plants a comment on issue n carrying its creation time, never edited (its
// update time is its creation time, as the forge reports an unedited comment).
func plantAtComment(s *stubRemote, n int, cid, login string, id int64, typ, at, body string) string {
	return plantEditedComment(s, n, cid, login, id, typ, at, at, body)
}

// plantEditedComment is plantAtComment with the forge's update time given separately.
func plantEditedComment(s *stubRemote, n int, cid, login string, id int64, typ, at, updated, body string) string {
	issueURL := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", testRepo, n)
	html := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%s", testRepo, n, cid)
	s.comment[cid] = fmt.Sprintf(
		`{"id":%s,"html_url":%q,"issue_url":%q,"body":%q,"minimized":false,"created_at":%q,"updated_at":%q,`+
			`"user":{"login":%q,"id":%d,"type":%q}}`,
		cid, html, issueURL, body, at, updated, login, id, typ)
	return html
}

// recDecode reads the one block in the close comment.
func recDecode(t *testing.T, s *stubRemote) deskkit.HumanDecisionRecord {
	t.Helper()
	var rec deskkit.HumanDecisionRecord
	if err := json.Unmarshal([]byte(recBlockJSON(t, commentBody(t, s))), &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func recClose(t *testing.T, rul, url string, extra ...string) (int, string) {
	t.Helper()
	args := append([]string{modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionHumanDecided, "--decision", url, "--tracker", "#40", "--rulings", rul}, extra...)
	return execCLI(args...)
}

func recLocalPath(home string) string {
	return filepath.Join(home, ".config", "assay", "decision-records.jsonl")
}

// recLastTriageDetail reads back the newest triage audit entry's detail.
func recLastTriageDetail(t *testing.T, home string) string {
	t.Helper()
	f, err := os.Open(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if err != nil {
		t.Fatalf("audit log: %v", err)
	}
	defer f.Close()
	detail := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for sc.Scan() {
		var e deskkit.Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Verb == modeTriage {
			detail = e.Detail
		}
	}
	return detail
}

func recBlockJSON(t *testing.T, body string) string {
	t.Helper()
	m := recBlockRe.FindAllStringSubmatch(body, -1)
	if len(m) != 1 {
		t.Fatalf("want exactly one human-decision-v1 block in the close comment, got %d:\n%s", len(m), body)
	}
	return m[0][1]
}

// TestTriageHumanDecidedRecord — Verify row 2: options A/B/C with B recommended, opened at
// T0, ruled "B" at T1 → the close comment's block and the local line are the same bytes,
// and the record carries the pick, the recommendation and T1 − T0.
func TestTriageHumanDecidedRecord(t *testing.T) {
	s, rul, home := recWorld(t, recT0)
	url := plantAtComment(s, triageIssue, "7101", blessLogin, blessID, "User", recT1, "B")
	code, out := recClose(t, rul, url)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	assertClosedWith(t, s, reasonNotPlanned)

	block := recBlockJSON(t, commentBody(t, s))
	local, err := os.ReadFile(recLocalPath(home))
	if err != nil {
		t.Fatalf("local copy: %v", err)
	}
	if string(local) != block+"\n" {
		t.Fatalf("forge and local copies differ:\nforge: %s\nlocal: %s", block, local)
	}

	var rec deskkit.HumanDecisionRecord
	if err := json.Unmarshal([]byte(block), &rec); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, o := range rec.Options {
		ids = append(ids, o.ID)
	}
	if strings.Join(ids, ",") != "A,B,C" || rec.Recommended != "B" || rec.Picked != "B" ||
		!rec.PickedIsRecommended || rec.PickedVia != deskkit.DecisionViaDirect {
		t.Fatalf("options %v recommended %q picked %q via %q is-rec %t", ids, rec.Recommended,
			rec.Picked, rec.PickedVia, rec.PickedIsRecommended)
	}
	if rec.OpenedAt != recT0 || rec.RuledAt != recT1 || rec.OpenedToRuledS != recLatency ||
		rec.AskedToRuledS != recLatency {
		t.Fatalf("latency: opened %s ruled %s → %d s, want %d", rec.OpenedAt, rec.RuledAt,
			rec.OpenedToRuledS, recLatency)
	}
	if rec.Repo != testRepo || rec.Issue != triageIssue || rec.Tracker != testRepo+"#40" ||
		rec.Brief != "demo-stream/07" || rec.OptionsSource != deskkit.DecisionSourceBody {
		t.Fatalf("identity fields: %+v", rec)
	}
	if err := deskkit.ValidateDecisionRecord(rec); err != nil {
		t.Fatalf("the posted record does not validate: %v", err)
	}
	if d := recLastTriageDetail(t, home); !strings.Contains(d, "decision-record: forge+local") {
		t.Fatalf("audit detail should name both copies: %q", d)
	}
}

// recStrings collects every string value in a decoded record, skipping the digest fields
// (hex can contain a short login by chance; a digest is never a carrier of text).
func recStrings(v any, key string, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			recStrings(vv, k, out)
		}
	case []any:
		for _, vv := range x {
			recStrings(vv, key, out)
		}
	case string:
		if !strings.HasSuffix(key, "_sha256") && key != "tool_sha" {
			*out = append(*out, x)
		}
	}
}

// TestTriageHumanDecidedRecordNeverCollects — Verify row 5: the record carries digests and
// closed-set values only — no option text, no ruling text, no ruler or App login.
func TestTriageHumanDecidedRecordNeverCollects(t *testing.T) {
	s, rul, home := recWorld(t, recT0)
	// A trusted App relay restates the options (walk order) and records the answer; the
	// human ratifies it. Both bodies are text the record must never carry.
	relay := "Asked as: the parser decision.\n\n## Options\nA. " + recOptB + " (recommended)\nB. " +
		recOptA + "\nC. " + recOptC + "\n\nAnswer: A\n"
	plantAtComment(s, triageIssue, "7201", recAppSlug, recAppID, "Bot", "2026-10-02T08:00:00Z", relay)
	ruling := "Ratified, take the rewrite and ship it this week"
	url := plantAtComment(s, triageIssue, "7202", blessLogin, blessID, "User", recT1, ruling)
	code, out := recClose(t, rul, url)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	block := recBlockJSON(t, commentBody(t, s))

	for _, banned := range []string{recOptA, recOptB, recOptC, ruling, "Ratified", recAppSlug,
		"Asked as", sharedLogin} {
		if strings.Contains(block, banned) {
			t.Fatalf("the record carries %q:\n%s", banned, block)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(block), &decoded); err != nil {
		t.Fatal(err)
	}
	var vals []string
	recStrings(decoded, "", &vals)
	for _, v := range vals {
		if strings.Contains(v, blessLogin) || strings.Contains(v, fmt.Sprint(blessID)) ||
			strings.Contains(v, fmt.Sprint(recAppID)) {
			t.Fatalf("a record field carries a login or account id: %q in\n%s", v, block)
		}
	}
	if decoded["ruler"] != "driver" {
		t.Fatalf("ruler = %v, want driver", decoded["ruler"])
	}
	var rec deskkit.HumanDecisionRecord
	_ = json.Unmarshal([]byte(block), &rec)
	if rec.Picked != "A" || rec.PickedVia != deskkit.DecisionViaRatified ||
		rec.OptionsSource != deskkit.DecisionSourceRelay || rec.AskedAt != "2026-10-02T08:00:00Z" {
		t.Fatalf("relay-anchored pick: %+v", rec)
	}
	if rec.Options[0].TextSHA256 != deskkit.Sha256Hex([]byte(recOptB)) {
		t.Fatal("relay option A should carry the digest of its own cleaned text")
	}
	if local, _ := os.ReadFile(recLocalPath(home)); string(local) != block+"\n" {
		t.Fatalf("local copy differs from the forge copy:\n%s", local)
	}
}

// TestTriageHumanDecidedRecordStoreUnwritable — Verify row 6: the record never gates the
// close. An unwritable local store still closes with the block posted and the audit naming
// local-unwritten; an invalid record still closes, with no block and no local line.
func TestTriageHumanDecidedRecordStoreUnwritable(t *testing.T) {
	t.Run("local store unwritable", func(t *testing.T) {
		s, rul, home := recWorld(t, recT0)
		// A directory where the file should be: the append cannot open it.
		if err := os.MkdirAll(recLocalPath(home), 0o700); err != nil {
			t.Fatal(err)
		}
		url := plantAtComment(s, triageIssue, "7301", blessLogin, blessID, "User", recT1, "B")
		code, out := recClose(t, rul, url)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
		recBlockJSON(t, commentBody(t, s))
		if d := recLastTriageDetail(t, home); !strings.Contains(d, "decision-record: local-unwritten") {
			t.Fatalf("audit detail should name local-unwritten: %q", d)
		}
	})

	t.Run("invalid record", func(t *testing.T) {
		// No forge creation time for the ask: opened_at cannot be stated, so no record.
		s, rul, home := recWorld(t, "")
		url := plantAtComment(s, triageIssue, "7302", blessLogin, blessID, "User", recT1, "B")
		code, out := recClose(t, rul, url)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
		if body := commentBody(t, s); strings.Contains(body, "human-decision-v1") {
			t.Fatalf("an invalid record was posted:\n%s", body)
		}
		if _, err := os.Stat(recLocalPath(home)); !os.IsNotExist(err) {
			t.Fatalf("an invalid record reached the local store: %v", err)
		}
		if d := recLastTriageDetail(t, home); !strings.Contains(d, "decision-record: invalid (opened_at)") {
			t.Fatalf("audit detail should name the invalid field: %q", d)
		}
	})
}

// TestTriageRecordDryRunNoLine — a human-decided dry-run shows the block it WOULD post and
// writes nothing: no forge write, no local line.
func TestTriageRecordDryRunNoLine(t *testing.T) {
	s, rul, home := recWorld(t, recT0)
	url := plantAtComment(s, triageIssue, "7401", blessLogin, blessID, "User", recT1, "B")
	code, out := recClose(t, rul, url, "--dry-run")
	if code != deskkit.ExitOK || !strings.Contains(out, "dry-run") {
		t.Fatalf("want exit 0 dry-run, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
	if !strings.Contains(out, "<!-- human-decision-v1 {") {
		t.Fatalf("dry-run should show the block it would post:\n%s", out)
	}
	if _, err := os.Stat(recLocalPath(home)); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote a local line: %v", err)
	}
}

// TestTriageNotPlannedNoRecord — the not-planned lane closes no ruled decision, so it posts
// no block and writes no local line, even when the issue body states options.
func TestTriageNotPlannedNoRecord(t *testing.T) {
	s, rul, home := recWorld(t, recT0)
	s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = fmt.Sprintf(
		`{"number":%d,"title":"stub item","state":"open","body":%q,"labels":[],"created_at":%q}`,
		triageIssue, recIssueBody(), recT0)
	plantTrustedMarker(s, triageIssue)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if strings.Contains(commentBody(t, s), "human-decision-v1") {
		t.Fatal("the not-planned lane posted a decision record")
	}
	if _, err := os.Stat(recLocalPath(home)); !os.IsNotExist(err) {
		t.Fatalf("the not-planned lane wrote a local line: %v", err)
	}
}

// regression: medici-finance/assay#2426 (SEC-1/SEC-2) — the record states whether any text
// it was read from changed after the ruling, from what the forge reports at close time.
func TestTriageRecordEditState(t *testing.T) {
	key := fmt.Sprintf("%s#%d", testRepo, triageIssue)
	t.Run("unedited", func(t *testing.T) {
		s, rul, _ := recWorld(t, recT0)
		url := plantAtComment(s, triageIssue, "7501", blessLogin, blessID, "User", recT1, "B")
		if code, out := recClose(t, rul, url); code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if rec := recDecode(t, s); rec.EditState != deskkit.DecisionEditUnedited || rec.Picked != "B" {
			t.Fatalf("edit_state %q picked %q, want unedited B", rec.EditState, rec.Picked)
		}
	})
	t.Run("body edited after the ruling", func(t *testing.T) {
		s, rul, _ := recWorld(t, recT0)
		s.bodyEdited[key] = "2026-10-04T00:00:00Z"
		url := plantAtComment(s, triageIssue, "7502", blessLogin, blessID, "User", recT1, "B")
		if code, out := recClose(t, rul, url); code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		rec := recDecode(t, s)
		if rec.EditState != deskkit.DecisionEditEdited || rec.Picked != deskkit.DecisionPickUnparsed ||
			rec.Recommended != deskkit.DecisionRecommendedNone || rec.PickedIsRecommended {
			t.Fatalf("an edit after the ruling kept a reading: %+v", rec)
		}
	})
	t.Run("ruling edited after it was posted", func(t *testing.T) {
		s, rul, _ := recWorld(t, recT0)
		url := plantEditedComment(s, triageIssue, "7503", blessLogin, blessID, "User", recT1, "2026-10-03T11:00:00Z", "B")
		if code, out := recClose(t, rul, url); code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if rec := recDecode(t, s); rec.EditState != deskkit.DecisionEditEdited || rec.Picked != deskkit.DecisionPickUnparsed {
			t.Fatalf("edit_state %q picked %q, want edited unparsed", rec.EditState, rec.Picked)
		}
	})
	t.Run("body history unknown", func(t *testing.T) {
		s, rul, _ := recWorld(t, recT0)
		s.bodyEditUnknown = true
		url := plantAtComment(s, triageIssue, "7504", blessLogin, blessID, "User", recT1, "B")
		if code, out := recClose(t, rul, url); code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		if rec := recDecode(t, s); rec.EditState != deskkit.DecisionEditUnverifiable {
			t.Fatalf("edit_state %q, want unverifiable", rec.EditState)
		}
	})
}

// regression: medici-finance/assay#2426 (definite-pick-not-in-ruling) — the pick is read
// only from comments BEFORE the verified ruling: an App relay posted after it never supplies
// the answer a ratification resolves through.
func TestTriageRecordRelayAfterRuling(t *testing.T) {
	s, rul, _ := recWorld(t, recT0)
	url := plantAtComment(s, triageIssue, "7601", blessLogin, blessID, "User", recT1, "ratified")
	relay := "Relayed.\n\n## Options\nA. " + recOptB + " (recommended)\nB. " + recOptA + "\n\nAnswer: A\n"
	plantAtComment(s, triageIssue, "7602", recAppSlug, recAppID, "Bot", "2026-10-03T12:00:00Z", relay)
	if code, out := recClose(t, rul, url); code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	rec := recDecode(t, s)
	if rec.Picked != deskkit.DecisionPickUnparsed || rec.OptionsSource != deskkit.DecisionSourceBody {
		t.Fatalf("a relay posted after the ruling supplied the pick: %+v", rec)
	}
}

// regression: medici-finance/assay#2426 (close-blocked-by-forge-copy) — a record block the
// outbound check refuses never costs the close: the prose-only comment is posted, the issue
// closes, and the local line is the record's only copy.
func TestTriageRecordBlockRefusedStillCloses(t *testing.T) {
	s, rul, home := recWorld(t, recT0)
	// The tracker repo is public here, and the brief id is a withheld identifier: the block
	// naming it cannot be posted, while the prose comment (which never names the brief) can.
	roster := strings.Replace(fixtureRoster, "example-org/tracker:ci:private", "example-org/tracker:ci:public", 1) +
		deskkit.EnvWithheldIdentifiers + "=example-withheld-slug\n"
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Setenv(deskkit.EnvWithheldIdentifiers, "example-withheld-slug")
	s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = fmt.Sprintf(
		`{"number":%d,"title":"stub item","state":"open","body":%q,"labels":[{"name":"human-decided"}],"created_at":%q}`,
		triageIssue, strings.Replace(recIssueBody(), "demo-stream/07", "example-withheld-slug/3", 1), recT0)
	url := plantAtComment(s, triageIssue, "7701", blessLogin, blessID, "User", recT1, "B")
	code, out := recClose(t, rul, url)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	assertClosedWith(t, s, reasonNotPlanned)
	if body := commentBody(t, s); strings.Contains(body, "human-decision-v1") || strings.Contains(body, "example-withheld-slug") {
		t.Fatalf("the refused block was posted:\n%s", body)
	}
	local, err := os.ReadFile(recLocalPath(home))
	if err != nil || !strings.Contains(string(local), `"brief":"example-withheld-slug/3"`) {
		t.Fatalf("the local copy should hold the record: %v\n%s", err, local)
	}
	if d := recLastTriageDetail(t, home); !strings.Contains(d, "decision-record: local only") {
		t.Fatalf("audit detail should name the local-only copy: %q", d)
	}
}

// regression: medici-finance/assay#2426 (C-4) — only an outbound refusal earns the prose-only
// retry. A comment post that fails any other way may have landed, so it is attempted once and
// stops the close, exactly as before the record existed.
func TestTriageRecordPostFailNoRetry(t *testing.T) {
	s, rul, home := recWorld(t, recT0)
	s.failPost[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = true
	url := plantAtComment(s, triageIssue, "7751", blessLogin, blessID, "User", recT1, "B")
	code, out := recClose(t, rul, url)
	if code == deskkit.ExitOK {
		t.Fatalf("a failed pre-close comment must stop the close, got exit 0\n%s", out)
	}
	if s.postAttempts != 1 {
		t.Fatalf("want exactly one post attempt, got %d", s.postAttempts)
	}
	if n := s.closes(); n != 0 {
		t.Fatalf("want no close after a failed comment post, got %d", n)
	}
	if _, err := os.Stat(recLocalPath(home)); !os.IsNotExist(err) {
		t.Fatalf("a local line was written: %v", err)
	}
}

// regression: medici-finance/assay#2426 (thread re-read) — a thread that cannot be re-read
// for the record leaves the record unwritten and the close standing.
func TestTriageRecordThreadUnreadable(t *testing.T) {
	s, rul, home := recWorld(t, recT0)
	// The lane's own authorization reads the thread first; the record's re-read is the second.
	s.threadFailFrom[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = 2
	url := plantAtComment(s, triageIssue, "7801", blessLogin, blessID, "User", recT1, "B")
	code, out := recClose(t, rul, url)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	assertClosedWith(t, s, reasonNotPlanned)
	if strings.Contains(commentBody(t, s), "human-decision-v1") {
		t.Fatal("a record was composed without the thread")
	}
	if _, err := os.Stat(recLocalPath(home)); !os.IsNotExist(err) {
		t.Fatalf("a local line was written: %v", err)
	}
	if d := recLastTriageDetail(t, home); !strings.Contains(d, "comment thread unreadable") {
		t.Fatalf("audit detail should name the unreadable thread: %q", d)
	}
}

// regression: medici-finance/assay#2426 — the local line is written only after the close is
// confirmed: a close that fails, or does not take, leaves no local line claiming a ruled close.
func TestTriageRecordNoLocalLineWithoutClose(t *testing.T) {
	key := fmt.Sprintf("%s#%d", testRepo, triageIssue)
	for _, tc := range []struct {
		name string
		arm  func(s *stubRemote)
	}{
		{"close refused", func(s *stubRemote) { s.failClose[key] = true }},
		{"close did not take", func(s *stubRemote) { s.closeNoReflect[key] = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rul, home := recWorld(t, recT0)
			tc.arm(s)
			url := plantAtComment(s, triageIssue, "7901", blessLogin, blessID, "User", recT1, "B")
			if code, out := recClose(t, rul, url); code == deskkit.ExitOK {
				t.Fatalf("a failed close exited 0:\n%s", out)
			}
			if _, err := os.Stat(recLocalPath(home)); !os.IsNotExist(err) {
				t.Fatalf("a local line was written for a close that did not happen: %v", err)
			}
		})
	}
}
