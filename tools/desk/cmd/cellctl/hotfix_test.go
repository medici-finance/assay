//go:build unix

package main

// `cellctl hotfix` and the shim layer. Every test works in temp dirs: a throwaway overlay, a
// throwaway pinned dir, an offline git repository standing in for the public source, and an
// httptest forge. None reads or writes /opt/desk-tools or a real overlay, and the build and
// sudo gate are test seams (package vars only _test.go reassigns), so no network and no
// privilege is used.

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	hfBase    = "v1.0.41"
	hfMoved   = "v1.0.42"
	hfGoVer   = "1.27.2"
	hfNowText = "2026-10-25T12:00:00Z"
)

type hotfixWorld struct {
	t                           *testing.T
	root, overlay, pinned, home string
	srcDir, sha                 string
	mu                          sync.Mutex
	mainStatus                  string   // forge answer for main...<sha>
	pinStatus                   string   // forge answer for <pin>...<sha>
	pinFiles                    []string // files the forge lists for <pin>...<sha>
	compares                    []string
	builds                      []hotfixBuildSpec
	now                         time.Time
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir,
		"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newHotfixWorld builds the world and binds every seam; cleanup restores them.
func newHotfixWorld(t *testing.T, pin string) *hotfixWorld {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base := t.TempDir()
	w := &hotfixWorld{t: t, mainStatus: "behind", pinStatus: "ahead",
		root: filepath.Join(base, "repo"), overlay: filepath.Join(base, "overlay"),
		pinned: filepath.Join(base, "pinned"), home: filepath.Join(base, "home"), srcDir: filepath.Join(base, "src")}
	w.now, _ = time.Parse(time.RFC3339, hfNowText)
	for _, d := range []string{w.root, w.pinned, w.home, filepath.Join(w.srcDir, ".github", "workflows"),
		filepath.Join(w.srcDir, "tools", "desk", "cmd", "deskboard"), filepath.Join(w.srcDir, "tools", "desk", "cmd", "deskstatus"),
		filepath.Join(w.srcDir, "tools", "desk", "cmd", "cellctl"), filepath.Join(w.srcDir, "tools", "desk", "cmd", "deskboot"),
		filepath.Join(w.srcDir, "statusgen")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(w.root, ".assay-versions"), "desk-tools "+pin+" 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n")
	write(filepath.Join(w.srcDir, ".github", "workflows", "release.yml"), "env:\n  GO_VERSION: \""+hfGoVer+"\"\n")
	write(filepath.Join(w.srcDir, "tools", "desk", "go.sum"), "example.com/x v0.0.0 h1:fixture\n")
	for _, tool := range []string{"deskboard", "deskstatus", "cellctl", "deskboot"} {
		write(filepath.Join(w.srcDir, "tools", "desk", "cmd", tool, "main.go"), "package main\n")
	}
	write(filepath.Join(w.srcDir, "statusgen", "go.mod"), "module github.com/medici-finance/assay/statusgen\n")
	write(filepath.Join(w.srcDir, "statusgen", "go.sum"), "example.com/statusgen-dep v0.0.0 h1:fixture\n")
	write(filepath.Join(w.srcDir, "statusgen", "main.go"), "package main\n")
	gitIn(t, w.srcDir, "init", "-q", "-b", "main")
	gitIn(t, w.srcDir, "add", "-A")
	gitIn(t, w.srcDir, "commit", "-q", "-m", "fixture source")
	w.sha = gitIn(t, w.srcDir, "rev-parse", "HEAD")

	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.compares = append(w.compares, r.URL.Path)
		spec := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		base, _, _ := strings.Cut(spec, "...")
		status, files := w.mainStatus, []string(nil)
		if base != "main" {
			status, files = w.pinStatus, w.pinFiles
		}
		var fl []string
		for _, f := range files {
			fl = append(fl, fmt.Sprintf(`{"filename":%q}`, f))
		}
		fmt.Fprintf(rw, `{"status":%q,"files":[%s]}`, status, strings.Join(fl, ","))
	}))
	t.Cleanup(srv.Close)

	uid := uint32(os.Getuid())
	save := []any{hotfixForge, hotfixWho, hotfixCloneURL, hotfixGoVersion, hotfixRunBuild, hotfixOwner, hotfixClock}
	t.Cleanup(func() {
		hotfixForge, hotfixWho, hotfixCloneURL = save[0].(func() deskkit.HotfixForge), save[1].(func() (hotfixInvoker, error)), save[2].(func() string)
		hotfixGoVersion, hotfixRunBuild = save[3].(func(hotfixInvoker, string, []string) (string, error)), save[4].(func(hotfixBuildSpec) error)
		hotfixOwner, hotfixClock = save[5].(func() deskkit.HotfixOwnerCheck), save[6].(func() time.Time)
	})
	hotfixForge = func() deskkit.HotfixForge {
		return deskkit.HotfixForge{BaseURL: srv.URL, Repo: deskkit.HotfixRepoSlug, HTTP: srv.Client()}
	}
	hotfixWho = func() (hotfixInvoker, error) {
		return hotfixInvoker{User: "operator", UID: uid, GID: uint32(os.Getgid())}, nil
	}
	hotfixCloneURL = func() string { return "file://" + w.srcDir }
	hotfixGoVersion = func(hotfixInvoker, string, []string) (string, error) { return "go" + hfGoVer, nil }
	hotfixOwner = func() deskkit.HotfixOwnerCheck { return deskkit.HotfixOwnedBy(uid) }
	hotfixClock = func() time.Time { return w.now }
	hotfixRunBuild = func(s hotfixBuildSpec) error {
		w.builds = append(w.builds, s)
		tag := deskkit.HotfixTag(hfBase, w.sha)
		for _, tool := range s.Tools {
			script := "#!/bin/sh\necho 'releaseTag=" + tag + "'\n"
			if err := os.WriteFile(filepath.Join(s.Out, tool), []byte(script), 0o755); err != nil {
				return err
			}
		}
		return nil
	}
	t.Setenv(deskkit.HotfixOverlayEnv, w.overlay)
	t.Setenv(deskkit.DeskToolsBinEnv, w.pinned)
	t.Chdir(w.root)
	return w
}

