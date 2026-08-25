package main

import (
	"fmt"
	"testing"
)

// TestShardSelectPartitions is the coverage guarantee the -shard flag rests on:
// for ANY shard count n and ANY spec size, the n shards are pairwise DISJOINT
// and their union is EXACTLY the spec, in spec order. A CI matrix that runs all
// n shards has therefore run every mutation exactly once — none dropped, none
// duplicated. If this test fails, sharded sweeps cannot be trusted to cover the
// spec and the flag must not be used.
func TestShardSelectPartitions(t *testing.T) {
	for _, size := range []int{0, 1, 2, 3, 5, 8, 23, 24, 25, 100} {
		ms := make([]Mutation, size)
		for i := range ms {
			ms[i] = Mutation{Name: fmt.Sprintf("m%03d", i)}
		}
		for _, n := range []int{1, 2, 3, 4, 5, 7, 24, 30} {
			t.Run(fmt.Sprintf("size=%d/n=%d", size, n), func(t *testing.T) {
				seen := make(map[string]int, size) // name -> how many shards claimed it
				total := 0
				for i := 0; i < n; i++ {
					got := shardSelect(ms, i, n)
					total += len(got)
					prev := -1
					for _, m := range got {
						seen[m.Name]++
						if seen[m.Name] > 1 {
							t.Fatalf("mutation %q selected by more than one shard — shards are not disjoint", m.Name)
						}
						// Spec order must survive within a shard: verdict rows
						// keep their spec-relative ordering.
						var idx int
						if _, err := fmt.Sscanf(m.Name, "m%03d", &idx); err != nil {
							t.Fatalf("unparseable test name %q: %v", m.Name, err)
						}
						if idx <= prev {
							t.Fatalf("shard %d/%d out of spec order: index %d after %d", i, n, idx, prev)
						}
						prev = idx
					}
				}
				if total != size {
					t.Fatalf("shards cover %d mutations, spec has %d — union is not the whole spec", total, size)
				}
				for _, m := range ms {
					if seen[m.Name] != 1 {
						t.Fatalf("mutation %q selected %d times across all shards; want exactly 1", m.Name, seen[m.Name])
					}
				}
			})
		}
	}
}

// TestShardSelectUnshardedIdentity: 1 shard of 1 IS the full spec — the flagless
// and "-shard 0/1" invocations are the same sweep.
func TestShardSelectUnshardedIdentity(t *testing.T) {
	ms := []Mutation{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	got := shardSelect(ms, 0, 1)
	if len(got) != len(ms) {
		t.Fatalf("shard 0/1 selected %d of %d mutations; want all", len(got), len(ms))
	}
	for i := range ms {
		if got[i].Name != ms[i].Name {
			t.Fatalf("shard 0/1 reordered: got %q at %d, want %q", got[i].Name, i, ms[i].Name)
		}
	}
}

// TestParseShard: the accepted grammar is exactly "i/n" with 0 <= i < n and
// n >= 1. Everything else must be a refusal — a malformed -shard silently
// running everything (or nothing) would corrupt a sharded sweep's coverage
// claim.
func TestParseShard(t *testing.T) {
	valid := []struct {
		in   string
		i, n int
	}{
		{"0/1", 0, 1},
		{"0/3", 0, 3},
		{"1/3", 1, 3},
		{"2/3", 2, 3},
		{"9/10", 9, 10},
	}
	for _, tc := range valid {
		i, n, err := parseShard(tc.in)
		if err != nil {
			t.Errorf("parseShard(%q): unexpected error %v", tc.in, err)
			continue
		}
		if i != tc.i || n != tc.n {
			t.Errorf("parseShard(%q) = (%d,%d); want (%d,%d)", tc.in, i, n, tc.i, tc.n)
		}
	}

	invalid := []string{
		"",      // empty
		"3",     // no count
		"3/",    // empty count
		"/3",    // empty index
		"1/2/3", // too many parts
		"3/3",   // index == count (0-based!)
		"4/3",   // index > count
		"-1/3",  // negative index
		"0/0",   // zero shards
		"0/-1",  // negative count
		"a/3",   // non-integer index
		"0/b",   // non-integer count
		"0 / 3", // whitespace is not part of the grammar
	}
	for _, in := range invalid {
		if _, _, err := parseShard(in); err == nil {
			t.Errorf("parseShard(%q): want error, got nil", in)
		}
	}
}
