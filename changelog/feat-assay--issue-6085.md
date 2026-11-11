### Fixed

- Pre-work attestation records filed on GitLab are admitted when read back. GitLab stores an issue description without its trailing newline, so every worker-kit and verifier-kit record, and the statusgen credit reader, refused its own record. The one shared comparison now admits the exact rendering, or the rendering with exactly one trailing newline removed. A second dropped newline, a carriage return, a space, or any other byte difference still refuses.
- A refused attestation read-back now names the reason after the generic line, for example `has a body that is not the exact rendering of its binding` or `has title ...`. The reason never quotes the body read back.
