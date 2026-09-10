### Changed
- **`statusgen --lint`'s same-tag pin check now honours a per-line exemption marker**, so
  `.assay-versions` stays lintable for adopters who legitimately pin one artifact on a different
  tag. A trailing `# same-tag: exempt — <reason>` comment removes that one line from the
  one-tag-one-tree grouping while keeping it a fully valid, lint-visible pin. It clears both real
  cases — a guard binary frozen on an earlier tag by a maintainer ruling, and a
  separate-repository artifact on its own release cadence the umbrella never ships. Every
  non-exempt artifact must still share one tag: a genuine undeclared mixed-tag state still
  PROBLEMs, and the exempt line's off-tag never leaks into that message.
