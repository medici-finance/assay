package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

type stateFixture struct{ root, spec, state, trace, hold, marker string }

func newStateFixture(t *testing.T) stateFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX shell and Unix interruption")
	}
	base := t.TempDir()
	f := stateFixture{root: filepath.Join(base, "tree"), spec: filepath.Join(base, "spec.json"), state: filepath.Join(base, "run.jsonl"), trace: filepath.Join(base, "trace"), hold: filepath.Join(base, "hold"), marker: filepath.Join(base, "marker")}
	if err := os.Mkdir(f.root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(f.root, "guard.txt"), "control=1\nfirst=1\nsecond=1\n# comment\n")
	script := "cat guard.txt >> " + shellQuote(f.trace) + "\nsleep 0.01\ngrep -q 'control=1' guard.txt || exit 1\ngrep -q 'first=1' guard.txt || exit 1\nif grep -q 'second=2' guard.txt && test -f " + shellQuote(f.hold) + "; then\n (while :; do echo pulse >> " + shellQuote(f.marker) + "; sleep 0.03; done) &\n wait\nfi\n"
	writeFixture(t, filepath.Join(f.root, "check.sh"), script)
	s := spec{Root: f.root, Test: "sh check.sh", Build: "sleep 0.01", Control: Mutation{Name: "control", File: "guard.txt", Old: "control=1", New: "control=2"}, Mutations: []Mutation{
		{Name: "first", File: "guard.txt", Old: "first=1", New: "first=2"}, {Name: "second", File: "guard.txt", Old: "second=1", New: "second=2"}, {Name: "comment", File: "guard.txt", Old: "# comment", New: "# changed"}, {Name: "stale", File: "guard.txt", Old: "absent", New: "new"},
	}}
	raw, _ := json.Marshal(s)
	writeFixture(t, f.spec, string(raw))
	return f
}
func stateCommand(bin string, f stateFixture, args ...string) *exec.Cmd {
	return exec.Command(bin, append([]string{"-spec", f.spec, "-state", f.state}, args...)...)
}
func runStateCmd(t *testing.T, bin string, f stateFixture, args ...string) (string, int) {
	t.Helper()
	cmd := stateCommand(bin, f, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		return stderr.String() + stdout.String(), exit.ExitCode()
	}
	return stdout.String(), 0
}
func readStateLines(t *testing.T, path string) []stateEntry {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("invalid trailing state: %q", raw)
	}
	var records []stateEntry
	for _, line := range bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'}) {
		var r stateEntry
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatal(err)
		}
		records = append(records, r)
	}
	return records
}
func waitStateFile(t *testing.T, path string) {
	t.Helper()
	for end := time.Now().Add(15 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if raw, _ := os.ReadFile(path); len(raw) > 0 {
			return
		}
	}
	t.Fatal("fixture never reached slow mutation")
}
func stopStateCmd(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			t.Fatalf("interrupt = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("interrupt did not finish")
	}
}

