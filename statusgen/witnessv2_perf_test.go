//go:build witnessperf

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestWitnessPerfMedian qualifies the binary being timed, rather than using
// the presence of a test in its source as evidence of v2 support. SG_NEW_BIN
// lets an immutable copy of this harness exercise a historical candidate too.
// The original five-sample, plain-output corpus budget is retained. The v2
// fixture samples separately show that actual successful v2 evaluation ran;
// they are not a substitute corpus or a relaxation of that budget.
func TestWitnessPerfMedian(t *testing.T) {
	old := oldStatusgenBin(t)
	candidate := os.Getenv("SG_NEW_BIN")
	if candidate == "" {
		t.Fatal("SG_NEW_BIN must name the explicit tree-under-test binary")
	}
	root := os.Getenv("SG_PERF_ROOT")
	if root == "" || !filepath.IsAbs(root) {
		t.Fatal("SG_PERF_ROOT must name the absolute corpus path under test")
	}
	f := newGitV2Fixture(t)
	readme, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(witnessRegisterREADME)))
	if err != nil {
		t.Fatal(err)
	}
	f.ExtraFiles = map[string]string{witnessRegisterREADME: string(readme)}
	f.build()

	// Both binaries must parse this same nonempty tree. The baseline must
	// refuse its v2 claims; the candidate must actually release them.
	baselineRows := coverageVia(t, old, f.Root)
	candidateRows := coverageVia(t, candidate, f.Root)
	baseCount, newCount, schema2 := 0, 0, 0
	for _, b := range baselineRows {
		if b.Brief != gv2BriefID {
			continue
		}
		for _, c := range b.Claims {
			baseCount++
			if c.Released {
				t.Fatalf("baseline unexpectedly releases v2 claim %q", c.Claim)
			}
		}
	}
	for _, b := range candidateRows {
		if b.Brief != gv2BriefID {
			continue
		}
		for _, c := range b.Claims {
			newCount++
			if !c.Released {
				t.Fatalf("v2 non-vacuity: candidate holds %q: %s", c.Claim, c.Reason)
			}
		}
	}
	for _, b := range f.cliCoverage(candidate) {
		if b.BriefID != gv2BriefID {
			continue
		}
		for _, c := range b.Claims {
			if c.Witness != nil && c.Witness.Schema == 2 && c.Released {
				schema2++
			}
		}
	}
	if baseCount < len(f.Rows) || newCount != baseCount || schema2 != len(f.Rows) {
		t.Fatalf("v2 non-vacuity: baseline claims=%d candidate claims=%d released schema2=%d; want the same nonempty claims and %d schema2 releases", baseCount, newCount, schema2, len(f.Rows))
	}

	// Time the same plain-output mode as the original row. Check every timed
	// v2 output, so an empty/held fast path cannot contribute an accepted sample.
	run := func(bin, tree string) (time.Duration, string) {
		t.Helper()
		deadline, ok := t.Deadline()
		if !ok {
			t.Fatal("performance measurement requires a bounded test timeout")
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline.Add(-time.Second))
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, "--root", tree, "--coverage")
		start := time.Now()
		out, err := cmd.Output()
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("coverage sample failed: %v\n%s", err, out)
		}
		return elapsed, strings.TrimSpace(string(out))
	}
	var v2, before, after []time.Duration
	for i := 0; i < 5; i++ {
		d, out := run(candidate, f.Root)
		if out != fmt.Sprintf("%s released %d", gv2BriefID, newCount) {
			t.Fatalf("timed v2 sample %d did not release every claim: %s", i+1, out)
		}
		v2 = append(v2, d)
	}
	for _, pair := range []struct {
		name string
		bin  string
		dst  *[]time.Duration
	}{{"base", old, &before}, {"new", candidate, &after}} {
		for i := 0; i < 5; i++ {
			d, _ := run(pair.bin, root)
			*pair.dst = append(*pair.dst, d)
			t.Logf("corpus %s sample=%d elapsed=%s", pair.name, i+1, d)
		}
	}
	for _, samples := range [][]time.Duration{v2, before, after} {
		slices.Sort(samples)
	}
	t.Logf("v2: claims=%d schema2=%d samples=5 median=%s", newCount, schema2, v2[2])
	t.Logf("corpus: samples=5 base=%s new=%s ratio=%.6f budget=1.10", before[2], after[2], float64(after[2])/float64(before[2]))
	if float64(after[2]) > 1.10*float64(before[2]) {
		t.Fatalf("corpus median exceeds the original 1.10x budget")
	}
}
