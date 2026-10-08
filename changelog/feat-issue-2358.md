### Fixed
- The desk preflight's write-transport check no longer reads a 502 or 503 as a REJECTED identity when the forge's URL happens to contain `401` or `403` (for example a test server on port 40109). Rejection is now judged on the wording of the transport's answer only, with URLs and host:port addresses blanked first and status codes matched as whole tokens.
