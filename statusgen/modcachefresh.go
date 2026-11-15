package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// The open step's module cache (#4683). The verify-runner image bakes a Go
// module cache when it is built, so its rows can resolve only what the release
// that built the image required; a tree verified later may require more, and
// with no network its Go rows stop on module resolution (modcachegap.go names
// that cause). This file derives the cache from the tree under verification
// instead:
//
//  1. host open: with --module-cache <dir>, `verifyrun --open-bracket` screens
//     every module path the home's roots could request (moduleEgressProblem),
//     names the proxy, checksum database and roots on stdout, then runs
//     `go mod download` for every root the home's own moduleRootsManifest
//     lists, into that new directory, bound to each root's go.sum and to the
//     checksum database, and records the cache's digest and roots in the
//     receipt (receiptModuleCache). Any screen or download failure refuses the
//     open step with no receipt: there is no fall-back to the image's baked
//     cache;
//  2. container: the run reads the receipt, which must carry the cache
//     exactly when the home lists module roots (moduleCacheRequirementProblem,
//     also run at close), and, before any row, refuses
//     unless GOMODCACHE is a read-only mount that digests to the receipt's
//     cache, the root file system is read-only, and the module environment
//     keeps GOPROXY=off, GOTOOLCHAIN=local and checksum verification on
//     (containerModuleCacheProblem). Its witness records the digest as a
//     Module-Cache line;
//  3. host close: the witness's Module-Cache line must equal the receipt's
//     digest, and is present exactly when the receipt carries one.
//
// The host's own module cache is never used: the open step refuses a
// --module-cache directory that is, holds or lies inside it.

// moduleRootsManifest is the home-relative list of the Go module roots the
// verify-runner image provisions, one per line. The image's Dockerfile copies
// this file to bakedModuleRootsFile and downloads exactly the roots it lists,
// so the image and the open step read one list from the source.
const moduleRootsManifest = "images/verify-runner/go-module-roots.txt"

// containerModuleCacheTarget is the runner image's GOMODCACHE, where the open
// step's cache is mounted.
const containerModuleCacheTarget = "/go/pkg/mod"

const moduleCachePrefix = "Module-Cache: "

// receiptModuleCache is the open step's module cache, as the receipt records
// it: the cache's tree digest (homeTreeDigest) and the roots it was downloaded
// for.
type receiptModuleCache struct {
	Digest      string              `json:"digest"`
	Roots       []string            `json:"roots"`
	PublicAssay *receiptPublicAssay `json:"public_assay,omitempty"`
}

// goModDownload runs `go mod download` in dir under exactly env. A var so tests
// fake it: no test reaches a module proxy.
var goModDownload = func(dir string, env []string) ([]byte, error) {
	cmd := exec.Command("go", "mod", "download")
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}

