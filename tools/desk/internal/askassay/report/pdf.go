package report

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// A DETERMINISTIC PDF WRITER, AND WHY IT IS WRITTEN HERE
// ------------------------------------------------------
// "A PDF is a byte-identical function of the index at a timestamp" is the
// brief's own claim, and on this repo it is currently FALSE for every
// committed PDF, for a measured reason:
//
//   - renderdriftguard asks the right question — does the artifact still say
//     what the source renders to — and is red on Linux CI for every pair,
//     because an HTML-to-PDF render diverges by platform font stack. The
//     ruled fix is a pinned container render, and it is unbuilt. So a PDF
//     produced by driving a browser cannot be verified anywhere but the
//     machine that produced it.
//   - The guard that IS green (artifactguard) asks a weaker question — did
//     source and artifact last change in the same commit — and was baselined
//     over an existing drift: a committed page carries a "reviewed" line its
//     committed PDF does not, and all three of the guard's required-phrase
//     assertions report PRESENT against that stale PDF.
//
// Both failures come from the same place: the renderer is a large external
// program whose output depends on the host. This writer removes the host from
// the loop. It emits PDF bytes directly from the document model using the
// base-14 Helvetica faces, which every conforming reader supplies, so no font
// is embedded, no font is measured, and no glyph rasteriser participates.
//
// The remaining non-determinism in a PDF is enumerated and closed here rather
// than hoped about:
//
//   - /CreationDate and /ModDate: derived from the document's declared as-of
//     stamp. This file contains no call to time.Now, which
//     TestWriterReadsNoClock asserts by reading this source.
//   - /ID: a content hash of the body, not a random nonce.
//   - Object order: fixed by construction; no map is ever ranged over.
//   - Compression: none. Content streams are plain text, which also means the
//     text is extractable without any external tool — see extract.go, and the
//     reason that matters: a binary artifact can carry text no reviewer ever
//     reads, and a person's given name was found in customer-facing copy
//     exactly that way.
//   - Number formatting: fixed precision, "%.2f", locale-independent.
//
// The bound on the claim, stated: this is byte-determinism of the FILE, not
// pixel-determinism of the RENDER. Two readers may lay out Helvetica slightly
// differently. What is guaranteed is that the artifact committed to the tree
// is a pure function of the index and the stamp, so a drift check that
// re-renders and diffs BYTES is meaningful — which is precisely what
// renderdriftguard cannot do today for a browser-produced PDF.

const (
	pageW      = 612.0
	pageH      = 792.0
	marginX    = 54.0
	marginTop  = 54.0
	marginBot  = 54.0
	leadBody   = 13.0
	leadHead   = 20.0
	sizeH1     = 16.0
	sizeH2     = 12.0
	sizeBody   = 9.5
	sizeSmall  = 8.0
	barTrackW  = 300.0
	barH       = 9.0
	linesLimit = 4096 // refuse absurd documents rather than loop forever
)

// op is one drawing instruction in the page-independent document model.
// Everything a report can put on a page is one of these, so pagination is a
// pure function over a slice.
type op struct {
	kind    string // "h1" | "h2" | "body" | "small" | "rule" | "bar" | "hatch" | "gap"
	text    string
	frac    float64 // bar fill fraction 0..1
	shade   float64 // bar grey level 0..1 (single-hue ramp, printed as grey)
	advance float64 // vertical advance this op consumes
}

func (o op) height() float64 {
	if o.advance > 0 {
		return o.advance
	}
	switch o.kind {
	case "h1":
		return leadHead + 6
	case "h2":
		return leadHead
	case "rule":
		return 10
	case "bar", "hatch":
		return barH + 6
	case "gap":
		return 8
	default:
		return leadBody
	}
}

// buildPDF serialises the ops into PDF bytes. It is the only function that
// writes PDF syntax.
func buildPDF(title string, stamp time.Time, ops []op) ([]byte, error) {
	if len(ops) > linesLimit {
		return nil, fmt.Errorf("report refused: %d layout operations exceeds the %d cap — a document this long is a data dump, not a report", len(ops), linesLimit)
	}
	pages := paginate(ops)
	if len(pages) == 0 {
		return nil, fmt.Errorf("report refused: the document has no content, and a blank PDF is indistinguishable from a report of zeroes")
	}

	var contents []string
	for i, p := range pages {
		contents = append(contents, pageContent(p, i+1, len(pages)))
	}

	// Object numbering, fixed: 1 catalog, 2 pages, then per page a page
	// object and a content stream, then the two fonts, then info.
	n := len(pages)
	objs := make([]string, 0, 2*n+5)
	kids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i*2))
	}
	fontRegular := 3 + n*2
	fontBold := fontRegular + 1
	infoObj := fontBold + 1

	objs = append(objs, "<< /Type /Catalog /Pages 2 0 R >>")
	objs = append(objs, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n))
	for i := 0; i < n; i++ {
		objs = append(objs, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>",
			pageW, pageH, fontRegular, fontBold, 4+i*2))
		objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contents[i]), contents[i]))
	}
	objs = append(objs, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	objs = append(objs, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	objs = append(objs, fmt.Sprintf(
		"<< /Title (%s) /Producer (assay ask-assay report writer) /Creator (assay ask-assay report writer) /CreationDate (%s) /ModDate (%s) >>",
		pdfString(title), pdfDate(stamp), pdfDate(stamp)))

	var body strings.Builder
	body.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs)+1)
	for i, o := range objs {
		offsets[i+1] = body.Len()
		fmt.Fprintf(&body, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xrefAt := body.Len()
	fmt.Fprintf(&body, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&body, "%010d 00000 n \n", offsets[i])
	}

	// /ID is a content hash, never a nonce: a random file identifier is the
	// single most common reason two renders of the same document differ.
	sum := sha256.Sum256([]byte(body.String()))
	id := strings.ToUpper(hex.EncodeToString(sum[:16]))
	fmt.Fprintf(&body, "trailer\n<< /Size %d /Root 1 0 R /Info %d 0 R /ID [<%s> <%s>] >>\nstartxref\n%d\n%%%%EOF\n",
		len(objs)+1, infoObj, id, id, xrefAt)
	return []byte(body.String()), nil
}

