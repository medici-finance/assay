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
// added, removed or changed.
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_PARAMETERS", "")
			f := gittest.NewFixture(t)
			mustOut(t, f, "remote", "add", "origin", origin)
			for _, kv := range tc.env {
				t.Setenv(kv[0], kv[1])
			}
			if got := OriginConfigInjected(); got == tc.ok {
				t.Fatalf("OriginConfigInjected() = %v, want %v", got, !tc.ok)
			}
			repo, err := Open(f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			urls, err := repo.ScopedRemoteURLs("origin")
			if !tc.ok {
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
		})
	}
}
