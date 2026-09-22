package deskkit

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// privateFixtureRepo is the KNOWN-PRIVATE repo every pre-existing principal test now
// names as its target. Those tests assert the login form, which is the private-target
// form — naming the repo explicitly is what keeps them asserting that, rather than
// silently becoming public-target tests the day a fixture changes.
const privateFixtureRepo = "example-org/one"

// publicTargetFixtureRoster is a roster whose blessing authority's LOGIN differs from the
// neutral name the human-login map carries for it, and which configures one PUBLIC and
// one PRIVATE repo. Both halves are what make the split observable at all: with name ==
// login (the other fixtures' shape) a leaked login and a correct substitution render
// identically, so a test written against those fixtures could not tell them apart.
const publicTargetFixtureRoster = `ASSAY_BLESS_LOGIN=a-private-handle:2001
ASSAY_TRUSTED_LOGINS=a-private-handle:2001
ASSAY_HUMAN_LOGIN_MAP=ada:a-private-handle
ASSAY_TRUSTED_BOT_SLUGS=worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/pub:ci:public,example-org/one:ci:private
`

const (
	publicFixtureRepo = "example-org/pub"
	fixtureLogin      = "a-private-handle"
	fixtureName       = "ada"
)

// TestPrincipalPublicTargetNamesNeutralNameNotLogin is the defect this split fixes: on a
// `:public` target the trailer named the roster's mapped LOGIN — a world-readable stamp on
// every review, comment, PR body and issue filed there, and one that cannot be edited off a
// posted review afterwards. It must name the neutral form the human-login map already
// carries instead, and must never contain the login in any part of the body.
func TestPrincipalPublicTargetNamesNeutralNameNotLogin(t *testing.T) {
	plantRoster(t, publicTargetFixtureRoster)
	t.Setenv("DESK_SESSION", "no-such-session-"+t.Name())

	out, err := AppendOnBehalfOf([]byte("The comment body."), "", publicFixtureRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf(%q) = %v, want a resolved principal", publicFixtureRepo, err)
	}
	got := string(out)
	if strings.Contains(got, fixtureLogin) {
		t.Fatalf("AppendOnBehalfOf(%q) = %q — the trailer carries the roster login; want the neutral "+
			"name %q the human-login map carries as the public form", publicFixtureRepo, got, fixtureName)
	}
	if want := "On-behalf-of: human:" + fixtureName + " mode:unattended\n"; !strings.HasSuffix(got, want) {
		t.Fatalf("AppendOnBehalfOf(%q) = %q, want it to end with %q", publicFixtureRepo, got, want)
	}
}

// TestPrincipalPrivateTargetUnchanged is the regression guard on the other half: a
// known-private target keeps today's behaviour exactly — the login, not the neutral name.
// The split must not quietly re-anonymise the private audit trail while fixing the public
// one.
func TestPrincipalPrivateTargetUnchanged(t *testing.T) {
	plantRoster(t, publicTargetFixtureRoster)
	t.Setenv("DESK_SESSION", "no-such-session-"+t.Name())

	out, err := AppendOnBehalfOf([]byte("The comment body."), "", privateFixtureRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf(%q) = %v, want a resolved principal", privateFixtureRepo, err)
	}
	if want := "On-behalf-of: human:" + fixtureLogin + " mode:unattended\n"; !strings.HasSuffix(string(out), want) {
		t.Fatalf("AppendOnBehalfOf(%q) = %q, want it to end with %q (the private-target form is unchanged)",
			privateFixtureRepo, string(out), want)
	}
}

