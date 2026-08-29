package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ciDefaultBranch is the base branch an ordinary PR targets and the branch main's
// push half fires on. A `branches:` filter that excludes it makes the whole
// workflow unreachable for everyday work, however perfect its `paths:` are.
const ciDefaultBranch = "main"

// ciJobRef identifies the CI job that must run a registry entry's module for one
// event. Unlike the single display string it replaces, every field here is
// ASSERTED against the workflow (desk review of #205).
type ciJobRef struct {
	id          string // jobs.<id> key that must exist in the workflow
	matrixValue string // if set, must still be in that job's strategy.matrix (this is what renders the "(tools/desk)" suffix)
	check       string // check name GitHub renders, for failure messages
	note        string // optional caveat, printed on failure
}

// ciEntry is one row of the hand-maintained cross-module-reader registry.
type ciEntry struct {
	test     string   // repo-relative test file that reads out of its module (or, for a runInvokes row, the command's source)
	module   string   // repo-relative Go module dir the job must `go test` (or, for a runInvokes row, a module the gate runs)
	workflow string   // repo-relative workflow whose triggers must reach it
	prJob    ciJobRef // job that runs it on pull_request
	pushJob  ciJobRef // job that runs it on push
	reads    []string // repo-relative paths the test reads off disk
	// runInvokes, when non-empty, switches half (1)'s LAST check from "the job
	// runs `cd <module> && go test`" to "the job body contains each of these
	// command fragments verbatim". It is for gates that run a BINARY, not a test
	// (#607's tree-sweep runs `go run ./tools/pubmanifest stage`
	// then `go run ./tools/leaksweep run --tree`, neither a `go test`). Every
	// other half-(1) check — branches, job-exists, matrix, if-gate — and the
	// whole of half (2) still apply unchanged.
	runInvokes []string
	why        string
}

// TestCrossModuleTestsAreTriggeredByWhatTheyRead closes the class of hole that
// #199 recorded.
//
// THE HOLE. A handful of tests in this repo deliberately read a file that lives
// OUTSIDE their own Go module — scancoupling_test.go (this package) reads
// statusgen/{trustgate,scanissues}.go; issueboard/ownedrepos_coupling_test.go
// AST-parses statusgen/; statusgen/version_test.go reads
// .github/workflows/release-statusgen.yml. That is the point: a comment is not
// an enforcement, a failing test is. But GitHub path filters are usually scoped
// to the directory a suite LIVES in, and a guard whose trigger does not cover
// what it READS is advisory, not enforced — the change it exists to catch is
// exactly the change that does not run it.
//
// It fired for real: PR #193 touched only statusgen/**, so `test (tools/desk)`
// never ran, the scan-issues trust-gate tripwire never evaluated, and `main`
// merged RED. #198 then inherited a red it had not caused, and #202's tripwire
// went unexercised for the same reason.
//
// THE GUARD, IN TWO HALVES. A path filter is NECESSARY BUT NOT SUFFICIENT. For
// each registry entry, and for each of the two events (`pull_request`, `push`):
//
//	(1) REACHABILITY — the workflow must actually run a job that runs this
//	    test's module for that event: the event must not be fenced off by a
//	    `branches:`/`branches-ignore:` filter that excludes `main`; the named
//	    job must exist; if the job is matrix-generated, the module must still be
//	    in the matrix; the job's own `if:` must not exclude the event; and the
//	    job must still invoke `go test` in that module.
//	(2) COVERAGE — every path the test reads must be matched by at least one
//	    glob in that event's `paths:` filter.
//
// Half (2) alone was the original guard, and half (2) alone can certify a job
// that will never run: deleting `tools/desk` from tools.yml's module matrix, or
// adding `branches: [some-release-branch]` under `pull_request`, both disarm
// every guard in this package while leaving a paths-only check green. That is
// #199's own hole one level up, so half (1) is not decoration — it is the same
// finding applied to this file. (Desk review of #205.)
//
// This test is itself a cross-module reader — it reads both workflow files, and
// its staleness scanner walks every _test.go in the repo — and it is registered
// as one, so the recursion terminates: tools.yml must trigger on
// .github/workflows/**, which includes statusgen.yml.
//
// WHY THE ROWS ARE HAND-WRITTEN — AND WHAT NOW CATCHES A MISSING ONE. Full
// auto-discovery is still the wrong tool: working out WHICH paths a test reads
// needs intra-procedural dataflow (scancoupling_test.go builds its path from
// five separate string arguments; ownedrepos_coupling_test.go assigns the join
// to a variable and reads the variable), and a path-resolving heuristic
// false-positives on real content here — cmd/writeguard/guard_test.go compares
// a token against "../../../STATUS.md", a path that genuinely exists outside its
// module and is never read. What a row says (which workflow, which job, which
// reads, and WHY it matters) is a human judgement and stays one.
//
// The price of that is real and was paid inside this very PR: the sweep that
// produced this registry was honest when it ran, and went stale
// minutes later when a merge of main brought in
// tools/desk/cmd/issueboard/ownedrepos_coupling_test.go — a fourth cross-module
// reader of statusgen/, missing from the list until the desk review caught it
// (#205). A comment is not an enforcement, so the answer is not a comment:
// TestCrossModuleReaderRegistryIsNotSilentlyStale (below) now forces every
// _test.go that both names a filesystem read and carries a ".." path segment to
// be CLASSIFIED — a registry row or an opt-out with a reason. It would have gone
// red on that merge. It is a forcing function, not an oracle; its bound is
// documented on the test itself, and the hand sweep remains the backstop:
//
//	grep -rn 'filepath.Join("\.\.\|"\.\./' --include='*_test.go' .
//	grep -rn 'os\.ReadFile\|os\.ReadDir\|parser\.ParseFile' --include='*_test.go' .
//
// tools/desk/cmd/verifyloop USED to be in that set. A prior change moved the
// verify-desk skill into THIS repo, so dispatch_sync_test.go dropped its
// `consumer` tag and became a registered reader below — no longer the
// consumer's to own. Its read has since been re-pointed again, at the verifier
// dispatch kit under tools/desk/cmd/deskdispatch/references/, which is what the
// skill now declares as the prompt's source; the row moved with it.
//
// LIMITS — what this guard does NOT prove, stated rather than implied. It reads
// YAML with a hand-rolled parser (tools/desk is dependency-free), so it models
// only what it can model safely: it does NOT evaluate step-level `if:`,
// `continue-on-error`, `on.<event>.types:`, reusable-workflow (`uses: owner/repo`)
// indirection, runner availability, or whether a check name is required by
// branch protection. It DOES follow a LOCAL composite action (`uses: ./path`)
// one level, expanding that action's `${{ inputs.X }}` defaults and `env:`
// bindings, so relocating `go test` into `.github/actions/**` neither hides it
// nor reads as absent; an action it cannot read, cannot
// parse, or that nests another local action Fatals. It does not and cannot make GitHub evaluate a filter on
// demand. Where its parser meets a shape it does not understand it FATALS rather
// than returning "covered" — a false red on a legal reflow is a cost worth
// paying; a false green here is the bug this file exists to prevent.
//
// go.work USED to be matched by no filter in tools.yml, with no registered
// test reading it — a live gap recorded here because a workflow edit is out
// of this file's reach. #491 closed it: modprefix_test.go now
// reads go.work to enforce the module-path-prefix invariant, tools.yml gained
// a "go.work" paths: entry for both events, and the row is registered below.
func TestCrossModuleTestsAreTriggeredByWhatTheyRead(t *testing.T) {
	// repoRoot: this package is tools/desk/internal/deskkit.
	const repoRoot = "../../../.."

	skipIfFixtureAbsent(t, filepath.Join(repoRoot, ".github", "workflows", "tools.yml"),
		".github/ and the sibling tool tree are not part of this repository's published file set")

	registry := ciCrossModuleRegistry()

	for _, e := range registry {
		t.Run(e.test, func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(e.test))); err != nil {
				t.Fatalf("registry is stale: %s does not exist (%v) — if the test moved, "+
					"re-point this row; if it was deleted, delete the row DELIBERATELY", e.test, err)
			}
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(e.module))); err != nil {
				t.Fatalf("registry is stale: module %s does not exist (%v)", e.module, err)
			}

			wfPath := filepath.Join(repoRoot, filepath.FromSlash(e.workflow))
			raw, err := os.ReadFile(wfPath)
			if err != nil {
				t.Fatalf("cannot read %s: %v — the workflow that runs %q must exist",
					e.workflow, err, e.prJob.check)
			}
			content := string(raw)

			for _, event := range []string{"pull_request", "push"} {
				job := e.prJob
				if event == "push" {
					job = e.pushJob
				}

				// HALF (1) — reachability. A perfect paths: filter on a job
				// that cannot run is worth nothing.
				ciAssertEventReachesJob(t, content, e.workflow, e.module, event, job, e.runInvokes, e.test, e.why)

				// HALF (2) — coverage. Every read must be matched by a glob.
				globs := workflowEventPaths(t, content, e.workflow, event)
				for _, read := range e.reads {
					if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(read))); err != nil {
						t.Errorf("registry is stale: %s reads %s, which does not exist (%v)", e.test, read, err)
						continue
					}
					if !anyGlobMatches(t, globs, read) {
						t.Errorf("%s (%s) reads %s, but %s's on.%s.paths does not cover it (globs: %v).\n"+
							"A cross-module guard whose trigger excludes what it reads is ADVISORY, not enforced — %s.\n"+
							"Fix the FILTER (add a glob covering %s); do NOT weaken or delete the test (#199).",
							e.test, job.check, read, e.workflow, event, globs, e.why, read)
					}
				}
			}
		})
	}
}

