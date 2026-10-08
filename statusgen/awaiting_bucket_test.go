package main

import (
	"strconv"
	"strings"
	"testing"
)

// verifySection builds a Verify section from "class|command|expect" triples.
func bucketVerifySection(rows ...string) string {
	var b strings.Builder
	b.WriteString("| # | Class | Command | Expect |\n|---|---|---|---|\n")
	for i, r := range rows {
		parts := strings.SplitN(r, "|", 3)
		b.WriteString("| " + strconv.Itoa(i+1) + " | " + parts[0] + " | " + parts[1] + " | " + parts[2] + " |\n")
	}
	return b.String()
}

// awaitRowsOf parses a Verify section the way the renderer does.
func awaitRowsOf(section string) awaitRows {
	var rows []verifyRowCells
	verifyRowTable(section, func(r verifyRowCells) { rows = append(rows, r) })
	return awaitRows{Rows: rows, Section: section}
}

const evidenceHeader = "| # | Command | Exit | Output | Date | Runner |\n|---|---|---|---|---|---|\n"

// TestBucketAwaiting is the bucket table, one fixture per row, plus the three
// negative cases. The subtest names are the Verify contract (bucket1–bucket7,
// negative1–negative3).
func TestBucketAwaiting(t *testing.T) {
	ciRow := "check:ci|`go test ./...`|exit 0"
	tests := []struct {
		name        string
		brief       awaitBrief
		rows        awaitRows
		ev          awaitEvidence
		oc          awaitOutcomes
		wantBucket  awaitingBucket
		wantOwner   string
		wantNextAct string
	}{
		{
			name:        "bucket1",
			brief:       awaitBrief{Status: "implemented", Gate: "human"},
			rows:        awaitRowsOf(bucketVerifySection(ciRow)),
			ev:          awaitEvidence{Text: evidenceHeader + "| 1 | `go test ./...` | 0 | ok | 2026-10-01 | verifier-model |\n\n**VERIFY: PASS** — 2026-10-01\n"},
			wantBucket:  bucketHumanGate,
			wantOwner:   ownerDriver,
			wantNextAct: nextActCloseCard,
		},
		{
			name:        "bucket2",
			brief:       awaitBrief{Status: "implemented", Gate: "model"},
			rows:        awaitRowsOf(bucketVerifySection(ciRow)),
			ev:          awaitEvidence{Text: evidenceHeader + "| 1 | `go test ./...` | 1 | FAIL | 2026-10-01 | verifier-model |\n\n**VERIFY: FAIL** — 2026-10-01\n"},
			oc:          awaitOutcomes{Latest: &awaitOutcome{Outcome: "verify-fail", BlockerKind: "implementation", BlockerRef: "acme/widget#42"}},
			wantBucket:  bucketRework,
			wantOwner:   ownerWorker,
			wantNextAct: "fix, cite acme/widget#42",
		},
		{
			name:        "bucket3",
			brief:       awaitBrief{Status: "verified", Gate: "model", Reviewed: "—"},
			rows:        awaitRowsOf(bucketVerifySection(ciRow)),
			ev:          awaitEvidence{Text: evidenceHeader + "| 1 | `go test ./...` | 0 | ok | 2026-10-01 | verifier-model |\n\n**VERIFY: PASS**\n"},
			oc:          awaitOutcomes{Latest: &awaitOutcome{Outcome: "verified"}},
			wantBucket:  bucketRunnerPending,
			wantOwner:   ownerCIAutoFlip,
			wantNextAct: nextActAutoFlip,
		},
		{
			name:  "bucket4",
			brief: awaitBrief{Status: "implemented", Gate: "model"},
			rows:  awaitRowsOf(bucketVerifySection(ciRow, "check|`probe-live.sh --env staging`|exit 0")),
			ev: awaitEvidence{Text: evidenceHeader +
				"| 1 | `go test ./...` | 0 | ok | 2026-10-01 | verifier-model |\n" +
				"| 2 | `probe-live.sh` | — | could-not-check: needs `$STAGING_TOKEN` offline | 2026-10-01 | verifier-model |\n"},
			wantBucket:  bucketEnvBlocked,
			wantOwner:   ownerOperator,
			wantNextAct: "probe-live.sh --env staging",
		},
		{
			name:        "bucket5",
			brief:       awaitBrief{Status: "implemented", Gate: "model"},
			rows:        awaitRowsOf(bucketVerifySection(ciRow, "check|`sh scripts/lint.sh`|exit 0")),
			ev:          awaitEvidence{Text: ""},
			wantBucket:  bucketRunnerPending,
			wantOwner:   ownerVerifyRunner,
			wantNextAct: nextActNone,
		},
		{
			name:        "bucket6",
			brief:       awaitBrief{Status: "implemented", Gate: "model"},
			rows:        awaitRowsOf(bucketVerifySection(ciRow, "gate:model|read the guide end to end|the steps reproduce")),
			ev:          awaitEvidence{Text: evidenceHeader + "| 1 | `go test ./...` | 0 | ok | 2026-10-01 | verifier-model |\n"},
			wantBucket:  bucketDeskActionable,
			wantOwner:   ownerVerifyDesk,
			wantNextAct: nextActJudge,
		},
		{
			name:        "bucket7",
			brief:       awaitBrief{Status: "implemented"}, // legacy: no gate, no rows
			rows:        awaitRows{},
			ev:          awaitEvidence{Text: ""},
			wantBucket:  bucketDeskActionable,
			wantOwner:   ownerVerifyDesk,
			wantNextAct: nextActTriage,
		},
		{
			// A gate:human brief with no PASS marker is not the driver's yet:
			// it is the desk's (here a judgement row, bucket 6).
			name:        "negative1",
			brief:       awaitBrief{Status: "implemented", Gate: "human"},
			rows:        awaitRowsOf(bucketVerifySection(ciRow, "gate:human|read the release note|reads true")),
			ev:          awaitEvidence{Text: evidenceHeader + "| 1 | `go test ./...` | 0 | ok | 2026-10-01 | implementer |\n"},
			wantBucket:  bucketDeskActionable,
			wantOwner:   ownerVerifyDesk,
			wantNextAct: nextActJudge,
		},
		{
			// A verified gate:human row is never runner-pending: the card is
			// the driver's to close.
			name:        "negative2",
			brief:       awaitBrief{Status: "verified", Gate: "human", Reviewed: ""},
			rows:        awaitRowsOf(bucketVerifySection(ciRow)),
			ev:          awaitEvidence{Text: evidenceHeader + "| 1 | `go test ./...` | 0 | ok | 2026-10-01 | verifier-model |\n\n**VERIFY: PASS**\n"},
			wantBucket:  bucketHumanGate,
			wantOwner:   ownerDriver,
			wantNextAct: nextActCloseCard,
		},
		{
			// An Evidence section the parser cannot read is could-not-check,
			// never silently desk-actionable.
			name:       "negative3",
			brief:      awaitBrief{Status: "implemented"},
			rows:       awaitRows{},
			ev:         awaitEvidence{Text: "<!-- appended at implementation time\n\n| 1 | `x` | 0 | ok | 2026-10-01 | verifier |\n"},
			wantBucket: bucketCouldNotCheck,
			wantOwner:  ownerVerifyDesk,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, owner, next := bucketAwaiting(tt.brief, tt.rows, tt.ev, tt.oc)
			if got != tt.wantBucket {
				t.Fatalf("bucket = %v (%s), want %v", got, next, tt.wantBucket)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if tt.wantNextAct != "" && next != tt.wantNextAct {
				t.Errorf("next act = %q, want %q", next, tt.wantNextAct)
			}
			if tt.wantBucket == bucketCouldNotCheck && !strings.HasPrefix(next, couldNotCheckLabel) {
				t.Errorf("could-not-check next act must carry the reason, got %q", next)
			}
		})
	}
}

