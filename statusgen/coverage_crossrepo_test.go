package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// regression: #5501
func TestCoverageCrossSource(t *testing.T) {
	for _, mode := range []string{"current", "tracker changed", "source changed", "missing checkout", "wrong checkout", "unmerged source", "changed expectation", "failed witness", "malformed", "dirty witness", "host tracker changed"} {
		t.Run(mode, func(t *testing.T) {
			fixtureMode := "current"
			if mode == "malformed" {
				fixtureMode = "malformed"
			}
			d, sourceSHA, _, source := foldTrackingSource(t, fixtureMode)
			addCrossCoverageSpan(t, d.path, sourceSHA)
			root := strings.TrimSuffix(d.path, filepath.FromSlash(trackingFixtureRel))
			foldGit(t, source, foldMailWorker, "remote", "add", "origin", "https://github.com/example-org/agents.git")
			foldGit(t, source, foldMailWorker, "update-ref", "refs/remotes/origin/main", sourceSHA)
			t.Setenv("DESK_ROOTS", "example-org/agents="+source)
			switch mode {
			case "tracker changed", "host tracker changed":
				if err := os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("unrelated tracker change"), 0600); err != nil {
					t.Fatal(err)
				}
				foldGit(t, root, foldMailWorker, "add", "unrelated.txt")
				foldGit(t, root, foldMailWorker, "commit", "-qm", "unrelated tracker change")
				if mode == "host tracker changed" {
					raw, err := os.ReadFile(d.path)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(d.path, []byte(strings.Replace(string(raw), "sandbox=container-netns", "sandbox=host", 1)), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "source changed", "unmerged source":
				if err := os.WriteFile(filepath.Join(source, "implementation.txt"), []byte("changed source"), 0600); err != nil {
					t.Fatal(err)
				}
				foldGit(t, source, foldMailWorker, "add", "implementation.txt")
				foldGit(t, source, foldMailWorker, "commit", "-qm", "source change")
				if mode == "source changed" {
					foldGit(t, source, foldMailWorker, "update-ref", "refs/remotes/origin/main", "HEAD")
				}
			case "missing checkout":
				t.Setenv("DESK_ROOTS", "")
			case "wrong checkout":
				foldGit(t, source, foldMailWorker, "remote", "set-url", "origin", "https://github.com/example-org/wrong.git")
			case "changed expectation", "failed witness", "dirty witness":
				body, err := os.ReadFile(d.path)
				if err != nil {
					t.Fatal(err)
				}
				text := string(body)
				switch mode {
				case "changed expectation":
					text = strings.Replace(text, "| 1 | `true` | exit 0 |", "| 1 | `true` | exit 0 and additional assertion |", 1)
				case "failed witness":
					text = strings.Replace(text, "pass exit=0", "fail exit=1", 1)
				case "dirty witness":
					text = strings.Replace(text, "@ "+sourceSHA[:12], "@ "+sourceSHA[:12]+"+dirty", 1)
				}
				if err := os.WriteFile(d.path, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			got := evaluateCoverage(root, streams, coverageOptions{})["fx/01"]
			want := mode == "current" || mode == "tracker changed" || mode == "unmerged source"
			if got.Released != want {
				t.Fatalf("released=%v want=%v claims=%+v", got.Released, want, got.Claims)
			}
		})
	}
}

// regression: #5501 — configured source history must not turn malformed or
// ambiguous metadata into an applicability grant.
func TestCoverageCrossRefusals(t *testing.T) {
	for _, mode := range []string{"mixed", "inline prefix", "missing origin", "short run", "wrong repo", "wrong path", "wrong source", "short tracking", "malformed", "conflicting", "fenced", "prior run only", "foreign homed source missing"} {
		t.Run(mode, func(t *testing.T) {
			d, sourceSHA, _, source := foldTrackingSource(t, mode)
			addCrossCoverageSpan(t, d.path, sourceSHA)
			root := strings.TrimSuffix(d.path, filepath.FromSlash(trackingFixtureRel))
			foldGit(t, source, foldMailWorker, "remote", "add", "origin", "https://github.com/example-org/agents.git")
			foldGit(t, source, foldMailWorker, "update-ref", "refs/remotes/origin/main", "HEAD")
			t.Setenv("DESK_ROOTS", "example-org/agents="+source)
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			got := evaluateCoverage(root, streams, coverageOptions{})["fx/01"]
			if got.Released {
				t.Fatalf("invalid binding released coverage: %+v", got)
			}
		})
	}
}

// addCrossCoverageSpan makes the source-only execution profile explicit in a
// synthetic fixture. It creates no real admission or execution credit.
func addCrossCoverageSpan(t *testing.T, path, source string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(body), "pass exit=0", "pass exit=0 sandbox=container-netns", 1)
	marker := "Verification-Attestation:"
	index := strings.Index(text, marker)
	if index < 0 {
		return
	}
	tree := "sha256:" + strings.Repeat("b", 64)
	text = text[:index] + "Tree-Digest: " + tree + "\nContainer-Image: sha256:" + strings.Repeat("c", 64) + "\n" + text[index:]
	verdict := strings.Index(text, "**VERIFY: PASS**")
	if verdict < 0 {
		return
	}
	lines := strings.Split(text[:verdict], "\n")
	digest, header := witnessSpanDigest(lines, len(lines))
	if header < 0 {
		t.Fatal("fixture has no witness header")
	}
	bracket := fmt.Sprintf("Admission-Bracket: run=%s source=%s tree=%s witness=%s opened=2026-10-01T01:00:00Z closed=2026-10-01T01:01:00Z before=receipt after=check-verifier\n", strings.Repeat("a", 64), source, tree, digest)
	text = text[:verdict] + bracket + "\n" + text[verdict:]
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

// regression: #5501 — one closed source bracket cannot grant source-only
// scope to a host row, a row carried from another run, or a row outside it.
func TestCrossCoverageScope(t *testing.T) {
	d, sourceSHA, _, _ := foldTrackingSource(t, "current")
	addCrossCoverageSpan(t, d.path, sourceSHA)
	bf, ok, err := parseBriefFile(d.path)
	if err != nil || !ok {
		t.Fatal(err)
	}
	pass, err := flipLatestPass(bf.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	host := strings.Replace(foldRow, "{SHA}", sourceSHA[:12], 1)
	for _, mode := range []string{"source", "host policy row", "bound host", "host ci row", "carried row", "outside bracket"} {
		t.Run(mode, func(t *testing.T) {
			evidence, run := bf.Evidence, pass.Run
			rows := []evaluation.VerifyRow{{ID: "1"}}
			switch mode {
			case "host policy row", "bound host", "host ci row":
				host = strings.Replace(host, "| 1 |", "| 2 |", 1)
				host = strings.Replace(host, "`true`", "`cat ../tracker/policy.txt`", 1)
				run += "\n" + witnessHeader + "\n" + host
				evidence += "\n" + witnessHeader + "\n" + host
				class := classCheck
				if mode == "host ci row" {
					class = classCheckCI + " +flow"
				}
				rows = append(rows, evaluation.VerifyRow{ID: "2", ClassCell: class})
			case "carried row":
				rows = append(rows, evaluation.VerifyRow{ID: "2"})
				evidence = strings.Replace(host, "| 1 |", "| 2 |", 1) + "\n" + evidence
			case "outside bracket":
				newer := strings.Replace(strings.Replace(foldRow, "{SHA}", sourceSHA[:12], 1), "pass exit=0", "pass exit=0 sandbox=container-netns", 1)
				run += "\n" + newer
				evidence += "\n" + newer
			}
			got := crossCoverageSpan(evidence, run, rows, mode == "bound host" || mode == "host ci row")
			if (got == nil) != (mode == "source" || mode == "bound host") {
				t.Fatalf("scope=%v", got)
			}
		})
	}
}

// A host witness must bind both repositories. Its adjacent container bracket
// never licenses a changed tracking policy or a different source revision.
func TestCoverageCrossSourceMixedWitnesses(t *testing.T) {
	for _, mode := range []string{"current", "status only", "tracking policy", "tracking stream policy", "source changed", "missing tracking", "wrong source", "dirty host", "carried host", "malformed summary"} {
		t.Run(mode, func(t *testing.T) {
			d, sourceSHA, trackingSHA, source := foldTrackingSource(t, "mixed witnesses")
			root := strings.TrimSuffix(d.path, filepath.FromSlash(trackingFixtureRel))
			addCrossCoverageSpan(t, d.path, sourceSHA)
			raw, err := os.ReadFile(d.path)
			if err != nil {
				t.Fatal(err)
			}
			host := strings.ReplaceAll(foldRow, "{SHA}", sourceSHA[:12])
			host = strings.Replace(host, "| 1 | `true`", "| 2 | `cat ../tracker/policy.txt`", 1)
			text := strings.Replace(string(raw), "**VERIFY: PASS**", witnessHeader+"\n"+host+"\n**VERIFY: PASS**", 1)
			switch mode {
			case "status only", "tracking policy", "tracking stream policy":
				file := "policy.txt"
				if mode == "status only" {
					file = "STATUS.md"
				}
				if mode == "tracking stream policy" {
					file = "docs/streams/fx/policy.md"
				}
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(file)), []byte("changed input\n"), 0600); err != nil {
					t.Fatal(err)
				}
				foldGit(t, root, foldMailWorker, "add", file)
				foldGit(t, root, foldMailWorker, "commit", "-qm", "tracking change")
			case "source changed":
				if err := os.WriteFile(filepath.Join(source, "implementation.txt"), []byte("changed source\n"), 0600); err != nil {
					t.Fatal(err)
				}
				foldGit(t, source, foldMailWorker, "add", "implementation.txt")
				foldGit(t, source, foldMailWorker, "commit", "-qm", "source change")
			case "missing tracking":
				text = strings.ReplaceAll(text, "tracking-source="+trackingSHA, "tracking-source="+strings.Repeat("f", 40))
			case "wrong source":
				text = strings.ReplaceAll(text, "source="+sourceSHA, "source="+strings.Repeat("d", 40))
			case "dirty host":
				text = strings.Replace(text, host, strings.Replace(host, "@ "+sourceSHA[:12], "@ "+sourceSHA[:12]+"+dirty", 1), 1)
			case "carried host":
				text = strings.Replace(text, witnessHeader+"\n"+host+"\n", "", 1)
			case "malformed summary":
				text = strings.Replace(text, "**VERIFY: PASS**", "| Note | Date | Runner |\n|---|---|---|\n| summary | 2026-10-01 | fixture-model |\n**VERIFY: PASS**", 1)
			}
			if err := os.WriteFile(d.path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			foldGit(t, root, foldMailVerifier, "add", trackingFixtureRel)
			foldGit(t, root, foldMailVerifier, "commit", "-qm", "mixed evidence")
			foldGit(t, source, foldMailWorker, "remote", "add", "origin", "https://github.com/example-org/agents.git")
			foldGit(t, source, foldMailWorker, "update-ref", "refs/remotes/origin/main", "HEAD")
			t.Setenv("DESK_ROOTS", "example-org/agents="+source)
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			got := evaluateCoverage(root, streams, coverageOptions{})["fx/01"]
			want := mode == "current" || mode == "status only"
			if got.Released != want {
				t.Fatalf("released=%v want=%v claims=%+v", got.Released, want, got.Claims)
			}
		})
	}
}
