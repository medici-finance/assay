package fixture

// Beta references nothing declared in alpha.go — that independence is the whole
// content of this fixture. Adding a call to Alpha() here flips the paired
// control red, which is the cheapest way to confirm the check still fires.
func Beta() int { return 2 }
