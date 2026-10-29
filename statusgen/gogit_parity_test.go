package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// regression: #2395
func TestGoGitReadParity(t *testing.T) {
	t.Run("reftable-discovery", coreReftableParity)
	t.Run("nested-dotgit-discovery", coreNestedDotGitParity)
	t.Run("repository-metadata-errors", coreRepositoryProfileErrors)
	t.Run("blob-bytes-and-errors", TestGitShowObjectMatchesExec)
	t.Run("run-lifetime", TestGitReadSessionEndsWithRun)
	t.Run("strict-raw-author", TestVflipAmbiguousAuthor)
	t.Run("lenient-raw-author", TestVflipLintAmbiguousHeader)
	t.Run("lower-reader-policy", TestVflipLenientReaderCallers)
	t.Run("replacement-isolation", TestVflipReplaceObject)
	t.Run("graft-isolation", TestHistoryGitGrafts)
	t.Run("tree-and-profile-reads", coreParityReads)
	t.Run("commit-resolution", coreResolveParity)
	t.Run("caret-resolution", coreCaretParity)
	t.Run("treeish-selectors", coreTreeishParity)
	t.Run("native-path-selectors", corePathParity)
	t.Run("native-discovery", coreDiscoveryParity)
	t.Run("configured-worktree", coreWorktreeParity)
	t.Run("worktree-relative-prefix", coreWorktreePrefixParity)
	t.Run("shallow-metadata", coreShallowParity)
	t.Run("shallow-profiles", coreShallowProfiles)
	t.Run("directory-context", coreContextParity)
	t.Run("ref-namespace", coreRefNamespaceParity)
	t.Run("empty-environment", coreEmptyEnvParity)
	t.Run("stale-fail-message-multiset", coreStaleFailMessageParity)
	t.Run("last-path-time-and-reference-snapshot", coreMetadataReferenceParity)
	t.Run("reference-snapshot-history-independent", TestReferenceSnapshotHistoryIndependent)
}

func parityGitBytes(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	b, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		t.Fatalf("native oracle %v: %v", args, err)
	}
	return b
}