// ambientModuleCaches are the host's own module caches the open step must
// never write to or hand to a container: GOMODCACHE, each GOPATH entry's
// pkg/mod, $HOME/go/pkg/mod, and what `go env GOMODCACHE` reports (it reads
// the user's go env file). A var so tests can add fixtures.
var ambientModuleCaches = func() []string {
	var out []string
	if d := os.Getenv("GOMODCACHE"); d != "" {
		out = append(out, d)
	}
	for _, gp := range filepath.SplitList(os.Getenv("GOPATH")) {
		if gp != "" {
			out = append(out, filepath.Join(gp, "pkg", "mod"))
		}
	}
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		out = append(out, filepath.Join(h, "go", "pkg", "mod"))
	}
	if raw, err := exec.Command("go", "env", "GOMODCACHE").Output(); err == nil {
		if d := strings.TrimSpace(string(raw)); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// The open step's download endpoints: the public module proxy and checksum
// database, the same pins as the cluster backend's stage-cache.sh. The
// download environment pins them and the announcement names them.
const (
	moduleDownloadProxy = "https://proxy.golang.org"
	moduleDownloadSumDB = "sum.golang.org"
)

// moduleDownloadEnv is the environment of the open step's downloads, built
// from nothing (never from the caller's environment): PATH and the platform's
// temp and system-root settings, a scratch HOME and GOCACHE (so no .netrc, git
// or go configuration of the host's is read), then the pinned Go settings (the
// same set the cluster backend's stage-cache.sh pins). Checksum verification
// stays on: GOSUMDB is the public database, and no setting that loosens it can
// arrive from the host, nor from its go env file (GOENV=off).
func moduleDownloadEnv(cache, scratch string) []string {
	var env []string
	for _, k := range []string{"PATH", "TMPDIR", "SystemRoot"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return append(env,
		"HOME="+scratch,
		"GOCACHE="+filepath.Join(scratch, "gocache"),
		"GOENV=off",
		"GOMODCACHE="+cache,
		"GOWORK=off",
		"GOFLAGS=-mod=readonly",
		"GOTOOLCHAIN=local",
		"GOPROXY="+moduleDownloadProxy,
		"GOSUMDB="+moduleDownloadSumDB,
		"GOVCS=*:off",
	)
}

// approvedPublicModulePrefixes is the approved public set of stage-cache.sh's
// APPROVED pattern, the cluster backend's screen of the same download:
// public vanity hosts whose module paths may be sent to the public proxy.
// TestModEgressParity holds the two lists equal.
var approvedPublicModulePrefixes = []string{
	"golang.org/x/", "google.golang.org/", "gopkg.in/", "go.uber.org/",
	"go.opentelemetry.io/", "go.etcd.io/", "go.yaml.in/", "honnef.co/",
	"gotest.tools/", "sigs.k8s.io/", "k8s.io/", "cloud.google.com/",
	"modernc.org/", "rsc.io/", "lukechampine.com/", "filippo.io/",
	"dario.cat/", "gitlab.com/gitlab-org/api/",
}

// githubPublicModuleRe is stage-cache.sh's GHPUB: a github.com path is public
// unless it sits under an owner the screen refuses.
var githubPublicModuleRe = regexp.MustCompile(`^github\.com/[A-Za-z0-9_.-]+/`)

// goModEditJSON parses one go.mod with Go's own parser (`go mod edit -json`),
// which runs no module code and makes no request. A var so tests can fail it.
var goModEditJSON = func(gomod string, env []string) ([]byte, error) {
	cmd := exec.Command("go", "mod", "edit", "-json", gomod)
	cmd.Env = env
	return cmd.Output()
}

// operatorPrivateModulePatterns are the host's GOPRIVATE, GONOPROXY and
// GONOSUMDB patterns, from the environment and from the user's go env file.
// The download environment drops them, so the screen honours them instead: a
// path the operator marked private is never sent. A var so tests fix it.
var operatorPrivateModulePatterns = func() []string {
	keys := []string{"GOPRIVATE", "GONOPROXY", "GONOSUMDB"}
	var out []string
	for _, k := range keys {
		out = append(out, splitModulePatterns(os.Getenv(k))...)
	}
	if raw, err := exec.Command("go", append([]string{"env"}, keys...)...).Output(); err == nil {
		for _, l := range strings.Split(string(raw), "\n") {
			out = append(out, splitModulePatterns(l)...)
		}
	}
	return out
}

func splitModulePatterns(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// matchModulePattern reports whether pattern, a GOPRIVATE-style glob, matches
// a prefix of path's elements, as the go command matches it.
func matchModulePattern(pattern, path string) bool {
	n := strings.Count(strings.TrimSuffix(pattern, "/"), "/") + 1
	elems := strings.Split(path, "/")
	if len(elems) < n {
		return false
	}
	ok, err := pathpkg.Match(strings.TrimSuffix(pattern, "/"), strings.Join(elems[:n], "/"))
	return err == nil && ok
}

// moduleEgressProblem is the screen the open step runs before any request,
// the host twin of stage-cache.sh's: every module path the roots could send
// to the public proxy (each root's requires that are not replaced by a
// directory, every replacement that names a module version, and every module
// in its go.sum) must be on the approved public set or a github.com path, and
// none may match the operator's own private patterns. Owner paths default to
// denial; only the exact public Assay repository can use the host-only proof
// below, with exact committed module/version/checksum binding. That proof
// applies to every Assay path whatever the owners are: an adopter outside the
// Assay owner also needs it, so its Assay requirements now cost one
// unauthenticated GET and refuse when that GET fails. No request is made until
// all paths pass the local screen. owners are lower-case forge owners.
func moduleEgressProblem(home string, roots, owners []string, env []string, stdout io.Writer) (*receiptPublicAssay, error) {
	private := operatorPrivateModulePatterns()
	why := map[string]string{}
	var bindings []publicAssayModule
	for _, r := range roots {
		dir := filepath.Join(home, filepath.FromSlash(r))
		paths, err := moduleRequestPaths(dir, env)
		if err != nil {
			return nil, fmt.Errorf("could-not-check — module root %s: %v; nothing was sent to the proxy", r, err)
		}
		for _, p := range paths {
			if isPublicAssayModule(p) && !operatorPrivateModule(p, private) {
				continue // only provisionally admitted: proof below precedes every download
			}
			if w := modulePathEgressProblem(p, owners, private); w != "" {
				why[p] = w
			}
		}
		b, err := publicAssayBindings(dir, r, env)
		if err != nil {
			return nil, fmt.Errorf("could-not-check — %v; nothing was sent to the proxy", err)
		}
		bindings = append(bindings, b...)
	}
	if len(why) == 0 {
		if len(bindings) == 0 {
			return nil, nil
		}
		return provePublicAssay(bindings, stdout)
	}
	var bad []string
	for p, w := range why {
		bad = append(bad, p+" ("+w+")")
	}
	sort.Strings(bad)
	return nil, fmt.Errorf("could-not-check — module paths outside the approved public set, nothing was sent to the proxy: %s", strings.Join(bad, ", "))
}

// modulePathEgressProblem is why one module path may not be sent, "" when it
// may.
func modulePathEgressProblem(p string, owners, private []string) string {
	lc := strings.ToLower(p)
	for _, o := range owners {
		if strings.HasPrefix(lc, "github.com/"+o+"/") {
			return "under the verified repository's owner " + o
		}
	}
	for _, pat := range private {
		if matchModulePattern(pat, p) {
			return "matches the operator's private pattern " + pat
		}
	}
	for _, a := range approvedPublicModulePrefixes {
		if strings.HasPrefix(p, a) {
			return ""
		}
	}
	if githubPublicModuleRe.MatchString(p) {
		return ""
	}
	return "not on the approved public set"
}

// moduleRequestPaths is the module paths `go mod download` in dir could ask
// the proxy for: go.mod's requires not replaced by a directory, every
// replacement naming a module version, and every module go.sum names.
func moduleRequestPaths(dir string, env []string) ([]string, error) {
	raw, err := goModEditJSON(filepath.Join(dir, "go.mod"), env)
	if err != nil {
		return nil, fmt.Errorf("go.mod does not parse (%v)", err)
	}
	var m struct {
		Require []struct{ Path, Version string }
		Replace []struct {
			Old, New struct{ Path, Version string }
		}
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("go.mod does not parse (%v)", err)
	}
	set := map[string]bool{}
	dirReplaced := func(path, version string) bool {
		for _, rp := range m.Replace {
			if rp.Old.Path == path && (rp.Old.Version == "" || rp.Old.Version == version) && rp.New.Version == "" {
				return true
			}
		}
		return false
	}
	for _, r := range m.Require {
		if !dirReplaced(r.Path, r.Version) {
			set[r.Path] = true
		}
	}
	for _, rp := range m.Replace {
		if rp.New.Version != "" {
			set[rp.New.Path] = true
		}
	}
	sum, err := os.ReadFile(filepath.Join(dir, "go.sum"))
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(string(sum), "\n") {
		if f := strings.Fields(l); len(f) > 0 {
			set[f[0]] = true
		}
	}
	var out []string
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// moduleCachePathProblem refuses a --module-cache directory the open step may
// not fill: inside the home (it would change the tree the bracket binds), the
// host's own module cache or anything inside or around it, or a directory that
// already holds anything.
func moduleCachePathProblem(home, cache string) string {
	if !filepath.IsAbs(cache) {
		return fmt.Sprintf("--module-cache %s is not an absolute path", cache)
	}
	// The path is written into the printed --mount value, whose options are
	// comma-separated: a comma, quote, space or control character could add one.
	if i := strings.IndexFunc(cache, func(r rune) bool { return r == ',' || r == '"' || unicode.IsSpace(r) || unicode.IsControl(r) }); i >= 0 {
		return fmt.Sprintf("--module-cache %q carries %q; the path is written into the container's --mount option list, so it may hold no comma, quote, whitespace or control character", cache, cache[i:i+1])
	}
	if rel, ok := relWithin(home, cache); ok {
		return fmt.Sprintf("--module-cache %s sits inside the home (%s); the cache must be outside it", cache, rel)
	}
	for _, a := range ambientModuleCaches() {
		if _, in := relWithin(a, cache); in {
			return fmt.Sprintf("--module-cache %s is the host's own module cache %s or inside it; the open step fills a new verifier-owned directory and never reads or mounts the host's cache", cache, a)
		}
		if _, holds := relWithin(cache, a); holds {
			return fmt.Sprintf("--module-cache %s holds the host's own module cache %s; the open step fills a new verifier-owned directory and never reads or mounts the host's cache", cache, a)
		}
	}
	entries, err := os.ReadDir(cache)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return fmt.Sprintf("--module-cache %s: %v", cache, err)
	case len(entries) > 0:
		return fmt.Sprintf("--module-cache %s already holds files; the open step downloads into a new, empty directory, never into a cache it did not fill", cache)
	}
	return ""
}

// prepareModuleCache is the open step's download: `go mod download` for every
// root the home's moduleRootsManifest lists, into cache, then the cache's
// digest. Before any request the module paths are screened
// (moduleEgressProblem) and the network target is announced on stdout. Each
// root's go.mod and go.sum must be byte-identical afterwards, so the set
// downloaded is the one go.sum names. Any failure is returned, and the caller
// writes no receipt.
func prepareModuleCache(home, cache string, owners []string, stdout io.Writer) (receiptModuleCache, error) {
	manifest := filepath.Join(home, filepath.FromSlash(moduleRootsManifest))
	roots, ok := bakedModuleRoots(manifest)
	if !ok {
		return receiptModuleCache{}, fmt.Errorf("could-not-check — the home lists no Go module roots in %s, so there is nothing to download for --module-cache", moduleRootsManifest)
	}
	if p := moduleCachePathProblem(home, cache); p != "" {
		return receiptModuleCache{}, fmt.Errorf("%s", p)
	}
	for _, r := range roots {
		if !filepath.IsLocal(r) {
			return receiptModuleCache{}, fmt.Errorf("%s lists %q, which is not a home-relative module root", moduleRootsManifest, r)
		}
	}
	if len(owners) == 0 {
		return receiptModuleCache{}, fmt.Errorf("could-not-check — the check names no repository owner to screen module paths against; nothing was sent to the proxy")
	}
	scratch, err := os.MkdirTemp("", "verifyrun-moddl-")
	if err != nil {
		return receiptModuleCache{}, err
	}
	defer os.RemoveAll(scratch)
	env := moduleDownloadEnv(cache, scratch)
	// Each root's go.mod and go.sum are read once, before the screen, and the
	// screen reads a private copy of exactly those bytes. The download below is
	// then bound to the screened bytes on both sides, so neither the screen nor
	// the proof's network round trip can open a window in which the files the
	// screen passed and the files the download reads differ.
	snaps, screened, err := snapshotModuleRoots(home, roots, filepath.Join(scratch, "screened"))
	if err != nil {
		return receiptModuleCache{}, err
	}
	proof, err := moduleEgressProblem(screened, roots, owners, env, stdout)
	if err != nil {
		return receiptModuleCache{}, err
	}
	fmt.Fprintf(stdout, "module-download proxy=%s sumdb=%s roots=%s cache=%s\n", moduleDownloadProxy, moduleDownloadSumDB, strings.Join(roots, ","), cache)
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return receiptModuleCache{}, err
	}
	for _, r := range roots {
		dir := filepath.Join(home, filepath.FromSlash(r))
		before := snaps[r]
		for i, f := range moduleRootFiles {
			b, err := os.ReadFile(filepath.Join(dir, f))
			if err != nil || !bytes.Equal(b, before[i]) {
				return receiptModuleCache{}, fmt.Errorf("could-not-check — module root %s changed its %s after the egress screen; nothing was downloaded for it and no receipt was written", r, f)
			}
		}
		if out, err := goModDownload(dir, env); err != nil {
			return receiptModuleCache{}, fmt.Errorf("could-not-check — `go mod download` for %s failed (%v): %s. No receipt was written and the image's baked cache is not a substitute", r, err, lastLines(string(out), 5))
		}
		for i, f := range moduleRootFiles {
			b, err := os.ReadFile(filepath.Join(dir, f))
			if err != nil || !bytes.Equal(b, before[i]) {
				return receiptModuleCache{}, fmt.Errorf("could-not-check — `go mod download` for %s changed its %s; the download must be bound to the source's go.sum as committed", r, f)
			}
		}
	}
	digest, err := homeTreeDigest(cache)
	if err != nil {
		return receiptModuleCache{}, fmt.Errorf("could-not-check — %v", err)
	}
	return receiptModuleCache{Digest: digest, Roots: roots, PublicAssay: proof}, nil
}

// moduleRootFiles are the two files of a module root the download is bound to.
var moduleRootFiles = [2]string{"go.mod", "go.sum"}

// snapshotModuleRoots reads every root's go.mod and go.sum once and writes a
// copy of exactly those bytes under dst, at the same home-relative paths, for
// the egress screen to read. It returns the bytes per root and dst.
func snapshotModuleRoots(home string, roots []string, dst string) (map[string][2][]byte, string, error) {
	snaps := map[string][2][]byte{}
	for _, r := range roots {
		var snap [2][]byte
		for i, f := range moduleRootFiles {
			b, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(r), f))
			if err != nil {
				return nil, "", fmt.Errorf("could-not-check — module root %s: %v; the download is bound to each root's go.mod and go.sum, and nothing was sent to the proxy", r, err)
			}
			snap[i] = b
		}
		out := filepath.Join(dst, filepath.FromSlash(r))
		if err := os.MkdirAll(out, 0o700); err != nil {
			return nil, "", err
		}
		for i, f := range moduleRootFiles {
			if err := os.WriteFile(filepath.Join(out, f), snap[i], 0o600); err != nil {
				return nil, "", err
			}
		}
		snaps[r] = snap
	}
	return snaps, dst, nil
}

