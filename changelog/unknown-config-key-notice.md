### Changed
- The desk tools no longer refuse the whole roster over an unrecognised `ASSAY_*` key (a typo, or a key from a newer release): they print an `assay-config: NOTICE —` line naming the key, leave whatever it meant to configure at its unset behaviour, and keep the rest of the roster. Known-key value errors and the trust-surface refusals are unchanged; `statusgen` still refuses such a key.
