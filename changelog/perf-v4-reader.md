# Extensible local performance reader

- Add the bounded perf-v4 core and four typed optional facets to the canonical
shared recorder and deskperf reader. Historical fixtures, units and report groups
remain intact; unsupported facets retain useful core timing with explicit gaps.
Duplicate keys, malformed known payloads, role/linkage confusion and oversized
records refuse. Selected producers remain v3 until the separate reader-dependent
cutover. No collection, consent, identity, permission or release change.

- Count raw padding and CRLF bytes before v4 validation; preserve historical
  decoding and restore logical service query/transport/handler text counters.
- Continue after oversized history lines without growing the read bound, and expose v4 duplicate/conflict/invalid counters in the command overview.
- Preserve the historical final-line limit without LF while leaving strict perf-v4 wire validation unchanged.
