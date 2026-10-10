### Fixed
- The mutation runners (`muhar`, `loopadmin/mutate`) no longer poison the ambient Go build cache: each mutation run now gets a throwaway `GOCACHE` (overriding any inherited one) that is removed when the run ends, including on interrupt and error exits. `GOMODCACHE` stays shared — module downloads are immutable and shared-safe.
