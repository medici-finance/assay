package main

// The searchable projection — the layer that feeds the external engines.
//
// WHY THIS EXISTS, AND WHY extract.go IS A KEEP RATHER THAN A DELETE
// ------------------------
// The brief's file list marks extract.go as part of the engine being replaced.
// Measured against the actual scanners, that is wrong, and keeping it is the
// difference between detection parity and a regression. Both engines were run
// against this repo's own dirty fixture with a rule for the token planted inside
// leak.pdf's FlateDecode stream:
//
//   the token appears ZERO times in the PDF's raw bytes
//   gitleaks    → 1 finding, and it is the PROSE MENTION in the sibling .md
//   trufflehog  → 1 finding, the same prose mention
//   neither engine saw the planted token at all
//
// Neither scanner inflates a PDF content stream. Deleting extract.go and pointing
// the engines at the raw tree would therefore have shipped a green certificate
// over a tree with a token in it — reintroducing, on day one, the exact class of
// false clean-certificate this brief exists to end. The extraction layer is not
// part of the detection engine; it is the part that decides WHAT BYTES ARE
// SEARCHABLE AT ALL, and that judgment is ours to keep.
//
// So the flow is: corpus (extract.go's dispositions) → projection → engines.
// The engines get strictly MORE reach than they have natively, and the #528
// unsearchable-binary refusal still stands in front of everything.
//
// UNIFORM NAME MANGLING, WHICH IS A CORRECTNESS PROPERTY
// ------------------------
// Every projected file is written as `<original-path>.leaksweep-scan.txt`. This
// is not cosmetic:
//
//   - `.gitignore` / `.gitleaksignore` / `.trufflehogignore` copied into a scan
//     directory would FUNCTION there, silently removing files from the scan. A
//     swept tree could then exclude itself from its own sweep. Mangled, their
//     contents are still searched and their behaviour is inert.
//   - both engines' default configs skip paths by extension (images, archives,
//     lock files). Mangling defeats those skips, so a token in a skipped
//     extension is still read.
//   - extracted binary text has no meaningful original extension anyway.
//
// The mapping back to the tree-relative path is a single suffix strip, so a
// finding always names the real file.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Small aliases so scrubForScan reads as one idea rather than a pile of
// stdlib calls.
const utf8SelfMax = utf8.RuneSelf
const runeError = utf8.RuneError

func decodeRune(b []byte) (rune, int) { return utf8.DecodeRune(b) }
func itoa(i int) string               { return strconv.Itoa(i) }

// projectionSuffix is appended to every projected path. See the file comment for
// why this is load-bearing rather than tidy.
const projectionSuffix = ".leaksweep-scan.txt"

// canaryDir is the reserved directory inside the projection where the canary
// fixture is planted. Findings under it are the liveness proof, never leaks, and
// are subtracted before any leak is reported.
const canaryDir = "_leaksweep_canary"

// beaconPrefix marks the per-file read-proof beacon. See buildProjection.
const beaconPrefix = "LEAKSWEEP-FILE-BEACON-"

// beaconRuleID is the rule that hunts the beacons.
const beaconRuleID = "leaksweep-beacon"

// projection is a temporary directory holding the searchable text of every file
// the corpus could search, plus the canary.
//
// beacons maps beacon index → tree-relative path, so the orchestrator can turn
// "which beacons came back" into "which files the engine actually read".
type projection struct {
	dir     string
	beacons map[int]string
}

