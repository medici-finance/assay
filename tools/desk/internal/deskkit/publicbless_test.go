package deskkit

// Tests for the standing per-repo bless (publicbless.go) and its consult point
// in PublicRepoGate (repovis.go).
//
// The property under test is asymmetric on purpose: a well-formed sentinel
// naming the exact repo — and ONLY that — skips the per-write +1; every
// anomaly (missing file, unreadable file, loose permissions, malformed lines,
// wildcards, a different repo) leaves the gate applying exactly as it does in
// repovis_test.go. A corrupt sentinel can under-bless, never over-bless.

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withBlessSentinel installs a sentinel file with the given body under a
// private XDG_CONFIG_HOME and returns its path. Scoped to the test via
// t.Setenv / t.TempDir.
func withBlessSentinel(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	adir := filepath.Join(dir, "assay")
	if err := os.MkdirAll(adir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(adir, PublicBlessSentinelName)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// withNoSentinel points XDG_CONFIG_HOME at an empty private dir, so no
// sentinel resolves regardless of the machine running the tests.
func withNoSentinel(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

// captureBlessNotice swaps the NOTICE writer for a buffer for the duration of
// the test and returns it.
func captureBlessNotice(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := publicBlessNoticeW
	publicBlessNoticeW = &buf
	t.Cleanup(func() { publicBlessNoticeW = prev })
	return &buf
}

// (a) A blessed public repo passes the gate with issueNumber=0 (the create
// path: no reactions surface exists yet) AND without any +1. The stub's
// reactions lookup is armed to FAIL, proving the gate never consults the
// reactions surface on a bless — if it did, the error would surface as exit 6.
func TestPublicBlessSkipsGateOnCreatePath(t *testing.T) {
	withBlessSentinel(t, "example-org/example-k8s\n")
	captureBlessNotice(t)
	f := &stubRepoInfoFetcher{
		visibility:   "public",
		reactionsErr: errors.New("reactions must not be consulted on a bless"),
	}
	if err := PublicRepoGate(f, "example-org", "example-k8s", 0); err != nil {
		t.Fatalf("blessed public repo with issueNumber=0 should pass, got %v", err)
	}
}

func TestPublicBlessSkipsPlusOneOnReviewPath(t *testing.T) {
	withBlessSentinel(t, "example-org/example-k8s\n")
	captureBlessNotice(t)
	f := &stubRepoInfoFetcher{
		visibility:   "public",
		reactionsErr: errors.New("reactions must not be consulted on a bless"),
	}
	if err := PublicRepoGate(f, "example-org", "example-k8s", 42); err != nil {
		t.Fatalf("blessed public repo with an issue number and no +1 should pass, got %v", err)
	}
}

// "internal" is gated like "public", so the bless covers it the same way.
func TestPublicBlessCoversInternalVisibility(t *testing.T) {
	withBlessSentinel(t, "example-org/example-k8s\n")
	captureBlessNotice(t)
	f := &stubRepoInfoFetcher{visibility: "internal"}
	if err := PublicRepoGate(f, "example-org", "example-k8s", 0); err != nil {
		t.Fatalf("blessed internal repo should pass, got %v", err)
	}
}

// The match is exact but case-insensitive and whitespace-trimmed, on both the
// sentinel line and the gate's arguments.
func TestPublicBlessNormalisesCaseAndWhitespace(t *testing.T) {
	withBlessSentinel(t, "# standing authorization\n\n  Example-Org/Example-K8s  \n")
	captureBlessNotice(t)
	f := &stubRepoInfoFetcher{visibility: "public"}
	if err := PublicRepoGate(f, "example-org", "example-k8s", 0); err != nil {
		t.Fatalf("case/whitespace-normalised bless line should match, got %v", err)
	}
}

// (b) A NON-blessed public repo behaves exactly as before, sentinel present or
// not: exit 6 with no issue number, exit 5 with an issue number and no +1.
func TestPublicNonBlessedStillGated(t *testing.T) {
	t.Run("no_sentinel_no_issue_number", func(t *testing.T) {
		withNoSentinel(t)
		buf := captureBlessNotice(t)
		f := &stubRepoInfoFetcher{visibility: "public"}
		err := PublicRepoGate(f, "example-org", "example-k8s", 0)
		if !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6), got %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("no bless — nothing may be announced, got %q", buf.String())
		}
	})

	t.Run("sentinel_names_a_different_repo", func(t *testing.T) {
		withBlessSentinel(t, "example-org/some-other-repo\n")
		buf := captureBlessNotice(t)
		f := &stubRepoInfoFetcher{visibility: "public"}
		err := PublicRepoGate(f, "example-org", "example-k8s", 0)
		if !IsUnverifiable(err) {
			t.Fatalf("a bless for another repo must not cover this one; expected Unverifiable, got %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("no bless — nothing may be announced, got %q", buf.String())
		}
	})

	t.Run("same_name_different_owner_not_blessed", func(t *testing.T) {
		withBlessSentinel(t, "someone-else/example-k8s\n")
		captureBlessNotice(t)
		f := &stubRepoInfoFetcher{visibility: "public"}
		if err := PublicRepoGate(f, "example-org", "example-k8s", 0); !IsUnverifiable(err) {
			t.Fatalf("the match is owner AND name; expected Unverifiable, got %v", err)
		}
	})

	t.Run("sentinel_present_no_plus_one_still_refused", func(t *testing.T) {
		withBlessSentinel(t, "example-org/some-other-repo\n")
		captureBlessNotice(t)
		f := &stubRepoInfoFetcher{visibility: "public", reactions: nil}
		err := PublicRepoGate(f, "example-org", "example-k8s", 42)
		if !IsRefused(err) {
			t.Fatalf("expected Refused (exit 5), got %v", err)
		}
	})

	t.Run("plus_one_path_still_works_alongside_sentinel", func(t *testing.T) {
		withBlessSentinel(t, "example-org/some-other-repo\n")
		captureBlessNotice(t)
		f := &stubRepoInfoFetcher{
			visibility: "public",
			reactions: []Reaction{
				{User: ReactionUser{Login: "ada", Type: "User", ID: fixtureBlessID}, Content: "+1"},
			},
		}
		if err := PublicRepoGate(f, "example-org", "example-k8s", 42); err != nil {
			t.Fatalf("the per-issue +1 path must be untouched for non-blessed repos, got %v", err)
		}
	})
}

// (c) Every sentinel anomaly answers "not blessed" — the gate falls through
// and fails/refuses exactly as with no sentinel at all.
func TestPublicBlessFailsClosed(t *testing.T) {
	gateStillClosed := func(t *testing.T) {
		t.Helper()
		buf := captureBlessNotice(t)
		f := &stubRepoInfoFetcher{visibility: "public"}
		err := PublicRepoGate(f, "example-org", "example-k8s", 0)
		if !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6) — the anomaly must read as NOT blessed, got %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("an anomalous sentinel must not be announced as a bless, got %q", buf.String())
		}
	}

	t.Run("empty_file_blesses_nothing", func(t *testing.T) {
		// Deliberate divergence from the writeguard sentinel, whose empty file
		// claims ANY checkout: an empty bless file is zero repos, not all.
		withBlessSentinel(t, "")
		gateStillClosed(t)
	})

	t.Run("comments_only_blesses_nothing", func(t *testing.T) {
		withBlessSentinel(t, "# nothing here\n\n# still nothing\n")
		gateStillClosed(t)
	})

	t.Run("malformed_lines_bless_nothing", func(t *testing.T) {
		for name, body := range map[string]string{
			"bare name, no owner": "example-k8s\n",
			"trailing slash":      "example-org/\n",
			"leading slash":       "/example-k8s\n",
			"too many parts":      "example-org/example-k8s/extra\n",
			"owner wildcard":      "*/example-k8s\n",
			"name wildcard":       "example-org/*\n",
			"question wildcard":   "example-org/example-k8?\n",
			"charclass wildcard":  "example-org/example-k8[s]\n",
			"embedded space":      "example-org/example k8s\n",
			"two repos on a line": "example-org/example-k8s example-org/tracker\n",
		} {
			t.Run(name, func(t *testing.T) {
				withBlessSentinel(t, body)
				gateStillClosed(t)
			})
		}
	})

	t.Run("sentinel_is_a_directory", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		if err := os.MkdirAll(filepath.Join(dir, "assay", PublicBlessSentinelName), 0o700); err != nil {
			t.Fatal(err)
		}
		gateStillClosed(t)
	})

	t.Run("unreadable_sentinel_not_blessed", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("running as root — permission bits are not enforced")
		}
		p := withBlessSentinel(t, "example-org/example-k8s\n")
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o600) })
		gateStillClosed(t)
	})

	t.Run("group_or_world_writable_sentinel_not_blessed", func(t *testing.T) {
		// Any local process could have appended to a loose-permission file, so
		// it is not a human's standing authorization. Same posture as the
		// roster loader.
		for _, mode := range []os.FileMode{0o622, 0o646, 0o666} {
			p := withBlessSentinel(t, "example-org/example-k8s\n")
			if err := os.Chmod(p, mode); err != nil {
				t.Fatal(err)
			}
			gateStillClosed(t)
		}
	})
}

