package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// witnessdigest.go — `statusgen witnessdigest --json --brief <path>
// [--rev <rev> | --worktree] [--row <id>]` (statusgen/36). It prints the
// canonical v2 content fields of a brief at a revision — the same derivation
// the reader compares with (evaluation.ComputeWitnessContent) — so a writer in
// another module (statusgen/37) records the digests without reimplementing
// them. A relative --brief resolves against --root, and the brief is read at
// the same revision as the tree unless --worktree is given. It never prints a releasable record: no runner, result, date, change
// request or record id, only content fields and the row's text digests.

const (
	witnessDigestOK       = 0
	witnessDigestCouldNot = 2
)

// witnessDigestOut is the verb's JSON document.
type witnessDigestOut struct {
	Schema     int               `json:"schema"`
	Brief      string            `json:"brief"`
	Rev        string            `json:"rev"`
	Scope      string            `json:"scope"`
	Inputs     map[string]string `json:"inputs"`
	Implicit   map[string]string `json:"implicit"`
	Attributes map[string]string `json:"attributes"`
	TreeDigest string            `json:"tree_digest,omitempty"`
	DirtyPaths []string          `json:"dirty_paths,omitempty"`
	Row        string            `json:"row,omitempty"`
	CommandSHA string            `json:"command_sha256,omitempty"`
	ExpectSHA  string            `json:"expect_sha256,omitempty"`
	ClassSHA   string            `json:"class_sha256,omitempty"`
	InputsNote string            `json:"inputs_note,omitempty"`
}