// TestPrincipalVisibilityFailsClosedToTheNeutralForm: every target that is not stated
// private takes the neutral form — an unconfigured repo, one admitted only by an
// `owner/*` pattern (patterns carry no policy), and an empty repo argument. This is the
// direction the split must fail in: a wrong "public" costs audit precision, a wrong
// "private" is a disclosure that cannot be withdrawn.
func TestPrincipalVisibilityFailsClosedToTheNeutralForm(t *testing.T) {
	plantRoster(t, `ASSAY_BLESS_LOGIN=a-private-handle:2001
ASSAY_TRUSTED_LOGINS=a-private-handle:2001
ASSAY_HUMAN_LOGIN_MAP=ada:a-private-handle
ASSAY_TRUSTED_BOT_SLUGS=worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/pub:ci:public,example-org/one:ci:private,pattern-org/*
`)
	t.Setenv("DESK_SESSION", "no-such-session-"+t.Name())
	// Precondition: the pattern entry really did admit the repo, so the assertion below
	// is about a repo the roster ADMITS but states no visibility for — not about an
	// unconfigured one, which the next case already covers.
	if !IsAllowedRepo("pattern-org/admitted-by-pattern") {
		t.Fatalf("fixture: the owner/* pattern entry did not admit the repo; this case is not testing what it claims")
	}

	for _, repo := range []string{"", "some-org/never-configured", "pattern-org/admitted-by-pattern"} {
		p, err := ResolvePrincipal("", repo)
		if err != nil {
			t.Fatalf("ResolvePrincipal(repo=%q) = %v, want a resolved principal", repo, err)
		}
		if !p.Public {
			t.Fatalf("ResolvePrincipal(repo=%q).Public = false, want true (only a repo the roster "+
				"states is private may take the login form)", repo)
		}
		if p.Subject() != fixtureName {
			t.Fatalf("ResolvePrincipal(repo=%q).Subject() = %q, want %q", repo, p.Subject(), fixtureName)
		}
		if strings.Contains(p.Line(), fixtureLogin) {
			t.Fatalf("ResolvePrincipal(repo=%q).Line() = %q, want no login in it", repo, p.Line())
		}
	}
}

// TestPrincipalPublicTargetRefusesWithNoNeutralName: when the roster states no neutral
// name for the blessing authority, a public-target write REFUSES (exit 5). The two
// alternatives are both wrong — stamp the login (the disclosure this exists to prevent) or
// write with no trailer at all (retiring the presence guarantee downstream attribution is
// built on) — so no write happens until the roster carries the name. The refusal itself
// must not quote the login either.
func TestPrincipalPublicTargetRefusesWithNoNeutralName(t *testing.T) {
	plantRoster(t, `ASSAY_BLESS_LOGIN=a-private-handle:2001
ASSAY_TRUSTED_LOGINS=a-private-handle:2001
ASSAY_TRUSTED_BOT_SLUGS=worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/pub:ci:public,example-org/one:ci:private
`)
	t.Setenv("DESK_SESSION", "no-such-session-"+t.Name())

	_, err := AppendOnBehalfOf([]byte("The comment body."), "", publicFixtureRepo)
	if err == nil {
		t.Fatalf("AppendOnBehalfOf(%q) with no neutral name configured = nil error, want a refusal",
			publicFixtureRepo)
	}
	var derr *DeskError
	if !errors.As(err, &derr) {
		t.Fatalf("error = %v (%T), want a *DeskError", err, err)
	}
	if derr.Code != ExitRefused {
		t.Fatalf("error code = %d, want ExitRefused (%d)", derr.Code, ExitRefused)
	}
	if strings.Contains(err.Error(), fixtureLogin) {
		t.Fatalf("refusal text = %q, want it not to quote the login it is refusing to disclose", err.Error())
	}

	// Perturb: the SAME roster with the private target still writes, so the refusal is the
	// visibility split's, not a roster-loading failure.
	if _, perr := AppendOnBehalfOf([]byte("The comment body."), "", privateFixtureRepo); perr != nil {
		t.Fatalf("AppendOnBehalfOf(%q) = %v, want the private target unaffected", privateFixtureRepo, perr)
	}
}

