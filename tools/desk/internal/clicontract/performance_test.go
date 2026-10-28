package clicontract

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/spf13/cobra"
)

// regression: desktools-v2/64
func TestPerfV3CLIContextPrecedence(t *testing.T) {
	if _, ok := cli.PerformanceContext(cli.NewRoot("fresh", "unselected fixture")); ok {
		t.Fatal("unselected command has context")
	}
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("ASSAY_PERF_PROFILE", "workflow")
	t.Setenv("ASSAY_PERF_ROLE", "implement")
	t.Setenv("ASSAY_PERF_SESSION_ID", strings.Repeat("a", 32))
	t.Setenv("ASSAY_PERF_AGENT_ID", strings.Repeat("b", 32))
	t.Setenv("ASSAY_PERF_PARENT_AGENT_ID", strings.Repeat("c", 32))
	t.Setenv("ASSAY_PERF_PARENT_INVOCATION_ID", strings.Repeat("d", 32))
	t.Setenv("DESK_LOOP", "worker-desk")
	brief := `{"kind":"brief","repo":"example/project","brief":"example:project:work:1","primary":true}`
	issue := `{"kind":"issue","forge":"github","instance":"github.com","repo":"example/project","issue":2,"primary":true}`
	t.Setenv("ASSAY_PERF_WORK", "["+brief+"]")
	for _, tc := range []struct {
		name                string
		args                []string
		role, session, desk string
		work                int64
		profile             perfrec.Profile
	}{
		{"inherited", nil, "implement", strings.Repeat("a", 32), "worker-desk", 0, perfrec.ProfileWorkflow},
		{"override", []string{"--perf-role", "review", "--perf-session-id", "", "--perf-agent-id", strings.Repeat("e", 32), "--perf-parent-agent-id", "", "--perf-parent-invocation-id", "", "--perf-loop-role", "pr-review-desk", "--perf-work", issue}, "review", "", "pr-review-desk", 2, perfrec.ProfileWorkflow},
		{"clear", []string{"--perf-role=", "--perf-work="}, "", strings.Repeat("a", 32), "worker-desk", 0, perfrec.ProfileWorkflow},
		{"invalid", []string{"--perf-role=invalid metadata must stay private", "--perf-session-id=private text"}, "", "", "worker-desk", 0, perfrec.ProfileWorkflow},
		{"tool", []string{"--perf-profile=tool"}, "", "", "", 0, perfrec.ProfileTool},
		{"off", []string{"--perf-profile="}, "", "", "", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var selected perfrec.Context
			called := 0
			var output bytes.Buffer
			code := cli.Run(func() *cobra.Command {
				root := cli.NewRoot("fixture", "offline fixture")
				root.RunE = func(cmd *cobra.Command, _ []string) error {
					c, ok := cli.PerformanceContext(cmd)
					if !ok || c.InvocationID() != selected.InvocationID() {
						t.Fatal("handler lacks frozen parser context")
					}
					called++
					return nil
				}
				return root
			}, tc.args, cli.Options{IO: cli.IO{Out: &output, Err: &output}, Performance: func(c perfrec.Context) { selected = c }})
			if code != 0 || called != 1 || output.Len() != 0 {
				t.Fatalf("optional metadata changed operation: code=%d calls=%d", code, called)
			}
			d := selected.Data()
			if selected.Profile() != tc.profile || d.Role != tc.role || d.SessionID != tc.session || d.DeskRole != tc.desk {
				t.Fatalf("wrong context role/session/desk/profile")
			}
			if tc.work > 0 && (len(d.Work) != 1 || d.Work[0].Issue != tc.work) {
				t.Fatal("flag work did not replace inherited work")
			}
			if tc.name == "clear" && len(d.Work) != 0 {
				t.Fatal("empty work did not clear inheritance")
			}
			if tc.name == "override" && (d.ParentAgentID != "" || d.ParentInvocationID != "" || d.AgentID != strings.Repeat("e", 32)) {
				t.Fatal("declared identity override lost")
			}
			if os.Getenv("DESK_LOOP") != "worker-desk" {
				t.Fatal("attribution changed authority")
			}
		})
	}
	for _, name := range recorderBinaries(t) {
		t.Run(name, func(t *testing.T) { checkCurrentParser(t, name, issue) })
	}
}

