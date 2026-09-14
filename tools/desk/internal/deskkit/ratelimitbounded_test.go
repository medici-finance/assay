package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// plantRow appends one row at an explicit instant. The meters read nothing but ts, tool,
// result, repo and pr, so those are what the fixtures below control.
func plantRow(t *testing.T, dir string, at time.Time, tool, repo string, pr int, result string) {
	t.Helper()
	p := pr
	e := Entry{
		TS: at.UTC().Format(time.RFC3339), Tool: tool, Verb: "comment",
		Repo: repo, PR: &p, Result: result, Detail: "planted",
		ArgsDigest: "0000", SourceSHA: "dev", BuiltAt: "dev", SessionTag: "test-session",
	}
	line, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	appendRaw(t, filepath.Join(dir, "audit.jsonl"), string(line))
}

// wholeParsePointsFor is the pre-#1035 reader, kept in the test as the ORACLE the bounded
// arm is compared against. Comparing the bounded arm to itself would prove nothing.
func wholeParsePointsFor(t *testing.T, tool string) []auditPoint {
	t.Helper()
	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	mine, err := pointsFrom(entries, CanonicalToolKeyOr(tool), tool)
	if err != nil {
		t.Fatalf("pointsFrom: %v", err)
	}
	sortPoints(mine)
	return mine
}

