### Fixed
- An `ASSAY_HUMAN_LOGIN_MAP` (or `ASSAY_FORMER_HUMAN_LOGIN_MAP`) entry whose name contains a space, such as a GitLab display name, no longer refuses the whole roster: that one entry is not applied and the config echo names it in a `NOTICE`, while every other entry and key still loads. The human it names gets no mapping, so a write that needs their neutral name still refuses.
