package reviewflow

// ReviewContext describes one preparation, never a dispatch grant. Credentialed
// adapters hold the data and call the existing round/packet owners. No persistent
// cache or mutable-state authority is carried by this value.
type ReviewContext struct {
	Subject        ChangeIdentity
	AdapterVersion string
	Advisory       *ObservedChange // untrusted source attribution; never substitutes for live admission facts
}

// SourceKey binds immutable bytes to their interpretation and provider identity.
// Comparison keys additionally include the exact second endpoint in Base.
type SourceKey struct {
	Repository                Repository
	Head, Base, Path, Version string
}

func (c ReviewContext) SourceKey(path string) SourceKey {
	return SourceKey{Repository: c.Subject.Repository, Head: c.Subject.Head, Base: c.Subject.Base, Path: path, Version: c.AdapterVersion}
}