// ciCrossModuleRegistry is the hand-maintained roster of tests that deliberately
// read outside their own Go module. Both tests in this file consume it: the
// trigger/reachability guard above, and the staleness scanner below.
func ciCrossModuleRegistry() []ciEntry {
	toolsDeskJob := ciJobRef{
		id:          "test",
		matrixValue: "tools/desk",
		check:       "test (tools/desk)",
	}

	toolsAssessJob := ciJobRef{
		id:          "test",
		matrixValue: "tools/assess",
		check:       "test (tools/assess)",
	}

	toolsClaimsguardJob := ciJobRef{
		id:          "test",
		matrixValue: "tools/claimsguard",
		check:       "test (tools/claimsguard)",
	}

	toolsClosecheckJob := ciJobRef{
		id:          "test",
		matrixValue: "tools/closecheck",
		check:       "test (tools/closecheck)",
	}

	toolsReleaseguardJob := ciJobRef{
		id:          "test",
		matrixValue: "tools/releaseguard",
		check:       "test (tools/releaseguard)",
	}

	registry := []ciEntry{
		{
			// Registered with the guard it enforces (#392 review
			// B3): closecheck's entire advisory guarantee — that it cannot
			// force a PR — rests on continue-on-error: true and the absence of
			// --strict in closecheck.yml's run: line, and nothing but this
			// test reads that file to check it.
			test:     "tools/closecheck/workflow_test.go",
			module:   "tools/closecheck",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsClosecheckJob,
			pushJob:  toolsClosecheckJob,
			reads: []string{
				".github/workflows/closecheck.yml",
			},
			why: "closecheck's advisory-not-a-gate property (the headline of its own PR title) is " +
				"asserted nowhere else: deleting continue-on-error: true, or appending --strict to " +
				"the run: line, both left the suite green before this test existed. If editing " +
				"closecheck.yml does not run it, that regression ships silently",
		},
		{
			test:     "tools/assess/docs_test.go",
			module:   "tools/assess",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsAssessJob,
			pushJob:  toolsAssessJob,
			reads: []string{
				"docs/assessment.md",
			},
			why: "assess is a claimed-vs-evidenced tool, so its own worked example must be " +
				"regenerated rather than asserted: this test re-renders a recorded run and " +
				"fails if the doc's figures drift from what the tool emits. If editing the " +
				"doc does not run it, the figures become hand-writable again — which is the " +
				"exact defect the tool exists to name",
		},
		{
			test:     "tools/desk/internal/topology/drift_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				// The ONE declared source. compiled.go is its derivation and
				// this test is the diff.
				"topology.yaml",
				// The retired-table registry names sites in the OTHER module:
				// an edit that re-introduces statusgen's scanExcludedLabels must
				// run this guard, and statusgen/** is not tools/**.
				"statusgen/scanissues.go",
				// The hand-table scanner walks EVERY non-test .go file in the
				// tree, so a new table anywhere must trigger it. These two are
				// representatives of the directories a glob must cover; the
				// tools/** and statusgen/** globs cover the rest.
				"statusgen/topologyvalues.go",
				"tools/desk/cmd/issueboard/board.go",
			},
			why: "topology.yaml is the single declared source built to end #276's five " +
				"parallel hand tables and #829's divergent label sets. This test is the ONLY thing " +
				"stopping a sixth copy: it diffs both derivations against the source, asserts every " +
				"retired site stayed retired, and scans the tree for new ones. Scoped to tools/** alone, " +
				"the statusgen-only diff that re-introduces a hand table is exactly the diff that never " +
				"runs the guard",
		},
		{
			// Registered by the kill-switch loop-name roster (deskkit/loopnames.go).
			// A loop's identity is DECLARED by the `export DESK_LOOP=<name>` line in its
			// SKILL.md, under both skills roots, and documented in the `Loop-name
			// registry` bullet of tools/desk/README.md. loopnames_test.go is the diff
			// between those declarations and the roster the kill switch enforces. Both
			// skills roots are outside tools/**, so scoped to tools/** alone a
			// skill-only edit — a loop renaming itself, which is exactly the event that
			// orphans a human's held STOP.<name> — is the one edit that would not run
			// this guard.
			test:     "tools/desk/internal/deskkit/loopnames_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				// One representative per skills root; the `.claude/**` and `plugins/**`
				// globs cover the rest of both directories. `the-desk` is chosen because
				// it is a desk NOT being renamed by the desk-loop rename, so this row cannot
				// go stale on that rename.
				".claude/skills/the-desk/SKILL.md",
				"plugins/assay/skills/the-desk/SKILL.md",
				// The human-facing registry the roster is diffed against. In-module, so
				// tools/** already covers it — listed because it is a real read.
				"tools/desk/README.md",
			},
			why: "a loop that presents a DESK_LOOP name the roster does not know gets NO per-loop " +
				"stop protection, and before this roster that failed silently: a held " +
				"STOP.<oldname> simply stopped matching after a rename and the loop ran on. The " +
				"declarations live in the skills, not in tools/**, so a skills-only rename must " +
				"run the diff that catches it",
		},
		{
			test:     "tools/desk/internal/topology/example_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				// The WITHHELD declared source, and the public tree's schema-only
				// stand-in for it. This test diffs their SHAPE.
				"topology.yaml",
				"topology.example.yaml",
			},
			why: "the 2026-08-13 publication ruling withholds topology.yaml permanently and ships " +
				"topology.example.yaml in its place, relocated to topology.yaml at staging. A withheld " +
				"real file plus a hand-written public twin is the second-copy defect this test " +
				"exists to kill, and this shape diff is the ONLY thing closing it — a schema change to " +
				"the source reddens until a person states it in the public example. " +
				"topology.example.yaml is not under tools/** or statusgen/**, so without this entry the " +
				"example-only edit — a public example quietly drifting from the schema it demonstrates — " +
				"is exactly the edit that runs no job at all",
		},
		{
			test:     "tools/desk/internal/topology/secondcell_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				// The ADOPTER CONTRACT. This test asserts every compiled-in
				// topology fact — everything a second cell CANNOT change by
				// editing config — is named there as a cap.
				"docs/adopting-assay.md",
			},
			why: "the multi-cell claim is that adding a cell is CONFIG, not CODE. It is true of the " +
				"schema and false of the shipped binaries, which read the compiled derivation and " +
				"never read topology.yaml at run time, so five surfaces are frozen at build time. " +
				"That cap is only honest while docs/adopting-assay.md NAMES it, and the document is " +
				"a doc — nobody expects a doc edit to need a Go test, which is exactly why deleting " +
				"a cap from the contract is the edit that must not run silently. docs/** is not " +
				"otherwise covered by this workflow (only docs/assessment.md was), so without this " +
				"entry the contract could be emptied and every job would sit out the PR",
		},
		{
			test:     "tools/desk/internal/deskkit/scancoupling_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				"statusgen/trustgate.go",
				"statusgen/scanissues.go",
				// The roster moved out of source, so the
				// coupling now also reads statusgen's CONFIG READER and the twin
				// test that consumes the shared vector file. A statusgen-only diff
				// to either can break the cross-tree binding.
				"statusgen/rosterconfig.go",
				"statusgen/rosterconfig_test.go",
				// The SHARED cross-tree roster vectors. They live in statusgen's
				// tree so statusgen reads them in-module; this module's read is
				// the cross-module one, and it is what must stay triggered.
				"statusgen/testdata/roster_coupling.json",
			},
			why: "the --scan-issues trust gate is a security-ordering gate: statusgen's " +
				"scanner reads PUBLIC repos and turns issues into durable desk work items, " +
				"so an unrun tripwire means arbitrary external issue text can create them",
		},
		{
			// The risk×files cross-read trigger coupling.
			// risktrigger_coupling_test.go reads the shared vector out of statusgen's
			// tree and runs it through THIS module's policy-half classifier
			// (RiskPathTriggersFor + matchTrigger); statusgen's twin runs the same
			// file through its duplicate (riskpathtriggers.go). A statusgen-only diff
			// to the vector — or to the duplicated reader the vector guards — must run
			// this job or the two duplicated classifiers can drift with both suites
			// green. tools.yml's statusgen/** glob covers the read; this row makes the
			// cross-tree binding explicit and keeps the staleness scanner honest.
			test:     "tools/desk/internal/deskkit/risktrigger_coupling_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				// The SHARED cross-tree trigger vectors. They live in statusgen's tree
				// so statusgen reads them in-module; this module's read is the
				// cross-module one, and it is what must stay triggered.
				"statusgen/testdata/risk_trigger_coupling.json",
			},
			why: "the risk-path classifier's POLICY half is DUPLICATED into the statusgen lint so a " +
				"brief's declared paths can be cross-read against it; the two " +
				"copies are bound only by this shared vector, so a statusgen-only diff that drifts the " +
				"base list, the glob rule or the per-repo topology reading must run this job or the " +
				"gate a PR is classed by and the gate a brief is checked against silently disagree",
		},
		{
			test:     "tools/desk/internal/deskkit/citrigger_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				".github/workflows/tools.yml",
				".github/workflows/statusgen.yml",
				// Registered when the disclosure sweep landed: this
				// file now parses leaksweep.yml too, so an edit to it must run
				// the guard that checks it. tools.yml's .github/workflows/**
				// glob already covers this; the row makes the read explicit.
				".github/workflows/leaksweep.yml",
				// The staleness scanner in this same file walks EVERY _test.go
				// in the repo, statusgen/ included, so a statusgen-only diff
				// that adds a cross-module reader must run this job too.
				// version_test.go stands in for that whole directory.
				"statusgen/version_test.go",
			},
			why: "this very test reads both workflows' filters AND every test file in the repo; " +
				"if editing either workflow — or adding a test under statusgen/ — does not run it, " +
				"the trigger-coverage guard and its staleness scanner can be disarmed silently",
		},
		{
			// The assay-lint composite action is ADOPTER-FACING
			// code — it runs in someone else's CI and its whole contract is its
			// exit code — so its single-writer step is executed verbatim by
			// assaylintguard_test.go rather than merely read. That test lives in
			// tools/desk but reads .github/actions/assay-lint/action.yml, so
			// tools.yml's paths: must cover .github/actions/**. Without that
			// entry, editing the action is precisely the diff that does not run
			// the guard which proves the action can still fail — the #199 shape,
			// one directory over from where reviewer finding F6 found it.
			test:     "tools/desk/internal/deskkit/assaylintguard_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				".github/actions/assay-lint/action.yml",
			},
			why: "it runs the shipped single-writer guard's shell against constructed repos to prove it " +
				"goes red on the violation AND red on an errored diff; if editing the action does not " +
				"run it, the fail-open that check exists to prevent can be reintroduced with CI silent",
		},
		{
			// #621. The fail-first guard for the tree-sweep leak
			// gate's pipefail plumbing. treesweep_pipefail_test.go parses
			// leaksweep.yml and asserts the sweep step propagates leaksweep's
			// leak-exit (2) through the `| tee` capture: a pipeline's status is
			// tee's (0) unless pipefail is set, and that masking made the gate
			// report SUCCESS on a real leak (#619: failed=27, job success) while
			// also disarming the failure()-gated SOFT-ALERT. Registered so that
			// editing leaksweep.yml — the exact file it guards — runs it;
			// tools.yml's .github/workflows/** glob already covers the read, and
			// this row makes the coupling explicit and keeps the scanner honest.
			test:     "tools/desk/internal/deskkit/treesweep_pipefail_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				".github/workflows/leaksweep.yml",
			},
			why: "it is the regression guard proving the tree-sweep leak gate still FAILS on a leak: " +
				"the sweep is piped through `tee`, whose exit 0 masks leaksweep's leak-exit (2) unless " +
				"pipefail is set, and that masking made the gate report success on a real leak (#619). " +
				"If editing leaksweep.yml does not run this test, the pipefail plumbing can be dropped " +
				"again and the control silently reverts to a no-op on the runner",
		},
		{
			// Registered late — it arrived via a merge of main AFTER the sweep
			// that built this registry, and the desk review caught the gap
			// (#205). See "HOW IT GOES STALE" above.
			//
			// The test AST-parses EVERY non-test .go file in statusgen/, so the
			// registered read below is a representative of the whole directory:
			// scanissues.go is where scanRepos — the value it couples to — is
			// declared. Any glob that covers it (statusgen/**) covers the rest.
			test:     "tools/desk/cmd/issueboard/ownedrepos_coupling_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				"statusgen/scanissues.go",
			},
			why: "it enforces issueboard's ownedRepos == statusgen's scanRepos; they are the " +
				"read and write surfaces of the same lane, and they already drifted once " +
				"(scanRepos was widened, ownedRepos left behind) so owned issues were invisible " +
				"to the board — an unrun test lets that silently recur on any statusgen-only diff",
		},
		{
			// Registered when the read was restored (#404 part 3).
			// The read existed once before and was DELETED rather than
			// registered, because neither half of the guard could be
			// satisfied then: tools/plugindrift was not in tools.yml's matrix,
			// so half (1) had no job to name, and plugins/** was in neither
			// paths: filter, so half (2) could not cover the read. Both landed
			// later, so the row is now a checked fact rather than a
			// placation — and the negative control was run: deleting the
			// plugins/** glob from either event's filter turns this row red.
			//
			// The test reads the SHIPPED manifest and walks the SHIPPED bundle
			// (CheckCoverage globs skills/*/SKILL.md under plugins/assay), so
			// the second read below is a representative of that directory: any
			// glob covering it (plugins/**) covers every sibling skill.
			test:     "tools/plugindrift/drift_test.go",
			module:   "tools/plugindrift",
			workflow: ".github/workflows/tools.yml",
			prJob: ciJobRef{
				id:          "test",
				matrixValue: "tools/plugindrift",
				check:       "test (tools/plugindrift)",
			},
			pushJob: ciJobRef{
				id:          "test",
				matrixValue: "tools/plugindrift",
				check:       "test (tools/plugindrift)",
			},
			reads: []string{
				"plugins/assay/SOURCES.yaml",
				"plugins/assay/skills/the-desk/SKILL.md",
			},
			why: "TestCheckCoverage_ShippedManifestCoversShippedBundle asserts that every shipped " +
				"skills/*/SKILL.md is either pinned in files: with a source or declared in unported: " +
				"with a reason — the one check that catches a skill added to the bundle with no " +
				"provenance at all, which the per-source drift check cannot see because it only " +
				"re-fetches what is already pinned. The diffs that break it are exactly the ones " +
				"that touch plugins/**: add a skill, delete one, or edit SOURCES.yaml. Triggered " +
				"on tools/** alone it would never run on any of them",
		},
		{
			// The resident-rules single-source guard. harnessgen single-sources the resident
			// operating rules: plugins/assay/resident-rules.md is the ONE home,
			// and the committed delivery artifacts — the Claude SessionStart
			// payload and the Codex AGENTS.md fragment — are GENERATED from it.
			// harnessgen_test.go reads that source from the repo root
			// (../../plugins/assay/…) and TestCommittedArtifactsMatchSource runs
			// `resident --check` against ../.. to prove the committed artifacts
			// still match the source. The diffs that break it edit plugins/**
			// (the source, or a hand-edited generated artifact), not tools/**, so
			// a trigger scoped to tools/** alone would leave the drift unrun on
			// exactly the edit that causes it. tools.yml's plugins/** filter
			// covers the reads: resident-rules.md is the source read by the test
			// file, and AGENTS-assay.md is the representative of the generated set
			// the check compares.
			test:     "tools/harnessgen/harnessgen_test.go",
			module:   "tools/harnessgen",
			workflow: ".github/workflows/tools.yml",
			prJob: ciJobRef{
				id:          "test",
				matrixValue: "tools/harnessgen",
				check:       "test (tools/harnessgen)",
			},
			pushJob: ciJobRef{
				id:          "test",
				matrixValue: "tools/harnessgen",
				check:       "test (tools/harnessgen)",
			},
			reads: []string{
				"plugins/assay/resident-rules.md",
				"plugins/assay/codex/AGENTS-assay.md",
			},
			why: "the resident rules are the always-loaded guardrails, single-sourced so a " +
				"generation bug cannot silently ship a divergent rule set to every session; the " +
				"coupling test reads the source and re-checks the committed artifacts against it, " +
				"and the diffs that break it touch plugins/** (the source or a generated " +
				"artifact), never tools/** — scoped to tools/** the drift would go unrun",
		},
		{
			// The disclosure sweep walks the WHOLE
			// repository, so it is the one guard in this registry whose reads are
			// "every file" — hence its own workflow with a `**` filter rather than
			// a seat in tools.yml's matrix (widening THAT filter would run all six
			// modules on every PR in the repo).
			//
			// The four reads below are representatives of the four regions the
			// token really occupied before the scrub — compiled Go, the adopter
			// README, the App setup guide, and a repo-root file that only `**`
			// can match. AGENTS.md is the load-bearing one: it is matched by no
			// glob in tools.yml or statusgen.yml, so a registry row pointing at
			// either of those workflows would be red on this read alone. That is
			// the check working, not a nuisance.
			test:     "tools/desk/internal/deskkit/s2sweep_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/leaksweep.yml",
			prJob: ciJobRef{
				id:    "sweep",
				check: "sweep",
			},
			pushJob: ciJobRef{
				id:    "sweep",
				check: "sweep",
			},
			reads: []string{
				"AGENTS.md",
				"docs/github-apps-setup.md",
				"tools/desk/README.md",
				"tools/desk/internal/deskkit/config.go",
			},
			why: "it is the branch-level gate on the private-review-channel disclosure: the " +
				"channel exists so findings about PUBLIC-repo PRs stay unpublished, so shipping " +
				"its name converts the mitigation into a pointer at the withheld material, and a " +
				"name published once cannot be recalled (forks, search, archives). The token last " +
				"lived in docs and stream files as much as in Go, so a trigger scoped to tools/** " +
				"would leave the sweep unrun on precisely the docs-only diff that reintroduces it",
		},
		{
			// The corpus withheld-stream guard (prereq for #1316). corpusleak_test.go
			// reads the LIVE docs/streams and docs/archive directory listing (out of
			// the tools/desk module) to derive the withheld-stream set, then greps the
			// tools/desk copy set for any real stream path. A new stream landing under
			// docs/streams/** changes what the guard forbids, so — exactly like the S2
			// sweep above — a trigger scoped to tools/** would leave it unrun on the
			// docs-only diff that adds a stream a copy file then names. It runs in the
			// SAME leaksweep.yml `sweep` job on the `**` filter (the job's `-run`
			// selector matches TestS2|TestCorpus).
			test:     "tools/desk/internal/deskkit/corpusleak_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/leaksweep.yml",
			prJob: ciJobRef{
				id:    "sweep",
				check: "sweep",
			},
			pushJob: ciJobRef{
				id:    "sweep",
				check: "sweep",
			},
			reads: []string{
				"docs/streams",
				"docs/archive",
			},
			why: "it is the class-closer that stops a withheld docs/streams path re-entering a " +
				"shipping tools/desk file (the #1316 leak class): the whole docs/streams tree is " +
				"do-not-copy, so a real stream path in a copy file publishes a map to withheld " +
				"material. It derives its forbidden set from the live docs/streams + docs/archive " +
				"listing, so a trigger that excludes those trees would let a newly-added stream be " +
				"named in a copy file with the guard unrun",
		},
		{
			// #607. The staged-tree token sweep — the gate that a
			// docs/testdata PR slipped a withheld token past twice (PR #605's
			// docs/streams README, cleared by #604; an tracker
			// recurrence in a statusgen fixture, cleared by #609) BECAUSE `leaksweep
			// run --tree` was invoked only by hand and gated nothing.
			//
			// It is the one row here that is NOT a `go test`. The leaksweep.yml
			// `tree-sweep` job runs `pubmanifest stage` to materialise the public
			// tree, then `leaksweep run --tree` over it. So `test` names the sweep
			// command's SOURCE (there is no _test.go that drives it — the staged
			// tree is built by a binary, not asserted by a unit test) and the
			// runInvokes fragments below carry half (1)'s last check instead of the
			// `go test <module>` one: dropping, renaming, or splitting either
			// invocation reddens this.
			//
			// The reads are representatives of the COPY-ELIGIBLE set, which spans
			// the whole tree — a leak can be introduced by editing ANY file that
			// ends up copied. docs/adopting-assay.md is a copied docs-tree file (the
			// #605 leak class last lived in a docs README as much as in Go); the
			// statusgen fixture is the #609 one (statusgen/ is copied WHOLE, testdata
			// included); and CONTRIBUTING.md is the repo-root copied file that only a
			// `**` filter can match — a row pointing at any narrower filter would be
			// red on it, which is the check proving `**` is load-bearing here, not
			// decoration. (The read is a docs file that SHIPS: the docs/streams tree
			// is do-not-copy post-pivot, so `pubmanifest stage` never stages it and a
			// stream path would not be a representative of the staged tree at all.)
			test:     "tools/leaksweep/main.go",
			module:   "tools/leaksweep",
			workflow: ".github/workflows/leaksweep.yml",
			prJob: ciJobRef{
				id:    "tree-sweep",
				check: "tree-sweep",
			},
			pushJob: ciJobRef{
				id:    "tree-sweep",
				check: "tree-sweep",
			},
			runInvokes: []string{
				"go run ./tools/pubmanifest stage",
				"go run ./tools/leaksweep run --tree",
			},
			reads: []string{
				"docs/adopting-assay.md",
				"statusgen/testdata/attribution/docs/streams/attr/brief-08-escapedpipe.md",
				"CONTRIBUTING.md",
			},
			why: "it is the branch-level gate on publishing a withheld token (example-org, a human " +
				"name, a registered private repo slug) into the one-way public copy: a name published " +
				"once cannot be recalled (forks, search, archives). The tokens last lived in a " +
				"docs/streams README (#605) and a statusgen test fixture (#609) as much as in Go, so a " +
				"trigger scoped to tools/** would leave the sweep unrun on precisely the docs- or " +
				"testdata-only diff that reintroduces one",
		},
		{
			// The standing-approvals gate. Like the
			// tree-sweep row above this is NOT a `go test`: the tools.yml
			// `test (tools/approvalguard)` leg builds the approvalguard
			// binary and runs it against the CHECKED-OUT REPO's real
			// .claude/settings.json + approvals.v1.yaml, so `test` names the
			// command's source and runInvokes carries half (1)'s last check.
			//
			// The `go build` fragment is pinned deliberately: `go run` reports
			// its own wrapper status and collapses could-not-check (2) onto
			// checked-failed (1), so rewriting the step to `go run` would
			// silently destroy the three-state contract while still "running
			// the checker". Dropping, renaming or splitting either invocation
			// reddens this row.
			//
			// The reads are the two halves of the invariant, and listing BOTH
			// is the entire point of this row. `.claude/settings.json` was
			// already covered by tools.yml's `.claude/**`, so the ADD side
			// (a new unregistered allow entry) always triggered the gate. The
			// register is a repo-root file that matched no glob, so the REMOVE
			// side did not: deleting the row authorising a live allow entry ran
			// no go-test job, so the only checker that compares the two files
			// never executed. Precisely that — NOT "no CI ran": leaksweep.yml
			// and publication-boundary.yml trigger on `paths: ["**"]`, so a
			// repo-root edit does fire tree-sweep/sweep/boundary. Those jobs
			// sweep for leaked tokens and publication-boundary violations; none
			// reads the register. A job running is not the same as the guard
			// running, which is this file's whole subject. Half (2) now fails
			// unless a glob covers the register too.
			test:     "tools/approvalguard/main.go",
			module:   "tools/approvalguard",
			workflow: ".github/workflows/tools.yml",
			prJob: ciJobRef{
				id:          "test",
				matrixValue: "tools/approvalguard",
				check:       "test (tools/approvalguard)",
			},
			pushJob: ciJobRef{
				id:          "test",
				matrixValue: "tools/approvalguard",
				check:       "test (tools/approvalguard)",
			},
			runInvokes: []string{
				"go build -o /tmp/approvalguard .",
				`/tmp/approvalguard --root "$GITHUB_WORKSPACE"`,
			},
			reads: []string{
				".claude/settings.json",
				"approvals.v1.yaml",
			},
			why: "permissions.allow is what agents may run UNATTENDED, and the register is the " +
				"only reviewed record of who approved each entry and why. The gate is what keeps " +
				"the two in step; a trigger covering only the allowlist enforces the invariant from " +
				"one side, leaving a register-only edit — the edit that REMOVES an approval's " +
				"justification while the grant stays live — running no job at all",
		},
		{
			// The assay.guide claim gate — the SECOND non-`go test`
			// row here (see the tree-sweep row above for the runInvokes shape).
			//
			// `test` names the gate's source, not a _test.go: sitehonesty is a bash
			// gate, so there is no Go suite to drive it. The runInvokes fragments
			// carry half (1)'s last check — the job must still run BOTH the fixture
			// suite (the proof it can fail) and the gate itself. Dropping either
			// leaves a page certified by a gate whose controls nobody ran.
			//
			// The reads are what make this row load-bearing rather than decorative.
			// Every anchor in tools/sitehonesty/anchors.tsv pairs a sentence on the
			// page with a file that recomputes it, and the whole class this gate
			// exists for is a claim that was TRUE WHEN WRITTEN and went FALSE when
			// a SIBLING file moved — not when the page did. A filter scoped to
			// web/** alone would leave the gate unrun on precisely the diff that
			// falsifies a claim: flipping docs/articles/ to `copy` in the
			// publication manifest falsifies the two "coming soon" non-links;
			// renaming a lifecycle state falsifies the five-state explainer;
			// deleting writeguard falsifies "the board's single writer". None of
			// those diffs touch web/**. (The reads listed here are the anchors this
			// gate actually recomputes; a stream this page merely mentions is not
			// among them, so no docs/streams path is named in this shipping file.)
			test:     "tools/sitehonesty/sitehonesty.sh",
			module:   "tools/sitehonesty",
			workflow: ".github/workflows/site-honesty.yml",
			prJob: ciJobRef{
				id:    "site-honesty",
				check: "site-honesty",
			},
			pushJob: ciJobRef{
				id:    "site-honesty",
				check: "site-honesty",
			},
			runInvokes: []string{
				"bash tools/sitehonesty/sitehonesty.test.sh",
				"bash tools/sitehonesty/sitehonesty.sh",
			},
			reads: []string{
				"web/site/index.html",
				"tools/sitehonesty/anchors.tsv",
				"docs/lifecycle.md",
				"LICENSE",
				"docs/publication-manifest.yaml",
				"tools/desk/cmd/writeguard/main.go",
			},
			why: "it is the branch-level gate on the assay.guide landing page overclaiming: the page " +
				"is the product's first publicly-visible surface and the product's whole positioning " +
				"is that it refuses to overclaim, so a claim that quietly went false contradicts the " +
				"thing being sold. The overclaims measured in this repo were NOT banned words — they " +
				"were anchored claims that came loose when a sibling file moved, which is exactly the " +
				"diff a web/**-only trigger would let through in silence",
		},
		{
			test:     "statusgen/version_test.go",
			module:   "statusgen",
			workflow: ".github/workflows/statusgen.yml",
			prJob: ciJobRef{
				id:    "lint",
				check: "lint",
			},
			// NOT `lint`: statusgen.yml's lint job carries
			// `if: github.event_name == 'pull_request'`, so it cannot fire on
			// push at all. `regen` is what runs the statusgen suite on main
			// (desk review of #205). The push half is still asserted — a
			// filter that excludes release-statusgen.yml would leave the stamp
			// check unrun on main too.
			pushJob: ciJobRef{
				id:    "regen",
				check: "regen",
				note: "regen is additionally skipped when the head commit message contains " +
					"[skip-status-regen] (the bot's own regen commit), so push-half coverage " +
					"is best-effort by design; the pull_request half is the enforcing one",
			},
			reads: []string{
				".github/workflows/release-statusgen.yml",
			},
			why: "it proves the release build still stamps -X main.statusgenVersion=; " +
				"an unstamped release answers \"dev\" and silently defeats every consumer pin check",
		},
		{
			test:     "statusgen/corroboratewiring_test.go",
			module:   "statusgen",
			workflow: ".github/workflows/statusgen.yml",
			prJob: ciJobRef{
				id:    "lint",
				check: "lint",
			},
			// Same split as version_test.go, topologyvalues_test.go, unrun_test.go,
			// and channels_test.go above, and for the same reason: statusgen.yml's
			// lint job is `if: github.event_name == 'pull_request'`, so `regen` is
			// what runs the suite on main.
			pushJob: ciJobRef{
				id:    "regen",
				check: "regen",
				note: "regen is additionally skipped when the head commit message contains " +
					"[skip-status-regen] (the bot's own regen commit), so push-half coverage " +
					"is best-effort by design; the pull_request half is the enforcing one",
			},
			reads: []string{
				// The workflow that must actually invoke `--corroborate --pr`. This
				// test parses it and asserts the invocation is a live gate (job
				// gated on pull_request, no continue-on-error, no `|| true`, bound
				// to the event's own PR number) rather than present-but-dead.
				".github/workflows/statusgen.yml",
			},
			why: "the register-authorization anchor (`authorized-by: human:<name>`) is only as real " +
				"as the `--corroborate` invocation that checks it, and every unit test on the " +
				"corroboration logic itself (corroborate_test.go) passes whether or not the workflow " +
				"calls it. This test is the only thing proving the gate is wired in and alive; a " +
				"workflow-only edit that quietly drops, disables, or hardcodes the invocation must " +
				"run it, or the anchor becomes self-issuable in CI",
		},
		{
			test:     "statusgen/topologyvalues_test.go",
			module:   "statusgen",
			workflow: ".github/workflows/statusgen.yml",
			prJob: ciJobRef{
				id:    "lint",
				check: "lint",
			},
			// Same split as version_test.go below, and for the same reason:
			// statusgen.yml's lint job is `if: github.event_name ==
			// 'pull_request'`, so `regen` is what runs the suite on main.
			pushJob: ciJobRef{
				id:    "regen",
				check: "regen",
				note: "regen is additionally skipped when the head commit message contains " +
					"[skip-status-regen] (the bot's own regen commit), so push-half coverage " +
					"is best-effort by design; the pull_request half is the enforcing one",
			},
			reads: []string{
				// The ONE declared source of org/repo/App/product topology.
				// statusgen/topologyvalues.go is a DERIVATION of it, and this
				// test is the diff that makes it a derivation rather than a
				// sixth hand table.
				"topology.yaml",
			},
			why: "topology.yaml is the single declared source built to end #276's " +
				"five parallel hand tables; statusgen carries a compiled derivation because it ships " +
				"as a pinned binary run against an arbitrary --root. If an edit to the SOURCE does not " +
				"run the diff, the derivation silently becomes a hand table again — which is exactly " +
				"how #829 happened (two copies of one label set, kept equal by a comment, and not equal)",
		},
		{
			test:     "statusgen/unrun_test.go",
			module:   "statusgen",
			workflow: ".github/workflows/statusgen.yml",
			prJob: ciJobRef{
				id:    "lint",
				check: "lint",
			},
			// Same split as version_test.go above, and for the same reason:
			// statusgen.yml's lint job is `if: github.event_name ==
			// 'pull_request'`, so `regen` is what runs the suite on main.
			pushJob: ciJobRef{
				id:    "regen",
				check: "regen",
				note: "regen is additionally skipped when the head commit message contains " +
					"[skip-status-regen] (the bot's own regen commit), so push-half coverage " +
					"is best-effort by design; the pull_request half is the enforcing one",
			},
			reads: []string{
				// loadStreams("..") walks the WHOLE of docs/streams/**; this
				// operational-stream file (the issue-flow rulings register) is a
				// representative of the directory that ALWAYS exists, and any glob
				// covering it (docs/**) covers the rest. Chosen operational-and-durable
				// on purpose: it is read at runtime, so it can never be a withheld-stream
				// path leaked into this shipping file, and it will not be renamed away.
				"docs/streams/issue-flow/rulings.md",
			},
			why: "two of its cases run the UNRUN derivation against this repo's REAL board rather " +
				"than a fixture — one proves the merge-base grandfathering resolves a non-empty " +
				"closed set, the other proves the stale-implemented alarm actually fires once the " +
				"clock is wound past the threshold (the principle that \"a state that never " +
				"fires is not a state\"). Both answers change when docs/streams/** changes, so a " +
				"docs-only diff that does not run this suite lets the board state silently go quiet",
		},
		{
			test:     "statusgen/channels_test.go",
			module:   "statusgen",
			workflow: ".github/workflows/statusgen.yml",
			prJob: ciJobRef{
				id:    "lint",
				check: "lint",
			},
			// Same split, same reason as version_test.go and unrun_test.go
			// above: statusgen.yml's lint job is `if: github.event_name ==
			// 'pull_request'`, so `regen` is what runs the suite on main.
			pushJob: ciJobRef{
				id:    "regen",
				check: "regen",
				note: "regen is additionally skipped when the head commit message contains " +
					"[skip-status-regen] (the bot's own regen commit), so push-half coverage " +
					"is best-effort by design; the pull_request half is the enforcing one",
			},
			reads: []string{
				// The prose VIEW of the declared channel set. This is the
				// derive-or-diff pair: statusgen/channels.go is the declared
				// source, this table is the human-readable copy, and the test
				// diffs them.
				"docs/adopting-assay.md",
				// This read also carries this entry's glob coverage for the
				// known-accepted register the sweep loads: both live under docs/**,
				// so its docs/** coverage subsumes the register's. The register's
				// real path is named only in statusgen/channelconformance.go (a
				// separately-dispositioned module), never in this shipping tools/desk
				// file — so no withheld-stream path is baked in here.
			},
			why: "the A–E channel letters are a PUBLISHED contract an adopter cites, and they exist " +
				"in two places by necessity — an executable table in statusgen/channels.go and a " +
				"prose table in the adopter runbook, which cannot be regenerated without losing its " +
				"voice. The test is the diff that keeps them the same fact. A docs-only edit that " +
				"renumbers a letter, or drops a channel's not-sanctioned marking, is exactly the diff " +
				"that must run it — and it is exactly the diff that would not touch statusgen/**",
		},
		{
			test:     "tools/releaseguard/workflow_shape_test.go",
			module:   "tools/releaseguard",
			workflow: ".github/workflows/tools.yml",
			prJob: ciJobRef{
				id:          "test",
				matrixValue: "tools/releaseguard",
				check:       "test (tools/releaseguard)",
			},
			pushJob: ciJobRef{
				id:          "test",
				matrixValue: "tools/releaseguard",
				check:       "test (tools/releaseguard)",
			},
			reads: []string{
				".github/workflows/release-desk.yml",
			},
			why: "it asserts that EVERY trigger in release-desk.yml's `on:` block has a matching " +
				"tag-move guard step. That coupling used to be only a comment, and the comment was " +
				"wrong: the guard carried `if: github.event_name == 'workflow_dispatch'` and was " +
				"SKIPPED on the tag-push path desk-tools/v0.2.3 was actually cut on (#519). " +
				"An edit to that workflow that does not run this job re-opens the gap silently — which " +
				"is exactly how it stayed invisible the first time",
		},
		{
			// Registered when the desk skills moved into
			// THIS repo's .claude/skills/ and added the skillslint check that closes
			// #452's `.claude/**` gap. The `skills` job in statusgen.yml runs
			// tools/skillslint, whose TestLintSkills_RealRepoSkillsAreValid reads
			// EVERY .claude/skills/*/SKILL.md at the repo root. The read below is a
			// representative of that whole directory: any glob covering it (.claude/**)
			// covers every sibling skill. This is the row a Verify mutation targets — deleting
			// ".claude/**" from statusgen.yml turns it red, proving the trigger is
			// pinned and cannot be narrowed back out silently.
			test:     "tools/skillslint/skills_read_test.go",
			module:   "tools/skillslint",
			workflow: ".github/workflows/statusgen.yml",
			prJob:    ciJobRef{id: "skills", check: "skills"},
			pushJob:  ciJobRef{id: "skills", check: "skills"},
			reads: []string{
				".claude/skills/linkedin-post/SKILL.md",
			},
			why: "the desk skills (the-desk, worker-desk, pr-review-desk, verify-desk, intake-desk, …) " +
				"are homed here now and loaded by every role window at boot; skillslint is the only check " +
				"that READS them, so a skill-only edit that does not run the `skills` job merges a broken " +
				"or misnamed skill with no signal — the exact `.claude/**` gap #452 documents",
		},
		{
			// Registered by the guardrail-sync guard.
			// TestGuardrailsInSyncInRealRepo byte-diffs every shared guardrail copy
			// against the ONE declared source, .claude/guardrails/GUARDRAILS.md —
			// same shape as topology.yaml -> compiled.go above: one source, a
			// generator (`make guardrail-sync`), and a test that is the diff.
			// THREE reads, three different reasons a narrower filter would defang it:
			//   - the declared source itself, so editing a rule runs the diff;
			//   - a canonical skill copy (.claude/**, already pinned by the row above);
			//   - a PUBLISHED twin under plugins/**, which is the read that made
			//     "plugins/**" necessary in statusgen.yml's filters. The twins carry
			//     the same blocks with the publication scrub applied, so a
			//     bundle-only edit — including pasting un-scrubbed house text back
			//     into a shipped file — must run this job.
			test:     "tools/skillslint/guardrail_real_test.go",
			module:   "tools/skillslint",
			workflow: ".github/workflows/statusgen.yml",
			prJob:    ciJobRef{id: "skills", check: "skills"},
			pushJob:  ciJobRef{id: "skills", check: "skills"},
			reads: []string{
				".claude/guardrails/GUARDRAILS.md",
				".claude/skills/the-desk/SKILL.md",
				"plugins/assay/skills/verify-desk/SKILL.md",
			},
			why: "F-guardrails-dup ran open for 27 days because six rule blocks were restated " +
				"across five skills with nothing comparing them; the fix is derive-or-diff, and a " +
				"diff whose trigger excludes half the copies it diffs is the same defect wearing a " +
				"checkmark. Dropping \"plugins/**\" in particular would let the published bundle " +
				"drift — or lose its publication scrub — with the guardrail job never running",
		},
		{
			// Registered when verify-desk's SKILL.md moved into this repo and this
			// test dropped its `consumer` build tag, when the requirement list it
			// parses was skill prose. That list is gone: the skill now says the
			// prompt is a KIT and names the kit as its source, so the test reads the
			// kit instead. The row is RE-POINTED rather than deleted — the coupling
			// is the point, only its far end moved.
			//
			// The read now sits under tools/**, so the trigger is the module's own
			// filter and the #199 hole (a source outside tools/** editable without
			// running the guard that reads it) closes by construction. Registered
			// anyway: the row is what makes the read visible when the kit or the
			// filter next moves.
			test:     "tools/desk/cmd/verifyloop/dispatch_sync_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				"tools/desk/cmd/deskdispatch/references/verifier-prompt.md",
			},
			why: "dispatch_sync_test.go pins verifyloop's emitted verifier prompt against the verifier " +
				"dispatch kit's clauses; the two already diverged once (the canonical text gained the " +
				"name-and-derive step, the Go template did not). An edit to the kit that does not run " +
				"tools/desk lets that recur silently",
		},
		{
			// #20 (F-34/F-35): writeguard was built and unit-tested in this
			// module but was never actually wired into a live PreToolUse hook
			// for sessions working in THIS repo's own shared checkout — only
			// a sibling product repo that adopts tools/desk had it armed.
			// This row registers the regression guard added alongside
			// .claude/settings.json (settings_wiring_test.go), which reads
			// that file to prove the wiring is still live.
			test:     "tools/desk/cmd/writeguard/settings_wiring_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				".claude/settings.json",
				// The test resolves the expected hook command from the
				// Makefile's INSTALL_DIR rather than repeating the literal
				// path, so an install-prefix move that strands the hook on a
				// nonexistent binary reddens instead of passing silently.
				"Makefile",
			},
			why: "settings_wiring_test.go proves the compiled writeguard binary is still wired into " +
				"hooks.PreToolUse for Edit/Write/MultiEdit/NotebookEdit and Bash — the mechanical " +
				"isolation backstop #20 asked for. An edit to .claude/settings.json that " +
				"silently drops that wiring (or breaks its JSON) is exactly the diff this test exists " +
				"to catch; scoped to tools/** alone it would not run. It also reads the Makefile's " +
				"INSTALL_DIR to pin the hook command: writeguard FAILS OPEN, so a hook pointed at a " +
				"path with no binary behind it disarms every surface with nothing failing loudly, and " +
				"a Makefile-only edit must therefore run this job",
		},
		{
			// Registered by #491. modprefix_test.go walks
			// go.work's `use` block and every go.mod it names, so its
			// registered read is go.work itself — the one file that names a
			// module before any tools/** or statusgen/** path exists to
			// cover it. Every go.mod it also reads already lives under
			// tools/** or statusgen/**, both already covered.
			test:     "tools/desk/internal/deskkit/modprefix_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				"go.work",
			},
			why: "PR #485 renamed all nine module paths to the github.com/medici-finance/assay/ " +
				"prefix, but nothing held that invariant for a module landing afterward — PR #336 " +
				"proved it live, adding tools/assess with the OLD prefix while lint, test " +
				"(tools/assess), and go build ./... all stayed green. A go.work-only edit (adding " +
				"a `use` entry ahead of the module's own go.mod landing under a covered path) is " +
				"exactly the diff this guard exists to catch, so it must not be the one diff that " +
				"fails to run it",
		},
		{
			// Registered by the release-stamp guard. version_test.go's
			// TestVersionStampedFromReleaseWorkflow reads release-desk.yml and
			// fails if the `-X …deskkit.ReleaseTag=$RELEASE_TAG` stamp is
			// removed — the stamp that maps a running desk-tools binary back to
			// its desk-tools/vX.Y.Z. A release-desk.yml-only edit dropping the
			// stamp is exactly the diff this guard catches, so scoped to tools/**
			// alone it would be the one diff that does not run it (mirrors
			// statusgen/version_test.go for release-statusgen.yml).
			test:     "tools/desk/internal/deskkit/version_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				".github/workflows/release-desk.yml",
			},
			why: "TestVersionStampedFromReleaseWorkflow proves the release build still stamps " +
				"-X …deskkit.ReleaseTag=$RELEASE_TAG; an unstamped release ships desk-tools binaries " +
				"that answer \"dev\" and cannot be mapped back to their desk-tools/vX.Y.Z, silently " +
				"defeating pin checks. A release-desk.yml-only edit that drops the stamp must run this test",
		},
		{
			// Registered by the raised-by label guard. raisedbyskills_test.go is the DIFF
			// half of derive-or-diff for the raised-by:<role> label vocabulary: the
			// roster's role-bindings are the declared source, and each desk SKILL.md
			// carries a hand-written `--raised-by <role>` copy of its own entry. The
			// thing EDITED is the skill, and .claude/** is not tools/** — scoped to
			// tools/** alone, the exact edit this guard exists to catch (a skill naming
			// a role nobody bound) is the one edit that would not run it.
			test:     "tools/desk/internal/deskkit/raisedbyskills_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				// Representatives of the four filing desks whose SKILL.md the scanner
				// walks; the `.claude/**` glob covers the rest of the directory.
				".claude/skills/pr-review-desk/SKILL.md",
				".claude/skills/verify-desk/SKILL.md",
				".claude/skills/intake-desk/SKILL.md",
				".claude/skills/worker-desk/SKILL.md",
			},
			why: "a skill instructing `--raised-by <role>` for a role the roster does not bind " +
				"fails at exit 5 INSIDE `deskfile new`, mid-filing, with the finding already in " +
				"hand — the failure surfaces on the filing desk at the worst moment rather than " +
				"in CI. The drift is skill-side, so a skill-only edit must run the diff",
		},
		{
			// Registered by example-stream/02, and deliberately NOT registered by
			// example-stream/01. The manifest landed first with no reader, and a
			// row then would have paired a trigger with a test that read
			// nothing — the same shape as the plugindrift assertion above,
			// which was correctly DELETED while its two halves were absent and
			// restored once they landed. claimsguard is the reader, so the row
			// and the globs land together here.
			test:     "tools/claimsguard/repo_test.go",
			module:   "tools/claimsguard",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsClaimsguardJob,
			pushJob:  toolsClaimsguardJob,
			reads: []string{
				// The DECLARED SOURCE. Every policy value claimsguard applies —
				// the prohibited strings, the scan surfaces, the exclusions, the
				// matcher, the exemptions, the stated bound — is read from this
				// file at run time, and repo_test.go re-derives its own positive
				// controls from it.
				"release-claims.v1.json",
				// The SCANNED SURFACES themselves. This is the read that makes
				// the lint enforced rather than advisory: a diff that adds a
				// prohibited claim edits one of these files and nothing under
				// tools/**, so without these globs the exact change the lint
				// exists to catch is the change that runs no job.
				"README.md",
				// Representative of web/**; the glob covers the rest of the
				// site copy (executive-brief, ceo-cfo-brief, ma-vertical,
				// teaser).
				"web/site/index.html",
				// The registry itself: repo_test.go asserts its own row is
				// present, so an edit deleting the row must run the test that
				// notices. tools/** already covers this; the entry makes the
				// read explicit.
				"tools/desk/internal/deskkit/citrigger_test.go",
			},
			why: "a prohibited-claims list nobody checks is prose with extra steps. The lint is the " +
				"only thing standing between the manifest and a release surface that claims to be " +
				"certified, audited or guaranteed — and the diff that makes such a claim is a README " +
				"or site-copy edit, which touches neither tools/** nor statusgen/**. Scoped to the " +
				"module the lint LIVES in, the guard would run on every diff except the ones it " +
				"guards against. The manifest read matters for the same reason one level up: editing " +
				"the claims list, the scan surfaces or the exemptions is editing the policy itself, " +
				"and it must not be possible to widen an exemption or drop a surface without running " +
				"the reader that applies it",
		},
		{
			// The other half of example-stream/02: releaseguard's claims
			// precondition composes by EXEC, so its test builds and runs the
			// real claimsguard binary and parses the verdict line the tool
			// prints. No module in this repo imports another, so the verdict
			// prefix exists in two places with nothing but this coupling test
			// holding them together.
			test:     "tools/releaseguard/claims_test.go",
			module:   "tools/releaseguard",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsReleaseguardJob,
			pushJob:  toolsReleaseguardJob,
			reads: []string{
				// The tool whose verdict line releaseguard parses. Under
				// tools/**, so the existing glob already covers it; the row
				// makes the read explicit and keeps the pair reviewable.
				"tools/claimsguard/report.go",
			},
			why: "renaming claimsguard's verdict prefix would make releaseguard read EVERY release as " +
				"could-not-check — the guard would refuse every cut and the cause would be a string " +
				"constant in a different module. The two cannot share a constant without the tree's first " +
				"cross-module import, so a test is the only thing holding them together, and it must run " +
				"on the diff that breaks it",
		},
		{
			// Registered by the desk-loop-names guard. retiredskillnames_test.go is the DIFF half of
			// derive-or-diff for the DESK LOOP NAMES (issue-loop -> intake-desk,
			// batch-fanout -> worker-desk). One desk skill lives in TWO places — the
			// canonical body under .claude/skills/ and its scrubbed port under
			// plugins/assay/skills/ — and nothing byte-diffs them: plugindrift's expected
			// verdict for a scrubbed port is `behind`, not `in-sync`, so a retired name
			// that lands in one copy and not the other produces no signal there. Both
			// edited surfaces are outside tools/**, so scoped to tools/** alone the exact
			// edit this guard exists to catch — a half-done rename — is the one edit that
			// would not run it.
			test:     "tools/desk/internal/deskkit/retiredskillnames_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				// One representative per skills root; the `.claude/**` and `plugins/**`
				// globs cover the rest of both directories.
				".claude/skills/worker-desk/SKILL.md",
				"plugins/assay/skills/worker-desk/SKILL.md",
			},
			why: "a rename that leaves the old desk name in one of the two copies leaves two " +
				"names for one desk, and the copies are prose kept in sync by hand — the " +
				"failure mode the rename existed to end. Nothing else in CI diffs the two " +
				"skill roots against each other, so a skills-only edit must run this check",
		},
		{
			// Registered by #258. skillrepolist_test.go is the DIFF half of
			// derive-or-diff for the DESK WRITE-REPO SET. The acting desks (pr-review-desk,
			// worker-desk, verify-desk) enumerated the repos they act on, and those lists
			// drifted from the tools' write-authorisation set in BOTH directions (phantom
			// coverage + a monitoring blind spot). The fix is to carry no list — run
			// `deskroster repos` — and this guard keeps the parallel list from creeping
			// back. Both scanned surfaces are outside tools/**, so scoped to tools/** alone
			// the exact edit this guard exists to catch — a re-pasted repo list in a skill —
			// is the one edit that would not run it.
			test:     "tools/desk/internal/deskkit/skillrepolist_test.go",
			module:   "tools/desk",
			workflow: ".github/workflows/tools.yml",
			prJob:    toolsDeskJob,
			pushJob:  toolsDeskJob,
			reads: []string{
				// One representative of the scanned root; the `.claude/**` glob covers the
				// rest. The `plugins/assay/skills` port is deliberately NOT scanned by this
				// guard (out of scope — a scrubbed `behind` mirror; see skillrepolist_test.go).
				".claude/skills/pr-review-desk/SKILL.md",
			},
			why: "a repo list re-pasted into an acting desk drifts from the write boundary the " +
				"tools enforce, silently — a skill file has nothing to disagree with. The drift " +
				"is skill-side and the scanned root is outside tools/**, so a skills-only edit " +
				"must run this diff",
		},
	}

	return registry
}

