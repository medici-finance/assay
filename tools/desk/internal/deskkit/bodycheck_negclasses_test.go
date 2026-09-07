package deskkit

import (
	"errors"
	"strings"
	"testing"
)

// The three measured false-positive classes and their NEGATIVE controls. Each Rule clears
// a class WITHOUT widening what the scan admits, so every test here is a boundary case that
// must STILL refuse — the paired positive corpus fixtures are the other half of the same
// bound (TestBodycheckPositives).

// hexRun returns a lowercase-hex run of length n (n <= 64), realistically interleaved so
// no letter run reaches word length — a real MD5/hash never decomposes into words, which is
// exactly why the scan cannot tell it from a token by shape and the length gate has to.
func hexRun(n int) string {
	base := strings.Repeat("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6", 2) // 64 hex chars
	return base[:n]
}

// TestHexPathSegmentBoundaries pins Rule 1 (doc-path 32-hex segment). The exemption is
// length-EXACT (only 32), requires a doc extension or a slash-word neighbour, and requires
// the surrounding token to be a real path — so a 31/33/48-hex segment, a bare 32-hex run,
// a 32-hex ".md" in prose with no '/', and a slash-joined hex pair with no word neighbour
// all STILL refuse. One clean control proves the exemption actually fires.
func TestHexPathSegmentBoundaries(t *testing.T) {
	// The length-exact boundary lives in the DASH/DOT filename context, where reBase64ish
	// reports the hex STEM as a bare run and the doc-extension arm (isDocPathHexSegment) is
	// what decides it — so a 33/48/63-hex stem, or a 32-hex stem before a NON-doc extension,
	// stays refused even though its surrounding path is otherwise identical to the clean one.
	stem := func(hex, ext string) string {
		return "recorded at docs/streams/example/2026-08-30-" + hex + ext + " today"
	}
	refused := []struct {
		name string
		body string
	}{
		{"33-hex filename stem, doc extension", stem(hexRun(33), ".md")},
		{"48-hex filename stem, doc extension", stem(hexRun(48), ".md")},
		{"63-hex filename stem, doc extension", stem(hexRun(63), ".md")},
		{"32-hex filename stem, NON-doc extension", stem(hexRun(32), ".bin")},
		{"bare 32-hex run in prose", "the digest is " + hexRun(32) + " today"},
		{"32-hex .md in prose with no slash", "open the file " + hexRun(32) + ".md now"},
		{"slash-joined 32-hex pair, no word neighbour", "blob at " + hexRun(32) + "/" + hexRun(32) + ".md"},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			if err := BodyCheck([]byte(c.body)); err == nil {
				t.Fatalf("Rule 1 boundary ADMITTED %q — the exemption widened past its statement", c.body)
			} else if !IsRefused(err) {
				t.Fatalf("%q refused with %v, want exit-5 refusal", c.body, err)
			}
		})
	}
	// The exemption DOES fire on a real doc path — a 32-hex filename stem with a doc
	// extension inside a path that carries a '/'. Without this the boundary suite could pass
	// on a rule that simply refuses everything.
	clean := "recorded at docs/streams/example/2026-08-30-" + hexRun(32) + ".md for context"
	if err := BodyCheck([]byte(clean)); err != nil {
		t.Fatalf("Rule 1 clean control refused %q: %v — the exemption is not firing", clean, err)
	}
}

// TestIssueNumberSlashListBoundaries pins Rule 2 (issue-number slash-list). The exemption
// admits ONLY short (1-6-digit) all-numeric groups joined by '/', so an 8-digit group, a
// bare long numeric run, and a group carrying a non-digit all STILL refuse.
func TestIssueNumberSlashListBoundaries(t *testing.T) {
	refused := []struct {
		name string
		body string
	}{
		{"8-digit groups wearing slashes", "batch 12345678/90123456/78901234/56789012 landed"},
		{"bare long numeric run", "account 12345678901234567890123456789012 today"},
		{"a group carrying a non-digit", "batch 1017/1024/103a/1042/1055/1063/1078/1082/1096 here"},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			if err := BodyCheck([]byte(c.body)); err == nil {
				t.Fatalf("Rule 2 boundary ADMITTED %q — the exemption widened past its statement", c.body)
			} else if !IsRefused(err) {
				t.Fatalf("%q refused with %v, want exit-5 refusal", c.body, err)
			}
		})
	}
	clean := "superseded by #1017/1024/1031/1042/1055/1063/1078/1082/1096/1103 in one wave"
	if err := BodyCheck([]byte(clean)); err != nil {
		t.Fatalf("Rule 2 clean control refused %q: %v — the exemption is not firing", clean, err)
	}
}

