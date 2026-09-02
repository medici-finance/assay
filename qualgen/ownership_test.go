package main

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func testCommitBy(sha, email string, when time.Time) Commit {
	return Commit{SHA: sha, AuthorEmail: email, AuthorWhen: when}
}

// TestOwnershipIdentitiesHashed pins the privacy guard: the emitted
// identity_shares / role_shares keys are STABLE ANONYMIZED DIGESTS, never a raw
// commit-author email or a bot-account slug — no raw identity may reach a
// committed, published artifact (the leak-sweep surface). It also asserts the
// hashing does NOT distort the measurement: distinct identities stay distinct
// and the share VALUES are preserved.
func TestOwnershipIdentitiesHashed(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	commits := []Commit{
		testCommitBy("c1", "alice@users.noreply.github.com", now),
		testCommitBy("c2", "123456+example-worker-app[bot]@users.noreply.github.com", now),
		testCommitBy("c3", "carol@users.noreply.github.com", now),
	}
	diffs := []FileDiff{
		measuredDiff("c1", "f.go", addHunk("a1", "a2")), // alice: 2 lines
		measuredDiff("c2", "f.go", addHunk("b1")),       // bot: 1 line
		measuredDiff("c3", "f.go", addHunk("c1x")),      // carol: 1 line
	}
	recs := ComputeOwnership(commits, diffs, DefaultIdentityClassifier, DefaultBusFactorThresholdPct, now)
	var rec OwnershipRecord
	for _, r := range recs {
		if r.Grain == "file" && r.Path == "f.go" {
			rec = r
		}
	}
	if rec.Metric == "" {
		t.Fatalf("expected an ownership row for f.go")
	}

	keyRe := regexp.MustCompile(`^sha256-[0-9a-f]{16}$`)
	checkKeys := func(field string, m map[string]float64) {
		for k := range m {
			if !keyRe.MatchString(k) {
				t.Errorf("%s key %q is not a stable anonymized digest (want ^sha256-[0-9a-f]{16}$) — a raw identity leaked into the artifact", field, k)
			}
			if strings.Contains(k, "@") || strings.Contains(k, "noreply.github.com") || strings.Contains(k, "example-worker-app") {
				t.Errorf("%s key %q leaks a raw identity token into the published artifact", field, k)
			}
		}
	}
	checkKeys("identity_shares", rec.IdentityShares)
	checkKeys("role_shares", rec.RoleShares)

	// Measurement preserved: three distinct authors -> three distinct keys.
	if len(rec.IdentityShares) != 3 {
		t.Errorf("hashing must preserve the distinct-identity COUNT: want 3 keys, got %d", len(rec.IdentityShares))
	}
	// Share VALUES preserved: alice authored 2 of 4 surviving lines -> top share 0.5.
	var maxShare float64
	for _, v := range rec.IdentityShares {
		if v > maxShare {
			maxShare = v
		}
	}
	if maxShare < 0.49 || maxShare > 0.51 {
		t.Errorf("hashing must preserve share VALUES: want top share ~0.5, got %v", maxShare)
	}
	// Bus-factor count preserved (independent of the raw key strings).
	if rec.BusFactorIdentity.State != StateMeasured || rec.BusFactorIdentity.Value != 2 {
		t.Errorf("hashing must preserve bus-factor COUNT: want 2, got %+v", rec.BusFactorIdentity)
	}
	// Stability: the same identity hashes identically across calls/runs.
	if hashIdentity("x@example.test") != hashIdentity("x@example.test") {
		t.Error("hashIdentity must be deterministic (stable across runs)")
	}
}

