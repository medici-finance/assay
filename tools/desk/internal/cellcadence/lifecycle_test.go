package cellcadence

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// The lifecycle model is pure: these tests feed it explicit observations. The checkpoint writer
// it fronts is exercised against a real lease and checkpoint at the end of the matrix.

func TestCellLifecycleMatrix(t *testing.T) {
	t.Run("process identity", func(t *testing.T) {
		boom := errors.New("ps failed")
		for _, tc := range []struct {
			name     string
			recorded string
			probe    ProcessProbe
			want     ProcessVerdict
			alive    bool
			unknown  bool
			departed bool
		}{
			{"gone", "t1", ProcessProbe{Gone: true}, ProcessGone, false, false, true},
			{"gone with no recorded start", "", ProcessProbe{Gone: true}, ProcessGone, false, false, true},
			{"unreadable is not gone", "t1", ProcessProbe{Err: boom}, ProcessUnreadable, false, true, false},
			{"running, no recorded start", "", ProcessProbe{Start: "t9"}, ProcessNoRecordedStart, false, true, false},
			{"pid reused", "t1", ProcessProbe{Start: "t2"}, ProcessReused, false, false, true},
			{"same pid and start", "t1", ProcessProbe{Start: "t1"}, ProcessSame, true, false, false},
		} {
			v := JudgeProcess(tc.recorded, tc.probe)
			if v != tc.want || v.Alive() != tc.alive || v.Unknown() != tc.unknown || v.Departed() != tc.departed {
				t.Errorf("%s: verdict %d (alive %v unknown %v departed %v), want %d", tc.name, v, v.Alive(), v.Unknown(), v.Departed(), tc.want)
			}
		}
	})

	t.Run("record class", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			vs   []ProcessVerdict
			want string
		}{
			{"nothing recorded", nil, ClassDead},
			{"supervisor gone", []ProcessVerdict{ProcessGone}, ClassDead},
			{"reused pid reads dead, never alive", []ProcessVerdict{ProcessReused, ProcessGone}, ClassDead},
			{"supervisor alive", []ProcessVerdict{ProcessSame, ProcessGone}, ClassAlive},
			{"alive beats unknown", []ProcessVerdict{ProcessUnreadable, ProcessSame}, ClassAlive},
			{"unreadable is could-not-check", []ProcessVerdict{ProcessGone, ProcessUnreadable}, ClassCouldNotCheck},
			{"no recorded start is could-not-check", []ProcessVerdict{ProcessNoRecordedStart}, ClassCouldNotCheck},
		} {
			if got := ClassifyProcesses(tc.vs...); got != tc.want {
				t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
			}
		}
	})

	t0 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	running := func(pid int, seq uint64, start time.Time) State {
		return State{Mode: "interactive", Running: true, PID: pid, Sequence: seq, LastStart: start}
	}
	owner := Generation{PID: 100, Start: "s100", Sequence: 7, LastStart: t0}

	t.Run("owner generation", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			g         Generation
			s         State
			start     string
			attribute bool
			owns      bool
		}{
			// The #6251 repair, as a characterization case: the clean exit released the lease, so
			// no owner record survives (start empty); the checkpoint read before the signal binds.
			{"6251: owner record gone, same checkpoint", owner, running(100, 7, t0), "", true, true},
			{"6251: owner record gone, replacement owner's sequence", owner, running(100, 8, t0), "", false, false},
			{"6251: owner record gone, same pid, later start", owner, running(100, 7, t0.Add(time.Second)), "", false, false},
			{"6251: owner record gone, another pid", owner, running(101, 7, t0), "", false, false},
			{"6251: checkpoint was not read", Generation{PID: 100, Start: "s100"}, running(100, 7, t0), "", false, true},
			{"owner record survives, start matches", owner, running(100, 7, t0), "s100", true, true},
			{"owner record survives, pid reused (start differs)", owner, running(100, 7, t0), "s999", false, true},
			{"unnamed generation", Generation{PID: 100}, running(100, 7, t0), "", false, false},
			{"no pid", Generation{Start: "s100", Sequence: 7, LastStart: t0}, running(100, 7, t0), "", false, false},
			{"finished checkpoint", owner, State{Mode: "interactive", PID: 100, Sequence: 7, LastStart: t0}, "", true, false},
			{"cadence checkpoint", owner, State{Mode: "", Running: true, PID: 100, Sequence: 7, LastStart: t0}, "", true, false},
		} {
			if got := tc.g.Attributes(tc.s, tc.start); got != tc.attribute {
				t.Errorf("%s: Attributes = %v, want %v", tc.name, got, tc.attribute)
			}
			if got := tc.g.OwnsUnfinished(tc.s); got != tc.owns {
				t.Errorf("%s: OwnsUnfinished = %v, want %v", tc.name, got, tc.owns)
			}
		}
	})

	t.Run("settle", func(t *testing.T) {
		boom := errors.New("ps failed")
		rec := func(mut func(*SettleRecord)) *SettleRecord {
			r := &SettleRecord{HasState: true, State: running(100, 7, t0), Class: ClassDead}
			if mut != nil {
				mut(r)
			}
			return r
		}
		for _, tc := range []struct {
			name string
			in   SettleInput
			want SettleAction
		}{
			{"lease could not be probed", SettleInput{Owner: owner, LeaseErr: boom, Record: rec(nil)}, SettleRetry},
			{"owner still holds the lease", SettleInput{Owner: owner, LeaseBusy: true, Record: rec(nil)}, SettleRetry},
			{"owner recorded its own cleanup", SettleInput{Owner: owner}, SettleDone},
			{"repeated down: nothing left to finish", SettleInput{Owner: owner}, SettleDone},
			{"owner record did not identify the owner", SettleInput{Owner: Generation{}, Record: rec(nil)}, SettleRefuse},
			{"checkpoint of another pid", SettleInput{Owner: owner, Record: rec(func(r *SettleRecord) { r.State.PID = 101 })}, SettleRefuse},
			{"checkpoint is not interactive", SettleInput{Owner: owner, Record: rec(func(r *SettleRecord) { r.State.Mode = "" })}, SettleRefuse},
			{"owner record survives with another start: pid reuse", SettleInput{Owner: owner, Record: rec(func(r *SettleRecord) { r.Start = "s999" })}, SettleRefuse},
			{"processes could not be listed", SettleInput{Owner: owner, ScanErr: boom, Record: rec(nil)}, SettleRetry},
			{"a marked process runs", SettleInput{Owner: owner, MarkedRunning: true, Record: rec(nil)}, SettleRetry},
			{"record alive", SettleInput{Owner: owner, Record: rec(func(r *SettleRecord) { r.Class = ClassAlive })}, SettleRetry},
			{"record could-not-check is never dead", SettleInput{Owner: owner, Record: rec(func(r *SettleRecord) { r.Class = ClassCouldNotCheck })}, SettleRetry},
			{"record has no readable state", SettleInput{Owner: owner, Record: rec(func(r *SettleRecord) { r.HasState = false; r.State = State{} })}, SettleRefuse},
			{"6251: replacement owner's checkpoint, no owner record", SettleInput{Owner: owner, Record: rec(func(r *SettleRecord) { r.State.Sequence = 8 })}, SettleRefuse},
			{"6251: same checkpoint, no owner record", SettleInput{Owner: owner, Record: rec(nil)}, SettleRecordStopped},
			{"owner record survives with the same start", SettleInput{Owner: owner, Record: rec(func(r *SettleRecord) { r.Start = "s100" })}, SettleRecordStopped},
		} {
			d := JudgeSettle(tc.in)
			if d.Action != tc.want {
				t.Errorf("%s: action %d (%q), want %d", tc.name, d.Action, d.Reason, tc.want)
			}
			if d.Action != SettleDone && d.Action != SettleRecordStopped && d.Reason == "" {
				t.Errorf("%s: a non-final decision names no reason", tc.name)
			}
		}
	})

	t.Run("plan observation", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			o         RoleObservation
			class     string
			plannable bool
		}{
			{"stale dead record", RoleObservation{Kind: KindStale, HasRecord: true, RecordClass: ClassDead}, ClassDead, true},
			{"stale alive record", RoleObservation{Kind: KindStale, HasRecord: true, RecordClass: ClassAlive}, ClassAlive, false},
			{"stale could-not-check record", RoleObservation{Kind: KindStale, HasRecord: true, RecordClass: ClassCouldNotCheck}, ClassCouldNotCheck, false},
			{"orphans alone", RoleObservation{Kind: KindStale, Orphan: true}, ClassDead, true},
			{"a record outranks orphans", RoleObservation{Kind: KindStale, HasRecord: true, RecordClass: ClassAlive, Orphan: true}, ClassAlive, false},
			{"a running window is not planned", RoleObservation{Kind: KindUp, HasRecord: true, RecordClass: ClassDead}, ClassDead, false},
			{"absent", RoleObservation{Kind: KindAbsent}, "-", false},
		} {
			if got := tc.o.Class(); got != tc.class {
				t.Errorf("%s: class %q, want %q", tc.name, got, tc.class)
			}
			if got := tc.o.Plannable(); got != tc.plannable {
				t.Errorf("%s: plannable %v, want %v", tc.name, got, tc.plannable)
			}
			if state, class := tc.o.Preconditions(); state != tc.o.Kind || class != tc.class {
				t.Errorf("%s: preconditions (%s, %s)", tc.name, state, class)
			}
		}
	})

	t.Run("checkpoint writer", func(t *testing.T) {
		dir := t.TempDir()
		l := take(t, dir)
		if err := l.RunInteractive(context.Background(), "sample", "worker-desk", func(context.Context) Result { return Result{Uncertain: true} }); !errors.Is(err, ErrUnfinished) {
			t.Fatal(err)
		}
		s, err := Read(dir)
		if err != nil || !s.Running {
			t.Fatalf("setup: %+v %v", s, err)
		}
		mine := Generation{PID: os.Getpid(), Start: "s", Sequence: s.Sequence, LastStart: s.LastStart}
		for name, g := range map[string]Generation{
			"reused pid":       {PID: os.Getpid() + 1, Start: "s", Sequence: s.Sequence, LastStart: s.LastStart},
			"stale sequence":   {PID: os.Getpid(), Start: "s", Sequence: s.Sequence + 1, LastStart: s.LastStart},
			"stale start":      {PID: os.Getpid(), Start: "s", Sequence: s.Sequence, LastStart: s.LastStart.Add(time.Second)},
			"pid alone":        {PID: os.Getpid()},
			"unnamed":          {},
			"sequence, no pid": {Sequence: s.Sequence, LastStart: s.LastStart},
		} {
			err := l.RecordStopped(g, "x")
			if !errors.Is(err, ErrNotOwnersCheckpoint) {
				t.Errorf("%s: err %v, want ErrNotOwnersCheckpoint", name, err)
			}
			if after, err := Read(dir); err != nil || after != s {
				t.Errorf("%s: the refused write changed the checkpoint: %+v %v", name, after, err)
			}
		}
		if err := l.RecordStopped(mine, "stopped"); err != nil {
			t.Fatal(err)
		}
		if err := l.RecordStopped(mine, "again"); !errors.Is(err, ErrNotOwnersCheckpoint) {
			t.Fatalf("repeated stop of a finished record invented a transition: %v", err)
		}
	})
}