// regression: F-state-resume
func TestStateResume(t *testing.T) {
	f := newStateFixture(t)
	bin := muharTestBinary(t)
	pristine, _ := os.ReadFile(filepath.Join(f.root, "guard.txt"))
	writeFixture(t, f.hold, "hold")
	cmd := stateCommand(bin, f)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	waitStateFile(t, f.marker)
	records := readStateLines(t, f.state)
	if len(records) != 3 || records[2].Index != 0 || records[2].Verdict != "CAUGHT" {
		t.Fatalf("completed record missing: %+v", records)
	}
	stopStateCmd(t, cmd)
	got, _ := os.ReadFile(filepath.Join(f.root, "guard.txt"))
	if !bytes.Equal(got, pristine) {
		t.Fatalf("source not restored: %s", got)
	}
	before, _ := os.ReadFile(f.marker)
	time.Sleep(150 * time.Millisecond)
	after, _ := os.ReadFile(f.marker)
	if len(after) != len(before) {
		t.Fatal("suite child survived interruption")
	}
	readStateLines(t, f.state)
	if err := os.Remove(f.hold); err != nil {
		t.Fatal(err)
	}
	trace, _ := os.ReadFile(f.trace)
	out, code := runStateCmd(t, bin, f)
	if code != 0 {
		t.Fatalf("resume %d: %s", code, out)
	}
	fresh, _ := os.ReadFile(f.trace)
	delta := string(fresh[len(trace):])
	if strings.Contains(delta, "first=2") || strings.Count(delta, "control=2") != 1 {
		t.Fatalf("resume failed to skip first or rerun control: %s", delta)
	}
	records = readStateLines(t, f.state)
	if records[1].BaselineNS <= 0 || records[1].ControlNS <= 0 || records[2].BuildNS <= 0 || records[2].TestNS <= 0 || records[2].Selection == "" {
		t.Fatalf("missing measurements: %+v", records[:3])
	}
	file, err := os.OpenFile(f.state, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString(`{"kind":"mutation"`)
	_ = file.Close()
	trace, _ = os.ReadFile(f.trace)
	out, code = runStateCmd(t, bin, f)
	if code != 1 || !strings.Contains(out, "incomplete trailing record") {
		t.Fatalf("torn tail accepted: %d %s", code, out)
	}
	fresh, _ = os.ReadFile(f.trace)
	if !bytes.Equal(trace, fresh) {
		t.Fatal("torn tail ran command")
	}
	t.Run("identity-interrupt", func(t *testing.T) {
		root := t.TempDir()
		scratch := t.TempDir()
		marker := filepath.Join(scratch, "pulse")
		writeFixture(t, filepath.Join(root, "go.mod"), "module example.test/interrupt\n\ngo 1.27.0\n")
		writeFixture(t, filepath.Join(root, "p.go"), "package p\nfunc Value() int { return 1 }\n")
		realGo, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		shim := filepath.Join(scratch, "go")
		writeFixture(t, shim, "#!/bin/sh\nif [ \"$1\" = list ]; then\n (while :; do echo pulse >> "+shellQuote(marker)+"; sleep 0.03; done) &\n wait\nfi\nexec "+shellQuote(realGo)+" \"$@\"\n")
		if err := os.Chmod(shim, 0o755); err != nil {
			t.Fatal(err)
		}
		g := stateFixture{root: root, spec: filepath.Join(scratch, "spec.json"), state: filepath.Join(scratch, "run.jsonl")}
		raw, _ := json.Marshal(spec{Root: root, Test: "go test -count=1 .", Control: Mutation{Name: "control", File: "p.go", Old: "return 1", New: "return 0"}})
		writeFixture(t, g.spec, string(raw))
		child := stateCommand(bin, g)
		child.Env = append(os.Environ(), "PATH="+scratch+string(os.PathListSeparator)+os.Getenv("PATH"))
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = child.Process.Kill() }()
		waitStateFile(t, marker)
		stopStateCmd(t, child)
		before, _ := os.ReadFile(marker)
		time.Sleep(100 * time.Millisecond)
		after, _ := os.ReadFile(marker)
		if !bytes.Equal(before, after) {
			t.Fatal("identity discovery child survived interrupt")
		}
		state, err := openRunState(g.state)
		if err != nil {
			t.Fatalf("interrupt retained state writer lock: %v", err)
		}
		defer state.close()
		if err := state.bind("after-discovery-interrupt"); err != nil {
			t.Fatal(err)
		}
		pristine, _ := os.ReadFile(filepath.Join(root, "p.go"))
		if strings.Contains(string(pristine), "return 0") {
			t.Fatal("identity discovery planted a mutation")
		}
	})

}