func coreParityReads(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	for p, b := range map[string][]byte{"docs/f": {0, 1, 2, '\r', '\n', 255}, "tab\t-é": []byte("text\n")} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("docs/f", filepath.Join(root, "symbolic")); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "first")
	before := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	runGit(t, root, "mv", "docs/f", "docs/g")
	runGit(t, root, "commit", "-qm", "rename")
	head := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	check := func(t *testing.T, dir string) {
		t.Helper()
		repoCacheMu.Lock()
		clear(treeFilesCache)
		clear(treeModesCache)
		clear(rootPrefixCache)
		repoCacheMu.Unlock()
		rawCommitAuthors.Delete(head)
		end := beginGitReadSession()
		defer end()
		if got := gitCurrentSHA(dir); got != strings.TrimSpace(string(parityGitBytes(t, dir, "rev-parse", "HEAD"))) {
			t.Fatalf("HEAD id %s", got)
		}
		top, err := repoTopLevel(dir)
		if err != nil || top != strings.TrimSpace(string(parityGitBytes(t, dir, "rev-parse", "--show-toplevel"))) {
			t.Fatalf("toplevel %q %v", top, err)
		}
		read := gitCoreFor(dir)
		if read.err != nil || read.native != "" {
			t.Fatalf("qualified cold profile unavailable: %+v", read)
		}
		nativeRaw := parityGitBytes(t, dir, "--no-replace-objects", "cat-file", "commit", head)
		raw, err := read.repo.RawCommit(head)
		if err != nil || !bytes.Equal(raw, nativeRaw) {
			t.Fatalf("cold raw object differs: %v", err)
		}
		wantAuthor, err := parseRawAuthor(string(nativeRaw))
		if err != nil {
			t.Fatal(err)
		}
		gotAuthor, err := rawCommitAuthor(dir, head)
		if err != nil || gotAuthor != wantAuthor {
			t.Fatalf("cold author adapter differs: %v %v", gotAuthor, err)
		}
		r := coverageRepo{root: dir}
		if !r.InRepository() {
			t.Fatal("repository disappeared")
		}
		sh, ok := r.IsShallow()
		nativeSh := strings.TrimSpace(string(parityGitBytes(t, dir, "rev-parse", "--is-shallow-repository"))) == "true"
		if !ok || sh != nativeSh || isShallowRepository(dir) != nativeSh {
			t.Fatal("shallow result differs")
		}
		prefix, ok := r.RootPrefix()
		if !ok || prefix != strings.TrimSpace(string(parityGitBytes(t, dir, "rev-parse", "--show-prefix"))) {
			t.Fatalf("prefix %q %v", prefix, ok)
		}
		for _, rev := range []string{"HEAD", head, before} {
			id, ok := r.ResolveCommit(rev)
			if !ok || id != strings.TrimSpace(string(parityGitBytes(t, dir, "rev-parse", "--verify", rev+"^{commit}"))) {
				t.Fatalf("commit resolution %q %s %v", rev, id, ok)
			}
			wantFiles := splitNUL(parityGitBytes(t, dir, "ls-tree", "-r", "-z", "--name-only", "--full-tree", rev))
			repoCacheMu.Lock()
			_, cached := treeFilesCache[dir+"\x00"+rev]
			repoCacheMu.Unlock()
			if cached {
				t.Fatal("cold storage profile reused listing cache")
			}
			directEntries, err := read.repo.TreeEntries(rev, true, false)
			if err != nil {
				t.Fatal(err)
			}
			directNames := make([]string, 0, len(directEntries))
			for _, e := range directEntries {
				directNames = append(directNames, e.Path)
			}
			if !reflect.DeepEqual(directNames, wantFiles) {
				t.Fatalf("direct storage listing differs: %q != %q", directNames, wantFiles)
			}
			if got := r.TreeFiles(rev); !reflect.DeepEqual(got, wantFiles) {
				t.Fatalf("tree files %q != %q", got, wantFiles)
			}
			var wantModes []evaluation.TreeEntry
			for _, entry := range splitNUL(parityGitBytes(t, dir, "ls-tree", "-r", "-z", "--full-tree", rev)) {
				meta, path, _ := strings.Cut(entry, "\t")
				wantModes = append(wantModes, evaluation.TreeEntry{Mode: strings.Fields(meta)[0], Path: path})
			}
			gotModes, ok := r.TreeModes(rev)
			if !ok || !reflect.DeepEqual(gotModes, wantModes) {
				t.Fatalf("tree modes %v != %v", gotModes, wantModes)
			}
		}
		want := splitNUL(parityGitBytes(t, dir, "diff", "--name-only", "--no-renames", "-z", before, head, "--"))
		got, ok := r.DiffNames(before, head)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("no-rename diff %q != %q", got, want)
		}
		coreNoGitReads(t, dir, head)
	}
	t.Run("ordinary", func(t *testing.T) { check(t, root) })
	t.Run("nested", func(t *testing.T) { check(t, filepath.Join(root, "docs")) })
	runGit(t, root, "repack", "-adq")
	t.Run("packed", func(t *testing.T) { check(t, root) })
	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, root, "worktree", "add", "-q", "--detach", linked, "HEAD")
	t.Run("linked-worktree", func(t *testing.T) { check(t, linked) })
	shared := filepath.Join(t.TempDir(), "shared")
	runGit(t, root, "clone", "-q", "--shared", root, shared)
	t.Run("absolute-alternate", func(t *testing.T) { check(t, shared) })
	t.Run("relative-alternate", func(t *testing.T) {
		rel, err := filepath.Rel(filepath.Join(shared, ".git", "objects"), filepath.Join(root, ".git", "objects"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(shared, ".git", "objects", "info", "alternates"), []byte(rel+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if read := gitCoreFor(shared); read.err != nil || !strings.Contains(read.native, "relative alternate") {
			t.Fatalf("unsupported profile not observed: %+v", read)
		}
		want := parityGitBytes(t, shared, "show", "HEAD:docs/g")
		got, err := gitShowObject(shared, "HEAD", "docs/g")
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("native alternate compatibility %q %v", got, err)
		}
	})
	t.Run("missing-object-no-retry", func(t *testing.T) {
		bin := t.TempDir()
		log := filepath.Join(bin, "starts")
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\necho started >>'"+log+"'\nexit 99\n"), 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if _, err := rawCommitAuthor(root, strings.Repeat("e", 40)); err == nil {
			t.Fatal("missing provenance object read as an author")
		}

		if _, err := os.Stat(log); !os.IsNotExist(err) {
			t.Fatalf("failed in-process read retried native Git: %v", err)
		}
	})
}

