package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The open step's module screen, its announcement and the receipt's binding to
// the home's module roots (#4683). Fixtures only: the fake deskdispatch, a
// faked `go mod download` and the read-only mount check stubbed. No module
// proxy, checksum database, container engine or network is reached.

// plantRoot rewrites the fixture's tools/desk root to go.mod and go.sum and
// commits them.
func plantRoot(t *testing.T, fx containerFixture, gomod, gosum string) {
	t.Helper()
	putFile(t, filepath.Join(fx.home, "tools", "desk", "go.mod"), gomod)
	putFile(t, filepath.Join(fx.home, "tools", "desk", "go.sum"), gosum)
	runGit(t, fx.home, "add", ".")
	runGit(t, fx.home, "commit", "-m", "plant")
}

const plantSum = "golang.org/x/text v0.43.0 h1:fixture=\ngolang.org/x/text v0.43.0/go.mod h1:fixture=\n"

// regression: #4683
// TestModEgressScreened: before any request, every module path a root could
// send to the public proxy is screened. A path under the verified repository's
// owner (in any case), one off the approved public set, a replacement that
// names an unapproved module version, a module only go.sum names, and one the
// operator marked private each refuse as could-not-check with no download and
// no receipt; a require replaced by a directory sends nothing and passes.
func TestModEgressScreened(t *testing.T) {
	head := "module example.com/desk\n\ngo 1.27.0\n\n"
	for _, c := range []struct {
		name, gomod, gosum, private, refused string
	}{
		{"owner require", head + "require (\n\tgolang.org/x/text v0.43.0\n\tgithub.com/example-org/secret v1.0.0\n)\n",
			plantSum + "github.com/example-org/secret v1.0.0 h1:x=\n", "", "github.com/example-org/secret"},
		{"owner in another case", head + "require github.com/Example-Org/secret v1.0.0\n",
			"github.com/Example-Org/secret v1.0.0 h1:x=\n", "", "github.com/Example-Org/secret"},
		{"unapproved host", head + "require example.net/private/mod v1.0.0\n",
			"example.net/private/mod v1.0.0 h1:x=\n", "", "example.net/private/mod"},
		{"version replacement", head + "require golang.org/x/text v0.43.0\n\nreplace golang.org/x/text => example.net/fork v1.0.0\n",
			plantSum, "", "example.net/fork"},
		{"go.sum only", head + "require golang.org/x/text v0.43.0\n",
			plantSum + "github.com/example-org/hidden v1.0.0/go.mod h1:x=\n", "", "github.com/example-org/hidden"},
		{"operator private pattern", head + "require github.com/acme-corp/lib v1.0.0\n",
			"github.com/acme-corp/lib v1.0.0 h1:x=\n", "github.com/acme-corp/*", "github.com/acme-corp/lib"},
		{"directory replacement", head + "require (\n\tgolang.org/x/text v0.43.0\n\tgithub.com/example-org/two v0.0.0\n)\n\nreplace github.com/example-org/two => ../two\n",
			plantSum, "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			fx := newContainerFixture(t)
			t.Setenv("DESK_LOOP", "")
			t.Setenv("GOPRIVATE", c.private)
			t.Setenv("GONOPROXY", "")
			t.Setenv("GONOSUMDB", "")
			withModuleRoots(t, fx)
			plantRoot(t, fx, c.gomod, c.gosum)
			dl := stubDownload(t)
			stubAmbient(t)
			putFile(t, fx.check, validCheck)
			cache := filepath.Join(t.TempDir(), "modcache")
			res, out := captureVerifyrunOut(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt, "--module-cache", cache})
			if c.refused == "" {
				if res.code != verifyrunExitPass || len(dl.calls) != 1 {
					t.Fatalf("a directory-replaced require: exit %d, downloads %v: %s", res.code, dl.calls, out)
				}
				return
			}
			if res.code != verifyrunExitCouldNot || !strings.Contains(out, "could-not-check") || !strings.Contains(out, c.refused) || !strings.Contains(out, "nothing was sent to the proxy") {
				t.Fatalf("exit %d, want could-not-check naming %s with nothing sent:\n%s", res.code, c.refused, out)
			}
			if len(dl.calls) != 0 {
				t.Fatalf("the screen let a download run: %v", dl.calls)
			}
			if _, err := os.Stat(fx.receipt); err == nil {
				t.Fatal("a refused screen wrote a receipt")
			}
			if _, err := os.Stat(cache); err == nil {
				t.Fatal("a refused screen created the cache directory")
			}
		})
	}
}

// regression: #4683
// TestModEgressAnnouncedFirst: the open step names the proxy, the checksum
// database and the manifest's roots on stdout before its first download.
func TestModEgressAnnouncedFirst(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	withModuleRoots(t, fx)
	dl := stubDownload(t)
	stubAmbient(t)
	putFile(t, fx.check, validCheck)
	var stdout bytes.Buffer
	var seen []string
	inner := goModDownload
	goModDownload = func(dir string, env []string) ([]byte, error) {
		seen = append(seen, stdout.String())
		return inner(dir, env)
	}
	cache := filepath.Join(t.TempDir(), "modcache")
	if code := runOpenBracket(fx.home, fx.brief, fx.receipt, cache, &stdout, io.Discard); code != verifyrunExitPass {
		t.Fatalf("open: exit %d: %s", code, stdout.String())
	}
	if len(dl.calls) != 1 || len(seen) != 1 {
		t.Fatalf("downloads %v, want one", dl.calls)
	}
	for _, want := range []string{"proxy=https://proxy.golang.org", "sumdb=sum.golang.org", "roots=tools/desk"} {
		if !strings.Contains(seen[0], want) {
			t.Errorf("stdout before the first download lacks %q:\n%q", want, seen[0])
		}
	}
}