// TestK8sSecretTemplateBoundaries pins Rule 3 (k8s Secret template). The exemption is
// PER-VALUE, never a "skip fences" rule, so ONE literal value among placeholders STILL
// refuses — inside a fence exactly as outside it.
func TestK8sSecretTemplateBoundaries(t *testing.T) {
	fenced := func(body string) string { return "```yaml\n" + body + "\n```\n" }
	manifest := func(pw string) string {
		return "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app\ntype: Opaque\nstringData:\n" +
			"  username: <REDACTED>\n  password: " + pw + "\n  api-token: {{ vault.token }}\n"
	}
	// A literal base64 value among placeholders (synthetic — base64 of "supersecretpassword").
	literal := "c3VwZXJzZWNyZXRwYXNzd29yZA=="
	for _, c := range []struct {
		name string
		body string
	}{
		{"one literal among placeholders (bare)", manifest(literal)},
		{"one literal among placeholders (fenced)", fenced(manifest(literal))},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := BodyCheck([]byte(c.body)); err == nil {
				t.Fatalf("Rule 3 boundary ADMITTED a literal value among placeholders — a decrypted Secret")
			} else if !IsRefused(err) {
				t.Fatalf("refused with %v, want exit-5 refusal", err)
			}
		})
	}
	// All-placeholder templates DO pass — bare and fenced alike.
	for _, c := range []struct {
		name string
		body string
	}{
		{"all placeholders (bare)", manifest("${DB_PASSWORD}")},
		{"all placeholders (fenced)", fenced(manifest("PLACEHOLDER"))},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := BodyCheck([]byte(c.body)); err != nil {
				t.Fatalf("Rule 3 clean control refused an all-placeholder template: %v", err)
			}
		})
	}
}

// TestExplainNamesRuleAndLineNeverSpan proves the --explain payload names the rule and the
// correct 1-based line on a multi-line body and NEVER carries the offending span. This is
// the property that keeps the refusal from itself becoming the leak.
func TestExplainNamesRuleAndLineNeverSpan(t *testing.T) {
	// A 40-char base64 secret (synthetic, mixed-case, no slash) on line 3.
	secret := "Qx7pLk2wZt9mNc4bYf6RhVs8" + "Ju3XoAeG5idWn1Dz"
	body := "line one is ordinary prose\n" +
		"line two is also fine\n" +
		"here is the value: " + secret + "\n"

	err := BodyCheck([]byte(body))
	if err == nil {
		t.Fatal("expected a refusal on the embedded high-entropy run")
	}
	var f *ScanFinding
	if !errors.As(err, &f) {
		t.Fatalf("refusal carried no ScanFinding; err=%v", err)
	}
	if f.Rule != "high-entropy-run" {
		t.Fatalf("finding rule = %q, want high-entropy-run", f.Rule)
	}
	if f.Line != 3 {
		t.Fatalf("finding line = %d, want 3 (the line the run sits on)", f.Line)
	}
	if f.Length != len(secret) {
		t.Fatalf("finding length = %d, want %d", f.Length, len(secret))
	}
	// The span must be absent from the explain text, both whole and as its interior — only
	// the two ends and the class survive redaction.
	out := f.Explain()
	if strings.Contains(out, secret) {
		t.Fatalf("explain text LEAKS the full span:\n%s", out)
	}
	if mid := secret[2 : len(secret)-2]; strings.Contains(out, mid) {
		t.Fatalf("explain text LEAKS the span interior:\n%s", out)
	}
	if !strings.Contains(out, "rule=high-entropy-run") || !strings.Contains(out, "line=3") {
		t.Fatalf("explain text does not name the rule and line: %q", out)
	}
}