func coreResolveParity(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "f")
	runGit(t, root, "commit", "-qm", "first")
	first := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("two\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "commit", "-qam", "second")
	head := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	tree := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD^{tree}")))
	blob := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD:f")))
	for tag, object := range map[string]string{"commit-tag": head, "tree-tag": tree, "blob-tag": blob} {
		runGit(t, root, "-c", "tag.gpgsign=false", "tag", "-a", tag, object, "-m", tag)
	}
	runGit(t, root, "branch", "lookalike", head)
	r := coverageRepo{root: root}
	for _, token := range []string{"HEAD", "HEAD~1", "HEAD^", "HEAD@{1}", "refs/heads/lookalike", "commit-tag", "refs/tags/commit-tag", "tree-tag", "blob-tag", tree, blob, first[:12], head, strings.Repeat("f", 40), "missing-ref"} {
		t.Run(token, func(t *testing.T) {
			out, err := historyGit(root, "rev-parse", "--verify", "--quiet", "--end-of-options", token+"^{commit}").Output()
			want := ""
			if err == nil {
				want = strings.TrimSpace(string(out))
			}
			got, ok := r.ResolveCommit(token)
			if ok != (err == nil) || got != want {
				t.Fatalf("ResolveCommit(%q)=(%q,%v), native=(%q,%v)", token, got, ok, want, err)
			}
		})
	}
}

func coreDiscoveryParity(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "f"), []byte("selected\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "one")
	var err error
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "docs")
	head := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	t.Setenv("GIT_CEILING_DIRECTORIES", root)
	native, err := exec.Command("git", "-C", nested, "rev-parse", "--show-toplevel").Output()
	if err == nil {
		t.Fatalf("native discovery ceiling fixture did not stop discovery: %q", native)
	}
	if top, e := repoTopLevel(nested); e == nil {
		t.Fatalf("ordinary discovery ignored native ceiling: %s", top)
	}
	if top, e := provenanceRepoTop(nested); e == nil {
		t.Fatalf("provenance discovery ignored native ceiling: %s", top)
	}
	rawCommitAuthors.Delete(head)
	if _, e := rawCommitAuthor(nested, head); e == nil {
		t.Fatal("provenance object read ignored native discovery ceiling")
	}
	read := gitCoreFor(nested)
	if read.err != nil || !strings.Contains(read.native, "discovery") {
		t.Fatalf("native discovery profile not selected before object I/O: %+v", read)
	}
}

