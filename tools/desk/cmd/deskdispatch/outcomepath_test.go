package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// outcomeDirLines returns the backticked directory of every "Outcome directory" line.
func outcomeDirLines(t *testing.T, prompt string) []string {
	t.Helper()
	var dirs []string
	for _, line := range strings.Split(prompt, "\n") {
		const lead = "Outcome directory (dispatcher-named, absolute, outside the home): `"
		if i := strings.Index(line, lead); i >= 0 {
			rest := line[i+len(lead):]
			if j := strings.Index(rest, "`"); j >= 0 {
				dirs = append(dirs, rest[:j])
			}
		}
	}
	return dirs
}

// regression: F-outcome-record-inside-home
//
// (A verifier outcome record written inside the home refuses verifier admission.)
//
// TestVerifierOutcomePath: the verifier prompt names ONE absolute outcome directory outside
// the home and the dispatcher root, with the exact `deskevidence --outcome-record … --root`
// landing command; an allocation that resolves inside the home or the root (directly or
// through a symlink alias) is refused, never moved elsewhere; the cache form is owner-only
// and week-pruned; and no other kit's prompt carries the line.
func TestVerifierOutcomePath(t *testing.T) {
	t.Run("verifier prompt names one absolute outside path and the landing command", func(t *testing.T) {
		home, root, cache := t.TempDir(), t.TempDir(), t.TempDir()
		old := packetCacheDir
		packetCacheDir = func() (string, error) { return cache, nil }
		t.Cleanup(func() { packetCacheDir = old })
		o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, brief: filepath.Join(root, vpBriefRel)}
		plan := dispatchPlan{repo: "example/repo", claimKey: "repo--example-stream--07"}
		dir, err := allocateOutcomeDir(o, plan, home)
		if err != nil {
			t.Fatalf("allocateOutcomeDir: %v", err)
		}
		plan.outcomePath = dir
		prompt, err := assemblePrompt(o, plan, home)
		if err != nil {
			t.Fatal(err)
		}
		got := outcomeDirLines(t, prompt)
		if len(got) != 1 {
			t.Fatalf("outcome directory lines = %q, want exactly one:\n%s", got, prompt)
		}
		if !filepath.IsAbs(got[0]) || vpWithin(home, got[0]) || vpWithin(root, got[0]) {
			t.Errorf("outcome directory %q is not an absolute path outside home %s and root %s", got[0], home, root)
		}
		cmd := "deskevidence example/repo main --outcome-record " + filepath.Join(dir, "<file>.json") + " --root " + home
		if strings.Count(prompt, cmd) != 1 {
			t.Errorf("prompt lacks the exact landing command %q:\n%s", cmd, prompt)
		}
		if !strings.Contains(prompt, "never write the record inside the home") {
			t.Error("prompt does not say never to write the record inside the home")
		}
		if !strings.Contains(prompt, "an earlier run's record may already be there") {
			t.Error("prompt does not say the per-claim directory may hold an earlier run's record")
		}
	})

	t.Run("dry-run verifier prompt carries the line beside --prompt-file", func(t *testing.T) {
		asg := assignmentSection(t, dispatchPrompt(t, "verifier", "verify-item-9"))
		got := outcomeDirLines(t, asg)
		if len(got) != 1 || !filepath.IsAbs(got[0]) || filepath.Base(filepath.Dir(got[0])) != outcomeDirName ||
			filepath.Base(got[0]) != "assay--verify-item-9" {
			t.Fatalf("dry-run verifier outcome lines = %q, want one absolute <prompt dir>/%s/<claim key>:\n%s", got, outcomeDirName, asg)
		}
		if !strings.Contains(asg, "--outcome-record ") || !strings.Contains(asg, " --root ") {
			t.Errorf("dry-run verifier assignment lacks the --outcome-record … --root command:\n%s", asg)
		}
		if _, err := os.Stat(got[0]); !os.IsNotExist(err) {
			t.Errorf("a dry run created the outcome directory %s (stat err %v); it must touch nothing", got[0], err)
		}
	})

	t.Run("allocation inside the home or the root is refused, never moved", func(t *testing.T) {
		home, root := t.TempDir(), t.TempDir()
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(home, alias); err != nil {
			if runtime.GOOS == "windows" {
				t.Skip("symlinks unavailable")
			}
			t.Fatal(err)
		}
		cases := map[string]string{
			"prompt file in the home":            filepath.Join(home, "prompt.md"),
			"prompt file deep in the home":       filepath.Join(home, "docs", "streams", "prompt.md"),
			"prompt file in the root":            filepath.Join(root, "prompt.md"),
			"prompt file through a home symlink": filepath.Join(alias, "prompt.md"),
		}
		for name, pf := range cases {
			t.Run(name, func(t *testing.T) {
				o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, promptFile: pf}
				plan := dispatchPlan{repo: "example/repo", claimKey: "repo--example-stream--07"}
				dir, err := allocateOutcomeDir(o, plan, home)
				if err == nil || !deskkit.IsRefused(err) {
					t.Fatalf("allocateOutcomeDir(%s) = %q, %v; want a refusal", pf, dir, err)
				}
				if !strings.Contains(err.Error(), "OUTSIDE") {
					t.Errorf("refusal does not name the outside-the-home rule: %v", err)
				}
				if _, serr := os.Lstat(filepath.Join(filepath.Dir(pf), outcomeDirName)); !os.IsNotExist(serr) {
					t.Errorf("a refused allocation still created a directory (stat err %v)", serr)
				}
			})
		}
		t.Run("cache directory inside the home", func(t *testing.T) {
			old := packetCacheDir
			packetCacheDir = func() (string, error) { return filepath.Join(alias, ".cache"), nil }
			t.Cleanup(func() { packetCacheDir = old })
			o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root}
			if _, err := allocateOutcomeDir(o, dispatchPlan{claimKey: "repo--example-stream--07"}, home); err == nil || !deskkit.IsRefused(err) {
				t.Fatalf("cache allocation inside the home: err = %v, want a refusal", err)
			}
			if _, err := os.Stat(filepath.Join(home, ".cache")); !os.IsNotExist(err) {
				t.Errorf("a refused cache allocation wrote into the home (stat err %v)", err)
			}
		})
		t.Run("dry run refuses the same way", func(t *testing.T) {
			o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, promptFile: filepath.Join(root, "p.md")}
			if _, err := outcomeDirFor(o, dispatchPlan{claimKey: "repo--example-stream--07"}, ""); err == nil || !deskkit.IsRefused(err) {
				t.Fatalf("outcomeDirFor with the prompt in the root and no home yet: err = %v, want a refusal", err)
			}
		})
	})

	t.Run("cache form is owner-only, per claim key and week-pruned", func(t *testing.T) {
		cache := t.TempDir()
		old, oldNow := packetCacheDir, packetNow
		now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
		packetCacheDir = func() (string, error) { return cache, nil }
		packetNow = func() time.Time { return now }
		t.Cleanup(func() { packetCacheDir, packetNow = old, oldNow })
		base := filepath.Join(cache, "assay", "outcomes")
		stale, fresh, file := filepath.Join(base, "stale-key"), filepath.Join(base, "fresh-key"), filepath.Join(base, "stray.json")
		for _, d := range []string{stale, fresh} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{stale, file} {
			if err := os.Chtimes(p, now.Add(-8*24*time.Hour), now.Add(-8*24*time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(fresh, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		home, root := t.TempDir(), t.TempDir()
		o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root}
		dir, err := allocateOutcomeDir(o, dispatchPlan{claimKey: "repo--example-stream--07"}, home)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(base, "repo--example-stream--07"); dir != want {
			t.Errorf("cache outcome dir = %s, want %s", dir, want)
		}
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			t.Fatalf("outcome dir not created: %v", err)
		}
		if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700 {
			t.Errorf("outcome dir mode = %v, want owner-only 0700", fi.Mode().Perm())
		}
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Errorf("a week-old outcome directory was not pruned (stat err %v)", err)
		}
		for _, keep := range []string{fresh, file} {
			if _, err := os.Stat(keep); err != nil {
				t.Errorf("pruning removed %s, which it must keep: %v", keep, err)
			}
		}
	})

	t.Run("two claims with prompt files in one directory get different outcome directories", func(t *testing.T) {
		home, root, side := t.TempDir(), t.TempDir(), t.TempDir()
		oa := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, promptFile: filepath.Join(side, "prompt-07.md")}
		ob := dispatchOpts{kit: "verifier", item: "example-stream/08", root: root, promptFile: filepath.Join(side, "prompt-08.md")}
		a, err := allocateOutcomeDir(oa, dispatchPlan{claimKey: "repo--example-stream--07"}, home)
		if err != nil {
			t.Fatal(err)
		}
		b, err := allocateOutcomeDir(ob, dispatchPlan{claimKey: "repo--example-stream--08"}, home)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(side, outcomeDirName, "repo--example-stream--07")
		if a != want || a == b || filepath.Dir(a) != filepath.Dir(b) {
			t.Errorf("beside-prompt outcome dirs = %s, %s; want per-claim subdirectories like %s", a, b, want)
		}
	})

	t.Run("traversal-shaped claim keys are refused on both branches", func(t *testing.T) {
		home, root, side, cache := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
		old := packetCacheDir
		packetCacheDir = func() (string, error) { return cache, nil }
		t.Cleanup(func() { packetCacheDir = old })
		branches := map[string]string{
			"beside --prompt-file": filepath.Join(side, "prompt.md"),
			"user cache":           "",
		}
		keys := []string{"../escape", "a/../../b", `a\..\b`, "..", "a..b", "/abs", "a/b", " repo--x", ""}
		for branch, pf := range branches {
			for _, key := range keys {
				t.Run(branch+"/"+key, func(t *testing.T) {
					o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, promptFile: pf}
					dir, err := allocateOutcomeDir(o, dispatchPlan{claimKey: key}, home)
					if err == nil || !deskkit.IsRefused(err) {
						t.Fatalf("claim key %q on the %s branch = %q, %v; want a refusal", key, branch, dir, err)
					}
				})
			}
			parent := filepath.Join(side, outcomeDirName)
			if pf == "" {
				parent = filepath.Join(cache, "assay", "outcomes")
			}
			if entries, err := os.ReadDir(parent); err == nil && len(entries) != 0 {
				t.Errorf("%s branch: refused keys still created %d entr(ies) under %s", branch, len(entries), parent)
			}
		}
		for _, outside := range []string{filepath.Join(side, "escape"), filepath.Join(cache, "assay", "escape"), filepath.Join(side, "b")} {
			if _, err := os.Lstat(outside); !os.IsNotExist(err) {
				t.Errorf("a traversal-shaped key reached %s (stat err %v)", outside, err)
			}
		}
	})

	t.Run("a symlinked outcomes base or parent is refused and never pruned through", func(t *testing.T) {
		now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
		oldNow := packetNow
		packetNow = func() time.Time { return now }
		t.Cleanup(func() { packetNow = oldNow })
		// plant returns a victim tree holding a month-old directory with a file in it.
		plant := func(t *testing.T) (victim, precious string) {
			t.Helper()
			victim = t.TempDir()
			precious = filepath.Join(victim, "outcomes", "precious-old-dir")
			if err := os.MkdirAll(precious, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(precious, "data"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			old := now.Add(-30 * 24 * time.Hour)
			if err := os.Chtimes(precious, old, old); err != nil {
				t.Fatal(err)
			}
			return victim, precious
		}
		symlink := func(t *testing.T, target, link string) {
			t.Helper()
			if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				if runtime.GOOS == "windows" {
					t.Skip("symlinks unavailable")
				}
				t.Fatal(err)
			}
		}
		cases := []struct {
			name string
			// link plants the symlink for cache (or, for the prompt-file form, side) and
			// returns the --prompt-file to use ("" for the cache form).
			link func(t *testing.T, cache, side, victim string) string
		}{
			{"cache base <cache>/assay/outcomes", func(t *testing.T, cache, _, victim string) string {
				symlink(t, filepath.Join(victim, "outcomes"), filepath.Join(cache, "assay", "outcomes"))
				return ""
			}},
			{"cache parent <cache>/assay", func(t *testing.T, cache, _, victim string) string {
				symlink(t, victim, filepath.Join(cache, "assay"))
				return ""
			}},
			{"prompt-file base <prompt dir>/outcomes", func(t *testing.T, _, side, victim string) string {
				symlink(t, filepath.Join(victim, "outcomes"), filepath.Join(side, outcomeDirName))
				return filepath.Join(side, "prompt.md")
			}},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				cache, side, home, root := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
				old := packetCacheDir
				packetCacheDir = func() (string, error) { return cache, nil }
				t.Cleanup(func() { packetCacheDir = old })
				victim, precious := plant(t)
				pf := c.link(t, cache, side, victim)
				o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, promptFile: pf}
				dir, err := allocateOutcomeDir(o, dispatchPlan{claimKey: "repo--example-stream--07"}, home)
				if err == nil || !deskkit.IsRefused(err) {
					t.Errorf("allocation through a symlinked %s = %q, %v; want a refusal", c.name, dir, err)
				}
				if _, serr := os.Stat(filepath.Join(precious, "data")); serr != nil {
					t.Errorf("the prune deleted a directory outside this tool's own base through the symlink: %v", serr)
				}
				if _, serr := os.Lstat(filepath.Join(victim, "outcomes", "repo--example-stream--07")); !os.IsNotExist(serr) {
					t.Errorf("a refused allocation created the leaf in the symlink's target (stat err %v)", serr)
				}
			})
		}
	})

	t.Run("an existing leaf that is a symlink or a file is refused", func(t *testing.T) {
		home, root, side, elsewhere := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
		base := filepath.Join(side, outcomeDirName)
		if err := os.MkdirAll(base, 0o700); err != nil {
			t.Fatal(err)
		}
		leaf := filepath.Join(base, "repo--example-stream--07")
		o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, promptFile: filepath.Join(side, "p.md")}
		plants := map[string]func() error{
			"symlink to elsewhere":  func() error { return os.Symlink(elsewhere, leaf) },
			"symlink into the home": func() error { return os.Symlink(home, leaf) },
			"regular file":          func() error { return os.WriteFile(leaf, []byte("x"), 0o600) },
		}
		for name, plant := range plants {
			t.Run(name, func(t *testing.T) {
				_ = os.Remove(leaf)
				if err := plant(); err != nil {
					if runtime.GOOS == "windows" {
						t.Skip("symlinks unavailable")
					}
					t.Fatal(err)
				}
				dir, err := allocateOutcomeDir(o, dispatchPlan{claimKey: "repo--example-stream--07"}, home)
				// A symlink into the home is caught earlier, by the symlink-resolving containment
				// check; the other plants reach the leaf's own Lstat guard.
				want := "is not a directory"
				if name == "symlink into the home" {
					want = "OUTSIDE"
				}
				if err == nil || !deskkit.IsRefused(err) || !strings.Contains(err.Error(), want) {
					t.Fatalf("leaf planted as %s: allocateOutcomeDir = %q, %v; want a refusal naming %q", name, dir, err, want)
				}
			})
		}
	})

	t.Run("an outcomes base other users can write to is refused", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX permission bits do not apply")
		}
		home, root, side := t.TempDir(), t.TempDir(), t.TempDir()
		base := filepath.Join(side, outcomeDirName)
		for _, mode := range []os.FileMode{0o777, 0o770, os.ModeSticky | 0o777} {
			t.Run(mode.String(), func(t *testing.T) {
				if err := os.MkdirAll(base, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(base, mode); err != nil {
					t.Fatal(err)
				}
				o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, promptFile: filepath.Join(side, "p.md")}
				dir, err := allocateOutcomeDir(o, dispatchPlan{claimKey: "repo--example-stream--07"}, home)
				if err == nil || !deskkit.IsRefused(err) || !strings.Contains(err.Error(), "writable by other users") {
					t.Fatalf("base mode %v: allocateOutcomeDir = %q, %v; want the writable-by-others refusal", mode, dir, err)
				}
				if _, serr := os.Lstat(filepath.Join(base, "repo--example-stream--07")); !os.IsNotExist(serr) {
					t.Errorf("a refused allocation created the leaf (stat err %v)", serr)
				}
			})
		}
	})

	t.Run("non-verifier prompts carry no outcome line", func(t *testing.T) {
		for _, p := range []struct{ kit, item string }{{"worker", "example-stream--07"}, {"review", "review-item-9"}} {
			prompt := dispatchPrompt(t, p.kit, p.item)
			if strings.Contains(prompt, "Outcome directory") || strings.Contains(prompt, "--outcome-record") {
				t.Errorf("%s prompt carries the verifier outcome line:\n%s", p.kit, assignmentSection(t, prompt))
			}
		}
	})
}
