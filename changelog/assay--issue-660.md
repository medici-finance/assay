### Fixed
- `deskboot` step 5 now logs the roster-preflight's OWN verdict when the envelope is
  red — the `preflight role=… RED n/5` summary and every `<check>=checked-failed: … →
  fix: …` remediation — instead of `firstLine`-ing the captured output, which always
  quoted the `assay-config: … configured=true` banner every desk tool prints first and
  left the failing check unknown in the pod log.
