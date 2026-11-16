### Changed
- `cellctl --apply` now asks for a one-word confirm code (for example `dahlia`) instead of 8 hex characters. The word is chosen from the plan's digest out of the plan-id word lists, is never a word of the plan's own id, and is typed without regard to case. A plan stored with the old 8-hex confirm is refused `plan-invalid` with a reason naming the retired form; make the plan again.
