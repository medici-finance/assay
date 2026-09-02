package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The exec-boundary tests run the shim as a REAL PROCESS reached through a
// symlink, because that is the only way to exercise the property under test:
// the guard keys on the NAME it was invoked as (argv[0]) and, on pass-through,
// resolves the next binary of that name on PATH. An in-process call cannot
// observe either.
//
// The subprocess is this test binary re-entered: TestMain sees
// shimTestModeEnv=1 and hands control straight to run(), so a symlink named
// `kubectl` pointing at the test binary behaves exactly like a symlink named
// `kubectl` pointing at the installed clusterguard.
//
// Every CLI reached in these tests is a FIXTURE — a two-line /bin/sh script
// written into a temp dir. No test contacts a live CLI, a live cluster, or the
// network.

const shimTestModeEnv = "CLUSTERGUARD_SHIM_TESTMODE"

func TestMain(m *testing.M) {
	if os.Getenv(shimTestModeEnv) == "1" {
		os.Exit(run(os.Args, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// --- harness -----------------------------------------------------------------

type harness struct {
	shimDir string // the shim directory: symlinks named for each CLI
	fakeDir string // fixture binaries, reached only on pass-through
	home    string // isolated config home: stop flags + the clusterguard log
}

// newHarness builds the three directories and the shim symlinks. The shim dir
// is FIRST on the PATH it hands out, the fixture dir second — the installed
// layout.
func newHarness(t *testing.T, clis ...string) *harness {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("cannot resolve the test binary: %v", err)
	}
	root := t.TempDir()
	h := &harness{
		shimDir: filepath.Join(root, "shims"),
		fakeDir: filepath.Join(root, "fake"),
		home:    filepath.Join(root, "home"),
	}
	for _, d := range []string{h.shimDir, h.fakeDir, h.home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if len(clis) == 0 {
		clis = []string{"kubectl"}
	}
	for _, c := range clis {
		if err := os.Symlink(self, filepath.Join(h.shimDir, c)); err != nil {
			t.Fatalf("symlink %s: %v", c, err)
		}
	}
	return h
}

// fixture writes an executable stand-in for a cluster CLI into the fixture dir.
// It echoes a name marker and its own arguments, so a test can prove both that
// pass-through reached THIS binary and that argv survived intact.
func (h *harness) fixture(t *testing.T, cli string) {
	t.Helper()
	body := "#!/bin/sh\necho FAKE-" + strings.ToUpper(cli) + " args=\"$@\"\n"
	p := filepath.Join(h.fakeDir, cli)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write fixture %s: %v", p, err)
	}
}

type result struct {
	code   int
	stdout string
	stderr string
}

// invoke runs the shim through its symlink, exactly as a shell resolving the
// CLI name on PATH would. optIn == "" means the opt-in variable is ABSENT
// (not empty): the default posture.
func (h *harness) invoke(t *testing.T, cli, optIn string, pathDirs []string, args ...string) result {
	t.Helper()
	if pathDirs == nil {
		pathDirs = []string{h.shimDir, h.fakeDir, "/usr/bin", "/bin"}
	}
	cmd := exec.Command(filepath.Join(h.shimDir, cli), args...)
	env := []string{
		shimTestModeEnv + "=1",
		"HOME=" + h.home,
		"PATH=" + strings.Join(pathDirs, string(os.PathListSeparator)),
	}
	if optIn != "" {
		env = append(env, allowClusterEnv+"="+optIn)
	}
	cmd.Env = env
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running %s: %v (stderr %q)", cli, err, errb.String())
		}
	}
	return result{code: code, stdout: out.String(), stderr: errb.String()}
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

func (h *harness) log(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(h.home, ".config", "assay", "clusterguard.log"))
	if err != nil {
		return ""
	}
	return string(b)
}

// --- 1. refusal without the opt-in -------------------------------------------

// TestRefusesWithoutOptIn is the default posture: a cluster CLI reached through
// the shim with no operator opt-in exported is REFUSED with exit 5, and the
// refusal NAMES the policy rather than failing opaquely. Exit 5 is a refusal,
// never a fallback trigger — the message says so.
func TestRefusesWithoutOptIn(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	got := h.invoke(t, "kubectl", "", nil, "get", "pods")

	if got.code != 5 {
		t.Errorf("exit = %d, want 5 (deskkit ExitRefused); stderr %q", got.code, got.stderr)
	}
	for _, want := range []string{"clusterguard", "offline-by-default", allowClusterEnv} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("refusal does not name %q — a refusal that does not state its policy teaches the caller to route around it.\nstderr: %s", want, got.stderr)
		}
	}
	if strings.Contains(got.stdout, "FAKE-KUBECTL") {
		t.Error("the fixture CLI RAN despite the refusal — the guard passed through on its default path")
	}
}

