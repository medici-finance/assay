package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The open step's module cache (#4683). The verify-runner image bakes a Go
// module cache when it is built, so its rows can resolve only what the release
// that built the image required; a tree verified later may require more, and
// with no network its Go rows stop on module resolution (modcachegap.go names
// that cause). This file derives the cache from the tree under verification
// instead:
//
//  1. host open: with --module-cache <dir>, `verifyrun --open-bracket` runs
//     `go mod download` for every root the home's own moduleRootsManifest
//     lists, into that new directory, bound to each root's go.sum and to the
//     checksum database, and records the cache's digest and roots in the
//     receipt (receiptModuleCache). Any download failure refuses the open step
//     with no receipt: there is no fall-back to the image's baked cache;
//  2. container: the run reads the receipt and, before any row, refuses
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
	Digest string   `json:"digest"`
	Roots  []string `json:"roots"`
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

// moduleDownloadEnv is the environment of the open step's downloads: the
// host's, with every Go, git and proxy setting removed, then the pinned ones
// (the same set the cluster backend's stage-cache.sh pins). Checksum
// verification stays on: GOSUMDB is the public database, and GONOSUMDB,
// GONOSUMCHECK, GOINSECURE, GOPRIVATE, GONOPROXY and GOFLAGS=-insecure cannot
// arrive from the host, nor from its go env file (GOENV=off).
func moduleDownloadEnv(cache string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(k, "GO"), strings.HasPrefix(k, "GIT_"), strings.HasPrefix(k, "CGO_"):
			continue
		case strings.EqualFold(k, "http_proxy"), strings.EqualFold(k, "https_proxy"), strings.EqualFold(k, "all_proxy"), strings.EqualFold(k, "no_proxy"):
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"GOENV=off",
		"GOMODCACHE="+cache,
		"GOWORK=off",
		"GOFLAGS=-mod=readonly",
		"GOTOOLCHAIN=local",
		"GOPROXY=https://proxy.golang.org",
		"GOSUMDB=sum.golang.org",
		"GOVCS=*:off",
	)
}

// moduleCachePathProblem refuses a --module-cache directory the open step may
// not fill: inside the home (it would change the tree the bracket binds), the
// host's own module cache or anything inside or around it, or a directory that
// already holds anything.
func moduleCachePathProblem(home, cache string) string {
	if !filepath.IsAbs(cache) {
		return fmt.Sprintf("--module-cache %s is not an absolute path", cache)
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
// digest. Each root's go.mod and go.sum must be byte-identical afterwards, so
// the set downloaded is the one go.sum names. Any failure is returned, and the
// caller writes no receipt.
func prepareModuleCache(home, cache string) (receiptModuleCache, error) {
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
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return receiptModuleCache{}, err
	}
	env := moduleDownloadEnv(cache)
	for _, r := range roots {
		dir := filepath.Join(home, filepath.FromSlash(r))
		var before [2][]byte
		for i, f := range []string{"go.mod", "go.sum"} {
			b, err := os.ReadFile(filepath.Join(dir, f))
			if err != nil {
				return receiptModuleCache{}, fmt.Errorf("could-not-check — module root %s: %v; the download is bound to each root's go.mod and go.sum", r, err)
			}
			before[i] = b
		}
		if out, err := goModDownload(dir, env); err != nil {
			return receiptModuleCache{}, fmt.Errorf("could-not-check — `go mod download` for %s failed (%v): %s. No receipt was written and the image's baked cache is not a substitute", r, err, lastLines(string(out), 5))
		}
		for i, f := range []string{"go.mod", "go.sum"} {
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
	return receiptModuleCache{Digest: digest, Roots: roots}, nil
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
