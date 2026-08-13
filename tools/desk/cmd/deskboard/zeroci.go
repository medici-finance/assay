package main

// zeroci.go — zero-CI disambiguation.
//
// THE GAP THIS CLOSES. An empty check rollup used to render as a bare
// `0✓ 0pend 0fail` on every board row, and that one rendering covered two
// opposite facts:
//
//   - "no checks are configured for this diff" (the repo has no workflows, or
//     every workflow's branch/path filters exclude the change) — a LEGITIMATE
//     zero, the only zero that may ever read as vacuously green, and
//   - "checks never ran" (workflows exist that would fire on this diff, but no
//     run exists at the head sha) — the PR is UNVALIDATED, and on a board whose
//     top line is MERGE-NOW an unvalidated PR was indistinguishable from a
//     repo that legitimately has no gates. A PR sat on the board
//     in exactly that state while sibling PRs opened in the same window got
//     their runs.
//
// This is the three-state rule applied to
// the CI cell: checked-clean, checked-failed, could-not-check. The states:
//
//	no-checks         verified: nothing ran AND no workflow at the head would
//	                  fire on this diff. The zero is checked, not assumed.
//	checks-never-ran  verified: nothing ran AND ≥1 workflow at the head WOULD
//	                  fire on this diff. Never green, on any repo.
//	unverified        COULD-NOT-CHECK: a read failed, a list truncated, a
//	                  workflow file had a shape the parser does not model, or
//	                  check runs/commit statuses exist at head that the rollup
//	                  counted as nothing. Never green. Saying "we cannot tell"
//	                  on the line is the whole point — a bare 0✓ 0pend 0fail
//	                  resolved every ambiguity in the direction that looked
//	                  like progress.
//
// FAIL-CLOSED STANCE. Any shape the parser does not model (an unquoted shape, a
// filter key it does not know, a glob with a character class, a YAML
// anchor/alias/merge key it cannot resolve, a tab-indented line whose nesting
// it cannot track, a truncated changed-file list) is `unverified`, never
// silently `no-checks`: a wrong
// "no-checks" waves an unvalidated PR through, a wrong "unverified" costs a
// human look. The safe direction is the only cheap one.
//
// READ COST. The probe runs ONLY for rows whose rollup counted 0/0/0. Worst
// path per such PR: 1 check-runs read + 1 combined-status read + 1 workflows
// directory listing + ≤zeroCIWorkflowCap workflow content reads + (only when a
// workflow carries a path filter) the changed-files walk. All through ghRun,
// all GETs — the PATH-shim test enumerates them.
//
// WHAT no-checks DOES NOT CLAIM. The trigger model is GitHub's documented
// pull_request semantics as far as they are stated: branch filters, path
// filters with ordered `!` negation, and the default activity types. It does
// not model workflow `if:` gates, reusable-workflow indirection, disabled
// workflows, or concurrency-group suppression — a workflow counted as
// "would fire" that was in fact disabled yields checks-never-ran, which is the
// safe direction (not green; a human looks and finds the disabled workflow).
// The default activity types modelled are the GitHub defaults (opened,
// synchronize, reopened); a workflow restricted to `ready_for_review` alone is
// treated as not firing pre-flip, because pre-flip is the state the board
// renders.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Zero-rollup states. The empty string means "not a zero row" (the rollup
// counted at least one check) — an absent state, never a claim.
const (
	zeroCINoChecks   = "no-checks"
	zeroCINeverRan   = "checks-never-ran"
	zeroCIUnverified = "unverified"
)

// zeroCIWorkflowCap bounds the workflow-content reads per probed PR. Above it
// the probe reports unverified with the cap named — truncation is a lie
// (sub-rule 2), even here.
const zeroCIWorkflowCap = 25

// prLifecycleTypes are the pull_request activity types a workflow fires on in
// the ordinary pre-flip lifecycle — GitHub's defaults when `types:` is absent.
var prLifecycleTypes = []string{"opened", "synchronize", "reopened"}

