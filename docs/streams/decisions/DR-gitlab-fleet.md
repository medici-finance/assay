---
id: DR-gitlab-fleet
date: "2026-10-22"
title: "Interim separate GitLab provisioner ownership and parity"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Consolidate into deskfleet through a separately reviewed compatibility-preserving migration, then retire deskgitlab and reroute the shell adapter."
  - "The original retire-before-port option predates the merged implementation; it cannot silently delete or retire the landed command."
accepted:
  - "Retain the landed separate deskgitlab provisioner provisionally, with explicit desk-tools maintenance ownership and cross-command parity obligations."
  - "Keep ownership direction separate from the brief 30 compatibility-report design approval, independent verification and final acceptance."
---

## State and provenance

**Prepared approval record; awaiting own-human ratification. This is not an approved design record.**

The driver answered in the 2026-10-22 audit conversation, verbatim:

> retain for now with explicit ownership and parity obligations. prepare the approval record, then walk me through the next decision.

This text is a relay of that answer, not an own-human GitHub ruling. The driver supplied no further rationale; none is attributed here. The obligations below make the requested ownership/parity condition concrete for review.

The ownership decision remains on https://github.com/medici-finance/assay/issues/2987

The landed implementation is https://github.com/medici-finance/assay/pull/3155

## Bounded direction

Retain `tools/desk/cmd/deskgitlab` as an interim supported provisioner and preserve the shell create adapter. This corresponds to the existing issue's option 3 ownership direction, qualified by the provisional maintenance contract below. It does not retrospectively authorize the earlier dispatch or merge while the decision was unruled.

The desk-tools component maintainer owns the command, adapter and parity contract together. The worker desk routes defects and future edits to that owner; the review desk checks the cross-command contract; independent verification remains with the verify desk. A command-local implementer cannot declare cross-command parity by inspecting only its own handler. These are role obligations, not an assignment to an unnamed individual.

## Maintenance and parity obligations

1. Keep the provisioner and renewal role tables aligned with their canonical shared definitions. Any edit to a shared role, membership, scope or renewal table must enumerate both `deskgitlab` and `deskfleet` consumers, update parity controls and show both consumers' results.
2. Preserve the shell create adapter's declared flags, help, output and exit ordering, host defaults, paid-tier/avatar settings, generated-token modes and safe output-directory behavior. Enumerate adopter docs and install/adopt skill references whenever that contract changes.
3. Preserve environment-only owner-token input for this retained command, no token material in argv/help/errors, generated-token and writer-identity file permissions of 0600, and the existing no-redirect REST boundary. This direction does not authorize widening `deskfleet` custody or moving secrets between custody models.
4. A parity failure blocks accepting the affected contract change. Fix the mismatch or obtain a specifically scoped human contract ruling; never silently weaken a parity assertion, omit a consumer or treat one command's success as evidence for both.
5. At the next proposed consolidation, renewal-owner change, script retirement or token-custody change, reopen ownership explicitly with a consumer migration and compatibility report. No date-based automatic retirement, new provisioning authority, release or live operation follows from this interim choice.

## Approval boundaries and follow-up

The own-human ownership ratification below records this interim direction only. The original brief 30 gate separately requires review and approval of the exact compatibility report, including token scope, custody-file permissions, idempotency and secret-free surfaces. The recorded review follow-ups concerning out-dir mode-mask coverage and writer-identity 0600 coverage remain work to inspect; this record does not declare those tests present or passed.

Do not add a `ruling:` link, claim this record approved, promote a lifecycle cell or approve brief 54 solely from the chat relay. Before this record can qualify as a design approval, establish the exact record/brief binding markers on the design decision issue and obtain a separate own-human named-record approval of that bounded report. Independent verification and final human acceptance remain separate.

## Exact ownership ratification to post on issue 2987

`3 — retain deskgitlab for now with the explicit ownership and parity obligations in DR-gitlab-fleet. This ratifies the interim ownership direction only; brief 30 compatibility-report approval, independent verification and final acceptance remain separate.`

After ratification, link the exact unedited own-human comment as ownership provenance, then prepare the separate compatibility approval packet. Worker desk owns that follow-up under claim key `medici-finance/assay--desktools-v2--30`; no new implementation dispatch is authorized by this relay.
