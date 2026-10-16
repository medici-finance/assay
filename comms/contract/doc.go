// Package contract is the transport-neutral cellmsg-v1 message contract: the
// versioned envelope, typed addresses, delivery, receipt and error types, the
// structural parser, and the envelope-level identity assertion primitives.
//
// WHAT IT OWNS. Structure only: the one well-known envelope shape, its size
// caps, its parse-or-refuse order, and the mint/verify mechanism for the
// short-TTL signed assertion an envelope carries. A payload that does not parse
// as cellmsg-v1 is refused before any field is inspected.
//
// WHAT IT DOES NOT OWN. Policy. The verb and handling-class vocabulary, the role
// list, the lane ACL and every human-gate refusal are supplied by the host
// through the Vocabulary port or applied after the parse; this package holds no
// Assay role name and no allow-matrix. Acceptance is never widened here: a
// structurally valid envelope is not an authorised one, and a parse success is
// not an authenticated message.
//
// DEPENDENCY CLOSURE. Standard library only, and not the network, process or
// environment packages: it reads no environment, makes no network call and
// launches no process. Signing keys are never held or read here; a caller
// supplies a Signer, and the trust store carries public keys only.
// TestContractDependencyBoundary walks the whole closure and fails on anything
// else, so a transitive route cannot slip in.
//
// COMPATIBILITY. The wire is cellmsg-v1 and is unchanged by this package's
// extraction. Delivery tokens and per-client identity are DECLARED future
// capabilities (see Capabilities) and are not guarantees of cellmsg-v1: the
// legacy trust store holds one public key per cell, and role names do not
// establish independent per-client cryptographic identity.
package contract
