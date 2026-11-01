package main

// Tests for role dispatch instructions: the Codex `instructions` binding, the narrowed Codex refusal and
// the `dispatch_instructions` key cellctl carries to deskdispatch through the window environment.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellstop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const workerSentinel = "Example sentinel W-30: quote this line when asked for your role instructions.\nCite the brief row you ran.\n"

// codexRoleFixture is a launchable cell whose worker-desk runs on Codex, with a stub codex that
// prints each argument on its own line.
func codexRoleFixture(t *testing.T) *policyFixture {
	t.Helper()
	f := catalogFixture(t)
	f.prepareLocalLaunch(t)
	if err := os.WriteFile(filepath.Join(f.binDir, "codex"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCatalogJSON(t, filepath.Join(f.cellDir, "providers.json"), map[string]any{"roles": map[string]string{"worker-desk": "codex"}})
	f.writeCellFile(t, "context/worker.md", workerSentinel)
	return f
}

func (f *policyFixture) writeCellFile(t *testing.T, rel, body string) string {
	t.Helper()
	p := filepath.Join(f.cellDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fileSHA(t *testing.T, p string) string {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// treeListing names every file under dir with its size and digest, .git excluded.
func treeListing(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		raw, _ := os.ReadFile(p)
		sum := sha256.Sum256(raw)
		out = append(out, rel+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// codexArgv is the codex stub's output as argv, from its first argument to the role prompt.
func codexArgv(stdout string) []string {
	var argv []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "[") || strings.HasPrefix(line, "HEAD is now at ") || line == "" {
			continue
		}
		argv = append(argv, line)
	}
	return argv
}

// regression: F-role-dispatch-instructions
func TestCodexRoleInstructions(t *testing.T) {
	f := codexRoleFixture(t)
	f.declareRoleContext(t, map[string]any{"worker-desk": map[string]any{"instructions": "context/worker.md"}})
	text, err := json.Marshal(strings.TrimSpace(workerSentinel))
	if err != nil {
		t.Fatal(err)
	}
	wantArg := codexInstructionsKey + "=" + string(text)
	declSHA := fileSHA(t, filepath.Join(f.cellDir, "role-context.json"))

	launch := func() runResult {
		t.Helper()
		r := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", "worker-desk", "--cadence", "off")
		if r.code != 0 {
			t.Fatalf("codex launch: %+v", r)
		}
		return r
	}
	first := launch()
	if !strings.Contains(first.stdout, "\n-c\n"+wantArg+"\n") {
		t.Fatalf("the standing argv does not carry the instructions as -c %s:\n%s", codexInstructionsKey, first.stdout)
	}
	var decoded string
	if err := json.Unmarshal(text, &decoded); err != nil || decoded != strings.TrimSpace(workerSentinel) {
		t.Fatalf("the bound value does not decode to the file's text: %q %v", decoded, err)
	}
	if !strings.Contains(first.stdout, "[context] role=worker-desk source=") || !strings.Contains(first.stdout, "sha256="+declSHA) {
		t.Errorf("the [context] line does not name the source and its sha256:\n%s", first.stdout)
	}
	worktree := filepath.Join(f.cellDir, "worktrees", "worker-desk")
	before := treeListing(t, worktree)
	second := launch()
	if after := treeListing(t, worktree); !reflect.DeepEqual(before, after) {
		t.Errorf("the launch changed the role worktree:\n before %q\n after  %q", before, after)
	}
	for _, l := range before {
		if strings.HasPrefix(filepath.Base(strings.Fields(l)[0]), "AGENTS") {
			t.Errorf("the binding wrote an instruction file into the worktree: %s", l)
		}
	}
	// Per-launch values (session name, scratch id) differ between launches; the binding does not.
	if !strings.Contains(second.stdout, "\n-c\n"+wantArg+"\n") {
		t.Errorf("the second launch does not carry the same binding:\n%s", second.stdout)
	}
	for _, out := range []string{first.stdout, second.stdout} {
		if argv := codexArgv(out); len(argv) == 0 || strings.Contains(argv[len(argv)-1], strings.TrimSpace(workerSentinel)) {
			t.Errorf("the instructions leaked into the role prompt: %q", argv)
		}
	}

	d1, d2 := f.dryRunDesk(t, "worker-desk"), f.dryRunDesk(t, "worker-desk")
	if d1.code != 0 || d1.stdout != d2.stdout {
		t.Errorf("two dry runs differ or fail:\n%+v\n%+v", d1, d2)
	}
	if !strings.Contains(d1.stdout, "[dry-run] context flags: -c "+codexInstructionsKey+"=<") {
		t.Errorf("the dry run does not name the Codex binding:\n%s", d1.stdout)
	}

	// The cadence launch keeps every element between the executable and the prompt.
	decl, err := loadRoleContext(filepath.Join(f.cellDir, "role-context.json"), f.cellDir)
	if err != nil {
		t.Fatal(err)
	}
	standing, err := applyCodexRoleContext([]string{"codex", "-m", "example-model", "/assay:worker-desk"}, decl.Roles["worker-desk"])
	if err != nil {
		t.Fatal(err)
	}
	if standing[len(standing)-1] != "/assay:worker-desk" {
		t.Errorf("the role prompt moved: %q", standing)
	}
	tick, _, err := prepareTickLaunch("codex", standing, nil, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := 0; i+1 < len(tick); i++ {
		if tick[i] == "-c" && tick[i+1] == wantArg {
			found = true
		}
	}
	if !found || tick[1] != "exec" {
		t.Errorf("the cadence argv does not carry the instructions: %q", tick)
	}
}

// regression: F-role-dispatch-instructions
func TestCodexUnboundKeysRefused(t *testing.T) {
	for _, c := range []struct {
		key  string
		role map[string]any
	}{
		{"memory_dir", map[string]any{"instructions": "context/worker.md", "memory_dir": "memory/pr-review-desk"}},
		{"tools_off", map[string]any{"instructions": "context/worker.md", "tools_off": []string{"WebFetch"}}},
		{"agents", map[string]any{"agents": []string{"builtin:lean-reviewer"}}},
	} {
		t.Run(c.key, func(t *testing.T) {
			f := codexRoleFixture(t)
			f.declareRoleContext(t, map[string]any{"worker-desk": c.role})
			r := f.dryRunDesk(t, "worker-desk")
			if r.code == 0 || !strings.Contains(r.stderr, "role worker-desk runs on codex; "+c.key) {
				t.Errorf("a codex role declaring %s should refuse naming it: %+v", c.key, r)
			}
			if r := f.run(t, nil, "check", "example"); r.code == 0 || !strings.Contains(r.stdout, "MISS  role context: worker-desk runs on codex; "+c.key) {
				t.Errorf("check should MISS naming %s:\n%s", c.key, r.stdout)
			}
		})
	}
	t.Run("instructions and dispatch_instructions accepted", func(t *testing.T) {
		f := codexRoleFixture(t)
		f.declareRoleContext(t, map[string]any{"worker-desk": map[string]any{
			"instructions": "context/worker.md", "dispatch_instructions": map[string]string{"worker": "context/worker.md"},
		}})
		if r := f.dryRunDesk(t, "worker-desk"); r.code != 0 {
			t.Errorf("a codex role with only bound keys should plan: %+v", r)
		}
	})
	t.Run("cursor refuses instructions", func(t *testing.T) {
		f := codexRoleFixture(t)
		f.declareRoleContext(t, map[string]any{"worker-desk": map[string]any{"instructions": "context/worker.md"}})
		env := &Env{vals: map[string]string{}, set: map[string]bool{}, src: map[string]string{}}
		env.Put(roleContextKey, filepath.Join(f.cellDir, "role-context.json"))
		c := &Cell{Dir: f.cellDir, Kind: "house", Env: env}
		if _, _, err := c.roleContextFor("worker-desk", "cursor"); err == nil || !strings.Contains(err.Error(), "runs on cursor") {
			t.Errorf("a cursor role with instructions should be refused: %v", err)
		}
	})
}

// dryRunEnvValue is the value of the dry run's ASSAY_DISPATCH_INSTRUCTIONS line, "" when absent.
func dryRunEnvValue(stdout string) (string, bool) {
	const prefix = "[dry-run] env " + deskkit.EnvDispatchInstructions + "="
	const suffix = " (role-context dispatch_instructions)"
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, suffix) {
			return strings.TrimSuffix(strings.TrimPrefix(line, prefix), suffix), true
		}
	}
	return "", false
}

// deskdispatchBinary builds deskdispatch once per test binary.
var deskdispatchBuilt string

func deskdispatchBinary(t *testing.T) string {
	t.Helper()
	if deskdispatchBuilt != "" {
		return deskdispatchBuilt
	}
	out := filepath.Join(os.TempDir(), "deskdispatch-dc30-"+strings.ReplaceAll(t.Name(), "/", "_"))
	cmd := exec.Command("go", "build", "-o", out, "../deskdispatch")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building deskdispatch: %v\n%s", err, b)
	}
	deskdispatchBuilt = out
	return out
}

// dispatchFlowRoster is the smallest roster deskdispatch plans a worker and a review dispatch
// under: the role commit identities and the target repository in scope. Example values.
const dispatchFlowRoster = "ASSAY_BLESS_LOGIN=example-human:2001\n" +
	"ASSAY_TRUSTED_LOGINS=example-human:2001\n" +
	"ASSAY_HUMAN_LOGIN_MAP=example:example-human\n" +
	"ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-app:300000001,reviewer=example-reviewer-app:300000004,worker=example-worker-app:300000006\n" +
	"ASSAY_ALLOWED_REPOS=example-org/example-repo:ci:private\n"

// regression: F-role-dispatch-instructions
func TestBinaryDispatchRulesFlow(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	for _, n := range []string{"tmux", "codex"} {
		if err := os.WriteFile(filepath.Join(f.binDir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.writeCellFile(t, "rules/worker.md", "# Worker dispatch rules\n\n- Example: name the Verify row each commit serves.\n")
	f.writeCellFile(t, "rules/review.md", "# Review dispatch rules\n\n- Example: read the whole diff before the first finding.\n")
	review := reviewRole()
	review["dispatch_instructions"] = map[string]string{"review": "rules/review.md"}
	f.declareRoleContext(t, map[string]any{
		"worker-desk":    map[string]any{"dispatch_instructions": map[string]string{"worker": "rules/worker.md"}},
		"pr-review-desk": review,
	})
	dd := deskdispatchBinary(t)
	for _, c := range []struct{ role, kit, file, item string }{
		{"worker-desk", "worker", "rules/worker.md", "example-stream--30"},
		{"pr-review-desk", "review", "rules/review.md", "example-repo--pr-42"},
	} {
		t.Run(c.role, func(t *testing.T) {
			r := f.dryRunDesk(t, c.role)
			value, ok := dryRunEnvValue(r.stdout)
			if r.code != 0 || !ok {
				t.Fatalf("the dry run prints no %s line: %+v", deskkit.EnvDispatchInstructions, r)
			}
			entries, err := deskkit.ParseDispatchInstructions(value)
			if err != nil || len(entries) != 1 || entries[0].Kit != c.kit {
				t.Fatalf("value %q: %+v %v", value, entries, err)
			}

			// deskdispatch --dry-run under that environment, against a scratch checkout.
			root := t.TempDir()
			git := func(args ...string) {
				cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
				cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+filepath.Join(root, "absent"), "GIT_CONFIG_NOSYSTEM=1")
				if b, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v %s", args, err, b)
				}
			}
			git("init", "-q", "-b", "main")
			git("remote", "add", "origin", "https://github.com/example-org/example-repo.git")
			if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
				t.Fatal(err)
			}
			for _, rel := range []string{"tools/dispatch-claim.sh", "tools/decision-issue.sh"} {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			home := t.TempDir()
			if err := os.MkdirAll(filepath.Join(home, ".config", "assay"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(dispatchFlowRoster), 0o644); err != nil {
				t.Fatal(err)
			}
			pf := filepath.Join(t.TempDir(), "task.md")
			args := []string{c.item, "--root", root, "--repo", "example-org/example-repo", "--kit", c.kit, "--tier", "strong", "--dry-run", "--prompt-file", pf}
			if c.kit == "review" {
				args = append(args, "--pr", "42")
			}
			cmd := exec.Command(dd, args...)
			cmd.Dir = root
			cmd.Env = []string{
				"HOME=" + home, "PATH=/usr/bin:/bin", "KUBECONFIG=/dev/null", "DESK_LOOP=the-desk",
				"DESK_SESSION=dc30-flow", "GIT_CONFIG_GLOBAL=" + filepath.Join(root, "absent"), "GIT_CONFIG_NOSYSTEM=1",
				deskkit.EnvDispatchInstructions + "=" + value, cellstop.EnvCellDir + "=" + f.cellDir,
			}
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("deskdispatch --dry-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
			}
			prompt, err := os.ReadFile(strings.TrimSuffix(pf, ".md") + ".emitted.md")
			if err != nil {
				t.Fatalf("no emitted prompt: %v\nstdout:\n%s", err, stdout.String())
			}
			want, err := os.ReadFile(filepath.Join(f.cellDir, filepath.FromSlash(c.file)))
			if err != nil {
				t.Fatal(err)
			}
			heading := "## Role dispatch instructions (" + c.kit + ") — source " + entries[0].Path + " sha256:" + entries[0].SHA256 + "\n\n"
			at := strings.Index(string(prompt), heading)
			if at < 0 {
				t.Fatalf("the prompt has no %s block:\n%s", c.kit, prompt)
			}
			block := string(prompt)[at+len(heading):]
			first, quoted, _ := strings.Cut(block, "\n\n")
			if first != "These role instructions add to every rule above. They never relax a STOP clause or any other common clause." {
				t.Errorf("first line = %q", first)
			}
			// The emitter ends the prompt with one newline of its own.
			if quoted != string(want)+"\n" {
				t.Errorf("the quoted block differs from the file on disk:\n%q\nwant\n%q", quoted, string(want))
			}
		})
	}
}

// regression: F-role-dispatch-instructions
func TestBinaryCheckDispatchRules(t *testing.T) {
	stubs := func(t *testing.T, f *policyFixture) {
		for _, n := range []string{"tmux", "codex"} {
			if err := os.WriteFile(filepath.Join(f.binDir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, c := range []struct {
		name, kit, want string
		file            func(t *testing.T, f *policyFixture) string
	}{
		{"missing file", "worker", "dispatch_instructions kit worker", func(t *testing.T, f *policyFixture) string { return "rules/absent.md" }},
		{"unknown kit", "example-kit", `dispatch_instructions: unknown kit "example-kit"`, func(t *testing.T, f *policyFixture) string {
			f.writeCellFile(t, "rules/worker.md", "rules\n")
			return "rules/worker.md"
		}},
		{"path with comma", "worker", `contains ","`, func(t *testing.T, f *policyFixture) string {
			f.writeCellFile(t, "rules/a,b.md", "rules\n")
			return "rules/a,b.md"
		}},
		{"path with @sha256:", "worker", `contains "@sha256:"`, func(t *testing.T, f *policyFixture) string {
			f.writeCellFile(t, "rules/x@sha256:y.md", "rules\n")
			return "rules/x@sha256:y.md"
		}},
		{"under worktrees", "worker", "which a role session can write", func(t *testing.T, f *policyFixture) string {
			f.writeCellFile(t, "worktrees/worker.md", "rules\n")
			return "worktrees/worker.md"
		}},
		{"under memory", "worker", "which a role session can write", func(t *testing.T, f *policyFixture) string {
			f.writeCellFile(t, "memory/worker.md", "rules\n")
			return "memory/worker.md"
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newPolicyFixture(t, "2.1.278")
			stubs(t, f)
			f.prepareLocalLaunch(t)
			recordingClaude(t, f)
			f.declareRoleContext(t, map[string]any{"worker-desk": map[string]any{"dispatch_instructions": map[string]string{c.kit: c.file(t, f)}}})
			r := f.run(t, nil, "check", "example")
			if r.code == 0 || !strings.Contains(r.stdout, "  MISS  role-context: role worker-desk: ") || !strings.Contains(r.stdout, c.want) {
				t.Errorf("check should MISS naming role and kit (%s):\n%s", c.want, r.stdout)
			}
			if !strings.Contains(r.stdout, c.kit) {
				t.Errorf("the MISS row does not name kit %s:\n%s", c.kit, r.stdout)
			}
			// Every role of the cell is refused, before a worktree exists.
			for _, role := range []string{"worker-desk", "pr-review-desk"} {
				r := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", role)
				if r.code == 0 || len(launchArgv(r.stdout)) != 0 || !strings.Contains(r.stderr, "role worker-desk: dispatch_instructions") {
					t.Errorf("desk %s should refuse: %+v", role, r)
				}
				if _, err := os.Stat(filepath.Join(f.cellDir, "worktrees", role)); err == nil {
					t.Errorf("desk %s created the role worktree before refusing", role)
				}
			}
		})
	}
	t.Run("one file for three uses", func(t *testing.T) {
		f := newPolicyFixture(t, "2.1.278")
		stubs(t, f)
		p := f.writeCellFile(t, "rules/worker.md", "# Worker rules\n\n- Example rule.\n")
		f.declareRoleContext(t, map[string]any{"worker-desk": map[string]any{
			"instructions":          "rules/worker.md",
			"dispatch_instructions": map[string]string{"worker": "rules/worker.md", "worker-objective": "rules/worker.md"},
		}})
		r := f.run(t, nil, "check", "example")
		if strings.Contains(r.stdout, "MISS  role") {
			t.Fatalf("a valid declaration has a MISS row:\n%s", r.stdout)
		}
		real, _ := filepath.EvalSymlinks(p)
		sha := fileSHA(t, p)
		for _, kit := range []string{"worker", "worker-objective"} {
			want := "  ok    role context: worker-desk dispatch_instructions kit=" + kit + " path=" + real + " size=" + strconv.Itoa(len("# Worker rules\n\n- Example rule.\n")) + " sha256=" + sha
			if strings.Count(r.stdout, want+"\n") != 1 {
				t.Errorf("check should print one row for kit %s (%q):\n%s", kit, want, r.stdout)
			}
		}
	})
}

// regression: F-role-dispatch-instructions
func TestDispatchKeyAbsent(t *testing.T) {
	envOf := func(t *testing.T, f *policyFixture, role string, extra ...string) map[string]string {
		t.Helper()
		r := f.run(t, append([]string{"CELLCTL_DESKWT=0"}, extra...), "desk", "example", role)
		if r.code != 0 {
			t.Fatalf("launch %s: %+v", role, r)
		}
		out := map[string]string{}
		for _, line := range strings.Split(r.stdout, "\n") {
			if k, v, ok := strings.Cut(line, "="); ok && (k == deskkit.EnvDispatchInstructions || k == cellstop.EnvCellDir) {
				out[k] = v
			}
		}
		return out
	}
	envClaude := func(t *testing.T, f *policyFixture) {
		script := "#!/bin/sh\ncase \"$1\" in\n --version) echo '2.1.278 (stub)'; exit 0;;\n plugin) exit 0;;\nesac\n" +
			"for a in \"$@\"; do printf 'ARG=%s\\n' \"$a\"; done\n" +
			"if [ -n \"${" + deskkit.EnvDispatchInstructions + "+x}\" ]; then printf '" + deskkit.EnvDispatchInstructions + "=%s\\n' \"$" + deskkit.EnvDispatchInstructions + "\"; fi\n" +
			"printf '" + cellstop.EnvCellDir + "=%s\\n' \"$" + cellstop.EnvCellDir + "\"\n"
		if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exported := []string{
		deskkit.EnvDispatchInstructions + "=worker=/example/elsewhere.md@sha256:" + strings.Repeat("a", 64),
		cellstop.EnvCellDir + "=/example/other-cell",
	}

	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	envClaude(t, f)
	cellDir, _ := filepath.EvalSymlinks(f.cellDir)
	sameCell := func(got string) bool {
		r, _ := filepath.EvalSymlinks(got)
		return got != "" && r == cellDir
	}

	// An undeclared cell: argv unchanged by the shell export, no value, the cell's own directory.
	quiet := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", "worker-desk")
	loud := f.run(t, append([]string{"CELLCTL_DESKWT=0"}, exported...), "desk", "example", "worker-desk")
	unnamed := func(argv []string) []string {
		for i := 0; i+1 < len(argv); i++ {
			if argv[i] == "--name" {
				argv[i+1] = strings.TrimRight(argv[i+1], "0123456789TZ")
			}
		}
		return argv
	}
	if quiet.code != 0 || loud.code != 0 || !reflect.DeepEqual(unnamed(launchArgv(quiet.stdout)), unnamed(launchArgv(loud.stdout))) {
		t.Errorf("an undeclared cell's argv changed with the shell export:\n%+v\n%+v", quiet, loud)
	}
	env := envOf(t, f, "worker-desk")
	if _, ok := env[deskkit.EnvDispatchInstructions]; ok || !sameCell(env[cellstop.EnvCellDir]) {
		t.Errorf("an undeclared cell's environment changed: %v", env)
	}
	env = envOf(t, f, "worker-desk", exported...)
	if _, ok := env[deskkit.EnvDispatchInstructions]; ok {
		t.Errorf("an undeclared cell passed the shell's value through: %v", env)
	}
	if !sameCell(env[cellstop.EnvCellDir]) {
		t.Errorf("%s = %q, want the cell's own directory", cellstop.EnvCellDir, env[cellstop.EnvCellDir])
	}

	// A declared cell: the declaring role gets its own value, the other role none.
	f.writeCellFile(t, "rules/review.md", "# Review rules\n")
	review := reviewRole()
	review["dispatch_instructions"] = map[string]string{"review": "rules/review.md"}
	f.declareRoleContext(t, map[string]any{"pr-review-desk": review})
	env = envOf(t, f, "worker-desk", exported...)
	if _, ok := env[deskkit.EnvDispatchInstructions]; ok {
		t.Errorf("a role with no dispatch_instructions carries a value: %v", env)
	}
	if !sameCell(env[cellstop.EnvCellDir]) {
		t.Errorf("worker-desk %s = %q, want the cell's own directory", cellstop.EnvCellDir, env[cellstop.EnvCellDir])
	}
	env = envOf(t, f, "pr-review-desk", exported...)
	entries, err := deskkit.ParseDispatchInstructions(env[deskkit.EnvDispatchInstructions])
	if err != nil || len(entries) != 1 || entries[0].Kit != "review" || strings.Contains(env[deskkit.EnvDispatchInstructions], "/example/elsewhere.md") {
		t.Errorf("pr-review-desk value = %q (%v)", env[deskkit.EnvDispatchInstructions], err)
	}
	if !sameCell(env[cellstop.EnvCellDir]) {
		t.Errorf("pr-review-desk %s = %q, want the cell's own directory", cellstop.EnvCellDir, env[cellstop.EnvCellDir])
	}
}

// regression: F-role-dispatch-instructions
func TestShippedExampleDispatchRules(t *testing.T) {
	raw, err := os.ReadFile(exampleRoleContextPath)
	if err != nil {
		t.Fatal(err)
	}
	var top struct {
		Roles map[string]struct {
			Agents               []string          `json:"agents"`
			DispatchInstructions map[string]string `json:"dispatch_instructions"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	role := top.Roles["pr-review-desk"]
	ref := role.DispatchInstructions["review"]
	if ref == "" || !contains(role.Agents, "builtin:lean-reviewer") {
		t.Fatalf("the example review role should name a review dispatch file next to builtin:lean-reviewer: %+v", role)
	}

	// The example loads through the real parser, with the files it names copied beside it.
	f := newPolicyFixture(t, "2.1.278")
	for _, n := range []string{"tmux", "codex"} {
		if err := os.WriteFile(filepath.Join(f.binDir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exampleDir := filepath.Dir(exampleRoleContextPath)
	body, err := os.ReadFile(filepath.Join(exampleDir, filepath.FromSlash(ref)))
	if err != nil {
		t.Fatalf("the example names %s, which is not shipped beside it: %v", ref, err)
	}
	f.writeCellFile(t, ref, string(body))
	if err := os.MkdirAll(filepath.Join(f.cellDir, "memory", "pr-review-desk"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.writeCellFile(t, "role-context.json", string(raw))
	d, err := loadRoleContext(filepath.Join(f.cellDir, "role-context.json"), f.cellDir)
	if err != nil {
		t.Fatalf("the shipped example does not load: %v", err)
	}
	rc := d.Roles["pr-review-desk"]
	if len(rc.Dispatch) != 1 || rc.Dispatch[0].Kit != "review" {
		t.Fatalf("the example review role resolves no review entry: %+v", rc.Dispatch)
	}
	envPath := filepath.Join(f.cellDir, "cell.env")
	cellEnv, _ := os.ReadFile(envPath)
	if err := os.WriteFile(envPath, append(cellEnv, []byte("CELL_ROLE_CONTEXT=role-context.json\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	r := f.dryRunDesk(t, "pr-review-desk")
	printed, ok := dryRunEnvValue(r.stdout)
	if r.code != 0 || !ok {
		t.Fatalf("the dry run prints no value line: %+v", r)
	}

	// The doc quotes the line in the shape the binary prints it; the cell path is the reader's.
	doc, err := os.ReadFile("../../../../docs/cellctl-role-context.md")
	if err != nil {
		t.Fatal(err)
	}
	var quoted string
	for _, line := range strings.Split(string(doc), "\n") {
		if strings.HasPrefix(line, "[dry-run] env "+deskkit.EnvDispatchInstructions+"=") {
			quoted = line
		}
	}
	if quoted == "" {
		t.Fatal("docs/cellctl-role-context.md quotes no [dry-run] env line")
	}
	docValue, ok := dryRunEnvValue(quoted)
	if !ok {
		t.Fatalf("the quoted line is not in the printed shape: %q", quoted)
	}
	docEntries, err := deskkit.ParseDispatchInstructions(docValue)
	if err != nil || len(docEntries) != 1 {
		t.Fatalf("the quoted value does not parse: %q %v", docValue, err)
	}
	gotEntries, _ := deskkit.ParseDispatchInstructions(printed)
	cellDir, _ := filepath.EvalSymlinks(f.cellDir)
	rel, err := filepath.Rel(cellDir, gotEntries[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if docEntries[0].Kit != gotEntries[0].Kit || docEntries[0].SHA256 != gotEntries[0].SHA256 ||
		!strings.HasSuffix(docEntries[0].Path, "/"+filepath.ToSlash(rel)) {
		t.Errorf("the doc's quoted line does not match the binary:\n doc    %q\n binary %q", quoted, "[dry-run] env "+deskkit.EnvDispatchInstructions+"="+printed+" (role-context dispatch_instructions)")
	}
}
