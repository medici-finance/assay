package main

import (
	"os"
	"strings"
	"testing"
)

// regression: #4563. These are source-fixture checks, not
// independent verification of desk-tools/38 or a substitute human ruling.
func TestTransitionDeliveredPlaceholder(t *testing.T) {
	const placeholder = "<!-- Independent verifier appends one row per Verify command after implementation. -->"
	cases := []string{"quoted-placeholder", "nested-placeholder", "unchanged-result", "changed-quotation", "missing-placeholder", "placeholder-plus-result", "arbitrary-comment", "lookalike", "quoted-result", "changed-amendment-history", "lost-current-history", "offline"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			delivered := placeholder
			switch name {
			case "placeholder-plus-result":
				delivered += "\nHistorical execution: FAIL exit=1\n"
			case "arbitrary-comment":
				delivered = "<!-- Historical execution: FAIL exit=1 -->"
			case "lookalike":
				delivered = "<!-- Independent verifier appends one row per Verify command after implementation. Changed. -->"
			case "quoted-result", "unchanged-result":
				delivered = "Historical execution: FAIL exit=1"
			}
			f := contextHistoryFixture(t, nil, func(stage, body string) string {
				if stage == "original" {
					body = strings.Replace(body, "## Evidence\n", "## Evidence\n"+delivered+"\n", 1)
				}
				if stage == "amendment" && name == "changed-amendment-history" {
					body = strings.Replace(body, "## Evidence\n", "## Evidence\nAmendment altered history.\n", 1)
				}
				return body
			}, func(body string) string {
				quote := "> "
				if name == "nested-placeholder" {
					quote = "> > > "
				}
				replacement := quote + strings.ReplaceAll(delivered, "\n", "\n"+quote)
				if name == "changed-quotation" {
					replacement = strings.Replace(replacement, "after implementation.", "after editing.", 1)
				}
				if name == "unchanged-result" {
					replacement = delivered
				}
				if name == "missing-placeholder" {
					replacement = ""
				}
				body = strings.Replace(body, delivered, replacement, 1)
				// Keep the fixture's append point after the full preserved record,
				// as the real verifier does; do not splice fresh rows into history.
				const history = "Historical result: row 2 failed; retained for diagnosis.\n"
				body = strings.Replace(body, history, "", 1)
				return strings.Replace(body, "## Review\n", history+"\n## Review\n", 1)
			})
			f.evidence(t, nil)
			if name == "lost-current-history" {
				raw, err := os.ReadFile(f.path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.path, []byte(strings.Replace(string(raw), f.history, "", 1)), 0644); err != nil {
					t.Fatal(err)
				}
				vfCommit(t, f.root, vfMailVerifier)
			}
			if name == "offline" {
				transitionForge = false
			}
			verify, evidence, err := briefSections(f.path)
			if err != nil {
				t.Fatal(err)
			}
			_, why := protectedAcceptanceFor(verify, evidence, f.descriptor, f.path)
			want := name == "quoted-placeholder" || name == "nested-placeholder" || name == "unchanged-result"
			if (why == "") != want {
				t.Fatalf("plan accepted=%v want=%v: %s", why == "", want, why)
			}
			if !want {
				reason := "pre-amendment history lost delivered Evidence"
				switch name {
				case "changed-amendment-history":
					reason = "amendment changed historical Evidence"
				case "lost-current-history":
					reason = "historical Evidence was not preserved"
				case "offline":
					reason = "--acceptance-forge is required"
				}
				if !strings.Contains(why, reason) {
					t.Fatalf("wrong refusal: %s, want %s", why, reason)
				}
			}
			if want {
				findings, why := closureWitnesses(verify, evidence, f.path)
				if why != "" || len(findings) != 3 {
					t.Fatalf("fresh closure: %s %+v", why, findings)
				}
				for _, finding := range findings {
					if !closureSatisfied(finding) {
						t.Fatalf("unsatisfied: %+v", finding)
					}
				}
			}
		})
	}
}
