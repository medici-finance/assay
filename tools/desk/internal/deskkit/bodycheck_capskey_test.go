package deskkit

import (
	"math/rand"
	"strings"
	"testing"
)

// Fixtures for the ALL-CAPS KEY=<value> cases: a `make … TEST=<GoTestName>` selector in a
// brief's Verify row (#1642), an env-var data directory (#5825) and a recorded input SHA
// (#5974). Split across concatenation for the reason recorded at scanSecret40.
var (
	capsSel51   = "TEST=" + "TestPublicationAdmission" + "WithUpperLayerBypassed" // 51, field refusal 1
	capsSel39   = "TEST=" + "TestCatalogObject" + "RevisionRoundTrip"             // 39, field refusal 2
	capsIdent   = "TestCatalogObject" + "RevisionRoundTrip"                       // the value alone
	capsCamelTl = "TestCatalogObject" + "Qx7pLk2wZt9mNc4bYf6R"                    // CamelCase head, entropy tail
	capsPath    = "/var/lib/" + "postgresql/18/docker"                            // #5825's value
	// Entropy made to look like a path: a word-shaped path whose last segment is a
	// CamelCase head on an entropy tail, and base62 split by a slash into segments that
	// outnumber the words. (isPathLike already admits a lone short opaque segment among
	// more words, bare, on main; that value stays exactly as exempt behind a key.)
	capsTailPath = "/var/lib/postgresql/" + capsCamelTl
	capsLowPath  = "/var/" + "Xq7pLk2wZt9mNc4b/" + "Yf6RqZ3HvW8tKp2L"
)

// capsKeyAdmitLines are whole lines a Verify row, a Makefile or a shell command carries.
// Each value is an exempt identifier standing alone; only the key used to sink it.
var capsKeyAdmitLines = []string{
	"| 1 | `make -C tools/desk test " + capsSel51 + "` | exits 0 |",
	"| 2 | `make -C tools/desk test " + capsSel39 + "` | exits 0 |",
	"make test RUN=" + capsIdent,
	"GO_TEST_RUN=" + capsIdent + " make test",
	"export FILTER=" + identK8s,
	"PATTERN=" + identPRs,
}

// capsKeyPathSHALines carry a path (#5825) or a git SHA (#5974) behind an ALL-CAPS key:
// an Evidence note, a CI env line, a recorded input pair.
var capsKeyPathSHALines = []string{
	"RISK-VALUE: `PGDATA=" + capsPath + "` is the data directory.",
	"GOMODCACHE=/opt/toolcache/" + "gocache/pkg/mod go build ./...",
	"export GOPATH=/srv/ci/work/" + "toolchain/gopath",
	"row 5 inputs: `DELIVERY_HEAD=" + fixHex40 + "` and `DELIVERY_BASE=" + fixHex40 + "`.",
	"RELEASE_TREE=" + fixHex64,
}

// TestAssignmentCapsKeyPathSHA pins the #5825 / #5974 extension: a value that is a git SHA
// or a word-shaped path on its own is admitted behind the same ALL-CAPS key the identifier
// rule accepts.
// regression: #5825 #5974
func TestAssignmentCapsKeyPathSHA(t *testing.T) {
	for _, line := range capsKeyPathSHALines {
		if err := BodyCheck([]byte(line + "\n")); err != nil {
			t.Errorf("BodyCheck rejected %q: %v", line, err)
		}
		if err := ScanSurfaceSecrets("branch diff vs origin/main", []byte(line+"\n")); err != nil {
			t.Errorf("ScanSurfaceSecrets rejected %q: %v", line, err)
		}
	}
}

// TestAssignmentCapsKeyIdent pins the admission: an ALL-CAPS key in front of a value that
// is an exempt identifier on its own is the "word-shaped key" the refusal text names, so
// the key no longer turns that value into a refusal.
// regression: #1642
func TestAssignmentCapsKeyIdent(t *testing.T) {
	for _, line := range capsKeyAdmitLines {
		if err := BodyCheck([]byte(line + "\n")); err != nil {
			t.Errorf("BodyCheck rejected %q: %v", line, err)
		}
		// The diff surface: the field refusals were unchanged context lines, which reach
		// the secret arms with their diff marker already stripped.
		if err := ScanSurfaceSecrets("branch diff vs origin/main", []byte(line+"\n")); err != nil {
			t.Errorf("ScanSurfaceSecrets rejected %q: %v", line, err)
		}
	}
}

// capsRandomKeys are the three key forms the admission covers (a bare caps key, a
// snake_case caps name, a caps key in front of a path), each written in front of every
// random value below.
var capsRandomKeys = []string{"TEST=", "DELIVERY_HEAD=", "PGDATA=/var/lib/"}

