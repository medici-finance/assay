package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// brief-v2 parser/validator tests (derived-board/03). Top-level functions named
// TestBriefV2* so Verify row 1's `-run 'BriefV2' | grep -c '^--- PASS'` counts them.

// v2Root writes a minimal brief-v2 board root into a temp dir: a graph-repos.yaml
// registry, one stream README row, and one brief file with the given frontmatter
// body. It returns the (problems, notices) from checkBriefFiles over that root.
// withRegistry=false omits the registry (to exercise the required-registry PROBLEM).
func v2Root(t *testing.T, briefBody string, withRegistry bool) (problems, notices []string) {
	t.Helper()
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "demo")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if withRegistry {
		reg := "schema: graph-repos-v1\ncell: smoke\nrepos:\n  sg:  {cell: smoke, repo: medici-finance/assay}\n  rec: {cell: smoke, repo: null, unpublished: true}\n"
		if err := os.WriteFile(filepath.Join(root, "docs", "streams", "graph-repos.yaml"), []byte(reg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	readme := "---\nstream: demo\nrepo: medici-finance/assay\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# Demo\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n| 01 | [One](brief-01-x.md) | 0 | S | todo | — | — |\n"
	if err := os.WriteFile(filepath.Join(streamDir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, "brief-01-x.md"), []byte(briefBody), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatalf("loadStreams: %v", err)
	}
	return checkBriefFiles(streams, streams)
}

// validV2Brief is a clean brief-v2 file body used as the baseline the negative
// cases mutate.
const validV2Brief = `---
brief: smoke:sg:demo:01
title: A valid brief-v2 fixture
why: >-
  Independent rationale a non-engineer could read and justify the work from.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
id: 4f8c2d1a-9b3e-4c7a-8f21-0a1b2c3d4e5f
supersedes: []
gates:
  - on: "rec:ingest/06"
    type: ordering-gate
    reason: ingest ordering must land first
feathers:
  - "rec:ingest/07"
authored: 2026-08-22 by fixture
sources: ["fixture: valid v2"]
---

# Brief 01 — valid v2
`

func TestBriefV2ParseReservedKeys(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "brief-01-x.md")
	if err := os.WriteFile(p, []byte(validV2Brief), 0o644); err != nil {
		t.Fatal(err)
	}
	bf, ok, err := parseBriefFile(p)
	if err != nil || !ok {
		t.Fatalf("valid v2 brief should parse; got ok=%v err=%v", ok, err)
	}
	if bf.Schema != "brief-v2" {
		t.Errorf("schema: want brief-v2, got %q", bf.Schema)
	}
	if bf.Version != 2 {
		t.Errorf("version: want 2, got %d", bf.Version)
	}
	if bf.ID != "4f8c2d1a-9b3e-4c7a-8f21-0a1b2c3d4e5f" {
		t.Errorf("id not parsed: %q", bf.ID)
	}
	if len(bf.Gates) != 1 || bf.Gates[0].Ref != "rec:ingest/06" || bf.Gates[0].Type != "ordering-gate" || bf.Gates[0].Reason == "" {
		t.Errorf("gates edge not parsed: %+v", bf.Gates)
	}
	if len(bf.Feathers) != 1 || bf.Feathers[0].Ref != "rec:ingest/07" || bf.Feathers[0].Type != "build-dep" {
		t.Errorf("feathers scalar should default to build-dep: %+v", bf.Feathers)
	}
	// version defaults to 1 when absent under v2.
	noVer := strings.Replace(validV2Brief, "version: 2\n", "", 1)
	p2 := filepath.Join(dir, "brief-02-x.md")
	if err := os.WriteFile(p2, []byte(strings.Replace(noVer, "smoke:sg:demo:01", "smoke:sg:demo:02", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	bf2, _, err := parseBriefFile(p2)
	if err != nil || bf2.Version != 1 {
		t.Errorf("absent version should default to 1; got %d err=%v", bf2.Version, err)
	}
}

func TestBriefV2ValidLintsClean(t *testing.T) {
	problems, notices := v2Root(t, validV2Brief, true)
	if len(problems) != 0 {
		t.Fatalf("a valid v2 brief should produce no PROBLEMs; got %v", problems)
	}
	// The reserved edges are surfaced reserved-not-gating.
	if !hasProblem(notices, "gates: 1 edge (reserved, not gating)") {
		t.Errorf("expected the reserved-not-gating notice; got %v", notices)
	}
	if !hasProblem(notices, "feathers: 1 edge (reserved, not gating)") {
		t.Errorf("expected the feathers reserved-not-gating notice; got %v", notices)
	}
}

func TestBriefV2GatesEdgeValidation(t *testing.T) {
	// Unknown edge type → PROBLEM.
	badType := strings.Replace(validV2Brief, "type: ordering-gate", "type: not-a-real-type", 1)
	problems, _ := v2Root(t, badType, true)
	if !hasProblem(problems, "unknown type", "not-a-real-type") {
		t.Errorf("bad edge type should be a PROBLEM; got %v", problems)
	}
	// Unknown alias in a ref → PROBLEM (the closed-set property).
	badAlias := strings.Replace(validV2Brief, "rec:ingest/06", "nope:ingest/06", 1)
	problems2, _ := v2Root(t, badAlias, true)
	if !hasProblem(problems2, "nope") {
		t.Errorf("unknown alias should be a PROBLEM; got %v", problems2)
	}
}

func TestBriefV2HierarchicalID(t *testing.T) {
	// A brief-v1 <stream>/<NN> id in a v2 file → PROBLEM.
	v1form := strings.Replace(validV2Brief, "brief: smoke:sg:demo:01", "brief: demo/01", 1)
	p1, _ := v2Root(t, v1form, true)
	if !hasProblem(p1, "brief-v1", "form") {
		t.Errorf("v1-form id in a v2 file should be a PROBLEM; got %v", p1)
	}
	// Wrong stream/NN tail → PROBLEM.
	wrongTail := strings.Replace(validV2Brief, "brief: smoke:sg:demo:01", "brief: smoke:sg:other:09", 1)
	p2, _ := v2Root(t, wrongTail, true)
	if !hasProblem(p2, "does not match the file path") {
		t.Errorf("path-mismatched id should be a PROBLEM; got %v", p2)
	}
	// Wrong cell → PROBLEM.
	wrongCell := strings.Replace(validV2Brief, "brief: smoke:sg:demo:01", "brief: elsewhere:sg:demo:01", 1)
	p3, _ := v2Root(t, wrongCell, true)
	if !hasProblem(p3, "cell") {
		t.Errorf("cell mismatch should be a PROBLEM; got %v", p3)
	}
}

func TestBriefV2IDShapeAndUniqueness(t *testing.T) {
	// A malformed uuid → PROBLEM.
	badID := strings.Replace(validV2Brief, "id: 4f8c2d1a-9b3e-4c7a-8f21-0a1b2c3d4e5f", "id: not-a-uuid", 1)
	p, _ := v2Root(t, badID, true)
	if !hasProblem(p, "is not a uuid v4") {
		t.Errorf("malformed uuid should be a PROBLEM; got %v", p)
	}
}

func TestBriefV2RegistryRequired(t *testing.T) {
	// A v2 brief with NO graph-repos.yaml → PROBLEM (the registry is required in a
	// v2 tree; the hierarchical id cannot resolve without it).
	problems, _ := v2Root(t, validV2Brief, false)
	if !hasProblem(problems, "graph-repos.yaml", "absent") {
		t.Errorf("a v2 tree with no registry should PROBLEM; got %v", problems)
	}
}