// TestBucketAwaitingHumanGateNeedsMarker: a gate:human brief whose Evidence lacks
// the bold PASS marker is not bucketed human gate — neither an unbolded verdict
// token nor a bold span that does not open with the marker counts.
func TestBucketAwaitingHumanGateNeedsMarker(t *testing.T) {
	rows := awaitRowsOf(bucketVerifySection("check:ci|`go test ./...`|exit 0"))
	for name, evidence := range map[string]string{
		"no verdict":          evidenceHeader + "| 1 | `go test ./...` | 0 | ok | 2026-10-01 | implementer |\n",
		"unbolded PASS":       "VERIFY: PASS — all rows green\n",
		"heading, not bold":   "### Verifier run — VERIFY: PASS (2026-10-01)\n",
		"bold span elsewhere": "**Verifier run** — VERIFY: PASS · 2026-10-01\n",
	} {
		for _, b := range []awaitBrief{
			{Status: "implemented", Gate: "human"},
			{Status: "verified", Gate: "human"},
			{Status: "implemented", Gate: "model", Irreversible: true},
		} {
			got, _, next := bucketAwaiting(b, rows, awaitEvidence{Text: evidence}, awaitOutcomes{})
			if got == bucketHumanGate {
				t.Errorf("%s / %+v: bucketed human gate (%s) without the bold **VERIFY: PASS** marker", name, b, next)
			}
		}
	}
	// The positive controls: the same brief WITH the marker is the driver's,
	// including the live forms that wrap the marker in a longer bold span.
	for _, evidence := range []string{
		"**VERIFY: PASS** — 2026-10-01\n",
		"**Non-implementer verifier run — VERIFY: PASS** · 2026-10-01\n",
		"**VERIFY: PASS — all 6 rows green.**\n",
	} {
		got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "human"}, rows,
			awaitEvidence{Text: evidence}, awaitOutcomes{})
		if got != bucketHumanGate {
			t.Errorf("positive control %q = %v, want human gate", evidence, got)
		}
	}
	// Recency: a later FAIL supersedes an earlier bold PASS.
	got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "human"}, rows,
		awaitEvidence{Text: "**VERIFY: PASS** — 2026-10-01\n\n**VERIFY: FAIL** — 2026-10-03\n"},
		awaitOutcomes{Latest: &awaitOutcome{Outcome: "verify-fail", BlockerKind: "implementation", BlockerRef: "#7"}})
	if got != bucketRework {
		t.Errorf("PASS then FAIL = %v, want implementer rework", got)
	}
}

