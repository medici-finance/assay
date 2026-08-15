package deskkit

import (
	"fmt"
	"io/fs"
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Fixtures for the structured-format tests, assembled from SPLIT fragments for the reason
// recorded at scanSecret40: deskpr secret-scans the branch diff before it pushes, so a
// contiguous credential or lockfile literal on an added line would refuse the very PR that
// adds the test. Runtime values are exact.
var (
	goSumDigest  = "ypeBEsobvcr6wjGzmiPcTa" + "eG7/gUfE5yuYB3ha/uSLs="
	sri512Digest = "pKvURIxJVi2CgRXROh/M6p" + "J/UrTVRZKX+LQ+QtqJI4vB" + "NibkPcs43bCCSIkn7JBPtC" + "BXRDmD6IWFF51QVRr+Yg=="
	sopsEnvData  = "Uk5wYldZa2hoYVZZY0hkc1" + "praFdibkJv"
	sopsEnvIV    = "YkdWaGNtNXBibWNnZEc4Z2" + "NtVmhaQT09"
	sopsEnvTag   = "ZEdocGN5QnBjeUJoYmlCbG" + "VHRnRjR3hs"
	pemBeginRSA  = "-----" + "BEGIN RSA PRIVATE KEY-----"
	armorBegin   = "-----" + "BEGIN AGE ENCRYPTED FILE-----"
	armorEnd     = "-----" + "END AGE ENCRYPTED FILE-----"
	// The complete-envelope prefix, split for the same reason: the PRE-ground-truth/06
	// scanner refuses any surface carrying a contiguous ENC[AES256…GCM marker, and it scans
	// the branch diff, so spelling it out here would refuse the PR that adds this test.
	ageRecipient  = "age1ql3z7hjy54pw3" + "hyww5ayyfg7zqgvc7w" + "3j2elw8zmrj2kg5sfn9aqmcac8p"
	armorBody     = "WVdkbExXVnVZM0o1" + "Y0hScGIyNHViM0pu" + "TDNZeENpMCtJRmd5"
	sopsEncPrefix = "ENC[AES256" + "_GCM,data:"
)

func sopsEnvelope() string {
	return sopsEncPrefix + sopsEnvData + ",iv:" + sopsEnvIV + ",tag:" + sopsEnvTag + ",type:str]"
}

// TestStructuredExemptionsAreAnchored is the guard-strength half of the structured-format
// pass. Every exemption in structured.go is anchored on a field marker AND length-exact,
// and this proves BOTH halves are load-bearing by removing one at a time.
//
// The rows matter because a "recognise the lockfile" fix has an obvious wrong spelling —
// exempt any 44-char base64 run — that clears the same false positives and opens a
// laundering route wide enough to drive a credential through. #781 warned against
// piecemeal patching; this is the test that stops the generalisation from being one.
func TestStructuredExemptionsAreAnchored(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantRefuse bool
	}{
		// go.sum: marker present, length exact.
		{"go.sum line", "github.com/google/go-cmp v0.6.0 h1:" + goSumDigest, false},
		{"go.mod suffix variant", "golang.org/x/mod v0.17.0/go.mod h1:" + goSumDigest, false},
		{"same digest with NO h1: marker", "checksum " + goSumDigest, true},
		{"h1: marker but no module/version in front", "h1:" + goSumDigest, true},
		{"h1: marker with a digest one character short",
			"github.com/x/y v1.0.0 h1:" + strings.TrimSuffix(goSumDigest, "s=") + "=", true},
		{"h1: marker followed by trailing content on the line",
			"github.com/x/y v1.0.0 h1:" + goSumDigest + " and then some", true},

		// SRI: marker present, length exact per algorithm.
		{"npm integrity field", `"integrity": "sha512-` + sri512Digest + `"`, false},
		{"same payload with no integrity key", "sha512-" + sri512Digest, true},
		{"integrity key with a payload two characters long for sha512",
			`"integrity": "sha512-AA` + sri512Digest + `"`, true},
		{"integrity key claiming sha256 over an sha512-length payload",
			`"integrity": "sha256-` + sri512Digest + `"`, true},

		// sops: complete envelope AND the document signature.
		{"sanctioned sops document", "value: " + sopsEnvelope() + "\nsops" + ":\n    mac: " +
			sopsEnvelope() + "\n    version: 3.8.1", false},
		{"complete envelope with no sops document", "value: " + sopsEnvelope(), true},
		{"sops document with no complete envelope", "sops" + ":\n    kms: []\n    version: 3.8.1", true},
		// A BARE FOOTER: the document signature and a complete envelope are both present,
		// but the only envelope is the `mac:` INSIDE the metadata block — the integrity tag
		// over the ciphertext, not ciphertext of anything the author wrote. It carries no
		// encrypted content, so the justification for admitting sops content does not apply
		// to it. This is the row that keeps the exemption on the artifact #778 could not
		// commit and off everything that merely looks like it.
		{"sops footer whose only envelope is the mac inside the metadata block",
			"sops" + ":\n    mac: " + sopsEnvelope() + "\n    version: 3.8.1", true},
		{"sops document whose envelope is missing its tag field",
			"sops" + ":\n    mac: " + sopsEncPrefix + sopsEnvData + ",iv:" + sopsEnvIV +
				",type:str]\n    version: 3.8.1", true},

		// Containment: the literal-pattern arms still scan a sanctioned document in full.
		{"github token inside a sanctioned sops document",
			"data:\n    t: " + scanGHToken + "\nsops" + ":\n    mac: " + sopsEnvelope() +
				"\n    version: 3.8.1", true},
		{"private key inside a sanctioned sops document",
			"data:\n    k: " + pemBeginRSA + "\nsops" + ":\n    mac: " + sopsEnvelope() +
				"\n    version: 3.8.1", true},
		{"opaque blob OUTSIDE the sops block of a sanctioned document",
			"data:\n    t: " + scanSecret40 + "\nsops" + ":\n    mac: " + sopsEnvelope() +
				"\n    version: 3.8.1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := BodyCheck([]byte(c.body))
			if c.wantRefuse && err == nil {
				t.Fatalf("BodyCheck(%q) = nil, want a refusal", c.body)
			}
			if !c.wantRefuse && err != nil {
				t.Fatalf("BodyCheck(%q) = %v, want nil", c.body, err)
			}
		})
	}
}

