package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
	"github.com/medici-finance/assay/tools/desk/internal/kubecompat/testfixture"
)

func kubeWorld(t *testing.T) (*cliWorld, *testfixture.Server) {
	t.Helper()
	s := testfixture.New(t)
	w := newCLIWorld(t, "CELL_KIND=k8s\nCELL_KUBERNETES_CONFIG="+s.ProfilePath+"\nDESKD=0\n")
	return w, s
}

// regression: #4759
func TestKubeCompatCellctlLifecycle(t *testing.T) {
	w, s := kubeWorld(t)
	check := w.run(t, nil, "check", "example")
	if check.Code != 0 || s.Calls != 0 {
		t.Fatal("offline check contacted API", check)
	}
	plan := w.run(t, nil, "up", "example", "--plan", "--json")
	if plan.Code != 0 || s.Calls != 0 {
		t.Fatal("offline plan contacted API", plan)
	}
	var stored cellop.Plan
	if e := json.Unmarshal([]byte(plan.Stdout), &stored); e != nil || stored.Verify() != nil {
		t.Fatal("invalid cell-op plan", e, plan.Stdout)
	}
	for _, role := range kc.Roles {
		got := w.run(t, nil, "desk", "example", role)
		if got.Code != 0 {
			t.Fatal(role, got)
		}
		again := w.run(t, nil, "desk", "example", role)
		if again.Code != 0 {
			t.Fatal("repeat start", again)
		}
		cmd := exec.Command(cellctlBinary(t), "attach", "example", role)
		cmd.Env = w.baseEnv()
		cmd.Stdin = strings.NewReader("retained " + role + "\x1dunknown\n\x1dASSAY_STOP not-confirmed\n")
		out, e := cmd.CombinedOutput()
		if e != nil || !strings.Contains(string(out), "retained terminal") {
			t.Fatal("native attach", e, string(out))
		}
		if got := s.Input(role); got != "retained "+role+"unknown\nASSAY_STOP not-confirmed\n" {
			t.Fatal("compiled attach lost escape text or forwarded stop control", got)
		}
	}
	starts, _, _ := s.Counts()
	if starts != 5 {
		t.Fatal("reconnect/repeated up changed invocation count", starts)
	}
	state := w.run(t, nil, "status", "example")
	if state.Code != 0 || strings.Count(state.Stdout, "agent execution observed") != 5 {
		t.Fatal(state)
	}
	code, out := kubeTTY(t, w, 3*time.Second, "down", "example")
	if code != 0 {
		t.Fatal("controlled stop", code, out)
	}
	for _, role := range kc.Roles {
		receipt, e := kc.ReadReceipt(filepath.Join(w.cellDir, "run", "kubecompat", role+".json"))
		if e != nil || receipt.State != "clean-stopped" {
			t.Fatal("missing exact clean receipt", receipt, e)
		}
	}
	result := w.run(t, nil, "up", "example", "--json")
	if result.Code != 0 {
		t.Fatal(result)
	}
	var doc cellop.Result
	if e := json.Unmarshal([]byte(result.Stdout), &doc); e != nil || doc.Schema != cellop.Schema || len(doc.Roles) != 5 {
		t.Fatal("cell-op result drift", e, result.Stdout)
	}
	starts, _, _ = s.Counts()
	if starts != 10 {
		t.Fatal("clean new execution count", starts)
	}
}

// regression: #4759
func TestKubeCompatStopUncertainty(t *testing.T) {
	for _, fault := range []string{"busy", "unknown-work", "absent", "changed", "no-ack", "survivor", "partition"} {
		t.Run(fault, func(t *testing.T) {
			w, s := kubeWorld(t)
			if run := w.run(t, nil, "desk", "example", "worker-desk"); run.Code != 0 {
				t.Fatal(run)
			}
			s.Fault = fault
			code, _ := kubeTTY(t, w, 700*time.Millisecond, "down", "example", "worker-desk")
			if code == 0 {
				t.Fatal("uncertain stop passed")
			}
			r, e := kc.ReadReceipt(filepath.Join(w.cellDir, "run", "kubecompat", "worker-desk.json"))
			if e != nil || r.State != "running" {
				t.Fatal("uncertainty turned into clean stop", r, e)
			}
			starts, _, _ := s.Counts()
			if starts != 1 {
				t.Fatal("replacement invocation", starts)
			}
		})
	}
}

