package main

// Tests for the check:ci network-off sandbox with loopback up (issue #1925,
// netns.go). Three properties, each pinned on its own — and each must hold both
// on the host and when this suite itself runs INSIDE the check:ci sandbox, as a
// statusgen-suite Verify row does (TestNetnsTestsPassInSandbox guards that):
//
//  1. inside the sandbox a 127.0.0.1 listen + connect works (the fix);
//  2. in the same sandbox an outbound connection to a NON-loopback address still
//     fails — with an in-test negative control proving the probe connects when
//     the network is open, so a pass is never the probe being blind;
//  3. a sandbox that cannot bring lo up, or is not isolated, REFUSES: the row
//     never runs, and the result is could-not-run — never a fallback to an
//     un-sandboxed run.
//
// The Linux-only tests need `unshare --net --map-root-user` (unprivileged user
// namespaces). They SKIP, naming the reason, only where that raw facility is
// absent (netnsFacility); where the facility works they REQUIRE the full
// sandbox, helper included, so a broken helper fails rather than skips.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// netProbeEnv selects a network probe mode for the test binary (TestMain →
// maybeRunNetProbe). "listen" = httptest server on loopback + GET it;
// "dial=<host:port>" = TCP connect. Exit 0 = connected, netProbeFailExit = the
// probe ran and could not connect.
const (
	netProbeEnv      = "STATUSGEN_TEST_NETPROBE"
	netProbeFailExit = 3
)

func maybeRunNetProbe() {
	mode := os.Getenv(netProbeEnv)
	if mode == "" {
		return
	}
	fail := func(stage string, err error) {
		os.Stderr.WriteString("netprobe " + stage + ": " + err.Error() + "\n")
		os.Exit(netProbeFailExit)
	}
	switch {
	case mode == "listen":
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, "loopback-ok")
		}))
		defer srv.Close()
		resp, err := http.Get(srv.URL)
		if err != nil {
			fail("get", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != "loopback-ok" {
			os.Stderr.WriteString("netprobe: unexpected body " + string(body) + "\n")
			os.Exit(netProbeFailExit)
		}
		os.Stdout.WriteString("netprobe: loopback listen+connect ok\n")
		os.Exit(0)
	case strings.HasPrefix(mode, "dial="):
		c, err := net.DialTimeout("tcp", strings.TrimPrefix(mode, "dial="), 3*time.Second)
		if err != nil {
			fail("dial", err)
		}
		c.Close()
		os.Stdout.WriteString("netprobe: outbound connected\n")
		os.Exit(0)
	}
	os.Stderr.WriteString("netprobe: unknown mode " + mode + "\n")
	os.Exit(2)
}

func selfExe(t *testing.T) string {
	t.Helper()
	p, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return p
}

// probeRow is the Verify-row command that runs the test binary as a probe.
func probeRow(t *testing.T, mode string) string {
	return netProbeEnv + "=" + mode + " '" + selfExe(t) + "'"
}

// requireSandbox skips only where the raw namespace facility is absent, then
// REQUIRES the full sandbox (helper included) to come up.
func requireSandbox(t *testing.T) []string {
	t.Helper()
	if _, ok, why := netnsFacility(); !ok {
		t.Skipf("SKIP (could-not-check, not a pass): needs Linux + unprivileged `unshare --net --map-root-user`: %s", why)
	}
	w, ok, why := networkOffWrapper()
	if !ok {
		t.Fatalf("the namespace facility works on this host but the sandbox does not come up: %s", why)
	}
	return w
}

// ---- portable: run on every host ------------------------------------------

func TestNetnsWrapperArgvShape(t *testing.T) {
	got := netnsWrapperArgv("/usr/bin/unshare", "/x/statusgen", "net:[4026531840]")
	want := []string{"/usr/bin/unshare", "--net", "--map-root-user", "/x/statusgen", netnsHelperArg, "net:[4026531840]"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("sandbox argv = %q, want %q (a new net namespace whose first process is the lo-up helper)", got, want)
	}
}

