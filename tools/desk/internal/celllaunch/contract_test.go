package celllaunch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The Go test executable doubles as both compiled fixture runner and compiled
// child. No shell, source build subprocess, production runner, or credentials
// are used. Its permit argument is independently supplied by the test driver.
func init() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "--celllaunch-child":
		if len(os.Args) < 3 {
			os.Exit(79)
		}
		code, err := strconv.Atoi(os.Args[2])
		if err != nil {
			os.Exit(79)
		}
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(79)
		}
		env := os.Environ()
		sort.Strings(env)
		_ = json.NewEncoder(os.Stdout).Encode(childResult{os.Args[3:], env, cwd})
		os.Exit(code)
	case "--celllaunch-runner":
		os.Exit(fixtureRun())
	}
}

func fixtureRun() int {
	refuse := func() int { fmt.Fprintln(os.Stderr, "fixture launch refused"); return 78 }
	if len(os.Args) != 4 {
		return refuse()
	}
	expected, err := ParseLaunch([]byte(os.Args[2]))
	if err != nil {
		return refuse()
	}
	permit, err := NewPermit(expected)
	if err != nil {
		return refuse()
	}
	var ref SpecRef
	if json.Unmarshal([]byte(os.Args[3]), &ref) != nil {
		return refuse()
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, MaxRecordBytes+1))
	if err != nil {
		return refuse()
	}
	s, err := BindLaunch(data, ref, permit)
	if err != nil {
		return refuse()
	}
	if len(s.Credentials) != 0 || s.NativePaths(runtime.GOOS) != nil {
		return refuse()
	}
	cmd := exec.Command(s.Executable, s.Args...)
	cmd.Dir = s.Cwd
	cmd.Env = []string{}
	for k, v := range s.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err = cmd.Run(); err != nil {
		if x, ok := err.(*exec.ExitError); ok {
			return x.ExitCode()
		}
		return refuse()
	}
	return 0
}

type childResult struct {
	Args []string
	Env  []string
	Cwd  string
}

func fixtureSpec(t *testing.T) LaunchSpec {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// Canonicalize the test root once; macOS temp directories may have an alias.
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.Join(dir, "work space's &%()雪")
	if err = os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"LANG": "en_US.UTF-8", "HOME": dir, "TMPDIR": dir}
	if runtime.GOOS == "windows" {
		env["SYSTEMROOT"] = os.Getenv("SYSTEMROOT")
	}
	return LaunchSpec{Schema: LaunchSchema, LaunchID: "launch-1", Cell: "sample", Role: "worker-desk", OS: runtime.GOOS, Intent: "host", Cockpit: "orca", Operation: "harness", Executable: exe, Args: []string{"--celllaunch-child", "0"}, Env: env, Cwd: dir}
}

