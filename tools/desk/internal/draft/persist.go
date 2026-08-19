package draft

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// DRAFT PERSISTENCE (§7.6), AND ITS BOUNDARY
// ------------------------------------------
// "A draft survives navigation/failure so a composed answer is never lost
// before the human sees it." Surviving needs two things: a form that
// round-trips exactly, and somewhere to keep it. This package supplies the
// FORM and cannot supply the store, because it cannot write — which is the
// same property that makes it unable to post, and is not negotiable for one
// half without losing the other.
//
// So [Draft.Encode] and [Decode] are pure string functions. The shell that
// owns the store (absent) holds the bytes.
//
// FAIL CLOSED ON A DAMAGED RESTORE
// --------------------------------
// A half-restored draft is the dangerous shape: the body survives and a
// caveat does not, leaving a confident claim with its qualification silently
// stripped, in front of a human about to put their name on it. So the
// envelope carries a digest over its own canonical form and [Decode] REFUSES
// anything that does not match. There is no partial restore and no
// best-effort path — a damaged payload yields the zero Draft and an error.

const encodeVersion = "draft/v1"

// Encode renders a draft into a self-describing, digest-checked payload. It is
// deterministic: the same draft encodes to the same bytes every time, so a
// store can dedupe on it and a test can compare on it.
func (d Draft) Encode() string {
	body := d.canonical()
	sum := sha256.Sum256([]byte(body))
	return body + "sum\t" + hex.EncodeToString(sum[:]) + "\n"
}

// canonical is everything the digest covers.
func (d Draft) canonical() string {
	var b strings.Builder
	b.WriteString(encodeVersion + "\n")
	writeField(&b, "kind", string(d.kind))
	writeField(&b, "repo", d.target.Repo)
	writeField(&b, "tkind", d.target.Kind)
	writeField(&b, "ref", d.target.Ref)
	writeField(&b, "title", d.title)
	writeField(&b, "body", d.body)
	for i, id := range d.cited {
		chip := ""
		if i < len(d.chips) {
			chip = d.chips[i]
		}
		b.WriteString("cite\t" + esc(id) + "\t" + esc(chip) + "\n")
	}
	for _, c := range d.caveats {
		writeField(&b, "caveat", c)
	}
	return b.String()
}

func writeField(b *strings.Builder, k, v string) { b.WriteString(k + "\t" + esc(v) + "\n") }

// esc makes a value safe for a tab-separated, newline-terminated line. The
// order matters: backslash first, or the escapes escape each other.
func esc(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// unesc reverses esc. It is written as a single left-to-right scan rather than
// three ReplaceAll calls, because sequential replacement of overlapping
// escapes does not round-trip: `\\n` would become a newline instead of a
// literal backslash followed by n.
func unesc(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		i++
		if i >= len(s) {
			return "", fmt.Errorf("%w: payload ends in a dangling escape", ErrMalformed)
		}
		switch s[i] {
		case '\\':
			b.WriteByte('\\')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		default:
			return "", fmt.Errorf("%w: payload holds an undeclared escape %q", ErrMalformed, `\`+string(s[i]))
		}
	}
	return b.String(), nil
}

// Decode restores a draft from an [Draft.Encode] payload, or refuses. It never
// returns a partially restored draft: every refusal yields the zero Draft.
func Decode(payload string) (Draft, error) {
	lines := strings.Split(payload, "\n")
	if len(lines) == 0 || lines[0] != encodeVersion {
		return Draft{}, fmt.Errorf("%w: payload is not %s", ErrMalformed, encodeVersion)
	}

	// Locate the digest line and verify BEFORE interpreting any field, so a
	// tampered payload never gets as far as producing a value.
	var sumIdx = -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "sum\t") {
			sumIdx = i
			break
		}
	}
	if sumIdx < 0 {
		return Draft{}, fmt.Errorf("%w: payload carries no digest, so a truncated restore would be indistinguishable from a short draft", ErrMalformed)
	}
	canonical := strings.Join(lines[:sumIdx], "\n") + "\n"
	want := strings.TrimPrefix(lines[sumIdx], "sum\t")
	got := sha256.Sum256([]byte(canonical))
	if hex.EncodeToString(got[:]) != want {
		return Draft{}, fmt.Errorf("%w: the payload's digest does not match its content — a damaged restore is refused whole, because the field most likely to be missing is a caveat", ErrMalformed)
	}

	var d Draft
	for _, ln := range lines[1:sumIdx] {
		if ln == "" {
			continue
		}
		k, rest, ok := strings.Cut(ln, "\t")
		if !ok {
			return Draft{}, fmt.Errorf("%w: payload line %q has no field separator", ErrMalformed, ln)
		}
		if k == "cite" {
			id, chip, ok := strings.Cut(rest, "\t")
			if !ok {
				return Draft{}, fmt.Errorf("%w: a citation line carries no provenance chip", ErrMalformed)
			}
			uid, err := unesc(id)
			if err != nil {
				return Draft{}, err
			}
			uchip, err := unesc(chip)
			if err != nil {
				return Draft{}, err
			}
			d.cited = append(d.cited, uid)
			d.chips = append(d.chips, uchip)
			continue
		}
		v, err := unesc(rest)
		if err != nil {
			return Draft{}, err
		}
		switch k {
		case "kind":
			d.kind = Kind(v)
		case "repo":
			d.target.Repo = v
		case "tkind":
			d.target.Kind = v
		case "ref":
			d.target.Ref = v
		case "title":
			d.title = v
		case "body":
			d.body = v
		case "caveat":
			d.caveats = append(d.caveats, v)
		default:
			return Draft{}, fmt.Errorf("%w: payload holds an undeclared field %q — an unrecognised field is refused, not ignored, because ignoring it is how a caveat field silently stops being read", ErrMalformed, k)
		}
	}

	// A restored draft must still satisfy the composition rules it was built
	// under. A payload that decodes cleanly but violates them was not produced
	// by Compose, and is refused rather than trusted for having a valid digest.
	if !d.kind.valid() {
		return Draft{}, fmt.Errorf("%w: restored draft declares kind %q", ErrMalformed, d.kind)
	}
	if err := d.target.Validate(); err != nil {
		return Draft{}, err
	}
	if strings.TrimSpace(d.title) == "" || strings.TrimSpace(d.body) == "" {
		return Draft{}, fmt.Errorf("%w: restored draft has no title or no body", ErrMalformed)
	}
	if len(d.cited) != len(d.chips) {
		return Draft{}, fmt.Errorf("%w: restored draft has %d citations and %d provenance chips", ErrMalformed, len(d.cited), len(d.chips))
	}
	if len(d.cited) > 0 && len(d.caveats) == 0 {
		return Draft{}, fmt.Errorf("%w: restored draft quotes %d citation(s) and carries no caveat — the exact loss this digest exists to catch", ErrUncaveatedClaim, len(d.cited))
	}
	return d, nil
}
