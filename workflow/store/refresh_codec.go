package store

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
)

// These are private storage encodings, not wire protocols. Opaque identity
// strings become byte arrays before JSON, preserving every byte. Fixed structs
// and ordered arrays give process-independent canonical bytes; gob type IDs do
// not provide a canonical index/profile encoding.
type refreshProfileWire struct {
	Identity     [][]byte
	Limits       refresh.Limits
	Native       NativeLimits
	PollInterval time.Duration
}

func refreshBytes(v any) ([]byte, error) {
	var payload any
	var kind byte
	switch v := v.(type) {
	case RefreshConfig:
		kind = 'P'
		payload = refreshProfileWire{Identity: [][]byte{[]byte(v.SourceID), []byte(v.AdmissionID), []byte(v.AccessEpoch), []byte(v.ConfigEpoch), []byte(v.PolicyClass), []byte(v.PolicyEpoch)}, Limits: v.Limits, Native: v.Native, PollInterval: v.PollInterval}
	case NativeLimits:
		kind = 'B'
		payload = v
	case refresh.FillKey:
		kind = 'K'
		s := v.Scope
		payload = [][]byte{[]byte(s.Repository.Provider), []byte(s.Repository.Host), []byte(s.Repository.Path), []byte(s.Kind), []byte(s.AdmissionID), []byte(v.SourceID), []byte(v.AccessEpoch), []byte(v.ConfigEpoch), []byte(v.PolicyClass), []byte(v.PolicyEpoch)}
	default:
		return nil, refresh.ErrPolicy
	}
	raw, e := json.Marshal(payload)
	if e != nil || len(raw) > 64<<10 {
		return nil, refresh.ErrPolicy
	}
	return append([]byte{1, kind}, raw...), nil
}
func refreshDecode(raw []byte, v any) error {
	key, ok := v.(*refresh.FillKey)
	if !ok || len(raw) < 2 || len(raw) > 64<<10 || raw[0] != 1 || raw[1] != 'K' {
		return refresh.ErrHeld
	}
	var fields [][]byte
	d := json.NewDecoder(bytes.NewReader(raw[2:]))
	if d.Decode(&fields) != nil || len(fields) != 10 {
		return refresh.ErrHeld
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return refresh.ErrHeld
	}
	*key = refresh.FillKey{Scope: mirror.Scope{Repository: mirror.RepositoryID{Provider: mirror.Provider(fields[0]), Host: string(fields[1]), Path: string(fields[2])}, Kind: mirror.RecordKind(fields[3]), AdmissionID: string(fields[4])}, SourceID: string(fields[5]), AccessEpoch: string(fields[6]), ConfigEpoch: string(fields[7]), PolicyClass: string(fields[8]), PolicyEpoch: string(fields[9])}
	canonical, e := refreshBytes(*key)
	if e != nil || !bytes.Equal(canonical, raw) {
		return refresh.ErrHeld
	}
	return nil
}