// TestRefusesScriptWrappedCall is the mutation row: the call the string-matched
// deny rules cannot see. A kubectl invocation inside a shell script is refused
// identically, because the guard sits at the EXEC boundary and never reads the
// caller's text.
func TestRefusesScriptWrappedCall(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	cmd := exec.Command("/bin/sh", "-c", "kubectl version --client")
	cmd.Env = []string{
		shimTestModeEnv + "=1",
		"HOME=" + h.home,
		"PATH=" + strings.Join([]string{h.shimDir, h.fakeDir, "/usr/bin", "/bin"}, string(os.PathListSeparator)),
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()

	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !asExitError(err, &ee) {
			t.Fatalf("running the wrapper: %v", err)
		}
		code = ee.ExitCode()
	}
	if code != 5 {
		t.Errorf("script-wrapped call exit = %d, want 5; stderr %q", code, errb.String())
	}
	if strings.Contains(out.String(), "FAKE-KUBECTL") {
		t.Error("the script-wrapped call reached the fixture CLI")
	}
}

// --- 2. pass-through with the opt-in ------------------------------------------

// TestPassesThroughWithOptIn proves the operator lane: with the opt-in
// exported, a read-only verb reaches the NEXT binary of that name on PATH with
// its arguments intact.
func TestPassesThroughWithOptIn(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	got := h.invoke(t, "kubectl", "1", nil, "get", "pods")

	if got.code != 0 {
		t.Errorf("exit = %d, want 0; stderr %q", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "FAKE-KUBECTL") {
		t.Errorf("pass-through did not reach the fixture binary; stdout %q", got.stdout)
	}
	if !strings.Contains(got.stdout, "get pods") {
		t.Errorf("arguments did not survive pass-through; stdout %q", got.stdout)
	}
}

// TestPassThroughSkipsTheShimDirectory is the self-resolution guard — the
// classic shim bug. The shim directory appears TWICE on PATH and the fixture
// only once, after both. A guard that resolved by name alone would exec itself
// and spin; this asserts it resolves past every copy of itself.
func TestPassThroughSkipsTheShimDirectory(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	got := h.invoke(t, "kubectl", "1", []string{h.shimDir, h.shimDir, h.fakeDir, "/usr/bin", "/bin"}, "get", "pods")

	if got.code != 0 || !strings.Contains(got.stdout, "FAKE-KUBECTL") {
		t.Errorf("exit = %d, stdout %q — pass-through did not resolve past the shim directory", got.code, got.stdout)
	}
}

// TestResolveRealSkipsSelf is the unit-level companion: the resolver refuses to
// return the running executable no matter how many PATH entries offer it.
func TestResolveRealSkipsSelf(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "self")
	if err := os.WriteFile(self, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "shims")
	real := filepath.Join(dir, "real")
	for _, d := range []string{shim, real} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(self, filepath.Join(shim, "kubectl")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(real, "kubectl")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	path := strings.Join([]string{shim, real}, string(os.PathListSeparator))
	got, err := resolveReal("kubectl", path, self)
	if err != nil {
		t.Fatalf("resolveReal: %v", err)
	}
	if got == filepath.Join(shim, "kubectl") {
		t.Fatal("resolveReal returned the shim symlink to ITSELF — the self-exec loop")
	}
	if got != target {
		t.Errorf("resolveReal = %q, want %q", got, target)
	}
}

// --- 3. symlink-name dispatch --------------------------------------------------

// TestSymlinkNameDispatch proves argv[0] keying across the whole shimmed set:
// one binary, five names, each resolving to the fixture of its OWN name.
func TestSymlinkNameDispatch(t *testing.T) {
	clis := []string{"kubectl", "flux", "helm", "talosctl", "k9s"}
	h := newHarness(t, clis...)
	for _, c := range clis {
		h.fixture(t, c)
	}
	for _, c := range clis {
		t.Run(c, func(t *testing.T) {
			got := h.invoke(t, c, "mutate", nil, "whoami")
			if got.code != 0 {
				t.Fatalf("exit = %d, want 0; stderr %q", got.code, got.stderr)
			}
			marker := "FAKE-" + strings.ToUpper(c)
			if !strings.Contains(got.stdout, marker) {
				t.Errorf("invoked as %s but reached %q — argv[0] is not keying the dispatch", c, got.stdout)
			}
		})
	}
}

// TestShimmedSetIsTheDocumentedFive pins the compiled-in set. Adding a CLI is a
// decision, not a drive-by edit.
func TestShimmedSetIsTheDocumentedFive(t *testing.T) {
	want := map[string]bool{"kubectl": true, "flux": true, "helm": true, "talosctl": true, "k9s": true}
	if len(shimmedCLIs) != len(want) {
		t.Fatalf("shimmedCLIs = %v, want the five documented CLIs", shimmedCLIs)
	}
	for _, c := range shimmedCLIs {
		if !want[c] {
			t.Errorf("unexpected shimmed CLI %q", c)
		}
	}
}

// TestUnknownInvocationNameIsUnverifiable — a symlink the guard does not
// recognise is not a licence to exec whatever it finds. It fails CLOSED with
// exit 6 (could not verify what this is), never exit 0.
func TestUnknownInvocationNameIsUnverifiable(t *testing.T) {
	h := newHarness(t, "terraform")
	h.fixture(t, "terraform")

	got := h.invoke(t, "terraform", "mutate", nil, "apply")

	if got.code != 6 {
		t.Errorf("exit = %d, want 6 (unverifiable); stderr %q", got.code, got.stderr)
	}
	if strings.Contains(got.stdout, "FAKE-TERRAFORM") {
		t.Error("an unrecognised shim name passed through to a binary")
	}
}

// --- 4. read-only verbs vs mutating verbs --------------------------------------

// TestVerbClassification is the policy table. The allowlist is the fail-closed
// direction: a verb nobody classified is MUTATING, so a new upstream subcommand
// is refused rather than waved through.
func TestVerbClassification(t *testing.T) {
	cases := []struct {
		cli      string
		args     []string
		readOnly bool
		why      string
	}{
		{"kubectl", []string{"get", "pods"}, true, "get is read-only"},
		{"kubectl", []string{"describe", "pod", "x"}, true, "describe is read-only"},
		{"kubectl", []string{"logs", "x"}, true, "logs is read-only"},
		{"kubectl", []string{"version", "--client"}, true, "version is read-only"},
		{"kubectl", []string{"-n", "ns", "get", "pods"}, true, "a namespace flag and its value are not the verb"},
		{"kubectl", []string{"--context=ctx", "get", "pods"}, true, "a self-contained flag is skipped"},
		{"kubectl", []string{"delete", "pod", "x"}, false, "delete mutates"},
		{"kubectl", []string{"apply", "-f", "x.yaml"}, false, "apply mutates"},
		{"kubectl", []string{"-n", "ns", "delete", "pod", "x"}, false, "flags do not hide a mutating verb"},
		{"kubectl", []string{"drain", "node"}, false, "drain mutates"},
		{"kubectl", []string{"exec", "-it", "pod", "--", "sh"}, false, "exec is a shell into the cluster, never read-only"},
		{"kubectl", []string{"port-forward", "pod", "8080:80"}, false, "port-forward opens a tunnel"},
		{"kubectl", []string{"delete", "-f", "get"}, false, "an argument that spells a read-only verb does not reclassify the call"},
		{"kubectl", nil, false, "no verb at all is not a read-only verb"},
		{"helm", []string{"list"}, true, "helm list is read-only"},
		{"helm", []string{"status", "rel"}, true, "helm status is read-only"},
		{"helm", []string{"install", "rel", "chart"}, false, "helm install mutates"},
		{"helm", []string{"upgrade", "rel", "chart"}, false, "helm upgrade mutates"},
		{"helm", []string{"uninstall", "rel"}, false, "helm uninstall mutates"},
		{"flux", []string{"get", "kustomizations"}, true, "flux get is read-only"},
		{"flux", []string{"check"}, true, "flux check is read-only"},
		{"flux", []string{"reconcile", "source", "git", "x"}, false, "flux reconcile mutates"},
		{"flux", []string{"bootstrap", "github"}, false, "flux bootstrap mutates"},
		{"talosctl", []string{"get", "members"}, true, "talosctl get is read-only"},
		{"talosctl", []string{"dmesg"}, true, "talosctl dmesg is read-only"},
		{"talosctl", []string{"reset"}, false, "talosctl reset mutates"},
		{"talosctl", []string{"upgrade"}, false, "talosctl upgrade mutates"},
		{"talosctl", []string{"apply-config"}, false, "talosctl apply-config mutates"},
		{"k9s", nil, false, "k9s is an interactive TUI that can mutate from inside it — it has NO read-only lane"},
		{"k9s", []string{"--readonly"}, false, "a flag the guard cannot enforce does not make k9s read-only"},
	}
	for _, c := range cases {
		if got := isReadOnly(c.cli, c.args); got != c.readOnly {
			t.Errorf("isReadOnly(%q, %v) = %v, want %v — %s", c.cli, c.args, got, c.readOnly, c.why)
		}
	}
}

// TestReadOnlyTierRefusesMutatingVerbs — the read-only opt-in is not the
// mutating opt-in. A mutating verb under ASSAY_ALLOW_CLUSTER=1 is refused with
// exit 5 and the fixture never runs.
func TestReadOnlyTierRefusesMutatingVerbs(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	got := h.invoke(t, "kubectl", "1", nil, "delete", "pod", "x")

	if got.code != 5 {
		t.Errorf("exit = %d, want 5; stderr %q", got.code, got.stderr)
	}
	if strings.Contains(got.stdout, "FAKE-KUBECTL") {
		t.Error("a mutating verb reached the CLI under the read-only opt-in")
	}
	if !strings.Contains(got.stderr, "mutate") {
		t.Errorf("the refusal does not name the tier that would allow it; stderr %q", got.stderr)
	}
}

// TestMutateTierAllowsMutatingVerbs — the escalated tier is real, not
// decorative.
func TestMutateTierAllowsMutatingVerbs(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	got := h.invoke(t, "kubectl", "mutate", nil, "delete", "pod", "x")

	if got.code != 0 || !strings.Contains(got.stdout, "FAKE-KUBECTL") {
		t.Errorf("exit = %d, stdout %q — the mutate tier did not pass through", got.code, got.stdout)
	}
	if !strings.Contains(got.stdout, "delete pod x") {
		t.Errorf("arguments did not survive; stdout %q", got.stdout)
	}
}

// TestUnrecognisedOptInValueRefuses — a typo in the opt-in is a REFUSAL, never
// a silent downgrade to "unset" and never an upgrade to "allowed".
func TestUnrecognisedOptInValueRefuses(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	got := h.invoke(t, "kubectl", "yes-please", nil, "get", "pods")

	if got.code != 5 {
		t.Errorf("exit = %d, want 5; stderr %q", got.code, got.stderr)
	}
	if strings.Contains(got.stdout, "FAKE-KUBECTL") {
		t.Error("an unrecognised opt-in value passed through")
	}
}

func TestParseTier(t *testing.T) {
	cases := []struct {
		v       string
		present bool
		want    optInTier
	}{
		{"", false, tierNone},
		{"", true, tierNone},
		{"   ", true, tierNone},
		{"1", true, tierReadOnly},
		{"ro", true, tierReadOnly},
		{"read-only", true, tierReadOnly},
		{"READ-ONLY", true, tierReadOnly},
		{"mutate", true, tierMutate},
		{"MUTATE", true, tierMutate},
		{"0", true, tierInvalid},
		{"true", true, tierInvalid},
		{"yes", true, tierInvalid},
	}
	for _, c := range cases {
		if got := parseTier(c.v, c.present); got != c.want {
			t.Errorf("parseTier(%q, %v) = %v, want %v", c.v, c.present, got, c.want)
		}
	}
}

// --- 5. the documented limit: an absolute-path invocation is not intercepted ---

// TestAbsolutePathInvocationBypassesTheShim is a NEGATIVE CONTROL, not a bug
// report. PATH resolution is this layer's single point of failure and a call
// made by absolute path never consults PATH, so it never reaches the guard.
// The test exists so the limit is PROVEN and cannot quietly stop being true —
// a future "fix" that made this test fail would mean the guard had grown a
// mechanism it does not have.
func TestAbsolutePathInvocationBypassesTheShim(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	cmd := exec.Command(filepath.Join(h.fakeDir, "kubectl"), "delete", "pod", "x")
	cmd.Env = []string{
		"HOME=" + h.home,
		"PATH=" + strings.Join([]string{h.shimDir, h.fakeDir, "/usr/bin", "/bin"}, string(os.PathListSeparator)),
	}
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("the fixture did not run: %v", err)
	}
	if !strings.Contains(out.String(), "FAKE-KUBECTL") {
		t.Fatalf("the absolute-path call did not reach the fixture; stdout %q", out.String())
	}
	if strings.Contains(h.log(t), "delete") {
		t.Error("the guard logged an absolute-path call it cannot have seen — the fixture is not isolated")
	}
}