func runWitnessDigest(args []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("witnessdigest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	briefPath := flags.String("brief", "", "brief file whose declared inputs are digested (a relative path resolves against --root)")
	root := flags.String("root", ".", "repository root")
	rev := flags.String("rev", "", "revision to digest (default HEAD); the brief is read at the same revision")
	worktree := flags.Bool("worktree", false, "digest the working tree (HEAD overlaid with working-tree blob ids) and report dirty_paths; the brief is read from its working file")
	row := flags.String("row", "", "Verify row id: also print that row's command/expect/class digests")
	jsonMode := flags.Bool("json", false, "print the JSON document (required)")
	if err := flags.Parse(args); err != nil {
		return witnessDigestCouldNot
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "statusgen witnessdigest: could-not-check: "+format+"\n", a...)
		return witnessDigestCouldNot
	}
	if !*jsonMode {
		return fail("--json is required (the only output form)")
	}
	if *briefPath == "" {
		return fail("--brief <path> is required")
	}
	if *worktree && *rev != "" {
		return fail("--rev and --worktree are alternatives; pass one")
	}
	base := *rev
	if base == "" {
		base = "HEAD"
	}
	sha, err := gitOutput(*root, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	if err != nil {
		return fail("revision %q does not name a commit: %v", base, err)
	}
	sha = strings.TrimSpace(sha)

	// A relative --brief names a path under --root, wherever the verb runs.
	// The brief is read at the digested revision (HEAD by default), so its
	// Verify row and inputs: field match the tree; only --worktree reads the
	// working file.
	briefArg := *briefPath
	if !filepath.IsAbs(briefArg) {
		briefArg = filepath.Join(*root, briefArg)
	}
	briefFile := briefArg
	if !*worktree {
		abs, aerr := filepath.Abs(briefArg)
		rootAbs, rerr := filepath.Abs(*root)
		if aerr != nil || rerr != nil {
			return fail("cannot resolve %s against %s", *briefPath, *root)
		}
		rel, rerr := filepath.Rel(rootAbs, abs)
		if rerr != nil || strings.HasPrefix(rel, "..") {
			return fail("%s lies outside the root", *briefPath)
		}
		text, gerr := gitOutput(*root, "show", "--end-of-options", sha+":"+filepath.ToSlash(rel))
		if gerr != nil {
			return fail("%s does not exist at %s", filepath.ToSlash(rel), base)
		}
		dir, derr := os.MkdirTemp("", "sg-witnessdigest-")
		if derr != nil {
			return fail("%v", derr)
		}
		defer os.RemoveAll(dir)
		// expectedBriefID reads the stream from the parent directory name.
		sub := filepath.Join(dir, filepath.Base(filepath.Dir(abs)))
		if derr := os.MkdirAll(sub, 0o755); derr != nil {
			return fail("%v", derr)
		}
		briefFile = filepath.Join(sub, filepath.Base(abs))
		if derr := os.WriteFile(briefFile, []byte(text), 0o644); derr != nil {
			return fail("%v", derr)
		}
	}
	bf, ok, perr := parseBriefFile(briefFile)
	if perr != nil {
		return fail("%v", perr)
	}
	if !ok || bf == nil {
		return fail("%s is not a schema: brief-v1 file", *briefPath)
	}
	id, _, okName := expectedBriefID(briefFile)
	if !okName {
		return fail("%s is not a brief-<NN>[-slug].md file", *briefPath)
	}

	declared, found, problem := evaluation.ParseInputsDeclaration(extractSectionByPrefix(bf.Body, "Context"))
	spec := evaluation.WitnessClaimSpec(declared, found, nil, *row)

	repo := coverageRepo{root: *root}
	committed, okList := repo.ListTree(sha)
	if !okList {
		return fail("cannot list the tree at %s", sha)
	}
	listing := committed
	var dirty []string
	if *worktree {
		var paths []string
		for _, e := range committed {
			if _, serr := os.Lstat(filepath.Join(*root, filepath.FromSlash(e.Path))); serr == nil {
				paths = append(paths, e.Path)
			}
		}
		untracked, uerr := gitOutput(*root, "ls-files", "-z", "--others", "--exclude-standard")
		if uerr != nil {
			return fail("cannot list untracked files: %v", uerr)
		}
		paths = append(paths, splitNUL([]byte(untracked))...)
		hashed, okHash := repo.HashWorktree(paths)
		if !okHash {
			return fail("cannot hash the working tree")
		}
		listing, dirty = evaluation.WorktreeListing(committed, hashed)
		// A committed path deleted from the working tree is dirty and absent.
		var kept []evaluation.TreeBlob
		present := map[string]bool{}
		for _, p := range paths {
			present[p] = true
		}
		for _, e := range listing {
			if _, inHash := hashed[e.Path]; !inHash && !present[e.Path] {
				dirty = append(dirty, e.Path)
				continue
			}
			kept = append(kept, e)
		}
		listing = kept
	}
	// Attributes are resolved for the required paths at the revision (the
	// committed .gitattributes are the authority a digest binds).
	pre := evaluation.ComputeWitnessContent(spec, listing, nil)
	var attrPaths []string
	for p := range pre.Inputs {
		attrPaths = append(attrPaths, p)
	}
	attrs, okAttr := repo.CheckAttr(sha, attrPaths)
	if !okAttr {
		return fail("cannot read attributes at %s", sha)
	}
	content := evaluation.ComputeWitnessContent(spec, listing, attrs)

	out := witnessDigestOut{
		Schema: 2, Brief: id, Rev: sha, Scope: content.Scope,
		Inputs: content.Inputs, Implicit: content.Implicit, Attributes: content.Attributes,
		TreeDigest: content.TreeDigest, InputsNote: problem,
	}
	if *worktree {
		out.Rev = "worktree"
		out.DirtyPaths = uniqueSorted(dirty)
	}
	if *row != "" {
		var found bool
		rows, classCells := briefVerifyRowsWithClassCells(bf.Verify)
		for i, r := range rows {
			if r.ID != *row {
				continue
			}
			found = true
			out.Row = r.ID
			out.CommandSHA = evaluation.WitnessTextDigest(r.Command)
			out.ExpectSHA = evaluation.WitnessTextDigest(r.Expect)
			out.ClassSHA = evaluation.WitnessTextDigest(classCells[i])
		}
		if !found {
			return fail("%s has no Verify row %q", id, *row)
		}
	}
	buf, merr := json.MarshalIndent(out, "", "  ")
	if merr != nil {
		return fail("%v", merr)
	}
	fmt.Fprintln(stdout, string(buf))
	return witnessDigestOK
}

func gitOutput(root string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := witnessGit(root, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func uniqueSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
