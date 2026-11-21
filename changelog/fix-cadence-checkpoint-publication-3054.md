### Fixed
- Reading a cadence checkpoint can now recognize a complete replacement published during the read, preventing a healthy comms shutdown from failing its identity check. A further replacement during that check still requires another observation.
