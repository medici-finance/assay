package trust

import (
	"strconv"
	"strings"
)

// KnownRosterKeys is the shared ASSAY namespace schema, independent of source selection.
func KnownRosterKeys() []string {
	return []string{
		"ASSAY_USAGE_DIR",
		"ASSAY_ALLOWED_REPOS",
		"ASSAY_ALLOW_CLUSTER",
		"ASSAY_ASSERTED_HUMANS",
		"ASSAY_AUTHORIZED_AUTHORS",
		"ASSAY_AUTOAPPROVE_AREAS",
		"ASSAY_AUTOAPPROVE_DAILY_CAP",
		"ASSAY_AUTOAPPROVE_EJECT_LINE",
		"ASSAY_AUTOAPPROVE_FPY_FLOOR",
		"ASSAY_AUTOAPPROVE_SIGNOFF_THREAD",
		"ASSAY_BLESS_LOGIN",
		"ASSAY_CHANNEL_DRIFT_TARGET",
		"ASSAY_CLAIM_DIR",
		"ASSAY_CLAIM_SINGLE_HOST",
		"ASSAY_CLAIM_STORE",
		"ASSAY_CONTRIBUTOR_LEDGER",
		"ASSAY_CRITICAL_STAMP_AUTHORITIES",
		"ASSAY_DETERMINISTIC_GATE_PATTERNS",
		"ASSAY_FORMER_HUMAN_LOGIN_MAP",
		"ASSAY_GITLAB_DISPLAY_NAMES",
		"ASSAY_GITLAB_ROLE_EMAILS",
		"ASSAY_GITLAB_SESSION_EMAILS",
		"ASSAY_HOME_REPO",
		"ASSAY_HUMAN_LOGIN_MAP",
		"ASSAY_OS_USER_LOGIN_MAP",
		"ASSAY_OUTBOUND_CALLOUT",
		"ASSAY_OUTBOUND_CALLOUT_REQUIRED",
		"ASSAY_OUTBOUND_CALLOUT_TIMEOUT",
		"ASSAY_RELEASE_REPO",
		"ASSAY_REPO_ALIASES",
		"ASSAY_REPO_FORGES",
		"ASSAY_REVIEWER_VENDOR",
		"ASSAY_RISK_CALLOUT",
		"ASSAY_RISK_PATH_TRIGGERS_EXTRA",
		"ASSAY_ROSTER_SCHEMA",
		"ASSAY_RUN_CREDENTIALS",
		"ASSAY_SCAN_REPOS",
		"ASSAY_STAMP_TRUSTED_LOGINS",
		"ASSAY_STREAM_CAP",
		"ASSAY_SWEEP_WITHHELD_STREAMS",
		"ASSAY_TRUSTED_BOT_SLUGS",
		"ASSAY_TRUSTED_LOGINS",
		"ASSAY_VERIFIER_VENDOR",
		"ASSAY_WITHHELD_IDENTIFIERS",
		"ASSAY_WRITEGUARD_CALLOUT",
	}
}

type ForgeProfile uint8

const (
	KnownForges ForgeProfile = iota
	ObserveUnknownForge
)

type BotIdentity struct {
	Forge, Slug string
	ID          int64
	Inferred    bool
}

func ExplicitForge(head, rest string) bool {
	if head == "" {
		return false
	}
	if _, e := strconv.ParseInt(head, 10, 64); e == nil {
		return false
	}
	return strings.Contains(rest, ":")
}

// ParseBot preserves the reader's explicit unknown-forge observation profile.
func ParseBot(entry string, profile ForgeProfile) (BotIdentity, bool) {
	if profile != KnownForges && profile != ObserveUnknownForge {
		return BotIdentity{}, false
	}
	b := BotIdentity{Forge: "github", Inferred: true}
	head, rest, found := strings.Cut(entry, ":")
	h := strings.ToLower(strings.TrimSpace(head))
	if found && (h == "github" || h == "gitlab" || (profile == ObserveUnknownForge && ExplicitForge(h, rest))) {
		b.Forge = h
		b.Inferred = false
		entry = rest
	}
	slug, id, ok := SplitIdentity(entry)
	if !ok {
		return BotIdentity{}, false
	}
	b.Slug = slug
	b.ID = id
	return b, true
}
func AcceptedLogins(forge, slug string) []string {
	switch forge {
	case "github":
		return []string{slug + "[bot]", "app/" + slug}
	case "gitlab":
		return []string{slug}
	default:
		return nil
	}
}
