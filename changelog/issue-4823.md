### Fixed
- Preserve credential-free repository origin metadata in immutable verification closure checkouts, so protected acceptance transitions can corroborate their existing human approvals when explicitly authorized. Offline approval checks remain held. (#4823)
- Honor supported effective origin scopes, URL-list resets and fetch rewrites; withhold uncertain authority without aborting unrelated closures, and refuse command-priority Git config injection at the approval read. (#5109)
- Consume owner-proven origin authority directly so ordinary ambient rewrites cannot turn missing snapshot metadata into a foreign approval origin.
- Support checkout-v7's proven authority-neutral conditional credential includes, without persisting headers/helpers or accepting authority-changing, unreadable or unsupported includes.
