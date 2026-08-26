package main

import (
	"fmt"
	"sort"
	"strings"
)

// riskfilescrossread.go — cross-read a brief's declared paths against the risk
// classifier (mistake-proofing/01, spec §4 B3).
//
// THE HOLE. A brief's review gate is DERIVED correctly from four risk booleans
// the author writes by hand (any "yes" forces the human gate; checkBriefFiles
// enforces the derivation). Nothing checks those booleans against the paths the
// SAME brief declares it will touch. An author can answer all four "no" on a
// brief whose declared paths are workflow, secret, credential or gate-code
// surfaces; the gate then computes to "model" from wrong INPUTS, and the human
// review the risk answers exist to trigger never happens. Every other authoring
// mistake makes a brief worse; this one makes a brief LOOK safe.
//
// This check adds a SECOND, independent layer that fails for a different reason (a
// path match) in a different component (the lint, at pull-request time) than the
// reviewer's read. It does not remove the reviewer layer and is not a claim to.
//
// INPUTS, NOT THE DERIVATION. The derivation is already proofed and correct — a
// brief that touches a security path and honestly answers "yes" on any risk
// question is gate:human and passes cleanly here. This check questions the
// INPUTS: four "no"s on a brief that declares a trigger path. Its message says so
// plainly, because that is the part reviewers read wrong.
//
// POLICY HALF ONLY. The trigger set comes from riskpathtriggers.go, which
// duplicates the desk classifier's POLICY half and NOTHING of its mechanism half
// — see that file's header for why feeding a brief through the top-level accessor
// would class the whole corpus.
//
// SEVERITY PHASING (spec §3 D4 — task 4). riskFilesCrossReadFatal gates the
// class. It lands FALSE — every hit and every could-not-check is an advisory
// NOTICE — so the inherited corpus can be censused and triaged before the check
// bites. A standing NOTICE nobody acts on is negative value, so this is NOT an
// acceptable resting state: the landing pull-request body records the census and
// the condition for the flip to a fatal PROBLEM. See that body and brief 03
// (typed obligation classes), which distinguishes an EDITED declared path from a
// READ-ONLY reference — the distinction the flip waits on, because a read-only
// reference to gate code (as this very brief declares) is a clean model-gated
// brief, not a downgraded one.
//
// COVERAGE BOUNDARY (spec §3 D6, kept beside the code). This check does NOT
// catch: a risky surface that matches no trigger; a brief whose declared paths
// are wrong or absent (that is a could-not-check, reported as itself); and the
// adopter case where only the generic base list is configured. It also does not
// know whether a declared path is edited or merely referenced — that is brief
// 03's typed obligation, and until it exists a read-only reference to a trigger
// path reads here as a hit for a human to confirm.
const riskFilesCrossReadFatal = false

// ruleRiskFilesCrossRead is the stable [rule-tag] bracket token every emitted
// line carries, so lintaudit.go's firing audit can attribute the rule (an
// untagged line falls into its unattributed: bucket).
const ruleRiskFilesCrossRead = "risk-files-crossread"

// riskFilesCrossRead walks every opted-in brief-v1 file in streams and cross-reads
// its declared paths against the per-repo risk-path trigger set. A brief that
// declares a trigger path while answering all four risk questions "no" is
// reported; a brief whose declared-paths line is absent or unparseable is a
// could-not-check. Returns (problems, notices); which one a hit lands in is gated
// by riskFilesCrossReadFatal.
func riskFilesCrossRead(streams []*Stream) (problems, notices []string) {
	tag := "[" + ruleRiskFilesCrossRead + "]"
	emit := func(msg string) {
		if riskFilesCrossReadFatal {
			problems = append(problems, tag+" "+msg)
		} else {
			notices = append(notices, tag+" "+msg)
		}
	}

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				// Malformed files are reported by checkBriefFiles; legacy/opted-out
				// files are exempt here as everywhere else.
				continue
			}

			// COULD-NOT-CHECK: an absent, empty or unparseable declared-paths line
			// is reported AS ITSELF, never rounded up to "no risky paths".
			if !bf.DeclaredPathsFound {
				emit(fmt.Sprintf("%s: brief %s COULD-NOT-CHECK the risk×files cross-read — no parseable `files:` "+
					"declared-paths line in its `## Context` section, so the gate inputs cannot be cross-read "+
					"against the risk-path classifier. This is could-not-check, not a pass: a brief with no "+
					"declared paths is not a brief with no risky paths (docs/three-state-instrument-rule.md). "+
					"Declare the paths this brief touches on a `files:` line under `## Context`",
					path, bf.Brief))
				continue
			}

			// The derivation is only downgraded when EVERY risk answer is "no".
			// A single "yes" already forces gate:human (checkBriefFiles enforces
			// it), so such a brief is correctly gated and never a hit here — this
			// is the test that proves the check does not class an honest brief.
			if riskAnswerYes(bf.Risk) {
				continue
			}

			for _, p := range bf.DeclaredPaths {
				trigger, hit := lintRiskTriggerHit(s.Repo, p)
				if !hit {
					continue
				}
				emit(fmt.Sprintf("%s: brief %s answers all four risk questions \"no\" but declares path %q, which "+
					"matches the security-path trigger %q for %s. This flags the INPUTS to the gate derivation — "+
					"the hand-written risk answers — not the derivation, which correctly computes gate:model from "+
					"four \"no\"s. The question is whether those answers are right for a brief that touches %s: if "+
					"the path is genuinely read-only or non-sensitive the answers stand; otherwise correct the risk "+
					"answer so the human gate fires. (The classification is the generic base list plus %s's topology "+
					"triggers plus any adopter ASSAY_RISK_PATH_TRIGGERS_EXTRA; an adopter that configured nothing "+
					"gets the generic base only.)",
					path, bf.Brief, p, trigger, crossReadRepoLabel(s.Repo), trigger, crossReadRepoLabel(s.Repo)))
				break // one finding per brief is enough to send a human to it
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// riskAnswerYes reports whether any of a brief's four risk answers is "yes".
// Mirrors the anyYes derivation checkBriefFiles uses for the gate.
func riskAnswerYes(risk map[string]string) bool {
	for _, v := range risk {
		if v == "yes" {
			return true
		}
	}
	return false
}

// repoLabel renders a stream's repo for a message, naming the unset case rather
// than printing an empty slug — a stream with no `repo:` gets the base list only.
func crossReadRepoLabel(repo string) string {
	if strings.TrimSpace(repo) == "" {
		return "this stream's repo (unstated — generic base list only)"
	}
	return repo
}
