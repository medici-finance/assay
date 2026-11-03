package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

const (
	ratchetHeadRoot = "/repo"
	ratchetBaseRoot = "/tmp/base-worktree"
	ratchetBriefA   = "docs/streams/alpha/brief-01-claims.md"
	ratchetBriefB   = "docs/streams/alpha/brief-02-target.md"
	ratchetOther    = "docs/streams/alpha/README.md"
)

// ratchetCfg is a diffLintConfig whose head and base lints return the given
// NOTICE lines (rendered under each tree's own root), with the changed-brief
// seam fixed to `changed`.
func ratchetCfg(base, head []string, changed map[string]string, cerr error) diffLintConfig {
	cfg := fakeDiffLint("basesha", "headsha", map[string][]string{}, nil)
	cfg.lintRunner = func(treeRoot string, _ []string, _, _ string) ([]string, []string, error) {
		if treeRoot == ratchetHeadRoot {
			return nil, head, nil
		}
		return nil, base, nil
	}
	cfg.changedBriefs = func(string, string) (map[string]string, error) { return changed, cerr }
	return cfg
}

func rowNotice(root, path, row, rule, msg string) string {
	return fmt.Sprintf("%s/%s: Verify row %s [%s] %s", root, path, row, rule, msg)
}

func consumerNotice(root, path, entry, tail string) string {
	return fmt.Sprintf("%s/%s: consumers entry %q %s", root, path, entry, tail)
}

