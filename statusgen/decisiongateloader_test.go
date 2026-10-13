package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Loader-view tests for the decision-gate hold (lifecycle-v1 §4.5): the hold
// counts a brief as ON THE BOARD only where the board's own loaders do, so a
// stream the loaders skip, or move off the active board, cannot shed a
// gate: human brief unseen; and a board path the board never reads through is
// not a board record at all. Each route below is two states: the first lands
// on the base, the second is judged against it.

// dhRegisterReadme is a stream index whose frontmatter fails to parse and which
// declares itself a register: loadStreams skips it (spec/registers-v1.md §7).
func dhRegisterReadme(rows ...string) string {
	s := "---\nstream: [unclosed\n---\n\nThis directory is a register, not a stream.\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n"
	for _, r := range rows {
		s += r + "\n"
	}
	return s + "\n"
}

// Register route: the index of a stream holding an unruled gate: human brief is
// rewritten so it fails to parse and says it is a register, rows untouched. The
// loader skips the stream (STATUS.md drops it) — the brief left the board, and
// that is a drop the hold refuses.
func TestDGRegisterRouteRefused(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhReadmePath: dhRegisterReadme(dhRow("18", "todo"))})
	dhRefusedBoth(t, root, dhBoardID)
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "drops it")
}

// Controls for the register route: a well-formed stream index that merely says
// "register, not a stream" in prose is still a stream (the phrase is consulted
// only after a parse failure), and a broken index that does NOT say it is a
// register aborts the board's load elsewhere — the hold keeps reading its rows
// and has nothing of its own to refuse for an unchanged row.
func TestDGRegisterRouteControls(t *testing.T) {
	dhSeams(t, nil)
	t.Run("well-formed index quoting the phrase", func(t *testing.T) {
		root := dhFixture(t, unruledBase())
		dhWrite(t, root, map[string]string{dhReadmePath: strings.Replace(dhReadme(dhRow("18", "todo")), "priority: P1\n---\n", "priority: P1\n---\n\nNot a register, not a stream index.\n", 1)})
		if p := dhLayerOne(t, root); len(p) != 0 {
			t.Fatalf("a well-formed stream index was read as a register: %v", p)
		}
	})
	t.Run("broken index that is no register", func(t *testing.T) {
		root := dhFixture(t, unruledBase())
		dhWrite(t, root, map[string]string{dhReadmePath: strings.Replace(dhRegisterReadme(dhRow("18", "todo")), "register, not a stream", "stream with a typo", 1)})
		if p := dhLayerOne(t, root); len(p) != 0 {
			t.Fatalf("a broken, non-register index was read as off the board: %v", p)
		}
	})
}

// dhArchive moves the whole stream directory into docs/archive.
func dhArchive(t *testing.T, root string) {
	t.Helper()
	mustMkdirAll(t, filepath.Join(root, "docs", "archive"))
	if err := os.Rename(filepath.Join(root, "docs", "streams", dhStream), filepath.Join(root, "docs", "archive", dhStream)); err != nil {
		t.Fatal(err)
	}
}

// Archive route: a stream holding an unruled gate: human brief that is not done
// is moved whole into docs/archive, rows kept. Archived work is completed work
// (archivecheck.go): the stream leaves the active board with its decision owed.
func TestDGArchiveOfOpenBriefRefused(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhArchive(t, root)
	dhRefusedBoth(t, root, dhBoardID)
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "archived", "no recorded ruling can stand for archiving")
}

// An archived index that fails to parse is skipped by the board's loaders (an
// archived register is no stream), whatever the text says, so the rows it lists
// are not on the board: the hold reports the brief as dropped, not as archived.
func TestDGArchivedBrokenIndexIsOffTheBoard(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhArchive(t, root)
	arch := filepath.Join(root, "docs", "archive", dhStream, "README.md")
	raw, err := os.ReadFile(arch)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(arch, append([]byte("not frontmatter\n"), raw...), 0o644); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(dhLayerOne(t, root), "\n")
	wantContains(t, "layer-one refusal", got, "drops it")
	if strings.Contains(got, "no recorded ruling can stand for archiving") {
		t.Fatalf("a broken archived index was read as on the board: %s", got)
	}
}

// A ruling does not lift it (the work is not done), a record that was already
// archived at the base is not turned red by a later change (a pin bump), and an
// archived DONE brief is not refused (TestDecisionGateArchiveIsNotDrop).
func TestDGArchiveOfOpenBriefNotLiftedByRulingAndGrandfathered(t *testing.T) {
	dhSeams(t, nil)
	ruled := map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", dhRuledLines()),
	}
	root := dhFixture(t, ruled)
	dhArchive(t, root)
	if p := dhLayerOne(t, root); len(p) == 0 {
		t.Fatal("a recorded ruling lifted the archiving of a brief that is not done")
	}
	dhLand(t, root)
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("a brief archived before the hold existed was refused again with nothing changed: %v", p)
	}
}

