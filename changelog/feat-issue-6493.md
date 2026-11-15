### Changed
- The `deskd` route-walk test now also runs the served-mux walk and the only-mux check as subtests, so a mount outside the identity wrap fails the route-walk selector too. Test-only; no production change.
