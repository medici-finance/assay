package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type chainFixture struct {
	root, path, base, delivery, body, descriptor string
	amendments, branches, payloads, urls         []string
	replies                                      map[int]string
	issue                                        string
	reads                                        int
	offline                                      bool
}

func chainFixtureAt(t *testing.T, shape string) *chainFixture {
	t.Helper()
	o := vfDefaults()
	o.evidence = ""
	o.status = "verified"
	o.verified = "2026-08-13 human:alex"
	count := 7
	if shape == "repeat" {
		count = 15
	}
	o.verify = "| # | Command | Expect | Class | Shell |\n|---|---|---|---|---|\n"
	for i := 1; i <= count; i++ {
		command, class := "`true`", "check"
		if shape != "repeat" && i == 5 {
			command = "Run `true` then `true` and `true`"
		}
		if shape != "repeat" && i == 7 {
			command = "Inspect `one.go`, `two.go` and `three.go`"
			class = "gate:model +flow +dereference"
		}
		o.verify += fmt.Sprintf("| %d | %s | exit 0 | %s | sh |\n", i, command, class)
	}
	root, _ := vfFixture(t, o)
	path := vfBrief(root)
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	body := strings.Replace(string(raw), "schema: brief-v1", "schema: brief-v2\nversion: 1", 1)
	f := &chainFixture{root: root, path: path, body: body, replies: map[int]string{}, issue: rlIssueJSON("<!-- acceptance-transition: " + vfBriefRel + " -->")}
	f.write(t, body)
	vfGit(t, root, vfMailHuman, "remote", "add", "origin", "https://github.com/"+rlRepo+".git")
	vfCommit(t, root, vfMailHuman)
	f.base = vfRev(t, root, "HEAD")
	f.write(t, strings.Replace(body, "## Evidence\n", "## Evidence\nHistorical result retained.\n", 1))
	vfCommit(t, root, vfMailWorker)
	f.delivery = vfRev(t, root, "HEAD")
	first := f.body
	ids := []int{1, 2, 3, 4, 6}
	if shape == "repeat" {
		ids = []int{12, 15}
	}
	var named []string
	for _, id := range ids {
		first = strings.Replace(first, fmt.Sprintf("| %d | `true` |", id), fmt.Sprintf("| %d | `true && true` |", id), 1)
		named = append(named, fmt.Sprintf("%d:Command", id))
	}
	f.amend(t, first, strings.Join(named, ","), true)
	second := f.body
	if shape == "repeat" {
		second = strings.Replace(second, "| 12 | `true && true` |", "| 12 | `true && true && true` |", 1)
		f.amend(t, second, "12:Command", true)
		// The existing v1 approval is retained for the first link.
		f.payloads[0] = fmt.Sprintf("Approve acceptance-transition v1\nrepo=%s\nbrief=%s\ndelivery=%s\nbase=%s\noriginal=%s\namendment=%s\nrows=12,15", rlRepo, vfBriefRel, f.delivery, f.base, f.base, f.amendments[0])
	} else {
		second = strings.Replace(second, "Run `true` then `true` and `true` | exit 0", "`true` | exit 0 and checked", 1)
		second = strings.Replace(second, "Inspect `one.go`, `two.go` and `three.go` | exit 0", "Inspect `a.go`, `b.go`, `c.go`, `d.go`, `e.go`, `f.go`, `g.go`, `h.go`, `i.go`, `j.go` | exit 0 and inspected", 1)
		f.amend(t, second, "5:Command+Expect,7:Command+Expect", true)
	}
	f.bind(t)
	return f
}