func runRatchet(t *testing.T, cfg diffLintConfig) diffLintResult {
	t.Helper()
	res, err := runDiffLintOne(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// regression: F-notice-ratchet
func TestNoticeRatchet(t *testing.T) {
	legacy := rowNotice(ratchetHeadRoot, ratchetBriefA, "1", ruleTriviallyGreen, "is trivially green: `true` exits 0")
	legacyBase := rowNotice(ratchetBaseRoot, ratchetBriefA, "1", ruleTriviallyGreen, "is trivially green: `true` exits 0")
	second := rowNotice(ratchetHeadRoot, ratchetBriefA, "2", ruleGrepZeroCount, "grep -c exits 1 on zero")
	edited := map[string]string{ratchetBriefA: ratchetBriefA}

	t.Run("legacy_unchanged_passes", func(t *testing.T) {
		// The brief is edited but its debt is unchanged: nothing is new.
		res := runRatchet(t, ratchetCfg([]string{legacyBase}, []string{legacy}, edited, nil))
		if len(res.ratchetIntroduced) != 0 || res.ratchetCarried != 1 || res.ratchetUnavailable != "" {
			t.Fatalf("unchanged debt must carry, not fire: %+v", res)
		}
		if emitDiffLintResult(res) != 0 {
			t.Fatal("unchanged legacy debt must not contribute to the exit count")
		}
		if len(res.notices) != 1 {
			t.Fatalf("carried debt must stay a visible NOTICE: %v", res.notices)
		}
	})

	t.Run("edited_brief_second_defect", func(t *testing.T) {
		res := runRatchet(t, ratchetCfg([]string{legacyBase}, []string{legacy, second}, edited, nil))
		if len(res.ratchetIntroduced) != 1 || res.ratchetIntroduced[0] != second {
			t.Fatalf("only the second defect is new: %v", res.ratchetIntroduced)
		}
		if res.ratchetCarried != 1 {
			t.Fatalf("the legacy finding must be counted as carried: %d", res.ratchetCarried)
		}
		for _, n := range res.notices {
			if n == second {
				t.Fatal("a promoted finding must not also print as a NOTICE")
			}
		}
		if emitDiffLintResult(res) != 1 {
			t.Fatal("a promoted finding must count toward the exit code")
		}
	})

	t.Run("added_brief_second_defect", func(t *testing.T) {
		// An added brief has no base: every actionable finding on it is new.
		added := map[string]string{ratchetBriefB: ""}
		one := rowNotice(ratchetHeadRoot, ratchetBriefB, "1", ruleMetavar, "unsubstituted <name>")
		two := rowNotice(ratchetHeadRoot, ratchetBriefB, "2", ruleBREAlternation, `alternation \|`)
		res := runRatchet(t, ratchetCfg(nil, []string{one, two}, added, nil))
		if len(res.ratchetIntroduced) != 2 {
			t.Fatalf("both findings on an added brief are new: %v", res.ratchetIntroduced)
		}
		if emitDiffLintResult(res) != 2 {
			t.Fatal("the exit count must include every promoted finding")
		}
	})

	t.Run("unrelated_file_passes", func(t *testing.T) {
		// A new finding in a brief the diff does not touch is not this diff's.
		res := runRatchet(t, ratchetCfg(nil, []string{legacy, second}, map[string]string{ratchetBriefB: ratchetBriefB}, nil))
		if len(res.ratchetIntroduced) != 0 {
			t.Fatalf("an unchanged brief must not be gated: %v", res.ratchetIntroduced)
		}
		if len(res.notices) != 2 {
			t.Fatalf("untouched debt stays a visible NOTICE: %v", res.notices)
		}
		res = runRatchet(t, ratchetCfg(nil, []string{legacy, second}, map[string]string{}, nil))
		if len(res.ratchetIntroduced) != 0 || res.ratchetUnavailable != "" {
			t.Fatalf("a diff with no briefs is a clean, available comparison: %+v", res)
		}
	})

	t.Run("moved_roots_and_wording", func(t *testing.T) {
		// Base was rendered under another absolute root with other wording; the
		// structured identity is equal, so nothing is new.
		reworded := rowNotice(ratchetBaseRoot, ratchetBriefA, "1", ruleTriviallyGreen, "older wording of the same defect")
		res := runRatchet(t, ratchetCfg([]string{reworded}, []string{legacy}, edited, nil))
		if len(res.ratchetIntroduced) != 0 || res.ratchetCarried != 1 {
			t.Fatalf("a moved root or reworded message must not mint a finding: %+v", res)
		}
		// An absolute root that is a prefix of the path elsewhere in the line.
		inflated := rowNotice(ratchetBaseRoot+"/nested", ratchetBriefA, "1", ruleTriviallyGreen, "x")
		if f, ok := parseLintNotice(inflated, ratchetBaseRoot+"/nested"); !ok || f.path != ratchetBriefA {
			t.Fatalf("root relativisation failed: %+v ok=%v", f, ok)
		}
	})

	t.Run("renamed_brief_keeps_debt", func(t *testing.T) {
		old := rowNotice(ratchetBaseRoot, ratchetBriefA, "1", ruleTriviallyGreen, "x")
		moved := rowNotice(ratchetHeadRoot, ratchetBriefB, "1", ruleTriviallyGreen, "x")
		res := runRatchet(t, ratchetCfg([]string{old}, []string{moved}, map[string]string{ratchetBriefB: ratchetBriefA}, nil))
		if len(res.ratchetIntroduced) != 0 || res.ratchetCarried != 1 {
			t.Fatalf("a renamed brief carries its base debt: %+v", res)
		}
	})

	t.Run("history_and_unknowns_visible", func(t *testing.T) {
		visible := []string{
			"[gotest-run-vacuous] 3 Verify row(s) in 2 closed brief(s) — historical, not edited",
			"[id-dereference] " + ratchetHeadRoot + "/" + ratchetBriefA + ": identifier dereference COULD-NOT-CHECK: read failed",
			"[risk-files-crossread] " + ratchetHeadRoot + "/" + ratchetBriefA + ": brief alpha/01 COULD-NOT-CHECK the risk\u00d7files cross-read \u2014 no files line",
			ratchetHeadRoot + "/" + ratchetBriefA + ": [table-touches-no-files] COULD-NOT-CHECK \u2014 the files line yields no path",
			ratchetHeadRoot + "/" + ratchetBriefA + ": consumers: brief alpha/01 reads as changing a shared surface but declares no consumers: list",
			rowNotice(ratchetHeadRoot, ratchetBriefA, "1", "consumer-delivery-range", "consumer command has no explicit --base"),
			ratchetHeadRoot + "/" + ratchetBriefA + ": brief alpha/01 names no outcome: add one",
			ratchetHeadRoot + "/docs/streams/alpha/README.md: ordering gate: brief 02 should precede 01",
			ratchetHeadRoot + "/" + ratchetBriefA + ": gate: human has no decision-issue",
			ratchetHeadRoot + "/" + ratchetBriefA + ": eligibility-could-not-check: no network",
			rowNotice(ratchetHeadRoot, ratchetBriefA, "1", "verified-runner-attribution", "runner attribution absent"),
		}
		res := runRatchet(t, ratchetCfg(nil, visible, edited, nil))
		if len(res.ratchetIntroduced) != 0 {
			t.Fatalf("historical, unknown and non-member lines must never become failures: %v", res.ratchetIntroduced)
		}
		if len(res.notices) != len(visible) {
			t.Fatalf("every excluded line must stay visible: got %d want %d", len(res.notices), len(visible))
		}
		if emitDiffLintResult(res) != 0 {
			t.Fatal("excluded lines must not move the exit code")
		}
	})

	t.Run("unavailable_never_clean", func(t *testing.T) {
		cases := map[string]diffLintConfig{
			"unresolvable_base": func() diffLintConfig {
				c := ratchetCfg(nil, []string{second}, edited, nil)
				c.resolveBase = func(string, string) (string, string, error) { return "", "headsha", nil }
				return c
			}(),
			"base_equals_head": func() diffLintConfig {
				c := ratchetCfg(nil, []string{second}, edited, nil)
				c.resolveBase = func(string, string) (string, string, error) { return "same", "same", nil }
				return c
			}(),
			"changed_list_error": ratchetCfg(nil, []string{second}, nil, errors.New("git exploded")),
			"no_changed_source": func() diffLintConfig {
				c := ratchetCfg(nil, []string{second}, edited, nil)
				c.changedBriefs = nil
				return c
			}(),
		}
		for name, cfg := range cases {
			res := runRatchet(t, cfg)
			if res.ratchetUnavailable == "" {
				t.Fatalf("%s: unavailable comparison was not recorded", name)
			}
			if len(res.ratchetIntroduced) != 0 {
				t.Fatalf("%s: nothing may be promoted without a comparison", name)
			}
			var code int
			errOut := captureStderr(t, func() { code = emitDiffLintResult(res) })
			if code != 0 {
				t.Fatalf("%s: exit count %d, want 0 (unknown is not a finding)", name, code)
			}
			if !strings.Contains(errOut, "could-not-check") || !strings.Contains(errOut, "UNAVAILABLE") {
				t.Fatalf("%s: the could-not-check must be explicit: %q", name, errOut)
			}
			if strings.Contains(errOut, "0 new actionable notice(s)") {
				t.Fatalf("%s: printed a clean comparison it did not make: %q", name, errOut)
			}
			if !strings.Contains(errOut, "NOTICE: "+ratchetHeadRoot) {
				t.Fatalf("%s: the head notices must stay visible: %q", name, errOut)
			}
		}
	})

	t.Run("unavailable_keeps_problems", func(t *testing.T) {
		c := ratchetCfg(nil, nil, edited, nil)
		c.resolveBase = func(string, string) (string, string, error) { return "", "headsha", nil }
		c.lintRunner = func(string, []string, string, string) ([]string, []string, error) {
			return []string{"PROBLEM: brief-7: unbacked row"}, []string{second}, nil
		}
		res := runRatchet(t, c)
		if len(res.introduced) != 1 || res.couldNotCheck == "" {
			t.Fatalf("full-strength PROBLEMs must survive an unavailable base: %+v", res)
		}
	})

	t.Run("summary_reports_available_run", func(t *testing.T) {
		res := runRatchet(t, ratchetCfg([]string{legacyBase}, []string{legacy, second}, edited, nil))
		errOut := captureStderr(t, func() { emitDiffLintResult(res) })
		for _, want := range []string{"PROBLEM: [notice-ratchet]", "notice-ratchet: 1 new actionable notice(s)", "1 already at the base"} {
			if !strings.Contains(errOut, want) {
				t.Fatalf("missing %q in %q", want, errOut)
			}
		}
		var exit int
		out := captureStdout(t, func() {
			exit = runDiffLintRootsWith([]string{"r"}, cfgForMap(map[string]diffLintConfig{
				"r": ratchetCfg([]string{legacyBase}, []string{legacy, second}, edited, nil),
			}))
		})
		if exit != 1 || !strings.Contains(out, "LINT: FAIL 1 problem(s)") {
			t.Fatalf("a promoted finding must fail the run: exit=%d out=%q", exit, out)
		}
	})

	t.Run("included_rules_classified", func(t *testing.T) {
		rows := []string{
			ruleERELiteralPipe, ruleGrepZeroCount, ruleExitSwallowed, ruleRE2LiteralPipe, ruleMetavar,
			ruleGoRunExit, ruleBREAlternation, ruleShreddedCell, ruleMovingRef, rulePortability,
			ruleGoTestRunVacuous, ruleProseLedCommand, ruleCmdMarkerAmbiguous, ruleCmdMarkerOverrides,
			ruleCmdMarkerNotHonoured, ruleCmdMarkerVacuous, ruleGrepPipeZeroCount, "verify-row-portability",
			ruleTriviallyGreen, ruleNoOutputAssert,
		}
		for _, r := range rows {
			line := rowNotice(ratchetHeadRoot, ratchetBriefA, "3", r, "msg")
			f, ok := parseLintNotice(line, ratchetHeadRoot)
			if !ok || f.rule != r || f.path != ratchetBriefA || f.row != "3" {
				t.Errorf("row rule %q not classified: %+v ok=%v", r, f, ok)
			}
			// A planted second instance of each rule on a changed brief fires.
			res := runRatchet(t, ratchetCfg([]string{rowNotice(ratchetBaseRoot, ratchetBriefA, "1", r, "old")},
				[]string{rowNotice(ratchetHeadRoot, ratchetBriefA, "1", r, "old"), line}, edited, nil))
			if len(res.ratchetIntroduced) != 1 || res.ratchetCarried != 1 {
				t.Errorf("rule %q: second instance not promoted: %+v", r, res)
			}
		}
		lead := []string{
			"[id-dereference] " + ratchetHeadRoot + "/" + ratchetBriefA + ": named identifier `TestGone` resolves against no file",
			"[" + ruleRiskFilesCrossRead + "] " + ratchetHeadRoot + "/" + ratchetBriefA + `: brief alpha/01 declares path ".github/workflows/x.yml", all risk no`,
			ratchetHeadRoot + "/" + ratchetBriefA + ": [" + ruleTableNoFiles + "] the Verify table touches none of the declared files",
		}
		for _, l := range lead {
			if _, ok := parseLintNotice(l, ratchetHeadRoot); !ok {
				t.Errorf("brief-level finding not classified: %s", l)
			}
		}
		// Subjects separate two findings of one rule on one brief.
		a, _ := parseLintNotice(lead[0], ratchetHeadRoot)
		b, _ := parseLintNotice(strings.Replace(lead[0], "TestGone", "TestOther", 1), ratchetHeadRoot)
		if a.identity() == b.identity() {
			t.Error("two identifiers on one brief collapsed into one identity")
		}
	})

	// regression: F-notice-ratchet-cnc
	t.Run("cnc_token_in_echoed_text_promoted", func(t *testing.T) {
		const tok = "COULD-NOT-CHECK"
		// Row rules echo the Command cell. A new defective row whose command
		// mentions the token is still a finding: the token is brief-controlled
		// text there, not an emitter-owned could-not-check.
		cmds := []struct{ cell, expect string }{
			{"`grep -E '" + tok + "\\|x' f`", "x"},
			{"`true " + tok + "`", "x"},
			{"see `" + tok + "` then `go test ./...`", "x"},
			{"`grep -c " + tok + " f`", "0"},
		}
		promoted := 0
		for _, c := range cmds {
			for _, rf := range rowFindingsCtx(c.cell, c.expect, nil) {
				if !ratchetRowRules[rf.rule] {
					continue
				}
				line := rowNotice(ratchetHeadRoot, ratchetBriefA, "2", rf.rule, rf.msg)
				if !strings.Contains(line, tok) {
					continue // this finding does not echo the command
				}
				promoted++
				f, ok := parseLintNotice(line, ratchetHeadRoot)
				if !ok || f.rule != rf.rule || f.row != "2" {
					t.Errorf("rule %s with the token in its echoed command was dropped: ok=%v %+v", rf.rule, ok, f)
				}
				res := runRatchet(t, ratchetCfg(nil, []string{line}, edited, nil))
				if len(res.ratchetIntroduced) != 1 {
					t.Errorf("rule %s with the token in its echoed command not promoted: %+v", rf.rule, res)
				}
			}
		}
		if promoted < 2 {
			t.Fatalf("the fixture exercised only %d echoing findings; the probe is not reaching the emitters", promoted)
		}
		// Brief-level findings echo brief-controlled text too (a declared path).
		risk := "[" + ruleRiskFilesCrossRead + "] " + ratchetHeadRoot + "/" + ratchetBriefA +
			": brief alpha/01 answers all four risk questions \"no\" but declares path \".github/" + tok + ".yml\", which matches the trigger"
		if f, ok := parseLintNotice(risk, ratchetHeadRoot); !ok || f.rule != ruleRiskFilesCrossRead {
			t.Errorf("risk finding with the token in a declared path was dropped: ok=%v %+v", ok, f)
		}
		deref := "[id-dereference] " + ratchetHeadRoot + "/" + ratchetBriefA + ": named identifier `" + tok + "` resolves against no file"
		if f, ok := parseLintNotice(deref, ratchetHeadRoot); !ok || f.rule != "id-dereference" {
			t.Errorf("id-dereference finding with the token in the identifier was dropped: ok=%v %+v", ok, f)
		}
		// The emitter-owned could-not-check shapes stay unknown.
		for _, l := range []string{
			"[id-dereference] " + ratchetHeadRoot + "/" + ratchetBriefA + ": identifier dereference " + tok + ": read failed",
			"[" + ruleRiskFilesCrossRead + "] " + ratchetHeadRoot + "/" + ratchetBriefA + ": brief alpha/01 " + tok + " the risk cross-read",
			ratchetHeadRoot + "/" + ratchetBriefA + ": [" + ruleTableNoFiles + "] " + tok + " \u2014 no path",
		} {
			if _, ok := parseLintNotice(l, ratchetHeadRoot); ok {
				t.Errorf("an emitter-owned could-not-check was classified as a finding: %s", l)
			}
		}
	})

	// regression: F-notice-ratchet-unnumbered
	t.Run("unnumbered_portability_row_classified", func(t *testing.T) {
		line := ratchetHeadRoot + "/" + ratchetBriefA + ": Verify row  [verify-row-portability] OS-specific command without explicit OS marker"
		f, ok := parseLintNotice(line, ratchetHeadRoot)
		if !ok || f.rule != "verify-row-portability" || f.row != "" || f.path != ratchetBriefA {
			t.Fatalf("an empty row id must still classify: ok=%v %+v", ok, f)
		}
		res := runRatchet(t, ratchetCfg(nil, []string{line}, edited, nil))
		if len(res.ratchetIntroduced) != 1 {
			t.Fatalf("an unnumbered portability row on a changed brief must be promoted: %+v", res)
		}
	})

	t.Run("whitespace_row_id_classified", func(t *testing.T) {
		// The row id is the author's raw # cell. Spaces or brackets in it must
		// not take a new defect out of the comparison.
		for _, row := range []string{"1 a", "[1]", "1 [x]"} {
			line := rowNotice(ratchetHeadRoot, ratchetBriefA, row, ruleTriviallyGreen, "is trivially green: `true` exits 0")
			f, ok := parseLintNotice(line, ratchetHeadRoot)
			if !ok || f.rule != ruleTriviallyGreen || f.row != row || f.path != ratchetBriefA {
				t.Fatalf("# cell %q must classify with its whole id: ok=%v %+v", row, ok, f)
			}
			res := runRatchet(t, ratchetCfg(nil, []string{line}, edited, nil))
			if len(res.ratchetIntroduced) != 1 {
				t.Fatalf("# cell %q: a new defect on a changed brief must be promoted: %+v", row, res)
			}
		}
	})

	t.Run("real_emitters", func(t *testing.T) {
		root := consumersFixture(t, []string{
			"web/a.go",
			"web/b.go: follow-up",
			"web/c.go: follow-up alpha/02",
			"web/d.go: out-of-scope",
		}, nil)
		_, cons := lintConsumers(t, root)
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		_, why := checkBriefFiles(streams, streams)
		_, rows := unfailableRowAudit(streams, nil, false, true)

		got := map[string]int{}
		for _, l := range append(append(append([]string{}, cons...), why...), rows...) {
			if f, ok := parseLintNotice(l, root); ok {
				got[f.rule]++
				if f.path != "docs/streams/alpha/brief-01-claims.md" && f.path != "docs/streams/alpha/brief-02-target.md" {
					t.Errorf("path not repo-relative: %q", f.path)
				}
			}
		}
		for rule, n := range map[string]int{
			"consumers-unrouted":               1,
			"consumers-followup-no-target":     1,
			"consumers-followup-one-way":       1,
			"consumers-out-of-scope-no-reason": 1,
			"checkBriefFiles/why-absent":       2,
			ruleTriviallyGreen:                 1,
		} {
			if got[rule] != n {
				t.Errorf("real emitter output for %q classified %d times, want %d (all: %v)", rule, got[rule], n, got)
			}
		}
		for _, l := range rows {
			if strings.Contains(l, "[consumer-delivery-range]") {
				if _, ok := parseLintNotice(l, root); ok {
					t.Errorf("consumer-delivery-range is outside the ratchet: %s", l)
				}
			}
		}
	})

	t.Run("real_brief_level_emitters", func(t *testing.T) {
		// identifier dereference: two phantom names on one brief are two findings.
		root := t.TempDir()
		dir := filepath.Join(root, "docs", "streams", "s")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := writeTemp(t, dir, "brief-01-x.md", "Verify `TestPhantomName` fires. Also `func phantomFunc` is nowhere.\n")
		ids := map[string]bool{}
		for _, l := range idDerefLines(root, []string{p}) {
			f, ok := parseLintNotice(l, root)
			if !ok || f.rule != "id-dereference" || f.path != "docs/streams/s/brief-01-x.md" {
				t.Fatalf("id-dereference output not classified: %q -> %+v", l, f)
			}
			ids[f.identity()] = true
		}
		if len(ids) != 2 {
			t.Fatalf("each phantom identifier is its own finding: %v", ids)
		}

		// risk x files: classified from the emitter's real output. The fixture
		// writes into an unknown temp dir, so root the parse at that directory.
		_, notices := runCrossRead(t, "medici-finance/assay", map[string]string{
			"brief-01-down.md": crossReadBrief("crossread/01", "model", true,
				"files:\n- `.github/workflows/deploy.yml` — the deploy workflow."),
			"brief-02-none.md": crossReadBrief("crossread/02", "model", true, "no files line"),
		})
		var fired, unknown int
		for _, l := range notices {
			m := regexp.MustCompile(`\] (\S+)/brief-0\d-\w+\.md: `).FindStringSubmatch(l)
			if m == nil {
				t.Fatalf("unexpected risk-files line shape: %q", l)
			}
			f, ok := parseLintNotice(l, m[1])
			switch {
			case ok && f.rule == ruleRiskFilesCrossRead && strings.HasPrefix(f.path, "brief-01"):
				fired++
			case !ok && strings.Contains(l, "COULD-NOT-CHECK"):
				unknown++
			default:
				t.Errorf("unexpected classification of %q: %+v ok=%v", l, f, ok)
			}
		}
		if fired != 1 || unknown != 1 {
			t.Fatalf("want 1 classified finding and 1 visible could-not-check, got %d/%d: %v", fired, unknown, notices)
		}
	})

	t.Run("closed_at_base_records_stay_historical", func(t *testing.T) {
		// statusgen/28 interaction, driven through the real --lint: the inner
		// lint gets --all only when the caller passed it, so a brief already
		// closed at the merge-base reports its findings as path-less
		// [historical-notices] totals. Those totals are never ratchet findings
		// (closed records are not rewritten); an edited active brief is still
		// gated, and --all restores per-finding comparison on the closed one.
		root := noticeFixtureRoot(t)
		withBase(t, true, "demo/02", "demo/03")
		closed := "docs/streams/demo/brief-02-predone.md"
		active := "docs/streams/demo/brief-01-active.md"
		lintNotices := func(all bool) []string {
			resetNoticeReportState(t)
			lintDetailAll = all
			_, _, stderr := lintFixture(t, root)
			var out []string
			for _, l := range strings.Split(stderr, "\n") {
				if strings.HasPrefix(l, "NOTICE:") {
					out = append(out, l)
				}
			}
			return out
		}
		bounded := lintNotices(false)
		var summaries int
		for _, l := range bounded {
			if strings.Contains(l, noticeSummaryTag) {
				summaries++
			}
		}
		if summaries == 0 {
			t.Fatalf("fixture produced no [historical-notices] totals: %v", bounded)
		}
		if rr := evaluateNoticeRatchet(bounded, nil, root, root, map[string]string{closed: closed}); len(rr.introduced) != 0 {
			t.Fatalf("a historical total on a record closed at the base must stay a NOTICE: %v", rr.introduced)
		}
		if rr := evaluateNoticeRatchet(bounded, nil, root, root, map[string]string{active: active}); len(rr.introduced) == 0 {
			t.Fatal("an edited active brief's findings must still be compared and promoted")
		}
		if rr := evaluateNoticeRatchet(lintNotices(true), nil, root, root, map[string]string{closed: closed}); len(rr.introduced) == 0 {
			t.Fatal("with --all the closed record's findings are listed one by one and must be compared")
		}
	})
}

// TestChangedBriefsGit drives the production changed-brief listing against a
// real repository: added, edited, renamed, deleted, untracked and non-brief
// changes.
func TestChangedBriefsGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		full := append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)
		out, err := exec.Command("git", full...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, body string) {
		t.Helper()
		mustWrite(t, filepath.Join(root, rel), body)
	}
	git("init", "-q")
	d := "docs/streams/s/"
	write(d+"brief-01-edit.md", "one\n")
	write(d+"brief-02-move.md", "a long enough body so the rename is detected as a rename\nline two\nline three\n")
	write(d+"brief-03-gone.md", "gone\n")
	write(d+"README.md", "r\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")

	write(d+"brief-01-edit.md", "two\n")
	write(d+"README.md", "changed\n")
	git("mv", d+"brief-02-move.md", d+"brief-02-renamed.md")
	git("rm", "-q", d+"brief-03-gone.md")
	write(d+"brief-04-added.md", "new\n")
	git("add", d+"brief-04-added.md")
	write(d+"brief-05-untracked.md", "u\n")

	got, err := productionChangedBriefs(root, base)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		d + "brief-01-edit.md":      d + "brief-01-edit.md",
		d + "brief-02-renamed.md":   d + "brief-02-move.md",
		d + "brief-04-added.md":     "",
		d + "brief-05-untracked.md": "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changed briefs = %v, want %v", got, want)
	}
	if _, err := productionChangedBriefs(root, "no-such-ref"); err == nil {
		t.Fatal("an unresolvable base must be an error, never an empty (clean) set")
	}
	// An option-shaped base is refused before git sees it: `--output=<f>` would
	// otherwise make git write the diff to a file and print nothing, which reads
	// as a clean, empty change set.
	sink := filepath.Join(t.TempDir(), "diff.out")
	if got, err := productionChangedBriefs(root, "--output="+sink); err == nil {
		t.Fatalf("an option-shaped base must be an error, never a change set: %v", got)
	}
	if _, err := os.Stat(sink); err == nil {
		t.Fatal("an option-shaped base reached git: it wrote the diff to a file")
	}
}
