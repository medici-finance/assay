package deskkit

import (
	"strings"
	"testing"
)

// The fixture roster used by every case below. It reuses the package fixture's own repo
// census (rosterfixture_test.go): example-org/example-k8s is PUBLIC and is the write target
// in these tests; example-org/tracker and medici-finance/assay are PRIVATE and are the
// spans a public body must not name.
const (
	scPublicRepo  = "example-org/example-k8s"
	scPrivateRepo = "example-org/tracker"
)

func scRoster(t *testing.T) {
	t.Helper()
	withRoster(t, map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedLogins:   "ada:2001",
		EnvTrustedBotSlugs: "worker=assay-worker-app:300000006",
		EnvAllowedRepos: "example-org/example-k8s:ci:public,example-org/tracker:ci:private," +
			"example-org/proposals:no-ci:public,medici-finance/assay:ci:private",
		EnvRepoAliases: "example-org/tracker=trk:products",
	})
}

// TestSelfContainRefusesOnePerCategory is the golden table: one body per REFUSING category,
// each carrying exactly one offending span so the refusal message can be asserted to name
// it. Every row here is a shape that has reached, or could reach, a public surface.
func TestSelfContainRefusesOnePerCategory(t *testing.T) {
	scRoster(t)
	t.Setenv(EnvWithheldIdentifiers, "closed-stream,other-closed")

	cases := []struct {
		name     string
		body     string
		wantSpan string
		wantCat  string
	}{
		{
			name:     "absolute path under /Users",
			body:     "I ran it from /Users/operator/work/checkout and it passed.",
			wantSpan: "/Users/operator/work/checkout",
			wantCat:  "absolute machine path",
		},
		{
			name:     "absolute path under a scratch worktree root",
			body:     "the branch lives in /private/tmp/tracker-thing/tools/desk",
			wantSpan: "/private/tmp/tracker-thing/tools/desk",
			wantCat:  "absolute machine path",
		},
		{
			name:     "bare scratch worktree name",
			body:     "run it in tracker-widget-04 before pushing",
			wantSpan: "tracker-widget-04",
			wantCat:  "scratch worktree name",
		},
		{
			name:     "session id",
			body:     "session fbf250ed-be0d-4832-9ebb-2bff546eebbf recorded the run",
			wantSpan: "fbf250ed-be0d-4832-9ebb-2bff546eebbf",
			wantCat:  "session id",
		},
		{
			name:     "agent id",
			body:     "dispatched as agent-af6a38b6b77428ec5 on the strong tier",
			wantSpan: "agent-af6a38b6b77428ec5",
			wantCat:  "agent id",
		},
		{
			name:     "private repo named by full slug",
			body:     "the same guard lives in " + scPrivateRepo + " already",
			wantSpan: scPrivateRepo,
			wantCat:  "private repository name",
		},
		{
			name:     "cross-repo ref into a private repo",
			body:     "this closes " + scPrivateRepo + "#1420 as well",
			wantSpan: scPrivateRepo + "#1420",
			wantCat:  "cross-repo reference",
		},
		{
			name:     "cross-repo ref through a configured private alias",
			body:     "see trk#1420 for the original report",
			wantSpan: "trk#1420",
			wantCat:  "cross-repo reference",
		},
		{
			name:     "withheld register stream slug",
			body:     "the rationale is recorded in the closed-stream register",
			wantSpan: "closed-stream",
			wantCat:  "withheld register identifier",
		},
		{
			name:     "withheld register brief id",
			body:     "delivered by closed-stream/07 last week",
			wantSpan: "closed-stream/07",
			wantCat:  "withheld register identifier",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := SelfContainCheck("PR body", []byte(c.body),
				SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: io_Discard{}})
			if err == nil {
				t.Fatalf("body %q was ADMITTED; it must refuse (%s)", c.body, c.wantCat)
			}
			if ExitCodeOf(err) != ExitRefused {
				t.Fatalf("exit code = %d, want %d (refused)", ExitCodeOf(err), ExitRefused)
			}
			if !strings.Contains(err.Error(), c.wantSpan) {
				t.Errorf("refusal does not NAME the span %q: %s", c.wantSpan, err.Error())
			}
			if !strings.Contains(err.Error(), c.wantCat) {
				t.Errorf("refusal does not name the category %q: %s", c.wantCat, err.Error())
			}
			if !strings.Contains(err.Error(), "PR body") {
				t.Errorf("refusal does not name the surface: %s", err.Error())
			}
		})
	}
}