func (f *chainFixture) write(t *testing.T, body string) {
	t.Helper()
	if e := os.WriteFile(f.path, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	f.body = body
	resetReadMemo()
}
func (f *chainFixture) amend(t *testing.T, body, rows string, merge bool) {
	t.Helper()
	k := len(f.amendments) + 1
	body = strings.Replace(body, fmt.Sprintf("version: %d", k), fmt.Sprintf("version: %d", k+1), 1)
	if merge {
		vfGit(t, f.root, vfMailHuman, "checkout", "-q", "-b", fmt.Sprintf("link-%d", k))
	}
	f.write(t, body)
	vfCommit(t, f.root, vfMailHuman)
	branch := vfRev(t, f.root, "HEAD")
	if merge {
		vfGit(t, f.root, vfMailHuman, "checkout", "-q", "main")
		vfGit(t, f.root, vfMailHuman, "merge", "--no-ff", "-q", "-m", fmt.Sprintf("amendment %d", k), fmt.Sprintf("link-%d", k))
	}
	rev := vfRev(t, f.root, "HEAD")
	prior := f.delivery
	if k > 1 {
		prior = f.amendments[k-2]
	}
	f.amendments = append(f.amendments, rev)
	f.branches = append(f.branches, branch)
	f.urls = append(f.urls, rlURL(rlRepo, rlIssue, rlComment+k-1))
	f.payloads = append(f.payloads, fmt.Sprintf("Approve acceptance-transition v2\nrepo=%s\nbrief=%s\ndelivery=%s\nbase=%s\noriginal=%s\nlink=%d\npredecessor=%s\namendment=%s\nrows=%s", rlRepo, vfBriefRel, f.delivery, f.base, f.base, k, prior, rev, rows))
	f.record()
}
func (f *chainFixture) record() {
	f.descriptor = fmt.Sprintf("Verification-Table: delivery=%s base=%s original=%s", f.delivery, f.base, f.base)
	for i, a := range f.amendments {
		f.descriptor += fmt.Sprintf("\nAcceptance-Transition: link=%d amendment=%s approval=%s", i+1, a, f.urls[i])
	}
}
func (f *chainFixture) bind(t *testing.T) {
	t.Helper()
	oldClient, oldForge := transitionClient, transitionForge
	login, _ := HumanLogin("alex")
	transitionClient = func() *ghClient {
		return &ghClient{base: "https://api.github.com", token: "fixture", doer: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			f.reads++
			if f.offline {
				return nil, fmt.Errorf("fixture forge unreachable")
			}
			body := f.issue
			if strings.Contains(r.URL.Path, "/comments/") {
				n, e := strconv.Atoi(filepath.Base(r.URL.Path))
				if e != nil {
					t.Fatal(e)
				}
				i := n - rlComment
				if i < 0 || i >= len(f.payloads) {
					t.Fatalf("unexpected approval read %s", r.URL.Path)
				}
				body = rlCommentJSONWith(login, "User", rlRepo, rlIssue, f.payloads[i], rlPosted, rlPosted)
				body = strings.ReplaceAll(body, "issuecomment-9001", fmt.Sprintf("issuecomment-%d", n))
				body = strings.Replace(body, `"id":9001`, fmt.Sprintf(`"id":%d`, n), 1)
				if override, ok := f.replies[i]; ok {
					body = override
				}
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})}}
	}
	transitionForge = true
	t.Cleanup(func() { transitionClient, transitionForge = oldClient, oldForge })
}
func (f *chainFixture) plan(t *testing.T) (protectedPlan, string) {
	t.Helper()
	verify, evidence, e := briefSections(f.path)
	if e != nil {
		t.Fatal(e)
	}
	return protectedAcceptanceFor(verify, evidence, f.descriptor, f.path)
}
func (f *chainFixture) refuse(t *testing.T, contains string, structural bool) {
	t.Helper()
	f.reads = 0
	if structural {
		old := transitionClient
		transitionClient = func() *ghClient { t.Fatal("structural refusal reached approval read"); return nil }
		defer func() { transitionClient = old }()
	}
	_, why := f.plan(t)
	if why == "" || !strings.Contains(why, contains) {
		t.Fatalf("wanted %q refusal, got %q", contains, why)
	}
	if structural && f.reads != 0 {
		t.Fatal("structural refusal read the forge")
	}
}