// regression: F-state-identity
func TestStateRejectsNewTree(t *testing.T) {
	bin := muharTestBinary(t)
	for _, which := range []string{"source", "tests", "select", "shard-change", "shard-remove"} {
		t.Run(which, func(t *testing.T) {
			f := newStateFixture(t)
			var initial []string
			if strings.HasPrefix(which, "shard-") {
				initial = []string{"-shard", "0/2"}
			}
			if out, code := runStateCmd(t, bin, f, initial...); code != 0 {
				t.Fatalf("initial %d: %s", code, out)
			}
			args := initial
			switch which {
			case "source":
				writeFixture(t, filepath.Join(f.root, "guard.txt"), "changed source")
			case "tests":
				writeFixture(t, filepath.Join(f.root, "check.sh"), "exit 0\n")
			case "select":
				args = []string{"-select"}
			case "shard-change":
				args = []string{"-shard", "1/2"}
			case "shard-remove":
				args = nil
			}
			trace, _ := os.ReadFile(f.trace)
			out, code := runStateCmd(t, bin, f, args...)
			if code != 1 || !strings.Contains(out, "identity/schema mismatch") {
				t.Fatalf("changed %s reused state: %d %s", which, code, out)
			}
			after, _ := os.ReadFile(f.trace)
			if !bytes.Equal(trace, after) {
				t.Fatal("identity refusal ran suite")
			}
		})
	}
	t.Run("runner-build", func(t *testing.T) {
		f := newStateFixture(t)
		s, code := loadSpec(f.spec, os.Stderr)
		if code != 0 {
			t.Fatal(code)
		}
		path := filepath.Join(t.TempDir(), "muhar")
		writeFixture(t, path, "classifier one")
		original := runnerExecutable
		runnerExecutable = func() (string, error) { return path, nil }
		defer func() { runnerExecutable = original }()
		o := options{spec: f.spec, state: f.state}
		base, err := stateIdentity(s, o)
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, path, "classifier two")
		changed, err := stateIdentity(s, o)
		if err != nil {
			t.Fatal(err)
		}
		if base == changed {
			t.Fatal("changed runner classifier reused identity")
		}
	})
	t.Run("command-words", func(t *testing.T) {
		for _, command := range []string{"cargo test", "cargo test && true", "echo mango fruit"} {
			if _, _, goCmd, err := stateGoArgs(command); err != nil || goCmd {
				t.Fatalf("non-Go command %q classified as Go: %v", command, err)
			}
		}
		for _, command := range []string{"/custom/bin/go test .", "./go test .", "'/custom/bin/go' test . && true", "go test . && true"} {
			if _, _, _, err := stateGoArgs(command); err == nil {
				t.Fatalf("unresolved Go command accepted: %s", command)
			}
		}
	})

	t.Run("Go-inputs", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX toolchain shim")
		}
		root := t.TempDir()
		dep := t.TempDir()
		writeFixture(t, filepath.Join(dep, "go.mod"), "module example.test/dep\n\ngo 1.27.0\n")
		writeFixture(t, filepath.Join(dep, "dep.go"), "package dep\nfunc Value() int { return 1 }\n")
		writeFixture(t, filepath.Join(root, "go.mod"), "module example.test/state\n\ngo 1.27.0\nrequire example.test/dep v0.0.0\nreplace example.test/dep => "+filepath.ToSlash(dep)+"\n")
		writeFixture(t, filepath.Join(root, "p.go"), "package p\nimport \"example.test/dep\"\nfunc Value() int { return dep.Value() }\n")
		writeFixture(t, filepath.Join(root, "p_test.go"), "package p\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value()!=1 {t.Fatal(\"bad\")} }\n")
		specPath := filepath.Join(root, "spec.json")
		writeFixture(t, specPath, `{"test":"go test -count=1 ."}`)
		realGo, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		shimDir := t.TempDir()
		shim := filepath.Join(shimDir, "go")
		writeFixture(t, shim, "#!/bin/sh\nexec "+shellQuote(realGo)+" \"$@\"\n")
		if err := os.Chmod(shim, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		s := spec{Root: root, Test: "go test -count=1 ."}
		o := options{spec: specPath, state: filepath.Join(t.TempDir(), "run.jsonl")}
		base, err := stateIdentity(s, o)
		if err != nil {
			t.Fatal(err)
		}
		same, err := stateIdentity(s, o)
		if err != nil || same != base {
			t.Fatalf("unchanged Go inputs changed identity: %v", err)
		}
		o.jobs = 2
		scheduled, err := stateIdentity(s, o)
		if err != nil || scheduled != base {
			t.Fatalf("scheduling changed identity: %v", err)
		}
		t.Run("external-flags", func(t *testing.T) {
			t.Setenv("GOFLAGS", "-overlay="+filepath.Join(t.TempDir(), "overlay.json"))
			if _, err := stateIdentity(s, o); err == nil || !strings.Contains(err.Error(), "cannot resolve external inputs") {
				t.Fatalf("unbound inherited overlay accepted: %v", err)
			}
		})
		prefixed := s
		prefixed.Test = "GOFLAGS='-overlay=unbound.json' go test -count=1 ."
		if _, err := stateIdentity(prefixed, o); err == nil || !strings.Contains(err.Error(), "cannot resolve external inputs") {
			t.Fatalf("unbound command overlay accepted: %v", err)
		}
		for _, which := range []string{"toolchain", "compiled-test", "dependency-metadata", "external-source", "external-metadata"} {
			path := shim
			if which == "compiled-test" {
				path = filepath.Join(root, "p_test.go")
			}
			if which == "dependency-metadata" {
				path = filepath.Join(root, "go.mod")
			}
			if which == "external-source" {
				path = filepath.Join(dep, "dep.go")
			}
			if which == "external-metadata" {
				path = filepath.Join(dep, "go.mod")
			}
			old, _ := os.ReadFile(path)
			comment := "\n// changed\n"
			if which == "toolchain" {
				comment = "\n# changed\n"
			}
			writeFixture(t, path, string(old)+comment)
			next, err := stateIdentity(s, o)
			if err != nil {
				t.Fatal(err)
			}
			if next == base {
				t.Fatalf("identity ignored %s", which)
			}
			writeFixture(t, path, string(old))
			restored, err := stateIdentity(s, o)
			if err != nil || restored != base {
				t.Fatalf("restored %s did not restore identity: %v", which, err)
			}
		}
	})
}

