package briefdoc

import "testing"

// regression: #4947
func TestParseMapping(t *testing.T) {
	legacy := "why: 'An example says customer: yes'\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n"
	if _, err := ParseMapping(legacy); err != nil {
		t.Fatal("structure applied risk policy:", err)
	}
	if _, err := ParseFields(legacy); err == nil {
		t.Fatal("existing risk policy was relaxed")
	}
	for _, bad := range []string{
		"stream: example\n...\nreader-requirements: {}\n",
		"stream: example\n--- {reader-requirements: {}}\n",
		"reader\u200b-requirements: {}\n",
		"reader-requirements\u00a0: {}\n",
		"stream: example\nstream: other\n",
	} {
		if _, err := ParseMapping(bad); err == nil {
			t.Errorf("accepted invalid structure: %q", bad)
		}
	}
}
