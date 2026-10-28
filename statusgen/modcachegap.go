package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The verify-runner image's baked Go module cache against the tree under
// verification (#4683).
//
// The image provisions Go modules by baking them at build time: its Dockerfile
// runs `go mod download` for a fixed set of module roots and writes those roots,
// one per line, to bakedModuleRootsFile. Rows then run with no network, a
// read-only root and GOPROXY=off, so a row can resolve only what was baked. The
// image is built from a release, while the tree under verification is merged
// main; when main has moved a requirement since that release, every Go row in
// that module fails on module resolution before any test runs, and the failure
// reads like N unrelated row failures.
//
// moduleCacheGap names that cause. In container-netns mode, before any row, it
// reads each baked root's go.mod in the tree and lists the requirements the
// image's module cache does not hold. A row that then exits non-zero and whose
// output names one of those missing modules never reached its own check: it is
// recorded could-not-run with modCacheGapNote, which names the module, the
// version the tree wants and the versions the image has. Nothing is predicted:
// a row that does not trip on a missing module keeps its own verdict.

// bakedModuleRootsFile is where the verify-runner image records the module
// roots it baked. Absent (a runner that is not that image), no check runs. A
// var so tests can point it at a fixture.
var bakedModuleRootsFile = "/usr/local/share/verify-runner/go-module-roots.txt"

// modGap is one requirement of the tree the module cache lacks.
type modGap struct {
	Root string   // the tree-relative module root whose go.mod requires it
	Path string   // the module path
	Want string   // the version the tree requires
	Have []string // the versions of Path the cache does hold, sorted
}

// modRequirement is one module requirement a go.mod resolves to, after its
// replace directives.
type modRequirement struct{ Path, Version string }

// bakedModuleRoots reads the image's baked-roots manifest. ok is false when
// the file is absent or names no root.
func bakedModuleRoots(path string) (roots []string, ok bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	for _, l := range strings.Split(string(raw), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		roots = append(roots, filepath.ToSlash(filepath.Clean(l)))
	}
	return roots, len(roots) > 0
}

// moduleCacheDir is the cache rows resolve modules from: GOMODCACHE, else the
// first GOPATH entry's pkg/mod. Empty when neither is set.
func moduleCacheDir() string {
	if d := os.Getenv("GOMODCACHE"); d != "" {
		return d
	}
	if gp := os.Getenv("GOPATH"); gp != "" {
		return filepath.Join(filepath.SplitList(gp)[0], "pkg", "mod")
	}
	return ""
}

// runModuleCacheGap is the container-netns pre-row check: the gaps between the
// tree at root and the module cache, for the roots the image baked. Nil when
// this runner records no baked roots or names no cache.
func runModuleCacheGap(root string) []modGap {
	roots, ok := bakedModuleRoots(bakedModuleRootsFile)
	cache := moduleCacheDir()
	if !ok || cache == "" {
		return nil
	}
	return moduleCacheGap(root, roots, cache)
}

// moduleCacheGap lists, for each module root's go.mod under root, every
// requirement whose .mod file, or whose .zip and extracted directory both, are
// missing from cache. A root whose go.mod is absent or unreadable contributes
// nothing: its rows fail, or not, on their own.
func moduleCacheGap(root string, roots []string, cache string) []modGap {
	var gaps []modGap
	seen := map[string]bool{}
	for _, r := range roots {
		reqs, err := goModRequirements(filepath.Join(root, filepath.FromSlash(r), "go.mod"))
		if err != nil {
			continue
		}
		for _, q := range reqs {
			key := q.Path + "@" + q.Version
			if seen[key] || moduleCached(cache, q.Path, q.Version) {
				continue
			}
			seen[key] = true
			gaps = append(gaps, modGap{Root: r, Path: q.Path, Want: q.Version, Have: cachedVersions(cache, q.Path)})
		}
	}
	return gaps
}

// moduleCached reports whether cache holds what the go command reads for
// path@version offline: its go.mod, and its source as a zip or extracted tree.
func moduleCached(cache, path, version string) bool {
	ep, ev := escapeModulePath(path), escapeModulePath(version)
	dl := filepath.Join(cache, "cache", "download", filepath.FromSlash(ep), "@v")
	if !fileExists(filepath.Join(dl, ev+".mod")) {
		return false
	}
	return fileExists(filepath.Join(dl, ev+".zip")) || fileExists(filepath.Join(cache, filepath.FromSlash(ep)+"@"+ev))
}

// cachedVersions lists the versions of path the cache holds a zip for.
func cachedVersions(cache, path string) []string {
	dl := filepath.Join(cache, "cache", "download", filepath.FromSlash(escapeModulePath(path)), "@v")
	entries, err := os.ReadDir(dl)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if v, ok := strings.CutSuffix(e.Name(), ".zip"); ok {
			out = append(out, unescapeModulePath(v))
		}
	}
	sort.Slice(out, func(i, j int) bool { return semverLess(out[i], out[j]) })
	return out
}