func reference(data []byte, id string) SpecRef {
	sum := sha256.Sum256(data)
	return SpecRef{ID: id, Digest: hex.EncodeToString(sum[:])}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func runFixture(t *testing.T, expected LaunchSpec, data []byte, ref SpecRef) ([]byte, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "--celllaunch-runner", string(mustJSON(t, expected)), string(mustJSON(t, ref)))
	cmd.Stdin = bytes.NewReader(data)
	// Deliberately hostile ambient values must not reach the child or diagnostics.
	cmd.Env = append(os.Environ(), "GH_TOKEN=synthetic-secret-never-forward", "AWS_SECRET_ACCESS_KEY=synthetic-cloud-never-forward")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	code := 0
	if err != nil {
		if x, ok := err.(*exec.ExitError); ok {
			code = x.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return out.Bytes(), stderr.String(), code
}

func TestLaunchSpecRoundTrip(t *testing.T) {
	for _, code := range []int{0, 23} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			s := fixtureSpec(t)
			s.Args = append([]string{"--celllaunch-child", strconv.Itoa(code)}, "", "two words", "apostrophe's", `quote"double`, "a&b", "%PATH%", "(role)", "雪", `C:\trailing\`, "line\nvalue")
			data := mustJSON(t, s)
			parsed, err := ParseLaunch(data)
			if err != nil || !reflect.DeepEqual(parsed, s) {
				t.Fatalf("roundtrip: %v", err)
			}
			out, stderr, gotCode := runFixture(t, s, data, reference(data, s.LaunchID))
			if gotCode != code || stderr != "" {
				t.Fatalf("exit=%d want=%d stderr=%s", gotCode, code, stderr)
			}
			var got childResult
			if err = json.Unmarshal(out, &got); err != nil {
				t.Fatalf("child output: %v (%s)", err, out)
			}
			wantEnv := []string{}
			for k, v := range s.Env {
				wantEnv = append(wantEnv, k+"="+v)
			}
			sort.Strings(wantEnv)
			if !reflect.DeepEqual(got.Args, s.Args[2:]) || !reflect.DeepEqual(got.Env, wantEnv) || got.Cwd != s.Cwd {
				t.Fatalf("child differed: %#v", got)
			}
		})
	}
	t.Run("strict-json", func(t *testing.T) {
		s := fixtureSpec(t)
		valid := mustJSON(t, s)
		for name, data := range map[string][]byte{
			"unknown":      append([]byte(`{"unexpected":true,`), valid[1:]...),
			"duplicate":    append([]byte(`{"schema":"wrong",`), valid[1:]...),
			"case-alias":   append([]byte(`{"Schema":"wrong",`), valid[1:]...),
			"single-alias": bytes.Replace(valid, []byte(`"schema"`), []byte(`"Schema"`), 1),
			"trailing":     append(append([]byte{}, valid...), []byte(` {}`)...),
			"invalid-utf8": bytes.Replace(valid, []byte("en_US.UTF-8"), []byte{0xff}, 1),
			"null-arg":     bytes.Replace(valid, []byte(`"--celllaunch-child"`), []byte(`null`), 1),
			"null-env":     bytes.Replace(valid, []byte(`"en_US.UTF-8"`), []byte(`null`), 1),
			"oversize":     bytes.Repeat([]byte(" "), MaxRecordBytes+1),
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := ParseLaunch(data); err == nil {
					t.Fatal("malformed record accepted")
				}
			})
		}
		s.Args = make([]string, 20)
		for i := range s.Args {
			s.Args[i] = strings.Repeat("x", 16384)
		}
		if _, err := json.Marshal(s); err == nil {
			t.Fatal("serialized unparseable oversized launch")
		}
	})
}

func readySession(t *testing.T) SessionRecord {
	s := fixtureSpec(t)
	b := mustJSON(t, s)
	return SessionRecord{Schema: SessionSchema, LaunchID: s.LaunchID, Cell: s.Cell, Role: s.Role, OS: s.OS, Intent: s.Intent, State: "ready", Spec: reference(b, s.LaunchID), Console: &ConsoleIdentity{Backend: "orca", Workspace: "workspace-1", Handle: "terminal-1", LaunchID: s.LaunchID}, Process: &ProcessIdentity{PID: 42, Created: "start-1", Executable: s.Executable, Owner: "owner-1", Tree: "tree-1"}}
}

func TestSessionRecordRefusal(t *testing.T) {
	valid := readySession(t)
	if err := valid.Match(Observation{Owned, valid}); err != nil {
		t.Fatal(err)
	}
	for _, state := range []ObservationState{Missing, Foreign, Unknown, ""} {
		if err := valid.Match(Observation{state, valid}); err == nil {
			t.Fatalf("accepted observation %q", state)
		}
	}
	changes := map[string]func(*SessionRecord){
		"schema":           func(s *SessionRecord) { s.Schema = "v2" },
		"foreign-cell":     func(s *SessionRecord) { s.Cell = "foreign" },
		"foreign-role":     func(s *SessionRecord) { s.Role = "the-desk" },
		"reused-pid":       func(s *SessionRecord) { s.Process.Created = "start-2" },
		"foreign-owner":    func(s *SessionRecord) { s.Process.Owner = "other" },
		"foreign-tree":     func(s *SessionRecord) { s.Process.Tree = "other" },
		"other-terminal":   func(s *SessionRecord) { s.Console.Handle = "terminal-2" },
		"other-backend":    func(s *SessionRecord) { s.Console.Backend = "herdr" },
		"pending":          func(s *SessionRecord) { s.State = "pending" },
		"exited":           func(s *SessionRecord) { s.State = "exited" },
		"missing-handle":   func(s *SessionRecord) { s.Console = nil },
		"missing-workload": func(s *SessionRecord) { s.Process = nil },
		"no-start":         func(s *SessionRecord) { s.Process.Created = "" },
		"zero-pid":         func(s *SessionRecord) { s.Process.PID = 0 },
		"wrong-generation": func(s *SessionRecord) { s.Spec.Digest = strings.Repeat("b", 64) },
		"mixed-kind":       func(s *SessionRecord) { s.Container = &ContainerIdentity{} },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			clone, err := ParseSession(mustJSON(t, valid))
			if err != nil {
				t.Fatal(err)
			}
			change(&clone)
			if err = valid.Match(Observation{Owned, clone}); err == nil {
				t.Fatal("accepted mismatched identity")
			}
		})
	}
	container := valid
	container.Intent = "linux-container"
	container.Process = nil
	container.Container = &ContainerIdentity{Endpoint: "local-engine", Engine: "daemon-1", ID: strings.Repeat("a", 64), Cell: valid.Cell, Role: valid.Role, PolicyDigest: strings.Repeat("c", 64)}
	if err := container.Match(Observation{Owned, container}); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*ContainerIdentity){
		"endpoint":           func(c *ContainerIdentity) { c.Endpoint = "other-local-engine" },
		"engine":             func(c *ContainerIdentity) { c.Engine = "daemon-2" },
		"same-name-other-id": func(c *ContainerIdentity) { c.ID = strings.Repeat("d", 64) },
		"short-id":           func(c *ContainerIdentity) { c.ID = "abc" },
		"isolation":          func(c *ContainerIdentity) { c.PolicyDigest = strings.Repeat("e", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			clone, err := ParseSession(mustJSON(t, container))
			if err != nil {
				t.Fatal(err)
			}
			change(clone.Container)
			if err = container.Match(Observation{Owned, clone}); err == nil {
				t.Fatal("accepted mismatched container")
			}
		})
	}
	pending := valid
	pending.State = "pending"
	pending.Console = nil
	pending.Process = nil
	if err := pending.Validate(); err != nil {
		t.Fatalf("diagnostic pending record rejected: %v", err)
	}
	if err := pending.Match(Observation{Owned, pending}); err == nil {
		t.Fatal("pending state authorized ownership")
	}
	// Independently confirmed component ownership remains useful during rollback,
	// even before a launch has produced every handle needed for ready state.
	partial := valid
	partial.State = "pending"
	partial.Process = nil
	co := ConsoleObservation{Owned, partial.Scope(), *partial.Console}
	if err := partial.MatchConsole(co); err != nil {
		t.Fatal(err)
	}
	po := ProcessObservation{Owned, valid.Scope(), *valid.Process}
	if err := valid.MatchProcess(po); err != nil {
		t.Fatal(err)
	}
	processOnly := valid
	processOnly.State = "pending"
	processOnly.Console = nil
	if err := processOnly.MatchProcess(po); err != nil {
		t.Fatal(err)
	}
	survivor := container
	survivor.State = "pending"
	survivor.Console = nil
	io := ContainerObservation{Owned, survivor.Scope(), *survivor.Container}
	if err := survivor.MatchContainer(io); err != nil {
		t.Fatal(err)
	}
	for _, state := range []ObservationState{Missing, Foreign, Unknown, ""} {
		co.State = state
		po.State = state
		io.State = state
		if partial.MatchConsole(co) == nil || valid.MatchProcess(po) == nil || survivor.MatchContainer(io) == nil {
			t.Fatalf("component accepted %q", state)
		}
	}
	co.State = Owned
	po.State = Owned
	io.State = Owned
	co.Identity.Handle = "foreign"
	po.Identity.Created = "reused"
	io.Identity.ID = strings.Repeat("f", 64)
	if partial.MatchConsole(co) == nil || valid.MatchProcess(po) == nil || survivor.MatchContainer(io) == nil {
		t.Fatal("component accepted wrong resource")
	}
	co.Identity = *partial.Console
	po.Identity = *valid.Process
	io.Identity = *survivor.Container
	co.Scope.Role = "the-desk"
	po.Scope.Cell = "foreign"
	io.Scope.LaunchID = "launch-2"
	if partial.MatchConsole(co) == nil || valid.MatchProcess(po) == nil || survivor.MatchContainer(io) == nil {
		t.Fatal("component accepted wrong scope")
	}
	empty := pending
	if empty.MatchConsole(ConsoleObservation{Owned, empty.Scope(), *valid.Console}) == nil || empty.MatchProcess(ProcessObservation{Owned, empty.Scope(), *valid.Process}) == nil {
		t.Fatal("absent component treated as owned")
	}
	empty.Intent = "linux-container"
	if empty.MatchContainer(ContainerObservation{Owned, empty.Scope(), *container.Container}) == nil {
		t.Fatal("absent container treated as owned")
	}
}

func TestLaunchSpecCustody(t *testing.T) {
	s := fixtureSpec(t)
	data := mustJSON(t, s)
	ref := reference(data, s.LaunchID)
	for _, id := range []string{"CON", "con", "NUL", "AUX", "PRN", "COM1", "com9", "LPT1", "lpt9"} {
		reserved := ref
		reserved.ID = id
		if err := reserved.Validate(); err == nil {
			t.Fatalf("reserved device reference %q accepted", id)
		}
	}
	permit, err := NewPermit(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = BindLaunch(data, ref, permit); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../escape", "C:\\escape", `\\server\share`, "root/file", "id:stream", ".hidden", "", "launch-2"} {
		r := ref
		r.ID = id
		if _, err = BindLaunch(data, r, permit); err == nil {
			t.Fatalf("untrusted reference %q accepted", id)
		}
	}
	r := ref
	r.Digest = strings.Repeat("0", 64)
	if _, err = BindLaunch(data, r, permit); err == nil {
		t.Fatal("changed bytes accepted")
	}
	for name, change := range map[string]func(*LaunchSpec){
		"executable":  func(s *LaunchSpec) { s.Executable = filepath.Join(s.Cwd, "other-executable") },
		"cell":        func(s *LaunchSpec) { s.Cell = "another-cell" },
		"role":        func(s *LaunchSpec) { s.Role = "the-desk" },
		"generation":  func(s *LaunchSpec) { s.LaunchID = "launch-2" },
		"operation":   func(s *LaunchSpec) { s.Operation = "deskd" },
		"cwd":         func(s *LaunchSpec) { s.Cwd = filepath.Dir(s.Cwd) },
		"argument":    func(s *LaunchSpec) { s.Args[1] = "23" },
		"environment": func(s *LaunchSpec) { s.Env["LANG"] = "changed" },
	} {
		t.Run(name, func(t *testing.T) {
			mut, err := ParseLaunch(data)
			if err != nil {
				t.Fatal(err)
			}
			change(&mut)
			b := mustJSON(t, mut)
			if _, err = BindLaunch(b, reference(b, mut.LaunchID), permit); err == nil {
				t.Fatal("changed authority accepted")
			}
			_, stderr, code := runFixture(t, s, b, reference(b, mut.LaunchID))
			if code != 78 || stderr != "fixture launch refused\n" {
				t.Fatalf("runner refusal=%d %q", code, stderr)
			}
		})
	}
	// Permit construction freezes nested maps/slices rather than sharing authority
	// with the caller's later mutable configuration.
	s.Args[1] = "23"
	s.Env["LANG"] = "changed"
	if err = permit.Check(s); err == nil {
		t.Fatal("permit mutated with its source")
	}
	for _, key := range []string{"GH_TOKEN", "AWS_SECRET_ACCESS_KEY", "LD_PRELOAD", "NODE_OPTIONS", "CUSTOM_TOKEN"} {
		mut, _ := ParseLaunch(data)
		mut.Env[key] = "synthetic-secret-never-serialize"
		if b, err := json.Marshal(mut); err == nil || bytes.Contains(b, []byte("synthetic-secret")) {
			t.Fatalf("secret env %s serialized", key)
		}
	}
	mut, _ := ParseLaunch(data)
	mut.Env["PATH"] = "one"
	mut.Env["Path"] = "two"
	if _, err = json.Marshal(mut); err == nil {
		t.Fatal("case duplicate environment accepted")
	}
	mut, _ = ParseLaunch(data)
	mut.Credentials = []CredentialRef{{Kind: "forge-role", Role: mut.Role, Provider: "github"}}
	encoded := mustJSON(t, mut)
	if _, err = ParseLaunch(encoded); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"anthropic", "glm", "kimi", "codex"} {
		profile, _ := ParseLaunch(data)
		profile.Credentials = []CredentialRef{{Kind: "model-provider", Role: profile.Role, Provider: provider}}
		if _, err = ParseLaunch(mustJSON(t, profile)); err != nil {
			t.Fatal(err)
		}
	}
	for _, bad := range []CredentialRef{{Kind: "file", Role: mut.Role, Provider: "github"}, {Kind: "forge-role", Role: "the-desk", Provider: "github"}, {Kind: "model-provider", Role: mut.Role, Provider: "../file-path"}} {
		mut.Credentials = []CredentialRef{bad}
		if _, err = json.Marshal(mut); err == nil {
			t.Fatal("invalid credential reference accepted")
		}
	}
	for _, bad := range [][]byte{
		bytes.Replace(encoded, []byte(`"provider":"github"`), []byte(`"provider":"github","Provider":"gitlab"`), 1),
		bytes.Replace(encoded, []byte(`"provider":"github"`), []byte(`"provider":"github","token":"synthetic-secret"`), 1),
	} {
		if _, err = ParseLaunch(bad); err == nil {
			t.Fatal("credential JSON widening accepted")
		}
	}
	request := RunnerRequest{Runner: fixtureSpec(t).Executable, Ref: ref, Cell: "sample", Role: "worker-desk", OS: runtime.GOOS}
	if err = request.Validate(); err != nil {
		t.Fatal(err)
	}
	request.Ref.ID = "../foreign-root"
	if err = request.Validate(); err == nil {
		t.Fatal("foreign root accepted in runner request")
	}
}
