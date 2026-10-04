package deskkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func laneRec(seq int, role ActorRole, lane, head string, fs ...Finding) ForgeRecord {
	kind := RecordReview
	if role == RoleWorker {
		kind = RecordReply
	}
	return ForgeRecord{Seq: seq, Kind: kind, Role: role, Lane: lane, Head: head,
		Block: &FindingBlockV1{Schema: FindingBlockSchema, Findings: fs}}
}

// TestLaneKeepsIDsApart is the reported instance: the correctness and security
// reviewers each number findings from A1, so A3 names two unrelated findings. They must
// derive as two findings with independent class, state and evidence.
func TestLaneKeepsIDsApart(t *testing.T) {
	sec := blockingFinding("A3", "workflow-read-scopes", "open")
	cor := blockingFinding("A3", "stale-streak-dir", "open")
	l := DeriveLedger([]ForgeRecord{
		laneRec(1, RoleReviewer, "security", "h1", sec),
		laneRec(2, RoleReviewer, "correctness", "h1", cor),
	})
	if len(l.Findings) != 2 {
		t.Fatalf("two lanes reusing A3 merged into %d finding(s): %v", len(l.Findings), l.FindingIDs())
	}
	s, c := l.Findings["security/A3"], l.Findings["correctness/A3"]
	if s == nil || c == nil {
		t.Fatalf("expected security/A3 and correctness/A3, got %v", l.FindingIDs())
	}
	if s.Class != "workflow-read-scopes" || c.Class != "stale-streak-dir" {
		t.Fatalf("a lane inherited the other lane's class: sec=%q cor=%q", s.Class, c.Class)
	}
	if len(s.Evidence) != 1 || len(c.Evidence) != 1 {
		t.Fatalf("evidence was merged across lanes: sec=%v cor=%v", s.Evidence, c.Evidence)
	}

	// The security lane resolves its A3 at the current head; the correctness A3 must stay open.
	resolved := sec
	resolved.State = StateResolved
	resolved.EvidenceHead = "h1"
	l = DeriveLedger([]ForgeRecord{
		laneRec(1, RoleReviewer, "security", "h1", sec),
		laneRec(2, RoleReviewer, "correctness", "h1", cor),
		laneRec(3, RoleReviewer, "security", "h1", resolved),
	})
	if l.Findings["security/A3"].State != StateResolved {
		t.Fatalf("security A3 not resolved: %s", l.Findings["security/A3"].State)
	}
	if l.Findings["correctness/A3"].State != StateOpen {
		t.Fatalf("a security verdict advanced the correctness finding: %s", l.Findings["correctness/A3"].State)
	}
	if got := l.OpenBlocking(); len(got) != 1 || got[0] != "correctness/A3" {
		t.Fatalf("open blocking = %v, want [correctness/A3]", got)
	}
	if !l.ResolvedAtHead("security/A3", "h1") || l.ResolvedAtHead("correctness/A3", "h1") {
		t.Fatal("ResolvedAtHead did not tell the lanes apart")
	}
}

// TestLaneKeepsRoundsApart: the round cap is per class, and a class name two
// lanes happen to share is still two counters — one lane's worker round never advances the
// other's.
func TestLaneKeepsRoundsApart(t *testing.T) {
	f := func(id string) Finding { return blockingFinding(id, "same-class", "open") }
	var recs []ForgeRecord
	seq := 0
	next := func() int { seq++; return seq }
	recs = append(recs, laneRec(next(), RoleReviewer, "security", "h", f("A1")))
	recs = append(recs, laneRec(next(), RoleReviewer, "correctness", "h", f("A1")))
	// Security lane: worker response then a re-review = one completed round.
	disp := f("A1")
	disp.State = StateDisputed
	recs = append(recs, laneRec(next(), RoleWorker, "security", "h", disp))
	recs = append(recs, laneRec(next(), RoleReviewer, "security", "h", f("A1")))
	l := DeriveLedger(recs)
	if got := l.Rounds["security/same-class"]; got != 1 {
		t.Fatalf("security rounds = %d, want 1 (%v)", got, l.Rounds)
	}
	if got := l.Rounds["correctness/same-class"]; got != 0 {
		t.Fatalf("correctness rounds = %d, want 0 — the other lane's round leaked (%v)", got, l.Rounds)
	}
	if l.Findings["correctness/A1"].Rounds != 0 || l.Findings["security/A1"].Rounds != 1 {
		t.Fatal("per-finding Rounds mirror used the wrong class counter")
	}
}

