package clicontract

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Discovered is one entrypoint found in the tree, independent of the registry.
type Discovered struct {
	Path string // Go: the main package's directory; script: the file
	Kind string // go | script
}

// scriptExts are the extensions treated as script launchers. An extensionless file is a
// launcher when it starts with a "#!" interpreter line.
var scriptExts = map[string]bool{".sh": true, ".bash": true, ".ps1": true, ".py": true, ".mjs": true, ".js": true}

// skipDir reports directories discovery never enters: version-control state, vendored
// or installed dependencies, and hidden directories other than .github (local worktrees
// and editor state live there).
func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor":
		return true
	case ".github":
		return false
	}
	return strings.HasPrefix(name, ".") && name != "."
}

// Discover walks fsys and returns every Go main package directory and every script
// launcher, sorted by path. It reads only package clauses and the first two bytes of
// extensionless files.
func Discover(fsys fs.FS) ([]Discovered, error) {
	goMains := map[string]bool{}
	var out []Discovered
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		name := d.Name()
		ext := path.Ext(name)
		switch {
		case ext == ".go":
			if strings.HasSuffix(name, "_test.go") {
				return nil
			}
			src, rerr := fs.ReadFile(fsys, p)
			if rerr != nil {
				return rerr
			}
			f, perr := parser.ParseFile(token.NewFileSet(), p, src, parser.PackageClauseOnly)
			if perr != nil {
				return nil // not a parseable Go file; the compiler owns that failure
			}
			if f.Name.Name == "main" {
				goMains[path.Dir(p)] = true
			}
		case scriptExts[ext]:
			out = append(out, Discovered{Path: p, Kind: "script"})
		case ext == "":
			if hasShebang(fsys, p) {
				out = append(out, Discovered{Path: p, Kind: "script"})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for dir := range goMains {
		out = append(out, Discovered{Path: dir, Kind: "go"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func hasShebang(fsys fs.FS, p string) bool {
	f, err := fsys.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 2)
	n, _ := f.Read(b)
	return n == 2 && string(b) == "#!"
}

// MatchPattern reports whether p matches a registry glob: "**/" matches zero or more
// leading directories, "**" any run of characters, "*" any run without a slash.
func MatchPattern(pattern, p string) bool {
	return globRegexp(pattern).MatchString(p)
}

func globRegexp(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		switch {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 3
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i += 2
		case pattern[i] == '*':
			b.WriteString("[^/]*")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
			i++
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// Shipped reports whether an entrypoint is distributed or run as a product: anything in
// the plugin bundle, any desk command the Makefile packages, or a path named by the
// release workflow or the Makefile — except test-named suites (TestNamed). A shipped
// entrypoint can never be excluded.
func Shipped(fsys fs.FS, p string) bool {
	if TestNamed(p) {
		return false
	}
	if strings.HasPrefix(p, "plugins/") {
		return true
	}
	for _, f := range []string{".github/workflows/release.yml", "Makefile"} {
		raw, err := fs.ReadFile(fsys, f)
		if err != nil {
			continue
		}
		text := string(raw)
		if f == "Makefile" && path.Dir(p) == "tools/desk/cmd" && strings.Contains(text, "$(DESK_DIR)/cmd/*") {
			return true
		}
		if namesPath(text, p) {
			return true
		}
	}
	return false
}

// TestNamed reports whether a file carries a test-suite name (x.test.sh, x_test.sh,
// x.test.py): a release gate or the plugin bundle may run or carry it, but it is a test
// of an entrypoint, never an entrypoint an operator invokes.
func TestNamed(p string) bool {
	base := path.Base(p)
	stem := strings.TrimSuffix(base, path.Ext(base))
	return strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, "_test")
}

// namesPath reports whether text contains p as a whole path token.
func namesPath(text, p string) bool {
	re := regexp.MustCompile(`(^|[^\w./-])(\./)?` + regexp.QuoteMeta(p) + `($|[^\w-])`)
	return re.MatchString(text)
}