// (d) Private repos never reach the bless: they pass the gate before it, with
// or without a sentinel, and nothing is announced.
func TestPrivateRepoUntouchedByBless(t *testing.T) {
	withBlessSentinel(t, "example-org/private-repo\n")
	buf := captureBlessNotice(t)
	f := &stubRepoInfoFetcher{visibility: "private"}
	if err := PublicRepoGate(f, "example-org", "private-repo", 42); err != nil {
		t.Fatalf("private repo must pass the gate as before, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("a private repo's pass is not a bless-skip and must not be announced, got %q", buf.String())
	}
}

// The bless is consulted AFTER the visibility read: a repo whose visibility
// cannot be determined, or is unrecognised, still fails closed even when the
// sentinel names it. The bless relaxes the +1, never the visibility read.
func TestBlessNeverOverridesVisibilityFailClosed(t *testing.T) {
	t.Run("visibility_error", func(t *testing.T) {
		withBlessSentinel(t, "example-org/example-k8s\n")
		captureBlessNotice(t)
		f := &stubRepoInfoFetcher{visibilityErr: errors.New("connection refused")}
		if err := PublicRepoGate(f, "example-org", "example-k8s", 42); !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6) despite the bless, got %v", err)
		}
	})
	t.Run("visibility_unrecognised", func(t *testing.T) {
		withBlessSentinel(t, "example-org/example-k8s\n")
		captureBlessNotice(t)
		f := &stubRepoInfoFetcher{visibility: "restricted"}
		if err := PublicRepoGate(f, "example-org", "example-k8s", 42); !IsUnverifiable(err) {
			t.Fatalf("expected Unverifiable (exit 6) despite the bless, got %v", err)
		}
	})
}

