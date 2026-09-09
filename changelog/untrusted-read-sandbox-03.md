### Added
- `deskscanuntrusted` — a deterministic INBOUND exfil + injection pre-scanner (the inbound
  sibling of the outbound `bodycheck`). It reads untrusted content bytes with no model and
  no network and returns a three-state, fail-closed verdict across three independent
  detector families: exfil/callout markers (env reads, outbound sinks, secret-file paths),
  invisible-Unicode / bidi / imperative-lure injection markers, and a Semgrep code-exec leg
  (base64→exec, install-hook override, command overwrite, steganographic extract→exec).
  A missing Semgrep makes the code leg could-not-check, never clean.
- The scanner emits a NEUTRALISED rendering — every invisible/bidi/control codepoint
  escaped to a visible `\uXXXX`, the whole body fenced as inert data — for downstream
  quarantined reads (briefs 05/06) to consume in place of raw untrusted bytes.
- An only-widens house rule-pack seam (`ASSAY_UNTRUSTSCAN_CALLOUT`) can ADD detections but
  can never clear a built-in flag; a configured pack that fails to answer degrades a clean
  family to could-not-check. Built on the positive-control corpus from #634.
