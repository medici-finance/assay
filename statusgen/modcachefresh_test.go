package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskcore/attestation"
)

// The open step's module cache (#4683). Every test here runs on fixtures: the
// container bracket fixture's home, a faked `go mod download` that writes into
// the GOMODCACHE it is handed, and the read-only mount check stubbed. No
// container engine, module proxy or network is used.

// withModuleRoots adds the module-roots manifest and one root (go.mod and
// go.sum) to the fixture home and commits them.
func withModuleRoots(t *testing.T, fx containerFixture) {
	t.Helper()
	for path, body := range map[string]string{
		moduleRootsManifest: "tools/desk\n",
		"tools/desk/go.mod": "module example.com/desk\n\ngo 1.27.0\n\nrequire golang.org/x/text v0.43.0\n",
		"tools/desk/go.sum": "golang.org/x/text v0.43.0 h1:fixture=\ngolang.org/x/text v0.43.0/go.mod h1:fixture=\n",
	} {
		p := filepath.Join(fx.home, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		putFile(t, p, body)
	}
	runGit(t, fx.home, "add", ".")
	runGit(t, fx.home, "commit", "-m", "module roots")
}

// fakeDownload replaces `go mod download`: it records each call's dir and env
// and, unless fail is set, plants the root's module in the GOMODCACHE env names.
type fakeDownload struct {
	calls []string
	envs  [][]string
	fail  error
}

func stubDownload(t *testing.T) *fakeDownload {
	t.Helper()
	f := &fakeDownload{}
	orig := goModDownload
	goModDownload = func(dir string, env []string) ([]byte, error) {
		f.calls = append(f.calls, dir)
		f.envs = append(f.envs, env)
		if f.fail != nil {
			return []byte("go: golang.org/x/text@v0.43.0: Get \"https://proxy.golang.org/...\": dial tcp: no route\n"), f.fail
		}
		for _, kv := range env {
			if c, ok := strings.CutPrefix(kv, "GOMODCACHE="); ok {
				bakeModule(t, c, "golang.org/x/text", "v0.43.0")
			}
		}
		return nil, nil
	}
	t.Cleanup(func() { goModDownload = orig })
	return f
}

// stubMountRO makes mountReadOnly answer ro for every path.
func stubMountRO(t *testing.T, ro bool) {
	t.Helper()
	orig := mountReadOnly
	mountReadOnly = func(string) (bool, error) { return ro, nil }
	t.Cleanup(func() { mountReadOnly = orig })
}

// stubAmbient fixes the host's ambient module caches to dirs.
func stubAmbient(t *testing.T, dirs ...string) {
	t.Helper()
	orig := ambientModuleCaches
	ambientModuleCaches = func() []string { return dirs }
	t.Cleanup(func() { ambientModuleCaches = orig })
}

// containerModuleEnv is the module environment the open step's flags set.
func containerModuleEnv(t *testing.T, cache string) {
	t.Helper()
	t.Setenv("GOMODCACHE", cache)
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOFLAGS", "-mod=readonly")
	for _, k := range []string{"GONOSUMDB", "GONOSUMCHECK", "GOINSECURE", "GOPRIVATE", "GONOPROXY", "GOSUMDB"} {
		t.Setenv(k, "")
	}
}

func fixedClock(t *testing.T) *time.Time {
	t.Helper()
	clock := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	saved := nowFunc
	nowFunc = func() time.Time { return clock }
	t.Cleanup(func() { nowFunc = saved })
	return &clock
}

// regression: #4683
// TestModCacheMountsReadOnly: the open step downloads the home's roots into
// the fresh cache and records its digest; the container run admits only that
// cache, mounted read-only, records the digest as Module-Cache, and the close
// step binds it to the receipt's.
func TestModCacheMountsReadOnly(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	withModuleRoots(t, fx)
	dl := stubDownload(t)
	ambient := t.TempDir()
	stubAmbient(t, ambient)
	clock := fixedClock(t)
	cache := filepath.Join(t.TempDir(), "modcache")
	putFile(t, fx.check, validCheck)

	res, out := captureVerifyrunOut(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt, "--module-cache", cache})
	if res.code != verifyrunExitPass {
		t.Fatalf("open: exit %d: %s", res.code, out)
	}
	if len(dl.calls) != 1 || dl.calls[0] != filepath.Join(fx.home, "tools", "desk") {
		t.Fatalf("download calls %v, want one in the home's tools/desk", dl.calls)
	}
	var receipt bracketReceipt
	if err := json.Unmarshal([]byte(getFile(t, fx.receipt)), &receipt); err != nil {
		t.Fatal(err)
	}
	want, err := homeTreeDigest(cache)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ModuleCache == nil || receipt.ModuleCache.Digest != want || strings.Join(receipt.ModuleCache.Roots, ",") != "tools/desk" {
		t.Fatalf("receipt module cache %+v, want digest %s for tools/desk", receipt.ModuleCache, want)
	}
	mount := "--mount type=bind,source=" + cache + ",target=/go/pkg/mod,readonly"
	if !strings.Contains(out, mount) || strings.Contains(out, ambient) {
		t.Fatalf("the open step's container flags do not mount the fresh cache read-only (or name the ambient one):\n%s", out)
	}

	// Container: the right cache, read-only, under the hard-line env.
	containerModuleEnv(t, cache)
	stubMountRO(t, false)
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code == verifyrunExitPass || !strings.Contains(stderr, "mounted read-write") {
		t.Fatalf("read-write mount: exit %d stderr %q", res.code, stderr)
	}
	fx.assertNothingRan(t, getFile(t, fx.brief))
	stubMountRO(t, true)
	other := t.TempDir()
	bakeModule(t, other, "golang.org/x/text", "v0.41.0")
	t.Setenv("GOMODCACHE", other)
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code == verifyrunExitPass || !strings.Contains(stderr, "not the "+want) {
		t.Fatalf("another cache (the image's baked one): exit %d stderr %q", res.code, stderr)
	}
	fx.assertNothingRan(t, getFile(t, fx.brief))
	t.Setenv("GOMODCACHE", cache)
	*clock = clock.Add(time.Minute)
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container run: exit %d stderr %q", res.code, stderr)
	}
	witness := getFile(t, fx.witness)
	if !regexp.MustCompile(`(?m)^Container-Image: \S+\nModule-Cache: ` + regexp.QuoteMeta(want) + `\nVerification-Attestation: `).MatchString(witness) {
		t.Fatalf("the witness does not record Module-Cache %s between the image and the attestation:\n%s", want, witness)
	}

	// Close: a witness whose Module-Cache is not the receipt's, or that
	// carries none, is refused; the honest one closes.
	closeArgs := []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness}
	for name, w := range map[string]string{
		"other digest": strings.Replace(witness, moduleCachePrefix+want, moduleCachePrefix+"sha256:"+strings.Repeat("e", 64), 1),
		"no line":      strings.Replace(witness, moduleCachePrefix+want+"\n", "", 1),
	} {
		putFile(t, fx.witness, w)
		if res, stderr := captureVerifyrun(t, closeArgs); res.code == verifyrunExitPass || !strings.Contains(stderr, "not the receipt's module cache") {
			t.Fatalf("close with %s: exit %d stderr %q", name, res.code, stderr)
		}
	}
	putFile(t, fx.witness, witness)
	*clock = clock.Add(time.Minute)
	if res, out := captureVerifyrunOut(t, closeArgs); res.code != verifyrunExitPass {
		t.Fatalf("close: exit %d: %s", res.code, out)
	}
	checks := bracketChecks(getFile(t, fx.witness))
	if len(checks) != 1 || checks[0].verdict != "match" {
		t.Fatalf("bracket checks %+v, want one match", checks)
	}
}

