package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2794
func TestReceiptAttribution(t *testing.T) {
	for _, fault := range []string{"valid", "missing", "blob", "moved", "wrong-author", "wrong-account", "unknown-author"} {
		t.Run(fault, func(t *testing.T) {
			f, _ := setupFake(t)
			res := &deskkit.WriteFileResult{Changed: true, ContentID: "blob-id", CommitSHA: "exact-commit", Author: verifierBotDisplay()}
			switch fault {
			case "missing":
				res.CommitSHA = ""
			case "blob":
				f.commitErr = errors.New("422: object is a blob")
			case "moved":
				f.commitSHAs = map[string]string{"exact-commit": "other-commit"}
			case "wrong-author":
				res.Author = "someone-else"
			case "wrong-account":
				f.commitAuthorLogin = "someone-else"
			case "unknown-author":
				res.Author = ""
			}
			detail, err := checkWriteAttribution(f, deskkit.ForgeRepo{}, res)
			if fault == "valid" || fault == "unknown-author" {
				if err != nil {
					t.Fatalf("%s: %v", detail, err)
				}
				if fault == "unknown-author" && detail != "attribution=could-not-check" {
					t.Fatalf("unknown author became proven: %s", detail)
				}
			} else if err == nil {
				t.Fatalf("%s receipt accepted: %s", fault, detail)
			}
			if fault != "missing" && (len(f.getCommitSHAs) != 1 || f.getCommitSHAs[0] != "exact-commit") {
				t.Fatalf("must dereference the exact commit once: %v", f.getCommitSHAs)
			}
		})
	}
}

// regression: #2794
func TestLandingCommitReceipt(t *testing.T) {
	for _, record := range []bool{false, true} {
		for _, draft := range []bool{false, true} {
			t.Run(strings.Join([]string{map[bool]string{false: "Evidence", true: "outcome"}[record], map[bool]string{false: "direct", true: "draft"}[draft]}, "/"), func(t *testing.T) {
				f, errBuf := setupFake(t)
				const commit = "1234567890abcdef1234567890abcdef12345678"
				f.writeReceipt = &deskkit.WriteFileResult{Changed: true, ContentID: "file-blob", CommitSHA: commit, Author: verifierBotDisplay()}
				// A draft may have advanced by the time it is opened. It is never our receipt.
				f.prHeadSHA = "later-commit"
				if draft {
					f.defaultBranch = "main"
					f.writeReceipt.Author = ""
					f.commitAuthorLogin = verifierBotDisplay()
				}
				var args []string
				if record {
					line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
					args = []string{"example-org/tracker", "main", "--outcome-record", writeRepoFile(t, "record.json", line+"\n")}
				} else {
					target := "docs/streams/x/brief.md"
					args = []string{"example-org/tracker", "main", "--evidence-file", target, "--root", rootWithFile(t, target, "row\n")}
				}
				if code := run(args); code != 0 {
					t.Fatalf("exit %d: %s", code, errBuf)
				}
				detail := lastAudit(t).Detail + "\n" + stdout.(*bytes.Buffer).String()
				if !strings.Contains(detail, commit) || strings.Contains(detail, "file-blob") || strings.Contains(detail, "later-commit") {
					t.Fatalf("wrong commit receipt: %s", detail)
				}
				if len(f.getCommitSHAs) != 1 || f.getCommitSHAs[0] != commit {
					t.Fatalf("attributed another object: %v", f.getCommitSHAs)
				}
			})
		}
	}
}

// regression: #2794
func TestWriteNoopReceipt(t *testing.T) {
	f, _ := setupFake(t)
	f.writeReceipt = &deskkit.WriteFileResult{ContentID: "existing-file"}
	target := "docs/streams/x/brief.md"
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", rootWithFile(t, target, "row\n")}); code != 0 {
		t.Fatalf("noop exit %d", code)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultNoop || strings.Contains(e.Detail, "committed") || f.getCommitCalls != 0 {
		t.Fatalf("noop became a commit: %+v calls=%d", e, f.getCommitCalls)
	}
}
