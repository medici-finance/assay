package topology

import (
	"strings"
	"testing"
)

// cellmodel_test.go — ground-truth/08's half of the loader contract.
//
// WHAT THE CELL MODEL IS. The adoption model scales an enterprise as 10-15
// INDEPENDENT cells (a lead plus its agent fleet). A cell may FILE ISSUES into
// another cell's repos, but never modifies them and never runs jobs against
// them. So there is ONE topology file PER CELL — this tree's is the
// assay cell's instance, never an enterprise registry — and each repo
// entry states that cell's `relationship:` to it: `owned` or `upstream`.
//
// THE STRICT PARSE IS THE ONE CONTROL ON RELATIONSHIP SANITY. Behind it sit
// three independent layers: parse-time refusal (here), test-time proof that it
// REFUSES (the positive controls below), and CI-time diff of every derivation
// (TestTopologyDriftRegistry, TestTopologyValuesMatchSource). This file is the
// middle layer, and it is the layer that would be missing if the refusals were
// only asserted in prose.
//
// THE FOUR DECISIONS THIS FILE PINS, so a later edit has to argue with a test
// rather than with a comment:
//
//	absent relationship        => upstream. Least authority. "Not stated" must
//	                              never read as "owned".
//	unrecognised relationship  => parse ERROR naming the line. There is no third
//	                              legal value, so a present-but-unknown one is a
//	                              typo, and a typo that defaulted would be
//	                              indistinguishable from a stated fact.
//	duplicate slug             => parse ERROR. This IS the in-file one-owner
//	                              mechanism: a repo appears once, so it carries
//	                              one relationship. Across cells, one-owner is an
//	                              audit convention and explicitly not a mechanism.
//	owned with no `cell:`      => parse ERROR. An ownership claim with no
//	                              claimant cannot be audited against the
//	                              one-owner boundary.
//
// SCHEMA EVOLUTION, decided here and not left implicit: both fields are ADDITIVE
// and the schema stays `topology-v1`. Parse REFUSES a schema string it does not
// recognise (topology.go), so a version bump is a hard break for every file not
// edited in the same commit — a cost worth paying only when an EXISTING field
// changes meaning. A new optional field whose absence has a fail-closed reading
// changes no existing meaning: the "a pre-08 topology-v1 file still loads"
// subtest below is that compatibility claim, executed.

// cellFixture is a minimal but COMPLETE topology-v1 document. It is complete on
// purpose: Parse refuses a source missing `repos:` or `labels.system_state`, so
// a fixture that omitted them would fail for a reason the test is not about, and
// a reader would learn nothing from the red.
func cellFixture(cellLine, relLine string) string {
	return "schema: topology-v1\n" +
		cellLine +
		"repos:\n" +
		"  - slug: example-org/tracker\n" +
		"    visibility: public\n" +
		relLine +
		"  - slug: example-org/other\n" +
		"    visibility: unknown\n" +
		"labels:\n" +
		"  system_state:\n" +
		"    - name: verify-gate\n" +
		"      why: a closeable state the machinery emits\n" +
		"  decision_owed:\n" +
		"    - name: question\n" +
		"      why: the escalation label\n"
}