// regression: #4683
// TestModCacheDownloadFails: a failed download refuses the open step as
// could-not-check and writes no receipt; and a home that lists module roots is
// refused without --module-cache, so its rows never fall back to the image's
// baked cache.
func TestModCacheDownloadFails(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	withModuleRoots(t, fx)
	dl := stubDownload(t)
	stubAmbient(t)
	putFile(t, fx.check, validCheck)
	open := []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt}

	res, stderr := captureVerifyrun(t, open)
	if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "could-not-check") || !strings.Contains(stderr, "--module-cache") {
		t.Fatalf("no --module-cache on a home with module roots: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.receipt); err == nil {
		t.Fatal("an open step without its module cache wrote a receipt")
	}

	dl.fail = fmt.Errorf("exit status 1")
	cache := filepath.Join(t.TempDir(), "modcache")
	res, stderr = captureVerifyrun(t, append(open, "--module-cache", cache))
	if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "could-not-check") || !strings.Contains(stderr, "not a substitute") {
		t.Fatalf("failed download: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.receipt); err == nil {
		t.Fatal("a failed download wrote a receipt")
	}

	// A download that rewrites go.sum is not bound to it.
	dl.fail = nil
	orig := goModDownload
	goModDownload = func(dir string, env []string) ([]byte, error) {
		return nil, os.WriteFile(filepath.Join(dir, "go.sum"), []byte("rewritten\n"), 0o644)
	}
	t.Cleanup(func() { goModDownload = orig })
	res, stderr = captureVerifyrun(t, append(open, "--module-cache", filepath.Join(t.TempDir(), "modcache")))
	if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "changed its go.sum") {
		t.Fatalf("go.sum rewritten: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.receipt); err == nil {
		t.Fatal("a download that rewrote go.sum wrote a receipt")
	}
}

