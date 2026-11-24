package gitcore

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// grepPinEnv spells the admitted row's grep pins literally, so the test does
// not read the set it checks.
var grepPinEnv = [][2]string{
	{"GIT_CONFIG_COUNT", "2"},
	{"GIT_CONFIG_KEY_0", "grep.patternType"}, {"GIT_CONFIG_VALUE_0", "default"},
	{"GIT_CONFIG_KEY_1", "grep.extendedRegexp"}, {"GIT_CONFIG_VALUE_1", "false"},
}

func withPin(i int, v string) [][2]string {
	out := append([][2]string(nil), grepPinEnv...)
	out[i][1] = v
	return out
}

// TestScopedRemoteQualifiedConfig: a scoped remote read accepts exactly
// GIT_CONFIG_COUNT=0 and the admitted row's two grep pins, and still refuses
// any other command-priority config, including a pin set with one entry
// added, removed, changed or reordered, and GIT_CONFIG_PARAMETERS or
// GIT_CONFIG beside either qualified form.
// regression: #7007
func TestScopedRemoteQualifiedConfig(t *testing.T) {
	const origin = "https://example.com/example-org/example-repo.git"
	const foreign = "https://example.com/foreign/example-repo.git"
	for _, tc := range []struct {
		name string
		env  [][2]string
		ok   bool
	}{
		{"count-zero", [][2]string{{"GIT_CONFIG_COUNT", "0"}}, true},
		{"admitted-grep-pins", grepPinEnv, true},
		{"one-grep-pin", [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "grep.patternType"}, {"GIT_CONFIG_VALUE_0", "default"}}, false},
		{"pins-plus-url", append(withPin(0, "3"), [2]string{"GIT_CONFIG_KEY_2", "remote.origin.url"}, [2]string{"GIT_CONFIG_VALUE_2", foreign}), false},
		{"pin-value-changed", withPin(2, "fixed"), false},
		{"pin-key-changed", append(withPin(3, "remote.origin.url"), [2]string{"GIT_CONFIG_VALUE_1", foreign}), false},
		{"pin-key-case", withPin(1, "Grep.patternType"), false},
		{"count-padded", withPin(0, "02"), false},
		{"count-zero-parameters", [][2]string{{"GIT_CONFIG_COUNT", "0"}, {"GIT_CONFIG_PARAMETERS", "'remote.origin.url'='" + foreign + "'"}}, false},
		{"pins-parameters", append(withPin(0, "2"), [2]string{"GIT_CONFIG_PARAMETERS", "'remote.origin.url'='" + foreign + "'"}), false},
		{"count-zero-git-config", [][2]string{{"GIT_CONFIG_COUNT", "0"}, {"GIT_CONFIG", "injected.config"}}, false},
		{"pins-git-config", append(withPin(0, "2"), [2]string{"GIT_CONFIG", "injected.config"}), false},
		{"pins-reordered", [][2]string{
			{"GIT_CONFIG_COUNT", "2"},
			{"GIT_CONFIG_KEY_0", "grep.extendedRegexp"}, {"GIT_CONFIG_VALUE_0", "false"},
			{"GIT_CONFIG_KEY_1", "grep.patternType"}, {"GIT_CONFIG_VALUE_1", "default"},
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkScopedRemote(t, origin, tc.env, tc.ok)
		})
	}
}

// urlKeys are command-priority keys that can change the URL list native git
// resolves for a remote, or the repository it reads. Each still refuses alone
// and beside core.quotePath.
var urlKeys = [][2]string{
	{"url.https://example.com/foreign/.insteadOf", "https://example.com/example-org/"},
	{"remote.origin.url", "https://example.com/foreign/example-repo.git"},
	{"remote.origin.pushurl", "https://example.com/foreign/example-repo.git"},
	{"include.path", "injected.config"},
	{"includeIf.onbranch:main.path", "injected.config"},
	{"core.sshCommand", "ssh -F injected"},
	{"credential.helper", "store"},
	{"http.extraHeader", "X-Injected: 1"},
}

// quotePathEnv spells the qualified core.quotePath set literally.
var quotePathEnv = [][2]string{
	{"GIT_CONFIG_COUNT", "1"},
	{"GIT_CONFIG_KEY_0", "core.quotePath"}, {"GIT_CONFIG_VALUE_0", "false"},
}