// moduleCacheRequirementProblem binds a receipt's module cache to the home it
// admits, at every stage that reads a receipt (readBracketReceipt: the
// container run and the close): a home that lists module roots needs a
// receipt carrying the cache downloaded for exactly those roots, and a home
// that lists none needs a receipt carrying none. A receipt from an open step
// that predates the cache, or one edited after it, refuses here.
func moduleCacheRequirementProblem(home string, mc *receiptModuleCache) error {
	roots, listed := bakedModuleRoots(filepath.Join(home, filepath.FromSlash(moduleRootsManifest)))
	switch {
	case listed && mc == nil:
		return fmt.Errorf("could-not-check — the home lists Go module roots in %s, but the receipt carries no module_cache; its rows would resolve from the image's baked cache. Re-open the bracket with --module-cache", moduleRootsManifest)
	case !listed && mc != nil:
		return fmt.Errorf("the receipt carries a module cache, but the home lists no Go module roots in %s", moduleRootsManifest)
	case listed && !slices.Equal(roots, mc.Roots):
		return fmt.Errorf("the receipt's module cache was downloaded for roots %s, but the home lists %s in %s", strings.Join(mc.Roots, ","), strings.Join(roots, ","), moduleRootsManifest)
	}
	if mc != nil {
		return publicAssayReceiptProblem(home, roots, mc.PublicAssay)
	}
	return nil
}

