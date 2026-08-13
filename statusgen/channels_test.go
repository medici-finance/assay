package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deadPath is the retired invocation this brief removed from the emitters
// (#249). It is assembled at run time rather than written as one literal so
// this file does not itself become a repo-grep hit for the string it forbids.
var deadPath = "go run ./tools/" + "statusgen"

// TestDeadPathNeverEmitted asserts over the EMITTED STRINGS, not over the repo
// tree (distribution/05 Task 1).
//
// Scoping it to emitted strings is deliberate and load-bearing. A repo-wide
// grep for the dead path cannot legitimately reach zero: verifyrows_test.go
// holds it on purpose as a fixture for the unfailable-row detector, and the
// triage/open-core records quote it verbatim as the defect they describe.
// Asserting on the tree would force those edits, which reddens `go test ./...`
// and destroys a historical record — so the assertion is made where it
// actually matters: on what a user is told to run.
func TestDeadPathNeverEmitted(t *testing.T) {
	t.Run("generated STATUS.md header", func(t *testing.T) {
		s := &Stream{Name: "example", Track: "platform", Status: "active"}
		out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
		if strings.Contains(out, deadPath) {
			t.Errorf("emit() writes the retired invocation %q into every generated STATUS.md", deadPath)
		}
		if !strings.Contains(out, "Regenerate:") {
			t.Fatal("emit() emits no Regenerate: line at all — the header lost its instruction rather than fixing it")
		}
	})

	t.Run("stale-generated-file remediation messages", func(t *testing.T) {
		// The three stderr sites (main.go --check, and both register views)
		// share ONE constructor, so this covers all of them.
		for _, name := range []string{"STATUS.md", "INTAKE.md", "FINDINGS.md"} {
			for _, root := range []string{".", "..", "/tmp/some/root"} {
				msg := staleGeneratedFileMsg(name, root)
				if strings.Contains(msg, deadPath) {
					t.Errorf("staleGeneratedFileMsg(%q, %q) = %q — still names the retired invocation", name, root, msg)
				}
				if !strings.Contains(msg, name) {
					t.Errorf("staleGeneratedFileMsg(%q, %q) = %q — does not name the stale file", name, root, msg)
				}
			}
		}
	})

	t.Run("situation-aware hint in every branch", func(t *testing.T) {
		saved := statusgenVersion
		t.Cleanup(func() { statusgenVersion = saved })

		// Stamped release binary: it knows it was installed, so it must name
		// the installed binary and must not tell the user to `go run` a tree
		// they do not have.
		statusgenVersion = "statusgen/v0.8.0"
		got := regenerateHint("..")
		if strings.Contains(got, deadPath) || strings.Contains(got, "go run") {
			t.Errorf("stamped build hint = %q — a released binary must not advertise a source tree", got)
		}
		if !strings.HasPrefix(got, "statusgen --root") {
			t.Errorf("stamped build hint = %q — want the installed-binary invocation", got)
		}

		// Unstamped build, no source tree reachable from cwd: it cannot tell,
		// so it must say the channel-neutral thing and point at the pin spec
		// rather than guess a path. Guessing is exactly what #249 was.
		statusgenVersion = "dev"
		dir := t.TempDir()
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("chdir: %v", err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
		got = regenerateHint(".")
		if strings.Contains(got, deadPath) {
			t.Errorf("unstamped no-tree hint = %q — still names the retired invocation", got)
		}
		if !strings.Contains(got, pinSpecRef) {
			t.Errorf("unstamped no-tree hint = %q — must point at the pin spec when it cannot tell", got)
		}

		// Unstamped build sitting next to ./statusgen/main.go.
		if err := os.MkdirAll(filepath.Join(dir, "statusgen"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "statusgen", "main.go"), []byte("package main\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		got = regenerateHint(".")
		if got != "go run ./statusgen --root ." {
			t.Errorf("unstamped sibling-tree hint = %q, want %q", got, "go run ./statusgen --root .")
		}
	})

	t.Run("scaffold template teaches no retired invocation", func(t *testing.T) {
		if strings.Contains(initWorkflow, deadPath) {
			t.Errorf("initWorkflow scaffolds the retired invocation %q into every adopter's CI", deadPath)
		}
		if strings.Contains(initNextSteps, deadPath) {
			t.Errorf("initNextSteps prints the retired invocation %q", deadPath)
		}
		// The vendored-directory prose (init.go:165) is a separate defect from
		// the dead command: it does not contain the command, so driving the
		// command grep to zero never touches it. Assert it directly.
		if strings.Contains(initWorkflow, "if you vendored it elsewhere") {
			t.Error("initWorkflow still tells the adopter to adjust the path 'if you vendored it elsewhere' — vendoring is retired (channel A)")
		}
	})
}

// --- channel-conformance sweep -------------------------------------------

// writeRoot builds a scratch lint root containing the named files.
func writeRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return dir
}

func countContaining(lines []string, sub string) int {
	n := 0
	for _, l := range lines {
		if strings.Contains(l, sub) {
			n++
		}
	}
	return n
}

// TestChannelConformanceFiresOnPositiveControl is the proof-it-can-fail run
// for the drift check, in-process. A clean fixture must produce zero findings;
// the SAME fixture with one retired invocation reintroduced must produce at
// least one. A check that has never been shown to go red is a check nobody
// knows is wired up.
func TestChannelConformanceFiresOnPositiveControl(t *testing.T) {
	const cleanDoc = "# Adopting\n\nInstall the pinned release binary named in .assay-versions.\n"

	clean := writeRoot(t, map[string]string{"README.md": cleanDoc, "docs/adopting-assay.md": cleanDoc})
	before := channelConformanceNotices(clean)
	if got := countContaining(before, "non-sanctioned channel"); got != 0 {
		t.Fatalf("clean fixture reported %d finding(s), want 0:\n%s", got, strings.Join(before, "\n"))
	}
	if countContaining(before, "channel-conformance summary:") != 1 {
		t.Error("clean run printed no summary line — a passing check that prints nothing is indistinguishable from one that did not run")
	}

	// Positive control, one per pattern. Each must go red on its own.
	for _, tc := range []struct{ id, line string }{
		{"vendored-go-run", "Regenerate: " + deadPath},
		{"vendor-copy-command", `Run cp -R "$SRC/statusgen/." "$TARGET/statusgen/" to install.`},
		{"vendor-copy-prose", "1. Copy `statusgen/` into your repo."},
		{"go-install", "Run go install github.com/example/statusgen@latest"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			mutated := writeRoot(t, map[string]string{
				"README.md":              cleanDoc,
				"docs/adopting-assay.md": cleanDoc + tc.line + "\n",
			})
			after := channelConformanceNotices(mutated)
			if got := countContaining(after, "non-sanctioned channel"); got < 1 {
				t.Fatalf("mutated fixture reported %d finding(s), want >=1:\n%s", got, strings.Join(after, "\n"))
			}
			if countContaining(after, "["+tc.id+"]") < 1 {
				t.Errorf("finding does not name the pattern %q that fired:\n%s", tc.id, strings.Join(after, "\n"))
			}
			// cleanDoc is 3 lines, so the injected line is line 4. The exact
			// number is asserted on purpose: "reported a finding" is weaker
			// than "reported the finding at the right place", and an
			// off-by-one locator sends the reader to the wrong line.
			if countContaining(after, "docs/adopting-assay.md:4") < 1 {
				t.Errorf("finding does not carry path:line:\n%s", strings.Join(after, "\n"))
			}
		})
	}
}

// TestChannelConformanceRetiredMarkerIsNotAdvice guards the false-positive
// direction. The runbook must be able to record what channel A WAS without the
// sweep reading the record as a recommendation — otherwise the only way to
// pass the check is to delete the reason the channel was retired.
func TestChannelConformanceRetiredMarkerIsNotAdvice(t *testing.T) {
	root := writeRoot(t, map[string]string{
		"docs/adopting-assay.md": "| A | Vendor the source — `cp -R \"$SRC/statusgen/.\" \"$T/statusgen/\"` | **RETIRED as a recommendation.** |\n",
	})
	got := channelConformanceNotices(root)
	if n := countContaining(got, "non-sanctioned channel"); n != 0 {
		t.Fatalf("a line marked RETIRED was reported as advice (%d finding(s)):\n%s", n, strings.Join(got, "\n"))
	}
}

// TestChannelConformanceCouldNotCheck is the three-state assertion
// (desk-hardening/01): a surface the sweep could not read must report
// could-not-check and must NOT be counted clean. This is the defect the
// invariant exists to prevent — a sweep that cannot open part of its corpus
// and prints a green line anyway.
func TestChannelConformanceCouldNotCheck(t *testing.T) {
	root := writeRoot(t, map[string]string{"README.md": "clean\n"})
	// A directory where a document is expected: readable metadata, unreadable
	// as text. Portable, and does not depend on running as non-root (a 0000
	// file is readable by root, so a permission-based fixture passes on a
	// developer laptop and silently no-ops on a root CI runner).
	if err := os.MkdirAll(filepath.Join(root, "docs", "adopting-assay.md"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := channelConformanceNotices(root)
	if countContaining(got, "COULD-NOT-CHECK") < 1 {
		t.Fatalf("unreadable surface produced no could-not-check line:\n%s", strings.Join(got, "\n"))
	}
	if countContaining(got, "docs/adopting-assay.md") < 1 {
		t.Errorf("could-not-check line does not name the path:\n%s", strings.Join(got, "\n"))
	}
	// And the summary must not count it clean.
	for _, l := range got {
		if strings.Contains(l, "channel-conformance summary:") && strings.Contains(l, "0 could-not-check") {
			t.Errorf("summary reports 0 could-not-check while a surface was unreadable: %s", l)
		}
	}
}

// TestChannelConformanceSummaryDeclaresItsBound asserts the no-silent-caps
// rule: the sweep bounds its coverage, so it must SAY what it dropped.
func TestChannelConformanceSummaryDeclaresItsBound(t *testing.T) {
	got := channelConformanceNotices(writeRoot(t, map[string]string{"README.md": "clean\n"}))
	var summary string
	for _, l := range got {
		if strings.Contains(l, "channel-conformance summary:") {
			summary = l
		}
	}
	if summary == "" {
		t.Fatal("no summary line")
	}
	for _, want := range []string{"DECLARED scope", "NOT scanned", "absent"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary omits %q — the cap would read as full coverage: %s", want, summary)
		}
	}
	// The mutation test in the brief counts "non-sanctioned channel" before and
	// after. A summary carrying the phrase would make the clean baseline 1.
	if strings.Contains(summary, "non-sanctioned channel") {
		t.Errorf("summary carries the finding phrase, poisoning the before/after count: %s", summary)
	}
}

// TestAcceptedDriftUnparseableIsBlindNotEmpty: an unreadable allowlist must
// never be read as an empty one.
func TestAcceptedDriftUnparseableIsBlindNotEmpty(t *testing.T) {
	root := writeRoot(t, map[string]string{
		"README.md":          "clean\n",
		acceptedDriftRelPath: "accepted: [ this is not: valid: yaml\n",
	})
	got := channelConformanceNotices(root)
	if countContaining(got, "COULD-NOT-CHECK") < 1 {
		t.Fatalf("unparseable accepted-drift register produced no could-not-check line:\n%s", strings.Join(got, "\n"))
	}
	if countContaining(got, "KNOWN-ACCEPTED") != 0 {
		t.Error("entries were reported as accepted from a register that did not parse")
	}
}

// TestAcceptedDriftIncompleteEntryIsBlind: an entry with no tracking reference
// is not auditable, so it must not be counted as an accepted exception.
func TestAcceptedDriftIncompleteEntryIsBlind(t *testing.T) {
	root := writeRoot(t, map[string]string{
		"README.md": "clean\n",
		acceptedDriftRelPath: "accepted:\n" +
			"  - id: complete\n    scope: cross-repo\n    where: somewhere\n    channel: A\n    why: because\n    tracking: some-issue\n" +
			"  - id: incomplete\n    scope: cross-repo\n    where: somewhere\n    channel: A\n    why: because\n",
	})
	got := channelConformanceNotices(root)
	if countContaining(got, "KNOWN-ACCEPTED [complete]") != 1 {
		t.Errorf("complete entry not reported:\n%s", strings.Join(got, "\n"))
	}
	if countContaining(got, "KNOWN-ACCEPTED [incomplete]") != 0 {
		t.Error("entry missing `tracking` was trusted as an accepted exception")
	}
	if countContaining(got, "missing tracking") < 1 {
		t.Errorf("incomplete entry produced no could-not-check line naming the missing field:\n%s", strings.Join(got, "\n"))
	}
}

// TestAcceptedDriftAbsentIsNotAnError: the register is a do-not-copy stream
// doc, so it does not exist in the published tree. Absent must be a clean,
// quiet state — not a could-not-check, which would make the published tree
// permanently blind-looking.
func TestAcceptedDriftAbsentIsNotAnError(t *testing.T) {
	got := channelConformanceNotices(writeRoot(t, map[string]string{"README.md": "clean\n"}))
	if countContaining(got, "COULD-NOT-CHECK") != 0 {
		t.Errorf("absent accepted-drift register reported as could-not-check:\n%s", strings.Join(got, "\n"))
	}
}

// TestChannelPatternsResolve is the derive-or-diff guard: every drift pattern
// must name a channel that exists in the ONE declared set, so a NOTICE can
// cite the reason rather than restate it.
func TestChannelPatternsResolve(t *testing.T) {
	for _, p := range channelDriftPatterns {
		ch, ok := channelByID(p.Channel)
		if !ok {
			t.Errorf("pattern %q names channel %q, absent from sanctionedChannelSet", p.ID, p.Channel)
			continue
		}
		if ch.Sanctioned {
			t.Errorf("pattern %q flags channel %q, which is SANCTIONED — the check would report correct advice as drift", p.ID, p.Channel)
		}
		if strings.TrimSpace(ch.Why) == "" {
			t.Errorf("channel %q carries no Why — a NOTICE citing it would assert without a reason", ch.ID)
		}
	}
	if sanctionedIDs() == "(none)" {
		t.Error("no channel is sanctioned — every adopter instruction would be a finding")
	}
}

// TestChannelSetMatchesAdopterRunbook is the derive-or-diff arm (brief rule:
// one declared source per fact; other copies regenerated or CI-diffed).
//
// sanctionedChannelSet is the declared source; docs/adopting-assay.md's
// "Channels that are NOT the default" table is a prose VIEW of it that a human
// reads. Prose cannot be regenerated from the struct without losing the
// runbook's voice, so it is CI-diffed instead: every declared ID must appear as
// a row there, and every channel this code calls NOT sanctioned must be marked
// as such in the prose. The failure this prevents is renumbering the letters in
// one place — they are a published contract an adopter cites.
func TestChannelSetMatchesAdopterRunbook(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "docs", "adopting-assay.md"))
	if os.IsNotExist(err) {
		t.Skip("adopter runbook not present in this tree")
	}
	if err != nil {
		// Present but unreadable is could-not-check, and a check that could not
		// look must not report clean.
		t.Fatalf("adopter runbook could not be read, so the channel tables were NOT diffed: %v", err)
	}
	doc := string(raw)

	// Locate the channel table's rows: "| **A** | ... |".
	rows := map[string]string{}
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| **") {
			continue
		}
		rest := strings.TrimPrefix(trimmed, "| **")
		i := strings.Index(rest, "**")
		if i <= 0 {
			continue
		}
		id := rest[:i]
		if len(id) == 1 && id[0] >= 'A' && id[0] <= 'Z' {
			rows[id] = trimmed
		}
	}
	if len(rows) == 0 {
		t.Fatal("no channel rows found in docs/adopting-assay.md — the diff could not be taken, so this is could-not-check, not a pass")
	}

	for _, c := range sanctionedChannelSet {
		row, ok := rows[c.ID]
		if !ok {
			// E is the default and is documented in the install PRIMITIVE
			// rather than in the "NOT the default" table, so its absence from
			// that table is expected; it must still be named somewhere.
			if c.Sanctioned && strings.Contains(doc, "channel "+c.ID) {
				continue
			}
			t.Errorf("channel %s (%s) is declared in statusgen/channels.go but appears in no row of the adopter runbook", c.ID, c.Name)
			continue
		}
		if !c.Sanctioned && !retiredMarkerRe.MatchString(row) {
			t.Errorf("channel %s is NOT sanctioned in code, but its runbook row carries no retired/not-sanctioned marker — the drift check would read that row as advice: %s", c.ID, row)
		}
	}
}

// TestRepoAcceptedDriftRegisterIsValid runs the loader against the register
// actually committed in this repo, so a typo in it is caught by `go test`
// rather than discovered as a silently-empty exception list in a lint run.
func TestRepoAcceptedDriftRegisterIsValid(t *testing.T) {
	root := ".."
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(acceptedDriftRelPath))); os.IsNotExist(err) {
		t.Skip("register not present in this tree (do-not-copy path; absent is a valid state)")
	}
	accepted, blind := loadAcceptedDrift(root)
	if len(blind) > 0 {
		t.Errorf("committed accepted-drift register produced could-not-check lines: %v", blind)
	}
	if len(accepted) == 0 {
		t.Error("committed accepted-drift register parsed to zero entries")
	}
	for _, a := range accepted {
		if _, ok := channelByID(a.Channel); !ok {
			t.Errorf("entry %q names channel %q, absent from the declared set", a.ID, a.Channel)
		}
	}
}