// regression: #4683
// TestModCacheHardLineFlags: the container flags the open step prints keep
// --network none, the read-only root, the read-only mount, GOPROXY=off and
// GOTOOLCHAIN=local, and carry no checksum-loosening setting; the download's
// environment drops every such setting the host has; the container step
// refuses each one.
func TestModCacheHardLineFlags(t *testing.T) {
	flags := strings.Join(containerModuleCacheFlags("/fresh/cache"), " ")
	for _, want := range []string{"--network none", "--read-only", "--mount type=bind,source=/fresh/cache,target=/go/pkg/mod,readonly", "--env GOPROXY=off", "--env GOTOOLCHAIN=local"} {
		if !strings.Contains(flags, want) {
			t.Errorf("container flags %q lack %q", flags, want)
		}
	}
	forbidden := []string{"GONOSUMDB", "GONOSUMCHECK", "GOINSECURE", "GOPRIVATE", "GONOPROXY", "-insecure", "GOSUMDB=off", "--network host", "--network=host", "rw"}
	for _, bad := range forbidden {
		if strings.Contains(flags, bad) {
			t.Errorf("container flags %q carry %q", flags, bad)
		}
	}

	for k, v := range map[string]string{"GONOSUMDB": "*", "GONOSUMCHECK": "1", "GOINSECURE": "*", "GOPRIVATE": "*", "GONOPROXY": "*", "GOFLAGS": "-insecure", "GOSUMDB": "off", "GOPROXY": "direct", "GOMODCACHE": "/ambient", "GIT_CONFIG_GLOBAL": "/x", "HTTPS_PROXY": "http://proxy.invalid", "HOME": "/host/home", "NETRC": "/host/.netrc", "FORGE_TOKEN": "fixture-token"} {
		t.Setenv(k, v)
	}
	env := strings.Join(moduleDownloadEnv("/fresh/cache", "/scratch"), "\n") + "\n"
	for _, want := range []string{"GOMODCACHE=/fresh/cache\n", "GOSUMDB=sum.golang.org\n", "GOPROXY=https://proxy.golang.org\n", "GOTOOLCHAIN=local\n", "GOFLAGS=-mod=readonly\n", "GOENV=off\n", "GOWORK=off\n", "GOVCS=*:off\n", "HOME=/scratch\n", "GOCACHE=/scratch/gocache\n"} {
		if !strings.Contains(env, want) {
			t.Errorf("download env lacks %q", want)
		}
	}
	for _, bad := range forbidden[:6] {
		if strings.Contains(env, bad) {
			t.Errorf("download env carries %q:\n%s", bad, env)
		}
	}
	// The environment is built from nothing: no host HOME (its .netrc, git and
	// go configuration), no forge credential, nothing but PATH and the pins.
	for _, bad := range []string{"GOMODCACHE=/ambient", "GIT_CONFIG_GLOBAL", "HTTPS_PROXY", "GOPROXY=direct", "/host/home", "NETRC", "fixture-token"} {
		if strings.Contains(env, bad) {
			t.Errorf("download env carries the host's %q", bad)
		}
	}

	cache := t.TempDir()
	digest, err := homeTreeDigest(cache)
	if err != nil {
		t.Fatal(err)
	}
	mc := &receiptModuleCache{Digest: digest, Roots: []string{"tools/desk"}}
	stubMountRO(t, true)
	root := t.TempDir()
	containerModuleEnv(t, cache)
	if err := containerModuleCacheProblem(root, mc); err != nil {
		t.Fatalf("hard-line env refused: %v", err)
	}
	for _, c := range []struct{ k, v, why string }{
		{"GOPROXY", "https://proxy.golang.org", "GOPROXY=off"},
		{"GOTOOLCHAIN", "auto", "GOTOOLCHAIN=local"},
		{"GONOSUMDB", "*", "GONOSUMDB is set"},
		{"GONOSUMCHECK", "1", "GONOSUMCHECK is set"},
		{"GOINSECURE", "*", "GOINSECURE is set"},
		{"GOPRIVATE", "*", "GOPRIVATE is set"},
		{"GOSUMDB", "off", "GOSUMDB=off"},
		{"GOFLAGS", "-mod=readonly -insecure", "-insecure"},
		{"GOMODCACHE", "", "GOMODCACHE is unset"},
		{"GOMODCACHE", filepath.Join(root, "mod"), "inside the root"},
	} {
		containerModuleEnv(t, cache)
		t.Setenv(c.k, c.v)
		if err := containerModuleCacheProblem(root, mc); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s=%q: %v, want a refusal naming %q", c.k, c.v, err, c.why)
		}
	}
}