// ---------------------------------------------------------------------------
// the probe
// ---------------------------------------------------------------------------

// probeZeroCI determines WHY a PR's rollup counted zero checks. It never
// errors: every failure is a first-class `unverified` state with the reason,
// so one flaky read degrades exactly one board row instead of the board —
// the same fail-closed-narrowly contract as assessRepoBranch (health.go).
func probeZeroCI(repo string, p prBase) (state, detail string) {
	unv := func(format string, args ...any) (string, string) {
		return zeroCIUnverified, fmt.Sprintf(format, args...)
	}

	// (1) Did anything run at this sha that the rollup failed to count? A
	// rollup that says zero while check runs or commit statuses exist is a
	// suspect READ, not a checked zero — and an all-SKIPPED/NEUTRAL set ran
	// but produced no verdict, which is not green either (a flip needs at
	// least one completed successful check, not the absence of a failure).
	runs, truncated, err := fetchCheckRuns(repo, p.HeadRefOid)
	if err != nil {
		return unv("cannot read check-runs at head %s — %s", short(p.HeadRefOid), oneLine(err.Error()))
	}
	if truncated {
		return unv("check-run list truncated at the per_page=%d cap for %s — cannot prove nothing ran", bhPageCap, short(p.HeadRefOid))
	}
	evidence, skipped := 0, 0
	for _, r := range runs {
		if !strings.EqualFold(r.Status, "completed") {
			evidence++ // still running — the rollup should have counted it pending
			continue
		}
		switch strings.ToUpper(r.Conclusion) {
		case "SKIPPED", "NEUTRAL":
			skipped++
		default: // SUCCESS, FAILURE, CANCELLED, ... — the rollup should have counted it
			evidence++
		}
	}
	if evidence > 0 {
		return unv("%d check run(s) exist at head %s but the PR rollup counted none — the rollup read is suspect, "+
			"not the CI", evidence, short(p.HeadRefOid))
	}
	if skipped > 0 {
		return unv("%d check run(s) at head %s, all SKIPPED/NEUTRAL — checks ran but produced no verdict; "+
			"no completed successful check exists", skipped, short(p.HeadRefOid))
	}
	statuses, serr := fetchCombinedStatusTotal(repo, p.HeadRefOid)
	if serr != nil {
		return unv("cannot read combined commit status at head %s — %s", short(p.HeadRefOid), oneLine(serr.Error()))
	}
	if statuses > 0 {
		return unv("%d commit status context(s) exist at head %s but the PR rollup counted none — the rollup read "+
			"is suspect (external CI posts statuses, not check runs)", statuses, short(p.HeadRefOid))
	}

	// (2) Nothing ran at this sha. Would any workflow at the head have fired
	// on this PR? A 404 on the workflows directory is a KNOWN answer, not an
	// error — a repo with no workflows has no checks by construction.
	names, werr := listWorkflowFiles(repo, p.HeadRefOid)
	if werr != nil {
		if isNotFound(werr) {
			return zeroCINoChecks, "no .github/workflows directory at head — verified there is nothing to run"
		}
		return unv("cannot list .github/workflows at head %s — %s", short(p.HeadRefOid), oneLine(werr.Error()))
	}
	if len(names) == 0 {
		return zeroCINoChecks, ".github/workflows at head has no workflow files — verified there is nothing to run"
	}
	if len(names) > zeroCIWorkflowCap {
		return unv("%d workflow files at head exceeds the probe cap %d — refusing to judge a truncated set",
			len(names), zeroCIWorkflowCap)
	}

	var triggers []prTrigger
	for _, n := range names {
		content, cerr := fetchWorkflowContent(repo, n, p.HeadRefOid)
		if cerr != nil {
			return unv("cannot read workflow %s at head — %s", n, oneLine(cerr.Error()))
		}
		tr, perr := parseWorkflowPRTrigger(n, content)
		if perr != nil {
			return unv("cannot parse workflow %s — %s", n, oneLine(perr.Error()))
		}
		if tr.present {
			triggers = append(triggers, tr)
		}
	}
	if len(triggers) == 0 {
		return zeroCINoChecks, fmt.Sprintf("%d workflow(s) at head, none triggers on pull_request — "+
			"verified nothing fires for a PR", len(names))
	}

	// Branch filters need the base branch; path filters need the diff. Both
	// are fetched lazily — a workflow set with no filters never pays for them.
	needBase, needFiles := false, false
	for _, tr := range triggers {
		if len(tr.branches)+len(tr.branchesIgnore) > 0 {
			needBase = true
		}
		if len(tr.paths)+len(tr.pathsIgnore) > 0 {
			needFiles = true
		}
	}
	if needBase && p.BaseRefName == "" {
		return unv("workflow branch filters are present but the PR's base branch is unknown")
	}
	var files []string
	if needFiles {
		fset, complete, ferr := fetchChangedFiles(repo, p.Number)
		if ferr != nil {
			return unv("cannot read changed files for %s#%d — %s", repo, p.Number, oneLine(ferr.Error()))
		}
		if !complete {
			return unv("changed-file list for %s#%d is TRUNCATED — cannot prove no workflow's path filter matches "+
				"the diff", repo, p.Number)
		}
		files = make([]string, 0, len(fset))
		for f := range fset {
			files = append(files, f)
		}
		sort.Strings(files)
	}

	var would []string
	for _, tr := range triggers {
		fire, ferr := tr.wouldFire(p.BaseRefName, files)
		if ferr != nil {
			return unv("cannot evaluate workflow %s's filters — %s", tr.name, oneLine(ferr.Error()))
		}
		if fire {
			would = append(would, tr.name)
		}
	}
	if len(would) > 0 {
		return zeroCINeverRan, fmt.Sprintf("workflow(s) %s would fire on this diff but produced no run at head %s — "+
			"CI NEVER RAN; the PR is unvalidated", strings.Join(would, ", "), short(p.HeadRefOid))
	}
	return zeroCINoChecks, fmt.Sprintf("%d pull_request workflow(s) at head, none matches this diff "+
		"(branch/path filters) — a checked zero", len(triggers))
}

