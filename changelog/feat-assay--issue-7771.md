### Fixed
- statusgen tests no longer deadlock when captured output passes the pipe buffer: every stdout/stderr capture goes through one helper that drains the pipe while the code under test runs, and a guard test refuses any other `os.Pipe()` capture in the package's tests.
