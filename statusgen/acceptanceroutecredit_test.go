package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreditAcceptanceRuns(t *testing.T) {
	creditFakeDeskread(t)
	t.Setenv("CREDIT_FAKE_CI", "1")
	line := creditBaseAtt(41)
	attestation, ok := parseContainerAttestation(line)
	if !ok {
		t.Fatal("test fixture has no attestation")
	}
	witness := landed(t, vfMailVerifier, line)
	selected := acceptanceRun{Run: attestation.run, Source: fixtureSource}
	second := acceptanceRun{Run: strings.Repeat("d", 64), Source: strings.Repeat("c", 40)}
	secondRecord := filepath.Join(os.Getenv("CREDIT_FAKE_DIR"), "issue-42.json")
	record := strings.ReplaceAll(getFile(t, secondRecord), fixtureSource, second.Source)
	putFile(t, secondRecord, strings.ReplaceAll(record, "claude-opus-4", "claude-sonnet-4"))
	labels := filepath.Join(os.Getenv("CREDIT_FAKE_DIR"), "label-events-42.json")
	putFile(t, labels, strings.ReplaceAll(getFile(t, labels), "claude-opus-4", "claude-sonnet-4"))
	secondLine := creditAtt(creditRepo, 42, second.Run, second.Source, creditBriefRel, "claude-sonnet-4", "strong")
	witness.commit(t, vfMailVerifier, getFile(t, witness.brief)+"\n"+creditBlock(t, secondLine, creditClosed))
	verify := "| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n"
	if err := creditAcceptanceRuns(witness.brief, verify, []acceptanceRun{selected, second}, true); err != nil {
		t.Fatalf("credit selected runs on different sources: %v", err)
	}

	for _, tc := range []struct {
		name string
		runs []acceptanceRun
		why  string
	}{
		{"empty selection", nil, "two independent verifier runs"},
		{"stale source", []acceptanceRun{{Run: selected.Run, Source: strings.Repeat("a", 40)}, second}, "do not match"},
		{"repeated run", []acceptanceRun{selected, selected}, "more than once"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := creditAcceptanceRuns(witness.brief, verify, tc.runs, true); err == nil || !strings.Contains(err.Error(), tc.why) {
				t.Fatalf("want refusal containing %q, got %v", tc.why, err)
			}
		})
	}
	stale := strings.Replace(verify, "`true`", "`false`", 1)
	if err := creditAcceptanceRuns(witness.brief, stale, []acceptanceRun{selected, second}, true); err == nil || !strings.Contains(err.Error(), "original row 1") {
		t.Fatalf("changed original row must refuse, got %v", err)
	}
	t.Setenv("CREDIT_FAKE_FAIL", "label-events")
	if err := creditAcceptanceRuns(witness.brief, verify, []acceptanceRun{selected, second}, true); err == nil || !strings.Contains(err.Error(), "could-not-check") {
		t.Fatalf("missing label history must fail closed, got %v", err)
	}
}
