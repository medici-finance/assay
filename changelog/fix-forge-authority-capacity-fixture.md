### Fixed
- The forge-local authority test fixture now returns only once its own startup reads have released every data slot, so `TestForgeLocalAuthorityControlCapacity` no longer flakes when a slow dial lands while a startup read still holds a slot.