// regression: #6064
func TestTransitionChain(t *testing.T) {
	for _, shape := range []string{"container", "repeat"} {
		t.Run(shape, func(t *testing.T) {
			f := chainFixtureAt(t, shape)
			p, why := f.plan(t)
			if why != "" {
				t.Fatal(why)
			}
			want := "1=Command,2=Command,3=Command,4=Command,5=Command+Expect,6=Command,7=Command+Expect"
			if shape == "repeat" {
				want = "12=Command,15=Command"
			}
			if p.Changes != want {
				t.Fatalf("changes %q want %q", p.Changes, want)
			}
			rows := briefVerifyRows(p.Verify)
			if shape == "repeat" {
				if rows[11].Command != "true && true && true" || p.Revisions["12"] != f.amendments[1] || p.Revisions["15"] != f.amendments[0] {
					t.Fatalf("repeat plan %+v", p)
				}
			} else {
				for _, id := range []string{"1", "2", "3", "4", "6"} {
					if p.Revisions[id] != f.amendments[0] {
						t.Fatalf("lost first link row %s", id)
					}
				}
				if rows[4].CommandCell != "`true`" || !strings.Contains(rows[6].CommandCell, "`j.go`") || p.Revisions["5"] != f.amendments[1] || p.Revisions["7"] != f.amendments[1] {
					t.Fatalf("whole-cell plan %+v", p)
				}
			}
			if f.reads != 4 {
				t.Fatalf("each link must read its own comment and issue: %d", f.reads)
			}
			if shape == "repeat" {
				f.payloads[0] = "\n" + strings.ReplaceAll(f.payloads[0], "\n", "\r\n") + "\n"
				if _, why := f.plan(t); why != "" {
					t.Fatalf("v1 normalization changed: %s", why)
				}
			}
		})
	}
}

// regression: #6064
func TestTransitionCombinedColumns(t *testing.T) {
	for _, mode := range []string{"approved", "Command", "Expect", "class", "shell", "added", "supplement", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			// Existing one-link fixture supplies the unmodified legacy policy boundary.
			f := contextFixture(t, nil, func(stage, body string) string {
				if stage == "original" {
					body = strings.Replace(body, "| Expect |", "| Expect | Class | Shell |", 1)
					body = strings.Replace(body, "| exit 1 |", "| exit 1 | check | sh |", 1)
				}
				if stage == "amendment" {
					body = strings.Replace(body, "| exit 1 |", "| exit 0 |", 1)
					switch mode {
					case "class":
						body = strings.Replace(body, "| check |", "| gate:model |", 1)
					case "shell":
						body = strings.Replace(body, "| sh |", "| pwsh |", 1)
					case "added":
						body = strings.Replace(body, "## Evidence", "| 4 | `true` | exit 0 | check | sh |\n\n## Evidence", 1)
					case "supplement":
						body = strings.Replace(body, "`printf supplemental`", "`printf changed`", 1)
					}
				}
				return body
			})
			descriptor := f.descriptor
			if mode != "legacy" {
				descriptor = strings.Replace(descriptor, "Acceptance-Transition:", "Acceptance-Transition: link=1", 1)
				column := "Command+Expect"
				if mode == "Command" || mode == "Expect" {
					column = mode
				}
				payload := strings.Replace(f.payload, "transition v1", "transition v2", 1)
				payload = strings.Replace(payload, "amendment=", "link=1\npredecessor="+f.delivery+"\namendment=", 1)
				payload = strings.Replace(payload, "rows=2", "rows=2:"+column, 1)
				login, _ := HumanLogin("alex")
				f.forge.commentBody = rlCommentJSONWith(login, "User", rlRepo, rlIssue, payload, rlPosted, rlPosted)
			}
			if mode == "class" || mode == "shell" || mode == "added" || mode == "supplement" {
				transitionClient = func() *ghClient { t.Fatal("structural refusal read approval"); return nil }
			}
			v, e, _ := briefSections(f.path)
			p, why := protectedAcceptanceFor(v, e, descriptor, f.path)
			if mode == "approved" {
				if why != "" || p.Changes != "2=Command+Expect" {
					t.Fatalf("%s %+v", why, p)
				}
			} else if why == "" {
				t.Fatal("unsupported change accepted")
			}
			if mode == "legacy" && !strings.Contains(why, "only command-only or Expect-only original-row changes are supported") {
				t.Fatal(why)
			}
		})
	}
}