// TestArmorLabelPolicyFailsClosed pins the replacement of the blanket "any five-dash BEGIN
// refuses" rule with a CLOSED allowlist of ciphertext/public labels.
//
// The change buys #380 — the detector's own quoted marker is not a well-formed
// delimiter and never was — and #778's age/pgp recipient blocks. What it must
// not buy is a way past the arm by inventing a label, which is why an unknown label is
// treated exactly like a private key.
func TestArmorLabelPolicyFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		rest  string // everything after the BEGIN keyword on the line
		fires bool   // is this delimiter secret-bearing?
	}{
		{"RSA private key", " RSA PRIVATE KEY-----", true},
		{"OpenSSH private key", " OPENSSH PRIVATE KEY-----", true},
		{"encrypted private key", " ENCRYPTED PRIVATE KEY-----", true},
		{"bare private key", " PRIVATE KEY-----", true},
		{"private key with no closing dashes (a truncated paste)", " EC PRIVATE KEY", true},
		{"unknown label fails closed", " ACME VAULT UNSEAL SHARE-----", true},
		{"misspelled certificate label fails closed", " CERTIFICATTE-----", true},
		{"age ciphertext", " AGE ENCRYPTED FILE-----", false},
		{"pgp message", " PGP MESSAGE-----", false},
		{"certificate", " CERTIFICATE-----", false},
		{"public key", " PUBLIC KEY-----", false},
		{"the detector's own quoted marker", `"):`, false},
		{"the detector's own elided refusal message", " …)", false},
		{"prose mentioning the marker", " of a PEM block, roughly", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := armorBeginIsSecretBearing(c.rest); got != c.fires {
				t.Fatalf("armorBeginIsSecretBearing(%q) = %v, want %v", c.rest, got, c.fires)
			}
		})
	}
}

