package main

// classify_attention_test.go — the v2 attention-class axis.
//
// The relay axis has its own accuracy pin (accuracy_test.go). This file pins the
// SECOND axis three ways, mirroring that discipline:
//
//  1. One EXEMPLAR per family — the rule for each class actually fires.
//  2. A FALSE-POSITIVE corpus of ordinary substantive messages that must all land
//     `other` — the class the whole design must not over-claim into.
//  3. The re-labelled accuracy corpus: every corpus.json entry's `family` ground
//     truth must match the classifier. This is the attention-axis analogue of the
//     relay accuracy floor, and it is the reason the corpus carries a `family`
//     field. A rule change that moves any assignment fails here until the corpus
//     is re-labelled AND ClassifierVersion is bumped (TestClassifierVersionIsPinnedToItsScore).

import (
	"encoding/json"
	"os"
	"testing"
)

// TestAttentionFamilies is Verify row 3.
func TestAttentionFamilies(t *testing.T) {
	// (1) one exemplar per family — proves each rule fires.
	exemplars := []struct {
		text string
		want string
	}{
		{"ask the review desk to pick this up", AttnRoute},
		{"ping the worker desk on this", AttnRoute},
		{"wrong window", AttnRoute},
		{"where are we on the queue", AttnStatus},
		{"what's the status of the release", AttnStatus},
		{"is it done", AttnStatus},
		{"walk me through the rebase step by step", AttnToil},
		{"give me the command to run that", AttnToil},
		{"no, do not push to main", AttnCorrection},
		{"stop, open a draft PR instead", AttnCorrection},
		{"go with option 2", AttnDecision},
		{"ratify the release", AttnDecision},
		{"approve it", AttnDecision},
		{"investigate why the runner queue stalls", AttnIdea},
		{"compare the two approaches and evaluate which is faster", AttnIdea},
		{"ok", AttnAck},
		{"continue", AttnAck},
		{"ship it", AttnAck},
	}
	for _, e := range exemplars {
		norm := normalise(e.text)
		if got := AttentionFamily(norm, len(tokens(norm))); got != e.want {
			t.Errorf("AttentionFamily(%q) = %q, want %q", e.text, got, e.want)
		}
	}

	// (2) false-positive corpus — ordinary substantive messages that must all
	// land `other`. Each is engineered to MENTION a cue word without being that
	// kind of operator load: a rule statement about approvals, a documentation
	// instruction naming a command, a design decision that mentions routing.
	mustBeOther := []string{
		"The desk must never self-approve; route that verdict to the review desk.",
		"Document the exact command that produces the export.",
		"Use the pinned release binary instead of vendoring the shared tool.",
		"Split the collector into a classifier file and an emit file.",
		"it is approved",
		"we should stop and think about whether that step belongs in the loop at all",
		"Prefer counts over identifiers in anything committed to the repository.",
		"Make the percentile nearest-rank so it invents no number nobody observed.",
	}
	for _, m := range mustBeOther {
		norm := normalise(m)
		if got := AttentionFamily(norm, len(tokens(norm))); got != AttnOther {
			t.Errorf("false positive: AttentionFamily(%q) = %q, want %q", m, got, AttnOther)
		}
	}

	// (3) the re-labelled corpus: classifier must agree with every `family` label.
	raw, err := os.ReadFile("testdata/labelled/corpus.json")
	if err != nil {
		t.Fatalf("cannot read the labelled corpus: %v", err)
	}
	var corpus []struct {
		Text   string `json:"text"`
		Family string `json:"family"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("labelled corpus is not valid JSON: %v", err)
	}
	labelled, seen := 0, map[string]bool{}
	for _, e := range corpus {
		if e.Family == "" {
			t.Errorf("corpus entry %q carries no family label — the corpus must be fully re-labelled for v2", e.Text)
			continue
		}
		labelled++
		seen[e.Family] = true
		norm := normalise(e.Text)
		if got := AttentionFamily(norm, len(tokens(norm))); got != e.Family {
			t.Errorf("corpus disagreement: AttentionFamily(%q) = %q, labelled %q — re-label the corpus AND bump ClassifierVersion in the same commit", e.Text, got, e.Family)
		}
	}
	if labelled < 40 {
		t.Fatalf("only %d family-labelled corpus entries — too few to pin the axis", labelled)
	}
	// The corpus must exercise every family with a real member, or a rule could
	// rot unnoticed. `other` and the seven cued families all appear.
	for _, fam := range AllAttentionFamilies {
		if !seen[fam] {
			t.Errorf("no corpus entry is labelled %q — every attention family must have at least one ground-truth member", fam)
		}
	}
	t.Logf("attention axis: %d corpus entries, all %d families covered, classifier in full agreement",
		labelled, len(AllAttentionFamilies))
}
