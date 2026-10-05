// Package hotspot ranks the files under tools/desk by change history: churn × complexity
// (Tornhill's hotspot), and reports temporal coupling (files that change together). It is
// the metric half of the brittle mark (docs/contracts.md, "Brittle marks"): the ranking
// NOMINATES, the class defect history CONFIRMS, and nothing in this package marks anything.
//
// WHY. The weight counter (internal/weight) says how much machinery there is; it does not
// say where the next fix will land. Change history does: a few per cent of files take most
// of the changes and most of the defects (Nagappan & Ball; Graves et al.; Rahman &
// Devanbu). This package is a pure, reproducible count over a parsed `git log` stream and a
// file tree, so the same window at the same ref gives the same ranking on any machine.
//
// DEFINITIONS (all over in-scope files — Options.Prefix, `.go`, not `_test.go`, not under a
// testdata/ or vendor/ directory — and only commits inside [Since, Until]):
//
//	churn      — commits in the log that touch the file. The log is the first-parent line
//	             (`git log --first-parent`), so a merge or squash counts once.
//	fixes      — those commits whose subject matches FixPattern. A proxy, stated as one;
//	             the error-class record is the real defect history.
//	complexity — the sum of leading tab counts over the file's lines in the tree at Until
//	             (indentation sum). Read from the tree, never from the log.
//	score      — churn × complexity, ranked descending (ties: churn desc, then path).
//	             Pct is 100·rank/n: "in the top Pct per cent".
//	coupling   — over commits touching at most MaxChangeset in-scope files, the count of
//	             commits touching both files of a pair; reported when count ≥ MinCount and
//	             count / min(churn_a, churn_b) ≥ MinRatio. Coupling is an architecture
//	             signal (a hidden seam), not a defect predictor.
//
// The only git reads are in hotspot_test.go (TestPrintHotspots), read-only. This package
// runs no process and is never a CI gate: the ranking needs full history, and a ranking is
// not a pass/fail.
package hotspot

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"time"
)

// FixPattern is the subject matcher for the fixes proxy. `\b` on both sides keeps
// "prefix" and "fixture" out; "revert" matches any subject that reverts.
var FixPattern = regexp.MustCompile(`(?i)\bfix(es|ed)?\b|revert`)

// DateLayout is the `%ci` layout the log carries.
const DateLayout = "2006-01-02 15:04:05 -0700"

// Commit is one parsed log entry.
type Commit struct {
	Hash    string
	Date    time.Time // UTC
	Subject string
	Files   []string // repository-relative paths, each once, in log order
}

// Options scopes a count. The zero value of each field takes the default below.
type Options struct {
	Prefix       string    // in-scope path prefix; default "tools/desk/"
	Since, Until time.Time // inclusive window; a zero bound is open
	MaxChangeset int       // coupling: commits with more in-scope files are skipped; default 8
	MinCount     int       // coupling: minimum shared commits; default 5
	MinRatio     float64   // coupling: minimum count/min(churn); default 0.3
}

func (o Options) withDefaults() Options {
	if o.Prefix == "" {
		o.Prefix = "tools/desk/"
	}
	if o.MaxChangeset == 0 {
		o.MaxChangeset = 8
	}
	if o.MinCount == 0 {
		o.MinCount = 5
	}
	if o.MinRatio == 0 {
		o.MinRatio = 0.3
	}
	return o
}

// FileScore is one ranked file.
type FileScore struct {
	Path       string
	Churn      int
	Fixes      int
	Complexity int
	Lines      int
	Score      int
	Rank       int     // 1-based
	Pct        float64 // 100·Rank/len(ranking)
}

// Pair is one reported temporal-coupling pair, A < B.
type Pair struct {
	A, B  string
	Count int
	Ratio float64 // Count / min(churn_A, churn_B)
}