// TestWorkerLaneResolution: a worker reply names no lane. It attaches to the one
// lane holding the ID, and is reported Blind — asserting nothing — when two lanes hold it.
func TestWorkerLaneResolution(t *testing.T) {
	fixed := blockingFinding("A3", "workflow-read-scopes", "fixed-awaiting-review")

	one := DeriveLedger([]ForgeRecord{
		laneRec(1, RoleReviewer, "security", "h", blockingFinding("A3", "workflow-read-scopes", "open")),
		laneRec(2, RoleWorker, "", "h", fixed),
	})
	if len(one.Findings) != 1 || one.Findings["security/A3"].State != StateFixedAwaitingReview {
		t.Fatalf("lane-less worker reply did not attach to the sole holder: %v", one.FindingIDs())
	}

	both := DeriveLedger([]ForgeRecord{
		laneRec(1, RoleReviewer, "security", "h", blockingFinding("A3", "workflow-read-scopes", "open")),
		laneRec(2, RoleReviewer, "correctness", "h", blockingFinding("A3", "stale-streak-dir", "open")),
		laneRec(3, RoleWorker, "", "h", fixed),
	})
	if len(both.Findings) != 2 {
		t.Fatalf("an ambiguous worker reply created or merged a finding: %v", both.FindingIDs())
	}
	for k, lf := range both.Findings {
		if lf.State != StateOpen {
			t.Fatalf("ambiguous worker reply advanced %s to %s", k, lf.State)
		}
	}
	if len(both.Blind) != 1 || !strings.Contains(both.Blind[0], "names no lane") {
		t.Fatalf("ambiguity not reported as could-not-check: %v", both.Blind)
	}

	// A worker that names the lane in its block is never ambiguous.
	named := fixed
	named.Lane = "correctness"
	got := DeriveLedger([]ForgeRecord{
		laneRec(1, RoleReviewer, "security", "h", blockingFinding("A3", "workflow-read-scopes", "open")),
		laneRec(2, RoleReviewer, "correctness", "h", blockingFinding("A3", "stale-streak-dir", "open")),
		laneRec(3, RoleWorker, "", "h", named),
	})
	if got.Findings["correctness/A3"].State != StateFixedAwaitingReview || got.Findings["security/A3"].State != StateOpen {
		t.Fatalf("explicit block lane not honoured: %v", got.FindingIDs())
	}
}

// TestNoLaneKeysByBareID: with no lane established anywhere the ledger keys by the
// bare ID exactly as before, so a single-lane thread reads the same.
func TestNoLaneKeysByBareID(t *testing.T) {
	l := DeriveLedger([]ForgeRecord{laneRec(1, RoleReviewer, "", "h", blockingFinding("A", "c", "open"))})
	if l.Findings["A"] == nil || l.Rounds["c"] != 0 {
		t.Fatalf("unlaned finding not keyed by bare id/class: %v %v", l.FindingIDs(), l.Rounds)
	}
}

