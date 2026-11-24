package deskkit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// NpmStaging is a verifier run's admitted npm dependency set (#7572), the npm
// counterpart of the container bracket's Go module cache. The staging step
// (StageVerifierNpm) runs `npm ci --offline` from a cache filled before the
// run, against the attested package-lock.json of one package directory in the
// home, and records the digest of what it wrote. Admission then passes over the
// staged directories (and only those) instead of refusing them as unattested
// files, and refuses unless their bytes still hash to this digest.
//
// It is local-only like World and never part of the issue body. It is bound
// once: a run that has a staging record cannot be staged again.
type NpmStaging struct {
	// Package is the home-relative directory holding package.json and
	// package-lock.json, slash-separated and clean.
	Package string
	// Lock is the git object id of the attested package-lock.json blob, as
	// the attested source tree lists it.
	Lock string
	// Built records that staging also ran the package's own build script, so
	// the package's dist directory is staged and digested with node_modules.
	Built bool `json:",omitempty"`
	// Digest is sha256:<64 hex> over every staged file's path, mode and content.
	Digest string
}

// npmStagingDigestRe is the shape of NpmStaging.Digest, matching the
// container bracket's Module-Cache digest.
var npmStagingDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// stagedDirs are the home-relative directories staging may write under the
// package directory: node_modules always, dist only when the build ran.
func (s NpmStaging) stagedDirs() []string {
	dirs := []string{path.Join(s.Package, "node_modules")}
	if s.Built {
		dirs = append(dirs, path.Join(s.Package, "dist"))
	}
	return dirs
}

