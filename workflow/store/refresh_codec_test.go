package store

import (
	"bytes"
	"testing"

	"github.com/medici-finance/assay/forgeread/refresh"
)

// regression: F-refresh-canonical-byte-identity
func TestPGRefreshIdentityCodec(t *testing.T) {
	c := testRefreshConfig()
	c.AccessEpoch = "opaque\x00\xff"
	one, e := refreshBytes(c)
	if e != nil {
		t.Fatal(e)
	}
	two, e := refreshBytes(c)
	if e != nil || !bytes.Equal(one, two) {
		t.Fatal("profile bytes vary")
	}
	_, u := mirrorInput()
	k := refresh.FillKey{Scope: u.Scope, SourceID: c.SourceID, AccessEpoch: c.AccessEpoch, ConfigEpoch: c.ConfigEpoch, PolicyClass: c.PolicyClass, PolicyEpoch: c.PolicyEpoch}
	raw, e := refreshBytes(k)
	if e != nil {
		t.Fatal(e)
	}
	var decoded refresh.FillKey
	if refreshDecode(raw, &decoded) != nil || decoded != k {
		t.Fatal("opaque identity lost")
	}
	for _, bad := range [][]byte{append(append([]byte{}, raw...), 0), append([]byte{2}, raw[1:]...), append([]byte{1, 'P'}, raw[2:]...)} {
		if refreshDecode(bad, &decoded) == nil {
			t.Fatal("noncanonical identity accepted")
		}
	}
}
