package main

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeRosterForge serves the two reads deskroster makes through the Forge seam —
// GetPullRequest and ListOpenChanges — from the FAKEGH_* env fixtures the existing suites set.
// It embeds deskkit.Forge, so any OTHER op deskroster never calls would panic if reached.
//
// The env formats are preserved verbatim from the former fake-gh binary so no existing test
// body changed: FAKEGH_PR_<num>=state|isDraft|title (state is gh-style OPEN/MERGED/CLOSED),
// the FAKEGH_PR_MERGED_<num>=1 / FAKEGH_PR_CLOSED_<num>=1 shortcuts, and
// FAKEGH_LIST_PRS="num:draft:title;num:draft:title;...". The gh-style state is mapped back onto
// the interface's semantics (State "open"/"closed" plus a separate Merged flag) here, which is
// exactly the shape the real GitHub backend returns and ghViewPR derives its display state from.
type fakeRosterForge struct {
	deskkit.Forge
}

var fakeRosterForgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, _ := strings.Cut(repo, "/")
	return &fakeRosterForge{}, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
}

func (fakeRosterForge) GetPullRequest(_ deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error) {
	prNum := strconv.Itoa(number)
	if v := os.Getenv("FAKEGH_PR_" + prNum); v != "" {
		parts := strings.SplitN(v, "|", 3)
		state, draft, title := parts[0], "false", ""
		if len(parts) > 1 {
			draft = parts[1]
		}
		if len(parts) > 2 {
			title = parts[2]
		}
		return ghStateToPR(number, state, draft == "true", title), nil
	}
	if os.Getenv("FAKEGH_PR_MERGED_"+prNum) == "1" {
		return &deskkit.PullRequest{Number: number, State: "closed", Merged: true, Title: "merged pr " + prNum}, nil
	}
	if os.Getenv("FAKEGH_PR_CLOSED_"+prNum) == "1" {
		return &deskkit.PullRequest{Number: number, State: "closed", Title: "closed pr " + prNum}, nil
	}
	return &deskkit.PullRequest{Number: number, State: "open", Draft: true, Title: "draft pr " + prNum}, nil
}

func (fakeRosterForge) ListOpenChanges(deskkit.ForgeRepo) (*deskkit.OpenChanges, error) {
	oc := &deskkit.OpenChanges{Cap: 50}
	prs := os.Getenv("FAKEGH_LIST_PRS")
	if prs == "" {
		return oc, nil
	}
	for _, e := range strings.Split(prs, ";") {
		if e == "" {
			continue
		}
		f := strings.SplitN(e, ":", 3)
		num, _ := strconv.Atoi(f[0])
		draft, title := false, ""
		if len(f) > 1 {
			draft = f[1] == "true"
		}
		if len(f) > 2 {
			title = f[2]
		}
		oc.Changes = append(oc.Changes, deskkit.OpenChange{Number: num, Title: title, Draft: draft, State: "OPEN"})
	}
	return oc, nil
}

// ghStateToPR maps a gh-style state word onto the interface's PullRequest semantics.
func ghStateToPR(number int, ghState string, draft bool, title string) *deskkit.PullRequest {
	p := &deskkit.PullRequest{Number: number, Draft: draft, Title: title}
	switch strings.ToUpper(ghState) {
	case "MERGED":
		p.State, p.Merged = "closed", true
	case "CLOSED":
		p.State = "closed"
	default:
		p.State = "open"
	}
	return p
}

// stubForgeReturning installs a forgeFor that returns pr / oc for the duration of the test,
// so the mapping assertions below feed crafted interface values directly rather than via env.
func stubForgeReturning(t *testing.T, pr *deskkit.PullRequest, prErr error, oc *deskkit.OpenChanges, ocErr error) {
	t.Helper()
	prev := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return &mappingForge{pr: pr, prErr: prErr, oc: oc, ocErr: ocErr}, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	t.Cleanup(func() { forgeFor = prev })
}

type mappingForge struct {
	deskkit.Forge
	pr    *deskkit.PullRequest
	prErr error
	oc    *deskkit.OpenChanges
	ocErr error
}

