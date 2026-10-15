package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// discover.go — the open scan PR is READ from the forge, not remembered by the session.
//
// THE CLASS THIS CLOSES. The coalesce window decides from the open scan PR it is handed, and the
// only hand-off used to be the --scan-pr flags. The drain is invoked without them (one bare
// `scanloop run --root <repo>` per tick), so every tick read "no open scan PR" and cut a FRESH
// carrier PR — a duplicate of the still-open draft, with the same delta, minutes apart. Whether a
// scan PR is open is a fact about the scan target, not about the session that happens to be
// running: a draft scan PR opened by ANY session of the same App inside the window is the one this
// batch joins.
//
// WHAT COUNTS. An open change on the scan target whose head branch carries the scan branch prefix
// (branchFor) AND whose author is the App this session acts as. A scan-shaped branch pushed by
// another account is never pushed to: the identity is the boundary, the name is only the filter.
//
// UNREADABLE IS NOT NONE. If the open-change list cannot be read, or the App this session acts as
// cannot be established, whether a scan PR is already open is could-not-check — and cutting a fresh
// PR on that reading is exactly the duplicate this file exists to prevent. The scan dispatch is
// refused (exit 6) instead; nothing is pushed, and the next pass retries with a fresh read.

// scanBranchPrefix is the head-branch prefix every scan carrier branch is cut under (branchFor).
const scanBranchPrefix = "chore/intake-scan-"

// listOpenChangesFn reads the scan target's open changes. It is a seam so a test can stand in for
// the forge; production resolves the forge through the same minted-App-token path the trust gate
// uses, never an ambient identity.
var listOpenChangesFn = func(repo string) (*deskkit.OpenChanges, error) {
	f, fr, err := forgeFor(repo)
	if err != nil {
		return nil, err
	}
	return f.ListOpenChanges(fr)
}

// discoverOpenScanPR finds the open scan PR by this session's App on the scan target, or nil when
// there is none. An error means the answer could not be established.
func discoverOpenScanPR(target string, now time.Time, window time.Duration) (*OpenScanPR, error) {
	role, _, rerr := sessionRoleFn("scanloop")
	if rerr != nil {
		return nil, deskkit.Unverifiable("scanloop: cannot establish which App's open scan PR to look for on "+
			target+" — whether a scan PR is already open is could-not-check, and a fresh PR cut on that "+
			"reading would duplicate it", rerr)
	}
	login, ok := deskkit.RoleAppLogin(role)
	if !ok {
		return nil, deskkit.Unverifiable("scanloop: the roster binds no App login to the "+role+
			" role, so its open scan PR on "+target+" cannot be recognised — refusing rather than cutting "+
			"a possible duplicate", nil)
	}
	oc, err := listOpenChangesFn(target)
	if err != nil {
		return nil, deskkit.Unverifiable("scanloop: cannot read the open scan PR on "+target+
			" — whether one is already open is could-not-check, and a fresh PR cut on that reading "+
			"would duplicate it", err)
	}
	return selectOpenScanPR(oc, login, now, window)
}

// selectOpenScanPR picks the open scan PR this batch is decided against: the youngest DRAFT scan PR
// by login, else the youngest flipped one (so the decision reports the flip boundary rather than
// pretending nothing is open). A creation time that does not parse is kept as ZERO, which the
// coalesce policy reads as could-not-check — never as young.
func selectOpenScanPR(oc *deskkit.OpenChanges, login string, now time.Time, window time.Duration) (*OpenScanPR, error) {
	if oc == nil {
		return nil, deskkit.Unverifiable("scanloop: the open-change read returned nothing at all — not an empty list", nil)
	}
	var found []OpenScanPR
	for _, c := range oc.Changes {
		if !strings.HasPrefix(c.HeadRef, scanBranchPrefix) || !strings.EqualFold(c.Author.Login, login) {
			continue
		}
		pr := OpenScanPR{Number: c.Number, Branch: c.HeadRef, State: ScanPRReady}
		if c.Draft {
			pr.State = ScanPRDraft
		}
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(c.CreatedAt)); err == nil {
			pr.CreatedAt = t.UTC()
		}
		found = append(found, pr)
	}
	if len(found) > 0 {
		sort.SliceStable(found, func(i, j int) bool {
			if (found[i].State == ScanPRDraft) != (found[j].State == ScanPRDraft) {
				return found[i].State == ScanPRDraft
			}
			return found[i].CreatedAt.After(found[j].CreatedAt)
		})
		pr := found[0]
		return &pr, nil
	}
	// None seen. A list cut at the forge's page cap is still a complete answer when the oldest
	// change it holds is already at or past the window: the read is newest-first, so every change
	// it did not return is older still and could not absorb this batch. Otherwise the unseen tail
	// may hold a young scan PR, and "none" would be a guess.
	if oc.TruncatedAtCap {
		oldest, ok := oldestCreated(oc.Changes)
		if !ok || now.Sub(oldest) < window {
			return nil, deskkit.Unverifiable(fmt.Sprintf(
				"scanloop: the open-change read stopped at its cap of %d inside the %s coalesce window — "+
					"an open scan PR may be among the changes not returned", oc.Cap, window), nil)
		}
	}
	return nil, nil
}

func oldestCreated(changes []deskkit.OpenChange) (time.Time, bool) {
	var oldest time.Time
	ok := false
	for _, c := range changes {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(c.CreatedAt))
		if err != nil {
			continue
		}
		if !ok || t.Before(oldest) {
			oldest, ok = t, true
		}
	}
	return oldest, ok
}

// onePerPass wraps a reader so the forge is read at most ONCE per pass, on first use: a pass with
// no scan dispatch reads nothing, and a pass with one never re-reads mid-dispatch.
func onePerPass(read func() (*OpenScanPR, error)) func() (*OpenScanPR, error) {
	var (
		once sync.Once
		pr   *OpenScanPR
		err  error
	)
	return func() (*OpenScanPR, error) {
		once.Do(func() { pr, err = read() })
		return pr, err
	}
}