// perfInventoryPath is the maintained-entrypoint inventory desktools-v2/64 records.
const perfInventoryPath = "docs/performance-entrypoints.json"

// perfEntrypoint is one inventory row. Unknown keys are refused, so a renamed key
// cannot silently drop a binary from the exercised set.
type perfEntrypoint struct {
	Executable  string `json:"executable"`
	Path        string `json:"path"`
	Module      string `json:"module"`
	ParserOwner string `json:"parser_owner"`
	ParserState string `json:"parser_state"`
	Context     string `json:"context"`
	Owner       string `json:"owner"`
	Gap         string `json:"gap,omitempty"`
}

// parserArgs are the offline operations each exercised binary runs. They are
// fixtures only: the exercised set comes from the inventory and the registry,
// and a key that set does not name, or a named binary with no key, fails.
var parserArgs = map[string][]string{
	"cellctl":       {"show", "nosuchcell"},
	"deskboard":     {"nextup"},
	"deskboot":      {"worker-desk", "--dry-run"},
	"deskclaim":     {"list"},
	"deskclaim-ref": {"list"},
	"deskdispatch":  nil,
	"deskfleet":     {"labels", "--forge", "github", "--repo", "example/project", "--dry-run"},
	"deskgitlab":    {"create"},
	"deskread":      {"issues", "--repo", "example/project"},
	"desktoken":     {"coverage", "invalid-role"},
	"deskwt":        nil,
}

// recorderBinaries derives the binaries whose parsed context reaches the
// recorder from the inventory, and fails when a row's parser owner or state
// (its parser exemption) disagrees with the CLI migration registry, or when
// registry validation reports a problem naming an exercised row or its owner.
func recorderBinaries(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(RegistryPath)))
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	reg := decode(t, raw)
	inv, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(perfInventoryPath)))
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	var doc struct {
		Schema      string            `json:"schema"`
		Source      string            `json:"source"`
		Scope       string            `json:"scope"`
		Entrypoints []perfEntrypoint  `json:"entrypoints"`
		Runtime     []json.RawMessage `json:"runtime"`
	}
	dec := json.NewDecoder(bytes.NewReader(inv))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil || doc.Schema != "performance-entrypoints/v1" {
		t.Fatalf("inventory %s: schema %q: %v", perfInventoryPath, doc.Schema, err)
	}
	rows := map[string]*Entry{}
	for i := range reg.Entrypoints {
		rows[reg.Entrypoints[i].Path] = &reg.Entrypoints[i]
	}
	var names, touched []string
	for _, p := range doc.Entrypoints {
		if p.Context != "parsed-to-recorder" {
			continue
		}
		e := rows[p.Path]
		switch {
		case e == nil:
			t.Errorf("inventory %s has no registry row", p.Path)
			continue
		case e.Executable != p.Executable || e.Owner != p.ParserOwner || e.State != p.ParserState:
			t.Errorf("inventory %s parser %s/%s disagrees with registry %s/%s", p.Path, p.ParserOwner, p.ParserState, e.Owner, e.State)
		case p.Module != "tools/desk" || p.Path != "tools/desk/cmd/"+p.Executable:
			t.Errorf("inventory %s has no desk build route", p.Path)
		}
		if _, ok := parserArgs[p.Executable]; !ok {
			t.Errorf("inventory %s has no offline fixture", p.Path)
		}
		names = append(names, p.Executable)
		touched = append(touched, p.Path, e.Owner)
	}
	for name := range parserArgs {
		if !contains(names, name) {
			t.Errorf("fixture %s names no parsed-to-recorder inventory row", name)
		}
	}
	for _, problem := range Validate(os.DirFS(repoRoot), reg) {
		for _, key := range touched {
			if namesPath(problem, key) {
				t.Errorf("registry problem on an exercised row: %s", problem)
				break
			}
		}
	}
	if len(names) == 0 {
		t.Error("inventory names no parsed-to-recorder entrypoint")
	}
	if t.Failed() {
		t.FailNow()
	}
	sort.Strings(names)
	return names
}

