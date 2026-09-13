### Added
- `assay.harness` is now an exclusively-bound key (composability/04): the Claude
  Code, Codex, and Cursor delivery shapes are owned by three adapter
  components (`components/harness-{claude-code,codex,cursor}/component.yaml`)
  that each `provide: assay.harness` with a `flavour`; `deskmanifest lint`
  refuses a tree with more than one ACTIVE provider and its new `--activation`
  flag reports every component's computed ACTIVE/INACTIVE state.
- `deskmanifest`'s manifest schema grows two `provides` attributes: `flavour`
  (also usable as an `inject` constraint, e.g. the hooks component now
  requires `assay.harness` at `flavour: claude-code`) and `evidence` (a
  repo-relative installed-shape marker that decides which adapter is ACTIVE
  ahead of the desired-state record).
