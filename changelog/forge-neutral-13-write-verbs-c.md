### Added
- **`forge-neutral` brief 13 — write verbs C (`deskpr` / `deskfile` / `deskclose`) onto the
  forge resolver.** The code-aware follow-on that brief 04 (`#509`) was ruled down to leave
  undone: it plans the four operations these three verbs still lack (a branch→change lookup,
  a change body/title edit, an issue text-search, and a read-only list-labels), then re-seats
  each verb onto `ForgeFor` — the step that answers the identity-class permit rows instead of
  moving them, supersedes `deskfile`'s interim GitLab named-refusal (`#691`), and drops the
  forge-CLI ratchet by three. Doc/plan only; no tool behaviour changes in this PR.
