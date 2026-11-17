### Fixed
- A `review-finding/v1` block carrying a field the schema does not define (such as `resolvedHead`) is still refused, but the refusal now names the field, lists the v1 finding fields, and for a head-like field points at `evidenceHead`, the field that records the head a resolution was checked at. The schema doc and the reviewer kit say so too.