// fetchCombinedStatusTotal reads the combined commit status at a sha and
// returns GitHub's own context count. External CI (non-Actions) posts commit
// statuses, not check runs — a zero-check-runs sha with statuses here is a
// suspect rollup read, not a checked zero.
func fetchCombinedStatusTotal(repo, sha string) (int, error) {
	out, err := ghRun("api", fmt.Sprintf("repos/%s/commits/%s/status", repo, sha))
	if err != nil {
		return 0, err
	}
	var v struct {
		TotalCount int `json:"total_count"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return 0, fmt.Errorf("unparseable combined-status payload: %v", err)
	}
	return v.TotalCount, nil
}

// isNotFound reports whether a gh api failure was an HTTP 404 — for the
// workflows directory that is a known answer (no workflows), not a read
// failure.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "404")
}

// listWorkflowFiles returns the .yml/.yaml file names under .github/workflows
// at ref, sorted for deterministic messages.
func listWorkflowFiles(repo, ref string) ([]string, error) {
	out, err := ghRun("api", fmt.Sprintf("repos/%s/contents/.github/workflows?ref=%s", repo, ref))
	if err != nil {
		return nil, err
	}
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("unparseable workflows listing: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		if strings.HasSuffix(e.Name, ".yml") || strings.HasSuffix(e.Name, ".yaml") {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// fetchWorkflowContent reads one workflow file at ref (GET-only contents API).
func fetchWorkflowContent(repo, name, ref string) (string, error) {
	out, err := ghRun("api", fmt.Sprintf("repos/%s/contents/.github/workflows/%s?ref=%s", repo, name, ref))
	if err != nil {
		return "", err
	}
	var v struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return "", fmt.Errorf("unparseable workflow payload: %v", err)
	}
	content := v.Content
	if v.Encoding == "base64" {
		dec, derr := base64.StdEncoding.DecodeString(strings.ReplaceAll(content, "\n", ""))
		if derr != nil {
			return "", fmt.Errorf("cannot base64-decode workflow %s: %v", name, derr)
		}
		content = string(dec)
	}
	return content, nil
}

// ---------------------------------------------------------------------------
// the trigger model (hand-rolled, dependency-free, fail-closed)
// ---------------------------------------------------------------------------

// prTrigger is the pull_request-relevant slice of one workflow's `on:` block.
type prTrigger struct {
	name           string // workflow file name, for messages
	present        bool   // on: includes pull_request or pull_request_target
	branches       []string
	branchesIgnore []string
	paths          []string
	pathsIgnore    []string
	types          []string
}

// parseWorkflowPRTrigger extracts the pull_request / pull_request_target
// trigger filters from a workflow file. EVERY unmodelled shape is an error —
// the caller renders that as unverified, so a parser gap can never masquerade
// as "no workflow would fire".
//
// Modelled shapes (everything the watched repos' workflows use, verified by the
// tests): `on: <scalar>`, `on: [flow, list]`, `on:` + block sequence, and the
// `on:` block map with per-event filter keys branches / branches-ignore /
// paths / paths-ignore / types in block or single-line flow form. Non-PR
// events and their subtrees (schedule cron items, workflow_dispatch inputs,
// anything else) are skipped without inspection.
func parseWorkflowPRTrigger(name, content string) (prTrigger, error) {
	tr := prTrigger{name: name}
	lines := strings.Split(content, "\n")

	onIdx, onInline, onRaw := -1, "", ""
	for i, ln := range lines {
		for _, prefix := range []string{"on:", `"on":`, `'on':`} {
			if strings.HasPrefix(ln, prefix) {
				// wfUnquote strips a trailing `# comment` — without that an
				// `on: # triggers` line reads as an inline scalar and the block
				// map below it is never parsed.
				onRaw = strings.TrimSpace(ln[len(prefix):])
				onIdx, onInline = i, wfUnquote(onRaw)
				break
			}
		}
		if onIdx >= 0 {
			break
		}
	}
	if onIdx < 0 {
		return tr, fmt.Errorf("no top-level `on:` found — cannot determine whether the workflow fires on pull_request")
	}
	if wfAliasLike(onRaw) {
		return tr, fmt.Errorf("%s: on: is the YAML alias/anchor %q, which this parser cannot resolve — "+
			"unmodelled shape", name, onRaw)
	}

	isPREvent := func(ev string) bool { return ev == "pull_request" || ev == "pull_request_target" }

	// `on: push` / `on: [push, pull_request]` — the whole trigger is inline.
	if onInline != "" {
		var events []string
		if strings.HasPrefix(onInline, "[") {
			vals, err := wfFlowList(onInline)
			if err != nil {
				return tr, fmt.Errorf("on: %v", err)
			}
			events = vals
		} else {
			events = []string{wfUnquote(onInline)}
		}
		for _, ev := range events {
			if isPREvent(ev) {
				tr.present = true
			}
		}
		return tr, nil
	}

	// Block form. `on:` followed directly by `- event` items is a block
	// sequence (no filters); otherwise it is a map of event → filters. The
	// sequence items may sit at column 0 — YAML allows a block sequence at the
	// parent key's indentation.
	i := onIdx + 1
	for i < len(lines) && wfIgnorable(lines[i]) {
		i++
	}
	if i < len(lines) {
		if trimmed := strings.TrimSpace(lines[i]); strings.HasPrefix(trimmed, "- ") {
			for ; i < len(lines); i++ {
				l2 := lines[i]
				if wfIgnorable(l2) {
					continue
				}
				if wfTabIndented(l2) {
					return tr, fmt.Errorf("%s: on: item %q is tab-indented — this parser tracks indentation by "+
						"spaces and cannot see the nesting; unmodelled shape", name, strings.TrimSpace(l2))
				}
				t2 := strings.TrimSpace(l2)
				if !strings.HasPrefix(t2, "- ") {
					if len(l2)-len(strings.TrimLeft(l2, " ")) == 0 {
						break // next top-level key — the sequence is done
					}
					return tr, fmt.Errorf("on: mixes a block sequence with a mapping at %q — unmodelled shape", t2)
				}
				item := strings.TrimSpace(t2[2:])
				if wfAliasLike(item) {
					return tr, fmt.Errorf("%s: on: item %q is a YAML alias/anchor this parser cannot resolve — "+
						"unmodelled shape", name, item)
				}
				if isPREvent(wfUnquote(item)) {
					tr.present = true
				}
			}
			return tr, nil
		}
	}

	// Block map: collect event entries (lines at the block's minimum indent),
	// then parse only the PR events' filter subtrees.
	type evLine struct {
		name  string
		rest  string
		start int
		end   int
	}
	var events []evLine
	evIndent := -1
	for i = onIdx + 1; i < len(lines); i++ {
		ln := lines[i]
		if wfIgnorable(ln) {
			continue
		}
		if wfTabIndented(ln) {
			return tr, fmt.Errorf("%s: %q under on: is tab-indented — this parser tracks indentation by spaces "+
				"and cannot see the nesting; unmodelled shape", name, strings.TrimSpace(ln))
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if indent == 0 {
			break
		}
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "- ") {
			// A list item belongs to an earlier event's own block (e.g.
			// schedule cron entries); it is not an event line.
			continue
		}
		if evIndent < 0 {
			evIndent = indent
		}
		if indent != evIndent {
			// Nested under the previous event — EXCEPT a pull_request key here:
			// no event's filter subtree legitimately contains one, so this means
			// the events are not uniformly indented, which this parser does not
			// model. Error rather than silently miss a firing trigger.
			nested, _ := wfSplitKey(trimmed)
			if isPREvent(wfUnquote(nested)) {
				return tr, fmt.Errorf("%s: %s is indented deeper than the other events — "+
					"uneven event indentation is an unmodelled shape", name, nested)
			}
			continue
		}
		key, rest := wfSplitKey(trimmed)
		if wfAliasLike(key) {
			// `<<: *base` at the event level splices in another map's events —
			// skipping it as "some non-PR event" would silently drop a
			// pull_request trigger and answer no-checks.
			return tr, fmt.Errorf("%s: key %q under on: is a YAML merge/alias key this parser cannot resolve — "+
				"unmodelled shape", name, key)
		}
		events = append(events, evLine{name: wfUnquote(key), rest: rest, start: i})
	}
	for k := range events {
		end := len(lines)
		if k+1 < len(events) {
			end = events[k+1].start
		}
		events[k].end = end
	}

	for _, ev := range events {
		if !isPREvent(ev.name) {
			continue
		}
		tr.present = true
		switch ev.rest {
		case "", "{}", "null", "~":
		default:
			return tr, fmt.Errorf("%s: %s has inline value %q — unmodelled shape", name, ev.name, ev.rest)
		}
		if err := tr.parseFilters(name, ev.name, lines[ev.start+1:ev.end], evIndent); err != nil {
			return tr, err
		}
	}
	return tr, nil
}

