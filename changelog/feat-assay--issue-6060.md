### Fixed

- `deskpr create --help`, the desk-tools README and the Evidence publishing contract now say that the confirmed destination tip narrows only the publish-identity check. The secret scan and the outbound check judge the whole branch diff against the default branch in every mode, so a value an earlier push cleared with `--force-scan-override` is refused again by `--check-remote` and by the next create. Scan behaviour is unchanged; a regression test pins it.