// regression: #4759
func TestKubeCompatParity(t *testing.T) {
	for _, flag := range []string{"DISABLED", "STOP", "STOP.worker-desk", "STOP.cell.example", "STOP.cell.example.worker-desk"} {
		w, s := kubeWorld(t)
		path := filepath.Join(w.cellDir, "home", ".config", "assay", flag)
		if e := os.WriteFile(path, []byte("held"), 0600); e != nil {
			t.Fatal(e)
		}
		got := w.run(t, nil, "desk", "example", "worker-desk")
		if got.Code == 0 || s.Calls != 0 {
			t.Fatal("existing STOP bypassed", flag, got)
		}
	}
	if aliases, ok := deskkit.LoopFlagNames("worker-desk"); ok {
		for _, alias := range aliases {
			w, s := kubeWorld(t)
			path := filepath.Join(w.cellDir, "home", ".config", "assay", deskkit.CellStopFlagName("example", alias))
			_ = os.WriteFile(path, []byte("held"), 0600)
			got := w.run(t, nil, "desk", "example", "worker-desk")
			if got.Code == 0 || s.Calls != 0 {
				t.Fatal("role STOP alias bypassed", alias, got)
			}
		}
	}
	w, s := kubeWorld(t)
	for _, args := range [][]string{{"desk", "example", "worker-desk", "--model", "foreign"}, {"desk", "example", "worker-desk", "--cockpit", "tmux"}, {"up", "example", "--harness", "codex"}} {
		got := w.run(t, nil, args...)
		if got.Code == 0 || s.Calls != 0 {
			t.Fatal("unsupported override reached API", args, got)
		}
	}
	w.writeHouseEnv(t, "CELL_KIND=k8s\nCELL_KUBERNETES_CONFIG="+s.ProfilePath+"\nCELL_ROLE_CONTEXT=missing.json\n")
	got := w.run(t, nil, "desk", "example", "worker-desk")
	if got.Code == 0 || s.Calls != 0 {
		t.Fatal("unsupported declared role context ignored", got)
	}
	w.writeHouseEnv(t, "CELL_KIND=k8s\nCELL_KUBERNETES_CONFIG="+s.ProfilePath+"\n")
	lease, e := cellcadence.AcquireRole(filepath.Join(w.cellDir, "run", "cadence", "worker-desk"), cellcadence.RoleLock{Cell: "example", Role: "worker-desk"})
	if e != nil {
		t.Fatal(e)
	}
	got = w.run(t, nil, "desk", "example", "worker-desk")
	_ = lease.Close()
	if got.Code == 0 || s.Calls != 0 {
		t.Fatal("legacy local role lease bypassed", got)
	}
	w.writeHouseEnv(t, "")
	got = w.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
	if got.Code != 0 || s.Calls != 0 || !strings.Contains(got.Stdout, "[dry-run]") {
		t.Fatal("legacy house routing changed", got)
	}
	// The actual retained pre-migration corpus covers non-selected k8s/container/scrubbed
	// routing and existing JSON/confirm behavior; this row invokes that check as a neighbour.
	TestCLINonHelpParity(t)
}
func kubeRunDeadline(t *testing.T, w *cliWorld, timeout time.Duration, args ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cellctlBinary(t), args...)
	cmd.Env = w.baseEnv()
	out, e := cmd.CombinedOutput()
	if e == nil {
		return 0, string(out)
	}
	if exit, ok := e.(*exec.ExitError); ok {
		return exit.ExitCode(), string(out)
	}
	return 1, string(out)
}

