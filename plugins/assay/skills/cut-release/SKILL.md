---
name: cut-release
description: >-
  Cut a versioned release of a repo's shipped artifacts and carry it to every consumer pin.
  Use for "cut a release", "ship vX.Y.Z", "tag the release", "re-pin to the new release". Not
  for moving an adopter between versions (`upgrade-assay`).
---

# Cut a release, then carry it to the consumers

A release is not done when the tag exists. It is done when every consumer that pins it is green
on the new pin. This skill covers the chain from "main holds what we want to ship" to that point.
The repo's release workflow, guards and pin checkers own the mechanics; this skill owns the order
and the decisions. It names verbs, never flags: run `<verb> --help` before a verb's first use.

## STOP: the hard lines (each stated once)

- **Never move, delete or re-point a published tag.** Consumers pin by tag + sha256, so a moved
  tag is a silent substitution. A bad release is fixed FORWARD with the next number.
- **Never reuse a burned number**: one that was ever tagged or published, even if later deleted.
- **Never hand-type a hash.** Every pinned sha256 comes from the PUBLISHED release's
  `checksums.txt`. A local build lacks the release stamp and hashes differently.
- **Never merge, ready-flip or approve** a release or re-pin PR, and never dispatch or approve a
  gated publish yourself. Those are the driver's acts.
- **A guard refusal is a STOP**, not a puzzle. Report it; never route around it.

## 1. Preconditions: all four, re-read live

1. Everything that ships is merged to main.
2. Main is green at the exact commit that will be tagged, not "green recently".
3. The tag is free and the number was never burned. Check the remote's tags and its release
   list, not memory.
4. The tree is diffed against the previous release, and the changes are what the notes will
   claim. An empty diff is not a release.

Where the repo stamps a manifest version into its artifacts, the release run does the stamping,
and its guard refuses a changed artifact that carries no version bump. Never hand-stamp a version
to get past that guard.

## 2. Notes come from the fragments

Where the repo collects per-PR changelog fragments, the release run aggregates them into the
notes and refuses to cut with none. Never hand-write a parallel list. Read the pending fragments
before the cut: thin fragments make thin notes. Notes are descriptive highlights grouped
Added / Fixed / Changed, never a commit list.

## 3. Dry run, then hand off the publish

Dispatch the release workflow in its dry-run mode first. A dry run resolves the version, proves
the tag free, runs the tests and builds every artifact, then stops: no tag, no release, no upload.
A red dry run ends the cut here.

The real run is a gated publish. It runs in a protected environment that needs a human reviewer,
and the dispatching identity is recorded as the release's authorizer. So the dispatch and the
approval belong to the driver. Hand them over as exact commands with `human-runsheet`: the
workflow, the version, and the green dry run it rests on. Then wait. Never ask for a broader token
so you can do it yourself.

The tag is cut inside the release run on purpose. A tag pushed with a workflow's own token does not
trigger another workflow, so a separate "push a tag, let release fire" step fails silently. If
the repo ships a break-glass tag-cutting verb, it covers only the routine, no-waiver case. A waived
or hotfix release is a human act, never that verb's.

## 4. Harvest

After publication, read `checksums.txt` from the release itself. Confirm it lists every platform
artifact the pins name, and take each sha256 from it. A missing asset means an incomplete release:
stop and report. Never pin a partial set.

## 5. Re-pin every consumer

- **The repo's own pairing.** A manifest that pairs the package version with a tool release and
  its per-platform hashes is re-pinned in one PR. Bump the tag, refresh every hash, never edit
  one hash in place, then run the repo's pin checker.
- **Each consumer.** Before the re-pin PR, read that consumer's guard and the variables it is
  configured with. A pin its own guard rejects is not a re-pin. Update its pin file (the
  per-artifact lines, plus the umbrella line if it carries one), validate with `deskpins` and
  confirm with `deskversion`. If the consumer pins the plugin marketplace to a ref, move that ref
  in the same PR.
- **A behaviour-carrying re-pin** also writes the consumer's warmup marker naming the change, so
  its reviewers know a behaviour window is open.
- Every re-pin is a draft PR under that consumer's own review and merge gate.

Moving an adopter across versions, migrations included, is `upgrade-assay`'s job.

## 6. Done means green downstream

The chain is done when every consumer is green on the new pin. A consumer that goes red on the
re-pin is a finding against the release. Diagnose it. If the artifact is wrong, fix forward with a
new number. Never move the tag.

## This bundle's own release

The bundle's own release runs the chain above:

- One plain `vX.Y.Z` umbrella tag is cut by the repo's `release` workflow through its manual
  dispatch. The release carries statusgen and desk-tools binaries plus one `checksums.txt`.
- After publish, the changelog roll clears the fragments and stamps main to the next patch
  version.
- `plugins/assay/paired-versions.yaml` is re-pinned in a follow-up PR, checked by
  `plugins/assay/scripts/check-paired-versions.sh`.
- Adopters pin as `docs/distribution.md` describes.

A release-by-merge lane is staged but not live until its decision record is approved. A staged
workflow is not the release path.