func coreWorktreeParity(t *testing.T) {
	for _, mode := range []string{"local-bare", "local-worktree", "global-bare", "global-worktree"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			gitInit(t, root, "T", "t@example.invalid")
			if err := os.WriteFile(filepath.Join(root, "f"), []byte("selected\n"), 0644); err != nil {
				t.Fatal(err)
			}
			runGit(t, root, "add", "f")
			runGit(t, root, "commit", "-qm", "one")
			other := t.TempDir()
			key, value := "core.bare", "true"
			if strings.HasSuffix(mode, "worktree") {
				key, value = "core.worktree", other
			}
			if strings.HasPrefix(mode, "local") {
				runGit(t, root, "config", "--local", key, value)
			} else {
				global := filepath.Join(t.TempDir(), "config")
				if err := os.WriteFile(global, []byte("[core]\n"+strings.TrimPrefix(key, "core.")+" = "+value+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GIT_CONFIG_GLOBAL", global)
			}
			native, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
			want := ""
			if err == nil {
				want = strings.TrimSpace(string(native))
			}
			got, gotErr := repoTopLevel(root)
			if (gotErr == nil) != (err == nil) || got != want {
				t.Fatalf("ordinary %s discovery=(%q,%v), native=(%q,%v)", mode, got, gotErr, want, err)
			}
			got, gotErr = provenanceRepoTop(root)
			if (gotErr == nil) != (err == nil) || got != want {
				t.Fatalf("provenance %s discovery=(%q,%v), native=(%q,%v)", mode, got, gotErr, want, err)
			}
		})
	}
}

// coreTreeishParity keeps the generic show helper's accepted native selectors.
func coreTreeishParity(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "f")
	runGit(t, root, "commit", "-qm", "first")
	tree := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD^{tree}")))
	runGit(t, root, "-c", "tag.gpgsign=false", "tag", "-a", "tree-tag", tree, "-m", "tree")
	runGit(t, root, "-c", "tag.gpgsign=false", "tag", "-a", "commit-tree-tag", "HEAD", "-m", "commit")
	for _, rev := range []string{"HEAD", "HEAD^{tree}", tree, "tree-tag", "refs/tags/tree-tag", "commit-tree-tag", "refs/tags/commit-tree-tag", strings.Repeat("e", 40), "HEAD~0", "", ":0"} {
		t.Run(rev, func(t *testing.T) {
			native, wantErr := exec.Command("git", "-C", root, "show", rev+":f").Output()
			got, err := gitShowObject(root, rev, "f")
			if (err == nil) != (wantErr == nil) || !bytes.Equal(got, native) {
				t.Fatalf("generic show %q=(%q,%v), native=(%q,%v)", rev, got, err, native, wantErr)
			}
		})
	}
}

func coreWorktreePrefixParity(t *testing.T) {
	for _, profile := range []string{"inside", "outside"} {
		t.Run(profile, func(t *testing.T) {
			admin := t.TempDir()
			gitInit(t, admin, "T", "t@example.invalid")
			if err := os.WriteFile(filepath.Join(admin, "f"), []byte("stored\n"), 0644); err != nil {
				t.Fatal(err)
			}
			nested := filepath.Join(admin, "nested")
			if err := os.MkdirAll(nested, 0755); err != nil {
				t.Fatal(err)
			}
			runGit(t, admin, "add", "f")
			runGit(t, admin, "commit", "-qm", "stored")
			worktree := nested
			if profile == "outside" {
				worktree = t.TempDir()
			}
			runGit(t, admin, "config", "core.worktree", worktree)
			wantPrefix := strings.TrimSpace(string(parityGitBytes(t, nested, "rev-parse", "--show-prefix")))
			gotPrefix, ok := (coverageRepo{root: nested}).RootPrefix()
			if !ok || gotPrefix != wantPrefix {
				t.Errorf("configured prefix=(%q,%v), native=%q", gotPrefix, ok, wantPrefix)
			}
			wantBlob, wantErr := exec.Command("git", "-C", nested, "show", "HEAD:./f").Output()
			if profile == "inside" && wantErr != nil {
				t.Fatalf("positive configured path fixture: %v", wantErr)
			}
			for label, read := range map[string]func(string, string, string) ([]byte, error){"ordinary": gitShowObject} {
				got, err := read(nested, "HEAD", "./f")
				if (err == nil) != (wantErr == nil) || !bytes.Equal(got, wantBlob) {
					t.Errorf("%s configured relative blob=(%q,%v), native=(%q,%v)", label, got, err, wantBlob, wantErr)
				}
			}
		})
	}
}

