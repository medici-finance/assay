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

func TestLaneSeparatorRefused(t *testing.T) {
	f := blockingFinding("A", "c", "open")
	f.Lane = "sec/urity"
	b := &FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{f}}
	if err := b.Validate(RoleReviewer); err == nil {
		t.Fatal("a lane containing the key separator was accepted")
	}
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
