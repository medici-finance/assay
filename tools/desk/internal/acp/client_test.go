package acp

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// spawnFake starts the current test binary as a fake ACP agent (see
// fakeagent_test.go / TestMain) in the given mode.
func spawnFake(t *testing.T, mode string) *Client {
	t.Helper()
	cl, err := Spawn([]string{os.Args[0]}, Opts{
		Env: []string{"ACP_TEST_FAKE_AGENT=" + mode},
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

func ctxT(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestHappyPath drives initialize -> session/new -> session/prompt against
// the fake agent end to end, and checks at least one SessionUpdate arrives
// on the update channel while the turn is in flight.
func TestHappyPath(t *testing.T) {
	cl := spawnFake(t, "happy")
	ctx := ctxT(t)

	initRes, err := cl.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if initRes.ProtocolVersion != 1 {
		t.Fatalf("ProtocolVersion = %d, want 1", initRes.ProtocolVersion)
	}
	if initRes.AgentName != "fake-acp-agent" {
		t.Fatalf("AgentName = %q, want fake-acp-agent", initRes.AgentName)
	}

	sid, err := cl.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if sid == "" {
		t.Fatalf("NewSession returned empty SessionID")
	}

	res, err := cl.Prompt(ctx, sid, "hello")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if res.StopReason != "end_turn" {
		t.Fatalf("StopReason = %q, want end_turn", res.StopReason)
	}

	select {
	case u := <-cl.Updates():
		if u.Kind != "agent_message_chunk" {
			t.Fatalf("update Kind = %q, want agent_message_chunk", u.Kind)
		}
	default:
		t.Fatalf("expected at least one SessionUpdate to be buffered by the time Prompt returned")
	}
}

// TestPermissionRefusal is the permission-refusal round trip Task 4
// requires: the fake agent issues a session/request_permission callback
// mid-turn; this package's default PermissionPolicy (DefaultRefusePermission)
// must select the "reject" option, and the fake agent (which only reports
// stopReason "refusal" when it saw exactly that) confirms the refusal made
// it back over the wire.
func TestPermissionRefusal(t *testing.T) {
	cl := spawnFake(t, "permission-refuse")
	ctx := ctxT(t)

	if _, err := cl.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	sid, err := cl.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	// Opts.PermissionPolicy was left nil on this Client -> DefaultRefusePermission.
	res, err := cl.Prompt(ctx, sid, "write a file")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if res.StopReason != "refusal" {
		t.Fatalf("StopReason = %q, want refusal (the fake agent only reports this when the client's session/request_permission reply selected the reject option)", res.StopReason)
	}
	t.Logf("permission request refused end-to-end: default PermissionPolicy selected the reject option, fake agent observed it and reported stopReason=refusal")
}

// TestUnexpectedProtocolVersionFailsClosed is Task 3/4's version-refusal
// case: the fake agent negotiates protocolVersion 999, outside this
// client's default SupportedProtocolVersions ([]int{1}). Initialize must
// return *ErrUnsupportedProtocolVersion, and the client must refuse to
// proceed to session/new afterward (ErrNotInitialized) rather than attempt
// it anyway.
func TestUnexpectedProtocolVersionFailsClosed(t *testing.T) {
	cl := spawnFake(t, "bad-version")
	ctx := ctxT(t)

	_, err := cl.Initialize(ctx)
	if err == nil {
		t.Fatalf("Initialize: expected error for unsupported protocol version, got nil")
	}
	var verErr *ErrUnsupportedProtocolVersion
	if !errors.As(err, &verErr) {
		t.Fatalf("Initialize error = %v (%T), want *ErrUnsupportedProtocolVersion", err, err)
	}
	if verErr.Negotiated != 999 {
		t.Fatalf("Negotiated = %d, want 999", verErr.Negotiated)
	}

	if _, err := cl.NewSession(ctx, t.TempDir()); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("NewSession after failed Initialize = %v, want ErrNotInitialized (client must fail closed, not proceed on an unsupported version)", err)
	}
}

// TestFileAccessRefusesOutsideRoot exercises Task 2's fs default policy
// directly (no agent round trip needed): with FSRoot set, a path outside it
// is refused; a path inside it is allowed.
func TestFileAccessRefusesOutsideRoot(t *testing.T) {
	root := t.TempDir()
	policy := rootScopedFileAccess(root)

	if policy(context.Background(), FileAccessRequest{Path: "/etc/passwd", Write: false}) {
		t.Fatalf("default FileAccessPolicy allowed a path outside FSRoot")
	}
	if !policy(context.Background(), FileAccessRequest{Path: root + "/ok.txt", Write: true}) {
		t.Fatalf("default FileAccessPolicy refused a path inside FSRoot")
	}
	// Sibling directory sharing a prefix ("<root>-evil") must not be treated as contained.
	if policy(context.Background(), FileAccessRequest{Path: root + "-evil/ok.txt", Write: false}) {
		t.Fatalf("default FileAccessPolicy allowed a sibling path sharing root's string prefix")
	}
}

// TestFileAccessRefusesEmptyRoot covers the empty-FSRoot case: refuse
// everything, per doc.go's fail-closed defaults.
func TestFileAccessRefusesEmptyRoot(t *testing.T) {
	policy := rootScopedFileAccess("")
	if policy(context.Background(), FileAccessRequest{Path: "/anything", Write: false}) {
		t.Fatalf("default FileAccessPolicy with empty FSRoot allowed a path")
	}
}