// (e) The skip is announced: the NOTICE names the repo and the sentinel, so a
// standing authorization is visible on the audit trail, never silent.
func TestBlessSkipEmitsNotice(t *testing.T) {
	sentinel := withBlessSentinel(t, "example-org/example-k8s\n")
	buf := captureBlessNotice(t)
	f := &stubRepoInfoFetcher{visibility: "public"}
	if err := PublicRepoGate(f, "example-org", "example-k8s", 0); err != nil {
		t.Fatalf("blessed repo should pass, got %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "NOTICE:") {
		t.Fatalf("the skip must be announced as a NOTICE, got %q", out)
	}
	if !strings.Contains(out, "example-org/example-k8s") {
		t.Fatalf("the NOTICE must name the repo, got %q", out)
	}
	if !strings.Contains(out, sentinel) {
		t.Fatalf("the NOTICE must name the sentinel file, got %q", out)
	}
}

// The production NOTICE writer is stderr — the audit-trail property depends on
// it, so pin the binding the same way gatewired_test.go pins the gate seam.
func TestBlessNoticeWriterIsStderrInProduction(t *testing.T) {
	if w, ok := publicBlessNoticeW.(*os.File); !ok || w != os.Stderr {
		var _ io.Writer = publicBlessNoticeW
		t.Fatalf("publicBlessNoticeW is not os.Stderr at init — the bless-skip would be invisible in production")
	}
}
