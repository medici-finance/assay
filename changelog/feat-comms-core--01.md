### Added
- `comms/` is a new stdlib-only Go module (`github.com/medici-finance/assay/comms`) holding the public cellmsg-v1 contract: the envelope, structural parser, typed address/message/receipt/error mapping and the envelope identity primitives. It imports nothing from desk-tools, and a dependency-boundary test walks its whole closure.

### Changed
- `tools/desk/internal/comms` is now the Assay policy adapter over that contract: the verb and class vocabulary, compiled lane ACL and a new `Admit` entrypoint stay there, and the existing names are thin aliases of the contract's, so `commsgw`, `commsloop`, `deskcomms` and `commsqueue` accept and refuse exactly what they did.
