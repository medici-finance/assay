### Changed
- The adopter docs now record the permission `deskflip`'s ready-flip actually needs: the
  **reviewer App requires `Administration: Read-only`**. `deskflip` reads a branch's required status
  checks through the legacy branch-protection endpoint first, and that endpoint is the only one that
  can read a required set the **rules API cannot express** — the rules API surfaces rulesets only,
  and within a ruleset only a `required_status_checks` rule carries contexts. Two different
  configurations therefore look identical to the gate (`protected: true`, no contexts named): a
  branch under **classic** protection, and a branch under a **ruleset that carries no
  `required_status_checks` rule** — say, one whose rules are only `deletion` and `non_fast_forward`.
  The second is the easier one to have by accident. In both, the gate fails closed to
  could-not-check by design and no PR on that repo is ever flipped ready.
  `docs/adopting-assay.md` gains a *Required checks a ruleset does not express* subsection under
  `setup-reviewer-app` — the mechanism, why failing closed is right, and **both** human remedies
  stated neutrally: grant the reviewer App `Administration: Read-only` (the durable fix, and the
  only one that works for a branch genuinely under classic protection), or add a
  `required_status_checks` rule to the branch's ruleset (no permission change needed, but it fixes
  only the ruleset case and changes what the forge enforces at merge time). The grant's sequence is
  spelled out — toggle, accept on each installation, then re-mint, since an issued token keeps its
  old scopes for `desktoken`'s 50-minute cache window. Every place that enumerated the reviewer
  App's grant now names the permission: the App inventory table, the post-install checklist, the
  primitive, the Verify read-back, and Scenario 2's fleet-wide install step.
  `docs/enforcement-model.md` records that role-scoped **reads** sit outside `requiredDuties`, so a
  missing one costs no boot, only every flip. Which roles: the reviewer App REQUIRES it (each
  instance separately — the grant is per App installation, so a cell's own reviewer twin needs its
  own); the desk App SHOULD have it for board reads of the required set; worker, verifier and the
  inbound-lane Apps do not. The `deskapps` tier manifests gain it too, so a future install is not
  born blocked. `docs/adopting-assay-gitlab.md` states the GitLab equivalent honestly: there is no
  toggle of that shape, the equivalent reads are protected-branch + external-status-check API calls
  under the plain `api` scope, and whether the reviewer's Developer level can make them is
  **could-not-check** until read back on the instance. (#1020)