// regression: #6064
func TestChainRefusesForgedApproval(t *testing.T) {
	for _, mode := range []string{"bot", "unmapped", "edited", "marker", "other-repo", "unreachable", "whitespace", "repo", "brief", "delivery", "base", "original", "link", "predecessor", "amendment", "rows"} {
		t.Run(mode, func(t *testing.T) {
			f := chainFixtureAt(t, "container")
			login, _ := HumanLogin("alex")
			typ, repo, posted := "User", rlRepo, rlPosted
			body := f.payloads[1]
			switch mode {
			case "bot":
				login, typ = "example-bot[bot]", "Bot"
			case "unmapped":
				login = "example-unmapped"
			case "edited":
				posted = "2026-09-02T10:00:00Z"
			case "marker":
				// Only the second approval's issue loses its binding.
				f.issue = rlIssueJSON("no marker")
				// First link's approval is valid; use a per-read client issue wrapper below.
			case "other-repo":
				repo = "other/project"
			case "unreachable":
				f.offline = true
			case "whitespace":
				body = "\n" + body
			default:
				body = strings.Replace(body, "\n"+mode+"=", "\n"+mode+"=wrong-", 1)
			}
			reply := rlCommentJSONWith(login, typ, repo, rlIssue, body, rlPosted, posted)
			reply = strings.ReplaceAll(reply, "issuecomment-9001", "issuecomment-9002")
			reply = strings.Replace(reply, `"id":9001`, `"id":9002`, 1)
			f.replies[1] = reply
			if mode == "marker" || mode == "unreachable" {
				old := transitionClient
				calls := 0
				transitionClient = func() *ghClient {
					calls++
					if calls == 1 {
						f.issue = rlIssueJSON("<!-- acceptance-transition: " + vfBriefRel + " -->")
						f.offline = false
					} else if mode == "marker" {
						f.issue = rlIssueJSON("no marker")
					} else {
						f.offline = true
					}
					return old()
				}
			}
			expected := "acceptance transition: link 2: "
			if mode == "unreachable" {
				expected += "could-not-check"
			}
			f.refuse(t, expected, false)
		})
	}
}

// regression: #6064
func TestChainRefusesReplayedApproval(t *testing.T) {
	for _, mode := range []string{"duplicate-url", "replay", "amendment", "predecessor", "brief", "v1"} {
		t.Run(mode, func(t *testing.T) {
			f := chainFixtureAt(t, "repeat")
			switch mode {
			case "duplicate-url":
				f.descriptor = strings.Replace(f.descriptor, f.urls[1], f.urls[0], 1)
			case "replay":
				f.payloads[1] = f.payloads[0]
			case "v1":
				f.payloads[1] = strings.Replace(f.payloads[1], "transition v2", "transition v1", 1)
			default:
				f.payloads[1] = strings.Replace(f.payloads[1], "\n"+mode+"=", "\n"+mode+"=wrong-", 1)
			}
			f.refuse(t, "link 2:", mode == "duplicate-url")
		})
	}
}

