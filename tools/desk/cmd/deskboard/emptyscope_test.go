package main

// #489 — the read surface must report COULD-NOT-CHECK, never green-and-empty.
//
// The roster's fail-closed invariant held for the WRITE boundary (an unknown repo
// refuses at exit 5) and did not hold for the READ boundary: with no repo in the
// configured set, every board-wide verb swept zero repos and reported the empty sweep
// as a clean result at exit 0 — and `actions` went further and fabricated merges.
//
// These tests drive the REAL `run` entry point end-to-end (argv in, exit code and
// rendered output out) rather than calling the handlers directly, because the defect
// was never in a handler's logic: each handler did exactly what it was written to do
// with the set it was given. The defect was that nothing asked whether the set was
// worth sweeping. A test that called cmdActions directly would have to construct that
// question itself and would pass with the guard removed.
//
// Every test here was run against the UNFIXED tree and observed to fail with the
// production symptom before the fix was restored; the symptom each one reproduces is
// named in its doc comment so a future reader can re-run that experiment.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// partialRosterNoRepos is the state that actually reproduced #489, and the reason
// this whole file exists: it is not an ABSENT roster, it is a USABLE one with the
// scope missing. Config.Configured() answers TRUE here — it consults Problems, Bless
// and Logins and never consults Repos — so every "is the roster configured?" check in
// the tree is satisfied while there is nothing to read.
//
// Absent config failed loudly already. Partial config did not fail at all.
const partialRosterNoRepos = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=reviewer=assay-reviewer-app:300000004
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

// rememberedPRs are the ids a prior run recorded as OPEN. They are real, open PRs at
// the time #489 was filed — including #455, the PR carrying this fix, which the
// unfixed binary announced as merged.
var rememberedPRs = []string{
	"example-org/tracker#1775",
	"example-org/tracker#1771",
	"example-org/tracker#455",
}