// TestSelfContainAdmitsSelfContainedBodies is the other direction, and it is the half that
// decides whether the check is usable at all: a scan that refuses ordinary review prose
// starves the write budget exactly as the entropy rule did (#209, #1255).
func TestSelfContainAdmitsSelfContainedBodies(t *testing.T) {
	scRoster(t)
	t.Setenv(EnvWithheldIdentifiers, "closed-stream")

	cases := []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"plain prose", "LGTM — the mapping table reads correctly and the fixtures cover both directions."},
		{
			"identifier-heavy review body",
			// Split across concatenation for the same reason bodycheck_test.go's fixtures
			// are: written contiguously, these are exactly the runs the PRE-fix scanner
			// refuses, so the PR that fixes them could not be pushed by the tool it repairs.
			"`" + "TestCIRequiredMatches" + "AllowedRepoPolicy` and `" +
				"TestPDFIsByteIdentical" + "AcrossRenders` " +
				"both pass; see tools/desk/internal/deskkit/bodycheck.go:45 for the rule.",
		},
		{
			"public-only cross-repo refs",
			"tracked at example-org/proposals#12; the sibling change is example-org/example-k8s#3.",
		},
		{
			"repo-relative paths and go module paths",
			"ok github.com/medici-finance/assay/tools/desk/internal/deskkit 2.531s — " +
				"the change is in tools/desk/cmd/deskpr/deskpr.go:187.",
		},
		{
			"the target repo may name ITSELF",
			"this PR opens against " + scPublicRepo + " and closes " + scPublicRepo + "#7.",
		},
		{
			// A stream slug the withheld set does NOT name must pass untouched, or the
			// register category would refuse every brief reference a public repo makes
			// about its own published streams.
			"a non-withheld stream slug and brief id",
			"delivered by example-stream/02; brief `docs/streams/example-stream/brief-02.md`.",
		},
		{
			"a git sha and a version tag",
			"verified at 5d529c27e3b1a04f9c2d8e7b6a1f0c3d4e5f6a7b, released as v0.14.0.",
		},
		{
			"a relative tmp path that is not a machine root",
			"the cache lives under tmp/build/go/pkg/mod on the runner.",
		},
		{
			// The #380 shape: text DOCUMENTING the check has to spell the roots it covers,
			// and a guard that refuses its own documentation is a guard that gets routed
			// around instead of fixed. A bare root names no user, machine or directory, so
			// tolerating it costs no detection — see reAbsMachinePath's trailing `+`.
			"the roots spelled bare, as this check's own documentation must",
			"REFUSED: an absolute machine path (`/Users/`, `/home/`, `/private/tmp/`, " +
				"`/private/var/folders/`, `/tmp/tracker-`).",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := SelfContainCheck("PR body", []byte(c.body),
				SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: io_Discard{}}); err != nil {
				t.Fatalf("self-contained body was REFUSED: %v\nbody: %s", err, c.body)
			}
		})
	}
}

