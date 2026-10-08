// Package config resolves the desk's tunable knobs: budgets, rates, fault guards, lifecycle
// periods, leases, timeouts and counters. It is the only package in deskcore that knows a knob's
// name, its environment variable, its default or its bounds; everything else asks a Resolved
// value for a knob by name.
//
// Resolution is pure. Resolve takes the environment as a map and the operator's config file as
// already-decoded data, so this package imports neither os nor any file or network package and
// gives the same answer for the same inputs.
//
// Precedence is fixed: an environment variable ASSAY_DESK_<KNOB> beats the config file's knobs
// section, which beats the compiled default. There is no command-line flag layer. Every value
// records where it came from, and a value outside its knob's bounds is refused, naming the knob,
// the value and the layer it came from, rather than clamped.
//
// Authority is not a knob. Which operations a role may request, on which targets and hosts, is
// approved policy data (see the policy package); no setting here can widen it. Budgets and rates
// can only limit, they default to 0 (off), and they are counted per role.
package config

import (
	"fmt"
	"strings"
	"time"
)

// Kind is the type of a knob's value.
type Kind int

const (
	// Count is a non-negative integer.
	Count Kind = iota
	// Duration is a time span, written with an explicit unit ("90s", "15m", "2h", "90d").
	Duration
)

// Class says what a knob governs. It decides whether 0 means off and whether a request may
// narrow the value.
type Class string

// Knob classes.
const (
	ClassBudget       Class = "budget"
	ClassBudgetWindow Class = "budget window"
	ClassRate         Class = "rate"
	ClassFaultGuard   Class = "fault guard"
	ClassEscalation   Class = "escalation threshold"
	ClassCapacity     Class = "capacity"
	ClassLifecycle    Class = "lifecycle"
	ClassLease        Class = "lease"
	ClassStorage      Class = "storage"
	ClassRead         Class = "read"
	ClassTimeout      Class = "timeout"
	ClassCounter      Class = "counter"
)

// Knob describes one tunable value. For a Duration knob, Default, Min and Max are nanoseconds
// (a time.Duration converted to int64).
type Knob struct {
	Name    string
	Class   Class
	Kind    Kind
	Unit    string // for a Count knob: what is counted
	Default int64
	Min     int64
	Max     int64
	// PerRole knobs may be set for one role and are read for one role; their counters are keyed
	// per role (see CounterKey).
	PerRole bool
	// V1 is the integer literal the earlier, pre-deskcore tools hard-coded for this setting, or 0
	// when they had none. The archtest knob-read lint looks for it outside this package, so a
	// hard-coded copy of a knob cannot creep back in beside the resolver.
	V1  int64
	Doc string
}

const day = 24 * time.Hour

func dur(d time.Duration) int64 { return int64(d) }

// limitClasses are the classes whose zero value means "off" (no limit) and which a request may
// narrow but never widen.
func (k Knob) limitClass() bool { return k.Class == ClassBudget || k.Class == ClassRate }

// narrowable classes: a smaller value is always the more conservative one.
func (k Knob) narrowable() bool { return k.limitClass() || k.Class == ClassCapacity }