// TestBucketAwaitingUnreadableIsCouldNotCheck: an input the function cannot read
// yields could-not-check with the reason — never a bucket, and never
// desk-actionable.
func TestBucketAwaitingUnreadableIsCouldNotCheck(t *testing.T) {
	rows := awaitRowsOf(bucketVerifySection("check:ci|`go test ./...`|exit 0"))
	cases := map[string]struct {
		brief awaitBrief
		ev    awaitEvidence
		oc    awaitOutcomes
	}{
		"brief file unreadable": {
			brief: awaitBrief{Status: "implemented", Gate: "model"},
			ev:    awaitEvidence{Unreadable: "cannot read brief-03.md: permission denied"},
		},
		"unterminated comment hides the rows": {
			brief: awaitBrief{Status: "implemented", Gate: "model"},
			ev:    awaitEvidence{Text: "<!-- appended at implementation time\n" + evidenceHeader + "| 1 | `go test ./...` | 0 | ok | 2026-10-01 | v |\n"},
		},
		"outcome store unreadable": {
			brief: awaitBrief{Status: "implemented", Gate: "model"},
			ev:    awaitEvidence{Text: ""},
			oc:    awaitOutcomes{Unreadable: "cannot read docs/streams/verify-outcomes/s: permission denied"},
		},
		"FAIL with no outcome record": {
			brief: awaitBrief{Status: "implemented", Gate: "model"},
			ev:    awaitEvidence{Text: "**VERIFY: FAIL** — 2026-10-01\n"},
		},
	}
	for name, c := range cases {
		got, _, next := bucketAwaiting(c.brief, rows, c.ev, c.oc)
		if got != bucketCouldNotCheck {
			t.Errorf("%s: bucket = %v (%s), want could-not-check", name, got, next)
			continue
		}
		if !strings.HasPrefix(next, couldNotCheckLabel) || len(next) <= len(couldNotCheckLabel) {
			t.Errorf("%s: next act %q must name the reason", name, next)
		}
	}
	// A brief never verified (no FAIL, no outcome record) is NOT could-not-check:
	// nothing is unread, the rework arm simply does not apply.
	got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, rows, awaitEvidence{}, awaitOutcomes{})
	if got == bucketCouldNotCheck {
		t.Error("a never-verified brief with no outcome record must still bucket")
	}
}