// regression: #4683
// TestModCacheNeverAmbient: the open step refuses a --module-cache that is the
// host's own module cache, lies inside it or holds it, or already holds files,
// before any download and with no receipt.
func TestModCacheNeverAmbient(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	withModuleRoots(t, fx)
	dl := stubDownload(t)
	parent := t.TempDir()
	ambient := filepath.Join(parent, "go", "pkg", "mod")
	bakeModule(t, ambient, "golang.org/x/text", "v0.43.0")
	stubAmbient(t, ambient)
	putFile(t, fx.check, validCheck)
	used := t.TempDir()
	putFile(t, filepath.Join(used, "stale"), "x\n")
	for _, c := range []struct{ dir, why string }{
		{ambient, "the host's own module cache"},
		{filepath.Join(ambient, "sub"), "the host's own module cache"},
		{parent, "holds the host's own module cache"},
		{used, "already holds files"},
		{filepath.Join(fx.home, "mod"), "inside the home"},
		{"relative/mod", "not an absolute path"},
	} {
		res, stderr := captureVerifyrun(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt, "--module-cache", c.dir})
		if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, c.why) {
			t.Errorf("--module-cache %s: exit %d stderr %q, want a refusal naming %q", c.dir, res.code, stderr, c.why)
		}
		if _, err := os.Stat(fx.receipt); err == nil {
			t.Fatalf("--module-cache %s wrote a receipt", c.dir)
		}
	}
	if len(dl.calls) != 0 {
		t.Fatalf("a refused --module-cache still downloaded: %v", dl.calls)
	}
	// The production ambient list names the host's GOMODCACHE, GOPATH caches
	// and the default under HOME.
	host := t.TempDir()
	t.Setenv("GOMODCACHE", filepath.Join(host, "gomodcache"))
	t.Setenv("GOPATH", filepath.Join(host, "gopath"))
	t.Setenv("HOME", filepath.Join(host, "userhome"))
	goTelemetryOff(t)
	got := strings.Join(origAmbientModuleCaches(), "\n") + "\n"
	for _, want := range []string{
		filepath.Join(host, "gomodcache") + "\n",
		filepath.Join(host, "gopath", "pkg", "mod") + "\n",
		filepath.Join(host, "userhome", "go", "pkg", "mod") + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ambient caches %q do not name %q", got, want)
		}
	}
}

var origAmbientModuleCaches = ambientModuleCaches