// TestBoundedPointsForMatchesFullParseVerdicts is the SPOF row. The bounded reader is only
// allowed to stop where the meter would have stopped reading, so the assertion compares the
// meters' VERDICTS — not their counts, and not the two readers' slice lengths, either of
// which can agree while the answer differs.
//
// The fixture is built to straddle every stop condition at once:
//   - charged writes either side of the one-hour budget window edge;
//   - a trailing consecutive-refusal run of exactly BreakerTrip whose OLDEST members are
//     days old, which a time-horizon reader would truncate into a closed breaker;
//   - an in-scope progress entry that resets a longer run on a second target;
//   - results the breaker ignores (noop / ratelimited / dryrun) interleaved throughout.
func TestBoundedPointsForMatchesFullParseVerdicts(t *testing.T) {
	dir := setup(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	now := time.Now().UTC()
	const tool = "deskpost"
	const repo = "example-org/repo"

	// Oldest first, as the file is written.
	// A refusal run on PR 11 that begins DAYS ago and ends one minute ago: run == 5.
	plantRow(t, dir, now.Add(-96*time.Hour), tool, repo, 11, ResultRefused)
	plantRow(t, dir, now.Add(-72*time.Hour), tool, repo, 11, ResultRefused)
	plantRow(t, dir, now.Add(-48*time.Hour), tool, repo, 11, ResultRefused)
	// Noise the breaker must ignore rather than treat as progress.
	plantRow(t, dir, now.Add(-40*time.Hour), tool, repo, 11, ResultNoop)
	plantRow(t, dir, now.Add(-36*time.Hour), tool, repo, 11, ResultRateLimited)
	plantRow(t, dir, now.Add(-24*time.Hour), tool, repo, 11, ResultRefused)
	// Another tool's rows, which must not enter this tool's meters at all.
	plantRow(t, dir, now.Add(-20*time.Hour), "deskpr", repo, 11, ResultRefused)
	// A long refusal run on PR 22 that a progress entry RESETS.
	for i := 0; i < 8; i++ {
		plantRow(t, dir, now.Add(-time.Duration(19-i)*time.Hour), tool, repo, 22, ResultRefused)
	}
	plantRow(t, dir, now.Add(-90*time.Minute), tool, repo, 22, ResultOK)
	// Charged writes either side of the one-hour window edge.
	plantRow(t, dir, now.Add(-70*time.Minute), tool, repo, 33, ResultOK)
	plantRow(t, dir, now.Add(-50*time.Minute), tool, repo, 33, ResultOK)
	plantRow(t, dir, now.Add(-10*time.Minute), tool, repo, 33, ResultOK)
	plantRow(t, dir, now.Add(-time.Minute), tool, repo, 11, ResultRefused)

	// The fixture must actually trip something, or the comparison is vacuous.
	oracle := wholeParsePointsFor(t, tool)
	if run, _, _ := breakerRun(oracle, func(e auditPoint) bool {
		return e.repo == repo && e.pr != nil && *e.pr == 11
	}); run < BreakerTrip {
		t.Fatalf("fixture too weak: PR 11's run is %d, want at least BreakerTrip (%d)", run, BreakerTrip)
	}

	bounded, ok := boundedPointsFor(CanonicalToolKeyOr(tool), now)
	if !ok {
		t.Fatal("the bounded read declined on a well-formed ledger — it should have reached determinacy")
	}
	sortPoints(bounded)

	// The verdicts, tier by tier and target by target.
	type probe struct {
		name string
		f    func(mine []auditPoint) error
	}
	probes := []probe{
		{"breaker pr=11", func(m []auditPoint) error { return checkBreaker(tool, repo, 11, now, m) }},
		{"breaker pr=22", func(m []auditPoint) error { return checkBreaker(tool, repo, 22, now, m) }},
		{"breaker pr=33", func(m []auditPoint) error { return checkBreaker(tool, repo, 33, now, m) }},
		{"breaker repo-wide", func(m []auditPoint) error { return checkBreakerRepo(tool, repo, now, m) }},
		{"breaker backstop", func(m []auditPoint) error { return checkBreakerBackstop(tool, now, m) }},
		{"budget pr=33", func(m []auditPoint) error { return checkPRBudget(tool, repo, 33, now, m) }},
		{"budget repo", func(m []auditPoint) error { return checkRepoBudget(tool, repo, now, m) }},
		{"budget repo-wide", func(m []auditPoint) error { return checkRepoWideBudget(tool, repo, now, m) }},
		{"budget tool-wide", func(m []auditPoint) error { return checkToolBudget(tool, now, m) }},
	}
	tripped := 0
	for _, p := range probes {
		want, got := p.f(oracle), p.f(bounded)
		if (want == nil) != (got == nil) {
			t.Fatalf("%s: bounded verdict %v, whole-parse verdict %v — a bounded read must never differ from the parse it replaces", p.name, got, want)
		}
		if want != nil {
			tripped++
			if want.Error() != got.Error() {
				t.Fatalf("%s: refusal text differs:\n bounded: %s\n   whole: %s", p.name, got.Error(), want.Error())
			}
		}
	}
	if tripped == 0 {
		t.Fatal("fixture too weak: no tier refused, so agreement proves only that both readers said yes")
	}

	// And the end-to-end gate, which is what a caller actually sees.
	if a, b := AllowWriteAt(tool, repo, 11, now), checkBreaker(tool, repo, 11, now, oracle); (a == nil) != (b == nil) {
		t.Fatalf("AllowWriteAt disagrees with the whole-parse breaker on the tripped target: %v vs %v", a, b)
	}
}

// TestBoundedPointsForFallsBackWhenUndetermined is the fail-closed control. "Not determined"
// must never be answered — it must re-read. The two ways determinacy fails in the field are
// the hard cap and a malformed line, and the second must produce the SAME exit-6 refusal the
// whole-file parse has always produced, naming the recovery verb.
func TestBoundedPointsForFallsBackWhenUndetermined(t *testing.T) {
	t.Run("hard cap discards the partial read and the full parse answers", func(t *testing.T) {
		dir := setup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		now := time.Now().UTC()
		const tool = "deskpost"
		const repo = "example-org/repo"

		// A ledger with NO progress entry: the breaker walk can never terminate on a
		// reset, so only the trip or the start of history ends it. Cap it below the row
		// count and the bounded arm must decline.
		for i := 0; i < 40; i++ {
			plantRow(t, dir, now.Add(-time.Duration(40-i)*time.Minute), tool, repo, 11, ResultRateLimited)
		}
		plantRow(t, dir, now.Add(-time.Minute), tool, repo, 11, ResultRefused)

		old := boundedReadMaxLines
		boundedReadMaxLines = 5
		defer func() { boundedReadMaxLines = old }()

		if _, ok := boundedPointsFor(CanonicalToolKeyOr(tool), now); ok {
			t.Fatal("the bounded read answered without reaching determinacy — it must decline")
		}
		// pointsFor still returns the whole parse's answer, cap or no cap.
		got, err := pointsFor(tool, now)
		if err != nil {
			t.Fatalf("pointsFor: %v", err)
		}
		want := wholeParsePointsFor(t, tool)
		if len(got) != len(want) {
			t.Fatalf("fallback returned %d points, want the full parse's %d", len(got), len(want))
		}
	})

	t.Run("a malformed line refuses exactly as the whole-file parse does", func(t *testing.T) {
		dir := setup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		now := time.Now().UTC()
		plantRow(t, dir, now.Add(-time.Minute), "deskpost", "example-org/repo", 11, ResultOK)
		appendRaw(t, filepath.Join(dir, "audit.jsonl"), "{half a line")

		if _, ok := boundedPointsFor("deskpost", now); ok {
			t.Fatal("the bounded read answered over a malformed line — it must decline, never skip")
		}
		_, err := pointsFor("deskpost", now)
		if !IsUnverifiable(err) {
			t.Fatalf("pointsFor over a corrupt ledger = %v, want Unverifiable (exit 6)", err)
		}
		if err == nil || !containsAll(err.Error(), "malformed audit line", "deskaudit recover") {
			t.Fatalf("the refusal must name the line and the recovery verb; got: %v", err)
		}
	})

	t.Run("an unparseable timestamp stays fail-closed", func(t *testing.T) {
		dir := setup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		now := time.Now().UTC()
		plantRow(t, dir, now.Add(-time.Minute), "deskpost", "example-org/repo", 11, ResultOK)
		appendRaw(t, filepath.Join(dir, "audit.jsonl"),
			`{"ts":"not-a-timestamp","tool":"deskpost","verb":"comment","repo":"example-org/repo","pr":11,"headSHA":null,"result":"ok","detail":"bad ts"}`)

		if _, ok := boundedPointsFor("deskpost", now); ok {
			t.Fatal("the bounded read answered over an unparseable timestamp — it must decline")
		}
		_, err := pointsFor("deskpost", now)
		if !IsUnverifiable(err) {
			t.Fatalf("pointsFor over an unparseable ts = %v, want Unverifiable (never a silent assume-under-budget)", err)
		}
	})
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
