package auth

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/socktest"
)

// fakeGit fails with a known exit code, 250 numbered stderr lines and one
// stdout line, so the test can see both streams and the tail bound.
const fakeGit = `#!/bin/sh
i=1
while [ $i -le 250 ]; do printf 'example-err-%04d\n' $i >&2; i=$((i+1)); done
echo example-stdout-marker
exit 3
`

// timeoutHarness drives the real main() with subprocess.run replaced by one
// that expires after capturing partial output, reaching run()'s timeout branch.
const timeoutHarness = `import subprocess, sys
sys.path.insert(0, sys.argv[1])
import qualify_fixture as q
def expire(argv, **kw):
    raise subprocess.TimeoutExpired(argv, kw["timeout"], output=b"example-partial-out\n", stderr=b"example-partial-err\n")
subprocess.run = expire
sys.argv = ["qualify_fixture.py"] + sys.argv[2:]
q.main()
`

// regression: #6082
func TestQualifyFixtureShowsOutput(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("could-not-check: python3 is required to drive qualify_fixture.py")
	}
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	// A short directory keeps the Unix socket path under the platform limit.
	dir := socktest.Dir(t, "qf", "s")
	src, bin := filepath.Join(dir, "src"), filepath.Join(dir, "bin")
	for _, d := range []string{src, bin} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(src, ".git"):   "gitdir: /nonexistent\n",
		filepath.Join(bin, "go"):     "#!/bin/sh\nexit 0\n",
		filepath.Join(bin, "docker"): "#!/bin/sh\nexit 0\n",
		filepath.Join(bin, "git"):    fakeGit,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sock := filepath.Join(dir, "s")
	listener, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal("could-not-check: local Unix socket unavailable: ", err)
	}
	t.Cleanup(func() { listener.Close() })
	args := func(work string) []string {
		return []string{"--source", src, "--work", filepath.Join(dir, work),
			"--go", filepath.Join(bin, "go"), "--python", python, "--docker", filepath.Join(bin, "docker"),
			"--endpoint", "unix://" + sock, "--platform", "linux/amd64",
			"--module-cache", filepath.Join(dir, "mod"), "--go-cache", filepath.Join(dir, "cache")}
	}
	cases := []struct {
		name, label string
		argv        []string
		want, never []string
	}{
		{"exit", "RuntimeError: UNQUALIFIED: source\n",
			append([]string{filepath.Join(testdata, "qualify_fixture.py")}, args("exit")...),
			[]string{"exit 3", "example-stdout-marker", "example-err-0051\n", "example-err-0250\n", "last 200 of 250 lines"},
			[]string{"example-err-0050\n"}},
		{"timeout", "RuntimeError: UNQUALIFIED: bounded call timeout: source\n",
			append([]string{"-c", timeoutHarness, testdata}, args("timeout")...),
			[]string{"timeout after 60s", "example-partial-out", "example-partial-err"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(python, c.argv...)
			cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1", "KUBECONFIG=/dev/null")
			out, err := cmd.CombinedOutput()
			text := string(out)
			// The failure must stay a failure with its RuntimeError text unchanged.
			if err == nil || !strings.Contains(text, c.label) {
				t.Fatalf("failed call must still raise %q: err=%v\n%s", c.label, err, text)
			}
			var missing, extra []string
			for _, w := range c.want {
				if !strings.Contains(text, w) {
					missing = append(missing, w)
				}
			}
			for _, n := range c.never {
				if strings.Contains(text, n) {
					extra = append(extra, n)
				}
			}
			if len(missing)+len(extra) > 0 {
				t.Fatalf("failed call output missing %q; beyond its tail bound %q:\n%s", missing, extra, text)
			}
		})
	}
}
