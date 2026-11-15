package store

import (
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/workflow/internal/refreshcodec"
)

func refreshBytes(v any) ([]byte, error) {
	switch v := v.(type) {
	case RefreshConfig:
		return refreshcodec.Profile([6]string{v.SourceID, v.AdmissionID, v.AccessEpoch, v.ConfigEpoch, v.PolicyClass, v.PolicyEpoch}, v.Limits, refreshcodec.Native(v.Native), v.PollInterval)
	case NativeLimits:
		return refreshcodec.Budget(refreshcodec.Native(v))
	case refresh.FillKey:
		return refreshcodec.Key(v)
	default:
		return nil, refresh.ErrPolicy
	}
}
func refreshDecode(raw []byte, v any) error {
	key, ok := v.(*refresh.FillKey)
	if !ok {
		return refresh.ErrHeld
	}
	return refreshcodec.DecodeKey(raw, key)
}
