package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// specSite is the source a muhar spec entry mutates: the lines its `old` text
// spans on its first occurrence (muhar replaces the first occurrence), and the
// function those lines sit in.
type specSite struct {
	Spec      string `json:"spec"`
	Entry     string `json:"entry"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Func      string `json:"func"`
}

// staleEntry is a spec entry whose target could not be located. muhar itself
// refuses such an entry, so this is reported, never silently dropped.
type staleEntry struct {
	Spec   string `json:"spec"`
	Entry  string `json:"entry"`
	File   string `json:"file"`
	Reason string `json:"reason"`
}

type specIndex struct {
	Specs  []string
	byFile map[string][]specSite
	Stale  []staleEntry
	src    *sourceCache
}

type specEntry struct {
	Name string `json:"name"`
	File string `json:"file"`
	Old  string `json:"old"`
}

type specDoc struct {
	Control   *specEntry  `json:"control"`
	Mutations []specEntry `json:"mutations"`
}

// loadSpecs reads every mutations*.json under moduleRoot (skipping testdata and
// dot directories). Entry file paths are relative to moduleRoot, as muhar
// resolves them. Every spec is read because one package's code is often guarded
// by a spec that lives beside a different package.
func loadSpecs(moduleRoot string, src *sourceCache) (*specIndex, error) {
	idx := &specIndex{byFile: map[string][]specSite{}, src: src}
	root := os.DirFS(moduleRoot)
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != "." && (name == "testdata" || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if ok, _ := path.Match("mutations*.json", d.Name()); !ok {
			return nil
		}
		return idx.addSpec(moduleRoot, p)
	})
	if err != nil {
		return nil, err
	}
	if len(idx.Specs) == 0 {
		return nil, fmt.Errorf("no mutations*.json found under %s", moduleRoot)
	}
	return idx, nil
}

func (idx *specIndex) addSpec(moduleRoot, rel string) error {
	b, err := os.ReadFile(filepath.Join(moduleRoot, filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	var doc specDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("%s: %v", rel, err)
	}
	idx.Specs = append(idx.Specs, rel)
	entries := doc.Mutations
	if doc.Control != nil {
		entries = append([]specEntry{*doc.Control}, entries...)
	}
	for _, e := range entries {
		file := path.Clean(filepath.ToSlash(e.File))
		site, reason := idx.locate(file, e.Old)
		if reason != "" {
			idx.Stale = append(idx.Stale, staleEntry{Spec: rel, Entry: e.Name, File: file, Reason: reason})
			continue
		}
		site.Spec, site.Entry = rel, e.Name
		idx.byFile[file] = append(idx.byFile[file], site)
	}
	return nil
}

func (idx *specIndex) locate(file, old string) (specSite, string) {
	if old == "" {
		return specSite{}, "empty old text"
	}
	sf, err := idx.src.get(file)
	if err != nil {
		return specSite{}, "file unreadable"
	}
	at := strings.Index(sf.text, old)
	if at < 0 {
		return specSite{}, "old text not found"
	}
	start := 1 + strings.Count(sf.text[:at], "\n")
	end := start + strings.Count(old, "\n")
	return specSite{File: file, StartLine: start, EndLine: end, Func: sf.funcAt(start)}, ""
}

// coverage classes for a LIVED mutant.
const (
	classSpecLine  = "spec-line" // a spec entry mutates this very line
	classSpecFunc  = "spec-func" // a spec entry mutates elsewhere in the same function
	classCandidate = "candidate" // no spec entry anywhere in the function
)

// classify places a LIVED mutant at file:line against the specs. fileSpecced
// reports whether any spec touches the file at all.
func (idx *specIndex) classify(file string, line int) (class, fn string, fileSpecced bool) {
	sites := idx.byFile[file]
	fileSpecced = len(sites) > 0
	for _, s := range sites {
		if line >= s.StartLine && line <= s.EndLine {
			return classSpecLine, s.Func, true
		}
	}
	if sf, err := idx.src.get(file); err == nil {
		fn = sf.funcAt(line)
	}
	if fn != "" {
		for _, s := range sites {
			if s.Func == fn {
				return classSpecFunc, fn, true
			}
		}
	}
	return classCandidate, fn, fileSpecced
}

// sourceFile is one Go file's text and its top-level function spans.
type sourceFile struct {
	text  string
	lines []string
	funcs []funcSpan
}

type funcSpan struct {
	name       string
	start, end int
}

func (sf *sourceFile) funcAt(line int) string {
	for _, f := range sf.funcs {
		if line >= f.start && line <= f.end {
			return f.name
		}
	}
	return ""
}

func (sf *sourceFile) line(n int) string {
	if n < 1 || n > len(sf.lines) {
		return ""
	}
	return sf.lines[n-1]
}

// sourceCache reads and parses each module-relative file once.
type sourceCache struct {
	root  string
	files map[string]*sourceFile
	errs  map[string]error
}

func newSourceCache(root string) *sourceCache {
	return &sourceCache{root: root, files: map[string]*sourceFile{}, errs: map[string]error{}}
}

func (c *sourceCache) get(rel string) (*sourceFile, error) {
	if sf, ok := c.files[rel]; ok {
		return sf, nil
	}
	if err, ok := c.errs[rel]; ok {
		return nil, err
	}
	sf, err := parseSource(filepath.Join(c.root, filepath.FromSlash(rel)))
	if err != nil {
		c.errs[rel] = err
		return nil, err
	}
	c.files[rel] = sf
	return sf, nil
}

func parseSource(p string) (*sourceFile, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	sf := &sourceFile{text: string(b), lines: strings.Split(string(b), "\n")}
	if !strings.HasSuffix(p, ".go") {
		return sf, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, p, b, parser.SkipObjectResolution)
	if err != nil {
		// Spans stay empty; lines are still classified by spec line ranges.
		return sf, nil
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		sf.funcs = append(sf.funcs, funcSpan{
			name:  funcName(fd),
			start: fset.Position(fd.Pos()).Line,
			end:   fset.Position(fd.End()).Line,
		})
	}
	sort.Slice(sf.funcs, func(i, j int) bool { return sf.funcs[i].start < sf.funcs[j].start })
	return sf, nil
}

// funcName is "Name" for a function and "Type.Name" for a method.
func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
			continue
		case *ast.IndexExpr:
			t = x.X
			continue
		case *ast.IndexListExpr:
			t = x.X
			continue
		case *ast.Ident:
			return x.Name + "." + fd.Name.Name
		}
		return fd.Name.Name
	}
}
