package main

// workerkind_test.go — a worker run is handed what its KIND of run needs (#2439).
//
// THE DEFECT. A worker dispatched with --pr onto an OPEN change got the implementing
// assignment — "Open the draft PR", `deskpr create`, "Self-register the instant your draft
// PR opens" — and a recreate recipe that cut a detached tree at the mainline tip. A run
// that followed it opened a second change for an item that already had one.
//
// WHAT THESE TESTS PIN. The artifact is the PROMPT. A shepherding run's assignment opens
// nothing and names the change, branch and head it resumes; an implementing run's is
// unchanged. The kit is cut per kind by whole-line markers, the cut never loses unmarked
// text, and a kit whose markers cannot be read is refused rather than guessed at.

import (
	"strings"
	"testing"
)

// implementingScaffold belongs ONLY to a run that has no change open yet. A shepherding run
// handed any of it can open a second change for an item that already has one.
var implementingScaffold = []string{
	"Open the draft PR",
	"Run `deskpr create`",
	"Self-register the instant your draft PR opens",
	"the PR opens THERE",
	"refs/remotes/origin/main --detach",
}

func TestShepherdAssignmentOpensNothing(t *testing.T) {
	for _, kit := range []string{"worker", "worker-objective"} {
		t.Run(kit, func(t *testing.T) {
			prompt := dispatchPrompt(t, kit, "item-1", "--pr", "42")
			assign := assignmentSection(t, prompt)
			for _, s := range implementingScaffold {
				if strings.Contains(assign, s) {
					t.Errorf("a dispatch onto an open change carries the implementing scaffold %q:\n%s", s, assign)
				}
			}
			for _, want := range []string{
				"## Work the open change — do not open another",
				"`" + allowedRepo + "#42`",
				"source branch `feat/item-1`",
				"cut at `" + resumeSHA + "`",
				"Do NOT run `deskpr create`",
				"`deskpr update`",
				"Stop at `implemented`: never set verified/done and never flip a PR ready.",
				"export DESK_LOOP=worker-desk",
				"Release the dispatch claim once your branch is pushed — branch-as-claim takes over:",
				"the open change you are resuming belongs THERE",
				"fetch origin\n",
				" feat/item-1\n```",
				"merge-base --is-ancestor " + resumeSHA + " HEAD",
			} {
				if !strings.Contains(assign, want) {
					t.Errorf("the shepherding assignment lacks %q:\n%s", want, assign)
				}
			}
		})
	}
}

// The implementing half is the control: the same item with no --pr still opens a change,
// and gets none of the shepherding half.
func TestImplementingAssignmentIsUnchangedByKind(t *testing.T) {
	for _, kit := range []string{"worker", "worker-objective"} {
		t.Run(kit, func(t *testing.T) {
			assign := assignmentSection(t, dispatchPrompt(t, kit, "item-1"))
			for _, want := range implementingScaffold {
				if !strings.Contains(assign, want) {
					t.Errorf("the implementing assignment lost %q:\n%s", want, assign)
				}
			}
			for _, banned := range []string{"Work the open change", "Do NOT run `deskpr create`", "RESUMES"} {
				if strings.Contains(assign, banned) {
					t.Errorf("a dispatch with no open change carries the shepherding half %q", banned)
				}
			}
		})
	}
}

// An issue-only item resumed onto its open change keeps the issue-comment verb and never
// gets the create-time trailer rule in its assignment.
func TestShepherdAssignmentForIssueItem(t *testing.T) {
	assign := assignmentSection(t, dispatchPrompt(t, "worker", "issue-77", "--pr", "42"))
	if !strings.Contains(assign, "deskfile attach -R "+allowedRepo+" --to 77 --body-file F") {
		t.Errorf("a resumed issue item lost the sanctioned issue-comment verb:\n%s", assign)
	}
	if strings.Contains(assign, "trailer line `Issue: #77`") {
		t.Errorf("a resumed issue item is told a `deskpr create` trailer rule it can never apply:\n%s", assign)
	}
}

// kitSection is the class-kit half of an emitted prompt.
func kitSection(t *testing.T, prompt, kit string) string {
	t.Helper()
	marker := "# Standing clauses — " + kit
	i := strings.Index(prompt, marker)
	if i < 0 {
		t.Fatalf("prompt has no %q section", marker)
	}
	return prompt[i:]
}

