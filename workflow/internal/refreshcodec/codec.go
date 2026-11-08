// Package refreshcodec owns only the private process-independent storage encoding.
package refreshcodec

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
type profileWire struct {
	Identity     [][]byte
	Limits       refresh.Limits
	Native       Native
	PollInterval time.Duration
}

type Native struct {
	CallerPerWindow, AdmissionPerWindow, SourcePerWindow int64
	PerFill, Concurrent, Reservations                    int64
	Window                                               time.Duration
}

func encode(kind byte, v any) ([]byte, error) {
	raw, e := json.Marshal(v)
	if e != nil || len(raw) > 64<<10 {
		return nil, refresh.ErrPolicy
	}
	return append([]byte{1, kind}, raw...), nil
}
func Profile(identity [6]string, limits refresh.Limits, native Native, poll time.Duration) ([]byte, error) {
	fields := make([][]byte, len(identity))
	for i, v := range identity {
		fields[i] = []byte(v)
	}
	return encode('P', profileWire{Identity: fields, Limits: limits, Native: native, PollInterval: poll})
}
func Budget(native Native) ([]byte, error) { return encode('B', native) }

// Initialization is private immutable operator provenance. Counts use charged
// accounting units; it is neither a provider observation nor a wire request.
type Initialization struct {
	Kind                 string
	Identity             [][]byte
	Profile              []byte
	Known                bool
	WindowMicros         int64
	Fills, Attempts      int64
	Provenance           []byte
	RetryKnown, HasRetry bool
	RetryMicros          int64
}

func Initial(v Initialization) ([]byte, []byte, error) {
	if v.Kind != "source" && v.Kind != "admission" && v.Kind != "caller" {
		return nil, nil, refresh.ErrPolicy
	}
	identity, e := encode('I', v.Identity)
	if e != nil {
		return nil, nil, e
	}
	data, e := encode('O', v)
	return identity, data, e
}

func InitialSet(records [][]byte) ([]byte, error) { return encode('S', records) }

func Key(v refresh.FillKey) ([]byte, error) {
	s := v.Scope
	return encode('K', [][]byte{[]byte(s.Repository.Provider), []byte(s.Repository.Host), []byte(s.Repository.Path), []byte(s.Kind), []byte(s.AdmissionID), []byte(v.SourceID), []byte(v.AccessEpoch), []byte(v.ConfigEpoch), []byte(v.PolicyClass), []byte(v.PolicyEpoch)})
}
func DecodeKey(raw []byte, key *refresh.FillKey) error {
	if key == nil || len(raw) < 2 || len(raw) > 64<<10 || raw[0] != 1 || raw[1] != 'K' {
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
	canonical, e := Key(*key)
	if e != nil || !bytes.Equal(canonical, raw) {
		return refresh.ErrHeld
	}
	return nil
}
