package deskkit

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
)

// InputKeyScopePrefix binds declaration membership, including planned absence.
// Older readers cannot resolve this additive key and report could-not-check.
const InputKeyScopePrefix = "scope:"

// scopeDigestFormat is exactly what ResolveDeclaredScope writes as a scope: value.
var scopeDigestFormat = regexp.MustCompile(`^[0-9a-f]{64}$`)

func scopeDeclIsGlob(decl string) bool { return strings.ContainsAny(decl, "*?[") }

// ScopeInputKey is the input key for a canonical declaration whose literal path was
// observed as kind. A literal directory ("tree") is keyed with a trailing slash; a
// literal file, an absent literal and every glob keep scope:<decl>. The slash belongs
// to the key, never to the declaration, and the digest is the same either way.
func ScopeInputKey(decl, kind string) string {
	if !scopeDeclIsGlob(decl) && kind == "tree" {
		return InputKeyScopePrefix + decl + "/"
	}
	return InputKeyScopePrefix + decl
}

// ParseScopeInputKey returns the canonical declaration a scope: key names, and whether
// the key was in the directory (trailing slash) form. It strips at most one slash and
// never normalises: an empty declaration, a double slash, a non-canonical declaration
// or a slash-form glob (never emitted) is ok=false.
func ParseScopeInputKey(key string) (decl string, dirForm bool, ok bool) {
	rest, found := strings.CutPrefix(key, InputKeyScopePrefix)
	if !found {
		return "", false, false
	}
	decl, dirForm = strings.CutSuffix(rest, "/")
	if decl == "" || strings.HasSuffix(decl, "/") {
		return "", false, false
	}
	if dirForm && scopeDeclIsGlob(decl) {
		return "", false, false
	}
	canonical, err := expandDeclaration(decl)
	if err != nil || len(canonical) != 1 || canonical[0] != decl {
		return "", false, false
	}
	return decl, dirForm, true
}

const maxScopeWork = 32768

// maxScopePaths bounds the declared paths (see briefdoc.MaxScopePaths).
const maxScopePaths = briefdoc.MaxScopePaths

// The Context files: declaration grammar lives in statusgen/briefdoc
// (declared.go); these are the deskkit-facing projections of it, mapping a
// briefdoc.Refusal onto the refusal exit.

func fromBriefdoc(err error) error {
	if r, ok := err.(*briefdoc.Refusal); ok {
		return Refused(r.Msg)
	}
	return err
}

// BriefDeclaredFiles reads only declaration positions in Context's files field.
// Unsupported declarations are errors, never an accidentally empty input scope.
func BriefDeclaredFiles(content string) ([]string, error) {
	out, err := briefdoc.DeclaredFiles(content)
	return out, fromBriefdoc(err)
}

// TrimScopeAnnotation drops the trailing " (...)" annotation an author wrote
// inside a path's code span (see briefdoc.TrimScopeAnnotation).
func TrimScopeAnnotation(p string) string { return briefdoc.TrimScopeAnnotation(p) }

func expandDeclaration(decl string) ([]string, error) {
	out, err := briefdoc.ExpandDeclaration(decl)
	return out, fromBriefdoc(err)
}

func canonicalRepoPath(p string) bool { return briefdoc.CanonicalRepoPath(p) }

// SiblingDeclaration splits a ../<sibling>/<path> declaration (see
// briefdoc.SiblingDeclaration).
func SiblingDeclaration(decl string) (sibling, rel string, ok bool) {
	return briefdoc.SiblingDeclaration(decl)
}

// SplitSiblingDeclarations separates the declarations this tree resolves from
// the sibling-repository ones only a cross-repository source binding covers.
func SplitSiblingDeclarations(declarations []string) (local, siblings []string) {
	for _, decl := range declarations {
		if _, _, ok := SiblingDeclaration(decl); ok {
			siblings = append(siblings, decl)
		} else {
			local = append(local, decl)
		}
	}
	return local, siblings
}

// declaresSibling reports whether a sibling declaration names repo's repository.
func declaresSibling(siblings []string, repo string) bool {
	_, name := splitRepo(repo)
	for _, decl := range siblings {
		if sibling, _, ok := SiblingDeclaration(decl); ok && name != "" && strings.EqualFold(sibling, name) {
			return true
		}
	}
	return false
}