// TestSelfContainPrivateAndUnconfiguredAreUnchanged pins #203's stated boundary from both
// sides. A body that refuses on a public repo must pass UNCHANGED on a private one, and on
// a deployment that has no roster at all — otherwise this change would hand every existing
// adopter a wall of new refusals on writes the tool has never gated.
func TestSelfContainPrivateAndUnconfiguredAreUnchanged(t *testing.T) {
	// Every refusing category at once.
	body := "ran in /Users/op/work on session fbf250ed-be0d-4832-9ebb-2bff546eebbf " +
		"in tracker-thing; see " + scPrivateRepo + "#1420 and closed-stream/07."

	t.Run("private target repo", func(t *testing.T) {
		scRoster(t)
		t.Setenv(EnvWithheldIdentifiers, "closed-stream")
		if err := SelfContainCheck("PR body", []byte(body),
			SelfContainOpts{Repo: scPrivateRepo, Notices: io_Discard{}}); err != nil {
			t.Fatalf("a PRIVATE target repo must be unchanged, got: %v", err)
		}
		if SelfContainApplies(scPrivateRepo) {
			t.Error("SelfContainApplies said a known-private repo is in scope")
		}
	})

	t.Run("unconfigured roster", func(t *testing.T) {
		withNoRoster(t)
		t.Setenv(EnvWithheldIdentifiers, "closed-stream")
		if err := SelfContainCheck("PR body", []byte(body),
			SelfContainOpts{Repo: scPublicRepo, Notices: io_Discard{}}); err != nil {
			t.Fatalf("an UNCONFIGURED roster must be unchanged, got: %v", err)
		}
		if SelfContainApplies(scPublicRepo) {
			t.Error("SelfContainApplies said an unconfigured deployment is in scope")
		}
	})

	t.Run("a repo whose visibility is UNSTATED is in scope (fail closed)", func(t *testing.T) {
		withRoster(t, map[string]string{
			EnvBlessLogin:      "ada:2001",
			EnvTrustedLogins:   "ada:2001",
			EnvTrustedBotSlugs: "worker=assay-worker-app:300000006",
			EnvAllowedRepos:    "example-org/unstated:ci",
		})
		if !SelfContainApplies("example-org/unstated") {
			t.Fatal("a repo with no stated visibility must be IN scope — unstated must not " +
				"read as private, or leaving one field blank silently disables the scan")
		}
	})
}

// TestSelfContainConfigAbsentDegradesToNotice is the adopter-usability property #203 names
// explicitly: a deployment with no withheld register must not be blocked by a category it
// cannot satisfy, and the scan must SAY that the category went unchecked rather than
// letting a could-not-check read as clean.
func TestSelfContainConfigAbsentDegradesToNotice(t *testing.T) {
	scRoster(t)
	t.Setenv(EnvWithheldIdentifiers, "")

	body := "the rationale is recorded in the closed-stream register, delivered by closed-stream/07."
	var out strings.Builder
	err := SelfContainCheck("PR body", []byte(body),
		SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: &out})
	if err != nil {
		t.Fatalf("with %s unset the register category must NOTICE, never refuse; got: %v",
			EnvWithheldIdentifiers, err)
	}
	if !strings.Contains(out.String(), "NOT CHECKED") ||
		!strings.Contains(out.String(), EnvWithheldIdentifiers) {
		t.Errorf("the unchecked category was not reported as itself; notices were:\n%s", out.String())
	}
}

