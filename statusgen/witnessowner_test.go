package main

// witnessowner_test.go — statusgen/36 Verify row 10: the witness digest and
// Runner-cell grammar have ONE owner, statusgen/evaluation. A class guard
// scans every non-test Go file in statusgen and tools/desk outside that
// package for a second implementation: a Runner-cell `@` split, a hand-parsed
// schema marker, a git tree/attribute/worktree read, or a witness-record field
// key. Each hit must sit on the committed allow-list with a reason; a stale
// allow-list entry fails too.

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const witnessOwnerAllowlist = "testdata/witnessv2/owner-allowlist.txt"

type ownerPattern struct {
	id string
	re *regexp.Regexp
	// needs, when non-nil, must match the whole file for the line to count.
	needs *regexp.Regexp
}

var witnessOwnerPatterns = []ownerPattern{
	{id: "runner-at-split", re: regexp.MustCompile(`(Cut|CutPrefix|Index|IndexByte|LastIndex|LastIndexByte|Split|SplitN)\(.*("@"|'@')`), needs: regexp.MustCompile(`Runner|(?i:witness)`)},
	{id: "schema-marker", re: regexp.MustCompile(`\[w[0-9]+\]|\\\[w`)},
	{id: "git-tree-read", re: regexp.MustCompile(`ls-tree|hash-object|check-attr`)},
	{id: "record-field-key", re: regexp.MustCompile(`"(tree_digest|dirty_paths|command_sha256|expect_sha256|class_sha256|output_sha256)"|json:"(tree_digest|dirty_paths|command_sha256|expect_sha256|class_sha256|output_sha256)`)},
	{id: "record-dir", re: regexp.MustCompile(`WitnessRecordDir`)},
}

type ownerHit struct {
	File, Pattern string
	Line          int
}

// witnessOwnerHits scans the given files (path → content) and returns every
// line matching a pattern, skipping comment lines.
func witnessOwnerHits(files map[string]string) []ownerHit {
	var hits []ownerHit
	for path, src := range files {
		for _, p := range witnessOwnerPatterns {
			if p.needs != nil && !p.needs.MatchString(src) {
				continue
			}
			for i, line := range strings.Split(src, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				if p.re.MatchString(line) {
					hits = append(hits, ownerHit{path, p.id, i + 1})
				}
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].File != hits[j].File {
			return hits[i].File < hits[j].File
		}
		if hits[i].Pattern != hits[j].Pattern {
			return hits[i].Pattern < hits[j].Pattern
		}
		return hits[i].Line < hits[j].Line
	})
	return hits
}

// readOwnerAllowlist parses `path | pattern | reason` lines.
func readOwnerAllowlist(t *testing.T, text string) map[[2]string]string {
	t.Helper()
	out := map[[2]string]string{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		parts := strings.SplitN(l, "|", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[2]) == "" {
			t.Fatalf("allow-list line %q: want `path | pattern | reason` with a reason", l)
		}
		out[[2]string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])}] = strings.TrimSpace(parts[2])
	}
	return out
}

// ownerViolations returns the hits not on the allow-list, and the allow-list
// entries that matched nothing.
func ownerViolations(hits []ownerHit, allow map[[2]string]string) (unlisted []ownerHit, stale [][2]string) {
	used := map[[2]string]bool{}
	for _, h := range hits {
		k := [2]string{h.File, h.Pattern}
		if _, ok := allow[k]; ok {
			used[k] = true
			continue
		}
		unlisted = append(unlisted, h)
	}
	for k := range allow {
		if !used[k] {
			stale = append(stale, k)
		}
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i][0]+stale[i][1] < stale[j][0]+stale[j][1] })
	return unlisted, stale
}

// repoWitnessSources loads every non-test Go file of statusgen, tools/desk and the
// lifted gitcore module outside statusgen/evaluation, keyed by repo-relative path.
func repoWitnessSources(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, base := range []struct{ dir, prefix string }{{".", "statusgen"}, {filepath.Join("..", "tools", "desk"), "tools/desk"}, {filepath.Join("..", "gitcore"), "gitcore"}} {
		err := filepath.WalkDir(base.dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(base.dir, p)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if base.prefix == "statusgen" && (rel == "evaluation" || rel == "testdata") || d.Name() == "testdata" || d.Name() == "vendor" || d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			files[base.prefix+"/"+rel] = string(b)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// TestWitnessDigestOneOwner: every second-implementation signature outside
// statusgen/evaluation is on the committed allow-list, with a reason.
// regression: #2026
func TestWitnessDigestOneOwner(t *testing.T) {
	raw, err := os.ReadFile(witnessOwnerAllowlist)
	if err != nil {
		t.Fatal(err)
	}
	files := repoWitnessSources(t)
	if len(files) < 100 {
		t.Fatalf("non-vacuity: scanned only %d files", len(files))
	}
	allow := readOwnerAllowlist(t, string(raw))
	unlisted, stale := ownerViolations(witnessOwnerHits(files), allow)
	for _, h := range unlisted {
		t.Errorf("%s:%d matches %s outside statusgen/evaluation and is not on %s — route it through the library, or list it with a reason", h.File, h.Line, h.Pattern, witnessOwnerAllowlist)
	}
	for _, k := range stale {
		t.Errorf("allow-list entry %s | %s matches nothing — remove it", k[0], k[1])
	}
}

// TestWitnessOwnerFlagsPlant: the guard flags a planted second parser, and an
// allow-list entry with a reason clears exactly that file and pattern.
// regression: #2026
func TestWitnessOwnerFlagsPlant(t *testing.T) {
	plant := `package main

import ("os/exec"; "strings")

// a second Runner-cell parser, a second tree read, a second record key
func parse(cell string) string {
	id, _, _ := strings.Cut(cell, "@")
	_ = exec.Command("git", "ls-tree", "-r", "HEAD")
	_ = "tree_digest"
	return strings.TrimSpace(id) + "[w2]"
}
`
	clean := "package main\n\nfunc add(a, b int) int { return a + b }\n"
	files := map[string]string{"statusgen/plant.go": plant, "statusgen/clean.go": clean}
	hits := witnessOwnerHits(files)
	got := map[string]bool{}
	for _, h := range hits {
		if h.File != "statusgen/plant.go" {
			t.Errorf("clean file flagged: %+v", h)
		}
		got[h.Pattern] = true
	}
	for _, want := range []string{"runner-at-split", "git-tree-read", "record-field-key", "schema-marker"} {
		if !got[want] {
			t.Errorf("the planted %s site was not flagged (hits: %+v)", want, hits)
		}
	}
	unlisted, _ := ownerViolations(hits, map[[2]string]string{})
	if len(unlisted) != len(hits) {
		t.Errorf("with an empty allow-list every hit is a violation: %d of %d", len(unlisted), len(hits))
	}
	allow := readOwnerAllowlist(t, "statusgen/plant.go | runner-at-split | planted\nstatusgen/plant.go | git-tree-read | planted\nstatusgen/plant.go | record-field-key | planted\nstatusgen/plant.go | schema-marker | planted\n")
	if unlisted, stale := ownerViolations(hits, allow); len(unlisted) != 0 || len(stale) != 0 {
		t.Errorf("a listed plant still violates: unlisted=%v stale=%v", unlisted, stale)
	}
}