// Landing-lane masking: the landing branch already carries an unreadable record
// for the brief (its file is a link). A status move written beside it is a
// fault of its own, so a before/after diff of the refused lines (deskevidence's
// network layer) sees the move as new.
func TestDGMoveBesideUnreadableRecordIsItsOwnFault(t *testing.T) {
	dhSymlinks(t)
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhLinkedBrief(t, root)
	dhLand(t, root)
	_, before := dhGate(t, root)
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented"))})
	code, after := dhGate(t, root)
	if code == 0 {
		t.Fatalf("--decision-gate exit 0 for a move beside an unreadable record:\n%s", after)
	}
	have := map[string]bool{}
	for _, l := range strings.Split(before, "\n") {
		have[l] = true
	}
	var introduced []string
	for _, l := range strings.Split(after, "\n") {
		if strings.Contains(l, " REFUSED") && !have[l] {
			introduced = append(introduced, l)
		}
	}
	if len(introduced) == 0 {
		t.Fatalf("the move beside an unreadable record adds no refusal line to the report; before:\n%s\nafter:\n%s", before, after)
	}
	wantContains(t, "introduced refusal", strings.Join(introduced, "\n"), "moves it to implemented")
	wantContains(t, "layer-one refusals", strings.Join(dhLayerOne(t, root), "\n"), "moves it to implemented")
}

// Inert links: a symbolic link where a stream directory or a done/ folder would
// be, leading to a file or to nothing, is read by no board loader (they read
// streams and done/ briefs through directories only). It is not a board record:
// adding it, leaving it and removing it are all quiet, on a board with no
// gate: human brief and on one holding an unruled one.
func TestDGInertLinksAreNotBoardRecords(t *testing.T) {
	dhSymlinks(t)
	dhSeams(t, nil)
	for _, boards := range []struct {
		name  string
		files map[string]string
	}{
		{"no gate: human brief", map[string]string{dhReadmePath: dhReadme(dhRow("18", "todo")), dhBriefPath: dhFM("gate: model", "")}},
		{"unruled gate: human brief", unruledBase()},
	} {
		for _, pl := range []struct{ name, rel, target string }{
			{"notes link to a file", "docs/streams/NOTES.md", "../../README.md"},
			{"stream-dir position, dangling", "docs/streams/dangling", "../../nowhere"},
			{"archive position, to a file", "docs/archive/NOTES.md", "../../README.md"},
			{"done/ to a file", "docs/streams/" + dhStream + "/done", "../../../README.md"},
		} {
			t.Run(boards.name+"/"+pl.name, func(t *testing.T) {
				files := map[string]string{"README.md": "x\n"}
				for k, v := range boards.files {
					files[k] = v
				}
				root := dhFixture(t, files)
				dhLink(t, root, pl.rel, pl.target)
				if p := dhLayerOne(t, root); len(p) != 0 {
					t.Fatalf("adding the link was refused: %v", p)
				}
				if code, out := dhGate(t, root); code != 0 {
					t.Fatalf("adding the link turned the network lane red (exit %d):\n%s", code, out)
				}
				dhLand(t, root) // the pin bump: the link is now at the base
				if p := dhLayerOne(t, root); len(p) != 0 {
					t.Fatalf("a board holding the link was refused with nothing changed: %v", p)
				}
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(pl.rel))); err != nil {
					t.Fatal(err)
				}
				if p := dhLayerOne(t, root); len(p) != 0 {
					t.Fatalf("removing the link was refused: %v", p)
				}
				if code, out := dhGate(t, root); code != 0 {
					t.Fatalf("removing the link turned the network lane red (exit %d):\n%s", code, out)
				}
			})
		}
	}
}

// Both readers classify an inert link alike (the base from the git tree, the
// change from the working tree), and a link that CAN lead to a directory — an
// existing one, one through a link, or one leaving the repository — is still
// unreadable on both.
func TestDGLinkKindsAgree(t *testing.T) {
	dhSymlinks(t)
	cases := []struct {
		name, rel, target string
		unreadable        bool
		prep              func(t *testing.T, root string)
	}{
		{"to a file", "docs/streams/a", "../../README.md", false, nil},
		{"dangling", "docs/streams/b", "../../none", false, nil},
		{"to a file through a missing directory", "docs/streams/c", "../../none/x/y", false, nil},
		{"to a directory", "docs/streams/d", "../../elsewhere", true, func(t *testing.T, root string) {
			mustMkdirAll(t, filepath.Join(root, "elsewhere"))
			dhWrite(t, root, map[string]string{"elsewhere/x.md": "x\n"})
		}},
		{"through a link", "docs/streams/e", "linked-dir/sub", true, func(t *testing.T, root string) {
			mustMkdirAll(t, filepath.Join(root, "elsewhere", "sub"))
			dhLink(t, root, "docs/streams/linked-dir", "../../elsewhere")
		}},
		{"leaving the repository", "docs/streams/f", "../../../outside", true, nil},
		{"absolute", "docs/streams/g", "/tmp", true, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := unruledBase()
			files["README.md"] = "x\n"
			root := dhFixture(t, files)
			if c.prep != nil {
				c.prep(t, root)
			}
			dhLink(t, root, c.rel, c.target)
			dhLand(t, root)
			base, head := mustGateAtRev(t, root), gateSnapshotOnDisk(root)
			scope := gateScopeKeyPrefix + c.rel
			for _, side := range []struct {
				name string
				snap gateSnapshot
			}{{"base", base}, {"working tree", head}} {
				if got := side.snap[scope] != nil; got != c.unreadable {
					t.Errorf("%s reader: %s unreadable = %t, want %t", side.name, c.rel, got, c.unreadable)
				}
			}
		})
	}
}

