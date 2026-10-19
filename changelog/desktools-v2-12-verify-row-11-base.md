### Fixed
- desktools-v2/12's consumer-check Verify row now pins its delivering commit and first-parent base, and asserts the exact corroborated / disproved / unchecked summary, so it satisfies the `consumer-delivery-range` lint instead of running against an empty post-merge diff.