// CheckSiblingCoverage requires every sibling declaration to name exactly one
// repo-source:<owner>/<sibling> input. That key pins the whole deliverable
// commit, so any change to it wakes the receipt; a sibling the run did not bind
// is refused rather than resolved anywhere else.
//
// A single-repository run binds no repository at all (#5097). There, and only
// there, a sibling is covered by a declared:<decl> = unbound input instead: the
// receipt records the declaration as out of this home, and since no reader
// resolves that key, the planner reads the receipt as could-not-check, never
// unchanged. A marker beside any repo-source: key, a marker naming no declared
// sibling, or any other marker value is refused.
func CheckSiblingCoverage(siblings []string, inputs map[string]string) error {
	crossRepo := false
	for k := range inputs {
		crossRepo = crossRepo || strings.HasPrefix(k, InputKeyRepoSourcePrefix)
	}
	declared := map[string]bool{}
	for _, decl := range siblings {
		declared[decl] = true
	}
	for k, v := range inputs {
		decl, isMarker := strings.CutPrefix(k, InputKeyDeclaredPrefix)
		if isMarker && (crossRepo || v != DeclaredUnbound || !declared[decl]) {
			return Refused("refused: input " + fmt.Sprintf("%q", k) + " is not an unbound sibling declaration of a single-repository run")
		}
	}
	for _, decl := range siblings {
		// A malformed declaration has no name, so nothing binds it.
		name, _, _ := SiblingDeclaration(decl)
		bound := 0
		for k, v := range inputs {
			repo, isSource := strings.CutPrefix(k, InputKeyRepoSourcePrefix)
			if _, repoName := splitRepo(repo); isSource && name != "" && strings.TrimSpace(v) != "" && strings.EqualFold(repoName, name) {
				bound++
			}
		}
		unbound := name != "" && !crossRepo && inputs[InputKeyDeclaredPrefix+decl] == DeclaredUnbound
		if bound != 1 && !unbound {
			return Refused("refused: Context files: path " + fmt.Sprintf("%q", decl) + " names a sibling repository this run's cross-repository source does not bind")
		}
	}
	return nil
}

// DeclaredScope has one membership digest per canonical declaration, and every
// existing file it selects. Dirs maps every path a declaration observed as a
// directory (a literal directory, or one a glob matched) to its sorted members.
// No bytes are taken from a different tree.
type DeclaredScope struct {
	Files  []string
	Shapes map[string]string
	Dirs   map[string][]string
}

// Uncovered returns the selected files that inputs does not cover with a digest:
// neither a file:<path> key nor a file:<dir>/ key for a declared directory holding
// it. Presence only; the values are bound elsewhere (re-derivation, the wake read).
func (s DeclaredScope) Uncovered(inputs map[string]string) []string {
	covered := map[string]bool{}
	for dir, members := range s.Dirs {
		if _, ok := inputs[DirInputKey(dir)]; ok {
			for _, f := range members {
				covered[f] = true
			}
		}
	}
	var out []string
	for _, f := range s.Files {
		if _, ok := inputs[inputKeyFilePrefix+f]; !ok && !covered[f] {
			out = append(out, f)
		}
	}
	return out
}

// DirInputKey is the content input key of a declared directory: file:<dir>/. Its value
// is DirInputDigest over every member, so one key covers what one file:<member> key per
// member covered before #4865. The trailing slash keeps the digest where the pattern
// sweep's generic-api-key rule never reads it as a secret: that rule's identifier cannot
// span a slash, and nothing follows the last one.
func DirInputKey(dir string) string { return inputKeyFilePrefix + dir + "/" }

// ParseDirInputKey returns the directory a file:<dir>/ key names. It strips exactly one
// slash and never normalises: a file key without one, an empty or non-canonical
// directory, or a double slash is ok=false.
func ParseDirInputKey(key string) (string, bool) {
	rest, found := strings.CutPrefix(key, inputKeyFilePrefix)
	if !found {
		return "", false
	}
	dir, slash := strings.CutSuffix(rest, "/")
	if !slash || !canonicalRepoPath(dir) {
		return "", false
	}
	return dir, true
}

