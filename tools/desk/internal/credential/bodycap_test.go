package credential

// bodycap_test.go — the credential-path forge reads are capped: an over-cap body is refused
// with no bytes handed back, never read as a truncated answer.

import (
	"strings"
	"testing"
)

// regression: #6730
func TestReadCappedResponseOverCap(t *testing.T) {
	raw, err := readCappedN(strings.NewReader(`{"token":"x"}`+strings.Repeat(" ", 100)), 64)
	if err == nil {
		t.Fatalf("an over-cap body read as success (%d bytes)", len(raw))
	}
	if raw != nil {
		t.Errorf("an over-cap read handed back %d bytes; want none", len(raw))
	}
	raw, err = readCappedN(strings.NewReader(strings.Repeat(" ", 64)), 64)
	if err != nil || len(raw) != 64 {
		t.Errorf("a body at the cap: %d bytes, %v; want 64, nil", len(raw), err)
	}
}

// regression: #6730
func TestMaxResponseBytesIsWired(t *testing.T) {
	if MaxResponseBytes != 32<<20 {
		t.Errorf("MaxResponseBytes = %d, want 32 MiB", MaxResponseBytes)
	}
	// readCappedResponse applies MaxResponseBytes: a body one byte over it is refused.
	big := strings.NewReader(strings.Repeat(" ", int(MaxResponseBytes)+1))
	if raw, err := readCappedResponse(big); err == nil {
		t.Errorf("a body one byte over MaxResponseBytes read as success (%d bytes)", len(raw))
	}
}
