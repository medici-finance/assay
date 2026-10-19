package perfrec

// consumer_test.go — the public recorder boundary as an EXTERNAL module sees it. A synthetic
// consumer module outside this repository requires perfrec, resolves it with GOWORK=off to
// THIS directory through a replace directive (the same way statusgen and tools/desk do), and
// is built and run. Its dependency closure is then checked; planted copies of perfrec that
// import a network, process-launch, credential or desk package are the negative controls.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const perfrecPath = "github.com/medici-finance/assay/perfrec"

// consumerMain is the synthetic consumer: one measured invocation through the public API.
const consumerMain = `package main

import (
	"os"

	"github.com/medici-finance/assay/perfrec"
)

func main() {
	s := perfrec.Begin(perfrec.SystemProbe(), perfrec.ProcessEntry(), []string{"load"})
	stop := s.Phase("load")
	stop()
	s.Observe(perfrec.CounterSubprocess, perfrec.CounterHTTP)
	code := 0
	if len(os.Args) > 1 {
		code = 5
	}
	if perfrec.Enabled() {
		perfrec.Append(s.Finish("consumer", "run", "", code, "dev"))
	}
	os.Exit(code)
}
`

// boundaryClass names why a dependency may not enter the recorder's closure, or "".
func boundaryClass(dep string) string {
	switch {
	case dep == perfrecPath || dep == "example.com/perfconsumer":
		return ""
	case dep == "net" || strings.HasPrefix(dep, "net/") || dep == "crypto/tls":
		return "network"
	case dep == "os/exec":
		return "exec"
	case dep == "os/user" || regexp.MustCompile(`(?i)cred|token|secret|keychain|oauth`).MatchString(dep):
		return "credential"
	case strings.Contains(dep, "/tools/desk") || strings.Contains(dep, "deskkit"):
		return "desk"
	case strings.HasPrefix(dep, "database/"):
		return "database"
	case regexp.MustCompile(`^[^/]+\.[^/]+/`).MatchString(dep):
		return "non-stdlib"
	}
	return ""
}

func goEnv(extra ...string) []string {
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	return append(env, extra...)
}

func goCmd(t *testing.T, dir string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = goEnv()
	return cmd.CombinedOutput()
}

