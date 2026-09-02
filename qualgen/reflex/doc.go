// Package reflex implements quality's M4 methodology-reflexivity layer (spec
// §7, brief-12): gate-yield accounting (§7.1) and the ritual-effectiveness
// natural-experiment joins (§7.2). M1-M3 measure the CODE; M4 turns the same
// instruments on the PROCESS and harness, so a review lane or an authoring
// ritual is judged on outcomes rather than on faith.
//
// Everything here is a JOIN of already-recorded M1/M2/M3 outputs — no new
// mining (spec §7 preamble). This package holds no git-history access, no
// filesystem I/O, and no network calls; every input is a caller-supplied
// struct translated from an already-mined artifact (metrics.jsonl,
// defects.jsonl, the attribution ledger), exactly the same "join input, not
// a shared mining type" discipline qualgen/dorajoin and qualgen/m4 already
// establish. TestNoNewMining (gateyield_test.go) enforces this structurally
// by parsing the package's own imports.
//
// This package DOES import qualgen/attribution directly (unlike dorajoin,
// which mirrors package-main types because it cannot import package main):
// attribution is a real subpackage, so reflex reads its frozen ledger and
// review-escape overlay through a genuine Go import, never a re-derivation
// (brief-12 Context: "CONSUMES qualgen/attribution/... per-stage ledger +
// review-escape overlay").
//
// # Files
//
//   - gateyield.go — §7.1 gate-yield accounting: per review lane, pre-merge
//     catches vs M3-attributed escapes, as catch-rate / escape-rate /
//     latency cost.
//   - ritual.go — §7.2 the natural-experiment joins: cost per durable KLOC
//     by model tier × brittleness band, Verify-depth vs escape rate, and the
//     industry-named agent-metrics family (agent-PR survival rate,
//     first-pass approval rate, review-discipline guardrails).
//   - stratify.go — the observational-validity guard: brittleness-band
//     stratification as the minimum control, a mandatory confounders block,
//     and the single emit gate every ritual-effectiveness readout must pass
//     through before it can be serialized (brief-12's single point of
//     failure, by design the ONE choke point both gateyield and ritual
//     route through).
//
// # Honest-claims discipline (spec §10, brief-12 ground rules)
//
// Ritual effectiveness is a NATURAL EXPERIMENT, not a controlled one: harder
// code draws both a stronger execution tier and more churn, so a bare
// tier-vs-outcome or Verify-depth-vs-outcome number is never presented as
// causal. stratify.EmitRitual is the only path that serializes a
// ritual-effectiveness readout, and it refuses (error, not warning) one that
// is not brittleness-band stratified or that carries no confounders block.
package reflex
