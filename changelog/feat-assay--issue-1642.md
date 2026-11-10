### Fixed
- The desk tools' secret scan no longer refuses a `make test TEST=<GoTestName>` selector in a Verify row: an ALL-CAPS env-var key in front of a word-shaped identifier is now treated as the "short or word-shaped key" the refusal text always named. A name carrying a credential or digest stem, a key over 16 capitals, and any value that is not an exempt identifier on its own still refuse.
