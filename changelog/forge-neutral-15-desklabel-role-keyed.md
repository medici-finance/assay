### Added
- **`forge-neutral` brief 15 — `desklabel`, a role-keyed label verb, on the forge resolver.**
  Specifies a new verb that sets or clears one label at a time: any role may touch the shared
  escalation vocabulary (`question`, `help wanted`, `needs-decision`), a role may touch only
  the disposition markers its own lane already applies (worker: `superseded?` and the
  `disposition:*` family; reviewer: `authorization-needed`/`approval-needed`), and every other
  label — including `human-decided`, refused for every role — is refused outright (exit 5).
  Adds one new `Forge` operation, `ApplyIssueLabels`, because GitLab's existing `ApplyLabels`
  reconciles only a merge request's labels and a plain issue is a separate resource there
  (GitHub already serves both kinds from one endpoint, so its side is unchanged). Cites a real
  gap the design closes: `deskclose superseded --dispute` never removes the worker's
  `superseded?` proposal marker, and before this brief only a raw, unscoped label write could
  clear it. Doc only; no tool behaviour changes in this PR.
