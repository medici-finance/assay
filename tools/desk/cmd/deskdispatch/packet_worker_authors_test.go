package main

// packet_worker_authors_test.go — what the shepherding packet says about WHO wrote a quoted
// item and WHICH commit a finding record speaks of.
//
//   - a finding record in a review at an earlier commit is judged against that review's
//     commit, not read as though it were pinned to the current head;
//   - comments are admitted newest first, so the cap leaves out the oldest;
//   - every quoted comment, and every review by another account, carries what the
//     reviewer binding and the trusted list say of its author — or says that the list is
//     not configured, never a guess.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// wpTrustedList plants the package's fixture roster in a private config home: `ada` (id
// 2001) and the role apps are on the trusted list, nobody else is.
func wpTrustedList(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(deskkit.EnvConfigHome, "")
	plantFixtureRoster(t, home)
	if !deskkit.EffectiveConfig().Configured() || !deskkit.TrustedAuthorID("ada", 2001) {
		t.Fatal("control: the fixture roster did not load")
	}
}

// wpNoTrustedList points the config home at an empty directory: no list is configured.
func wpNoTrustedList(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(deskkit.EnvConfigHome, "")
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	if deskkit.EffectiveConfig().Configured() {
		t.Fatal("control: a trusted list is configured in an empty config home")
	}
}

func wpComment(id int64, login string, uid int64, body string) deskkit.Comment {
	c := deskkit.Comment{DatabaseID: id, Body: body, CreatedAt: fmt.Sprintf("2026-03-02T00:00:%02dZ", id%60)}
	c.Author.Login, c.Author.ID = login, uid
	return c
}

// TestWorkerShepherdPacketJudgesAFindingAgainstItsReviewsCommit: a record that says
// `resolved` and names no evidence commit means the commit its review is at. In a review at
// an earlier commit that is not this head, so the entry is still a standing blocker here —
// the packet's own definition is "not resolved with evidence at that head".
func TestWorkerShepherdPacketJudgesAFindingAgainstItsReviewsCommit(t *testing.T) {
	root, home := wpRoot(t), wpWorktree(t)
	f := baseWorkerForge()
	f.reviews[1].Body = "second round\n\n" + wpFindings(
		deskkit.Finding{ID: "G-1", Class: "naming", Severity: deskkit.SeverityBlocking,
			State: deskkit.StateResolved, OriginHead: wpOldHead, Resolution: "renamed"},
		deskkit.Finding{ID: "G-2", Class: "naming", Severity: deskkit.SeverityBlocking,
			State: deskkit.StateResolved, OriginHead: wpOldHead, EvidenceHead: rpHead, Resolution: "renamed"},
		deskkit.Finding{ID: "G-3", Class: "off-by-one", Severity: deskkit.SeverityBlocking,
			State: deskkit.StateOpen, OriginHead: wpOldHead, Failure: "TestRows fails"},
		deskkit.Finding{ID: "G-4", Class: "wording", Severity: deskkit.SeverityAdvisory,
			State: deskkit.StateOpen, OriginHead: wpOldHead},
	)
	// A review the forge reports no commit for: its record's evidence is at no commit this
	// tool can compare with the head.
	f.reviews[2].CommitID = ""
	f.reviews[2].Body = "third round\n\n" + wpFindings(
		deskkit.Finding{ID: "H-1", Class: "naming", Severity: deskkit.SeverityBlocking,
			State: deskkit.StateResolved, OriginHead: wpOldHead, Resolution: "renamed"},
	)
	useWorkerForge(t, f)
	p, err := buildWorkerPacket(t, wpShepherdInput(root, home))
	if err != nil {
		t.Fatal(err)
	}
	reviews := packetSection(t, p.Text, "Reviews and findings")
	wpWantAll(t, "Reviews and findings", reviews,
		"  - finding `G-1` — class `naming` — state `resolved` — a STANDING BLOCKER at this head by this record: "+
			"resolved with evidence at commit `"+wpOldHead+"`, which is not this head\n",
		"  - finding `G-2` — class `naming` — state `resolved` — not a standing blocker at this head by this record\n",
		"  - finding `G-3` — class `off-by-one` — state `open` — a STANDING BLOCKER at this head by this record\n",
		"  - finding `G-4` — class `wording` — state `open` — not a standing blocker at this head by this record\n",
		"  - finding `H-1` — class `naming` — state `resolved` — a STANDING BLOCKER at this head by this record: "+
			"resolved with evidence at its review's commit, which the forge does not report\n",
		"A record that names no evidence commit means the commit its review is at",
		"Only the first finding record in a review's text is read")
}

