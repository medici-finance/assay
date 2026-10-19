# cell-workspaces-v1 — the shared workspace contract

**Status:** DRAFT, published for review. Part of the v1.0-draft specification set; breaking changes MAY be made without a major-version bump.
**Schema:** [`schemas/cell-workspace-v1.json`](../schemas/cell-workspace-v1.json)
**Reference implementation:** the `workspace` Go module (`github.com/medici-finance/assay/workspace`), with its contract fixtures under `workspace/testdata/contract/`.
**Caller census:** [`docs/workspace-callers.md`](../docs/workspace-callers.md)

## 1. Purpose and boundary

Every tool that creates, attaches to or removes a checkout for an agent does it today with its own local rules for placement, naming and cleanup, and those rules disagree. This contract is the one meaning they move onto: what identifies a workspace, how a request for one is made idempotent, how its lifecycle advances under a generation, and how an operation answers when it cannot succeed.

The contract is identity, validation and lifecycle only. It performs no Git, filesystem, process, network or clock effect; effects plug in through ports (section 8). It owns no claim, candidate, credential, process lease, scratch run or evidence: it stores references into the owners that already hold those. An allocation grants no authority. It never selects work, acquires or steals a task claim, or marks work done.

The existing `S-worktree` owner (`tools/desk/cmd/deskwt`) keeps its effects until the allocation step moves them behind this contract; until then this module is the contract that step and every other caller adopt.

Key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## 2. Four rules

1. **Unknown stays unknown.** A check that could not run yields an `unknown` outcome. It is never success, never an empty result, and never a refusal that a caller may treat as "nothing there".
2. **Identity is not a path.** A workspace is identified by its allocation, cell, repository, kind and owner scope. Its path and branch name are attributes that are checked against that identity, never the identity.
3. **Refuse what you do not understand.** A document of another schema version, an unknown field, an operation or capability this reader does not define, is refused. A reader never downgrades a request.
4. **No failure hands out a workspace.** Only a successful result carries a record, so no refused, conflicting or unknown outcome can give a caller a path to work in.

## 3. Identities

| Identity | Form | Notes |
|---|---|---|
| Cell | `{uuid, root}`: a lowercase RFC 9562 UUID and the absolute root it is admitted at | The UUID survives a rename or move of the cell; the root is where it is admitted now. |
| Repository | `{forge, repository}`: a lowercase forge host (optionally `:port`) and the canonical `owner/name` path on it | Never a URL, user name or credential. A path ending in `.git` or containing `..` is invalid. |
| Repository key | `<base>-<16 hex>` | The lowercased base name (non-alphanumerics folded to `-`, at most 32 characters, `repo` when empty) for readability, then the first 16 hex digits of `sha256(forge NUL repository)`. Two forges or namespaces with the same base name never share a key. A record whose key does not derive from its repository is an `identity-mismatch`. |
| Kind | `desk`, `worker`, `review`, `verify`, `scan`, `check` | A `desk` workspace is a role session's home; every other kind is one attempt at a task. |
| Owner scope | `session` (desk) or `task` (every other kind) | A desk workspace MUST name its role session and MUST NOT name a task. Every other kind MUST name its task or claim as a reference (`owner` `task` or `claim`); the claim itself stays at its existing owner. |
| Attempt | `attempt_id`: 1–256 visible ASCII characters | Distinct attempts at one task never share a mutable checkout. |
| Request | `request_id`: 1–256 visible ASCII characters | Chosen by the caller; repeated on retry. |
| Allocation | `allocation_id`: a lowercase RFC 9562 UUID | Assigned by the allocator when it reserves intent. |
| Subject | `{commit, mode, branch?}`: a full lowercase commit id (40 or 64 hex), `branch` or `detached` | Pinned before any checkout is created. `review`, `verify` and `check` workspaces hold a detached subject; a detached subject names no branch. |

Every identifier a caller chooses — `request_id`, `attempt_id`, a task or claim reference's `id`, and a role session — is 1–256 visible ASCII characters (U+0021–U+007E): no space, control, or non-ASCII character. The characters JSON and HTML treat specially (`"`, `\`, `<`, `>`, `&`) are allowed, which is why section 4.2 fixes how they are written.

Every path in these documents — the cell root, a record's logical and physical paths, and its Git common and administration directories — is an absolute local path: valid UTF-8 of at most 4096 bytes that starts at a POSIX root (`/` not followed by a second separator) or a drive root (`C:/` or `C:\`). It is never a network root (`//<host>/<share>`, `\\<host>\<share>`, `\\?\`), never contains a `..` segment between either separator, and never contains a control character (U+0000–U+001F or U+007F). The schema carries this rule as one pattern on every path member, the same string the reference implementation checks.