func TestNetnsIsolationProblem(t *testing.T) {
	lo := netnsIface{Name: "lo", Loopback: true, Up: true, Addrs: 2}
	cases := []struct {
		name      string
		own, par  string
		ifs       []netnsIface
		wantClean bool
		wantSub   string
	}{
		{"isolated, lo up", "net:[2]", "net:[1]", []netnsIface{lo}, true, ""},
		{"down fallback tunnels tolerated", "net:[2]", "net:[1]", []netnsIface{lo, {Name: "sit0"}, {Name: "ip6tnl0"}}, true, ""},
		{"same namespace as caller", "net:[1]", "net:[1]", []netnsIface{lo}, false, "still in the caller's network namespace"},
		{"own id unknown", "", "net:[1]", []netnsIface{lo}, false, "identity"},
		{"caller id unknown", "net:[2]", "", []netnsIface{lo}, false, "identity"},
		{"non-loopback interface up", "net:[2]", "net:[1]", []netnsIface{lo, {Name: "eth0", Up: true, Addrs: 1}}, false, "eth0 is up"},
		{"down interface with an address", "net:[2]", "net:[1]", []netnsIface{lo, {Name: "eth0", Addrs: 1}}, false, "eth0 carries"},
		{"lo still down", "net:[2]", "net:[1]", []netnsIface{{Name: "lo", Loopback: true}}, false, "lo is not up"},
		{"no lo at all", "net:[2]", "net:[1]", nil, false, "lo is not up"},
		{"interface named lo that is not loopback", "net:[2]", "net:[1]", []netnsIface{{Name: "lo", Up: true}}, false, "lo is up inside"},
	}
	for _, c := range cases {
		got := netnsIsolationProblem(c.own, c.par, c.ifs)
		if c.wantClean && got != "" {
			t.Errorf("%s: refused (%q), want isolated", c.name, got)
		}
		if !c.wantClean && (got == "" || !strings.Contains(got, c.wantSub)) {
			t.Errorf("%s: problem = %q, want one containing %q", c.name, got, c.wantSub)
		}
	}
}

func TestNetnsRefusalClassified(t *testing.T) {
	refused := runResult{exit: netnsRefusedExit, output: []byte("x\n" + netnsRefusedMarker + "could not bring lo up: EPERM\n")}
	got := classifyNetnsRefusal(refused)
	if !got.couldNotRun || !strings.Contains(got.reason, "could not bring lo up: EPERM") {
		t.Errorf("helper refusal: couldNotRun=%v reason=%q, want could-not-run carrying the helper's reason", got.couldNotRun, got.reason)
	}
	// The row's own exit 125 with no marker is the row's verdict, untouched.
	own := classifyNetnsRefusal(runResult{exit: netnsRefusedExit, output: []byte("row output\n")})
	if own.couldNotRun {
		t.Error("exit 125 without the refusal marker must stay the row's own result")
	}
	// The marker without the refusal exit is not a refusal.
	pass := classifyNetnsRefusal(runResult{exit: 0, output: []byte(netnsRefusedMarker + "x\n")})
	if pass.couldNotRun || pass.exit != 0 {
		t.Error("the marker alone (exit 0) must not be reclassified")
	}
}

