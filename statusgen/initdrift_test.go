package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The stale-embed class (#5004). The GitLab template carries pushcred.sh and
// boardadd.sh embedded verbatim as script anchors, and init never overwrites
// an existing file. An adopter who scaffolded with an older statusgen keeps
// running that older embedded copy after every upgrade, so a board-push fix
// shipped in pushcred.sh never reaches their pipeline, and a re-run of init
// said only "exists ... (unchanged)".

// TestInitGitlabStaleAnchorNamed: a re-run of init over a .gitlab-ci.yml whose
// embedded anchor differs from this statusgen's copy names the stale anchor and
// the way to refresh it, and still leaves the file untouched.
//
// regression: #5004
func TestInitGitlabStaleAnchorNamed(t *testing.T) {
	dir := t.TempDir()
	if code := runInitForge(dir, forgeGitLab, true, "", false); code != 0 {
		t.Fatalf("first init exit = %d", code)
	}
	ciPath := filepath.Join(dir, ".gitlab-ci.yml")
	cur, err := os.ReadFile(ciPath)
	if err != nil {
		t.Fatal(err)
	}
	old := gitlabScriptAnchor("statusgen-pushcred", "pushcred_push() {\n  git push \"$@\"\n}\n", "# an older embedded copy\n")
	stale := strings.Replace(string(cur), gitlabPushcredAnchor(), old, 1)
	if stale == string(cur) {
		t.Fatal("test setup: the scaffolded file does not carry the current pushcred anchor")
	}
	if err := os.WriteFile(ciPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	var code int
	out := captureStdout(t, func() { code = runInitForge(dir, forgeGitLab, true, "", false) })
	if code != 0 {
		t.Fatalf("re-run exit = %d, want 0:\n%s", code, out)
	}
	if !strings.Contains(out, "STALE") || !strings.Contains(out, ".statusgen-pushcred") {
		t.Errorf("re-run did not name the stale .statusgen-pushcred anchor:\n%s", out)
	}
	if strings.Contains(out, ".statusgen-boardadd") {
		t.Errorf("re-run named the current .statusgen-boardadd anchor as stale:\n%s", out)
	}
	if !strings.Contains(out, "statusgen migrate gitlab-anchors --root .") {
		t.Errorf("stale notice does not say how to get the current anchor:\n%s", out)
	}
	if got, _ := os.ReadFile(ciPath); string(got) != stale {
		t.Error("init rewrote an existing .gitlab-ci.yml; it must never overwrite")
	}
}

// staleGitlabScaffold scaffolds a GitLab tree and swaps its .statusgen-pushcred
// anchor block for an older one, keeping the comment above it, as a file
// scaffolded by an earlier release reads. It returns the root, the current
// scaffold body and the stale body now on disk.
func staleGitlabScaffold(t *testing.T) (dir, cur, stale string) {
	t.Helper()
	dir = t.TempDir()
	_ = captureStdout(t, func() {
		if code := runInitForge(dir, forgeGitLab, true, "", false); code != 0 {
			t.Fatalf("init exit = %d", code)
		}
	})
	b, err := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	cur = string(b)
	want := gitlabAnchorWant("statusgen-pushcred", pushcredSh)
	old := ".statusgen-pushcred: &statusgen-pushcred\n  - |\n    pushcred_push() {\n      git push \"$@\"\n    }"
	stale = strings.Replace(cur, want, old, 1)
	if stale == cur {
		t.Fatal("test setup: the scaffold does not carry the current pushcred anchor")
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitlab-ci.yml"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, cur, stale
}

// TestMigrateAnchorsRefresh: `statusgen migrate gitlab-anchors`
// previews under --dry-run without writing, then rewrites the stale anchor so
// the file is byte-identical to a fresh scaffold, and a second run is a no-op.
//
// regression: #5004
func TestMigrateAnchorsRefresh(t *testing.T) {
	for _, crlf := range []bool{false, true} {
		name := "lf"
		if crlf {
			name = "crlf"
		}
		t.Run(name, func(t *testing.T) {
			dir, cur, stale := staleGitlabScaffold(t)
			ciPath := filepath.Join(dir, ".gitlab-ci.yml")
			if crlf {
				cur = strings.ReplaceAll(cur, "\n", "\r\n")
				stale = strings.ReplaceAll(stale, "\n", "\r\n")
				if err := os.WriteFile(ciPath, []byte(stale), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errb bytes.Buffer
			if code := runMigrate([]string{"gitlab-anchors", "--root", dir, "--dry-run"}, &out, &errb); code != 0 {
				t.Fatalf("dry-run exit %d: %s%s", code, out.String(), errb.String())
			}
			if !strings.Contains(out.String(), ".statusgen-pushcred: WOULD refresh") || !strings.Contains(out.String(), ".statusgen-boardadd: current") {
				t.Errorf("dry-run plan:\n%s", out.String())
			}
			if got, _ := os.ReadFile(ciPath); string(got) != stale {
				t.Fatal("--dry-run wrote the file")
			}
			out.Reset()
			if code := runMigrate([]string{"gitlab-anchors", "--root", dir}, &out, &errb); code != 0 {
				t.Fatalf("apply exit %d: %s%s", code, out.String(), errb.String())
			}
			if got, _ := os.ReadFile(ciPath); string(got) != cur {
				t.Errorf("refreshed file differs from a fresh scaffold:\n%s", got)
			}
			out.Reset()
			if code := runMigrate([]string{"gitlab-anchors", "--root", dir}, &out, &errb); code != 0 || !strings.Contains(out.String(), "nothing to change") {
				t.Errorf("second run exit %d, want a no-op:\n%s", code, out.String())
			}
		})
	}
}

// TestMigrateAnchorsKeepRest: only the anchor block changes; an
// adopter's own comment above it and a job after it survive, and a tree with
// no .gitlab-ci.yml is a clean no-op.
func TestMigrateAnchorsKeepRest(t *testing.T) {
	dir, _, stale := staleGitlabScaffold(t)
	ciPath := filepath.Join(dir, ".gitlab-ci.yml")
	edited := strings.Replace(stale, ".statusgen-pushcred: &", "# adopter note\n.statusgen-pushcred: &", 1) + "\nmy-job:\n  script:\n    - echo mine\n"
	if err := os.WriteFile(ciPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runMigrate([]string{"gitlab-anchors", "--root", dir}, &out, &errb); code != 0 {
		t.Fatalf("apply exit %d: %s%s", code, out.String(), errb.String())
	}
	got, _ := os.ReadFile(ciPath)
	for _, keep := range []string{"# adopter note\n.statusgen-pushcred: &", "my-job:\n  script:\n    - echo mine\n"} {
		if !strings.Contains(string(got), keep) {
			t.Errorf("refresh dropped %q", keep)
		}
	}
	if d := gitlabAnchorDrift(string(got)); len(d) != 0 {
		t.Errorf("still drifted after refresh: %v", d)
	}

	out.Reset()
	if code := runMigrate([]string{"gitlab-anchors", "--root", t.TempDir()}, &out, &errb); code != 0 || !strings.Contains(out.String(), "nothing to refresh") {
		t.Errorf("no-file run exit %d:\n%s", code, out.String())
	}
}

// TestLintGitlabAnchorDrift: statusgen --lint names a stale embedded anchor
// with the refresh command, and says nothing once it is refreshed.
//
// regression: #5004
func TestLintGitlabAnchorDrift(t *testing.T) {
	dir, cur, _ := staleGitlabScaffold(t)
	lint := func() string {
		var errs string
		out := captureStdout(t, func() {
			errs = captureStderr(t, func() { _ = run(dir, "lint", nil, nil, "") })
		})
		return out + errs
	}
	const want = ".gitlab-ci.yml: the embedded .statusgen-pushcred anchor differs"
	if got := lint(); !strings.Contains(got, want) || !strings.Contains(got, "statusgen migrate gitlab-anchors --root .") {
		t.Errorf("lint did not name the stale anchor and its refresh command:\n%s", got)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitlab-ci.yml"), []byte(cur), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lint(); strings.Contains(got, "anchor differs") {
		t.Errorf("lint reported drift on a current scaffold:\n%s", got)
	}
}

// TestInitGitlabAnchorQuiet: a re-run over a current scaffold, and over
// a file with no embedded anchors at all, says nothing about drift.
func TestInitGitlabAnchorQuiet(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"current", ""},
		{"no-anchors", "stages: [build]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if code := runInitForge(dir, forgeGitLab, true, "", false); code != 0 {
				t.Fatalf("first init exit = %d", code)
			}
			if tc.body != "" {
				if err := os.WriteFile(filepath.Join(dir, ".gitlab-ci.yml"), []byte(tc.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out := captureStdout(t, func() { runInitForge(dir, forgeGitLab, true, "", false) })
			if strings.Contains(out, "STALE") {
				t.Errorf("drift reported for a %s file:\n%s", tc.name, out)
			}
		})
	}
}
