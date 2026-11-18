package main

import (
	"strings"
	"testing"
)

// regression: #6616
func TestEvidenceRepairVerifyRow(t *testing.T) {
	installStatusgenFixture(t)
	f, repair := repairChain(t, repairSplit, repairMessage)
	f.withRepair(repair)
	parent := vfRev(t, f.root, f.amendments[0]+"^1")
	historical, e := flipEvidenceAt(f.root+"/docs/streams/vf", parent, "brief-01-flip.md")
	if e != nil {
		t.Fatal(e)
	}
	read := func(rev string) (string, error) {
		if rev == repair {
			return "| 1 | `true` | exit 1 | check | sh |", nil
		}
		return "| 1 | `true` | exit 0 | check | sh |", nil
	}
	if _, _, why := evidenceRepairBaseline(f.path, historical, f.descriptor, parent, true, read); !strings.Contains(why, "cannot touch any Verify row") {
		t.Fatalf("wanted Verify-row refusal, got %q", why)
	}
	same := func(string) (string, error) { return "| 1 | `true` | exit 0 | check | sh |", nil }
	if _, used, why := evidenceRepairBaseline(f.path, historical, f.descriptor, parent, true, same); why != "" || len(used) != 1 {
		t.Fatalf("wanted the repair applied, got %q (%d used)", why, len(used))
	}
}

// regression: #6616
func TestEvidenceRepairEditorial(t *testing.T) {
	before := "Historical result retained.\n" + repairDelivered
	ok := repairSplit(before)
	if why := evidenceRepairEditorial(before, ok); why != "" {
		t.Fatalf("desk-containers/15 shape refused: %s", why)
	}
	for _, c := range []struct{ name, after, want string }{
		{"command", strings.Replace(ok, "go test ./x", "go test ./y", 1), "Command cell"},
		{"expected", strings.Replace(ok, "| exit 0 | exit 0; ok", "| exit 1 | exit 0; ok", 1), "Expected cell"},
		{"date", strings.Replace(ok, "| 2026-10-16 | verifier", "| 2026-10-17 | verifier", 1), "Date / Runner text"},
		{"runner", strings.Replace(ok, "verifier-app[bot] (model verifier) (on-behalf", "other-app[bot] (model verifier) (on-behalf", 1), "Date / Runner text"},
		{"principal", strings.Replace(ok, "(on-behalf-of human:alex) |", "(on-behalf-of human:alex) (on-behalf-of human:bo) |", 1), "Date / Runner text"},
		{"row", ok + "| 2 | go vet | exit 0 | exit 0 | 2026-10-16 | x |\n", "add or remove table rows"},
		{"prose", strings.Replace(ok, "Historical result retained.\n", "", 1), "removed or altered"},
		{"marker", ok + "Verification-Table: delivery=x\n", "cannot add a marker"},
		{"verdict", ok + "VERIFY: PASS\n", "cannot add a verdict"},
		{"header", strings.Replace(ok, "| Date | Runner |", "| When | Runner |", 1), "split the Date / Runner header"},
	} {
		if why := evidenceRepairEditorial(before, c.after); !strings.Contains(why, c.want) {
			t.Errorf("%s: wanted %q, got %q", c.name, c.want, why)
		}
	}
}

// regression: #6616
func TestEvidenceRepairRecordKind(t *testing.T) {
	vt := "Verification-Table: delivery=a base=b original=c"
	at := "Acceptance-Transition: link=1 amendment=" + strings.Repeat("a", 40) + " approval=" + rlURL(rlRepo, rlIssue, rlComment)
	er := "Evidence-Repair: commit=" + strings.Repeat("b", 40) + " approval=https://github.com/" + rlRepo + "/pull/77#issuecomment-1"
	if transition, why := acceptanceRecordKind(vt + "\n" + at + "\n" + er); why != "" || !transition {
		t.Fatalf("chain plus repair refused: %q", why)
	}
	if _, why := acceptanceRecordKind(vt + "\n" + er); why == "" {
		t.Fatal("a repair without a transition was admitted")
	}
	if _, why := acceptanceRecordKind(vt + "\n" + at + "\n" + er + "\n" + er); why == "" {
		t.Fatal("a repeated repair was admitted")
	}
}
