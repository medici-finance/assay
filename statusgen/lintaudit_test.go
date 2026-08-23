package main

import (
	"strings"
	"testing"
)

func TestRuleTagFor(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"PROBLEM: /path/brief.md: Verify row 7 [gnu-only] uses process substitution", "gnu-only"},
		{"NOTICE: some check [budget]: over words", "budget"},
		{"PROBLEM: no bracket tag here at all", "unattributed:no-bracket-tag-here-at-all"},
	}
	for _, c := range cases {
		if got := ruleTagFor(c.line); got != c.want {
			t.Errorf("ruleTagFor(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestLintAuditTalliesPerRuleAndFlagsCold(t *testing.T) {
	var runnerDirs []string
	cfg := lintAuditConfig{
		root: "/fixture",
		logSampler: func(root string) ([]auditSample, error) {
			return []auditSample{
				{sha: "a1", date: "2026-07-25"},
				{sha: "a2", date: "2026-07-26"},
				{sha: "a3", date: "2026-07-27"},
			}, nil
		},
		worktreeFn: func(root, sha string) (string, func(), error) {
			return "tree-" + sha, func() {}, nil
		},
		lintRunner: func(treeDir, executable string) []string {
			runnerDirs = append(runnerDirs, treeDir)
			switch treeDir {
			case "/fixture": // HEAD seed run: registers a rule that never fires in history
				return []string{"PROBLEM: x [cold-rule]: never fires"}
			case "tree-a1":
				return []string{
					"PROBLEM: /x/brief.md: [link-refs]: broken",
					"NOTICE: [budget]: over words",
				}
			case "tree-a2":
				return []string{"PROBLEM: [budget]: over words again"}
			default:
				return nil
			}
		},
		testGrepper: func(pkgDir, rule string) bool { return rule == "link-refs" },
	}

	out := lintAuditReport(cfg)

	// Every sample tree plus the HEAD seed must have been linted.
	if len(runnerDirs) != 4 {
		t.Fatalf("expected 4 lint runs (3 samples + HEAD seed), got %d: %v", len(runnerDirs), runnerDirs)
	}

	for _, want := range []string{
		"rule | 30-day firings | gates-a-test?",
		"budget | 2",
		"link-refs | 1",
		"cold-rule | 0",
		"COLD — retirement candidate",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\nreport:\n%s", want, out)
		}
	}

	// Ascending order by firings: cold-rule(0) before link-refs(1) before budget(2).
	ci := strings.Index(out, "cold-rule")
	li := strings.Index(out, "link-refs")
	bi := strings.Index(out, "budget")
	if !(ci < li && li < bi) {
		t.Errorf("rows not sorted ascending by firings: cold-rule@%d link-refs@%d budget@%d\n%s", ci, li, bi, out)
	}

	// link-refs gates a test and must NOT be flagged COLD; budget fired and must not be.
	coldRow := out[strings.Index(out, "cold-rule"):]
	if strings.Count(coldRow, "COLD — retirement candidate") < 1 {
		t.Errorf("cold-rule row not flagged COLD:\n%s", out)
	}
	linkRow := out[strings.Index(out, "link-refs"):strings.Index(out, "budget")]
	if strings.Contains(linkRow, "COLD — retirement candidate") {
		t.Errorf("link-refs gates a test but is flagged COLD:\n%s", out)
	}
}

func TestLintAuditEmptyWindowReportsNoSamples(t *testing.T) {
	cfg := lintAuditConfig{
		root: "/fixture",
		logSampler: func(root string) ([]auditSample, error) {
			return nil, nil
		},
		worktreeFn: func(root, sha string) (string, func(), error) { return "", func() {}, nil },
		lintRunner: func(treeDir, executable string) []string {
			return []string{"PROBLEM: [some-rule]: present today"}
		},
		testGrepper: func(pkgDir, rule string) bool { return false },
	}
	out := lintAuditReport(cfg)
	if !strings.Contains(out, "no sampled commits") {
		t.Errorf("empty window should report no sampled commits:\n%s", out)
	}
}
