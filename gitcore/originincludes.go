package gitcore

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	format "github.com/go-git/go-git/v5/plumbing/format/config"
)

const originIncludeDepth = 8

// originIncludes observes Git's include language without importing any value.
// A condition is skipped only when Git's own rule provably does not match; every
// condition this observer cannot evaluate exactly is treated as matching. A
// (possibly) included file is refused only when it can affect fetch authority:
// remote, url or extensions sections. Identity, HTTP and credential values never
// enter the snapshot, its output, persistence, logs or transport.
type originIncludes struct {
	texts  []string // spellings Git may match a gitdir condition against
	branch string   // checked-out branch; "" with known means detached
	known  bool
	stack  map[string]bool
}

func (r *Repo) checkOriginIncludes(raw *format.Config, file string) error {
	if !raw.HasSection("include") && !raw.HasSection("includeIf") {
		return nil
	}
	admin, _, err := resolveGitDir(r.dir)
	if err != nil {
		return unavailable("repository layout is unreadable")
	}
	admin, err = filepath.Abs(admin)
	if err != nil {
		return unavailable("repository layout is unreadable")
	}
	physical, err := filepath.EvalSymlinks(admin)
	if err != nil {
		return unavailable("repository layout is unreadable")
	}
	// Git matches its realpath, then its absolute spelling (which follows $PWD
	// when $PWD names the current directory). Extra spellings only widen matches.
	texts := []string{filepath.ToSlash(physical), filepath.ToSlash(admin)}
	if pwd := os.Getenv("PWD"); filepath.IsAbs(pwd) {
		if real, err := filepath.EvalSymlinks(pwd); err == nil {
			if rel, err := filepath.Rel(real, physical); err == nil && filepath.IsLocal(rel) {
				texts = append(texts, filepath.ToSlash(filepath.Join(pwd, rel)))
			}
		}
	}
	observer := originIncludes{texts: texts, stack: map[string]bool{}}
	observer.branch, observer.known = headBranch(admin)
	if canonical, err := filepath.EvalSymlinks(file); err == nil {
		observer.stack[canonical] = true
	}
	return observer.includes(raw, file, 0)
}

// headBranch mirrors Git's onbranch input: the short name of the branch HEAD
// names symbolically. A detached HEAD is known not to be on any branch; a
// reftable placeholder, nested symbolic ref or unreadable HEAD is unknown.
func headBranch(admin string) (string, bool) {
	head, err := os.ReadFile(filepath.Join(admin, "HEAD"))
	if err != nil {
		return "", false
	}
	target, symbolic := strings.CutPrefix(strings.TrimRight(string(head), "\r\n"), "ref: ")
	if !symbolic {
		return "", true
	}
	name, ok := strings.CutPrefix(strings.TrimSpace(target), "refs/heads/")
	if !ok {
		return "", true
	}
	if name == "" || name == ".invalid" {
		return "", false
	}
	if common, err := commonDirOf(admin); err == nil {
		if common == "" {
			common = admin
		}
		if ref, err := os.ReadFile(filepath.Join(common, "refs", "heads", filepath.FromSlash(name))); err == nil && bytes.HasPrefix(ref, []byte("ref:")) {
			return "", false
		}
	} else {
		return "", false
	}
	return name, true
}

