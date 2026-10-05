package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// This file covers the line a cockpit pane is handed. tmux runs a POSIX shell, so its line is the
// POSIX rendering, pinned byte for byte below. herdr and orca open the platform's default shell,
// which on Windows is PowerShell — where a leading quoted path is a string expression, not an
// invocation, and POSIX `'\''` is not an escape at all — so there the line is rendered with the
// `&` call operator and PowerShell single-quote literals.

// TestRoleCmdPOSIXBytesPinned pins the POSIX rendering byte for byte, so the PowerShell arm can
// never leak into what tmux runs.
func TestRoleCmdPOSIXBytesPinned(t *testing.T) {
	c := &Cell{Name: "o'brien cell", Dir: filepath.Join("cells", "o'brien cell"), KindOverride: "house",
		Cadence: &cadenceOptions{Interval: 5 * time.Minute, Budget: 20 * time.Minute}}
	o := upOverrides{Model: "m'x", Harness: "claude", Provider: "glm", Cockpit: "tmux"}
	got := c.roleCmdIn(shellPOSIX, "/opt/assay/bin/cell ctl", "worker-desk", "/home/a b/.claude", o)
	want := `'/opt/assay/bin/cell ctl' --cells-root 'cells' desk 'o'\''brien cell' 'worker-desk'` +
		` --kind 'house' --model 'm'\''x' --harness 'claude' --provider 'glm' --cockpit 'tmux'` +
		` --cadence '5m0s' --tick-budget '20m0s' '/home/a b/.claude'`
	if got != want {
		t.Fatalf("POSIX roleCmd drifted:\n got %s\nwant %s", got, want)
	}
	// roleCmd is that rendering with this binary as the program word; the tmux arm's
	// paneRoleCmd is the same line on every platform.
	if a, b := c.roleCmd("worker-desk", "/cfg", o), c.roleCmdIn(shellPOSIX, selfPath(), "worker-desk", "/cfg", o); a != b {
		t.Fatalf("roleCmd != POSIX roleCmdIn:\n%s\n%s", a, b)
	}
	if a, b := c.paneRoleCmd("worker-desk", "/cfg", o), c.roleCmd("worker-desk", "/cfg", o); a != b {
		t.Fatalf("tmux pane line is not the POSIX line:\n%s\n%s", a, b)
	}
}

func TestPaneShellFor(t *testing.T) {
	for _, tc := range []struct {
		goos, cockpit string
		want          paneShell
	}{
		{"windows", "herdr", shellPowerShell},
		{"windows", "orca", shellPowerShell},
		{"windows", "tmux", shellPOSIX},
		{"windows", "", shellPOSIX},
		{"linux", "herdr", shellPOSIX},
		{"darwin", "orca", shellPOSIX},
		{"darwin", "tmux", shellPOSIX},
	} {
		if got := paneShellFor(tc.goos, tc.cockpit); got != tc.want {
			t.Errorf("paneShellFor(%s, %s) = %v, want %v", tc.goos, tc.cockpit, got, tc.want)
		}
	}
}

