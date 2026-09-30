package deskkit

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// THE .ps1 ENCODING GUARD (#1569, regression of #678).
//
// WHAT IT GUARDS. Windows PowerShell 5.1 (powershell.exe, the one every Windows host
// ships) reads a .ps1 that has NO byte-order mark in the system ANSI code page
// (Windows-1252 on an en-US host), not as UTF-8. The UTF-8 bytes of an em dash, E2 80 94,
// then decode as three characters, and 0x94 is U+201D RIGHT DOUBLE QUOTATION MARK, which
// the PowerShell lexer accepts as a string delimiter. An em dash inside a double-quoted
// string therefore ends the string early and the whole script fails to parse under 5.1,
// while pwsh 7 (which defaults to UTF-8) parses it fine, so a pwsh-only author never sees
// it. That is exactly how scripts/build-windows.ps1 broke twice.
//
// THE RULE. Every *.ps1 in the tree is either pure ASCII (every byte <= 0x7F) or begins
// with the UTF-8 BOM (EF BB BF), which makes 5.1 decode it as UTF-8. Nothing else passes.
//
// WHY A GO TEST HERE. It is a byte scan, so it needs no PowerShell and runs on the Linux
// CI runner. The ci workflow's build-test job already runs `go test ./...` in tools/desk,
// so living in this package puts the guard on every push and pull request with no
// workflow edit. tools/winparity carries a stricter, single-file version of the same
// check for scripts/build-windows.ps1, but its tests are not run by CI (its workflow is
// still staged), which is how the #1521 em dashes got back in.
//
// THREE STATES. A walk that finds no .ps1 at all cannot tell "clean" from "looked in the
// wrong place", so the control path below must be among the files scanned; if it is not,
// the result is could-not-check, which fails the test the same as a finding does.

// ps1RepoRoot: this package is tools/desk/internal/deskkit.
const ps1RepoRoot = "../../../.."

// ps1ControlPath is a .ps1 known to be tracked in this tree (tools/winparity pins it too).
// If the walk does not reach it, the walk is broken, and its silence proves nothing.
const ps1ControlPath = "scripts/build-windows.ps1"

// utf8BOM is the byte-order mark that makes Windows PowerShell 5.1 decode a script as UTF-8.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type ps1SweepState string

const (
	ps1CheckedClean  ps1SweepState = "checked-clean"
	ps1CheckedFailed ps1SweepState = "checked-failed"
	ps1CouldNotCheck ps1SweepState = "could-not-check"
)

type ps1SweepReport struct {
	state    ps1SweepState
	scanned  []string // repo-relative, slash-separated
	findings []string // one line per offending file
}

// ps1NonASCIIFinding returns "" when content is 5.1-safe (pure ASCII, or UTF-8 with a BOM),
// else a description naming the first byte above 0x7F and how many lines carry one.
func ps1NonASCIIFinding(content []byte) string {
	if bytes.HasPrefix(content, utf8BOM) {
		return ""
	}
	first := ""
	lines := 0
	for i, line := range bytes.Split(content, []byte("\n")) {
		for col, b := range line {
			if b > 0x7F {
				if first == "" {
					first = fmt.Sprintf("line %d col %d byte 0x%02X", i+1, col+1, b)
				}
				lines++
				break
			}
		}
	}
	if first == "" {
		return ""
	}
	return fmt.Sprintf("non-ASCII with no UTF-8 BOM: first at %s; %d line(s) affected", first, lines)
}

// ps1EncodingSweep walks root and checks every regular file whose name ends in .ps1
// (any case). .git, node_modules and vendor are skipped by name: they are never-authored
// trees. control is a repo-relative path that must be scanned for the result to count.
func ps1EncodingSweep(root, control string) (ps1SweepReport, error) {
	var rep ps1SweepReport
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(d.Name()), ".ps1") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		content, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rep.scanned = append(rep.scanned, rel)
		if f := ps1NonASCIIFinding(content); f != "" {
			rep.findings = append(rep.findings, rel+": "+f)
		}
		return nil
	})
	if err != nil {
		return ps1SweepReport{state: ps1CouldNotCheck}, err
	}
	sort.Strings(rep.findings)
	controlSeen := false
	for _, s := range rep.scanned {
		if s == control {
			controlSeen = true
			break
		}
	}
	switch {
	case !controlSeen:
		rep.state = ps1CouldNotCheck
	case len(rep.findings) > 0:
		rep.state = ps1CheckedFailed
	default:
		rep.state = ps1CheckedClean
	}
	return rep, nil
}

