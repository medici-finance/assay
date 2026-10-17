package main

// readledger_test.go — review finding refused-read-opens-write-breaker (class
// read-charges-write-budget, the same class as log-reads-charge-write-budget).
//
// The write verbs (dispatch, approve, retry) are gated by deskkit.AllowWrite(toolName, repo, 0),
// which walks the tool's audit lines for TWO meters: the rolling-hour budget (charged lines) and
// the circuit breaker (consecutive refused/unwritten lines, per repo and tool-wide). Keeping a
// read's lines off the budget is not enough: a read's REFUSAL is non-progress, so five refused
// reads on one repo (or twenty anywhere) used to open the write verbs' breaker. The class is
// "a read verb's audit line reaches a meter the write gate reads", and these tests close it for
// every read verb and every outcome a read records.

import (
	"bufio"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// readOutcome is one way a read verb can end, set up on a planted world.
type readOutcome struct {
	name  string
	args  []string
	want  string // the result EVERY invocation must record, exactly once, under the read key
	setup func(t *testing.T, w *world)
}

func failForge(err error) func(*testing.T, *world) {
	return func(t *testing.T, w *world) { w.fake.fail = err }
}

func failCustody(err error) func(*testing.T, *world) {
	return func(t *testing.T, w *world) {
		forgeForFn = func(deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
			return nil, deskkit.ForgeResolution{}, err
		}
		logForgeFn = func(deskkit.ForgeRepo, string) (deskkit.Forge, deskkit.ForgeResolution, error) {
			return nil, deskkit.ForgeResolution{}, err
		}
	}
}

// readOutcomes enumerates every way a read verb can end once its <owner/repo> has parsed into
// the desk-tools repo set — the point from which each read writes exactly ONE audit line (the
// argument errors before it name no repo to attribute a line to, and write none, as the write
// verbs' do). readVerbsUnderTest is the verb list; a verb with no row here fails the test.
func readOutcomes() []readOutcome {
	tracker := []string{"example-org/tracker", "501"}
	none := func(*testing.T, *world) {}
	ok, refused, ccc := deskkit.ResultOK, deskkit.ResultRefused, deskkit.ResultUnverifiable
	return []readOutcome{
		{"log/success", append([]string{"log"}, tracker...), ok, none},
		{"log/bad-run-id", []string{"log", "example-org/tracker", "../runs"}, ccc, none},
		{"log/no-loop", append([]string{"log"}, tracker...), refused, func(t *testing.T, w *world) { t.Setenv("DESK_LOOP", "") }},
		{"log/unknown-loop", append([]string{"log"}, tracker...), ccc, func(t *testing.T, w *world) { setLoop(t, "no-such-desk") }},
		{"log/role-refused", append([]string{"log"}, tracker...), refused, func(t *testing.T, w *world) { setLoop(t, "verify-desk") }},
		{"log/custody-refused", append([]string{"log"}, tracker...), refused, failCustody(deskkit.Refused("refused: no token provisioned"))},
		{"log/custody-unverifiable", append([]string{"log"}, tracker...), ccc, failCustody(errors.New("mint failed"))},
		{"log/forge-refused", append([]string{"log"}, tracker...), refused, failForge(deskkit.Refused("refused: the forge answered 403"))},
		{"log/forge-unverifiable", append([]string{"log"}, tracker...), ccc, failForge(errors.New("the forge answered 502"))},
		{"status/success", append([]string{"status"}, tracker...), ok, none},
		{"status/bad-run-id", []string{"status", "example-org/tracker", "../runs"}, ccc, none},
		{"status/human-bound", []string{"status", "example-org/console", "501"}, refused, none},
		{"status/unbound", []string{"status", "example-org/agents", "501"}, ccc, none},
		{"status/custody-refused", append([]string{"status"}, tracker...), refused, failCustody(deskkit.Refused("refused: no token provisioned"))},
		{"status/custody-unverifiable", append([]string{"status"}, tracker...), ccc, failCustody(errors.New("mint failed"))},
		{"status/forge-refused", append([]string{"status"}, tracker...), refused, failForge(deskkit.Refused("refused: the forge answered 403"))},
		{"status/forge-unverifiable", append([]string{"status"}, tracker...), ccc, failForge(errors.New("the forge answered 502"))},
	}
}

// TestDeskrunReadsLeaveWriteGate is the class guard. Each read outcome is repeated past BOTH
// breaker trips (the per-repo run and the tool-wide backstop), on the wall clock the meter reads;
// then the write gate must still be clear and retry and dispatch must still reach the backend.
// It also pins the other half: every invocation, whatever its outcome, writes exactly one line
// under the read key carrying that outcome's result — so the fix cannot be "record nothing",
// and a path that records nothing (or twice) is red here, named by its row.
func TestDeskrunReadsLeaveWriteGate(t *testing.T) {
	covered := map[string]bool{}
	for _, o := range readOutcomes() {
		covered[o.args[0]] = true
		t.Run(o.name, func(t *testing.T) {
			w := plantWorld(t, deskkit.ForgeGitHub)
			nowFunc = time.Now // the meter reads wall time; stamp the ledger on the same clock
			keepForge, keepLog := forgeForFn, logForgeFn
			o.setup(t, w)
			for i := 0; i < deskkit.BreakerBackstopTrip; i++ {
				runArgs(t, o.args...)
			}
			forgeForFn, logForgeFn, w.fake.fail = keepForge, keepLog, nil
			setLoop(t, "worker-desk")

			if err := deskkit.AllowWrite(toolName, "example-org/tracker", 0); err != nil {
				t.Fatalf("after %d reads (%s) the write gate is shut: %v", deskkit.BreakerBackstopTrip, o.name, err)
			}
			if code, _, msg := runArgs(t, "retry", "example-org/tracker", "9001"); code != deskkit.ExitOK || len(w.fake.retries) != 1 {
				t.Fatalf("retry after reads (%s): exit %d (%s), retries %d", o.name, code, msg, len(w.fake.retries))
			}
			if code, _, msg := runArgs(t, "example-org/tracker", "release.yml", "--ref", "main"); code != deskkit.ExitOK || len(w.fake.runs) != 1 {
				t.Fatalf("dispatch after reads (%s): exit %d (%s), runs %d", o.name, code, msg, len(w.fake.runs))
			}

			inWrite, recorded := readLines(t)
			if len(inWrite) > 0 {
				t.Fatalf("%s left read line(s) %v in the write verbs' bucket (%s)", o.name, inWrite, toolName)
			}
			if len(recorded) != deskkit.BreakerBackstopTrip {
				t.Fatalf("%s: %d invocations wrote %d read line(s) %v — every read outcome must write exactly one",
					o.name, deskkit.BreakerBackstopTrip, len(recorded), recorded)
			}
			for _, r := range recorded {
				if r != o.want {
					t.Fatalf("%s: recorded %v, want every line %q", o.name, recorded, o.want)
				}
			}
		})
	}
	for _, v := range readVerbsUnderTest {
		if !covered[v] {
			t.Errorf("read verb %q has no outcome row in readOutcomes", v)
		}
	}
}

// readLines scans this test's ledger: every read-verb line whose tool key resolves to the write
// verbs' bucket ("verb:result"), and the result of every read-verb line under the read key.
func readLines(t *testing.T) (inWrite, recorded []string) {
	t.Helper()
	f, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer f.Close()
	reads := map[string]bool{}
	for _, v := range readVerbsUnderTest {
		reads[v] = true
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e struct{ Tool, Verb, Result string }
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("ledger line %q: %v", sc.Text(), err)
		}
		if !reads[e.Verb] {
			continue
		}
		if e.Tool == deskkit.DeskrunReadTool {
			recorded = append(recorded, e.Result)
		}
		if deskkit.CanonicalToolKeyOr(e.Tool) == toolName {
			inWrite = append(inWrite, e.Verb+":"+e.Result)
		}
	}
	sort.Strings(inWrite)
	return inWrite, recorded
}

