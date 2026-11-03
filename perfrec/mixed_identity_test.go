package perfrec

import "testing"

// regression: #4360
func TestMixedSchemaIdentities(t *testing.T) {
	t.Setenv(EnvVar, "1")
	r := v3Record(v3Context("workflow"))
	r.Install, _ = NewID()
	v4, e := ToV4(r)
	if e != nil {
		t.Fatal(e)
	}
	kept, n := ExcludeMixedSchemaIdentities([]Record{r, v4, r, v4})
	if len(kept) != 0 || n != 1 {
		t.Fatal("cross-schema measurement counted more than once", n)
	}
	legacy := New("tool", "run", "", 0, 0, "", SystemProbe().now())
	kept, n = ExcludeMixedSchemaIdentities([]Record{legacy, r, r})
	if len(kept) != 3 || n != 0 {
		t.Fatal("single-version selection changed")
	}
	other := v4
	other.V4 = &V4Data{Profile: v4.V4.Profile, Kind: v4.V4.Kind, InvocationID: v4.V4.InvocationID}
	other.V4.InvocationID, _ = NewID()
	kept, n = ExcludeMixedSchemaIdentities([]Record{r, v4, other, legacy})
	if len(kept) != 2 || n != 1 || kept[0].V4.InvocationID != other.V4.InvocationID {
		t.Fatal("unrelated identities removed")
	}
}
