package deskkit

import (
	"encoding/json"
	"strings"
	"testing"
)

// regression: #7838
func TestAbsPathSkipsRegexEscape(t *testing.T) {
	bs := `\`
	// Anchors as a mutation spec holds them: the regex, then its JSON encoding, so the
	// doubled backslashes are the ones a real file carries. `@` stands for a backslash in
	// the literals below, so this file's own text never spells the shape under test.
	bsAt := func(s string) string { return strings.ReplaceAll(s, "@", bs) }
	regexes := map[string]string{
		"header_class":  bsAt(`[^"@@@x00-@x20@x7f]+"@]$`),
		"value_class":   bsAt(`[^"@@#;@x00-@x1f@x7f]*$`),
		"unicode_class": bsAt(`[^@u0000-@u001f@u007f]+$`),
		"wide_class":    bsAt(`[^@U00000000-@U0000001f]+$`),
	}
	for name, re := range regexes {
		enc, err := json.Marshal(map[string]string{"anchor": re})
		if err != nil {
			t.Fatal(err)
		}
		for form, text := range map[string]string{
			"raw":  re,
			"json": string(enc),
		} {
			t.Run(name+"/"+form, func(t *testing.T) {
				if loc := findAbsMachinePath(text); loc != nil {
					t.Fatalf("regex escapes read as a machine path %q in %q", text[loc[0]:loc[1]], text)
				}
			})
		}
	}

	// The same bodies must still refuse when a real UNC path sits beside the escapes, and a
	// host that only starts like an escape stays a host.
	real := bs + bs + `fileserver` + bs + `share` + bs + `team`
	still := map[string]string{
		"real_unc_after_class": bsAt(`[^@x00-@x20] see `) + real,
		"real_unc_before":      real + bsAt(` then [^@x00-@x20]`),
		"escape_like_host":     bs + bs + `x00-files` + bs + `share`,
		"short_hex_host":       bs + bs + `x0` + bs + `share`,
		"long_hex_host":        bs + bs + `x001` + bs + `share`,
		"json_real_unc":        strings.ReplaceAll(real, bs, bs+bs),
	}
	for name, text := range still {
		t.Run("refuses/"+name, func(t *testing.T) {
			if findAbsMachinePath(text) == nil {
				t.Fatalf("UNC path not found in %q", text)
			}
		})
	}
}
