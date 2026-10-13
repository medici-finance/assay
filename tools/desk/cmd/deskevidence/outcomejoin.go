package main

// outcomejoin.go — the forge-driven half of the verify-outcome join keys:
// the dereference of every `prs` entry (Task 3) and the register redaction (Task 4). The pure
// input-shape checks are deskkit.ParseJoinInput; this file runs only for an ADMITTED landing,
// at step 4 of the writer's step order (see cmdOutcomeRecordWrite).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forgeKindFn answers WHICH forge software serves the landing's repository. It is a package var
// (beside forgeForFn) so a test can stand a GitLab-shaped fake behind it; production binds it to
// the resolver's own answer. The ancestry rule is chosen by this kind, never by catching a
// CompareRefs could-not-check — that would turn an outage into a weaker check.
var forgeKindFn = func(owner, name string) (deskkit.ForgeKind, error) {
	res, err := deskkit.ForgeKindFor(deskkit.ForgeRepo{Owner: owner, Name: name})
	if err != nil {
		return "", err
	}
	return res.Kind, nil
}

// joinEntry is one dereferenced prs entry: the forge's canonical repository spelling, plus
// whether that repository is exactly public.
type joinEntry struct {
	Repo     string // canonical <owner>/<name>, as the forge spells it
	Number   int
	MergeSHA string
	Public   bool // the ENTRY repo's normalised live visibility is exactly "public"
}

// joinFacts is everything the redaction needs, read from the forge before it runs.
type joinFacts struct {
	Private bool        // the register's normalised live visibility is exactly "private"
	Entries []joinEntry // one per input prs entry, in order
}

// normVisibility is the allow-list normalisation of a live visibility read (lowercase, trimmed),
// the same shape as the public-repo gate's.
func normVisibility(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

// resolveJoinFacts dereferences every prs entry (exists, merged, merge commit, ancestry) and
// reads the visibilities the redaction needs. recSHA and recTS are the record's own sha and ts.
// A refusal about the PR is exit 5; any forge read that fails, an empty merge commit on a merged
// change, a canonical name the URL does not yield, and a read the register's token cannot make
// are exit 6.
func resolveJoinFacts(fg deskkit.Forge, fr deskkit.ForgeRepo, kind deskkit.ForgeKind, in deskkit.JoinInput, recSHA, recTS string) (joinFacts, error) {
	var facts joinFacts
	for i, pr := range in.PRs {
		e, err := dereferencePR(fg, fr, kind, i, pr, recSHA, recTS)
		if err != nil {
			return facts, err
		}
		facts.Entries = append(facts.Entries, e)
	}
	if len(in.PRs) == 0 && in.DispatchRef == "" {
		return facts, nil // nothing to redact: no visibility read
	}
	regVis, err := fg.RepoVisibility(fr)
	if err != nil {
		return facts, deskkit.Unverifiable("could-not-check: cannot read the visibility of the register repository "+fr.Slug(), err)
	}
	facts.Private = normVisibility(regVis) == "private"
	if facts.Private {
		return facts, nil // a private register keeps prs in clear; no per-entry read
	}
	for i := range facts.Entries {
		e := &facts.Entries[i]
		if strings.EqualFold(e.Repo, fr.Slug()) {
			e.Public = normVisibility(regVis) == "public"
			continue
		}
		er, _ := entryForgeRepo(e.Repo)
		v, verr := fg.RepoVisibility(er)
		if verr != nil {
			return facts, deskkit.Unverifiable("could-not-check: cannot read the visibility of "+e.Repo+" (prs["+strconv.Itoa(i)+"])", verr)
		}
		e.Public = normVisibility(v) == "public"
	}
	return facts, nil
}

func entryForgeRepo(slug string) (deskkit.ForgeRepo, bool) {
	o, n, ok := strings.Cut(slug, "/")
	return deskkit.ForgeRepo{Owner: o, Name: n}, ok
}

// canonicalRepoFromURL extracts <owner>/<name> from a change's page URL: the path before
// /pull/<n> (GitHub) or /-/merge_requests/<n> (GitLab).
func canonicalRepoFromURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false
	}
	p := strings.Trim(u.Path, "/")
	for _, sep := range []string{"/-/merge_requests/", "/pull/"} {
		if i := strings.Index(p, sep); i > 0 {
			p = p[:i]
			break
		}
	}
	if !deskkit.ValidPRRepo(p) {
		return "", false
	}
	return p, true
}