// TestArmorBodyStillScansOutsideSopsDocuments pins the containment on the armor change: a
// benign delimiter stops the PEM arm firing, but the BODY between the delimiters still
// reaches the high-entropy-run loop. So an armored blob pasted into a PR body is still
// refused — with an entropy message rather than a PEM one — unless a sanctioned sops
// document accounts for it.
func TestArmorBodyStillScansOutsideSopsDocuments(t *testing.T) {
	body := armorBegin + "\n" + armorBody + "\n" + armorEnd
	if err := BodyCheck([]byte(body)); err == nil {
		t.Fatalf("an armored ciphertext blob outside a sops document was ADMITTED — the "+
			"label allowlist stops the PEM arm, it does not exempt the payload: %q", body)
	}
	// The SAME blob inside a sanctioned sops document's recipient block is the artifact
	// #778 could not commit, and it must pass.
	inSops := "apiVersion: v1\nkind: Secret\nstringData:\n    k: " + sopsEnvelope() +
		"\nsops" + ":\n    age:\n        - recipient: " + ageRecipient + "\n" +
		"          enc: |\n            " + strings.ReplaceAll(body, "\n", "\n            ") +
		"\n    mac: " + sopsEnvelope() + "\n    version: 3.8.1"
	if err := BodyCheck([]byte(inSops)); err != nil {
		t.Fatalf("the same armored blob inside a sanctioned sops recipient block was "+
			"REFUSED — this is #778's artifact: %v", err)
	}
}