// parseFilters reads the nested filter map under a pull_request event. Lines
// is the event's own block (after the event line, before the next event);
// evIndent is the indent events sit at, so every filter line is deeper.
func (tr *prTrigger) parseFilters(file, event string, lines []string, evIndent int) error {
	keyIndent := -1
	var cur *[]string
	curClosed := false // a flow/scalar value was already assigned to cur's key
	seen := map[string]bool{}
	for _, ln := range lines {
		if wfIgnorable(ln) {
			continue
		}
		if wfTabIndented(ln) {
			return fmt.Errorf("%s: %q under %s is tab-indented — this parser tracks indentation by spaces and "+
				"cannot see the nesting; unmodelled shape", file, strings.TrimSpace(ln), event)
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if indent == 0 || indent <= evIndent {
			break // left the on: block or reached the next event (defensive)
		}
		trimmed := strings.TrimSpace(ln)
		if item, isItem := strings.CutPrefix(trimmed, "- "); isItem {
			if wfAliasLike(item) {
				return fmt.Errorf("%s: list item %q under %s is a YAML alias/anchor this parser cannot resolve — "+
					"unmodelled shape", file, trimmed, event)
			}
			if cur == nil {
				return fmt.Errorf("%s: list item %q under %s appears before any filter key — unmodelled shape",
					file, trimmed, event)
			}
			if curClosed {
				return fmt.Errorf("%s: list item %q under %s follows an inline value for the same key — "+
					"unmodelled shape", file, trimmed, event)
			}
			v := wfUnquote(strings.TrimSpace(item))
			if v == "" {
				return fmt.Errorf("%s: empty filter value under %s", file, event)
			}
			*cur = append(*cur, v)
			continue
		}
		if keyIndent < 0 {
			keyIndent = indent
		}
		if indent != keyIndent {
			return fmt.Errorf("%s: unexpected indentation at %q under %s — unmodelled shape", file, trimmed, event)
		}
		kname, rest := wfSplitKey(trimmed)
		if wfAliasLike(kname) {
			return fmt.Errorf("%s: key %q under %s is a YAML merge/alias key this parser cannot resolve — "+
				"unmodelled shape", file, kname, event)
		}
		switch kname {
		case "branches":
			cur = &tr.branches
		case "branches-ignore":
			cur = &tr.branchesIgnore
		case "paths":
			cur = &tr.paths
		case "paths-ignore":
			cur = &tr.pathsIgnore
		case "types":
			cur = &tr.types
		default:
			return fmt.Errorf("%s: unsupported filter key %q under %s — teach the parser rather than guess",
				file, kname, event)
		}
		if seen[kname] {
			return fmt.Errorf("%s: duplicate filter key %q under %s — unmodelled shape", file, kname, event)
		}
		seen[kname] = true
		curClosed = false
		if rest == "" {
			continue
		}
		if rest == "|" || rest == ">" || strings.HasPrefix(rest, "| ") || strings.HasPrefix(rest, "> ") {
			return fmt.Errorf("%s: %s.%s uses a multi-line scalar — unmodelled shape", file, event, kname)
		}
		if wfAliasLike(rest) {
			return fmt.Errorf("%s: %s.%s is the YAML alias/anchor %q, which this parser cannot resolve — "+
				"unmodelled shape", file, event, kname, rest)
		}
		if strings.HasPrefix(rest, "[") {
			vals, err := wfFlowList(rest)
			if err != nil {
				return fmt.Errorf("%s: %s.%s: %v", file, event, kname, err)
			}
			if len(vals) == 0 {
				return fmt.Errorf("%s: %s.%s is an empty list — cannot reason about it", file, event, kname)
			}
			*cur = append(*cur, vals...)
			curClosed = true
			continue
		}
		// Bare scalar (`branches: main`) — one value.
		*cur = append(*cur, wfUnquote(rest))
		curClosed = true
	}
	return nil
}

// wouldFire reports whether the workflow runs for a PR targeting baseBranch
// and changing files, under GitHub's documented filter semantics:
//
//   - branches / branches-ignore filter the BASE branch (globs, no `!`).
//   - types, when present, must include a lifecycle type (opened /
//     synchronize / reopened — see prLifecycleTypes).
//   - paths: the workflow runs iff ≥1 changed file is included by the ordered
//     pattern set (`!` re-excludes, a later positive re-includes).
//   - paths-ignore: the workflow is skipped only when EVERY changed file is
//     ignored — a single surviving file runs it.
//
// An error means the filters could not be evaluated (e.g. a glob shape the
// matcher does not model) — the caller renders unverified, never a guess.
func (tr prTrigger) wouldFire(baseBranch string, files []string) (bool, error) {
	if !tr.present {
		return false, nil
	}
	if len(tr.branches) > 0 {
		m, err := wfMatchAny(tr.branches, baseBranch)
		if err != nil || !m {
			return false, err
		}
	}
	if len(tr.branchesIgnore) > 0 {
		m, err := wfMatchAny(tr.branchesIgnore, baseBranch)
		if err != nil || m {
			return false, err
		}
	}
	if len(tr.types) > 0 {
		fires := false
		for _, ty := range tr.types {
			for _, lt := range prLifecycleTypes {
				if ty == lt {
					fires = true
				}
			}
		}
		if !fires {
			return false, nil
		}
	}
	if len(tr.paths) > 0 {
		for _, f := range files {
			inc, err := wfPathSetIncludes(tr.paths, f)
			if err != nil {
				return false, err
			}
			if inc {
				return true, nil
			}
		}
		return false, nil
	}
	if len(tr.pathsIgnore) > 0 {
		for _, f := range files {
			ignored, err := wfPathSetIncludes(tr.pathsIgnore, f)
			if err != nil {
				return false, err
			}
			if !ignored {
				return true, nil
			}
		}
		return false, nil
	}
	return true, nil
}

// wfIgnorable reports whether a YAML line carries no content.
func wfIgnorable(ln string) bool {
	t := strings.TrimSpace(ln)
	return t == "" || strings.HasPrefix(t, "#")
}

// wfAliasLike reports whether a RAW YAML token — still quoted, not yet passed
// through wfUnquote — is a node alias (`*name`), an anchor (`&name …`) or the
// merge key (`<<`). This parser does not build a node graph, so it cannot
// resolve any of them; taking the token literally is how `paths: *p` became the
// glob `*p`, matched nothing in the diff, and rendered a confident `no-checks`
// on a PR whose CI genuinely would have fired.
//
// The check must run on the RAW token because wfUnquote strips the quotes that
// distinguish an alias from a glob: `*p` is an alias, `'**.md'` is a path
// pattern. An UNQUOTED value starting with `*` or `&` is not a legal plain
// scalar in YAML at all, so reading it as a glob was never right.
func wfAliasLike(raw string) bool {
	t := strings.TrimSpace(raw)
	if t == "" {
		return false
	}
	if t[0] == '*' || t[0] == '&' {
		return true
	}
	return t == "<<" || strings.HasPrefix(t, "<<:") || strings.HasPrefix(t, "<< ")
}

// wfTabIndented reports whether a line's LEADING whitespace contains a tab.
// Tabs are illegal as YAML indentation, and this parser tracks indentation by
// counting leading spaces — so a tab makes it mis-read the nesting silently
// (`on:` + a tab-indented `pull_request:` reads as an EMPTY on: block, and the
// trigger vanishes). Whatever the right answer is for such a file, the parser
// must not reach one by not seeing the block.
func wfTabIndented(ln string) bool {
	return strings.ContainsRune(ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))], '\t')
}