func (w *hotfixWorld) run(args ...string) (code int, stdout, stderr string) {
	var o, e bytes.Buffer
	code = cmdHotfix(args, &o, &e)
	return code, o.String(), e.String()
}

func (w *hotfixWorld) manifest() *deskkit.HotfixManifest {
	w.t.Helper()
	m, err := deskkit.ReadHotfixManifest(w.overlay)
	if err != nil {
		w.t.Fatal(err)
	}
	return m
}

func (w *hotfixWorld) install(tools ...string) {
	w.t.Helper()
	code, out, errs := w.run(append(append([]string{"install"}, tools...), "--source", w.sha)...)
	if code != 0 {
		w.t.Fatalf("install rc=%d\n%s\n%s", code, out, errs)
	}
}

func hfEnvValue(env []string, key string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], key+"="); ok {
			return v, true
		}
	}
	return "", false
}

// Row 2: install builds a main-resident commit, stamps it, and records the eleven-field
// manifest entry; the build environment is set from scratch whatever the caller exports.
func TestHotfixInstall(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOMODCACHE", "/caller/modcache")
	t.Setenv("GOTOOLCHAIN", "auto")
	t.Setenv("GOPROXY", "https://caller.invalid")
	t.Setenv("GOFLAGS", "-mod=mod")

	code, out, errs := w.run("install", "deskboard", "--source", w.sha, "--ack", "per the driver's ruling")
	if code != 0 {
		t.Fatalf("install rc=%d\n%s\n%s", code, out, errs)
	}
	wantTag := hfBase + "+hotfix-" + w.sha[:12]
	if !strings.Contains(out, "hotfix installed: deskboard "+wantTag) {
		t.Errorf("install output lacks the tag %s:\n%s", wantTag, out)
	}
	bin := deskkit.HotfixBinaryPath(w.overlay, "deskboard")
	got, err := exec.Command(bin, "--version").Output()
	if err != nil || deskkit.ParseReleaseTagOutput(string(got)) != wantTag {
		t.Errorf("overlay --version = %q (%v), want the tag %s", got, err, wantTag)
	}
	e, ok := w.manifest().Entry("deskboard")
	if !ok {
		t.Fatal("manifest has no deskboard entry")
	}
	sum, _ := deskkit.FileSHA256(bin)
	for name, v := range map[string]string{
		"tool": e.Tool, "sha256": e.SHA256, "source_sha": e.SourceSHA, "base_pin": e.BasePin, "built_at": e.BuiltAt,
		"expires": e.Expires, "source_rule": e.SourceRule, "ack": e.Ack, "installed_by": e.InstalledBy,
		"go_version": e.GoVersion, "go_sum_sha256": e.GoSumSHA256,
	} {
		if v == "" {
			t.Errorf("manifest field %s is empty", name)
		}
	}
	if e.SHA256 != sum || e.SourceSHA != w.sha || e.BasePin != hfBase || e.SourceRule != deskkit.HotfixSourceRule ||
		e.GoVersion != hfGoVer || e.InstalledBy != "operator" || e.Ack != "per the driver's ruling" {
		t.Errorf("manifest entry does not match what was installed: %+v", e)
	}
	built, _ := time.Parse(time.RFC3339, e.BuiltAt)
	exp, _ := time.Parse(time.RFC3339, e.Expires)
	if exp.Sub(built) != deskkit.HotfixDefaultTTL {
		t.Errorf("default lifetime = %s, want %s", exp.Sub(built), deskkit.HotfixDefaultTTL)
	}
	if len(w.builds) != 1 {
		t.Fatalf("builds = %d", len(w.builds))
	}
	env := w.builds[0].Env
	for k, want := range map[string]string{"GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "local", "GOSUMDB": "sum.golang.org",
		"GONOSUMDB": "", "GOPRIVATE": "", "GONOPROXY": "", "GOENV": "off", "GOWORK": "off"} {
		if v, ok := hfEnvValue(env, k); !ok || v != want {
			t.Errorf("build env %s = %q (set %v), want %q", k, v, ok, want)
		}
	}
	if v, _ := hfEnvValue(env, "GOMODCACHE"); v == "/caller/modcache" || !strings.Contains(v, "gomodcache") {
		t.Errorf("build GOMODCACHE = %q, want a fresh directory", v)
	}
	if v, _ := hfEnvValue(env, "GOPROXY"); strings.Contains(v, "caller.invalid") {
		t.Errorf("build GOPROXY took the caller's value %q", v)
	}
	if mc, _ := hfEnvValue(env, "GOMODCACHE"); mc != "" {
		if _, err := os.Stat(mc); err == nil {
			t.Errorf("the throwaway module cache %s outlived the install", mc)
		}
	}
	if !strings.Contains(w.builds[0].Ldflags, "-X main.cellctlVersion="+wantTag) ||
		!strings.Contains(w.builds[0].Ldflags, "ReleaseTag="+wantTag) {
		t.Errorf("ldflags do not stamp the hotfix tag: %s", w.builds[0].Ldflags)
	}
	for _, bad := range []string{"HotfixSourceRule", "HotfixMaxTTL", "HotfixDefaultTTL", "HotfixRequireSudo"} {
		if strings.Contains(w.builds[0].Ldflags, bad) {
			t.Errorf("ldflags set the ruled constant %s", bad)
		}
	}

	for _, tool := range []string{"cellctl", "deskboot"} {
		code, _, errs := w.run("install", tool, "--source", w.sha)
		if code != hotfixExitRefused || !strings.Contains(errs, "never a hotfix target") {
			t.Errorf("install %s rc=%d, want a refusal naming never a hotfix target:\n%s", tool, code, errs)
		}
	}
	if len(w.builds) != 1 {
		t.Errorf("a refused target still built: %d builds", len(w.builds))
	}
}

