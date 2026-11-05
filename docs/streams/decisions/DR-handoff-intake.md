---
id: DR-handoff-intake
date: "2026-10-25"
title: "Retain outstanding intake work and require an accountable disposition before repair scheduling"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4307#issuecomment-6399188555"
alternatives:
  - "Keep delta-only collection — rejected because unanswered work disappears after polling advances or a session restarts."
  - "Infer assignment from a label, comment or mechanical scan PR — rejected because an observation does not establish an accountable successor."
  - "Add a central lifecycle database — rejected because forge, register, claim and acceptance owners already hold that authority."
accepted:
  - "Intake retains source-event identity and outstanding obligations using durable observations and existing forge/register owners; issue closure alone does not acknowledge a handoff."
  - "Both intake-exit-v1 writers remain byte-compatible; separately negotiated companion observations use deskcore/issueflow assessment in-process."
  - "Scoping requires a linked authoring task and strong author; external work requires an owner, action and wake condition; delivered code routes to qualification."
  - "Scanner scheduling and issueboard share disposition meaning, with existing trust, history, claim, human and independent acceptance checks retained."
  - "Legacy and unreadable records remain classification-needed; reader support is established before a required companion writer is enabled."
---

This record adopts the bounded design in merged PR #4310 and the Task of
[verification-handoff/03](../verification-handoff/brief-03-require-an-accountable-intake-disposition-before-repair-scheduling.md).

The driver approved `verification-handoff/03` and `DR-handoff-intake` as described in PR #4310: https://github.com/medici-finance/assay/issues/4307#issuecomment-6399188555

The shared /01 library supplies advisory assessment, not assignment or acceptance.
The intake adapters must implement the production paths and regression coverage
in the brief. This decision records design approval; it does not claim that those
paths have shipped, that the Verify rows pass, or that a consumer is activated.
