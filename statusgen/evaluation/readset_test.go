package evaluation

import (
	"strings"
	"testing"
)

// readset_test.go — build-less-brittle/15: a witness's recorded read set
// replaces the scope it speaks for, and every guard around it still binds.
// Each fixture is a witness commit W (the tree the row ran on), an Evidence
// commit E that adds the read-set file, and an item commit T on top of E that
// changes only the paths a test names.

const (
	rsBrief   = "docs/streams/s/brief-01-x.md"
	rsBriefID = "s/01"
	rsCommand = "cd tools/x && go test ./..."
)

// rsRepo is fakeRepo with W an ancestor of every later commit: the merge base
// of (a, b) is a, so the landing check finds W's own tree on T's history.
type rsRepo struct{ *fakeRepo }

func (rsRepo) MergeBase(string, string) History { return HistoryRelated }
func (rsRepo) MergeBaseAll(a, _ string) ([]string, bool) {
	return []string{a}, true
}

type rsFx struct {
	t       *testing.T
	repo    *fakeRepo
	tree    map[string]fakeEntry
	attrs   map[string][]string
	entries []string
	header  func(*ReadSetHeader)
	// token: "sha256" (default), "none" or a could-not-check reason.
	token   string
	dirty   bool
	runCmd  string // the command the witness ran ("" = rsCommand)
	obs     func(*ReadSetObservation)
	floors  GoFloorMap
	src     GoFloorSource // when set, used in place of floors
	W, E, T string
	digest  string
	runner  string
	content []byte
}

func newRsFx(t *testing.T) *rsFx {
	return &rsFx{
		t:    t,
		repo: newFakeRepo(),
		tree: map[string]fakeEntry{
			"tools/x/go.mod":                  {"100644", "module x\n\ngo 1.22\n"},
			"tools/x/a.go":                    {"100644", "package x\n\nfunc A() int { return 1 }\n"},
			"tools/x/b.go":                    {"100644", "package x\n\nfunc B() int { return 2 }\n"},
			"tools/x/a_test.go":               {"100644", "package x\n"},
			"tools/x/testdata/in.txt":         {"100644", "fixture-1\n"},
			"tools/y/y.go":                    {"100644", "package y\n\nconst Y = 1\n"},
			".claude-plugin/marketplace.json": {"100644", "{\"v\":1}\n"},
			".assay-surfaces":                 {"100644", "surface\n"},
			".gitattributes":                  {"100644", "*.go text\n"},
			"STATUS.md":                       {"100644", "board\n"},
			"docs/streams/s/README.md":        {"100644", "readme\n"},
			"docs/streams/s/notes.md":         {"100644", "notes\n"},
			rsBrief:                           {"100644", "brief\n"},
		},
		attrs: map[string][]string{},
		entries: []string{
			"read tools/x/go.mod",
			"read tools/x/a.go",
			"read tools/x/b.go",
			"read tools/x/a_test.go",
			"read tools/x/testdata/in.txt",
			"list tools/x",
		},
		token: "sha256",
	}
}

func rsRow() VerifyRow { return VerifyRow{ID: "1", Command: rsCommand, Expect: "exit 0"} }

// build commits W and E; item(change) then commits T.
func (fx *rsFx) build() {
	fx.W = fx.repo.commit(nil, "run", fx.tree, fx.attrs)
	h := ReadSetHeader{Commit: fx.W, Command: ReadSetCommandDigest(rsCommand), Brief: rsBrief, Row: "1"}
	if fx.header != nil {
		fx.header(&h)
	}
	fx.content = FormatReadSet(h, fx.entries)
	fx.digest = ReadSetDigest(fx.content)
	treeE := cloneTree(fx.tree)
	treeE[ReadSetPath("docs/streams/s", fx.digest)] = fakeEntry{"100644", string(fx.content)}
	treeE[rsBrief] = fakeEntry{"100644", "brief\n\n## Evidence\nrow\n"}
	fx.E = fx.repo.commit([]string{fx.W}, "evidence", treeE, fx.attrs)
	fx.repo.expectAt[fx.W] = rsRow().Expect
	fx.runner = "local @ " + fx.W
	switch fx.token {
	case "sha256":
		fx.runner += " " + ReadSetCellToken(fx.digest, "")
	case "none":
	default:
		fx.runner += " " + ReadSetCellToken("", fx.token)
	}
}