// The binary is only installed when --version prints exactly the stamped tag.
func TestHotfixInstallRefusesWrongStamp(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	hotfixRunBuild = func(s hotfixBuildSpec) error {
		return os.WriteFile(filepath.Join(s.Out, "deskboard"), []byte("#!/bin/sh\necho 'releaseTag="+hfBase+"'\n"), 0o755)
	}
	code, _, errs := w.run("install", "deskboard", "--source", w.sha)
	if code != hotfixExitRefused || !strings.Contains(errs, "want exactly") {
		t.Fatalf("rc=%d\n%s", code, errs)
	}
	if _, err := os.Stat(deskkit.HotfixBinaryPath(w.overlay, "deskboard")); err == nil {
		t.Error("a binary stamped with the wrong tag was installed")
	}
}

// Row 5: once the pin moves, shim generation points at the pinned binary with a NOTICE.
func TestHotfixShimPinMoved(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	w.install("deskboard")
	writePinsFile(t, w.root, hfBase)
	c, shim := w.cell(t)
	c.genShims()
	body := readFile(t, shim, "deskboard")
	if !strings.Contains(body, `hotfix_bin="`+deskkit.HotfixBinaryPath(w.overlay, "deskboard")+`"`) {
		t.Fatalf("a live hotfix is not in the shim:\n%s", body)
	}

	writePinsFile(t, w.root, hfMoved)
	stderr := captureStderrOf(t, func() { c.genShims() })
	body = readFile(t, shim, "deskboard")
	if strings.Contains(body, "hotfix_bin") || !strings.Contains(body, filepath.Join(w.pinned, "deskboard")) {
		t.Errorf("after the pin moved the shim still points at the overlay:\n%s", body)
	}
	if !strings.Contains(stderr, "hotfix") || !strings.Contains(stderr, deskkit.HotfixReasonOutlived) {
		t.Errorf("no NOTICE naming the retired hotfix and why:\n%s", stderr)
	}
}

