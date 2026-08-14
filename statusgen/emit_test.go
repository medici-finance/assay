package main

import (
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
		"## Awaiting verification / review (1 desk-actionable of 1 total — 1 at implemented, 0 verified awaiting review)",
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

	want := "## Awaiting verification / review (3 desk-actionable of 3 total — 2 at implemented, 1 verified awaiting review)"
	if !strings.Contains(out, want) {
		t.Errorf("heading missing expected counts:\nwant: %s\ngot:\n%s", want, out)
	}
}

func TestAwaitingSegmentedAssertions(t *testing.T) {
	// Fixture spanning all five segment classes.
	active := mkStream("active-s", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"}, // desk-actionable (no gate, no evidence)
		Brief{Num: "02", Wave: 0, Status: "implemented",
			Gate: "human", Evidence: "**VERIFY: PASS**\n\n| 1 | go test | 0 | PASS | 2026-07-15 | opus-verifier |",
		}, // human-gate
		Brief{Num: "03", Wave: 0, Status: "verified", Verified: "2026-07-08",
			Evidence: "VERIFY: FAIL — test broken",
		}, // rework
		Brief{Num: "04", Wave: 0, Status: "implemented",
			BlockedBy: "env",
		}, // env-blocked
	)
	paused := mkStream("paused-s", "paused", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"}, // paused stream — all awaiting briefs here are paused-segment
	)

	streams := []*Stream{active, paused}
	// LastTouch needed for gate-score staleness.
	active.LastTouch = day(5)
	paused.LastTouch = day(5)

	out := emit(streams, nil, nextUp(streams, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")

	// Heading: desk-actionable = 1 (active-s/01), total = 5, implemented = 4, verified = 1
	wantHeading := "## Awaiting verification / review (1 desk-actionable of 5 total — 4 at implemented, 1 verified awaiting review)"
	if !strings.Contains(out, wantHeading) {
		t.Errorf("heading mismatch:\nwant: %s\ngot:\n%s", wantHeading, out)
	}

	// Sub-headings assert each segment appears with the right label and count.
	for _, want := range []string{
		"### Desk-actionable (1)",
		"### Awaiting human gate (1)",
		"### Awaiting implementer rework (1)",
		"### Paused stream (1)",
		"### Env-blocked (1)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing expected segment sub-heading %q\n---\n%s", want, out)
		}
	}

	// Verify the specific briefs land in the right segment tables.
	// active-s/01 (desk-actionable) must appear under Desk-actionable heading.
	deskSection := out[strings.Index(out, "### Desk-actionable"):strings.Index(out, "### Awaiting human gate")]
	if !strings.Contains(deskSection, "| active-s | 01 |") {
		t.Error("active-s/01 must appear under Desk-actionable segment")
	}
	if strings.Contains(deskSection, "| active-s | 02 |") {
		t.Error("active-s/02 (human-gate) must NOT appear under Desk-actionable segment")
	}

	// active-s/02 (human-gate) must appear under Awaiting human gate.
	humanSection := out[strings.Index(out, "### Awaiting human gate"):strings.Index(out, "### Awaiting implementer rework")]
	if !strings.Contains(humanSection, "| active-s | 02 |") {
		t.Error("active-s/02 must appear under Awaiting human gate segment")
	}

	// active-s/03 (rework) must appear under rework segment.
	reworkSection := out[strings.Index(out, "### Awaiting implementer rework"):strings.Index(out, "### Paused stream")]
	if !strings.Contains(reworkSection, "| active-s | 03 |") {
		t.Error("active-s/03 must appear under Awaiting implementer rework segment")
	}

	// paused-s/01 must appear under Paused stream segment.
	pausedSection := out[strings.Index(out, "### Paused stream"):strings.Index(out, "### Env-blocked")]
	if !strings.Contains(pausedSection, "| paused-s | 01 |") {
		t.Error("paused-s/01 must appear under Paused stream segment")
	}

	// active-s/04 must appear under Env-blocked segment.
	envSection := out[strings.Index(out, "### Env-blocked"):]
	if !strings.Contains(envSection, "| active-s | 04 |") {
		t.Error("active-s/04 must appear under Env-blocked segment")
	}
}