// item commits T = E with change applied and returns the row's claim at T.
func (fx *rsFx) item(change func(tree map[string]fakeEntry, attrs map[string][]string)) Claim {
	fx.t.Helper()
	if fx.W == "" {
		fx.build()
	}
	tree := cloneTree(fx.repo.commits[fx.E].tree)
	attrs := cloneAttrs(fx.attrs)
	if change != nil {
		change(tree, attrs)
	}
	fx.T = fx.repo.commit([]string{fx.E}, "item", tree, attrs)
	return fx.claimAt(fx.T)
}

func (fx *rsFx) claimAt(rev string) Claim {
	fx.t.Helper()
	tree := fx.W
	if fx.dirty {
		tree += "+dirty"
	}
	runner := strings.Replace(fx.runner, "@ "+fx.W, "@ "+tree, 1)
	cmd := fx.runCmd
	if cmd == "" {
		cmd = rsCommand
	}
	o := &ReadSetObservation{Content: fx.content, Found: true, IntroFound: true, IntroRunner: runner, IntroFile: true}
	if fx.obs != nil {
		fx.obs(o)
	}
	w := Witness{Command: cmd, State: StatePass, Tree: tree, Runner: runner, Result: "pass", ReadSet: o}
	b := BriefObservation{ID: rsBriefID, BriefRel: rsBrief, VerifyRows: []VerifyRow{rsRow()}, Witnesses: map[string][]Witness{"1": {w}}}
	in := CoverageInput{Revision: rev, Briefs: []BriefObservation{b}, GoFloors: fx.floors}
	if fx.src != nil {
		in.GoFloors = fx.src
	}
	cs := EvaluateCoverage(in, rsRepo{fx.repo})[rsBriefID].Claims
	if len(cs) == 0 {
		fx.t.Fatal("no claim")
	}
	return cs[0]
}

func edit(p, content string) func(map[string]fakeEntry, map[string][]string) {
	return func(t map[string]fakeEntry, _ map[string][]string) { t[p] = fakeEntry{"100644", content} }
}

func edits(fs ...func(map[string]fakeEntry, map[string][]string)) func(map[string]fakeEntry, map[string][]string) {
	return func(t map[string]fakeEntry, a map[string][]string) {
		for _, f := range fs {
			f(t, a)
		}
	}
}

// unreadOnly changes the three paths no row of this fixture reads.
var unreadOnly = edits(
	edit(".claude-plugin/marketplace.json", "{\"v\":2}\n"),
	edit(".assay-surfaces", "surface-2\n"),
	edit(".gitattributes", "*.go text\n*.md text\n"),
)

func wantResult(t *testing.T, name string, c Claim, result string, contains ...string) {
	t.Helper()
	if c.Result != result {
		t.Errorf("%s: want %s, got %s: %s", name, result, c.Result, c.Reason)
		return
	}
	if result != ResultPass && c.Released {
		t.Errorf("%s: a %s claim is released", name, result)
	}
	for _, s := range contains {
		if !strings.Contains(c.Reason, s) {
			t.Errorf("%s: the reason %q does not contain %q", name, c.Reason, s)
		}
	}
}

func wantReleased(t *testing.T, name string, c Claim) {
	t.Helper()
	if c.Result != ResultPass || !c.Released {
		t.Errorf("%s: want a release, got %s: %s", name, c.Result, c.Reason)
	}
}

