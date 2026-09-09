//go:build !untrustscan_broken_unicode

package deskkit

// injectionUnicodeRanges is the LIVE Unicode smuggling-channel table the injection family
// scans for: the Tag block, zero-width joiners + BOM, and the bidi embedding/override/
// isolate controls. Each range is tagged with the marker it fires.
//
// This file's twin (untrustscan_unicode_broken.go, built under the
// `untrustscan_broken_unicode` tag) provides the SAME symbol EMPTIED. Verify row 5 builds
// with that tag and asserts the injection detector goes silent on row 3's sample — a green
// Verify over a live control, not a lamp wired to nothing.
var injectionUnicodeRanges = []unicodeRange{
	{0xE0000, 0xE007F, "unicode-tagblock"},
	{0x200B, 0x200D, "unicode-zerowidth"},
	{0xFEFF, 0xFEFF, "unicode-zerowidth"},
	{0x202A, 0x202E, "bidi-override"},
	{0x2066, 0x2069, "bidi-override"},
}
