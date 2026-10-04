package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// verifyflip_hist_test.go — the provenance layers' git reads: every merge
// base judged, an ambiguous commit identity read as could-not-check, and
// reads that runner-local state cannot steer.

// vfGitAt runs git in root as mail with author and committer dates pinned.
func vfGitAt(t *testing.T, root, mail, date string, args ...string) {
	t.Helper()
	name, _, _ := strings.Cut(mail, "@")
	runGitEnv(t, root, []string{"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + mail,
		"GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + mail,
		"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date},
		append([]string{"-c", "commit.gpgsign=false"}, args...)...)
}

// vfRev resolves rev in root.
func vfRev(t *testing.T, root, rev string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", rev).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", rev, err)
	}
	return strings.TrimSpace(string(out))
}

// TestVflipEveryMergeBase (criss-cross): two branches from the PASS, one
// carrying the verifier's later FAIL; a human merges them dropping the FAIL
// (X), the verifier merges them keeping it (Y), and another App merges Y into
// X keeping X's Evidence (M). X and Y have TWO merge bases, and they disagree
// about Y's Evidence: one holds the FAIL, one does not. No single base then
// shows Y's side made no net change, so M is judged. Each case puts the FAIL
// on a different base, so a check of only the first base git lists is red in
// one of them whichever order git lists them in.
func TestVflipEveryMergeBase(t *testing.T) {
	for _, c := range []struct {
		name, merger string
		failFirst    bool
		code         int
		want         string
	}{
		{"fail-on-older-base", vfMailWorker, true, verifyflipExitRefused, "provenance"},
		{"fail-on-newer-base", vfMailWorker, false, verifyflipExitRefused, "provenance"},
		{"human-merger", vfMailHuman, true, verifyflipExitOK, "flipped"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, readme := vfFixture(t, vfDefaults())
			pass := vfRev(t, root, "HEAD")
			side := func(branch, date string, fail bool) {
				runGitEnv(t, root, nil, "checkout", "-q", "-b", branch, pass)
				if fail {
					vfAppend(t, root, "\n\n"+vfLaterFail)
				} else if err := os.WriteFile(filepath.Join(root, branch+".txt"), []byte(branch+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				vfGitAt(t, root, vfMailVerifier, date, "add", "-A")
				vfGitAt(t, root, vfMailVerifier, date, "commit", "-q", "-m", branch)
			}
			side("a", "2030-01-01T10:00:00Z", c.failFirst)
			a1 := vfRev(t, root, "HEAD")
			side("b", "2030-01-01T11:00:00Z", !c.failFirst)

			runGitEnv(t, root, nil, "checkout", "-q", "a")
			vfGitAt(t, root, vfMailHuman, "2030-01-01T12:00:00Z", "merge", "-q", "--no-commit", "--no-ff", "b")
			runGitEnv(t, root, nil, "checkout", pass, "--", vfBriefRel)
			vfGitAt(t, root, vfMailHuman, "2030-01-01T12:00:00Z", "commit", "-q", "-m", "x")

			runGitEnv(t, root, nil, "checkout", "-q", "b")
			vfGitAt(t, root, vfMailVerifier, "2030-01-01T13:00:00Z", "merge", "-q", "--no-ff", "--no-edit", a1)

			runGitEnv(t, root, nil, "checkout", "-q", "a")
			vfGitAt(t, root, c.merger, "2030-01-01T14:00:00Z", "merge", "-q", "-s", "ours", "--no-edit", "b")
			runGitEnv(t, root, nil, "checkout", "-q", "main")
			runGitEnv(t, root, nil, "merge", "-q", "--ff-only", "a")

			if out, err := exec.Command("git", "-C", root, "merge-base", "--all", "HEAD^1", "HEAD^2").Output(); err != nil || len(strings.Fields(string(out))) != 2 {
				t.Fatalf("positive control: the final merge's parents must have two merge bases; got %q (err %v)", out, err)
			}
			vfExpect(t, root, readme, c.code, c.want)
		})
	}
}

// TestFlipMergeBasesObjectIDs: the allow-listed merge-base caller refuses any
// operand that is not a commit object id, so a name never reaches git.
func TestFlipMergeBasesObjectIDs(t *testing.T) {
	root, _ := vfFixture(t, vfDefaults())
	head := vfRev(t, root, "HEAD")
	if got, err := flipMergeBases(root, head, head); err != nil || len(got) != 1 || got[0] != head {
		t.Fatalf("positive control: merge-base of HEAD with itself = %v (err %v), want [%s]", got, err, head)
	}
	for _, bad := range [][2]string{{"main", head}, {head, "HEAD"}, {"--output=x", head}, {head[:12], head}} {
		if got, err := flipMergeBases(root, bad[0], bad[1]); err == nil {
			t.Errorf("flipMergeBases(%q, %q) = %v, want a refusal of a non-object-id operand", bad[0], bad[1], got)
		}
	}
}

// TestParseRawAuthor: a commit identity any other reader could see
// differently is unreadable, never a verdict.
func TestParseRawAuthor(t *testing.T) {
	const tz = " 1786000000 +0000"
	hdr := func(lines ...string) string {
		return "tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n" + strings.Join(lines, "\n") + "\n\nmsg\nauthor fake <x@y> 1 +0000\n"
	}
	ok := "author Ada <" + vfMailHuman + ">" + tz
	if a, err := parseRawAuthor(hdr(ok, "committer Ada <"+vfMailHuman+">"+tz)); err != nil || a.Name != "Ada" || a.Email != vfMailHuman {
		t.Fatalf("clean header: %+v, %v", a, err)
	}
	if _, err := parseRawAuthor(hdr(ok, "committer Ada <"+vfMailHuman+">"+tz, "encoding UTF-8")); err != nil {
		t.Fatalf("a UTF-8 encoding header is unambiguous: %v", err)
	}
	for name, obj := range map[string]string{
		"encoding":        hdr(ok, "committer Ada <"+vfMailHuman+">"+tz, "encoding ISO-8859-1"),
		"two-authors":     hdr(ok, "committer Ada <"+vfMailHuman+">"+tz, "author Bob <"+vfMailWorker+">"+tz),
		"spaced-address":  hdr("author Ada < "+vfMailHuman+">"+tz, "committer Ada <"+vfMailHuman+">"+tz),
		"padded-name":     hdr("author  Ada <"+vfMailHuman+">"+tz, "committer Ada <"+vfMailHuman+">"+tz),
		"control":         hdr("author Ada\x01 <"+vfMailHuman+">"+tz, "committer Ada <"+vfMailHuman+">"+tz),
		"invalid-utf8":    hdr("author Ada\xff <"+vfMailHuman+">"+tz, "committer Ada <"+vfMailHuman+">"+tz),
		"no-zone":         hdr("author Ada <"+vfMailHuman+"> 1786000000", "committer Ada <"+vfMailHuman+">"+tz),
		"extra-bracket":   hdr("author Ada <x> <"+vfMailHuman+">"+tz, "committer Ada <"+vfMailHuman+">"+tz),
		"no-author":       hdr("committer Ada <" + vfMailHuman + ">" + tz),
		"empty-name-part": hdr("author <"+vfMailHuman+">"+tz, "committer Ada <"+vfMailHuman+">"+tz),
	} {
		if a, err := parseRawAuthor(obj); err == nil {
			t.Errorf("%s: read %+v, want an unreadable (ambiguous) author", name, a)
		}
	}
}

// vfRewriteHead writes a copy of HEAD's commit object with edit applied and
// returns its id. No ref moves.
func vfRewriteHead(t *testing.T, root string, edit func(string) string) string {
	t.Helper()
	raw, err := exec.Command("git", "-C", root, "cat-file", "commit", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "hash-object", "-t", "commit", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(edit(string(raw)))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hash-object: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestVflipAmbiguousAuthor: the verifier's PASS commit, rewritten so the
// author address carries a leading space. git keeps that byte (git log prints
// it), so trimming it would read the verifier where git log reads another
// address. The flip is could-not-check, never a pass.
func TestVflipAmbiguousAuthor(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	id := vfRewriteHead(t, root, func(s string) string {
		return strings.Replace(s, "<"+vfMailVerifier+">", "< "+vfMailVerifier+">", 1)
	})
	runGitEnv(t, root, nil, "update-ref", "refs/heads/main", id)
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%ae", "HEAD").Output()
	if err != nil || string(out) != " "+vfMailVerifier+"\n" {
		t.Fatalf("positive control: git log reads %q (err %v), want the address with its leading space", out, err)
	}
	vfExpect(t, root, readme, verifyflipExitCouldNotCheck, "ambiguous")
}

// TestVflipReplaceObject: another App wrote the PASS, then a local
// refs/replace/ entry substitutes a copy of that commit naming the verifier.
// The provenance reads the object store's own commit.
func TestVflipReplaceObject(t *testing.T) {
	o := vfDefaults()
	o.author = vfMailWorker
	root, readme := vfFixture(t, o)
	head := vfRev(t, root, "HEAD")
	wname, _, _ := strings.Cut(vfMailWorker, "@")
	vname, _, _ := strings.Cut(vfMailVerifier, "@")
	id := vfRewriteHead(t, root, func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, vfMailWorker, vfMailVerifier), wname, vname)
	})
	runGitEnv(t, root, nil, "replace", head, id)
	if out, _ := exec.Command("git", "-C", root, "cat-file", "commit", head).Output(); !strings.Contains(string(out), vfMailVerifier) {
		t.Fatalf("positive control: the replacement must be in effect for a plain read; got\n%s", out)
	}
	if a, err := rawCommitAuthor(root, head); err != nil || a.Email != vfMailWorker {
		t.Errorf("rawCommitAuthor = %+v (err %v), want the object store's own author %s", a, err, vfMailWorker)
	}
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipIgnoreRevsConfig: another App edits a verifier line; user-level
// configuration then tells blame to skip that commit, which hands the line
// back to the verifier's commit. The blame layers (and the Evidence-actor
// lint, which shares the read) must still name the editor.
func TestVflipIgnoreRevsConfig(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	raw, err := os.ReadFile(vfBrief(root))
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(vfRow2, "pass exit=1", "pass exit=1 (re-run)", 1)
	if err := os.WriteFile(vfBrief(root), []byte(strings.Replace(string(raw), vfRow2, edited, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	tmp := t.TempDir()
	revs, cfg := filepath.Join(tmp, "revs"), filepath.Join(tmp, "gitconfig")
	if err := os.WriteFile(revs, []byte(vfRev(t, root, "HEAD")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("[blame]\n\tignoreRevsFile = "+filepath.ToSlash(revs)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	line := 0
	for i, l := range strings.Split(mustRead(t, vfBrief(root)), "\n") {
		if l == edited {
			line = i + 1
		}
	}
	if line == 0 {
		t.Fatal("edited row not found")
	}
	L := strconv.Itoa(line) + "," + strconv.Itoa(line)
	out, err := exec.Command("git", "-C", root, "blame", "--line-porcelain", "-L", L, "--", vfBriefRel).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := blamePorcelainLines(string(out)); len(got) != 1 || got[0].Commit != vfRev(t, root, "HEAD^") {
		t.Fatalf("positive control: the configuration must hand the edited line to the verifier's commit; got %+v", got)
	}
	authors, _, err := blameEvidenceAuthors(root, vfBriefRel, line, line)
	if err != nil || len(authors) != 1 || authors[0].Email != vfMailWorker {
		t.Errorf("blameEvidenceAuthors = %+v (err %v), want the editor %s", authors, err, vfMailWorker)
	}
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// historyGitLeaks lists every git command in file (restricted to the named
// functions when only is non-nil) that is not isolated from runner-local
// state: an exec.Command that is not the direct argument of historyGitEnv, or
// one that is but whose literal arguments are not "git" + historyGitPrefix.
// historyGit itself (which builds from historyGitPrefix) is exempt.
func historyGitLeaks(t *testing.T, fset *token.FileSet, file string, src any, only map[string]bool) []string {
	t.Helper()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	isExec := func(n ast.Node) (*ast.CallExpr, bool) {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return nil, false
		}
		s, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || s.Sel.Name != "Command" {
			return nil, false
		}
		x, ok := s.X.(*ast.Ident)
		return c, ok && x.Name == "exec"
	}
	prefix := append([]string{"git"}, historyGitPrefix("")...)
	prefix = prefix[:len(prefix)-1] // the directory is not a literal
	var leaks []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name == "historyGit" || (only != nil && !only[fn.Name.Name]) {
			continue
		}
		wrapped := map[ast.Node]bool{}
		ast.Inspect(fn, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "historyGitEnv" && len(c.Args) == 1 {
					wrapped[c.Args[0]] = true
				}
			}
			return true
		})
		ast.Inspect(fn, func(n ast.Node) bool {
			c, ok := isExec(n)
			if !ok {
				return true
			}
			at := fset.Position(c.Pos()).String()
			if !wrapped[c] {
				leaks = append(leaks, at+": "+fn.Name.Name+" runs a command outside historyGit")
				return true
			}
			good := len(c.Args) > len(prefix)
			for i := 0; good && i < len(prefix); i++ {
				lit, ok := c.Args[i].(*ast.BasicLit)
				s, err := strconv.Unquote(litValue(lit, ok))
				good = ok && err == nil && s == prefix[i]
			}
			if !good {
				leaks = append(leaks, at+": "+fn.Name.Name+" spells a git read without historyGitPrefix")
			}
			return true
		})
	}
	return leaks
}

func litValue(lit *ast.BasicLit, ok bool) string {
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	return lit.Value
}

// TestHistoryGitIsolated is the class guard for provenance reads steered by
// runner-local state: every git command the flip runs (verifyflip.go), and
// the commit-identity and blame reads it shares with the Evidence-actor lint,
// go through historyGit — or, where the merge-base choke-point guard needs the
// call spelled out, through historyGitEnv with historyGitPrefix's literals.
// The planted sources are the positive control.
func TestHistoryGitIsolated(t *testing.T) {
	fset := token.NewFileSet()
	for _, plant := range []string{
		"package p\nimport \"os/exec\"\nfunc planted(d string) { exec.Command(\"git\", \"-C\", d, \"log\").Run() }\n",
		"package p\nimport \"os/exec\"\nfunc planted(d string) { historyGitEnv(exec.Command(\"git\", \"-C\", d, \"merge-base\")).Run() }\n",
	} {
		if got := historyGitLeaks(t, fset, "plant.go", plant, nil); len(got) != 1 {
			t.Fatalf("positive control: leaks = %v, want exactly one for %q", got, plant)
		}
	}
	for _, leak := range historyGitLeaks(t, fset, "verifyflip.go", nil, nil) {
		t.Error(leak)
	}
	for _, leak := range historyGitLeaks(t, fset, "evidenceactor.go", nil,
		map[string]bool{"rawCommitAuthor": true, "parseRawAuthor": true, "blameEvidenceAuthors": true}) {
		t.Error(leak)
	}
}