// TestSelfContainBareRefIsNoticeNotRefusal pins the heuristic half. A bare `#N` is the
// CORRECT spelling for this repo's own issues, so refusing it would refuse the normal case;
// the check warns on the ones this repo cannot own and says so when it cannot tell.
func TestSelfContainBareRefIsNoticeNotRefusal(t *testing.T) {
	scRoster(t)
	t.Setenv(EnvWithheldIdentifiers, "closed-stream")

	t.Run("a number this repo plausibly owns is silent", func(t *testing.T) {
		var out strings.Builder
		if err := SelfContainCheck("PR body", []byte("fixes #12 as reported"),
			SelfContainOpts{Repo: scPublicRepo, NumberHint: 200, Notices: &out}); err != nil {
			t.Fatalf("bare #N below the hint must not refuse: %v", err)
		}
		if strings.Contains(out.String(), "#12") {
			t.Errorf("a plausible local ref should not be noticed: %s", out.String())
		}
	})

	t.Run("a number far above the hint is a NOTICE, not a refusal", func(t *testing.T) {
		var out strings.Builder
		if err := SelfContainCheck("PR body", []byte("this mirrors #1420 on the other tracker"),
			SelfContainOpts{Repo: scPublicRepo, NumberHint: 200, Notices: &out}); err != nil {
			t.Fatalf("a bare #N must never refuse — #203 asks for a warning: %v", err)
		}
		if !strings.Contains(out.String(), "#1420") {
			t.Errorf("the implausible ref was not noticed at all: %s", out.String())
		}
	})

	t.Run("no hint reports the category unchecked", func(t *testing.T) {
		var out strings.Builder
		if err := SelfContainCheck("PR body", []byte("this mirrors #1420"),
			SelfContainOpts{Repo: scPublicRepo, NumberHint: 0, Notices: &out}); err != nil {
			t.Fatalf("a bare #N must never refuse: %v", err)
		}
		if !strings.Contains(out.String(), "NOT CHECKED") {
			t.Errorf("with no reference number the category must report itself unchecked: %s",
				out.String())
		}
	})
}

// TestSelfContainPrivateShortNameIsNoticeOnly pins the OTHER heuristic: a repo's short name
// is frequently an ordinary word, so it warns rather than refusing. The FULL slug is what
// refuses, and that asymmetry is the whole reason both exist.
func TestSelfContainPrivateShortNameIsNoticeOnly(t *testing.T) {
	scRoster(t)
	t.Setenv(EnvWithheldIdentifiers, "closed-stream")

	var out strings.Builder
	if err := SelfContainCheck("PR body", []byte("the tracker board already carries this row"),
		SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: &out}); err != nil {
		t.Fatalf("a bare short name must not refuse: %v", err)
	}
	if !strings.Contains(out.String(), "tracker") {
		t.Errorf("the private repo's short name was not noticed: %s", out.String())
	}

	// The noise floor: an alias under minShortNameNotice characters is ordinary English and
	// is not noticed at all. `trk` is the configured alias of a private repo in this
	// fixture's roster, and a notice keyed on a token that short fires on prose.
	var short strings.Builder
	if err := SelfContainCheck("PR body", []byte("the trk column reads correctly"),
		SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: &short}); err != nil {
		t.Fatalf("a short alias must not refuse: %v", err)
	}
	if strings.Contains(short.String(), "\"trk\"") {
		t.Errorf("a %d-character alias must stay under the noise floor; notices were:\n%s",
			len("trk"), short.String())
	}
	// …but the FULL slug of that same repo still refuses, at any alias length.
	if err := SelfContainCheck("PR body", []byte("see "+scPrivateRepo+" for the original"),
		SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: io_Discard{}}); !IsRefused(err) {
		t.Fatalf("the full private slug must still refuse whatever its alias length: %v", err)
	}
}

// io_Discard is a local io.Writer sink. The tests that assert on REFUSALS do not care about
// notices, and routing them to a sink keeps `go test` output readable while still exercising
// the real notice path.
type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }

// scRosterWithheld is scRoster plus a roster-carried ASSAY_WITHHELD_IDENTIFIERS value. The
// fixture writes an actual roster.env, so this is the real configuration path an adopter
// uses — not a stand-in for it.
func scRosterWithheld(t *testing.T, withheld string) {
	t.Helper()
	withRoster(t, map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedLogins:   "ada:2001",
		EnvTrustedBotSlugs: "worker=assay-worker-app:300000006",
		EnvAllowedRepos: "example-org/example-k8s:ci:public,example-org/tracker:ci:private," +
			"example-org/proposals:no-ci:public,medici-finance/assay:ci:private",
		EnvRepoAliases:         "example-org/tracker=trk:products",
		EnvWithheldIdentifiers: withheld,
	})
}