// TestWorkerPacketAdmitsCommentsNewestFirst: ten comments that cannot all fit. The two
// outside the newest eight are listed as earlier; of the eight, the ones the packet's
// overall cap leaves out are the OLDEST, and every one left out is listed.
func TestWorkerPacketAdmitsCommentsNewestFirst(t *testing.T) {
	root, home := wpRoot(t), wpWorktree(t)
	f := baseWorkerForge()
	f.comments = nil
	const each = 30 << 10
	for id := int64(101); id <= 110; id++ {
		f.comments = append(f.comments, wpComment(id, wpReviewer, 0, fmt.Sprintf("BODY-%d ", id)+strings.Repeat("c", each-9)))
	}
	useWorkerForge(t, f)
	p, err := buildWorkerPacket(t, wpShepherdInput(root, home))
	if err != nil {
		t.Fatal(err)
	}
	if each > workerPacketPerItem || 6*each > workerPacketOverall || 7*each <= workerPacketOverall {
		t.Fatalf("control: the fixture assumes six %d-byte comments fit the %d-byte overall cap and seven do not", each, workerPacketOverall)
	}
	for id := 105; id <= 110; id++ {
		if !strings.Contains(p.Text, fmt.Sprintf("BODY-%d ", id)) {
			t.Errorf("comment %d, one of the six newest, is not quoted", id)
		}
	}
	for id := 101; id <= 104; id++ {
		if strings.Contains(p.Text, fmt.Sprintf("BODY-%d ", id)) {
			t.Errorf("comment %d is quoted; the six newest are what fits", id)
		}
	}
	for _, id := range []int{103, 104} {
		if o, ok := omissionNamed(p, fmt.Sprintf("comment %d", id)); !ok || o.Size != each || !strings.Contains(o.Reason, "overall cap") {
			t.Errorf("comment %d was not listed as left out for the overall cap, with its size: %+v", id, p.Omitted)
		}
	}
	if o, ok := omissionNamed(p, "the 2 earlier comment(s) on the change"); !ok || o.Size != 2*each {
		t.Errorf("the two comments outside the newest eight were not listed with their size: %+v", p.Omitted)
	}
	comments := packetSection(t, p.Text, "Comments")
	last := -1
	for id := 110; id >= 103; id-- {
		i := strings.Index(comments, fmt.Sprintf("- comment %d — by ", id))
		if i < 0 || i < last {
			t.Fatalf("comment %d is not on its own line after every newer one (at %d, the one before at %d)", id, i, last)
		}
		last = i
	}
	wpWantAll(t, "Comments", comments, "10 comment(s) on the change", "NEWEST FIRST", "not a sort by date")
}