// TestNetnsHelperRefusesOutside: the helper, started in the SAME network
// namespace as its caller — no fresh namespace entered — must refuse and never
// run the command, in every environment the suite runs in. That includes
// running the suite INSIDE the check:ci sandbox (a statusgen-suite row): there
// the test process is mapped root in a lo-only namespace it owns, so lo-up
// succeeds and only the identity check stands between the helper and the
// command. The test therefore hands the helper this process's OWN
// `/proc/self/ns/net` id as the caller id, exactly what networkOffWrapper would
// pass, so the identity check refuses regardless of privilege. (A bogus id here
// let the helper run the command inside the sandbox — review F1.)
//
// Which refusal trips depends on privilege, and both are pinned: unprivileged
// (no CAP_NET_ADMIN over the namespace) it is the lo-up refusal; with it (real
// root, or the sandbox's mapped root) it is the identity refusal. Off Linux the
// stub refuses unconditionally.
func TestNetnsHelperRefusesOutside(t *testing.T) {
	callerNS := "net:[unread-off-linux]"
	if runtime.GOOS == "linux" {
		ns, err := os.Readlink("/proc/self/ns/net")
		if err != nil {
			t.Fatalf("read this process's network namespace id: %v", err)
		}
		callerNS = ns
	}
	cmd := exec.Command(selfExe(t), netnsHelperArg, callerNS, "sh", "-c", "echo RAN-UNSANDBOXED")
	out, err := cmd.CombinedOutput()
	ee, isExit := err.(*exec.ExitError)
	if !isExit || ee.ExitCode() != netnsRefusedExit {
		t.Fatalf("helper in its caller's namespace: err=%v out=%q, want exit %d (refusal)", err, out, netnsRefusedExit)
	}
	if strings.Contains(string(out), "RAN-UNSANDBOXED") {
		t.Fatalf("the helper RAN the command un-sandboxed: %q", out)
	}
	if !strings.Contains(string(out), netnsRefusedMarker) {
		t.Fatalf("refusal carries no marker: %q", out)
	}
	if runtime.GOOS != "linux" {
		return
	}
	loUp := strings.Contains(string(out), "could not bring lo up")
	identity := strings.Contains(string(out), "still in the caller's network namespace")
	if os.Geteuid() != 0 && !loUp {
		t.Fatalf("non-root, in the caller's namespace: refusal = %q, want the lo-up refusal", out)
	}
	if !loUp && !identity {
		t.Fatalf("refusal = %q, want the lo-up or the namespace-identity refusal — nothing else may be what stops a helper in its caller's namespace", out)
	}
}

// TestNetnsBinaryDispatchesHelper pins main()'s dispatch of the helper (review
// A1): the test binary reaches the helper through TestMain, so only a BUILT
// statusgen proves main() does too. Without the dispatch the hidden argument
// falls through to the CLI and the marker never appears; with it, the helper
// refuses in its caller's namespace. Where the namespace facility works, the
// real binary must also run the row inside the full sandbox.
func TestNetnsBinaryDispatchesHelper(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("SKIP (could-not-check, not a pass): go toolchain unavailable to build statusgen")
	}
	bin := filepath.Join(t.TempDir(), "statusgen")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	callerNS := "net:[unread-off-linux]"
	if runtime.GOOS == "linux" {
		ns, err := os.Readlink("/proc/self/ns/net")
		if err != nil {
			t.Fatalf("read this process's network namespace id: %v", err)
		}
		callerNS = ns
	}
	out, err := exec.Command(bin, netnsHelperArg, callerNS, "sh", "-c", "echo RAN-UNSANDBOXED").CombinedOutput()
	ee, isExit := err.(*exec.ExitError)
	if !isExit || ee.ExitCode() != netnsRefusedExit || !strings.Contains(string(out), netnsRefusedMarker) {
		t.Fatalf("built statusgen %s: err=%v out=%q, want the helper's refusal (exit %d + marker) — main() does not dispatch the helper", netnsHelperArg, err, out, netnsRefusedExit)
	}
	if strings.Contains(string(out), "RAN-UNSANDBOXED") {
		t.Fatalf("the built helper RAN the command in its caller's namespace: %q", out)
	}

	unshare, ok, _ := netnsFacility()
	if !ok {
		return
	}
	argv := append(netnsWrapperArgv(unshare, bin, callerNS), "sh", "-c", "echo RAN-SANDBOXED")
	out, err = exec.Command(argv[0], argv[1:]...).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "RAN-SANDBOXED") {
		t.Fatalf("built statusgen as the sandbox helper: err=%v out=%q, want the row run inside the fresh namespace", err, out)
	}
}

