// gitlaunch inventories actual native Git spawn definitions separately from
// operation callers through retained read seams. It is an offline source check,
// not a measure of executed processes or production performance.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type site struct {
	Key         string `json:"key"`
	Expression  string `json:"expression"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
}
type reason struct {
	Operation string `json:"operation"`
	Why       string `json:"why"`
	Owner     string `json:"owner"`
}
type inventory struct {
	Direct  int               `json:"direct_ceiling"`
	Dynamic int               `json:"dynamic_ceiling"`
	Seams   int               `json:"seam_ceiling"`
	Reasons map[string]reason `json:"reasons"`
	Sites   []site            `json:"sites"`
}

// These are the existing native seams plus typed adapters that retain explicit
// unsupported-profile branches. Both direct calls and aliases are inspected.
var nativeSeams = map[string]bool{"historyGit": true, "gitOut": true, "git": true, "coverageGit": true, "gitShowObject": true, "gitMergeBaseOut": true, "provenanceShowObject": true, "repoTopLevel": true, "provenanceRepoTop": true, "provenanceParents": true, "provenanceRootPrefix": true, "gitCurrentSHA": true, "isShallowRepository": true, "gitHeadOutput": true, "gitShallowOutput": true, "baselineGit": true}

type bindings struct {
	info   *types.Info
	values map[types.Object][]ast.Expr
}

func (b bindings) alternatives(e ast.Expr, seen map[types.Object]bool, stopSeams bool) []ast.Expr {
	if p, ok := e.(*ast.ParenExpr); ok {
		return b.alternatives(p.X, seen, stopSeams)
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return []ast.Expr{e}
	}
	// A declared native seam remains an operation boundary even when its
	// local definition is a function literal. Function aliases stop here;
	// executable-string candidates still follow all values below.
	if stopSeams && nativeSeams[id.Name] {
		return []ast.Expr{e}
	}
	obj := b.info.ObjectOf(id)
	if obj == nil || seen[obj] || len(b.values[obj]) == 0 {
		return []ast.Expr{e}
	}
	seen[obj] = true
	var out []ast.Expr
	for _, value := range b.values[obj] {
		branch := map[types.Object]bool{}
		for k, v := range seen {
			branch[k] = v
		}
		out = append(out, b.alternatives(value, branch, stopSeams)...)
	}
	return out
}
func (b bindings) strings(e ast.Expr, seen map[types.Object]bool) ([]string, bool) {
	var out []string
	unknown := false
	for _, a := range b.alternatives(e, seen, false) {
		switch v := a.(type) {
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				value, err := strconv.Unquote(v.Value)
				if err == nil {
					out = append(out, value)
				} else {
					unknown = true
				}
			} else {
				unknown = true
			}
		case *ast.BinaryExpr:
			if v.Op == token.ADD {
				xs, xu := b.strings(v.X, seen)
				ys, yu := b.strings(v.Y, seen)
				unknown = unknown || xu || yu
				for _, x := range xs {
					for _, y := range ys {
						out = append(out, x+y)
					}
				}
			} else {
				unknown = true
			}
		default:
			// Known alternatives never erase an unresolved reassignment.
			unknown = true
		}
	}
	return out, unknown
}

func scanFile(name string, src []byte) ([]site, error) {
	return scanFiles(map[string][]byte{name: src})
}

// scanFiles resolves bindings across all non-test source files of one package.
// Type-check diagnostics are not a build verdict: only source bindings are used;
// the normal module build remains separately required. Imported os/exec functions
// are identified by their defining package, including cross-file value aliases.
func scanFiles(sources map[string][]byte) ([]site, error) {
	fset := token.NewFileSet()
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	files := make([]*ast.File, 0, len(names))
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, sources[name], parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return nil, nil
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	cfg := types.Config{Importer: importer.Default(), Error: func(error) {}}
	_, _ = cfg.Check("source-inventory", fset, files, info)
	bound := bindings{info: info, values: map[types.Object][]ast.Expr{}}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch d := n.(type) {
			case *ast.ValueSpec:
				for i, id := range d.Names {
					if i < len(d.Values) {
						if obj := info.ObjectOf(id); obj != nil {
							bound.values[obj] = append(bound.values[obj], d.Values[i])
						}
					}
				}
			case *ast.AssignStmt:
				for i, e := range d.Lhs {
					if id, ok := e.(*ast.Ident); ok && i < len(d.Rhs) {
						if obj := info.ObjectOf(id); obj != nil {
							bound.values[obj] = append(bound.values[obj], d.Rhs[i])
						}
					}
				}
			}
			return true
		})
	}

	var out []site
	var formatErr error
	for i, f := range files {
		name := names[i]
		inspect := func(owner string, n ast.Node) {
			ord := map[string]int{}
			ast.Inspect(n, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				kind := ""
				for _, fun := range bound.alternatives(c.Fun, map[types.Object]bool{}, true) {
					command := ""
					var obj types.Object
					switch fn := fun.(type) {
					case *ast.SelectorExpr:
						obj = info.ObjectOf(fn.Sel)
					case *ast.Ident:
						obj = info.ObjectOf(fn)
					}
					if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "os/exec" {
						command = fn.Name()
					}
					if id, ok := fun.(*ast.Ident); ok && nativeSeams[id.Name] {
						kind = "seam"
					}
					index := -1
					if command == "Command" {
						index = 0
					}
					if command == "CommandContext" {
						index = 1
					}
					if index >= 0 && len(c.Args) > index {
						bins, unknown := bound.strings(c.Args[index], map[types.Object]bool{})
						// An unresolved executable can name Git at runtime. Keep it in
						// the conservative native-spawn inventory; dynamic argv never
						// exempts a construction site from the ratchet.
						if unknown || len(bins) == 0 {
							kind = "dynamic"
						}
						for _, bin := range bins {
							base := strings.ToLower(filepath.Base(bin))
							if base == "git" || base == "git.exe" {
								kind = "direct"
							} else if kind != "direct" {
								switch base {
								case "sh", "bash", "dash", "zsh", "ksh", "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe", "env":
									kind = "dynamic"
								}
							}
						}
					}
				}
				if kind == "" {
					return true
				}
				ord[kind]++
				var b bytes.Buffer
				if err := format.Node(&b, fset, c); err != nil {
					formatErr = err
					return false
				}
				out = append(out, site{Key: fmt.Sprintf("%s::%s::%s#%d", name, owner, kind, ord[kind]), Expression: b.String()})
				return true
			})
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				owner := d.Name.Name
				if d.Recv != nil {
					owner = "method." + owner
				}
				if d.Body != nil {
					inspect(owner, d.Body)
				}
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					if v, ok := sp.(*ast.ValueSpec); ok {
						for i, value := range v.Values {
							owner := "initializer"
							if i < len(v.Names) {
								owner = v.Names[i].Name
							}
							inspect(owner, value)
						}
					}
				}
			}
		}

	}
	return out, formatErr
}

func scan(root string) ([]site, error) {
	groups := map[string]map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header, err := parser.ParseFile(token.NewFileSet(), path, src, parser.PackageClauseOnly)
		if err != nil {
			return err
		}
		group := filepath.Dir(rel) + "\x00" + header.Name.Name
		if groups[group] == nil {
			groups[group] = map[string][]byte{}
		}
		groups[group][filepath.ToSlash(rel)] = src
		return nil
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []site
	for _, key := range keys {
		sites, err := scanFiles(groups[key])
		if err != nil {
			return nil, err
		}
		out = append(out, sites...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
func verify(got []site, want inventory) error {
	recorded := map[string]site{}
	direct, dynamic, seams := 0, 0, 0
	for _, s := range want.Sites {
		if _, exists := recorded[s.Key]; exists {
			return fmt.Errorf("duplicate recorded site %s", s.Key)
		}
		if s.Disposition != "moved" && s.Disposition != "retained" && s.Disposition != "deferred" {
			return fmt.Errorf("unclassified site %s", s.Key)
		}
		r, ok := want.Reasons[s.Reason]
		if !ok || r.Operation == "" || r.Why == "" || r.Owner == "" {
			return fmt.Errorf("site %s lacks operation/reason/owner", s.Key)
		}
		if strings.Contains(s.Key, "::direct#") {
			direct++
		} else if strings.Contains(s.Key, "::dynamic#") {
			dynamic++
		} else if strings.Contains(s.Key, "::seam#") {
			seams++
		} else {
			return fmt.Errorf("unknown inventory kind %s", s.Key)
		}
		recorded[s.Key] = s
	}
	if direct != want.Direct || dynamic != want.Dynamic || seams != want.Seams {
		return fmt.Errorf("ceiling must equal classified inventory: %d/%d/%d recorded, %d/%d/%d ceiling", direct, dynamic, seams, want.Direct, want.Dynamic, want.Seams)
	}
	actualDirect, actualDynamic, actualSeams := 0, 0, 0
	for _, s := range got {
		if strings.Contains(s.Key, "::direct#") {
			actualDirect++
		} else if strings.Contains(s.Key, "::dynamic#") {
			actualDynamic++
		} else if strings.Contains(s.Key, "::seam#") {
			actualSeams++
		} else {
			return fmt.Errorf("unknown source kind %s", s.Key)
		}
		old, ok := recorded[s.Key]
		if !ok {
			return fmt.Errorf("unrecorded Git launch or native seam caller: %s: %s", s.Key, s.Expression)
		}
		if old.Expression != s.Expression {
			return fmt.Errorf("changed Git operation at %s: %s", s.Key, s.Expression)
		}
		delete(recorded, s.Key)
	}
	if len(recorded) > 0 {
		var stale []string
		for k := range recorded {
			stale = append(stale, k)
		}
		sort.Strings(stale)
		return fmt.Errorf("retired Git sites must lower the inventory and ceiling: %s", strings.Join(stale, ", "))
	}
	if actualDirect > want.Direct || actualDynamic > want.Dynamic || actualSeams > want.Seams {
		return fmt.Errorf("Git launch ratchet exceeded")
	}
	fmt.Printf("statusgen git launches: %d literal Git spawn definitions (ceiling %d); %d unresolved executable constructions (ceiling %d); %d operation callers through native seams (ceiling %d)\n", actualDirect, want.Direct, actualDynamic, want.Dynamic, actualSeams, want.Seams)
	return nil
}
func ceilingDown(prior, current inventory) error {
	if current.Direct > prior.Direct || current.Dynamic > prior.Dynamic || current.Seams > prior.Seams {
		return fmt.Errorf("native Git ceilings may only decrease: old %d/%d/%d new %d/%d/%d", prior.Direct, prior.Dynamic, prior.Seams, current.Direct, current.Dynamic, current.Seams)
	}
	return nil
}

// baselineGit only reads the immutable comparison base. Its construction and
// callers are included in this source inventory, separate from CLI workloads.
func baselineGit(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"--no-replace-objects", "-C", root}, args...)...)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_GRAFT_FILE":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "GIT_GRAFT_FILE="+os.DevNull)
	return cmd.Output()
}

func checkPrior(root string, current inventory) error {
	out, err := baselineGit(root, "merge-base", "refs/remotes/origin/main", "HEAD")
	if err != nil {
		return fmt.Errorf("could-not-check prior ratchet revision: %w", err)
	}
	base := strings.TrimSpace(string(out))
	if len(base) != 40 && len(base) != 64 {
		return fmt.Errorf("invalid prior ratchet revision")
	}
	for _, c := range base {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return fmt.Errorf("invalid prior ratchet revision")
		}
	}
	// --full-tree is essential: root is the statusgen subdirectory, while this
	// path names the immutable repository-root file, independent of cwd prefix.
	out, err = baselineGit(root, "ls-tree", "--full-tree", "-z", base, "--", "statusgen/git-launches.json")
	if err != nil {
		return fmt.Errorf("could-not-check prior ratchet existence: %w", err)
	}
	if len(out) == 0 {
		fmt.Println("initial statusgen ratchet: immutable merge base has no ceiling file")
		return nil
	}
	records := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	if len(records) != 1 {
		return fmt.Errorf("ambiguous prior ratchet entry")
	}
	meta, path, ok := strings.Cut(string(records[0]), "\t")
	fields := strings.Fields(meta)
	if !ok || path != "statusgen/git-launches.json" || len(fields) != 3 || fields[1] != "blob" {
		return fmt.Errorf("invalid prior ratchet entry")
	}
	out, err = baselineGit(root, "show", base+":statusgen/git-launches.json")
	if err != nil {
		return fmt.Errorf("could-not-check prior ratchet: %w", err)
	}
	var prior inventory
	if err := json.Unmarshal(out, &prior); err != nil {
		return fmt.Errorf("invalid prior ratchet: %w", err)
	}
	return ceilingDown(prior, current)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gitlaunch <statusgen-source-root>")
		os.Exit(2)
	}
	root := os.Args[1]
	sites, err := scan(root)
	if err == nil {
		var inv inventory
		var data []byte
		data, err = os.ReadFile(filepath.Join(root, "git-launches.json"))
		if err == nil {
			err = json.Unmarshal(data, &inv)
		}
		if err == nil {
			err = checkPrior(root, inv)
		}
		if err == nil {
			err = verify(sites, inv)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
