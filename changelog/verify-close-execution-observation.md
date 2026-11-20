### Fixed

- The verify-runner image seeds its checkout beneath the work volume so the non-root init container can preserve the checkout metadata on a kubelet-owned volume.
- `statusgen verifyrun --close-bracket --kubernetes-observation <digest>` seals a host-observed execution digest with the validated witness. A container-supplied observation is refused.
