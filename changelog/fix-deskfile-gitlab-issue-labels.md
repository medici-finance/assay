### Fixed
- `deskfile new` on a GitLab project no longer stamps its labels on the MERGE REQUEST that
  happens to share the new issue's number, leaving the issue itself unlabelled. GitLab numbers
  issues and merge requests in two separate sequences, and the label write behind `deskfile`
  only knew how to address a merge request — so `to:<role>` addressing and the label-keyed
  dedupe both silently missed every issue it filed. Field evidence from a GitLab adopter cell:
  three `deskfile new` runs each labelled an unrelated, already-merged MR and left the issue
  unstamped.
- The label reconciliation now carries WHAT it is labelling: `LabelChange` names its target as
  an issue or a change (PR/MR), and every caller states it — `deskfile new` and `deskclose`'s
  issue arm label the issue, `deskflip`, `deskpost` and `deskdispatch` label the change. On
  GitLab an issue target goes to `PUT /projects/:id/issues/:iid` with the same
  `add_labels`/`remove_labels` reconciliation the MR route uses (and the same ensure-label
  step). GitHub shares one number space and one labels endpoint for both kinds, so its requests
  are unchanged.
- A label write that names NO target is refused on both forges rather than defaulted to the
  merge-request route. A caller that forgot the target would otherwise reproduce this exact
  defect on GitLab while passing every GitHub test; the refusal is what makes the omission loud
  on the forge most contributors run.
