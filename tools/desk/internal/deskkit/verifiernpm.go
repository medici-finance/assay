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
	// Lock is the sha256 of the attested package-lock.json blob.
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

// npmStageCommand runs one npm command in dir. Tests replace it; the shipped
// step runs the npm on PATH with the admission git environment.
var npmStageCommand = func(dir string, args ...string) error {
	cmd := exec.Command("npm", args...)
	cmd.Dir = dir
	cmd.Env = verifierEnv()
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return RunProcess(cmd)
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
// target, which must stay inside its staged directory; any other file type
// refuses.
func npmStagingDigest(home string, s NpmStaging) (string, error) {
	type entry struct{ path, mode, sum string }
	var entries []entry
	for _, dir := range s.stagedDirs() {
		root := filepath.Join(home, filepath.FromSlash(dir))
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() {
			return "", Refused("npm staging directory " + dir + " is missing or not a directory")
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
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", e.path, e.mode, e.sum)
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
	if !npmStagingDigestRe.MatchString(s.Digest) || len(s.Lock) != 64 {
		return Refused("invalid npm staging record")
	}
	if p := npmPackageProblem(s.Package, tree, s.Built); p != "" {
		return Refused(p)
	}
	lock, err := verifierGitBytes(home, "cat-file", "blob", source+":"+path.Join(s.Package, "package-lock.json"))
	if err != nil {
		return Unverifiable("cannot read attested package-lock.json", err)
	}
	if verifierDigest(lock) != s.Lock {
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

// StageVerifierNpm is the admitted npm staging step (#7572). The home must pass
// local pre-work admission first, so nothing is staged into a home that already
// holds an unattested file. It runs, in the package directory,
//
//	npm ci --offline --ignore-scripts --no-audit --no-fund --cache <cache>
//
// and, with build, `npm run build --offline --ignore-scripts` (the package's
// own attested build script; no dependency's install script runs). The home is
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
	lock, err := verifierGitBytes(home, "cat-file", "blob", r.Binding.Source+":"+path.Join(pkg, "package-lock.json"))
	if err != nil {
		return r.VerifierReceipt, Unverifiable("cannot read attested package-lock.json", err)
	}
	dir := filepath.Join(home, filepath.FromSlash(pkg))
	if err := npmStageCommand(dir, "ci", "--offline", "--ignore-scripts", "--no-audit", "--no-fund", "--cache", cache); err != nil {
		return r.VerifierReceipt, Unverifiable("npm ci --offline could not stage the package's dependencies from the cache", err)
	}
	if build {
		if err := npmStageCommand(dir, "run", "build", "--offline", "--ignore-scripts"); err != nil {
			return r.VerifierReceipt, Unverifiable("the package's build script failed during npm staging", err)
		}
	}
	s := NpmStaging{Package: pkg, Lock: verifierDigest(lock), Built: build}
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
