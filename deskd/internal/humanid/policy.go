// Package humanid owns the immutable, non-secret startup identity policy.
// It performs no I/O and holds no credential. Empty configuration disables
// human operations. Only the command startup initializes the process policy.
package humanid

import (
	"errors"
	"regexp"
	"strings"
	"sync"
)

var ErrDisabled = errors.New("human operations disabled: supply --human-login at startup")
var loginRE = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)

type policy struct {
	mu          sync.RWMutex
	initialized bool
	login       string
}

var startup policy

// Configure freezes the process policy once. Invalid input is refused without
// changing it; a second initialization cannot replace an existing policy.
func Configure(login string) error { return startup.configure(login) }
func (p *policy) configure(login string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		return errors.New("human identity policy already initialized")
	}
	if login != "" && (len(login) > 39 || !loginRE.MatchString(login)) {
		return errors.New("invalid --human-login: expected a GitHub login")
	}
	p.login = login
	p.initialized = true
	return nil
}
func (p *policy) get() string { p.mu.RLock(); defer p.mu.RUnlock(); return p.login }

// Login returns the explicitly configured principal, or empty when disabled.
func Login() string { return startup.get() }

// Enabled reports whether startup explicitly enabled human operations.
func Enabled() bool { return Login() != "" }

// Matches never treats an empty caller as an authorized identity.
func Matches(login string) bool {
	expected := Login()
	return expected != "" && strings.TrimSpace(login) != "" && strings.EqualFold(strings.TrimSpace(login), expected)
}
