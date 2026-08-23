package main

// The orchestrator — the certificate shell, now driving two external engines.
//
// This is the part the simplification review called the write-up-worthy piece and
// the part that is deliberately NOT delegated: the three-state model, the
// positive controls, the canary, the refusal to report clean on an error, and the
// content-hash-bound certificate. gitleaks and trufflehog supply detection. They
// do not supply epistemics, and their defaults are the opposite of what a
// publication gate needs — trufflehog exits 0 with findings on stdout, and both
// are perfectly happy to report "no findings" for a directory they never read.
//
// BOTH ENGINES MUST CERTIFY. An entry is checked-clean only when, for EVERY
// engine: the engine ran, its rule for that entry fired on the sample fixture,
// its control rule for that entry fired on the swept tree, and it found no
// unexcused, unallowed hit. Any engine failing any of those makes the entry
// could-not-check. Two engines are not two chances to pass; they are two
// witnesses who must agree.
//
// THE FOUR INDEPENDENT WAYS A RUN IS REFUSED, and what each one catches:
//
//   1. engine did not run          → the scanner is missing/broken (could-not-check)
//   2. rule did not fire on sample → THAT RULE is dead, even though the engine ran
//   3. control did not fire on tree→ the engine did not read the tree we think it read
//   4. canary did not fire         → the engine's own stock rule set is dead (#643)
//
// Every one of these was previously indistinguishable from "clean".

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// runOutcome is the whole two-engine run.
type runOutcome struct {
	tree        string
	engines     []*engineOutcome
	results     []entryResult
	corpus      *corpus
	filesSwept  int
	contentHash string
	provenance  string
	// blockers are run-level refusals (dead canary, missing engine) that are not
	// attributable to any single entry but forbid certification outright.
	blockers []string
}

// orchestrate runs every engine over the projection and assembles the three-state
// report. It never returns a clean outcome on an error path.
func orchestrate(tl *TokenList, c *corpus, engines []engine) (*runOutcome, error) {
	if err := checkCanaryEmbed(); err != nil {
		return nil, err
	}
	rules, err := compileRules(tl)
	if err != nil {
		return nil, err
	}
	rules = append(rules, beaconRule())
	proj, err := buildProjection(c)
	if err != nil {
		return nil, err
	}
	defer proj.cleanup()
	sampleDir, err := buildSampleFixture(rules)
	if err != nil {
		return nil, err
	}
	defer removeAllQuiet(sampleDir)
	canaryDirPath, err := buildCanaryFixture()
	if err != nil {
		return nil, err
	}
	defer removeAllQuiet(canaryDirPath)

	out := &runOutcome{tree: c.tree, corpus: c, filesSwept: len(c.files), contentHash: contentHash(c)}
	for _, e := range engines {
		out.engines = append(out.engines, runEngine(e, proj.dir, sampleDir, canaryDirPath, rules))
	}
	out.blockers = append(out.blockers, coverageBlockers(proj, out.engines)...)
	out.results = assemble(tl, rules, out.engines)
	if r, ok := stockFindings(tl, rules, out.engines); ok {
		out.results = append(out.results, r)
	}
	for _, eo := range out.engines {
		if !eo.ranOK {
			out.blockers = append(out.blockers, fmt.Sprintf("could-not-check: engine %s did not run — %s", eo.name, eo.failReason))
			continue
		}
		if len(eo.canaryRules) == 0 {
			out.blockers = append(out.blockers, "could-not-check: "+deadAlerter(eo.name))
		}
	}
	if len(engines) == 0 {
		out.blockers = append(out.blockers,
			"could-not-check: no engines were selected — a sweep with no engine is a gate that cannot fail")
	}
	return out, nil
}

// runEngine performs one engine's three passes: liveness, canary, tree.
//
// Order is deliberate. The liveness and canary passes run BEFORE the tree pass,
// so an engine that cannot prove itself never gets to contribute a reassuring
// empty finding list. The zero value of engineOutcome has ranOK false, so every
// early return below is already the refusing answer.
func runEngine(e engine, projDir, sampleDir, canaryPath string, rules []rule) *engineOutcome {
	eo := &engineOutcome{name: e.name(), rulesFired: map[string]bool{}}
	bin, version, err := e.locate()
	if err != nil {
		eo.failReason = err.Error()
		return eo
	}
	eo.binPath, eo.version = bin, version

	// Pass 1 — per-rule liveness on the generated sample fixture.
	sampleFindings, err := e.scan(bin, sampleDir, rules)
	if err != nil {
		eo.failReason = "liveness pass failed: " + err.Error()
		return eo
	}
	for _, f := range sampleFindings {
		eo.rulesFired[f.ruleID] = true
	}

	// Pass 2 — the canary, on the engine's OWN stock rules.
	canaryRules, err := e.scanStock(bin, canaryPath)
	if err != nil {
		eo.failReason = "canary pass failed: " + err.Error()
		return eo
	}
	eo.canaryRules = canaryRules

	// Pass 3 — the tree itself.
	treeFindings, err := e.scan(bin, projDir, rules)
	if err != nil {
		eo.failReason = "tree pass failed: " + err.Error()
		return eo
	}
	eo.findings = treeFindings
	eo.ranOK = true
	return eo
}

