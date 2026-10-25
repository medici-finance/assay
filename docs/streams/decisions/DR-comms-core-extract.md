---
id: DR-comms-core-extract
date: "2026-10-25"
title: "Approve the bounded extraction of the cellmsg-v1 comms contract, with Assay policy kept internal"
consequence: major
decided-by: "human:ian"
alternatives:
  - "Narrow or revise: remove incompatible changes and resubmit the compatibility and authority report. This was the second option the decision issue offered. The driver chose option 1 instead, on a report that lists no changed credential source, accepted message, refusal or persisted record."
  - "Leave the contract inside tools/desk/internal/comms. This is the status quo the brief exists to retire: Go's internal-package boundary stops any external client or host from importing the envelope, so every new consumer would need its own copy (brief comms-core/01 `why:`)."
accepted:
  - "The public `comms` module (`github.com/medici-finance/assay/comms`, package `comms/contract`) holds only structural types, the structural parser and key-free envelope identity primitives. Verb and class vocabulary, the compiled lane ACL and reserved-verb refusal stay in the Assay adapter under `tools/desk/internal/comms`. Service admission stays authoritative. A client-side pre-check is advisory."
  - "The public package's dependency closure excludes deskkit, token minting, forge access, role execution and process launch. `TestContractDependencyBoundary` enforces that exclusion. If the boundary is wrong, only that test catches it before review."
  - "One implementation with thin aliases: the existing names in `tools/desk/internal/comms` delegate to the contract, so the consumers compile unchanged. The aliases stay until those callers move."
  - "Two capabilities are declared but not implemented: `delivery-token/v1` and `client-identity/v1`. cellmsg-v1 does not guarantee either one."
  - "Open gaps go to later work and are not closed here. The commsqueue quarantine filer's deskkit coupling moves in comms-core/02. The desk-tools `comms` requirement is a placeholder pseudo-version, resolved through a local replace, until the module is released. CI path filters and a CI job for the `comms` module need a maintainer who holds the workflows scope."
---

**Ruling recorded (2026-10-25): option 1, approve the bounded extraction.** The driver
ruled on the decision issue for brief comms-core/01,
[#4397](https://github.com/medici-finance/assay/issues/4397), from the driver's own login. The
ruling comment
([2026-10-25T15:12:53Z](https://github.com/medici-finance/assay/issues/4397#issuecomment-6399352645))
reads: `comms-core/01: 1 — approve the bounded extraction per the compatibility and authority
report in medici-finance/assay#3061.` The record's `date:` is that comment's posting date (UTC).

This record copies that ruling into the register for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4). It makes no new decision. The human act is the driver's comment
on #4397.

**Why there is no `ruling:` link.** The ruling comment names the brief by its board id. It
does not name this record. The register's corroboration check requires the comment's own text
to name the record's `DR-<slug>` id (`spec/registers-v1.md` §7.5, condition 5), so a link to
that comment would fail as `record-not-named`. The real name in `decided-by` is therefore
corroborated the other way §7.5 allows: through the driver's approval of the pull request that
adds this record. The driver's merge of that pull request is what approves this record. If the
driver later posts a comment on #4397 that names `DR-comms-core-extract`, a follow-up can add
that comment as the `ruling:` link.

**The decision.** The cellmsg-v1 message contract is extracted into a standalone public Go
module, and Assay's own message policy stays internal to desk-tools. That is the bounded
extraction the issue's option 1 describes, approved "with the existing authority and
disclosure controls preserved".

**What the approval rests on.** The driver ruled on the compatibility and authority report in
the description of [#3061](https://github.com/medici-finance/assay/pull/3061) (merged
2026-10-15), under its heading "Compatibility and authority report". This record cites that
report and does not restate it. The report covers changed credential sources, accepted
messages, refusals, persisted records, the public API of `comms/contract`, and what stays out
of the public dependency closure. The brief's own `## Human decision` asked for exactly those
items.

**The tree's present condition (an observation, not a ruling).** The extraction this record
approves is already on main: #3061 delivered it before the ruling, and its description left the
brief's board row at `todo` because a risk-gated brief cannot reach `implemented` without a
cited design record. This record supplies that citation. It is not evidence that the work is
correct. That is the job of the brief's Verify table, run by an independent verifier on merged
main.

**The consequence level.** The ruling comment states no level. `consequence: major` is a desk
default, declared in the pull request that adds this record and open to change by the
maintainer. If the split between contract and policy is wrong, a public consumer could accept
messages that Assay policy refuses, or a client could gain authority that belongs to the
server. Either needs rework, but nothing is irreversible: no release, activation or broker
switch rides on this record.

**What this record does not decide.** It covers comms-core/01 only. It does not decide the
server extraction (comms-core/02), the client SDK (comms-core/03) or configuration
(comms-core/04). It does not choose a broker, select a new trust model, or approve native
session activation or a supported release, all of which the stream README sets aside for
later briefs. As the register itself states, it records that the alternatives were weighed and
names a human approver. Whether the extraction is correct is for the review gate and the
brief's Verify table to judge.