func dereferencePR(fg deskkit.Forge, fr deskkit.ForgeRepo, kind deskkit.ForgeKind, idx int, pr deskkit.DeliveredPR, recSHA, recTS string) (joinEntry, error) {
	at := fmt.Sprintf("prs[%d] (%s#%d)", idx, pr.Repo, pr.Number)
	er, _ := entryForgeRepo(pr.Repo)

	got, err := fg.GetPullRequest(er, pr.Number)
	if err != nil {
		if deskkit.IsForgeNotFound(err) {
			// A 404 on another repository is ambiguous: GitHub answers it for a private
			// repository the register's token cannot read, the same as for a missing change.
			// Only the register's own repository (which the token demonstrably reads) can
			// refuse "does not exist"; anywhere else it is could-not-check.
			if !strings.EqualFold(pr.Repo, fr.Slug()) {
				return joinEntry{}, deskkit.Unverifiable("could-not-check: cannot read "+at+" — the forge answered not found, which for a repository other than the register means the change is missing or the register's token cannot read it", err)
			}
			return joinEntry{}, deskkit.Refused("refused: " + at + " does not exist on the forge (exists)")
		}
		return joinEntry{}, deskkit.Unverifiable("could-not-check: cannot read "+at, err)
	}
	if got == nil {
		return joinEntry{}, deskkit.Unverifiable("could-not-check: the forge returned no change for "+at, nil)
	}
	if !got.Merged {
		return joinEntry{}, deskkit.Refused("refused: " + at + " is not merged (merged)")
	}
	canon, ok := canonicalRepoFromURL(got.URL)
	if !ok {
		return joinEntry{}, deskkit.Unverifiable("could-not-check: the forge's URL for "+at+" ("+strconv.Quote(got.URL)+") does not name an <owner>/<name> repository", nil)
	}
	if got.MergeCommitSHA == "" {
		return joinEntry{}, deskkit.Unverifiable("could-not-check: the forge reports no merge commit for the merged change "+at, nil)
	}
	if strings.ToLower(got.MergeCommitSHA) != pr.MergeSHA {
		return joinEntry{}, deskkit.Refused("refused: " + at + " merge_sha " + pr.MergeSHA + " is not the change's merge commit (merge commit)")
	}
	cr, _ := entryForgeRepo(canon)
	switch kind {
	case deskkit.ForgeGitLab:
		// CompareRefs is deferred on GitLab: the writer instead requires the change to have
		// merged into the repository's default branch no later than the record's ts. Weaker than
		// ancestry — it does not show the record's sha contains the merge commit.
		def, derr := fg.RepoDefaultBranch(cr)
		if derr != nil {
			return joinEntry{}, deskkit.Unverifiable("could-not-check: cannot read the default branch of "+canon, derr)
		}
		if got.BaseRef != def {
			return joinEntry{}, deskkit.Refused(fmt.Sprintf("refused: %s merged into %q, not the default branch %q of %s (ancestry)", at, got.BaseRef, def, canon))
		}
		merged, perr := time.Parse(time.RFC3339, got.MergedAt)
		if perr != nil {
			return joinEntry{}, deskkit.Unverifiable("could-not-check: the forge's merge time for "+at+" is not an RFC3339 timestamp", perr)
		}
		ts, terr := time.Parse(time.RFC3339, recTS)
		if terr != nil {
			return joinEntry{}, deskkit.Refused("refused: ts " + strconv.Quote(recTS) + " is not an RFC3339 timestamp, so " + at + " cannot be ordered against it")
		}
		if merged.After(ts) {
			return joinEntry{}, deskkit.Refused("refused: " + at + " merged at " + got.MergedAt + ", after the record's ts " + recTS + " (ancestry)")
		}
	default:
		head := ""
		if strings.EqualFold(canon, fr.Slug()) {
			head = strings.TrimSpace(recSHA)
			if head == "" {
				return joinEntry{}, deskkit.Refused("refused: sha is empty — " + at + " is checked as an ancestor of the record's sha")
			}
		} else {
			// Only a cross-repo entry needs the default branch; a same-repo entry is
			// checked against the record's sha and makes no extra forge read.
			def, derr := fg.RepoDefaultBranch(cr)
			if derr != nil {
				return joinEntry{}, deskkit.Unverifiable("could-not-check: cannot read the default branch of "+canon, derr)
			}
			head = def
		}
		cmp, cerr := fg.CompareRefs(cr, pr.MergeSHA, head)
		if cerr != nil {
			return joinEntry{}, deskkit.Unverifiable("could-not-check: cannot compare "+pr.MergeSHA+" with "+head+" on "+canon+" for "+at, cerr)
		}
		if cmp == nil || (cmp.Status != "identical" && cmp.Status != "ahead") {
			status := ""
			if cmp != nil {
				status = cmp.Status
			}
			return joinEntry{}, deskkit.Refused(fmt.Sprintf("refused: %s merge_sha is not an ancestor of %s on %s (ancestry; compare status %q)", at, head, canon, status))
		}
	}
	return joinEntry{Repo: canon, Number: pr.Number, MergeSHA: pr.MergeSHA}, nil
}