// assemble is the three-state decision, per entry, across all engines.
func assemble(tl *TokenList, rules []rule, engines []*engineOutcome) []entryResult {
	// Index the rules serving each entry.
	tokenRule := map[string]string{}
	controlRule := map[string]string{}
	for _, r := range rules {
		if r.kind == ruleToken {
			tokenRule[r.token] = r.id
		} else {
			controlRule[r.token] = r.id
		}
	}
	byRule := map[string][]finding{}
	for _, eo := range engines {
		for _, f := range eo.findings {
			byRule[f.ruleID] = append(byRule[f.ruleID], f)
		}
	}

	var out []entryResult
	for _, e := range tl.Entries {
		tid := tokenRule[e.Token]
		cid, hasControl := controlRule[e.Token]

		// (2) and (3): every engine must have proven this entry's rules live, and
		// must have found the control in the tree. Either failure is
		// could-not-check for the entry, whatever the findings say.
		var whyNot []string
		for _, eo := range engines {
			if !eo.ranOK {
				whyNot = append(whyNot, fmt.Sprintf("%s did not run", eo.name))
				continue
			}
			if !eo.rulesFired[tid] {
				whyNot = append(whyNot, fmt.Sprintf("%s: this entry's rule did not fire on its own positive-control sample — the rule is not live in this engine", eo.name))
			}
			if hasControl && !engineFoundRule(eo, cid) {
				whyNot = append(whyNot, fmt.Sprintf("%s: control %q was not found in the tree — the search proved nothing, so a zero token count is not evidence of absence", eo.name, e.Control))
			}
		}

		// Findings, filtered by the allow paths and the exception predicate.
		hits, engineNames := entryHits(e, byRule[tid])
		switch {
		case len(hits) > 0:
			// A definite leak dominates: an entry we KNOW is dirty is reported as
			// dirty even if another engine could not be proven live, because the
			// actionable fact is the leak.
			out = append(out, entryResult{
				token: e.Token, state: stateFailed,
				reason: fmt.Sprintf("%s [caught by %s]", e.Reason, strings.Join(engineNames, ", ")),
				hits:   hits,
			})
		case len(whyNot) > 0:
			out = append(out, entryResult{token: e.Token, state: stateCouldNotCheck, reason: strings.Join(whyNot, "; ")})
		default:
			names := make([]string, 0, len(engines))
			for _, eo := range engines {
				names = append(names, eo.name)
			}
			r := "rule live and 0 hits in " + strings.Join(names, " + ")
			if hasControl {
				r = fmt.Sprintf("control %q present, rule live, 0 hits in %s", e.Control, strings.Join(names, " + "))
			}
			out = append(out, entryResult{token: e.Token, state: stateClean, reason: r})
		}
	}
	return out
}

// beaconRule is the per-file read-proof rule. It is synthesised rather than read
// from the token file, because its job is to check the instrument rather than the
// tree — and it must exist even if the token file were empty.
//
// Its sample plants beacon index 0 so the rule proves itself live in the same
// liveness pass as every house rule; a beacon rule that did not fire would make
// every file look unread, which is loud but wrong, and the liveness pass
// distinguishes the two.
func beaconRule() rule {
	e := &Entry{Token: beaconRuleID, Match: matchPattern, Sample: beaconPrefix + "0"}
	return rule{
		id:      beaconRuleID,
		token:   beaconRuleID,
		kind:    ruleToken,
		regex:   beaconPrefix + "[0-9]+",
		keyword: "leaksweep",
		entry:   e,
	}
}