// buildProjection materialises the corpus as a scannable directory tree.
//
// TWO THINGS HAPPEN HERE THAT ARE NOT OBVIOUS, and both were found by running
// the real scanners rather than by reasoning about them.
//
//  1. THE TEXT IS SCRUBBED. The extracted form of a PDF is raw bytes plus every
//     inflated zlib stream, so it contains NUL bytes. Handed that file, gitleaks
//     reported `scanned ~0 bytes` and found nothing — it classifies a NUL-bearing
//     file as binary and skips it wholesale. The planted token was IN the file and
//     the scanner declined to look. Scrubbing to text-safe bytes (below) is what
//     makes the extraction layer actually reach the engines.
//
//  2. EVERY FILE CARRIES A BEACON. The failure above is the general shape of the
//     worst bug this tool can have: the scanner ran, exited 0, reported nothing,
//     and had silently skipped a file. No amount of rule quality detects that.
//     So each projected file ends with a UNIQUE marker, and one rule hunts them
//     all; the orchestrator then compares the beacons that came back against the
//     files it wrote. A file whose beacon does not return was not read, and is
//     reported as could-not-check BY NAME. This turns "the engine scanned the
//     tree" from an assumption into per-file evidence — which is the same move the
//     per-token control makes one level up, applied to file coverage.
//
//     The beacons are unique per file on purpose: trufflehog de-duplicates
//     findings by secret value, so an identical marker in 600 files would come
//     back as one finding and prove nothing about the other 599.
func buildProjection(c *corpus) (*projection, error) {
	dir, err := os.MkdirTemp("", "leaksweep-projection-")
	if err != nil {
		return nil, withExit(exitCouldNotCheck, "could-not-check: cannot create a scan projection: %v", err)
	}
	p := &projection{dir: dir, beacons: map[int]string{}}
	for i, f := range c.files {
		p.beacons[i] = f.path
		// The header line is not a label — it is what stops the scanners' content
		// sniffing from skipping the file.
		//
		// gitleaks classifies a file by its MAGIC BYTES, not its extension. A PDF's
		// extracted text still begins `%PDF`, so even after scrubbing to clean UTF-8
		// and renaming to `.txt`, gitleaks reported `scanned ~0 bytes` and exited 0
		// on a file with a planted token in it. Displacing the magic by one line
		// makes the same file scan (`scanned ~751 bytes`, token found). The header
		// also names the real source path, so a human reading a raw projection can
		// tell what they are looking at.
		header := "leaksweep-projection: " + f.path + "\n"
		body := header + neutraliseMagic(scrubForScan(f.ex.text)) + "\n" + beaconPrefix + itoa(i) + "\n"
		if err := p.write(f.path+projectionSuffix, []byte(body)); err != nil {
			p.cleanup()
			return nil, withExit(exitCouldNotCheck,
				"could-not-check: cannot project %s into the scan tree (%v) — a file the engines never received cannot be reported clean", f.path, err)
		}
	}
	if err := p.write(filepath.Join(canaryDir, canaryFileName), []byte(canaryContent())); err != nil {
		p.cleanup()
		return nil, withExit(exitCouldNotCheck, "could-not-check: cannot plant the canary: %v", err)
	}
	return p, nil
}

// neutraliseMagic blanks the ASCII file-format signatures that make a scanner
// decide a file is a document and refuse to read it.
//
// THE TWO ENGINES SNIFF DIFFERENTLY, and both skip silently:
//
//	gitleaks    keys on the magic being at the START of the file. Displacing it
//	            with the projection header was enough: `scanned ~0 bytes` became
//	            `scanned ~751 bytes` and the planted token was found.
//	trufflehog  finds the magic ANYWHERE in the leading bytes and skips anyway.
//	            Measured directly: the identical file with `%PDF` rewritten to
//	            `@PDF` was read; as-is it was not; renaming it changed nothing.
//
// So the signature itself is blanked. Only the first byte is replaced, by a
// space, so length and every offset are preserved and nothing that could carry a
// token is touched — a format signature is not a secret.
//
// This is a blocklist, and blocklists are exactly what this tool distrusts. It is
// safe here only because it is not load-bearing: the per-file beacon check is
// what guarantees coverage. If some future format signature causes a skip this
// list does not know about, the beacon for that file does not come back and the
// run reports could-not-check by name. This function makes the common case
// WORK; the beacon is what makes the uncommon case FAIL LOUDLY.
//
// PNG needs no entry: its signature begins 0x89, which is not valid UTF-8 and is
// already blanked by scrubForScan.
func neutraliseMagic(s string) string {
	for _, sig := range []string{"%PDF"} {
		s = strings.ReplaceAll(s, sig, " "+sig[1:])
	}
	return s
}

