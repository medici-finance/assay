package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestPlanningRevisionReuseAndRefresh(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "input")
	if err := os.WriteFile(file, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	fresh := func() *planningRevisionReader {
		return &planningRevisionReader{RootRevisionReader: deskkit.NewRootRevisionReader(root, "fixture"), revisions: map[string]planningRevision{}}
	}
	pass := fresh()
	before, ok := pass.Revision("file:input")
	if !ok {
		t.Fatal("initial read failed")
	}
	if _, ok := pass.Revision("file:missing"); ok {
		t.Fatal("missing input accepted")
	}
	if err := os.WriteFile(file, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "missing"), []byte("now present"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, ok := pass.Revision("file:input"); !ok || got != before {
		t.Fatal("one pass reread an observed input")
	}
	if _, ok := pass.Revision("file:missing"); ok {
		t.Fatal("unknown input was reread within pass")
	}
	if got, ok := fresh().Revision("file:input"); !ok || got == before {
		t.Fatal("new pass retained old input")
	}
	if _, ok := fresh().Revision("file:missing"); !ok {
		t.Fatal("new pass retained old failure")
	}
	if _, ok := pass.Revision("file:../outside"); ok {
		t.Fatal("escaped root")
	}
	// Method promotion must retain receipt-scope validation, not just WakeInputs.
	var scoped interface {
		CheckReceiptScope(deskkit.WakeReceipt) error
	} = pass
	if err := scoped.CheckReceiptScope(deskkit.WakeReceipt{Brief: "sample/01"}); err == nil {
		t.Fatal("incomplete receipt scope was accepted")
	}
}

type observedStateForge struct {
	deskkit.Forge
	reads int
	state string
	err   error
}

func (f *observedStateForge) GetIssue(_ deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	f.reads++
	return &deskkit.Issue{Number: n, State: f.state}, f.err
}
func (f *observedStateForge) GetIssueTyped(r deskkit.ForgeRepo, n int, _ deskkit.TargetKind) (*deskkit.Issue, error) {
	return f.GetIssue(r, n)
}

func TestBlockerObservationReuse(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker", "example-org/agents")
	forge := &observedStateForge{state: "open"}
	src := &forgeIssueSource{forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) { return forge, nil }}
	read := func(raw string) (deskkit.BlockerState, string) {
		t.Helper()
		ref, err := deskkit.ParseBlockerRef(raw, "example-org", "tracker")
		if err != nil {
			t.Fatal(err)
		}
		return src.IssueState(ref)
	}
	for _, raw := range []string{"#7", "example-org/tracker#7", "https://github.com/example-org/tracker/issues/7"} {
		if st, why := read(raw); st != deskkit.BlockerOpen || why != "" {
			t.Fatalf("%s: %v %s", raw, st, why)
		}
	}
	if forge.reads != 1 {
		t.Fatalf("equivalent references fetched %d times", forge.reads)
	}
	read("https://github.com/example-org/tracker/pull/7")
	read("example-org/agents#7")
	read("#8")
	if forge.reads != 4 {
		t.Fatal("target/repository/number collision")
	}
	forge.state = "closed"
	if st, _ := read("#7"); st != deskkit.BlockerOpen {
		t.Fatal("one pass reread mutable state")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "streams"), 0700); err != nil {
		t.Fatal(err)
	}
	v := &VerifyLoop{Root: root, Issues: src}
	if _, err := v.SelectQueue(); err != nil {
		t.Fatal(err)
	}
	if st, _ := read("#7"); st != deskkit.BlockerClosed || forge.reads != 5 {
		t.Fatal("new pass did not refresh")
	}
	forge.err = errors.New("unavailable")
	for _, raw := range []string{"#9", "example-org/tracker#9"} {
		if st, why := read(raw); st != deskkit.BlockerCouldNotCheck || !strings.Contains(why, raw) || !strings.Contains(why, "unavailable") {
			t.Fatalf("caller diagnostic lost: %v %s", st, why)
		}
	}
	if forge.reads != 6 {
		t.Fatal("failure not reused")
	}
	forge.err = nil
	if st, _ := read("#9"); st != deskkit.BlockerCouldNotCheck {
		t.Fatal("cached failure became closed")
	}
	src.beginPlanningPass()
	if st, _ := read("#9"); st != deskkit.BlockerClosed {
		t.Fatal("failure persisted into next pass")
	}
	read("unconfigured/repo#7")
	if forge.reads != 7 {
		t.Fatal("unconfigured target bypassed allowlist")
	}
}

type schedulingIssueFunc func(deskkit.BlockerRef) (deskkit.BlockerState, string)

func (f schedulingIssueFunc) IssueState(ref deskkit.BlockerRef) (deskkit.BlockerState, string) {
	return f(ref)
}

func TestPlanningRevisionProductionScan(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{"01": planBrief("model", "no", "no", "no", "no"), "02": planBrief("model", "no", "no", "no", "no")})
	common := filepath.Join(root, "shared.txt")
	if err := os.WriteFile(common, []byte("initial"), 0600); err != nil {
		t.Fatal(err)
	}
	var receipts []deskkit.WakeReceipt
	for _, num := range []string{"01", "02"} {
		rec := heldReceipt(holdTS)
		rec.Brief = "example-stream/" + num
		rec.Repo = "example-org/tracker"
		rec.ID += "-" + num
		path := "docs/streams/example-stream/brief-" + num + "-x.md"
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		rec.Inputs = map[string]string{"file:" + path: deskkit.Sha256Hex(body), "file:shared.txt": deskkit.Sha256Hex([]byte("initial"))}
		receipts = append(receipts, rec)
	}
	writeReceipts(t, root, receipts...)
	reads := 0
	issues := schedulingIssueFunc(func(deskkit.BlockerRef) (deskkit.BlockerState, string) {
		reads++
		if reads == 1 {
			if err := os.WriteFile(common, []byte("changed"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return deskkit.BlockerOpen, ""
	})
	v := &VerifyLoop{Root: root, RootRepo: "example-org/tracker", Issues: issues, Now: fixedClock("2026-09-20T12:00:00Z")}
	first, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("expected both unchanged receipts to reach blocker observation, got %d: %+v", reads, first)
	}
	for _, it := range first {
		if it.Payload["wake_state"] != "hold" {
			t.Fatalf("input reread within scan: %+v", it)
		}
	}
	second, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range second {
		if it.Payload["wake_state"] != "fire" {
			t.Fatalf("next scan missed changed input: %+v", it)
		}
	}
}
