package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These fixtures drive the public host/container/host command path. Admission
// and approval are fakes; no container engine, real forge or network is used.
type transBracketFixture struct {
	containerFixture
	transitionFixture
	record string
}

func newTransBracket(t *testing.T, drift bool) transBracketFixture {
	t.Helper()
	c := newContainerFixture(t)
	f := contextFixture(t, nil, func(stage, body string) string {
		if drift {
			if stage == "original" {
				body = strings.Replace(body, "| 1 | `true` | exit 0 |", "| 1 | `printf drift` | exit 0; output is `original` |", 1)
			}
			if stage == "delivery" {
				body = strings.Replace(body, "output is `original`", "output is `drift`", 1)
			}
		}
		// Every row uses the real container sandbox precheck, including the
		// supplemental, while fixtures stay shell-only and offline.
		if stage == "original" {
			if !drift {
				body = strings.Replace(body, "`true`", "`touch "+c.marker+"`", 1)
			}
			body = strings.Replace(body, "| # | Command | Expect |", "| # | Command | Expect | Class |", 1)
			body = strings.Replace(body, "|---|---|---|", "|---|---|---|---|", 1)
			body = strings.Replace(body, "| exit 1 |", "| exit 1 | check:ci |", 1)
			if drift {
				body = strings.Replace(body, "output is `original` |", "output is `original` | check:ci |", 1)
			} else {
				body = strings.Replace(body, "| exit 0 |", "| exit 0 | check:ci |", 1)
			}
		}
		if stage == "delivery" {
			body = strings.Replace(body, "`printf supplemental` | exit 0 |", "`printf supplemental` | exit 0 | check:ci |", 1)
		}
		return body
	})
	c.home, c.brief = f.root, f.path
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	putFile(t, c.check, checkJSON(1, fixtureRun, f.amendment, vfBriefRel, "m"))
	record := filepath.Join(t.TempDir(), "record.txt")
	putFile(t, record, f.descriptor+"\n")
	oldNow := nowFunc
	nowFunc = func() time.Time { v, _ := time.Parse(time.RFC3339, fixtureOpened); return v }
	t.Cleanup(func() { nowFunc = oldNow })
	return transBracketFixture{c, f, record}
}

func (f transBracketFixture) open(t *testing.T) (int, string) {
	t.Helper()
	r, diag := captureVerifyrun(t, []string{"--open-bracket", "--root", f.home, "--brief", f.brief, "--out", f.receipt, "--acceptance-record", f.record, "--acceptance-forge"})
	return r.code, diag
}

func (f transBracketFixture) start(t *testing.T) {
	t.Helper()
	if code, diag := f.open(t); code != 0 {
		t.Fatalf("open: %d %s", code, diag)
	}
}

func (f transBracketFixture) run(t *testing.T, extra ...string) (int, string) {
	t.Helper()
	args := append(f.runArgs(), "--acceptance-record", f.record)
	r, diag := captureVerifyrun(t, append(args, extra...))
	return r.code, diag
}

func (f transBracketFixture) close(t *testing.T, record bool) (int, string) {
	t.Helper()
	nowFunc = func() time.Time { v, _ := time.Parse(time.RFC3339, fixtureOpened); return v.Add(time.Minute) }
	args := []string{"--close-bracket", "--root", f.home, "--brief", f.brief, "--admission-receipt", f.receipt, "--witness", f.witness}
	if record {
		args = append(args, "--acceptance-record", f.record, "--acceptance-forge")
	}
	r, diag := captureVerifyrun(t, args)
	return r.code, diag
}

func (f transBracketFixture) editReceipt(t *testing.T, edit func(*bracketReceipt)) {
	t.Helper()
	var r bracketReceipt
	if err := json.Unmarshal([]byte(getFile(t, f.receipt)), &r); err != nil {
		t.Fatal(err)
	}
	edit(&r)
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, f.receipt, string(raw)+"\n")
}

