package main

// The verb-level controls for the pooled sweeps and the per-root pool: byte-identity against a
// serial (limit=1) run, the failing repo NAMED from either end of the roster, the lowest-index
// root's error winning when two roots fail, one root resolution per `throughput` run, and the
// stages-read count on both of throughput's partial-failure paths.
//
// The pool primitives already have their own unit tests (sweep_test.go). What those cannot
// show is that each CALLER merges the pool's results in input order and renders the same
// bytes whatever order the workers finished in; that is what this file drives, through the
// real verb functions, with the finish order deliberately scrambled.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// poolSeed perturbs every jittered read. It is written only between runs, never while a
// sweep is in flight, so the workers that read it share no mutable state with the writer.
var poolSeed int

// poolJitter sleeps 0-3 ms, chosen by hashing (seed, key). Each run uses a different seed,
// so each run gets a different finish order — which is the only thing that can expose a
// merge that depends on finish order rather than on input order.
func poolJitter(key string) {
	h := fnv.New32a()
	fmt.Fprintf(h, "%d|%s", poolSeed, key)
	time.Sleep(time.Duration(h.Sum32()%4) * time.Millisecond)
}

// jitterForge is listFnForge with a jittered visibility read and a per-repo visibility
// failure set, so the drift probe's pooled reads are scrambled too.
type jitterForge struct {
	listFnForge
	failVis map[string]bool
}

func (f *jitterForge) RepoVisibility(fr deskkit.ForgeRepo) (string, error) {
	poolJitter("vis|" + f.repo)
	if f.failVis[f.repo] {
		return "", deskkit.Unverifiable("cannot read repo metadata for "+f.repo, nil)
	}
	return f.listFnForge.RepoVisibility(fr)
}

// poolHead is the head sha of PR num in roster position idx.
func poolHead(idx, num int) string { return fmt.Sprintf("%02x%05x0", idx, num) }

// poolChanges is one repo's open-PR list for the pooled-output fixture: a repo-dependent
// number of PRs, mixed drafts and non-drafts, one untrusted author (an `external` row in
// `prs`), and — for `truncated` — a full page at the listing cap.
func poolChanges(idx int, truncated bool) *deskkit.OpenChanges {
	n := 1 + idx%4
	if truncated {
		n = prListLimit
	}
	oc := &deskkit.OpenChanges{Cap: prListLimit, TruncatedAtCap: truncated}
	for k := 1; k <= n; k++ {
		num := 100*idx + k
		author := stalledAuthorGH
		if k == 3 {
			author = "mallory" // untrusted → quarantined in prs; non-draft, so stalled skips it
		}
		oc.Changes = append(oc.Changes, deskkit.OpenChange{
			Number: num, Title: fmt.Sprintf("change %d", num), State: "OPEN", Draft: k%2 == 0,
			Author: deskkit.Account{Login: author}, HeadSHA: poolHead(idx, num),
			HeadRef: fmt.Sprintf("feat/x-%d", num), MergeStateStatus: "BLOCKED",
			Rollup: []deskkit.RollupNode{{Typename: "CheckRun", Name: "ci", Status: "COMPLETED", Conclusion: "SUCCESS"}},
		})
	}
	return oc
}

