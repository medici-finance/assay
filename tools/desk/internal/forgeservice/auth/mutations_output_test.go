package auth

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mutationsHarness drives the real main() of local_authority_mutations.py with
// its source checkout, archive and nested go test runs replaced by stand-ins.
// The archive carries one owner file holding the first fault's anchor, so the
// mutant step can apply its edit. Each mode makes one check fail:
// baseline, mutant (not caught) or restored, or the run times out.
const mutationsHarness = `import io, pathlib, subprocess, sys, tarfile, tempfile
sys.path.insert(0, sys.argv[1])
import local_authority_mutations as m
mode = sys.argv[2]
root = pathlib.Path(tempfile.mkdtemp(prefix="example-root-"))
(root / ".git").write_text("gitdir: /nonexistent\n")
m.ROOT = root
m.command = lambda args, cwd=None: "" if "status" in args else "example-head"
buf = io.BytesIO()
with tarfile.open(fileobj=buf, mode="w") as tar:
    data = b"if CheckConfig(cfg) != nil || !clockPresent(wall)\n"
    info = tarfile.TarInfo("tools/desk/internal/forgeservice/auth/controller.go")
    info.size = len(data)
    tar.addfile(info, io.BytesIO(data))
subprocess.check_output = lambda args, cwd=None: buf.getvalue()
lines = "".join("example-go-%04d\n" % i for i in range(1, 251))
passed = (lines + "--- PASS: TestForgeLocalAuthorityLaunch (0.01s)\n").encode()
caught = (lines + "malformed configuration accepted\n--- FAIL: TestForgeLocalAuthorityLaunch (0.01s)\n").encode()
calls = []
def fake(argv, **kw):
    calls.append(argv)
    step = len(calls)
    if mode == "timeout":
        raise subprocess.TimeoutExpired(argv, kw["timeout"], output=lines.encode())
    if mode == "baseline":
        return subprocess.CompletedProcess(argv, 1, lines.encode())
    if mode == "mutant":
        return subprocess.CompletedProcess(argv, 0, passed)
    if step == 1:
        return subprocess.CompletedProcess(argv, 0, passed)
    if step == 2:
        return subprocess.CompletedProcess(argv, 1, caught)
    return subprocess.CompletedProcess(argv, 1, lines.encode())
subprocess.run = fake
m.main()
`

// regression: #6082
func TestMutationsScriptShowsNestedOutput(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("could-not-check: python3 is required to drive local_authority_mutations.py")
	}
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	packet := filepath.Join(t.TempDir(), "example-packet.json")
	if err := os.WriteFile(packet, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// first is the oldest line inside the 200-line tail; cut is the newest line beyond it.
	cases := []struct{ mode, label, outcome, raised, total, first, cut string }{
		{"baseline", "construction-policy-baseline", "exit 1", "UNQUALIFIED baseline: construction-policy\n", "250", "0051", "0050"},
		{"mutant", "construction-policy-mutant", "exit 0", "RuntimeError: NOT CAUGHT by independent compiling witness: construction-policy\n", "251", "0052", "0051"},
		{"restored", "construction-policy-restored", "exit 1", "restored baseline failed: construction-policy\n", "250", "0051", "0050"},
		{"timeout", "construction-policy-baseline", "timeout after 330s", "subprocess.TimeoutExpired", "250", "0051", "0050"},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			cmd := exec.Command(python, "-c", mutationsHarness, testdata, c.mode)
			// TMPDIR keeps the script's receipt directories inside the test's own scratch.
			cmd.Env = append(os.Environ(), "TMPDIR="+t.TempDir(), "AUTH57_CUSTODY_INPUT="+packet, "PYTHONDONTWRITEBYTECODE=1", "KUBECONFIG=/dev/null")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the failing %s check exited 0:\n%s", c.mode, out)
			}
			text := string(out)
			var missing []string
			for _, want := range []string{
				"UNQUALIFIED call " + c.label + ": " + c.outcome + "\n",
				"----- begin " + c.label + ".log (last 200 of " + c.total + " lines) -----\n",
				"example-go-" + c.first + "\n", "example-go-0250\n",
				"----- end " + c.label + ".log -----\n",
				c.raised,
			} {
				if !strings.Contains(text, want) {
					missing = append(missing, want)
				}
			}
			if len(missing) > 0 || strings.Contains(text, "example-go-"+c.cut+"\n") {
				t.Fatalf("nested go test output missing %q (or shown beyond its 200-line tail):\n%s", missing, text)
			}
		})
	}
}
