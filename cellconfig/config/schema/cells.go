// Package schema defines and validates the cells.yaml config format: the
// single answer every desk-console pods/deskd brief reads for "what is a
// cell, which loops run it, on what schedule, with which model ladder."
//
// Source of truth: docs/desk-console-design.md §5.1 (cells.yaml sketch),
// §9.2 (loop roles + model ladders) and §13.8/open-question-3 (the §13.3
// notification triad ruling). See docs/streams/desk-console/brief-02-cell-config-schema.md.
package schema

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// LoopRole is a per-cell CronJob loop role. §9.2 lists review, verify,
// dispatch and metrics as the loop roles this stream builds (briefs 05-07);
// "inbound" is reserved for a later stream and deliberately not accepted
// here yet — widen this set only when a brief actually consumes it.
type LoopRole string

const (
	RoleReview   LoopRole = "review"
	RoleVerify   LoopRole = "verify"
	RoleDispatch LoopRole = "dispatch"
	RoleMetrics  LoopRole = "metrics"
)

var validRoles = map[LoopRole]bool{
	RoleReview:   true,
	RoleVerify:   true,
	RoleDispatch: true,
	RoleMetrics:  true,
}

// NotificationTier is one leg of the §13.3 ambient-agents triad
// (notify / question / review) that decision-model escalations map onto.
type NotificationTier string

const (
	TierReview   NotificationTier = "review"   // gold gates — always push
	TierQuestion NotificationTier = "question" // collect silently — badge only
	TierNotify   NotificationTier = "notify"   // FYI stream — no badge
)

// PushBehavior is what a notification tier resolves to on the menubar/Slack
// push path (brief-11).
type PushBehavior string

const (
	PushAlways PushBehavior = "push"  // always push (menubar + Slack)
	PushBadge  PushBehavior = "badge" // badge only, no push
	PushNone   PushBehavior = "none"  // no badge, FYI only
)

var validPushBehaviors = map[PushBehavior]bool{
	PushAlways: true,
	PushBadge:  true,
	PushNone:   true,
}

// Loop is one CronJob-per-role-per-cell entry (§9.2).
type Loop struct {
	// Role selects which loop this is; see LoopRole.
	Role LoopRole `yaml:"role"`
	// Schedule is a standard 5-field cron expression.
	Schedule string `yaml:"schedule"`
	// Models is the worker-initiated escalation ladder (§13.8 ruling):
	// cheapest-first, e.g. [deepseek-chat, glm-5.2]. May be empty for a
	// zero-AI, deterministic loop (metrics — brief-05's "zero-AI first
	// mover"); every other role requires at least one model.
	Models []string `yaml:"models"`
	// App is the GitHub App identity name (bot slug, no [bot] suffix)
	// this loop authenticates as — brief-04 names the Secret per role
	// from this field.
	App string `yaml:"app"`
}

// Cell is one entry in cells.yaml — a repo set plus a stream set (design
// doc §3.2: "cell config is data, not code ... a second cell is a config
// entry rather than a fork").
type Cell struct {
	Name              string                            `yaml:"name"`
	Repos             []string                          `yaml:"repos"`
	StreamsRoot       string                            `yaml:"streams_root"`
	Loops             []Loop                            `yaml:"loops"`
	SilenceThreshold  string                            `yaml:"silence_threshold"`
	NotificationTiers map[NotificationTier]PushBehavior `yaml:"notification_tiers"`
}

// File is the top-level cells.yaml document.
type File struct {
	Cells []Cell `yaml:"cells"`
}

// Load reads and strictly validates a cells.yaml file at path: unknown
// fields anywhere in the document are errors (brief-02 task 2), and every
// structural/semantic rule below must hold.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(data)
}

// Parse strictly decodes and validates cells.yaml content.
func Parse(data []byte) (*File, error) {
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // unknown fields are errors
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// Validate checks every semantic rule cells.yaml must satisfy. Structural
// unknown-field rejection already happened during decode (KnownFields).
func (f *File) Validate() error {
	if len(f.Cells) == 0 {
		return fmt.Errorf("cells: at least one cell is required")
	}
	seen := map[string]bool{}
	for i, c := range f.Cells {
		if err := c.validate(); err != nil {
			return fmt.Errorf("cells[%d] (%s): %w", i, c.Name, err)
		}
		if seen[c.Name] {
			return fmt.Errorf("cells[%d]: duplicate cell name %q", i, c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

func (c *Cell) validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(c.Repos) == 0 {
		return fmt.Errorf("repos: at least one repo is required")
	}
	for _, r := range c.Repos {
		if r == "" {
			return fmt.Errorf("repos: entries must not be empty")
		}
	}
	if c.StreamsRoot == "" {
		return fmt.Errorf("streams_root is required")
	}
	if len(c.Loops) == 0 {
		return fmt.Errorf("loops: at least one loop is required")
	}
	seenRoles := map[LoopRole]bool{}
	for i, l := range c.Loops {
		if err := l.validate(); err != nil {
			return fmt.Errorf("loops[%d]: %w", i, err)
		}
		if seenRoles[l.Role] {
			return fmt.Errorf("loops[%d]: duplicate role %q for cell %q", i, l.Role, c.Name)
		}
		seenRoles[l.Role] = true
	}
	if c.SilenceThreshold == "" {
		return fmt.Errorf("silence_threshold is required")
	}
	if _, err := time.ParseDuration(c.SilenceThreshold); err != nil {
		return fmt.Errorf("silence_threshold: %w", err)
	}
	if err := validateNotificationTiers(c.NotificationTiers); err != nil {
		return err
	}
	return nil
}

func (l *Loop) validate() error {
	if l.Role == "" {
		return fmt.Errorf("role is required")
	}
	if !validRoles[l.Role] {
		return fmt.Errorf("role %q is not a recognised loop role", l.Role)
	}
	if l.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}
	if err := validateCronFields(l.Schedule); err != nil {
		return fmt.Errorf("schedule: %w", err)
	}
	// Zero-AI loops (metrics — brief-05) may ship an empty ladder;
	// every other role needs at least one model to escalate from.
	if l.Role != RoleMetrics && len(l.Models) == 0 {
		return fmt.Errorf("models: at least one model is required for role %q", l.Role)
	}
	if l.App == "" {
		return fmt.Errorf("app is required")
	}
	return nil
}

// validateNotificationTiers enforces the §13.3 triad exactly: all three
// keys (review, question, notify) present, no extras, values from the
// known push-behavior set.
func validateNotificationTiers(m map[NotificationTier]PushBehavior) error {
	if m == nil {
		return fmt.Errorf("notification_tiers is required")
	}
	want := []NotificationTier{TierReview, TierQuestion, TierNotify}
	if len(m) != len(want) {
		return fmt.Errorf("notification_tiers: must have exactly the triad %v, got %d entries", want, len(m))
	}
	for _, tier := range want {
		behavior, ok := m[tier]
		if !ok {
			return fmt.Errorf("notification_tiers: missing tier %q", tier)
		}
		if !validPushBehaviors[behavior] {
			return fmt.Errorf("notification_tiers[%q]: unrecognised push behavior %q", tier, behavior)
		}
	}
	return nil
}

// validateCronFields does a light structural check (5 whitespace-separated
// fields) — not a full cron-expression parse, which is out of scope for
// this brief.
func validateCronFields(schedule string) error {
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return fmt.Errorf("expected 5 cron fields (min hour dom month dow), got %d in %q", len(fields), schedule)
	}
	return nil
}
