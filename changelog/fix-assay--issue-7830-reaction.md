### Added
- The `forgechange` client can add a reaction to a GitHub pull request or GitLab merge request. `AddReaction` accepts GitHub's eight reaction values, maps them to GitLab award emoji, and checks the reviewed head and repository visibility the same way `PublishComment` does. A reaction the identity already gave counts as success.
