package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantLanding appends n audit rows for tool on repo at `at` to the throwaway HOME's ledger.
// A numbered tool (deskpr) gets a distinct PR per row; deskevidence rows are unnumbered.
func plantLanding(t *testing.T, at time.Time, tool, repo string, n int, raw string) {
	t.Helper()
	dir, err := deskkit.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if raw != "" {
		f.WriteString(raw + "\n")
	}
	for i := 0; i < n; i++ {
		e := deskkit.Entry{TS: at.UTC().Format(time.RFC3339), Tool: tool, Verb: "land", Repo: repo,
			Result: deskkit.ResultOK, Detail: "planted", ArgsDigest: "0", SourceSHA: "dev", BuiltAt: "dev", SessionTag: "t"}
		if tool == "deskpr" {
			pr := 500 + i
			e.Verb, e.PR = "create", &pr
		}
		b, _ := json.Marshal(e)
		f.Write(append(b, '\n'))
	}
}

func capStage(t *testing.T, loop, stage string, depth, slots, max int, now time.Time) (*stageRow, string) {
	t.Helper()
	s := &stageRow{Stage: stage, Loop: loop, Depth: intp(depth), Slots: slots, MaxSlots: max, BoundBy: "the roster maximum"}
	s.finish()
	landingFor(s, now)
	rep := &throughputReport{Stages: []stageRow{*s}, StagesTotal: 1}
	pickBottleneck(rep)
	return s, throughputAdvice(rep)
}

// capHome isolates HOME (ledger + width store) and re-plants the fixture roster there.
func capHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
}

func watchedRepos(t *testing.T) []string {
	t.Helper()
	r := deskkit.AllowedRepos()
	sort.Strings(r)
	if len(r) < 2 {
		t.Fatalf("fixture needs two watched repos, have %v", r)
	}
	return r
}

