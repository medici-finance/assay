# Service contract fixtures

request-source.json, response-source.json, legacy-issues.json and operation-cases.json are independently authored literal values. They are not output from the service codec.

review-observations.json freezes data from the existing reviewflow producer at main `214b4c8308b3cc57beed91feccf45e6a03f2eecd`. The original independent TestReviewContractFlow classifier oracle, population/scope and empty/contradiction cases passed while an observational Go test overlay logged each unmodified BuildObservation result. Producer source and assertions were unchanged. The source test blob is SHA-256 `d25a299984e88f3195e3f837d193122b2a718a4a154c3f1820912bada2b3b306`.

These literals include full 151-row population, independent lanes, host/provider separation, WAIT/SURFACE/NO-OP rows, exact head/base/body/review/check applicability and explicit known-empty values. They qualify transport preservation; current source collection, review enrichment, policy evaluation, admission, serving and cutover remain their existing owners. The transport does not regenerate expected classifier decisions. Original producer fixtures and schema are retained unchanged.

operation-cases.json covers bounded legacy mirror complete-empty, Diff/MergeBase, existing-key issue/comment effects, unknown operation lookup, exact observation progress and role issue projection. Projection rows map the existing issueboard row fields/action vocabulary at the pinned source, including explicit unknown history and quarantine; these literals do not recompute selection, trust or current collection.