// TestNetnsHermeticPathClassifies pins runHermeticallyWith's two outcomes on
// every host, through the networkOffWrapperFn seam (review A2, A4):
//   - sandbox unavailable → could-not-run, and the row NEVER runs un-sandboxed;
//   - a wrapper whose helper refuses → could-not-run (the refusal goes through
//     runSandboxed), never the row's own exit 125.
func TestNetnsHermeticPathClassifies(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("SKIP (could-not-check, not a pass): no `sh` on this host to stand in for the wrapper")
	}
	plan := resolveShellPlan()
	if _, notRun := shellDispatch(rowShellSh, plan); notRun != nil {
		t.Skipf("SKIP (could-not-check, not a pass): no POSIX row shell: %s", notRun.reason)
	}
	defer func(orig func() ([]string, bool, string)) { networkOffWrapperFn = orig }(networkOffWrapperFn)

	marker := filepath.Join(t.TempDir(), "row-ran")
	row := "touch '" + marker + "'; echo RAN-UNSANDBOXED"

	networkOffWrapperFn = func() ([]string, bool, string) { return nil, false, "planted: no sandbox" }
	res := runHermeticallyWith(t.TempDir(), row, 30*time.Second, plan, rowShellSh)
	if !res.couldNotRun || !strings.Contains(res.reason, "planted: no sandbox") {
		t.Fatalf("sandbox unavailable: exit=%d couldNotRun=%v reason=%q, want could-not-run carrying the reason", res.exit, res.couldNotRun, res.reason)
	}
	if _, err := os.Stat(marker); err == nil || strings.Contains(string(res.output), "RAN-UNSANDBOXED") {
		t.Fatal("sandbox unavailable: the row RAN without the sandbox")
	}

	refusing := []string{"sh", "-c", "echo '" + netnsRefusedMarker + "planted refusal' >&2; exit 125", "wrapper"}
	networkOffWrapperFn = func() ([]string, bool, string) { return refusing, true, "" }
	res = runHermeticallyWith(t.TempDir(), row, 30*time.Second, plan, rowShellSh)
	if !res.couldNotRun || !strings.Contains(res.reason, "planted refusal") {
		t.Fatalf("helper refusal: exit=%d couldNotRun=%v reason=%q out=%q, want could-not-run (runHermeticallyWith must go through runSandboxed)", res.exit, res.couldNotRun, res.reason, res.output)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("helper refusal: the row RAN")
	}
}

// ---- Linux: the live sandbox ----------------------------------------------

// (1) 127.0.0.1 listen + connect works inside the sandbox.
func TestNetnsLoopbackListenConnect(t *testing.T) {
	w := requireSandbox(t)
	res := runSandboxed(t.TempDir(), probeRow(t, "listen"), 60*time.Second, w, resolveShellPlan(), rowShellSh)
	if res.couldNotRun || res.exit != 0 {
		t.Fatalf("loopback inside the sandbox: exit=%d couldNotRun=%v reason=%q out=%q, want exit 0", res.exit, res.couldNotRun, res.reason, res.output)
	}
	// And through the production entry point check:ci rows take.
	res = runHermetically(t.TempDir(), probeRow(t, "listen"), 60*time.Second)
	if res.couldNotRun || res.exit != 0 {
		t.Fatalf("runHermetically loopback: exit=%d couldNotRun=%v reason=%q out=%q", res.exit, res.couldNotRun, res.reason, res.output)
	}
}

