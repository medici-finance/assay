package main

import (
	"strings"
	"testing"
)

func TestAcceptanceLatestRunMustBeSelectedExactly(t *testing.T) {
	line := creditBaseAtt(41)
	selected := []acceptanceRun{{Run: fixtureRun, Source: fixtureSource}}
	if err := checkAcceptanceLatestRun(line, selected); err != nil {
		t.Fatal(err)
	}
	for name, latest := range map[string]string{
		"no attestation":   "PASS",
		"duplicates":       line + "\n" + line,
		"different source": strings.Replace(line, fixtureSource, strings.Repeat("c", 40), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkAcceptanceLatestRun(latest, selected); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}
