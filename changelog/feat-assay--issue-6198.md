### Fixed

- The ready-flip checks-green gate no longer passes a head whose classic branch-protection checks never reported. When the required set is read without administration rights, it is now the union of the ruleset contexts and the contexts the branch read shows for classic protection; if classic protection cannot be ruled out or read, the set is could-not-check instead of a partial list that looked complete.
