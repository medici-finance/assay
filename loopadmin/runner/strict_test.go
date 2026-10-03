package runner

import "testing"

// TestScanStrictTrailing exercises the token scan on its own. DecodeRequest's
// later key check also refuses trailing data, so this is the test that proves
// the scan is a layer of its own and not carried by the one behind it.
func TestScanStrictTrailing(t *testing.T) {
	for name, doc := range map[string]string{
		"trailing brace": `{"a":1}}`,
		"trailing value": `{"a":1} {}`,
		"trailing word":  `{"a":1} x`,
		"duplicate key":  `{"a":1,"a":2}`,
	} {
		if err := scanStrict([]byte(doc)); err == nil {
			t.Errorf("%s: the scan accepted it", name)
		}
	}
	// Positive control: one well-formed value with surrounding space passes.
	if err := scanStrict([]byte(" {\"a\":[1,{\"b\":null}]}\n")); err != nil {
		t.Fatalf("one value refused: %v", err)
	}
}
