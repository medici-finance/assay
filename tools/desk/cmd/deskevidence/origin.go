package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The ORIGIN of a secret-scan verdict (#1161).
//
// The scan judges only the bytes THIS landing adds (see cmdEvidence's secret-scan comment),
// so a refusal is always about something the caller typed — but the caller sees the brief's
// text and the scan side by side and has no way to tell, from a message that names only a
// run length, whether the run it refused was theirs or one the brief carried before they
// arrived. That is what stalled #1161: a verifier isolated the trigger by hand, over several
// attempts, on a brief whose verification had already PASSED. Two helpers close that gap:
//
//   - addedOrigin maps a finding on the scanned surface back to the line of the LOCAL
//     evidence file that carries it, and withAddedOrigin appends that to the refusal, so the
//     operator opens the file they wrote and goes to that line;
//   - preexistingNotice says, on stderr and never as a refusal, where the branch copy of
//     the target already carries a secret-shaped run — `pre-existing in <path>:<line>` — so
//     the operator who sees a bare 40-hex value in the brief knows the scan has already
//     accounted for it.
//
// Neither ever carries the span itself: the finding's line is the only thing repeated, so
// the message that explains a refusal cannot become the leak the refusal exists to stop.

// addedOriginLabel is the origin every refusal on the added-bytes surface names. The
// evidence file is the only input the caller controls, on both landing paths: with
// --brief-path it is the block being merged in, without it the whole target the caller
// merged locally — so the line it names is always a line of THAT file.
const addedOriginLabel = "added by --evidence-file"

// addedOrigin renders the origin of finding line `line` (1-based) of the scanned surface as
// the corresponding line of the local evidence file. The surface is a line-subset of the
// content being committed — addedLines keeps every added line byte-for-byte, and the
// --brief-path merge trims only the block's outer whitespace — so the mapping is a lookup:
// the surface line, matched exactly against the evidence file's lines, else matched with
// both sides' surrounding whitespace trimmed (the merge's TrimSpace reaches the first and
// last line of the block). A surface line the evidence file does not carry — which the
// scoping above makes unreachable, but which this function must not guess about — is
// reported as unmapped, with its position on the added-bytes surface, rather than pinned to
// a line the operator would then look at in vain.
func addedOrigin(surface, local []byte, line int) string {
	unmapped := fmt.Sprintf("added by this landing (line %d of the added bytes; not found in --evidence-file)", line)
	surfaceLines := strings.Split(string(surface), "\n")
	if line < 1 || line > len(surfaceLines) {
		return unmapped
	}
	want := surfaceLines[line-1]
	localLines := strings.Split(string(local), "\n")
	for i, ln := range localLines {
		if ln == want {
			return fmt.Sprintf("%s:%d", addedOriginLabel, i+1)
		}
	}
	wantTrim := strings.TrimSpace(want)
	if wantTrim == "" {
		return unmapped
	}
	for i, ln := range localLines {
		if strings.TrimSpace(ln) == wantTrim {
			return fmt.Sprintf("%s:%d", addedOriginLabel, i+1)
		}
	}
	return unmapped
}

// withAddedOrigin returns the scan refusal `err` with the origin of its finding appended to
// the message. The exit code, the fail-closed semantics and the --explain payload are the
// scanner's own, unchanged — only the sentence grows, and only by a line reference. An error
// that carries no ScanFinding (nothing to locate) is returned as it is.
func withAddedOrigin(err error, surface, local []byte) error {
	var de *deskkit.DeskError
	var f *deskkit.ScanFinding
	if !errors.As(err, &de) || !errors.As(err, &f) || f == nil {
		return err
	}
	return &deskkit.DeskError{
		Code:    de.Code,
		Msg:     de.Msg + " — " + addedOrigin(surface, local, f.Line),
		Err:     de.Err,
		Finding: f,
	}
}

// preexistingNotice scans the branch copy of the target for its own account and, when it
// carries a secret-shaped run, prints ONE notice line to w naming the rule and the line —
// `pre-existing in <target>:<line>` — and nothing else. It is a NOTICE: the landing's
// verdict was already decided on the bytes it adds, and text merged to the branch before
// this landing (through the normal PR scan) is not this landing's to refuse. It exists so
// the operator who sees a bare secret-shaped value in the brief is told, in the same
// transcript, that the scan saw it too and where, instead of isolating it by hand.
//
// The secrets-only scan (no impersonated-ruling arm) is the right instrument: the question
// is "does the branch copy carry a run the scan would refuse", and a ruling-claim marker
// on a merged brief is neither a run nor this landing's concern.
func preexistingNotice(w io.Writer, target string, remote []byte) {
	err := deskkit.ScanSurfaceSecrets("target", remote)
	if err == nil {
		return
	}
	var f *deskkit.ScanFinding
	if !errors.As(err, &f) || f == nil {
		return
	}
	fmt.Fprintf(w, "deskevidence: notice: the branch copy of %s already carries a secret-shaped run "+
		"(rule %s) — pre-existing in %s:%d; it was merged before this landing and is outside its "+
		"scope, so only the bytes this landing adds were scanned\n", target, f.Rule, target, f.Line)
}