// withheldDigest is the writer's `withheld` value for a prs entry the register's audience may
// not see: sha256("<canonical repo>#<number>@<merge_sha>").
func withheldDigest(e joinEntry) string {
	return deskkit.Sha256Hex([]byte(e.Repo + "#" + strconv.Itoa(e.Number) + "@" + e.MergeSHA))
}

// redactOutcomeFn is the register redaction behind a function variable (like forgeForFn and
// publicRepoGateFn), so the step-5 TARGET-TIE check can be driven by a rewrite that is not the
// real one.
var redactOutcomeFn = redactOutcomeRecord

// redactOutcomeRecord rewrites a record for the register it lands in. It is an allow-list:
//
//   - every prs entry names its repository's CANONICAL spelling;
//   - a private register keeps prs in clear and replaces dispatch_ref with dispatch_ref_sha256
//     (sha256 of the FULL clear ref); the clear value is dropped;
//   - any other register writes a prs entry in clear only when the ENTRY repository is exactly
//     public, else only {withheld}; dispatch_ref is dropped and not hashed.
//
// A record with nothing to rewrite is returned byte for byte.
func redactOutcomeRecord(raw []byte, in deskkit.JoinInput, f joinFacts) ([]byte, error) {
	if in.DispatchRef == "" && !prsChange(in.PRs, f) {
		return raw, nil
	}
	fields := map[string]json.RawMessage{}
	for k, v := range in.Fields {
		fields[k] = v
	}
	if in.PRsPresent && len(in.PRs) > 0 {
		out := make([]deskkit.DeliveredPR, 0, len(f.Entries))
		for _, e := range f.Entries {
			if f.Private || e.Public {
				out = append(out, deskkit.DeliveredPR{Repo: e.Repo, Number: e.Number, MergeSHA: e.MergeSHA})
			} else {
				out = append(out, deskkit.DeliveredPR{Withheld: withheldDigest(e)})
			}
		}
		b, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		fields["prs"] = b
	}
	if in.DispatchRef != "" {
		delete(fields, "dispatch_ref")
		if f.Private {
			b, err := json.Marshal(deskkit.Sha256Hex([]byte(in.DispatchRef)))
			if err != nil {
				return nil, err
			}
			fields["dispatch_ref_sha256"] = b
		}
	}
	return marshalNoHTMLEscape(fields)
}

// marshalNoHTMLEscape encodes v as compact JSON without escaping <, > and & in strings, so a
// rewritten record keeps its submitted string bytes. Object keys are still emitted in sorted order.
func marshalNoHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// prsChange reports whether redaction would rewrite any prs entry: a different (canonical)
// spelling, or a withheld form.
func prsChange(in []deskkit.DeliveredPR, f joinFacts) bool {
	if len(in) != len(f.Entries) {
		return len(in) > 0
	}
	for i, e := range f.Entries {
		if in[i].Repo != e.Repo {
			return true
		}
		if !f.Private && !e.Public {
			return true
		}
	}
	return false
}

// tieOutcomeTarget is the second half of the TARGET-TIE rule: the record the writer is about to
// name and write must carry the very brief and ts that steps 1-2 checked and admission bound,
// and its FINAL path must still be admitted by the receipt step 2 produced. A rewrite that moves
// either, or a path the receipt does not cover, is exit 5 with nothing written.
func tieOutcomeTarget(receipt deskkit.VerifierReceipt, brief, ts string, final deskkit.OutcomeRecord, finalPath string) error {
	if final.Brief != brief {
		return deskkit.Refused("refused: the rewritten --outcome-record names brief " + strconv.Quote(final.Brief) +
			", not the admitted " + strconv.Quote(brief))
	}
	if final.TS != ts {
		return deskkit.Refused("refused: the rewritten --outcome-record carries ts " + strconv.Quote(final.TS) +
			", not the checked " + strconv.Quote(ts))
	}
	if err := receipt.CheckEvidenceTarget(finalPath); err != nil {
		return deskkit.Refused("refused: the --outcome-record's final target " + finalPath + " is not bound to the admitted run: " + err.Error())
	}
	return nil
}

// checkRecordTier cross-checks a self-reported verifier_tier against the admitted binding:
// when the binding carries a Tier, a different value is refused.
func checkRecordTier(receipt deskkit.VerifierReceipt, in deskkit.JoinInput) error {
	bound := strings.TrimSpace(receipt.Binding.Tier)
	if bound == "" {
		return nil
	}
	v, has := in.Fields["verifier_tier"]
	if !has {
		return nil
	}
	var tier string
	if json.Unmarshal(v, &tier) != nil || tier != bound {
		return deskkit.Refused("refused: verifier_tier " + strings.TrimSpace(string(v)) + " disagrees with the admitted run's tier " + strconv.Quote(bound))
	}
	return nil
}
