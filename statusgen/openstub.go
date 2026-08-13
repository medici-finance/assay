package main

// openstub.go — statusgen's off-board deploy-drift and product-config hooks.
//
// An off-board deploy-drift feature and its product-config namespace are not part
// of the open-core statusgen; these no-op stand-ins are the only implementations.
// main.go's off-board call site and rosterconfig.go's product-config hooks resolve
// against them, so statusgen reports no deploy-drift problems and recognises no
// product configuration.

// darSyncCheck is the off-board deploy-drift check: a no-op here, so it reports
// no problems and no notices. main.go's call site (offBoardProblems) is unchanged.
func darSyncCheck(root string, changed []string) (problems, notices []string) {
	return nil, nil
}

// scanProductConfigKeys / scanApplyProductConfig / scanProductConfigLines are the
// no-op product-config hooks: the roster reader recognises no product (non-ASSAY_)
// keys, applies none, and echoes none.
func scanProductConfigKeys() []string                                { return nil }
func scanApplyProductConfig(cfg *scanConfig, vals map[string]string) {}
func scanProductConfigLines(c scanConfig) []string                   { return nil }