// TestPathRuleDifferential is the #410 measurement, in BOTH directions with
// its sample sizes reported, and it is the test that decides isPathLike's rule (4).
//
// #410's own words: "Any change here MUST be validated by a differential over a large
// corpus in BOTH directions (REFUSED→CLEAN and CLEAN→REFUSED) with the sample size
// reported — the #397 review caught a 0.43% false-negative class that a 15-sample spot
// check missed." So this test draws from a fixed seed, prints the counts, and asserts:
//
//   - FALSE-NEGATIVE direction: the admitted fraction of the 13/7/18 slash-layout
//     population — #410's measured 3.07% under the budget rule alone — collapses;
//   - FALSE-POSITIVE direction: the real-path population is UNCHANGED at 100% admitted.
//
// The second assertion is the one that chose rule (4)'s SEGMENT-COUNT spelling over the
// character-ratio spelling. Both close the #410 class; the ratio rule also newly refuses
// ordinary paths carrying one long opaque component (a content-hash build directory, a
// cache path), and #209/#1255 record what refusing real paths costs. The ratio's cost
// is measured here too, so the choice is a number rather than a preference.
func TestPathRuleDifferential(t *testing.T) {
	const n = 200000
	rng := rand.New(rand.NewSource(0x610))
	const alnum = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789"

	draw := func(k int) string {
		b := make([]byte, k)
		for i := range b {
			b[i] = alnum[rng.Intn(len(alnum))]
		}
		return string(b)
	}

	// (b) from #410: 40 characters forced into the published example key's 13/7/18 slash
	// layout, over [A-Za-z0-9] only. This is the population the 3.07% was measured on.
	admittedByBudgetOnly, admittedNow := 0, 0
	for i := 0; i < n; i++ {
		run := draw(13) + "/" + draw(7) + "/" + draw(18)
		if pathLikeBudgetOnly(run) {
			admittedByBudgetOnly++
		}
		if isPathLike(run) {
			admittedNow++
		}
	}
	before := 100 * float64(admittedByBudgetOnly) / float64(n)
	after := 100 * float64(admittedNow) / float64(n)
	t.Logf("FALSE-NEGATIVE direction, n=%d runs in the 13/7/18 AWS layout: "+
		"rule(3) alone admitted %d (%.5f%%); with rule(4) %d (%.5f%%)",
		n, admittedByBudgetOnly, before, admittedNow, after)
	if before < 1.0 {
		t.Fatalf("the control is not reproducing #410: rule(3) alone admitted only %.5f%% "+
			"of the 13/7/18 population, but #410 measured 3.07%% — the differential is "+
			"measuring the wrong thing, so its verdict on rule(4) means nothing", before)
	}
	if after >= before/10 {
		t.Errorf("rule (4) did not close the #410 class: %.5f%% still admitted, want under "+
			"a tenth of the %.5f%% baseline", after, before)
	}

	// The FALSE-POSITIVE direction, over runs harvested from THIS REPO'S OWN TREE plus the
	// shapes review bodies carry that the tree cannot supply (absolute operator paths,
	// URLs, Go module paths).
	//
	// Harvested rather than hand-written, because a hand-written path population is a
	// population of paths the author already believed would pass — the exact circularity
	// #410 caught in the comment it corrected. And harvested THROUGH reBase64ish, because
	// isPathLike never sees a path: it sees the base64ish RUN extracted from one, and `-`,
	// `.` and `_` are outside that charset. A population of literal paths measures a
	// function nothing calls.
	harvested := harvestRepoPathRuns(t)
	t.Logf("harvested %d distinct 32+ char base64ish runs from the repo tree", len(harvested))
	if len(harvested) < 100 {
		t.Fatalf("only %d runs harvested from the repo tree — too small a population to "+
			"measure the false-positive direction against; this is could-not-check, not "+
			"a pass", len(harvested))
	}
	falsePositives := 0
	for _, run := range harvested {
		if !isPathLike(run) {
			falsePositives++
			if falsePositives <= 10 {
				t.Errorf("FALSE POSITIVE: isPathLike(%q) = false — rule (4) refuses a run "+
					"extracted from a real path in this repo", run)
			}
		}
	}
	t.Logf("FALSE-POSITIVE direction, n=%d harvested repo-path runs: %d refused",
		len(harvested), falsePositives)

	// A hand-picked set for the shapes the tree cannot supply.
	realPaths := []string{
		"tools/desk/internal/deskkit/bodycheck",
		"tools/desk/cmd/deskpost/internal/bodycheck",
		"github.com/medici-finance/assay/tools/desk/internal/deskkit",
		"/Users/operator/work/example/slides",
		"/home/user/documents/notes/reference",
		"/opt/homebrew/opt/python311/bin/python3",
		"/tmp/build/go/pkg/mod/cache/download/sumdb",
		"frontend/src/components/dashboard/PositionSummaryPanel",
		"//localhost/api/v2/state/deployments",
		"services/app/internal/api/handlers/reviewwrite/",
		"docs/reports/2026/07/30/factory-floor-summary",
		"src/Example/Settlement",
		"com/example/desk/commit/5d529c27e3b1a04f9c2d8e7b6a1f0c3d4e5f6a7b",
		"docs/streams/groundtruth/brief06gategoldencorpus",
		"tools/desk/internal/IsOnByDefault/router",
		"docs/reports/2026/07/30/factoryfloorsummary",
		// A build/cache path carrying ONE long opaque component. This is the row that
		// separates rule (4) from the character-ratio spelling: the ratio refuses it.
		"dist/assets/a1b2c3d4e5f6a7b8c9d0/main",
		"var/cache/objects/9f86d081884c7d659a2feaa0c55ad0/blob",
	}
	ratioWouldRefuse := 0
	for _, p := range realPaths {
		if !isPathLike(p) {
			t.Errorf("FALSE POSITIVE: isPathLike(%q) = false — rule (4) refuses a real path", p)
		}
		if !pathLikeRatioRule(p) {
			ratioWouldRefuse++
		}
	}
	t.Logf("FALSE-POSITIVE direction, n=%d real path runs: rule(4) admits %d/%d; "+
		"the rejected character-ratio spelling would have refused %d of them",
		len(realPaths), len(realPaths), len(realPaths), ratioWouldRefuse)
	if ratioWouldRefuse == 0 {
		t.Errorf("the character-ratio spelling refused none of the real-path population — " +
			"the stated reason for preferring the segment-count rule no longer holds, so " +
			"either the population or the comment on isPathLike needs revisiting")
	}
}