// TestReadVerbSetMatchesCode is the structural half of the class guard, over the package's
// non-test source. A verb is a read exactly when its cmd function never calls a write gate
// (deskkit.AllowWrite*), and the read set (readVerbs) must be exactly those verbs — so a new
// read verb that forgets to register, or a write verb registered as a read, goes red here. And
// the audit ledger is reached ONLY through auditLine, the one place that routes a read verb's
// line out of the write verbs' bucket; any other deskkit.Log call fails, naming its site. And every
// read verb's function starts a readTrail and DEFERS its finish, the shape that gives each read
// invocation its one audit line whatever return it ends on — a read verb that records by hand at
// each return instead fails here before a new unrecorded return can reach review.
func TestReadVerbSetMatchesCode(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}
	gated := map[string]bool{}   // verb -> its cmd function calls a write gate
	trailed := map[string]bool{} // verb -> its cmd function starts a readTrail and defers finish
	var stray []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				verb, gate, starts, defers := "", false, false, false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.CallExpr:
						if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "startReadTrail" {
							starts = true
						}
					case *ast.DeferStmt:
						ast.Inspect(x.Call, func(m ast.Node) bool {
							if sel, ok := m.(*ast.SelectorExpr); ok && sel.Sel.Name == "finish" {
								defers = true
							}
							return true
						})
					case *ast.ValueSpec:
						if len(x.Names) == 1 && x.Names[0].Name == "verb" && len(x.Values) == 1 {
							if lit, ok := x.Values[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								verb = strings.Trim(lit.Value, `"`)
							}
						}
					case *ast.SelectorExpr:
						if id, ok := x.X.(*ast.Ident); ok && id.Name == "deskkit" {
							if strings.HasPrefix(x.Sel.Name, "AllowWrite") {
								gate = true
							}
							if x.Sel.Name == "Log" && fn.Name.Name != "auditLine" {
								stray = append(stray, fset.Position(x.Pos()).String()+" in "+fn.Name.Name)
							}
						}
					}
					return true
				})
				if verb != "" {
					gated[verb] = gated[verb] || gate
					trailed[verb] = trailed[verb] || (starts && defers)
				}
			}
		}
	}
	for _, s := range stray {
		t.Errorf("deskkit.Log called outside auditLine at %s — a ledger write that bypasses the read routing", s)
	}
	if len(gated) == 0 {
		t.Fatal("found no `const verb` in any function — the scan matched nothing, so it proves nothing")
	}
	for verb, g := range gated {
		if !g && !readVerbs[verb] {
			t.Errorf("verb %q calls no write gate but is not in readVerbs — its audit lines land in the write verbs' bucket", verb)
		}
		if g && readVerbs[verb] {
			t.Errorf("verb %q calls a write gate but is registered in readVerbs — its writes would leave the write bucket", verb)
		}
	}
	for verb := range readVerbs {
		if _, ok := gated[verb]; !ok {
			t.Errorf("readVerbs names %q, but no function declares that verb", verb)
		}
		if !trailed[verb] {
			t.Errorf("read verb %q does not start a readTrail and defer its finish — a return path could end with no audit line", verb)
		}
	}
	var want []string
	for v := range readVerbs {
		want = append(want, v)
	}
	sort.Strings(want)
	got := append([]string(nil), readVerbsUnderTest...)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("readVerbsUnderTest = %v, readVerbs = %v — the behavioural guard must cover every read verb", got, want)
	}
}