// Row 7: the shim past expiry runs the pinned binary and says so on stderr.
func TestHotfixShimExpired(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	w.install("deskboard")
	c, shim := w.cell(t)
	c.genShims()
	if !strings.Contains(readFile(t, shim, "deskboard"), "hotfix_bin") {
		t.Fatal("expected a hotfix shim before expiry")
	}
	w.now = w.now.Add(deskkit.HotfixDefaultTTL + time.Hour)
	stderr := captureStderrOf(t, func() { c.genShims() })
	body := readFile(t, shim, "deskboard")
	if strings.Contains(body, "hotfix_bin") || !strings.Contains(stderr, deskkit.HotfixReasonExpired) {
		t.Errorf("an expired hotfix is still selected (stderr %q):\n%s", stderr, body)
	}

	// An already generated hotfix shim also expires by its own clock, with the NOTICE.
	w.now = w.now.Add(-deskkit.HotfixDefaultTTL - time.Hour)
	c.genShims()
	shimPath := filepath.Join(shim, "deskboard")
	b, _ := os.ReadFile(shimPath)
	past := strings.Replace(string(b), "now >= ", "now >= 0 || now >= ", 1)
	if err := os.WriteFile(shimPath, []byte(past), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"deskboard"} {
		if err := os.WriteFile(filepath.Join(w.pinned, tool), []byte("#!/bin/sh\necho pinned-ran\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", shimPath)
	cmd.Env = append(os.Environ(), "HOME="+w.home)
	var se bytes.Buffer
	cmd.Stderr = &se
	out, _ := cmd.Output()
	if !strings.Contains(se.String(), "expired") || !strings.Contains(string(out), "pinned-ran") {
		t.Errorf("expired shim: stdout %q stderr %q, want the pinned binary and an expiry NOTICE", out, se.String())
	}
}

// regression: #4405
// Layer 1 on its own: shim generation refuses an overlay its owner check does not accept and
// names the pinned binary with a NOTICE, whatever the other layers decide.
func TestHotfixShimOverlayOwner(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	w.install("deskboard")
	c, shim := w.cell(t)
	c.genShims()
	if !strings.Contains(readFile(t, shim, "deskboard"), "hotfix_bin") {
		t.Fatal("control: a valid hotfix is not in the shim")
	}
	hotfixOwner = func() deskkit.HotfixOwnerCheck { return deskkit.HotfixOwnedBy(uint32(os.Getuid()) + 1) }
	stderr := captureStderrOf(t, func() { c.genShims() })
	body := readFile(t, shim, "deskboard")
	if strings.Contains(body, "hotfix_bin") || !strings.Contains(body, filepath.Join(w.pinned, "deskboard")) {
		t.Errorf("an overlay the owner check refuses is still selected:\n%s", body)
	}
	if !strings.Contains(stderr, deskkit.HotfixReasonNotRootOwned) {
		t.Errorf("no NOTICE naming %q:\n%s", deskkit.HotfixReasonNotRootOwned, stderr)
	}
}

// Row 10: one changed byte in the overlay binary retires the hotfix at every layer.
func TestHotfixTampered(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	w.install("deskboard")
	if code, out, errs := w.run("status"); code != 0 || !strings.Contains(out, "valid") {
		t.Fatalf("fresh status rc=%d\n%s\n%s", code, out, errs)
	}
	bin := deskkit.HotfixBinaryPath(w.overlay, "deskboard")
	b, _ := os.ReadFile(bin)
	b[len(b)-2] ^= 0x01
	if err := os.WriteFile(bin, b, 0o755); err != nil {
		t.Fatal(err)
	}
	c, shim := w.cell(t)
	stderr := captureStderrOf(t, func() { c.genShims() })
	if body := readFile(t, shim, "deskboard"); strings.Contains(body, "hotfix_bin") {
		t.Errorf("a tampered hotfix is still selected:\n%s", body)
	}
	if !strings.Contains(stderr, "sha256") {
		t.Errorf("NOTICE does not name sha256:\n%s", stderr)
	}
	code, out, errs := w.run("status")
	if code != 1 || !strings.Contains(out+errs, "sha256") || !strings.Contains(out, deskkit.HotfixReasonTampered) {
		t.Errorf("status rc=%d, want 1 naming sha256:\n%s\n%s", code, out, errs)
	}
}

// Row 11: the source rule is the forge's, never a local ref's; an unreachable forge is
// could-not-check; a lifetime over the ceiling is refused naming it.
func TestHotfixSourceRule(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	w.mainStatus = "diverged"
	code, _, errs := w.run("install", "deskboard", "--source", w.sha)
	if code != hotfixExitRefused || !strings.Contains(errs, deskkit.HotfixReasonNotOnMain) || !strings.Contains(errs, deskkit.HotfixSourceRule) {
		t.Errorf("diverged: rc=%d\n%s", code, errs)
	}
	if len(w.builds) != 0 {
		t.Error("a source off main was built")
	}
	// The local checkout holds the commit on a local branch named main: it must not matter.
	for _, c := range w.compares {
		if strings.Contains(c, "refs/") {
			t.Errorf("a local ref was consulted: %s", c)
		}
	}
	w.mainStatus = "ahead"
	if code, _, errs := w.run("install", "deskboard", "--source", w.sha); code != hotfixExitRefused || !strings.Contains(errs, deskkit.HotfixReasonNotOnMain) {
		t.Errorf("ahead: rc=%d\n%s", code, errs)
	}

	hotfixForge = func() deskkit.HotfixForge {
		return deskkit.HotfixForge{BaseURL: "http://127.0.0.1:1", Repo: deskkit.HotfixRepoSlug}
	}
	code, _, errs = w.run("install", "deskboard", "--source", w.sha)
	if code != hotfixExitUnknown || !strings.Contains(errs, "could-not-check") {
		t.Errorf("unreachable forge: rc=%d, want 6\n%s", code, errs)
	}

	for _, ttl := range []string{"169h", "720h"} {
		code, _, errs := w.run("install", "deskboard", "--source", w.sha, "--ttl", ttl)
		if code != hotfixExitRefused || !strings.Contains(errs, "ceiling") {
			t.Errorf("--ttl %s: rc=%d, want a refusal naming the ceiling\n%s", ttl, code, errs)
		}
	}
	// The ceiling itself is allowed.
	w2 := newHotfixWorld(t, hfBase)
	if code, out, errs := w2.run("install", "deskboard", "--source", w2.sha, "--ttl", "168h"); code != 0 {
		t.Errorf("--ttl 168h: rc=%d\n%s\n%s", code, out, errs)
	}
}

// Row 26: a source whose diff against the pin touches an ineligible path is refused naming the
// entry; the same source with the path removed installs.
func TestHotfixIneligibleSource(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	for _, p := range []string{"migrations/001.sql", "tools/desk/internal/deskkit/pins.go", "tools/desk/internal/deskkit/version.go", "tools/desk/cmd/deskmigrate/main.go"} {
		w.pinFiles = []string{"tools/desk/cmd/deskboard/main.go", p}
		code, _, errs := w.run("install", "deskboard", "--source", w.sha)
		if code != hotfixExitRefused || !strings.Contains(errs, deskkit.HotfixReasonIneligible) || !strings.Contains(errs, p) {
			t.Errorf("%s: rc=%d, want hotfix-ineligible naming the path\n%s", p, code, errs)
		}
	}
	if len(w.builds) != 0 {
		t.Error("an ineligible source was built")
	}
	w.pinFiles = []string{"tools/desk/cmd/deskboard/main.go"}
	if code, out, errs := w.run("install", "deskboard", "--source", w.sha); code != 0 {
		t.Errorf("eligible source: rc=%d\n%s\n%s", code, out, errs)
	}
	w.pinStatus = "behind" // a hotfix never rolls a tool back
	if code, _, errs := w.run("install", "deskboard", "--source", w.sha); code != hotfixExitRefused || !strings.Contains(errs, "not ahead") {
		t.Errorf("behind the pin: rc=%d\n%s", code, errs)
	}
}

// Row 20: with the real, unseamed sudo gate an unprivileged run fails before any clone.
func TestHotfixInstallNeedsSudo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: the gate would not refuse on the effective uid")
	}
	w := newHotfixWorld(t, hfBase)
	hotfixWho = realHotfixWho
	cloned := false
	hotfixCloneURL = func() string { cloned = true; return "file://" + w.srcDir }
	code, _, errs := w.run("install", "deskboard", "--source", w.sha)
	if code == 0 || !strings.Contains(errs, "sudo") || cloned || len(w.builds) != 0 {
		t.Errorf("rc=%d cloned=%v builds=%d, want a refusal naming sudo before any clone\n%s", code, cloned, len(w.builds), errs)
	}
	if code, _, errs := w.run("clear"); code == 0 || !strings.Contains(errs, "sudo") {
		t.Errorf("clear without sudo: rc=%d\n%s", code, errs)
	}
	if _, err := os.Stat(w.overlay); err == nil {
		t.Error("an unprivileged install created the overlay")
	}
}

