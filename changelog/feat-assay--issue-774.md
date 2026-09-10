### Added
- **`create-fleet-gitlab.sh` now provisions the desk labels a GitLab adopter's project needs,
  closing the last gap before `deskflip` can drive an MR to ready.** The fleet script created
  every account, protection and merge gate but never any labels, so a GitLab project had no
  `authorization-needed` / `approval-needed` queue-legibility pair (nor the `review-request`
  dispatch token or the `raised-by:<role>` provenance stamps). A missing label degrades
  **silently** on GitLab — `deskflip`'s `authorization-needed` → `approval-needed` swap fails,
  and `deskfile --raised-by` drops the stamp — so the script now creates the full set under
  `--project` via `POST /projects/:id/labels`, idempotently (a duplicate name answers 409, or
  400 "already exists" — both the success case for an ensure, matching the forge seam). This is
  the GitLab twin of the GitHub `create-labels` adoption primitive: the same names, colors and
  descriptions, so the two adoption profiles are label-parity. Colors are sent with the leading
  `#` GitLab requires. The GitLab adoption guide's by-hand table gains the matching endpoint row.