// installPoolFixture wires the jittered forge over the fixture roster. outOfInstall names
// the roster positions whose open-PR read fails as outside the App installation (the `prs`
// carve-out; `stalled` has none, so its run passes an empty set). It returns the scope.
func installPoolFixture(t *testing.T, outOfInstall map[int]bool) []string {
	t.Helper()
	scope := deskkit.AllowedRepos()
	if len(scope) < 10 {
		t.Fatalf("fixture roster has %d repos; this control needs at least 10", len(scope))
	}
	idxOf := make(map[string]int, len(scope))
	for i, r := range scope {
		idxOf[r] = i
	}
	const truncIdx = 2
	list := func(repo string) (*deskkit.OpenChanges, error) {
		poolJitter("list|" + repo)
		i := idxOf[repo]
		if outOfInstall[i] {
			return nil, outOfInstallationErr(repo)
		}
		return poolChanges(i, i == truncIdx), nil
	}
	failVis := map[string]bool{scope[5]: true, scope[9]: true}
	prev := forgeFor
	t.Cleanup(func() { forgeFor = prev })
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, ok := strings.Cut(repo, "/")
		if !ok {
			return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
		}
		return &jitterForge{listFnForge: listFnForge{fakeForge: fakeForge{repo: repo}, list: list}, failVis: failVis},
			deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}

	// The stalled per-PR reads, keyed off the same (repo, number) the list served. Every
	// commit time sits at the MIDDLE of daysRounded's 6-minute bucket, so the few seconds
	// between the serial run and the last pooled run cannot move a rounded value.
	t0 := time.Now()
	forgeHooks.reviews = func(repo string, num int) ([]deskkit.Review, error) {
		poolJitter(fmt.Sprintf("rev|%s|%d", repo, num))
		if num%7 == 5 {
			return nil, errors.New("HTTP 502 on the reviews read") // → an unassessable row
		}
		return []deskkit.Review{{
			Author: deskkit.Account{Login: stalledOtherREST}, State: "CHANGES_REQUESTED",
			CommitID: poolHead(idxOf[repo], num), SubmittedAt: "2026-08-01T00:00:00Z",
		}}, nil
	}
	forgeHooks.getCommit = func(repo, sha string) (*deskkit.RepoCommit, error) {
		poolJitter("commit|" + repo + "|" + sha)
		var idx, num int
		fmt.Sscanf(sha, "%02x%05x", &idx, &num)
		at := t0.Add(-time.Duration(3+num%9)*24*time.Hour - 3*time.Minute)
		return &deskkit.RepoCommit{SHA: sha, CommittedDate: at.UTC().Format(time.RFC3339),
			CommitterLogin: stalledAuthorREST}, nil
	}
	forgeHooks.comments = func(repo string, num int) ([]deskkit.Comment, error) {
		poolJitter(fmt.Sprintf("com|%s|%d", repo, num))
		return []deskkit.Comment{}, nil
	}
	forgeHooks.compare = func(repo, base, head string) (*deskkit.RefComparison, error) {
		poolJitter("cmp|" + repo + "|" + head)
		var idx, num int
		fmt.Sscanf(head, "%02x%05x", &idx, &num)
		return &deskkit.RefComparison{Status: "behind", BehindBy: (num * 37) % 400}, nil
	}
	// One configured-public repo reports private, so the drift list carries a real mismatch
	// beside the two NOT OBSERVED entries.
	pub := publicRepos()
	if len(pub) > 0 {
		pub = pub[1:]
	}
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(pub, " "))
	return scope
}

// reportBytes is everything a verb emits: the rendered table, the indented JSON exactly as
// main.go encodes it, and the audit detail.
func reportBytes(t *testing.T, rep *Report) string {
	t.Helper()
	var b bytes.Buffer
	rep.render(&b)
	b.WriteString("\n--json--\n")
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep.value); err != nil {
		t.Fatal(err)
	}
	b.WriteString("--detail--\n" + rep.detail)
	return b.String()
}

func driftBytes(t *testing.T, a policyDriftAlarm) string {
	t.Helper()
	var b bytes.Buffer
	a.renderAlarms(&b)
	js, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return b.String() + string(js) + "\n" + a.auditDetail()
}