// regression: #4683
// TestModCacheReceiptRequired: the manifest's requirement binds at every stage
// that reads a receipt, not at the open step alone. On a home that lists module
// roots, a receipt with no module_cache (an older open step's, or one edited
// after the open step) is refused by the container run before any row and by
// the close step with nothing appended; so is one downloaded for other roots.
func TestModCacheReceiptRequired(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	withModuleRoots(t, fx)
	stubDownload(t)
	stubAmbient(t)
	clock := fixedClock(t)
	cache := filepath.Join(t.TempDir(), "modcache")
	putFile(t, fx.check, validCheck)
	if res, out := captureVerifyrunOut(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt, "--module-cache", cache}); res.code != verifyrunExitPass {
		t.Fatalf("open: exit %d: %s", res.code, out)
	}
	honest := getFile(t, fx.receipt)
	edit := func(f func(map[string]any)) string {
		var m map[string]any
		if err := json.Unmarshal([]byte(honest), &m); err != nil {
			t.Fatal(err)
		}
		f(m)
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return string(b) + "\n"
	}
	stripped := edit(func(m map[string]any) { delete(m, "module_cache") })
	otherRoots := edit(func(m map[string]any) { m["module_cache"].(map[string]any)["roots"] = []string{"statusgen"} })

	containerModuleEnv(t, cache)
	stubMountRO(t, true)
	for _, c := range []struct{ name, receipt string }{{"no module_cache", stripped}, {"other roots", otherRoots}} {
		putFile(t, fx.receipt, c.receipt)
		brief := getFile(t, fx.brief)
		if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code == verifyrunExitPass || !strings.Contains(stderr, "no Verify rows executed") || !strings.Contains(stderr, moduleRootsManifest) {
			t.Errorf("container run on a receipt with %s: exit %d stderr %q", c.name, res.code, stderr)
		}
		if _, err := os.Stat(fx.marker); err == nil {
			t.Errorf("container run on a receipt with %s ran a Verify row", c.name)
			_ = os.Remove(fx.marker)
		}
		if _, err := os.Stat(fx.witness); err == nil {
			t.Errorf("container run on a receipt with %s wrote a witness", c.name)
			_ = os.Remove(fx.witness)
		}
		if getFile(t, fx.brief) != brief {
			t.Fatalf("container run on a receipt with %s changed the brief", c.name)
		}
	}

	// An honest container run, then a close handed the stripped receipt and a
	// witness with its Module-Cache line removed: the two would agree on "no
	// cache", so only the home's manifest can refuse it.
	putFile(t, fx.receipt, honest)
	*clock = clock.Add(time.Minute)
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container run: exit %d stderr %q", res.code, stderr)
	}
	witness := getFile(t, fx.witness)
	var lines []string
	for _, l := range strings.SplitAfter(witness, "\n") {
		if !strings.HasPrefix(l, moduleCachePrefix) {
			lines = append(lines, l)
		}
	}
	noCache := strings.Join(lines, "")
	if noCache == witness {
		t.Fatalf("the witness carries no %s line:\n%s", moduleCachePrefix, witness)
	}
	putFile(t, fx.witness, noCache)
	putFile(t, fx.receipt, stripped)
	*clock = clock.Add(time.Minute)
	closeArgs := []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness}
	if res, stderr := captureVerifyrun(t, closeArgs); res.code == verifyrunExitPass || !strings.Contains(stderr, moduleRootsManifest) {
		t.Fatalf("close on a stripped receipt: exit %d stderr %q", res.code, stderr)
	}
	if got := getFile(t, fx.witness); got != noCache {
		t.Fatalf("a refused close changed the witness:\n%s", got)
	}
}

// regression: #4683
// TestModCachePathOptionChars: the cache path is written into the printed
// --mount option list, so a path that could add a mount option (a comma, a
// quote, whitespace) is refused before any download.
func TestModCachePathOptionChars(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	withModuleRoots(t, fx)
	dl := stubDownload(t)
	stubAmbient(t)
	putFile(t, fx.check, validCheck)
	base := t.TempDir()
	for _, name := range []string{"mod,readonly=false", "mod cache", "mod\"x", "mod\tx"} {
		res, out := captureVerifyrunOut(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt, "--module-cache", filepath.Join(base, name)})
		if res.code != verifyrunExitCouldNot || !strings.Contains(out, "--mount option list") {
			t.Errorf("--module-cache %q: exit %d, want a refusal naming the mount option list:\n%s", name, res.code, out)
		}
	}
	if len(dl.calls) != 0 {
		t.Fatalf("a refused cache path still downloaded: %v", dl.calls)
	}
}
