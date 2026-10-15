package deskkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Defect class: a lock on the desk launch path that surfaces a bare ErrLockBusy
// — no lock name, no holder, no bounded-wait verdict (#2709). AcquireHeld is the
// choke point. In the launch-path packages below, a direct TryLockExclusive call
// is allowed only in the listed functions, each of which takes a lock whose busy
// read is classified by its caller rather than shown to an operator.
var launchLockPackages = []string{"../cellscratch", "../cellcache", "../cellcadence"}

var launchLockAllow = map[string]string{
	// Per-task execution leases: a busy one means "live task", which Sweep and
	// Acknowledge classify themselves; a fresh task's lease cannot be busy.
	"cellscratch/store.go:Store.lock": "per-task execution lease",
	// The reconcile preflight's non-destructive role-lease probe (#2880): it returns a bool the
	// caller classifies as "owned by a running window", never a bare ErrLockBusy, and it must
	// not go through AcquireHeld, which would overwrite a crashed owner's record.
	"cellcadence/evidence.go:Probe": "reconcile lease probe",
}

func rawLockSites(dir, name string, src any) ([]string, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, name, src, 0)
	if err != nil {
		return nil, err
	}
	var sites []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		owner := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) == 1 {
			t := fn.Recv.List[0].Type
			if s, ok := t.(*ast.StarExpr); ok {
				t = s.X
			}
			if id, ok := t.(*ast.Ident); ok {
				owner = id.Name + "." + owner
			}
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "TryLockExclusive" {
				sites = append(sites, fmt.Sprintf("%s/%s:%s", filepath.Base(dir), filepath.Base(name), owner))
			}
			return true
		})
	}
	return sites, nil
}

func TestLaunchLocksNameHolder(t *testing.T) {
	var unexpected []string
	for _, dir := range launchLockPackages {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("cannot list %s: %v", dir, err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			sites, err := rawLockSites(dir, file, src)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range sites {
				if launchLockAllow[s] == "" {
					unexpected = append(unexpected, s)
				}
			}
		}
	}
	sort.Strings(unexpected)
	if len(unexpected) > 0 {
		t.Fatalf("launch-path lock bypasses deskkit.AcquireHeld (bare ErrLockBusy, no lock or holder named): %v", unexpected)
	}
}

// Positive control: a planted second instance of the class is caught.
func TestLaunchLockGuardBites(t *testing.T) {
	planted := `package cellcache
import "github.com/medici-finance/assay/tools/desk/internal/deskkit"
func (m *manager) relock() error { return deskkit.TryLockExclusive(m.f) }`
	sites, err := rawLockSites("../cellcache", "planted.go", planted)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0] != "cellcache/planted.go:manager.relock" || launchLockAllow[sites[0]] != "" {
		t.Fatalf("planted raw lock not reported: %v", sites)
	}
}
