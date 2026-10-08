### Added
- A findings entry can now carry an effectiveness record (`effectiveness:`, `effectiveness-date:`, `effectiveness-by:`, required together or not at all). `statusgen --lint` fails a `resolved: yes` finding dated on or after 2026-10-08 that has none, and advises on earlier ones. The check covers presence and attribution only, not whether the named command would catch a recurrence.