// --- the class guard ----------------------------------------------------------------------------

// The lifecycle class: a decision about whether a recorded role owner is the process now running,
// or whether a checkpoint is the one that owner wrote, made anywhere but the model. The guard
// parses the module's production source and reports a violation by rule and function, never by
// message, so a reworded error cannot hide a second site.
const (
	ruleProcessIdentity = "process-identity" // reads a start time without judging it through JudgeProcess
	ruleStopGeneration  = "stop-generation"  // finishes a checkpoint with no settle decision or a bare pid
	ruleCheckpointWrite = "checkpoint-writer"
	rulePlanClass       = "plan-class" // builds a role's class precondition outside planPreconditions
)

type violation struct{ Rule, Where string }

// retainedProcessSites read a start time at an effect boundary and keep their own check there
// (the brief's independent process-identity and fencing checks), or are the raw observation the
// model judges. Each needs a reason; an entry that no longer matches a site fails the guard.
var retainedProcessSites = map[string]string{
	"internal/cellstop/marked.go:withKernelStart": "records the kernel start time at listing; an unreadable one is could-not-check for the whole listing",
	"internal/cellstop/stop_unix.go:signalMarked": "re-reads the start time immediately before the signal and refuses on any mismatch (the signalling fence)",
	"cmd/cellctl/role_surface.go:Start":           "psTable.Start is the raw process-table observation verifySurface judges with JudgeProcess",
}

