package deskkit

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// The help-path echo tests (medici-finance/assay#3754). A help screen is not a run, so a verb that
// OPTS IN through EchoConfigHelpNotice prints no resolved roster VALUE on it. A verb that does not opt in
// keeps the full echo whatever its argv looks like. The roster here is synthetic and every value is
// distinctive, so a single substring hit is a leak.

// helpRosterValues are the distinctive synthetic values the help path must never print.
var helpRosterValues = []string{"qzhelpbless", "9101", "qzhelptrust", "9102", "qz-help-bot", "9103", "qzorg/qzrepo", "qzknobvalue"}

// helpRoster is a CONFIGURED roster (so the echo carries its values) plus one co-tenant key
// outside the ASSAY_ namespace, which the echo reports as unrecognised.
func helpRoster() map[string]string {
	return map[string]string{
		EnvBlessLogin:      "qzhelpbless:9101",
		EnvTrustedLogins:   "qzhelpbless:9101,qzhelptrust:9102",
		EnvTrustedBotSlugs: "reviewer=qz-help-bot:9103",
		EnvAllowedRepos:    "qzorg/qzrepo:ci:private",
		"QZ_HELP_KNOB":     "qzknobvalue",
	}
}

// withArgs points os.Args at a tool invocation for one test.
func withArgs(t *testing.T, args ...string) {
	t.Helper()
	prev := os.Args
	os.Args = append([]string{"desktool"}, args...)
	t.Cleanup(func() { os.Args = prev })
}

// helpShapes are help-shaped argv lists. A verb that does not opt in may still act on any of them.
var helpShapes = [][]string{
	{"--help"}, {"-h"}, {"-help"}, {"help"}, {"--help", "finish"}, {"help", "create"},
	{"finish", "--help"}, {"create", "-h"}, {"review", "-help"},
}

func assertNoHelpValues(t *testing.T, out string) {
	t.Helper()
	for _, v := range helpRosterValues {
		if strings.Contains(out, v) {
			t.Errorf("the help-screen echo printed the roster value %q:\n%s", v, out)
		}
	}
}

// regression: #3754
func TestEchoForHelpPrintsNoValues(t *testing.T) {
	t.Setenv(EnvConfigEcho, "full") // the loudest form: a help screen must still print no value
	withRoster(t, helpRoster())
	if !EffectiveConfig().Configured() {
		t.Fatalf("fixture roster must load configured, or the echo carries no value to leak: %v",
			EffectiveConfig().Problems)
	}
	var b bytes.Buffer
	EchoConfigHelpNotice(&b)
	assertNoHelpValues(t, b.String())
	// The diagnostic that remains names the KEY of the unrecognised entry, never its value.
	if !strings.Contains(b.String(), "QZ_HELP_KNOB") {
		t.Errorf("the keys-only diagnostic must still name the unrecognised key:\n%s", b.String())
	}
}

// regression: #3754
func TestEchoForHelpRefusedKeysOnly(t *testing.T) {
	// A REFUSED roster is never quieter than it was: the help screen still says so, by count.
	t.Setenv(EnvConfigEcho, "full")
	vals := helpRoster()
	vals["ASSAY_QZ_HELP_KNOB"] = "qzknobvalue"
	withRoster(t, vals)
	if len(EffectiveConfig().Problems) == 0 {
		t.Fatal("fixture must carry a refused key")
	}
	var b bytes.Buffer
	EchoConfigHelpNotice(&b)
	assertNoHelpValues(t, b.String())
	if !strings.Contains(b.String(), "REFUSED") {
		t.Errorf("a refused configuration must still be reported:\n%s", b.String())
	}
}

// regression: #3754
func TestEchoIgnoresHelpArgv(t *testing.T) {
	// The echo a verb calls WITHOUT opting in never reads argv. Some verbs act under a help-shaped
	// argv (a hook that ignores argv, a verb that parses positionals), and hiding the echo there
	// hides it from a run that acts. Only EchoConfigHelpNotice, called under the verb's own predicate, may
	// leave the values out.
	t.Setenv(EnvConfigEcho, "full")
	withRoster(t, helpRoster())
	for _, args := range helpShapes {
		withArgs(t, args...)
		var b bytes.Buffer
		EchoEffectiveConfig(&b)
		if !isFullEcho(b.String()) || !strings.Contains(b.String(), "qzhelptrust") {
			t.Errorf("argv %q: EchoEffectiveConfig must keep the full block:\n%s", args, b.String())
		}
		b.Reset()
		EchoEffectiveConfigUnsurfaced(&b)
		if !strings.Contains(b.String(), "qzhelptrust") {
			t.Errorf("argv %q: EchoEffectiveConfigUnsurfaced must keep the full block:\n%s", args, b.String())
		}
	}
}

// regression: #3754
func TestUsageHelpShapes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--help"}, true},
		{[]string{"-h", "x"}, true},
		{[]string{"help"}, true},
		{[]string{"help", "create"}, true},
		{[]string{"-help"}, false},
		{[]string{"finish", "--help"}, true},
		{[]string{"finish", "review", "--help"}, false},
		{[]string{"create", "--title", "--help"}, false},
		{[]string{"create", "--", "--help"}, false},
		{[]string{"create", "--help", "--json"}, false},
	} {
		if got := UsageHelp(tc.args); got != tc.want {
			t.Errorf("UsageHelp(%q) = %t, want %t", tc.args, got, tc.want)
		}
	}
}