// DirInputDigest binds a directory's membership AND each member's content: sha256 over
// the sorted members' "<path>\x00<sha256 of its bytes>" terms, NUL-joined. Adding,
// removing, renaming or editing any member changes it. digest returns a member's content
// digest (a symlink's link text, as git stores it).
func DirInputDigest(members []string, digest func(string) (string, error)) (string, error) {
	sorted := append([]string(nil), members...)
	sort.Strings(sorted)
	terms := make([]string, 0, len(sorted))
	for _, f := range sorted {
		d, err := digest(f)
		if err != nil {
			return "", err
		}
		terms = append(terms, f+"\x00"+d)
	}
	return Sha256Hex([]byte(strings.Join(terms, "\x00"))), nil
}

func ResolveDeclaredScope(declarations []string, tree OutcomeTree) (DeclaredScope, error) {
	result := DeclaredScope{Shapes: map[string]string{}, Dirs: map[string][]string{}}
	files := map[string]bool{}
	work := 0
	consume := func(n int) error {
		if n > maxScopeWork-work {
			return Refused("refused: declared scope exceeds resolution work limit")
		}
		work += n
		return nil
	}
	for _, decl := range declarations {
		if err := consume(1); err != nil {
			return result, err
		}
		canonical, err := expandDeclaration(decl)
		if err != nil {
			return result, err
		}
		if len(canonical) != 1 || canonical[0] != decl {
			return result, Refused("refused: noncanonical input scope declaration")
		}
		if _, _, ok := SiblingDeclaration(decl); ok {
			return result, Refused("refused: Context files: path " + fmt.Sprintf("%q", decl) + " names a sibling repository; it never resolves against this tree")
		}
		paths := []string{decl}
		literalKind := ""
		if scopeDeclIsGlob(decl) {
			// List only below the literal prefix; match files AND directories,
			// then expand matched directories recursively, as literal directories do.
			i := strings.IndexAny(decl, "*?[")
			prefix := path.Dir(decl[:i] + "x")
			all, err := tree.ListFiles(prefix)
			if err != nil {
				return result, Unverifiable("cannot list declared glob "+decl, err)
			}
			if len(all) > maxScopePaths {
				return result, Refused("refused: declared glob exceeds file limit")
			}
			if err := consume(len(all)); err != nil {
				return result, err
			}
			// <dir>/** (the only ** expandDeclaration admits) matches each entry
			// directly below <dir>; a matched directory then expands recursively
			// below, so it selects every file under <dir> at any depth.
			pattern := decl
			if dir, recursive := strings.CutSuffix(decl, "/**"); recursive {
				pattern = dir + "/*"
			}
			candidates := map[string]bool{}
			for _, f := range all {
				for p := f; p != "." && p != "/"; p = path.Dir(p) {
					if err := consume(1); err != nil {
						return result, err
					}
					candidates[p] = true
				}
			}
			paths = nil
			for p := range candidates {
				if err := consume(1); err != nil {
					return result, err
				}
				if matched, _ := path.Match(pattern, p); matched {
					paths = append(paths, p)
				}
			}
			sort.Strings(paths)
		}
		var shape []string
		for _, p := range paths {
			if err := consume(1); err != nil {
				return result, err
			}
			kind, err := tree.PathKind(p)
			if err != nil {
				return result, Unverifiable("cannot resolve declared input "+p, err)
			}
			shape = append(shape, kind+":"+p)
			literalKind = kind
			switch kind {
			case "":
			case "blob":
				files[p] = true
			case "tree":
				members, err := tree.ListFiles(p)
				if err != nil {
					return result, Unverifiable("cannot list declared directory "+p, err)
				}
				if len(members) > maxScopePaths {
					return result, Refused("refused: declared directory exceeds file limit")
				}
				if err := consume(len(members)); err != nil {
					return result, err
				}
				sort.Strings(members)
				for _, f := range members {
					if !canonicalRepoPath(f) || !strings.HasPrefix(f, p+"/") {
						return result, Unverifiable("invalid declared directory member", nil)
					}
					files[f] = true
					shape = append(shape, "blob:"+f)
				}
				result.Dirs[p] = members
			default:
				return result, Refused("refused: unsupported declared input kind " + kind + " for " + p)
			}
			if len(files) > maxScopePaths {
				return result, Refused("refused: declared input scope exceeds file limit")
			}
		}
		sort.Strings(shape)
		result.Shapes[ScopeInputKey(decl, literalKind)] = Sha256Hex([]byte(strings.Join(shape, "\x00")))
	}
	for f := range files {
		result.Files = append(result.Files, f)
	}
	sort.Strings(result.Files)
	return result, nil
}

