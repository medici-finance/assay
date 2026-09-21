### Added
- The Go `cellctl` launcher reads shared provider model and desk-effort defaults from `CELLS_ROOT/providers.json`, with partial per-cell overrides. `cellctl providers init` creates the editable catalog without overwriting it, and Claude launches export the selected provider's Fable, Opus, Sonnet and Haiku mappings. Existing complete model policies retain precedence.