// ciAssertEventReachesJob is half (1) of the guard: it proves that `event` can
// still reach a job that runs `module`'s test suite. Four independent ways to
// disarm a guard without touching a single path glob, all checked here:
//
//	branches:      fence the event off from main
//	jobs.<id>      delete the job
//	matrix         drop the module from a matrix-generated job
//	if:            gate the job to the other event
//
// plus the blunt one: keep the job but stop running `go test` in the module.
//
// For a runInvokes row (a command gate, not a `go test` — #607) the
// last check is swapped: instead of "runs `go test` in module" it requires each
// declared command fragment to be present verbatim in the job body. Checks (a)-(d)
// are identical.
func ciAssertEventReachesJob(t *testing.T, content, wfName, module, event string, job ciJobRef, runInvokes []string, testFile, why string) {
	t.Helper()

	fail := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		extra := ""
		if job.note != "" {
			extra = "\nNote on this row: " + job.note + "."
		}
		t.Errorf("%s is registered as a cross-module guard run by %q on %s, but %s.\n"+
			"A path filter is NECESSARY BUT NOT SUFFICIENT: a guard whose job does not RUN is as "+
			"advisory as one whose filter excludes what it reads — %s.\n"+
			"Fix the WORKFLOW; do NOT weaken or delete the test (#199, #205).%s",
			testFile, job.check, event, msg, why, extra)
	}

	// (a) branches: / branches-ignore: on the event itself.
	if branches, ok := ciOnEventKey(t, content, wfName, event, "branches"); ok {
		if !anyGlobMatches(t, branches, ciDefaultBranch) {
			fail("%s's on.%s.branches is %v, which does not include %q — the workflow never fires "+
				"for ordinary work on the default branch", wfName, event, branches, ciDefaultBranch)
			return
		}
	}
	if ignored, ok := ciOnEventKey(t, content, wfName, event, "branches-ignore"); ok {
		if anyGlobMatches(t, ignored, ciDefaultBranch) {
			fail("%s's on.%s.branches-ignore is %v, which excludes %q", wfName, event, ignored, ciDefaultBranch)
			return
		}
	}

	// (b) the job must exist.
	jobLines, ok := ciWorkflowJobLines(t, content, wfName, job.id)
	if !ok {
		fail("%s has no jobs.%s — the job that ran it is gone", wfName, job.id)
		return
	}
	jobText := strings.Join(jobLines, "\n")

	// (c) if it is matrix-generated, the module must still be in the matrix.
	if job.matrixValue != "" {
		values := ciJobMatrixValues(t, jobLines, wfName, job.id)
		if !ciContains(values, job.matrixValue) {
			fail("jobs.%s's strategy.matrix no longer contains %q (it has %v), so the check %q is "+
				"never generated", job.id, job.matrixValue, values, job.check)
			return
		}
		// Expand the matrix variable to the value we care about, so the
		// go-test-directory check below sees a concrete path.
		jobText = ciMatrixVarRe.ReplaceAllString(jobText, job.matrixValue)
	}

	// (d) the job's own `if:` must not exclude this event.
	allowed, gated := ciJobEventGate(t, jobLines, wfName, job.id)
	if gated && !allowed[event] {
		fail("jobs.%s carries an `if:` that only runs for %v, so it cannot fire on %s — "+
			"re-point this row at the job that DOES carry the %s half, or remove the gate",
			job.id, ciAllowedEvents(allowed), event, event)
		return
	}

	// (e) the job must still run `go test` in the module — directly, or inside a
	// LOCAL composite action it `uses:`. Composite indirection is resolved (and
	// only local `./` paths are: a third-party action's contents are not in this
	// checkout to read), because a step relocated into `.github/actions/**` is
	// still run by this job. Resolution is fail-closed — an unreadable or
	// unparseable local action Fatals rather than contributing nothing, so
	// "moved into an action we could not read" can never render as "runs no
	// go test" OR as a pass.
	jobText += "\n" + ciResolveLocalComposites(t, wfName, job.id, jobText)

	// (f) the job must still run the thing that runs the gate.
	//
	// Command-gate form (#607): some gates run a BINARY, not a test —
	// tree-sweep runs `go run ./tools/pubmanifest stage` then `go run
	// ./tools/leaksweep run --tree`, so there is no `go test` to find. The guard
	// instead requires each declared fragment to appear verbatim in the job body.
	// Dropping, renaming, or splitting an invocation defangs the gate exactly the
	// way deleting a `go test` step would, and reddens here.
	if len(runInvokes) > 0 {
		for _, frag := range runInvokes {
			if !strings.Contains(jobText, frag) {
				fail("jobs.%s no longer invokes %q — the command that runs this gate was removed, "+
					"renamed, or split; a gate whose command is gone is as advisory as one whose "+
					"filter excludes what it reads", job.id, frag)
			}
		}
		return
	}

	// go-test form: the job must still run `go test` in the module.
	dirs := ciGoTestDirs(jobText)
	if len(dirs) == 0 {
		fail("jobs.%s runs no `go test` at all", job.id)
		return
	}
	if !ciContains(dirs, module) {
		fail("jobs.%s runs `go test` in %v, not in %s", job.id, dirs, module)
	}
}

