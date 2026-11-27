### Fixed

- `deskdispatch` run from a `verify-desk` window now acquires, advances and releases a verifier dispatch's claim as the verifier App, and its NOT LAUNCHED, stale-claim and release commands name that same role with no `--cross-role` desk mint. The model stamp stays on the desk App, and dispatches from every other loop are unchanged.
