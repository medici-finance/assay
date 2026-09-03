package main

import (
	"strings"
	"testing"
)

// containsTag reports whether any line in msgs carries the [tag] rule token.
func containsTag(msgs []string, tag string) bool {
	for _, m := range msgs {
		if strings.Contains(m, "["+tag+"]") {
			return true
		}
	}
	return false
}

// mentionsDoc reports whether any line names the given doc path AND carries tag.
func mentionsDoc(msgs []string, doc, tag string) bool {
	for _, m := range msgs {
		if strings.Contains(m, doc) && strings.Contains(m, "["+tag+"]") {
			return true
		}
	}
	return false
}

// TestRoutesToRequiredLint pins §8.3: an approved/routed document missing
// `**Routes-to:**` reddens the lint (a PROBLEM tagged [lifecycle-routes-to]); a
// draft does not; an approved/routed WITH a destination does not.
func TestRoutesToRequiredLint(t *testing.T) {
	root := t.TempDir()
	lcWriteFile(t, root, "spec/approved-no-routes.md", "# A\n\n**Status:** approved\n")
	lcWriteFile(t, root, "spec/routed-no-routes.md", "# B\n\n**Status:** routed\n")
	lcWriteFile(t, root, "spec/approved-ok.md", "# C\n\n**Status:** approved\n**Routes-to:** docs/streams/x/\n")
	lcWriteFile(t, root, "spec/draft.md", "# D\n\n**Status:** draft\n")
	lcWriteFile(t, root, "spec/legacy.md", "# E\n\n**Status:** DRAFT — legacy prose\n")

	problems, notices := lifecycleLintChecks(root, nil)

	if !mentionsDoc(problems, "spec/approved-no-routes.md", tagLifecycleRoutesTo) {
		t.Errorf("approved w/o Routes-to should PROBLEM [%s]; problems=%v", tagLifecycleRoutesTo, problems)
	}
	if !mentionsDoc(problems, "spec/routed-no-routes.md", tagLifecycleRoutesTo) {
		t.Errorf("routed w/o Routes-to should PROBLEM [%s]; problems=%v", tagLifecycleRoutesTo, problems)
	}
	for _, ok := range []string{"spec/approved-ok.md", "spec/draft.md"} {
		if mentionsDoc(problems, ok, tagLifecycleRoutesTo) {
			t.Errorf("%s should NOT trip the Routes-to PROBLEM; problems=%v", ok, problems)
		}
	}
	// A draft is exempt from Routes-to and is classified, so it must not appear as
	// unclassified either.
	if mentionsDoc(notices, "spec/draft.md", tagLifecycleUnclassified) {
		t.Errorf("a valid draft must not be flagged unclassified; notices=%v", notices)
	}
	// The uppercase-DRAFT legacy doc is unclassified (a NOTICE), never a PROBLEM.
	if mentionsDoc(problems, "spec/legacy.md", tagLifecycleRoutesTo) {
		t.Errorf("an unclassified legacy doc must never PROBLEM; problems=%v", problems)
	}
	if !mentionsDoc(notices, "spec/legacy.md", tagLifecycleUnclassified) {
		t.Errorf("uppercase DRAFT should be an unclassified NOTICE; notices=%v", notices)
	}
}

// TestLifecycleOwedNoticeIntegration exercises the §8.5 owed condition through
// the full lint (loadStreams + allBriefSources): an approved doc no brief cites
// is an owed NOTICE; adding a brief whose sources: contains the doc's path
// removes the NOTICE — proving the owed condition is a real dereference against
// authored provenance, not merely that the detector ran.
func TestLifecycleOwedNoticeIntegration(t *testing.T) {
	build := func(t *testing.T, withCitingBrief bool) (problems, notices []string) {
		t.Helper()
		root := t.TempDir()
		lcWriteFile(t, root, "spec/plan.md", "# Plan\n\n**Status:** approved\n**Routes-to:** docs/streams/st/\n")
		lcWriteFile(t, root, "docs/streams/st/README.md",
			"---\nstream: st\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# St\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|---|---|---|---|---|---|\n")
		src := `["fixture origin"]`
		if withCitingBrief {
			src = "[\"`spec/plan.md` §1 — the approved plan this brief routes\"]"
		}
		lcWriteFile(t, root, "docs/streams/st/brief-01-x.md",
			"---\nbrief: st/01\ntitle: X\nwave: 0\ndepends: []\nunblocks: []\neffort: M\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v1\nauthored: 2026-08-30 by fixture\nsources: "+src+"\n---\n\n# Brief 01\n\n## Verify\n| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n\n## Evidence\n<!-- x -->\n\n## Review\nGate: model.\n")
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatalf("loadStreams: %v", err)
		}
		return lifecycleLintChecks(root, streams)
	}

	t.Run("uncited approved is owed", func(t *testing.T) {
		_, notices := build(t, false)
		if !mentionsDoc(notices, "spec/plan.md", tagLifecycleOwed) {
			t.Errorf("uncited approved doc should be OWED [%s]; notices=%v", tagLifecycleOwed, notices)
		}
	})
	t.Run("a citing brief clears the owed NOTICE", func(t *testing.T) {
		_, notices := build(t, true)
		if mentionsDoc(notices, "spec/plan.md", tagLifecycleOwed) {
			t.Errorf("a brief citing the doc's path must clear the owed NOTICE (real dereference); notices=%v", notices)
		}
	})
}
