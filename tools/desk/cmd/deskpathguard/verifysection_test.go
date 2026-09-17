package main

import "testing"

const verifyTableEditDiff = `diff --git a/docs/streams/verify-integrity/brief-01-protected-verifier-paths.md b/docs/streams/verify-integrity/brief-01-protected-verifier-paths.md
index 1111111..2222222 100644
--- a/docs/streams/verify-integrity/brief-01-protected-verifier-paths.md
+++ b/docs/streams/verify-integrity/brief-01-protected-verifier-paths.md
@@ -40,7 +40,7 @@ ## Verify
 | # | Class | Command | Expect |
 |---|-------|---------|--------|
-| 1 | check | go test ./... | pass |
+| 1 | check | true | pass |
`

const evidenceEditDiff = `diff --git a/docs/streams/verify-integrity/brief-01-protected-verifier-paths.md b/docs/streams/verify-integrity/brief-01-protected-verifier-paths.md
index 1111111..2222222 100644
--- a/docs/streams/verify-integrity/brief-01-protected-verifier-paths.md
+++ b/docs/streams/verify-integrity/brief-01-protected-verifier-paths.md
@@ -60,3 +60,4 @@ ## Evidence
 | 1 | 0 | ok | 2026-09-17 | worker |
+| 2 | 0 | ok | 2026-09-17 | worker |
`

func TestVerifySectionTouchedFromHunkTrailer(t *testing.T) {
	path := "docs/streams/verify-integrity/brief-01-protected-verifier-paths.md"
	if !verifySectionTouched(verifyTableEditDiff, path) {
		t.Fatalf("a hunk whose @@ trailer names ## Verify must be reported touched")
	}
}

func TestVerifySectionNotTouchedOutsideVerify(t *testing.T) {
	path := "docs/streams/verify-integrity/brief-01-protected-verifier-paths.md"
	if verifySectionTouched(evidenceEditDiff, path) {
		t.Fatalf("a hunk whose @@ trailer names ## Evidence must NOT be reported as touching ## Verify")
	}
}

func TestVerifySectionTouchedFromInlineHeading(t *testing.T) {
	// No hunk-trailer heading this time; the heading line itself is IN the hunk body as
	// context, which the walk must still pick up.
	diff := `diff --git a/docs/streams/x/brief-02-y.md b/docs/streams/x/brief-02-y.md
index 1111111..2222222 100644
--- a/docs/streams/x/brief-02-y.md
+++ b/docs/streams/x/brief-02-y.md
@@ -10,4 +10,4 @@
 ## Verify
 | # | Class | Command | Expect |
 |---|-------|---------|--------|
-| 1 | check | old | pass |
+| 1 | check | new | pass |
`
	if !verifySectionTouched(diff, "docs/streams/x/brief-02-y.md") {
		t.Fatalf("an inline ## Verify heading in the hunk body must be tracked")
	}
}

func TestVerifySectionIgnoresOtherFiles(t *testing.T) {
	// The diff touches ## Verify, but for a DIFFERENT file — asking about a file not in the
	// diff at all must report false, not accidentally match the other file's hunk.
	if verifySectionTouched(verifyTableEditDiff, "docs/streams/other/brief-09-z.md") {
		t.Fatalf("a diff for a different file must not be reported touched")
	}
}
