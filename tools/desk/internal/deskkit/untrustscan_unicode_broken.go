//go:build untrustscan_broken_unicode

package deskkit

// injectionUnicodeRanges is EMPTIED here on purpose. This file replaces its live twin
// (untrustscan_unicode.go) only under the `untrustscan_broken_unicode` build tag, which
// Verify row 5 uses to DISARM the injection detector's Unicode range table. With the
// table empty the detector must find no Tag-block / zero-width / bidi codepoints and go
// silent — the mutation the negative-path test asserts, proving row 3's control is real.
var injectionUnicodeRanges = []unicodeRange{}
