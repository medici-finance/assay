package desk_test

// The help-echo guard across verbs (medici-finance/assay#3754). A help screen prints no resolved
// roster value, but only on a verb that OPTS IN, by calling deskkit.EchoConfigHelpNotice under the
// same predicate its own dispatch answers with its usage screen before doing any work. Every other
// verb keeps the full echo whatever its argv looks like, because some verbs do real work under a
// help-shaped argv: writeguard never reads argv, deskversion runs its marker read, untrustcorpus
// loads its corpus. Hiding the echo there would hide it from a run that acts.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// helpOptIn is the closed set of verbs that call deskkit.EchoConfigHelpNotice. A verb joins it
// only with its help path proven here to return usage, exit 0, and print no value.
var helpOptIn = []string{"deskpost", "deskpr", "deskreply"}

// helpWorkers are verbs that do work under a help-shaped argv; they must keep the full echo.
var helpWorkers = []string{"deskversion", "untrustcorpus", "writeguard"}

// helpEchoRoster is synthetic; every value is distinctive, so one substring hit is a leak.
const helpEchoRoster = `ASSAY_BLESS_LOGIN=qzverbbless:9301
ASSAY_TRUSTED_LOGINS=qzverbbless:9301,qzverbtrust:9302
ASSAY_TRUSTED_BOT_SLUGS=reviewer=qz-verb-bot:9303
ASSAY_ALLOWED_REPOS=qzorg/qzverbrepo:ci:private
`

var helpEchoValues = []string{"qzverbbless", "qzverbtrust", "qz-verb-bot", "qzverbrepo"}

var helpNoticeCall = regexp.MustCompile(`\bdeskkit\.EchoConfigHelpNotice\(`)

// regression: #3754
func TestHelpEchoOptInIsDeclared(t *testing.T) {
	// A verb cannot opt in silently: the callers of EchoConfigHelpNotice are exactly helpOptIn,
	// so a new one has to join the behavioural proof below.
	files, err := filepath.Glob(filepath.Join("cmd", "*", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if helpNoticeCall.Match(src) {
			seen[filepath.Base(filepath.Dir(f))] = true
		}
	}
	var got []string
	for v := range seen {
		got = append(got, v)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(helpOptIn, ",") {
		t.Fatalf("the verbs calling deskkit.EchoConfigHelpNotice are %v; the proven opt-in set is %v", got, helpOptIn)
	}
}

// regression: #3754
func TestHelpEchoAcrossVerbs(t *testing.T) {
	bin := t.TempDir()
	pkgs := []string{}
	for _, v := range append(append([]string{}, helpOptIn...), helpWorkers...) {
		pkgs = append(pkgs, "./cmd/"+v)
	}
	build := exec.Command("go", append([]string{"build", "-o", bin + string(os.PathSeparator)}, pkgs...)...)
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("building the verbs: %v\n%s", err, out)
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(helpEchoRoster), 0o600); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	run := func(verb string, args ...string) (string, string, int) {
		cmd := exec.Command(filepath.Join(bin, verb), args...)
		cmd.Dir = cwd
		cmd.Stdin = strings.NewReader("")
		cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME=", "ASSAY_CONFIG_ECHO=full",
			"DESK_LOOP=pr-review-desk")
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		code := 0
		if err := cmd.Run(); err != nil {
			ee, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("running %s: %v", verb, err)
			}
			code = ee.ExitCode()
		}
		return out.String(), errb.String(), code
	}

	// An opted-in verb answers each shape with its usage, exit 0, and no value on either stream.
	for _, verb := range helpOptIn {
		for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}, {"--help", "x"}, {"x", "--help"}, {"x", "-h"}} {
			stdout, stderr, code := run(verb, args...)
			if code != 0 {
				t.Errorf("%s %q: help must exit 0, got %d\n%s", verb, args, code, stderr)
			}
			if !strings.Contains(strings.ToLower(stderr), "usage") {
				t.Errorf("%s %q: no usage screen on stderr:\n%s", verb, args, stderr)
			}
			for _, v := range helpEchoValues {
				if strings.Contains(stdout+stderr, v) {
					t.Errorf("%s %q: printed the roster value %q:\nstdout:\n%s\nstderr:\n%s", verb, args, v, stdout, stderr)
				}
			}
		}
	}

	// A verb that works under a help-shaped argv keeps the full echo on every one of them. The
	// no-argument run is the control: it proves the synthetic roster loads and is echoed at all.
	for _, verb := range helpWorkers {
		for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"-help"}, {"help"}, {"x", "--help"}, {"x", "-h"}} {
			_, stderr, _ := run(verb, args...)
			if !strings.Contains(stderr, "qzverbtrust:9302") {
				t.Errorf("%s %q: the effective-config echo is missing, but this verb does work here:\n%s", verb, args, stderr)
			}
		}
	}
}
