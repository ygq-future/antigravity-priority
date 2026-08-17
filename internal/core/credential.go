package core

import "encoding/json"

// Provider indicates the upstream capability domain for credentials.
type Provider string

const (
	// ProviderUnknown represents an unrecognized provider.
	ProviderUnknown Provider = "unknown"
	// ProviderAntigravity represents Google Antigravity.
	ProviderAntigravity Provider = "antigravity"
)

// ModelGroup indicates the upstream independent quota measurement unit in Antigravity.
type ModelGroup string

const (
	// ModelGroupGemini represents Gemini models quota group.
	ModelGroupGemini ModelGroup = "gemini"
	// ModelGroupClaudeGPT represents Claude and GPT models quota group.
	ModelGroupClaudeGPT ModelGroup = "claude_gpt"
)

// CredentialType indicates the auth-file type.
type CredentialType string

const (
	// CredentialTypeUnknown represents an unknown credential type.
	CredentialTypeUnknown CredentialType = "unknown"
	// CredentialTypeAntigravity represents an Antigravity credential type.
	CredentialTypeAntigravity CredentialType = "antigravity"
)

// CredentialStatus indicates the host-side credential status.
type CredentialStatus string

const (
	// CredentialStatusUnknown represents an unknown status.
	CredentialStatusUnknown CredentialStatus = "unknown"
	// CredentialStatusActive represents an active credential status.
	CredentialStatusActive CredentialStatus = "active"
	// CredentialStatusInactive represents an inactive credential status.
	CredentialStatusInactive CredentialStatus = "inactive"
)

// PlanType indicates the subscription plan discovered during probing.
type PlanType string

const (
	// PlanTypeUnknown represents an unknown plan.
	PlanTypeUnknown PlanType = "unknown"
	// PlanTypeFree represents a free tier plan.
	PlanTypeFree PlanType = "free"
	// PlanTypePlus represents a plus tier plan.
	PlanTypePlus PlanType = "plus"
	// PlanTypePro represents a pro tier plan.
	PlanTypePro PlanType = "pro"
	// PlanTypeTeam represents a team tier plan.
	PlanTypeTeam PlanType = "team"
)

// Freshness indicates whether the quota evidence comes from a fresh probe.
type Freshness string

const (
	// FreshnessUnknown represents no probe evidence available yet.
	FreshnessUnknown Freshness = "unknown"
	// FreshnessFresh represents evidence obtained from the current scheduling round.
	FreshnessFresh Freshness = "fresh"
	// FreshnessStale represents historical evidence that cannot directly promote.
	FreshnessStale Freshness = "stale"
)

// ProbeStatus indicates the prober status determination for the credential.
type ProbeStatus string

const (
	// ProbeStatusUnknown represents probing not yet performed.
	ProbeStatusUnknown ProbeStatus = "unknown"
	// ProbeStatusReady represents successful probe yielding valid quota evidence.
	ProbeStatusReady ProbeStatus = "ready"
	// ProbeStatusUnsupported represents an unsupported provider or configuration.
	ProbeStatusUnsupported ProbeStatus = "unsupported"
)

// CanPromote represents whether a credential is eligible for automatic priority promotion.
type CanPromote bool

const (
	// CannotPromote indicates the credential must not be promoted.
	CannotPromote CanPromote = false
	// CanPromoteAfterFreshProbe indicates the credential has fresh probe evidence and can be promoted.
	CanPromoteAfterFreshProbe CanPromote = true
)

// Credential is the domain snapshot of a host auth file.
type Credential struct {
	Name            string
	AuthIndex       string
	Provider        Provider
	Type            CredentialType
	Status          CredentialStatus
	Disabled        bool
	Unavailable     bool
	Priority        int
	PriorityMissing bool
	Account         string
	Email           string
	PlanType        PlanType
	Freshness       Freshness
	ProbeStatus     ProbeStatus
	RawJSON         json.RawMessage
}

// WithProbe returns a copy of the credential with updated probe metadata.
func (c Credential) WithProbe(freshness Freshness, probeStatus ProbeStatus) Credential {
	c.Freshness = freshness
	c.ProbeStatus = probeStatus
	return c
}

// PromotionFromProbe determines whether probe metadata allows automatic priority promotion.
func PromotionFromProbe(freshness Freshness, probeStatus ProbeStatus) CanPromote {
	if freshness == FreshnessFresh && probeStatus == ProbeStatusReady {
		return CanPromoteAfterFreshProbe
	}
	return CannotPromote
}
