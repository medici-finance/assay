// Package evaluation is the canonical eligibility and evidence-coverage
// evaluator (contract S-eligibility and S-evidence-coverage).
//
// Two typed functions answer the two questions a dispatcher and a flip gate
// ask of a board:
//
//   - EvaluateEligibility: may this brief be dispatched, given its depends:,
//     gates: and feathers: edges? (eligibility.go)
//   - EvaluateCoverage: does every mandatory claim of this brief have a
//     passing result at the item's revision? (coverage*.go)
//
// Both are pure functions of the observations they are handed. The package
// opens no file, runs no process, reads no environment variable, clock, token
// or network, and keeps no global state; a repository is reached only through
// the Repo port, which the caller implements (statusgen's own adapter shells
// to git; a frozen-observation implementation needs nothing at all). A value
// the caller cannot supply is reported as could-not-check, never rounded to
// satisfied or pass. The verdicts grant nothing: eligibility is not admission
// and a released coverage verdict is not publication.
//
// The statusgen command, the eligibility and coverage CLIs, Next-up, the drive
// frontier and the auto-flip lane all read these functions; there is no second
// implementation. Identity reports the evaluator version and a digest of the
// policy constants the verdicts depend on, so a consumer can record which
// evaluator produced a snapshot.
package evaluation
