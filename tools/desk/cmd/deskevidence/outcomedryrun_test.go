package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// countBudget replaces the write-budget spend with a counter, so a test can assert that a
// dry-run never spends a slot.
func countBudget(t *testing.T) *int {
	t.Helper()
	n := 0
	old := evidenceWriteBudget
	evidenceWriteBudget = func(string, string, int) error { n++; return nil }
	t.Cleanup(func() { evidenceWriteBudget = old })
	return &n
}

// assertNoWrite fails unless the run reached no write and spent no budget.
func assertNoWrite(t *testing.T, f *fakeForge, budget int) {
	t.Helper()
	if f.putCalls != 0 || len(f.writes) != 0 || budget != 0 {
		t.Fatalf("dry-run wrote: %d WriteFile call(s), %d write(s), %d budget spend(s); want 0/0/0",
			f.putCalls, len(f.writes), budget)
	}
	for _, h := range f.hits {
		if strings.HasPrefix(h, "PUT ") {
			t.Fatalf("dry-run reached the commits API: %v", f.hits)
		}
	}
}

// dryRunFixtures is one passing landing per route that honours --dry-run. Each returns the
// argument list for run(); the landing would succeed without --dry-run.
var dryRunFixtures = map[string]func(t *testing.T, f *fakeForge) []string{
	routeEvidenceFile: func(t *testing.T, f *fakeForge) []string {
		p := "docs/streams/x/brief.md"
		root := rootWithFile(t, p, "# Brief\n\n## Evidence\n| 1 | ... | evidence row |\n")
		f.setFile(p, "# Brief\n\n## Evidence\n")
		return []string{fixtureRepo, "main", "--evidence-file", p, "--root", root}
	},
	routeOutcomeRecord: func(t *testing.T, f *fakeForge) []string {
		line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
		return []string{fixtureRepo, "main", "--outcome-record", writeRepoFile(t, "record.json", line+"\n")}
	},
}

// regression: #4554
//
// TestDryRunEveryRoute: every route that declares --dry-run honours it — a passing landing
// run with --dry-run prints a plan and makes no write and no budget spend. A route that
// declares the flag with no fixture here fails, so a new route cannot claim dry-run untested.
func TestDryRunEveryRoute(t *testing.T) {
	for route, flags := range routeHonours {
		declares := false
		for _, fl := range flags {
			declares = declares || fl == "dry-run"
		}
		if !declares {
			continue
		}
		t.Run(route, func(t *testing.T) {
			fixture, ok := dryRunFixtures[route]
			if !ok {
				t.Fatalf("route %s declares --dry-run but has no dry-run fixture", route)
			}
			f, errBuf := setupFake(t)
			budget := countBudget(t)
			args := fixture(t, f)
			code := run(append(args, "--dry-run"))
			if code != deskkit.ExitOK {
				t.Fatalf("%s --dry-run: exit = %d, want 0; stderr %q", route, code, errBuf.String())
			}
			assertNoWrite(t, f, *budget)
			if out := stdout.(*bytes.Buffer).String(); !strings.Contains(out, "dry-run plan") {
				t.Fatalf("%s --dry-run printed no plan:\n%s", route, out)
			}
		})
	}
}

// regression: #4554
//
// TestOutcomeDryRunNoWrite: --dry-run with --outcome-record, in ordinary and --outcome-only
// mode, runs the gates, names the target, and makes no commits-API call and no budget spend.
func TestOutcomeDryRunNoWrite(t *testing.T) {
	t.Run("ordinary", func(t *testing.T) {
		f, errBuf := setupFake(t)
		budget := countBudget(t)
		line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
		want, err := deskkit.RecordName([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		code := run([]string{fixtureRepo, "main", "--outcome-record", writeRepoFile(t, "record.json", line+"\n"), "--dry-run"})
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0; stderr %q", code, errBuf.String())
		}
		assertNoWrite(t, f, *budget)
		if len(f.reads) == 0 {
			t.Fatal("dry-run skipped the immutability read; it must run every gate")
		}
		out := stdout.(*bytes.Buffer).String()
		if !strings.Contains(out, "dry-run plan") || !strings.Contains(out, want) {
			t.Fatalf("plan does not name the target %s:\n%s", want, out)
		}
	})
	t.Run("outcome-only", func(t *testing.T) {
		f, errBuf := setupFake(t)
		realVerdictCheck(t)
		budget := countBudget(t)
		root, bound := ooBoard(t, "verified")
		code := ooRun(t, root, ooRecord(t, "blocked", vgSource, evidenceWithheldReason), true, "--dry-run")
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0; stderr %q", code, errBuf.String())
		}
		assertNoWrite(t, f, *budget)
		if *bound != 0 {
			t.Fatalf("outcome-only dry-run reached the Evidence binding %d time(s)", *bound)
		}
		if out := stdout.(*bytes.Buffer).String(); !strings.Contains(out, "outcome-only record") {
			t.Fatalf("plan does not name the outcome-only mode:\n%s", out)
		}
	})
}