func coreCaretParity(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("stored\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "f")
	runGit(t, root, "commit", "-qm", "first")
	first := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	tree := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD^{tree}")))
	second := strings.TrimSpace(string(parityGitBytes(t, root, "commit-tree", tree, "-p", first, "-m", "second")))
	third := strings.TrimSpace(string(parityGitBytes(t, root, "commit-tree", tree, "-p", first, "-m", "third")))
	for _, parents := range [][]string{{first, second}, {first, second, third}} {
		args := []string{"commit-tree", tree, "-m", "merge"}
		for _, p := range parents {
			args = append(args, "-p", p)
		}
		merge := strings.TrimSpace(string(parityGitBytes(t, root, args...)))
		runGit(t, root, "update-ref", "HEAD", merge)
		want, wantErr := historyGit(root, "rev-parse", "--verify", "HEAD^3^{commit}").Output()
		got, ok := (coverageRepo{root: root}).ResolveCommit("HEAD^3")
		if ok != (wantErr == nil) || got != strings.TrimSpace(string(want)) {
			t.Errorf("%d-parent caret=(%q,%v), native=(%q,%v)", len(parents), got, ok, want, wantErr)
		}
	}
}

func corePathParity(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0755); err != nil {
		t.Fatal(err)
	}
	for p, b := range map[string]string{"f": "root\n", "dir/inner": "nested\n"} {
		if err := os.WriteFile(filepath.Join(root, p), []byte(b), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "paths")
	for _, c := range []struct{ cwd, path string }{{root, "dir/"}, {root, "./dir/../f"}, {filepath.Join(root, "dir"), "../f"}, {filepath.Join(root, "dir"), "./../f"}, {root, ""}} {
		t.Run(c.path, func(t *testing.T) {
			want := parityGitBytes(t, c.cwd, "show", "HEAD:"+c.path)
			for label, read := range map[string]func(string, string, string) ([]byte, error){"ordinary": gitShowObject} {
				got, err := read(c.cwd, "HEAD", c.path)
				if err != nil || !bytes.Equal(got, want) {
					t.Errorf("%s path %q=(%q,%v), native=%q", label, c.path, got, err, want)
				}
			}
		})
	}
}

func coreShallowParity(t *testing.T) {
	for _, mode := range []string{"absent", "empty", "valid", "upper", "short", "invalid", "long"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			gitInit(t, root, "T", "t@example.invalid")
			runGit(t, root, "commit", "--allow-empty", "-qm", "initial")
			head := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
			payload := map[string]string{"empty": "", "valid": head + "\n", "upper": strings.ToUpper(head) + "\n", "short": "ab\n", "invalid": "not-a-hash\n", "long": head + "00\n"}[mode]
			if mode != "absent" {
				if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), []byte(payload), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, wantErr := exec.Command("git", "-C", root, "rev-parse", "--is-shallow-repository").Output()
			got, err := gitShallowOutput(root)
			if (err == nil) != (wantErr == nil) || !bytes.Equal(got, want) {
				t.Errorf("ordinary shallow=(%q,%v), native=(%q,%v)", got, err, want, wantErr)
			}
			shallow, ok := (coverageRepo{root: root}).IsShallow()
			if ok != (wantErr == nil) || shallow != (strings.TrimSpace(string(want)) != "false") {
				t.Errorf("typed shallow=(%v,%v), native=(%q,%v)", shallow, ok, want, wantErr)
			}
		})
	}
}

