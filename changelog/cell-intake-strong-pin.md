### Changed
- New cells pin the intake role to a strong tier: `cellctl new` scaffolds `DESK_MODEL_intake_desk=opus`
  and the seeded provider defaults give `intake-desk` tier `strong` under every provider, so a new
  cell's intake can do its judgment work from the first boot. Existing cells and provider-defaults
  files are not rewritten; the docs list how to move one.

### Added
- `cellctl set` prints a notice on stderr, and still writes with the same exit code, when a model pin
  it wrote is not read by the cell: the cell resolves through a model policy or provider defaults, or
  the key is in a pin namespace the cell's harness does not read. The notice names what decides.
