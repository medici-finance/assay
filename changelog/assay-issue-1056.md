### Fixed
- The login a desk ROLE is expected to act under is now resolved from the forge its roster entry
  declares, instead of always rendering GitHub's `<slug>[bot]`. On GitLab a service account is
  attributed by its bare username, so the expected reviewer login never matched an actual one —
  and because the actor comparison carries an is-an-App flag as well as a name, the two could not
  match even when the slug was identical. Every gate keyed on that comparison read "no verdict"
  for approvals that were really there.
- `deskflip` therefore refused `reviewer-approved` on a GitLab merge request that the rostered
  reviewer had APPROVED at the current head, so no GitLab change could ever be flipped
  ready-for-human.
- The review board reduced the same approval to no-verdict, classifying an approved merge request
  as NEEDS-REVIEW and firing its UNREVIEWED alarm on it indefinitely. The board and the flip gate
  share the expected login, so both surfaces are fixed by the same resolution rather than
  separately.
- GitHub behaviour is unchanged: a GitHub App still renders `<slug>[bot]`, and a bare App slug is
  still not a role login — the fail-close that stops a user named after an App slug from
  satisfying a role comparison.
