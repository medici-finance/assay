package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// serveEvidenceGraphQL extends the existing offline attestation API with the
// batched read. The Contents handler enforces file compare-and-swap. All admission, custody,
// outbound checks and publication wiring remain real in these tests.
func (a *attestationAPI) serveEvidenceGraphQL(w http.ResponseWriter, r *http.Request) bool {
	var in struct {
		Query     string
		Variables struct {
			File   string
			Branch string
		}
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return false
	}
	enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if a.head == "" {
		a.head = strings.Repeat("a", 40)
	}
	if strings.Contains(in.Query, "query EvidenceLandingSnapshot") {
		a.calls[len(a.calls)-1].Operation = "snapshot"
		a.branch = strings.TrimPrefix(in.Variables.Branch, "refs/heads/")
		labels := []any{}
		for _, l := range a.labels {
			labels = append(labels, map[string]any{"name": l})
		}
		events := []any{}
		for _, ev := range a.timeline {
			typ := "LabeledEvent"
			if ev["event"] == "unlabeled" {
				typ = "UnlabeledEvent"
			}
			events = append(events, map[string]any{"__typename": typ, "label": ev["label"], "actor": map[string]any{"login": "assay-desk-app", "__typename": "Bot"}})
		}
		var file any
		if content, ok := a.files[in.Variables.File]; ok {
			file = map[string]any{"mode": 0100644, "object": map[string]any{"__typename": "Blob", "oid": blobID(content), "text": content, "byteSize": len(content), "isBinary": false}}
		}
		repo := map[string]any{
			"visibility": "PRIVATE",
			"ref":        map[string]any{"target": map[string]any{"__typename": "Commit", "oid": a.head, "file": file}},
			"issue": map[string]any{"number": 77, "title": a.title, "body": a.body, "state": strings.ToUpper(a.state), "url": "https://example.invalid/attestation/77",
				"author": map[string]any{"login": "assay-desk-app", "__typename": "Bot", "databaseId": 300000001}, "lastEditedAt": nil,
				"comments":      map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}},
				"labels":        map[string]any{"nodes": labels, "pageInfo": map[string]any{"hasNextPage": false}},
				"timelineItems": map[string]any{"nodes": events, "pageInfo": map[string]any{"hasNextPage": false}}},
		}
		if a.snapshotChange != nil {
			a.snapshotChange(repo)
		}
		enc(map[string]any{"data": map[string]any{"repository": repo}})
		if a.afterSnapshot != nil {
			a.afterSnapshot()
		}
		return true
	}

	return false
}

