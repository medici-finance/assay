---
brief: assay:assay:graph-execution:11
title: Optional pinned Laya provider with explicit CPU and GPU profiles
why: A local decision model can keep sensitive context on the operator’s hardware. Its convenience downloads, device fallback and input truncation must be made visible and controlled before its answers are usable.
wave: 1
depends:
- graph-execution/10
unblocks:
- graph-execution/12
effort: M
gate: human
risk:
  regulatory: no
  customer: no
  irreversible: no
  sensitive-data: yes
gate-why: The owner confirms local data handling, artifact trust and permitted device fallback before the provider is enabled.
decision-trigger: spec
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'tools/desk/internal/deskkit: fixed-here'
- 'providers/laya: fixed-here'
- 'evaluation/decisions: follow-up graph-execution/12'
version: 1
id: 4e8b1c05-e163-4f79-95ba-044e5b5998f4
---

# Brief 11 — Optional pinned Laya provider with explicit CPU and GPU profiles

## Context

files: `providers/laya/pyproject.toml` (planned), `providers/laya/assay_laya/__init__.py` (planned), `providers/laya/assay_laya/adapter.py` (planned), `providers/laya/tests/test_adapter.py` (planned), `providers/laya/artifacts.example.json` (planned), `providers/laya/README.md` (planned), `tools/desk/internal/deskkit/layaadvisor.go` (planned), `tools/desk/internal/deskkit/layaadvisor_test.go` (planned), `changelog/graph-execution-11-laya-local-provider.md` (planned)

facts: Upstream source exposes CPU, CUDA and MPS selection with CPU fallback. The preference is CPU first; CUDA optional, MPS unqualified until tested. No checkpoint has been executed for this brief.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Human decision

Decision-trigger: spec. At implementation pickup, prepare concrete policy choices and negative-path evidence, then file a self-contained decision issue. No response permits no activation; schema/test work may proceed within the declared scope.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Task

1. Pin code commit, model revision, tokenizer/encoder assets and dependency lock; document code/weight/base licenses separately and verify artifact checksums. Provide an optional subprocess JSON protocol behind Advisor, bounded in bytes/time with diagnostics on stderr. No Python dependency for non-users.
2. Implement offline local-only loading, read-only artifact validation, explicit requested/actual backend reporting and policy-controlled fallback. Reject overlong state or options before the upstream tokenizer can silently truncate. Keep question/option ordering stable and capture all omissions if approved context selection is used.
3. Add dependency-light stub tests for GPU-unavailable/OOM fallback, tampered/missing asset, attempted network access, token overflow, malformed output and subprocess timeout. Real CPU/GPU smoke commands are separately opt-in and report unavailable hardware as could-not-check, never pass.
4. Document initial planning envelope: existing 8–16 GB RAM CPU machine for short-input trials; optional 4–8 GB CUDA VRAM, subject to measurement. No purchase, speed or calibration guarantee. Upstream action/confidence fields do not authorize effects.

## Interface contract

Go Advisor → optional local process → typed assessment records actual backend; no external fallback or effect occurs.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestLayaAdvisor" ./...` | exit 0; output includes PASS for TestLayaAdvisor, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestLayaAdvisorUnauthorizedFallback" ./...` | exit 0; output includes PASS for TestLayaAdvisorUnauthorizedFallback, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestLayaAdvisorRoundTrip" ./...` | exit 0; output includes PASS for TestLayaAdvisorRoundTrip, with no [no tests to run] for its owning package |
| 4 | check:ci +mutation | `python3 -m unittest discover -s providers/laya/tests -p test_adapter.py -v` | exit 0; network-attempt, input-overflow, artifact-tamper and device-fallback tests execute using stubs without model downloads |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