// TestLaneKeyFieldsRefused: the write gate refuses the key separator in a lane, an id or a
// class, and any lane word that is not a published lane, so a key written through the gate
// is never ambiguous and a typo cannot fork a finding.
func TestLaneKeyFieldsRefused(t *testing.T) {
	cases := []struct {
		name, id, class, lane string
		ok                    bool
	}{
		{"separator in lane", "A", "c", "sec/urity", false},
		{"separator in id", "security/A3", "c", "", false},
		{"separator in class", "A3", "security/c", "", false},
		{"unknown lane word", "A3", "c", "sec", false},
		{"the reserved ambiguous lane", "A3", "c", LaneAmbiguous, false},
		{"published lane", "A3", "c", "security", true},
		{"published lane, other case", "A3", "c", "Fact-Check", true},
		{"no lane", "A3", "c", "", true},
	}
	for _, tc := range cases {
		f := blockingFinding(tc.id, tc.class, "open")
		f.Lane = tc.lane
		b := &FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{f}}
		err := b.Validate(RoleReviewer)
		if tc.ok && err != nil {
			t.Errorf("%s: refused: %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
	}
}

// TestLaneArbiterPacket: a laned class driven to the cap yields exactly one packet, keyed by
// the lane-scoped class and holding only that lane's finding; the other lane's same-named
// class is neither held nor counted into it.
func TestLaneArbiterPacket(t *testing.T) {
	sec := blockingFinding("A1", "c", "open")
	cor := blockingFinding("A1", "c", "open")
	disp := blockingFinding("A1", "c", "disputed")
	disp.Lane = "security" // a worker names the lane: A1 is held by two lanes
	seq := 0
	next := func() int { seq++; return seq }
	recs := []ForgeRecord{
		laneRec(next(), RoleReviewer, "security", "h", sec),
		laneRec(next(), RoleReviewer, "correctness", "h", cor),
	}
	for i := 0; i <= RoundCap; i++ {
		recs = append(recs, laneRec(next(), RoleWorker, "", "h", disp))
		recs = append(recs, laneRec(next(), RoleReviewer, "security", "h", sec))
	}
	l := DeriveLedger(recs)
	if len(l.Arbiter) != 1 {
		t.Fatalf("arbiter packets = %d, want 1: %+v", len(l.Arbiter), l.Arbiter)
	}
	p := l.Arbiter[0]
	if p.Class != "security/c" || len(p.FindingIDs) != 1 || p.FindingIDs[0] != "security/A1" {
		t.Fatalf("packet = class %q findings %v, want security/c holding [security/A1]", p.Class, p.FindingIDs)
	}
	if !strings.Contains(p.Summary, `class "c" (lane security)`) || !strings.Contains(p.Summary, "1 finding(s)") {
		t.Fatalf("packet summary does not name the bare class, its lane and one finding: %q", p.Summary)
	}
	if !l.Held["security/c"] || l.Held["correctness/c"] {
		t.Fatalf("held = %v, want only security/c", l.Held)
	}
	if l.Rounds["correctness/c"] != 0 || l.Findings["correctness/A1"].State != StateOpen {
		t.Fatalf("the correctness lane was counted into the security cap: rounds=%v state=%s",
			l.Rounds, l.Findings["correctness/A1"].State)
	}
	if ref := l.ClassOf(p.Class); ref.Lane != "security" || ref.Class != "c" {
		t.Fatalf("ClassOf(%q) = %+v, want lane security class c", p.Class, ref)
	}
}

// TestLanePrecedence: a worker has no lane of its own, so its block lane wins over the
// record's; a reviewer record with an established lane speaks only for that lane, so a block
// lane naming another is reported and keyed under the record's lane — one lane's verdict can
// never resolve the other lane's finding.
func TestLanePrecedence(t *testing.T) {
	base := func() []ForgeRecord {
		return []ForgeRecord{
			laneRec(1, RoleReviewer, "security", "h", blockingFinding("A3", "sc", "open")),
			laneRec(2, RoleReviewer, "correctness", "h", blockingFinding("A3", "cc", "open")),
		}
	}

	w := blockingFinding("A3", "cc", "fixed-awaiting-review")
	w.Lane = "correctness"
	l := DeriveLedger(append(base(), laneRec(3, RoleWorker, "security", "h", w)))
	if l.Findings["correctness/A3"].State != StateFixedAwaitingReview || l.Findings["security/A3"].State != StateOpen {
		t.Fatalf("worker block lane did not win over the record lane: sec=%s cor=%s",
			l.Findings["security/A3"].State, l.Findings["correctness/A3"].State)
	}

	r := blockingFinding("A3", "sc", "resolved")
	r.Lane = "security"
	r.EvidenceHead = "h"
	l = DeriveLedger(append(base(), laneRec(3, RoleReviewer, "correctness", "h", r)))
	if l.Findings["security/A3"].State != StateOpen {
		t.Fatalf("a correctness-lane record resolved the security finding via its block lane: %s", l.Findings["security/A3"].State)
	}
	if l.Findings["correctness/A3"].State != StateResolved {
		t.Fatalf("the reviewer assertion was not keyed under the record's own lane: %s", l.Findings["correctness/A3"].State)
	}
	if len(l.Blind) != 1 || !strings.Contains(l.Blind[0], "speaks only for its own lane") {
		t.Fatalf("lane mismatch not reported as itself: %v", l.Blind)
	}

	// A reviewer record with no established lane takes the block's.
	l = DeriveLedger([]ForgeRecord{laneRec(1, RoleReviewer, "", "h", r)})
	if l.Findings["security/A3"] == nil {
		t.Fatalf("lane-less reviewer record did not take the block lane: %v", l.FindingIDs())
	}
}

// TestLaneCaseFolded: lane names compare case-insensitively, so "Security" and "security"
// are one lane, not two identity spaces.
func TestLaneCaseFolded(t *testing.T) {
	l := DeriveLedger([]ForgeRecord{
		laneRec(1, RoleReviewer, "Security", "h", blockingFinding("A3", "c", "open")),
		laneRec(2, RoleReviewer, "security ", "h", blockingFinding("A3", "c", "open")),
	})
	if len(l.Findings) != 1 || l.Findings["security/A3"] == nil {
		t.Fatalf("lane case or spacing forked the finding: %v", l.FindingIDs())
	}
}

// TestAmbiguousLaneResolvesNothing: a reviewer record whose lane is LaneAmbiguous (one body
// claiming both verdicts) speaks only for that reserved lane. A block that states a published
// lane is reported and ignored, so the record can raise a finding but never resolve one of a
// published lane.
func TestAmbiguousLaneResolvesNothing(t *testing.T) {
	for _, target := range []string{"correctness", "security"} {
		r := blockingFinding("A3", "c", "resolved")
		r.Lane = target
		r.EvidenceHead = "h"
		l := DeriveLedger([]ForgeRecord{
			laneRec(1, RoleReviewer, target, "h", blockingFinding("A3", "c", "open")),
			laneRec(2, RoleReviewer, LaneAmbiguous, "h", r),
		})
		if got := l.Findings[target+"/A3"].State; got != StateOpen {
			t.Errorf("a both-verdict record stating lane %s resolved that lane's A3: %s", target, got)
		}
		if len(l.Blind) != 1 || !strings.Contains(l.Blind[0], "speaks only for its own lane") {
			t.Errorf("lane %s: the stated block lane was not reported as itself: %v", target, l.Blind)
		}
		if got := l.ContentDefects(); len(got) != 1 || got[0] != target+"/A3" {
			t.Errorf("lane %s: content defects = %v, want [%s/A3]", target, got, target)
		}
	}
}

// TestReadSideKeyRules: the fold re-applies the write gate's key rules to a record that
// bypassed it. A block lane outside the vocabulary is not honoured (so no block writes into
// the reserved ambiguous space), and an id carrying the key separator is not keyed (so a
// lane-less "security/A3" cannot land on the security lane's A3).
func TestReadSideKeyRules(t *testing.T) {
	inject := blockingFinding("A3", "c", "resolved")
	inject.Lane = LaneAmbiguous
	inject.EvidenceHead = "h"
	l := DeriveLedger([]ForgeRecord{
		laneRec(1, RoleReviewer, LaneAmbiguous, "h", blockingFinding("A3", "c", "open")),
		laneRec(2, RoleReviewer, "", "h", inject),
	})
	if got := l.Findings[LaneAmbiguous+"/A3"].State; got != StateOpen {
		t.Errorf("a block naming the reserved lane resolved its finding: %s", got)
	}
	if !blindHas(l, "not a published lane") {
		t.Errorf("unknown block lane not reported: %v", l.Blind)
	}

	slash := blockingFinding("security/A3", "c", "resolved")
	slash.EvidenceHead = "h"
	l = DeriveLedger([]ForgeRecord{
		laneRec(1, RoleReviewer, "security", "h", blockingFinding("A3", "c", "open")),
		laneRec(2, RoleReviewer, "", "h", slash),
	})
	if got := l.Findings["security/A3"].State; got != StateOpen {
		t.Errorf("a lane-less id carrying the separator resolved the security lane's A3: %s", got)
	}
	if !blindHas(l, "would collide") {
		t.Errorf("separator in an id not reported: %v", l.Blind)
	}
}

// TestMixedLanesReported is the class guard over lane-less reviewer records: a payload that
// sets the lane on some reviewer records and not others is reported Blind, whichever producer
// built it. A thread laned throughout, or lane-less throughout, is not.
func TestMixedLanesReported(t *testing.T) {
	f := func() Finding { return blockingFinding("A3", "c", "open") }
	mixed := DeriveLedger([]ForgeRecord{laneRec(1, RoleReviewer, "security", "h", f()), laneRec(2, RoleReviewer, "", "h", f())})
	if !blindHas(mixed, "mix laned (seq 1) and lane-less (seq 2)") {
		t.Fatalf("mixed laned/lane-less reviewer records not reported: %v", mixed.Blind)
	}
	for name, recs := range map[string][]ForgeRecord{
		"laned":     {laneRec(1, RoleReviewer, "security", "h", f()), laneRec(2, RoleReviewer, "correctness", "h", f())},
		"lane-less": {laneRec(1, RoleReviewer, "", "h", f()), laneRec(2, RoleReviewer, "", "h", f())},
		"worker":    {laneRec(1, RoleReviewer, "security", "h", f()), laneRec(2, RoleWorker, "", "h", f())},
	} {
		if l := DeriveLedger(recs); len(l.Blind) != 0 {
			t.Errorf("%s thread reported blind: %v", name, l.Blind)
		}
	}
}

// TestClassKeyFollowsLatest: a finding's class is the one its latest record names, so its
// lane-scoped class key — and the round counter it mirrors — follows a class change.
func TestClassKeyFollowsLatest(t *testing.T) {
	for _, lane := range []string{"", "security"} {
		l := DeriveLedger([]ForgeRecord{
			laneRec(1, RoleReviewer, lane, "h", blockingFinding("A1", "c", "open")),
			laneRec(2, RoleReviewer, lane, "h", blockingFinding("A1", "d", "open")),
		})
		lf := l.Findings[ledgerKey(lane, "A1")]
		if want := ledgerKey(lane, "d"); lf == nil || lf.ClassKey != want {
			t.Fatalf("lane %q: class key did not follow the class change: %+v, want %s", lane, lf, want)
		}
		if got := l.findingsInClass(ledgerKey(lane, "c")); len(got) != 0 {
			t.Fatalf("lane %q: the finding still counts in its old class: %v", lane, got)
		}
	}
}

func blindHas(l *FindingLedger, sub string) bool {
	for _, b := range l.Blind {
		if strings.Contains(b, sub) {
			return true
		}
	}
	return false
}

// laneLessLiterals returns the position of every ForgeRecord composite literal
// in src that does not set Lane. A reader that builds a ForgeRecord and forgets the lane
// reintroduces the cross-lane merge for that caller, whatever DeriveLedger does.
func laneLessLiterals(src string) []int {
	var bad []int
	for from := 0; ; {
		i := strings.Index(src[from:], "ForgeRecord{")
		if i < 0 {
			return bad
		}
		start := from + i
		open := start + len("ForgeRecord{") - 1
		depth, end := 0, open
		for ; end < len(src); end++ {
			if src[end] == '{' {
				depth++
			} else if src[end] == '}' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if !strings.Contains(src[open:end], "Lane:") {
			bad = append(bad, start)
		}
		from = end
	}
}

// TestReadersSetLane is the CLASS guard for the lane-merge defect: every
// production site that builds a ForgeRecord from a forge review must establish its lane.
// The positive control proves the matcher still flags a planted lane-less literal.
func TestReadersSetLane(t *testing.T) {
	planted := "recs = append(recs, deskkit.ForgeRecord{Seq: 1, Role: deskkit.RoleReviewer, Head: h})"
	if len(laneLessLiterals(planted)) != 1 {
		t.Fatal("positive control: the guard did not flag a lane-less ForgeRecord literal")
	}
	ok := "x := ForgeRecord{Seq: 1, Lane: \"security\"}"
	if len(laneLessLiterals(ok)) != 0 {
		t.Fatal("positive control: the guard flagged a literal that sets Lane")
	}

	root := filepath.Join("..", "..") // tools/desk
	scanned := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		scanned++
		for _, at := range laneLessLiterals(string(b)) {
			line := 1 + strings.Count(string(b[:at]), "\n")
			t.Errorf("%s:%d builds a ForgeRecord without a Lane — two review lanes reusing a finding id would merge", p, line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 50 {
		t.Fatalf("guard scanned only %d files — the walk root is wrong", scanned)
	}
}