// TestWithheldIdentifiersResolveThroughRoster is #490's regression: the withheld register set
// must resolve the way every other ASSAY_ key resolves — roster.env, overridden by the
// environment — rather than from the environment ALONE.
//
// The bug it pins is the silent half of a control failure, which is why each arm asserts on
// the SCAN's behaviour and not only on the accessor: the accessor returning nil is invisible,
// while a public body that names a withheld slug and is allowed through is the damage.
func TestWithheldIdentifiersResolveThroughRoster(t *testing.T) {
	// (a) ROSTER-ONLY, no environment at all. This is the configuration the bug report
	// describes: roster.env carries the key, every shell that runs deskpr/deskpost does not.
	t.Run("roster only reaches the scan", func(t *testing.T) {
		scRosterWithheld(t, "closed-stream,other-closed")
		t.Setenv(EnvWithheldIdentifiers, "")

		if got := WithheldIdentifiers(); len(got) != 2 || got[0] != "closed-stream" ||
			got[1] != "other-closed" {
			t.Fatalf("a roster-configured withheld set must reach WithheldIdentifiers(); got %q", got)
		}
		var out strings.Builder
		err := SelfContainCheck("PR body", []byte("delivered by closed-stream/07, see the register"),
			SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: &out})
		if !IsRefused(err) {
			t.Fatalf("a body naming a ROSTER-configured withheld brief id must refuse; got: %v", err)
		}
		if !strings.Contains(err.Error(), "closed-stream/07") {
			t.Errorf("the refusal must name the span the author has to edit; got: %v", err)
		}
		if strings.Contains(out.String(), "NOT CHECKED") {
			t.Errorf("a configured category must not also report itself unchecked; notices were:\n%s",
				out.String())
		}
	})

	// (b) ENV OVERRIDES ROSTER, the precedence every other key uses. The env value is the
	// one in force: the roster's own slug no longer refuses, the env's does.
	t.Run("environment overrides roster", func(t *testing.T) {
		scRosterWithheld(t, "closed-stream")
		t.Setenv(EnvWithheldIdentifiers, "Env-Stream")

		if got := WithheldIdentifiers(); len(got) != 1 || got[0] != "env-stream" {
			t.Fatalf("the environment must override the roster (lowercased); got %q", got)
		}
		if err := SelfContainCheck("PR body", []byte("delivered by env-stream/02"),
			SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: io_Discard{}}); !IsRefused(err) {
			t.Fatalf("the overriding env value must be the set in force: %v", err)
		}
		if err := SelfContainCheck("PR body", []byte("delivered by closed-stream/02"),
			SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: io_Discard{}}); err != nil {
			t.Fatalf("the overridden roster value must NOT still refuse: %v", err)
		}
	})

	// (c) UNSET IN BOTH is nil, and stays a complete adopter configuration: a NOTICE naming
	// BOTH sources, never a refusal.
	t.Run("unset in both is nil and notices both sources", func(t *testing.T) {
		scRoster(t) // no EnvWithheldIdentifiers in the roster
		t.Setenv(EnvWithheldIdentifiers, "")

		if got := WithheldIdentifiers(); got != nil {
			t.Fatalf("unset in both sources must be nil; got %q", got)
		}
		var out strings.Builder
		if err := SelfContainCheck("PR body", []byte("delivered by closed-stream/07"),
			SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: &out}); err != nil {
			t.Fatalf("an unconfigured register category must NOTICE, never refuse: %v", err)
		}
		notice := out.String()
		if !strings.Contains(notice, "NOT CHECKED") ||
			!strings.Contains(notice, "not configured in roster.env or the environment") {
			t.Errorf("the notice must name BOTH sources it looked in; notices were:\n%s", notice)
		}
	})
}