// regression: #2026
func TestReadSetReleasesUnreadPathChange(t *testing.T) {
	wantReleased(t, "read set, unread paths changed", newRsFx(t).item(unreadOnly))
	// The same witness without a read set keeps today's scope and is held:
	// the release above is the read set's doing.
	fx := newRsFx(t)
	fx.token = "none"
	wantResult(t, "no read set", fx.item(unreadOnly), ResultWrongRevision)
}

// regression: #2026
func TestReadSetRefusesChangedDependency(t *testing.T) {
	fx := newRsFx(t)
	fx.entries = append(fx.entries, "read tools/y/y.go")
	wantReleased(t, "unread paths only", fx.item(unreadOnly))
	c := fx.item(edits(unreadOnly, edit("tools/y/y.go", "package y\n\nconst Y = 2\n")))
	wantResult(t, "depended-on file changed", c, ResultWrongRevision,
		"differs between the witness's tree and the item's and the witness speaks for it", "tools/y/y.go")
}

// regression: #2026
func TestReadSetFloorCatchesUnderRecordedGoInput(t *testing.T) {
	goRun := GoRunEntry(GoRun{Dir: "tools/x", Args: []string{"go", "test", "./..."}})
	floor := GoFloor{Files: []string{"tools/x/go.mod", "tools/x/a.go", "tools/x/b.go", "tools/x/a_test.go"}}
	under := func(fx *rsFx) {
		// The record omits tools/x/b.go, a file the package compiles.
		var kept []string
		for _, e := range fx.entries {
			if e != "read tools/x/b.go" {
				kept = append(kept, e)
			}
		}
		fx.entries = append(kept, goRun)
	}
	changeB := edit("tools/x/b.go", "package x\n\nfunc B() int { return 3 }\n")

	// The upper layer alone (the tree-wise comparison of the recorded set)
	// releases: b.go is not recorded and is the only change.
	upper := newRsFx(t)
	under(upper)
	upper.entries = upper.entries[:len(upper.entries)-1]
	wantReleased(t, "upper layer alone", upper.item(changeB))

	fx := newRsFx(t)
	under(fx)
	fx.build()
	fx.floors = map[string]GoFloor{GoFloorKey(fx.W, goRun): floor}
	wantResult(t, "floor", fx.item(changeB), ResultWrongRevision, ReasonFloorOmits, "tools/x/b.go")

	// A floor that the record covers does not refuse an unrelated change.
	ok := newRsFx(t)
	ok.entries = append(ok.entries, goRun)
	ok.build()
	ok.floors = map[string]GoFloor{GoFloorKey(ok.W, goRun): floor}
	wantReleased(t, "covered floor", ok.item(unreadOnly))

	// A floor that could not be computed is could-not-check.
	bad := newRsFx(t)
	bad.entries = append(bad.entries, goRun)
	bad.build()
	bad.floors = map[string]GoFloor{GoFloorKey(bad.W, goRun): {Problem: "go list failed"}}
	wantResult(t, "floor problem", bad.item(unreadOnly), ResultCouldNotCheck, "go list failed")
}

// regression: #2026
func TestReadSetMissingFailsClosed(t *testing.T) {
	cases := map[string]func(*ReadSetObservation){
		"file absent": func(o *ReadSetObservation) { o.Found, o.Content = false, nil },
		"other digest": func(o *ReadSetObservation) {
			o.Content = append(append([]byte(nil), o.Content...), "read tools/y/y.go\n"...)
		},
		"never observed":  nil,
		"absent at exact": func(o *ReadSetObservation) { o.Found = false },
		"empty but found": func(o *ReadSetObservation) { o.Content = nil },
	}
	for name, mut := range cases {
		fx := newRsFx(t)
		if mut != nil {
			fx.obs = mut
		} else {
			fx.obs = func(o *ReadSetObservation) { *o = ReadSetObservation{} }
		}
		var c Claim
		if name == "absent at exact" {
			fx.build()
			c = fx.claimAt(fx.W)
		} else {
			c = fx.item(unreadOnly)
		}
		if c.Result != ResultCouldNotCheck || c.Reason != ReasonReadSetUnreadable || c.Released {
			t.Errorf("%s: want could-not-check %q, got %s: %s", name, ReasonReadSetUnreadable, c.Result, c.Reason)
		}
	}
}