// regression: #6064
func TestChainRefusesSkippedLink(t *testing.T) {
	for _, mode := range []string{"gap", "intermediate", "after", "version-only", "reverted", "version-jump", "merge-base"} {
		t.Run(mode, func(t *testing.T) {
			f := chainFixtureAt(t, "repeat")
			switch mode {
			case "gap":
				f.descriptor = strings.Replace(f.descriptor, "link=2", "link=3", 1)
			case "intermediate":
				body := strings.Replace(f.body, "| 12 | `true && true && true` |", "| 12 | `true && true && true && true` |", 1)
				f.amend(t, body, "12:Command", true)
				lines := strings.Split(f.descriptor, "\n")
				f.descriptor = lines[0] + "\n" + lines[1] + "\n" + strings.Replace(lines[3], "link=3", "link=2", 1)
			case "after":
				f.write(t, strings.Replace(f.body, "exit 0", "exit 0 and unapproved", 1))
				vfCommit(t, f.root, vfMailHuman)
			case "version-only":
				f.write(t, strings.Replace(f.body, "version: 3", "version: 4", 1))
				vfCommit(t, f.root, vfMailHuman)
			case "reverted":
				original := f.body
				f.write(t, strings.Replace(f.body, "exit 0", "exit 0 and unapproved", 1))
				vfCommit(t, f.root, vfMailHuman)
				f.write(t, original)
				vfCommit(t, f.root, vfMailHuman)
			case "version-jump":
				vfGit(t, f.root, vfMailHuman, "reset", "--hard", f.amendments[0])
				raw, _ := os.ReadFile(f.path)
				f.body = string(raw)
				f.amendments = f.amendments[:1]
				f.branches = f.branches[:1]
				f.urls = f.urls[:1]
				f.payloads = f.payloads[:1]
				body := strings.Replace(f.body, "version: 2", "version: 4", 1)
				body = strings.Replace(body, "`true && true`", "`true && true && true`", 1)
				f.amend(t, body, "12:Command", false)
			case "merge-base":
				// Branch diverges before link 1; main retains the approved prior table.
				vfGit(t, f.root, vfMailHuman, "checkout", "-q", "-b", "diverged", f.delivery)
				raw, _ := os.ReadFile(f.path)
				body := strings.Replace(string(raw), "version: 1", "version: 4", 1)
				body = strings.Replace(body, "| 1 | `true` |", "| 1 | `true && true` |", 1)
				f.write(t, body)
				vfCommit(t, f.root, vfMailHuman)
				vfGit(t, f.root, vfMailHuman, "checkout", "-q", "main")
				// Construct a merge fixture with the exact final approved table; the
				// distinct divergence baseline is the rejection under test.
				prior := f.body
				raw, _ = os.ReadFile(f.path)
				prior = string(raw)
				vfGit(t, f.root, vfMailHuman, "merge", "--no-ff", "--no-commit", "-s", "ours", "diverged")
				f.write(t, strings.Replace(strings.Replace(prior, "version: 3", "version: 4", 1), "| 1 | `true` |", "| 1 | `true && true` |", 1))
				vfCommit(t, f.root, vfMailHuman)
				rev := vfRev(t, f.root, "HEAD")
				f.amendments = append(f.amendments, rev)
				f.urls = append(f.urls, rlURL(rlRepo, rlIssue, rlComment+2))
				f.record()
			}
			expected := "acceptance transition: link "
			if mode == "intermediate" {
				expected = "link 2 missing at " + f.amendments[1][:12]
			}
			f.refuse(t, expected, true)
		})
	}
}

// regression: #6064
func TestChainRefusesReorderedChain(t *testing.T) {
	t.Run("per-link-first-parent", func(t *testing.T) {
		f := chainFixtureAt(t, "repeat")
		read := func(rev string) (string, error) {
			raw, e := historyGit(f.root, "show", "--end-of-options", rev+":"+vfBriefRel).Output()
			return extractSectionByPrefix(string(raw), "Verify"), e
		}
		prior, e := read(f.amendments[0])
		if e != nil {
			t.Fatal(e)
		}
		current, e := read(f.branches[1])
		if e != nil {
			t.Fatal(e)
		}
		_, evidence, _ := briefSections(f.path)
		marker := fmt.Sprintf("Acceptance-Transition: amendment=%s approval=%s", f.branches[1], f.urls[1])
		chain := transitionLink{number: 2, amendment: f.branches[1], approval: f.urls[1], predecessor: f.amendments[0], preflight: true}
		_, _, why := acceptanceStep(f.path, current, evidence, marker, f.delivery, f.base, f.base, prior, prior, false, read, &chain)
		if !strings.Contains(why, "first-parent chain") {
			t.Fatalf("per-link ancestry accepted: %q", why)
		}
	})
	for _, mode := range []string{"lines", "amendments", "non-descendant", "branch", "mixed", "malformed", "broken-link", "url"} {
		t.Run(mode, func(t *testing.T) {
			f := chainFixtureAt(t, "repeat")
			lines := strings.Split(f.descriptor, "\n")
			switch mode {
			case "lines":
				f.descriptor = lines[0] + "\n" + lines[2] + "\n" + lines[1]
			case "amendments":
				f.descriptor = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(f.descriptor, f.amendments[0], "SWAP"), f.amendments[1], f.amendments[0]), "SWAP", f.amendments[1])
			case "non-descendant":
				f.descriptor = strings.Replace(f.descriptor, f.amendments[1], f.delivery, 1)
			case "branch":
				f.descriptor = strings.Replace(f.descriptor, f.amendments[1], f.branches[1], 1)
			case "mixed":
				f.descriptor = strings.Replace(f.descriptor, "link=1 ", "", 1)
			case "malformed":
				f.descriptor = strings.Replace(f.descriptor, "link=2", "link=02", 1)
			case "broken-link":
				f.descriptor = strings.Replace(f.descriptor, "link=2", "link =2", 1)
			case "url":
				f.descriptor = strings.Replace(f.descriptor, f.urls[1], "https://other.invalid/approval", 1)
			}
			f.refuse(t, "acceptance transition: link ", true)
		})
	}
}