// seedPriorOpenSet writes an audit ledger under HOME recording a deskboard run that
// saw rememberedPRs open two minutes ago — inside tombstonesFor's one-hour window.
// This is the memory the tombstone rule reasons from.
func seedPriorOpenSet(t *testing.T) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(
		`{"ts":%q,"tool":"deskboard","verb":"prs","argsDigest":"seed","repo":"","pr":null,`+
			`"headSHA":null,"result":"ok","detail":"open=%s","sourceSHA":"seed","builtAt":"seed",`+
			`"sessionTag":"at489-test"}`+"\n",
		time.Now().Add(-2*time.Minute).UTC().Format(time.RFC3339),
		strings.Join(rememberedPRs, " "))
	if err := os.WriteFile(filepath.Join(dir, "audit.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
}

// emptyScopeBoard installs the fake gh, then replaces the fixture roster with the
// partial one, then seeds the prior open set. The order is load-bearing: installFakeGH
// plants the FULL fixture roster under its own HOME, so the partial roster has to be
// installed after it or the repo set is not empty and none of this tests anything.
func emptyScopeBoard(t *testing.T) (ghLog string) {
	t.Helper()
	ghLog = installFakeGH(t)
	installBoardRoster(t, partialRosterNoRepos)
	seedPriorOpenSet(t)

	// Preconditions, asserted rather than assumed. If either of these stops holding,
	// every test in this file passes for the wrong reason.
	if !deskkit.EffectiveConfig().Configured() {
		t.Fatal("precondition: the partial roster must load as CONFIGURED — that is the whole " +
			"point of #489; if it now reports unconfigured, Configured() gained a Repos check " +
			"and these tests are asserting against a contract that changed")
	}
	if n := len(deskkit.AllowedRepos()); n != 0 {
		t.Fatalf("precondition: the allowed-repo set must be EMPTY, got %d", n)
	}
	return ghLog
}

// TestActions_EmptyScopeDoesNotFabricateMerges is the #489 headline.
//
// UNFIXED SYMPTOM (observed): exit 0, and one line per remembered PR reading
// "MERGED — drop from your list" — a positive statement that is false, carrying an
// instruction to discard the desk's entire queue.
func TestActions_EmptyScopeDoesNotFabricateMerges(t *testing.T) {
	emptyScopeBoard(t)

	var out, errb bytes.Buffer
	code := run([]string{"actions", "--table"}, &out, &errb)
	combined := out.String() + errb.String()

	if strings.Contains(combined, "MERGED") {
		t.Errorf("deskboard actions announced a MERGE it could not have observed — the repo set "+
			"was empty, so NO pr list was read and every remembered PR looked \"not seen\".\n"+
			"\"I saw it before and don't see it now\" is evidence of a merge only if the tool "+
			"could see anything at all.\noutput:\n%s", combined)
	}
	for _, id := range rememberedPRs {
		// The ids render short (tracker #1775), so check the number, which is what a
		// reader would act on.
		num := id[strings.LastIndex(id, "#"):]
		if strings.Contains(out.String(), num+" MERGED") {
			t.Errorf("remembered PR %s was tombstoned from an empty sweep", id)
		}
	}
	if code != deskkit.ExitUnverifiable {
		t.Errorf("exit = %d, want %d (unverifiable): a sweep of zero repos knows nothing, and "+
			"exit 0 tells every caller and every CI step that it does", code, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(combined, "COULD-NOT-CHECK") {
		t.Errorf("the refusal does not name the state it is in; output:\n%s", combined)
	}
}

// TestTombstonesFor_EmptyScopeIsNotEvidenceOfAMerge pins the unsound inference at its
// source, independently of the dispatch guard. dispatch makes this branch unreachable
// in the shipped binary today; this test is what stops a future caller from making it
// reachable again.
//
// UNFIXED SYMPTOM (observed): three tombstones returned from an empty scope.
func TestTombstonesFor_EmptyScopeIsNotEvidenceOfAMerge(t *testing.T) {
	installFakeGH(t)
	seedPriorOpenSet(t)

	// Positive control FIRST: with a non-empty scope and the same seeded memory, the
	// rule must still fire. Without this, a `return nil` at the top of tombstonesFor
	// would pass the test below and destroy the feature.
	scope := []string{"example-org/tracker", "medici-finance/assay"}
	if got := tombstonesFor(map[string]bool{}, scope); len(got) != len(rememberedPRs) {
		t.Fatalf("positive control: with a REAL scope and %d remembered PRs none of which are "+
			"open now, tombstonesFor returned %d tombstones, want %d — the rule itself is broken, "+
			"so the empty-scope assertion below would prove nothing",
			len(rememberedPRs), len(got), len(rememberedPRs))
	}

	if got := tombstonesFor(map[string]bool{}, nil); len(got) != 0 {
		t.Errorf("tombstonesFor returned %d tombstones from an EMPTY scope: %+v\n"+
			"currentOpen is empty here for a reason that has nothing to do with the PRs — "+
			"nothing was read.", len(got), got)
	}
}

// TestPolicyDrift_EmptyScopeIsNotAPass — policydrift is the drift GATE.
//
// UNFIXED SYMPTOM (observed): "no drift — compiled-in visibility matches GitHub" at
// exit 0, having iterated zero repos and made zero API calls.
//
// The gate already carries the right principle for the repos it DOES iterate ("a
// repo we could not read is NOT a pass"). It just never applied it to the case of
// having no repo to iterate.
func TestPolicyDrift_EmptyScopeIsNotAPass(t *testing.T) {
	emptyScopeBoard(t)

	var out, errb bytes.Buffer
	code := run([]string{"policydrift", "--table"}, &out, &errb)
	combined := out.String() + errb.String()

	if strings.Contains(combined, "no drift") {
		t.Errorf("policydrift reported a clean pass without checking anything — a gate that "+
			"cannot run must not report a pass.\noutput:\n%s", combined)
	}
	if code != deskkit.ExitUnverifiable {
		t.Errorf("exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
}

// TestHealth_EmptyScopeUsesItsOwnCouldNotCheckState — health already models three
// states (green / RED / COULD-NOT-CHECK) and reported zero-of-zero as the healthy one.
//
// UNFIXED SYMPTOM (observed): "0 green · 0 RED · 0 pending · 0 not-assessed ·
// 0 COULD-NOT-CHECK — scope: 0 watched repo(s)" at exit 0.
func TestHealth_EmptyScopeUsesItsOwnCouldNotCheckState(t *testing.T) {
	emptyScopeBoard(t)

	var out, errb bytes.Buffer
	code := run([]string{"health", "--table"}, &out, &errb)
	combined := out.String() + errb.String()

	if strings.Contains(combined, "0 green") {
		t.Errorf("health rendered a zero-of-zero summary as a result. Counting nothing is not "+
			"the same as finding nothing wrong, and this line is read as a green board.\n"+
			"output:\n%s", combined)
	}
	if code != deskkit.ExitUnverifiable {
		t.Errorf("exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
}

// TestStalled_EmptyScopeIsNotACleanSweep is the #515 headline. `stalled` iterates
// deskkit.AllowedRepos() exactly like the original five board-wide verbs
// but was left out of boardWideVerbs when the #489
// guard was added, so an empty scope reported a clean board instead of
// COULD-NOT-CHECK.
//
// UNFIXED SYMPTOM (observed): exit 0, stdout "asOf ... (stall window 48h; stalled 0,
// unassessable 0)" / "(no stalled drafts)" — indistinguishable from a real sweep that
// found nothing, exactly as the issue describes.
func TestStalled_EmptyScopeIsNotACleanSweep(t *testing.T) {
	ghLog := emptyScopeBoard(t)

	var out, errb bytes.Buffer
	code := run([]string{"stalled", "--table"}, &out, &errb)
	combined := out.String() + errb.String()

	if strings.Contains(combined, "no stalled drafts") {
		t.Errorf("stalled reported a clean sweep without checking anything — the repo set "+
			"was empty, so NO repo was listed.\noutput:\n%s", combined)
	}
	if code != deskkit.ExitUnverifiable {
		t.Errorf("exit = %d, want %d (unverifiable): a sweep of zero repos knows nothing, and "+
			"exit 0 tells every caller it is a real result", code, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(combined, "COULD-NOT-CHECK") {
		t.Errorf("the refusal does not name the state it is in; output:\n%s", combined)
	}
	// Same fail-closed timing requirement as the other board-wide verbs: the guard must fire
	// before any gh call, not after summarising an empty read.
	if b, err := os.ReadFile(ghLog); err == nil && len(bytes.TrimSpace(b)) > 0 {
		t.Errorf("stalled made gh calls despite having no repo to read:\n%s", b)
	}
}

// TestBoardWideVerbs_EmptyScopeAllRefuse sweeps the whole read surface, so a verb added
// later that sweeps AllowedRepos() cannot quietly keep the old behaviour.
//
// UNFIXED SYMPTOM (observed): all six at exit 0 — prs "(no open PRs)", queue
// "(no verify-gate issues)", health "0 green", policydrift "no drift", stalled
// "(no stalled drafts)", actions the fabricated tombstones.
func TestBoardWideVerbs_EmptyScopeAllRefuse(t *testing.T) {
	for _, verb := range []string{"prs", "actions", "queue", "health", "policydrift", "stalled"} {
		t.Run(verb, func(t *testing.T) {
			ghLog := emptyScopeBoard(t)

			var out, errb bytes.Buffer
			code := run([]string{verb, "--table"}, &out, &errb)
			combined := out.String() + errb.String()

			if code != deskkit.ExitUnverifiable {
				t.Errorf("%s exited %d, want %d — an empty sweep is COULD-NOT-CHECK, not a result",
					verb, code, deskkit.ExitUnverifiable)
			}
			if !strings.Contains(combined, "COULD-NOT-CHECK") {
				t.Errorf("%s did not say it could not check:\n%s", verb, combined)
			}
			// The refusal must precede the work, not summarise it afterwards: with no
			// repo to read there is no read to make, and a guard that fires only after
			// the sweep would still burn API budget on every run.
			if b, err := os.ReadFile(ghLog); err == nil && len(bytes.TrimSpace(b)) > 0 {
				t.Errorf("%s made gh calls despite having no repo to read:\n%s", verb, b)
			}
		})
	}
}

// TestBoardWideVerbs_ConfiguredScopeStillWorks is the POSITIVE CONTROL for every
// assertion above. Each test above passes if the binary refuses everything; this is
// what stops the guard from being a blanket refusal that "fixes" #489 by breaking
// the tool. With the ordinary fixture roster (thirteen repos) the same verbs must
// still reach their handlers and succeed.
func TestBoardWideVerbs_ConfiguredScopeStillWorks(t *testing.T) {
	for _, verb := range []string{"prs", "actions", "queue", "health", "stalled"} {
		t.Run(verb, func(t *testing.T) {
			installFakeGH(t) // plants the FULL fixture roster
			if len(deskkit.AllowedRepos()) == 0 {
				t.Fatal("precondition: the fixture roster must configure a non-empty repo set")
			}
			var out, errb bytes.Buffer
			if code := run([]string{verb, "--table"}, &out, &errb); code != deskkit.ExitOK {
				t.Fatalf("%s exited %d with a CONFIGURED scope — the scope guard is refusing "+
					"work it should permit.\nstderr:\n%s", verb, code, errb.String())
			}
			if strings.Contains(out.String()+errb.String(), "COULD-NOT-CHECK: the allowed-repo set is EMPTY") {
				t.Errorf("%s hit the empty-scope guard with a populated repo set", verb)
			}
		})
	}
}

// TestNextupIsNotScopeGuarded records a deliberate exclusion. nextup sweeps CONFIGURED
// ROOTS, not the repo set, and already states its own coverage — it is the precedent
// the guard generalises, not a subject of it. Guarding it on AllowedRepos() would tie
// it to a set it does not read.
func TestNextupIsNotScopeGuarded(t *testing.T) {
	if boardWideVerbs["nextup"] {
		t.Error("nextup was added to boardWideVerbs — it sweeps roots, not AllowedRepos()")
	}
}