// ciLocalUsesRe matches a `uses: ./<path>` step — a composite action living in
// this repo. Third-party (`owner/repo@ref`) and reusable-workflow forms are
// deliberately NOT matched: their bodies are not in the checkout, so following
// them is impossible and pretending otherwise would be the fail-open this file
// exists to prevent. A row registered against a job whose `go test` lives in a
// third-party action still fails red, as it did before.
var ciLocalUsesRe = regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*(\./[^\s#]+)`)

// ciResolveLocalComposites returns the concatenated `run:` text of every local
// composite action the job `uses:`. One level deep: an action that itself
// `uses:` another local action Fatals rather than being silently ignored.
func ciResolveLocalComposites(t *testing.T, wfName, jobID, jobText string) string {
	t.Helper()

	// repoRoot: this package is tools/desk/internal/deskkit.
	const repoRoot = "../../../.."

	var out strings.Builder
	for _, m := range ciLocalUsesRe.FindAllStringSubmatch(jobText, -1) {
		ref := strings.TrimSuffix(m[1], "/")

		var body []byte
		var readErr error
		found := ""
		for _, name := range []string{"action.yml", "action.yaml"} {
			p := filepath.Join(repoRoot, filepath.FromSlash(strings.TrimPrefix(ref, "./")), name)
			b, err := os.ReadFile(p)
			if err == nil {
				body, found = b, p
				break
			}
			readErr = err
		}
		if found == "" {
			t.Fatalf("%s: jobs.%s uses local action %q, but neither action.yml nor action.yaml is "+
				"readable there (%v). This guard must never treat an unreadable action as "+
				"contributing no steps — that would let `go test` be moved into a broken action "+
				"and read as either a pass or an unrelated failure.", wfName, jobID, ref, readErr)
		}

		text := string(body)
		if !strings.Contains(text, "using: 'composite'") && !strings.Contains(text, `using: "composite"`) && !strings.Contains(text, "using: composite") {
			t.Fatalf("%s: jobs.%s uses local action %q, but %s does not declare `runs.using: composite` — "+
				"this parser models composite actions only. Re-point the row, or extend the parser "+
				"DELIBERATELY; do not let an unmodelled shape read as covered.", wfName, jobID, ref, found)
		}
		if ciLocalUsesRe.MatchString(text) {
			t.Fatalf("%s: jobs.%s uses local action %q, which itself `uses:` another local action. "+
				"This parser resolves ONE level; nesting must be modelled deliberately rather than "+
				"silently truncated.", wfName, jobID, ref)
		}
		out.WriteString(ciExpandActionVars(t, found, text))
		out.WriteString("\n")
	}
	return out.String()
}

var (
	// `  name:` at two-space indent under `inputs:`, and its `default:` child.
	ciActionInputRe   = regexp.MustCompile(`(?m)^  ([A-Za-z0-9_-]+):\s*$`)
	ciActionDefaultRe = regexp.MustCompile(`(?m)^    default:\s*(.+?)\s*$`)
	// `${{ inputs.foo }}` and `$FOO` / `${FOO}` / `"$FOO"`.
	ciInputsExprRe = regexp.MustCompile(`\$\{\{\s*inputs\.([A-Za-z0-9_-]+)\s*\}\}`)
	ciShellVarRe   = regexp.MustCompile(`\$\{?([A-Z][A-Z0-9_]*)\}?`)
	// An `env:` binding line: `        VAR: value`.
	ciEnvBindRe = regexp.MustCompile(`(?m)^\s+([A-Z][A-Z0-9_]*):\s*(.+?)\s*$`)
)

// ciExpandActionVars resolves a composite action's own indirection so the
// `cd <dir> && go test` matcher sees concrete paths: `${{ inputs.X }}` becomes
// X's declared default, and `$VAR` becomes whatever an `env:` block binds it to.
//
// Substituting the DEFAULT is the conservative reading — it is what runs when a
// caller passes nothing. A caller that overrides the input to a different module
// is a shape this parser does not model, and (e)'s `runs go test in %v, not in
// %s` fires rather than passing.
func ciExpandActionVars(t *testing.T, path, text string) string {
	t.Helper()

	defaults := map[string]string{}
	// Slice out the inputs: block so a `default:` elsewhere cannot be captured.
	if i := strings.Index(text, "\ninputs:"); i >= 0 {
		block := text[i:]
		if j := strings.Index(block, "\nruns:"); j >= 0 {
			block = block[:j]
		}
		names := ciActionInputRe.FindAllStringSubmatchIndex(block, -1)
		for k, loc := range names {
			end := len(block)
			if k+1 < len(names) {
				end = names[k+1][0]
			}
			name := block[loc[2]:loc[3]]
			if d := ciActionDefaultRe.FindStringSubmatch(block[loc[1]:end]); d != nil {
				defaults[name] = strings.Trim(d[1], `"'`)
			}
		}
	}

	expandInputs := func(s string) string {
		return ciInputsExprRe.ReplaceAllStringFunc(s, func(m string) string {
			name := ciInputsExprRe.FindStringSubmatch(m)[1]
			if v, ok := defaults[name]; ok {
				return v
			}
			t.Fatalf("%s: references ${{ inputs.%s }}, which declares no default — this parser "+
				"cannot know what the caller passes. Give the input a default, or extend the "+
				"parser DELIBERATELY.", path, name)
			return m
		})
	}

	text = expandInputs(text)

	env := map[string]string{}
	for _, m := range ciEnvBindRe.FindAllStringSubmatch(text, -1) {
		env[m[1]] = strings.Trim(m[2], `"'`)
	}
	return ciShellVarRe.ReplaceAllStringFunc(text, func(m string) string {
		name := ciShellVarRe.FindStringSubmatch(m)[1]
		if v, ok := env[name]; ok {
			return v
		}
		return m
	})
}

// ciMatrixVarRe matches a `${{ matrix.<key> }}` expression, tolerating whitespace.
var ciMatrixVarRe = regexp.MustCompile(`\$\{\{\s*matrix\.[A-Za-z0-9_-]+\s*\}\}`)

// ciGoTestDirRe matches `cd <dir> && go test`, the shape every job in this repo
// uses (statusgen and each tools/ module are separate Go modules, so every test
// step cd's into one first).
var ciGoTestDirRe = regexp.MustCompile(`cd\s+([^\s&|;]+)\s*&&\s*go\s+test`)

func ciGoTestDirs(jobText string) []string {
	var dirs []string
	for _, m := range ciGoTestDirRe.FindAllStringSubmatch(jobText, -1) {
		dirs = append(dirs, strings.Trim(m[1], `"'`))
	}
	return dirs
}

func ciContains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func ciAllowedEvents(m map[string]bool) []string {
	var out []string
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	return out
}

// workflowEventPaths extracts on.<event>.paths from a workflow file.
//
// A missing block is a Fatal, never a silent empty list — "no paths found" must
// not read as "everything is covered".
func workflowEventPaths(t *testing.T, content, name, event string) []string {
	t.Helper()
	globs, _ := ciOnEventKey(t, content, name, event, "paths")
	if len(globs) == 0 {
		t.Fatalf("%s: on.%s.paths is absent or empty. If the filter was removed the workflow now "+
			"runs on every change, which is safe but should be deliberate; if the file's shape "+
			"changed, re-point this parser — list items must be indented UNDER `paths:` "+
			"(`      - \"x/**\"`), not at the key's own indent. Never let this read as \"covered\".",
			name, event)
	}
	return globs
}

// ciOnEventKey extracts on.<event>.<key> from a workflow file, handling both the
// block form (`paths:` then `- item` lines) and the inline flow form
// (`branches: [main]`). The bool reports whether the key was present at all,
// which is distinct from present-but-empty.
//
// Hand-rolled rather than yaml-parsed on purpose: tools/desk is deliberately
// dependency-free (no require block, no go.sum), and these files are ours, with
// a fixed 2/4/6-space shape.
func ciOnEventKey(t *testing.T, content, name, event, key string) ([]string, bool) {
	t.Helper()

	lines := strings.Split(content, "\n")

	// Locate `on:` at column 0.
	onIdx := -1
	for i, ln := range lines {
		if strings.TrimRight(ln, " \t") == "on:" {
			onIdx = i
			break
		}
	}
	if onIdx < 0 {
		t.Fatalf("%s: no top-level `on:` block found — cannot verify its triggers", name)
	}

	// Within `on:`, locate `  <event>:` and then `    <key>:`.
	inEvent := false
	inKey := false
	found := false
	var vals []string
	for _, ln := range lines[onIdx+1:] {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))

		if indent == 0 {
			break // left the `on:` block entirely
		}
		if indent <= 2 {
			inEvent = trimmed == event+":"
			inKey = false
			continue
		}
		if !inEvent {
			continue
		}
		if indent <= 4 {
			inKey = false
			if trimmed == key+":" {
				inKey, found = true, true
			} else if rest, isKey := strings.CutPrefix(trimmed, key+":"); isKey {
				found = true
				vals = append(vals, ciParseFlowSequence(t, name, event+"."+key, rest)...)
			}
			continue
		}
		if !inKey {
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			inKey = false
			continue
		}
		vals = append(vals, unquoteYAMLScalar(strings.TrimSpace(trimmed[2:])))
	}
	return vals, found
}

