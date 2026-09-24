package deskkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// credhelperchain_test.go — the REAL transport probe, against real git, with no stubs.
//
// Every other ambient-identity test injects CredHelperMatchesApp, so none of them can say
// anything about how the production probe resolves git's config. These run
// credHelperMatchesAppProbe itself over a throwaway repository whose system/global config is
// isolated to the test (GIT_CONFIG_NOSYSTEM, GIT_CONFIG_GLOBAL), so the host's own git config
// cannot leak in and nothing leaves the temp directory. No helper is ever run and no remote is
// contacted: the probe only reads config.
//
// FAIL-FIRST: the pre-fix probe asked `git config --get-urlmatch credential.helper`, which
// returns only the LAST configured helper, and read the fetch URL rather than the push URL.
// Against it (run at the PR head before this fix) six cases read clean — "competing earlier
// helper", "scheme-less host helper", "authorization extraHeader", "ssh push URL", "push URL
// differs from fetch URL" and "embedded URL credential" — the false greens this suite pins red.

const (
	chainURL       = "https://github.com:443/example-org/example-repo.git"
	embeddedSecret = "example-placeholder-value"
)

type chainRepo struct {
	t      *testing.T
	dir    string
	global string
	home   string
	token  string
}

func newChainRepo(t *testing.T) *chainRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	tmp := t.TempDir()
	global := filepath.Join(tmp, "global.gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // the netrc read also consults it on Windows
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	for _, k := range []string{"GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_DIR", "GIT_WORK_TREE",
		"NETRC", "GIT_ASKPASS", "SSH_ASKPASS"} {
		unsetForTest(t, k)
	}
	dir := filepath.Join(tmp, "repo")
	r := &chainRepo{t: t, dir: dir, global: global, home: tmp, token: filepath.Join(tmp, "worker-token-123456")}
	r.git("init", "-q", dir)
	r.local("remote.origin.url", chainURL)
	return r
}

