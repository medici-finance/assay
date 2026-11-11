### Fixed
- `deskdispatch` context pickup now reads a context receipt up to the same 16 MiB bound `statusgen context` accepts, instead of holding any receipt over 8 MiB. Both tools take that bound from one shared constant in the `contexthandoff` package, so the two limits can no longer drift apart.
