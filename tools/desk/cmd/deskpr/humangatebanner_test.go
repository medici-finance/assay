package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for brief-18 Task 3: the fixed PR-body banner required of any PR delivering a
// gate:human brief. Pure, offline, no git/network — a temp dir standing in for --root.

func writeBriefFixture(t *testing.T, root, stream, nn, frontmatterExtra string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nschema: brief-v1\nbrief: " + stream + "/" + nn + "\ntitle: fixture\n" +
		frontmatterExtra + "\n---\n\nfixture brief body.\n"
	if err := os.WriteFile(filepath.Join(dir, "brief-"+nn+"-fixture.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckHumanGateBanner_NotGateHuman_NoOp(t *testing.T) {
	root := t.TempDir()
	writeBriefFixture(t, root, "fixture", "01", "gate: model")
	if err := checkHumanGateBanner([]byte("anything at all"), root, root, "fixture", "01"); err != nil {
		t.Fatalf("gate:model brief must never require the banner, got: %v", err)
	}
}

func TestCheckHumanGateBanner_UnresolvableBrief_NoOp(t *testing.T) {
	root := t.TempDir() // no brief written at all
	if err := checkHumanGateBanner([]byte("anything"), root, root, "fixture", "99"); err != nil {
		t.Fatalf("an unresolvable brief must not error here (requireTrailer already refused it): %v", err)
	}
}

func TestCheckHumanGateBanner_GateHuman_NoBanner_Refused(t *testing.T) {
	root := t.TempDir()
	writeBriefFixture(t, root, "fixture", "02", "gate: human")
	err := checkHumanGateBanner([]byte("a plain PR body with no banner at all\n\nBrief: fixture/02\n"), root, root, "fixture", "02")
	if err == nil {
		t.Fatal("gate:human brief with no banner must be refused")
	}
	if !strings.Contains(err.Error(), humanGateBannerMarker) {
		t.Errorf("refusal must name the required marker, got: %v", err)
	}
}

func TestCheckHumanGateBanner_GateHuman_BannerButNoIssueNumber_NoRecordedIssue_Allowed(t *testing.T) {
	root := t.TempDir()
	writeBriefFixture(t, root, "fixture", "03", "gate: human") // no decision-issue: field
	body := "> " + humanGateBannerMarker + " — no decision issue filed yet.\n\nBrief: fixture/03\n"
	if err := checkHumanGateBanner([]byte(body), root, root, "fixture", "03"); err != nil {
		t.Fatalf("banner honestly stating 'no decision issue filed yet' must be accepted, got: %v", err)
	}
}

func TestCheckHumanGateBanner_GateHuman_BannerPresentButSaysNothingAboutState_Refused(t *testing.T) {
	root := t.TempDir()
	writeBriefFixture(t, root, "fixture", "04", "gate: human")
	body := "> " + humanGateBannerMarker + " — see the brief for details.\n\nBrief: fixture/04\n"
	err := checkHumanGateBanner([]byte(body), root, root, "fixture", "04")
	if err == nil {
		t.Fatal("a banner that never states the (absent) decision issue's status must be refused")
	}
}

func TestCheckHumanGateBanner_GateHuman_HasDecisionIssue_MustNameIt(t *testing.T) {
	root := t.TempDir()
	writeBriefFixture(t, root, "fixture", "05", "gate: human\ndecision-issue: 42")
	// Banner present, but does not cite #42 at all.
	body := "> " + humanGateBannerMarker + " — pending.\n\nBrief: fixture/05\n"
	if err := checkHumanGateBanner([]byte(body), root, root, "fixture", "05"); err == nil {
		t.Fatal("banner that never names the actual decision issue number must be refused")
	}
	// Now with the issue named -> allowed.
	body2 := "> " + humanGateBannerMarker + " #42 (state as of authoring: unread).\n\nBrief: fixture/05\n"
	if err := checkHumanGateBanner([]byte(body2), root, root, "fixture", "05"); err != nil {
		t.Fatalf("banner naming the real decision issue must be accepted, got: %v", err)
	}
}

func TestCheckHumanGateBannerFromBody_NoTrailer_NoOp(t *testing.T) {
	root := t.TempDir()
	if err := checkHumanGateBannerFromBody([]byte("no trailer here at all"), root, root); err != nil {
		t.Fatalf("a body with no Brief:/Issue: trailer must be a no-op here, got: %v", err)
	}
}

func TestCheckHumanGateBannerFromBody_IssueTrailer_NoOp(t *testing.T) {
	root := t.TempDir()
	if err := checkHumanGateBannerFromBody([]byte("Issue: #7\n"), root, root); err != nil {
		t.Fatalf("an Issue: trailer must be a no-op here (this check applies to Brief: PRs only), got: %v", err)
	}
}

func TestCheckHumanGateBannerFromBody_BriefTrailer_EndToEnd(t *testing.T) {
	root := t.TempDir()
	writeBriefFixture(t, root, "fixture", "06", "gate: human")
	noBanner := "Brief: fixture/06\n"
	if err := checkHumanGateBannerFromBody([]byte(noBanner), root, root); err == nil {
		t.Fatal("Brief: trailer resolving to a gate:human brief with no banner must be refused")
	}
	withBanner := "> " + humanGateBannerMarker + " — no decision issue filed yet.\n\nBrief: fixture/06\n"
	if err := checkHumanGateBannerFromBody([]byte(withBanner), root, root); err != nil {
		t.Fatalf("Brief: trailer resolving to a gate:human brief WITH the honest banner must pass, got: %v", err)
	}
}