// TestPooledOutputMatchesSerial — Verify row 3. `prs`, `stalled` and the drift probe, run
// on the production pool width over a fixture whose finish order is scrambled differently on
// every repeat, emit exactly the bytes (table, JSON and audit detail) a limit=1 run of the
// same fixture emits. 20 repeats, so a finish-order dependence cannot pass by luck.
//
// The `prs` fixture includes two out-of-installation repos and a repo at the listing cap:
// the coverage `unreadable` list and the truncated-repo list are the two places in that
// verb's merge that are NOT re-sorted afterwards, so a finish-order merge shows up there.
func TestPooledOutputMatchesSerial(t *testing.T) {
	if sweepConcurrency < 2 || boardSweepLimit != sweepConcurrency {
		t.Fatalf("boardSweepLimit=%d sweepConcurrency=%d: the pooled run must be the production width (>1)",
			boardSweepLimit, sweepConcurrency)
	}
	hdr := Header{AsOf: "2026-01-01T00:00:00Z"}
	const repeats = 20

	verbs := []struct {
		name         string
		outOfInstall map[int]bool
		emit         func(t *testing.T) string
	}{
		{"prs", map[int]bool{1: true, 4: true}, func(t *testing.T) string {
			rep, err := cmdPRs(hdr)
			if err != nil {
				t.Fatalf("prs: %v", err)
			}
			return reportBytes(t, rep)
		}},
		{"stalled", nil, func(t *testing.T) string {
			rep, err := cmdStalled(hdr, 0)
			if err != nil {
				t.Fatalf("stalled: %v", err)
			}
			return reportBytes(t, rep)
		}},
		{"policydrift", nil, func(t *testing.T) string {
			return driftBytes(t, assessPolicyDrift())
		}},
	}
	for _, v := range verbs {
		t.Run(v.name, func(t *testing.T) {
			installFakeGH(t)
			installPoolFixture(t, v.outOfInstall)

			boardSweepLimit = 1
			poolSeed = -1
			serial := v.emit(t)
			boardSweepLimit = sweepConcurrency
			t.Cleanup(func() { boardSweepLimit = sweepConcurrency })

			// The fixture must actually exercise each part of the output a scrambled merge
			// could disturb; a vacuous fixture would make byte-equality prove nothing.
			for _, want := range map[string][]string{
				"prs":         {`"unreadable"`, "TRUNCATED", `"external"`, `"draft": true`},
				"stalled":     {"UNASSESSABLE", "close-candidate", "shepherd", "TRUNCATED"},
				"policydrift": {"NOT OBSERVED", "POLICY-DRIFT"},
			}[v.name] {
				if !strings.Contains(serial, want) {
					t.Fatalf("serial %s output lacks %q — the fixture no longer exercises it:\n%s", v.name, want, serial)
				}
			}

			for i := 0; i < repeats; i++ {
				poolSeed = i
				if got := v.emit(t); got != serial {
					t.Fatalf("pooled %s run %d differs from the limit=1 run\n--- serial ---\n%s\n--- pooled ---\n%s",
						v.name, i, serial, got)
				}
			}
		})
	}
}

// TestPooledSweepNamesFailingRepo — Verify row 2, the SPOF row's negative control. One repo's
// open-PR read fails with an error that does NOT itself carry the repo name, so it is the
// verb's own wrapping that has to name it. `prs` and `stalled` must each exit 6, name THAT
// repo, and print no board at all — with the failing repo first in roster order, last in
// roster order, and both at once (the lowest index wins, even when it fails last in time).
func TestPooledSweepNamesFailingRepo(t *testing.T) {
	const bare = "HTTP 502: simulated read failure" // names no repo on purpose
	for _, verb := range []string{"prs", "stalled"} {
		scope := deskkit.AllowedRepos()
		first, last := scope[0], scope[len(scope)-1]
		cases := []struct {
			name    string
			failing []string
			named   string
			absent  string
		}{
			{"first", []string{first}, first, ""},
			{"last", []string{last}, last, ""},
			{"both", []string{first, last}, first, last},
		}
		for _, c := range cases {
			t.Run(verb+"/"+c.name, func(t *testing.T) {
				installFakeGH(t)
				fail := map[string]bool{}
				for _, r := range c.failing {
					fail[r] = true
				}
				stubForgeList(t, func(repo string) (*deskkit.OpenChanges, error) {
					if fail[repo] {
						if repo == first && len(c.failing) > 1 {
							time.Sleep(40 * time.Millisecond) // the lowest index fails LAST in time
						}
						return nil, errors.New(bare)
					}
					return poolChanges(1, false), nil
				})

				var out, errb bytes.Buffer
				code := run([]string{verb, "--json"}, &out, &errb)
				if code != deskkit.ExitUnverifiable {
					t.Fatalf("%s exited %d with %v unreadable, want %d (fail-closed)\nstdout=%s",
						verb, code, c.failing, deskkit.ExitUnverifiable, out.String())
				}
				if out.Len() != 0 {
					t.Errorf("%s printed a board although a repo could not be read — a partial board reads as the board:\n%s",
						verb, out.String())
				}
				if !strings.Contains(errb.String(), c.named) {
					t.Errorf("%s failed without naming %s:\n%s", verb, c.named, errb.String())
				}
				if c.absent != "" && strings.Contains(errb.String(), c.absent) {
					t.Errorf("%s named %s, a HIGHER-index failure — the lowest-index repo's error must win:\n%s",
						verb, c.absent, errb.String())
				}
			})
		}
	}
}

