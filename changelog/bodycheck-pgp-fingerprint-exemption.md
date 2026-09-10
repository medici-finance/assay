### Fixed
- **The desk body-check no longer refuses an uppercase-hex PGP key fingerprint as a
  possible secret.** A 40-char OpenPGP v4 fingerprint is written in UPPERCASE hex, but the
  high-entropy-run scanner exempted only LOWERCASE hex (git SHAs), so a body quoting a
  `.sops.yaml` recipient list (`pgp:`) or a sops metadata `fp:` field tripped the "40-char
  high-entropy run (possible secret)" refusal — blocking writes that merely referenced a
  PUBLIC key fingerprint. A new narrowly-anchored exemption admits a run that is EXACTLY 40
  uppercase hex ONLY when a `pgp:`/`fp:` recipient key precedes it (a `.sops.yaml` recipient
  entry or a sops `fp:` field), separated by nothing but YAML/JSON value scaffolding or a
  comma-list of fingerprints. The anchor is load-bearing and the check is not loosened for
  genuine secrets: a bare uppercase-hex run with no recipient key, a lowercase/mixed-case
  40-char run, and a real high-entropy token wearing the same field all still refuse (an AWS
  secret key, for instance, is 40 mixed-case base64 and never qualifies).
