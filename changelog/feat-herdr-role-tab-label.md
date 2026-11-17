### Changed
- `cellctl up` on herdr labels each role tab with the bare role (and the deskd tab `deskd`), so herdr's agents window shows `<cell> - <role>` instead of repeating the cell name. A per-role `down` still recognises a tab and receipt labelled `<cell>-<role>` by an older build; tmux windows and orca terminals keep their `<cell>-<role>` names.
