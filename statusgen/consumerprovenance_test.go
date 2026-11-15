package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Invariant tests for the new internal observation stay separate from the
// cross-revision behavioral probe, which must compile on original source.
// regression: #5684
func TestV2ConsumerProvenance(t *testing.T) {
	f := v2ConsumerFixture(t, nil)
	c := f.coverage()
	if !c.Released || len(c.Claims) != 2 {
		t.Fatalf("non-vacuity: %+v", c)
	}
	for _, claim := range c.Claims {
		w := claim.Witness
		if w == nil || w.Brief != gv2BriefID || w.Landing != f.Merge || w.Landing == f.Base || w.RecordID != f.RecordIDs[w.Row] || w.CR != gv2CR || w.Runner != "verifier-app" || w.Date != gv2Date {
			t.Fatalf("unbound canonical witness provenance: %+v", w)
		}
		if claim.Revision != f.Base {
			t.Fatal("informational revision's existing wire meaning changed")
		}
	}
	wire, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), f.Merge) || strings.Contains(string(wire), "Landing") || strings.Contains(string(wire), "RecordID") {
		t.Fatalf("internal provenance escaped public JSON: %s", wire)
	}
	data, _, err := loadReconcileFoldData(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := latestCoveragePass(data[0].evidence, c)
	if err != nil {
		t.Fatal(err)
	}
	if p.RecordLanding != f.Merge || p.SHA != "" || p.Stamp != "" || p.StampErr == nil {
		t.Fatalf("record landing became an execution SHA or write stamp: %+v", p)
	}
	c.Claims[0].Witness.Landing = strings.Repeat("a", 40)
	fresh := f.coverage()
	if fresh.Claims[0].Witness.Landing != f.Merge {
		t.Fatal("caller mutation contaminated a later evaluation")
	}
	cases := []struct {
		name   string
		mutate func(*Coverage)
	}{
		{"missing-provenance", func(c *Coverage) { c.Claims[0].Witness = nil }},
		{"missing-landing", func(c *Coverage) { c.Claims[0].Witness.Landing = "" }},
		{"malformed-landing", func(c *Coverage) { c.Claims[0].Witness.Landing = "unverified" }},
		{"foreign-brief", func(c *Coverage) { c.Claims[0].Witness.Brief = "foreign/01" }},
		{"mixed-landing", func(c *Coverage) { c.Claims[0].Witness.Landing = f.Base }},
		{"mixed-runner", func(c *Coverage) { c.Claims[0].Witness.Runner = "worker-app" }},
		{"mixed-date", func(c *Coverage) { c.Claims[0].Witness.Date = "2026-10-24" }},
		{"mixed-change-request", func(c *Coverage) { c.Claims[0].Witness.CR = "8" }},
		{"mixed-record", func(c *Coverage) { c.Claims[0].Witness.RecordID = strings.Repeat("a", 64) }},
		{"duplicate-row", func(c *Coverage) { c.Claims[1].Witness.Row = c.Claims[0].Witness.Row }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := f.coverage()
			tc.mutate(&c)
			if _, err := latestCoveragePass(data[0].evidence, c); err == nil {
				t.Fatal("unbound or mixed provenance supplied a PASS")
			}
		})
	}
}

// regression: #5684
func TestV2LatestBadProvenance(t *testing.T) {
	f := v2ConsumerFixture(t, func(f *gitV2Fixture) {
		f.Wits = append(f.Wits, gv2Witness{Row: "1", Identity: "verifier-app", Tail: " (forge-identity)", RecordCommand: "false"})
	})
	claim := claimFor(t, f.coverage(), "1")
	if claim.Released || claim.Witness != nil {
		t.Fatalf("old good record supplied provenance for a bad latest witness: %+v", claim)
	}
}

// regression: #5684
func TestV2ConsumerActorRequired(t *testing.T) {
	f := v2ConsumerFixture(t, nil)
	b, err := os.ReadFile(f.BriefPath)
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.Replace(string(b), "consumer flow fixture.", "worker-authored PASS.", 1))
	if err := os.WriteFile(f.BriefPath, b, 0644); err != nil {
		t.Fatal(err)
	}
	gv2GitAs(t, f.Root, fixtureWorkerName, fixtureWorkerEmail, "add", "-A")
	gv2GitAs(t, f.Root, fixtureWorkerName, fixtureWorkerEmail, "commit", "-q", "-m", "fixture: untrusted PASS author")
	if c := f.coverage(); !c.Released {
		t.Fatalf("test must reach the separate actor guard: %+v", c)
	}
	data, streams, err := loadReconcileFoldData(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	tree := treeFoldInputs(f.Root, streams, data)
	if tree.witnesses[gv2BriefID].Passed || tree.witnesses[gv2BriefID].Released {
		t.Fatal("canonical content provenance bypassed Evidence actor authority")
	}
}