// ciParseFlowSequence parses `[a, "b", 'c']`. Anything else on the right-hand side
// of a key we were asked about is a shape this parser does not model, and it
// Fatals rather than guessing — a wrong "covered" is the failure mode this file
// exists to prevent.
func ciParseFlowSequence(t *testing.T, name, where, rest string) []string {
	t.Helper()
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil
	}
	if i := strings.IndexByte(rest, '#'); i >= 0 && !strings.HasPrefix(rest, "[") {
		rest = strings.TrimSpace(rest[:i])
	}
	if !strings.HasPrefix(rest, "[") || !strings.HasSuffix(rest, "]") {
		t.Fatalf("%s: on.%s is %q — this parser models only a block list or an inline "+
			"[a, b] flow sequence. Teach it the new shape; do NOT let it read as absent.",
			name, where, rest)
	}
	inner := strings.TrimSpace(rest[1 : len(rest)-1])
	if inner == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(inner, ",") {
		if v := unquoteYAMLScalar(strings.TrimSpace(part)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ciWorkflowJobLines returns the body lines of jobs.<id>, and whether it exists.
// It assumes `jobs:` sits at column 0 and job ids at indent 2 — true of every
// workflow in this repo, and Fatal if the `jobs:` block itself is missing.
func ciWorkflowJobLines(t *testing.T, content, name, id string) ([]string, bool) {
	t.Helper()

	lines := strings.Split(content, "\n")
	jobsIdx := -1
	for i, ln := range lines {
		if strings.TrimRight(ln, " \t") == "jobs:" {
			jobsIdx = i
			break
		}
	}
	if jobsIdx < 0 {
		t.Fatalf("%s: no top-level `jobs:` block found — cannot verify that any job runs", name)
	}

	inJob := false
	var body []string
	for _, ln := range lines[jobsIdx+1:] {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if inJob {
				body = append(body, ln)
			}
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if indent == 0 {
			break // left the `jobs:` block
		}
		if indent <= 2 {
			if inJob {
				break // next job id ends this one
			}
			inJob = trimmed == id+":"
			continue
		}
		if inJob {
			body = append(body, ln)
		}
	}
	return body, inJob
}

// ciJobMatrixValues gathers every scalar under a job's `matrix:` block. It is
// deliberately loose about WHICH matrix key a value came from — the question it
// answers is "is this module still in the matrix at all", and a value that has
// moved to a different key still generates a job.
func ciJobMatrixValues(t *testing.T, jobLines []string, name, id string) []string {
	t.Helper()

	matrixIdx, matrixIndent := -1, 0
	for i, ln := range jobLines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "matrix:" {
			matrixIdx = i
			matrixIndent = len(ln) - len(strings.TrimLeft(ln, " "))
			break
		}
	}
	if matrixIdx < 0 {
		t.Fatalf("%s: jobs.%s has no `matrix:` block, but the registry says its check name is "+
			"matrix-generated. Either the matrix was removed (in which case the check name "+
			"changed and every consumer of it broke) or this row needs re-pointing.", name, id)
	}

	var vals []string
	for _, ln := range jobLines[matrixIdx+1:] {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent := len(ln) - len(strings.TrimLeft(ln, " ")); indent <= matrixIndent {
			break
		}
		if item, isItem := strings.CutPrefix(trimmed, "- "); isItem {
			trimmed = strings.TrimSpace(item) // `- include`-style mapping falls through below
			if !strings.Contains(trimmed, ":") {
				vals = append(vals, unquoteYAMLScalar(trimmed))
				continue
			}
		}
		i := strings.IndexByte(trimmed, ':')
		if i < 0 {
			continue
		}
		rest := strings.TrimSpace(trimmed[i+1:])
		if rest == "" {
			continue // a block list follows; its items are picked up above
		}
		if strings.HasPrefix(rest, "[") {
			vals = append(vals, ciParseFlowSequence(t, name, "jobs."+id+".matrix", rest)...)
			continue
		}
		vals = append(vals, unquoteYAMLScalar(rest))
	}
	if len(vals) == 0 {
		t.Fatalf("%s: jobs.%s's `matrix:` block parsed to zero values — re-point this parser; "+
			"an empty matrix must not read as \"the module is present\".", name, id)
	}
	return vals
}

var (
	ciEventNameEqRe  = regexp.MustCompile(`github\.event_name\s*==\s*['"]([a-z_]+)['"]`)
	ciEventNameNeqRe = regexp.MustCompile(`github\.event_name\s*!=\s*['"]([a-z_]+)['"]`)
)

// ciEvents is the set ciJobEventGate reasons over. It is not GitHub's full event
// list — it is the two events this guard asserts coverage for.
var ciEvents = []string{"pull_request", "push"}

// ciJobEventGate reads a job's own `if:` and reports which of ciEvents it can
// still run for, plus whether it is gated at all. It models exactly two shapes:
// `github.event_name == 'x'` and `github.event_name != 'x'`. If the expression
// mentions github.event_name in any other shape it Fatals — an `if:` this
// parser cannot read must never be assumed permissive.
func ciJobEventGate(t *testing.T, jobLines []string, name, id string) (map[string]bool, bool) {
	t.Helper()

	childIndent := -1
	for _, ln := range jobLines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent := len(ln) - len(strings.TrimLeft(ln, " ")); childIndent < 0 || indent < childIndent {
			childIndent = indent
		}
	}

	expr := ""
	for _, ln := range jobLines {
		trimmed := strings.TrimSpace(ln)
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if indent != childIndent {
			continue // step-level `if:` is not modelled; see LIMITS
		}
		if rest, isIf := strings.CutPrefix(trimmed, "if:"); isIf {
			expr = strings.TrimSpace(rest)
			break
		}
	}

	allowed := map[string]bool{}
	if expr == "" {
		for _, e := range ciEvents {
			allowed[e] = true
		}
		return allowed, false
	}
	if expr == "|" || expr == ">" || strings.HasPrefix(expr, "|") || strings.HasPrefix(expr, ">") {
		t.Fatalf("%s: jobs.%s has a multi-line `if:`, which this parser does not model — "+
			"teach it the shape rather than assuming the job runs.", name, id)
	}
	if strings.TrimSpace(strings.Trim(strings.TrimSuffix(strings.TrimPrefix(expr, "${{"), "}}"), " ")) == "false" {
		t.Errorf("%s: jobs.%s is `if: false` — it can never run", name, id)
		return allowed, true
	}
	if !strings.Contains(expr, "github.event_name") {
		// Gated on something orthogonal to the event (inputs, needs, labels).
		// Not modelled — treated as permissive, and recorded in LIMITS.
		for _, e := range ciEvents {
			allowed[e] = true
		}
		return allowed, false
	}

	eqs := ciEventNameEqRe.FindAllStringSubmatch(expr, -1)
	neqs := ciEventNameNeqRe.FindAllStringSubmatch(expr, -1)
	if len(eqs) == 0 && len(neqs) == 0 {
		t.Fatalf("%s: jobs.%s's `if:` mentions github.event_name in a shape this parser does not "+
			"model (%q) — teach it the shape rather than assuming the job runs.", name, id, expr)
	}
	if len(eqs) > 0 {
		for _, m := range eqs {
			allowed[m[1]] = true
		}
	} else {
		for _, e := range ciEvents {
			allowed[e] = true
		}
	}
	for _, m := range neqs {
		allowed[m[1]] = false
	}
	return allowed, true
}