// Row 24 (offline half): --dry-run decides the source rule against the forge only.
func TestHotfixDryRun(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	hotfixWho = func() (hotfixInvoker, error) { return hotfixInvoker{}, fmt.Errorf("must not be asked") }
	hotfixCloneURL = func() string { t.Error("dry-run cloned"); return "" }
	code, out, errs := w.run("install", "deskboard", "--source", w.sha, "--dry-run")
	if code != 0 || !strings.Contains(out, "dry-run") {
		t.Fatalf("on main: rc=%d\n%s\n%s", code, out, errs)
	}
	w.mainStatus = "diverged"
	if code, _, errs := w.run("install", "deskboard", "--source", w.sha, "--dry-run"); code != hotfixExitRefused || !strings.Contains(errs, deskkit.HotfixReasonNotOnMain) {
		t.Errorf("off main: rc=%d\n%s", code, errs)
	}
	if code, _, _ := w.run("install", "deskboard", "--source", "nothex", "--dry-run"); code != hotfixExitRefused {
		t.Errorf("non-sha source: rc=%d", code)
	}
}

func TestHotfixClear(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	w.install("deskboard", "deskstatus")
	if code, _, errs := w.run("clear", "deskboard"); code != 0 {
		t.Fatalf("clear rc=%d %s", code, errs)
	}
	if _, ok := w.manifest().Entry("deskboard"); ok {
		t.Error("deskboard entry survived clear")
	}
	if _, ok := w.manifest().Entry("deskstatus"); !ok {
		t.Error("clear removed an entry it was not asked to")
	}
	if code, _, _ := w.run("clear"); code != 0 {
		t.Fatal("clear all failed")
	}
	if len(w.manifest().Entries) != 0 {
		t.Error("clear all left entries")
	}
	if _, err := os.Stat(deskkit.HotfixBinaryPath(w.overlay, "deskstatus")); err == nil {
		t.Error("clear all left a binary")
	}
}

