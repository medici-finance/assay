### Fixed
- The GitLab CI template `statusgen init --forge gitlab` emits, and `docs/adopting-assay-gitlab.md`,
  now require the regen job's push credential `STATUSGEN_PUSH_TOKEN` to be a **masked and
  protected** CI/CD variable; both previously said masked only. A masked-only variable is still
  injected into merge-request pipelines, which run the MR branch's own CI file, so a member who
  could open an MR could read the token and push to the default branch past the merge gate.
  Protected limits it to pipelines on protected refs — the default branch the regen job runs on,
  which the adopting doc's provisioning step already protects. The template's guidance comment and
  stop message, and the doc's UI and API creation forms (`protected=true`), carry the requirement;
  the scaffold test pins it. Found by a GitLab adopter cell's review.