// lastLines is the last n non-empty lines of s, joined with " / ".
func lastLines(s string, n int) string {
	var kept []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			kept = append(kept, l)
		}
	}
	if len(kept) > n {
		kept = kept[len(kept)-n:]
	}
	if len(kept) == 0 {
		return "no output"
	}
	return strings.Join(kept, " / ")
}

// containerModuleCacheFlags are the container-engine flags a run under a
// receipt with a module cache needs, as the open step prints them: no network,
// a read-only root, the cache mounted read-only at the image's GOMODCACHE, and
// the module environment that keeps resolution offline and checked. The
// container step re-checks each of these it can see (containerModuleCacheProblem).
func containerModuleCacheFlags(cache string) []string {
	return []string{
		"--network", "none",
		"--read-only",
		"--mount", "type=bind,source=" + cache + ",target=" + containerModuleCacheTarget + ",readonly",
		"--env", "GOMODCACHE=" + containerModuleCacheTarget,
		"--env", "GOPROXY=off",
		"--env", "GOTOOLCHAIN=local",
		"--env", "GOFLAGS=-mod=readonly",
	}
}

// mountReadOnly reports whether the file system holding path is mounted
// read-only. A var so tests stub it; off Linux it cannot be checked.
var mountReadOnly = platformMountReadOnly