// wfSplitKey splits `key: rest` into the (untrimmed) key and the trimmed
// inline value. A line with no colon yields the whole line as key.
func wfSplitKey(trimmed string) (key, rest string) {
	if idx := strings.IndexByte(trimmed, ':'); idx >= 0 {
		return strings.TrimSpace(trimmed[:idx]), strings.TrimSpace(trimmed[idx+1:])
	}
	return trimmed, ""
}

// wfFlowList parses a single-line `[a, "b", 'c']` flow sequence. Anything
// else is an error — an unmodelled shape must surface as unverified, never as
// a silently wrong filter.
func wfFlowList(rest string) ([]string, error) {
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "[") || !strings.HasSuffix(rest, "]") {
		return nil, fmt.Errorf("%q is not a single-line [a, b] flow sequence — unmodelled shape", rest)
	}
	inner := strings.TrimSpace(rest[1 : len(rest)-1])
	if inner == "" {
		return nil, nil
	}
	var out []string
	for _, part := range strings.Split(inner, ",") {
		if wfAliasLike(part) {
			return nil, fmt.Errorf("entry %q is a YAML alias/anchor this parser cannot resolve — unmodelled shape",
				strings.TrimSpace(part))
		}
		if v := wfUnquote(strings.TrimSpace(part)); v != "" {
			out = append(out, v)
		}
	}
	return out, nil
}