func TestTopologyCellModel(t *testing.T) {
	t.Run("parses cell and both relationship values", func(t *testing.T) {
		got, err := Parse([]byte(cellFixture("cell: platform\n", "    relationship: owned\n")))
		if err != nil {
			t.Fatalf("COULD-NOT-CHECK: a well-formed cell-model source failed to parse: %v", err)
		}
		if got.Cell != "platform" {
			t.Errorf("cell: got %q, want %q", got.Cell, "platform")
		}
		tracker, ok := got.Repo("example-org/tracker")
		if !ok {
			t.Fatal("example-org/tracker is absent from the parsed source")
		}
		if tracker.Relationship != RelationshipOwned || !tracker.RelationshipStated {
			t.Errorf("repos[example-org/tracker]: got relationship=%s stated=%v, want owned/true",
				tracker.Relationship, tracker.RelationshipStated)
		}
		if want := []string{"example-org/tracker"}; len(got.OwnedRepos()) != 1 || got.OwnedRepos()[0] != want[0] {
			t.Errorf("OwnedRepos(): got %v, want %v", got.OwnedRepos(), want)
		}

		up, err := Parse([]byte(cellFixture("cell: platform\n", "    relationship: upstream\n")))
		if err != nil {
			t.Fatalf("COULD-NOT-CHECK: an `upstream` source failed to parse: %v", err)
		}
		r, _ := up.Repo("example-org/tracker")
		if r.Relationship != RelationshipUpstream || !r.RelationshipStated {
			t.Errorf("stated upstream: got relationship=%s stated=%v, want upstream/true", r.Relationship, r.RelationshipStated)
		}
		if len(up.OwnedRepos()) != 0 {
			t.Errorf("OwnedRepos() on an all-upstream source: got %v, want empty", up.OwnedRepos())
		}
	})

	t.Run("absent relationship defaults to upstream, and records that it was NOT stated", func(t *testing.T) {
		got, err := Parse([]byte(cellFixture("cell: platform\n", "")))
		if err != nil {
			t.Fatalf("COULD-NOT-CHECK: a source omitting relationship failed to parse: %v", err)
		}
		for _, r := range got.Repos {
			if r.Relationship != RelationshipUpstream {
				t.Errorf("repos[%s]: absent relationship read as %s — absent must be the LEAST authority (upstream)",
					r.Slug, r.Relationship)
			}
			if r.RelationshipStated {
				t.Errorf("repos[%s]: RelationshipStated is true for a repo that stated nothing — "+
					"the public example's shape check depends on telling silence from a statement", r.Slug)
			}
		}
	})

	t.Run("a pre-cell-model topology-v1 file still loads (additive, no version bump)", func(t *testing.T) {
		// No `cell:`, no `relationship:` anywhere — the shape of every file
		// written before ground-truth/08. It must load, or the additive-schema
		// decision is wrong and topology-v2 was the honest choice.
		got, err := Parse([]byte(cellFixture("", "")))
		if err != nil {
			t.Fatalf("a topology-v1 file predating the cell model no longer parses: %v\n"+
				"  Both fields were added as ADDITIVE and the schema stayed topology-v1 on the strength "+
				"of exactly this: an old file loads, and every repo in it reads `upstream`. If that is no "+
				"longer true the schema version must move, because Parse refuses an unrecognised schema.", err)
		}
		if got.Cell != "" {
			t.Errorf("cell: got %q from a file that states none, want empty", got.Cell)
		}
		if len(got.OwnedRepos()) != 0 {
			t.Errorf("OwnedRepos(): got %v from a file with no relationship keys — an unstated file "+
				"must claim nothing", got.OwnedRepos())
		}
	})

	t.Run("an unrecognised relationship is an error naming the line", func(t *testing.T) {
		_, err := Parse([]byte(cellFixture("cell: platform\n", "    relationship: maintained\n")))
		requireParseError(t, err, "unrecognised relationship value", "maintained", "line 6")
	})

	t.Run("an empty relationship is an error, not a silent default", func(t *testing.T) {
		_, err := Parse([]byte(cellFixture("cell: platform\n", "    relationship: \"\"\n")))
		requireParseError(t, err, "empty relationship value", "relationship", "line 6")
	})

	t.Run("a duplicate slug is an error — the in-file one-owner mechanism", func(t *testing.T) {
		src := "schema: topology-v1\ncell: platform\n" +
			"repos:\n" +
			"  - slug: example-org/tracker\n    relationship: owned\n" +
			"  - slug: example-org/tracker\n    relationship: upstream\n" +
			"labels:\n  system_state:\n    - name: verify-gate\n      why: w\n" +
			"  decision_owed:\n    - name: question\n      why: w\n"
		_, err := Parse([]byte(src))
		requireParseError(t, err, "duplicate slug", "duplicate repo", "example-org/tracker")
	})

	t.Run("`owned` with no cell named is refused — a claim with no claimant", func(t *testing.T) {
		_, err := Parse([]byte(cellFixture("", "    relationship: owned\n")))
		requireParseError(t, err, "owned with no cell", "no `cell:`", "example-org/tracker")
	})

	t.Run("a stated but EMPTY cell is refused", func(t *testing.T) {
		_, err := Parse([]byte(cellFixture("cell: \"\"\n", "    relationship: upstream\n")))
		requireParseError(t, err, "empty cell", "`cell:` is stated but EMPTY", "line 2")
	})

	t.Run("this tree's own source states a cell and a relationship for every repo", func(t *testing.T) {
		// The fixtures above prove the LOADER. This proves the FILE — that
		// topology.yaml actually took the schema up, which no in-memory fixture
		// can show. loadSourceOrFail is three-state: an unreadable source fails.
		src := loadSourceOrFail(t)
		if src.Cell == "" {
			t.Errorf("%s states no `cell:` — this file is the assay cell's INSTANCE, "+
				"not an enterprise registry, and an instance that names no cell cannot state ownership", SourceFile)
		}
		for _, r := range src.Repos {
			if !r.RelationshipStated {
				t.Errorf("%s: repos[%s] states no `relationship:` — it would read as `upstream` by "+
					"default, which is correct but UNSTATED. Every entry in this cell's own file states "+
					"one explicitly, so the default is reserved for files that predate the field", SourceFile, r.Slug)
			}
		}
		if len(src.OwnedRepos()) == 0 {
			t.Errorf("%s claims no owned repo at all — the assay cell owns the toolkit; "+
				"a cell file that owns nothing has nothing for the cross-cell audit to attribute", SourceFile)
		}
	})
}

