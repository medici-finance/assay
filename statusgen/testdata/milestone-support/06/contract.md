---
schema: milestone-v1
milestone: example:widgets:build:m-01
contract-version: 1
title: Joined build capability is supported
owner: example-team
sources: [src.go]
contributes: ["example:widgets:build:b-07@1", "example:widgets:build:b-08@1"]
requires-milestones: ["example:widgets:build:m-02@1"]
---
## Outcome
A joined, independently verified build capability.
## Supported scope
The exact environment, configuration, interface and profile in the manifest.
## Acceptance
| ID | Criterion |
|---|---|
| C01 | First contribution has independent evidence. |
| C02 | Second contribution has independent evidence. |
## Coverage
| Criterion | Contributors | Evidence |
|---|---|---|
| C01 | example:widgets:build:b-07@1 | evidence/receipt.json |
| C02 | example:widgets:build:b-08@1 | evidence/receipt.json |
## Prerequisites
The exact prerequisite receipt in the manifest, with a compatible profile.
## Acceptance authority
The existing policy at evidence/authority.json, selected at this contract source.
## Change and open decisions
Fixture only. No operational authority or milestone acceptance is recorded here.