## 4. Requests

A request is a JSON object (`$defs.Request`). Version 1 defines one operation, `allocate`. The other lifecycle operations (attach, renew, finish, reconcile, inventory, retirement, adoption, transfer) are added by the versions that implement them; until then a reader refuses them as `unsupported-capability`.

### 4.1 Strict decoding

A document is decoded strictly, at every depth:

- the `schema` member is read first; any value other than `cell-workspace/v1`, or no `schema`, is refused as `unknown-schema` before any other rule, so a newer document is never mistaken for a malformed old one;
- an object MUST NOT repeat a key;
- a key MUST match a defined field name exactly, including case, and no other key is allowed;
- an optional field MUST be absent rather than `null` or empty;
- numbers MUST be in canonical form;
- nothing but whitespace MAY follow the document.

The last three are one check: the document must equal its own canonical re-encoding member for member. Every violation is `malformed`. A document that decodes but breaks a rule of sections 3–5 is `invalid`.

### 4.2 Canonical encoding and payload digest

The canonical encoding of a document is its members in schema order with no insignificant whitespace and with absent optional members omitted, as UTF-8 with nothing before or after the value — no byte-order mark and no trailing newline. A string is written as it is, except that:

- `"` and `\` are written `\"` and `\\`;
- U+0008, U+0009, U+000A, U+000C and U+000D are written `\b`, `\t`, `\n`, `\f` and `\r`;
- every other character below U+0020 is written `\u00` followed by two lowercase hex digits.

