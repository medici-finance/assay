### Changed
- The release workflow pins `actions/upload-artifact` v6.0.0 and `actions/download-artifact` v7.0.0 (both run on Node 24), replacing v4.6.2 and v4.3.0, which target the deprecated Node 20 runtime and drew a deprecation warning on the `test (deskmerge)` legs of every release run. Inputs are unchanged. The staged and proposed copies of the workflow carry the same pins.
