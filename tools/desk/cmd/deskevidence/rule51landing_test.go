package main

// rule51landing_test.go — the rule-51 lane end to end through this tool
// (#4823): a verified outcome record lands through `deskevidence
// --outcome-record` for a brief whose repo-wide lint row is red offline and
// carries a tool stamp of a clean --acceptance-forge run, with this tree's
// statusgen on PATH running the closure gate; an unstamped or forge-red row
// refuses with no write.

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rule51LandingResult renders the witness Result cell of a red offline lint
// with a rule-51 stamp, the digest built from the documented input
// (statusgen/rule51.go rule51Digest) so the test does not share the code it
// checks: rule51-v1, row, command, offline state, exit and output hash, tree,
// date, forge state, rc and output hash, pin, credential class and digest.
func rule51LandingResult(command, forgeState, forgeRC, tree string) string {
	const cred = "sha256:0f0f0f0f0f0f"
	fields := []string{"rule51-v1", "1", command, "fail", "1", "abcdef012345", tree, "2026-09-22",
		forgeState, forgeRC, "aaaaaaaaaaaa", "v1.0.99", "verifier-run-read", cred}
	sum := sha256.Sum256([]byte(strings.Join(fields, "\n")))
	return "fail exit=1 rule51=" + forgeState + " forge-rc=" + forgeRC + " forge-out=sha256:aaaaaaaaaaaa pin=v1.0.99" +
		" cred=verifier-run-read cred-digest=" + cred + " stamp=sha256:" + hex.EncodeToString(sum[:])
}

func TestRule51OutcomeRecordLands(t *testing.T) {
	landedStatusgenOnPath(t)
	const command = "statusgen --root . --lint"
	oldClosure, oldLanded, oldGuard := verifiedClosureCheckFn, landedClosureFn, outcomeGuardFn
	t.Cleanup(func() { verifiedClosureCheckFn, landedClosureFn, outcomeGuardFn = oldClosure, oldLanded, oldGuard })
	cases := []struct {
		name   string
		result func(tree string) string
		lands  bool
	}{
		{"stamped forge-clean", func(tree string) string { return rule51LandingResult(command, "pass", "0", tree) }, true},
		{"unstamped offline red", func(string) string { return "fail exit=1" }, false},
		{"stamped forge red", func(tree string) string { return rule51LandingResult(command, "fail", "1", tree) }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, errbuf := setupFake(t)
			for _, line := range strings.Split(fixtureRoster, "\n") {
				if key, value, ok := strings.Cut(line, "="); ok && strings.HasPrefix(key, "ASSAY_") {
					t.Setenv(key, value)
				}
			}
			root, source, head, files := landedFixtureRow(t, false, command, c.result)
			flGit(t, root, "update-ref", "refs/remotes/origin/main", head)
			f.commitSHAs = map[string]string{"main": head}
			for p, b := range files {
				f.setFile(p, b)
			}
			verifiedClosureCheckFn = verifiedClosureCheckAt
			landedClosureFn = landedClosureTree
			outcomeGuardFn = guardVerifiedOutcomes
			raw := "{\"ts\":\"2026-09-22T02:00:00Z\",\"brief\":\"vf/01\",\"outcome\":\"verified\",\"rows_passed\":1,\"rows_total\":1,\"sha\":\"" + source + "\"}\n"
			recordFile := writeRepoFile(t, "record.json", raw)
			f.writes = nil
			errbuf.Reset()
			code := run([]string{"example-org/tracker", "main", "--root", root, "--outcome-record", recordFile})
			if c.lands {
				if code != 0 || len(f.writes) != 1 || f.writes[0].ExpectedHead != head {
					t.Fatalf("want one pinned landing, got exit=%d writes=%+v: %s", code, f.writes, errbuf.String())
				}
				return
			}
			if code != deskkit.ExitRefused || len(f.writes) != 0 {
				t.Fatalf("want a refusal with no write, got exit=%d writes=%d: %s", code, len(f.writes), errbuf.String())
			}
		})
	}
}
