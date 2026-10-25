- Issueboard groups positive closure observations for absent active placeholders
  through a bounded read-only state adapter, preserving positive open/closed proof,
  titles, complete-list refusals and all trust/history checks. Singleton and other
  provider paths retain their existing reads; partial/error results still refuse
  the sweep. The shared adapter carries no credential or arbitrary query API and
  does not move statusgen's online reads across its existing deskread boundary.

- Regression and mutation controls cover request counts, strict response identity,
  partial data, GraphQL errors, repository scope and original fail-closed behavior.

Issue: #4360
