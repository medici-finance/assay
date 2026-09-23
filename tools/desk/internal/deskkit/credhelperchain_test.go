package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	for _, k := range []string{"GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_DIR", "GIT_WORK_TREE"} {
		unsetForTest(t, k)
	}
	dir := filepath.Join(tmp, "repo")
	r := &chainRepo{t: t, dir: dir, global: global, token: filepath.Join(tmp, "worker-token-123456")}
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

func (r *chainRepo) appHelper() string {
	return "!f() { echo username=x-access-token; echo password=$(cat " + r.token + "); }; f"
}

func (r *chainRepo) probe() (bool, string) {
	r.t.Helper()
	ok, detail, err := credHelperMatchesAppProbe(Landing{Dir: r.dir, Remote: "origin"}, r.token)
	if err != nil {
		r.t.Fatalf("probe error (could-not-check): %v", err)
	}
	return ok, detail
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
