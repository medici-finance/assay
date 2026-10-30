package record

import (
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
)

// regression: #4531
func TestRecordRequirements(t *testing.T) {
	for _, raw := range []string{"# legacy\nreader-requirements is prose", "---\nstream: legacy\n---\nreader-requirements in body only"} {
		r, err := Requirements([]byte(raw))
		if err != nil || len(r.Required) != 0 {
			t.Fatalf("legacy changed: %+v %v", r, err)
		}
	}
	raw := "---\nstream: sample\nreader-requirements:\n  schema: reader-requirements-v1\n  required:\n    - operation: dispatch\n      schemas: []\n      semantics: [future-v9]\n---\n"
	r, err := Requirements([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	peer := readercaps.Advertisement{Schema: readercaps.AdvertisementSchema, Provider: "old", Version: "v99", Operations: []readercaps.Operation{}}
	if got := readercaps.Assess(r, readercaps.Observation{State: readercaps.Current, Advertisement: peer}); got.State != readercaps.Unsupported {
		t.Fatal(got)
	}
	for _, bad := range []string{
		"---\nreader-requirements: null\n---\n",
		"---\nreader-requirements: {schema: reader-requirements-v1, required: [], unknown: []}\n---\n",
		"---\nreader-requirements: {}\nreader-requirements: {}\n---\n",
		"---\nreader-requirements: {}\n...\n{}\n---\n",
	} {
		if _, err := Requirements([]byte(bad)); err == nil {
			t.Errorf("accepted malformed opt-in: %q", bad)
		}
	}
}