func TestPSQuote(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "''"},
		{"plain", "'plain'"},
		{"work's folder", "'work''s folder'"},
		{"''", "''''''"},
		{`C:\Users\Jane Doe\AppData\Local\Assay\bin\cellctl.exe`, `'C:\Users\Jane Doe\AppData\Local\Assay\bin\cellctl.exe'`},
		{`\\server\share\a b`, `'\\server\share\a b'`},
		{"$env:HOME `n $(whoami); & x", "'$env:HOME `n $(whoami); & x'"},
		{"it\u2019s \u2018q\u2018 \u201a \u201b", "'it\u2019\u2019s \u2018\u2018q\u2018\u2018 \u201a\u201a \u201b\u201b'"},
		{"bad\xffbyte", "'bad\xffbyte'"},
	} {
		if got := psQuote(tc.in); got != tc.want {
			t.Errorf("psQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRoleCmdPowerShell pins the PowerShell rendering: the issue's own observed line, now with the
// call operator, and a line carrying embedded single quotes, spaces and backslash paths.
func TestRoleCmdPowerShell(t *testing.T) {
	c := &Cell{Name: "test"}
	got := c.roleCmdIn(shellPowerShell, `%LOCALAPPDATA%\Assay\bin\cellctl.exe`, "pr-review-desk", `%USERPROFILE%\.claude`, upOverrides{Cockpit: "herdr"})
	want := `& '%LOCALAPPDATA%\Assay\bin\cellctl.exe' desk 'test' 'pr-review-desk' --cockpit 'herdr' '%USERPROFILE%\.claude'`
	if got != want {
		t.Fatalf("PowerShell roleCmd:\n got %s\nwant %s", got, want)
	}

	c = &Cell{Name: "o'brien cell", KindOverride: "house"}
	self := `C:\Program Files\Jane's Tools\cellctl.exe`
	cfg := `C:\Users\Jane O'Neil\.claude`
	o := upOverrides{Model: "m'x", Provider: "glm", Cockpit: "orca", Cadence: "5m"}
	got = c.roleCmdIn(shellPowerShell, self, "worker-desk", cfg, o)
	want = `& 'C:\Program Files\Jane''s Tools\cellctl.exe' --cells-root '.' desk 'o''brien cell' 'worker-desk'` +
		` --kind 'house' --model 'm''x' --provider 'glm' --cockpit 'orca' --cadence '5m' 'C:\Users\Jane O''Neil\.claude'`
	if got != want {
		t.Fatalf("PowerShell roleCmd:\n got %s\nwant %s", got, want)
	}
	if strings.Contains(got, `'\''`) {
		t.Fatalf("a POSIX escape reached the PowerShell line: %s", got)
	}
	// Decoded by PowerShell's own single-quote rule, the line is exactly the intended argv.
	argv, err := decodePSInvocation(got)
	if err != nil {
		t.Fatalf("decode %s: %v", got, err)
	}
	wantArgv := []string{self, "--cells-root", ".", "desk", "o'brien cell", "worker-desk", "--kind", "house",
		"--model", "m'x", "--provider", "glm", "--cockpit", "orca", "--cadence", "5m", cfg}
	if !reflect.DeepEqual(argv, wantArgv) {
		t.Fatalf("argv round trip:\n got %q\nwant %q", argv, wantArgv)
	}
}

// decodePSInvocation reads `& <word> <word>...` the way PowerShell's tokenizer reads it in
// argument mode: a word is either a single-quoted literal (a doubled quote is one quote) or a
// bare run of non-space characters. It refuses anything else, so a line PowerShell would read
// differently cannot decode cleanly.
func decodePSInvocation(line string) ([]string, error) {
	if !strings.HasPrefix(line, "& ") {
		return nil, fmt.Errorf("no call operator")
	}
	rs := []rune(line[2:])
	isQ := func(r rune) bool { return r == '\'' || (r >= '\u2018' && r <= '\u201b') }
	var out []string
	for i := 0; i < len(rs); {
		if rs[i] == ' ' {
			i++
			continue
		}
		var w strings.Builder
		if isQ(rs[i]) {
			i++
			for {
				if i >= len(rs) {
					return nil, fmt.Errorf("unterminated literal")
				}
				if isQ(rs[i]) {
					if i+1 < len(rs) && isQ(rs[i+1]) {
						w.WriteRune(rs[i])
						i += 2
						continue
					}
					i++
					break
				}
				w.WriteRune(rs[i])
				i++
			}
			if i < len(rs) && rs[i] != ' ' {
				return nil, fmt.Errorf("literal runs into %q", string(rs[i]))
			}
		} else {
			for i < len(rs) && rs[i] != ' ' {
				if strings.ContainsRune("'\"$`;&|(){}@<>", rs[i]) {
					return nil, fmt.Errorf("bare word carries %q", string(rs[i]))
				}
				w.WriteRune(rs[i])
				i++
			}
		}
		out = append(out, w.String())
	}
	return out, nil
}

// TestFirstWindowCmdsPerShell pins the first window's lines: the POSIX forms byte for byte as the
// tmux arm has always run them, and PowerShell forms with no POSIX-only syntax in them.
func TestFirstWindowCmdsPerShell(t *testing.T) {
	if got, want := deskdStandCmd(shellPOSIX, "/opt/cellctl", "c1"), "CELL_ATTENDED=1 '/opt/cellctl' deskd 'c1'"; got != want {
		t.Errorf("POSIX stand: %s", got)
	}
	if got, want := deskdWatchCmd(shellPOSIX, "127.0.0.1:7070"), "echo '[deskd] already up on 127.0.0.1:7070 — watching /healthz every 60s'; while :; do date -u +%H:%MZ; curl -s --max-time 5 http://127.0.0.1:7070/healthz | head -c 240; echo; sleep 60; done"; got != want {
		t.Errorf("POSIX watch: %s", got)
	}
	if got, want := shellPOSIX.echo("[cell] x"), "echo '[cell] x'"; got != want {
		t.Errorf("POSIX echo: %s", got)
	}
	if got, want := deskdHandStart(shellPOSIX, "c1"), "CELL_ATTENDED=1 cellctl deskd c1"; got != want {
		t.Errorf("POSIX hand start: %s", got)
	}

	if got, want := deskdStandCmd(shellPowerShell, `C:\Jane's\cellctl.exe`, "c'1"), `$env:CELL_ATTENDED='1'; & 'C:\Jane''s\cellctl.exe' deskd 'c''1'`; got != want {
		t.Errorf("PowerShell stand: %s", got)
	}
	if got, want := shellPowerShell.echo("[cell] o'brien"), `Write-Output '[cell] o''brien'`; got != want {
		t.Errorf("PowerShell echo: %s", got)
	}
	watch := deskdWatchCmd(shellPowerShell, "127.0.0.1:7070")
	for _, bad := range []string{"while :", "; do ", "done", "| head", "date -u", " curl "} {
		if strings.Contains(watch, bad) {
			t.Errorf("PowerShell watch carries POSIX-only %q: %s", bad, watch)
		}
	}
	if !strings.Contains(watch, "curl.exe ") || !strings.Contains(watch, "'http://127.0.0.1:7070/healthz'") {
		t.Errorf("PowerShell watch must call curl.exe on the quoted healthz URL: %s", watch)
	}
}

// TestPaneLinesParseInRealPowerShell hands every PowerShell line to PowerShell's own parser when
// one is on PATH. Where none is, it SKIPS — a could-not-check, never a pass.
func TestPaneLinesParseInRealPowerShell(t *testing.T) {
	ps := ""
	for _, name := range []string{"pwsh", "powershell"} {
		if p, err := exec.LookPath(name); err == nil {
			ps = p
			break
		}
	}
	if ps == "" {
		t.Skip("could-not-check: no pwsh/powershell on PATH to parse the PowerShell pane lines")
	}
	self := `C:\Program Files\Jane's Tools\cellctl.exe`
	cfg := `C:\Users\Jane O'Neil\.claude`
	c := &Cell{Name: "o'brien cell"}
	role := c.roleCmdIn(shellPowerShell, self, "worker-desk", cfg, upOverrides{Model: "m'x", Cockpit: "herdr"})
	script := `$t = $null; $e = $null
$ast = [System.Management.Automation.Language.Parser]::ParseInput($env:CELLCTL_PS_LINE, [ref]$t, [ref]$e)
if ($e.Count -gt 0) { $e | ForEach-Object { 'ERR ' + $_.Message }; exit 3 }
if ($env:CELLCTL_PS_ARGV -ne '1') { exit 0 }
$c = $ast.EndBlock.Statements[0].PipelineElements[0]
'OP ' + $c.InvocationOperator
foreach ($el in $c.CommandElements) { 'ARG ' + $el.Extent.Text + ' => ' + $el.SafeGetValue() }`
	run := func(line string, argv bool) string {
		cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script)
		flag := "0"
		if argv {
			flag = "1"
		}
		cmd.Env = append(os.Environ(), "CELLCTL_PS_LINE="+line, "CELLCTL_PS_ARGV="+flag)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("PowerShell refused %s: %v\n%s", line, err, out)
		}
		return string(out)
	}
	out := run(role, true)
	if !strings.Contains(out, "OP Ampersand") {
		t.Errorf("role line is not an invocation: %s", out)
	}
	for _, want := range []string{"=> " + self, "=> o'brien cell", "=> m'x", "=> " + cfg} {
		if !strings.Contains(out, want) {
			t.Errorf("PowerShell did not read %q from the role line:\n%s", want, out)
		}
	}
	for _, line := range []string{
		deskdStandCmd(shellPowerShell, self, "o'brien cell"),
		deskdWatchCmd(shellPowerShell, "127.0.0.1:7070"),
		shellPowerShell.echo("[deskd] NOT stood. Run: " + deskdHandStart(shellPowerShell, "c1")),
	} {
		run(line, false)
	}
}

// TestPaneLineClassGuard is the class guard: a role or first-window line rendered in a FIXED
// shell, reaching a cockpit pane whose shell may be another. It walks every non-test source file
// in this package and fails on a call to a fixed-dialect renderer from anywhere but its allowed
// wrappers — so a new cockpit arm, NOTICE or dry-run line that reaches for the POSIX rendering
// directly is red here before review.
func TestPaneLineClassGuard(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var parsed []*ast.File
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, af)
	}
	if len(parsed) == 0 {
		t.Fatal("guard saw no source files")
	}
	if v := paneLineViolations(fset, parsed); len(v) != 0 {
		t.Fatalf("fixed-dialect pane line outside its wrappers:\n%s", strings.Join(v, "\n"))
	}

	// Positive control: a planted arm reaching for each renderer directly must be flagged.
	const plant = `package main
func (c *Cell) upPlanted(cfg string, o upOverrides) {
	_ = c.roleCmd("worker-desk", cfg, o)
	_ = c.roleCmdIn(shellPOSIX, "x", "worker-desk", cfg, o)
	_ = cockpitQuote(cfg)
	_, _ = c.firstWindow(shellPOSIX)
}
`
	pf := token.NewFileSet()
	af, err := parser.ParseFile(pf, "planted.go", plant, 0)
	if err != nil {
		t.Fatal(err)
	}
	v := paneLineViolations(pf, []*ast.File{af})
	if len(v) != 4 {
		t.Fatalf("positive control: want 4 violations from the planted arm, got %d:\n%s", len(v), strings.Join(v, "\n"))
	}
}

// paneLineViolations lists every call to a fixed-dialect renderer outside its allow-list. A
// firstWindow call is allowed from any caller as long as its shell argument is paneShellFor(...).
func paneLineViolations(fset *token.FileSet, files []*ast.File) []string {
	allowed := map[string][]string{
		"roleCmd":      {},                         // POSIX convenience: tests only
		"roleCmdIn":    {"roleCmd", "paneRoleCmd"}, // the two renderings
		"cockpitQuote": {"quote"},                  // paneShell.quote picks it
		"firstWindow":  {},                         // only with a paneShellFor(...) argument
	}
	var out []string
	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				callers, guarded := allowed[name]
				if !guarded {
					return true
				}
				for _, c := range callers {
					if c == fd.Name.Name {
						return true
					}
				}
				if name == "firstWindow" && len(call.Args) == 1 {
					if inner, ok := call.Args[0].(*ast.CallExpr); ok {
						if id, ok := inner.Fun.(*ast.Ident); ok && id.Name == "paneShellFor" {
							return true
						}
					}
				}
				out = append(out, fmt.Sprintf("%s: %s calls %s", fset.Position(call.Pos()), fd.Name.Name, name))
				return true
			})
		}
	}
	sort.Strings(out)
	return out
}
