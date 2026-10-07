### Fixed
- `evidence-automerge`: the refusal classifier now judges only the latest run per check name, so a superseded CANCELLED run (one replaced by a later run of the same check) no longer reddens the `enable` check and strands a ready, approved Evidence PR without auto-merge. A check whose latest run is red or cancelled still reddens.
