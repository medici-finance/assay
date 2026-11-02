package gitcore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	format "github.com/go-git/go-git/v5/plumbing/format/config"
)

const originIncludeDepth = 8

// Only authority-neutral include contents are supported. This covers checkout-v7
// authentication files without importing their headers/helpers into the authority
// snapshot, or implementing Git's general inline/conditional include language.
type originIncludes struct {
	dirs  []string
	stack map[string]bool
}

func (r *Repo) checkOriginIncludes(raw *format.Config, file string) error {
	if !raw.HasSection("include") && !raw.HasSection("includeIf") {
		return nil
	}
	admin, _, err := resolveGitDir(r.dir)
	if err != nil {
		return err
	}
	admin, err = filepath.Abs(admin)
	if err != nil {
		return err
	}
	physical, err := filepath.EvalSymlinks(admin)
	if err != nil {
		return err
	}
	canonical, err := filepath.EvalSymlinks(file)
	if err != nil {
		return err
	}
	observer := originIncludes{dirs: []string{admin, physical}, stack: map[string]bool{canonical: true}}
	return observer.includes(raw, file, 0)
}

func (o *originIncludes) includes(raw *format.Config, file string, depth int) error {
	for _, path := range raw.Section("include").OptionAll("path") {
		if err := o.read(path, file, depth+1); err != nil {
			return err
		}
	}
	for _, sub := range raw.Section("includeIf").Subsections {
		matched, err := o.match(sub.Name)
		if err != nil {
			return err
		}
		if !matched {
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

func (o *originIncludes) match(condition string) (bool, error) {
	pattern, fold := condition, false
	if strings.HasPrefix(pattern, "gitdir/i:") {
		pattern = strings.TrimPrefix(pattern, "gitdir/i:")
		fold = true
	} else if strings.HasPrefix(pattern, "gitdir:") {
		pattern = strings.TrimPrefix(pattern, "gitdir:")
	} else {
		return false, fmt.Errorf("unsupported origin include condition")
	}
	// Support the absolute ASCII exact/single-component shape used by checkout-v7 here.
	// Git's relative, trailing-slash and double-star expansions remain unavailable.
	if !filepath.IsAbs(filepath.FromSlash(pattern)) || strings.Contains(pattern, "**") || strings.ContainsAny(pattern, "?[]\\~") || strings.HasSuffix(pattern, "/") {
		return false, fmt.Errorf("unsupported origin gitdir pattern")
	}
	for _, value := range append([]string{pattern}, o.dirs...) {
		for _, r := range value {
			if r > 127 {
				return false, fmt.Errorf("non-ASCII origin include condition")
			}
		}
	}
	for _, dir := range o.dirs {
		candidate := filepath.ToSlash(dir)
		if fold {
			candidate = strings.ToLower(candidate)
		}
		current := pattern
		if fold {
			current = strings.ToLower(current)
		}
		matched, err := filepath.Match(filepath.FromSlash(current), filepath.FromSlash(candidate))
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (o *originIncludes) read(path, parent string, depth int) error {
	if depth > originIncludeDepth {
		return fmt.Errorf("origin include depth exceeded")
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, path[2:])
	} else if strings.HasPrefix(path, "~") {
		return fmt.Errorf("unsupported origin include path")
	}
	if !filepath.IsAbs(path) {
		dir, err := filepath.Abs(filepath.Dir(parent))
		if err != nil {
			return err
		}
		physical, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return err
		}
		if physical != dir {
			return fmt.Errorf("relative origin include through aliased directory")
		}
		path = filepath.Join(dir, path)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if o.stack[canonical] {
		return fmt.Errorf("origin include cycle")
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("origin include is not a regular file")
	}
	content, err := os.ReadFile(canonical)
	if err != nil {
		return err
	}
	raw := format.New()
	if err := format.NewDecoder(bytes.NewReader(content)).Decode(raw); err != nil {
		return err
	}
	for _, section := range raw.Sections {
		switch strings.ToLower(section.Name) {
		case "http", "credential", "include", "includeif":
		default:
			return fmt.Errorf("origin include may affect authority")
		}
	}
	// Values in HTTP/credential sections never enter authority configuration or
	// output, persistence, logs or transport. Only neutral nested includes are read.
	o.stack[canonical] = true
	defer delete(o.stack, canonical)
	return o.includes(raw, path, depth)
}