// TestTopologyCellModelPositiveControl is the proof the cell-model checks can
// fail. Two things need proving separately, because they fail in different ways:
// the PARSE refusals (would a lenient reader still be caught?) and the DRIFT
// comparison over the two new fields (does topologyDiffs actually look at them?).
//
// A green TestTopologyCellModel without these is consistent with a reader that
// refuses everything and a comparison that compares nothing.
func TestTopologyCellModelPositiveControl(t *testing.T) {
	t.Run("the refusals discriminate — the same fixtures parse when made valid", func(t *testing.T) {
		// The negative direction. Each refusal above must be caused by the ONE
		// thing it names; if these valid variants also failed, the subtests would
		// be passing for the wrong reason and would keep passing after the rule
		// they check was deleted.
		valid := []struct {
			name string
			src  string
		}{
			{"owned with a cell named", cellFixture("cell: platform\n", "    relationship: owned\n")},
			{"upstream with no cell named", cellFixture("", "    relationship: upstream\n")},
			{"no relationship at all", cellFixture("cell: platform\n", "")},
		}
		for _, v := range valid {
			if _, err := Parse([]byte(v.src)); err != nil {
				t.Errorf("POSITIVE CONTROL FAILED: %s should PARSE, got: %v\n"+
					"  The refusal subtests are then not discriminating — they would go red for a "+
					"source that is fine, which is how a check gets weakened away.", v.name, err)
			}
		}
	})

	t.Run("the drift comparison catches a bent cell name", func(t *testing.T) {
		src := loadSourceOrFail(t)
		bent := Compiled()
		bent.Cell = bent.Cell + "-other"
		diffs := topologyDiffs(src, bent)
		if len(diffs) == 0 {
			t.Fatal("POSITIVE CONTROL FAILED: renaming the derivation's cell produced NO diff against " +
				"the declared source — the cell comparison is vacuous, and a derivation could claim to " +
				"be a different cell's file entirely.")
		}
		if !strings.Contains(strings.Join(diffs, "\n"), "cell:") {
			t.Errorf("POSITIVE CONTROL FAILED: the diff fired but did not name `cell`: %v", diffs)
		}
	})

	t.Run("the drift comparison catches a flipped relationship", func(t *testing.T) {
		src := loadSourceOrFail(t)
		bent := Compiled()
		flipped := ""
		for i, r := range bent.Repos {
			if r.Relationship == RelationshipOwned {
				bent.Repos[i].Relationship = RelationshipUpstream
				flipped = r.Slug
				break
			}
		}
		if flipped == "" {
			t.Fatal("the derivation states no owned repo, so this control has nothing to flip — " +
				"compiled.go and topology.yaml must both state the cell's owned set")
		}
		diffs := topologyDiffs(src, bent)
		if len(diffs) == 0 {
			t.Fatalf("POSITIVE CONTROL FAILED: flipping repos[%s] from owned to upstream in the "+
				"derivation produced NO diff. Silently downgrading an ownership claim is exactly the "+
				"drift the diff exists to catch.", flipped)
		}
		if !strings.Contains(strings.Join(diffs, "\n"), "relationship") {
			t.Errorf("POSITIVE CONTROL FAILED: the diff fired but did not name `relationship`: %v", diffs)
		}
	})
}

// requireParseError asserts Parse REFUSED, and that the refusal names each
// wanted fragment. Asserting the message content is not pedantry: "line %d" and
// the offending value are what turn a refusal into a fix, and a message that
// stopped naming them would still pass a bare err != nil check.
func requireParseError(t *testing.T, err error, what string, want ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("POSITIVE CONTROL FAILED: %s was ACCEPTED. The strict parse is the single control "+
			"on relationship sanity; a reader that accepts it has no second layer behind it.", what)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("the %s refusal does not name %q — a refusal that does not say WHERE or WHAT "+
				"is a refusal the author has to guess at.\n  got: %v", what, w, err)
		}
	}
}
