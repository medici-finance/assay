package main

import (
	"strings"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/readercaps/record"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func capabilityError(code int, reason string) error {
	detail := readercaps.Diagnostic(strings.TrimPrefix(strings.TrimSpace(reason), toolName+": "))
	if code == deskkit.ExitRefused {
		return deskkit.Refused(detail)
	}
	return deskkit.Unverifiable(detail, nil)
}

func capabilityResultError(result readercaps.Result) error {
	if result.State == readercaps.Supported {
		return nil
	}
	code := deskkit.ExitUnverifiable
	if result.State == readercaps.Unsupported {
		code = deskkit.ExitRefused
	}
	return capabilityError(code, "reader capabilities: "+result.Reason)
}

func requireRecordCapabilities(raw []byte, source string) error {
	result := record.Check(raw, deskkit.ReaderCapabilities(deskkit.EffectiveToolVersion(version), toolName))
	if result.State != readercaps.Supported {
		result.Reason = source + ": " + result.Reason
	}
	return capabilityResultError(result)
}
