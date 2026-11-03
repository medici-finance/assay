# Forge service contract

The typed protocol and separately imported client are offline implementation work under forge-neutral/39. Serving, credential custody, source refresh ownership and activation remain their existing owners. This document records no operational adoption.

## Architecture pickup

Continue against source main `214b4c8308b3cc57beed91feccf45e6a03f2eecd`, carrying the published pickup clarification from PR 5413 through genuine merge `3fd78f3b7288f087fb66b320bffb2b8381a59d7a`. Review observation transport maps the existing reviewflow definitions and review-observation-v1 schema; it does not compute classifier rows, candidate selection, eligibility, ready authority or policy approval. The predecessor's design approval is distinct from pending independent verification and this contract's own acceptance issue 5414.

Reuse the forgeread root and mirror identity/observation meanings. Existing operation keys are opaque values from their canonical effect owner; the transport neither mints a competing key nor implements reconciliation. Stdlib encoding/json handles protocol coding and net/http is confined to the separate client. No third-party dependency is added to the stdlib-only module.

The client requires an already-admitted transport and an explicitly supplied trusted endpoint and scope binding. It has no default transport, credential lookup or client-side provider fallback. Redirects, unknown required protocol values and scope widening are refused. Fixture HTTP handlers qualify the wire/client boundary only; they are not a production listener or proof of current provider entitlement.