// TestBusFactorConcentration is Verify row 4: a file owned > K% by one
// identity/role yields bus factor 1; an evenly-shared file yields a higher
// bus factor; a single role over threshold surfaces a role-SPOF even when
// line-authors vary.
func TestBusFactorConcentration(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

	t.Run("single owner yields bus factor 1, evenly shared yields more", func(t *testing.T) {
		commits := []Commit{
			testCommitBy("c1", "alice@example.test", now),
			testCommitBy("c2", "bob@example.test", now),
			testCommitBy("c3", "carol@example.test", now),
			testCommitBy("c4", "dave@example.test", now),
		}
		diffs := []FileDiff{
			// owned.go: 100% authored by alice.
			measuredDiff("c1", "owned.go", addHunk("l1", "l2", "l3", "l4")),
			// shared.go: evenly split four ways.
			measuredDiff("c1", "shared.go", addHunk("a1")),
			measuredDiff("c2", "shared.go", addHunk("b1")),
			measuredDiff("c3", "shared.go", addHunk("c1x")),
			measuredDiff("c4", "shared.go", addHunk("d1")),
		}

		recs := ComputeOwnership(commits, diffs, DefaultIdentityClassifier, DefaultBusFactorThresholdPct, now)
		byPath := map[string]OwnershipRecord{}
		for _, r := range recs {
			if r.Grain == "file" {
				byPath[r.Path] = r
			}
		}

		owned, ok := byPath["owned.go"]
		if !ok {
			t.Fatalf("expected an ownership row for owned.go")
		}
		if owned.BusFactorIdentity.State != StateMeasured || owned.BusFactorIdentity.Value != 1 {
			t.Errorf("a file owned entirely by one identity must have bus factor 1, got %+v", owned.BusFactorIdentity)
		}

		shared, ok := byPath["shared.go"]
		if !ok {
			t.Fatalf("expected an ownership row for shared.go")
		}
		if shared.BusFactorIdentity.State != StateMeasured || shared.BusFactorIdentity.Value <= owned.BusFactorIdentity.Value {
			t.Errorf("an evenly-shared file must have a HIGHER bus factor than a singly-owned file: shared=%+v owned=%+v",
				shared.BusFactorIdentity, owned.BusFactorIdentity)
		}
	})

	t.Run("role-SPOF surfaces when one role spans many identities", func(t *testing.T) {
		// Three distinct author identities, all dispatched under the SAME
		// role — identity concentration looks fine (bus factor > 1), but the
		// role is a single point of failure.
		roleClassify := func(c Commit) (string, string) {
			return c.AuthorEmail, "worker-role"
		}
		commits := []Commit{
			testCommitBy("c1", "session-1@example.test", now),
			testCommitBy("c2", "session-2@example.test", now),
			testCommitBy("c3", "session-3@example.test", now),
		}
		diffs := []FileDiff{
			measuredDiff("c1", "spof.go", addHunk("l1")),
			measuredDiff("c2", "spof.go", addHunk("l2")),
			measuredDiff("c3", "spof.go", addHunk("l3")),
		}

		recs := ComputeOwnership(commits, diffs, roleClassify, DefaultBusFactorThresholdPct, now)
		var rec OwnershipRecord
		for _, r := range recs {
			if r.Grain == "file" && r.Path == "spof.go" {
				rec = r
			}
		}
		if rec.BusFactorIdentity.State != StateMeasured || rec.BusFactorIdentity.Value <= 1 {
			t.Fatalf("test fixture invalid: identity bus factor should be > 1 (three distinct identities), got %+v", rec.BusFactorIdentity)
		}
		if rec.BusFactorRole.State != StateMeasured || rec.BusFactorRole.Value != 1 {
			t.Errorf("a single role spanning all identities must have role bus factor 1, got %+v", rec.BusFactorRole)
		}
		if !rec.RoleSPOF {
			t.Error("expected RoleSPOF to be flagged when one role owns above threshold despite varying line-authors")
		}
	})

	t.Run("deletions reduce surviving-line ownership", func(t *testing.T) {
		commits := []Commit{
			testCommitBy("c1", "alice@example.test", now),
			testCommitBy("c2", "alice@example.test", now),
		}
		diffs := []FileDiff{
			measuredDiff("c1", "shrink.go", addHunk("keep", "gone")),
			measuredDiff("c2", "shrink.go", delHunk("gone")),
		}
		recs := ComputeOwnership(commits, diffs, DefaultIdentityClassifier, DefaultBusFactorThresholdPct, now)
		var rec OwnershipRecord
		for _, r := range recs {
			if r.Grain == "file" && r.Path == "shrink.go" {
				rec = r
			}
		}
		if rec.SurvivingLines != 1 {
			t.Errorf("expected 1 surviving line after the delete, got %d", rec.SurvivingLines)
		}
	})

	t.Run("package grain aggregates its files", func(t *testing.T) {
		commits := []Commit{testCommitBy("c1", "alice@example.test", now)}
		diffs := []FileDiff{
			measuredDiff("c1", "pkg/a.go", addHunk("l1", "l2")),
			measuredDiff("c1", "pkg/b.go", addHunk("l3")),
		}
		recs := ComputeOwnership(commits, diffs, DefaultIdentityClassifier, DefaultBusFactorThresholdPct, now)
		var pkgRec OwnershipRecord
		var found bool
		for _, r := range recs {
			if r.Grain == "package" && r.Path == "pkg" {
				pkgRec = r
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a package-grain row for pkg")
		}
		if pkgRec.SurvivingLines != 3 {
			t.Errorf("expected the package row to sum its files' surviving lines, got %d", pkgRec.SurvivingLines)
		}
	})
}
