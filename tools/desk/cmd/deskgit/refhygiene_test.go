package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #226 item 1 — `refs/`-prefixed values and the literal `HEAD` reached git as
// --branch destinations. `branchRe` admits them and `branchRejectReason` rejected neither,
// so both passed validation and landed in the constructed refspec:
//
//	--branch refs/heads/x  ->  created the NESTED ref refs/heads/refs/heads/x
//	--branch HEAD          ->  created refs/heads/HEAD, so git emits "ambiguous refname"
//	                           on every later resolution
//
// Neither escapes the ref namespace and neither rewrites an existing ref (verified:
// byte-identical before/after, and resolution still returns the true branch), so
// this is hygiene, not a security defect. Refusing them removes the ambiguity-warning
// class outright.
func TestFetch_BranchRejectsRefsPrefixAndHEAD(t *testing.T) {
	for _, bad := range []string{
		"refs/heads/main-ish",
		"refs/tags/v1",
		"REFS/heads/x", // case variant — same file on a case-insensitive FS
		"HEAD",
		"head", // ditto: refs/heads/head and refs/heads/HEAD collide on darwin/APFS
	} {
		t.Run(bad, func(t *testing.T) {
			work := newRepo(t, allowedSlug)
			calls := withEnv(t, work)
			if code := run([]string{"fetch", "--branch", bad}); code != deskkit.ExitRefused {
				t.Errorf("--branch %q exit = %d, want %d (refused by deskgit before delegation)",
					bad, code, deskkit.ExitRefused)
			}
			if fetchArgv(*calls) != nil {
				t.Errorf("git fetch must not run for --branch %q — the ambiguous/nested ref "+
					"would already have been created", bad)
			}
		})
	}

	// It must not over-refuse. These are ordinary destinations that merely resemble the
	// rejected shapes, and the guard is worthless if it eats them.
	for _, ok := range []string{
		"release/v1.2.3",
		"refsimple",         // starts with "refs" but is not the "refs/" prefix
		"fix/refs/handling", // "refs/" appears, but not at the start
		"HEADLESS",
		"my-HEAD",
	} {
		if why := branchRejectReason(ok); why != "" {
			t.Errorf("branchRejectReason(%q) = %q, want ok — the #226 guard is over-refusing", ok, why)
		}
	}
}

// #226 item 2 — the case-collision refusal is deliberately ordered BEFORE the
// origin-URL parse (that ordering is what keeps `git fetch` off a colliding destination),
// so its audit line had no parsed slug and emitted an EMPTY `repo` field. Nothing was
// untraceable — result/detail/argsDigest still identify the attempt — but a ledger consumer
// filtering or grouping by repo could not see these refusals under the repository they were
// aimed at.
//
// The refusal itself is unchanged: it is decided first, and the lookup only LABELS the line.
func TestFetch_PreParseRefusalRecordsTheRepo(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	mustGit(t, work, "branch", "feature-x")

	if code := run([]string{"fetch", "--branch", "Feature-X"}); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("the refusal must still precede any fetch — the ordering #226 relies on is intact")
	}

	entries, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatalf("load audit: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no audit entry for the refusal (exactly one line per invocation)")
	}
	e := entries[len(entries)-1]
	if e.Result != deskkit.ResultRefused {
		t.Fatalf("audit result = %q, want %q", e.Result, deskkit.ResultRefused)
	}
	if e.Repo != allowedSlug {
		t.Errorf("audit repo = %q, want %q — a pre-parse refusal is invisible to a ledger "+
			"consumer grouping by repo while this field is empty (#226 item 2)", e.Repo, allowedSlug)
	}
	if !strings.Contains(e.Detail, "differs only by case") {
		t.Errorf("audit detail lost the refusal reason: %q", e.Detail)
	}
}