// goTelemetryOff sets Go telemetry to off in the current HOME's user config
// directory. A go command run under a HOME inside t.TempDir otherwise leaves a
// telemetry child process writing there after it returns, which races the
// TempDir cleanup ("directory not empty").
func goTelemetryOff(t *testing.T) {
	t.Helper()
	cfg, err := os.UserConfigDir()
	if err != nil {
		return // the go command finds no config directory either
	}
	dir := filepath.Join(cfg, "go", "telemetry")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mode"), []byte("off"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// regression: #4683
// TestModCacheFromSourceOnly is the defect-class guard: an execution
// environment whose dependency set is fixed at image-build time rather than
// derived from the artifact under test. Every tracked Dockerfile whose final
// (runtime) stage provisions a Go module cache must take its roots from the
// source's own moduleRootsManifest; a planted image that bakes a literal root
// goes red.
func TestModCacheFromSourceOnly(t *testing.T) {
	out, err := exec.Command("git", "-C", "..", "ls-files", "--", "*Dockerfile*").Output()
	if err != nil {
		t.Skipf("git ls-files: %v", err)
	}
	checked := 0
	for _, f := range strings.Fields(string(out)) {
		if strings.HasSuffix(f, ".dockerignore") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("..", f))
		if err != nil {
			t.Fatal(err)
		}
		if p := bakedModuleCacheProblem(string(raw)); p != "" {
			t.Errorf("%s: %s", f, p)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no Dockerfile checked")
	}
	planted := "FROM golang:1.27.3 AS build\nRUN go mod download\nFROM debian:bookworm-slim\nENV GOMODCACHE=/go/pkg/mod\nCOPY tools/desk/go.mod tools/desk/go.sum /src/\nRUN cd /src && go mod download\n"
	if p := bakedModuleCacheProblem(planted); p == "" {
		t.Fatal("a runtime stage that bakes a literal module root passed the guard")
	}
}

// bakedModuleCacheProblem says why a Dockerfile's final stage provisions a Go
// module cache from anything but the source's moduleRootsManifest. Build
// stages (any stage before the last FROM) may download what they compile.
func bakedModuleCacheProblem(df string) string {
	lines := strings.Split(df, "\n")
	last := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(l)), "FROM ") {
			last = i
		}
	}
	final := strings.Join(lines[last+1:], "\n")
	downloads := regexp.MustCompile(`go mod download`).FindAllStringIndex(final, -1)
	if len(downloads) == 0 && !strings.Contains(final, "GOMODCACHE") {
		return ""
	}
	if !strings.Contains(final, "COPY "+moduleRootsManifest+" "+bakedModuleRootsFile+"\n") {
		return fmt.Sprintf("the runtime stage provisions a Go module cache but does not copy %s to %s", moduleRootsManifest, bakedModuleRootsFile)
	}
	loop := regexp.MustCompile(`while read -r root; do[^\n]*\n(?:[^\n]*\n)*?\s*\(cd "/tmp/modsrc/\$root" && go mod download\); \\\n\s*done < ` + regexp.QuoteMeta(bakedModuleRootsFile) + `;`)
	if n := len(loop.FindAllString(final, -1)); n != 1 || len(downloads) != 1 {
		return fmt.Sprintf("the runtime stage runs `go mod download` %d time(s), %d of them driven by %s; every root must come from the manifest", len(downloads), n, moduleRootsManifest)
	}
	return ""
}

// regression: #4683
// TestModEgressParity: the open step's approved public set is the cluster
// stager's (stage-cache.sh APPROVED and GHPUB), so the two screens of the same
// download cannot drift apart; a planted stager with one more host goes red.
func TestModEgressParity(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "images", "verify-runner", "stage-cache.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if p := modEgressParityProblem(string(raw)); p != "" {
		t.Fatal(p)
	}
	planted := strings.Replace(string(raw), "APPROVED='^(", "APPROVED='^(example[.]net|", 1)
	if planted == string(raw) {
		t.Fatal("the planted stager did not change APPROVED")
	}
	if p := modEgressParityProblem(planted); !strings.Contains(p, "example.net/") {
		t.Fatalf("parity blind to a planted extra host: %q", p)
	}
}

// modEgressParityProblem compares stage-cache.sh's APPROVED and GHPUB with
// approvedPublicModulePrefixes and githubPublicModuleRe.
func modEgressParityProblem(script string) string {
	m := regexp.MustCompile(`(?m)^APPROVED='\^\(([^)]*)\)/'$`).FindStringSubmatch(script)
	if m == nil {
		return "stage-cache.sh carries no APPROVED='^(...)/' line"
	}
	var hosts []string
	for _, h := range strings.Split(m[1], "|") {
		hosts = append(hosts, strings.ReplaceAll(h, "[.]", ".")+"/")
	}
	if got, want := strings.Join(hosts, " "), strings.Join(approvedPublicModulePrefixes, " "); got != want {
		return fmt.Sprintf("stage-cache.sh APPROVED is %q, the open step's set is %q", got, want)
	}
	g := regexp.MustCompile(`(?m)^GHPUB='([^']*)'$`).FindStringSubmatch(script)
	if g == nil || strings.ReplaceAll(g[1], "[.]", `\.`) != githubPublicModuleRe.String() {
		return fmt.Sprintf("stage-cache.sh GHPUB %q is not the open step's %q", g, githubPublicModuleRe.String())
	}
	return ""
}