// What is cut per kind, read off the EMITTED prompt.
func TestKitIsCutForTheKindOfRun(t *testing.T) {
	const createTrailer = "An ISSUE-ONLY item's `deskpr create` body must carry the trailer line"
	const continuity = "## Continuity — read the workpad first"
	for _, tc := range []struct {
		kit            string
		pr             bool
		wantTrailer    bool
		wantContinuity bool
	}{
		{kit: "worker", pr: false, wantTrailer: true},
		{kit: "worker", pr: true, wantTrailer: false},
		{kit: "worker-objective", pr: false, wantTrailer: true, wantContinuity: false},
		{kit: "worker-objective", pr: true, wantTrailer: false, wantContinuity: true},
	} {
		name := tc.kit + "/implementing"
		var extra []string
		if tc.pr {
			name, extra = tc.kit+"/shepherding", []string{"--pr", "42"}
		}
		t.Run(name, func(t *testing.T) {
			prompt := dispatchPrompt(t, tc.kit, "item-1", extra...)
			body := kitSection(t, prompt, tc.kit)
			if got := strings.Contains(body, createTrailer); got != tc.wantTrailer {
				t.Errorf("create-time trailer rule present = %v, want %v", got, tc.wantTrailer)
			}
			if got := strings.Contains(body, continuity); got != tc.wantContinuity {
				t.Errorf("continuity section present = %v, want %v", got, tc.wantContinuity)
			}
			if strings.Contains(prompt, kindMarkerPrefix) {
				t.Errorf("a kind marker line reached the prompt")
			}
			// The common clauses are quoted ONCE per prompt, whichever kit follows them.
			for _, once := range []string{"## C1. Home worktree — the isolation floor", "KUBECONFIG=/dev/null", "## C7. "} {
				if n := strings.Count(prompt, once); n != 1 {
					t.Errorf("%q appears %d times in the prompt, want exactly once", once, n)
				}
			}
			// Both kinds carry the gathering, packet and waiting clauses.
			for _, want := range []string{
				"Gather in few requests — read whole, read together",
				"Packet first — only when the assignment names one",
				"Wait in one bounded command — never a look per request",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("the %s kit lacks the clause %q", name, want)
				}
			}
		})
	}
}

// THE SAFETY PROPERTY: a cut never loses a line that is not inside a stretch marked for the
// OTHER kind (or inside the file's common-clause copy). Checked line by line against the
// file as written, for every worker kit and every kind — so marking a stretch is the only
// way text stops reaching a kind, and that stretch is visible in the file.
func TestKindCutKeepsEveryUnmarkedLine(t *testing.T) {
	for _, kit := range []string{"worker", "worker-objective"} {
		raw, err := kitText(kit)
		if err != nil {
			t.Fatal(err)
		}
		for _, kind := range workerKinds() {
			cut, err := workerKitForKind(raw, kind)
			if err != nil {
				t.Fatalf("kit %q cannot be cut for %s: %v", kit, kind, err)
			}
			emitted := map[string]bool{}
			for _, l := range strings.Split(cut, "\n") {
				emitted[l] = true
			}
			other, inCopy := false, false
			for i, l := range strings.Split(raw, "\n") {
				switch {
				case strings.TrimSpace(l) == commonCopyBegin:
					inCopy = true
				case strings.TrimSpace(l) == commonCopyEnd:
					inCopy = false
				case kindMarkerRE.MatchString(l):
					m := kindMarkerRE.FindStringSubmatch(l)
					other = m[2] == "begin" && m[1] != string(kind)
				case inCopy || other || strings.TrimSpace(l) == "":
				default:
					if !emitted[l] {
						t.Errorf("kit %q cut for %s lost unmarked line %d: %q", kit, kind, i+1, l)
					}
				}
			}
		}
	}
}

// The stretches in the shipped kits are exactly the ones this change reasons about. A new
// stretch is a new claim that one kind is never bound by some text: it must be added here,
// in the same change, by someone who made that claim on purpose.
func TestShippedKindStretchesAreTheDeclaredOnes(t *testing.T) {
	want := map[string][]string{
		"worker":           {"implementing"},
		"worker-objective": {"shepherding", "implementing"},
	}
	for kit, kinds := range want {
		raw, err := kitText(kit)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, l := range strings.Split(raw, "\n") {
			if m := kindMarkerRE.FindStringSubmatch(l); m != nil && m[2] == "begin" {
				got = append(got, m[1])
			}
		}
		if strings.Join(got, ",") != strings.Join(kinds, ",") {
			t.Errorf("kit %q marks stretches %v, want exactly %v — a stretch withholds text from a kind", kit, got, kinds)
		}
	}
	// No other kit is cut by kind: a marker there would be quoted to an agent as text.
	for _, kit := range kitNames() {
		if workerKit(kit) {
			continue
		}
		raw, err := kitText(kit)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(raw, kindMarkerPrefix) {
			t.Errorf("kit %q carries a kind marker, but only the worker kits are cut by kind", kit)
		}
	}
	common, err := commonKitText()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(common, kindMarkerPrefix) {
		t.Error("the common clauses carry a kind marker — they reach every class whole")
	}
}

