### Fixed
- `ASSAY_WITHHELD_IDENTIFIERS` is now read from `roster.env` as well as from the
  environment, like every other `ASSAY_` key (environment first, roster second).
  It was environment-only while the roster parser merely *recognised* the key, so a
  house that configured its withheld register in `roster.env` — the documented home
  of every other value — got `withheld register identifiers NOT CHECKED` on every
  public write and the register category of the public-repo self-containment scan
  never ran, unless each shell invoking `deskpr`/`deskpost` also exported the
  variable. Unset in both sources is still a complete adopter configuration: the
  category degrades to a notice, which now names both places it looked.