// checkpointWriters are the only functions of package cellcadence that write the checkpoint.
var checkpointWriters = map[string]bool{"Run": true, "RunInteractive": true, "RecordStopped": true}

func calls(fn *ast.FuncDecl) (names map[string]bool, all []*ast.CallExpr) {
	names = map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			all = append(all, c)
			switch f := c.Fun.(type) {
			case *ast.Ident:
				names[f.Name] = true
			case *ast.SelectorExpr:
				names[f.Sel.Name] = true
			}
		}
		return true
	})
	return names, all
}

func isBareOwnerPID(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.SelectorExpr:
		return x.Sel.Name == "PID" || x.Sel.Name == "Pid"
	case *ast.CallExpr:
		if s, ok := x.Fun.(*ast.SelectorExpr); ok {
			return s.Sel.Name == "Getpid"
		}
	}
	return false
}

// scanFile applies the guard's rules to one parsed file. rel is its module-relative path;
// inCadence marks package cellcadence's own source (where the checkpoint writer rule applies).
func scanFile(fset *token.FileSet, f *ast.File, rel string, inCadence bool, used map[string]bool) []violation {
	var out []violation
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		where := rel + ":" + fn.Name.Name
		names, all := calls(fn)
		for _, c := range all {
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			switch sel.Sel.Name {
			case "ProcessStart":
				if names["JudgeProcess"] {
					continue
				}
				if _, ok := retainedProcessSites[where]; ok {
					used[where] = true
					continue
				}
				out = append(out, violation{ruleProcessIdentity, where})
			case "RecordStopped":
				if len(c.Args) == 0 || isBareOwnerPID(c.Args[0]) || !names["JudgeSettle"] {
					out = append(out, violation{ruleStopGeneration, where})
				}
			}
		}
		if inCadence && names["write"] && !checkpointWriters[fn.Name.Name] {
			out = append(out, violation{ruleCheckpointWrite, where})
		}
		if inCadence && fn.Name.Name == "RecordStopped" && !names["checkOwnsUnfinished"] {
			out = append(out, violation{ruleCheckpointWrite, where + " (no owner generation check)"})
		}
		for _, c := range all {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "roleKey" && len(c.Args) == 2 {
				if lit, ok := c.Args[1].(*ast.BasicLit); ok && lit.Value == `"class"` && fn.Name.Name != "planPreconditions" {
					out = append(out, violation{rulePlanClass, where})
				}
			}
		}
	}
	return out
}

