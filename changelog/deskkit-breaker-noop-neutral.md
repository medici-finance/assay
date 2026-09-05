### Fixed
- deskkit circuit breaker no longer trips a healthy *quiet* loop. A `noop` result — the
  tool confirming the desired state already holds, the shape of an idempotent verb a standing
  loop re-asserts on every quiet tick — is now neutral: invisible to the breaker's
  consecutive-non-progress meter, neither tripping it nor resetting it (the same treatment
  `dryrun` already gets). Previously five consecutive quiet ticks opened the breaker with
  nothing having failed, after which the refusals it produced were themselves non-progress and
  the run never reset. Only `refused`/`unwritten` now advance the run (#180).
