package deskkit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The echo-form tests. Each pins one row of the full-echo conditions in configecho.go, plus the
// default compact form and the digest's definition. Every test clears the CI and override
// variables first, because a CI runner sets CI=true and would otherwise turn every compact
// expectation into a full one.

// quietEchoEnv clears every variable that forces the full block, and lets the echo use its
// state file as it does outside a test binary.
func quietEchoEnv(t *testing.T) {
	t.Helper()
	prev := configEchoStateful
	configEchoStateful = true
	t.Cleanup(func() { configEchoStateful = prev })
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv(EnvConfigEcho, "")
}

func echoOnce(t *testing.T) string {
	t.Helper()
	var b bytes.Buffer
	EchoEffectiveConfig(&b)
	return b.String()
}

func echoLines(s string) []string { return strings.Split(strings.TrimRight(s, "\n"), "\n") }

// isFullEcho reports whether out is exactly today's full block for the effective config.
func isFullEcho(out string) bool {
	return out == strings.Join(EffectiveConfig().EffectiveConfigLines(), "\n")+"\n"
}

// regression: #2713
func TestEchoCompactAfterSeen(t *testing.T) {
	quietEchoEnv(t)
	withRoster(t, goldenRoster())
	first := echoOnce(t)
	if !isFullEcho(first) {
		t.Fatalf("first run in a fresh config home must print the full block (nothing seen yet):\n%s", first)
	}
	second := echoOnce(t)
	lines := echoLines(second)
	if len(lines) != 1 {
		t.Fatalf("a run whose digest was already seen must print ONE line, got %d:\n%s", len(lines), second)
	}
	sum := ConfigDigest(EffectiveConfig().EffectiveConfigLines())
	for _, want := range []string{
		"assay-config: class=write source=config file ",
		" configured=true digest=sha256:" + sum[:12] + " ",
		"humans=2 bots=6 roles=6 repos=0",
		"(full: ASSAY_CONFIG_ECHO=full)",
	} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("compact line lacks %q:\n%s", want, lines[0])
		}
	}
	if strings.Contains(second, "ada:2001") {
		t.Error("the compact line must not carry the identity lists")
	}
}

// regression: #2713
func TestEchoDigestIsFullBlock(t *testing.T) {
	quietEchoEnv(t)
	withRoster(t, goldenRoster())
	full := echoOnce(t)
	// An independent sha256 of the printed block — the canonical definition.
	if got, want := ConfigDigest(EffectiveConfig().EffectiveConfigLines()), sha256Hex(full); got != want {
		t.Fatalf("digest %s is not sha256 over the printed full block (%s)", got, want)
	}
	state, err := os.ReadFile(configEchoStatePath())
	if err != nil {
		t.Fatalf("the full print did not record the digest: %v", err)
	}
	if strings.TrimSpace(string(state)) != "sha256:"+sha256Hex(full) {
		t.Fatalf("state file holds %q, want the printed block's digest", state)
	}
}

// regression: #2713
func TestEchoFullOnDigestChange(t *testing.T) {
	quietEchoEnv(t)
	home := withRoster(t, goldenRoster())
	echoOnce(t)
	if out := echoOnce(t); len(echoLines(out)) != 1 {
		t.Fatalf("control: an unchanged config must print compact:\n%s", out)
	}
	// Narrow the roster IN THE SAME config home: the state file says the old digest.
	narrowed := goldenRoster()
	narrowed[EnvTrustedLogins] = "ada:2001"
	rewriteRoster(t, home, narrowed)
	out := echoOnce(t)
	if !isFullEcho(out) {
		t.Fatalf("a narrowed roster must print the full block on its first run:\n%s", out)
	}
	if strings.Contains(out, "shared-agent") {
		t.Fatal("the full block names the removed identity — it is not the effective value")
	}
	if out := echoOnce(t); len(echoLines(out)) != 1 {
		t.Fatalf("the run after the change was shown must be compact again:\n%s", out)
	}
}

// regression: #2713
func TestEchoFullOnOverride(t *testing.T) {
	quietEchoEnv(t)
	withRoster(t, goldenRoster())
	echoOnce(t)
	for _, v := range []string{"full", "FULL", "verbose", "1"} {
		t.Setenv(EnvConfigEcho, v)
		if out := echoOnce(t); !isFullEcho(out) {
			t.Errorf("%s=%s must print the full block (an unrecognised value is never quieter):\n%s", EnvConfigEcho, v, out)
		}
	}
	t.Setenv(EnvConfigEcho, "compact")
	if out := echoOnce(t); len(echoLines(out)) != 1 {
		t.Errorf("%s=compact is the default form:\n%s", EnvConfigEcho, out)
	}
	// The forced form neither reads nor writes the state file.
	if err := os.Remove(configEchoStatePath()); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigEcho, "full")
	echoOnce(t)
	if _, err := os.Stat(configEchoStatePath()); !os.IsNotExist(err) {
		t.Fatalf("%s=full must not write the state file (stat err %v)", EnvConfigEcho, err)
	}
}