// scrubForScan makes extracted text safe for a scanner that skips binary files,
// WITHOUT losing any token.
//
// Every byte that would make a scanner call the file binary — NUL, and any byte
// sequence that is not valid UTF-8 — is replaced by a space, ONE BYTE FOR ONE
// BYTE. Offsets are preserved, so a reported line number still means something,
// and no ASCII token can be destroyed: a token is by construction a run of
// printable ASCII, and printable ASCII is never rewritten here. What is removed
// is exactly the bytes that carry no token and make the file unreadable to the
// engines.
func scrubForScan(s string) string {
	b := []byte(s)
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		if c == 0x00 {
			out = append(out, ' ')
			i++
			continue
		}
		if c < utf8SelfMax {
			// ASCII. Control characters other than tab/newline/carriage return are
			// replaced too: some scanners treat a high density of them as binary.
			if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
				out = append(out, ' ')
			} else {
				out = append(out, c)
			}
			i++
			continue
		}
		r, size := decodeRune(b[i:])
		if r == runeError && size == 1 {
			out = append(out, ' ')
			i++
			continue
		}
		out = append(out, b[i:i+size]...)
		i += size
	}
	return string(out)
}

func (p *projection) write(rel string, data []byte) error {
	full := filepath.Join(p.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o600)
}

// cleanup removes the projection. Setting LEAKSWEEP_KEEP_PROJECTION keeps it and
// prints its path: when an engine reports a file unread, the only way to find out
// why is to look at the bytes that engine was actually handed.
func (p *projection) cleanup() {
	if p == nil || p.dir == "" {
		return
	}
	if os.Getenv("LEAKSWEEP_KEEP_PROJECTION") != "" {
		os.Stderr.WriteString("leaksweep: projection kept at " + p.dir + "\n")
		return
	}
	os.RemoveAll(p.dir)
}

// treePath maps a projection-relative path back to the swept tree's path, and
// reports whether the finding came from the canary fixture rather than the tree.
func treePath(rel string) (path string, isCanary bool) {
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, canaryDir+"/") {
		return rel, true
	}
	return strings.TrimSuffix(rel, projectionSuffix), false
}

// buildSampleFixture writes one file per rule containing that rule's positive
// control string, and returns the directory.
//
// This is the per-rule liveness proof, and it is generated from the SAME rule
// list the engines are handed rather than from a checked-in fixture. A
// hand-maintained fixture drifts: a token gets added, nobody plants it, and the
// rule is never shown to fire — which is a rule that reports clean forever. A
// generated fixture cannot drift, because a rule with no sample cannot load at
// all (tokens.go).
//
// Control rules get the CONTROL string as their sample, for the same reason: a
// control rule that cannot fire would turn every entry it serves into a permanent
// could-not-check, which is loud but still wrong.
func buildSampleFixture(rules []rule) (string, error) {
	dir, err := os.MkdirTemp("", "leaksweep-samples-")
	if err != nil {
		return "", withExit(exitCouldNotCheck, "could-not-check: cannot create the sample fixture: %v", err)
	}
	for _, r := range rules {
		s := r.entry.Sample
		if r.kind == ruleControl {
			s = r.entry.Control
		}
		// One rule per file so a rule that fires cannot be credited to a
		// neighbouring rule's plant, and so trufflehog's per-chunk keyword
		// prefilter sees each keyword in its own chunk.
		name := r.id + projectionSuffix
		if err := os.WriteFile(filepath.Join(dir, name), []byte("leaksweep rule liveness sample\n"+s+"\n"), 0o600); err != nil {
			os.RemoveAll(dir)
			return "", withExit(exitCouldNotCheck, "could-not-check: cannot write sample for %s: %v", r.id, err)
		}
	}
	return dir, nil
}

// buildCanaryFixture writes the canary on its own, for the stock-rule pass.
func buildCanaryFixture() (string, error) {
	dir, err := os.MkdirTemp("", "leaksweep-canary-")
	if err != nil {
		return "", withExit(exitCouldNotCheck, "could-not-check: cannot create the canary fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, canaryFileName), []byte(canaryContent()), 0o600); err != nil {
		os.RemoveAll(dir)
		return "", withExit(exitCouldNotCheck, "could-not-check: cannot write the canary fixture: %v", err)
	}
	return dir, nil
}
