package main

// properties_test.go — the three properties monitor.go's old header said a Go port would lose, each
// as a NAMED test that drives the verb's real read path (deskkit.ForgeFor → the GitHub client →
// the httptest replay). They stand on their own, without the bash oracle, so a runner with no bash
// still proves them.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	repoT = "example-org/tracker"
	repoA = "example-other/agents"
)

func issues(nums ...int) []fixtureIssue {
	out := make([]fixtureIssue, 0, len(nums))
	for _, n := range nums {
		out = append(out, fixtureIssue{Number: n, UpdatedAt: "2026-09-01T00:00:00Z"})
	}
	return out
}

// pollInbound runs one `deskmonitor inbound` cycle in-process.
func pollInbound(t *testing.T, fake *forgeFake, c fixtureCycle, args ...string) (string, int) {
	t.Helper()
	fake.setCycle(c)
	var out, errb bytes.Buffer
	code := run(append([]string{"inbound"}, args...), &out, &errb)
	if code == 1 {
		t.Fatalf("precondition failure: %s", errb.String())
	}
	return out.String(), code
}

// TestMonitorDropsRoleToken — property A, the negative path. The launching shell exported a role
// token (GH_TOKEN=leak, and GITHUB_TOKEN too). The verb must:
//   - read an owner that was handed --token-file as THAT file's token;
//   - read an owner with no token file as this session's minted App token;
//   - read an owner with no token file and NO resolvable session role not at all (a loud
//     MONITOR-DEGRADED), never as the inherited token;
//   - leave neither variable in its own environment, so no child it starts (the token minter)
//     inherits it.
//
// Every Authorization header the forge saw is checked: none may carry the inherited value.
func TestMonitorDropsRoleToken(t *testing.T) {
	const leak = "leak"
	if v := os.Getenv("GH_TOKEN"); v != "" && v != leak {
		t.Logf("GH_TOKEN was %q in the test environment; this test sets its own", v)
	}
	t.Setenv("GH_TOKEN", leak)
	t.Setenv("GITHUB_TOKEN", leak)
	t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
	t.Setenv("INBOUND_MONITOR_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	plantRoster(t, []string{repoT, repoA})
	fake := newForgeFake(t)
	tf := ownerTokenFile(t, "x-file-token")
	cycle := fixtureCycle{Reads: map[string]fixtureRead{repoT: {Issues: issues(1)}, repoA: {Issues: issues(2)}}}

	t.Run("token file for one owner, minted session token for the other", func(t *testing.T) {
		stubSessionIdentity(t, "worker", "x-minted-token", nil)
		fake.auths = nil
		out, code := pollInbound(t, fake, cycle, "--token-file", "example-org="+tf, repoT, repoA)
		if code != 0 || !strings.Contains(out, "MONITOR-ARMED: 2") {
			t.Fatalf("want a clean arm of both repos, got exit %d:\n%s", code, out)
		}
		assertAuths(t, fake.auths, leak, []string{"x-file-token", "x-minted-token"})
		// Checked here, straight after the verb ran and before any cleanup restores the test's
		// own environment: the verb dropped both from its process.
		for _, v := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
			if got, set := os.LookupEnv(v); set {
				t.Fatalf("%s is still set in the verb's environment (%q) — a child it starts would inherit it", v, got)
			}
		}
	})

	t.Run("no token file and no session role: degraded, never the inherited token", func(t *testing.T) {
		t.Setenv("GH_TOKEN", leak)
		t.Setenv("GITHUB_TOKEN", leak)
		t.Setenv("INBOUND_MONITOR_STATE_DIR", filepath.Join(t.TempDir(), "state"))
		stubSessionIdentity(t, "", "", errors.New("DESK_LOOP is unset"))
		fake.auths = nil
		out, code := pollInbound(t, fake, cycle, repoT, repoA)
		if code != 2 {
			t.Fatalf("want exit 2 (both repos DEGRADED), got %d:\n%s", code, out)
		}
		for _, r := range []string{repoT, repoA} {
			if !strings.Contains(out, "MONITOR-DEGRADED: "+r+" seed read FAILED") {
				t.Fatalf("%s was not reported DEGRADED:\n%s", r, out)
			}
		}
		if len(fake.auths) != 0 {
			t.Fatalf("with no identity to read as, the verb still reached the forge %d time(s) — as %q", len(fake.auths), fake.auths)
		}
	})

	t.Run("the pr poller never reads as the inherited token either", func(t *testing.T) {
		t.Setenv("GH_TOKEN", leak)
		t.Setenv("GITHUB_TOKEN", leak)
		t.Setenv("PR_MONITOR_STATE_DIR", filepath.Join(t.TempDir(), "pr-state"))
		stubSessionIdentity(t, "reviewer", "x-reviewer-token", nil)
		fake.auths = nil
		fake.setCycle(fixtureCycle{Reads: map[string]fixtureRead{repoT: {PRs: []fixturePR{{Number: 3, HeadRefOid: "abc", MergeStateStatus: "CLEAN"}}}}})
		var out, errb bytes.Buffer
		if code := run([]string{"pr", repoT}, &out, &errb); code != 0 {
			t.Fatalf("pr: exit %d: %s%s", code, out.String(), errb.String())
		}
		assertAuths(t, fake.auths, leak, []string{"x-reviewer-token"})
	})
}

