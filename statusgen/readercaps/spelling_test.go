package readercaps_test

import (
	"github.com/medici-finance/assay/statusgen/readercaps"
	"testing"
)

// regression: #4947
func TestReaderCapsSpelling(t *testing.T) {
	for name, bad := range map[string]string{
		"top-case":            `{"schema":"reader-requirements-v1","required":[{"operation":"dispatch","schemas":[],"semantics":["future-v1"]}],"REQUIRED":[]}`,
		"top-reversed":        `{"schema":"reader-requirements-v1","REQUIRED":[],"required":[{"operation":"dispatch","schemas":[],"semantics":["future-v1"]}]}`,
		"nested-case":         `{"schema":"reader-requirements-v1","required":[{"operation":"syntax","schemas":["milestone-v1"],"Schemas":[],"semantics":[]}]}`,
		"nested-folded":       `{"schema":"reader-requirements-v1","required":[{"operation":"syntax","schemas":["milestone-v1"],"ſchemas":[],"semantics":[]}]}`,
		"single-noncanonical": `{"schema":"reader-requirements-v1","REQUIRED":[]}`,
		"escaped-duplicate":   `{"schema":"reader-requirements-v1","required":[],"\u0072equired":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readercaps.DecodeRequirements([]byte(bad)); err == nil {
				t.Fatal("accepted a noncanonical or duplicate requirement field")
			}
		})
	}
	for _, bad := range []string{
		`{"schema":"reader-capabilities-v1","provider":"reader","version":"dev","operations":[{"operation":"syntax","schemas":[],"semantics":[]}],"OPERATIONS":[]}`,
		`{"schema":"reader-capabilities-v1","provider":"reader","version":"dev","operations":[{"operation":"syntax","schemas":[],"semantics":[],"SEMANTICS":[]}]}`,
	} {
		if _, err := readercaps.DecodeAdvertisement([]byte(bad)); err == nil {
			t.Error("accepted alternate advertisement field spelling")
		}
	}
	// JSON escapes which decode to the exact field name are canonical, but do
	// not create an exemption from the required capability.
	r, err := readercaps.DecodeRequirements([]byte(`{"schema":"reader-requirements-v1","\u0072equired":[{"operation":"dispatch","schemas":[],"semantics":["future-v1"]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	peer := readercaps.Advertisement{Schema: readercaps.AdvertisementSchema, Provider: "reader", Version: "dev", Operations: []readercaps.Operation{}}
	if got := readercaps.Assess(r, readercaps.Observation{State: readercaps.Current, Advertisement: peer}); got.State != readercaps.Unsupported {
		t.Fatal(got)
	}
}