// TestQuotePathQualifiedConfig: core.quotePath=false alone is qualified, and
// every other shape that carries it is not: another value, another spelling,
// a URL-rewriting key beside it in either order, the grep pins beside it, or
// GIT_CONFIG_PARAMETERS beside it. Each URL-rewriting key also still refuses
// alone, so the allowlist stays an allowlist.
// regression: #7533
func TestQuotePathQualifiedConfig(t *testing.T) {
	const origin = "https://example.com/example-org/example-repo.git"
	type tcase struct {
		name string
		env  [][2]string
		ok   bool
	}
	cases := []tcase{
		{"quotepath-alone", quotePathEnv, true},
		{"quotepath-true", [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "core.quotePath"}, {"GIT_CONFIG_VALUE_0", "true"}}, false},
		{"quotepath-lowercase", [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "core.quotepath"}, {"GIT_CONFIG_VALUE_0", "false"}}, false},
		{"quotepath-prefix", [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "core.quotePathX"}, {"GIT_CONFIG_VALUE_0", "false"}}, false},
		{"quotepath-count-two", [][2]string{{"GIT_CONFIG_COUNT", "2"}, {"GIT_CONFIG_KEY_0", "core.quotePath"}, {"GIT_CONFIG_VALUE_0", "false"}}, false},
		{"quotepath-grep-pins", [][2]string{
			{"GIT_CONFIG_COUNT", "3"},
			{"GIT_CONFIG_KEY_0", "core.quotePath"}, {"GIT_CONFIG_VALUE_0", "false"},
			{"GIT_CONFIG_KEY_1", "grep.patternType"}, {"GIT_CONFIG_VALUE_1", "default"},
			{"GIT_CONFIG_KEY_2", "grep.extendedRegexp"}, {"GIT_CONFIG_VALUE_2", "false"},
		}, false},
		{"quotepath-parameters", append(append([][2]string(nil), quotePathEnv...),
			[2]string{"GIT_CONFIG_PARAMETERS", "'remote.origin.url'='https://example.com/foreign/example-repo.git'"}), false},
	}
	for _, kv := range urlKeys {
		cases = append(cases,
			tcase{"alone/" + kv[0], [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", kv[0]}, {"GIT_CONFIG_VALUE_0", kv[1]}}, false},
			tcase{"after-quotepath/" + kv[0], [][2]string{
				{"GIT_CONFIG_COUNT", "2"},
				{"GIT_CONFIG_KEY_0", "core.quotePath"}, {"GIT_CONFIG_VALUE_0", "false"},
				{"GIT_CONFIG_KEY_1", kv[0]}, {"GIT_CONFIG_VALUE_1", kv[1]},
			}, false},
			tcase{"before-quotepath/" + kv[0], [][2]string{
				{"GIT_CONFIG_COUNT", "2"},
				{"GIT_CONFIG_KEY_0", kv[0]}, {"GIT_CONFIG_VALUE_0", kv[1]},
				{"GIT_CONFIG_KEY_1", "core.quotePath"}, {"GIT_CONFIG_VALUE_1", "false"},
			}, false},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkScopedRemote(t, origin, tc.env, tc.ok)
		})
	}
}

// checkScopedRemote sets env over a fixture whose origin is origin and checks
// that OriginConfigInjected and ScopedRemoteURLs both agree with ok; on ok,
// native git must resolve the same single URL.
func checkScopedRemote(t *testing.T, origin string, env [][2]string, ok bool) {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_PARAMETERS", "")
	t.Setenv("GIT_CONFIG", "") // restored after the test
	os.Unsetenv("GIT_CONFIG")  // git config would write to an empty path
	f := gittest.NewFixture(t)
	mustOut(t, f, "remote", "add", "origin", origin)
	for _, kv := range env {
		t.Setenv(kv[0], kv[1])
	}
	if got := OriginConfigInjected(); got == ok {
		t.Fatalf("OriginConfigInjected() = %v, want %v", got, !ok)
	}
	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	urls, err := repo.ScopedRemoteURLs("origin")
	if !ok {
		if !errors.Is(err, ErrRemoteScopeUnmerged) {
			t.Fatalf("command-priority config accepted: urls=%q err=%v", urls, err)
		}
		return
	}
	if err != nil || len(urls) != 1 || urls[0] != origin {
		t.Fatalf("qualified config refused: urls=%q err=%v", urls, err)
	}
	// Native control: git resolves the same single URL under this env.
	out, err := exec.Command("git", "-C", f.Dir, "remote", "get-url", "--all", "origin").Output()
	if err != nil || strings.TrimSpace(string(out)) != origin {
		t.Fatalf("native control: %q err=%v", out, err)
	}
}
