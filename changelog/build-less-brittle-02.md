### Added
- Every new brief now carries a `design-fit:` block in its Context (`owner`, `contract`, `retires`, `weight`, `why-add`), so the author answers which module owns the change, what it retires and how much weight it adds before a worker starts. Defined in `spec/brief-v1.md` §4.1, with an example in `docs/brief-template.md` and the author-brief skill template.

### Changed
- `exec-tier` derivation gains question (d), "Is this a design brief raised by an error-class trigger?" — yes means `strong`. The author-brief dispatch checklist folds `design-fit:` into its layering item and stays at nine items; the skill ends shorter than it started.
