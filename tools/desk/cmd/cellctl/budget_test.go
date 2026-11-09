package main

import (
	"slices"
	"strings"
	"testing"
)

// regression: F-muhar-budget
func TestMuharBudgetEnv(t *testing.T) {
	for _, configured := range []bool{false, true} {
		c := codexEnvironmentCell(t)
		if configured {
			c.Env.Put("MUHAR_LEASE_DIR", "/fixture/leases")
			c.Env.Put("MUHAR_TOTAL_SLOTS", "3")
		}
		want := []string(nil)
		if configured {
			want = []string{"MUHAR_LEASE_DIR=/fixture/leases", "MUHAR_TOTAL_SLOTS=3"}
		}
		env := c.muharBudgetEnv([]string{"MUHAR_LEASE_DIR=other-cell", "MUHAR_TOTAL_SLOTS=99"})
		for _, key := range []string{"MUHAR_LEASE_DIR", "MUHAR_TOTAL_SLOTS"} {
			if !knownCellEnvKey(key) {
				t.Errorf("unregistered %s", key)
			}
			if !configured && envValue(env, key) != "" {
				t.Errorf("host invented %s", key)
			}
		}
		values, err := c.codexCommandEnvironment(env)
		if err != nil {
			t.Fatal(err)
		}
		for _, kv := range want {
			k, v, _ := strings.Cut(kv, "=")
			if !slices.Contains(env, kv) || values[k] != v {
				t.Errorf("host/Codex omitted %s", k)
			}
		}
		s := scrubbedFixture(t)
		if configured {
			s.Env.Put("MUHAR_LEASE_DIR", "/fixture/leases")
			s.Env.Put("MUHAR_TOTAL_SLOTS", "3")
		}
		for _, h := range []string{"codex", "claude"} {
			got := s.scrubbedComposeEnv("worker-desk", h, "fixture").Pairs
			for _, kv := range want {
				if !slices.Contains(got, kv) {
					t.Errorf("scrubbed omitted %s", kv)
				}
			}
			if !configured && (envValue(got, "MUHAR_LEASE_DIR") != "" || envValue(got, "MUHAR_TOTAL_SLOTS") != "") {
				t.Fatal("scrubbed invented budget")
			}
		}
	}
}