// regression: #4554
//
// TestOutcomeDryRunGatesFire: --dry-run never relaxes a gate. Each case is refused under
// --dry-run exactly as without it, with no write and no budget spend.
func TestOutcomeDryRunGatesFire(t *testing.T) {
	t.Run("outcome-only on an implemented brief", func(t *testing.T) {
		f, errBuf := setupFake(t)
		realVerdictCheck(t)
		budget := countBudget(t)
		root, _ := ooBoard(t, "implemented")
		code := ooRun(t, root, ooRecord(t, "blocked", vgSource, evidenceWithheldReason), true, "--dry-run")
		if code != deskkit.ExitRefused || !strings.Contains(errBuf.String(), "land the Evidence first") {
			t.Fatalf("exit = %d stderr %q, want the outcome-only refusal", code, errBuf.String())
		}
		assertNoWrite(t, f, *budget)
	})
	t.Run("evidence_withheld without --outcome-only", func(t *testing.T) {
		f, errBuf := setupFake(t)
		budget := countBudget(t)
		root, _ := ooBoard(t, "verified")
		code := ooRun(t, root, ooRecord(t, "blocked", vgSource, evidenceWithheldReason), false, "--dry-run")
		if code != deskkit.ExitRefused || !strings.Contains(errBuf.String(), "--outcome-only") {
			t.Fatalf("exit = %d stderr %q, want the evidence_withheld refusal", code, errBuf.String())
		}
		assertNoWrite(t, f, *budget)
	})
	t.Run("Evidence binding refuses", func(t *testing.T) {
		f, _ := setupFake(t)
		budget := countBudget(t)
		root, bound := ooBoard(t, "verified")
		code := ooRun(t, root, ooRecord(t, "blocked", vgSource, ""), false, "--dry-run")
		if code != deskkit.ExitRefused || *bound != 1 {
			t.Fatalf("exit = %d binding = %d, want the Evidence binding to refuse", code, *bound)
		}
		assertNoWrite(t, f, *budget)
	})
	t.Run("immutable record", func(t *testing.T) {
		f, errBuf := setupFake(t)
		budget := countBudget(t)
		line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002","note":"a"}`)
		name, err := deskkit.RecordName([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		f.setFile(name, `{"other":"bytes"}`+"\n")
		code := run([]string{fixtureRepo, "main", "--outcome-record", writeRepoFile(t, "record.json", line+"\n"), "--dry-run"})
		if code != deskkit.ExitRefused || !strings.Contains(errBuf.String(), "immutable") {
			t.Fatalf("exit = %d stderr %q, want the immutability refusal", code, errBuf.String())
		}
		assertNoWrite(t, f, *budget)
	})
	t.Run("future ts", func(t *testing.T) {
		f, _ := setupFake(t)
		budget := countBudget(t)
		now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
		nowFn = func() time.Time { return now }
		t.Cleanup(func() { nowFn = time.Now })
		ts := now.Add(deskkit.MaxClockSkew + time.Minute).Format(time.RFC3339)
		line := `{"ts":"` + ts + `","brief":"example-stream/14","outcome":"verified","sha":"0000002"}`
		code := run([]string{fixtureRepo, "main", "--outcome-record", writeRepoFile(t, "record.json", line+"\n"), "--dry-run"})
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want refused", code)
		}
		assertNoWrite(t, f, *budget)
	})
}

// regression: #4554
//
// TestUnhonouredFlagRefused: a flag the selected route does not read is refused before mint
// or any forge call, never accepted and ignored. Every flag some route honours is tried on
// every route that does not.
func TestUnhonouredFlagRefused(t *testing.T) {
	value := map[string]string{
		"evidence-file": "docs/streams/x/brief.md", "brief-path": "docs/streams/x/brief.md",
		"outcome-record": "/nonexistent/record.json", "root": "/nonexistent", "row": "01",
		deskkit.ScanOverrideFlag: "fixture reason",
	}
	all := map[string]bool{}
	for _, flags := range routeHonours {
		for _, fl := range flags {
			all[fl] = true
		}
	}
	base := map[string][]string{
		routeEvidenceFile:  {"--evidence-file", "docs/streams/x/brief.md"},
		routeOutcomeRecord: {"--outcome-record", "/nonexistent/record.json"},
	}
	for route, honours := range routeHonours {
		own := map[string]bool{}
		for _, fl := range honours {
			own[fl] = true
		}
		for fl := range all {
			if own[fl] {
				continue
			}
			// The two routes' selector flags are already mutually exclusive, refused by
			// their own check; --outcome-only off its route has its own refusal too.
			if fl == "evidence-file" || fl == "brief-path" || fl == "outcome-record" || fl == "outcome-only" {
				continue
			}
			t.Run(route+"/"+fl, func(t *testing.T) {
				f, errBuf := setupFake(t)
				mints := 0
				mintTokenFn = func(string) error { mints++; return nil }
				args := append([]string{fixtureRepo, "main"}, base[route]...)
				if v, ok := value[fl]; ok {
					args = append(args, "--"+fl, v)
				} else {
					args = append(args, "--"+fl)
				}
				code := run(args)
				if code != deskkit.ExitRefused || !strings.Contains(errBuf.String(), "--"+fl+" has no effect") {
					t.Fatalf("--%s on %s: exit = %d stderr %q, want refused as ignored", fl, route, code, errBuf.String())
				}
				if mints != 0 || len(f.hits) != 0 {
					t.Fatalf("--%s on %s: refused after %d mint(s) and forge hits %v; want before either", fl, route, mints, f.hits)
				}
			})
		}
	}
}