// wfUnquote strips surrounding quotes and any trailing `# comment`.
func wfUnquote(s string) string {
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

// wfMatchAny reports whether any glob matches s (branch filters — no `!`
// semantics, per GitHub's docs).
func wfMatchAny(globs []string, s string) (bool, error) {
	for _, g := range globs {
		re, err := wfGlobToRegexp(g)
		if err != nil {
			return false, err
		}
		if re.MatchString(s) {
			return true, nil
		}
	}
	return false, nil
}

// wfPathSetIncludes evaluates a path filter's ordered pattern set against one
// path: a matching positive pattern includes it, a matching `!` pattern
// excludes it again, and a later positive re-includes it — GitHub's
// documented "order matters" semantics for `paths` and `paths-ignore`.
func wfPathSetIncludes(patterns []string, path string) (bool, error) {
	included := false
	for _, p := range patterns {
		neg := strings.HasPrefix(p, "!")
		pat := strings.TrimPrefix(p, "!")
		re, err := wfGlobToRegexp(pat)
		if err != nil {
			return false, err
		}
		if re.MatchString(path) {
			included = !neg
		}
	}
	return included, nil
}

// wfGlobToRegexp translates a GitHub filter-pattern glob: `*` matches any run
// except `/`, `**` any run including `/` — and `**/` matches ZERO or more
// whole path segments, so `k8s/**\/x.yaml` covers both `k8s/x.yaml` and
// `k8s/dev/x.yaml` (GitHub/minimatch globstar semantics). `?` matches one
// non-`/` character. Character classes and braces are NOT modelled — an
// error, so the probe reports unverified rather than a wrong match.
func wfGlobToRegexp(g string) (*regexp.Regexp, error) {
	if strings.ContainsAny(g, "[{") {
		return nil, fmt.Errorf("glob %q uses a character class or brace this matcher does not model", g)
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); i++ {
		switch g[i] {
		case '*':
			if i+1 < len(g) && g[i+1] == '*' {
				i++
				if i+1 < len(g) && g[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?") // globstar before a slash: zero or more segments
				} else {
					b.WriteString(".*")
				}
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

// ---------------------------------------------------------------------------
// rendering
// ---------------------------------------------------------------------------

// ciColumn renders the CI cell of a board row. A zero rollup is NEVER a bare
// `0✓ 0pend 0fail` — the cell names WHICH zero it is (#1652), and a caller
// that skipped the probe renders unverified rather than a lie.
func ciColumn(pass, pending, fail int, zero string) string {
	if pass+pending+fail > 0 {
		return fmt.Sprintf("%d✓ %dpend %dfail", pass, pending, fail)
	}
	switch zero {
	case zeroCINoChecks:
		return "0 checks (no-checks)"
	case zeroCINeverRan:
		return "0 checks (NEVER-RAN)"
	default:
		return "0 checks (unverified)"
	}
}
