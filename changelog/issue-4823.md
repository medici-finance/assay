### Fixed
- Preserve credential-free repository origin metadata in immutable verification closure checkouts, so protected acceptance transitions can corroborate their existing human approvals when explicitly authorized. Offline approval checks remain held. (#4823)
- Honor supported effective origin scopes, URL-list resets and fetch rewrites; withhold uncertain authority without aborting unrelated closures, and refuse command-priority Git config injection at the approval read. (#5109)