// v1Table is the compiled table. Order is display order.
var v1Table = []Knob{
	{Name: "issue.file.rate", Class: ClassBudget, Kind: Count, Unit: "issues", Default: 0, Min: 0, Max: 10000, PerRole: true, V1: 3,
		Doc: "issues one role may file per issue.file.window; 0 = off"},
	{Name: "issue.file.window", Class: ClassBudgetWindow, Kind: Duration, Default: dur(24 * time.Hour), Min: dur(time.Hour), Max: dur(30 * day), PerRole: true, V1: 24,
		Doc: "the window issue.file.rate is counted over"},
	{Name: "write.rate.per_pr_hour", Class: ClassRate, Kind: Count, Unit: "writes", Default: 0, Min: 0, Max: 10000, PerRole: true, V1: 20,
		Doc: "writes one role may make to one change request per hour; 0 = off"},
	{Name: "write.rate.per_repo_hour", Class: ClassRate, Kind: Count, Unit: "writes", Default: 0, Min: 0, Max: 10000, PerRole: true, V1: 100,
		Doc: "writes one role may make to one repository per hour; 0 = off"},
	{Name: "evidence.unnumbered_cap", Class: ClassBudget, Kind: Count, Unit: "entries", Default: 0, Min: 0, Max: 10000, PerRole: true, V1: 30,
		Doc: "unnumbered evidence entries one role may land per run; 0 = off"},
	{Name: "autolane.daily_cap", Class: ClassBudget, Kind: Count, Unit: "merges", Default: 0, Min: 0, Max: 10000, PerRole: true, V1: 100,
		Doc: "automatic-lane merges one role may perform per day; 0 = off"},
	{Name: "write.breaker.trip", Class: ClassFaultGuard, Kind: Count, Unit: "consecutive failures", Default: 5, Min: 1, Max: 100, V1: 5,
		Doc: "consecutive write failures that open the write breaker"},
	{Name: "write.breaker.cooldown", Class: ClassFaultGuard, Kind: Duration, Default: dur(15 * time.Minute), Min: dur(time.Minute), Max: dur(24 * time.Hour), V1: 15,
		Doc: "how long an open write breaker stays open"},
	{Name: "review.round_cap", Class: ClassEscalation, Kind: Count, Unit: "review rounds", Default: 3, Min: 1, Max: 20, V1: 3,
		Doc: "review rounds on one change before it escalates to a human"},
	{Name: "pool.width", Class: ClassCapacity, Kind: Count, Unit: "concurrent attempts", Default: 1, Min: 0, Max: 16, PerRole: true,
		Doc: "concurrent attempts one role may run; set per role"},
	{Name: "token.github.refresh_margin", Class: ClassLifecycle, Kind: Duration, Default: dur(10 * time.Minute), Min: dur(time.Minute), Max: dur(50 * time.Minute), V1: 50,
		Doc: "how long before expiry an installation token is refreshed"},
	{Name: "key.github.max_age", Class: ClassLifecycle, Kind: Duration, Default: dur(90 * day), Min: dur(day), Max: dur(730 * day),
		Doc: "age at which an application private key is reported for rotation"},
	{Name: "token.gitlab.pat_days", Class: ClassLifecycle, Kind: Count, Unit: "days", Default: 7, Min: 1, Max: 365, V1: 7,
		Doc: "lifetime requested for a minted access token"},
	{Name: "token.gitlab.expiry_warn", Class: ClassLifecycle, Kind: Duration, Default: dur(48 * time.Hour), Min: dur(time.Hour), Max: dur(30 * day),
		Doc: "how long before expiry an access token is reported"},
	{Name: "token.gitlab.in_use_window", Class: ClassLifecycle, Kind: Duration, Default: dur(day), Min: dur(time.Hour), Max: dur(30 * day),
		Doc: "how recently a token must have been used to count as in use"},
	{Name: "lease.ttl", Class: ClassLease, Kind: Duration, Default: dur(2 * time.Hour), Min: dur(time.Minute), Max: dur(24 * time.Hour), V1: 20,
		Doc: "how long a claim lease lives without renewal"},
	{Name: "lease.renew_every", Class: ClassLease, Kind: Duration, Default: dur(10 * time.Minute), Min: dur(time.Minute), Max: dur(12 * time.Hour),
		Doc: "how often a held lease is renewed; must be shorter than lease.ttl"},
	{Name: "records.retention", Class: ClassStorage, Kind: Duration, Default: dur(90 * day), Min: dur(day), Max: dur(3650 * day),
		Doc: "how long operation records are kept"},
	{Name: "forge.page_size", Class: ClassRead, Kind: Count, Unit: "items", Default: 100, Min: 1, Max: 100, V1: 100,
		Doc: "items requested per forge listing page"},
	{Name: "timeout.forge", Class: ClassTimeout, Kind: Duration, Default: dur(30 * time.Second), Min: dur(time.Second), Max: dur(10 * time.Minute), V1: 30,
		Doc: "deadline for one forge call"},
	{Name: "timeout.callout", Class: ClassTimeout, Kind: Duration, Default: dur(5 * time.Second), Min: dur(time.Second), Max: dur(10 * time.Minute), V1: 5,
		Doc: "deadline for one external callout"},
	{Name: "timeout.decide", Class: ClassTimeout, Kind: Duration, Default: dur(30 * time.Second), Min: dur(time.Second), Max: dur(10 * time.Minute), V1: 30,
		Doc: "deadline for one policy decision"},
	{Name: "timeout.hook", Class: ClassTimeout, Kind: Duration, Default: dur(60 * time.Second), Min: dur(time.Second), Max: dur(10 * time.Minute), V1: 60,
		Doc: "deadline for one hook run"},
	{Name: "policy.window", Class: ClassCounter, Kind: Duration, Default: dur(7 * day), Min: dur(day), Max: dur(90 * day),
		Doc: "the window policy counters are kept over"},
	{Name: "policy.stale_after", Class: ClassCounter, Kind: Count, Unit: "windows", Default: 4, Min: 1, Max: 52,
		Doc: "policy.window periods without a hit before a rule is reported stale"},
}