// Parse reads `git log --name-only --format=%H%x00%ci%x00%s`. A header line carries the
// two NUL separators; the non-blank lines after it are that commit's files. A commit with
// no files (an empty commit) is kept with no files.
func Parse(r io.Reader) ([]Commit, error) {
	var out []Commit
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if strings.Contains(line, "\x00") {
			parts := strings.SplitN(line, "\x00", 3)
			if len(parts) != 3 || !isHash(parts[0]) {
				return nil, fmt.Errorf("line %d: malformed commit header %q", lineNo, line)
			}
			d, err := time.Parse(DateLayout, parts[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: commit %s: date: %w", lineNo, parts[0], err)
			}
			out = append(out, Commit{Hash: parts[0], Date: d.UTC(), Subject: parts[2]})
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("line %d: file %q before any commit header", lineNo, line)
		}
		c := &out[len(out)-1]
		if !contains(c.Files, line) {
			c.Files = append(c.Files, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func isHash(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// InScope reports whether path is counted: under prefix, `.go`, not `_test.go`, and not
// inside a testdata/ or vendor/ directory (fixtures and vendored code are not the tool).
func InScope(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return false
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "testdata" || seg == "vendor" {
			return false
		}
	}
	return true
}

func inWindow(c Commit, o Options) bool {
	if !o.Since.IsZero() && c.Date.Before(o.Since) {
		return false
	}
	if !o.Until.IsZero() && c.Date.After(o.Until) {
		return false
	}
	return true
}

// scoped returns each in-window commit's in-scope files (nil for a commit with none).
func scoped(commits []Commit, o Options) [][]string {
	out := make([][]string, len(commits))
	for i, c := range commits {
		if !inWindow(c, o) {
			continue
		}
		for _, f := range c.Files {
			if InScope(f, o.Prefix) {
				out[i] = append(out[i], f)
			}
		}
	}
	return out
}

func churnOf(files [][]string) map[string]int {
	churn := map[string]int{}
	for _, fs := range files {
		for _, f := range fs {
			churn[f]++
		}
	}
	return churn
}

// Score ranks every in-scope file the window's commits touched and that exists in tree
// (the tree at Until, keyed by repository-relative path). A file absent from the tree —
// deleted or renamed away by Until — has no complexity to measure and is not ranked.
func Score(commits []Commit, tree fs.FS, opts Options) ([]FileScore, error) {
	o := opts.withDefaults()
	files := scoped(commits, o)
	churn := churnOf(files)
	fixes := map[string]int{}
	for i, c := range commits {
		if len(files[i]) > 0 && FixPattern.MatchString(c.Subject) {
			for _, f := range files[i] {
				fixes[f]++
			}
		}
	}
	var out []FileScore
	for path, n := range churn {
		src, err := fs.ReadFile(tree, path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		cx, lines := Indentation(src)
		out = append(out, FileScore{Path: path, Churn: n, Fixes: fixes[path], Complexity: cx, Lines: lines, Score: n * cx})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Churn != b.Churn {
			return a.Churn > b.Churn
		}
		return a.Path < b.Path
	})
	for i := range out {
		out[i].Rank = i + 1
		out[i].Pct = 100 * float64(i+1) / float64(len(out))
	}
	return out, nil
}

// Indentation returns the indentation sum (leading tabs per line, summed) and the line
// count of src.
func Indentation(src []byte) (sum, lines int) {
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		lines++
		for _, b := range sc.Bytes() {
			if b != '\t' {
				break
			}
			sum++
		}
	}
	return sum, lines
}

// Coupling returns the pairs that pass both filters, count descending, then by path.
func Coupling(commits []Commit, opts Options) []Pair {
	o := opts.withDefaults()
	files := scoped(commits, o)
	churn := churnOf(files)
	counts := map[[2]string]int{}
	for _, fs := range files {
		if len(fs) < 2 || len(fs) > o.MaxChangeset {
			continue
		}
		sorted := append([]string(nil), fs...)
		sort.Strings(sorted)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				counts[[2]string{sorted[i], sorted[j]}]++
			}
		}
	}
	var out []Pair
	for k, n := range counts {
		if n < o.MinCount {
			continue
		}
		m := churn[k[0]]
		if churn[k[1]] < m {
			m = churn[k[1]]
		}
		ratio := float64(n) / float64(m)
		if ratio < o.MinRatio {
			continue
		}
		out = append(out, Pair{A: k[0], B: k[1], Count: n, Ratio: ratio})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].A != out[j].A {
			return out[i].A < out[j].A
		}
		return out[i].B < out[j].B
	})
	return out
}