func unsetForTest(t *testing.T, k string) {
	t.Helper()
	if old, ok := os.LookupEnv(k); ok {
		t.Cleanup(func() { os.Setenv(k, old) })
	} else {
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	os.Unsetenv(k)
}

func (r *chainRepo) git(args ...string) {
	r.t.Helper()
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// local/global ADD a value (multi-valued keys keep their order, exactly as git reads them).
func (r *chainRepo) local(key, val string) { r.git("-C", r.dir, "config", "--add", key, val) }
func (r *chainRepo) glob(key, val string) {
	r.git("config", "--file", r.global, "--add", key, val)
}

// appHelper is the helper the desk installs (deskwt role-init) for this repo's token file.
func (r *chainRepo) appHelper() string { return AppTokenHelper("x-access-token", r.token) }

func (r *chainRepo) probe() (bool, string) {
	r.t.Helper()
	ok, detail, err := r.probeFull()
	if err != nil {
		r.t.Fatalf("probe error (could-not-check): %v", err)
	}
	return ok, detail
}

func (r *chainRepo) probeFull() (bool, string, error) {
	r.t.Helper()
	return credHelperMatchesAppProbe(Landing{Dir: r.dir, Remote: "origin"}, r.token)
}

func (r *chainRepo) writeHome(name, body string, mode os.FileMode) string {
	r.t.Helper()
	p := filepath.Join(r.home, name)
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		r.t.Fatal(err)
	}
	return p
}

func TestCredHelperChainRealGit(t *testing.T) {
	type tc struct {
		name  string
		setup func(r *chainRepo)
		want  bool
		// detailHas, when set, must appear in the detail (and secret must not).
		detailHas string
	}
	cases := []tc{
		{"competing earlier helper in global config is red", func(r *chainRepo) {
			r.glob("credential.helper", "store --file /tmp/elsewhere")
			r.local("credential.helper", r.appHelper())
		}, false, "helper 1"},
		{"reset then App helper alone is green despite a global helper", func(r *chainRepo) {
			r.glob("credential.helper", "store --file /tmp/elsewhere")
			r.local("credential.helper", "")
			r.local("credential.helper", r.appHelper())
		}, true, ""},
		{"per-URL reset then App helper is green", func(r *chainRepo) {
			r.glob("credential.helper", "store --file /tmp/elsewhere")
			r.local("credential.https://github.com:443.helper", "")
			r.local("credential.https://github.com:443.helper", r.appHelper())
		}, true, ""},
		{"competing per-URL helper AFTER the App helper is red", func(r *chainRepo) {
			r.local("credential.helper", r.appHelper())
			r.local("credential.https://github.com:443.helper", "cache")
		}, false, "helper 2"},
		{"helper scoped to another host does not apply", func(r *chainRepo) {
			r.glob("credential.https://gitlab.example.invalid.helper", "store --file /tmp/elsewhere")
			r.local("credential.helper", r.appHelper())
		}, true, ""},
		{"scheme-less host helper applies and is red", func(r *chainRepo) {
			r.glob("credential.github.com:443.helper", "store --file /tmp/elsewhere")
			r.local("credential.helper", r.appHelper())
		}, false, "helper 1"},
		{"authorization extraHeader is red", func(r *chainRepo) {
			r.glob("http.https://github.com:443/.extraheader", "AUTHORIZATION: "+"basic "+"example-placeholder")
			r.local("credential.helper", r.appHelper())
		}, false, "extraHeader"},
		{"no helper at all is red", func(r *chainRepo) {}, false, "no credential helper"},
		{"ssh push URL is red", func(r *chainRepo) {
			r.local("remote.origin.pushurl", "git@github.com:example-org/example-repo.git")
			r.local("credential.helper", r.appHelper())
		}, false, "not an http(s) URL"},
		{"push URL differs from fetch URL: the push URL is judged", func(r *chainRepo) {
			r.local("remote.origin.pushurl", "https://git.example.invalid/example-org/example-repo.git")
			r.local("credential.https://github.com:443.helper", r.appHelper())
		}, false, "git.example.invalid"},
		{"embedded URL credential is red and not echoed", func(r *chainRepo) {
			// Assembled at run time so no source line carries a URL-embedded credential.
			r.local("remote.origin.pushurl", "https://someone:"+embeddedSecret+"@github.com:443/example-org/example-repo.git")
			r.local("credential.helper", r.appHelper())
		}, false, "embeds a credential"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newChainRepo(t)
			c.setup(r)
			ok, detail := r.probe()
			if ok != c.want {
				t.Fatalf("probe = %v (%s), want %v", ok, detail, c.want)
			}
			if c.detailHas != "" && !strings.Contains(detail, c.detailHas) {
				t.Errorf("detail %q lacks %q", detail, c.detailHas)
			}
			if strings.Contains(detail, embeddedSecret) {
				t.Errorf("detail echoes an embedded credential: %q", detail)
			}
		})
	}
}

// foreignHelper is a helper that is NOT the desk's App token helper. Its shape never matters
// to the probe (no helper is ever run); it only has to be something other than AppTokenHelper.
const foreignHelper = "store --file /tmp/elsewhere"

// gitAppliesPattern asks the git binary on PATH itself whether a `credential.<pattern>.helper`
// entry applies to chainURL: it runs `git credential fill` with ONLY a reset plus a marker
// helper under that pattern (system/global config isolated by newChainRepo, prompts and
// askpass disabled), so the answer is git's own matcher on this git version. It is the
// ground truth the parity cases are pinned against; the probe under test never runs a helper.
func (r *chainRepo) gitAppliesPattern(pattern string) bool {
	r.t.Helper()
	const marker = "parity-marker"
	cmd := exec.Command("git", "-C", r.dir, "credential", "fill")
	cmd.Stdin = strings.NewReader("url=" + chainURL + "\n\n")
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=",
		"GIT_CONFIG_COUNT=3",
		"GIT_CONFIG_KEY_0=credential.helper", "GIT_CONFIG_VALUE_0=",
		"GIT_CONFIG_KEY_1=core.askPass", "GIT_CONFIG_VALUE_1=",
		"GIT_CONFIG_KEY_2=credential."+pattern+".helper",
		"GIT_CONFIG_VALUE_2=!f(){ echo username="+marker+"; echo password="+marker+"; }; f",
	)
	out, _ := cmd.CombinedOutput()
	return strings.Contains(string(out), "username="+marker)
}

