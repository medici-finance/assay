package main

import "testing"

// TestClassifyCueFamilies pins one example per relay family and the substantive
// default. These are the rules the ClassifierVersion is a version OF: if any row here
// changes, the version constant must change in the same commit (see
// TestClassifierVersionIsPinnedToItsScore).
func TestClassifyCueFamilies(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		class  Class
		family string
	}{
		{"sync cue", "sync to main", ClassRelay, familySync},
		{"sync cue, punctuated", "Sync to main, please.", ClassRelay, familySync},
		{"state echo", "PR #1401 is merged", ClassRelay, familyState},
		{"state echo, ci", "CI is green", ClassRelay, familyState},
		{"poke", "continue", ClassRelay, familyPoke},
		{"poke, bare ack", "ok", ClassRelay, familyPoke},
		{"lookup", "where are we on the review queue", ClassRelay, familyLookup},
		{"lookup, done", "is it done", ClassRelay, familyLookup},
		{"substantive short", "Prefer counts over identifiers in the artifact.", ClassSubstantive, ""},
		{"substantive corrective", "stop, do not flip that PR to ready", ClassSubstantive, ""},
		{"empty", "   ", ClassEmpty, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewClassifier().Classify("s", tc.text)
			if got.Class != tc.class || got.Family != tc.family {
				t.Fatalf("Classify(%q) = %s/%s, want %s/%s", tc.text, got.Class, got.Family, tc.class, tc.family)
			}
		})
	}
}

// TestLongMessageIsSubstantiveEvenWithACue is the failure-direction rule made
// explicit: a long message that happens to contain a cue is a decision that mentions
// plumbing, not plumbing. This is the clause that makes the classifier UNDER-count
// relays, which is the direction the README commits to.
func TestLongMessageIsSubstantiveEvenWithACue(t *testing.T) {
	long := "merge main into your branch and then rerun the full suite before you push anything at all to the remote"
	got := NewClassifier().Classify("s", long)
	if got.Class != ClassSubstantive {
		t.Fatalf("long cue-bearing message classified %s, want substantive", got.Class)
	}
}

// TestNearDuplicateIsRelayWithinASession covers the re-send shape: the operator
// repeating himself because the first send did not land. It must NOT fire across two
// sessions — the same instruction to two agents is not a re-send.
func TestNearDuplicateIsRelayWithinASession(t *testing.T) {
	c := NewClassifier()
	first := c.Classify("s1", "Please re-run the whole verification table and paste the real output into the PR body")
	if first.Class != ClassSubstantive {
		t.Fatalf("first send classified %s, want substantive", first.Class)
	}
	second := c.Classify("s1", "Please re-run the whole verification table and paste the real output into the PR body.")
	if second.Class != ClassRelay || second.Family != familyDuplicate {
		t.Fatalf("re-send classified %s/%s, want relay/duplicate", second.Class, second.Family)
	}

	c2 := NewClassifier()
	c2.Classify("s1", "Please re-run the whole verification table and paste the real output into the PR body")
	cross := c2.Classify("s2", "Please re-run the whole verification table and paste the real output into the PR body")
	if cross.Class == ClassRelay && cross.Family == familyDuplicate {
		t.Fatal("the same instruction sent to a DIFFERENT session was counted as a re-send")
	}
}

// TestCorrectionRecurrenceCandidates pins the candidate counter: two rephrasings of
// the same correction are one candidate; a single correction is none.
func TestCorrectionRecurrenceCandidates(t *testing.T) {
	c := NewClassifier()
	var labels []Label
	for _, m := range []string{
		"no, do not push to main; open a draft PR",
		"stop, do not flip that PR to ready",
		"stop, do not flip a PR to ready again",
		"Version the classifier so the trend is attributable.",
	} {
		labels = append(labels, c.Classify("s", m))
	}
	corrective := 0
	for _, l := range labels {
		if l.Corrective {
			corrective++
		}
	}
	if corrective != 3 {
		t.Fatalf("corrective messages = %d, want 3", corrective)
	}
	if got := correctionCandidates(labels); got != 1 {
		t.Fatalf("recurrence candidates = %d, want 1 (the two ready-flip corrections)", got)
	}
}

// TestJaccardEmptySetsAreNotSimilar pins the guard that stops two content-free turns
// from being "duplicates" of each other.
func TestJaccardEmptySetsAreNotSimilar(t *testing.T) {
	if got := jaccard(map[string]struct{}{}, map[string]struct{}{}); got != 0 {
		t.Fatalf("jaccard(∅,∅) = %v, want 0", got)
	}
}

// TestNormaliseIsPunctuationBlind proves the transform the cue patterns rely on:
// three spellings of one poke must reach the matcher identically.
func TestNormaliseIsPunctuationBlind(t *testing.T) {
	want := "sync to main"
	for _, in := range []string{"Sync to main", "`sync to main`", "**Sync to main!**", "  sync   to  main  "} {
		if got := normalise(in); got != want {
			t.Fatalf("normalise(%q) = %q, want %q", in, got, want)
		}
	}
}