// regression: #6064
func TestChainRefusesEditAfterApproval(t *testing.T) {
	for _, mode := range []string{"current", "evidence", "history", "unnamed-command", "repeat-unnamed", "expect-drift"} {
		t.Run(mode, func(t *testing.T) {
			f := chainFixtureAt(t, "repeat")
			switch mode {
			case "current":
				f.write(t, strings.Replace(f.body, "exit 0", "exit 0 ", 1))
			case "history":
				f.write(t, strings.Replace(f.body, "Historical result retained.", "", 1))
			case "evidence":
				vfGit(t, f.root, vfMailHuman, "reset", "--hard", f.amendments[0])
				raw, _ := os.ReadFile(f.path)
				f.body = string(raw)
				f.amendments = f.amendments[:1]
				f.urls = f.urls[:1]
				f.payloads = f.payloads[:1]
				f.amend(t, strings.Replace(strings.Replace(f.body, "Historical result retained.", "edited history", 1), "| 12 | `true && true` |", "| 12 | `true && true && true` |", 1), "12:Command", false)
			case "unnamed-command":
				f.payloads[0] = strings.Replace(f.payloads[0], "rows=12,15", "rows=15", 1)
			case "repeat-unnamed":
				f.payloads[1] = strings.Replace(f.payloads[1], "rows=12:Command", "rows=", 1)
			case "expect-drift":
				f.amend(t, strings.Replace(strings.Replace(f.body, "| 1 | `true` | exit 0", "| 1 | `true` | exit 1", 1), "| 12 | `true && true && true` |", "| 12 | `true && true && true && true` |", 1), "12:Command", false)
				p, why := f.plan(t)
				if why != "" {
					t.Fatal(why)
				}
				if briefVerifyRows(p.Verify)[0].Expect != "exit 0" || p.Revisions["1"] != "" {
					t.Fatal("unnamed Expect drift credited")
				}
				return
			}
			structural := mode == "current" || mode == "evidence" || mode == "history"
			expected := "acceptance transition: link "
			if mode == "unnamed-command" || mode == "repeat-unnamed" {
				expected = "row 12"
			}
			f.refuse(t, expected, structural)
		})
	}
}