// An echo nobody sees (a harness hook) must not count as having shown a changed configuration.
// regression: #2713
func TestEchoUnsurfacedNoRecord(t *testing.T) {
	quietEchoEnv(t)
	home := withRoster(t, goldenRoster())
	echoOnce(t)
	before, err := os.ReadFile(configEchoStatePath())
	if err != nil {
		t.Fatal(err)
	}
	narrowed := goldenRoster()
	narrowed[EnvTrustedLogins] = "ada:2001"
	rewriteRoster(t, home, narrowed)
	var b bytes.Buffer
	EchoEffectiveConfigUnsurfaced(&b)
	if !isFullEcho(b.String()) {
		t.Fatalf("the unsurfaced echo is always the full block:\n%s", b.String())
	}
	if after, err := os.ReadFile(configEchoStatePath()); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("the unsurfaced echo rewrote the state file (err %v)", err)
	}
	if out := echoOnce(t); !isFullEcho(out) {
		t.Fatalf("the first SEEN run after a change must print the full block, an unseen echo notwithstanding:\n%s", out)
	}
}

// A child whose stderr Run captures and strips must not count as having shown a change either.
// regression: #2713
func TestEchoRunChildStateless(t *testing.T) {
	quietEchoEnv(t)
	home := withRoster(t, goldenRoster())
	echoOnce(t)
	t.Setenv(EnvConfigEcho, "compact")
	var got []string
	for _, base := range [][]string{nil, {"K=V", EnvConfigEcho + "=compact"}} {
		var cmd *exec.Cmd
		Run(ToolCall{Name: "assay-no-such-binary", Env: base, Start: func(n string, a ...string) *exec.Cmd {
			cmd = exec.Command(n, a...)
			return cmd
		}})
		got = append(got, configEchoIn(cmd.Env))
	}
	for i, v := range got {
		if v != "full" {
			t.Fatalf("Run case %d gave the child %s=%q; a captured child must get the stateless full echo", i, EnvConfigEcho, v)
		}
	}
	// What that child then does: the narrowed config, echoed under the child's environment,
	// leaves the next seen run full.
	narrowed := goldenRoster()
	narrowed[EnvTrustedLogins] = "ada:2001"
	rewriteRoster(t, home, narrowed)
	t.Setenv(EnvConfigEcho, got[0])
	echoOnce(t)
	t.Setenv(EnvConfigEcho, "")
	if out := echoOnce(t); !isFullEcho(out) {
		t.Fatalf("the first SEEN run after a change must print the full block, a captured child notwithstanding:\n%s", out)
	}
}

// configEchoIn is the value os/exec gives the child for ASSAY_CONFIG_ECHO: the last one.
func configEchoIn(env []string) string {
	v := "(unset)"
	for _, kv := range env {
		if val, ok := strings.CutPrefix(kv, EnvConfigEcho+"="); ok {
			v = val
		}
	}
	return v
}

