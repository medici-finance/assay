package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// writeFixtureStream lays down a stream README + brief files under root for SelectQueue tests.
func writeFixtureStream(t *testing.T, root, stream, table string, briefs map[string]string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "---\nstream: " + stream + "\nstatus: active\n---\n\n# " + stream + "\n\n## Briefs\n\n" + table + "\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	for num, body := range briefs {
		if err := os.WriteFile(filepath.Join(dir, "brief-"+num+"-x.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func briefBody(risk, gate, evidence string) string {
	return "---\nbrief: x\ngate: " + gate + "\nrisk: {regulatory: " + risk + ", customer: no, irreversible: no, sensitive-data: no}\neffort: S\n---\n\n# Brief\n\n## Verify\n\n| # | Command | Expect |\n\n## Evidence\n<!-- appended at verification time -->\n" + evidence + "\n"
}

func TestSelectQueue_AwaitingFilterAndTierOrder(t *testing.T) {
	root := t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | A | 0 | S | done | x | x |\n" + // filtered out (done)
		"| 02 | B | 0 | S | verified | 2026-07-18 glm | — |\n" + // tier-2 (verified free-close)
		"| 03 | C | 0 | S | implemented | — | — |\n" + // tier-1 (implemented, empty evidence)
		"| 04 | D | 0 | S | todo | — | — |\n" // filtered out (todo)
	briefs := map[string]string{
		"01": briefBody("no", "model", "landed"),
		"02": briefBody("no", "model", "| 1 | `go test` | 0 | ok | 2026-07-18 | glm |"),
		"03": briefBody("no", "model", ""),
	}
	writeFixtureStream(t, root, "fixture", table, briefs)

	v := &VerifyLoop{Root: root, TargetSHA: "abc123"}
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	// done(01) and todo(04) are filtered; 03 (tier-1) must precede 02 (tier-2).
	if len(ids) != 2 {
		t.Fatalf("got %v; want exactly [fixture/03 fixture/02]", ids)
	}
	if ids[0] != "fixture/03" || ids[1] != "fixture/02" {
		t.Fatalf("tier order wrong: got %v; want tier-1 fixture/03 before tier-2 fixture/02", ids)
	}
	if items[0].TargetSHA != "abc123" {
		t.Fatalf("target SHA not stamped: %q", items[0].TargetSHA)
	}
}

func TestTierPolicy_Routing(t *testing.T) {
	v := &VerifyLoop{}
	cases := []struct {
		name string
		it   loopengine.Item
		want loopengine.Tier
	}{
		{"risk-clear -> local", loopengine.Item{Gate: "model"}, loopengine.TierLocal},
		{"gate human -> human", loopengine.Item{Gate: "human"}, loopengine.TierHuman},
		{"regulatory yes -> human", loopengine.Item{Gate: "human", Risk: loopengine.RiskFlags{Regulatory: true}}, loopengine.TierHuman},
		{"irreversible -> local (dispatched for Evidence)", loopengine.Item{Gate: "human", Risk: loopengine.RiskFlags{Irreversible: true}}, loopengine.TierLocal},
	}
	for _, c := range cases {
		got, err := v.TierPolicy(c.it)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestTierPolicy_F16RungOffByDefaultOneLineOn(t *testing.T) {
	risky := loopengine.Item{Gate: "human", Risk: loopengine.RiskFlags{Customer: true}} // risk-flagged, reversible
	off := &VerifyLoop{}
	if got, _ := off.TierPolicy(risky); got != loopengine.TierHuman {
		t.Fatalf("reversible-risk rung must be OFF by default: got %v want TierHuman", got)
	}
	on := &VerifyLoop{F16ReversibleRiskToSession: true}
	if got, _ := on.TierPolicy(risky); got != loopengine.TierSession {
		t.Fatalf("reversible-risk rung ON (one-line) should route reversible risk to session: got %v", got)
	}
	// Irreversible stays a human flip even with the rung ON.
	irr := loopengine.Item{Gate: "human", Risk: loopengine.RiskFlags{Irreversible: true}}
	if got, _ := on.TierPolicy(irr); got != loopengine.TierLocal {
		t.Fatalf("irreversible must stay TierLocal (Evidence, human flip) even with rung ON: got %v", got)
	}
}

func TestDispatchPrompt_CarriesEssentialsAndNoSharedCheckout(t *testing.T) {
	it := loopengine.Item{
		ID: "fixture/03", BriefPath: "docs/streams/fixture/brief-03-x.md", TargetSHA: "deadbeef",
		Payload: map[string]string{"verify_table": "| 1 | `go test ./...` | exit 0 |"},
	}
	p := renderDispatchPrompt(it, loopengine.TierLocal)
	for _, must := range []string{"deadbeef", "/private/tmp", "go test ./...", "verifier != author", "STRUCTURED", "docs/streams/fixture/brief-03-x.md"} {
		if !strings.Contains(p, must) {
			t.Fatalf("prompt missing %q:\n%s", must, p)
		}
	}
	if err := assertNoSharedCheckout(p); err != nil {
		t.Fatalf("clean prompt flagged: %v", err)
	}
	// A shared-checkout path must be refused.
	leaky := p + "\ncd /Users/x/tracker/.claude/worktrees/thedesk && go test"
	if err := assertNoSharedCheckout(leaky); err == nil {
		t.Fatal("shared-checkout path not refused")
	}
}

// recordingDurable captures Land's decisions so tests can assert Land consumes ONLY the
// structured Result and routes correctly.
type recordingDurable struct {
	evidence   map[string]string
	flipped    map[string]bool
	checkpoint []string
	routed     []string
	bugs       []string
}

func newRec() *recordingDurable {
	return &recordingDurable{evidence: map[string]string{}, flipped: map[string]bool{}}
}
func (r *recordingDurable) WriteEvidenceAndFlip(it loopengine.Item, ev string, flip bool) error {
	r.evidence[it.ID] = ev
	r.flipped[it.ID] = flip
	return nil
}
func (r *recordingDurable) Checkpoint(it loopengine.Item, ev string) (string, error) {
	r.checkpoint = append(r.checkpoint, it.ID)
	return "cp", nil
}
func (r *recordingDurable) RouteHuman(it loopengine.Item) (string, error) {
	r.routed = append(r.routed, it.ID)
	return "rh", nil
}
func (r *recordingDurable) FileBug(it loopengine.Item, detail string) (string, error) {
	r.bugs = append(r.bugs, it.ID+": "+detail)
	return "bug", nil
}

func fixedNow() time.Time { return time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) }

func TestLand_PassFlipsWithAttributedEvidence(t *testing.T) {
	rec := newRec()
	v := &VerifyLoop{DurableSink: rec, Now: fixedNow}
	r := loopengine.Result{
		Item:     loopengine.Item{ID: "fixture/03"},
		Verdict:  loopengine.VerdictPass,
		RunnerID: "local:glm-verifier",
		Rows:     []loopengine.EvidenceRow{{Command: "go test ./...", Exit: 0, Output: "ok"}},
	}
	if err := v.Land(r); err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !rec.flipped["fixture/03"] {
		t.Fatal("reversible PASS did not flip status")
	}
	ev := rec.evidence["fixture/03"]
	for _, must := range []string{"go test ./...", "2026-07-20", "local:glm-verifier", "PASS"} {
		if !strings.Contains(ev, must) {
			t.Fatalf("evidence missing %q:\n%s", must, ev)
		}
	}
	// Evidence is derived from the structured rows, not a bare check.
	if strings.Contains(ev, "✓") {
		t.Fatal("evidence contains a bare ✓ — must be command/exit/output derived")
	}
}

func TestLand_IrreversibleWritesEvidenceNoFlipCheckpoint(t *testing.T) {
	rec := newRec()
	v := &VerifyLoop{DurableSink: rec, Now: fixedNow}
	r := loopengine.Result{
		Item:     loopengine.Item{ID: "migrations/09", Risk: loopengine.RiskFlags{Irreversible: true}},
		Verdict:  loopengine.VerdictPass,
		RunnerID: "local:glm",
		Rows:     []loopengine.EvidenceRow{{Command: "go test", Exit: 0, Output: "ok"}},
	}
	if err := v.Land(r); err != nil {
		t.Fatalf("Land: %v", err)
	}
	if rec.flipped["migrations/09"] {
		t.Fatal("irreversible brief was flipped by a model — must be Evidence-only, human flip")
	}
	if _, ok := rec.evidence["migrations/09"]; !ok {
		t.Fatal("irreversible brief got no Evidence written")
	}
	if len(rec.checkpoint) != 1 || rec.checkpoint[0] != "migrations/09" {
		t.Fatalf("no checkpoint PR opened for irreversible brief: %v", rec.checkpoint)
	}
}

func TestLand_FailFilesBugNoFlip(t *testing.T) {
	rec := newRec()
	v := &VerifyLoop{DurableSink: rec, Now: fixedNow}
	r := loopengine.Result{
		Item:     loopengine.Item{ID: "x/01"},
		Verdict:  loopengine.VerdictFail,
		RunnerID: "local:glm",
		Rows:     []loopengine.EvidenceRow{{Command: "go test", Exit: 1, Output: "FAIL"}},
	}
	_ = v.Land(r)
	if rec.flipped["x/01"] {
		t.Fatal("a FAIL flipped status")
	}
	if len(rec.bugs) == 0 {
		t.Fatal("a FAIL did not file a change-failure bug")
	}
}

func TestLand_RouteHuman(t *testing.T) {
	rec := newRec()
	v := &VerifyLoop{DurableSink: rec, Now: fixedNow}
	r := loopengine.Result{Item: loopengine.Item{ID: "risky/02"}, Verdict: loopengine.VerdictRouteHuman}
	if err := v.Land(r); err != nil {
		t.Fatalf("Land: %v", err)
	}
	if len(rec.routed) != 1 || rec.routed[0] != "risky/02" {
		t.Fatalf("route-human not recorded: %v", rec.routed)
	}
}

func TestDispatch_InterimNoFeederIsBlockedOnHuman(t *testing.T) {
	v := &VerifyLoop{Emit: &bytes.Buffer{}} // no Feeder wired
	_, err := v.Dispatch(loopengine.Item{ID: "a"}, loopengine.TierLocal)
	if err == nil || !strings.Contains(err.Error(), "BLOCKED-ON-HUMAN") {
		t.Fatalf("interim dispatch without a feeder must refuse as BLOCKED-ON-HUMAN, got: %v", err)
	}
}

func TestCommitPushRace_RetriesThenSucceeds(t *testing.T) {
	var calls []string
	pushAttempts := 0
	g := &gitDurable{
		root:     "/repo",
		maxRetry: 5,
		run: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			calls = append(calls, joined)
			if strings.Contains(joined, "push") {
				pushAttempts++
				if pushAttempts < 3 {
					return "! [rejected] main -> main (fetch first)", os.ErrPermission // simulate race
				}
			}
			return "", nil
		},
	}
	if err := g.commitPushRace([]string{"STATUS-source.md"}, "verify(x): evidence"); err != nil {
		t.Fatalf("commitPushRace: %v", err)
	}
	// The retry did commit -> push -> pull --rebase -> push -> pull --rebase -> push.
	joined := strings.Join(calls, "\n")
	if pushAttempts != 3 {
		t.Fatalf("expected 3 push attempts (2 races), got %d", pushAttempts)
	}
	if !strings.Contains(joined, "pull --rebase origin main") {
		t.Fatalf("push race did not rebase between attempts:\n%s", joined)
	}
}
