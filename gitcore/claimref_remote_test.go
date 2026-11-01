package gitcore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-git/go-git/v5/plumbing"
)

// claimref_remote_test.go — a server's refusal must reach the caller WITH the server's own words.
//
// A forge that refuses a ref update sends two things: a one-word report-status ("failure",
// "pre-receive hook declined", …) and, on the sideband, the message saying why. Until the
// sideband was negotiated the second half never left the server, so every refusal the claim
// transport reported was a bare word and the cause of an intermittent forge refusal could not be
// read from any log. These tests plant a refusal with a known message in a local git server (a
// pre-receive hook that prints it and exits non-zero) and assert the message comes back.

const plantedRefusal = "policy: dispatch refs are frozen for maintenance"

// installRefusingHook makes server refuse every ref update, printing plantedRefusal on stderr —
// which git-receive-pack relays to the client on the sideband progress channel.
func installRefusingHook(t *testing.T, server string) {
	t.Helper()
	hook := filepath.Join(server, "hooks", "pre-receive")
	body := "#!/bin/sh\necho '" + plantedRefusal + "' >&2\nexit 1\n"
	if err := os.WriteFile(hook, []byte(body), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
}

// seedRefusingServer returns a server holding refs/dispatch/<id> (when seed) whose every later
// ref update is refused by a hook printing plantedRefusal, plus a freshly minted claim tag.
func seedRefusingServer(t *testing.T, id string, seed bool) (server string, ref plumbing.ReferenceName, tagSHA plumbing.Hash, u RefUpdate) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not on PATH")
	}
	server = bareServer(t)
	ref = plumbing.ReferenceName("refs/dispatch/" + id)
	store, sha, err := MintClaimTag(id, "dispatch-claim "+id+" owner=a state=claimed branch=-", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	u = RefUpdate{URL: server, Ref: ref, Old: plumbing.ZeroHash, New: sha, Objects: store}
	if seed {
		if res, err := PushRefUpdate(context.Background(), u); err != nil || res != RefUpdateApplied {
			t.Fatalf("seed create: res=%v err=%v", res, err)
		}
	}
	installRefusingHook(t, server)
	return server, ref, sha, u
}

// The rendered refusal a create returns carries the server's own words, not only its status.
func TestPushRefUpdateRefusalCarriesServerMessage(t *testing.T) {
	_, _, _, u := seedRefusingServer(t, "at--stream--31", false)
	res, reason, err := PushRefUpdateDetail(context.Background(), u)
	if err != nil {
		t.Fatalf("a hook refusal is a server verdict, not a transport error: %v", err)
	}
	if res != RefUpdateRejected {
		t.Fatalf("result = %v, want Rejected", res)
	}
	if !strings.Contains(reason, plantedRefusal) {
		t.Fatalf("refusal reason = %q, want it to carry the server's own message %q", reason, plantedRefusal)
	}
}

// The verdict keeps the status word and the server's words apart, so a caller branches on the
// word without parsing prose.
func TestPushRefUpdateVerdictSplitsStatusAndRemote(t *testing.T) {
	_, _, _, u := seedRefusingServer(t, "at--stream--33", false)
	v, err := PushRefUpdateVerdict(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if v.Result != RefUpdateRejected || strings.Contains(v.Status, plantedRefusal) || !strings.Contains(v.Remote, plantedRefusal) {
		t.Fatalf("verdict = %+v, want Rejected with the status word in Status and %q in Remote", v, plantedRefusal)
	}
}

// A refused delete's error carries the server's own words.
func TestDeleteRefRefusalCarriesServerMessage(t *testing.T) {
	server, ref, _, _ := seedRefusingServer(t, "at--stream--32", true)
	_, err := DeleteRef(context.Background(), server, nil, ref)
	if err == nil {
		t.Fatal("DeleteRef succeeded against a server whose hook refuses every update")
	}
	if !strings.Contains(err.Error(), plantedRefusal) {
		t.Fatalf("delete refusal = %q, want it to carry the server's own message %q", err.Error(), plantedRefusal)
	}
}

// A refused delete is TYPED, and names the value it was compare-and-swapped against, so a caller
// that re-reads the ref can tell "it moved under me" from "the server refused the write".
func TestDeleteRefRefusalIsTypedWithOldValue(t *testing.T) {
	server, ref, tagSHA, _ := seedRefusingServer(t, "at--stream--34", true)
	_, err := DeleteRef(context.Background(), server, nil, ref)
	var rej *RefRejectedError
	if !errors.As(err, &rej) {
		t.Fatalf("DeleteRef error = %v (%T), want a *RefRejectedError", err, err)
	}
	if rej.Old != tagSHA {
		t.Fatalf("RefRejectedError.Old = %s, want the value the delete was CAS'd against (%s)", rej.Old, tagSHA)
	}
	if !strings.Contains(rej.Remote, plantedRefusal) || strings.Contains(rej.Status, plantedRefusal) {
		t.Fatalf("RefRejectedError = %+v, want the status word in Status and %q in Remote", rej, plantedRefusal)
	}
}

func TestRemoteTextIsOneBoundedPrintableLine(t *testing.T) {
	var b strings.Builder
	b.WriteString("line one\r\n\n\x1b[31mline two\x07\n")
	b.WriteString("c1:\u0085\u009b|")
	if b.Len()%2 == 0 {
		b.WriteString("|") // odd prefix, so the byte cap below lands INSIDE a two-byte rune
	}
	b.WriteString(strings.Repeat("\u00e9", maxRemoteMessageBytes))
	if utf8.ValidString(b.String()[:maxRemoteMessageBytes]) {
		t.Fatal("fixture broken: the byte cap does not split a rune, so this test would not exercise the repair")
	}
	buf := bytesBufferOf(b.String())
	got := remoteText(buf)
	if !utf8.ValidString(got) {
		t.Fatalf("remoteText returned invalid UTF-8 after the byte cap")
	}
	if strings.ContainsAny(got, "\r\n\x1b\x07\u0085\u009b") {
		t.Fatalf("remoteText kept a control character: %q", got)
	}
	if !strings.HasPrefix(got, "line one; [31mline two") {
		t.Fatalf("remoteText = %q, want lines joined with \"; \"", got[:40])
	}
	if len(got) > maxRemoteMessageBytes {
		t.Fatalf("remoteText length %d exceeds the %d-byte bound", len(got), maxRemoteMessageBytes)
	}
}

func bytesBufferOf(s string) *bytes.Buffer { return bytes.NewBufferString(s) }
