package deskkit

import (
	"strings"
	"testing"
)

const (
	exampleDigestA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	exampleDigestB = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
)

// regression: desk-containers/30
func TestParseDispatchInstructions(t *testing.T) {
	t.Run("two kits parse", func(t *testing.T) {
		v := "worker=/cells/example/rules/worker.md@sha256:" + exampleDigestA + ",review=/cells/example/rules/review.md@sha256:" + exampleDigestB
		got, err := ParseDispatchInstructions(v)
		if err != nil {
			t.Fatalf("a well-formed value was refused: %v", err)
		}
		want := []DispatchInstruction{
			{Kit: "worker", Path: "/cells/example/rules/worker.md", SHA256: exampleDigestA},
			{Kit: "review", Path: "/cells/example/rules/review.md", SHA256: exampleDigestB},
		}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("parsed %+v, want %+v", got, want)
		}
		if FormatDispatchInstructions(got) != v {
			t.Errorf("format does not round-trip: %q", FormatDispatchInstructions(got))
		}
	})
	t.Run("empty is no entries", func(t *testing.T) {
		if got, err := ParseDispatchInstructions(""); err != nil || got != nil {
			t.Fatalf("empty value: %+v, %v", got, err)
		}
	})
	for _, c := range []struct {
		name, value, want string
	}{
		// A comma splits the entry, so the second half is a malformed entry.
		{"path with comma", "worker=/cells/example/a,b.md@sha256:" + exampleDigestA, "carries no @sha256:"},
		// The value splits on the LAST @sha256:, leaving the separator inside the path; a
		// first-occurrence split would instead read a digest that is not 64 hex.
		{"path with @sha256:", "worker=/cells/example/x@sha256:y/rules.md@sha256:" + exampleDigestA, `contains "@sha256:"`},
		{"repeated kit", "worker=/cells/example/a.md@sha256:" + exampleDigestA + ",worker=/cells/example/b.md@sha256:" + exampleDigestB, "more than once"},
		{"unknown kit", "example-kit=/cells/example/a.md@sha256:" + exampleDigestA, "not one of"},
		{"relative path", "worker=rules/a.md@sha256:" + exampleDigestA, "not absolute"},
		{"upper-case digest", "worker=/cells/example/a.md@sha256:" + strings.ToUpper(exampleDigestA), "not 64 lowercase hex"},
		{"short digest", "worker=/cells/example/a.md@sha256:" + exampleDigestA[:63], "not 64 lowercase hex"},
		{"no digest", "worker=/cells/example/a.md", "carries no @sha256:"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseDispatchInstructions(c.value)
			if err == nil {
				t.Fatalf("%q parsed", c.value)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not say %q", err, c.want)
			}
		})
	}
	for _, p := range []string{"/cells/example/a,b.md", "/cells/example/x@sha256:y.md"} {
		if DispatchInstructionPathCarriable(p) == nil {
			t.Errorf("%q reported carriable", p)
		}
	}
}

// regression: desk-containers/30
func TestDispatchRecordInstructionsDigest(t *testing.T) {
	for _, c := range []struct {
		name string
		v    *string
	}{{"null", nil}, {"64 lowercase hex", strp(exampleDigestA)}} {
		t.Run("accepts "+c.name, func(t *testing.T) {
			r := validDispatched()
			r.DispatchInstructionsSHA256 = c.v
			if err := ValidateDispatchRecord(r); err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}
	for _, c := range []struct{ name, v string }{
		{"absolute path", "/cells/example/rules/worker.md"},
		{"path with digest", "/cells/example/rules/worker.md@sha256:" + exampleDigestA},
		{"upper-case hex", strings.ToUpper(exampleDigestA)},
		{"63 hex", exampleDigestA[:63]},
		{"65 hex", exampleDigestA + "0"},
		{"space", exampleDigestA[:32] + " " + exampleDigestA[33:]},
	} {
		t.Run("refuses "+c.name, func(t *testing.T) {
			r := validDispatched()
			r.DispatchInstructionsSHA256 = strp(c.v)
			if err := ValidateDispatchRecord(r); err == nil {
				t.Fatalf("%q accepted", c.v)
			}
		})
	}
	t.Run("refuses released line setting it", func(t *testing.T) {
		r := validReleased()
		r.DispatchInstructionsSHA256 = strp(exampleDigestA)
		if err := ValidateDispatchRecord(r); err == nil {
			t.Fatal("a released line with the digest was accepted")
		}
	})
}
