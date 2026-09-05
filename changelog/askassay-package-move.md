### Changed
- The Ask-Assay numbers-rule layer moved out of `tools/desk/internal/askassay` to `tools/desk/askassay` (carrying `chart/` and `report/`) so another module can import it. Pure relocation plus import-path updates — no behaviour change; the single exec-site guard's ledger key was repointed to the moved file so the control stays green.
