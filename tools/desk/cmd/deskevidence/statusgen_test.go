package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func rawAuditSites(name string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var sites []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || (filepath.Base(name) == "statusgen.go" && fn.Name.Name == "statusgenAudit") {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			index := -1
			switch target := call.Fun.(type) {
			case *ast.SelectorExpr:
				if owner, ok := target.X.(*ast.Ident); ok && owner.Name == "exec" {
					switch target.Sel.Name {
					case "Command":
						index = 0
					case "CommandContext":
						index = 1
					}
				}
			case *ast.Ident:
				if target.Name == "execCommand" {
					index = 0
				}
			}
			if index >= 0 && len(call.Args) > index {
				if lit, ok := call.Args[index].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if value, _ := strconv.Unquote(lit.Value); value == "statusgen" {
						sites = append(sites, fset.Position(call.Pos()).String())
					}
				}
			}
			return true
		})
	}
	return sites, nil
}

// regression: #4823
func TestAuditChildConstruction(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("cannot enumerate production callers: %v", err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		sites, err := rawAuditSites(name, src)
		if err != nil || len(sites) != 0 {
			t.Errorf("raw audit child bypasses custody: %v (parse=%v)", sites, err)
		}
	}
	// Positive control: a second caller outside the adapter must be detected.
	plant := []byte(`package main
import "os/exec"
func secondSite() { exec.Command("statusgen", "verifyclosure") }
`)
	if sites, err := rawAuditSites("second.go", plant); err != nil || len(sites) != 1 {
		t.Fatalf("second caller was not detected: %v (%v)", sites, err)
	}
}

// regression: #4823
func TestAuditCannotExecuteRows(t *testing.T) {
	oldFlag, oldToken := acceptanceForge, ghToken
	acceptanceForge, ghToken = true, "fixture-minted"
	t.Cleanup(func() { acceptanceForge, ghToken = oldFlag, oldToken })
	cmd := statusgenAudit(context.Background(), closureAudit("verifyrun"), "example-root", "--brief", "example.md")
	if !deskkit.IsUnverifiable(cmd.Err) {
		t.Fatal("row executor received an approval audit command")
	}
}

// regression: #4823
func TestFlipApprovalKeepsRole(t *testing.T) {
	for _, loop := range []string{"worker-desk", "verify-desk"} {
		t.Run(loop, func(t *testing.T) {
			flSetup(t)
			t.Setenv("DESK_LOOP", loop)
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			root := flRoot(t)
			flGit(t, root, "remote", "add", "origin", "https://github.com/example-org/tracker.git")
			mints := 0
			mintTokenFn = func(repo string) error {
				if repo != "example-org/tracker" {
					t.Fatalf("wrong approval subject: %s", repo)
				}
				mints++
				ghToken = "fixture-minted"
				ghTokenForge = deskkit.ForgeGitHub
				return nil
			}
			code := flRun(root, "--acceptance-forge", "--dry-run")
			wantCode, wantMints := deskkit.ExitOK, 1
			if loop == "worker-desk" {
				wantCode, wantMints = deskkit.ExitRefused, 0
			}
			if code != wantCode || mints != wantMints {
				t.Fatalf("flip code=%d mints=%d; want code=%d mints=%d", code, mints, wantCode, wantMints)
			}
			flUnchanged(t, root)
		})
	}
}

// regression: #4823
func TestApprovalForgeCustody(t *testing.T) {
	oldFlag, oldToken, oldForge := acceptanceForge, ghToken, ghTokenForge
	acceptanceForge, ghToken = true, "fixture-foreign"
	t.Cleanup(func() { acceptanceForge, ghToken, ghTokenForge = oldFlag, oldToken, oldForge })
	for _, forge := range []deskkit.ForgeKind{deskkit.ForgeGitLab, ""} {
		ghTokenForge = forge
		for _, kind := range []closureAudit{auditLint, auditBrief, auditClosure, auditFlip, auditVerdict} {
			cmd := statusgenAudit(context.Background(), kind, "example-root")
			if !deskkit.IsUnverifiable(cmd.Err) {
				t.Errorf("%s accepts an unproven or foreign-forge approval credential", kind)
			}
			for _, env := range cmd.Env {
				if strings.Contains(env, "fixture-foreign") {
					t.Error("foreign credential reached the approval child environment")
				}
			}
		}
	}
}