// writeModule lays out a module directory from name→content.
func writeModule(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func consumerModule(t *testing.T, perfrecDir string, extraReplace string) string {
	t.Helper()
	dir := t.TempDir()
	writeModule(t, dir, map[string]string{
		"go.mod": "module example.com/perfconsumer\n\ngo 1.27.0\n\nrequire " + perfrecPath +
			" v0.0.0-00010101000000-000000000000\n\nreplace " + perfrecPath + " => " + perfrecDir + "\n" + extraReplace,
		"main.go": consumerMain,
	})
	return dir
}

// closureViolations lists the consumer's dependencies that break the recorder boundary.
func closureViolations(t *testing.T, consumer string) map[string]string {
	t.Helper()
	out, err := goCmd(t, consumer, "list", "-deps", ".")
	if err != nil {
		t.Fatalf("go list -deps in the consumer: %v\n%s", err, out)
	}
	bad := map[string]string{}
	for _, d := range strings.Fields(string(out)) {
		if c := boundaryClass(d); c != "" {
			bad[d] = c
		}
	}
	return bad
}

// TestPublicRecorderConsumerBoundary builds and runs a synthetic external consumer against
// the ACTUAL perfrec directory with GOWORK=off: on, it appends one decodable perf-v2 record at
// the process-init boundary; off, it creates nothing and its output and exit are unchanged.
// Its closure holds nothing online, nothing that launches processes or reads credentials, and
// nothing from the desk tools — and each of those four, planted into a copy of perfrec, is
// caught.
// regression: desktools-v2/61
func TestPublicRecorderConsumerBoundary(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("go toolchain not on PATH: %v", err)
	}
	self, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	consumer := consumerModule(t, self, "")

	// It resolves THIS directory, not a copy or a cached version.
	out, err := goCmd(t, consumer, "list", "-m", "-f", "{{.Replace.Dir}}", perfrecPath)
	if err != nil || strings.TrimSpace(string(out)) != self {
		t.Fatalf("consumer resolved perfrec to %q (%v), want %s", out, err, self)
	}
	if bad := closureViolations(t, consumer); len(bad) != 0 {
		t.Fatalf("the consumer's closure crosses the recorder boundary: %v", bad)
	}

	bin := filepath.Join(t.TempDir(), "perfconsumer")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := goCmd(t, consumer, "build", "-o", bin, "."); err != nil {
		t.Fatalf("build the consumer: %v\n%s", err, out)
	}
	run := func(home, telemetry string, args ...string) (string, string, int) {
		cmd := exec.Command(bin, args...)
		var so, se bytes.Buffer
		cmd.Stdout, cmd.Stderr = &so, &se
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, EnvVar+"="+telemetry)
		_ = cmd.Run()
		return so.String(), se.String(), cmd.ProcessState.ExitCode()
	}
	for _, args := range [][]string{nil, {"refuse"}} {
		offHome, onHome := t.TempDir(), t.TempDir()
		offOut, offErr, offRC := run(offHome, "", args...)
		onOut, onErr, onRC := run(onHome, "1", args...)
		if entries, _ := os.ReadDir(offHome); len(entries) != 0 {
			t.Fatalf("switch off, the consumer created %v", entries)
		}
		if offOut != onOut || offErr != onErr || offRC != onRC {
			t.Fatalf("recording changed the consumer: off %q/%q/%d on %q/%q/%d", offOut, offErr, offRC, onOut, onErr, onRC)
		}
		files, _ := filepath.Glob(filepath.Join(onHome, ".config", "assay", "perf", "*.jsonl"))
		if len(files) != 1 {
			t.Fatalf("switch on wrote %v, want one day file", files)
		}
		raw, _ := os.ReadFile(files[0])
		rec, err := Decode(bytes.TrimSpace(raw))
		if err != nil || rec.Schema != SchemaV2 || rec.Boundary != BoundaryProcess || rec.Tool != "consumer" ||
			rec.Exit != onRC || len(rec.Phases) != 1 || rec.Counters[CounterHTTP] != 0 || len(rec.Unavailable) != len(Counters())-2 {
			t.Fatalf("consumer record: %+v %v\n%s", rec, err, raw)
		}
		if want := CPUMeasured; runtime.GOOS == "windows" {
			if rec.CPU != CPUUnsupported {
				t.Fatalf("windows consumer cpu %q", rec.CPU)
			}
		} else if rec.CPU != want {
			t.Fatalf("consumer cpu %q, want %q (switch on at process init)", rec.CPU, want)
		}
	}

	// Negative controls: a copy of perfrec with one forbidden import planted.
	stub := t.TempDir()
	writeModule(t, stub, map[string]string{
		"cred/go.mod":         "module example.com/credstore\n\ngo 1.27.0\n",
		"cred/credstore.go":   "package credstore\n\nvar Token string\n",
		"desk/go.mod":         "module github.com/medici-finance/assay/tools/desk\n\ngo 1.27.0\n",
		"desk/deskkit/kit.go": "package deskkit\n\nvar Loaded bool\n",
	})
	for _, tc := range []struct {
		class, imp, require string
	}{
		{"network", "net/http", ""},
		{"exec", "os/exec", ""},
		{"credential", "example.com/credstore", "require example.com/credstore v0.0.0\nreplace example.com/credstore => " + filepath.Join(stub, "cred") + "\n"},
		{"desk", "github.com/medici-finance/assay/tools/desk/deskkit", "require github.com/medici-finance/assay/tools/desk v0.0.0\nreplace github.com/medici-finance/assay/tools/desk => " + filepath.Join(stub, "desk") + "\n"},
	} {
		t.Run("planted "+tc.class, func(t *testing.T) {
			planted := t.TempDir()
			srcs, _ := filepath.Glob("*.go")
			for _, f := range append(srcs, "go.mod") {
				if strings.HasSuffix(f, "_test.go") {
					continue
				}
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				if f == "go.mod" {
					b = append(b, []byte("\n"+tc.require)...)
				}
				if err := os.WriteFile(filepath.Join(planted, f), b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			writeModule(t, planted, map[string]string{"zz_planted.go": "package perfrec\n\nimport _ \"" + tc.imp + "\"\n"})
			c := consumerModule(t, planted, tc.require)
			bad := closureViolations(t, c)
			if bad[tc.imp] != tc.class {
				t.Fatalf("planted %s import %s not caught: %v", tc.class, tc.imp, bad)
			}
		})
	}
}
