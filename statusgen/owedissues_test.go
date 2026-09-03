package main

import (
	"strings"
	"testing"
)

// hdr is a tiny constructor for a classified lifecycle header in these tests.
func hdr(rel, state string) *specLifecycleHeader {
	h := &specLifecycleHeader{Rel: rel, HasStatus: true, StateRaw: state}
	switch state {
	case lifecycleDraft, lifecycleApproved, lifecycleRouted:
		h.State = state
	}
	if state == lifecycleApproved || state == lifecycleRouted {
		h.Routes = true
		h.RoutesTo = "docs/streams/st/"
	}
	return h
}

func owedDocs(issues []owedIssue) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Doc)
	}
	return out
}

// TestAuthoringOwedDerivation is the positive control (§8.5): an approved,
// uncited document is owed; adding a brief whose sources: CONTAINS the doc's
// repo-relative path flips it to not-owed (the dereference is real, not a title
// match); and draft/routed documents are never owed.
func TestAuthoringOwedDerivation(t *testing.T) {
	headers := []*specLifecycleHeader{
		hdr("spec/plan.md", lifecycleApproved),
		hdr("spec/draft.md", lifecycleDraft),
		hdr("spec/routed.md", lifecycleRouted),
	}

	t.Run("approved + uncited is owed; draft/routed never owed", func(t *testing.T) {
		got := owedDocs(owedIssues(headers, []string{"unrelated source"}, map[string]bool{}))
		if len(got) != 1 || got[0] != "spec/plan.md" {
			t.Fatalf("owed = %v, want exactly [spec/plan.md]", got)
		}
	})

	t.Run("a citing brief flips approved to not-owed (real dereference)", func(t *testing.T) {
		sources := []string{"`spec/plan.md` §2 — the approved plan this brief implements"}
		got := owedDocs(owedIssues(headers, sources, map[string]bool{}))
		if len(got) != 0 {
			t.Fatalf("owed = %v, want none once a brief cites the path", got)
		}
	})

	t.Run("a title mention WITHOUT the path does not clear owed", func(t *testing.T) {
		// The citation rule is a path dereference: naming the document's title
		// without its repo-relative path does NOT count (§8.5).
		sources := []string{"The Plan — a great document we should implement"}
		got := owedDocs(owedIssues(headers, sources, map[string]bool{}))
		if len(got) != 1 || got[0] != "spec/plan.md" {
			t.Fatalf("owed = %v, want [spec/plan.md] (a title mention is not a citation)", got)
		}
	})

	t.Run("a longer path containing the doc path as a substring does not clear owed", func(t *testing.T) {
		// spec/plan.md must not be counted as cited by a citation of a DIFFERENT
		// path it is a substring of.
		sources := []string{"`spec/plan.md-notes` and `docs/spec/plan.md` are other files"}
		got := owedDocs(owedIssues(headers, sources, map[string]bool{}))
		if len(got) != 1 || got[0] != "spec/plan.md" {
			t.Fatalf("owed = %v, want [spec/plan.md] (substring of a longer path is not a citation)", got)
		}
	})
}

// TestOwedIssueMarkerDedup pins idempotency (§8.5 emitter): a document whose
// owed-marker is already in the existing-markers set emits NO payload, so there
// is exactly one open issue per owed document across re-runs.
func TestOwedIssueMarkerDedup(t *testing.T) {
	headers := []*specLifecycleHeader{hdr("spec/plan.md", lifecycleApproved)}
	sources := []string{"unrelated"}

	// First run: nothing filed yet -> one payload.
	first := owedIssues(headers, sources, map[string]bool{})
	if len(first) != 1 {
		t.Fatalf("first run: got %d payloads, want 1", len(first))
	}
	got := first[0]
	if got.Marker != owedMarker("spec/plan.md") {
		t.Errorf("marker = %q, want %q", got.Marker, owedMarker("spec/plan.md"))
	}
	if len(got.Body) == 0 || !strings.HasPrefix(got.Body, got.Marker+"\n") {
		t.Errorf("marker must be the first body line; body starts: %q", firstLine(got.Body))
	}
	if len(got.Labels) != 1 || got.Labels[0] != authoringOwedLabel {
		t.Errorf("labels = %v, want [%s]", got.Labels, authoringOwedLabel)
	}

	// Second run: the marker is already open -> nothing emitted.
	existing := map[string]bool{owedMarker("spec/plan.md"): true}
	second := owedIssues(headers, sources, existing)
	if len(second) != 0 {
		t.Fatalf("re-run over an open owed issue emitted %d payload(s), want 0 (idempotent)", len(second))
	}

	// loadOwedMarkers must recover the marker from a raw `gh issue list --json
	// body` blob, so the workflow can pipe issue bodies straight in.
	root := t.TempDir()
	raw := `[{"body":"` + owedMarker("spec/plan.md") + `\n\n## Authoring owed\n..."}]`
	markersPath := lcWriteFile(t, root, "owed-markers.json", raw)
	set, err := loadOwedMarkers(markersPath)
	if err != nil {
		t.Fatal(err)
	}
	if !set[owedMarker("spec/plan.md")] {
		t.Errorf("loadOwedMarkers did not recover the marker from raw issue-body JSON; set=%v", set)
	}
	if len(owedIssues(headers, sources, set)) != 0 {
		t.Errorf("marker recovered from raw JSON should still dedup")
	}

	// A missing markers path yields an empty set (nothing filed yet), never an
	// error — the first-run case.
	empty, err := loadOwedMarkers("")
	if err != nil || len(empty) != 0 {
		t.Errorf("empty markers path: got (%v, %v), want (empty set, nil)", empty, err)
	}
}