func TestSegmentClassifier(t *testing.T) {
	active := mkStream("s", "active", "P1")
	paused := mkStream("p", "paused", "P1")

	tests := []struct {
		name    string
		stream  *Stream
		brief   Brief
		wantSeg blockerSegment
	}{
		{
			name:    "desk-actionable legacy (no gate, no evidence)",
			stream:  active,
			brief:   Brief{Num: "01", Status: "implemented"},
			wantSeg: segmentDeskActionable,
		},
		{
			name:    "desk-actionable gate:model with evidence",
			stream:  active,
			brief:   Brief{Num: "02", Status: "implemented", Gate: "model", Evidence: "some evidence"},
			wantSeg: segmentDeskActionable,
		},
		{
			name:    "human-gate with VERIFY:PASS",
			stream:  active,
			brief:   Brief{Num: "03", Status: "implemented", Gate: "human", Evidence: "**VERIFY: PASS** model"},
			wantSeg: segmentHumanGate,
		},
		{
			name:    "human-gate without VERIFY:PASS (still awaiting dispatch)",
			stream:  active,
			brief:   Brief{Num: "04", Status: "implemented", Gate: "human", Evidence: ""},
			wantSeg: segmentDeskActionable,
		},
		{
			name:    "rework (VERIFY:FAIL)",
			stream:  active,
			brief:   Brief{Num: "05", Status: "implemented", Evidence: "VERIFY: FAIL — test crash"},
			wantSeg: segmentRework,
		},
		{
			// The recorded pass belongs to the desk, not to human:<name>: a gate:model
			// brief that passed is still the desk's to flip. Dropping the
			// Gate=="human" conjunct sends it to the human queue where it sits
			// forever and the headline undercounts.
			name:    "gate:model with a recorded pass stays desk-actionable",
			stream:  active,
			brief:   Brief{Num: "09", Status: "implemented", Gate: "model", Evidence: "**VERIFY: PASS** — all rows green"},
			wantSeg: segmentDeskActionable,
		},
		{
			// Precedence, now decided by recency rather than branch order: the
			// LAST verdict is FAIL, so the implementer owns it even though the
			// brief is human-gated and an earlier pass is on the record.
			name:   "human-gated, last verdict FAIL, is rework not human-gate",
			stream: active,
			brief: Brief{Num: "10", Status: "implemented", Gate: "human",
				Evidence: "**VERIFY: PASS** — 2026-07-15\n\nreopened\n\n**VERIFY: FAIL** — regression 2026-07-18",
			},
			wantSeg: segmentRework,
		},
		{
			// The mirror image: failed, reworked, passed. Evidence accumulates,
			// so any-occurrence matching would pin this in rework forever.
			name:   "human-gated, FAIL then PASS, is human-gate not rework",
			stream: active,
			brief: Brief{Num: "11", Status: "verified", Gate: "human",
				Evidence: "**VERIFY: FAIL** — 2026-07-16\n\nfixed\n\n**VERIFY: PASS** — 2026-07-20",
			},
			wantSeg: segmentHumanGate,
		},
		{
			// Marker forms live in this repo's Evidence today. The bold
			// delimiters wrap a longer span, so an exact `**VERIFY: PASS**`
			// literal never matches and the row misfiles as drainable.
			name:   "human-gate, pass inside a longer bold span",
			stream: active,
			brief: Brief{Num: "12", Status: "implemented", Gate: "human",
				Evidence: "**Non-implementer verifier run — VERIFY: PASS** · 2026-07-20 · `glm-5.2-verifier`",
			},
			wantSeg: segmentHumanGate,
		},
		{
			name:   "human-gate, pass with trailing prose in the span (loop-engine/01 form)",
			stream: active,
			brief: Brief{Num: "13", Status: "implemented", Gate: "human",
				Evidence: "**VERIFY: PASS — all 6 rows green.**",
			},
			wantSeg: segmentHumanGate,
		},
		{
			// A quoted marker is a reference to a verdict, not a verdict.
			name:   "blockquoted FAIL does not override the live PASS",
			stream: active,
			brief: Brief{Num: "14", Status: "implemented", Gate: "human",
				Evidence: "**VERIFY: PASS** — 2026-07-20\n\n> earlier VERIFY: FAIL flagged (superseded)",
			},
			wantSeg: segmentHumanGate,
		},
		{
			name:   "code-fenced FAIL does not override the live PASS",
			stream: active,
			brief: Brief{Num: "15", Status: "implemented", Gate: "human",
				Evidence: "**VERIFY: PASS** — 2026-07-20\n\n```\nlog line: VERIFY: FAIL\n```",
			},
			wantSeg: segmentHumanGate,
		},
		{
			name:   "struck-through FAIL does not override the live PASS",
			stream: active,
			brief: Brief{Num: "16", Status: "implemented", Gate: "human",
				Evidence: "**VERIFY: PASS** — 2026-07-20\n\n~~VERIFY: FAIL~~ (superseded)",
			},
			wantSeg: segmentHumanGate,
		},
		{
			// VERIFY: PARTIAL is neither verdict — it must not invent a sixth
			// segment nor silently read as a pass.
			name:    "VERIFY: PARTIAL is not a verdict",
			stream:  active,
			brief:   Brief{Num: "17", Status: "implemented", Gate: "human", Evidence: "**VERIFY: PARTIAL** — 3 of 5 rows"},
			wantSeg: segmentDeskActionable,
		},
		{
			name:    "spaceless VERIFY:FAIL still reads as a fail",
			stream:  active,
			brief:   Brief{Num: "18", Status: "implemented", Evidence: "VERIFY:FAIL — harness died"},
			wantSeg: segmentRework,
		},
		{
			name:    "paused stream trumps everything",
			stream:  paused,
			brief:   Brief{Num: "06", Status: "implemented", Gate: "human", Evidence: "**VERIFY: PASS**"},
			wantSeg: segmentPaused,
		},
		{
			name:    "env-blocked",
			stream:  active,
			brief:   Brief{Num: "07", Status: "implemented", BlockedBy: "env"},
			wantSeg: segmentEnvBlocked,
		},
		{
			name:    "paused trumps env-blocked",
			stream:  paused,
			brief:   Brief{Num: "08", Status: "implemented", BlockedBy: "env"},
			wantSeg: segmentPaused,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyAwaiting(tt.stream, &tt.brief)
			if got != tt.wantSeg {
				t.Errorf("classifyAwaiting() = %v, want %v", got, tt.wantSeg)
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