// regression: #4683
// TestModEgressPatterns: the screen matches the operator's private patterns as
// the go command does (a glob over a prefix of path elements), refuses the
// verified repository's owners in any case, and reads the patterns from the
// environment the download environment drops.
func TestModEgressPatterns(t *testing.T) {
	for _, c := range []struct {
		pattern, path string
		want          bool
	}{
		{"github.com/acme-corp/*", "github.com/acme-corp/lib/v2", true},
		{"github.com/acme-corp", "github.com/acme-corp/lib", true},
		{"*.corp.example", "git.corp.example/x", true},
		{"github.com/acme-corp/*", "github.com/acme-corpx/lib", false},
		{"github.com/acme-corp/lib/sub", "github.com/acme-corp/lib", false},
	} {
		if got := matchModulePattern(c.pattern, c.path); got != c.want {
			t.Errorf("matchModulePattern(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
	if w := modulePathEgressProblem("github.com/EXAMPLE-ORG/x", []string{"example-org"}, nil); !strings.Contains(w, "owner") {
		t.Errorf("an owner path in upper case: %q", w)
	}
	for _, p := range []string{"golang.org/x/text", "github.com/spf13/cobra"} {
		if w := modulePathEgressProblem(p, []string{"example-org"}, nil); w != "" {
			t.Errorf("%s refused: %s", p, w)
		}
	}
	if w := modulePathEgressProblem("golang.org/x/text", nil, []string{"golang.org/x"}); !strings.Contains(w, "private pattern") {
		t.Errorf("an operator-private approved path: %q", w)
	}
	t.Setenv("GOPRIVATE", "a.example/*")
	t.Setenv("GONOPROXY", "b.example")
	t.Setenv("GONOSUMDB", "")
	got := strings.Join(origOperatorPrivateModulePatterns(), ",")
	for _, want := range []string{"a.example/*", "b.example"} {
		if !strings.Contains(got, want) {
			t.Errorf("operator patterns %q lack %q", got, want)
		}
	}
	if owners, err := bindingOwners(attestation.Binding{Repo: "Example-Org/one", SourceRepo: "other/two", SiblingSources: "third/x@" + strings.Repeat("a", 40)}); err != nil || strings.Join(owners, ",") != "example-org,other,third" {
		t.Errorf("binding owners %v, %v", owners, err)
	}
}

var origOperatorPrivateModulePatterns = operatorPrivateModulePatterns

// regression: #4683
// TestModEgressMainRoots: the module paths the listed roots on main could
// request pass the open step's screen, dario.cat/mergo (gitcore, qualgen,
// statusgen, tools/desk) and gitlab.com/gitlab-org/api/client-go (tools/desk)
// among them, while a path under a neighbouring prefix that is not approved
// still refuses.
func TestModEgressMainRoots(t *testing.T) {
	orig := operatorPrivateModulePatterns
	operatorPrivateModulePatterns = func() []string { return nil }
	t.Cleanup(func() { operatorPrivateModulePatterns = orig })
	owners := []string{"medici-finance"}
	for _, p := range []string{"dario.cat/mergo", "gitlab.com/gitlab-org/api/client-go"} {
		if w := modulePathEgressProblem(p, owners, nil); w != "" {
			t.Errorf("%s refused: %s", p, w)
		}
	}
	for _, p := range []string{
		"dario.cats/mergo", "sub.dario.cat/mergo",
		"gitlab.com/gitlab-org/apix/y", "gitlab.com/gitlab-org/cli",
		"gitlab.com/someone/api/client-go", "gitlab.com/x",
	} {
		if w := modulePathEgressProblem(p, owners, nil); w != "not on the approved public set" {
			t.Errorf("%s: got %q, want the unapproved refusal", p, w)
		}
	}
	roots, ok := bakedModuleRoots(filepath.Join("..", filepath.FromSlash(moduleRootsManifest)))
	if !ok {
		t.Fatalf("%s names no root", moduleRootsManifest)
	}
	env := moduleDownloadEnv(t.TempDir(), t.TempDir())
	if _, err := moduleEgressProblem("..", roots, owners, env, io.Discard); err != nil {
		t.Fatal(err)
	}
}