// TestBucketAwaitingArms pins the defaults the table leaves open.
func TestBucketAwaitingArms(t *testing.T) {
	ci := awaitRowsOf(bucketVerifySection("check:ci|`go test ./...`|exit 0"))
	t.Run("rework needs an issue ref", func(t *testing.T) {
		got, _, next := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, ci,
			awaitEvidence{Text: "**VERIFY: FAIL**\n"},
			awaitOutcomes{Latest: &awaitOutcome{Outcome: "verify-fail", BlockerKind: "check-definition", BlockerRef: "none (no issue filed yet)"}})
		if got != bucketDeskActionable || next != nextActTriage {
			t.Errorf("no issue ref: got %v / %q, want desk-actionable triage", got, next)
		}
	})
	t.Run("untriaged FAIL is a judgement row", func(t *testing.T) {
		got, _, next := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, ci,
			awaitEvidence{Text: "**VERIFY: FAIL**\n"},
			awaitOutcomes{Latest: &awaitOutcome{Outcome: "verify-fail"}})
		if got != bucketDeskActionable || next != nextActJudge {
			t.Errorf("untriaged FAIL: got %v / %q, want desk-actionable judge", got, next)
		}
	})
	t.Run("check:cluster row is environment-blocked with its command", func(t *testing.T) {
		rows := awaitRowsOf(bucketVerifySection("check:cluster|`kubectl exec p -- probe-participant.sh`|exit 0"))
		got, owner, next := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, rows, awaitEvidence{}, awaitOutcomes{})
		if got != bucketEnvBlocked || owner != ownerOperator || next != "kubectl exec p -- probe-participant.sh" {
			t.Errorf("cluster row: got %v / %q / %q", got, owner, next)
		}
	})
	t.Run("billed probe row is environment-blocked", func(t *testing.T) {
		rows := awaitRowsOf(bucketVerifySection("check|`./probe.sh` (billed, needs the API key)|200"))
		got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, rows, awaitEvidence{}, awaitOutcomes{})
		if got != bucketEnvBlocked {
			t.Errorf("billed row: got %v, want environment-blocked", got)
		}
	})
	t.Run("blocked-by env frontmatter", func(t *testing.T) {
		got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", BlockedBy: "env"}, awaitRows{}, awaitEvidence{}, awaitOutcomes{})
		if got != bucketEnvBlocked {
			t.Errorf("blocked-by env: got %v, want environment-blocked", got)
		}
	})
	t.Run("presence gate is a judgement row", func(t *testing.T) {
		sec := "This table gates presence, not quality; quality is owned by the review gate.\n\n" + bucketVerifySection("check:ci|`grep -c x doc.md`|1")
		got, _, next := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "human"}, awaitRowsOf(sec), awaitEvidence{}, awaitOutcomes{})
		if got != bucketDeskActionable || next != nextActJudge {
			t.Errorf("presence gate: got %v / %q", got, next)
		}
	})
	t.Run("RISK-VALUE named is a judgement row", func(t *testing.T) {
		got, _, next := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "human"}, ci,
			awaitEvidence{Text: "RISK-VALUE: NAMED, NOT DERIVED — awaiting derivation\n"}, awaitOutcomes{})
		if got != bucketDeskActionable || next != nextActJudge {
			t.Errorf("RISK-VALUE: got %v / %q", got, next)
		}
	})
	t.Run("cmd-shell row is not runnable on Linux", func(t *testing.T) {
		sec := "| # | Class | Shell | Command | Expect |\n|---|---|---|---|---|\n| 1 | check | cmd | `findstr /c:x a.txt` | 0 |\n"
		got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, awaitRowsOf(sec), awaitEvidence{}, awaitOutcomes{})
		if got == bucketRunnerPending {
			t.Error("a cmd-shell row must not read as runner-pending")
		}
	})
	t.Run("recorded FAIL is not runner-pending", func(t *testing.T) {
		got, _, _ := bucketAwaiting(awaitBrief{Status: "implemented", Gate: "model"}, ci,
			awaitEvidence{Text: "**VERIFY: FAIL**\n"},
			awaitOutcomes{Latest: &awaitOutcome{Outcome: "verify-fail", BlockerKind: "human-action", BlockerRef: "#3"}})
		if got == bucketRunnerPending {
			t.Error("a FAIL already recorded must not re-queue for the runner")
		}
	})
}
