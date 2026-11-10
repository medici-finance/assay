### Changed

- `desktoken` on GitLab now says on stderr, after the path, that the printed file is the role's custody file and the only copy of its credential, and that it must not be deleted; a rotating call also says the previous value is now invalid. stdout is unchanged, and GitHub output is unchanged.
- The shipped desk skills, the desk-shell `gh` recipe and the adopter docs now resolve a role's credential with `desktoken --no-rotate <role> --repo <slug>`, so a read-only caller on a GitLab-resolved repo no longer rotates the role's credential as a side effect. Rotation is described as an explicit act.
- `docs/adopting-assay-gitlab.md` §5 gains a recovery note for a custody file lost after a rotation: the role is locked out, a group Owner issues a new credential, and the orphaned one is revoked.
