### Fixed

- deskevidence: the tests that pin a refused landing to one lock wait in total now count the wait each lock acquisition is granted instead of timing the whole run, so a loaded CI runner no longer reddens them and a real second wait still fails them on any runner.
