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

// applyBriefAdmission drops the rows a dispatch at tier (or "" for no tier) would refuse,
// counting each by reason. It runs on the merged queue, after every cap.
func applyBriefAdmission(rep *dispatchReport, tier string) {
	rep.DispatchTier = tier
	ratifier, haveRatifier := decisionRatifierFn()
	if !haveRatifier {
		ratifier = nil
	}
	unknown := func(row dispatchRow, why string) {
		rep.AdmissionUnknown = append(rep.AdmissionUnknown, dispatchDegraded{
			Repo: row.Repo, Root: row.Root, Reason: row.Brief + ": " + why,
		})
	}
	kept := make([]dispatchRow, 0, len(rep.Rows))
	for _, row := range rep.Rows {
		path := briefFileFor(row)
		if path == "" {
			unknown(row, "brief file not found under docs/streams — exec-tier and decision not checked")
			kept = append(kept, row)
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			unknown(row, "brief file unreadable — exec-tier and decision not checked")
			kept = append(kept, row)
			continue
		}
		b := deskkit.ParseBriefAdmission(string(raw))
		row.ExecTier = b.ExecTier
		if tier != "" {
			covers, terr := deskkit.TierCovers(tier, b.ExecTier)
			switch {
			case terr != nil:
				unknown(row, terr.Error())
			case !covers:
				rep.HeldByExecTier++
				rep.AdmissionHeld = append(rep.AdmissionHeld, dispatchAdmissionHeld{
					Repo: row.Repo, Brief: row.Brief,
					Reason: fmt.Sprintf("exec-tier: %s is stricter than --tier %s", b.ExecTier, tier),
				})
				continue
			}
		}
		if b.GatesHuman() {
			f, fr, ferr := decisionForgeFor(row.Repo)
			if ferr != nil || f == nil {
				unknown(row, "gate: human — the forge for its decision issue could not be resolved")
				kept = append(kept, row)
				continue
			}
			w := deskkit.BriefDecisionWait(f, fr, b, ratifier)
			switch {
			case w.Unknown != "":
				unknown(row, "gate: human — "+w.Unknown)
			case w.Waiting:
				rep.HeldByDecision++
				rep.AdmissionHeld = append(rep.AdmissionHeld, dispatchAdmissionHeld{
					Repo: row.Repo, Brief: row.Brief,
					Reason: fmt.Sprintf("gate: human — decision issue %s#%d is open with no ruling recorded", fr.Slug(), w.Issue),
				})
				continue
			}
		}
		kept = append(kept, row)
	}
	rep.Rows = kept
}
