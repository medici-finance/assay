//go:build !houseprivate

package main

// openstub.go — the DEFAULT (public / open-core) build of statusgen.
//
// A house-private deploy-drift feature and its product-config namespace are
// compiled only under `-tags houseprivate` and are excluded from the public copy
// by house ruling. This file provides the no-op stand-ins the default build links
// against instead, so main.go's off-board call site and rosterconfig.go's
// product-config hooks resolve unchanged while the shipped tree carries neither
// the feature nor its product configuration. The house build supplies the real
// implementations behind the build tag.

// darSyncCheck is the public no-op: the off-board deploy-drift check does not
// exist in the open-core copy, so it reports no problems and no notices. main.go's
// call site (offBoardProblems) is unchanged.
func darSyncCheck(root string, changed []string) (problems, notices []string) {
	return nil, nil
}

// scanProductConfigKeys / scanApplyProductConfig / scanProductConfigLines are the
// no-op product-config hooks for the default build: the shipped roster reader
// recognises no product (non-ASSAY_) keys, applies none, and echoes none.
func scanProductConfigKeys() []string                                { return nil }
func scanApplyProductConfig(cfg *scanConfig, vals map[string]string) {}
func scanProductConfigLines(c scanConfig) []string                   { return nil }