// --- 6. the log is the detection surface ---------------------------------------

// TestLogRecordsBothVerdicts — a stopped probe must leave a trace. A refusal
// that is silent is indistinguishable from a call nobody made.
func TestLogRecordsBothVerdicts(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")

	h.invoke(t, "kubectl", "", nil, "get", "pods")
	h.invoke(t, "kubectl", "1", nil, "get", "pods")

	log := h.log(t)
	if log == "" {
		t.Fatal("no log was written — the detection surface does not exist")
	}
	if !strings.Contains(log, "verdict=refused") {
		t.Errorf("the refusal is not on the log:\n%s", log)
	}
	if !strings.Contains(log, "verdict=allowed") {
		t.Errorf("the pass-through is not on the log:\n%s", log)
	}
	if strings.Count(log, "cli=kubectl") < 2 {
		t.Errorf("both verdicts should name the CLI:\n%s", log)
	}
}

// TestLogRedactsCredentialArguments — the log is written on every call, so it
// must not become a credential file.
func TestLogRedactsCredentialArguments(t *testing.T) {
	got := redactArgv([]string{"kubectl", "get", "pods", "--token", "s3cr3t", "--password=hunter2", "--server", "https://example.invalid"})
	joined := strings.Join(got, " ")
	if strings.Contains(joined, "s3cr3t") || strings.Contains(joined, "hunter2") {
		t.Errorf("credential values survived redaction: %q", joined)
	}
	if !strings.Contains(joined, "get") || !strings.Contains(joined, "pods") {
		t.Errorf("redaction ate the verb: %q", joined)
	}
}