// escapeModulePath is the module cache's case encoding: each upper-case letter
// becomes '!' and its lower-case form.
func escapeModulePath(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func unescapeModulePath(s string) string {
	var b strings.Builder
	up := false
	for _, r := range s {
		if r == '!' {
			up = true
			continue
		}
		if up && r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		up = false
		b.WriteRune(r)
	}
	return b.String()
}

// goModRequirements reads a go.mod's require directives, single-line and
// block, and applies its replace directives: a requirement replaced by a local
// directory is dropped (nothing to resolve from the cache), one replaced by
// another module version becomes that version.
func goModRequirements(path string) ([]modRequirement, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var reqs []modRequirement
	type repl struct{ to, toVersion string }
	replaces := map[string]repl{} // key: path, or path@version
	block := ""                   // "require" or "replace" inside a ( ) block
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		var kind string
		var args []string
		if block != "" {
			if fields[0] == ")" {
				block = ""
				continue
			}
			kind, args = block, fields
		} else {
			if fields[0] != "require" && fields[0] != "replace" {
				continue
			}
			if len(fields) == 2 && fields[1] == "(" {
				block = fields[0]
				continue
			}
			kind, args = fields[0], fields[1:]
		}
		switch kind {
		case "require":
			if len(args) >= 2 {
				reqs = append(reqs, modRequirement{Path: unquote(args[0]), Version: unquote(args[1])})
			}
		case "replace":
			// old [version] => new [version]
			arrow := -1
			for i, a := range args {
				if a == "=>" {
					arrow = i
					break
				}
			}
			if arrow < 1 || arrow > 2 || arrow+1 >= len(args) {
				continue
			}
			key := unquote(args[0])
			if arrow == 2 {
				key += "@" + unquote(args[1])
			}
			r := repl{to: unquote(args[arrow+1])}
			if arrow+2 < len(args) {
				r.toVersion = unquote(args[arrow+2])
			}
			replaces[key] = r
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := reqs[:0]
	for _, q := range reqs {
		r, ok := replaces[q.Path+"@"+q.Version]
		if !ok {
			r, ok = replaces[q.Path]
		}
		switch {
		case !ok:
			out = append(out, q)
		case r.toVersion == "":
			// a local directory: no module to resolve
		default:
			out = append(out, modRequirement{Path: r.to, Version: r.toVersion})
		}
	}
	return out, nil
}

// gapsNamedIn returns the gaps a row's output names: go's own lines name a
// module it could not resolve as `path@version`, as `downloading path version`
// or by its cache path `cache/download/<escaped path>/@v/<version>.`.
func gapsNamedIn(output []byte, gaps []modGap) []modGap {
	var hit []modGap
	for _, g := range gaps {
		p, v := regexp.QuoteMeta(g.Path), regexp.QuoteMeta(g.Want)
		cachePath := regexp.QuoteMeta("cache/download/"+escapeModulePath(g.Path)+"/@v/"+escapeModulePath(g.Want)) + `\.`
		re := regexp.MustCompile(`(?:` + p + `@` + v + `|downloading ` + p + ` ` + v + `)(?:[^0-9A-Za-z.+-]|$)|` + cachePath)
		if re.Match(output) {
			hit = append(hit, g)
		}
	}
	return hit
}

// maxGapsNamed bounds how many modules one note names.
const maxGapsNamed = 3

// modCacheGapCause names the gaps: each module, the version the tree wants and
// the versions the image has.
func modCacheGapCause(gaps []modGap) string {
	parts := make([]string, 0, maxGapsNamed+1)
	for i, g := range gaps {
		if i == maxGapsNamed {
			parts = append(parts, fmt.Sprintf("and %d more", len(gaps)-maxGapsNamed))
			break
		}
		have := "none"
		if len(g.Have) > 0 {
			have = strings.Join(g.Have, ",")
		}
		parts = append(parts, g.Path+" "+g.Want+" vs "+have)
	}
	return "image module cache " + gapDirection(gaps) + " go.sum (" + strings.Join(parts, "; ") + ")"
}

// gapDirection is "predates" when the image holds only older versions of every
// missing module (main moved past the release that built the image, the usual
// case), else "does not match" (an older tree, or a module the image never had).
func gapDirection(gaps []modGap) string {
	for _, g := range gaps {
		if len(g.Have) == 0 {
			return "does not match"
		}
		for _, h := range g.Have {
			if !semverLess(h, g.Want) {
				return "does not match"
			}
		}
	}
	return "predates"
}

// semverLess orders two module versions (vMAJOR.MINOR.PATCH[-pre][+build]):
// numerically on the release triple, then a pre-release before its release,
// then pre-releases as strings (which orders pseudo-versions by timestamp).
func semverLess(a, b string) bool {
	ra, pa := splitSemver(a)
	rb, pb := splitSemver(b)
	for i := range 3 {
		if ra[i] != rb[i] {
			return ra[i] < rb[i]
		}
	}
	switch {
	case pa == pb:
		return false
	case pa == "":
		return false
	case pb == "":
		return true
	}
	return pa < pb
}

func splitSemver(v string) (rel [3]int, pre string) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, pre = v[:i], v[i+1:]
	}
	for i, p := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(p)
		rel[i] = n
	}
	return rel, pre
}

// modCacheGapNote is the could-not-run reason for a row that tripped on a
// module the image's cache lacks: the cause, then the remedy.
func modCacheGapNote(gaps []modGap) string {
	return modCacheGapCause(gaps) + ": the row stopped on module resolution before its check ran. " +
		"Re-run in the verify-runner image of a release whose " + gapRoots(gaps) +
		" go.mod and go.sum match the verified tree, normally the first release that contains the verified commit"
}

// gapRoots names the module roots the gaps come from, in first-seen order.
func gapRoots(gaps []modGap) string {
	var roots []string
	seen := map[string]bool{}
	for _, g := range gaps {
		if !seen[g.Root] {
			seen[g.Root] = true
			roots = append(roots, g.Root)
		}
	}
	return strings.Join(roots, " and ")
}

// modCacheGapNotice is the one stderr line a container-netns run prints before
// its rows when the cache lacks requirements of the tree, so the cause is
// stated once, ahead of the rows it will touch.
func modCacheGapNotice(gaps []modGap) string {
	return "statusgen verifyrun: " + modCacheGapCause(gaps) +
		"; a row that stops on one of these modules is recorded could-not-run"
}
