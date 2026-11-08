### Fixed
- The forge-neutral/55 mutation script and Verify rows 7 and 10 can now give a verdict on merged main: the script runs on a clean detached checkout (it checks only the files it mutates and pins the delivery and its first-parent base), and both rows pin the delivery and base commits instead of resolving them from the verified checkout.