// assertAuths: no header carries leak, and every token in want was used at least once.
func assertAuths(t *testing.T, auths []string, leak string, want []string) {
	t.Helper()
	if len(auths) == 0 {
		t.Fatal("the forge saw no request at all — nothing was proved")
	}
	for _, a := range auths {
		if strings.Contains(a, leak) {
			t.Fatalf("a read carried the INHERITED credential: Authorization %q", a)
		}
	}
	for _, w := range want {
		found := false
		for _, a := range auths {
			if strings.HasSuffix(a, " "+w) {
				found = true
			}
		}
		if !found {
			t.Fatalf("no read used the expected identity %q; saw %q", w, auths)
		}
	}
}

// TestMonitorRetainsOnFailedRead — property B. Each of the four untrusted reads (a failed read, a
// zero-after-nonzero read, an at-limit read, a read collapsed below the retain floor) must keep the
// repo's baseline byte-for-byte and print MONITOR-DEGRADED naming the repo, and the recovery cycle
// must diff against that retained baseline: exactly the genuinely new issue, zero phantoms.
func TestMonitorRetainsOnFailedRead(t *testing.T) {
	cases := []struct {
		name string
		bad  fixtureRead
		env  map[string]string
		want string
	}{
		{"404 read", fixtureRead{Status: 404, Message: "Not Found"}, nil, "read FAILED"},
		{"zero after nonzero", fixtureRead{Issues: []fixtureIssue{}}, nil, "returned 0 (had 6)"},
		{"at the limit", fixtureRead{Issues: issues(1, 2, 3, 4, 5, 6, 7, 8)}, map[string]string{"INBOUND_MONITOR_LIMIT": "8"}, "results TRUNCATED"},
		{"collapsed below the floor", fixtureRead{Issues: issues(1, 2)}, nil, "treating as a partial read"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := filepath.Join(t.TempDir(), "state")
			t.Setenv("INBOUND_MONITOR_STATE_DIR", stateDir)
			t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			plantRoster(t, []string{repoT})
			stubSessionIdentity(t, "worker", "x-minted-token", nil)
			fake := newForgeFake(t)
			six := fixtureRead{Issues: issues(1, 2, 3, 4, 5, 6)}

			if out, code := pollInbound(t, fake, fixtureCycle{Reads: map[string]fixtureRead{repoT: six}}, repoT); code != 0 || out != "MONITOR-ARMED: 6\n" {
				t.Fatalf("seed: exit %d %q", code, out)
			}
			sf := filepath.Join(stateDir, "example-org__tracker.state")
			before, _ := os.ReadFile(sf)

			out, code := pollInbound(t, fake, fixtureCycle{Reads: map[string]fixtureRead{repoT: tc.bad}}, repoT)
			if code != 2 || !strings.HasPrefix(out, "MONITOR-DEGRADED: "+repoT+" ") || !strings.Contains(out, tc.want) {
				t.Fatalf("untrusted read: want exit 2 and a MONITOR-DEGRADED line containing %q, got exit %d:\n%s", tc.want, code, out)
			}
			if strings.Contains(out, "INBOUND:") {
				t.Fatalf("an untrusted read emitted INBOUND:\n%s", out)
			}
			after, _ := os.ReadFile(sf)
			if string(after) != string(before) {
				t.Fatalf("the baseline was NOT retained:\nbefore %q\nafter  %q", before, after)
			}

			out, code = pollInbound(t, fake, fixtureCycle{Reads: map[string]fixtureRead{repoT: {Issues: issues(1, 2, 3, 4, 5, 6, 7)}}}, repoT)
			if code != 0 || out != "INBOUND: example-org/tracker#7 2026-09-01T00:00:00Z\n" {
				t.Fatalf("recovery must report exactly the one new issue (the outage absorbed), got exit %d:\n%s", code, out)
			}
		})
	}
}