// capsKeyRefuseLines are real-shaped secrets of a similar length written as KEY=<value>
// behind a key the new rule would accept, plus the bounds on the key itself. Every one
// must keep refusing. The second return value maps each random row to its bare value.
func capsKeyRefuseLines() (map[string]string, map[string]string) {
	cases := map[string]string{
		// Prefix arms: refused before the run loop is reached.
		"github ghp_":         "TEST=" + scanGHToken,
		"github fine-grained": "TEST=" + "github" + "_pat_11ABCDEFG0123456789_abcdefghijklmnop",
		"gitlab glpat-":       "TEST=" + glToken,
		"gitlab in a path":    "PGDATA=/var/lib/" + glToken,
		"aws key id":          "TEST=" + scanAWSKeyID,
		"aws key id as head":  "DELIVERY_HEAD=" + scanAWSKeyID,
		// The run loop: the value is not exempt alone, so the key earns nothing.
		"aws secret key":         "TEST=" + awsExampleSecretKey,
		"aws secret key as path": "PGDATA=" + awsExampleSecretKey, // carries '/', not a path
		"aws secret key as head": "DELIVERY_HEAD=" + awsExampleSecretKey,
		"40-char base62":         "TEST=" + scanSecret40,
		"base62 path segment":    "PGDATA=/var/lib/" + scanSecret40,
		"base62 as head":         "DELIVERY_HEAD=" + scanSecret40,
		"48 hex as head":         "DELIVERY_HEAD=" + fixHex64[:48],
		"camel tail as path":     "PGDATA=" + capsTailPath,
		"b62 tail as path":       "PGDATA=" + capsLowPath,
		"tail path behind head":  "DELIVERY_HEAD=" + capsLowPath,
		"upper hex as head":      "DELIVERY_HEAD=" + strings.ToUpper(fixHex40),
		"camel head, tail":       "TEST=" + capsCamelTl,
		"k8s head, tail":         "TEST=" + camelPrefix,
		"plural head, tail":      "RUN=" + camelPrefix2,
		"leading acronym":        "TEST=" + identLeadAcr, // 2026-10-02 ruling: case (a) refuses
		"two mid acronyms":       "TEST=" + twoPlurals,
		"padding in the value":   "TEST=" + capsIdent + "==",
		// The key: a credential or digest stem anywhere in the full name, an over-long
		// key, a digit, lowercase, a nested assignment.
		"password key":        "PASSWORD=" + pass,
		"credential key":      "TOKEN=" + capsIdent,
		"secret key":          "export SECRET=" + capsIdent,
		"stem before tail":    "export API_KEY_TEST=" + capsIdent,
		"stem in prefix":      "SESSION_RUN=" + capsIdent,
		"auth key":            "AUTH=" + capsIdent,
		"cert dir path":       "CERT_DIR=/etc/ssl/" + "private/postgresql/server",
		"secret-named sha":    "WEBHOOK_SECRET=" + fixHex40,
		"deploy-named sha":    "DEPLOY_KEY=" + fixHex64,
		"stem key, sha":       "TOKEN=" + fixHex40,
		"stem name, sha":      "GITHUB_TOKEN=" + fixHex40,
		"password hex64":      "DB_PASSWORD=" + fixHex64,
		"session hex64":       "SESSION=" + fixHex64,
		"secret dir path":     "SECRET_DIR=" + capsPath,
		"stem file path":      "export TOKEN_FILE=" + capsPath,
		"digest-named key":    "ARG BASE_DIGEST_HEX=" + capsIdent,
		"sha-named key":       "IMAGE_SHA_RUN=" + capsIdent,
		"hex, no digest":      "MASTER_HEX=" + fixHex64,
		"17-letter key":       "ABCDEFGHI" + "JKLMNOPQ" + "=" + capsIdent,
		"17-letter sha key":   "ABCDEFGHI" + "JKLMNOPQ" + "=" + fixHex40,
		"key with a digit":    "TEST1=" + capsIdent,
		"mixed-case key":      "TESt=" + capsIdent,
		"nested assignment":   "TEST=A=" + capsIdent,
		"nested sha":          "HEAD=A=" + fixHex40,
		"plus-quantified sha": "HEAD=+" + fixHex40,
	}
	// The path lookalikes must not be exempt bare either, or their rows prove nothing.
	bare := map[string]string{
		"camel tail as path": capsTailPath,
		"b62 tail as path":   capsLowPath,
	}
	const b62 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789"
	rng := rand.New(rand.NewSource(16421110))
	for _, n := range []int{36, 48, 64} {
		for _, an := range []struct{ name, alpha string }{{"base62", b62}, {"base64", base64Alphabet}} {
			b := make([]byte, n)
			for i := range b {
				b[i] = an.alpha[rng.Intn(len(an.alpha))]
			}
			for _, key := range capsRandomKeys {
				name := an.name + " random " + string(rune('0'+n/10)) + string(rune('0'+n%10)) + " " + key
				cases[name] = key + string(b)
				bare[name] = key[strings.IndexByte(key, '=')+1:] + string(b)
			}
		}
	}
	return cases, bare
}

// TestAssignmentCapsKeyRefuse pins the bounds of the admissions above: the value must be
// an exempt identifier, git SHA or path on its own, and the key must be at most 16 ALL-CAPS
// letters in a name that carries no credential or digest stem.
// regression: #1642 #5825 #5974
func TestAssignmentCapsKeyRefuse(t *testing.T) {
	cases, bare := capsKeyRefuseLines()
	for name, line := range cases {
		if err := BodyCheck([]byte(line + "\n")); !IsRefused(err) {
			t.Errorf("%s: BodyCheck(%q) = %v, want Refused", name, line, err)
		}
		if err := ScanSurfaceSecrets("branch diff vs origin/main", []byte(" "+line+"\n")); !IsRefused(err) {
			t.Errorf("%s: ScanSurfaceSecrets(%q) = %v, want Refused", name, line, err)
		}
	}
	// No random value is exempt on its own; were one, its row would be admitted for the
	// value's sake and prove nothing about the key.
	for name, val := range bare {
		if isGitSHA(val) || isPathLike(val) || isIdentifierLike(val) {
			t.Errorf("%s: fixture value %q is itself exempt; re-seed", name, val)
		}
	}
}