func paginate(ops []op) [][]op {
	var pages [][]op
	var cur []op
	y := pageH - marginTop
	for _, o := range ops {
		h := o.height()
		if y-h < marginBot+18 && len(cur) > 0 {
			pages = append(pages, cur)
			cur = nil
			y = pageH - marginTop
		}
		cur = append(cur, o)
		y -= h
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	return pages
}

func pageContent(ops []op, pageNo, pageCount int) string {
	var b strings.Builder
	y := pageH - marginTop
	for _, o := range ops {
		switch o.kind {
		case "h1":
			y -= leadHead
			text(&b, "F2", sizeH1, marginX, y, o.text)
			y -= 6
		case "h2":
			y -= leadHead
			text(&b, "F2", sizeH2, marginX, y, o.text)
		case "small":
			y -= leadBody
			text(&b, "F1", sizeSmall, marginX, y, o.text)
		case "rule":
			y -= 10
			fmt.Fprintf(&b, "0.45 0.45 0.45 RG 0.6 w %.2f %.2f m %.2f %.2f l S\n", marginX, y+3, pageW-marginX, y+3)
		case "gap":
			y -= 8
		case "bar":
			y -= barH + 6
			// Track, then fill. The fill is a single-hue ramp expressed as a
			// grey level so the artifact survives a monochrome print — §7.6's
			// no-colour-only rule applies to paper too, and the numeral is
			// printed beside the bar regardless.
			fmt.Fprintf(&b, "0.16 0.16 0.16 rg %.2f %.2f %.2f %.2f re f\n", marginX, y, barTrackW, barH)
			w := barTrackW * clamp01(o.frac)
			if w > 0 {
				g := 0.28 + 0.42*clamp01(o.shade)
				fmt.Fprintf(&b, "%.2f %.2f %.2f rg %.2f %.2f %.2f %.2f re f\n", g*0.55, g*0.78, g, marginX, y, w, barH)
			}
			text(&b, "F1", sizeBody, marginX+barTrackW+10, y+1, o.text)
		case "hatch":
			y -= barH + 6
			// COULD-NOT-CHECK on paper: the full track, hatched, carrying the
			// literal token. Never an empty track — an empty track is a bar
			// of length zero, which is a drawn claim that the value is 0.
			fmt.Fprintf(&b, "0.16 0.16 0.16 rg %.2f %.2f %.2f %.2f re f\n", marginX, y, barTrackW, barH)
			fmt.Fprintf(&b, "q %.2f %.2f %.2f %.2f re W n\n", marginX, y, barTrackW, barH)
			fmt.Fprintf(&b, "0.48 0.50 0.53 RG 0.8 w\n")
			for x := marginX - barH; x < marginX+barTrackW; x += 5 {
				fmt.Fprintf(&b, "%.2f %.2f m %.2f %.2f l S\n", x, y, x+barH, y+barH)
			}
			b.WriteString("Q\n")
			text(&b, "F1", sizeBody, marginX+barTrackW+10, y+1, "? "+o.text)
		default:
			y -= leadBody
			text(&b, "F1", sizeBody, marginX, y, o.text)
		}
	}
	text(&b, "F1", sizeSmall, marginX, marginBot-14,
		fmt.Sprintf("page %d of %d", pageNo, pageCount))
	return b.String()
}

func text(b *strings.Builder, font string, size, x, y float64, s string) {
	fmt.Fprintf(b, "BT /%s %.2f Tf 0.91 0.91 0.91 rg %.2f %.2f Td (%s) Tj ET\n", font, size, x, y, pdfString(s))
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// pdfDate formats a PDF date string. It takes its instant from the caller —
// the document's declared as-of stamp — so the artifact does not change
// because a second elapsed.
func pdfDate(t time.Time) string {
	return t.UTC().Format("D:20060102150405") + "+00'00'"
}

// transliterations maps the characters this system's prose actually uses onto
// WinAnsi-representable ASCII. It is a fixed, sorted-by-construction slice
// rather than a map, because ranging a map is one of the classic sources of
// non-deterministic output.
var transliterations = [][2]string{
	{"⟡", "[gate]"},
	{"—", "-"},
	{"–", "-"},
	{"·", " - "},
	{"…", "..."},
	{"≥", ">="},
	{"≤", "<="},
	{"→", "->"},
	{"‡", "*"},
	{"“", `"`},
	{"”", `"`},
	{"‘", "'"},
	{"’", "'"},
	{"×", "x"},
	{"Δ", "delta "},
}

// pdfString escapes a Go string for a PDF literal string, transliterating
// anything outside printable ASCII. An unmappable rune becomes "?" — visible,
// rather than a silently dropped character in a customer-facing artifact.
func pdfString(s string) string {
	for _, t := range transliterations {
		s = strings.ReplaceAll(s, t[0], t[1])
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '(':
			b.WriteString(`\(`)
		case r == ')':
			b.WriteString(`\)`)
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case r >= 32 && r < 127:
			b.WriteRune(r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}