func TestEvidenceSnapshotTwoRequests(t *testing.T) {
	lf := newLandingFixture(t)
	fragment := filepath.Join(t.TempDir(), "evidence.md")
	writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
	args := []string{"--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment}
	code, calls := lf.land(t, args...)
	if code != deskkit.ExitOK {
		t.Fatalf("landing failed: %s", lf.errBuf)
	}
	if len(calls) != 2 || calls[0].Operation != "snapshot" || calls[1].Operation != "commit" {
		t.Fatalf("want one batched read and one conditional commit, got %+v", calls)
	}
	assertOneMintedIdentity(t, calls)
	if !strings.Contains(lf.api.puts[0].Message, lf.receipt.EvidenceBinding()) {
		t.Fatal("commit lost attestation trailer")
	}
	// A fresh invocation reads once and detects that the evidence already landed.
	code, calls = lf.land(t, args...)
	if code != deskkit.ExitOK || len(calls) != 1 || len(lf.api.puts) != 1 {
		t.Fatalf("repeat must read once and write nothing: code=%d calls=%+v err=%s", code, calls, lf.errBuf)
	}
}

func TestEvidenceSnapshotConcurrentFileEditRefuses(t *testing.T) {
	lf := newLandingFixture(t)
	fragment := filepath.Join(t.TempDir(), "evidence.md")
	writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
	lf.api.afterSnapshot = func() {
		lf.api.files[fixtureBrief] += "\nconcurrent evidence\n"
	}
	code, calls := lf.land(t, "--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment)
	if code == 0 || len(lf.api.puts) != 0 || !strings.Contains(lf.api.files[fixtureBrief], "concurrent evidence") {
		t.Fatalf("stale snapshot overwrote concurrent content: code=%d writes=%v err=%s", code, lf.api.puts, lf.errBuf)
	}
	if len(calls) != 2 {
		t.Fatalf("conflict must not automatically retry: %+v", calls)
	}
}

func TestEvidenceSnapshotUnrelatedCommitDoesNotConflict(t *testing.T) {
	lf := newLandingFixture(t)
	fragment := filepath.Join(t.TempDir(), "evidence.md")
	writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
	lf.api.afterSnapshot = func() {
		lf.api.head = strings.Repeat("b", 40)
		lf.api.files["unrelated.txt"] = "concurrent unrelated change\n"
	}
	code, calls := lf.land(t, "--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment)
	if code != 0 || len(calls) != 2 || len(lf.api.puts) != 1 || lf.api.files["unrelated.txt"] != "concurrent unrelated change\n" {
		t.Fatalf("unrelated commit should remain compatible: code=%d calls=%+v err=%s", code, calls, lf.errBuf)
	}
}

func TestEvidenceSnapshotLostAcknowledgementDoesNotReplay(t *testing.T) {
	lf := newLandingFixture(t)
	fragment := filepath.Join(t.TempDir(), "evidence.md")
	writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
	lf.api.loseCommitAck = true
	args := []string{"--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment}
	code, calls := lf.land(t, args...)
	if code == 0 || len(lf.api.puts) != 1 || len(calls) != 2 {
		t.Fatalf("unknown effect must stop: code=%d calls=%+v writes=%v", code, calls, lf.api.puts)
	}
	lf.api.loseCommitAck = false
	code, calls = lf.land(t, args...)
	if code != 0 || len(lf.api.puts) != 1 || len(calls) != 1 {
		t.Fatalf("fresh read must reconcile landed write without replay: code=%d calls=%+v err=%s", code, calls, lf.errBuf)
	}
}

func TestEvidenceSnapshotRefusesIncompleteOrUntrustedInputs(t *testing.T) {
	for _, fault := range []string{"edited-attestation", "foreign-stamp", "missing-edit-history", "missing-comments", "incomplete-comments", "missing-label-history", "missing-file-lookup", "truncated-blob", "unknown-visibility", "unauthorized-public"} {
		t.Run(fault, func(t *testing.T) {
			lf := newLandingFixture(t)
			lf.api.snapshotChange = func(repo map[string]any) {
				issue := repo["issue"].(map[string]any)
				switch fault {
				case "edited-attestation":
					issue["lastEditedAt"] = "2026-10-06T01:00:00Z"
				case "foreign-stamp":
					for _, ev := range issue["timelineItems"].(map[string]any)["nodes"].([]any) {
						ev.(map[string]any)["actor"] = map[string]any{"login": "untrusted-actor", "__typename": "User"}
					}
				case "missing-edit-history":
					delete(issue, "lastEditedAt")
				case "missing-comments":
					delete(issue, "comments")
				case "incomplete-comments":
					issue["comments"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true}
				case "missing-label-history":
					delete(issue, "timelineItems")
				case "missing-file-lookup":
					delete(repo["ref"].(map[string]any)["target"].(map[string]any), "file")
				case "truncated-blob":
					repo["ref"].(map[string]any)["target"].(map[string]any)["file"].(map[string]any)["object"].(map[string]any)["text"] = "truncated"
				case "unknown-visibility":
					delete(repo, "visibility")
				case "unauthorized-public":
					repo["visibility"] = "PUBLIC"
				}
			}
			if fault == "unauthorized-public" {
				publicRepoGateFn = deskkit.PublicRepoGate
			}
			fragment := filepath.Join(t.TempDir(), "evidence.md")
			writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
			code, calls := lf.land(t, "--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment)
			if code == 0 || len(calls) != 1 || len(lf.api.puts) != 0 {
				t.Fatalf("unsafe snapshot accepted: fault=%s code=%d calls=%+v err=%s", fault, code, calls, lf.errBuf)
			}
		})
	}
}

func TestEvidenceSnapshotOverflowUsesCompleteReader(t *testing.T) {
	for _, connection := range []string{"labels", "timelineItems"} {
		t.Run(connection, func(t *testing.T) {
			lf := newLandingFixture(t)
			lf.api.snapshotChange = func(repo map[string]any) {
				repo["issue"].(map[string]any)[connection].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true}
			}
			fragment := filepath.Join(t.TempDir(), "evidence.md")
			writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
			code, calls := lf.land(t, "--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment)
			if code != 0 || len(calls) != 3 || calls[1].Method != "GET" || len(lf.api.puts) != 1 {
				t.Fatalf("complete-reader fallback failed: code=%d calls=%+v err=%s", code, calls, lf.errBuf)
			}
		})
	}
}

// Bypass the command's preflight checks. The checked publication forge must
// still reject unsafe content, shrinking writes and the wrong file revision.
func TestEvidenceSnapshotLowerWriteGuards(t *testing.T) {
	for _, fault := range []string{"shrink", "wrong-revision", "outbound-content"} {
		t.Run(fault, func(t *testing.T) {
			lf := newLandingFixture(t)
			before := len(lf.api.calls)
			f, repo, _, err := admitLanding(lf.home, fixtureRepo, "example-org", "tracker", fixtureBrief, "main", &auditCtx{})
			if err != nil {
				t.Fatal(err)
			}
			in := deskkit.WriteFileInput{File: fixtureBrief, Branch: "main", Content: []byte(fixturePlan + "\nmore evidence\n"), Message: "Evidence: fixture", AppendOnly: true}
			switch fault {
			case "shrink":
				in.Content = []byte("short\n")
			case "wrong-revision":
				in.ExpectedSHA = strings.Repeat("b", 40)
			case "outbound-content":
				in.Content = append(in.Content, []byte("person"+"@"+"corp-mail.dev\n")...)
			}
			if _, err := f.WriteFile(repo, in); err == nil {
				t.Fatalf("lower layer admitted %s", fault)
			}
			if len(lf.api.puts) != 0 || len(lf.api.calls)-before != 1 {
				t.Fatalf("guard made an outward write: %+v", lf.api.calls[before:])
			}
		})
	}
}

func TestEvidenceSnapshotCreateUsesBranchPrecondition(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "concurrent-create"}[concurrent], func(t *testing.T) {
			lf := newLandingFixture(t)
			const target = "docs/streams/verify-outcomes/x/01-example.json"
			before := len(lf.api.calls)
			if concurrent {
				lf.api.afterSnapshot = func() { lf.api.head = strings.Repeat("b", 40); lf.api.files[target] = "concurrent receipt\n" }
			}
			f, repo, _, err := admitLanding(lf.home, fixtureRepo, "example-org", "tracker", target, "main", &auditCtx{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.WriteFile(repo, deskkit.WriteFileInput{File: target, Branch: "main", Content: []byte("{}\n"), Message: "Evidence: fixture"})
			if concurrent {
				if err == nil || len(lf.api.puts) != 0 || lf.api.files[target] != "concurrent receipt\n" {
					t.Fatal("concurrent create was overwritten")
				}
			} else if err != nil || len(lf.api.puts) != 1 {
				t.Fatalf("create failed: %v", err)
			}
			if len(lf.api.calls)-before != 2 {
				t.Fatalf("create must use one read and one conditional mutation: %+v", lf.api.calls[before:])
			}
		})
	}
}

// Each destination fault keeps the published bytes otherwise valid, so only the
// named guard stands between it and a write, and the refusal must come from the
// snapshot reader rather than a later gate. A symlink's blob is its target path;
// a binary or size-mismatched blob is converted text; a same-length edit keeps
// every size check green and only the Git blob hash can tell.
func TestEvidenceSnapshotRefusesNonRegularOrUnboundDestination(t *testing.T) {
	const link = "../elsewhere/brief-01-example.md"
	tampered := strings.Replace(fixturePlan, "Pending.", "Pendinf.", 1)
	if len(tampered) != len(fixturePlan) || tampered == fixturePlan {
		t.Fatal("tampered fixture must keep the length and change the bytes")
	}
	for _, fault := range []string{"symlink", "symlink-plan-bytes", "executable", "binary", "byte-size", "same-length-tamper"} {
		t.Run(fault, func(t *testing.T) {
			lf := newLandingFixture(t)
			lf.api.snapshotChange = func(repo map[string]any) {
				f := repo["ref"].(map[string]any)["target"].(map[string]any)["file"].(map[string]any)
				b := f["object"].(map[string]any)
				switch fault {
				case "symlink":
					f["mode"] = 0120000
					b["text"], b["oid"], b["byteSize"] = link, blobID(link), len(link)
				case "symlink-plan-bytes":
					f["mode"] = 0120000
				case "executable":
					f["mode"] = 0100755
				case "binary":
					b["isBinary"] = true
				case "byte-size":
					b["byteSize"] = len(fixturePlan) + 1
				case "same-length-tamper":
					b["text"] = tampered
				}
			}
			fragment := filepath.Join(t.TempDir(), "evidence.md")
			writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
			code, calls := lf.land(t, "--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment)
			if code == 0 || len(calls) != 1 || len(lf.api.puts) != 0 || lf.api.files[fixtureBrief] != fixturePlan {
				t.Fatalf("destination fault %s reached the write: code=%d calls=%+v err=%s", fault, code, calls, lf.errBuf)
			}
			want := "destination is not a complete regular text blob"
			if fault == "same-length-tamper" {
				want = "destination bytes do not match the returned Git blob"
			}
			if !strings.Contains(lf.errBuf.String(), want) {
				t.Fatalf("fault %s refused by the wrong guard; want %q: %s", fault, want, lf.errBuf)
			}
		})
	}
}

// The snapshot's own completeness gate refuses unknown body-edit history and a
// comments page whose hasNextPage is null. Admission would refuse some of these
// later as well; the reason pins that the snapshot itself never hands an
// incomplete history to the trust reducer.
func TestEvidenceSnapshotRefusesIncompleteHistoryAtTheSnapshot(t *testing.T) {
	for _, fault := range []string{"body-history-unknown", "comments-page-info-null"} {
		t.Run(fault, func(t *testing.T) {
			lf := newLandingFixture(t)
			lf.api.snapshotChange = func(repo map[string]any) {
				issue := repo["issue"].(map[string]any)
				switch fault {
				case "body-history-unknown":
					delete(issue, "lastEditedAt")
				case "comments-page-info-null":
					issue["comments"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": nil}
				}
			}
			fragment := filepath.Join(t.TempDir(), "evidence.md")
			writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
			code, calls := lf.land(t, "--root", lf.home, "--brief-path", fixtureBrief, "--evidence-file", fragment)
			if code == 0 || len(calls) != 1 || len(lf.api.puts) != 0 {
				t.Fatalf("incomplete history accepted: fault=%s code=%d calls=%+v err=%s", fault, code, calls, lf.errBuf)
			}
			if want := "incomplete attestation issue, labels or history"; !strings.Contains(lf.errBuf.String(), want) {
				t.Fatalf("fault %s not refused by the snapshot completeness gate; want %q: %s", fault, want, lf.errBuf)
			}
		})
	}
}

// One publication forge writes at most once, in process. A second write on
// the same scoped forge, after a confirmed write or a lost acknowledgement,
// must refuse before any request leaves; the CLI stopping is not the guard.
func TestEvidenceSnapshotSecondInProcessWriteRefuses(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "after-write", true: "after-lost-ack"}[lost], func(t *testing.T) {
			lf := newLandingFixture(t)
			f, repo, _, err := admitLanding(lf.home, fixtureRepo, "example-org", "tracker", fixtureBrief, "main", &auditCtx{})
			if err != nil {
				t.Fatal(err)
			}
			in := deskkit.WriteFileInput{File: fixtureBrief, Branch: "main", Content: []byte(fixturePlan + "\nmore evidence\n"), Message: "Evidence: fixture", AppendOnly: true}
			lf.api.loseCommitAck = lost
			if _, err := f.WriteFile(repo, in); (err != nil) != lost || len(lf.api.puts) != 1 {
				t.Fatalf("first write: lost=%v err=%v puts=%d", lost, err, len(lf.api.puts))
			}
			lf.api.loseCommitAck = false
			before := len(lf.api.calls)
			in.Content = append(in.Content, []byte("second evidence\n")...)
			_, err = f.WriteFile(repo, in)
			if err == nil || !strings.Contains(err.Error(), "evidence publication already attempted") {
				t.Fatalf("second in-process write was not refused by the publication guard: %v", err)
			}
			for _, c := range lf.api.calls[before:] {
				if c.Method != http.MethodGet {
					t.Fatalf("second in-process write reached the forge: %+v", lf.api.calls[before:])
				}
			}
			if len(lf.api.puts) != 1 {
				t.Fatalf("second in-process write landed: %+v", lf.api.puts)
			}
		})
	}
}