// coverageBlockers turns the beacons that came back into a per-file statement
// about what each engine actually read.
//
// This is the check that catches a scanner silently skipping a file — measured,
// not assumed. It found a real one on the first run: handed a PDF's extracted
// text with its NUL bytes intact, gitleaks reported `scanned ~0 bytes` and exited
// 0. Under the old shape that was a clean certificate over a tree with a token in
// it. Here it is a named could-not-check for that file.
func coverageBlockers(proj *projection, engines []*engineOutcome) []string {
	var out []string
	for _, eo := range engines {
		if !eo.ranOK {
			continue // already a blocker in its own right
		}
		seen := map[string]bool{}
		for _, f := range eo.findings {
			if f.ruleID != beaconRuleID {
				continue
			}
			p, isCanary := treePath(f.path)
			if isCanary {
				continue
			}
			seen[p] = true
		}
		var missed []string
		for _, path := range proj.beacons {
			if !seen[path] {
				missed = append(missed, path)
			}
		}
		sort.Strings(missed)
		if len(missed) == 0 {
			continue
		}
		shown := missed
		if len(shown) > 8 {
			shown = shown[:8]
		}
		out = append(out, fmt.Sprintf(
			"could-not-check: %s did not read %d of %d projected file(s) — their read-proof beacons never came back, so a token in them would have gone unseen: %s%s",
			eo.name, len(missed), len(proj.beacons), strings.Join(shown, ", "),
			map[bool]string{true: " …", false: ""}[len(missed) > len(shown)]))
	}
	return out
}

// stockSecretToken is the synthetic entry under which an engine's OWN detectors
// report on the swept tree.
const stockSecretToken = "<stock-secret-detectors>"

// stockFindings surfaces findings that came from an engine's built-in rule set
// rather than from a house rule.
//
// trufflehog's `--config` ADDS custom detectors to its built-ins rather than
// replacing them, so its stock detectors run over the swept tree on every house
// pass. Those findings arrive with rule ids that match no house rule. Dropping
// them on the floor — which an id-keyed lookup does silently — would mean a
// secret scanner found a credential in the tree and the gate said nothing. That
// is not a defensible behaviour for a security control, so anything not
// attributable to a house rule or a beacon is reported.
func stockFindings(tl *TokenList, rules []rule, engines []*engineOutcome) (entryResult, bool) {
	known := map[string]bool{beaconRuleID: true}
	for _, r := range rules {
		known[r.id] = true
	}
	seen := map[string]bool{}
	var paths, names []string
	seenEngine := map[string]bool{}
	for _, eo := range engines {
		for _, f := range eo.findings {
			if known[f.ruleID] {
				continue
			}
			p, isCanary := treePath(f.path)
			if isCanary {
				continue
			}
			if stockAllowed(tl, p) {
				continue
			}
			key := p + "|" + f.ruleID
			if seen[key] {
				continue
			}
			seen[key] = true
			paths = append(paths, fmt.Sprintf("%s (%s/%s)", p, eo.name, f.ruleID))
			if !seenEngine[eo.name] {
				seenEngine[eo.name] = true
				names = append(names, eo.name)
			}
		}
	}
	if len(paths) == 0 {
		return entryResult{}, false
	}
	sort.Strings(paths)
	return entryResult{
		token: stockSecretToken, state: stateFailed,
		reason: fmt.Sprintf("a scanner's OWN detectors flagged content in the swept tree [caught by %s]", strings.Join(names, ", ")),
		hits:   paths,
	}, true
}

// stockAllowed reports whether a stock-detector finding in path is expected and
// reviewed. Same path semantics as the per-token `allow`: exact match, or a
// trailing-slash directory prefix.
func stockAllowed(tl *TokenList, path string) bool {
	for _, a := range tl.StockAllow {
		if path == a.Path {
			return true
		}
		if strings.HasSuffix(a.Path, "/") && strings.HasPrefix(path, a.Path) {
			return true
		}
	}
	return false
}

func engineFoundRule(eo *engineOutcome, ruleID string) bool {
	for _, f := range eo.findings {
		if f.ruleID == ruleID {
			return true
		}
	}
	return false
}

// entryHits filters an entry's raw findings down to real leaks: canary-fixture
// findings are dropped (they are the liveness proof), allow-listed paths are
// dropped, and windows fully accounted for by a sanctioned longer form are
// dropped by the exception predicate.
//
// The exception filter runs HERE, in the orchestrator, over every engine's
// findings identically — see rules.go for why that is the authoritative copy and
// the per-engine config allowlists are only a conservative subset of it.
func entryHits(e *Entry, fs []finding) (paths []string, engines []string) {
	seenPath := map[string]bool{}
	seenEngine := map[string]bool{}
	for _, f := range fs {
		p, isCanary := treePath(f.path)
		if isCanary {
			continue
		}
		if allowed(e, p) {
			continue
		}
		if excused(e, f.match) {
			continue
		}
		if !seenPath[p] {
			seenPath[p] = true
			paths = append(paths, p)
		}
		if !seenEngine[f.engine] {
			seenEngine[f.engine] = true
			engines = append(engines, f.engine)
		}
	}
	sort.Strings(paths)
	sort.Strings(engines)
	return paths, engines
}