// TestOnBehalfOfLineAndCommitSuffixFollowTheSameSplit: the two other rendering entry
// points (deskflip's printed line, deskevidence's commit trailer — a commit message in a
// public repo's history is as world-readable as a comment body) resolve through the same
// path, so neither can drift from it.
func TestOnBehalfOfLineAndCommitSuffixFollowTheSameSplit(t *testing.T) {
	plantRoster(t, publicTargetFixtureRoster)
	t.Setenv("DESK_SESSION", "no-such-session-"+t.Name())

	line, err := OnBehalfOfLine("", publicFixtureRepo)
	if err != nil {
		t.Fatalf("OnBehalfOfLine(%q) = %v", publicFixtureRepo, err)
	}
	if strings.Contains(line, fixtureLogin) || !strings.Contains(line, "human:"+fixtureName) {
		t.Fatalf("OnBehalfOfLine(%q) = %q, want the neutral name", publicFixtureRepo, line)
	}
	suffix, err := OnBehalfOfCommitSuffix("", publicFixtureRepo)
	if err != nil {
		t.Fatalf("OnBehalfOfCommitSuffix(%q) = %v", publicFixtureRepo, err)
	}
	if strings.Contains(suffix, fixtureLogin) || !strings.Contains(suffix, "human:"+fixtureName) {
		t.Fatalf("OnBehalfOfCommitSuffix(%q) = %q, want the neutral name", publicFixtureRepo, suffix)
	}

	privLine, err := OnBehalfOfLine("", privateFixtureRepo)
	if err != nil {
		t.Fatalf("OnBehalfOfLine(%q) = %v", privateFixtureRepo, err)
	}
	if !strings.Contains(privLine, "human:"+fixtureLogin) {
		t.Fatalf("OnBehalfOfLine(%q) = %q, want the login form unchanged", privateFixtureRepo, privLine)
	}
}

// TestHumanNeutralNameIsDeterministic: a roster may legitimately map two names onto one
// login. The neutral form must not then depend on Go's map iteration order — the same
// roster must always yield the same public trailer.
func TestHumanNeutralNameIsDeterministic(t *testing.T) {
	plantRoster(t, `ASSAY_BLESS_LOGIN=a-private-handle:2001
ASSAY_TRUSTED_LOGINS=a-private-handle:2001
ASSAY_HUMAN_LOGIN_MAP=zed:a-private-handle,ada:a-private-handle
ASSAY_TRUSTED_BOT_SLUGS=worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/pub:ci:public
`)
	for i := 0; i < 50; i++ {
		got, ok := HumanNeutralName(fixtureLogin)
		if !ok || got != fixtureName {
			t.Fatalf("HumanNeutralName(login) = %q,%v on iteration %d, want %q,true (lexicographically "+
				"first of the aliases, every time)", got, ok, i, fixtureName)
		}
	}
	if _, ok := HumanNeutralName("nobody-maps-to-this"); ok {
		t.Fatalf("HumanNeutralName() answered ok for an unmapped login")
	}
}

// TestEveryWriteVerbNamesItsTarget is the one-code-path guard. The split only holds if
// every verb TELLS the resolver which repo it is writing to; a call site that passed an
// empty repo would silently take the neutral form everywhere, including on private
// targets, and the defect class (a verb resolving identity without reference to where the
// write lands) would be back. This reads the verbs' own source rather than trusting a
// reviewer to notice.
func TestEveryWriteVerbNamesItsTarget(t *testing.T) {
	// `deskkit.AppendOnBehalfOf(body, "", "")` / `OnBehalfOfLine("", "")` etc. — an empty
	// LAST argument, which is the repo.
	emptyRepoArg := regexp.MustCompile(`(AppendOnBehalfOf|OnBehalfOfLine|OnBehalfOfCommitSuffix|ResolvePrincipal)\([^)\n]*,\s*""\s*\)`)
	root := filepath.Join("..", "..", "cmd")
	seen := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "OnBehalfOf") && !strings.Contains(line, "ResolvePrincipal") {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			seen++
			if emptyRepoArg.MatchString(line) {
				t.Errorf("%s:%d passes an empty target repo to the on-behalf-of resolver: %s\n"+
					"every write verb must name the repo it is writing to, or the trailer cannot "+
					"choose the right form of the human for it", path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if seen == 0 {
		t.Fatalf("found no on-behalf-of call sites under %s — this guard is looking in the wrong place", root)
	}
}
