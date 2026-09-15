### Fixed
- The `deskclose` mutation gate (`cmd/deskclose/mutations.json`) is load-bearing again. Its
  "the dispute posts its reason but never applies needs-decision" plant matched the old
  `addLabel(repo, n, spec)` call; when the label write started carrying the item's kind
  (`addLabel(repo, n, it, spec)`, so a GitLab issue is never labelled as the merge request
  sharing its number) the plant's text stopped resolving, muhar reported it could-not-mutate,
  and the truth-suite's `mutation-gate (cmd/deskclose/mutations.json)` leg went red on main —
  not because a guard failed, but because that one guard was no longer being tested. The plant
  now names the current call; the gate's baseline and control were green throughout.
