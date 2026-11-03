package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pre-flip check and the lint invoked by deskevidence flip must apply the
// same fail-first policy, not independently maintained approximations.
func TestBriefInfoFailFirstMatchesFlipLint(t *testing.T) {
	for _, tc := range []struct {
		name, kind, want string
		wide, plain      bool
	}{
		{name: "missing Base", kind: "missing", want: "no fail-first witness"},
		{name: "not selected", kind: "not-selected", want: "no fail-first witness"},
		{name: "green base", kind: "green", want: "non-discriminating"},
		{name: "unproven base", kind: "unproven", want: "could not run at base"},
		{name: "head as base", kind: "head", want: "not the merge-base or an ancestor"},
		{name: "stray base", kind: "stray", want: "not the merge-base or an ancestor"},
		{name: "red base", kind: "red"},
		{name: "older base holding brief", kind: "older"},
		{name: "ordinary rerun replaces fail-first", kind: "rerun", want: "no fail-first witness"},
		{name: "brief-wide risk", kind: "missing", wide: true, want: "no fail-first witness"},
		{name: "non-risk ordinary witness", kind: "missing", plain: true},
		{name: "non-risk green base", kind: "green", plain: true},
		{name: "unplaced Verify row", kind: "unplaced", want: "cannot place"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withBase(t, true)
			root, older, base, _, head, stray := ffRepo(t)
			cell := failFirstCell(base[:12], baseRed, 1, head[:12])
			switch tc.kind {
			case "missing":
				cell = ""
			case "not-selected":
				cell = failFirstCell("", baseNotSelected, -1, head[:12])
			case "green":
				cell = failFirstCell(base[:12], baseGreen, 0, head[:12])
			case "unproven":
				cell = failFirstCell(base[:12], baseUnproven, -1, head[:12])
			case "head":
				cell = failFirstCell(head[:12], baseRed, 1, head[:12])
			case "stray":
				cell = failFirstCell(stray[:12], baseRed, 1, head[:12])
			case "older":
				cell = failFirstCell(older[:12], baseRed, 1, head[:12])
			}
			evidence := ffTable(cell, cell)
			if tc.kind == "rerun" {
				evidence += "\n\n" + ffTable("", "")
			}
			brief := ffBrief(evidence)
			if tc.wide || tc.plain {
				brief = strings.ReplaceAll(brief, "(live check)", "")
			}
			if tc.wide {
				brief = strings.ReplaceAll(brief, "regulatory: no", "regulatory: yes")
			}
			if tc.kind == "unplaced" {
				brief = strings.Replace(brief, "## Evidence", "| # | Cmd | Expect |\n|---|---|---|\n| 3 | `true` | exit 0 |\n\n## Evidence", 1)
			}
			streams := ffStreamBrief(t, root, brief)
			stampFFBoard(t, root)
			problems, _ := failFirstGateChecks(root, streams)
			code, out, diag := runBriefInfoCapture("--root", root, "--check-verified", "ff/01")
			sideCode, sideOut, sideErr := captureVerifyclosure(t, []string{"--root", root, "--brief", "ff/01"})
			if sideCode != code || (tc.want != "" && !strings.Contains(sideOut, tc.want)) {
				t.Fatalf("closure adapters disagree: brief=%d %s; sidecar=%d %s %s", code, diag, sideCode, sideOut, sideErr)
			}
			if tc.want == "" {
				if len(problems) != 0 || code != 0 {
					t.Fatalf("lint=%v; brief exit=%d: %s", problems, code, diag)
				}
			} else if !containsSub(problems, tc.want) || code != 1 || out != "" || !strings.Contains(diag, tc.want) {
				t.Fatalf("want both paths to refuse %q; lint=%v; brief exit=%d out=%q: %s", tc.want, problems, code, out, diag)
			}
		})
	}
}

func stampFFBoard(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "docs", "streams", "ff", "README.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(raw), "| 2026-10-08 |", "| 2026-10-08 human:fixture |")), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestBriefInfoFailFirstDoesNotGrandfatherSelectedClosure(t *testing.T) {
	withBase(t, true, "ff/01")
	root := t.TempDir()
	streams := ffStream(t, root, ffTable("", ""))
	stampFFBoard(t, root)
	if problems, _ := failFirstGateChecks(root, streams); len(problems) != 0 {
		t.Fatalf("inherited lint closure changed: %v", problems)
	}
	code, out, diag := runBriefInfoCapture("--root", root, "--check-verified", "ff/01")
	if code != 1 || out != "" || !strings.Contains(diag, "no fail-first witness") {
		t.Fatalf("selected closure exit=%d out=%q: %s", code, out, diag)
	}
}
