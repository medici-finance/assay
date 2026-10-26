package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The verify-runner image's stage-source entrypoint
// (images/verify-runner/stage-source.sh) checks the mounted installation token
// in two places: the custody check before any clone, and the credential helper
// git calls at the moment it needs the password. Both must accept the token
// shapes GitHub issues (dotted segments, base64url '-' and '_', a few hundred
// characters) and refuse anything that is not a bounded run of that charset.
//
// No network is used: a stub `git` first on PATH stands in for the fetch. It
// runs the helper exactly as git would (`sh -c '<helper> get'`, the request on
// stdin), records what the helper answered, and fails the fetch.

const stageSourceEntrypoint = "../images/verify-runner/stage-source.sh"

const stageSourceRepo = "example-org/one"

const stageSourceSHA = "0123456789abcdef0123456789abcdef01234567"

// tokenSeg is n characters of the base64url alphabet, '-' and '_' included.
func tokenSeg(n int) string {
	return strings.Repeat("aB3-x_Z9", n/8+1)[:n]
}

type tokenCase struct {
	name string
	tok  string
	ok   bool
}

// legacyToken is the underscore-and-alphanumeric shape the check accepted
// before this fix; it is assembled here so no token-shaped literal is
// committed.
func legacyToken() string {
	return "gh" + "s_" + strings.Repeat("Ab3", 12)
}