// regression: #2026
func TestReadSetDirtyWitnessStillRefuses(t *testing.T) {
	fx := newRsFx(t)
	fx.dirty = true
	wantResult(t, "+dirty witness", fx.item(unreadOnly), ResultWrongRevision, "uncommitted edits (+dirty)")
}

// regression: #2026
func TestReadSetEditedRowStillRefuses(t *testing.T) {
	fx := newRsFx(t)
	fx.runCmd = "cd tools/x && go test -run Old ./..."
	c := fx.item(unreadOnly)
	if c.Released || !strings.Contains(c.Reason, "the witness records a different command than the Verify row now carries") {
		t.Errorf("edited row: got %s: %s", c.Result, c.Reason)
	}
}

// regression: #2026
func TestReadSetHistoryReadNeedsExactRevision(t *testing.T) {
	fx := newRsFx(t)
	fx.entries = append(fx.entries, EntryHistory)
	c := fx.item(unreadOnly)
	wantResult(t, "history at another revision", c, ResultWrongRevision, ReasonReadSetHistory)
	wantReleased(t, "history at the exact revision", fx.claimAt(fx.W))
}

// regression: #2026
func TestReadSetNegativeInputsRefuse(t *testing.T) {
	fx := newRsFx(t)
	wantResult(t, "file added to a listed directory", fx.item(edit("tools/x/c.go", "package x\n")), ResultWrongRevision,
		"tools/x/c.go", "listed the directory tools/x")

	ab := newRsFx(t)
	ab.entries = append(ab.entries, "absent tools/y/config.json")
	wantReleased(t, "absent path still absent", ab.item(unreadOnly))
	wantResult(t, "absent path now exists", ab.item(edit("tools/y/config.json", "{}\n")), ResultWrongRevision,
		"tools/y/config.json", "found it absent")
}

// regression: #2026
func TestReadSetContentAttributeChangeRefuses(t *testing.T) {
	fx := newRsFx(t)
	eol := func(p string) func(map[string]fakeEntry, map[string][]string) {
		return func(tr map[string]fakeEntry, a map[string][]string) {
			tr[".gitattributes"] = fakeEntry{"100644", "*.go text\n" + p + " eol=crlf\n"}
			a[p] = []string{"text: set", "eol: crlf"}
		}
	}
	wantResult(t, "eol on a recorded path", fx.item(eol("tools/x/a.go")), ResultWrongRevision, "tools/x/a.go", "content attributes")
	wantReleased(t, "eol on an unrecorded path", fx.item(eol("tools/y/y.go")))
}

// regression: #2026
func TestLegacyWitnessKeepsConservativeScope(t *testing.T) {
	market := edit(".claude-plugin/marketplace.json", "{\"v\":2}\n")
	for _, tok := range []string{"none", "recorder is Linux-only (darwin)"} {
		fx := newRsFx(t)
		fx.token = tok
		wantResult(t, "token "+tok, fx.item(market), ResultWrongRevision, ".claude-plugin/marketplace.json")
	}
	// The same witness with a read set is released by the same change.
	wantReleased(t, "read set", newRsFx(t).item(market))
}

