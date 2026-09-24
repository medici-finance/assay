### Fixed
- `statusgen verifyrun` no longer writes a human's forge login into the on-behalf-of annotation of a witness row on a public repo. The witness now follows the same visibility split as the desk write verbs: the roster's neutral name on any repo the roster does not state is `:private` (read from the brief's stream `repo:` frontmatter; unstated counts as public), the login only on a known-private one, and no annotation at all when no neutral name is configured.
- The `statusgen --lint` principal-attribution check and the `--corroborate` lanes now accept an on-behalf-of principal in either form the writers stamp: a configured login or a configured neutral name. Before this, a public-target trailer read as an unrecognised principal or as an uncorroborated `human:<name>` stamp.

### Added
- Class guards for the on-behalf-of trailer in both modules (`TestOnBehalfOfRenderedOnlyThroughTheResolver` in the desk tools, `TestWitnessOnBehalfOfRenderedOnlyThroughTheRenderer` in statusgen). They fail on a second renderer, on `OnBehalfOfPrefix` used to compose a trailer, on a literal target repo, and on any resolver call site missing from a reviewed allow-list.