func stageTokenCases() []tokenCase {
	bad := func(name, ch string) tokenCase {
		return tokenCase{name: name, tok: "abc" + ch + "def"}
	}
	return []tokenCase{
		{name: "legacy", tok: legacyToken(), ok: true},
		{name: "dotted-390", tok: tokenSeg(36) + "." + tokenSeg(300) + "." + tokenSeg(52), ok: true},
		{name: "exact-2048", tok: tokenSeg(1000) + "." + tokenSeg(1047), ok: true},
		{name: "empty", tok: ""},
		bad("space", " "),
		bad("tab", "\t"),
		bad("newline", "\n"),
		bad("cr", "\r"),
		bad("equals", "="),
		bad("slash", "/"),
		bad("plus", "+"),
		bad("colon", ":"),
		bad("at", "@"),
		bad("percent", "%"),
		bad("backslash", `\`),
		{name: "over-2048", tok: strings.Repeat("Ab3_", 512) + "x"},
	}
}

// stageFixture is one run's credential mount, scratch, cache and stub git.
type stageFixture struct {
	cred, tmp, cache, bin, swap, out, code string
}

func shq(t *testing.T, s string) string {
	t.Helper()
	if strings.ContainsAny(s, "'\n") {
		t.Fatalf("fixture path %q cannot be single-quoted", s)
	}
	return "'" + s + "'"
}

func newStageFixture(t *testing.T, custodyTok string) stageFixture {
	t.Helper()
	root := t.TempDir()
	fx := stageFixture{
		cred:  filepath.Join(root, "cred"),
		tmp:   filepath.Join(root, "tmp"),
		cache: filepath.Join(root, "cache"),
		bin:   filepath.Join(root, "bin"),
		swap:  filepath.Join(root, "swap"),
		out:   filepath.Join(root, "helper.out"),
		code:  filepath.Join(root, "helper.code"),
	}
	for _, d := range []string{fx.cred, fx.tmp, fx.cache, fx.bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Unix()
	putFile(t, filepath.Join(fx.cred, "token"), custodyTok)
	putFile(t, filepath.Join(fx.cred, "issued-at"), strconv.FormatInt(now, 10)+"\n")
	putFile(t, filepath.Join(fx.cred, "expires-at"), strconv.FormatInt(now+1800, 10)+"\n")
	stub := "#!/bin/bash\n" +
		"if [ \"${1:-}\" = init ]; then mkdir -p \"$3\"; exit 0; fi\n" +
		"helper=\n" +
		"for a in \"$@\"; do case $a in 'credential.helper=!'*) helper=${a#credential.helper=!} ;; esac; done\n" +
		"for a in \"$@\"; do\n" +
		"  [ \"$a\" = fetch ] || continue\n" +
		"  if [ -f " + shq(t, fx.swap) + " ]; then cp " + shq(t, fx.swap) + " " + shq(t, filepath.Join(fx.cred, "token")) + "; fi\n" +
		"  printf 'protocol=https\\nhost=github.com\\npath=%s\\n\\n' " + shq(t, stageSourceRepo+".git") +
		" | /bin/sh -c \"$helper get\" > " + shq(t, fx.out) + "\n" +
		"  echo $? > " + shq(t, fx.code) + "\n" +
		"  exit 128\n" +
		"done\n" +
		"exit 99\n"
	putFile(t, filepath.Join(fx.bin, "git"), stub)
	if err := os.Chmod(filepath.Join(fx.bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return fx
}

// run starts the entrypoint at script with the stub git first on PATH and
// none of the caller's variables.
func (fx stageFixture) run(t *testing.T, script string) (int, string) {
	t.Helper()
	cmd := exec.Command("bash", script, "--repo="+stageSourceRepo, "--sha="+stageSourceSHA)
	cmd.Env = []string{
		"PATH=" + fx.bin + string(os.PathListSeparator) + "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + fx.tmp,
		"VR_CRED_DIR=" + fx.cred,
		"VR_CACHE_DIR=" + fx.cache,
		"VR_TMP_DIR=" + fx.tmp,
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("start %s: %v", script, err)
	}
	return code, stderr.String()
}

func requireStageBash(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the entrypoint is a bash script")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("SKIP (could-not-check, not a pass): no bash on this host")
	}
}

const custodyRefusal = "source custody is malformed (token): no clone was attempted"

// custodyFailures runs every case through the custody check of script and
// returns one line per case it decided wrongly.
func custodyFailures(t *testing.T, script string) []string {
	t.Helper()
	var bad []string
	for _, c := range stageTokenCases() {
		fx := newStageFixture(t, c.tok)
		code, stderr := fx.run(t, script)
		_, ferr := os.Stat(fx.code)
		fetched := ferr == nil
		refused := code == 1 && strings.Contains(stderr, custodyRefusal)
		switch {
		case c.ok && (refused || !fetched):
			bad = append(bad, c.name+": custody refused an accepted token: exit "+strconv.Itoa(code)+" "+strconv.Quote(stderr))
		case !c.ok && (!refused || fetched):
			bad = append(bad, c.name+": custody admitted a refused token: exit "+strconv.Itoa(code)+" "+strconv.Quote(stderr))
		}
	}
	return bad
}

// helperFailures passes custody with the legacy token, then swaps each case
// onto the mount before git calls the helper, and returns one line per case
// the helper answered wrongly.
func helperFailures(t *testing.T, script string) []string {
	t.Helper()
	var bad []string
	for _, c := range stageTokenCases() {
		fx := newStageFixture(t, legacyToken())
		putFile(t, fx.swap, c.tok)
		code, stderr := fx.run(t, script)
		if _, err := os.Stat(fx.code); err != nil {
			bad = append(bad, c.name+": the fetch was never reached: exit "+strconv.Itoa(code)+" "+strconv.Quote(stderr))
			continue
		}
		hcode := strings.TrimSpace(getFile(t, fx.code))
		out := getFile(t, fx.out)
		want := "username=x-access-token\npassword=" + c.tok + "\n"
		switch {
		case c.ok && (hcode != "0" || out != want):
			bad = append(bad, c.name+": helper refused an accepted token: exit "+hcode)
		case !c.ok && (hcode == "0" || out != ""):
			bad = append(bad, c.name+": helper answered a refused token: exit "+hcode)
		}
	}
	return bad
}

// regression: #4353
// TestStageSourceCustodyToken: the custody check admits the dotted and
// exactly-2048 token shapes and refuses every other character class and any
// longer value before git is started.
func TestStageSourceCustodyToken(t *testing.T) {
	requireStageBash(t)
	for _, f := range custodyFailures(t, stageSourceEntrypoint) {
		t.Error(f)
	}
}

// regression: #4353
// TestStageSourceHelperToken: the credential helper answers with the dotted
// and exactly-2048 token shapes and stays silent on every other character
// class and any longer value, though custody already passed.
func TestStageSourceHelperToken(t *testing.T) {
	requireStageBash(t)
	for _, f := range helperFailures(t, stageSourceEntrypoint) {
		t.Error(f)
	}
}

// regression: #4353
// TestStageSourceTokenMutants: each site's check is pinned on its own. A copy
// of the entrypoint with one site's check reverted to the pre-fix charset, or
// with one site's length bound dropped, must fail that site's cases and leave
// the other site's cases passing.
func TestStageSourceTokenMutants(t *testing.T) {
	requireStageBash(t)
	raw, err := os.ReadFile(stageSourceEntrypoint)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	const (
		helperFix   = `[ "${#tok}" -le 2048 ] && [[ $tok =~ ^[A-Za-z0-9_.-]+$ ]] || exit 1`
		custodyFix  = `[ "${#TOK}" -le 2048 ] && [[ $TOK =~ ^[A-Za-z0-9_.-]+$ ]] || die`
		helperOld   = `[[ $tok =~ ^[A-Za-z0-9_]+$ ]] || exit 1`
		custodyOld  = `[[ $TOK =~ ^[A-Za-z0-9_]+$ ]] || die`
		helperLong  = `[[ $tok =~ ^[A-Za-z0-9_.-]+$ ]] || exit 1`
		custodyLong = `[[ $TOK =~ ^[A-Za-z0-9_.-]+$ ]] || die`
	)
	mutants := []struct {
		name, before, after string
		helper              bool // which site's cases must fail
		want                []string
	}{
		{"helper-charset", helperFix, helperOld, true, []string{"dotted-390", "exact-2048", "over-2048"}},
		{"custody-charset", custodyFix, custodyOld, false, []string{"dotted-390", "exact-2048", "over-2048"}},
		{"helper-length", helperFix, helperLong, true, []string{"over-2048"}},
		{"custody-length", custodyFix, custodyLong, false, []string{"over-2048"}},
	}
	for _, m := range mutants {
		if n := strings.Count(src, m.before); n != 1 {
			t.Fatalf("%s: mutation anchor found %d times, want 1", m.name, n)
		}
		script := filepath.Join(t.TempDir(), "stage-source.sh")
		putFile(t, script, strings.Replace(src, m.before, m.after, 1))
		hit, other := custodyFailures(t, script), helperFailures(t, script)
		if m.helper {
			hit, other = other, hit
		}
		for _, w := range m.want {
			found := false
			for _, f := range hit {
				found = found || strings.HasPrefix(f, w+": ")
			}
			if !found {
				t.Errorf("%s: case %s survived the mutant; failures were %q", m.name, w, hit)
			}
		}
		if len(hit) != len(m.want) {
			t.Errorf("%s: %d cases failed, want exactly %v: %q", m.name, len(hit), m.want, hit)
		}
		if len(other) != 0 {
			t.Errorf("%s: the other site's cases failed too: %q", m.name, other)
		}
	}
}