func (m *mappingForge) GetPullRequest(_ deskkit.ForgeRepo, _ int) (*deskkit.PullRequest, error) {
	return m.pr, m.prErr
}
func (m *mappingForge) ListOpenChanges(deskkit.ForgeRepo) (*deskkit.OpenChanges, error) {
	return m.oc, m.ocErr
}

// TestGhViewPR_DerivesDisplayState pins the state-derivation ghViewPR does over the interface's
// (State + Merged) shape: the display expects the uppercase OPEN/MERGED/CLOSED the old
// `gh pr view --json state` returned, and a merged change (State "closed", Merged true, OR a
// non-empty MergedAt) must read as MERGED so the roster's auto-prune fires. A mapping that
// dropped the merged derivation would leave merged PRs displayed as CLOSED and never pruned.
func TestGhViewPR_DerivesDisplayState(t *testing.T) {
	cases := []struct {
		name      string
		pr        *deskkit.PullRequest
		wantState string
		wantDraft bool
		wantTitle string
	}{
		{"open draft", &deskkit.PullRequest{State: "open", Draft: true, Title: "wip"}, "OPEN", true, "wip"},
		{"open ready", &deskkit.PullRequest{State: "open", Draft: false, Title: "ready"}, "OPEN", false, "ready"},
		{"merged flag", &deskkit.PullRequest{State: "closed", Merged: true, Title: "m"}, "MERGED", false, "m"},
		{"merged via MergedAt", &deskkit.PullRequest{State: "closed", MergedAt: "2026-09-01T00:00:00Z", Title: "m2"}, "MERGED", false, "m2"},
		{"closed unmerged", &deskkit.PullRequest{State: "closed", Title: "c"}, "CLOSED", false, "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.pr.Number = 7
			stubForgeReturning(t, tc.pr, nil, nil, nil)
			got := ghViewPR("example-org/tracker", 7)
			if got == nil {
				t.Fatalf("ghViewPR returned nil, want a PRInfo")
			}
			if got.State != tc.wantState {
				t.Errorf("State = %q, want %q", got.State, tc.wantState)
			}
			if got.IsDraft != tc.wantDraft {
				t.Errorf("IsDraft = %v, want %v", got.IsDraft, tc.wantDraft)
			}
			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			if got.Number != 7 {
				t.Errorf("Number = %d, want 7", got.Number)
			}
		})
	}
}

// TestGhViewPR_SoftFailsOnForgeError pins that a forge-resolution or read failure returns nil —
// the graceful degradation the caller renders as "?" — never a panic or a spurious verdict.
func TestGhViewPR_SoftFailsOnForgeError(t *testing.T) {
	stubForgeReturning(t, nil, deskkit.Unverifiable("no readable identity", nil), nil, nil)
	if got := ghViewPR("example-org/tracker", 3); got != nil {
		t.Fatalf("ghViewPR on a read error = %+v, want nil", got)
	}
}

// TestGhListOpenPRs_MapsChanges pins that ListOpenChanges' rows map onto PRInfo by
// number/title/draft, and that a read error is a soft nil.
func TestGhListOpenPRs_MapsChanges(t *testing.T) {
	oc := &deskkit.OpenChanges{Cap: 50, Changes: []deskkit.OpenChange{
		{Number: 11, Title: "first", Draft: true, State: "OPEN"},
		{Number: 12, Title: "second", Draft: false, State: "OPEN"},
	}}
	stubForgeReturning(t, nil, nil, oc, nil)
	got := ghListOpenPRs("example-org/tracker")
	if len(got) != 2 {
		t.Fatalf("got %d PRInfo, want 2 (%+v)", len(got), got)
	}
	if got[0].Number != 11 || got[0].Title != "first" || !got[0].IsDraft {
		t.Errorf("row 0 = %+v, want {11 first draft}", got[0])
	}
	if got[1].Number != 12 || got[1].Title != "second" || got[1].IsDraft {
		t.Errorf("row 1 = %+v, want {12 second ready}", got[1])
	}

	stubForgeReturning(t, nil, nil, nil, deskkit.Unverifiable("read failed", nil))
	if got := ghListOpenPRs("example-org/tracker"); got != nil {
		t.Fatalf("ghListOpenPRs on a read error = %+v, want nil", got)
	}
}