// A hotfix-stamped cellctl reports the stamped tag unaltered.
func TestHotfixVersionString(t *testing.T) {
	tag := hfBase + "+hotfix-0123456789ab"
	if got := versionString(tag, nil); got != tag {
		t.Errorf("versionString = %q, want %q", got, tag)
	}
}

// ---- helpers ----------------------------------------------------------------------------

func (w *hotfixWorld) cell(t *testing.T) (*Cell, string) {
	t.Helper()
	dir := filepath.Join(filepath.Dir(w.root), "cell")
	for _, tool := range []string{"deskboard", "deskstatus"} {
		if err := os.WriteFile(filepath.Join(w.pinned, tool), []byte("#!/bin/sh\necho pinned\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := newEnvFromProcess()
	env.Put(deskkit.DeskToolsBinEnv, w.pinned)
	t.Setenv(deskkit.HotfixOverlayEnv, w.overlay)
	c := &Cell{Name: "fixture", Dir: dir, Home: w.home, Repo: w.root, Env: env}
	return c, filepath.Join(dir, "shim")
}

func writePinsFile(t *testing.T, root, pin string) {
	t.Helper()
	body := "desk-tools " + pin + " 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func captureStderrOf(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		_, _ = b.ReadFrom(r)
		done <- b.String()
	}()
	func() {
		defer func() { os.Stderr = old; w.Close() }()
		fn()
	}()
	return <-done
}

// statusgenBuild is a fake build of statusgen: it records the spec and writes a statusgen that
// prints its tag on one line, the shape statusgen's own --version has.
func (w *hotfixWorld) statusgenBuild(base string) {
	hotfixRunBuild = func(s hotfixBuildSpec) error {
		w.builds = append(w.builds, s)
		script := "#!/bin/sh\necho '" + deskkit.HotfixTag(base, w.sha) + "'\n"
		return os.WriteFile(filepath.Join(s.Out, "statusgen"), []byte(script), 0o755)
	}
}

func writeHotfixPins(t *testing.T, root, deskTools, statusgen string) {
	t.Helper()
	sum := " 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"
	body := "desk-tools " + deskTools + sum
	if statusgen != "" {
		body += "statusgen " + statusgen + sum
	}
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// regression: #4405
// TestHotfixInstallStatusgen: `cellctl hotfix install statusgen` builds statusgen's own module
// (statusgen/, package .) over statusgen's own pin line, never the desk-tools line, with
// statusgen's own -X stamp vars; status and shim selection check it against that same line, so
// a moved statusgen pin retires it while the desk-tools pin stands still. statusgen is refused
// in one call with desk tools, and with no statusgen pin line the install is could-not-check.
func TestHotfixInstallStatusgen(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	writeHotfixPins(t, w.root, hfMoved, hfBase)
	w.statusgenBuild(hfBase)

	code, out, errs := w.run("install", "statusgen", "--source", w.sha)
	if code != 0 {
		t.Fatalf("install statusgen rc=%d\n%s\n%s", code, out, errs)
	}
	tag := deskkit.HotfixTag(hfBase, w.sha)
	if !strings.Contains(out, "hotfix installed: statusgen "+tag+" over pin "+hfBase) {
		t.Errorf("install output does not name statusgen over its own pin:\n%s", out)
	}
	if len(w.builds) != 1 {
		t.Fatalf("builds = %d", len(w.builds))
	}
	b := w.builds[0]
	if filepath.Base(b.Dir) != "statusgen" || strings.Contains(b.Dir, filepath.Join("tools", "desk")) {
		t.Errorf("statusgen built in %s, want the source's statusgen module", b.Dir)
	}
	if len(b.Pkgs) != 1 || b.Pkgs[0] != "." {
		t.Errorf("statusgen packages = %v, want [.]", b.Pkgs)
	}
	for _, want := range []string{"-X main.statusgenVersion=" + tag, "-X main.statusgenBuiltAt=", "-X main.statusgenHotfixExpires="} {
		if !strings.Contains(b.Ldflags, want) {
			t.Errorf("statusgen ldflags lack %q: %s", want, b.Ldflags)
		}
	}
	if strings.Contains(b.Ldflags, "internal/deskkit") || strings.Contains(b.Ldflags, "cellctlVersion") {
		t.Errorf("statusgen ldflags carry desk-tools stamp vars: %s", b.Ldflags)
	}
	e, ok := w.manifest().Entry("statusgen")
	if !ok {
		t.Fatal("manifest has no statusgen entry")
	}
	sgSum, _ := deskkit.FileSHA256(filepath.Join(w.srcDir, "statusgen", "go.sum"))
	if e.BasePin != hfBase || e.GoSumSHA256 != sgSum {
		t.Errorf("statusgen entry base_pin %s go_sum %s, want %s and statusgen/go.sum's %s", e.BasePin, e.GoSumSHA256, hfBase, sgSum)
	}

	if code, out, _ := w.run("status"); code != 0 || !strings.Contains(out, "hotfix statusgen "+tag+" over pin "+hfBase) {
		t.Errorf("status against statusgen's own pin rc=%d:\n%s", code, out)
	}
	c, shim := w.cell(t)
	if err := os.WriteFile(filepath.Join(w.pinned, "statusgen"), []byte("#!/bin/sh\necho pinned\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	c.genShims()
	if body := readFile(t, shim, "statusgen"); !strings.Contains(body, `hotfix_bin="`+deskkit.HotfixBinaryPath(w.overlay, "statusgen")+`"`) {
		t.Fatalf("a live statusgen hotfix is not in its shim:\n%s", body)
	}
	writeHotfixPins(t, w.root, hfMoved, hfMoved)
	stderr := captureStderrOf(t, func() { c.genShims() })
	if body := readFile(t, shim, "statusgen"); strings.Contains(body, "hotfix_bin") {
		t.Errorf("after the statusgen pin moved the shim still points at the overlay:\n%s", body)
	}
	if !strings.Contains(stderr, deskkit.HotfixReasonOutlived) {
		t.Errorf("no NOTICE naming the outlived statusgen hotfix:\n%s", stderr)
	}
	if code, out, _ := w.run("status"); code != hotfixExitRefused || !strings.Contains(out, deskkit.HotfixReasonOutlived) {
		t.Errorf("status after the statusgen pin moved rc=%d:\n%s", code, out)
	}

	n := len(w.builds)
	if code, _, errs := w.run("install", "statusgen", "deskboard", "--source", w.sha); code != hotfixExitRefused || !strings.Contains(errs, "installed on its own") {
		t.Errorf("a mixed statusgen + desk-tool install rc=%d:\n%s", code, errs)
	}
	writeHotfixPins(t, w.root, hfBase, "")
	if code, _, errs := w.run("install", "statusgen", "--source", w.sha); code != hotfixExitUnknown || !strings.Contains(errs, "no statusgen pin") {
		t.Errorf("install statusgen with only a desk-tools pin rc=%d (want 6, never the desk-tools line):\n%s", code, errs)
	}
	if len(w.builds) != n {
		t.Errorf("a refused statusgen install still built")
	}
}