// worstExit collapses the run to one code, worst-state-wins. A blocker is
// could-not-check by construction — it is a statement that the instrument, not
// the tree, is the thing in doubt.
func (out *runOutcome) worstExit() int {
	leak, cnc := false, len(out.blockers) > 0
	for _, r := range out.results {
		switch r.state {
		case stateFailed:
			leak = true
		case stateCouldNotCheck:
			cnc = true
		}
	}
	if len(out.corpus.unreadable) > 0 || len(out.corpus.unsearchable) > 0 || len(out.corpus.nonRegular) > 0 {
		cnc = true
	}
	switch {
	case leak:
		return exitLeak
	case cnc:
		return exitCouldNotCheck
	default:
		return exitClean
	}
}

func (out *runOutcome) counts() (int, int, int) {
	var clean, failed, cnc int
	for _, r := range out.results {
		switch r.state {
		case stateClean:
			clean++
		case stateFailed:
			failed++
		case stateCouldNotCheck:
			cnc++
		}
	}
	return clean, failed, cnc
}

// write prints the engine banner, the per-entry states, the blockers, the
// summary, and — only on a fully clean run — the certificate.
//
// The engine banner is not decoration. A reader of a green log must be able to
// see WHICH scanners produced it, at which versions, and that each one proved
// itself live on this very run; a certificate whose provenance is invisible is
// the kind you find out was meaningless afterwards.
func (out *runOutcome) write(w io.Writer) {
	for _, eo := range out.engines {
		if !eo.ranOK {
			fmt.Fprintf(w, "ENGINE %s: UNAVAILABLE — %s\n", eo.name, eo.failReason)
			continue
		}
		fmt.Fprintf(w, "ENGINE %s: ran version=%q bin=%s rules-live=%d canary-rules=%d (%s)\n",
			eo.name, eo.version, eo.binPath, len(eo.rulesFired), len(eo.canaryRules),
			strings.Join(eo.canaryRules, ","))
	}
	for _, r := range out.results {
		if r.state == stateFailed {
			fmt.Fprintf(w, "%s %s — %s in %d file(s): %s\n",
				r.state, r.token, r.reason, len(r.hits), strings.Join(r.hits, ", "))
			continue
		}
		fmt.Fprintf(w, "%s %s — %s\n", r.state, r.token, r.reason)
	}
	for _, b := range out.blockers {
		fmt.Fprintf(w, "%s <engine> — %s\n", stateCouldNotCheck, strings.TrimPrefix(b, "could-not-check: "))
	}
	for _, u := range out.corpus.unreadable {
		fmt.Fprintf(w, "%s <unreadable> — could not read %s; a leak could hide in bytes never seen\n", stateCouldNotCheck, u)
	}
	for _, u := range out.corpus.unsearchable {
		fmt.Fprintf(w, "%s <unsearchable-binary> — %s is a binary whose content the sweep cannot exhaustively search (#528)\n", stateCouldNotCheck, u)
	}
	for _, u := range out.corpus.nonRegular {
		fmt.Fprintf(w, "%s <non-regular> — %s is not a regular file (symlink/device); a token could hide in its target string\n", stateCouldNotCheck, u)
	}
	clean, failed, cnc := out.counts()
	extra := len(out.blockers) + len(out.corpus.unreadable) + len(out.corpus.unsearchable) + len(out.corpus.nonRegular)
	fmt.Fprintf(w,
		"SUMMARY: entries=%d clean=%d failed=%d unchecked=%d files-swept=%d unreadable=%d unsearchable=%d non-regular=%d engines=%d tree=%s\n",
		len(out.results), clean, failed, cnc+extra, out.filesSwept,
		len(out.corpus.unreadable), len(out.corpus.unsearchable), len(out.corpus.nonRegular),
		len(out.engines), out.tree)
	if out.worstExit() == exitClean {
		prov := out.provenance
		if prov == "" {
			prov = "no pubmanifest stage marker (swept anyway)"
		}
		var eng []string
		for _, eo := range out.engines {
			eng = append(eng, eo.name+"="+eo.version)
		}
		fmt.Fprintf(w,
			"CERTIFICATE: tree=%s content-hash=sha256:%s files=%d entries-passed=%d engines=%s provenance=%s\n",
			out.tree, out.contentHash, out.filesSwept, clean, strings.Join(eng, ","), prov)
		return
	}
	fmt.Fprintf(w,
		"NOT CERTIFIED: tree=%s content-hash=sha256:%s — %d leak(s), %d unchecked; a tree that could not be fully checked is never certified\n",
		out.tree, out.contentHash, failed, cnc+extra)
}

func removeAllQuiet(dir string) {
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}
