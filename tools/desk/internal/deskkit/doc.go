// Package deskkit is the shared safety plumbing for the desk-tools suite.
// Every desk tool (deskboard, deskpost, deskpr, deskwt, deskreply) depends on
// this package for its kill switch, audit log, rate limit, idempotency store,
// secret scan, version stamp, and the canonical exit-code contract.
//
// Fail-closed is the desk rule: any state a helper cannot POSITIVELY verify
// is a refusal with a distinct exit code, never a silent default toward success.
package deskkit
