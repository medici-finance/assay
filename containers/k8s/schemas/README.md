# Offline Kubernetes schemas

Unmodified strict standalone schemas for Kubernetes **v1.34.1** from
[yannh/kubernetes-json-schema](https://github.com/yannh/kubernetes-json-schema/tree/e6b2bfea83d04b8cd25276f7db6f0a464efd9f75/v1.34.1-standalone-strict),
commit `e6b2bfea83d04b8cd25276f7db6f0a464efd9f75` (schema update 2025-09-11).
The upstream licence is retained in LICENSE. Only Namespace, ServiceAccount,
StatefulSet and NetworkPolicy are cached, matching the base's complete kind set.
`checksums.json` records their SHA-256 hashes; `check.sh` verifies all four before
validation and refuses external `$ref` values. Kubeconform uses these local files
exclusively, so missing schemas fail rather than fetching or skipping resources.

Upgrades are an explicit preparation change: assess the new version, fetch these
four upstream files at an immutable revision, retain the licence, recompute
checksums and change the Kubernetes version in the checker. Review the schema
and checksum diff together and rerun the hostile schema fixture. The runtime
checker never downloads schemas. These schemas establish API shape, not admission
controller results or compatibility with an uninspected cluster.