func checkCurrentParser(t *testing.T, name, work string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", bin, "./cmd/"+name)
	build.Dir = filepath.Join("..", "..")
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	home := t.TempDir()
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "USERPROFILE=" + home, "KUBECONFIG=/dev/null", "DESK_LOOP=worker-desk", "ASSAY_TELEMETRY=1", "ASSAY_PERF_PROFILE=workflow", "ASSAY_PERF_ROLE=implement", "ASSAY_PERF_SESSION_ID=" + strings.Repeat("a", 32), "ASSAY_PERF_AGENT_ID=" + strings.Repeat("b", 32), "ASSAY_PERF_PARENT_AGENT_ID=" + strings.Repeat("c", 32), "ASSAY_PERF_PARENT_INVOCATION_ID=" + strings.Repeat("d", 32), `ASSAY_PERF_WORK=[{"kind":"brief","repo":"example/project","brief":"example:project:work:1","primary":true}]`}
	args := parserArgs[name]
	run := func(extra ...string) ([]byte, int) {
		cmd := exec.Command(bin, append(append([]string(nil), args...), extra...)...)
		cmd.Dir = home
		cmd.Env = env
		out, _ := cmd.CombinedOutput()
		if cmd.ProcessState == nil {
			t.Fatal("binary did not start")
		}
		return out, cmd.ProcessState.ExitCode()
	}
	baseline, code := run()
	changed, got := run("--perf-role=review", "--perf-loop-role=pr-review-desk", "--perf-session-id=", "--perf-work="+work)
	if got != code || string(changed) != string(baseline) {
		t.Fatalf("optional flags changed current-parser output/exit: %d/%d\n%s\n%s", code, got, baseline, changed)
	}
	p := filepath.Join(home, ".config", "assay", "perf", time.Now().UTC().Format("2006-01-02")+".jsonl")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var records []perfrec.Record
	for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
		var r perfrec.Record
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatal(err)
		}
		records = append(records, r)
	}
	if len(records) != 2 {
		t.Fatalf("records=%d", len(records))
	}
	r := records[1]
	if r.Schema != perfrec.SchemaV3 || r.V3.Attribution.Role != "review" || r.V3.Attribution.DeskRole != "pr-review-desk" || r.V3.Attribution.SessionID != "" || r.V3.Attribution.AgentID != strings.Repeat("b", 32) || len(r.V3.Attribution.Work) != 1 || r.V3.Attribution.Work[0].Issue != 2 {
		t.Fatal("current parser did not reach typed recorder")
	}
	invalid, invalidCode := run("--perf-role=private invalid metadata", "--perf-work=not JSON")
	if invalidCode != code || string(invalid) != string(baseline) {
		t.Fatal("invalid optional metadata changed actual operation")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte{10})
	var invalidRecord perfrec.Record
	if err := json.Unmarshal(lines[len(lines)-1], &invalidRecord); err != nil {
		t.Fatal(err)
	}
	if invalidRecord.V3.Attribution.Role != "" || invalidRecord.V3.Attribution.Coverage[perfrec.FieldRole] != perfrec.CoverageInvalid {
		t.Fatal("invalid actual record was not filtered")
	}
	_, _ = run("--perf-profile=tool")
	raw, err = os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines = bytes.Split(bytes.TrimSpace(raw), []byte{10})
	var toolRecord perfrec.Record
	if err := json.Unmarshal(lines[len(lines)-1], &toolRecord); err != nil {
		t.Fatal(err)
	}
	if toolRecord.V3.Profile != perfrec.ProfileTool || toolRecord.V3.Attribution.Role != "" || toolRecord.V3.Attribution.SessionID != "" || toolRecord.V3.Attribution.AgentID != "" || toolRecord.V3.Attribution.ParentAgentID != "" || toolRecord.V3.Attribution.ParentInvocationID != "" || toolRecord.V3.Attribution.DeskRole != "" || len(toolRecord.V3.Attribution.Work) != 0 {
		t.Fatal("tool profile persisted inherited linkage")
	}

}
