package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// live.go — the NON-fixture claim source and observation source for `tick`. Neither path is
// exercised by the brief's Verify table (every row runs --claims-fixture /
// --observations-fixture), and neither can be — the offline envelope (C3 in the dispatch
// kit) forbids this session from contacting a live forge to prove it. It is written to the
// established conventions this tree already uses for the two things it does (see the two
// doc comments below), and is exactly as honest as those conventions let it be: a failure
// anywhere in this file surfaces as could-not-check, never a guess.

// dispatchClaimScriptRel mirrors cmd/deskdispatch/dispatch.go's claimScriptRel: the
// CONSUMER repo's own claim script, resolved under --root (deskdispatch additionally
// supports a --claim-root; this tool does not need that split, since it never dispatches —
// it only reads `show`).
const dispatchClaimScriptRel = "tools/dispatch-claim.sh"

// readLiveClaims enumerates every `state=dispatched` dispatch claim for repo, live:
//
//  1. list refs/dispatch/* on the repo's remote (an in-process go-git listing, gitcore.List
//     — never the git binary) to get the claim KEYS (the documented house convention: SKILL.md
//     and the worker prompt both name `git ls-remote origin 'refs/dispatch/*'` for this read);
//  2. for each key, shell to the consumer's own tools/dispatch-claim.sh `show <key> --repo
//     <repo>` — the SAME external script cmd/deskdispatch/dispatch.go already shells to for
//     acquire/show, so this is a second, read-only caller of an existing external contract,
//     not a new one — and parse its `state=`/`age=`/`owner=`/`branch=` fields (state= and
//     age= are the two fields cmd/deskdispatch/dispatch.go's own holderIsStale already
//     parses from this exact output; owner=/branch= follow the same convention, per the
//     brief's own fact that "the message carries owner, state and branch").
//
// Only state=dispatched claims are returned (state=claimed — acquired but never advanced —
// is a different TTL class the brief scopes out: "enumerate state=dispatched claims").
// dispatchedAt is derived from the script's own age=<N>m as now.Add(-N minutes) — the
// script's age math is authoritative; this tool does not re-derive it from a tag timestamp.
func readLiveClaims(root, repo string, now time.Time) ([]claimRecord, error) {
	if strings.TrimSpace(repo) == "" {
		return nil, deskkit.Refused("refused: tick needs --repo OWNER/NAME (or --claims-fixture) to read live dispatch claims")
	}
	script := filepath.Join(root, filepath.FromSlash(dispatchClaimScriptRel))
	if _, err := os.Stat(script); err != nil {
		return nil, deskkit.Unverifiable(
			dispatchClaimScriptRel+" is not present under "+root+
				" — no live claim can be enumerated (point --root at the checkout that carries it, or use --claims-fixture)", err)
	}

	refs, lerr := gitcore.List(gitcore.ListOpts{URL: "https://github.com/" + repo + ".git"})
	if lerr != nil {
		return nil, deskkit.Unverifiable("cannot list refs/dispatch/* on "+repo, lerr)
	}
	var keys []string
	const prefix = "refs/dispatch/"
	for _, r := range refs {
		name := string(r.Name())
		if strings.HasPrefix(name, prefix) {
			keys = append(keys, strings.TrimPrefix(name, prefix))
		}
	}

	var claims []claimRecord
	for _, key := range keys {
		out, serr := exec.Command(script, "show", key, "--repo", repo).CombinedOutput()
		if serr != nil {
			return nil, deskkit.Unverifiable(dispatchClaimScriptRel+" show "+key+" failed: "+strings.TrimSpace(string(out)), serr)
		}
		state := claimShowField(claimStateFieldRe, string(out))
		if state != "dispatched" {
			continue
		}
		ageMin, aerr := strconv.Atoi(claimShowField(claimAgeFieldRe, string(out)))
		if aerr != nil {
			return nil, deskkit.Unverifiable(dispatchClaimScriptRel+" show "+key+": no parseable age= field: "+strings.TrimSpace(string(out)), aerr)
		}
		claims = append(claims, claimRecord{
			Key:          key,
			Item:         key,
			Owner:        claimShowField(claimOwnerFieldRe, string(out)),
			Repo:         repo,
			Branch:       claimShowField(claimBranchFieldRe, string(out)),
			Tier:         "cheap", // the script's `show` carries no tier; see the doc note below
			State:        state,
			DispatchedAt: now.Add(-time.Duration(ageMin) * time.Minute).Format(time.RFC3339),
		})
	}
	return claims, nil
}

// claimStateFieldRe / claimAgeFieldRe / claimOwnerFieldRe / claimBranchFieldRe pull the
// `state=`, `age=<N>m`, `owner=` and `branch=` fields out of the claim tool's `show` output.
// state=/age= mirror cmd/deskdispatch/dispatch.go's own claimStateFieldRe/claimAgeFieldRe
// byte-for-byte (that file is a separate `main` package and cannot be imported, so this is
// a parallel implementation of the SAME external contract, not a divergent one — see the
// file doc's note on why this cannot be verified against a live script in this session).
var (
	claimStateFieldRe  = regexp.MustCompile(`state=([A-Za-z]+)`)
	claimAgeFieldRe    = regexp.MustCompile(`age=(\d+)m`)
	claimOwnerFieldRe  = regexp.MustCompile(`owner=(\S+)`)
	claimBranchFieldRe = regexp.MustCompile(`branch=(\S+)`)
)

func claimShowField(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// KNOWN GAP: `show` carries no per-claim TIER, so a live claim always classifies under
// TierCheap's wall cap (90m) regardless of the dispatch's real exec-tier. Getting the real
// tier onto the claim record is follow-on work (a `tier=` field on the show output, or a
// second read against the dispatch prompt/roster) — flagged here rather than silently
// assumed correct. Every Verify-fixture claim states its own tier explicitly and is
// unaffected by this gap.

// liveObservationSource is the production observationSource: run loopengine.HouseProbes()
// against an Item built from the claim's own fields (Payload carries the identity keys the
// probes read — see probes.go's Payload* constants), looking no earlier than the claim's
// dispatchedAt.
func liveObservationSource(probes *loopengine.ObservableProbes) observationSource {
	return func(claim claimRecord) (resolvedObservation, error) {
		since, err := time.Parse(time.RFC3339, claim.DispatchedAt)
		if err != nil {
			return resolvedObservation{}, fmt.Errorf("claim %s: dispatchedAt %q is not RFC3339: %w", claim.Key, claim.DispatchedAt, err)
		}
		it := loopengine.Item{
			ID: claim.Item,
			Payload: map[string]string{
				loopengine.PayloadSessionTag: claim.Owner,
				loopengine.PayloadRepo:       claim.Repo,
				loopengine.PayloadBranch:     claim.Branch,
			},
		}
		if claim.PR > 0 {
			it.Payload[loopengine.PayloadPR] = strconv.Itoa(claim.PR)
		}
		obs, perr := probes.Latest(it, since)
		if perr != nil {
			return resolvedObservation{observed: false}, nil
		}
		return resolvedObservation{obs: obs, observed: true}, nil
	}
}