// regression: #6064
func TestChainExecution(t *testing.T) {
	// An unrelated current definition changed at delivery remains owed at its
	// original value; use the established fixture for this execution projection.
	base := transitionFixtureAt(t)
	raw, _ := os.ReadFile(base.path)
	body := strings.Replace(string(raw), "version: 2", "version: 3", 1)
	body = strings.Replace(body, "sh check.sh --no-pull", "sh check.sh --no-pull --offline", 1)
	if e := os.WriteFile(base.path, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	vfCommit(t, base.root, vfMailHuman)
	last := vfRev(t, base.root, "HEAD")
	f := &chainFixture{root: base.root, path: base.path, base: base.base, delivery: base.delivery, body: body, amendments: []string{base.amendment, last}, urls: []string{rlURL(rlRepo, rlIssue, rlComment), rlURL(rlRepo, rlIssue, rlComment+1)}, payloads: []string{base.payload, fmt.Sprintf("Approve acceptance-transition v2\nrepo=%s\nbrief=%s\ndelivery=%s\nbase=%s\noriginal=%s\nlink=2\npredecessor=%s\namendment=%s\nrows=2:Command", rlRepo, vfBriefRel, base.delivery, base.base, base.base, base.amendment, last)}, replies: map[int]string{}, issue: rlIssueJSON("<!-- acceptance-transition: " + vfBriefRel + " -->")}
	f.record()
	f.bind(t)
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	descriptor := filepath.Join(t.TempDir(), "record.txt")
	for _, invalid := range []string{f.descriptor + "\nAcceptance-Changes: 2=Command", strings.Join([]string{strings.Split(f.descriptor, "\n")[1], strings.Split(f.descriptor, "\n")[0], strings.Split(f.descriptor, "\n")[2]}, "\n")} {
		os.WriteFile(descriptor, []byte(invalid), 0600)
		result, diag := captureVerifyrun(t, []string{"--root", f.root, "--brief", f.path, "--acceptance-record", descriptor, "--acceptance-forge"})
		if result.code == 0 || !strings.Contains(diag, "--acceptance-record") {
			t.Fatalf("invalid descriptor %d %s", result.code, diag)
		}
	}
	os.WriteFile(descriptor, []byte(f.descriptor), 0600)
	log, err := os.CreateTemp(t.TempDir(), "approval-log")
	if err != nil {
		t.Fatal(err)
	}
	priorStderr := os.Stderr
	os.Stderr = log
	result, diag := captureVerifyrun(t, []string{"--root", f.root, "--brief", f.path, "--acceptance-record", descriptor, "--acceptance-forge"})
	os.Stderr = priorStderr
	log.Close()
	approvalLog, err := os.ReadFile(log.Name())
	if err != nil {
		t.Fatal(err)
	}
	if result.code != 0 {
		t.Fatalf("execution %d %s", result.code, diag)
	}
	_, ev, _ := briefSections(f.path)
	rows := parseEvidenceRows(ev)
	if witnessCommandOf(rows["1"][0].Text) != "true" || witnessCommandOf(rows["2"][0].Text) != "sh check.sh --no-pull --offline" || strings.Count(ev, "Acceptance-Changes:") != 1 {
		t.Fatalf("effective execution %s", ev)
	}
	if strings.Count(string(approvalLog), "acceptance transition: reading approval") != 2 {
		t.Fatalf("read diagnostics %s", approvalLog)
	}
	vfAppend(t, f.root, "\n\n"+vfPass)
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	streams, _, e := loadStreams(f.root)
	if e != nil {
		t.Fatal(e)
	}
	if problems := protectedClosureProblems(streams); len(problems) != 0 {
		t.Fatalf("fresh closure %v", problems)
	}
	// A real committed verifier run is rewritten to cite link 1's execution
	// revision: provenance remains independent, freshness must still refuse.
	raw, _ = os.ReadFile(f.path)
	os.WriteFile(f.path, []byte(strings.ReplaceAll(string(raw), " @ "+last, " @ "+base.amendment)), 0644)
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	streams, _, _ = loadStreams(f.root)
	problems := protectedClosureProblems(streams)
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), "fresh witness") {
		t.Fatalf("stale witness accepted: %v", problems)
	}
}

// regression: #6064
func TestTransitionV1Unchanged(t *testing.T) {
	f := chainFixtureAt(t, "container")
	for _, tc := range []struct{ amendment, why string }{{f.branches[0], "amendment is not after delivery on the observed first-parent chain"}, {f.amendments[0], "current Verify differs from the approved amendment"}, {f.amendments[1], "acceptance changed between delivery and amendment"}} {
		f.descriptor = fmt.Sprintf("Verification-Table: delivery=%s base=%s original=%s\nAcceptance-Transition: amendment=%s approval=%s", f.delivery, f.base, f.base, tc.amendment, f.urls[0])
		_, why := f.plan(t)
		if why != "protected table: acceptance transition: "+tc.why {
			t.Fatalf("legacy refusal changed: %q", why)
		}
	}
}
