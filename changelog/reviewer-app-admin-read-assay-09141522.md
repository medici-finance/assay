### Changed
- The adopter docs now record the permission `deskflip`'s ready-flip actually needs: the
  **reviewer App requires `Administration: Read-only`**. `deskflip` reads a branch's required
  status checks through the legacy branch-protection endpoint first, and that endpoint — the
  only one that can see **classic** branch protection, since the rules API surfaces rulesets
  only — is gated on that permission. Without it the gate fails closed to could-not-check on
  every classically-protected repo and no PR there is ever flipped ready. `docs/adopting-assay.md`
  gains a *Classic branch protection needs `Administration: Read-only`* subsection under
  `setup-reviewer-app` (why the fallback cannot see classic protection, why failing closed is the
  design, the toggle-then-accept-then-re-mint sequence, and that an already-minted token keeps its
  old scopes for `desktoken`'s 50-minute cache window), and every place that enumerated the
  reviewer App's grant — the App inventory table, the post-install checklist, the primitive, the
  Verify read-back, and Scenario 2's fleet-wide install step — now names it. `docs/enforcement-model.md`
  records that role-scoped **reads** sit outside `requiredDuties`, so a missing one costs no boot,
  only every flip. Which roles: the reviewer App REQUIRES it (each instance separately — the grant
  is per App installation, so a cell's own reviewer twin needs its own); the desk App SHOULD have
  it for board reads of the required set; worker, verifier and the inbound-lane Apps do not.
  The `deskapps` tier manifests gain it too, so a future install is not born blocked.
  `docs/adopting-assay-gitlab.md` states the GitLab equivalent honestly: there is no toggle of that
  shape, the equivalent reads are protected-branch + external-status-check API calls under the
  plain `api` scope, and whether the reviewer's Developer level can make them is **could-not-check**
  until read back on the instance. Granting the permission is a human act — nothing in the tree can
  make it. (#1020)