// Table returns a copy of the compiled knob table.
func Table() []Knob { return append([]Knob(nil), v1Table...) }

// EnvName is the environment variable that sets k for every role.
func (k Knob) EnvName() string { return envPrefix + upperUnderscore(k.Name) }

// RoleEnvName is the environment variable that sets a per-role knob for one role.
func (k Knob) RoleEnvName(role string) string {
	return k.EnvName() + roleSep + upperUnderscore(role)
}

const (
	envPrefix = "ASSAY_DESK_"
	roleSep   = "__"
)

func upperUnderscore(s string) string {
	return strings.NewReplacer(".", "_", "-", "_").Replace(strings.ToUpper(s))
}

// format renders v in k's unit.
func (k Knob) format(v int64) string {
	if k.Kind == Count {
		return fmt.Sprintf("%d", v)
	}
	d := time.Duration(v)
	switch {
	case d != 0 && d%day == 0:
		return fmt.Sprintf("%dd", d/day)
	case d != 0 && d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d != 0 && d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", d/time.Second)
	}
	return d.String()
}

// validateTable checks the table itself: unique names and variables, a separator-free variable
// for every knob, and every default within its bounds.
func validateTable(table []Knob) error {
	names := map[string]bool{}
	envs := map[string]string{}
	for _, k := range table {
		if k.Name == "" || names[k.Name] {
			return fmt.Errorf("config table: knob name %q is empty or repeated", k.Name)
		}
		names[k.Name] = true
		env := k.EnvName()
		if other, dup := envs[env]; dup {
			return fmt.Errorf("config table: knobs %q and %q share variable %s", other, k.Name, env)
		}
		envs[env] = k.Name
		if strings.Contains(env, roleSep) {
			return fmt.Errorf("config table: variable %s for knob %q contains the role separator %q", env, k.Name, roleSep)
		}
		if k.Min > k.Max || k.Default < k.Min || k.Default > k.Max {
			return fmt.Errorf("config table: knob %q default %s is outside its bounds [%s, %s]", k.Name, k.format(k.Default), k.format(k.Min), k.format(k.Max))
		}
		if k.limitClass() && k.Default != 0 {
			return fmt.Errorf("config table: %s knob %q must default to 0 (off)", k.Class, k.Name)
		}
		if k.limitClass() && !k.PerRole {
			return fmt.Errorf("config table: %s knob %q must be counted per role", k.Class, k.Name)
		}
	}
	return nil
}
