package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const stateSchema = 1

// State owns observations, never verdict production. The locked inode is kept
// for the entire invocation; it is never renamed or replaced under a waiter.
type runState struct {
	file    *os.File
	mu      sync.Mutex
	records map[int]stateEntry
}

type stateEntry struct {
	Kind       string `json:"kind"`
	Schema     int    `json:"schema,omitempty"`
	Identity   string `json:"identity,omitempty"`
	Index      int    `json:"index"`
	Verdict    string `json:"verdict,omitempty"`
	Complete   bool   `json:"complete,omitempty"`
	Error      string `json:"error,omitempty"`
	Timeout    string `json:"timeout,omitempty"`
	LimitNS    int64  `json:"limit_ns,omitempty"`
	BuildNS    int64  `json:"build_ns"`
	TestNS     int64  `json:"test_ns"`
	BaselineNS int64  `json:"baseline_ns,omitempty"`
	ControlNS  int64  `json:"control_ns,omitempty"`
	Selection  string `json:"selection,omitempty"`
}

func openRunState(path string) (*runState, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	if err := deskkit.TryLockExclusive(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("writer lock: %w", err)
	}
	return &runState{file: f, records: map[int]stateEntry{}}, nil
}

func (s *runState) close() {
	_ = deskkit.UnlockFile(s.file)
	_ = s.file.Close()
}

func (s *runState) bind(identity string) error {
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(s.file)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return s.append(stateEntry{Kind: "header", Schema: stateSchema, Identity: identity})
	}
	// Refuse, rather than repair, a torn tail. In particular never skip over a
	// malformed record to find an apparently completed mutation after it.
	if raw[len(raw)-1] != '\n' {
		return fmt.Errorf("incomplete trailing record; use a new state path or repair the trailing record offline")
	}
	for i, line := range bytes.Split(raw[:len(raw)-1], []byte{'\n'}) {
		var r stateEntry
		if err := json.Unmarshal(line, &r); err != nil {
			return fmt.Errorf("invalid record %d: %w", i, err)
		}
		if i == 0 {
			if r.Kind != "header" || r.Schema != stateSchema || r.Identity != identity {
				return fmt.Errorf("identity/schema mismatch; use a new state path")
			}
			continue
		}
		switch r.Kind {
		case "mutation":
			// A later failed attempt invalidates any earlier completion at this index.
			delete(s.records, r.Index)
			if r.definitive() {
				s.records[r.Index] = r
			}
		case "phases", "broken", "interrupted":
		default:
			return fmt.Errorf("unknown state record kind %q", r.Kind)
		}
	}
	return nil
}

func (r stateEntry) definitive() bool {
	return r.Kind == "mutation" && r.Complete && r.Index >= 0 && r.Error == "" &&
		(r.Timeout == "" || (r.Timeout == "test" && r.Verdict == "CAUGHT" && r.LimitNS > 0)) &&
		(r.Verdict == "CAUGHT" || r.Verdict == "NOT_CAUGHT") && r.BuildNS >= 0 && r.TestNS >= 0 && r.Selection != ""
}

func (s *runState) append(r stateEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	n, err := s.file.Write(b) // one append under the writer lock, no buffered tail
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return s.file.Sync()
}

type mutationTiming struct {
	Build, Test        time.Duration
	Timeout, Selection string
}

func (h *Harness) stopped() bool { return h.Interrupted != nil && h.Interrupted() }