// regression: #2026
func TestReadSetAddedAfterWitnessIgnored(t *testing.T) {
	market := edit(".claude-plugin/marketplace.json", "{\"v\":2}\n")
	cases := map[string]func(*rsFx){
		// The introducing commit's row had no token and no file: a later
		// commit added both at once.
		"token and file added later": func(fx *rsFx) {
			fx.obs = func(o *ReadSetObservation) { o.IntroRunner, o.IntroFile = "local @ "+fx.W, false }
		},
		"file committed after the row": func(fx *rsFx) {
			fx.obs = func(o *ReadSetObservation) { o.IntroFile = false }
		},
		"introduction not found": func(fx *rsFx) {
			fx.obs = func(o *ReadSetObservation) { o.IntroFound = false }
		},
		"header names another row": func(fx *rsFx) {
			fx.header = func(h *ReadSetHeader) { h.Row = "2" }
		},
		"header names another commit": func(fx *rsFx) {
			fx.header = func(h *ReadSetHeader) { h.Commit = strings.Repeat("a", 40) }
		},
		"header names another command": func(fx *rsFx) {
			fx.header = func(h *ReadSetHeader) { h.Command = ReadSetCommandDigest("true") }
		},
		"header names another brief": func(fx *rsFx) {
			fx.header = func(h *ReadSetHeader) { h.Brief = "docs/streams/s/brief-02-y.md" }
		},
		"only exempt paths": func(fx *rsFx) {
			fx.entries = []string{"read " + rsBrief, "read STATUS.md", "read docs/streams/s/notes.md"}
		},
	}
	for name, set := range cases {
		fx := newRsFx(t)
		set(fx)
		wantResult(t, name, fx.item(market), ResultWrongRevision, ".claude-plugin/marketplace.json")
	}
}

// regression: #2026
func TestReadSetStatComparesContent(t *testing.T) {
	fx := newRsFx(t)
	fx.entries = []string{"read tools/x/a.go", "stat tools/x/testdata/in.txt"}
	wantReleased(t, "stat-ed file unchanged", fx.item(unreadOnly))
	// Same size, same mode, other content.
	wantResult(t, "stat-ed file changed", fx.item(edit("tools/x/testdata/in.txt", "fixture-2\n")), ResultWrongRevision,
		"tools/x/testdata/in.txt", "differs between the witness's tree and the item's")
}

// regression: #2026
func TestReadSetGoRunWithoutFloorCouldNotCheck(t *testing.T) {
	fx := newRsFx(t)
	fx.entries = append(fx.entries, GoRunEntry(GoRun{Dir: "tools/x", Args: []string{"go", "test", "./..."}}))
	wantResult(t, "no floor", fx.item(unreadOnly), ResultCouldNotCheck, ReasonFloorMissing)

	// A go-run outside the build family (go env) needs no floor.
	env := newRsFx(t)
	env.entries = append(env.entries, GoRunEntry(GoRun{Dir: ".", Args: []string{"go", "env", "GOROOT"}}))
	wantReleased(t, "go env", env.item(unreadOnly))

	// `go -C dir test` is still a test.
	if g := (GoRun{Args: []string{"go", "-C", "tools/x", "test", "./..."}}); !g.BuildFamily() {
		t.Error("go -C dir test is not read as build-family")
	}
}

// regression: #2026
func TestReadSetExemptPathsSubtracted(t *testing.T) {
	fx := newRsFx(t)
	fx.entries = append(fx.entries, "read "+rsBrief, "read docs/streams/s/notes.md", "read STATUS.md")
	c := fx.item(edits(
		edit(rsBrief, "brief, edited\n"),
		edit("docs/streams/s/notes.md", "notes 2\n"),
		edit("STATUS.md", "board 2\n"),
	))
	wantReleased(t, "exempt paths changed", c)
	if !strings.Contains(c.Reason, "3 recorded reads exempt") {
		t.Errorf("the reason %q does not state the exempt count", c.Reason)
	}
	wantResult(t, "a non-exempt recorded path changed", fx.item(edit("tools/x/a.go", "package x\n\nfunc A() int { return 9 }\n")),
		ResultWrongRevision, "tools/x/a.go")
}

