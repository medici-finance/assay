package main

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
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

// readLiveClaims is the unbounded read-only status inventory. Mutating ticks use
// readLiveClaimsPage so a backlog cannot turn one tick into unbounded forge traffic.
func readLiveClaims(root, repo string, now time.Time) ([]claimRecord, error) {
	claims, _, err := readLiveClaimsPage(repo, now, 0, "")
	return claims, err
}

func readLiveClaimsPage(repo string, now time.Time, limit int, after string) ([]claimRecord, string, error) {
	opts, err := claimListOpts(repo)
	if err != nil {
		return nil, "", err
	}
	refs, err := gitcore.List(opts)
	if err != nil {
		return nil, "", deskkit.Unverifiable("cannot list dispatch claims on "+repo, err)
	}
	return claimPage(refs, repo, now, limit, after, func(hash plumbing.Hash) (gitcore.TagPayload, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return gitcore.FetchTagPayload(ctx, opts.URL, opts.Auth, hash)
	})
}

// A payload is fetched by the advertised immutable object id. Mutable `show`
// output followed by a remembered ref id would mix two owners during a race.
func claimPage(refs []*plumbing.Reference, repo string, now time.Time, limit int, after string, fetch func(plumbing.Hash) (gitcore.TagPayload, error)) ([]claimRecord, string, error) {
	var selected []*plumbing.Reference
	for _, ref := range refs {
		key, ok := strings.CutPrefix(ref.Name().String(), deskkit.DispatchClaimActiveRefsPrefix)
		if ok && key > after && !strings.HasSuffix(key, "^{}") {
			selected = append(selected, ref)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Name() < selected[j].Name() })
	next := ""
	if limit > 0 && len(selected) > limit {
		selected = selected[:limit]
		next = strings.TrimPrefix(selected[len(selected)-1].Name().String(), deskkit.DispatchClaimActiveRefsPrefix)
	}
	var claims []claimRecord
	for _, ref := range selected {
		key := strings.TrimPrefix(ref.Name().String(), deskkit.DispatchClaimActiveRefsPrefix)
		if key == "" || strings.Contains(key, "/") {
			return nil, next, deskkit.Unverifiable("malformed claim key held for inspection: "+ref.Name().String(), nil)
		}
		payload, err := fetch(ref.Hash())
		if err != nil {
			return nil, next, deskkit.Unverifiable("claim held for inspection: "+ref.Name().String()+" at "+ref.Hash().String(), err)
		}
		fields := strings.Fields(payload.Message)
		if len(fields) < 5 || fields[0] != "dispatch-claim" || fields[1] != key {
			return nil, next, deskkit.Unverifiable("malformed claim held for inspection: "+ref.Name().String(), nil)
		}
		owner, ownerOK := strings.CutPrefix(fields[2], "owner=")
		state, stateOK := strings.CutPrefix(fields[3], "state=")
		branch, branchOK := strings.CutPrefix(fields[4], "branch=")
		if !ownerOK || !stateOK || !branchOK {
			return nil, next, deskkit.Unverifiable("malformed claim held for inspection: "+ref.Name().String(), nil)
		}
		if owner == "" || branch == "" || (state != "claimed" && state != "dispatched") || payload.When.IsZero() {
			return nil, next, deskkit.Unverifiable("unverifiable claim held for inspection: "+ref.Name().String(), nil)
		}
		if state != "dispatched" {
			continue
		}
		claim := claimRecord{Key: key, Item: key, Repo: repo, Owner: owner, Branch: branch, Tier: "cheap", State: state, DispatchedAt: payload.When.UTC().Format(time.RFC3339), Ref: ref.Name().String(), Version: ref.Hash().String()}
		if target, err := decodeClaimTarget(key, repo); err == nil {
			claim.Item, claim.PR = target.item, target.pr
		}
		// Unknown forms stay visible and are held blind by live reconciliation.
		claims = append(claims, claim)
	}
	return claims, next, nil
}