// harvestRepoPathRuns walks the repository tree from tools/desk and returns every distinct
// 32+ character base64ish RUN that the repo's own file paths produce — the population
// isPathLike is actually asked about in the field, drawn from artifacts nobody wrote for
// this test. Directories git ignores are skipped; unreadable subtrees are skipped rather
// than failing the walk, and the caller asserts a floor on the population size so a walk
// that silently found nothing cannot report green.
func harvestRepoPathRuns(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	var out []string
	// tools/desk/internal/deskkit -> tools/desk: this module's OWN root. Deliberately not
	// the repository root: a walk that leaves the module would make this test a
	// cross-module reader whose CI trigger cannot cover what it reads (#199), and the desk
	// module alone supplies a population well over the floor asserted by the caller.
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable subtree is skipped, not fatal
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		// Prefixed with the module's own repo-relative root, because that is the form the
		// scan actually meets: a review body cites `tools/desk/internal/deskkit/…` and a
		// diff header carries `a/tools/desk/…`. Harvesting the module-relative spelling
		// would measure a shorter string than anything BodyCheck is ever handed.
		cited := "tools/desk/" + filepath.ToSlash(rel)
		for _, run := range reBase64ish.FindAllString(cited, -1) {
			if !seen[run] {
				seen[run] = true
				out = append(out, run)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk the repo tree to harvest a path population: %v", err)
	}
	sort.Strings(out)
	return out
}

// pathLikeBudgetOnly is isPathLike WITHOUT rule (4) — the pre-#410 rule, kept here as the
// differential's control. A differential with no control cannot tell "the fix worked" from
// "the corpus was never reproducing the bug", and #410's headline number exists precisely
// because a fixture-only lock could not see the population.
func pathLikeBudgetOnly(run string) bool {
	if !strings.Contains(run, "/") || strings.ContainsAny(run, "+=") {
		return false
	}
	opaque := 0
	for _, seg := range strings.Split(run, "/") {
		if seg == "" || isGitSHA(seg) || looksLikeWords(seg) {
			continue
		}
		opaque += len(seg)
		if opaque >= maxOpaqueInPath {
			return false
		}
	}
	return true
}

// pathLikeRatioRule is the SPELLING OF RULE (4) THAT WAS REJECTED: cap the opaque material
// as a fraction of the run's length rather than counting segments. It is kept so the
// comparison in TestPathRuleDifferential is executable rather than asserted — the same
// discipline #410 applied to the comment it corrected.
func pathLikeRatioRule(run string) bool {
	if !strings.Contains(run, "/") || strings.ContainsAny(run, "+=") {
		return false
	}
	opaque := 0
	for _, seg := range strings.Split(run, "/") {
		if seg == "" || isGitSHA(seg) || looksLikeWords(seg) {
			continue
		}
		opaque += len(seg)
		if opaque >= maxOpaqueInPath {
			return false
		}
	}
	return opaque*2 < len(run)
}

// TestScanOverrideIsLoggedOrRefused pins the #585 contract: exit 5 without the
// flag remains a stop AND advertises the flag; with the flag the write proceeds and an
// audit row lands; and nothing here can take an UNLOGGED bypass.
func TestScanOverrideIsLoggedOrRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	body := []byte("token " + scanGHToken)
	scan := ScanSurface("PR body", body)
	if scan == nil {
		t.Fatal("fixture no longer refuses; the override contract cannot be exercised")
	}

	// No override: still refused, still exit 5, and the message now says a way through
	// exists — #585's worker left the sanctioned transport because the refusal said the
	// opposite ("no override flag by design").
	err := HandleScanRefusal(ScanOverride{Tool: "deskpr", Verb: "create", Surface: "PR body", Content: body}, scan)
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("unoverridden refusal = %v (exit %d), want exit %d", err, ExitCodeOf(err), ExitRefused)
	}
	if !strings.Contains(err.Error(), ScanOverrideFlag) {
		t.Errorf("refusal does not advertise --%s: %v", ScanOverrideFlag, err)
	}
	if !strings.Contains(err.Error(), "remains a STOP") {
		t.Errorf("refusal does not restate that exit 5 is a stop: %v", err)
	}

	// A reason too short to be a reason is itself refused: the audit row has to say WHY.
	if e := HandleScanRefusal(ScanOverride{Tool: "deskpr", Verb: "create", Reason: "fp",
		Surface: "PR body", Content: body}, scan); ExitCodeOf(e) != ExitRefused {
		t.Errorf("a two-character override reason was accepted: %v", e)
	}

	// A real override: proceeds, and writes exactly one scan_override audit row carrying
	// the surface digest, the reason and an identity.
	const reason = "go.sum digest block, corpus-verified false positive"
	if e := HandleScanRefusal(ScanOverride{Tool: "deskpr", Verb: "create", Repo: "example-org/tracker",
		Reason: reason, Surface: "PR body", Content: body}, scan); e != nil {
		t.Fatalf("a well-formed override was refused: %v", e)
	}
	entries, lerr := LoadEntries()
	if lerr != nil {
		t.Fatalf("cannot read the audit log back: %v", lerr)
	}
	rows := 0
	for _, e := range entries {
		if e.Verb != ScanOverrideVerb {
			continue
		}
		rows++
		if e.BodyDigest != Sha256Hex(body) {
			t.Errorf("override row digest = %q, want the scanned surface's digest", e.BodyDigest)
		}
		if !strings.Contains(e.Detail, reason) {
			t.Errorf("override row does not carry the stated reason: %q", e.Detail)
		}
		if !strings.Contains(e.Detail, OverrideIdentity()) {
			t.Errorf("override row does not carry an identity: %q", e.Detail)
		}
		if !strings.Contains(e.Detail, "GitHub token prefix") {
			t.Errorf("override row does not record WHAT was waved through: %q", e.Detail)
		}
	}
	if rows != 1 {
		t.Errorf("scan_override audit rows = %d, want exactly 1", rows)
	}

	// The impersonation guard is not a secret shape and is not overridable.
	t.Setenv("ASSAY_HUMAN_LOGIN_MAP", "alex=alexlogin")
	imp := ScanSurface("PR body", []byte("Decision (Alex, 2026-08-13): ship it"))
	if imp == nil {
		t.Skip("impersonation guard not configured in this environment")
	}
	e := HandleScanRefusal(ScanOverride{Tool: "deskpr", Verb: "create", Reason: reason,
		Surface: "PR body", Content: body}, imp)
	if ExitCodeOf(e) != ExitRefused || !strings.Contains(e.Error(), "does not apply to the") {
		t.Errorf("the impersonation guard accepted a scan override: %v", e)
	}
}

// TestOverrideHintNamesTheAuditedFields is a documentation lock: the hint has to tell a
// reader what the override COSTS them, or it reads as a free pass rather than an audited
// one. #585's fix direction was "a flag + logged justification", not "a flag".
func TestOverrideHintNamesTheAuditedFields(t *testing.T) {
	h := OverrideHint()
	for _, want := range []string{ScanOverrideFlag, "audit row", "digest", "identity", "STOP"} {
		if !strings.Contains(h, want) {
			t.Errorf("OverrideHint does not mention %q: %s", want, h)
		}
	}
	if err := ValidateScanOverride(""); ExitCodeOf(err) != ExitRefused {
		t.Errorf("an empty reason was accepted: %v", err)
	}
	if err := ValidateScanOverride("a reason long enough to be one"); err != nil {
		t.Errorf("a well-formed reason was refused: %v", err)
	}
	if err := ValidateScanOverride(fmt.Sprintf("bad\x07reason with a bell in it")); ExitCodeOf(err) != ExitRefused {
		t.Errorf("a control-character reason was accepted: %v", err)
	}
}