// TestMonitorBurstCollapse — property C. More new keys than the cap in one cycle for one repo
// collapse to ONE burst line naming the repo and the count; exactly the cap is listed; the baseline
// advances either way, so the burst is not replayed next cycle.
func TestMonitorBurstCollapse(t *testing.T) {
	t.Setenv("INBOUND_MONITOR_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	t.Setenv("INBOUND_MONITOR_BURST_CAP", "3")
	t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
	plantRoster(t, []string{repoT, repoA})
	stubSessionIdentity(t, "worker", "x-minted-token", nil)
	fake := newForgeFake(t)

	seed := fixtureCycle{Reads: map[string]fixtureRead{repoT: {Issues: issues(1)}, repoA: {Issues: issues(1)}}}
	if _, code := pollInbound(t, fake, seed, repoT, repoA); code != 0 {
		t.Fatalf("seed exit %d", code)
	}
	mass := fixtureCycle{Reads: map[string]fixtureRead{
		repoT: {Issues: issues(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12)}, // 11 new > cap 3
		repoA: {Issues: issues(1, 2, 3, 4)},                            // 3 new == cap 3
	}}
	out, code := pollInbound(t, fake, mass, repoT, repoA)
	want := "INBOUND-BURST: example-org/tracker 11 over 3 — listing suppressed\n" +
		"INBOUND: example-other/agents#2 2026-09-01T00:00:00Z\n" +
		"INBOUND: example-other/agents#3 2026-09-01T00:00:00Z\n" +
		"INBOUND: example-other/agents#4 2026-09-01T00:00:00Z\n"
	if code != 0 || out != want {
		t.Fatalf("burst cycle: exit %d\n got %q\nwant %q", code, out, want)
	}
	if out, code := pollInbound(t, fake, mass, repoT, repoA); code != 0 || out != "" {
		t.Fatalf("the cycle after a burst must be quiet (the baseline advanced), got exit %d %q", code, out)
	}
}

// TestMonitorTokenFilePreconditions — a --token-file the verb cannot use is a precondition failure
// (exit 1) BEFORE any read: it never swaps the identity it was told to use for another one.
func TestMonitorTokenFilePreconditions(t *testing.T) {
	t.Setenv("INBOUND_MONITOR_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	plantRoster(t, []string{repoT})
	stubSessionIdentity(t, "worker", "x-minted-token", nil)
	fake := newForgeFake(t)
	fake.setCycle(fixtureCycle{Reads: map[string]fixtureRead{repoT: {Issues: issues(1)}}})

	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	good := ownerTokenFile(t, "x-file-token")
	cases := map[string][]string{
		"missing file":      {"--token-file", "example-org=" + filepath.Join(dir, "absent")},
		"empty file":        {"--token-file", "example-org=" + empty},
		"a directory":       {"--token-file", "example-org=" + dir},
		"no OWNER=":         {"--token-file", good},
		"no value":          {"--token-file"},
		"owner given twice": {"--token-file", "example-org=" + good, "--token-file", "EXAMPLE-ORG=" + good},
	}
	if runtime.GOOS != "windows" { // mode bits are synthetic on Windows; the ACL rule is deskkit's own test
		loose := filepath.Join(dir, "loose")
		if err := os.WriteFile(loose, []byte("x-token\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cases["group/world readable"] = []string{"--token-file", "example-org=" + loose}
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			fake.auths = nil
			var out, errb bytes.Buffer
			code := run(append(append([]string{"inbound"}, args...), repoT), &out, &errb)
			if code != 1 {
				t.Fatalf("want exit 1 (precondition), got %d: %s%s", code, out.String(), errb.String())
			}
			if len(fake.auths) != 0 {
				t.Fatalf("a read was made despite an unusable --token-file")
			}
		})
	}
}