// npmStageCommand runs one npm command in dir with exactly env. Tests replace
// it; the shipped step runs the npm on PATH.
var npmStageCommand = func(dir string, env []string, args ...string) error {
	cmd := exec.Command("npm", args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return RunProcess(cmd)
}

// npmStagePassNames are the inherited variables the staging step's npm keeps,
// matched by exact name whatever its case, besides the locale's LC_* settings:
// the shape of an admitted Verify row's allow-list scrub (statusgen's
// admittedRowPassNames) without its Go toolchain settings. Each is a search
// path, an account or shell name, the bare CI flag, a locale or terminal
// setting, a proxy or trust-root setting, or a Windows system location. None
// names a credential; every family not listed here (npm_config_*, NODE_OPTIONS,
// forge and cloud tokens, credential pointers) is dropped.
var npmStagePassNames = []string{
	"PATH", "USER", "LOGNAME", "SHELL",
	"LANG", "LANGUAGE", "TERM", "COLORTERM", "NO_COLOR", "TZ", "COLUMNS", "LINES", "CI",
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "SSL_CERT_FILE", "SSL_CERT_DIR",
	"SYSTEMROOT", "SYSTEMDRIVE", "WINDIR", "COMSPEC", "PATHEXT", "OS",
	"PROCESSOR_ARCHITECTURE", "NUMBER_OF_PROCESSORS", "PROGRAMFILES", "PROGRAMFILES(X86)",
	"PROGRAMW6432", "COMMONPROGRAMFILES", "COMMONPROGRAMFILES(X86)", "COMMONPROGRAMW6432", "PROGRAMDATA",
}

// npmStageURLNames are the listed variables whose value is a URL a client
// authenticates with when it carries a user name; one carrying any '@' is
// dropped, so a proxy login never rides through on a listed name.
var npmStageURLNames = []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"}

// npmStageEnv is the whole environment of the staging step's npm commands:
// the allow-listed inherited variables, then a fresh, empty HOME and TMPDIR
// (and their Windows names) that the caller made for this step, and a global
// npm config path that does not exist, so npm reads no user or global npmrc
// and no stored login. Like an admitted row's scrub it removes the pointers,
// not the files: the step runs as the same user.
func npmStageEnv(environ []string, home, tmp string) []string {
	var env []string
	for _, kv := range environ {
		key, value, _ := strings.Cut(kv, "=")
		key = strings.ToUpper(key)
		if slices.Contains(npmStageURLNames, key) && strings.Contains(value, "@") {
			continue
		}
		if strings.HasPrefix(key, "LC_") || slices.Contains(npmStagePassNames, key) {
			env = append(env, kv)
		}
	}
	return append(env,
		"HOME="+home, "USERPROFILE="+home,
		"TMPDIR="+tmp, "TEMP="+tmp, "TMP="+tmp,
		"npm_config_userconfig="+filepath.Join(home, ".npmrc"),
		"npm_config_globalconfig="+filepath.Join(home, "global-npmrc"),
		"npm_config_update_notifier=false")
}

// npmStageDirs makes the staging step's own HOME and TMPDIR: fresh, empty 0700
// directories under a new temporary directory, which the caller removes before
// the home is closed again.
func npmStageDirs() (string, string, string, error) {
	dir, err := os.MkdirTemp("", "npm-stage-")
	if err != nil {
		return "", "", "", err
	}
	home, tmp := filepath.Join(dir, "home"), filepath.Join(dir, "tmp")
	for _, d := range []string{home, tmp} {
		if err := os.Mkdir(d, 0o700); err != nil {
			_ = os.RemoveAll(dir) // best effort; the mkdir error is the one reported
			return "", "", "", err
		}
	}
	return dir, home, tmp, nil
}

// npmPackageProblem validates a --package value and the attested tree under it:
// a clean, home-relative directory whose package.json and package-lock.json are
// tracked files, and whose staged directories are neither tracked paths nor
// prefixes of one. Skipping a staged directory therefore never hides a tracked
// file from the closure.
func npmPackageProblem(pkg string, tree map[string]string, built bool) string {
	if pkg == "" || !filepath.IsLocal(pkg) || path.Clean(filepath.ToSlash(pkg)) != pkg || strings.Contains(pkg, "\\") {
		return fmt.Sprintf("npm staging package %q is not a clean home-relative directory", pkg)
	}
	for _, f := range []string{"package.json", "package-lock.json"} {
		entry, ok := tree[path.Join(pkg, f)]
		if !ok || !strings.HasPrefix(entry, "100") {
			return fmt.Sprintf("npm staging package %s has no attested %s", pkg, f)
		}
	}
	for _, dir := range (NpmStaging{Package: pkg, Built: built}).stagedDirs() {
		for name := range tree {
			if name == dir || strings.HasPrefix(name, dir+"/") {
				return fmt.Sprintf("npm staging directory %s holds attested source %s; only an untracked directory can be staged", dir, name)
			}
		}
	}
	return ""
}

// npmLockObject is the object id of pkg's attested package-lock.json in the
// source tree, or "" when the tree has no such blob. npmPackageProblem has
// already required the entry to be a tracked file.
func npmLockObject(tree map[string]string, pkg string) string {
	entry, ok := tree[path.Join(pkg, "package-lock.json")]
	if !ok || !strings.HasPrefix(entry, "100") {
		return ""
	}
	_, id, _ := strings.Cut(entry, " ")
	return id
}

// npmCacheProblem mirrors moduleCachePathProblem's placement rule: the cache is
// an existing absolute directory outside the home.
func npmCacheProblem(home, cache string) string {
	if !filepath.IsAbs(cache) {
		return fmt.Sprintf("npm cache %s is not an absolute path", cache)
	}
	info, err := os.Stat(cache)
	if err != nil || !info.IsDir() {
		return fmt.Sprintf("npm cache %s is not an existing directory filled before the run", cache)
	}
	resolved, err := filepath.EvalSymlinks(cache)
	if err != nil {
		return fmt.Sprintf("npm cache %s: %v", cache, err)
	}
	if rel, err := filepath.Rel(home, resolved); err == nil && filepath.IsLocal(rel) || resolved == home {
		return fmt.Sprintf("npm cache %s sits inside the home; the cache must be outside it", cache)
	}
	return ""
}

// npmStagingDigest is the digest of the staged directories of one record:
// sha256 over sorted "path NUL mode NUL sha256(content) NUL" entries, with paths
// relative to home (the shape of the container bracket's homeTreeDigest). Each
// staged directory must be a real directory. A symlink is recorded by its
// target, which must stay inside its staged directory both as written and once
// its whole chain is resolved, and must resolve to an existing path; any other
// file type refuses.
func npmStagingDigest(home string, s NpmStaging) (string, error) {
	type entry struct{ path, mode, sum string }
	var entries []entry
	for _, dir := range s.stagedDirs() {
		root := filepath.Join(home, filepath.FromSlash(dir))
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() {
			return "", Refused("npm staging directory " + dir + " is missing or not a directory")
		}
		rootReal, err := filepath.EvalSymlinks(root)
		if err != nil {
			return "", Unverifiable("cannot resolve npm staging directory "+dir, err)
		}
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(home, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			info, err := d.Info()
			if err != nil {
				return err
			}
			var content []byte
			mode := "100644"
			switch {
			case info.Mode()&fs.ModeSymlink != 0:
				target, err := os.Readlink(p)
				if err != nil {
					return err
				}
				if filepath.IsAbs(target) {
					return Refused("npm staging symlink " + rel + " has an absolute target")
				}
				in, err := filepath.Rel(root, filepath.Join(filepath.Dir(p), target))
				if err != nil || !filepath.IsLocal(in) {
					return Refused("npm staging symlink " + rel + " leaves its staged directory " + dir)
				}
				// The lexical check above does not see through a chain: a link
				// whose target names another link (a/b/l1 -> ../.., then
				// l2 -> a/b/l1/../../x) can stay inside lexically and still
				// resolve outside. Resolve the whole chain and require the
				// result to exist inside the staged directory.
				resolved, err := filepath.EvalSymlinks(p)
				if err != nil {
					return Refused("npm staging symlink " + rel + " does not resolve to an existing path")
				}
				if in, err := filepath.Rel(rootReal, resolved); err != nil || !filepath.IsLocal(in) {
					return Refused("npm staging symlink " + rel + " resolves outside its staged directory " + dir)
				}
				content, mode = []byte(target), "120000"
			case info.Mode().IsRegular():
				if content, err = os.ReadFile(p); err != nil {
					return err
				}
				if info.Mode()&0o111 != 0 {
					mode = "100755"
				}
			default:
				return Refused("npm staging path " + rel + " is neither a regular file nor a symlink")
			}
			sum := sha256.Sum256(content)
			entries = append(entries, entry{rel, mode, hex.EncodeToString(sum[:])})
			return nil
		})
		if err != nil {
			if _, ok := err.(*DeskError); ok {
				return "", err
			}
			return "", Unverifiable("cannot digest npm staging directory "+dir, err)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	h := sha256.New()
	for _, e := range entries {
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00", e.path, e.mode, e.sum) // hash.Hash.Write never returns an error
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// verifierNpmCheck is admission's half of the staging record: the record is
// well formed, names the package's attested lock, and the staged directories'
// bytes still hash to the recorded digest. The closure has already established
// that the package and the staged directories are where the record says.
func verifierNpmCheck(home, source string, s NpmStaging) error {
	tree, err := verifierTree(home, source)
	if err != nil {
		return err
	}
	if !npmStagingDigestRe.MatchString(s.Digest) || s.Lock == "" {
		return Refused("invalid npm staging record")
	}
	if p := npmPackageProblem(s.Package, tree, s.Built); p != "" {
		return Refused(p)
	}
	if npmLockObject(tree, s.Package) != s.Lock {
		return Refused("npm staging record names a different package-lock.json than the attested source")
	}
	got, err := npmStagingDigest(home, s)
	if err != nil {
		return err
	}
	if got != s.Digest {
		return Refused(fmt.Sprintf("npm staging changed since it was staged (staged %s, now %s); no Verify rows or Evidence admitted", s.Digest, got))
	}
	return nil
}

// npmStageRun runs the staging step's npm commands in dir under npmStageEnv,
// with a fresh HOME and TMPDIR that are removed before it returns.
func npmStageRun(dir, cache string, build bool) error {
	scratch, home, tmp, err := npmStageDirs()
	if err != nil {
		return Unverifiable("cannot make the npm staging step's own HOME and TMPDIR", err)
	}
	defer func() { _ = os.RemoveAll(scratch) }() // best effort; the run's own result is what is reported
	env := npmStageEnv(os.Environ(), home, tmp)
	if err := npmStageCommand(dir, env, "ci", "--offline", "--ignore-scripts", "--no-audit", "--no-fund", "--cache", cache); err != nil {
		return Unverifiable("npm ci --offline could not stage the package's dependencies from the cache", err)
	}
	if build {
		if err := npmStageCommand(dir, env, "run", "build", "--offline", "--ignore-scripts"); err != nil {
			return Unverifiable("the package's build script failed during npm staging", err)
		}
	}
	return nil
}

// StageVerifierNpm is the admitted npm staging step (#7572). The home must pass
// local pre-work admission first, so nothing is staged into a home that already
// holds an unattested file. It runs, in the package directory,
//
//	npm ci --offline --ignore-scripts --no-audit --no-fund --cache <cache>
//
// and, with build, `npm run build --offline --ignore-scripts` (the package's
// own attested build script; no dependency's install script runs), both under
// npmStageEnv: an allow-listed environment with a fresh, empty HOME and TMPDIR,
// as an admitted Verify row gets. The home is
// then closed again with only the staged directories passed over, so the step
// wrote nothing else, and the digest of the staged directories is bound to the
// run record once. Only a version-1 (same-repository) run can be staged.
func StageVerifierNpm(root, brief, pkg, cache string, build bool) (VerifierReceipt, error) {
	r, err := verifierLoad(root)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if r.Binding.Version != 1 {
		return r.VerifierReceipt, Refused("npm staging is admitted for a same-repository verifier run only")
	}
	if r.NpmStaging != nil {
		return r.VerifierReceipt, Refused("verifier run already has an npm staging record; re-admit a new run")
	}
	if brief == "" {
		brief = r.Binding.Brief
	}
	if err := verifierLocalCheck(root, brief, r, verifierExecution); err != nil {
		return r.VerifierReceipt, err
	}
	_, home, err := verifierHome(root)
	if err != nil {
		return r.VerifierReceipt, err
	}
	pkg = filepath.ToSlash(pkg)
	tree, err := verifierTree(home, r.Binding.Source)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if p := npmPackageProblem(pkg, tree, build); p != "" {
		return r.VerifierReceipt, Refused(p)
	}
	if p := npmCacheProblem(home, cache); p != "" {
		return r.VerifierReceipt, Refused(p)
	}
	dir := filepath.Join(home, filepath.FromSlash(pkg))
	if err := npmStageRun(dir, cache, build); err != nil {
		return r.VerifierReceipt, err
	}
	s := NpmStaging{Package: pkg, Lock: npmLockObject(tree, pkg), Built: build}
	if s.Digest, err = npmStagingDigest(home, s); err != nil {
		return r.VerifierReceipt, err
	}
	// Close the home again exactly as admission will: the step may have
	// written the staged directories and nothing else.
	checkout, err := parseVerifierCheckout(r.Binding.Checkout)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if err := verifierSourceClosureStaged(home, r.Binding.Source, checkout, map[string]bool{r.Binding.Brief: true}, s.stagedDirs()); err != nil {
		return r.VerifierReceipt, err
	}
	r.NpmStaging = &s
	if err := verifierSave(root, r); err != nil {
		return r.VerifierReceipt, err
	}
	return r.VerifierReceipt, nil
}
