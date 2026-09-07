package deskkit

import "testing"

// rosterWithApps installs a roster binding the worker and reviewer role Apps, so the
// resolution under test reads real slugs rather than a hard-coded login.
func rosterWithApps(t *testing.T) {
	t.Helper()
	withRoster(t, map[string]string{
		EnvBlessLogin: "ada:2001",
		EnvTrustedBotSlugs: "worker=github:assay-worker-app:300000006," +
			"reviewer=github:assay-reviewer-app:300000004",
	})
}

func TestAuthorIsRoleApp(t *testing.T) {
	rosterWithApps(t)
	cases := []struct {
		login string
		want  bool
	}{
		{"assay-worker-app[bot]", true},    // a bound role App, [bot] rendering
		{"Assay-Worker-App[bot]", true},    // case-insensitive
		{"app/assay-reviewer-app", true},   // app/ rendering
		{"assay-verifier-app[bot]", false}, // a real slug shape but NOT in this roster
		{"ada", false},                     // a human login, never an App
		{"", false},                        // empty is never an App
		{"assay-worker-app", false},        // bare slug (no [bot]/app/) never resolves
	}
	for _, c := range cases {
		if got := AuthorIsRoleApp(c.login); got != c.want {
			t.Errorf("AuthorIsRoleApp(%q) = %v, want %v", c.login, got, c.want)
		}
	}
}

func TestAuthorIsRoleApp_unconfiguredFailsClosed(t *testing.T) {
	withNoRoster(t)
	if AuthorIsRoleApp("assay-worker-app[bot]") {
		t.Fatal("an unconfigured roster must not assert App-ness (fail closed)")
	}
}

// TestTrailerAbsentAppAnomaly is the #587 proof-it-can-fail: the anomaly fires for a role-App
// PR with no trailer, and stays clean for every other combination. Prove each against the
// pre-fix behaviour — before this rule existed, a trailer-less App PR was NOT risk-classed.
func TestTrailerAbsentAppAnomaly(t *testing.T) {
	rosterWithApps(t)
	const briefBody = "## Context\nDelivers the thing.\n\nBrief: some-stream/04\n"
	const issueBody = "## Context\nIssue-only work.\n\nIssue: #123\n"
	const noTrailer = "## Context\nA change with no link trailer at all.\n"
	const twoBriefs = "Brief: a/01\nBrief: b/02\n" // present but malformed (multiplicity)

	cases := []struct {
		name   string
		author string
		body   string
		want   bool
	}{
		{"App author, NO trailer -> ANOMALY", "assay-worker-app[bot]", noTrailer, true},
		{"App author, Brief: trailer -> clean", "assay-worker-app[bot]", briefBody, false},
		{"App author, Issue: trailer -> clean", "assay-reviewer-app[bot]", issueBody, false},
		{"HUMAN author, NO trailer -> not this anomaly", "ada", noTrailer, false},
		{"App author, malformed-but-present trailer -> not absent", "assay-worker-app[bot]", twoBriefs, false},
		{"non-roster bot, NO trailer -> not our App", "assay-random-app[bot]", noTrailer, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TrailerAbsentAppAnomaly(c.author, []byte(c.body)); got != c.want {
				t.Fatalf("TrailerAbsentAppAnomaly(%q, ...) = %v, want %v", c.author, got, c.want)
			}
		})
	}
}
