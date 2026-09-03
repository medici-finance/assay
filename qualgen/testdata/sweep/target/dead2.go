package target

// alsoDead is the second planted dead-code suspect, introduced on the second
// sweep run to exercise standing-lane incrementality (a NEW fingerprint).
func alsoDead() int {
	return 42
}