func (o *originIncludes) includes(raw *format.Config, file string, depth int) error {
	for _, path := range raw.Section("include").OptionAll("path") {
		if err := o.read(path, file, depth+1); err != nil {
			return err
		}
	}
	for _, sub := range raw.Section("includeIf").Subsections {
		if !o.match(sub.Name) {
			continue
		} // no path resolution, stat or open for a proven nonmatch
		for _, path := range sub.OptionAll("path") {
			if err := o.read(path, file, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// match reports whether Git may honour condition. False is returned only for a
// proven nonmatch of a gitdir, gitdir/i or onbranch condition.
func (o *originIncludes) match(condition string) bool {
	if runtime.GOOS == "windows" {
		return true // drive and separator rules are not modelled
	}
	if pattern, ok := strings.CutPrefix(condition, "onbranch:"); ok {
		if !o.known {
			return true
		}
		if o.branch == "" {
			return false
		}
		if strings.HasSuffix(pattern, "/") {
			pattern += "**"
		}
		return wildmatchMay(pattern, o.branch, false)
	}
	pattern, fold := "", false
	if p, ok := strings.CutPrefix(condition, "gitdir/i:"); ok {
		pattern, fold = p, true
	} else if p, ok := strings.CutPrefix(condition, "gitdir:"); ok {
		pattern = p
	} else {
		return true // hasconfig: and unknown conditions
	}
	// Git's interpolate_path: "~" and "~/" use $HOME; "~user" and "%(prefix)/"
	// are not modelled. "./" is relative to the config file's realpath.
	if pattern == "~" || strings.HasPrefix(pattern, "~/") {
		home := os.Getenv("HOME")
		if home == "" {
			return true
		}
		pattern = home + pattern[1:]
	} else if strings.HasPrefix(pattern, "~") || strings.HasPrefix(pattern, "%(prefix)/") || strings.HasPrefix(pattern, "./") {
		return true
	}
	if !strings.HasPrefix(pattern, "/") {
		pattern = "**/" + pattern
	}
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	for _, text := range o.texts {
		if wildmatchMay(pattern, text, fold) {
			return true
		}
	}
	return false
}

// wildmatchMay over-approximates Git's wildmatch(WM_PATHNAME): literal, "?",
// "*" and the slash-delimited "**" forms are exact; character classes, escapes
// and non-ASCII input are treated as matching.
func wildmatchMay(pattern, text string, fold bool) bool {
	if strings.ContainsAny(pattern, "[\\") {
		return true
	}
	for _, s := range []string{pattern, text} {
		for i := 0; i < len(s); i++ {
			if s[i] > 127 {
				return true
			}
		}
	}
	var b strings.Builder
	if fold {
		b.WriteString("(?i)")
	}
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		switch c := pattern[i]; c {
		case '*':
			j := i
			for j < len(pattern) && pattern[j] == '*' {
				j++
			}
			globstar := j-i >= 2 && (i == 0 || pattern[i-1] == '/') && (j == len(pattern) || pattern[j] == '/')
			switch {
			case globstar && j == len(pattern):
				b.WriteString(".*")
				i = j
			case globstar:
				b.WriteString("(?:.*/)?")
				i = j + 1
			default:
				b.WriteString("[^/]*")
				i = j
			}
		case '?':
			b.WriteString("[^/]")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
			i++
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	return err != nil || re.MatchString(text)
}

func (o *originIncludes) read(path, parent string, depth int) error {
	if depth > originIncludeDepth {
		return unavailable("an include is nested too deeply")
	}
	// Git's interpolate_path for include.path: "~" and "~/" use $HOME.
	if path == "~" || strings.HasPrefix(path, "~/") {
		home := os.Getenv("HOME")
		if home == "" {
			return unavailable("an include path is unsupported")
		}
		path = home + path[1:]
	} else if strings.HasPrefix(path, "~") || strings.HasPrefix(path, "%(prefix)/") {
		return unavailable("an include path is unsupported")
	}
	if !filepath.IsAbs(path) {
		// Git joins the including file's directory as spelled, without lexical
		// cleaning, and lets the kernel resolve the result.
		if slash := strings.LastIndexAny(parent, "/"+string(filepath.Separator)); slash >= 0 {
			path = parent[:slash+1] + path
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return nil // Git silently skips a missing include
		}
		return unavailable("an include is unreadable")
	}
	if !info.Mode().IsRegular() {
		return unavailable("an include is not a regular file")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return unavailable("an include is unreadable")
	}
	if o.stack[canonical] {
		return unavailable("an include forms a cycle")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return unavailable("an include is unreadable")
	}
	raw := format.New()
	if err := format.NewDecoder(bytes.NewReader(content)).Decode(raw); err != nil {
		return unavailable("an include is malformed")
	}
	for _, section := range raw.Sections {
		name := strings.ToLower(section.Name)
		if before, _, _ := strings.Cut(name, "."); before == "remote" || before == "url" || before == "extensions" {
			return unavailable("an include may affect authority")
		}
	}
	o.stack[canonical] = true
	defer delete(o.stack, canonical)
	return o.includes(raw, path, depth)
}