// rootScopeTree never follows ancestor symlinks. A final symlink remains a blob
// (its link text), matching Git's treatment; directory enumeration never follows it.
type rootScopeTree struct{ root string }

// CheckReceiptScope prevents an immutable historical partial manifest from
// establishing unchangedness. This reads today's declarations only to identify
// missing coverage; it never migrates the receipt's original observed hashes.
func (rr *RootRevisionReader) CheckReceiptScope(receipt WakeReceipt) error {
	stream, num, err := SplitBriefKey(receipt.Brief)
	if err != nil {
		return fmt.Errorf("cannot identify the brief for scope completeness")
	}
	prefix := "file:docs/streams/" + stream + "/brief-" + num + "-"
	brief := ""
	for k := range receipt.Inputs {
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, ".md") {
			if brief != "" {
				return fmt.Errorf("ambiguous brief input scope")
			}
			brief = strings.TrimPrefix(k, "file:")
		}
	}
	if brief == "" {
		return fmt.Errorf("missing brief input for scope completeness")
	}
	full, err := (rootScopeTree{root: rr.Root}).safePath(brief)
	if err != nil {
		return err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("brief scope is not a plain file")
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	declarations, err := BriefDeclaredFiles(string(b))
	if err != nil {
		return err
	}
	// A slash-form key's value sits where the pattern sweep's generic rule no longer
	// looks, so refuse to read any scope: value that is not the producer's digest.
	for k, v := range receipt.Inputs {
		if strings.HasPrefix(k, InputKeyScopePrefix) && !scopeDigestFormat.MatchString(v) {
			return fmt.Errorf("%s value is not a scope digest", k)
		}
		// The same holds for a file:<dir>/ key (#4865): its value is a digest or nothing.
		if strings.HasPrefix(k, inputKeyFilePrefix) && strings.HasSuffix(k, "/") && !scopeDigestFormat.MatchString(v) {
			return fmt.Errorf("%s value is not a directory digest", k)
		}
	}
	// Only local declarations carry scope: keys. A ../<sibling>/ declaration is
	// covered by its repo-source: commit below; ranging over every declaration
	// here would report each sibling as missing scope and disable sibling briefs.
	local, siblings := SplitSiblingDeclarations(declarations)
	for _, decl := range local {
		plain := receipt.Inputs[InputKeyScopePrefix+decl] != ""
		dir := !scopeDeclIsGlob(decl) && receipt.Inputs[InputKeyScopePrefix+decl+"/"] != ""
		if plain && dir {
			return fmt.Errorf("ambiguous scope key for %s: both scope:%s and scope:%s/ are present", decl, decl, decl)
		}
		if !plain && !dir {
			return fmt.Errorf("missing declared scope %s; fresh admission and observation required", decl)
		}
	}
	return CheckSiblingCoverage(siblings, receipt.Inputs)
}

func (r rootScopeTree) safePath(rel string) (string, error) {
	if rel == "." {
		return r.root, nil
	}
	if !canonicalRepoPath(rel) {
		return "", fmt.Errorf("invalid scope path")
	}
	parts := strings.Split(rel, "/")
	full := r.root
	for i, part := range parts {
		full = filepath.Join(full, part)
		if i == len(parts)-1 {
			break
		}
		info, err := os.Lstat(full)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("input ancestor is a symlink")
		}
	}
	return full, nil
}

func (r rootScopeTree) PathKind(rel string) (string, error) {
	full, err := r.safePath(rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "tree", nil
	}
	if info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "blob", nil
	}
	return "", fmt.Errorf("unsupported input kind")
}

func (r rootScopeTree) ReadFile(rel string) ([]byte, error) {
	return nil, fmt.Errorf("scope reader does not read file content")
}

func (r rootScopeTree) ListFiles(rel string) ([]string, error) {
	full, err := r.safePath(rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scope prefix is not a directory")
	}
	var out []string
	err = filepath.WalkDir(full, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name, err := filepath.Rel(r.root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(name))
		if len(out) > maxScopePaths {
			return fmt.Errorf("scope enumeration exceeds file limit")
		}
		return nil
	})
	return out, err
}
