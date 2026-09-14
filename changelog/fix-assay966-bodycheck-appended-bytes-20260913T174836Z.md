### Fixed
- `deskevidence`'s secret scan no longer re-scans a brief's WHOLE pre-existing body on an
  Evidence landing made without `--brief-path`. In that flow `--evidence-file` is the caller's
  own merged copy of the target file, so the scan previously treated every pre-existing byte —
  a Verify row's own quoted secret-shaped material — as part of THIS commit, forever. It now
  fetches the target's current remote content once, up front, and scans only the lines that
  are actually new relative to it (the same "added bytes only" scoping `--brief-path` merges
  already had). (#966)
- `internal/deskkit`'s secret/entropy scanner gained two narrow, closed exemptions so neither
  shape can trip a false positive again even when correctly scoped to newly-added text: a
  slash-list of short ALL-CAPS enum/status words (`PENDING/RUNNING/BLOCKED/DONE`), and a 32-hex
  run directly behind a hyphen and a recognised Kubernetes object-kind prefix (`pvc-<uid>`, the
  dashes-stripped rendering Kubernetes itself emits for a PersistentVolumeClaim's bound
  PersistentVolume). Both are bounded the same way every other exemption in this file is: a
  real secret pasted in either shape (mixed case, digits, or an unlisted prefix) still refuses.
  (#966)