func (h *Harness) completedMutation(root string, st *schemaTree, i int) (MutationResult, error) {
	index := i
	if h.Indices != nil {
		index = h.Indices[i]
	}
	m := h.Mutations[i]
	if h.State != nil {
		if r, ok := h.State.records[index]; ok {
			result := NotCaught
			if r.Verdict == "CAUGHT" {
				result = Caught
			}
			var err error
			if r.Timeout == "test" {
				err = suiteTimeout{time.Duration(r.LimitNS)}
			}
			return MutationResult{Mutation: m, Result: result, Err: err}, nil
		}
	}
	local := *h
	timing := &mutationTiming{Selection: "full command"}
	local.observation = timing
	// Schemata executes outside runOne, so account for it at the callback.
	if h.SchemaRun != nil {
		local.SchemaRun = func(root, overlay, selector string, timeout time.Duration) CommandResult {
			start := time.Now()
			out := h.SchemaRun(root, overlay, selector, timeout)
			timing.Test += time.Since(start)
			timing.Selection = "schemata: full command"
			return out
		}
	}
	if h.SchemaTest != nil {
		local.SchemaTest = func(root, overlay, selector string) bool {
			start := time.Now()
			red := h.SchemaTest(root, overlay, selector)
			timing.Test += time.Since(start)
			timing.Selection = "schemata: full command"
			return red
		}
	}
	result, err := local.runMutation(root, st, i)
	mr := MutationResult{Mutation: m, Result: result, Err: err}
	if h.State == nil || h.stopped() {
		return mr, nil
	}
	r := stateEntry{Kind: "mutation", Index: index, Verdict: result.String(), Complete: true,
		BuildNS: timing.Build.Nanoseconds(), TestNS: timing.Test.Nanoseconds(), Selection: timing.Selection, Timeout: timing.Timeout}
	var timeout suiteTimeout
	if errors.As(err, &timeout) {
		r.Timeout = "test"
		r.LimitNS = timeout.limit.Nanoseconds()
	} else if err != nil {
		r.Error = err.Error()
	}
	if err := h.State.append(r); err != nil {
		return mr, fmt.Errorf("persist mutation %d: %w", index, err)
	}
	return mr, nil
}