// statusgenWrap is a shim placed in front of the fake statusgen: it logs each invocation's
// argv (one line per call) and, for a root matching FAKE_SG_SLOW_MATCH, sleeps first — so the
// per-root pool finishes out of configured order.
const statusgenWrap = `#!/bin/sh
echo "$*" >> "$FAKE_SG_ARGLOG"
if [ -n "$FAKE_SG_SLOW_MATCH" ]; then
  case "$*" in *"$FAKE_SG_SLOW_MATCH"*) sleep 0.3 ;; esac
fi
exec "$FAKE_SG_REAL" "$@"
`

// installCountingStatusgen puts statusgenWrap in front of the fake statusgen and returns
// the argv log path.
func installCountingStatusgen(t *testing.T) string {
	t.Helper()
	real := installFakeStatusgen(t)
	dir := t.TempDir()
	wrap := filepath.Join(dir, "statusgen-wrap")
	if err := os.WriteFile(wrap, []byte(statusgenWrap), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "argv.log")
	t.Setenv("FAKE_SG_REAL", real)
	t.Setenv("FAKE_SG_ARGLOG", log)
	t.Setenv(statusgenBinEnv, wrap)
	return log
}

// argvLines returns the logged invocations, or none if statusgen never ran.
func argvLines(t *testing.T, log string) []string {
	t.Helper()
	b, err := os.ReadFile(log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func countPrefix(lines []string, prefix string) int {
	n := 0
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

// fiveRoots configures five roots (two pool waves at rootConcurrency=4), in the order
// ConfiguredRoots returns them (sorted by repo). The tracker root carries the pin. A root
// whose index is in fail gets "-fail-" in its directory name, which FAKE_SG_FAIL_MATCH
// can then select without touching the others.
func fiveRoots(t *testing.T, fail map[int]bool) []string {
	t.Helper()
	repos := []string{"example-org/agents", "example-org/console", "example-org/examples",
		"example-org/tracker", "medici-finance/assay"}
	var spec, paths []string
	for i, r := range repos {
		name := fmt.Sprintf("root%d-%s", i, strings.ReplaceAll(r, "/", "-"))
		if fail[i] {
			name = fmt.Sprintf("root%d-fail-%s", i, strings.ReplaceAll(r, "/", "-"))
		}
		p := makeRoot(t, name, r == "example-org/tracker")
		spec = append(spec, r+"="+p)
		paths = append(paths, p)
	}
	t.Setenv(deskkit.RootsEnv, strings.Join(spec, ","))
	return paths
}

// TestPerRootPoolMatchesSerial — Verify row 5, serial-equality half. `dispatch` and
// `awaiting` read five roots on the production per-root pool, with the FIRST root made slow
// so it finishes last; the rows, their order, and every other byte of the table, JSON and
// detail equal a rootPoolLimit=1 run of the same roots.
func TestPerRootPoolMatchesSerial(t *testing.T) {
	if rootConcurrency < 2 || rootPoolLimit != rootConcurrency {
		t.Fatalf("rootPoolLimit=%d rootConcurrency=%d: the pooled run must be the production width (>1)",
			rootPoolLimit, rootConcurrency)
	}
	hdr := Header{AsOf: "2026-01-01T00:00:00Z"}
	for _, verb := range []string{"dispatch", "awaiting"} {
		t.Run(verb, func(t *testing.T) {
			installFakeGH(t)
			installCountingStatusgen(t)
			paths := fiveRoots(t, nil)
			t.Setenv("FAKE_SG_SLOW_MATCH", filepath.Base(paths[0]))
			emit := func() string {
				var rep *Report
				var err error
				if verb == "dispatch" {
					rep, err = cmdDispatch(hdr, verb)
				} else {
					rep, err = cmdAwaiting(hdr, verb)
				}
				if err != nil {
					t.Fatalf("%s: %v", verb, err)
				}
				return reportBytes(t, rep)
			}
			rootPoolLimit = 1
			serial := emit()
			rootPoolLimit = rootConcurrency
			t.Cleanup(func() { rootPoolLimit = rootConcurrency })
			for _, p := range paths {
				if !strings.Contains(serial, filepath.Base(p)+"-stream/") {
					t.Fatalf("serial %s output carries no row from root %s — the fixture is vacuous:\n%s", verb, p, serial)
				}
			}
			if got := emit(); got != serial {
				t.Fatalf("pooled %s differs from the rootPoolLimit=1 run\n--- serial ---\n%s\n--- pooled ---\n%s",
					verb, serial, got)
			}
		})
	}
}

// TestPerRootPoolLowestIndexErr — Verify row 5, two-failure half. Two roots fail; the
// lower-index one is made slow, so it fails LAST in time. Each verb must still exit
// non-zero naming the lower-index root, and must not name the other.
func TestPerRootPoolLowestIndexErr(t *testing.T) {
	for _, verb := range []string{"dispatch", "awaiting"} {
		t.Run(verb, func(t *testing.T) {
			installFakeGH(t)
			installCountingStatusgen(t)
			paths := fiveRoots(t, map[int]bool{1: true, 3: true})
			lowF, highF := paths[1], paths[3]
			t.Setenv("FAKE_SG_FAIL_MATCH", "-fail-")
			t.Setenv("FAKE_SG_SLOW_MATCH", filepath.Base(lowF)) // the lower index fails LAST in time

			var out, errb bytes.Buffer
			code := run([]string{verb}, &out, &errb)
			if code == deskkit.ExitOK {
				t.Fatalf("%s exited 0 with two roots unreadable\nstdout=%s", verb, out.String())
			}
			msg := errb.String() + out.String()
			if !strings.Contains(msg, lowF) {
				t.Errorf("%s did not name the LOWER-index failing root %s:\n%s", verb, lowF, msg)
			}
			if strings.Contains(msg, highF) {
				t.Errorf("%s named the higher-index root %s, which failed FIRST in time — "+
					"which error surfaces must not depend on scheduling:\n%s", verb, highF, msg)
			}
		})
	}
}

// TestThroughputResolvesOnce — Verify row 6. A counting shim in front of statusgen records
// every invocation. One `throughput` run spawns exactly ONE `--version` probe (the probe is
// made only by resolveRootsOnce, after the pin walk — TestRootProbesOnlyInResolver pins
// that), and exactly one `--next-up` and one `--gate-scores` per root. The control is the
// same counter over `dispatch` then `awaiting` run separately: two probes, so the counter is
// live and a second resolution would be seen.
func TestThroughputResolvesOnce(t *testing.T) {
	throughputFixture(t)
	log := installCountingStatusgen(t)

	runThroughput(t)
	lines := argvLines(t, log)
	if got := countPrefix(lines, "--version"); got != 1 {
		t.Errorf("one throughput run made %d statusgen --version probes, want exactly 1 (one root resolution)\n%s",
			got, strings.Join(lines, "\n"))
	}
	if got := countPrefix(lines, "--next-up"); got != 2 {
		t.Errorf("--next-up ran %d times over two roots, want 2:\n%s", got, strings.Join(lines, "\n"))
	}
	if got := countPrefix(lines, "--gate-scores"); got != 2 {
		t.Errorf("--gate-scores ran %d times over two roots, want 2:\n%s", got, strings.Join(lines, "\n"))
	}

	// Control: the counter sees a second resolution when there is one.
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"dispatch", "awaiting"} {
		var out, errb bytes.Buffer
		if code := run([]string{verb}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("%s exited %d: %s", verb, code, errb.String())
		}
	}
	if got := countPrefix(argvLines(t, log), "--version"); got != 2 {
		t.Fatalf("control: dispatch + awaiting made %d --version probes, want 2 — the counter is not live", got)
	}
}

// TestRootProbesOnlyInResolver — Verify row 6, structural half. The roster read, the pin walk
// and the version probe each have exactly ONE call site in this package's non-test source,
// inside resolveRootsOnce; and cmdThroughput calls resolveRootsOnce exactly once. Together
// with the runtime probe count above, one `throughput` run is one resolution, one pin walk
// and one version probe.
func TestRootProbesOnlyInResolver(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	type site struct{ fn, file string }
	calls := map[string][]site{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, perr := parser.ParseFile(fset, f, nil, 0)
		if perr != nil {
			t.Fatal(perr)
		}
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				ce, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var name string
				switch fn := ce.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				case *ast.IndexExpr: // generic instantiation
					if id, ok := fn.X.(*ast.Ident); ok {
						name = id.Name
					}
				}
				switch name {
				case "ConfiguredRoots", "resolveStatusgenPin", "statusgenVersionOf", "resolveRootsOnce":
					calls[name] = append(calls[name], site{fd.Name.Name, f})
				}
				return true
			})
		}
	}
	for _, name := range []string{"ConfiguredRoots", "resolveStatusgenPin", "statusgenVersionOf"} {
		got := calls[name]
		if len(got) != 1 || got[0].fn != "resolveRootsOnce" {
			t.Errorf("%s call sites = %v, want exactly one, inside resolveRootsOnce", name, got)
		}
	}
	n := 0
	for _, s := range calls["resolveRootsOnce"] {
		if s.fn == "cmdThroughput" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("cmdThroughput calls resolveRootsOnce %d times, want exactly 1 (sites: %v)", n, calls["resolveRootsOnce"])
	}
}

