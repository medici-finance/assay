### Fixed
- `deskapps init --tier`/`--manifest` no longer posts a url-less `hook_attributes` (e.g.
  `{"active": false}`) to GitHub's App-manifest new-App page — GitHub's manifest schema
  requires `hook_attributes.url` whenever the object is present at all, and rejected the
  bare-`active` shape with `"url" wasn't supplied`. A webhook-less App now omits the
  `hook_attributes` key from the posted manifest JSON entirely.