// TestWorkerPacketMarksEveryQuotedAuthor: with a trusted list configured, each comment and
// each review by another account says what the reviewer binding and the list say of the
// account the forge names. A login on the list with the wrong account id is not on it.
func TestWorkerPacketMarksEveryQuotedAuthor(t *testing.T) {
	wpTrustedList(t)
	root, home := wpRoot(t), wpWorktree(t)
	f := baseWorkerForge()
	f.comments = []deskkit.Comment{
		wpComment(21, "ada", 2001, "from a listed account\n"),
		wpComment(22, "ada", 9999, "from a recycled login\n"),
		wpComment(23, "someone", 77, "from a passer-by\n"),
		wpComment(24, wpReviewer, 0, "from the reviewer\n"),
		wpComment(25, "", 0, "from nobody the forge names\n"),
	}
	other := func(id int64, login string, uid int64) deskkit.Review {
		r := deskkit.Review{ID: id, State: "COMMENTED", CommitID: rpHead, Body: "text\n", SubmittedAt: "2026-03-03T00:00:00Z"}
		r.Author.Login, r.Author.ID = login, uid
		return r
	}
	f.reviews = append(f.reviews, other(31, "ada", 2001), other(32, "someone", 77))
	useWorkerForge(t, f)
	p, err := buildWorkerPacket(t, wpShepherdInput(root, home))
	if err != nil {
		t.Fatal(err)
	}
	comments := packetSection(t, p.Text, "Comments")
	wpWantAll(t, "Comments", comments,
		"- comment 21 — by `ada` (on the trusted list) — created ",
		"- comment 22 — by `ada` (NOT on the trusted list) — created ",
		"- comment 23 — by `someone` (NOT on the trusted list) — created ",
		"- comment 24 — by `"+wpReviewer+"` (the reviewer identity) — created ",
		"- comment 25 — by (not reported) (no author to check) — created ",
		"a mark is about the account")
	reviews := packetSection(t, p.Text, "Reviews and findings")
	for _, want := range []string{
		"by `ada` — another account's; not the reviewer's; on the trusted list",
		"by `someone` — another account's; not the reviewer's; NOT on the trusted list",
	} {
		if !strings.Contains(reviews, want) {
			t.Errorf("the reviews section lacks %q:\n%s", want, reviews)
		}
	}

	// The reviewer identity cannot be resolved: nobody is marked as it.
	useReviewer(t, "", false)
	p, err = buildWorkerPacket(t, wpShepherdInput(root, home))
	if err != nil {
		t.Fatal(err)
	}
	comments = packetSection(t, p.Text, "Comments")
	wpWantAll(t, "Comments", comments, "- comment 24 — by `"+wpReviewer+"` (NOT on the trusted list) — created ",
		"- comment 21 — by `ada` (on the trusted list) — created ")
	wpWantNone(t, "the packet", p.Text, "(the reviewer identity)")
	wpWantAll(t, "Reviews and findings", packetSection(t, p.Text, "Reviews and findings"),
		"by `ada` — author not checked against the reviewer identity; on the trusted list",
		"by `someone` — author not checked against the reviewer identity; NOT on the trusted list")
}

// TestWorkerPacketSaysWhenNoTrustedListIsConfigured: with no list, no account is called
// trusted and none is called untrusted: the line says the list is not configured.
func TestWorkerPacketSaysWhenNoTrustedListIsConfigured(t *testing.T) {
	wpNoTrustedList(t)
	root, home := wpRoot(t), wpWorktree(t)
	f := baseWorkerForge()
	f.comments = []deskkit.Comment{wpComment(21, "ada", 2001, "text\n"), wpComment(24, wpReviewer, 0, "from the reviewer\n")}
	r := deskkit.Review{ID: 31, State: "COMMENTED", CommitID: rpHead, Body: "text\n"}
	r.Author.Login, r.Author.ID = "ada", 2001
	f.reviews = append(f.reviews, r)
	useWorkerForge(t, f)
	p, err := buildWorkerPacket(t, wpShepherdInput(root, home))
	if err != nil {
		t.Fatal(err)
	}
	wpWantAll(t, "Comments", packetSection(t, p.Text, "Comments"),
		"- comment 21 — by `ada` (no trusted list is configured here: not checked) — created ",
		"- comment 24 — by `"+wpReviewer+"` (the reviewer identity) — created ")
	wpWantAll(t, "Reviews and findings", packetSection(t, p.Text, "Reviews and findings"),
		"by `ada` — another account's; not the reviewer's; no trusted list is configured here: not checked")
	wpWantNone(t, "the packet", p.Text, "(on the trusted list)", "(NOT on the trusted list)", "; on the trusted list", "; NOT on the trusted list")
}

