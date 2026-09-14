### Fixed
- The GitLab CI template `statusgen init --forge gitlab` emits, and `docs/adopting-assay-gitlab.md`,
  now tell the operator to store the regen job's push credential `STATUSGEN_PUSH_TOKEN` as a
  **masked and protected** CI/CD variable, where both previously said masked only. A masked-only
  variable is still injected into merge-request pipelines, which run the MR branch's own CI file,
  so any member who could open an MR could read the token and push to the default branch past the
  merge gate. Protected limits the variable to pipelines on protected refs — the default branch
  the regen job runs on, which the adopting doc's provisioning step already protects. Found by a
  GitLab adopter cell's review; the template's stop message, the guidance comment, and the doc's
  UI and API creation forms (`protected=true`) all carry the new requirement, and the scaffold test
  pins it.