// claimListOpts builds the ListOpts readLiveClaims lists the claim namespace with: the repo's
// forge resolved by deskkit from the roster (ASSAY_REPO_FORGES), the session's token role
// resolved exactly as the reclaim leg does (actions.go's doReclaim → deskkit.SessionTokenRole),
// and the role's credential paired with the forge's git-basic username, dialed at the RESOLVED
// KIND's canonical instance host. It deliberately does NOT read the --root checkout's origin:
// the repo being listed is independent of that checkout (a desk sweeping another repo's claims),
// so its origin host names an unrelated forge, and binding the credential to it is a
// cross-forge credential leak (#1197 security review S1). deskkit.ForgeGitEndpointFor derives
// the host from the forge kind instead. Every failure — no loop identity, no forge, no
// instance host, no token — is could-not-check (exit 6), never an anonymous attempt whose empty
// answer could read as "no claims held".
func claimListOpts(repo string) (gitcore.ListOpts, error) {
	role, _, rerr := deskkit.SessionTokenRole("desksupervise")
	if rerr != nil {
		return gitcore.ListOpts{}, deskkit.Unverifiable(
			"cannot list "+deskkit.DispatchClaimActiveRefsPrefix+"*"+" on "+repo+": no session token role to list as", rerr)
	}
	ep, eerr := deskkit.ForgeGitEndpointFor(repo, role)
	if eerr != nil {
		return gitcore.ListOpts{}, deskkit.Unverifiable(
			"cannot list "+deskkit.DispatchClaimActiveRefsPrefix+"*"+" on "+repo+": no authenticated forge endpoint", eerr)
	}
	return ep.Opts, nil
}

// showClaim remains the consumer-script reader used by eligibility reconciliation.
// Initial inventory reads immutable payloads directly rather than mixing snapshots.
func showClaim(script, key, repo string) (string, error) {
	out, err := exec.Command(script, "show", key, "--repo", repo).CombinedOutput()
	return string(out), err
}

// claimStateFieldRe, claimAgeFieldRe, claimOwnerFieldRe and claimBranchFieldRe pull the
// `state=`, `age=<N>m`, `owner=` and `branch=` fields out of the claim tool's `show` output.
// state= and age= mirror cmd/deskdispatch/dispatch.go's own claimStateFieldRe and
// claimAgeFieldRe byte-for-byte (that file is a separate `main` package and cannot be
// imported, so this is a parallel implementation of the SAME external contract, not a
// divergent one — see the file doc's note on why this cannot be verified against a live
// script in this session).
var (
	claimStateFieldRe  = regexp.MustCompile(`state=([A-Za-z]+)`)
	claimAgeFieldRe    = regexp.MustCompile(`age=(\d+)m`)
	claimOwnerFieldRe  = regexp.MustCompile(`owner=(\S+)`)
	claimBranchFieldRe = regexp.MustCompile(`branch=(\S+)`)
)

// withoutAliasLines drops the ALIAS-HELD lines `show` appends for another spelling of the same PR
// (medici-finance/assay#3700). Those describe a DIFFERENT claim, so a field read for the key
// itself (owner=) must not take its value from them: on `FREE <key>` + `ALIAS-HELD ...` the FREE
// line carries no owner=, and the alias's would otherwise read as the key's holder.
func withoutAliasLines(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.Contains(line, "ALIAS-HELD ") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func claimShowField(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// The claim payload has no tier, so the existing cheap-tier liveness policy
// remains the default. This change binds that policy to an immutable claim;
// it does not expand the liveness evidence or treat age alone as death.

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

// Well-formed but unattributable or clock-skewed claims are held individually.
// Neither condition is evidence that their owner is dead.
func claimInventoryBlind(c claimRecord, now time.Time) string {
	if c.Owner == "unknown" {
		return "claim-owner-unknown"
	}
	if at, err := time.Parse(time.RFC3339, c.DispatchedAt); err == nil && at.After(now) {
		return "claim-future-timestamp"
	}
	return ""
}