func parseGo(t *testing.T, path string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	return fset, f
}

func ruleSet(vs []violation) string {
	var rules []string
	for _, v := range vs {
		rules = append(rules, v.Rule+"@"+v.Where)
	}
	sort.Strings(rules)
	return strings.Join(rules, "\n")
}

func TestCellLifecycleClassGuard(t *testing.T) {
	root := filepath.Join("..", "..")

	t.Run("positive control: planted second sites", func(t *testing.T) {
		const rel = "testdata/lifecycle_plant/second_site.go"
		fset, f := parseGo(t, rel)
		got := scanFile(fset, f, rel, true, map[string]bool{})
		want := []violation{
			{ruleProcessIdentity, rel + ":secondSiteReusedPid"},
			{ruleStopGeneration, rel + ":secondSiteStaleGeneration"},
			{ruleCheckpointWrite, rel + ":secondSiteWriter"},
			{rulePlanClass, rel + ":secondSitePlan"},
		}
		if ruleSet(got) != ruleSet(want) {
			t.Fatalf("the guard must reject exactly the planted sites (the model-routed neighbour stays clean)\n got:\n%s\nwant:\n%s", ruleSet(got), ruleSet(want))
		}
	})

	t.Run("production tree has no second site", func(t *testing.T) {
		used := map[string]bool{}
		var got []violation
		for _, top := range []string{"cmd", "internal"} {
			err := filepath.WalkDir(filepath.Join(root, top), func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() && d.Name() == "testdata" {
					return fs.SkipDir
				}
				if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
					return nil
				}
				rel, _ := filepath.Rel(root, p)
				rel = filepath.ToSlash(rel)
				fset, f := parseGo(t, p)
				got = append(got, scanFile(fset, f, rel, strings.HasPrefix(rel, "internal/cellcadence/"), used)...)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if len(got) > 0 {
			t.Fatalf("a lifecycle decision or checkpoint write bypasses the model:\n%s", ruleSet(got))
		}
		for site, why := range retainedProcessSites {
			if !used[site] {
				t.Errorf("retained site %s (%s) no longer reads a start time; remove the entry", site, why)
			}
		}
	})
}