// TestCredChainGitParity pins the credential-chain check to how git itself resolves the
// credential for a push, one group per parity item. FAIL-FIRST: every case marked red below
// read CLEAN against the check before this change (see the PR's fail-first section).
func TestCredChainGitParity(t *testing.T) {
	type tc struct {
		name      string
		setup     func(r *chainRepo)
		want      bool
		wantErr   bool   // could-not-check
		detailHas string // must appear in the detail (or the error)
	}
	cases := []tc{
		// 1. Host-less / partial-URL credential patterns apply the way git applies them.
		{"hostless pattern empty subsection is red", func(r *chainRepo) {
			r.glob("credential..helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, false, false, "helper 1"},
		{"hostless pattern slash is red", func(r *chainRepo) {
			r.glob("credential./.helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, false, false, "helper 1"},
		{"hostless pattern path-only is red", func(r *chainRepo) {
			r.glob("credential./example-org/example-repo.git.helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, false, false, "helper 1"},
		{"hostless pattern scheme-only is red", func(r *chainRepo) {
			r.glob("credential.https://.helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, false, false, "helper 1"},
		{"hostless pattern scheme and slash is red", func(r *chainRepo) {
			r.glob("credential.https:///.helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, false, false, "helper 1"},
		{"hostless pattern for another scheme does not apply", func(r *chainRepo) {
			r.glob("credential.http://.helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, true, false, ""},
		{"URL pattern scoped to another path does not apply", func(r *chainRepo) {
			r.glob("credential.https://github.com:443/other-org.helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, true, false, ""},
		{"wildcard URL pattern for another domain does not apply", func(r *chainRepo) {
			r.glob("credential.https://*.example.invalid.helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, true, false, ""},
		{"wildcard URL pattern for the host applies and is red", func(r *chainRepo) {
			r.glob("credential.https://*.com.helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
		}, false, false, "helper 1"},
		{"push URL with an empty host is could-not-check", func(r *chainRepo) {
			r.local("remote.origin.pushurl", "https:///example-org/example-repo.git")
			r.local("credential.helper", r.appHelper())
		}, false, true, "host"},

		// 2. Every push URL git pushes to is judged, not only the first.
		{"second pushurl with a foreign helper is red", func(r *chainRepo) {
			r.local("remote.origin.pushurl", chainURL)
			r.local("remote.origin.pushurl", "https://git.example.invalid/example-org/example-repo.git")
			r.local("credential.https://github.com:443.helper", r.appHelper())
			r.local("credential.https://git.example.invalid.helper", foreignHelper)
		}, false, false, "git.example.invalid"},
		{"second pushurl over ssh is red", func(r *chainRepo) {
			r.local("remote.origin.pushurl", chainURL)
			r.local("remote.origin.pushurl", "git@github.com:example-org/example-repo.git")
			r.local("credential.helper", r.appHelper())
		}, false, false, "not an http(s) URL"},
		{"two pushurls both served by the App helper are green", func(r *chainRepo) {
			r.local("remote.origin.pushurl", chainURL)
			r.local("remote.origin.pushurl", "https://github.com:443/example-org/example-mirror.git")
			r.local("credential.helper", r.appHelper())
		}, true, false, "2 push URL"},

		// 3. A netrc entry the transport consults before any helper.
		{"netrc machine entry for the host is red", func(r *chainRepo) {
			r.writeHome(".netrc", "machine github.com login other\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		{"netrc default entry is red", func(r *chainRepo) {
			r.writeHome(".netrc", "machine other.example.invalid login a\ndefault login other\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		// curl compares every netrc keyword case-insensitively (SR-1614-2).
		{"netrc MACHINE keyword in upper case is red", func(r *chainRepo) {
			r.writeHome(".netrc", "MACHINE github.com login other\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		{"netrc Machine keyword in mixed case is red", func(r *chainRepo) {
			r.writeHome(".netrc", "Machine github.com login other\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		{"netrc DEFAULT keyword in upper case is red", func(r *chainRepo) {
			r.writeHome(".netrc", "DEFAULT login other\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		{"netrc MACHINE with the host on the next line is red", func(r *chainRepo) {
			r.writeHome(".netrc", "MACHINE\ngithub.com\nlogin other\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		{"netrc MACDEF body in upper case does not count", func(r *chainRepo) {
			r.writeHome(".netrc", "MACDEF init\nmachine github.com login other\n\nmachine other.example.invalid login a\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, true, false, ""},
		// Outside a matched entry curl does not treat login/password/account as keywords, so
		// the token after one is read as a keyword itself: `default` there opens a default
		// entry and `machine <host>` a machine entry.
		{"netrc default after a login keyword of a non-matching entry is red", func(r *chainRepo) {
			r.writeHome(".netrc", "machine other.example.invalid login default\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		{"netrc machine after an account keyword of a non-matching entry is red", func(r *chainRepo) {
			r.writeHome(".netrc", "machine other.example.invalid account machine github.com\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		// Over-inclusive, not parity: the git+libcurl pairs checked did not read $NETRC. Reading
		// it can only redden the check, so this case pins that direction; do not "fix" it away.
		{"netrc named by NETRC is red", func(r *chainRepo) {
			p := r.writeHome("alt-netrc", "machine GitHub.com login other\n", 0o600)
			r.t.Setenv("NETRC", p)
			r.local("credential.helper", r.appHelper())
		}, false, false, "netrc"},
		{"netrc for another host only is green", func(r *chainRepo) {
			r.writeHome(".netrc", "machine other.example.invalid login a\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, true, false, ""},
		{"netrc machine inside a macdef body does not count", func(r *chainRepo) {
			r.writeHome(".netrc", "macdef init\nmachine github.com\n\nmachine other.example.invalid login a\n", 0o600)
			r.local("credential.helper", r.appHelper())
		}, true, false, ""},
		{"unreadable netrc is could-not-check", func(r *chainRepo) {
			if err := os.Mkdir(filepath.Join(r.home, ".netrc"), 0o700); err != nil {
				r.t.Fatal(err)
			}
			r.local("credential.helper", r.appHelper())
		}, false, true, "netrc"},

		// 4. Only the App token helper itself passes, and the detail names what git runs.
		{"same token file name in another directory is red", func(r *chainRepo) {
			r.local("credential.helper", AppTokenHelper("x-access-token",
				filepath.Join(r.home, "otherhome", filepath.Base(r.token))))
		}, false, false, "otherhome"},
		{"helper that only mentions the token path is red", func(r *chainRepo) {
			r.local("credential.helper", "!f(){ : '"+r.token+"'; cat /tmp/elsewhere; }; f")
		}, false, false, "shell"},
		{"store helper over the token file with askpass fallthrough is red", func(r *chainRepo) {
			r.local("credential.helper", "store --file "+r.token)
			r.local("core.askPass", "/tmp/elsewhere-askpass")
		}, false, false, "git credential-store"},
		{"App helper then a trailing appended command is red", func(r *chainRepo) {
			r.local("credential.helper", r.appHelper()+"; cat /tmp/elsewhere")
		}, false, false, "helper 1"},
		// git does not trim a helper value: a leading blank makes it `git credential- !f(){…`,
		// which answers nothing, so git falls through to askpass (SR-1614-1).
		{"App helper with a leading space is red", func(r *chainRepo) {
			r.local("credential.helper", " "+r.appHelper())
			r.local("core.askPass", "/tmp/elsewhere-askpass")
		}, false, false, "starts with whitespace"},
		{"App helper with a leading tab is red", func(r *chainRepo) {
			r.local("credential.helper", "\t"+r.appHelper())
			r.local("core.askPass", "/tmp/elsewhere-askpass")
		}, false, false, "starts with whitespace"},
		{"App helper alone is green even with an askpass configured", func(r *chainRepo) {
			// The App helper always answers with a username and a password (an empty one when
			// the file is unreadable), so git completes the credential before any askpass.
			r.local("credential.helper", r.appHelper())
			r.local("core.askPass", "/tmp/elsewhere-askpass")
		}, true, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newChainRepo(t)
			c.setup(r)
			ok, detail, err := r.probeFull()
			if c.wantErr {
				if err == nil {
					t.Fatalf("probe = %v (%s), want could-not-check", ok, detail)
				}
				if c.detailHas != "" && !strings.Contains(err.Error(), c.detailHas) {
					t.Errorf("error %q lacks %q", err, c.detailHas)
				}
				return
			}
			if err != nil {
				t.Fatalf("probe error (could-not-check): %v", err)
			}
			if ok != c.want {
				t.Fatalf("probe = %v (%s), want %v", ok, detail, c.want)
			}
			if c.detailHas != "" && !strings.Contains(detail, c.detailHas) {
				t.Errorf("detail %q lacks %q", detail, c.detailHas)
			}
		})
	}
}

// TestCredChainHostlessMatchesGit pins the host-less fixtures above to the git on PATH: each
// pattern git applies to the push URL, the check must treat as applying (red), and the one
// git does not apply, the check must not.
func TestCredChainHostlessMatchesGit(t *testing.T) {
	for pattern, gitWants := range map[string]bool{
		"":                              true,
		"/":                             true,
		"/example-org/example-repo.git": true,
		"https://":                      true,
		"https:///":                     true,
		"github.com:443":                true,
		"http://":                       false,
		"gitlab.example.invalid":        false,
	} {
		t.Run("pattern="+pattern, func(t *testing.T) {
			r := newChainRepo(t)
			if got := r.gitAppliesPattern(pattern); got != gitWants {
				t.Fatalf("fixture drift: git applies %q = %v, the fixture expects %v", pattern, got, gitWants)
			}
			r.glob("credential."+pattern+".helper", foreignHelper)
			r.local("credential.helper", r.appHelper())
			ok, detail := r.probe()
			if ok == gitWants {
				t.Fatalf("git applies %q = %v, but the check reads green=%v (%s)", pattern, gitWants, ok, detail)
			}
		})
	}
}

// gitFillPassword runs `git credential fill` for chainURL in r's repository with an askpass
// program that answers a marker, and returns the password git settled on. It is the ground
// truth for which credential git presents: the App token's contents, or the askpass marker
// when the helper chain yielded nothing. Nothing is contacted; fill only runs helpers.
func (r *chainRepo) gitFillPassword(askpass string) string {
	r.t.Helper()
	cmd := exec.Command("git", "-C", r.dir, "credential", "fill")
	cmd.Stdin = strings.NewReader("url=" + chainURL + "\n\n")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS="+askpass, "SSH_ASKPASS=")
	out, _ := cmd.CombinedOutput()
	for _, l := range strings.Split(string(out), "\n") {
		if v, ok := strings.CutPrefix(l, "password="); ok {
			return v
		}
	}
	return ""
}

// TestCredChainHelperWhitespaceMatchesGit pins SR-1614-1 to the git on PATH: git keeps a
// helper value's leading blank, so " "+AppTokenHelper(...) runs as `git credential- !f(){…`,
// answers nothing, and git takes the askpass credential instead. Wherever git does NOT
// present the App token, the check must read red; the unprefixed helper is the green control.
func TestCredChainHelperWhitespaceMatchesGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the askpass fixture is a POSIX shell script")
	}
	const tokenMarker, askpassMarker = "parity-app-token", "parity-askpass"
	for name, prefix := range map[string]string{"none": "", "space": " ", "tab": "\t"} {
		t.Run("leading="+name, func(t *testing.T) {
			r := newChainRepo(t)
			if err := os.WriteFile(r.token, []byte(tokenMarker), 0o600); err != nil {
				t.Fatal(err)
			}
			askpass := r.writeHome("askpass.sh", "#!/bin/sh\necho "+askpassMarker+"\n", 0o700)
			r.local("credential.helper", "")
			r.local("credential.helper", prefix+r.appHelper())
			got := r.gitFillPassword(askpass)
			if prefix != "" && got != askpassMarker {
				t.Fatalf("fixture drift: git presented %q for a %s-prefixed App helper, want the askpass marker", got, name)
			}
			gitPresentsToken := got == tokenMarker
			ok, detail := r.probe()
			if ok != gitPresentsToken {
				t.Fatalf("git presents the App token = %v (password %q), but the check reads green=%v (%s)",
					gitPresentsToken, got, ok, detail)
			}
		})
	}
}

// netrcLexCase is one netrc body (HOST stands for the push host) and what the check must read
// for it. The same table drives the unit parity test below, over the full probe, and the
// ground-truth test after it, over real git and libcurl.
type netrcLexCase struct {
	name string
	body string
	want string // "red", "green", or "cnc" (could-not-check)
}

// netrcLexCases pin the netrc lexer to curl's (lib/netrc.c). Each red row is one that curl
// answers from netrc and that a looser lexer read as no entry (SR-1614-2 round 2, SR-1614-3):
// a keyword swallowed by stale state fails open. Each green row is one curl answers nothing for
// (observed against git 2.55.0 with libcurl 8.7.1 on loopback); the check matches it rather than
// reddening, since the parity target is curl's own reading.
var netrcLexCases = []netrcLexCase{
	// '#' at the start of a token ends the line (SR-1614-3 a; SR-1614-2 rows A, B, C).
	{"comment ending in machine above the entry is red", "# work machine\nmachine HOST login op\n", "red"},
	{"comment ending in macdef above the entry is red", "# macdef note\nmachine HOST login op\n", "red"},
	{"comment naming no macdef above the entry is red", "# no macdef here\nmachine HOST login op\n", "red"},
	{"indented comment ending in machine is red", "   # indented machine\nmachine HOST login op\n", "red"},
	{"mid-line comment ending in machine is red",
		"machine other.example.invalid login x password y # machine\nmachine HOST login op\n", "red"},
	{"comment glued to its first word is red", "#machine\nmachine HOST login op\n", "red"},
	{"machine then a comment then the host on the next line is red", "machine #c\nHOST login op\n", "red"},
	// A macdef body ends only on a line whose first byte is a newline or carriage return
	// (SR-1614-3 b; SR-1614-2 row D), and an empty token on the macdef line itself ends it.
	{"macdef body with a spaces-only line is skipped whole", "macdef m\nx\n  \nfoo machine\n\nmachine HOST login op\n", "red"},
	{"macdef line ending in a blank ends the macro at once", "macdef m \nmachine HOST login op\n", "red"},
	{"macdef line ending in a tab ends the macro at once", "macdef m\t\nmachine HOST login op\n", "red"},
	{"line starting with a carriage return ends a macdef body", "macdef m\nx\n\rjunk\nmachine HOST login op\n", "red"},
	{"CRLF empty line ends a macdef body", "macdef m\r\nx\r\n\r\nmachine HOST login op\r\n", "red"},
	// An unquoted token ends at any whitespace byte, not only space and tab (SR-1614-2 rows E, N).
	{"vertical tab separates tokens", "machine\vHOST login op\n", "red"},
	{"form feed separates tokens", "machine\fHOST login op\n", "red"},
	{"carriage return separates tokens", "machine other.example.invalid login a\n\rmachine HOST login op\n", "red"},
	// Quoted tokens take the escapes \n \r \t, and the byte after a closing quote is skipped.
	{"quoted token with an escaped newline is not a keyword", "\"machi\\ne\" machine HOST login op\n", "red"},
	{"quoted token with an escaped letter is that letter", "\"mach\\ine\" HOST login op\n", "red"},
	{"byte after a closing quote is skipped", "\"a\"xmachine HOST login op\n", "red"},
	{"quoted host is red", "machine \"HOST\" login op\n", "red"},
	{"entry on a last line with no newline is red", "machine HOST login op", "red"},

	// Green: curl answers nothing for these, and neither does the check.
	{"spaces-only line inside a macdef does not end it", "macdef m\n  \nmachine HOST login op\n", "green"},
	{"form-feed line inside a macdef does not end it", "macdef m\n\f\nmachine HOST login op\n", "green"},
	{"comment after a non-matching entry hides its words", "machine other.example.invalid # note machine\nHOST login op\n", "green"},
	{"blank before the newline after machine is an empty host", "machine \nHOST login op\n", "green"},
	{"hash inside a token is not a comment", "machine other.example.invalid#x\nHOST login op\n", "green"},
	{"closing quote eats the next byte of a keyword", "\"a\"machine HOST login op\n", "green"},
	{"machine default names a host, not a default entry", "machine default login op\n", "green"},

	// curl refuses the whole file; the check does not guess.
	{"unterminated quote is could-not-check", "\"abc\nmachine HOST login op\n", "cnc"},
	{"NUL byte is could-not-check", "machine\x00x\nmachine HOST login op\n", "cnc"},
}

// TestCredChainNetrcLexer runs every netrcLexCases body through the full probe, with the App
// helper configured, so a netrc entry is the only thing that can redden it.
func TestCredChainNetrcLexer(t *testing.T) {
	for _, c := range netrcLexCases {
		t.Run(c.name, func(t *testing.T) {
			r := newChainRepo(t)
			r.writeHome(".netrc", strings.ReplaceAll(c.body, "HOST", "github.com"), 0o600)
			r.local("credential.helper", r.appHelper())
			ok, detail, err := r.probeFull()
			switch c.want {
			case "cnc":
				if err == nil {
					t.Fatalf("probe = %v (%s), want could-not-check", ok, detail)
				}
				if !strings.Contains(err.Error(), "netrc") {
					t.Errorf("error %q lacks %q", err, "netrc")
				}
			case "red":
				if err != nil {
					t.Fatalf("probe error (could-not-check): %v, want red", err)
				}
				if ok || !strings.Contains(detail, "netrc") {
					t.Fatalf("probe = %v (%s), want red naming netrc", ok, detail)
				}
			case "green":
				if err != nil {
					t.Fatalf("probe error (could-not-check): %v, want green", err)
				}
				if !ok {
					t.Fatalf("probe = %v (%s), want green", ok, detail)
				}
			}
		})
	}
}

// TestCredChainNetrcMatchesCurl pins netrcLexCases to the git and libcurl on PATH. For each
// body it has git ask a loopback-only server (127.0.0.1, which answers every request 401) for
// refs, with that body as the only netrc and no helper, and records whether curl presented the
// netrc credential. Wherever curl presents it, the check must read red or could-not-check;
// never green. That is the fail-open direction this check exists to refuse, and it is asserted
// against whatever libcurl the host carries. A check that reads red where curl presents
// nothing is over-inclusive and only logged here, since libcurl builds may differ on a
// corner; the table's green rows pin the check's own reading of those.
func TestCredChainNetrcMatchesCurl(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("curl also reads _netrc and USERPROFILE on Windows; this harness sets HOME only")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	var mu sync.Mutex
	var presented []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if user, _, ok := req.BasicAuth(); ok {
			mu.Lock()
			presented = append(presented, user)
			mu.Unlock()
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="parity"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	host := "127.0.0.1"
	if !strings.Contains(srv.URL, "://"+host+":") {
		t.Skipf("the loopback server is not on %s: %s", host, srv.URL)
	}
	tmp := t.TempDir()
	askpass := filepath.Join(tmp, "askpass.sh")
	if err := os.WriteFile(askpass, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(tmp, "global.gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// curlPresents reports whether git, through libcurl, sent the netrc login for body.
	curlPresents := func(t *testing.T, body string) bool {
		t.Helper()
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, ".netrc"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		presented = nil
		mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, gitPath, "ls-remote", srv.URL+"/example-org/example-repo.git")
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "GIT_CONFIG_NOSYSTEM=1",
			"GIT_CONFIG_GLOBAL=" + global, "XDG_CONFIG_HOME=" + filepath.Join(home, "xdg"),
			"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=" + askpass, "SSH_ASKPASS=", "NO_PROXY=*"}
		_, _ = cmd.CombinedOutput() // the server always answers 401; only what curl sent matters
		mu.Lock()
		defer mu.Unlock()
		for _, u := range presented {
			if u == "op" {
				return true
			}
		}
		return false
	}
	if !curlPresents(t, "machine "+host+" login op password parity\n") {
		t.Skip("the git on PATH did not present a plain netrc machine entry; it cannot serve as ground truth")
	}
	for _, c := range netrcLexCases {
		t.Run(c.name, func(t *testing.T) {
			body := strings.ReplaceAll(c.body, "HOST", host)
			sent := curlPresents(t, body)
			t.Setenv("HOME", t.TempDir())
			unsetForTest(t, "NETRC")
			if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".netrc"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			entry, err := netrcEntryFor(host)
			switch {
			case sent && err == nil && entry == "":
				t.Fatalf("curl presented the netrc credential, but the check finds no entry (reads green)")
			case !sent && (err != nil || entry != ""):
				t.Logf("over-inclusive: curl presented nothing, the check reads entry=%q err=%v", entry, err)
			}
		})
	}
}
