package main

import (
	"testing"
	"time"
)

// TestMissingCouplingPartner is Verify row 5: a co-changing pair (A,B) is
// flagged coupled, and a change touching A but not B raises the
// missing-partner signal; independent files raise neither.
func TestMissingCouplingPartner(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

	commits := []Commit{
		testCommitBy("c1", "a@example.test", now.AddDate(0, 0, -5)),
		testCommitBy("c2", "a@example.test", now.AddDate(0, 0, -4)),
		testCommitBy("c3", "a@example.test", now.AddDate(0, 0, -3)),
		// c4 touches A alone — the missing-partner instance.
		testCommitBy("c4", "a@example.test", now.AddDate(0, 0, -2)),
		// c5/c6 touch two files that never co-occur with anything.
		testCommitBy("c5", "a@example.test", now.AddDate(0, 0, -1)),
		testCommitBy("c6", "a@example.test", now),
	}
	diffs := []FileDiff{
		// server.go and server_test.go co-change three times.
		measuredDiff("c1", "server.go", addHunk("x")),
		measuredDiff("c1", "server_test.go", addHunk("y")),
		measuredDiff("c2", "server.go", addHunk("x")),
		measuredDiff("c2", "server_test.go", addHunk("y")),
		measuredDiff("c3", "server.go", addHunk("x")),
		measuredDiff("c3", "server_test.go", addHunk("y")),
		// c4: server.go changes WITHOUT its coupling partner.
		measuredDiff("c4", "server.go", addHunk("x")),
		// independent.go and other.go each change alone, never together.
		measuredDiff("c5", "independent.go", addHunk("z")),
		measuredDiff("c6", "other.go", addHunk("w")),
	}

	pairs, missing := ComputeCoupling(commits, diffs, DefaultCouplingParams(), now)

	var serverPair CouplingRecord
	var foundPair bool
	for _, p := range pairs {
		if (p.PathA == "server.go" && p.PathB == "server_test.go") ||
			(p.PathA == "server_test.go" && p.PathB == "server.go") {
			serverPair = p
			foundPair = true
		}
	}
	if !foundPair {
		t.Fatalf("expected a coupling row for (server.go, server_test.go)")
	}
	if !serverPair.Coupled {
		t.Errorf("server.go/server_test.go co-change 3/4 times, ratio %.2f — expected Coupled=true", serverPair.Ratio)
	}

	var sawMissing bool
	for _, m := range missing {
		if m.CommitSHA == "c4" && m.Path == "server.go" && m.Partner == "server_test.go" {
			sawMissing = true
		}
	}
	if !sawMissing {
		t.Errorf("expected a missing-partner record for c4 touching server.go without server_test.go; got %+v", missing)
	}

	// independent.go / other.go never co-occur: no coupling row, no
	// missing-partner record naming either of them.
	for _, p := range pairs {
		if (p.PathA == "independent.go" || p.PathB == "independent.go") &&
			(p.PathA == "other.go" || p.PathB == "other.go") {
			t.Errorf("independent files must not be scored as a coupled pair, got %+v", p)
		}
	}
	for _, m := range missing {
		if m.Path == "independent.go" || m.Path == "other.go" || m.Partner == "independent.go" || m.Partner == "other.go" {
			t.Errorf("independent files must never raise a missing-partner signal, got %+v", m)
		}
	}
}

// TestComputeCoupling_ExcludesShotgunCommits pins the MaxFilesPerCommit
// filter: a bulk commit touching many files at once must not flood the
// coupling table with combinatorial pairs (a k-file commit contributes
// C(k,2) pairs) — the real assay repo has "restage"-shaped commits touching
// 300+ files, and without this filter ComputeCoupling emitted ~184k pairs and
// ~730k missing-partner records from a 507-commit history, which took over
// ten minutes to append one record at a time and drowned any real signal.
func TestComputeCoupling_ExcludesShotgunCommits(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	var commits []Commit
	var diffs []FileDiff

	// A genuine small coupled pair, established across 3 ordinary commits.
	for i, sha := range []string{"c1", "c2", "c3"} {
		commits = append(commits, testCommitBy(sha, "x@example.test", now.AddDate(0, 0, i)))
		diffs = append(diffs, measuredDiff(sha, "real_a.go", addHunk("x")))
		diffs = append(diffs, measuredDiff(sha, "real_b.go", addHunk("y")))
	}

	// One shotgun commit touching 40 files at once (above the default-30
	// threshold) — must contribute zero pairs, not C(40,2)=780 of them.
	shotgunSHA := "shotgun"
	commits = append(commits, testCommitBy(shotgunSHA, "x@example.test", now))
	for i := 0; i < 40; i++ {
		p := "bulk/file" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ".go"
		diffs = append(diffs, measuredDiff(shotgunSHA, p, addHunk("z")))
	}

	pairs, _ := ComputeCoupling(commits, diffs, DefaultCouplingParams(), now)
	if len(pairs) != 1 {
		t.Fatalf("expected exactly the 1 genuine pair (real_a.go, real_b.go), the shotgun commit's C(40,2) pairs must be excluded; got %d pairs", len(pairs))
	}
	if pairs[0].PathA != "real_a.go" || pairs[0].PathB != "real_b.go" {
		t.Errorf("unexpected pair: %+v", pairs[0])
	}
	if !pairs[0].Coupled {
		t.Errorf("expected the genuine pair to be coupled: %+v", pairs[0])
	}
}

// TestCouplingBaselineThreshold pins the MinRatio/MinCoChanges gate: a pair
// that co-occurs only rarely relative to its own change counts is NOT
// flagged coupled even though it shares one co-change.
func TestCouplingBaselineThreshold(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	var commits []Commit
	var diffs []FileDiff
	// a.go changes 10 times; b.go rides along exactly once — far below the
	// default 0.5 ratio baseline.
	for i := 0; i < 10; i++ {
		sha := "c" + string(rune('a'+i))
		commits = append(commits, testCommitBy(sha, "x@example.test", now))
		diffs = append(diffs, measuredDiff(sha, "a.go", addHunk("l")))
	}
	diffs = append(diffs, measuredDiff("ca", "b.go", addHunk("l")))

	pairs, _ := ComputeCoupling(commits, diffs, DefaultCouplingParams(), now)
	for _, p := range pairs {
		if (p.PathA == "a.go" && p.PathB == "b.go") || (p.PathA == "b.go" && p.PathB == "a.go") {
			if p.Coupled {
				t.Errorf("a rare one-off co-change must not clear the ratio baseline: %+v", p)
			}
		}
	}
}
