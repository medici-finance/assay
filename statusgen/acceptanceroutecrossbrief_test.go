package main

import (
	"strings"
	"testing"
	"time"
)

// assertAdjudicatedRouteRefuses drives all three effect boundaries for brief
// with the fixture's current preparation and ruling: the issuer, the close
// plan and the canonical card close. Each must refuse with errText and leave
// the board unwritten.
func assertAdjudicatedRouteRefuses(t *testing.T, f adjudicatedRouteFixture, brief, errText string) {
	t.Helper()
	c := f.forge.client(t)
	before := getFile(t, f.readme)
	issue, err := makeAdjudicatedVerifyIssue(f.root, brief, rlIssue, f.ruling, c, rlLogins, false)
	if err == nil || !strings.Contains(err.Error(), errText) || issue.Body != "" {
		t.Fatalf("issuer: wanted no card and a refusal containing %q, got body=%q err=%v", errText, issue.Body, err)
	}
	_, plan, _, err := planAdjudicatedClose(f.root, brief, f.manifest, rlIssue, f.ruling, time.Now(), c, rlLogins, false)
	if err == nil || !strings.Contains(err.Error(), errText) || len(plan) != 0 || getFile(t, f.readme) != before {
		t.Fatalf("close plan: wanted no plan and a refusal containing %q, got %v", errText, err)
	}
	digest, err := f.manifest.digest()
	if err != nil {
		t.Fatal(err)
	}
	routeMarker, err := adjudicatedCardRef{Preparation: rlIssue, Ruling: f.ruling, Manifest: digest}.marker()
	if err != nil {
		t.Fatal(err)
	}
	cfg := writeAdjudicatedRouteCardFixture(t, adjudicatedRouteCard(verifyMarker(brief)+"\n"+routeMarker))
	err = closeAdjudicatedCard(f.root, brief, rlRepo, 60, c, cfg)
	t.Setenv("CREDIT_FAKE_CI", "") // later local-mode reads in the same test
	if err == nil || !strings.Contains(err.Error(), errText) || getFile(t, f.readme) != before {
		t.Fatalf("card close: wanted a refusal containing %q and no write, got %v", errText, err)
	}
}

// The route serves any gate: human brief, so a preparation or adjudication
// made for one brief must never authorize another (#4993).
func TestAdjudicatedRouteNeverCrossesBriefs(t *testing.T) {
	t.Run("preparation and ruling for X replayed for Y", func(t *testing.T) {
		f := newAdjudicatedRouteFixtureWith(t, "", "", true)
		if _, err := locateAcceptanceBrief(f.root, "vf/02"); err != nil {
			t.Fatalf("fixture needs a second human-gated brief: %v", err)
		}
		c := f.forge.client(t)
		if _, err := makeAdjudicatedVerifyIssue(f.root, "vf/01", rlIssue, f.ruling, c, rlLogins, false); err != nil {
			t.Fatalf("control: X's own records must still issue X's card: %v", err)
		}
		if got, err := acceptancePreparationBriefRef(rlRepo, rlIssue, false); err != nil || got != "vf/01" {
			t.Fatalf("the issuer must take X from X's preparation, got %q (%v)", got, err)
		}
		assertAdjudicatedRouteRefuses(t, f, "vf/02", "the preparation binds brief")
	})

	t.Run("adjudication of X cited on Y's preparation", func(t *testing.T) {
		f := newAdjudicatedRouteFixtureWith(t, "", "", true)
		xText, err := acceptanceAdjudicationText(f.manifest, rlIssue)
		if err != nil {
			t.Fatal(err)
		}
		f.adjudicate(t, f.manifestFor(t, adjudicatedTwinBrief(f.root)))
		c := f.forge.client(t)
		// Control: Y's own exact adjudication clears the ruling check, so the
		// refusal below is caused by the substituted comment alone.
		if _, err := makeAdjudicatedVerifyIssue(f.root, "vf/02", rlIssue, f.ruling, c, rlLogins, false); err != nil &&
			strings.Contains(err.Error(), "exact acceptance adjudication") {
			t.Fatalf("control: Y's own exact adjudication was refused: %v", err)
		}
		f.forge.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, xText, rlPosted, rlPosted)
		assertAdjudicatedRouteRefuses(t, f, "vf/02", "does not approve this exact acceptance adjudication")
	})

	t.Run("derived brief must be a canonical brief file", func(t *testing.T) {
		f := newAdjudicatedRouteFixtureWith(t, "", "", false)
		m := f.manifest
		m.Source.BriefPath = "docs/streams/vf/README.md"
		f.adjudicate(t, m)
		if got, err := acceptancePreparationBriefRef(rlRepo, rlIssue, false); err == nil {
			t.Fatalf("a manifest naming a non-brief file yielded %q", got)
		}
	})
}

// A preparation or adjudication binds one immutable source. Replaying it after
// the brief changed, or at a revision off the checked-out line, must refuse,
// and the new source needs its own exact adjudication (#4993).
func TestAdjudicatedRouteRefusesStaleRevisionReplay(t *testing.T) {
	t.Run("brief changed after adjudication", func(t *testing.T) {
		f := newAdjudicatedRouteFixture(t, "", "")
		oldText, err := acceptanceAdjudicationText(f.manifest, rlIssue)
		if err != nil {
			t.Fatal(err)
		}
		putFile(t, f.brief, strings.Replace(getFile(t, f.brief), "Gate: model.\n", "Gate: model.\nA later review note.\n", 1))
		vfCommit(t, f.root, vfMailVerifier)
		assertAdjudicatedRouteRefuses(t, f, "vf/01", "differs from the committed blob")

		// The changed brief needs its own preparation and its own exact
		// adjudication; the old ruling never carries over to it.
		fresh := f.manifestFor(t, f.brief)
		if fresh.Source.Revision == f.manifest.Source.Revision || fresh.Source.BriefBlob == f.manifest.Source.BriefBlob {
			t.Fatal("a fresh preparation must bind the new revision and blob")
		}
		f.adjudicate(t, fresh)
		c := f.forge.client(t)
		if _, err := makeAdjudicatedVerifyIssue(f.root, "vf/01", rlIssue, f.ruling, c, rlLogins, false); err != nil &&
			strings.Contains(err.Error(), "exact acceptance adjudication") {
			t.Fatalf("control: the fresh exact adjudication was refused: %v", err)
		}
		f.forge.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, oldText, rlPosted, rlPosted)
		assertAdjudicatedRouteRefuses(t, f, "vf/01", "does not approve this exact acceptance adjudication")
	})

	t.Run("revision off the checked-out line", func(t *testing.T) {
		f := newAdjudicatedRouteFixture(t, "", "")
		vfGit(t, f.root, vfMailVerifier, "commit", "-q", "--allow-empty", "-m", "off the line")
		off := vfRev(t, f.root, "HEAD")
		vfGit(t, f.root, vfMailVerifier, "reset", "-q", "--hard", "HEAD~1")
		m := f.manifest
		m.Source.Revision = off
		f.adjudicate(t, m)
		assertAdjudicatedRouteRefuses(t, f, "vf/01", "not a committed ancestor of HEAD")
	})
}
