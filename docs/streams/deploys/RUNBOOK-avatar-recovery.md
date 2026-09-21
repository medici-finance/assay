---
id: RUNBOOK-avatar-recovery
kind: runbook
date: "2026-09-18"
title: "Recover the avatar-generation pipeline after a bad deploy"
trigger: "the deskavatar generator produces illegible or wrong avatars in prod after a deploy"
cadence: "quarterly"
---

Worked example for [`../../deploy-model.md`](../../deploy-model.md) § "Runbooks" — a real
record shape, deliberately left **undrilled** (every Evidence cell below is empty) because
no one has actually rehearsed this recovery yet. This is the honest state a freshly-authored
runbook starts in, and it is also the committed proof of the negative path
`deploy-model.md` § "Runbooks" specifies: `statusgen --lint` reports this record
`could-not-check`, never a silent pass and never a hard failure. Running the first drill and
filling these cells is future work, not something to fabricate here.

**Preconditions:** the prior desk-tools release tag is still resolvable
(`docs/distribution.md`'s 14-day cache window has not lapsed) and the operator holds the
`prod` deploy authority named on [`DEPLOY-avatar-deploy-prod.md`](DEPLOY-avatar-deploy-prod.md).

**Steps:**
1. Re-point the prod pin to the prior desk-tools tag (the `rollback:` path on the DEPLOY
   record).
2. Regenerate one avatar for a known adopter slug and confirm it renders legibly.
3. Confirm the prod pin file records the reverted tag.

**Verification that the recovery worked:** the regenerated avatar in step 2 passes the same
20 px legibility proof `apps-installer/05` established, and the pin file in step 3 no longer
names the bad tag.

## Drill rows

| # | Action | Expect | Evidence |
|---|--------|--------|----------|
| 1 | Re-point the prod pin to the prior tag | pin file records the prior tag | |
| 2 | Regenerate one avatar for a known adopter slug | avatar renders legibly at 20px | |
| 3 | Confirm the pin file no longer names the bad tag | grep for the bad tag in the pin file returns 0 matches | |