// coreNoGitReads proves successful upper adapters use the selected library.
// Capture native answers before poisoning PATH, then clear every relevant memo
// and open a fresh existing read owner. Exceptions are exercised separately.
func coreNoGitReads(t *testing.T, dir, head string) {
	t.Helper()
	wantHead := parityGitBytes(t, dir, "rev-parse", "HEAD")
	wantTop := strings.TrimSpace(string(parityGitBytes(t, dir, "rev-parse", "--show-toplevel")))
	wantPrefix := strings.TrimSpace(string(parityGitBytes(t, dir, "rev-parse", "--show-prefix")))
	wantShallow := parityGitBytes(t, dir, "rev-parse", "--is-shallow-repository")
	wantParents := strings.Fields(string(parityGitBytes(t, dir, "show", "-s", "--format=%P", head)))
	wantFiles := splitNUL(parityGitBytes(t, dir, "ls-tree", "-r", "-z", "--name-only", "--full-tree", head))
	var wantModes []evaluation.TreeEntry
	for _, e := range splitNUL(parityGitBytes(t, dir, "ls-tree", "-r", "-z", "--full-tree", head)) {
		meta, path, _ := strings.Cut(e, "\t")
		wantModes = append(wantModes, evaluation.TreeEntry{Mode: strings.Fields(meta)[0], Path: path})
	}
	wantAuthor, err := parseRawAuthor(string(parityGitBytes(t, dir, "--no-replace-objects", "cat-file", "commit", head)))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("go-only-cold-owner", func(t *testing.T) {
		repoCacheMu.Lock()
		clear(treeFilesCache)
		clear(treeModesCache)
		clear(rootPrefixCache)
		repoCacheMu.Unlock()
		rawCommitAuthors.Delete(head)
		end := beginGitReadSession()
		defer end()
		bin := t.TempDir()
		marker := filepath.Join(bin, "starts")
		quoted := "'" + strings.ReplaceAll(marker, "'", "'\"'\"'") + "'"
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\necho native-start >>"+quoted+"\nexit 91\n"), 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if got := gitCurrentSHA(dir); got != strings.TrimSpace(string(wantHead)) {
			t.Errorf("HEAD used native or differs: %q", got)
		}
		if got, err := gitHeadOutput(dir); err != nil || !bytes.Equal(got, wantHead) {
			t.Errorf("HEAD bytes %q %v", got, err)
		}
		if got, err := repoTopLevel(dir); err != nil || got != wantTop {
			t.Errorf("top %q %v", got, err)
		}
		if got, err := provenanceRepoTop(dir); err != nil || got != wantTop {
			t.Errorf("provenance top %q %v", got, err)
		}
		r := coverageRepo{root: dir}
		if !r.InRepository() {
			t.Error("repository probe used native")
		}
		if got, ok := r.RootPrefix(); !ok || got != wantPrefix {
			t.Errorf("prefix %q %v", got, ok)
		}
		if got, ok := r.ResolveCommit("HEAD"); !ok || got != head {
			t.Errorf("commit %q %v", got, ok)
		}
		if got, err := gitShallowOutput(dir); err != nil || !bytes.Equal(got, wantShallow) {
			t.Errorf("shallow %q %v", got, err)
		}
		if got, ok := r.IsShallow(); !ok || got != (strings.TrimSpace(string(wantShallow)) == "true") {
			t.Errorf("typed shallow %v %v", got, ok)
		}
		if got := r.TreeFiles(head); !reflect.DeepEqual(got, wantFiles) {
			t.Errorf("treefiles %q", got)
		}
		if got, ok := r.TreeModes(head); !ok || !reflect.DeepEqual(got, wantModes) {
			t.Errorf("treemodes %v %v", got, ok)
		}

		if got, err := provenanceParents(dir, head); err != nil || !reflect.DeepEqual(got, wantParents) {
			t.Errorf("parents %v %v", got, err)
		}

		if got, err := rawCommitAuthor(dir, head); err != nil || got != wantAuthor {
			t.Errorf("raw author %v %v", got, err)
		}

		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Errorf("supported successful adapter launched native Git: %v", err)
		}
	})
}