// Identity deliberately includes the complete input tree as well as the Go
// compilation closure. This conservative superset also covers shell fixtures,
// embedded files and source-reading tests. Generated output must live outside
// root. State is the sole excluded file; .git metadata is not a source input.
func stateIdentity(s spec, o options) (string, error) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	statePath, err := filepath.Abs(o.state)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(statePath); err == nil {
		statePath = real
	}
	files := map[string]bool{}
	var addTree func(string) error
	addTree = func(base string) error {
		return filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == statePath {
				return nil
			}
			if d.Name() == ".git" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				info, err := os.Stat(path)
				if err != nil {
					return err
				}
				if info.IsDir() {
					return fmt.Errorf("state identity cannot enumerate directory symlink %s", path)
				}
			}
			if !d.IsDir() {
				files[path] = true
			}
			return nil
		})
	}
	if err := addTree(root); err != nil {
		return "", err
	}
	// Bind source locations too: tests may observe their absolute working path.
	values := []string{fmt.Sprint(stateSchema), runtime.Version(), root, s.Test, s.Build, o.shard, fmt.Sprint(o.useSelect)}
	raw, err := os.ReadFile(o.spec)
	if err != nil {
		return "", err
	}
	values = append(values, string(raw))
	// Hash the inherited environment without publishing it. GOCACHE and TMPDIR
	// name disposable output and never decide the compilation/test identity.
	env := envWithout(os.Environ(), "GOCACHE", "TMPDIR", "TMP", "TEMP")
	sort.Strings(env)
	values = append(values, env...)
	for _, m := range append([]Mutation{s.Control}, s.Mutations...) {
		if m.File != "" {
			files[resolve(root, m.File)] = true
		}
	}
	// Plain Go test commands expose their exact package closure through go list.
	// A shell composition containing Go cannot be resolved safely: fail closed.
	args, assigns, isGo, err := stateGoArgs(s.Test)
	if err != nil {
		return "", err
	}
	if isGo {
		goPath, err := exec.LookPath("go")
		if err != nil {
			return "", err
		}
		files[goPath] = true
		cmd := exec.Command("go", append([]string{"list", "-deps", "-test", "-json"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), assigns...)
		out, err := trackedOutput(cmd, false)
		if err != nil {
			return "", fmt.Errorf("resolve Go compilation inputs: %w", err)
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		for {
			var p struct {
				Dir                                                                                                                                                                             string
				GoFiles, CgoFiles, TestGoFiles, XTestGoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles, EmbedFiles, TestEmbedFiles, XTestEmbedFiles []string
				Module                                                                                                                                                                          *struct{ GoMod string }
			}
			if err := dec.Decode(&p); err == io.EOF {
				break
			} else if err != nil {
				return "", err
			}
			for _, group := range [][]string{p.GoFiles, p.CgoFiles, p.TestGoFiles, p.XTestGoFiles, p.CFiles, p.CXXFiles, p.MFiles, p.HFiles, p.FFiles, p.SFiles, p.SwigFiles, p.SwigCXXFiles, p.SysoFiles, p.EmbedFiles, p.TestEmbedFiles, p.XTestEmbedFiles} {
				for _, name := range group {
					files[resolve(p.Dir, name)] = true
				}
			}
			if p.Module != nil && p.Module.GoMod != "" {
				files[p.Module.GoMod] = true
				sum := strings.TrimSuffix(p.Module.GoMod, ".mod") + ".sum"
				if _, err := os.Stat(sum); err == nil {
					files[sum] = true
				}
			}
		}
		cmd = exec.Command("go", "env", "-json")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), assigns...)
		out, err = trackedOutput(cmd, false)
		if err != nil {
			return "", err
		}
		var settings map[string]string
		if err := json.Unmarshal(out, &settings); err != nil {
			return "", err
		}
		for _, k := range []string{"GOCACHE", "GOTMPDIR", "GOTELEMETRYDIR"} {
			delete(settings, k)
		}
		for _, k := range []string{"GOMOD", "GOWORK", "GOENV"} {
			if path := settings[k]; path != "" && path != "off" && path != os.DevNull {
				if _, err := os.Stat(path); err == nil {
					files[path] = true
					if k == "GOWORK" {
						if _, err := os.Stat(path + ".sum"); err == nil {
							files[path+".sum"] = true
						}
					}
				}
			}
		}
		toolSettings, _ := json.Marshal(settings)
		values = append(values, string(toolSettings))
	}
	keys := make([]string, 0, len(files))
	for path := range files {
		if path != statePath {
			keys = append(keys, path)
		}
	}
	sort.Strings(keys)
	digest := sha256.New()
	put := func(b []byte) { _, _ = fmt.Fprintf(digest, "%d:", len(b)); _, _ = digest.Write(b) }
	for _, value := range values {
		put([]byte(value))
	}
	for _, path := range keys {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("identity input %s: %w", path, err)
		}
		put([]byte(path))
		put(b)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func stateGoArgs(command string) (args, assigns []string, isGo bool, err error) {
	toks, e := shellWords(command)
	if e != nil {
		if strings.Contains(command, "go ") {
			return nil, nil, false, fmt.Errorf("-state cannot resolve Go shell composition: %w", e)
		}
		return nil, nil, false, nil
	}
	for len(toks) > 0 && strings.Contains(toks[0], "=") && !strings.HasPrefix(toks[0], "-") {
		name, _, _ := strings.Cut(toks[0], "=")
		if !envName.MatchString(name) {
			break
		}
		assigns = append(assigns, toks[0])
		toks = toks[1:]
	}
	if len(toks) < 2 || toks[0] != "go" || toks[1] != "test" {
		if strings.Contains(command, "go ") {
			return nil, nil, false, fmt.Errorf("-state requires a plain go test command to resolve Go inputs")
		}
		return nil, assigns, false, nil
	}
	for i := 2; i < len(toks); i++ {
		arg := toks[i]
		if !strings.HasPrefix(arg, "-") {
			args = append(args, arg)
			continue
		}
		name, _, hasValue := strings.Cut(arg, "=")
		if strings.HasPrefix(name, "--") {
			name = name[1:]
		}
		// External compiler/overlay inputs cannot be inferred from package metadata.
		switch name {
		case "-overlay", "-modfile", "-pgo", "-toolexec", "-exec":
			return nil, nil, false, fmt.Errorf("-state cannot resolve external inputs for %s", name)
		}
		switch goTestFlags[name] {
		case flagDropBool:
		case flagDropValue:
			if !hasValue {
				i++
			}
		case flagKeepBool:
			args = append(args, arg)
		case flagKeepValue:
			args = append(args, arg)
			if !hasValue {
				i++
				if i >= len(toks) {
					return nil, nil, false, fmt.Errorf("missing %s value", name)
				}
				args = append(args, toks[i])
			}
		default:
			return nil, nil, false, fmt.Errorf("-state cannot resolve flag %s", name)
		}
	}
	return args, assigns, true, nil
}