// regression: #1800
func TestOpenBracketTransition(t *testing.T) {
	for _, name := range []string{"approved", "drift", "unapproved", "edited", "unreadable", "origin", "dirty", "source", "no-forge"} {
		t.Run(name, func(t *testing.T) {
			f := newTransBracket(t, name == "drift")
			switch name {
			case "unapproved":
				f.forge.commentBody = strings.ReplaceAll(f.forge.commentBody, "rows=2", "rows=1")
			case "edited":
				f.forge.commentBody = strings.Replace(f.forge.commentBody, `"updated_at":"`+rlPosted, `"updated_at":"2026-10-01T10:00:00Z`, 1)
			case "unreadable":
				f.forge.commentStatus = 503
			case "origin":
				vfGit(t, f.home, vfMailHuman, "remote", "set-url", "origin", "https://elsewhere.invalid/"+rlRepo+".git")
			case "dirty":
				putFile(t, filepath.Join(f.home, "check.sh"), "exit 0\n")
			case "source":
				putFile(t, f.check, checkJSON(1, fixtureRun, f.base, vfBriefRel, "m"))
			}
			var code int
			var diag string
			if name == "no-forge" {
				r, d := captureVerifyrun(t, []string{"--open-bracket", "--root", f.home, "--brief", f.brief, "--out", f.receipt, "--acceptance-record", f.record})
				code, diag = r.code, d
			} else {
				code, diag = f.open(t)
			}
			if name != "approved" && name != "drift" {
				if code == 0 || getFile(t, f.receipt) != "" {
					t.Fatalf("refusal wrote receipt: %d %s", code, diag)
				}
				return
			}
			if code != 0 {
				t.Fatalf("open: %d %s", code, diag)
			}
			var r bracketReceipt
			if err := json.Unmarshal([]byte(getFile(t, f.receipt)), &r); err != nil {
				t.Fatal(err)
			}
			a := r.Acceptance
			if a == nil || a.DescriptorSHA != acceptanceSHA([]byte(getFile(t, f.record))) || a.Source != f.amendment || strings.Join(a.RunSet, ",") != "1,2" || a.Changes != "2=Command" || strings.Join(a.Added, ",") != "3" || a.VerifySHA != acceptanceSHA([]byte(a.Verify)) || a.Original != f.base || a.Revisions["2"] != f.amendment {
				t.Fatalf("receipt omitted the host plan: %+v", a)
			}
			if name == "drift" && !strings.Contains(a.Verify, "output is `original`") {
				t.Fatal("unselected Expect replaced original obligation")
			}
		})
	}
	t.Run("legacy bytes", func(t *testing.T) {
		f := newContainerFixture(t)
		putFile(t, f.check, validCheck)
		oldNow := nowFunc
		nowFunc = func() time.Time { v, _ := time.Parse(time.RFC3339, fixtureOpened); return v }
		defer func() { nowFunc = oldNow }()
		res, diag := captureVerifyrun(t, []string{"--open-bracket", "--root", f.home, "--brief", f.brief, "--out", f.receipt})
		if res.code != 0 {
			t.Fatalf("ordinary open: %d %s", res.code, diag)
		}
		tree, _ := homeTreeDigest(f.home)
		want := fmt.Sprintf("{\"check\":%s,\"tree\":%q,\"opened\":%q,\"runner\":\"human:fixture\",\"runner_source\":\"git-config\",\"runner_tree\":%q,\"on_behalf_of\":\"\"}\n", validCheck, tree, fixtureOpened, briefTreeSHA(f.home, f.brief))
		if got := getFile(t, f.receipt); got != want {
			t.Fatalf("ordinary receipt bytes changed:\ngot %s\nwant %s", got, want)
		}
	})
}