Nothing else is escaped: `<`, `>`, `&`, `/`, U+007F, every non-ASCII character, and U+2028 and U+2029 are written as their UTF-8 bytes. (This is the string form of RFC 8785. An encoder whose default escapes HTML-significant characters or the line and paragraph separators — Go's `json.Marshal`, for one — does not produce it unless told not to.)

The payload digest of a request is `sha256:` followed by the lowercase hex SHA-256 of its canonical encoding. The bytes a request arrived as are never hashed: two documents that differ only in whitespace, or in how a string is escaped, have the same digest.

### 4.3 Idempotency

An allocation is keyed by `(cell UUID, repository key, kind, owner scope, attempt_id, request_id)`, where the owner scope is `session:<session>` for a desk workspace and `<owner>:<id>` of the task reference otherwise. A request that arrives with the key of an existing record:

- with the record's payload digest, is the same allocation: the allocator returns the existing record and performs no new effect;
- with any other digest, is refused as `payload-mismatch`. A changed branch, subject, capability list or harness under a reused key is never silently applied.

### 4.4 Capabilities

`require` lists the capabilities the request depends on (for example `allocate/v1`). An allocator MUST refuse, as `unsupported-capability`, a request requiring a capability it does not declare. It MUST NOT drop the requirement and serve the request anyway.

## 5. Records and lifecycle

A record (`$defs.Record`) is the durable state of one workspace. Beyond the request's identity it carries the allocation id, the payload digest it was allocated under, the repository key, the logical path a caller is told and the resolved physical path the effect boundary verified, the Git common and administration directories verified at effect time, its state and generation, its provenance, and references: the process lease that holds it, and scratch, evidence and retention references. It never carries a key, token, credential, or a copy of claim or candidate authority.

### 5.1 States

| State | Meaning |
|---|---|
| `allocating` | Durable intent reserved before any Git effect. The only state without a verified Git identity. |
| `ready` | Git and identity checks done; eligible for one attach. |
| `active` | One session holds it under a validated process lease (`lease`, owner `process`). |
| `retained` | No writer; work, evidence or an unresolved question keeps it. Names its `reason`. |
| `retiring` | The exact generation and its preservation proof are reserved for removal. |
| `retired` | Absence independently verified; the record remains as a tombstone. |

Refused, conflicting and unknown are outcomes of an operation, never states. A missing directory is an observation to reconcile, never proof of retirement.

### 5.2 Transitions

| From | Allowed next states |
|---|---|
| `allocating` | `ready`, `retained`, `retiring` |
| `ready` | `active`, `retained`, `retiring` |
| `active` | `retained` |
| `retained` | `active`, `retiring` |
| `retiring` | `retired`, `retained` |
| `retired` | none |

Every transition is a compare-and-set on the record's generation: the caller names the generation it read; a different stored generation is a `stale-generation` conflict, and a transition the table does not allow is `illegal-transition`. A successful transition increments the generation by one and stamps `transitioned_at`. The generation starts at 1.

### 5.3 Provenance

`managed` records were allocated under this contract. `legacy` records describe a checkout that existed before it and was adopted by evidence; a legacy record is never `allocating` or `ready`, because it was never allocated here, and adoption grants it no cleanup right of its own.

## 6. Results and outcomes

A result (`$defs.Result`) answers one request by its `request_id`. Its outcome kind is one of:

- `ok` — the operation succeeded; the result carries the record it observed and no reason;
- `refused` — the request is not acceptable as sent;
- `conflict` — the request met state it must not overwrite;
- `unknown` — the operation could not check; nothing was decided.

Every reason belongs to exactly one kind, and a result whose reason does not match its kind is invalid:

| Reason | Kind | When |
|---|---|---|
| `unknown-schema` | refused | another schema version, or none |
| `malformed` | refused | a strict-decoding rule (section 4.1) |
| `invalid` | refused | a field or cross-field rule |
| `unsupported-capability` | refused | an undefined operation or an undeclared required capability |
| `payload-mismatch` | refused | a reused key with a changed payload |
| `identity-mismatch` | refused | a derived identity (repository key, path, Git directory) does not match |
| `illegal-transition` | refused | a lifecycle edge the table does not allow |
| `stale-generation` | conflict | the stored generation is not the one the caller read |
| `foreign-state` | conflict | the effect found a directory, branch or registration it does not own |
| `busy` | unknown | the per-repository lock could not be acquired within its bound |
| `evidence-unavailable` | unknown | an observation could not be made; also every error that is not a typed refusal |

`field` names the path of the failing member, never its content. A non-`ok` result carries no record.

## 7. Versioning

`schema` is the version gate of every document. Additive members and operations arrive in a new version; a v1 reader refuses them rather than ignore them, because it cannot tell whether they are mandatory.

## 8. Effects, owners and the module boundary

The contract module imports the standard library only. It MUST NOT import a consumer's internal packages, a command package or credential code, and its declared Go baseline MUST NOT be newer than either Go consumer's (the desk tools and `statusgen`), which require it by a repository-relative replacement the way they require `forgeread`. Non-Go callers speak the JSON documents of the schema through a command adapter; a Go caller does not shell out to cross the module boundary.

Effects plug in through ports, each reporting what it observed:

- **Effects** creates, observes and removes the checkout for one record at one generation, re-verifying path, Git common directory and administration identity at effect time; it refuses with `identity-mismatch` or `foreign-state` rather than overwrite.
- **Journal** stores records under a per-(cell, repository) exclusive lock: `busy` when the lock cannot be taken within its bound, `stale-generation` when the stored generation is not the expected one.
- **Leases** asks the existing process owner whether a lease is `alive`, `ended` or `unknown`; unknown retains the workspace.
- **Snapshots** asks the existing scratch owner for a metadata-free snapshot of an exact commit, for a check that needs immutable files rather than a Git checkout.

An error from a port that is not a typed refusal is `unknown`, never success. None of the ports carries or mints a credential; transport and role identity stay at their existing boundary.

## 9. Caller dispositions

Every production site that creates, removes, prunes, recursively copies or recursively deletes a checkout, delegates that to the existing owner, or emits a recipe telling an agent or operator to do it, has exactly one disposition:

- **managed** — the site becomes a request for a workspace of the matching kind; its inline Git effect is deleted.
- **snapshot** — the site needs immutable files of an exact commit, and reads a scratch snapshot instead of a checkout.
- **temporary-compatibility** — the site keeps its behaviour until its owner step lands, with a stated retirement condition. A site outside any cell (for example a CI runner) says why it stays outside. A site is never outside a cell merely because it writes under a temporary directory.

The census ([`docs/workspace-callers.md`](../docs/workspace-callers.md)) records every site with its source revision, disposition, owner step and retirement condition. It is checked against the real tree: a discovered site without a row, or a row whose site no longer exists, fails `TestWorkspaceCallerCensus`. A missing tree or census is a could-not-check run, never an empty pass.

## 10. Conformance

`workspace/testdata/contract/` holds the conformance fixtures: valid requests (one per owner scope; one non-canonically spaced; one carrying `<`, `>`, `&`, `"`, `\`, a non-ASCII character and U+2028 in its identifiers and paths, in canonical form and as a spaced twin that escapes every one of those characters), a record, one result per outcome family, a changed-payload replay, and negative documents each paired in `expected.json` with the reason it MUST produce. The expected payload digests and repository keys in `expected.json` are computed independently of the implementation (a SHA-256 of the fixture bytes and of `forge NUL repository`). A conforming implementation, in any language, decodes every fixture to those identities and refuses every negative fixture with its named reason. `TestWorkspaceSchemaCoverage` holds the published schema and the Go types to the same members, required set and closed value sets.