func TestWorkerKitForKindCuts(t *testing.T) {
	kit := strings.Join([]string{
		"# kit",
		"",
		"both one",
		"",
		"<!-- kind:implementing:begin -->",
		"implementing only",
		"",
		"<!-- kind:implementing:end -->",
		"<!-- kind:shepherding:begin -->",
		"shepherding only",
		"",
		"<!-- kind:shepherding:end -->",
		"both two",
		"",
		"<!-- common-clauses:begin -->",
		"the copy",
		"<!-- common-clauses:end -->",
		"",
	}, "\n")
	for kind, want := range map[workerKind]string{
		kindImplementing: "# kit\n\nboth one\n\nimplementing only\n\nboth two\n",
		kindShepherding:  "# kit\n\nboth one\n\nshepherding only\n\nboth two\n",
	} {
		got, err := workerKitForKind(kit, kind)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if got != want {
			t.Errorf("cut for %s =\n%q\nwant\n%q", kind, got, want)
		}
	}
}

// A kit whose boundaries cannot be read is refused, never guessed at.
func TestWorkerKitForKindRefusesUnreadableMarkers(t *testing.T) {
	for name, kit := range map[string]string{
		"unknown kind":     "a\n<!-- kind:reviewing:begin -->\nb\n<!-- kind:reviewing:end -->\n",
		"not a whole line": "a\ntext <!-- kind:implementing:begin -->\nb\n<!-- kind:implementing:end -->\n",
		"bad edge":         "a\n<!-- kind:implementing:start -->\nb\n",
		"nested":           "<!-- kind:implementing:begin -->\n<!-- kind:shepherding:begin -->\nb\n<!-- kind:shepherding:end -->\n<!-- kind:implementing:end -->\n",
		// Closed in order, so only the refusal to open one stretch inside another catches it.
		"nested, inner closed": "<!-- kind:implementing:begin -->\na\n<!-- kind:shepherding:begin -->\nb\n<!-- kind:shepherding:end -->\nc\n",
		"stray end":            "a\n<!-- kind:implementing:end -->\n",
		"mismatched end":       "<!-- kind:implementing:begin -->\nb\n<!-- kind:shepherding:end -->\n",
		"unclosed":             "a\n<!-- kind:shepherding:begin -->\nb\n",
		"unclosed copy":        "a\n<!-- common-clauses:begin -->\nb\n",
		"stray copy end":       "a\n<!-- common-clauses:end -->\n",
		"second copy":          "<!-- common-clauses:begin -->\nb\n<!-- common-clauses:end -->\n<!-- common-clauses:begin -->\nb\n<!-- common-clauses:end -->\n",
		"marker inside copy":   "<!-- common-clauses:begin -->\n<!-- kind:implementing:begin -->\n<!-- kind:implementing:end -->\n<!-- common-clauses:end -->\n",
		"copy inside stretch":  "<!-- kind:implementing:begin -->\n<!-- common-clauses:begin -->\nb\n<!-- common-clauses:end -->\n<!-- kind:implementing:end -->\n",
		"empty after the cut":  "<!-- kind:implementing:begin -->\nb\n<!-- kind:implementing:end -->\n",
	} {
		kind := kindImplementing
		if name == "empty after the cut" {
			kind = kindShepherding
		}
		got, err := workerKitForKind(kit, kind)
		if err == nil {
			t.Errorf("%s: the cut was accepted and emitted %q", name, got)
			continue
		}
		// The nested shapes are refused FOR being nested, not by a later check that
		// happens to trip on the same text.
		if strings.HasPrefix(name, "nested") && !strings.Contains(err.Error(), "opens shepherding inside the open implementing stretch") {
			t.Errorf("%s: refused for another reason: %v", name, err)
		}
	}
	if _, err := workerKitForKind("a\n", workerKind("reviewing")); err == nil {
		t.Error("a cut for a kind outside the closed set was accepted")
	}
}

// A kit other than the worker kits is quoted as written.
func TestOtherKitsAreQuotedAsWritten(t *testing.T) {
	for _, kit := range kitNames() {
		if workerKit(kit) {
			continue
		}
		raw, err := kitText(kit)
		if err != nil {
			t.Fatal(err)
		}
		got, err := workerKitText(dispatchOpts{kit: kit, pr: 42})
		if err != nil {
			t.Fatal(err)
		}
		if got != raw {
			t.Errorf("kit %q was altered on the way to the prompt", kit)
		}
	}
}

func TestWorkerKindIsDerivedFromTheDispatch(t *testing.T) {
	for _, tc := range []struct {
		o    dispatchOpts
		want workerKind
	}{
		{dispatchOpts{kit: "worker"}, kindImplementing},
		{dispatchOpts{kit: "worker-objective"}, kindImplementing},
		{dispatchOpts{kit: "worker", pr: 42}, kindShepherding},
		{dispatchOpts{kit: "worker-objective", pr: 42}, kindShepherding},
	} {
		if got := workerKindOf(tc.o); got != tc.want {
			t.Errorf("kind of %+v = %s, want %s", tc.o, got, tc.want)
		}
	}
}