// regression: #1800
func TestContainerNetnsTransition(t *testing.T) {
	for _, drift := range []bool{false, true} {
		t.Run(fmt.Sprint("drift=", drift), func(t *testing.T) {
			f := newTransBracket(t, drift)
			f.start(t)
			beforeDD := getFile(t, f.log)
			// Poison all authority subprocesses during the container arm only.
			// Any incidental Git launch is a failure even if its caller ignores it.
			bin := t.TempDir()
			launches := filepath.Join(t.TempDir(), "launches")
			for _, name := range []string{"git", "gh", "deskdispatch"} {
				putFile(t, filepath.Join(bin, name), "#!/bin/sh\necho "+name+" >> "+launches+"\nexit 99\n")
				if err := os.Chmod(filepath.Join(bin, name), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			oldPath := os.Getenv("PATH")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+oldPath)
			client := transitionClient
			transitionClient = func() *ghClient { t.Fatal("container tried to read approval"); return nil }
			code, diag := f.run(t)
			transitionClient = client
			t.Setenv("PATH", oldPath)
			want := verifyrunExitPass
			if drift {
				want = verifyrunExitFail
			}
			if code != want {
				t.Fatalf("container: %d want %d: %s", code, want, diag)
			}
			if calls := getFile(t, launches); calls != "" {
				t.Fatalf("container launched authority: %s", calls)
			}
			if getFile(t, f.log) != beforeDD {
				t.Fatal("container repeated admission")
			}
			body := getFile(t, f.witness)
			for _, required := range []string{f.descriptor, "Acceptance-Changes: 2=Command", treeDigestPrefix, containerImagePrefix, attestationPrefix, "author-added; execution", " @ " + f.amendment} {
				if !strings.Contains(body, required) {
					t.Fatalf("missing %q:\n%s", required, body)
				}
			}
			if drift && !strings.Contains(body, "| 1 | `printf drift` | fail exit=0 sandbox=container-netns |") {
				t.Fatalf("row was not judged against original Expect:\n%s", body)
			}
			if code, diag := f.close(t, true); code != 0 {
				t.Fatalf("close: %d %s", code, diag)
			}
			body = getFile(t, f.witness)
			if strings.Count(body, admissionBracketPrefix) != 1 {
				t.Fatalf("wrong bracket count:\n%s", body)
			}
			res, diag := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", f.witness})
			if res.code != 0 || !strings.Contains(diag, "verdict=match") || !strings.Contains(diag, "span-lines=tree-digest:1,container-image:1,attestation:1") {
				t.Fatalf("digest: %+v %s", res, diag)
			}
		})
	}
}

// regression: #1800
func TestTransitionBracketRefusals(t *testing.T) {
	for _, name := range []string{"no-receipt", "no-plan", "descriptor", "table-digest", "run-set", "added", "rows", "forge", "dropped", "version2", "in-container", "check"} {
		t.Run("container/"+name, func(t *testing.T) {
			f := newTransBracket(t, false)
			f.start(t)
			args := append(f.runArgs(), "--acceptance-record", f.record)
			switch name {
			case "no-receipt":
				args = f.containerArgs("--acceptance-record", f.record, "--container-image-digest", fixtureImage, "--witness", f.witness)
			case "no-plan":
				f.editReceipt(t, func(r *bracketReceipt) { r.Acceptance = nil })
			case "descriptor":
				putFile(t, f.record, f.descriptor+"\n\n")
			case "table-digest":
				f.editReceipt(t, func(r *bracketReceipt) { r.Acceptance.Verify += "\n" })
			case "run-set":
				f.editReceipt(t, func(r *bracketReceipt) { r.Acceptance.RunSet = []string{"1"} })
			case "added":
				f.editReceipt(t, func(r *bracketReceipt) { r.Acceptance.Added = []string{"4"} })
			case "rows":
				args = append(args, "--rows", "3")
			case "forge":
				args = append(args, "--acceptance-forge")
			case "dropped":
				args = f.runArgs()
			case "version2":
				f.editReceipt(t, func(r *bracketReceipt) {
					r.Check = json.RawMessage(checkJSON(2, fixtureRun, f.amendment, vfBriefRel, "m"))
				})
			case "in-container", "check":
				args = []string{"--root", f.home, "--brief", f.brief, "--acceptance-record", f.record, "--" + name}
			}
			r, diag := captureVerifyrun(t, args)
			if r.code == 0 || getFile(t, f.witness) != "" {
				t.Fatalf("not refused before witness: %d %s", r.code, diag)
			}
			f.containerFixture.assertNothingRan(t, getFile(t, f.brief))
			if (name == "in-container" || name == "check") && !strings.Contains(diag, "requires host execution and same-repository admission") {
				t.Fatalf("legacy refusal changed: %s", diag)
			}
		})
	}
	for _, name := range []string{"edited-approval", "unreadable-approval", "replan", "descriptor", "changes", "command", "table-bytes", "no-record", "ordinary-descriptor", "ordinary-changes", "no-plan", "no-descriptor", "no-changes"} {
		t.Run("close/"+name, func(t *testing.T) {
			f := newTransBracket(t, false)
			f.start(t)
			if code, diag := f.run(t); code != 0 {
				t.Fatalf("container: %d %s", code, diag)
			}
			record := true
			body := getFile(t, f.witness)
			switch name {
			case "edited-approval":
				f.forge.commentBody = strings.Replace(f.forge.commentBody, `"updated_at":"`+rlPosted, `"updated_at":"2026-10-01T10:00:00Z`, 1)
			case "unreadable-approval":
				f.forge.commentStatus = 503
			case "replan":
				f.editReceipt(t, func(r *bracketReceipt) { r.Acceptance.Changes = "1=Expect" })
			case "descriptor":
				body = strings.Replace(body, "amendment="+f.amendment, "amendment="+f.delivery, 1)
			case "changes":
				body = strings.Replace(body, "2=Command", "1=Expect", 1)
			case "command":
				body = strings.Replace(body, "`sh check.sh --no-pull`", "`true`", 1)
			case "table-bytes":
				f.editReceipt(t, func(r *bracketReceipt) {
					r.Acceptance.Verify += "\n"
					r.Acceptance.VerifySHA = acceptanceSHA([]byte(r.Acceptance.Verify))
				})
			case "no-record":
				record = false
			case "ordinary-descriptor":
				record = false
				f.editReceipt(t, func(r *bracketReceipt) { r.Acceptance = nil })
				body = strings.Replace(body, "Acceptance-Changes: 2=Command\n", "", 1)
			case "ordinary-changes":
				record = false
				f.editReceipt(t, func(r *bracketReceipt) { r.Acceptance = nil })
				body = strings.Replace(body, f.descriptor+"\n", "", 1)
			case "no-plan":
				f.editReceipt(t, func(r *bracketReceipt) { r.Acceptance = nil })
			case "no-descriptor":
				body = strings.Replace(body, f.descriptor+"\n", "", 1)
			case "no-changes":
				body = strings.Replace(body, "Acceptance-Changes: 2=Command\n", "", 1)
			}
			putFile(t, f.witness, body)
			code, diag := f.close(t, record)
			if code == 0 || getFile(t, f.witness) != body {
				t.Fatalf("close did not refuse atomically: %d %s", code, diag)
			}
		})
	}
}