func coreShallowProfiles(t *testing.T) {
	fixture := func(t *testing.T) string {
		root := t.TempDir()
		gitInit(t, root, "T", "t@example.invalid")
		runGit(t, root, "commit", "--allow-empty", "-qm", "initial")
		return root
	}
	check := func(t *testing.T, root string) {
		t.Helper()
		want, wantErr := exec.Command("git", "-C", root, "rev-parse", "--is-shallow-repository").Output()
		got, err := gitShallowOutput(root)
		if (err == nil) != (wantErr == nil) || !bytes.Equal(got, want) {
			t.Errorf("shallow=(%q,%v), native=(%q,%v)", got, err, want, wantErr)
		}
		sh, ok := (coverageRepo{root: root}).IsShallow()
		if ok != (wantErr == nil) || sh != (strings.TrimSpace(string(want)) != "false") {
			t.Errorf("typed shallow=(%v,%v), native=(%q,%v)", sh, ok, want, wantErr)
		}
	}
	t.Run("same-owner-metadata-change", func(t *testing.T) {
		root := fixture(t)
		end := beginGitReadSession()
		defer end()
		check(t, root)
		file := filepath.Join(root, ".git", "shallow")
		if err := os.WriteFile(file, nil, 0644); err != nil {
			t.Fatal(err)
		}
		check(t, root)
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		check(t, root)
	})
	t.Run("linked-common-directory", func(t *testing.T) {
		root := fixture(t)
		linked := filepath.Join(t.TempDir(), "linked")
		runGit(t, root, "worktree", "add", "-q", "--detach", linked, "HEAD")
		head := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
		if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), []byte(head+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		check(t, linked)
		if read := gitCoreFor(linked); read.native == "" || read.err != nil {
			t.Fatalf("common shallow profile not selected: %+v", read)
		}
	})
	for _, mode := range []string{"valid", "empty-file", "invalid", "empty-override", "broken-link"} {
		t.Run(mode, func(t *testing.T) {
			root := fixture(t)
			file := filepath.Join(t.TempDir(), "override")
			payload := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD"))) + "\n"
			if mode == "invalid" {
				payload = "ab\n"
			}
			if mode == "empty-file" {
				payload = ""
			}
			if mode == "broken-link" {
				if err := os.Symlink(file, filepath.Join(root, ".git", "shallow")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(file, []byte(payload), 0644); err != nil {
					t.Fatal(err)
				}
				if mode == "empty-override" {
					file = ""
				}
				t.Setenv("GIT_SHALLOW_FILE", file)
			}
			check(t, root)
			if read := gitCoreFor(root); read.native == "" || read.err != nil {
				t.Fatalf("explicit shallow profile not selected: %+v", read)
			}
		})
	}
}

func coreContextParity(t *testing.T) {
	t.Run("symlink-into-other-repo-subdirectory", func(t *testing.T) {
		a, b := t.TempDir(), t.TempDir()
		for root, body := range map[string]string{a: "A\n", b: "B\n"} {
			gitInit(t, root, "T", "t@example.invalid")
			if err := os.MkdirAll(filepath.Join(root, "inside"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "inside", "f"), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			runGit(t, root, "add", "-A")
			runGit(t, root, "commit", "-qm", "context")
		}
		link := filepath.Join(a, "link")
		if err := os.Symlink(filepath.Join(b, "inside"), link); err != nil {
			t.Fatal(err)
		}
		want := strings.TrimSpace(string(parityGitBytes(t, link, "rev-parse", "--show-toplevel")))
		if got, err := repoTopLevel(link); err != nil || got != want {
			t.Errorf("ordinary symlink root=(%q,%v), native=%q", got, err, want)
		}
		if got, err := provenanceRepoTop(link); err != nil || got != want {
			t.Errorf("immutable symlink root=(%q,%v), native=%q", got, err, want)
		}
		head := strings.TrimSpace(string(parityGitBytes(t, link, "rev-parse", "HEAD")))
		if got := gitCurrentSHA(link); got != head {
			t.Errorf("symlink HEAD %s != native %s", got, head)
		}
		prefix := strings.TrimSpace(string(parityGitBytes(t, link, "rev-parse", "--show-prefix")))
		if got, ok := (coverageRepo{root: link}).RootPrefix(); !ok || got != prefix {
			t.Errorf("symlink prefix=(%q,%v), native=%q", got, ok, prefix)
		}
	})
	for _, kind := range []string{"file", "fifo", "missing"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			gitInit(t, root, "T", "t@example.invalid")
			runGit(t, root, "commit", "--allow-empty", "-qm", "context")
			path := filepath.Join(root, "context")
			switch kind {
			case "file":
				if err := os.WriteFile(path, []byte("file\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				tool, err := exec.LookPath("mkfifo")
				if err != nil {
					t.Skip("could-not-check FIFO fixture: mkfifo unavailable")
				}
				if err := exec.Command(tool, path).Run(); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").Output(); err == nil {
				t.Fatalf("native non-directory context unexpectedly succeeds: %q", out)
			}
			if got, err := repoTopLevel(path); err == nil {
				t.Errorf("non-directory ordinary context enrolled parent %q", got)
			}
			if got, err := provenanceRepoTop(path); err == nil {
				t.Errorf("non-directory immutable context enrolled parent %q", got)
			}
			if (coverageRepo{root: path}).InRepository() {
				t.Error("non-directory context enrolled as repository")
			}
		})
	}
}

func coreRefNamespaceParity(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("stored\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "f")
	runGit(t, root, "commit", "-qm", "namespace")
	head := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	alias := strings.Repeat("e", 40)
	runGit(t, root, "update-ref", "refs/refs/heads/missing", head)
	runGit(t, root, "update-ref", "refs/heads/"+alias, head)
	for _, token := range []string{"refs/heads/missing", alias} {
		t.Run(token, func(t *testing.T) {
			out, err := historyGit(root, "rev-parse", "--verify", "--quiet", "--end-of-options", token+"^{commit}").Output()
			got, ok := (coverageRepo{root: root}).ResolveCommit(token)
			if ok != (err == nil) || got != strings.TrimSpace(string(out)) {
				t.Errorf("commit selector=(%q,%v), native=(%q,%v)", got, ok, out, err)
			}
			native, nativeErr := historyGit(root, "ls-tree", "-r", "-z", "--name-only", "--full-tree", "--end-of-options", token).Output()
			names := (coverageRepo{root: root}).TreeFiles(token)
			if nativeErr == nil && !reflect.DeepEqual(names, splitNUL(native)) {
				t.Errorf("tree selector=%q, native=%q", names, splitNUL(native))
			}
		})
	}
}

func coreEmptyEnvParity(t *testing.T) {
	for _, key := range historyRepoEnv {
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			gitInit(t, root, "T", "t@example.invalid")
			runGit(t, root, "commit", "--allow-empty", "-qm", "empty env")
			head := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
			end := beginGitReadSession()
			defer end()
			if _, err := gitHeadOutput(root); err != nil {
				t.Fatal(err)
			}
			t.Setenv(key, "")
			want, wantErr := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
			got, err := gitHeadOutput(root)
			if (err == nil) != (wantErr == nil) || !bytes.Equal(got, want) {
				t.Errorf("ordinary empty %s=(%q,%v), native=(%q,%v)", key, got, err, want, wantErr)
			}
			if read := ordinaryGitCore(root); read.native == "" {
				t.Error("present empty ordinary locator not preselected native")
			}
			if got, ok := (coverageRepo{root: root}).ResolveCommit("HEAD"); !ok || got != head {
				t.Errorf("provenance empty locator wasn't historically stripped: %q %v", got, ok)
			}
		})
	}
	for _, key := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM"} {
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			gitInit(t, root, "T", "t@example.invalid")
			runGit(t, root, "commit", "--allow-empty", "-qm", "empty profile")
			end := beginGitReadSession()
			defer end()
			if _, err := repoTopLevel(root); err != nil {
				t.Fatal(err)
			}
			t.Setenv(key, "")
			want, wantErr := historyGit(root, "rev-parse", "--show-toplevel").Output()
			got, err := provenanceRepoTop(root)
			if (err == nil) != (wantErr == nil) || got != strings.TrimSpace(string(want)) {
				t.Errorf("empty %s=(%q,%v), native=(%q,%v)", key, got, err, want, wantErr)
			}
			if read := gitCoreFor(root); read.native == "" {
				t.Error("unstripped empty native profile not freshly selected")
			}
		})
	}
}