// renderThroughputTable runs throughput and returns its table rendering and JSON report.
func renderThroughputTable(t *testing.T) (string, throughputReport) {
	t.Helper()
	var out, errb bytes.Buffer
	if code := run([]string{"throughput", "--table"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("throughput --table exited %d: %s", code, errb.String())
	}
	return out.String(), runThroughput(t)
}

// stageLine returns the table line for one stage.
func stageLine(t *testing.T, table, name string) string {
	t.Helper()
	for _, l := range strings.Split(table, "\n") {
		if strings.HasPrefix(l, name+" ") {
			return l
		}
	}
	t.Fatalf("no %s line in the table:\n%s", name, table)
	return ""
}

// TestThroughputSharedBlindCount — Verify row 7, shared-failure half. With the shared root
// resolution failing, dispatch and verify are both blind, neither renders a depth (the
// table says n/a, never 0), neither is the bottleneck, and the report counts only the
// stages actually READ: review alone, so `read 1 of 4 stages`.
func TestThroughputSharedBlindCount(t *testing.T) {
	throughputFixture(t)
	t.Setenv(deskkit.RootsEnv, "example-org/tracker=/no/such/root")

	table, rep := renderThroughputTable(t)
	for _, name := range []string{"dispatch", "verify"} {
		s := stage(t, rep, name)
		if s.Depth != nil || s.Ratio != nil {
			t.Errorf("%s has a depth/ratio although its read never ran: %+v", name, s)
		}
		if !strings.Contains(s.Blind, "shared stream-root resolution failed") {
			t.Errorf("%s blind reason does not name the shared resolution: %q", name, s.Blind)
		}
		if f := strings.Fields(stageLine(t, table, name)); len(f) < 3 || f[2] != "n/a" {
			t.Errorf("%s table DEPTH column is %v, want n/a — a blind stage never renders as a number", name, f)
		}
		if rep.Bottleneck == name {
			t.Errorf("blind stage %s was chosen as the bottleneck", name)
		}
	}
	if s := stage(t, rep, "review"); s.Depth == nil {
		t.Fatalf("review should still be read on this path: %+v", s)
	}
	if rep.StagesRead != 1 || rep.StagesTotal != 4 {
		t.Errorf("stagesRead/stagesTotal = %d/%d, want 1/4 (review only)", rep.StagesRead, rep.StagesTotal)
	}
	if !strings.Contains(table, "read 1 of 4 stages") {
		t.Errorf("table does not state `read 1 of 4 stages`:\n%s", table)
	}
}

// TestThroughputActionsOnlyBlind — Verify row 7, actions-only half. With only the `actions`
// read failing (one repo's open-PR read), the review stage alone goes blind; dispatch and
// verify still report depths, and the report counts 2 of 4 stages read.
func TestThroughputActionsOnlyBlind(t *testing.T) {
	throughputFixture(t)
	t.Setenv("DESKBOARD_GH_FAIL_REPO", "example-org/agents")

	table, rep := renderThroughputTable(t)
	rv := stage(t, rep, "review")
	if rv.Depth != nil || rv.Blind == "" {
		t.Errorf("review should be blind when actions fails: %+v", rv)
	}
	if f := strings.Fields(stageLine(t, table, "review")); len(f) < 3 || f[2] != "n/a" {
		t.Errorf("review table DEPTH column is %v, want n/a", f)
	}
	if rep.Bottleneck == "review" {
		t.Errorf("blind review stage was chosen as the bottleneck")
	}
	for _, name := range []string{"dispatch", "verify"} {
		if s := stage(t, rep, name); s.Depth == nil || s.Blind != "" {
			t.Errorf("%s should still report a depth when only actions failed: %+v", name, s)
		}
	}
	if rep.StagesRead != 2 || rep.StagesTotal != 4 {
		t.Errorf("stagesRead/stagesTotal = %d/%d, want 2/4", rep.StagesRead, rep.StagesTotal)
	}
	if !strings.Contains(table, "read 2 of 4 stages") {
		t.Errorf("table does not state `read 2 of 4 stages`:\n%s", table)
	}
}