// --- 7. fail-closed edges -------------------------------------------------------

// TestNoRealBinaryIsUnverifiable — the opt-in is present but nothing to run.
// That is exit 6 (a precondition could not be verified), DISTINCT from the
// exit-5 refusal and never a silent 0.
func TestNoRealBinaryIsUnverifiable(t *testing.T) {
	h := newHarness(t)
	// deliberately no fixture, and the fixture dir is off the PATH

	got := h.invoke(t, "kubectl", "1", []string{h.shimDir}, "get", "pods")

	if got.code != 6 {
		t.Errorf("exit = %d, want 6 (unverifiable); stderr %q", got.code, got.stderr)
	}
	if got.code == 5 {
		t.Error("a missing binary must not read as a policy refusal")
	}
}

// TestStopFlagRefusesEvenWithOptIn — the guard consults deskkit.Guard() like
// every other desk tool, but a stop flag can only make it STRICTER. An armed
// kill switch that made a refusal-guard stop intercepting would fail OPEN,
// which is the inversion this asserts cannot happen.
func TestStopFlagRefusesEvenWithOptIn(t *testing.T) {
	h := newHarness(t)
	h.fixture(t, "kubectl")
	dir := filepath.Join(h.home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "DISABLED"), []byte("test arming\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := h.invoke(t, "kubectl", "mutate", nil, "get", "pods")

	if got.code != 3 {
		t.Errorf("exit = %d, want 3 (disabled); stderr %q", got.code, got.stderr)
	}
	if strings.Contains(got.stdout, "FAKE-KUBECTL") {
		t.Error("an armed kill switch made the guard FAIL OPEN — the whole point of the decision it records")
	}
}
