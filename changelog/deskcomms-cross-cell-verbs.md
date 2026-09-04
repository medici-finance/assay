### Added
- **deskcomms cross-cell verb allow-set** — the coordinator-to-coordinator
  (the-desk ↔ the-desk) lane now carries the four ruled read-only/advisory verbs
  `status`, `metrics`, `help-offered`, `focus-on` (previously the cross-cell verb
  set shipped empty / fail-closed). `deskcomms send` gains an identity-independent
  cross-cell-verb preflight gate that reads the compiled ACL (never a second copy)
  and refuses any other cross-cell verb fail-fast with a distinct refusal, before
  identity or parse; the lane ACL's `Allow` stays the authoritative reach + verb
  check. `--verb focus-on` is documented as advisory — the receiving desk may
  decline it. None of the four mutates state on the receiving cell.