func TestReadSetTokenRoundTrip(t *testing.T) {
	d := strings.Repeat("ab", 32)
	cell := "verifyrun @ 0123456789ab " + ReadSetCellToken(d, "") + " (on behalf of x)"
	if got, _, ok := ReadSetToken(cell); !ok || got != d {
		t.Errorf("digest token: got %q %v", got, ok)
	}
	if got := StripReadSetToken(cell); got != "verifyrun @ 0123456789ab (on behalf of x)" {
		t.Errorf("strip: %q", got)
	}
	cnc := "verifyrun @ 0123456789ab " + ReadSetCellToken("", "recorder is Linux-only (darwin)")
	if _, why, ok := ReadSetToken(cnc); !ok || why != "recorder is Linux-only (darwin)" {
		t.Errorf("could-not-check token: got %q %v", why, ok)
	}
	if m := runnerTreeRe.FindStringSubmatch(cell); m == nil || m[1] != "0123456789ab" {
		t.Errorf("the tree token parser no longer reads the cell: %v", m)
	}
	odd := ReadSetCellToken("", "a | b\n(c (d)) e(")
	if _, why, ok := ReadSetToken("r @ 0123456789ab " + odd); !ok || strings.ContainsAny(why, "|\n") {
		t.Errorf("sanitised reason does not round-trip: %q -> %q %v", odd, why, ok)
	}
	h := ReadSetHeader{Commit: strings.Repeat("c", 40), Command: ReadSetCommandDigest("x"), Brief: rsBrief, Row: "3"}
	content := FormatReadSet(h, []string{"read b", "read a", "read a", EntryHistory})
	gh, entries, err := ParseReadSet(content)
	if err != nil || gh != h || strings.Join(entries, ",") != "history,read a,read b" {
		t.Errorf("parse: %v %+v %v", err, gh, entries)
	}
	for _, bad := range []string{"read /etc/passwd", "read ../x", "write a", "read a/../b"} {
		if _, _, err := ParseReadSet(FormatReadSet(h, []string{bad})); err == nil {
			t.Errorf("entry %q parsed", bad)
		}
	}
}

// countingFloors is a GoFloorSource that counts the floors it is asked for.
type countingFloors struct {
	floor GoFloor
	asked []string
}

func (c *countingFloors) GoFloor(commit, entry string) (GoFloor, bool) {
	c.asked = append(c.asked, GoFloorKey(commit, entry))
	return c.floor, true
}

// regression: #2026
// TestFloorAskedOnlyAdmitted — the floor is computed only for a read set the
// library has admitted: a read set refused at admission (added after the
// witness, or naming another commit) never asks for one, so a recorded go
// invocation in it reaches nothing; an admitted one asks exactly for its
// own entry at the witness commit.
func TestFloorAskedOnlyAdmitted(t *testing.T) {
	goRun := GoRunEntry(GoRun{Dir: "tools/x", Args: []string{"go", "test", "./..."}})
	floor := GoFloor{Files: []string{"tools/x/go.mod", "tools/x/a.go", "tools/x/b.go", "tools/x/a_test.go"}}
	market := edit(".claude-plugin/marketplace.json", "{\"v\":2}\n")
	refused := map[string]func(*rsFx){
		"file committed after the row": func(fx *rsFx) {
			fx.obs = func(o *ReadSetObservation) { o.IntroFile = false }
		},
		"header names another commit": func(fx *rsFx) {
			fx.header = func(h *ReadSetHeader) { h.Commit = strings.Repeat("a", 40) }
		},
	}
	for name, set := range refused {
		fx := newRsFx(t)
		fx.entries = append(fx.entries, goRun)
		set(fx)
		src := &countingFloors{floor: floor}
		fx.src = src
		wantResult(t, name, fx.item(market), ResultWrongRevision, ".claude-plugin/marketplace.json")
		if len(src.asked) != 0 {
			t.Errorf("%s: the floor was asked for %v before the read set was admitted", name, src.asked)
		}
	}
	fx := newRsFx(t)
	fx.entries = append(fx.entries, goRun)
	src := &countingFloors{floor: floor}
	fx.src = src
	wantReleased(t, "admitted", fx.item(market))
	if len(src.asked) == 0 || src.asked[0] != GoFloorKey(fx.W, goRun) {
		t.Errorf("admitted read set: floor asked for %v, want its entry at the witness commit", src.asked)
	}
}