// (2) an outbound connection to a non-loopback address still fails inside the
// sandbox. The negative control runs the SAME probe un-sandboxed first and
// requires it to connect, so the sandboxed failure is the sandbox, not a blind
// probe or an unreachable target.
func TestNetnsOutboundStillBlocked(t *testing.T) {
	w := requireSandbox(t)
	addr := nonLoopbackIPv4(t)
	ln, err := net.Listen("tcp", net.JoinHostPort(addr, "0"))
	if err != nil {
		t.Fatalf("listen on %s: %v", addr, err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	row := probeRow(t, "dial="+ln.Addr().String())
	plan := resolveShellPlan()

	open := runVerifyCommandWith(t.TempDir(), row, 60*time.Second, nil, plan, rowShellSh)
	if open.couldNotRun || open.exit != 0 {
		t.Fatalf("NEGATIVE CONTROL: the un-sandboxed probe could not reach %s (exit=%d out=%q) — the probe is blind here, so a sandboxed failure would prove nothing", ln.Addr(), open.exit, open.output)
	}

	res := runSandboxed(t.TempDir(), row, 60*time.Second, w, plan, rowShellSh)
	if res.couldNotRun {
		t.Fatalf("sandboxed outbound probe did not run: %s (out=%q)", res.reason, res.output)
	}
	if res.exit != netProbeFailExit {
		t.Fatalf("sandboxed outbound probe to %s: exit=%d out=%q — want exit %d (connection refused by the sandbox); the network is OPEN inside the check:ci sandbox", ln.Addr(), res.exit, res.output, netProbeFailExit)
	}
}

// (3a) a sandbox whose namespace step regressed (no new network namespace) must
// refuse — could-not-run, the row never ran — never run the row on the host net.
func TestNetnsRefusalIsCouldNotRun(t *testing.T) {
	w := requireSandbox(t)
	// The planted regression: the same helper, but `unshare` without `--net`.
	regressed := []string{w[0], "--map-root-user", w[3], netnsHelperArg, w[5]}
	res := runSandboxed(t.TempDir(), "echo RAN-UNSANDBOXED", 60*time.Second, regressed, resolveShellPlan(), rowShellSh)
	if !res.couldNotRun {
		t.Fatalf("regressed sandbox: exit=%d out=%q, want could-not-run (refusal)", res.exit, res.output)
	}
	if strings.Contains(string(res.output), "RAN-UNSANDBOXED") {
		t.Fatalf("the row RAN outside an isolated namespace: %q", res.output)
	}
	if !strings.Contains(res.reason, "refused to run the row") {
		t.Fatalf("reason = %q, want the sandbox refusal", res.reason)
	}
}

// (3c) the lo-up check, provoked for real in EVERY environment the facility
// works in — the host, real root, or the check:ci sandbox itself. The helper runs
// in a fresh USER namespace without `--net`: it is mapped root there, but the
// network namespace it sits in is owned by an ancestor user namespace, so it has
// no CAP_NET_ADMIN over it and the lo-up set must fail. The caller id is a bogus
// one so the identity check is NOT what trips: inside the sandbox (lo already up,
// lo the only interface) nothing but the lo-up refusal stands between the helper
// and the command, which is the case a privilege-dependent premise missed.
func TestNetnsLoUpFailureRefuses(t *testing.T) {
	w := requireSandbox(t)
	out, err := exec.Command(w[0], "--map-root-user", w[3], netnsHelperArg, "net:[0]", "sh", "-c", "echo RAN-UNSANDBOXED").CombinedOutput()
	ee, isExit := err.(*exec.ExitError)
	if !isExit || ee.ExitCode() != netnsRefusedExit {
		t.Fatalf("helper without CAP_NET_ADMIN over its namespace: err=%v out=%q, want exit %d", err, out, netnsRefusedExit)
	}
	if strings.Contains(string(out), "RAN-UNSANDBOXED") || !strings.Contains(string(out), "could not bring lo up") {
		t.Fatalf("helper without CAP_NET_ADMIN over its namespace: out=%q, want the lo-up refusal and no run", out)
	}
}

// (3b) the namespace-identity check, provoked for real: inside a fresh namespace
// (so lo CAN be brought up and only lo exists), tell the helper the caller's
// namespace is the one it is in. It must refuse on identity alone.
func TestNetnsSameNamespaceRefused(t *testing.T) {
	w := requireSandbox(t)
	script := `exec "$0" ` + netnsHelperArg + ` "$(readlink /proc/self/ns/net)" sh -c "echo RAN-UNSANDBOXED"`
	out, err := exec.Command(w[0], w[1], w[2], "sh", "-c", script, w[3]).CombinedOutput()
	ee, isExit := err.(*exec.ExitError)
	if !isExit || ee.ExitCode() != netnsRefusedExit {
		t.Fatalf("same-namespace helper: err=%v out=%q, want exit %d", err, out, netnsRefusedExit)
	}
	if strings.Contains(string(out), "RAN-UNSANDBOXED") || !strings.Contains(string(out), "still in the caller's network namespace") {
		t.Fatalf("same-namespace helper output %q, want the identity refusal and no run", out)
	}
}

// ---- class guard: the netns tests hold INSIDE the sandbox too --------------

// netnsNestedEnv marks the nested run TestNetnsTestsPassInSandbox starts, so the
// nested copy of that test does not recurse.
const netnsNestedEnv = "STATUSGEN_TEST_NETNS_NESTED"

// TestNetnsTestsPassInSandbox is the guard over review F1's class: a netns test
// whose premise holds on the host but not inside the check:ci sandbox (there the
// test process is mapped root in a lo-only namespace it owns). check:ci rows run
// the statusgen suite inside that sandbox, so such a test records the row `fail`
// for a reason that is the sandbox's, not the work's. The guard re-runs every
// TestNetns* test inside the real sandbox and requires them green, so the next
// test written with a host-only premise is red here rather than in a row.
func TestNetnsTestsPassInSandbox(t *testing.T) {
	if os.Getenv(netnsNestedEnv) != "" {
		t.Skip("SKIP (by design, not a gap): this IS the nested in-sandbox run")
	}
	w := requireSandbox(t)
	argv := append(append([]string{}, w...), selfExe(t), "-test.run=^TestNetns", "-test.count=1", "-test.v", "-test.timeout=300s")
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), netnsNestedEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the netns tests FAIL inside the check:ci sandbox (%v) — a test premise that holds only on the host:\n%s", err, out)
	}
	// Positive control: the nested run really ran the refusal tests in there,
	// rather than matching nothing and exiting 0.
	for _, name := range []string{"TestNetnsHelperRefusesOutside", "TestNetnsLoUpFailureRefuses", "TestNetnsSameNamespaceRefused"} {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Fatalf("nested in-sandbox run did not PASS %s — the guard did not look:\n%s", name, out)
		}
	}
}