// unquoteYAMLScalar strips surrounding quotes and any trailing `# comment`.
func unquoteYAMLScalar(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		if end := strings.IndexByte(s[1:], s[0]); end >= 0 {
			return s[1 : 1+end]
		}
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// anyGlobMatches reports whether the workflow's path globs admit path, using
// GitHub's filter-pattern semantics: `*` matches any run of characters
// except `/`, `**` matches any run including `/`, `?` matches one non-`/`
// character. Negation (`!`) follows GitHub's paths rule: the workflow runs if
// ANY positive glob matches AND NO negative glob matches — so a path that
// matches both a positive and a negative is EXCLUDED, and a negative alone can
// never admit anything. (Needed for the regen loop guard: statusgen.yml negates
// docs/streams/.history.jsonl so the bot's own history append cannot retrigger
// the workflow — same shape as the tracker's status-regen.yml.)
func anyGlobMatches(t *testing.T, globs []string, path string) bool {
	t.Helper()
	matched := false
	for _, g := range globs {
		negated := strings.HasPrefix(g, "!")
		if negated {
			g = strings.TrimPrefix(g, "!")
		}
		re, err := globToRegexp(g)
		if err != nil {
			t.Fatalf("path filter %q does not translate to a regexp: %v", g, err)
		}
		if !re.MatchString(path) {
			continue
		}
		if negated {
			return false
		}
		matched = true
	}
	return matched
}

func globToRegexp(g string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); i++ {
		switch g[i] {
		case '*':
			if i+1 < len(g) && g[i+1] == '*' {
				i++
				b.WriteString(".*")
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(g[i : i+1]))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// ciRegistryOptOut lists the test files the staleness scanner below flags that
// are NOT cross-module readers, each with the reason a human checked. An entry
// here is a CLAIM, and the scanner keeps it honest: an opt-out whose file is
// gone, or that the scanner no longer flags, is a hard failure — so the list
// cannot rot into a blanket suppression.
var ciRegistryOptOut = map[string]string{
	"tools/harnesslint/lint_test.go": `filepath.Join(refs, "..", "skills") joins ".." onto a t.TempDir() ` +
		`returned by copyRefs, deliberately pointing at a NONEXISTENT sibling of the temp dir to exercise the ` +
		`absent-roster could-not-check path (TestCheckBindings_ClosureFailsWithoutRoster / ` +
		`TestCheckBindings_CleanClosureAbsentRoster_CouldNotCheck). It is never resolved to a real tree. Every ` +
		`actual read in this file is either os.ReadFile under testdata/ (in-module) or under t.TempDir(); nothing ` +
		`leaves tools/harnesslint, and tools/** already covers the module`,
	"tools/desk/internal/deskkit/echocoverage_test.go": `filepath.Join("../../cmd", …) from internal/deskkit is ` +
		`tools/desk/cmd — this file's OWN module, not a sibling one. It enumerates every command directory and ` +
		`reads their non-test .go sources to pin the P3 echo + tool-class declaration across all of them ` +
		`(the P3-echo mutation), so "tools/**" already triggers this job on any edit it could observe`,
	"tools/desk/internal/deskkit/structured_test.go": `filepath.Join("..", "..") from internal/deskkit is ` +
		`tools/desk — this file's OWN module root, not a sibling one. TestPathRuleDifferential walks it to ` +
		`harvest a real population of base64ish path RUNS for the #410 false-positive ` +
		`measurement (a hand-written path list contains only paths the author already believed would pass, ` +
		`which is the circularity #410 caught in the comment it corrected). It reads file NAMES, never ` +
		`content, and "tools/**" already triggers this job on any edit that could change the population`,
	"tools/desk/internal/fleetharness/fleetharness_test.go": `filepath.Join("..", "..") from ` +
		`internal/fleetharness is tools/desk — this file's OWN module root, not a sibling one. It is the ` +
		`working directory the harness hands to ` + "`go build ./cmd/<verb>`" + ` so the acceptance run drives ` +
		`the REAL binaries (#517's defect was invisible to in-process unit tests, which is why ` +
		`this harness exists). Every other path it touches is under t.TempDir(): the origins, the clone, the ` +
		`worktrees, the private HOME and the PATH shims. "tools/**" already covers the module it builds`,
	"tools/desk/cmd/deskwt/deskwt_test.go": `"../escape" and ".." are REJECTED-INPUT fixtures for deskwt's ` +
		`worktree-name validator; nothing resolves or opens them`,
	"tools/prsync/main_test.go": `"../outside.md" and "docs/../STATUS.md" are traversal fixtures fed to ` +
		`prsync's path validator; the reads in this file are all under t.TempDir()`,
	"tools/desk/cmd/deskgit/deskgit_test.go": `"../../example-org/tracker" is a parseRepo INPUT ` +
		`vector pinning documented residual 2 (bare local paths take the lenient branch) — parseRepo is pure ` +
		`string parsing, so it is never resolved or opened. The file's only read is os.ReadFile of ` +
		`$HOME/.config/assay/audit.jsonl with HOME bound to t.TempDir()`,
	"tools/desk/cmd/deskboard/inventory_test.go": `filepath.Join("..", "..", "README.md") from cmd/deskboard is ` +
		`tools/desk/README.md — this file's OWN module, and the doc-parity check's whole point is that the ` +
		`enumerated verb list beside the code stays true. In-module, so tools/** already covers it. The ` +
		`file's only other reads are parser.ParseFile of board.go and main.go in this same package`,
	"tools/desk/cmd/reviewloop/actiontable_test.go": `"../deskboard/board.go" from cmd/reviewloop is ` +
		`tools/desk/cmd/deskboard — this file's OWN module, not a sibling one. The read parses deskboard's act* ` +
		`ACTION constants so the board-reactor's action table is DERIVED from them rather than hand-kept beside ` +
		`them: the pr-review-desk skill's nine-action description had already drifted nine ` +
		`actions behind the board, and a second hand-written list is exactly how a board state disappears from a ` +
		`consumer with nothing saying so. In-module, so "tools/**" already triggers this job on any edit to ` +
		`either side of the coupling`,
	"tools/desk/cmd/deskboard/nextup_test.go": `tracker + "/../" + base(tracker) is an UNCLEANED SPELLING of a t.TempDir() ` +
		`root, built by concatenation on purpose so that path resolution is observable; it never leaves the temp dir`,
	"statusgen/roadmap_test.go": `"../../streams/findings/<entry>.md" is the HREF the roadmap deck emits, ` +
		`resolved the way a browser would to prove it lands on a real file. It is joined onto a docs/reports/roadmap ` +
		`dir built under t.TempDir(), so the ".." segments walk within the temp tree, never out of the module. The ` +
		`file's only read is that one os.Stat; findingFileByID's ReadDir/ReadFile are likewise handed t.TempDir() roots`,
	"tools/desk/cmd/deskboard/health_test.go": `filepath.Join("..", "..", "README.md") from cmd/deskboard is ` +
		`tools/desk/README.md — this file's OWN module root, not a sibling module. The read pins the README's ` +
		`"do not probe" verb list against the mainHealth-absence guard (#295/#377 R1), and "tools/**" already ` +
		`triggers this job on any edit to it`,
	"statusgen/consumers_test.go": `"../outside/thing.md" is a REJECTED-INPUT fixture for classifySite's ` +
		`root-escape guard — the assertion is that it is refused, so it is never resolved or opened. Every ` +
		`read in this file is under t.TempDir(), and statusgen/** already covers the file's own module`,
	"statusgen/verifyrun_test.go": `filepath.Abs("..") from the statusgen package dir resolves to the REPO ` +
		`ROOT, but only as a round trip: it is immediately rejoined to "statusgen/testdata/verifyrun/…", so every ` +
		`file this test opens is back inside its own module. The root is also handed to runVerifyCommand as the ` +
		`subshell cwd, and the two shipped fixtures' Verify rows are "true", "printf", "grep -c ." and ` +
		`"test -f statusgen/testdata/verifyrun/brief-pass.md" — nothing they touch is outside statusgen/ either. ` +
		`Both the test and the fixtures that decide what it reaches live under statusgen/**, so that path filter ` +
		`already triggers this job on any edit that could widen the reach`,
	"tools/pubmanifest/stage_test.go": `"../../enforcement-model.md" and "../INTAKE.md" are INPUT vectors for ` +
		`resolveRef and boundaryReport, which are pure string functions over a doc-tree reference — nothing ` +
		`resolves or opens them, and the paths they name are deliberately WITHHELD ones that must never be ` +
		`read. Every os.* call in this file is rooted at t.TempDir(): the source repo, the --out directory ` +
		`and the sentinel are all built there`,
}

// ciTestFileReads matches the filesystem-read calls that make a test file a
// candidate reader. A file that names none of these cannot be reading anything.
var ciTestFileReads = regexp.MustCompile(
	`os\.ReadFile|os\.ReadDir|os\.Open|os\.Stat|os\.Lstat|parser\.ParseFile|filepath\.Walk|filepath\.Glob`)

// ciGoStringLit matches a double-quoted Go string literal.
var ciGoStringLit = regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"`)

// ciHasParentSegment reports whether any double-quoted literal in src has a
// path segment that is exactly "..". Segment-exact on purpose: a substring test
// for ".." false-positives on every `"./..."` and `"…;num:title;..."` in the
// repo, which is what made the earlier textual heuristic look unusable.
func ciHasParentSegment(src string) bool {
	for _, q := range ciGoStringLit.FindAllString(src, -1) {
		for _, seg := range strings.Split(q[1:len(q)-1], "/") {
			if seg == ".." {
				return true
			}
		}
	}
	return false
}

// TestCrossModuleReaderRegistryIsNotSilentlyStale makes the hand-maintained
// registry SELF-CHECKING, at file granularity.
//
// The desk review of #205 asked whether the registry could stop going stale in
// silence, having caught it doing exactly that: ownedrepos_coupling_test.go
// arrived via a merge of main seventeen minutes after the sweep and nothing
// noticed. Full auto-discovery is still the wrong tool — resolving WHICH paths a
// test reads needs intra-procedural dataflow (scancoupling_test.go builds its
// path from five separate string arguments; ownedrepos_coupling_test.go assigns
// the join to a variable and reads the variable), and a path-resolving heuristic
// false-positives on real content here: cmd/writeguard/guard_test.go compares a
// token against "../../../STATUS.md", which resolves to a file that genuinely
// exists outside its module and is never read.
//
// So this does the cheaper, exact thing. It does not try to work out what a file
// reads. It asks a FILE-level question with no resolution and no guessing:
//
//	does this _test.go both (a) name a filesystem-read call and (b) contain a
//	string literal with a ".." path segment?
//
// If yes, a human must have classified it — either as a registry row, or as an
// opt-out with a reason. It is a forcing function, not an oracle. Every one of
// the four registry rows is flagged by it, and so is the file that went missing,
// so it would have caught the very miss that prompted it.
//
// BOUND, stated rather than implied: a reader that escapes its module WITHOUT a
// ".." literal is not flagged — cmd/verifyloop/dispatch_sync_test.go walks up to
// the checkout root by filepath.Dir, holding no ".." literal. Since the skill
// moved into this repo it is no longer consumer-gated but a REGISTERED
// cross-module reader (its trigger is pinned in the registry regardless of the
// scanner), so it is covered even though this scanner cannot see it. The class is
// still real for an UNregistered such reader, and the hand sweep in the doc
// comment above is still the backstop. This check narrows the hole; it does not
// close it.
func TestCrossModuleReaderRegistryIsNotSilentlyStale(t *testing.T) {
	const repoRoot = "../../../.."

	skipIfFixtureAbsent(t, filepath.Join(repoRoot, ".github", "workflows", "tools.yml"),
		".github/ and the sibling tool tree are not part of this repository's published file set")

	registered := map[string]bool{}
	for _, e := range ciCrossModuleRegistry() {
		registered[e.test] = true
	}

	flagged := map[string]bool{}
	scanned := 0
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		scanned++
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		src := string(raw)
		if !ciTestFileReads.MatchString(src) || !ciHasParentSegment(src) {
			return nil
		}
		rel, rerr := filepath.Rel(repoRoot, path)
		if rerr != nil {
			return rerr
		}
		flagged[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", repoRoot, err)
	}
	if scanned == 0 {
		t.Fatal("scanned 0 test files — this check proved nothing; re-point repoRoot")
	}

	for rel := range flagged {
		if registered[rel] || ciRegistryOptOut[rel] != "" {
			continue
		}
		t.Errorf("UNCLASSIFIED cross-module-reader candidate: %s\n"+
			"It names a filesystem read AND contains a \"..\" path segment, so it may read outside its own "+
			"Go module — and a test whose CI trigger does not cover what it reads is advisory, not enforced "+
			"(#199).\n"+
			"Decide, do not silence: if it DOES read out of module, add a row to ciCrossModuleRegistry "+
			"(and make sure the workflow's paths: cover the read). If it does NOT, add it to "+
			"ciRegistryOptOut with the reason you checked.", rel)
	}

	for rel, reason := range ciRegistryOptOut {
		if registered[rel] {
			t.Errorf("%s is in BOTH ciCrossModuleRegistry and ciRegistryOptOut — it cannot be both", rel)
			continue
		}
		if _, serr := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(rel))); serr != nil {
			t.Errorf("stale opt-out: %s no longer exists (%v). Delete the row; a suppression for a file "+
				"that is gone is how a suppression list turns into a blanket one.\nIts reason was: %s",
				rel, serr, reason)
			continue
		}
		if !flagged[rel] {
			t.Errorf("stale opt-out: %s is no longer flagged by the scanner, so the suppression is dead "+
				"weight — delete the row.\nIts reason was: %s", rel, reason)
		}
	}
}

// TestCITriggerParsers covers the hand-rolled YAML readers that half (1) of the
// guard above depends on, against synthetic content rather than the real
// workflows — so a parser that silently degrades (and would therefore start
// answering "reachable" for everything) is caught here rather than by the
// absence of a failure over there.
//
// Only the non-Fatal paths are asserted: the Fatal branches are exercised
// end-to-end by the mutation matrix recorded on #205 (a legal YAML reflow, a
// negated glob, an unmodelled `if:` shape all Fatal by design).
func TestCITriggerParsers(t *testing.T) {
	const wf = `name: sample
on:
  pull_request:
    branches: [main, "release/*"]
    paths:
      - "tools/**"
      - "statusgen/**"   # trailing comment
  push:
    branches-ignore: [wip]
    paths:
      - "tools/**"

jobs:
  test:
    strategy:
      matrix:
        module: [tools/desk, tools/prsync]
    steps:
      - run: cd ${{ matrix.module }} && go test ./...
  lint:
    if: github.event_name == 'pull_request'
    steps:
      - run: cd statusgen && go test ./...
  regen:
    if: ${{ github.event_name == 'push' && !contains(github.event.head_commit.message, '[skip]') }}
    steps:
      - run: cd statusgen && go test ./...
  notpush:
    if: github.event_name != 'push'
    steps:
      - run: cd statusgen && go test ./...
`

	t.Run("onEventKey", func(t *testing.T) {
		if got := workflowEventPaths(t, wf, "sample", "pull_request"); !equalStrings(got, []string{"tools/**", "statusgen/**"}) {
			t.Errorf("pull_request paths = %v", got)
		}
		if got, ok := ciOnEventKey(t, wf, "sample", "pull_request", "branches"); !ok || !equalStrings(got, []string{"main", "release/*"}) {
			t.Errorf("pull_request branches = %v (present=%v); the inline flow form must parse", got, ok)
		}
		if _, ok := ciOnEventKey(t, wf, "sample", "push", "branches"); ok {
			t.Error("push has no branches: — absent must report absent, not empty-but-present")
		}
		if got, ok := ciOnEventKey(t, wf, "sample", "push", "branches-ignore"); !ok || !equalStrings(got, []string{"wip"}) {
			t.Errorf("push branches-ignore = %v (present=%v); branches: must not be mistaken for branches-ignore:", got, ok)
		}
	})

	// The composite-action resolver. Relocating a step into
	// .github/actions/** must not read as "the job runs no go test" — but the
	// resolution has to survive the action's own indirection, or it would only
	// appear to work.
	t.Run("compositeActionVarExpansion", func(t *testing.T) {
		const action = `name: 'Assay Lint Gate'
inputs:
  statusgen_path:
    description: 'Path to the module'
    required: false
    default: 'statusgen'
  root:
    description: 'Docs root'
    required: false
    default: '.'
runs:
  using: 'composite'
  steps:
    - name: test
      env:
        STATUSGEN_PATH: ${{ inputs.statusgen_path }}
      run: cd "$STATUSGEN_PATH" && go test ./...
      shell: bash
    - name: test again, direct interpolation
      run: cd ${{ inputs.statusgen_path }} && go test ./...
      shell: bash
    - name: lint
      run: cd ${{ inputs.statusgen_path }} && go run . --root ${{ inputs.root }} --lint
      shell: bash
`
		expanded := ciExpandActionVars(t, "fixture/action.yml", action)
		if got := ciGoTestDirs(expanded); !equalStrings(got, []string{"statusgen", "statusgen"}) {
			t.Errorf("go test dirs = %v; both the `env:`-indirect and the direct "+
				"${{ inputs.* }} form must resolve to the input's declared default — "+
				"if either stops resolving, a job that DOES run the suite reads as one that does not", got)
		}
		// `go run` is not `go test`: the matcher must not count the lint step.
		if strings.Count(expanded, "go test") != 2 {
			t.Errorf("expanded action has %d `go test` occurrences, want 2", strings.Count(expanded, "go test"))
		}
		if !strings.Contains(expanded, "--root . --lint") {
			t.Errorf("inputs.root did not expand to its default:\n%s", expanded)
		}
		// A step that runs no go test must not be conjured into one.
		if got := ciGoTestDirs(ciExpandActionVars(t, "fixture/action.yml",
			"inputs:\n  p:\n    default: 'statusgen'\nruns:\n  using: 'composite'\n  steps:\n    - run: echo hi\n")); len(got) != 0 {
			t.Errorf("go test dirs = %v for an action that runs no go test; want none", got)
		}
	})

	t.Run("jobLinesAndMatrix", func(t *testing.T) {
		if _, ok := ciWorkflowJobLines(t, wf, "sample", "nosuchjob"); ok {
			t.Error("a job id that does not exist must report absent")
		}
		lines, ok := ciWorkflowJobLines(t, wf, "sample", "test")
		if !ok {
			t.Fatal("jobs.test not found")
		}
		if got := ciJobMatrixValues(t, lines, "sample", "test"); !equalStrings(got, []string{"tools/desk", "tools/prsync"}) {
			t.Errorf("matrix values = %v", got)
		}
		body := ciMatrixVarRe.ReplaceAllString(strings.Join(lines, "\n"), "tools/desk")
		if got := ciGoTestDirs(body); !equalStrings(got, []string{"tools/desk"}) {
			t.Errorf("go test dirs = %v; the matrix variable must expand", got)
		}
		// A job id must not bleed into the next one.
		lintLines, _ := ciWorkflowJobLines(t, wf, "sample", "lint")
		if strings.Contains(strings.Join(lintLines, "\n"), "regen") {
			t.Error("jobs.lint's body leaked into the following job")
		}
	})

	t.Run("eventGate", func(t *testing.T) {
		for _, tc := range []struct {
			job     string
			gated   bool
			allowPR bool
			allowPu bool
		}{
			{"test", false, true, true},    // no if: at all
			{"lint", true, true, false},    // == 'pull_request'
			{"regen", true, false, true},   // == 'push', &&-ed with something else
			{"notpush", true, true, false}, // != 'push'
		} {
			lines, ok := ciWorkflowJobLines(t, wf, "sample", tc.job)
			if !ok {
				t.Fatalf("jobs.%s not found", tc.job)
			}
			allowed, gated := ciJobEventGate(t, lines, "sample", tc.job)
			if gated != tc.gated || allowed["pull_request"] != tc.allowPR || allowed["push"] != tc.allowPu {
				t.Errorf("jobs.%s: gated=%v allowed=%v; want gated=%v pr=%v push=%v",
					tc.job, gated, allowed, tc.gated, tc.allowPR, tc.allowPu)
			}
		}
	})
}
