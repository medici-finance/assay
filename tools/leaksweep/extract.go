package main

// Extraction. A leak sweep that only reads text has a silent hole: `grep -I`
// skips binary files, so a PDF with a name in its /Info metadata, or a token
// hidden in a FlateDecode content stream, sweeps clean. #245 recorded exactly
// this — the external-review PDFs and rendered HTML were never swept for embedded
// metadata or mock text. A text-only sweep reporting clean over such a file is
// the same class of defect as the word-boundary hole this brief exists to close.
//
// So every file gets a disposition (brief-06 task 3: extracted-and-swept OR
// refused from the staged tree):
//
//   - text files (valid UTF-8, no NUL): the content itself. An ASCII token in
//     valid UTF-8 cannot evade a fixed-byte substring, so text is always an
//     exhaustive search.
//   - searchable binaries: a format the sweep decompresses COMPLETELY, so the
//     same exhaustive-search guarantee holds. Today that is PDF (raw /Info
//     metadata + every zlib FlateDecode content stream) and PNG (raw tEXt
//     chunks + zlib zTXt/iTXt). Their raw bytes plus every embedded zlib stream
//     is the searchable representation.
//   - UNSEARCHABLE binaries: every other binary (ZIP/DOCX/XLSX raw-DEFLATE
//     members, GZIP, raw DEFLATE, and any format the sweep does not recognise).
//     These are REFUSED, not searched, and surface as could-not-check (#528):
//     a token hidden in a compression/encoding layer the sweep does not unwrap
//     would otherwise read as checked-clean. This is an allowlist, not a
//     blocklist — an unrecognised binary is refused by default, because the only
//     proof of absence the sweep can offer is "I decompressed every layer", and
//     for an unknown format it cannot make that claim.
//
// A file that cannot be READ AT ALL (permission, I/O) is NOT silently dropped —
// it is surfaced as unreadable, which the sweep turns into could-not-check. The
// one thing this tool never does is treat "could not look" as "nothing there".

import (
	"bytes"
	"compress/zlib"
	"io"
	"unicode/utf8"
)

// extracted is the searchable form of one file, or the note that the file is a
// binary the sweep refuses to certify. kind is one of:
//   - "text": a text file; text is the content.
//   - "binary": a searchable binary (PDF/PNG); text is raw bytes + every inflated
//     zlib stream.
//   - "binary-unsearchable": a binary the sweep cannot exhaustively search; text
//     is empty and the sweep surfaces the path as could-not-check (#528).
type extracted struct {
	text string
	kind string
}

// isText reports whether data should be searched as text. It mirrors the rule
// `grep -I` uses to decide a file is binary — a NUL byte — but adds a UTF-8
// validity check so that, for example, a Latin-1 file with high bytes is still
// treated as binary and gets the raw+inflate path rather than a lossy decode.
func isText(data []byte) bool {
	if bytes.IndexByte(data, 0) >= 0 {
		return false
	}
	return utf8.Valid(data)
}

// extract turns file bytes into searchable text, or marks the file as a binary
// the sweep cannot exhaustively search. See the package comment for the three
// dispositions.
func extract(data []byte) extracted {
	if isText(data) {
		return extracted{text: string(data), kind: "text"}
	}
	if !searchableBinary(data) {
		// A binary whose content the sweep cannot prove it fully decompressed.
		// Searching it anyway would risk a silent checked-clean over a token
		// hidden in a layer the sweep does not unwrap; refusing it surfaces the
		// gap as could-not-check instead.
		return extracted{kind: "binary-unsearchable"}
	}
	var b bytes.Buffer
	// Raw bytes first: this is what catches UNCOMPRESSED metadata — the /Info
	// dictionary of a PDF, a PNG tEXt chunk, an EXIF string. A text-only sweep
	// skips the whole file; here the raw bytes are always searched.
	b.Write(data)
	// Then every embedded zlib stream, inflated. This is what catches text in a
	// PDF FlateDecode content stream or a PNG zTXt chunk — bytes that are not
	// present in the raw form at all.
	inflateAll(data, &b)
	return extracted{text: b.String(), kind: "binary"}
}

// searchableBinary reports whether a binary file can be exhaustively searched by
// this tool's raw-byte + zlib-inflate path. It is an allowlist of formats whose
// ONLY compression/encoding layer is zlib/FlateDecode — so inflating every zlib
// stream plus scanning the raw bytes reaches every byte of text the file can
// carry. Anything not on the list is refused (could-not-check), because for an
// unknown or container format the sweep cannot claim it unwrapped every layer,
// and a token behind a layer it missed would certify dirty (#528).
//
// To admit a new binary format, prove the raw+inflate path reaches all of its
// text (add a test that plants a token in the format's most-compressed stream
// and asserts checked-failed), then add its magic here. PDF and PNG are the
// formats brief-06's review proved exhaustive.
func searchableBinary(data []byte) bool {
	switch {
	case hasMagic(data, "%PDF"): // PDF: /Info (raw) + FlateDecode (zlib)
		return true
	case len(data) >= 4 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G': // PNG: tEXt (raw) + zTXt (zlib)
		return true
	default:
		return false
	}
}

// hasMagic is a prefix check by byte value, so a magic string never needs
// escaping and stays readable.
func hasMagic(data []byte, magic string) bool {
	return len(data) >= len(magic) && string(data[:len(magic)]) == magic
}

// maxInflate caps the decompressed bytes appended per file so a zip-bomb-shaped
// artifact cannot exhaust memory. A leak is a short string; megabytes of inflated
// text is plenty to find one, and the cap is a refusal-to-hang, not a
// correctness compromise for the artifact sizes this tree carries.
const maxInflate = 64 << 20 // 64 MiB

// inflateAll scans data for zlib stream headers and appends whatever each one
// decompresses to. It is deliberately generous: it tries every plausible header
// position and ignores the ones that do not decompress, because a false start
// costs nothing and a missed stream is a missed leak.
func inflateAll(data []byte, out *bytes.Buffer) {
	total := 0
	for i := 0; i+1 < len(data); i++ {
		if !isZlibHeader(data[i], data[i+1]) {
			continue
		}
		zr, err := zlib.NewReader(bytes.NewReader(data[i:]))
		if err != nil {
			continue
		}
		limited := io.LimitReader(zr, int64(maxInflate-total))
		n, _ := io.Copy(out, limited)
		zr.Close()
		total += int(n)
		if total >= maxInflate {
			return
		}
	}
}

// isZlibHeader reports whether the two bytes are a zlib stream header. zlib's
// first byte is CMF (compression method 8 in the low nibble → 0x?8), and the
// pair (CMF*256 + FLG) must be a multiple of 31. That check alone yields few
// false starts, and a false start is discarded harmlessly by zlib.NewReader.
func isZlibHeader(cmf, flg byte) bool {
	if cmf&0x0f != 8 {
		return false
	}
	return (uint16(cmf)<<8|uint16(flg))%31 == 0
}
