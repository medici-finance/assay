package deskkit

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// #2060 — cite, never quote, sops material.
//
// A reviewer's verdict quoted sops-shaped text from the diff it was reviewing and the
// sops arm refused it, with no sanctioned way to state the finding. The fix is a remedy in
// the refusal, not a narrower rule: the arm still refuses every quotation (the corpus
// fixtures pos-review-quotes-sops-footer and pos-review-quotes-sops-envelope pin that),
// and the refusal now names the way through — cite the material by path:line and describe
// it. These tests pin that the remedy rides on EVERY sops-block refusal, whichever surface
// or outbound kind trips it, so no write path can hand a writer the bare refusal again.
//
// The markers are split for the reason recorded in this package's other fixtures: the
// scanner in force while this change is open reads the branch diff.

// sopsCiteWant is the remedy as the test states it, NOT read back from the code under
// test, so a mutation that empties or rewords the remedy reddens here.
const sopsCiteWant = "cite sops material by path:line"

// sopsCiteNever is the remedy's prohibition half, pinned beside its opening words so a
// mutation that keeps "cite" but drops "never quote" also reddens. The markers are split
// for the same branch-diff reason as the fixtures below.
const sopsCiteNever = "never quote its " + "sops" + ": footer or an " + "ENC" + "[…] envelope"

// sopsQuotedShapes are the two quotation shapes a review body trips the arm with: a bare
// footer (the reSopsKey + reSopsField half) and a short envelope (the reSopsEncVal half;
// its payloads are under the high-entropy threshold, so only the sops arm sees it).
func sopsQuotedShapes() map[string]string {
	return map[string]string{
		"footer": "the footer reads:\n\n```yaml\n" + "sops" + ":\n    kms: []\n" +
			"    lastmodified: \"2026-09-30T10:00:00Z\"\n    version: 3.8.1\n```\n",
		"envelope": "the value is `" + "ENC" + "[AES256_GCM,data:c2VjcmV0,iv:aXZpdg==," +
			"tag:dGFnZw==,type:str]`\n",
	}
}

// TestBodyCheckSopsCiteRemedy: every surface name a caller scans under gets the remedy.
func TestBodyCheckSopsCiteRemedy(t *testing.T) {
	surfaces := []string{SurfaceBody, "PR body", "PR title", "review body", "comment body",
		"issue body", "branch diff vs origin/main"}
	for shape, text := range sopsQuotedShapes() {
		for _, surface := range surfaces {
			t.Run(shape+"/"+surface, func(t *testing.T) {
				err := ScanSurface(surface, []byte("Verdict: request-changes\n\n"+text))
				if err == nil || !IsRefused(err) {
					t.Fatalf("a quoted sops %s on %q: want a refusal (exit 5), got %v", shape, surface, err)
				}
				var f *ScanFinding
				if !errors.As(err, &f) || f == nil || f.Rule != "sops-block" {
					t.Fatalf("a quoted sops %s on %q: want rule sops-block, got finding %+v (%v)",
						shape, surface, f, err)
				}
				for _, want := range []string{sopsCiteWant, sopsCiteNever} {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("sops-block refusal on %q lost its remedy (want %q): %v",
							surface, want, err)
					}
				}
			})
		}
	}
}

// TestBodyCheckSopsCiteOutbound: every outbound kind carries the remedy through the
// outward-write check, which is the path `deskpost review` takes.
func TestBodyCheckSopsCiteOutbound(t *testing.T) {
	obRoster(t)
	kinds := []string{OutboundKindIssue, OutboundKindChange, OutboundKindComment,
		OutboundKindReview, OutboundKindLabel, OutboundKindFile, OutboundKindCommit, OutboundKindRef}
	for shape, text := range sopsQuotedShapes() {
		for _, kind := range kinds {
			t.Run(shape+"/"+kind, func(t *testing.T) {
				setup(t)
				defer SetOutboundNoticeWriter(&bytes.Buffer{})()
				SetOutboundContext(OutboundContext{Tool: "deskpost", Verb: kind})
				err := OutboundCheck(OutboundWrite{Role: "reviewer", Repo: obPrivate, Kind: kind,
					Fields: []OutboundField{{Name: "body", Text: "Verdict: request-changes\n\n" + text}}})
				if err == nil || !IsRefused(err) {
					t.Fatalf("%s write quoting a sops %s: want a refusal, got %v", kind, shape, err)
				}
				if !strings.Contains(err.Error(), "refused: "+RuleSecretPrefix+"sops-block") {
					t.Fatalf("%s write quoting a sops %s: refusal does not name secret.sops-block: %v",
						kind, shape, err)
				}
				for _, want := range []string{sopsCiteWant, sopsCiteNever} {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("%s write: sops-block refusal lost its remedy (want %q): %v",
							kind, want, err)
					}
				}
			})
		}
	}
}

// TestBodyCheckSopsCitedProse: the form the remedy asks for passes a review write — the
// marker and key NAMES inline, every finding anchored on a path:line, no payload.
func TestBodyCheckSopsCitedProse(t *testing.T) {
	obRoster(t)
	setup(t)
	defer SetOutboundNoticeWriter(&bytes.Buffer{})()
	SetOutboundContext(OutboundContext{Tool: "deskpost", Verb: OutboundKindReview})
	body := "Verdict: request-changes\n\n" +
		"1. `deploy/apps/billing/secrets.enc.yaml:41` — the `" + "sops" + ":` footer has a `mac:` " +
		"and no encrypted value outside it.\n" +
		"2. `deploy/apps/billing/secrets.enc.yaml:12` — the value is an " + "ENC" +
		"[AES256_GCM,…] envelope with no `iv:` field.\n"
	if err := OutboundCheck(OutboundWrite{Role: "reviewer", Repo: obPrivate, Kind: OutboundKindReview,
		Fields: []OutboundField{{Name: "body", Text: body}}}); err != nil {
		t.Fatalf("a review citing sops material by path:line was refused: %v", err)
	}
}
