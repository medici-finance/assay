// Package comms is the peer-authentication backbone for inter-desk cell
// messaging: the wire envelope every message is carried in, the short-TTL signed
// identity assertion that authenticates its sender, and the lane ACL that decides
// whether a (from, verb, to) tuple is permitted at all.
//
// WHY IT EXISTS. Once cells run on different boxes with different tools, the
// sender of a message is no longer a trusted local process. Authentication is the
// backbone the whole guardrail stack stands on, and an ACL that lives in prose
// drifts and cannot refuse anything. This package makes both real, in three
// files:
//
//   - envelope.go — cellmsg-v1 parse-or-refuse. A wire payload that does not
//     parse as the one well-known shape is refused BEFORE any field is inspected;
//     size caps, an absent {from, to, verb} triple, an unknown schema/verb/class
//     are each a typed refusal, never a best-effort read.
//   - identity.go — assertion mint/verify. A message carries a signed assertion
//     binding {cell, role} to that specific message id, with a short TTL, a
//     bounded clock skew, and a single-use nonce. Verify-or-refuse runs before
//     every other check; an invalid, expired, replayed, or unknown-cell assertion
//     each refuses with a DISTINCT typed error.
//   - laneacl.go + laneacl.yaml — the (fromCell, fromRole, verb, toCell, toRole)
//     allow-matrix, compiled from one declared source and diffed against it by a
//     test. Absent is refused (least authority); a row or verb naming a human-gate
//     action is refused at LOAD; cross-cell reach is the-desk to the-desk only and
//     its verb allow-set ships EMPTY, a named open decision.
//
// DESIGN IS FIXED BY THE SPEC, NOT REDESIGNED HERE. The envelope fields, the
// two-layer identity model (gateway mTLS is the transport layer; this package
// owns the envelope-level identity) and the reach rules are settled by the
// cell-comms design and are implemented, not re-litigated, here.
//
// NO KEYS IN THE TREE. This package models the mint/verify MECHANISM and a trust
// store SHAPE only. Signing keys are per-role operator config under the App-PEM
// custody rules — mode 0600, never committed, never in the source tree. There is
// no field here that holds a private key path or a key, and adding one would be a
// schema change a reviewer must refuse.
package comms
