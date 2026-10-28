package main

// admission.go — the dispatch queue drops the rows `deskdispatch` would refuse at
// claim-acquire (medici-finance/assay#2744), so the ranked list and the dispatcher agree
// about what is startable, and `throughput` does not size a pool against rows no worker
// will take (medici-finance/assay#1958).
//
// Two conditions, both read through deskkit (briefadmission.go), the same functions
// deskdispatch calls:
//
//   - EXEC TIER. With `dispatch --tier T`, a row whose brief declares an `exec-tier:`
//     stricter than T is dropped. With no --tier the board does not know which tier will
//     dispatch from it, so it drops nothing on this axis: hiding strong work from a board
//     a strong session reads would be worse than showing it to a weaker one, which
//     deskdispatch then refuses before any claim.
//
//   - AN UNRULED DECISION. A `gate: human` row whose decision issue is open, carries the
//     decision label and has no ruling recorded since it was applied is dropped.
//
// A row whose brief file cannot be found, or whose state cannot be read, is KEPT and named
// as could-not-check: a read that fails does not hold, the same rule deskdispatch follows,
// so the board never shows a shorter queue than the dispatcher would accept.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// dispatchAdmissionHeld is one row the admission filter dropped, and why.
type dispatchAdmissionHeld struct {
	Repo   string `json:"repo"`
	Brief  string `json:"brief"`
	Reason string `json:"reason"`
}

// decisionForgeFor resolves the forge the decision read uses. A package var so tests
// substitute a fake; production is the board's own forgeFor.
var decisionForgeFor = func(repo string) (deskkit.DecisionForge, deskkit.ForgeRepo, error) {
	f, fr, err := forgeFor(repo)
	if err != nil || f == nil {
		return nil, fr, err
	}
	return f, fr, nil
}

// decisionRatifierFn answers who records a ruling: the configured bless authority, as
// deskdispatch's review gate reads it. ok is false when none is configured.
var decisionRatifierFn = func() (func(login string, id int64) bool, bool) {
	return deskkit.IsBlessAuthorityID, deskkit.BlessAuthorityLogin() != ""
}

// briefFileFor finds the brief file a `<stream>/<NN>` row names under its root. It returns
// "" when there is not exactly one match.
func briefFileFor(row dispatchRow) string {
	stream, num, ok := strings.Cut(row.Brief, "/")
	if !ok || stream == "" || num == "" || strings.ContainsAny(row.Brief, `*?[\`) || row.Root == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(row.Root, "docs", "streams", stream, "brief-"+num+"-*.md"))
	if err != nil || len(matches) != 1 {
		return ""
	}
	return matches[0]
}

// admissionResult is one row's observation. Workers never mutate the report;
// the final fold preserves row, hold and unknown ordering from the input queue.
type admissionResult struct {
	row      dispatchRow
	brief    deskkit.BriefAdmission
	held     string
	tierHeld bool
	unknown  []string
}

func prepareBriefAdmission(row dispatchRow, tier string) admissionResult {
	r := admissionResult{row: row}
	path := briefFileFor(row)
	if path == "" {
		r.unknown = append(r.unknown, "brief file not found under docs/streams — exec-tier and decision not checked")
		return r
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		r.unknown = append(r.unknown, "brief file unreadable — exec-tier and decision not checked")
		return r
	}
	r.brief = deskkit.ParseBriefAdmission(string(raw))
	r.row.ExecTier = r.brief.ExecTier
	if tier != "" {
		covers, err := deskkit.TierCovers(tier, r.brief.ExecTier)
		switch {
		case err != nil:
			r.unknown = append(r.unknown, err.Error())
		case !covers:
			r.tierHeld = true
			r.held = fmt.Sprintf("exec-tier: %s is stricter than --tier %s", r.brief.ExecTier, tier)
		}
	}
	return r
}

// applyBriefAdmission drops the rows a dispatch at tier (or "" for no tier) would refuse,
// counting each by reason. It runs on the merged queue, after every cap.
func applyBriefAdmission(rep *dispatchReport, tier string) {
	rep.DispatchTier = tier
	ratifier, haveRatifier := decisionRatifierFn()
	if !haveRatifier {
		ratifier = nil
	}
	results := make([]admissionResult, len(rep.Rows))
	sweeps := map[string]*deskkit.DecisionSweep{}
	repos := map[string]deskkit.ForgeRepo{}
	for i, row := range rep.Rows {
		results[i] = prepareBriefAdmission(row, tier)
		if results[i].held != "" || !results[i].brief.GatesHuman() {
			continue
		}
		if _, resolved := sweeps[row.Repo]; resolved {
			continue
		}
		f, fr, err := decisionForgeFor(row.Repo)
		sweeps[row.Repo] = nil // failed resolution is shared as unknown, never retried per row
		if err == nil && f != nil {
			sweeps[row.Repo] = deskkit.NewDecisionSweep(f, fr, ratifier)
			repos[row.Repo] = fr
		}
	}
	// One global pool: no nested per-repo fanout multiplies the read burst bound.
	results, _ = sweepConcurrent(results, sweepConcurrency, func(r admissionResult) (admissionResult, error) {
		if r.held != "" || !r.brief.GatesHuman() {
			return r, nil
		}
		sweep := sweeps[r.row.Repo]
		if sweep == nil {
			r.unknown = append(r.unknown, "gate: human — the forge for its decision issue could not be resolved")
			return r, nil
		}
		w := sweep.Wait(r.brief)
		switch {
		case w.Unknown != "":
			r.unknown = append(r.unknown, "gate: human — "+w.Unknown)
		case w.Waiting:
			r.held = fmt.Sprintf("gate: human — decision issue %s#%d is open with no ruling recorded", repos[r.row.Repo].Slug(), w.Issue)
		}
		return r, nil
	})
	kept := make([]dispatchRow, 0, len(rep.Rows))
	for _, r := range results {
		for _, why := range r.unknown {
			rep.AdmissionUnknown = append(rep.AdmissionUnknown, dispatchDegraded{Repo: r.row.Repo, Root: r.row.Root, Reason: r.row.Brief + ": " + why})
		}
		if r.held != "" {
			if r.tierHeld {
				rep.HeldByExecTier++
			} else {
				rep.HeldByDecision++
			}
			rep.AdmissionHeld = append(rep.AdmissionHeld, dispatchAdmissionHeld{Repo: r.row.Repo, Brief: r.row.Brief, Reason: r.held})
			continue
		}
		kept = append(kept, r.row)
	}
	rep.Rows = kept
}