// The state file is opened no-follow: a symlink planted on it is a bad state, never followed.
// regression: #2713
func TestEchoStateNoFollow(t *testing.T) {
	quietEchoEnv(t)
	withRoster(t, goldenRoster())
	echoOnce(t)
	path := configEchoStatePath()
	target := path + ".real"
	if err := os.Rename(path, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	if _, err := readConfigEchoState(path); err == nil {
		t.Fatal("readConfigEchoState followed a symlink on the state path")
	}
	if out := echoOnce(t); !isFullEcho(out) {
		t.Fatalf("a symlinked state file must print the full block:\n%s", out)
	}
}

// A full-echo condition (#2713): green before the change too, since the old echo was always
// full; it holds the new default from ever making this case quieter.
func TestEchoFullInCI(t *testing.T) {
	for _, kv := range [][2]string{{"CI", "true"}, {"CI", "1"}, {"GITHUB_ACTIONS", "true"}} {
		t.Run(kv[0]+"="+kv[1], func(t *testing.T) {
			quietEchoEnv(t)
			withRoster(t, goldenRoster())
			echoOnce(t)
			t.Setenv(kv[0], kv[1])
			// The roster loader is class-cached; the echo condition is read per call.
			if out := echoOnce(t); !isFullEcho(out) {
				t.Fatalf("in CI the full block must print every run:\n%s", out)
			}
		})
	}
	t.Run("no-state-written", func(t *testing.T) {
		quietEchoEnv(t)
		withRoster(t, goldenRoster())
		t.Setenv("CI", "true")
		echoOnce(t)
		if _, err := os.Stat(configEchoStatePath()); !os.IsNotExist(err) {
			t.Fatalf("a CI run must not write the state file (stat err %v)", err)
		}
	})
}

// A full-echo condition (#2713): green before the change too, since the old echo was always
// full; it holds the new default from ever making this case quieter.
func TestEchoFullOnProblems(t *testing.T) {
	quietEchoEnv(t)
	// A refused roster in a WRITABLE config home: the state file could record a digest, so
	// only the problem rule keeps the second run full.
	vals := goldenRoster()
	vals["ASSAY_EXAMPLE_UNKNOWN"] = "x"
	withRoster(t, vals)
	for i := 0; i < 2; i++ {
		out := echoOnce(t)
		if !isFullEcho(out) || !strings.Contains(out, "assay-config: REFUSED — ") {
			t.Fatalf("run %d: a refused roster must print the full block with its REFUSED line:\n%s", i, out)
		}
	}
	// The absent-roster refusal too.
	withNoRoster(t)
	if out := echoOnce(t); !isFullEcho(out) || !strings.Contains(out, "assay-config: REFUSED — ") {
		t.Fatalf("an absent roster must print the full block with its REFUSED line:\n%s", out)
	}
}

// A full-echo condition (#2713): green before the change too, since the old echo was always
// full; it holds the new default from ever making this case quieter.
func TestEchoFullOnUnknownKeys(t *testing.T) {
	quietEchoEnv(t)
	vals := goldenRoster()
	vals["EXAMPLE_COTENANT_KEY"] = "x"
	withRoster(t, vals)
	for i := 0; i < 2; i++ {
		out := echoOnce(t)
		if !isFullEcho(out) || !strings.Contains(out, "unrecognised-keys=EXAMPLE_COTENANT_KEY") {
			t.Fatalf("run %d: an unrecognised key must print the full block every run:\n%s", i, out)
		}
	}
}

// regression: #2713
func TestEchoTestBinaryStateless(t *testing.T) {
	quietEchoEnv(t)
	withRoster(t, goldenRoster())
	echoOnce(t)
	configEchoStateful = false // what every other test binary runs with
	if err := os.Remove(configEchoStatePath()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if out := echoOnce(t); !isFullEcho(out) {
			t.Fatalf("run %d: a test binary must print the full block:\n%s", i, out)
		}
	}
	if _, err := os.Stat(configEchoStatePath()); !os.IsNotExist(err) {
		t.Fatalf("a test binary must not write the state file (stat err %v)", err)
	}
}

// A full-echo condition (#2713): green before the change too, since the old echo was always
// full; it holds the new default from ever making this case quieter.
func TestEchoFullOnBadState(t *testing.T) {
	quietEchoEnv(t)
	withRoster(t, goldenRoster())
	echoOnce(t)
	path := configEchoStatePath()
	for name, body := range map[string]string{
		"malformed": "not a digest\n",
		"truncated": "sha256:abc\n",
		"oversized": strings.Repeat("x", configEchoStateMax+10),
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if out := echoOnce(t); !isFullEcho(out) {
			t.Errorf("%s state file must print the full block:\n%s", name, out)
		}
	}
}

// A full-echo condition (#2713): green before the change too, since the old echo was always
// full; it holds the new default from ever making this case quieter.
func TestEchoFullOnUnwritable(t *testing.T) {
	quietEchoEnv(t)
	withRoster(t, goldenRoster())
	// A directory squatting on the state path: unreadable as a file and not replaceable.
	if err := os.Mkdir(configEchoStatePath(), 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if out := echoOnce(t); !isFullEcho(out) {
			t.Fatalf("run %d: an unwritable state file must keep every run full:\n%s", i, out)
		}
	}
	// And a state path whose directory does not exist is never created.
	var b bytes.Buffer
	missing := filepath.Join(t.TempDir(), "absent", configEchoStateName)
	writeConfigEcho(&b, EffectiveConfig(), missing)
	if !isFullEcho(b.String()) {
		t.Fatalf("a missing state directory must print full:\n%s", b.String())
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatal("the echo created the config-home directory; it must never create it")
	}
}

// rewriteRoster replaces the roster under an existing withRoster home and reloads.
func rewriteRoster(t *testing.T, home string, vals map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + vals[k] + "\n")
	}
	file := filepath.Join(home, ".config", "assay", "roster.env")
	if err := os.WriteFile(file, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := secureTestRosterPaths(filepath.Dir(file), file); err != nil {
		t.Fatal(err)
	}
	ReloadConfig()
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
