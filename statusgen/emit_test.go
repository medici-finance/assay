package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitSections(t *testing.T) {
	s := mkStream("frontend", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done", Verified: "grandfathered", Reviewed: "grandfathered"},
		Brief{Num: "02", Wave: 1, Status: "implemented"},
		Brief{Num: "03", Wave: 1, Status: "todo", StaleRef: "F-02"},
	)
	s.Track = "product"
	s.LastTouch = day(5)
	findings := []Finding{{ID: "F-02", Date: "2026-07-08", Title: "Open thing", Affects: []string{"frontend/brief-03"}, Resolved: false}}
	out := emit([]*Stream{s}, findings, nextUp([]*Stream{s}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")

	for _, want := range []string{
		"GENERATED FILE",
		"### Product",
		"| [frontend](docs/streams/frontend/README.md) | P1 | active | 1/3 |",
		"## Next up",
		"## Awaiting verification / review (1 for the desk · 0 for the driver · 0 for workers · 0 for an operator · 0 runner-pending — of 1 total)",
		"| frontend | 02 |", // implemented brief awaiting verify
		"## Unresolved findings",
		"| F-02 |",
		"⚠ F-02", // stale marker on brief 03 in incomplete list
		"## Totals",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "| frontend | 03 |") {
		// brief 03 is stale-flagged: must NOT appear in Next up
		nextUpSection := out[strings.Index(out, "## Next up"):strings.Index(out, "## Intake")]
		if strings.Contains(nextUpSection, "03") {
			t.Error("stale brief 03 leaked into Next up")
		}
	}
}

func TestAwaitingHeadingCounts(t *testing.T) {
	// Fixture: 2 implemented, 1 verified, 2 done => awaiting=3, desk=3, impl=2, ver=1
	s := mkStream("test", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"},
		Brief{Num: "02", Wave: 0, Status: "implemented"},
		Brief{Num: "03", Wave: 0, Status: "verified", Verified: "2026-07-08"},
		Brief{Num: "04", Wave: 0, Status: "done", Verified: "grandfathered", Reviewed: "grandfathered"},
		Brief{Num: "05", Wave: 0, Status: "done", Verified: "grandfathered", Reviewed: "grandfathered"},
	)
	out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")

	want := "## Awaiting verification / review (3 for the desk · 0 for the driver · 0 for workers · 0 for an operator · 0 runner-pending — of 3 total)"
	if !strings.Contains(out, want) {
		t.Errorf("heading missing expected counts:\nwant: %s\ngot:\n%s", want, out)
	}
}

// writeOutcomeRecord writes one verify-outcome record under root.
func writeOutcomeRecord(t *testing.T, root, stream, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "verify-outcomes", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAwaitingSegmentedAssertions(t *testing.T) {
	// Fixture spanning every bucket plus the unbucketed paused segment.
	root := t.TempDir()
	writeOutcomeRecord(t, root, "active-s", "03-20260715T000000Z-aaaaaaaaaaaa.json",
		`{"brief":"active-s/03","ts":"2026-07-15T00:00:00Z","outcome":"verify-fail","blocker_kind":"implementation","blocker_ref":"#41"}`)
	active := mkStream("active-s", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"}, // desk-actionable (no gate, no evidence)
		Brief{Num: "02", Wave: 0, Status: "implemented",
			Gate: "human", Evidence: "**VERIFY: PASS**\n\n| 1 | go test | 0 | PASS | 2026-07-15 | opus-verifier |",
		}, // human gate
		Brief{Num: "03", Wave: 0, Status: "verified", Verified: "2026-07-08",
			Evidence: "VERIFY: FAIL — test broken",
		}, // implementer rework (outcome record names the issue)
		Brief{Num: "04", Wave: 0, Status: "implemented",
			BlockedBy: "env",
		}, // environment-blocked
		Brief{Num: "05", Wave: 0, Status: "verified", Gate: "model", Verified: "2026-07-09",
			Evidence: "**VERIFY: PASS**",
		}, // runner-pending: CI's auto-flip
		Brief{Num: "06", Wave: 0, Status: "implemented", Evidence: "**VERIFY: FAIL** — no record"}, // could-not-check
	)
	active.Root = root
	paused := mkStream("paused-s", "paused", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"}, // paused stream — unbucketed
	)

	streams := []*Stream{active, paused}
	// LastTouch needed for gate-score staleness.
	active.LastTouch = day(5)
	paused.LastTouch = day(5)

	out := emit(streams, nil, nextUp(streams, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")

	wantHeading := "## Awaiting verification / review (1 for the desk · 1 for the driver · 1 for workers · 1 for an operator · 1 runner-pending — of 7 total; 1 could-not-check)"
	if !strings.Contains(out, wantHeading) {
		t.Errorf("heading mismatch:\nwant: %s\ngot:\n%s", wantHeading, out)
	}

	// Fixed order: the four owned queues, the desk's queue, then the rest.
	order := []string{
		"### Awaiting human gate (1)",
		"### Awaiting implementer rework (1)",
		"### Environment-blocked (1)",
		"### Runner-pending (1)",
		"### Desk-actionable (1)",
		"### Could-not-check (1)",
		"### Paused stream (1)",
	}
	last := -1
	for _, h := range order {
		i := strings.Index(out, h)
		if i < 0 {
			t.Fatalf("output missing segment heading %q\n---\n%s", h, out)
		}
		if i < last {
			t.Errorf("segment %q out of order", h)
		}
		last = i
	}
	section := func(from, to string) string {
		return out[strings.Index(out, from):strings.Index(out, to)]
	}
	for _, c := range []struct{ from, to, row, owner, next string }{
		{"### Awaiting human gate", "### Awaiting implementer rework", "| active-s | 02 |", ownerDriver, nextActCloseCard},
		{"### Awaiting implementer rework", "### Environment-blocked", "| active-s | 03 |", ownerWorker, "fix, cite #41"},
		{"### Environment-blocked", "### Runner-pending", "| active-s | 04 |", ownerOperator, nextActNoEnvCmd},
		{"### Runner-pending", "### Desk-actionable", "| active-s | 05 |", ownerCIAutoFlip, nextActAutoFlip},
		{"### Desk-actionable", "### Could-not-check", "| active-s | 01 |", ownerVerifyDesk, nextActTriage},
		{"### Could-not-check", "### Paused stream", "| active-s | 06 |", ownerVerifyDesk, "could-not-check: "},
		{"### Paused stream", "## Age at the human gate", "| paused-s | 01 |", "—", "—"},
	} {
		sec := section(c.from, c.to)
		line := ""
		for _, l := range strings.Split(sec, "\n") {
			if strings.HasPrefix(l, c.row) {
				line = l
			}
		}
		if line == "" {
			t.Errorf("%s must appear under %q:\n%s", c.row, c.from, sec)
			continue
		}
		if !strings.Contains(line, "| "+c.owner+" | "+c.next) {
			t.Errorf("%s row must carry owner %q and next act %q; got %s", c.row, c.owner, c.next, line)
		}
	}
	if !strings.Contains(out, "| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |") {
		t.Error("segment tables must carry Owner and Next act columns, with Reviewed last")
	}
}

// TestAwaitingEmptyBucketsStillRender: the five bucket headings render even
// when a queue is empty, so an empty queue reads differently from a missing one.
func TestAwaitingEmptyBucketsStillRender(t *testing.T) {
	s := mkStream("only", "active", "P1", Brief{Num: "01", Wave: 0, Status: "done", Verified: "grandfathered", Reviewed: "grandfathered"})
	s.LastTouch = day(5)
	out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
	for _, h := range []string{"### Awaiting human gate (0)", "### Awaiting implementer rework (0)", "### Environment-blocked (0)", "### Runner-pending (0)", "### Desk-actionable (0)"} {
		if !strings.Contains(out, h) {
			t.Errorf("missing empty bucket heading %q", h)
		}
	}
	for _, h := range []string{"### Could-not-check", "### Paused stream", "### Parked stream"} {
		if strings.Contains(out, h) {
			t.Errorf("empty %q must not render", h)
		}
	}
}

// TestSegmentClassifier runs in-memory README rows (no brief file, no outcome
// store) through placeAwaiting: the Evidence verdict forms the board has always
// had to read, now under the bucket table.
func TestSegmentClassifier(t *testing.T) {
	active := mkStream("s", "active", "P1")
	paused := mkStream("p", "paused", "P1")
	label := func(p awaitPlacement) string {
		switch {
		case p.paused:
			return "paused"
		case p.parked:
			return "parked"
		}
		return p.bucket.String()
	}

	tests := []struct {
		name   string
		stream *Stream
		brief  Brief
		want   string
	}{
		{"desk-actionable legacy (no gate, no evidence)", active, Brief{Num: "01", Status: "implemented"}, "desk-actionable"},
		{"gate:model with evidence and no Verify rows is the desk's to triage", active, Brief{Num: "02", Status: "implemented", Gate: "model", Evidence: "some evidence"}, "desk-actionable"},
		{"human-gate with VERIFY:PASS", active, Brief{Num: "03", Status: "implemented", Gate: "human", Evidence: "**VERIFY: PASS** model"}, "human gate"},
		{"human-gate without VERIFY:PASS (still awaiting dispatch)", active, Brief{Num: "04", Status: "implemented", Gate: "human", Evidence: ""}, "desk-actionable"},
		// A FAIL with no outcome record: the blocker class is unrecorded, so the
		// row is could-not-check rather than a guessed owner.
		{"FAIL with no outcome record is could-not-check", active, Brief{Num: "05", Status: "implemented", Evidence: "VERIFY: FAIL — test crash"}, "could-not-check"},
		{"gate:model implemented with a recorded pass stays the desk's", active, Brief{Num: "09", Status: "implemented", Gate: "model", Evidence: "**VERIFY: PASS** — all rows green"}, "desk-actionable"},
		{"human-gated, last verdict FAIL, is not the human gate", active, Brief{Num: "10", Status: "implemented", Gate: "human",
			Evidence: "**VERIFY: PASS** — 2026-07-15\n\nreopened\n\n**VERIFY: FAIL** — regression 2026-07-18"}, "could-not-check"},
		{"human-gated, FAIL then PASS, is human-gate", active, Brief{Num: "11", Status: "verified", Gate: "human",
			Evidence: "**VERIFY: FAIL** — 2026-07-16\n\nfixed\n\n**VERIFY: PASS** — 2026-07-20"}, "human gate"},
		{"human-gate, pass inside a longer bold span", active, Brief{Num: "12", Status: "implemented", Gate: "human",
			Evidence: "**Non-implementer verifier run — VERIFY: PASS** · 2026-07-20 · `glm-5.2-verifier`"}, "human gate"},
		{"human-gate, pass with trailing prose in the span", active, Brief{Num: "13", Status: "implemented", Gate: "human",
			Evidence: "**VERIFY: PASS — all 6 rows green.**"}, "human gate"},
		{"blockquoted FAIL does not override the live PASS", active, Brief{Num: "14", Status: "implemented", Gate: "human",
			Evidence: "**VERIFY: PASS** — 2026-07-20\n\n> earlier VERIFY: FAIL flagged (superseded)"}, "human gate"},
		{"code-fenced FAIL does not override the live PASS", active, Brief{Num: "15", Status: "implemented", Gate: "human",
			Evidence: "**VERIFY: PASS** — 2026-07-20\n\n```\nlog line: VERIFY: FAIL\n```"}, "human gate"},
		{"struck-through FAIL does not override the live PASS", active, Brief{Num: "16", Status: "implemented", Gate: "human",
			Evidence: "**VERIFY: PASS** — 2026-07-20\n\n~~VERIFY: FAIL~~ (superseded)"}, "human gate"},
		{"VERIFY: PARTIAL is not a verdict", active, Brief{Num: "17", Status: "implemented", Gate: "human", Evidence: "**VERIFY: PARTIAL** — 3 of 5 rows"}, "desk-actionable"},
		{"spaceless VERIFY:FAIL still reads as a fail", active, Brief{Num: "18", Status: "implemented", Evidence: "VERIFY:FAIL — harness died"}, "could-not-check"},
		{"verified gate:model with empty Reviewed is CI's flip", active, Brief{Num: "19", Status: "verified", Gate: "model", Evidence: "**VERIFY: PASS**"}, "runner-pending"},
		{"paused stream trumps everything", paused, Brief{Num: "06", Status: "implemented", Gate: "human", Evidence: "**VERIFY: PASS**"}, "paused"},
		{"env-blocked", active, Brief{Num: "07", Status: "implemented", BlockedBy: "env"}, "environment-blocked"},
		{"paused trumps env-blocked", paused, Brief{Num: "08", Status: "implemented", BlockedBy: "env"}, "paused"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := label(placeAwaiting(tt.stream, &tt.brief)); got != tt.want {
				t.Errorf("placeAwaiting() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDebtNotice(t *testing.T) {
	t.Run("over threshold desk-actionable", func(t *testing.T) {
		// 12 implemented, all desk-actionable (legacy, no gate/evidence) => NOTICE fires
		var briefs []Brief
		for i := 0; i < 12; i++ {
			briefs = append(briefs, Brief{Num: "A", Wave: 0, Status: "implemented"})
		}
		s := mkStream("test", "active", "P1", briefs...)
		notice := debtNotice([]*Stream{s})
		if notice == "" {
			t.Error("expected NOTICE when desk-actionable > threshold")
		}
		if !strings.Contains(notice, "verification debt: 12 desk-actionable awaiting vs 0 done") {
			t.Errorf("unexpected NOTICE text: %s", notice)
		}
	})

	t.Run("total high but desk-actionable low", func(t *testing.T) {
		// 12 implemented total, but 11 are human-gated/VERIFY:PASS — only 1 is
		// desk-actionable. 1 > 0 done so NOTICE still fires, but the headline
		// count is 1, not 12 — the alarm now correctly says the queue the desk
		// can move is small, not that the total queue is large.
		var briefs []Brief
		for i := 0; i < 11; i++ {
			briefs = append(briefs, Brief{
				Num: "A", Wave: 0, Status: "implemented",
				Gate: "human", Evidence: "**VERIFY: PASS**",
			})
		}
		briefs = append(briefs, Brief{Num: "Z", Wave: 0, Status: "implemented"}) // 1 desk-actionable
		s := mkStream("test", "active", "P1", briefs...)
		notice := debtNotice([]*Stream{s})
		if notice == "" {
			t.Error("expected NOTICE when desk-actionable (1) > done (0)")
		}
		if !strings.Contains(notice, "verification debt: 1 desk-actionable awaiting vs 0 done") {
			t.Errorf("NOTICE should report desk-actionable count, not total; got: %s", notice)
		}
	})

	t.Run("desk-actionable exceeds done", func(t *testing.T) {
		// 5 desk-actionable, only 3 done — NOTICE fires on ratio (5 > 3).
		s := mkStream("test", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "implemented"},
			Brief{Num: "02", Wave: 0, Status: "implemented"},
			Brief{Num: "03", Wave: 0, Status: "implemented"},
			Brief{Num: "04", Wave: 0, Status: "implemented"},
			Brief{Num: "05", Wave: 0, Status: "implemented"},
			Brief{Num: "06", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
			Brief{Num: "07", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
			Brief{Num: "08", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
		)
		notice := debtNotice([]*Stream{s})
		if notice == "" {
			t.Error("expected NOTICE when desk-actionable > done")
		}
		if !strings.Contains(notice, "verification debt: 5 desk-actionable awaiting vs 3 done") {
			t.Errorf("unexpected NOTICE text: %s", notice)
		}
	})

	t.Run("under threshold", func(t *testing.T) {
		// 3 implemented, all desk-actionable, 5 done => no NOTICE
		s := mkStream("test", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "implemented"},
			Brief{Num: "02", Wave: 0, Status: "implemented"},
			Brief{Num: "03", Wave: 0, Status: "implemented"},
			Brief{Num: "04", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
			Brief{Num: "05", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
			Brief{Num: "06", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
			Brief{Num: "07", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
			Brief{Num: "08", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
		)
		notice := debtNotice([]*Stream{s})
		if notice != "" {
			t.Errorf("expected no NOTICE when desk-actionable <= threshold and <= done; got: %s", notice)
		}
	})

	t.Run("notice text stable", func(t *testing.T) {
		// 12 desk-actionable, 1 done => NOTICE fires
		s := mkStream("test", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "implemented"},
			Brief{Num: "02", Wave: 0, Status: "implemented"},
			Brief{Num: "03", Wave: 0, Status: "implemented"},
			Brief{Num: "04", Wave: 0, Status: "implemented"},
			Brief{Num: "05", Wave: 0, Status: "implemented"},
			Brief{Num: "06", Wave: 0, Status: "implemented"},
			Brief{Num: "07", Wave: 0, Status: "implemented"},
			Brief{Num: "08", Wave: 0, Status: "implemented"},
			Brief{Num: "09", Wave: 0, Status: "implemented"},
			Brief{Num: "10", Wave: 0, Status: "implemented"},
			Brief{Num: "11", Wave: 0, Status: "implemented"},
			Brief{Num: "12", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
		)
		notice := debtNotice([]*Stream{s})
		if notice == "" {
			t.Error("expected NOTICE when desk-actionable > threshold")
		}
		want := "verification debt: 11 desk-actionable awaiting vs 1 done — the queue is the constraint; drain before dispatching new implementation work"
		if notice != want {
			t.Errorf("NOTICE text mismatch:\ngot:  %s\nwant: %s", notice, want)
		}
	})

	t.Run("paused stream not counted in desk-actionable", func(t *testing.T) {
		// paused stream with 11 implemented briefs — all should be excluded
		// from desk-actionable count, so NOTICE fires on ratio only if active
		// desk-actionable exceeds done.
		active := mkStream("active", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
			Brief{Num: "02", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
		)
		paused := mkStream("paused", "paused", "P1")
		for i := 0; i < 11; i++ {
			paused.Briefs = append(paused.Briefs, Brief{Num: "A", Wave: 0, Status: "implemented"})
		}
		notice := debtNotice([]*Stream{active, paused})
		if notice != "" {
			t.Errorf("paused-stream briefs must not count as desk-actionable; got NOTICE: %s", notice)
		}
	})

	t.Run("human-gate with VERIFY:PASS not desk-actionable", func(t *testing.T) {
		// 11 human-gated WITH VERIFY:PASS — all await human:<name>, none desk-actionable.
		// With 1 done: desk-actionable = 0, so alarm must NOT fire.
		s := mkStream("test", "active", "P1",
			Brief{Num: "01", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
		)
		for i := 0; i < 11; i++ {
			s.Briefs = append(s.Briefs, Brief{
				Num: "A", Wave: 0, Status: "implemented",
				Gate: "human", Evidence: "**VERIFY: PASS**",
			})
		}
		notice := debtNotice([]*Stream{s})
		if notice != "" {
			t.Errorf("human-gated-with-VERIFY:PASS briefs must not count as desk-actionable; got NOTICE: %s", notice)
		}
	})

	t.Run("rework and env-blocked not desk-actionable", func(t *testing.T) {
		// 9 rework + 3 env-blocked + 2 done = total 12 implemented but 0 desk-actionable.
		// NOTICE must NOT fire (desk-actionable count is 0).
		s := mkStream("test", "active", "P1",
			Brief{Num: "d1", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
			Brief{Num: "d2", Wave: 0, Status: "done", Verified: "gf", Reviewed: "gf"},
		)
		for i := 0; i < 9; i++ {
			s.Briefs = append(s.Briefs, Brief{
				Num: "R", Wave: 0, Status: "implemented",
				Evidence: "VERIFY: FAIL — needs fix",
			})
		}
		for i := 0; i < 3; i++ {
			s.Briefs = append(s.Briefs, Brief{
				Num: "E", Wave: 0, Status: "implemented",
				BlockedBy: "env",
			})
		}
		notice := debtNotice([]*Stream{s})
		if notice != "" {
			t.Errorf("rework and env-blocked briefs must not count as desk-actionable; got NOTICE: %s", notice)
		}
	})
}