// regression: F-state-errors
func TestStateSkipsNoErrors(t *testing.T) {
	f := newStateFixture(t)
	bin := muharTestBinary(t)
	if out, code := runStateCmd(t, bin, f); code != 0 {
		t.Fatalf("seed header %d: %s", code, out)
	}
	header := readStateLines(t, f.state)[0]
	raw, _ := json.Marshal(header)
	writeFixture(t, f.state, string(raw)+"\n")
	if err := os.Remove(f.trace); err != nil {
		t.Fatal(err)
	}
	state, err := openRunState(f.state)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.bind(header.Identity); err != nil {
		t.Fatal(err)
	}
	records := []stateEntry{{Kind: "broken", Verdict: "HARNESS BROKEN", Complete: true}, {Kind: "interrupted", Index: 0, Verdict: "CAUGHT", Complete: true}, {Kind: "mutation", Index: 0, Verdict: "CAUGHT", Complete: false, Selection: "full command"}, {Kind: "mutation", Index: 1, Verdict: "CAUGHT", Complete: true, Error: "infrastructure error", Selection: "full command"}, {Kind: "mutation", Index: 2, Verdict: "COULD_NOT_MUTATE", Complete: true, Selection: "full command"}}
	for _, r := range records {
		if err := state.append(r); err != nil {
			t.Fatal(err)
		}
	}
	// A syntactically valid line with a completion flag still is not complete
	// when required timing fields are absent.
	_, err = state.file.WriteString(`{"kind":"mutation","index":2,"verdict":"CAUGHT","complete":true,"selection":"full command"}` + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := state.file.Sync(); err != nil {
		t.Fatal(err)
	}
	state.close()
	out, code := runStateCmd(t, bin, f)
	if code != 0 {
		t.Fatalf("run %d: %s", code, out)
	}
	trace, _ := os.ReadFile(f.trace)
	for _, want := range []string{"first=2", "second=2", "# changed"} {
		if !strings.Contains(string(trace), want) {
			t.Errorf("reused error instead of executing %s", want)
		}
	}
	t.Run("external-signal", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "value.txt"), "one")
		state, err := openRunState(filepath.Join(t.TempDir(), "signal.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		defer state.close()
		if err := state.bind("signal-case"); err != nil {
			t.Fatal(err)
		}
		h := Harness{Root: root, State: state, MutationTimeout: time.Second, Mutations: []Mutation{{Name: "killed", File: "value.txt", Old: "one", New: "two"}}, RunTest: func(root string, limit time.Duration) CommandResult {
			return runShell(root, "kill -KILL $$", os.Environ(), limit)
		}}
		result, err := h.completedMutation(root, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if result.Result != CouldNotMutate || result.Err == nil || !strings.Contains(result.Err.Error(), "external signal") {
			t.Fatalf("externally killed suite became a verdict: %+v", result)
		}
		records := readStateLines(t, state.file.Name())
		if len(records) != 2 || records[1].definitive() {
			t.Fatalf("external signal reused: %+v", records)
		}
	})

	t.Run("restore-error", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "value.txt")
		writeFixture(t, path, "original")
		state, err := openRunState(filepath.Join(t.TempDir(), "restore.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		defer state.close()
		if err := state.bind("restore-case"); err != nil {
			t.Fatal(err)
		}
		h := Harness{Root: root, State: state, Mutations: []Mutation{{Name: "restore", File: "value.txt", Old: "original", New: "changed"}}, Test: func(string) bool {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			return true
		}}
		result, err := h.completedMutation(root, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if result.Result != CouldNotMutate || result.Err == nil || !strings.Contains(result.Err.Error(), "restore") {
			t.Fatalf("restore error became definitive: %+v", result)
		}
		records := readStateLines(t, state.file.Name())
		if len(records) != 2 || records[1].definitive() {
			t.Fatalf("restore error reusable: %+v", records)
		}
	})

}

// regression: F-state-end-to-end
func TestStateEndToEnd(t *testing.T) {
	f := newStateFixture(t)
	bin := muharTestBinary(t)
	writeFixture(t, f.hold, "hold")
	cmd := stateCommand(bin, f)
	var holder bytes.Buffer
	cmd.Stdout = &holder
	cmd.Stderr = &holder
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	waitStateFile(t, f.marker)
	before, _ := os.ReadFile(f.state)
	out, code := runStateCmd(t, bin, f)
	after, _ := os.ReadFile(f.state)
	if code != 1 || !strings.Contains(out, "writer lock") || !bytes.Equal(before, after) {
		t.Fatalf("writer race: %d %s", code, out)
	}
	if err := os.Remove(f.hold); err != nil {
		t.Fatal(err)
	}
	stopStateCmd(t, cmd)
	resumed, code := runStateCmd(t, bin, f, "-j", "2")
	if code != 0 {
		t.Fatalf("resumed %d: %s", code, resumed)
	}
	second, code := runStateCmd(t, bin, f, "-j", "1")
	if code != 0 || second != resumed {
		t.Fatalf("resume parity %d\n%s\n%s", code, second, resumed)
	}
	normal, err := exec.Command(bin, "-spec", f.spec).Output()
	if err != nil || string(normal) != resumed {
		t.Fatalf("ordinary report parity %v\n%s\n%s", err, normal, resumed)
	}
	otherState := f.state
	f.state = filepath.Join(t.TempDir(), "full.jsonl")
	full, code := runStateCmd(t, bin, f, "-j", "1")
	if code != 0 || full != resumed {
		t.Fatalf("uninterrupted parity %d\n%s\n%s", code, full, resumed)
	}
	records := readStateLines(t, otherState)
	seen := map[int]bool{}
	for _, r := range records {
		if r.Kind == "mutation" {
			seen[r.Index] = true
		}
	}
	if len(seen) != 4 {
		t.Fatalf("missing spec indices: %v", seen)
	}
	t.Run("shard-indices", func(t *testing.T) {
		f.state = filepath.Join(t.TempDir(), "shard.jsonl")
		if out, code := runStateCmd(t, bin, f, "-shard", "1/2"); code != 0 {
			t.Fatalf("shard %d %s", code, out)
		}
		var got []int
		for _, r := range readStateLines(t, f.state) {
			if r.Kind == "mutation" {
				got = append(got, r.Index)
			}
		}
		if fmt.Sprint(got) != "[1 3]" {
			t.Fatalf("indices = %v", got)
		}
	})
	t.Run("selection-timing", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, filepath.Join(root, "value.txt"), "one")
			state, err := openRunState(filepath.Join(t.TempDir(), "timing.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			defer state.close()
			if err := state.bind("timing-case"); err != nil {
				t.Fatal(err)
			}
			h := Harness{Root: root, State: state, Selector: stateDelaySelector{}, Mutations: []Mutation{{Name: "uncovered", File: "value.txt", Old: "one", New: "two"}}}
			if _, err := h.completedMutation(root, nil, 0); err != nil {
				t.Fatal(err)
			}
			r := readStateLines(t, state.file.Name())[1]
			if r.TestNS != 0 || r.Selection != "skip: uncovered" {
				t.Fatalf("selection decision counted as a test: %+v", r)
			}
		})
	})
	t.Run("schemata-timing", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, filepath.Join(root, "value.txt"), "one")
			state, err := openRunState(filepath.Join(t.TempDir(), "schema.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			defer state.close()
			if err := state.bind("schema-case"); err != nil {
				t.Fatal(err)
			}
			h := Harness{Root: root, State: state, Mutations: []Mutation{{Name: "schema", File: "value.txt", Old: "one", New: "two"}}, Build: func(string, Mutation, time.Duration) CommandResult { time.Sleep(time.Second); return CommandResult{} }, RunTest: func(string, time.Duration) CommandResult { time.Sleep(3 * time.Second); return CommandResult{} }, SchemaRun: func(string, string, string, time.Duration) CommandResult {
				time.Sleep(2 * time.Second)
				return CommandResult{}
			}}
			tree := &schemaTree{ids: map[string]bool{schemaID(0): true}}
			if _, err := h.completedMutation(root, tree, 0); err != nil {
				t.Fatal(err)
			}
			r := readStateLines(t, state.file.Name())[1]
			if r.Selection != "schemata fallback -> full command" || r.TestNS != int64(5*time.Second) || r.BuildNS != int64(time.Second) {
				t.Fatalf("schemata fallback misreported: %+v", r)
			}
		})
	})

}

type stateDelaySelector struct{}

func (stateDelaySelector) Prepare(string) error          { return nil }
func (stateDelaySelector) Narrows(Mutation, []byte) bool { return true }
func (stateDelaySelector) For(Mutation, []byte) Selection {
	time.Sleep(time.Hour)
	return Selection{Skip: true}
}