var _ = os.ErrNotExist

// regression: #4759
func TestKubeCompatPlanInputGone(t *testing.T) {
	w, s := kubeWorld(t)
	got := w.run(t, nil, "up", "example", "--role", "worker-desk", "--plan", "--json")
	if got.Code != 0 {
		t.Fatal(got)
	}
	var plan cellop.Plan
	if e := json.Unmarshal([]byte(got.Stdout), &plan); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(s.ProfilePath); e != nil {
		t.Fatal(e)
	}
	code, out := kubeTTY(t, w, 3*time.Second, "up", "example", "--apply", plan.ID, "--json")
	if code != 6 {
		t.Fatal("unmeasured input did not return6", code, out)
	}
	normalized := strings.ReplaceAll(out, "\r", "")
	i := strings.LastIndex(normalized, "{\"schema\"")
	if i < 0 {
		t.Fatal("missing structured result", out)
	}
	var result cellop.Result
	if e := json.Unmarshal([]byte(normalized[i:]), &result); e != nil || result.Outcome != cellop.CouldNotCheck {
		t.Fatal("invalid could-not-check result", e, out)
	}
	if consumed, e := cellop.StoreFor(w.cellDir).Consumed(plan.ID); e != nil || consumed || s.Calls != 0 {
		t.Fatal("unmeasured plan consumed/effected", consumed, e, s.Calls)
	}
}

// regression: #4759
func TestKubeCompatPartialUp(t *testing.T) {
	w, s := kubeWorld(t)
	s.AfterStart = func(role string) {
		if role == "intake-desk" {
			if e := os.WriteFile(filepath.Join(w.cellDir, "home", ".config", "assay", "STOP.worker-desk"), []byte("held"), 0600); e != nil {
				t.Error(e)
			}
		}
	}
	got := w.run(t, nil, "up", "example", "--no-the-desk", "--role", "intake-desk", "--role", "worker-desk", "--json")
	var result cellop.Result
	if e := json.Unmarshal([]byte(got.Stdout), &result); e != nil {
		t.Fatal("partial up lost structured result", e, got)
	}
	if got.Code != 1 || result.Outcome != cellop.Partial || len(result.Roles) != 2 ||
		result.Roles[0].Role != "intake-desk" || *result.Roles[0].State != upStateUp ||
		result.Roles[1].Role != "worker-desk" || *result.Roles[1].State != upStateFailed {
		t.Fatal("partial up lost completed/refused roles", got, result)
	}
	starts, _, _ := s.Counts()
	if starts != 1 {
		t.Fatal("refused role started", starts)
	}
	if r, e := kc.ReadReceipt(filepath.Join(w.cellDir, "run", "kubecompat", "intake-desk.json")); e != nil || r.State != "running" {
		t.Fatal("earlier execution receipt lost", r, e)
	}
}

// regression: #4759
func TestKubeCompatInput(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"ordinary", "ordinary"},
		{"a\x1dunknown\nb", "aunknown\nb"},
		{"\x1dASSAY_STOP pod launch\n", "ASSAY_STOP pod launch\n"},
		{"\x1ddet", "det"},
	} {
		in := &kubeInput{source: strings.NewReader(tc.input), cancel: func() { t.Fatal("unexpected detach") }}
		got, e := io.ReadAll(in)
		if e != nil || string(got) != tc.want {
			t.Fatal("lost input or forwarded reserved control", tc, string(got), e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := &kubeInput{source: strings.NewReader("before\x1ddetach\nafter"), cancel: cancel}
	buf := make([]byte, 64)
	n, e := in.Read(buf)
	if e != nil || string(buf[:n]) != "before" || ctx.Err() != nil {
		t.Fatal("detach discarded preceding input", n, e, ctx.Err())
	}
	if n, e = in.Read(buf); n != 0 || e != context.Canceled || ctx.Err() == nil {
		t.Fatal("detach not consumed locally", n, e, ctx.Err())
	}
}
