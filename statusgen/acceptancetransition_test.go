package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transitionFixture struct {
	root, path, base, delivery, amendment, descriptor, payload, amended, history string
	forge                                                                        *rlForge
}

func transitionFixtureAt(t *testing.T, merge ...bool) transitionFixture {
	t.Helper()
	return contextFixture(t, merge, nil)
}

func contextFixture(t *testing.T, merge []bool, edit func(string, string) string) transitionFixture {
	t.Helper()
	return contextHistoryFixture(t, merge, edit, nil)
}

func contextHistoryFixture(t *testing.T, merge []bool, edit func(string, string) string, historyEdit func(string) string) transitionFixture {
	t.Helper()
	change := func(stage, body string) string {
		if edit != nil {
			return edit(stage, body)
		}
		return body
	}
	o := vfDefaults()
	o.evidence = ""
	o.verify = strings.Replace(vfVerify, "`false`", "`sh check.sh`", 1)
	if len(merge) > 1 && merge[1] {
		o.verify = strings.Replace(o.verify, "| Expect |", "| Expect | Class |", 1)
		o.verify = strings.Replace(o.verify, "|---|---|---|", "|---|---|---|---|", 1)
		o.verify = strings.Replace(o.verify, "| exit 1 |", "| exit 1 | check |", 1)
	}
	o.status = "verified"
	o.verified = "2026-08-13 human:alex"
	root, _ := vfFixture(t, o)
	path := vfBrief(root)
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	body := change("original", strings.Replace(string(raw), "schema: brief-v1", "schema: brief-v2\nversion: 1", 1))
	if e = os.WriteFile(path, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "check.sh"), []byte("#!/bin/sh\n# Synthetic local check; the option tightens execution policy.\nexit 1\n"), 0644); e != nil {
		t.Fatal(e)
	}
	vfGit(t, root, vfMailHuman, "remote", "add", "origin", "https://github.com/"+rlRepo+".git")
	vfCommit(t, root, vfMailHuman)
	base := vfRev(t, root, "HEAD")
	// An unrelated delivery edit MUST NOT become accepted by this transition.
	body = strings.Replace(body, "| 1 | `true` | exit 0 |", "| 1 | `false` | exit 9 |", 1)
	body = strings.Replace(body, "\n\n## Evidence", "\n| 3 | `printf supplemental` | exit 0 |\n\n## Evidence", 1)
	body = change("delivery", body)
	if e = os.WriteFile(path, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	vfCommit(t, root, vfMailWorker)
	delivery := vfRev(t, root, "HEAD")
	history := "Historical result: row 2 failed; retained for diagnosis.\n"
	body = strings.Replace(body, "## Evidence\n", "## Evidence\n"+history, 1)
	if historyEdit != nil {
		body = historyEdit(body)
	}
	if e = os.WriteFile(path, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	vfCommit(t, root, vfMailVerifier)
	if len(merge) > 0 && merge[0] {
		vfGit(t, root, vfMailHuman, "checkout", "-q", "-b", "amendment")
	}
	body = strings.Replace(body, "`sh check.sh`", "`sh check.sh --no-pull`", 1)
	body = change("amendment", strings.Replace(body, "version: 1", "version: 2", 1))
	if e = os.WriteFile(path, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	vfCommit(t, root, vfMailHuman)
	if len(merge) > 0 && merge[0] {
		vfGit(t, root, vfMailHuman, "checkout", "-q", "main")
		vfGit(t, root, vfMailHuman, "merge", "--no-ff", "-q", "-m", "amendment", "amendment")
	}
	amendment := vfRev(t, root, "HEAD")
	descriptor := fmt.Sprintf("Verification-Table: delivery=%s base=%s original=%s\nAcceptance-Transition: amendment=%s approval=%s", delivery, base, base, amendment, rlURL(rlRepo, rlIssue, rlComment))
	payload := fmt.Sprintf("Approve acceptance-transition v1\nrepo=%s\nbrief=%s\ndelivery=%s\nbase=%s\noriginal=%s\namendment=%s\nrows=2", rlRepo, vfBriefRel, delivery, base, base, amendment)
	login, ok := HumanLogin("alex")
	if !ok {
		t.Fatal("fixture human missing")
	}
	forge := defaultRLForge()
	forge.commentBody = rlCommentJSONWith(login, "User", rlRepo, rlIssue, payload, rlPosted, rlPosted)
	forge.issueBody = rlIssueJSON("<!-- acceptance-transition: " + vfBriefRel + " -->")
	client := forge.client(t)
	oldClient, oldOpt := transitionClient, transitionForge
	transitionClient = func() *ghClient { return client }
	transitionForge = true
	t.Cleanup(func() { transitionClient = oldClient; transitionForge = oldOpt })
	return transitionFixture{root, path, base, delivery, amendment, descriptor, payload, body, history, forge}
}

func (f transitionFixture) evidence(t *testing.T, mutate func(string) string) {
	t.Helper()
	ev := f.descriptor + "\n\n" + strings.ReplaceAll(witnessTableFor(vfRow1, strings.Replace(vfRow2, "`false`", "`sh check.sh --no-pull`", 1)), vfSHA, f.amendment) + "\n" +
		"| 3 | `printf supplemental` | author-added; execution exit=0; no acceptance credit | sha256:bbbbbbbbbbbb | 2026-08-13 | " + vfRunner + " @ " + f.amendment + " |\n\n" + vfPass + "\n"
	if mutate != nil {
		ev = mutate(ev)
	}
	body := strings.Replace(f.amended, f.history, f.history+"\n"+ev, 1)
	if e := os.WriteFile(f.path, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
}

// regression: #3467
func TestAcceptanceTransition(t *testing.T) {
	cases := []string{"approved", "approved-merge", "approved-scp", "approved-ssh", "approved-crlf", "origin-gitlab", "origin-ghe", "origin-ssh-port", "issue-marker-missing", "issue-marker-other", "tampered-current", "changed-expect", "offline", "absent", "bot", "stale", "edited", "other-brief", "wrong-base", "wrong-original", "wrong-amendment", "missing-row", "old-witness", "supplement-credit", "original-launder", "missing-provenance", "duplicate", "fenced", "quoted", "lost-history"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			f := transitionFixtureAt(t, name == "approved-merge")
			f.evidence(t, func(ev string) string {
				switch name {
				case "wrong-base":
					return strings.Replace(ev, "base="+f.base, "base="+f.delivery, 1)
				case "wrong-original":
					return strings.Replace(ev, "original="+f.base, "original="+f.delivery, 1)
				case "wrong-amendment":
					return strings.Replace(ev, "amendment="+f.amendment, "amendment="+f.base, 1)
				case "missing-row":
					return strings.Replace(ev, strings.ReplaceAll(vfRow1, vfSHA, f.amendment), "", 1)
				case "old-witness":
					return strings.ReplaceAll(ev, " @ "+f.amendment, " @ "+f.delivery)
				case "original-launder":
					return strings.Replace(ev, "pass exit=0", "author-added; execution exit=0; no acceptance credit", 1)
				case "supplement-credit":
					return strings.Replace(ev, "author-added; execution exit=0; no acceptance credit", "pass exit=0", 1)
				case "missing-provenance":
					return strings.Join(strings.Split(ev, "\n")[1:], "\n")
				case "fenced":
					return strings.Replace(ev, "Acceptance-Transition:", "```\nAcceptance-Transition:", 1) + "\n```"
				case "duplicate":
					return f.descriptor + "\n" + ev
				case "quoted":
					return strings.Replace(ev, "Acceptance-Transition:", "> Acceptance-Transition:", 1)
				}
				return ev
			})
			switch name {
			case "approved-scp":
				vfGit(t, f.root, vfMailHuman, "remote", "set-url", "origin", "git@github.com:"+rlRepo+".git")
			case "approved-ssh":
				vfGit(t, f.root, vfMailHuman, "remote", "set-url", "origin", "ssh://git@github.com:22/"+rlRepo+".git")
			case "approved-crlf":
				f.forge.commentBody = strings.ReplaceAll(f.forge.commentBody, `\n`, `\r\n`)
			case "origin-gitlab":
				vfGit(t, f.root, vfMailHuman, "remote", "set-url", "origin", "https://gitlab.example/"+rlRepo+".git")
			case "origin-ghe":
				vfGit(t, f.root, vfMailHuman, "remote", "set-url", "origin", "git@ghe.example:"+rlRepo+".git")
			case "origin-ssh-port":
				vfGit(t, f.root, vfMailHuman, "remote", "set-url", "origin", "ssh://git@other.example:2222/"+rlRepo+".git")
			case "issue-marker-missing":
				f.forge.issueBody = rlIssueJSON("Decision without an acceptance-transition marker.")
			case "issue-marker-other":
				f.forge.issueBody = rlIssueJSON("<!-- acceptance-transition: docs/streams/vf/brief-02-other.md -->")
			case "offline":
				transitionForge = false
			case "absent":
				f.forge.commentStatus = 404
			case "bot":
				f.forge.commentBody = rlCommentJSONWith(vfRunner, "Bot", rlRepo, rlIssue, f.payload, rlPosted, rlPosted)
			case "stale":
				f.forge.commentBody = strings.ReplaceAll(f.forge.commentBody, f.amendment, f.delivery)
			case "edited":
				f.forge.commentBody = strings.Replace(f.forge.commentBody, `"updated_at":"`+rlPosted, `"updated_at":"2026-10-01T10:00:00Z`, 1)
			case "other-brief":
				f.forge.commentBody = strings.ReplaceAll(f.forge.commentBody, vfBriefRel, "docs/streams/vf/brief-02-other.md")
			case "tampered-current", "changed-expect":
				raw, _ := os.ReadFile(f.path)
				before, after := "`sh check.sh --no-pull`", "`true`"
				if name == "changed-expect" {
					before, after = "exit 1", "exit 0"
				}
				os.WriteFile(f.path, []byte(strings.Replace(string(raw), before, after, 1)), 0644)
				vfCommit(t, f.root, vfMailHuman)
			case "lost-history":
				raw, _ := os.ReadFile(f.path)
				os.WriteFile(f.path, []byte(strings.Replace(string(raw), f.history, "", 1)), 0644)
				vfCommit(t, f.root, vfMailVerifier)
			}
			verify, evidence, e := briefSections(f.path)
			if e != nil {
				t.Fatal(e)
			}
			findings, why := closureWitnesses(verify, evidence, f.path)
			pass := why == "" && len(findings) == 3
			for _, v := range findings {
				pass = pass && closureSatisfied(v)
			}
			want := strings.HasPrefix(name, "approved")
			if pass != want {
				t.Fatalf("closure accepted=%v want=%v: %s %+v", pass, want, why, findings)
			}
			streams, _, e := loadStreams(f.root)
			if e != nil {
				t.Fatal(e)
			}
			cov := evaluateCoverage(f.root, streams, coverageOptions{Revision: f.amendment})
			if cov["vf/01"].Released != want {
				t.Fatalf("coverage: %+v", cov["vf/01"])
			}
			problems := protectedClosureProblems(streams)
			if (len(problems) == 0) != want {
				t.Fatalf("lint: %v", problems)
			}
			data, _, e := loadReconcileFoldData(f.root)
			if e != nil || len(data) != 1 {
				t.Fatalf("fold input: %v", e)
			}
			_, ok, why := foldVerified(data[0], cov)
			if ok != want {
				t.Fatalf("reconcile accepted=%v: %s", ok, why)
			}
			args := []string{"--root", f.root, "--brief", "vf/01"}
			if transitionForge {
				args = append(args, "--acceptance-forge")
			}
			code, out, diag := captureVerifyclosure(t, args)
			if (code == 0) != want {
				t.Fatalf("verifyclosure: %d %s %s", code, out, diag)
			}
			args = []string{"--check", "--brief", f.path}
			if transitionForge {
				args = append(args, "--acceptance-forge")
			}
			res, diag := captureVerifyrun(t, args)
			if (res.code == 0) != want {
				t.Fatalf("verifyrun: %d %s", res.code, diag)
			}
			readme := filepath.Join(f.root, "docs/streams/vf/README.md")
			raw, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			pending := strings.Replace(string(raw), "| verified | 2026-08-13 human:alex |", "| implemented | — |", 1)
			if err = os.WriteFile(readme, []byte(pending), 0644); err != nil {
				t.Fatal(err)
			}
			resetReadMemo()
			_, err = planVerifyFlip(f.root, "vf/01", f.amendment, vfRunner)
			if (err == nil) != want {
				t.Fatalf("status promotion: %v accepted=%v", err, want)
			}

		})
	}
}

// regression: #3467
func TestTransitionExecution(t *testing.T) {
	runTransitionExecution(t, transitionFixtureAt(t))
}

func runTransitionExecution(t *testing.T, f transitionFixture) {
	t.Helper()
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	descriptor := filepath.Join(t.TempDir(), "transition.txt")
	if err := os.WriteFile(descriptor, []byte(f.descriptor), 0600); err != nil {
		t.Fatal(err)
	}
	result, diag := captureVerifyrun(t, []string{"--brief", f.path, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge"})
	if result.code != 0 {
		t.Fatalf("execution: %d %s", result.code, diag)
	}
	_, ev, err := briefSections(f.path)
	if err != nil {
		t.Fatal(err)
	}
	rows := parseEvidenceRows(ev)
	for id, command := range map[string]string{"1": "true", "2": "sh check.sh --no-pull", "3": "printf supplemental"} {
		got := rows[id]
		if len(got) != 1 || witnessCommandOf(got[0].Text) != command || witnessTreeOf(got[0].Text) != f.amendment {
			t.Fatalf("row %s: %+v", id, got)
		}
	}
	if !strings.Contains(rows["3"][0].Text, "author-added; execution exit=0; no acceptance credit") {
		t.Fatalf("supplemental: %s", rows["3"][0].Text)
	}
	// The runner does not create a verdict: an independent verifier inspects the
	// primary output and finalizes the retained record before closure.
	if _, why := closureWitnesses(extractSectionByPrefix(f.amended, "Verify"), ev, f.path); why == "" {
		t.Fatal("unfinished execution was accepted as a completed independent run")
	}
	// Finalize the actual runner output under the independent verifier identity.
	vfAppend(t, f.root, "\n\n"+vfPass)
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	code, out, diag := captureVerifyclosure(t, []string{"--root", f.root, "--brief", "vf/01", "--acceptance-forge"})
	if code != 0 {
		t.Fatalf("completed actual execution: %d %s %s", code, out, diag)
	}
	streams, _, err := loadStreams(f.root)
	if err != nil {
		t.Fatal(err)
	}
	cov := evaluateCoverage(f.root, streams, coverageOptions{Revision: f.amendment})
	if !cov["vf/01"].Released {
		t.Fatalf("actual execution coverage: %+v", cov["vf/01"])
	}
}

// regression: #3467
func TestTransitionCISkip(t *testing.T) {
	f := transitionFixtureAt(t, false, true)
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	descriptor := filepath.Join(t.TempDir(), "transition.txt")
	if err := os.WriteFile(descriptor, []byte(f.descriptor), 0600); err != nil {
		t.Fatal(err)
	}
	result, diag := captureVerifyrun(t, []string{"--brief", f.path, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge", "--ci"})
	if result.code != 0 {
		t.Fatalf("CI execution: %d %s", result.code, diag)
	}
	_, evidence, err := briefSections(f.path)
	if err != nil || len(parseEvidenceRows(evidence)["2"]) != 0 {
		t.Fatalf("CI fabricated row 2 witness: %s %v", evidence, err)
	}
	vfAppend(t, f.root, "\n\n"+vfPass)
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	code, out, diag := captureVerifyclosure(t, []string{"--root", f.root, "--brief", "vf/01", "--acceptance-forge"})
	if code == 0 {
		t.Fatalf("CI skip conferred closure: %s %s", out, diag)
	}
	streams, _, err := loadStreams(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if cov := evaluateCoverage(f.root, streams, coverageOptions{Revision: f.amendment}); cov["vf/01"].Released {
		t.Fatalf("CI skip conferred coverage: %+v", cov["vf/01"])
	}
}

// regression: #4596
func TestTransitionRootPaths(t *testing.T) {
	for _, mode := range []string{"dot", "nested", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			f := transitionFixtureAt(t)
			f.evidence(t, nil)
			root := "."
			cwd := f.root
			switch mode {
			case "nested":
				cwd = filepath.Join(f.root, "docs", "streams", "vf")
				root = "../../.."
			case "symlink":
				root = filepath.Join(t.TempDir(), "checkout")
				if err := os.Symlink(f.root, root); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			t.Chdir(cwd)
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			if problems := protectedClosureProblems(streams); len(problems) != 0 {
				t.Fatalf("relative-root lint: %v", problems)
			}
			code, out, diag := captureVerifyclosure(t, []string{"--root", root, "--brief", "vf/01", "--acceptance-forge"})
			if code != 0 {
				t.Fatalf("relative-root closure: %d %s %s", code, out, diag)
			}
		})
	}
}

// regression: #4246
func TestTransitionRelativeBrief(t *testing.T) {
	tests := []struct {
		name string
		// brief is the --brief value spelled against the row's working
		// directory; empty keeps the fixture's absolute path.
		brief string
		// rootJoin builds the row's --brief as the fixture repository's
		// root, a slash, and the brief spelling, by plain string
		// concatenation: the raw absolute form of the same spelling, with
		// .. preserved where filepath.Join would clean it away.
		rootJoin bool
		// sub runs the row from a fresh subdirectory of the fixture
		// repository instead of its root.
		sub bool
		// prepare stages the row's filesystem shape once the approving
		// evidence has landed and before the run.
		prepare func(t *testing.T, f transitionFixture)
		// wantCode is the verifyrun exit code; 0 expects acceptance.
		wantCode int
		// want is the refusal substring the run must print; empty expects
		// acceptance, so a refusal cannot hide behind a nonzero code alone.
		want string
		// record drives the row through --acceptance-record instead of
		// --check: a --check audit blames the brief's working-tree file,
		// which a symlinked brief takes out of the repository before the
		// transition's brief-path check ever runs.
		record bool
	}{
		{
			// Sanity: the absolute fixture path from the root is accepted.
			name:     "approved-abs",
			wantCode: 0,
		},
		{
			// The same file named relatively from the root must be accepted
			// with the identical repository-relative payload binding.
			name:     "approved-relative-from-root",
			brief:    vfBriefRel,
			wantCode: 0,
		},
		{
			// The same file named with .. segments from a subdirectory: the
			// outcome and the payload's brief= value must not depend on where
			// the invocation ran from.
			name:     "approved-relative-from-subdir",
			brief:    "../docs/streams/vf/brief-01-flip.md",
			sub:      true,
			wantCode: 0,
		},
		{
			// A relative path naming no file is refused before the transition
			// check: the brief cannot even be read.
			name:     "nonexistent-relative",
			brief:    "docs/streams/vf/no-such-brief.md",
			wantCode: verifyrunExitCouldNot,
			want:     "no such file or directory",
		},
		{
			// A fully approving brief copy outside the repository, addressed
			// with .. segments from a subdirectory: the provenance reads in
			// the brief's directory cannot leave the repository, so the run
			// refuses before the transition's brief-path check.
			name:  "dotdot-escape",
			brief: "../../outside-brief.md",
			sub:   true,
			prepare: func(t *testing.T, f transitionFixture) {
				briefBesideRoot(t, f)
			},
			wantCode: verifyrunExitCouldNot,
			want:     "cannot resolve HEAD as an immutable commit",
		},
		{
			// A brief path that resolves through a symlink whose target sits
			// outside the repository: the canonical path lands outside the
			// root and the transition refuses it. Pinned by its exact message,
			// so a mutant that skips the symlink resolution or the local-path
			// check turns this row red through a different refusal.
			name:   "symlink-escape",
			brief:  vfBriefRel,
			record: true,
			prepare: func(t *testing.T, f transitionFixture) {
				outside := briefBesideRoot(t, f)
				if err := os.Remove(f.path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, f.path); err != nil {
					t.Fatal(err)
				}
			},
			wantCode: verifyrunExitCouldNot,
			want:     "protected table: acceptance transition: cannot resolve brief path",
		},
		{
			// The same escape spelled absolutely: a relative spelling is the
			// working directory plus the raw spelling, uncleaned, so both
			// spellings resolve through the same symlinks and the refusal
			// must not depend on how the command named the brief.
			name:   "symlink-escape-abs",
			record: true,
			prepare: func(t *testing.T, f transitionFixture) {
				outside := briefBesideRoot(t, f)
				if err := os.Remove(f.path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, f.path); err != nil {
					t.Fatal(err)
				}
			},
			wantCode: verifyrunExitCouldNot,
			want:     "protected table: acceptance transition: cannot resolve brief path",
		},
		{
			// A relative brief that escapes through a symlinked directory
			// component followed by ..: the spelling resolves to the
			// byte-identical copy outside the repository, while lexical
			// path cleaning hides the escape and admits the in-repository
			// brief. The refusal must consult the raw spelling; the row is
			// pinned by its exact message.
			name:     "symlink-dotdot-escape",
			brief:    "docs/streams/vf/esc/../brief-01-flip.md",
			record:   true,
			prepare:  symlinkDotdotEscape,
			wantCode: verifyrunExitCouldNot,
			want:     "protected table: acceptance transition: cannot resolve brief path",
		},
		{
			// The same escape spelled absolutely: the raw form is the
			// working directory plus the relative spelling, uncleaned, so
			// the refusal cannot depend on how the command named the brief.
			name:     "symlink-dotdot-escape-abs",
			brief:    "docs/streams/vf/esc/../brief-01-flip.md",
			rootJoin: true,
			record:   true,
			prepare:  symlinkDotdotEscape,
			wantCode: verifyrunExitCouldNot,
			want:     "protected table: acceptance transition: cannot resolve brief path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := transitionFixtureAt(t)
			f.evidence(t, nil)
			if tt.prepare != nil {
				tt.prepare(t, f)
			}
			dir := f.root
			if tt.sub {
				dir = filepath.Join(f.root, "sub")
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			brief := tt.brief
			if tt.rootJoin {
				brief = f.root + "/" + tt.brief
			}
			if brief == "" {
				brief = f.path
			}
			var res verifyrunResult
			var out string
			if tt.record {
				// The acceptance-record driver refuses before any row runs
				// when the transition rejects the brief path, so the identity
				// it would run rows as never matters here; it is pinned to the
				// verifier the sibling execution test uses.
				vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
				vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
				descriptor := filepath.Join(t.TempDir(), "transition.txt")
				if err := os.WriteFile(descriptor, []byte(f.descriptor), 0o600); err != nil {
					t.Fatal(err)
				}
				res, out = captureVerifyrunOut(t, []string{"--brief", brief, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge"})
			} else {
				res, out = captureVerifyrunOut(t, []string{"--check", "--brief", brief, "--acceptance-forge"})
			}
			if res.code != tt.wantCode {
				t.Fatalf("verifyrun: %d, want %d (output: %s)", res.code, tt.wantCode, out)
			}
			if tt.want != "" && !strings.Contains(out, tt.want) {
				t.Fatalf("output = %q, want it to contain %q", out, tt.want)
			}
		})
	}
}

// briefBesideRoot copies the row's fully approving brief to a sibling of the
// fixture repository's root, outside the repository, and returns its path.
func briefBesideRoot(t *testing.T, f transitionFixture) string {
	t.Helper()
	outside := filepath.Join(f.root, "..", "outside-brief.md")
	raw, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(outside) })
	return outside
}

// symlinkDotdotEscape stages the symlink-dotdot-escape rows' filesystem: a
// byte-identical copy of the approving brief outside the repository, and an
// esc symlink in the brief's own directory pointing at a directory outside
// the root, so esc/../<basename> resolves physically to the outside copy.
// The symlink is excluded in the fixture repository's .git/info/exclude, so
// the untracked link cannot trip the later clean-source guard: the escape
// must reach the transition's brief-path check, not be refused for dirt
// first.
func symlinkDotdotEscape(t *testing.T, f transitionFixture) {
	t.Helper()
	out := t.TempDir()
	deep := filepath.Join(out, "deep")
	if err := os.Mkdir(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, filepath.Base(vfBriefRel)), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	escAbs := filepath.Join(filepath.Dir(f.path), "esc")
	escRel := filepath.Join(filepath.Dir(vfBriefRel), "esc")
	if err := os.Symlink(deep, escAbs); err != nil {
		t.Fatal(err)
	}
	exclude, err := os.ReadFile(filepath.Join(f.root, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, ".git", "info", "exclude"), append(exclude, escRel+"\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
}
