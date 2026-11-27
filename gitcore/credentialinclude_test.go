package gitcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// checkoutCredential is the credentials config actions/checkout writes into
// RUNNER_TEMP before it swaps the placeholder for the encoded credential: one
// http.<server origin>/.extraheader key, written by `git config --file`.
const checkoutCredential = "[http \"https://github.com/\"]\n\textraheader = AUTHORIZATION: basic ***\n"

// includeRefusal is the reason prefix every refused include keeps.
const includeRefusal = "native discovery config includes are not merged by the selected reader"

type checkoutFixture struct {
	f     *gittest.Fixture
	admin string // the git directory as Git realpaths it
	cred  string // the host credentials config file
}

// newCheckoutFixture builds a repository whose .git/config carries the four
// includeIf entries actions/checkout persists its push credential through: the
// host git dir, its linked worktrees, and the same two spellings for a container
// workspace. Only the host git dir entry names this repository.
func newCheckoutFixture(t *testing.T) checkoutFixture {
	t.Helper()
	f := gittest.NewFixture(t)
	admin, err := filepath.EvalSymlinks(filepath.Join(f.Dir, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	cred := filepath.Join(t.TempDir(), "git-credentials-0b0e9c1e-5f3a-4c9d-9a4e-2f6d7c8b9a01.config")
	if err := os.WriteFile(cred, []byte(checkoutCredential), 0o600); err != nil {
		t.Fatal(err)
	}
	container := "/github/runner_temp/" + filepath.Base(cred)
	fx := checkoutFixture{f: f, admin: admin, cred: cred}
	fx.config(t, "includeIf.gitdir:"+admin+".path", cred)
	fx.config(t, "includeIf.gitdir:"+admin+"/worktrees/*.path", cred)
	fx.config(t, "includeIf.gitdir:/github/workspace/.git.path", container)
	fx.config(t, "includeIf.gitdir:/github/workspace/.git/worktrees/*.path", container)
	return fx
}

func (fx checkoutFixture) config(t *testing.T, key, value string) {
	t.Helper()
	if _, err := fx.f.Git("config", "--add", key, value); err != nil {
		t.Fatal(err)
	}
}

func (fx checkoutFixture) reason(t *testing.T, dir string) string {
	t.Helper()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	why, err := r.NativeDiscoveryReason()
	if err != nil {
		t.Fatalf("include observation turned into an error: %v", err)
	}
	return why
}

// nativeHeader is the control that the fixture is the shape Git merges: native
// Git reads the included key through the includeIf entry.
func nativeHeader(t *testing.T, f *gittest.Fixture) {
	t.Helper()
	got, err := f.Git("config", "--get", "http.https://github.com/.extraheader")
	if err != nil || got != "AUTHORIZATION: basic ***" {
		t.Fatalf("native Git did not merge the checkout include: %q %v", got, err)
	}
}

// FAIL-FIRST: credentialinclude-mutations.json (cd gitcore && muhar -spec
// credentialinclude-mutations.json).
//
// regression: #7833 — the workflow's own checkout made every delivering commit
// unreadable, so each risk-bearing Verify row failed its fail-first base check.
func TestCheckoutCredentialIncludeIsRead(t *testing.T) {
	t.Run("checkout", func(t *testing.T) {
		fx := newCheckoutFixture(t)
		nativeHeader(t, fx.f)
		if why := fx.reason(t, fx.f.Dir); why != "" {
			t.Fatalf("checkout credential include refused: %s", why)
		}
	})
	t.Run("encoded-credential", func(t *testing.T) {
		fx := newCheckoutFixture(t)
		body := "[http \"https://github.com/\"]\n\textraheader = AUTHORIZATION: basic " + strings.Repeat("Qx9", 20) + "+/==\n"
		if err := os.WriteFile(fx.cred, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if why := fx.reason(t, fx.f.Dir); why != "" {
			t.Fatalf("checkout credential value refused: %s", why)
		}
	})
	t.Run("linked-worktree", func(t *testing.T) {
		fx := newCheckoutFixture(t)
		linked := filepath.Join(t.TempDir(), "linked")
		if _, err := fx.f.Git("worktree", "add", "-q", "--detach", linked, "HEAD"); err != nil {
			t.Fatal(err)
		}
		// Only the worktrees/* entry names the linked worktree's git dir.
		if why := fx.reason(t, linked); why != "" {
			t.Fatalf("linked worktree of a checkout refused: %s", why)
		}
	})
}

// Every include other than the checkout credential shape stays refused, each for
// its own reason, so a guard that stops refusing is seen even where a later
// guard would also refuse.
func TestCheckoutCredentialIncludeRefusals(t *testing.T) {
	const (
		otherConfig  = "an include target holds config other than http.<url>.extraheader"
		unreadable   = "an include target is missing or unreadable"
		notPlain     = "an include target is not a plain file"
		notAbsolute  = "an include path is not a clean absolute path"
		notCondition = "an include condition other than a literal gitdir path"
	)
	writeCred := func(body string) func(*testing.T, checkoutFixture) {
		return func(t *testing.T, fx checkoutFixture) {
			if err := os.WriteFile(fx.cred, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	appendCred := func(extra string) func(*testing.T, checkoutFixture) {
		return writeCred(checkoutCredential + extra)
	}
	// badEntry adds an entry under condition whose target carries a worktree key,
	// beside the fixture's valid entries: skipping it would accept the config.
	badEntry := func(condition func(checkoutFixture) string) func(*testing.T, checkoutFixture) {
		return func(t *testing.T, fx checkoutFixture) {
			bad := filepath.Join(t.TempDir(), "bad.config")
			if err := os.WriteFile(bad, []byte(checkoutCredential+"[core]\n\tworktree = /example.invalid\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			fx.config(t, "includeIf."+condition(fx)+".path", bad)
		}
	}
	header := "[http \"https://github.com/\"]\n"
	for _, c := range []struct {
		name  string
		setup func(*testing.T, checkoutFixture)
		want  string
		// bare replaces the fixture's includes with only the entries setup adds.
		bare bool
	}{
		{name: "nested-include", want: otherConfig, setup: appendCred("[include]\n\tpath = /example.invalid/next.config\n")},
		{name: "nested-includeif", want: otherConfig, setup: appendCred("[includeIf \"gitdir:/example.invalid/.git\"]\n\tpath = /example.invalid/next.config\n")},
		{name: "worktree-key", want: otherConfig, setup: appendCred("[core]\n\tworktree = /example.invalid/elsewhere\n")},
		{name: "extension-key", want: otherConfig, setup: appendCred("[core]\n\trepositoryformatversion = 1\n[extensions]\n\tpreciousObjects = true\n")},
		{name: "remote-key", want: otherConfig, setup: appendCred("[remote \"origin\"]\n\turl = https://example.invalid/foreign.git\n")},
		{name: "other-http-key", want: otherConfig, setup: appendCred("\tproxy = http://example.invalid:3128\n")},
		{name: "extraheader-other-section", want: otherConfig, setup: writeCred("[remote \"origin\"]\n\textraheader = AUTHORIZATION: basic ***\n")},
		{name: "header-smuggles-section", want: otherConfig, setup: writeCred("[http \"a\"][core]worktree = /example.invalid\\\"]\n\textraheader = AUTHORIZATION: basic ***\n")},
		{name: "sectionless-http", want: otherConfig, setup: writeCred("[http]\n\textraheader = AUTHORIZATION: basic ***\n")},
		{name: "key-before-header", want: otherConfig, setup: writeCred("\textraheader = AUTHORIZATION: basic ***\n" + checkoutCredential)},
		{name: "key-spelling", want: otherConfig, setup: writeCred(header + "\textraHeader = AUTHORIZATION: basic ***\n")},
		{name: "continued-value", want: otherConfig, setup: writeCred(header + "\textraheader = AUTHORIZATION: basic ***\\\n[core]\n\tworktree = /example.invalid\n")},
		{name: "quoted-value", want: otherConfig, setup: writeCred(header + "\textraheader = \"AUTHORIZATION: basic ***\"\n")},
		{name: "comment-in-value", want: otherConfig, setup: writeCred(header + "\textraheader = AUTHORIZATION: basic *** ; trailing\n")},
		{name: "carriage-return", want: otherConfig, setup: writeCred(strings.ReplaceAll(checkoutCredential, "\n", "\r\n"))},
		{name: "blank-line", want: otherConfig, setup: writeCred(header + "\n\textraheader = AUTHORIZATION: basic ***\n")},
		{name: "no-final-newline", want: otherConfig, setup: writeCred(strings.TrimSuffix(checkoutCredential, "\n"))},
		{name: "header-only", want: otherConfig, setup: writeCred(header)},
		{name: "empty-target", want: otherConfig, setup: writeCred("")},
		{name: "oversized-target", want: "an include target is larger than a credential file", setup: writeCred(header + strings.Repeat("\textraheader = AUTHORIZATION: basic ***\n", 200))},
		{name: "missing-target", want: unreadable, setup: func(t *testing.T, fx checkoutFixture) {
			if err := os.Remove(fx.cred); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unreadable-target", want: unreadable, setup: func(t *testing.T, fx checkoutFixture) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a mode-000 file")
			}
			if err := os.Chmod(fx.cred, 0); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "directory-target", want: notPlain, setup: func(t *testing.T, fx checkoutFixture) {
			if err := os.Remove(fx.cred); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(fx.cred, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlink-target", want: notPlain, setup: func(t *testing.T, fx checkoutFixture) {
			real := filepath.Join(t.TempDir(), "real.config")
			if err := os.WriteFile(real, []byte(checkoutCredential), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(fx.cred); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(real, fx.cred); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "swapped-after-lstat", want: "an include target changed while it was read", setup: func(t *testing.T, fx checkoutFixture) {
			other := filepath.Join(t.TempDir(), "other.config")
			if err := os.WriteFile(other, []byte(checkoutCredential), 0o600); err != nil {
				t.Fatal(err)
			}
			open := openIncludeTarget
			openIncludeTarget = func(string) (*os.File, error) { return open(other) }
			t.Cleanup(func() { openIncludeTarget = open })
		}},
		{name: "relative-path", want: notAbsolute, bare: true, setup: func(t *testing.T, fx checkoutFixture) {
			rel := filepath.Join(fx.admin, "credential.config")
			if err := os.WriteFile(rel, []byte(checkoutCredential), 0o600); err != nil {
				t.Fatal(err)
			}
			fx.config(t, "includeIf.gitdir:"+fx.admin+".path", "credential.config")
		}},
		{name: "unclean-path", want: notAbsolute, bare: true, setup: func(t *testing.T, fx checkoutFixture) {
			fx.config(t, "includeIf.gitdir:"+fx.admin+".path", filepath.Dir(fx.cred)+"/./"+filepath.Base(fx.cred))
		}},
		{name: "includeif-empty-path", want: notAbsolute, bare: true, setup: func(t *testing.T, fx checkoutFixture) {
			fx.config(t, "includeIf.gitdir:"+fx.admin+".path", "")
		}},
		{name: "nonmatching-relative-path", want: notAbsolute, setup: func(t *testing.T, fx checkoutFixture) {
			fx.config(t, "includeIf.gitdir:/example.invalid/.git.path", "relative.config")
		}},
		{name: "unconditional-include", want: "an unconditional include", setup: func(t *testing.T, fx checkoutFixture) {
			fx.config(t, "include.path", fx.cred)
		}},
		{name: "condition-elsewhere", want: "no include condition names this repository", bare: true, setup: func(t *testing.T, fx checkoutFixture) {
			fx.config(t, "includeIf.gitdir:/example.invalid/work/.git.path", fx.cred)
			fx.config(t, "includeIf.gitdir:/example.invalid/work/.git/worktrees/*.path", fx.cred)
		}},
		{name: "condition-case-folded", want: notCondition, setup: badEntry(func(fx checkoutFixture) string { return "gitdir/i:" + strings.ToUpper(fx.admin) })},
		{name: "condition-onbranch", want: notCondition, setup: badEntry(func(checkoutFixture) string { return "onbranch:main" })},
		{name: "condition-hasconfig", want: notCondition, setup: badEntry(func(checkoutFixture) string { return "hasconfig:remote.*.url:https://github.com/**" })},
		{name: "condition-prefix", want: notCondition, setup: badEntry(func(fx checkoutFixture) string { return "gitdir:" + filepath.Dir(fx.admin) + "/" })},
		{name: "condition-relative", want: notCondition, setup: badEntry(func(checkoutFixture) string { return "gitdir:.git" })},
		{name: "condition-home", want: notCondition, setup: badEntry(func(checkoutFixture) string { return "gitdir:~/" })},
		{name: "condition-absolute-glob", want: notCondition, setup: badEntry(func(checkoutFixture) string { return "gitdir:/**" })},
		{name: "condition-other-glob-suffix", want: notCondition, setup: badEntry(func(fx checkoutFixture) string { return "gitdir:" + filepath.Dir(fx.admin) + "/*" })},
		{name: "condition-case-variant", want: otherConfig, setup: badEntry(func(fx checkoutFixture) string { return "gitdir:" + strings.ToUpper(fx.admin) })},
		{name: "includeif-other-key", want: "an includeIf entry carries a key other than path", setup: func(t *testing.T, fx checkoutFixture) {
			fx.config(t, "includeIf.gitdir:"+fx.admin+".other", fx.cred)
		}},
		{name: "includeif-no-path", want: "an includeIf entry without a path", setup: func(t *testing.T, fx checkoutFixture) {
			config := filepath.Join(fx.f.Dir, ".git", "config")
			body, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(config, append(body, "[includeIf \"gitdir:/example.invalid/.git\"]\n"...), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "includeif-sectionless-key", want: "an includeIf key without a condition", setup: func(t *testing.T, fx checkoutFixture) {
			fx.config(t, "includeIf.path", fx.cred)
		}},
		{name: "second-matching-target", want: otherConfig, setup: badEntry(func(fx checkoutFixture) string { return "gitdir:" + fx.admin })},
	} {
		t.Run(c.name, func(t *testing.T) {
			fx := newCheckoutFixture(t)
			if c.bare {
				for _, cond := range []string{fx.admin, fx.admin + "/worktrees/*", "/github/workspace/.git", "/github/workspace/.git/worktrees/*"} {
					if _, err := fx.f.Git("config", "--remove-section", "includeIf.gitdir:"+cond); err != nil {
						t.Fatal(err)
					}
				}
			}
			c.setup(t, fx)
			want := includeRefusal + " (" + c.want
			if why := fx.reason(t, fx.f.Dir); !strings.HasPrefix(why, want) {
				t.Fatalf("reason=%q, want %q", why, want)
			}
		})
	}
}

// The refusal never quotes the included file: it can hold the credential.
func TestCheckoutCredentialIncludeReasonQuotesNothing(t *testing.T) {
	fx := newCheckoutFixture(t)
	const marker = "fixture-marker-value"
	if err := os.WriteFile(fx.cred, []byte(checkoutCredential+"[core]\n\tworktree = "+marker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	why := fx.reason(t, fx.f.Dir)
	if !strings.HasPrefix(why, includeRefusal) {
		t.Fatalf("include accepted: %q", why)
	}
	for _, leak := range []string{marker, "AUTHORIZATION", fx.cred, filepath.Base(fx.cred)} {
		if strings.Contains(why, leak) {
			t.Fatalf("refusal quotes %q: %s", leak, why)
		}
	}
}