func nonLoopbackIPv4(t *testing.T) string {
	t.Helper()
	ifs, err := net.Interfaces()
	if err != nil {
		t.Fatalf("net.Interfaces: %v", err)
	}
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() {
				return ipn.IP.String()
			}
		}
	}
	t.Skip("SKIP (could-not-check, not a pass): no up non-loopback IPv4 address on this host to aim the outbound probe at")
	return ""
}

// ---- class guard ------------------------------------------------------------

// Defect class: a network-namespace sandbox assembled anywhere but
// netnsWrapperArgv — such a site would hand the row to a namespace with lo DOWN
// (issue #1925) and skip the isolation proof. The guard walks every non-test Go
// file in this package for the namespace primitives (a `"--net"` / `"--net=…"`
// argument literal, or a CLONE_NEWNET flag) and allows them only inside
// netnsWrapperArgv.
var netnsAllowedSites = map[string]bool{"netnsWrapperArgv": true}

func netnsPrimitiveSites(fset *token.FileSet, f *ast.File) []string {
	var sites []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		name := "<package scope>"
		if ok {
			name = fn.Name.Name
		}
		ast.Inspect(d, func(n ast.Node) bool {
			hit := false
			switch x := n.(type) {
			case *ast.BasicLit:
				v := strings.Trim(x.Value, "\"`")
				hit = x.Kind == token.STRING && (v == "--net" || strings.HasPrefix(v, "--net="))
			case *ast.SelectorExpr:
				hit = x.Sel.Name == "CLONE_NEWNET"
			case *ast.Ident:
				hit = x.Name == "CLONE_NEWNET"
			}
			if hit && !netnsAllowedSites[name] {
				sites = append(sites, fset.Position(n.Pos()).String()+" in "+name)
			}
			return true
		})
	}
	return sites
}

func TestNetnsWrapperOneChokePoint(t *testing.T) {
	// Positive control: a planted second instance MUST be flagged, so a matcher
	// that silently stopped matching fails here instead of reporting clean.
	plant := `package main
import "os/exec"
func plantedSandbox() *exec.Cmd { return exec.Command("unshare", "--net", "--map-root-user", "bash") }
`
	fset := token.NewFileSet()
	pf, err := parser.ParseFile(fset, "planted.go", plant, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := netnsPrimitiveSites(fset, pf); len(got) != 1 || !strings.Contains(got[0], "plantedSandbox") {
		t.Fatalf("POSITIVE CONTROL: guard flagged %q on the planted instance, want exactly plantedSandbox", got)
	}

	files, _ := filepath.Glob("*.go")
	var bad []string
	seenAllowed := false
	for _, p := range files {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		bad = append(bad, netnsPrimitiveSites(fset, f)...)
		if p == "netns.go" {
			seenAllowed = true
		}
	}
	if !seenAllowed {
		t.Fatal("netns.go not found — the guard is not looking at this package")
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("network-namespace sandbox assembled outside netnsWrapperArgv (it would skip the lo-up + isolation helper, issue #1925):\n  %s", strings.Join(bad, "\n  "))
	}
}