// regression: F-landing-capacity
func TestFlowCapacity(t *testing.T) {
	now := time.Now().UTC()

	t.Run("free landing still widens to the ceiling", func(t *testing.T) {
		capHome(t)
		s, adv := capStage(t, "worker-desk", "dispatch", 30, 8, 12, now)
		if s.Landing.Blind != "" || s.Landing.Items == nil || s.Landing.Scope != deskkit.LedgerScope {
			t.Fatalf("landing must be read, dated and scoped: %+v", s.Landing)
		}
		if !strings.Contains(adv, "deskroster set --role worker-desk --width 12") {
			t.Fatalf("an empty ledger leaves the roster ceiling as the bound: %q", adv)
		}
	})

	t.Run("exhausted budget on a second repo holds the width", func(t *testing.T) {
		capHome(t)
		repos := watchedRepos(t)
		second := repos[len(repos)-1]
		plantLanding(t, now.Add(-time.Minute), "deskpr", repos[0], 1, "")
		plantLanding(t, now.Add(-time.Minute), "deskpr", second, deskkit.RateLimitPerPRPerHour, "")
		s, adv := capStage(t, "worker-desk", "dispatch", 30, 8, 12, now)
		if !deskkit.IsRateLimited(deskkit.AllowPRCreateAt(second, now)) {
			t.Fatal("fixture: the enforced gate must refuse on the planted repo")
		}
		if !s.Landing.Blocked || s.Landing.Repo != second || s.Landing.FreeAt == "" {
			t.Fatalf("the binding landing reading must be the refused repo: %+v", s.Landing)
		}
		if strings.Contains(adv, "deskroster set") || !strings.Contains(adv, "no widening — bound by the landing write budget on "+second) {
			t.Fatalf("a refused landing gate must hold the width: %q", adv)
		}
	})

	t.Run("verify items cost two writes and reservations count", func(t *testing.T) {
		capHome(t)
		repo := watchedRepos(t)[0]
		left := deskkit.UnnumberedCapFor("deskevidence") - 14 // 14 writes = 7 verify items
		plantLanding(t, now.Add(-time.Minute), "deskevidence", repo, left, "")
		// 6 in flight (assumed: the width) reserve 12 writes; 2 more remain = 1 item.
		s, adv := capStage(t, "verify-desk", "verify", 20, 6, 6, now)
		if s.Landing.WritesPerItem != 2 || s.Landing.InFlight != 6 || *s.Landing.Items != 1 {
			t.Fatalf("landing = %+v, want 2 writes/item, 6 reserved, 1 more item", s.Landing)
		}
		if err := deskkit.AllowWriteAt("deskevidence", repo, 0, now); err != nil {
			t.Fatalf("the report says writes remain but the gate refuses: %v", err)
		}
		if strings.Contains(adv, "deskroster set") {
			t.Fatalf("a stage at its ceiling never widens: %q", adv)
		}
		_, adv = capStage(t, "verify-desk", "verify", 20, 6, 10, now)
		if !strings.Contains(adv, "deskroster set --role verify-desk --width 7") {
			t.Fatalf("with a ceiling of 10 the landing capacity (6 reserved + 1) must bind: %q", adv)
		}
	})

	t.Run("unreadable meter never widens", func(t *testing.T) {
		capHome(t)
		plantLanding(t, now, "deskpr", "", 0, "{not json")
		s, adv := capStage(t, "worker-desk", "dispatch", 30, 8, 12, now)
		if s.Landing.Blind == "" || s.Landing.Items != nil {
			t.Fatalf("an unreadable ledger must be blind with no item count: %+v", s.Landing)
		}
		if strings.Contains(adv, "deskroster set") || !strings.Contains(adv, "COULD-NOT-CHECK") {
			t.Fatalf("blind landing advice must say so and not widen: %q", adv)
		}
		if got := landingCell(s.Landing); got != "n/a" {
			t.Fatalf("an unread landing cell must render n/a, got %q", got)
		}
	})

	t.Run("breaker and reset boundaries match the gate", func(t *testing.T) {
		capHome(t)
		repo := watchedRepos(t)[0]
		at := now.Add(-time.Minute)
		dir, _ := deskkit.StateDir()
		os.MkdirAll(dir, 0o700)
		f, _ := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		for i := 0; i < deskkit.BreakerTrip; i++ {
			b, _ := json.Marshal(deskkit.Entry{TS: at.Format(time.RFC3339), Tool: "deskpr", Verb: "create", Repo: repo,
				Result: deskkit.ResultRefused, Detail: "planted", ArgsDigest: "0", SourceSHA: "dev", BuiltAt: "dev", SessionTag: "t"})
			f.Write(append(b, '\n'))
		}
		f.Close()
		for _, probe := range []time.Time{now, at.Add(deskkit.BreakerCooldown - time.Second), at.Add(deskkit.BreakerCooldown + time.Second)} {
			s, adv := capStage(t, "worker-desk", "dispatch", 30, 8, 12, probe)
			gate := deskkit.AllowPRCreateAt(repo, probe)
			if s.Landing.Blocked != deskkit.IsRateLimited(gate) {
				t.Fatalf("at %s report blocked=%v but gate=%v", probe, s.Landing.Blocked, gate)
			}
			// Open or half-open (one probe, and 8 lanes already reserved), it never widens.
			if strings.Contains(adv, "deskroster set") {
				t.Fatalf("a tripped breaker must never widen at %s: %q", probe, adv)
			}
		}
	})

	t.Run("held or human-only depth cannot widen", func(t *testing.T) {
		capHome(t)
		// The dispatch depth already excludes rows held on a human decision; with nothing
		// actionable left, free landing capacity is still no reason to widen.
		_, adv := capStage(t, "worker-desk", "dispatch", 0, 8, 12, now)
		if strings.Contains(adv, "deskroster set") {
			t.Fatalf("zero actionable depth widened: %q", adv)
		}
	})

	t.Run("throughput JSON carries the landing reading", func(t *testing.T) {
		throughputFixture(t)
		rep := runThroughput(t)
		for _, s := range rep.Stages {
			if s.Landing.Provider != deskkit.ProviderCapacityUnknown || s.Landing.Scope == "" {
				t.Errorf("stage %s landing lacks provider/scope labels: %+v", s.Stage, s.Landing)
			}
			if s.Landing.Blind == "" && s.Landing.Items == nil {
				t.Errorf("stage %s landing has neither items nor a reason: %+v", s.Stage, s.Landing)
			}
		}
	})
}