// TestPS1EncodingIs51Safe is the gate over the whole repository.
func TestPS1EncodingIs51Safe(t *testing.T) {
	rep, err := ps1EncodingSweep(ps1RepoRoot, ps1ControlPath)
	if err != nil {
		t.Fatalf("could-not-check: the .ps1 sweep could not walk the tree: %v", err)
	}
	switch rep.state {
	case ps1CouldNotCheck:
		t.Fatalf("could-not-check: the control %s was not among the %d .ps1 file(s) scanned "+
			"under %s, so an empty result proves nothing. Re-point ps1RepoRoot or ps1ControlPath.",
			ps1ControlPath, len(rep.scanned), ps1RepoRoot)
	case ps1CheckedFailed:
		t.Fatalf("checked-failed: %d .ps1 file(s) would be mis-decoded by Windows PowerShell 5.1 "+
			"(no BOM, so it reads them as Windows-1252; an em dash's 0x94 byte becomes a string "+
			"delimiter):\n  %s\nReplace each non-ASCII character with an ASCII equivalent (-- for a "+
			"dash, straight quotes for curly ones), or, only if the text must stay non-ASCII, save "+
			"the file as UTF-8 WITH a BOM.", len(rep.findings), strings.Join(rep.findings, "\n  "))
	}
	t.Logf("checked-clean: %d .ps1 file(s) scanned: %s", len(rep.scanned), strings.Join(rep.scanned, " "))
}

// TestPS1GuardRedOnEmDash is the fail-first proof: the same sweep, on a
// fixture tree, must go red on a BOM-less em dash and stay green on a BOM'd one and on ASCII.
func TestPS1GuardRedOnEmDash(t *testing.T) {
	emDash := "—" // UTF-8 bytes E2 80 94
	write := func(t *testing.T, root, rel string, content []byte) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	asciiScript := []byte("throw \"sign: missing EKU -- refusing\"\n")
	dashScript := []byte("# fine\nthrow \"sign: missing EKU " + emDash + " refusing\"\n")

	cases := []struct {
		name      string
		files     map[string][]byte
		wantState ps1SweepState
		wantHit   string
	}{
		{
			name:      "planted em dash, no BOM -> red",
			files:     map[string][]byte{"scripts/build-windows.ps1": asciiScript, "scripts/other.ps1": dashScript},
			wantState: ps1CheckedFailed,
			wantHit:   "scripts/other.ps1: non-ASCII with no UTF-8 BOM: first at line 2 col 26 byte 0xE2; 1 line(s) affected",
		},
		{
			name:      "planted em dash in an upper-case .PS1 -> red",
			files:     map[string][]byte{"scripts/build-windows.ps1": asciiScript, "tools/X.PS1": dashScript},
			wantState: ps1CheckedFailed,
			wantHit:   "tools/X.PS1:",
		},
		{
			name:      "same em dash behind a UTF-8 BOM -> green",
			files:     map[string][]byte{"scripts/build-windows.ps1": append(append([]byte{}, utf8BOM...), dashScript...)},
			wantState: ps1CheckedClean,
		},
		{
			name:      "pure ASCII -> green",
			files:     map[string][]byte{"scripts/build-windows.ps1": asciiScript},
			wantState: ps1CheckedClean,
		},
		{
			name:      "em dash in a non-.ps1 file is out of scope -> green",
			files:     map[string][]byte{"scripts/build-windows.ps1": asciiScript, "docs/notes.md": dashScript},
			wantState: ps1CheckedClean,
		},
		{
			name:      "control file absent -> could-not-check, never clean",
			files:     map[string][]byte{"scripts/other.ps1": asciiScript},
			wantState: ps1CouldNotCheck,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for rel, content := range tc.files {
				write(t, root, rel, content)
			}
			rep, err := ps1EncodingSweep(root, ps1ControlPath)
			if err != nil {
				t.Fatalf("sweep error: %v", err)
			}
			if rep.state != tc.wantState {
				t.Fatalf("state = %s, want %s (findings: %v)", rep.state, tc.wantState, rep.findings)
			}
			if tc.wantHit != "" {
				joined := strings.Join(rep.findings, "\n")
				if !strings.Contains(joined, tc.wantHit) {
					t.Fatalf("findings %q do not contain %q", joined, tc.wantHit)
				}
			}
		})
	}
}

// TestPS1GuardMissingTree: a root that does not exist is an error,
// never a clean result.
func TestPS1GuardMissingTree(t *testing.T) {
	rep, err := ps1EncodingSweep(filepath.Join(t.TempDir(), "absent"), ps1ControlPath)
	if err == nil && rep.state == ps1CheckedClean {
		t.Fatal("a missing tree read as checked-clean")
	}
}
