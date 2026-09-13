### Added
- Authored `docs/streams/desk-tools/brief-22-trust-gate-account-liveness-notice.md`: a
  read-only, fail-closed liveness check for the trust gate's configured logins, surfaced as a
  `deskroster liveness` NOTICE only — it does not change who `TrustedAuthor`/`TrustedHumanAuthor`
  trust today (closes #933; auto-revocation is tracked as separate follow-up).