// containerModuleCacheProblem is the container half: before any row, the
// module cache the rows resolve from must be the open step's, read-only, and
// the module environment must keep resolution offline and checksum-verified.
func containerModuleCacheProblem(root string, mc *receiptModuleCache) error {
	cache := os.Getenv("GOMODCACHE")
	if cache == "" {
		return fmt.Errorf("the receipt carries a module cache, but GOMODCACHE is unset; mount the open step's cache read-only at %s", containerModuleCacheTarget)
	}
	if rel, ok := relWithin(root, cache); ok {
		return fmt.Errorf("GOMODCACHE %s sits inside the root (%s)", cache, rel)
	}
	if v := os.Getenv("GOPROXY"); v != "off" {
		return fmt.Errorf("GOPROXY is %q; a container run under a module-cache receipt keeps GOPROXY=off", v)
	}
	if v := os.Getenv("GOTOOLCHAIN"); v != "local" {
		return fmt.Errorf("GOTOOLCHAIN is %q; a container run under a module-cache receipt keeps GOTOOLCHAIN=local", v)
	}
	// GONOSUMDB, GOINSECURE, GOPRIVATE, GONOPROXY and GOSUMDB=off are the live
	// checksum and proxy controls. GONOSUMCHECK and GOFLAGS -insecure are not
	// settings of the current go command; they are refused as legacy,
	// defensive refusals only.
	for _, k := range []string{"GONOSUMDB", "GONOSUMCHECK", "GOINSECURE", "GOPRIVATE", "GONOPROXY"} {
		if v := os.Getenv(k); v != "" {
			return fmt.Errorf("%s is set (%q); go.sum verification stays on in the container", k, v)
		}
	}
	if v := os.Getenv("GOSUMDB"); v == "off" {
		return fmt.Errorf("GOSUMDB=off; go.sum verification stays on in the container")
	}
	if strings.Contains(os.Getenv("GOFLAGS"), "-insecure") {
		return fmt.Errorf("GOFLAGS carries -insecure; go.sum verification stays on in the container")
	}
	for _, p := range []struct{ path, what string }{{"/", "the root file system"}, {cache, "the module cache " + cache}} {
		ro, err := mountReadOnly(p.path)
		if err != nil {
			return fmt.Errorf("could-not-check whether %s is mounted read-only: %v", p.what, err)
		}
		if !ro {
			return fmt.Errorf("%s is mounted read-write; mount it read-only", p.what)
		}
	}
	digest, err := homeTreeDigest(cache)
	if err != nil {
		return err
	}
	if digest != mc.Digest {
		return fmt.Errorf("the module cache at GOMODCACHE %s digests to %s, not the %s the open step downloaded; mount the open step's cache, never another", cache, digest, mc.Digest)
	}
	return nil
}

// receiptModuleCacheProblem validates a receipt's module cache record.
func receiptModuleCacheProblem(mc *receiptModuleCache) error {
	if mc == nil {
		return nil
	}
	if !sha256DigestRe.MatchString(mc.Digest) {
		return fmt.Errorf("the receipt's module cache carries no sha256 digest")
	}
	if len(mc.Roots) == 0 {
		return fmt.Errorf("the receipt's module cache names no module root")
	}
	for _, r := range mc.Roots {
		if !filepath.IsLocal(r) {
			return fmt.Errorf("the receipt's module cache names %q, which is not a home-relative module root", r)
		}
	}
	return nil
}

// spanModuleCaches returns the values of a span's Module-Cache lines, trimmed.
func spanModuleCaches(span []string) []string {
	var out []string
	for _, l := range span {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, strings.TrimSpace(moduleCachePrefix)) {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(l, strings.TrimSpace(moduleCachePrefix))))
		}
	}
	return out
}