// Class guard: every brief the hold counts as standing on the board (a README
// row it read) is also listed by the board's own loaders, and every gate: human
// brief the loaders list is one the hold covers — the hold's notion of "on the
// board" is the generator's, for each way a stream index can leave the board.
func TestDGBoardPresenceMatchesTheLoaders(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"plain", unruledBase()},
		{"register skipped by the loader", map[string]string{dhReadmePath: dhRegisterReadme(dhRow("18", "todo")), dhBriefPath: dhFM("gate: human", "decision-issue: 41\n")}},
		{"archived stream", map[string]string{
			"docs/archive/" + dhStream + "/README.md":   dhReadme(dhRow("18", "done")),
			"docs/archive/" + dhStream + "/brief-18.md": dhFM("gate: human", ""),
		}},
		{"archived index that does not parse", map[string]string{
			"docs/archive/" + dhStream + "/README.md":   dhRegisterReadme(dhRow("18", "done")),
			"docs/archive/" + dhStream + "/brief-18.md": dhFM("gate: human", ""),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := dhFixture(t, c.files)
			loaded := map[string]bool{}
			mustMkdirAll(t, filepath.Join(root, "docs", "streams"))
			active, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			archived, err := loadArchivedStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range append(active, archived...) {
				for _, b := range s.Briefs {
					loaded[filepath.Base(s.Dir)+"/"+normBriefNum(b.Num)] = true
				}
			}
			var held []string
			for k, gb := range gateSnapshotOnDisk(root) {
				if strings.HasPrefix(k, gateScopeKeyPrefix) || !gb.hasRow() {
					continue
				}
				held = append(held, k)
				if !loaded[k] {
					t.Errorf("the hold counts %s as on the board, but the board's loaders do not list it", k)
				}
			}
			sort.Strings(held)
			for k := range loaded {
				if !dhHas(held, k) {
					t.Errorf("the loaders list %s, but the hold does not count it as on the board (held: %v)", k, held)
				}
			}
		})
	}
}

func dhHas(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// A link to a directory in a stream position is a board record the hold cannot
// see behind. The refusal must not name as its remedy an edit that is itself
// refused (replacing or removing the link): it says there is no in-band exit
// and a human decides. Replacing or removing the link IS refused, in both lanes.
func TestDGLinkedStreamDirectoryHasNoInBandExit(t *testing.T) {
	dhSymlinks(t)
	dhSeams(t, nil)
	files := unruledBase()
	files["elsewhere/sdlc/brief-18.md"] = dhFM("gate: human", "decision-issue: 41\n")
	root := dhFixture(t, files)
	dhLink(t, root, "docs/streams/linked", "../../elsewhere/sdlc")
	dhLand(t, root)
	got := strings.Join(dhLayerOne(t, root), "\n")
	wantContains(t, "layer-one refusal", got, "needs-decision", "no in-band way to lift it", "docs/streams/linked is a board path")
	if strings.Contains(got, "docs/streams/linked is gate: human") {
		t.Errorf("the refusal calls a board path a gate: human brief:\n%s", got)
	}
	if err := os.Remove(filepath.Join(root, "docs", "streams", "linked")); err != nil {
		t.Fatal(err)
	}
	if p := dhLayerOne(t, root); len(p) == 0 {
		t.Error("removing the linked stream directory was not refused")
	}
}

// The reconcile hold stages a README move in memory before anything is written:
// layer one reads the board that write WOULD leave, so an unruled move of a
// gate: human brief is held, while the disk still says todo.
func TestDGReconcileHoldSeesStagedReadme(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("unchanged board refused: %v", p)
	}
	stage := newReadmeStage()
	t.Cleanup(stage.clear)
	stage.put(filepath.Join(root, filepath.FromSlash(dhReadmePath)), dhReadme(dhRow("18", "implemented")))
	got := strings.Join(dhLayerOne(t, root), "\n")
	wantContains(t, "layer-one refusal of a staged move", got, "sdlc/18")
}