// TestWorkerPacketCapsCommentsByOtherAccounts: a comment by an account that is neither the
// reviewer identity nor on the trusted list is quoted only up to the limit a review by
// another account has; over it, it is listed with its size. The same size from the
// reviewer identity or a listed account is quoted.
func TestWorkerPacketCapsCommentsByOtherAccounts(t *testing.T) {
	big := func(tag string) string { return tag + " " + strings.Repeat("c", reviewPacketOtherBodyCap) }
	build := func(t *testing.T) (string, func(name, body string) bool) {
		root, home := wpRoot(t), wpWorktree(t)
		f := baseWorkerForge()
		f.comments = []deskkit.Comment{
			wpComment(41, "someone", 77, big("BIG-OUTSIDER")),
			wpComment(42, wpReviewer, 0, big("BIG-REVIEWER")),
			wpComment(43, "ada", 2001, big("BIG-LISTED")),
			wpComment(44, "someone", 77, "SMALL-OUTSIDER\n"),
		}
		useWorkerForge(t, f)
		p, err := buildWorkerPacket(t, wpShepherdInput(root, home))
		if err != nil {
			t.Fatal(err)
		}
		return p.Text, func(name, body string) bool {
			o, ok := omissionNamed(p, name)
			return ok && o.Size == int64(len(body))
		}
	}
	t.Run("a list is configured", func(t *testing.T) {
		wpTrustedList(t)
		text, omitted := build(t)
		wpWantAll(t, "the packet", text, "BIG-REVIEWER", "BIG-LISTED", "SMALL-OUTSIDER",
			fmt.Sprintf("A comment by an account that is neither the reviewer identity nor on the trusted list is quoted only up to %d bytes.", reviewPacketOtherBodyCap))
		wpWantNone(t, "the packet", text, "BIG-OUTSIDER")
		if !omitted("comment 41", big("BIG-OUTSIDER")) {
			t.Error("the over-limit comment by another account is not listed as left out with its size")
		}
	})
	t.Run("no list is configured", func(t *testing.T) {
		wpNoTrustedList(t)
		text, omitted := build(t)
		wpWantAll(t, "the packet", text, "BIG-REVIEWER", "SMALL-OUTSIDER")
		wpWantNone(t, "the packet", text, "BIG-OUTSIDER", "BIG-LISTED")
		if !omitted("comment 41", big("BIG-OUTSIDER")) || !omitted("comment 43", big("BIG-LISTED")) {
			t.Error("with no list, an over-limit comment by any account but the reviewer identity is listed as left out")
		}
	})
}

// TestWorkerImplementPacketNeverShowsAnUnreadDependencyAsMet: a dependency whose board has
// no row for it, or whose board cannot be read, is "could not check" — never a status. The
// control row beside them is read normally.
func TestWorkerImplementPacketNeverShowsAnUnreadDependencyAsMet(t *testing.T) {
	root, home := wpRoot(t), wpWorktree(t)
	wpTree(t, home, map[string]string{
		wpBriefRel: strings.Replace(wpBrief, `depends: ["alpha/01", "beta/02", "gamma/09", "not a reference"]`,
			`depends: ["alpha/01", "alpha/77", "delta/01"]`, 1),
		// A board that exists and cannot be read as a file: a directory of that name.
		"docs/streams/delta/README.md/placeholder": "x\n",
	})
	useWorkerHead(t, wpBase, nil)
	p, err := buildWorkerPacket(t, wpImplementInput(root, home))
	if err != nil {
		t.Fatal(err)
	}
	deps := packetSection(t, p.Text, "Dependencies")
	wpWantAll(t, "Dependencies", deps,
		"- `alpha/01` — board status `done`\n",
		"- `alpha/77` — could not check: the stream's board has no readable row for `77`\n",
		"- `delta/01` — could not check: the stream's board could not be read (")
	var lines int
	for _, l := range strings.Split(deps, "\n") {
		if strings.HasPrefix(l, "- `alpha/77`") || strings.HasPrefix(l, "- `delta/01`") {
			lines++
			if strings.Contains(l, "board status") {
				t.Errorf("a dependency that was not read is shown with a status: %s", l)
			}
		}
	}
	if lines != 2 {
		t.Errorf("control: %d lines for the two unread dependencies, want 2:\n%s", lines, deps)
	}
}
