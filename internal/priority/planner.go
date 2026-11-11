package priority

import (
	"fmt"
	"slices"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
)

const (
	// Decision Reasons
	ReasonKeepCurrentState         = "keep current state"
	ReasonDisabledOnHost           = "disabled on host"
	ReasonFreshWeeklyDepleted      = "fresh weekly depleted"
	ReasonFreshShortWindowDepleted = "fresh short window depleted"
	ReasonFreshBoosted             = "fresh boosted"
	ReasonFreshRemainingPositive   = "fresh remaining positive"
	Reason429Cooldown              = "429 rate limit cooldown"
	ReasonServerErrorCooldown      = "5xx server error cooldown"
	ReasonForbiddenCooldown        = "403 forbidden cooldown"
	ReasonUnauthorizedCooldown     = "401 unauthorized cooldown"
)

// WeeklyRecoveryThreshold is the minimum 7d quota remaining required to re-enable
// an account that was automatically disabled due to weekly depletion.
const WeeklyRecoveryThreshold = 0.10

// QuotaEvidence represents a validated quota observation. It intentionally
// contains no freshness or probe-status flags: only the Evidence authority
// decides whether an instance belongs to the Fresh or Historical set supplied
// to the planner.
type QuotaEvidence struct {
	AuthIndex            string
	Provider             core.Provider
	ModelGroup           config.AntigravityModelGroup
	ObservedAt           time.Time
	ResetAt              *time.Time
	Remaining            *int64
	ShortWindowResetAt   *time.Time
	ShortWindowRemaining *int64
	LongWindowResetAt    *time.Time
	LongWindowRemaining  *int64
	ArgonWindowResetAt   *time.Time
	ArgonWindowRemaining *int64
	PlanType             core.PlanType
	CycleBurnRate        float64
}

// EvidenceSource is the narrow contract Planner accepts from the Evidence
// authority. Raw probe results and persisted observations cannot be supplied
// directly as fresh planning input.
type EvidenceSource interface {
	FreshQuotaEvidence() []QuotaEvidence
	HistoricalQuotaEvidence() []QuotaEvidence
}

type plannerEvidence struct {
	Fresh      []QuotaEvidence
	Historical []QuotaEvidence
}

func (evidence plannerEvidence) FreshQuotaEvidence() []QuotaEvidence {
	return evidence.Fresh
}

func (evidence plannerEvidence) HistoricalQuotaEvidence() []QuotaEvidence {
	return evidence.Historical
}

// Options defines configuration options for the priority planner.
type Options struct {
	Now                 time.Time
	BoostStartPriority  int
	NormalStartPriority int
	FreeStartPriority   int
	MinChange           int
	UrgencyTolerance    float64
	CooldownAuthIndexes map[string]time.Time
	CooldownReasons     map[string]string
	// CooldownReasonsByKey is the pre-filter composite-key form used by
	// projection to derive per-group CooldownReasons.
	CooldownReasonsByKey map[string]string
	CustomTiers          map[string]int
}

// PlanItem represents the calculated target state for a single credential.
type PlanItem struct {
	Credential           core.Credential
	Priority             int
	Disabled             bool
	PlanType             core.PlanType
	ResetAt              *time.Time
	Remaining            *int64
	ShortWindowResetAt   *time.Time
	ShortWindowRemaining *int64
	LongWindowResetAt    *time.Time
	LongWindowRemaining  *int64
	ArgonWindowResetAt   *time.Time
	ArgonWindowRemaining *int64
	Reason               string
	IsBoosted            bool
	Urgency              float64
	R7d                  float64
	T7d                  float64
	R5h                  float64
	T5h                  float64
	CycleBurnRate        float64
	TRequired            float64
	hasQuotaEvidence     bool
	quotaState           quotaState
	CustomTier           int
}

type quotaState uint8

const (
	quotaStateUnknown quotaState = iota
	quotaStateHealthy
	quotaStateShortDepleted
	quotaStateWeeklyDepleted
)

// Change represents a credential whose planned target differs from its current
// Host state. It drives display and diagnostics only; the planner never writes
// to the Host.
type Change struct {
	Credential core.Credential
	Priority   int
	Disabled   bool
	Reason     string
	IsBoosted  bool
}

// Plan encapsulates the complete immutable priority scheduling decision and change set.
type Plan struct {
	DecidedAt time.Time
	Items     []PlanItem
	Changes   []Change
}

// PlanWithEvidence produces a plan from the Evidence authority's two
// semantically separate sets. Fresh probe observations take precedence over
// persisted historical ones; neither set authorizes a Host write because the
// plan only drives in-memory routing and display.
func PlanWithEvidence(credentials []core.Credential, source EvidenceSource, options Options) Plan {
	if options.Now.IsZero() {
		panic("priority: explicit decision time is required")
	}
	normalizedOptions := normalizeOptions(options)
	evidence := plannerEvidence{}
	if source != nil {
		evidence = plannerEvidence{
			Fresh:      source.FreshQuotaEvidence(),
			Historical: source.HistoricalQuotaEvidence(),
		}
	}
	return planWithEvidence(credentials, evidence, normalizedOptions)
}

func planWithEvidence(credentials []core.Credential, evidence plannerEvidence, options Options) Plan {
	items := planItems(credentials, evidence, options)
	planPositive(items, options)
	SortPlanItems(items)
	return Plan{
		DecidedAt: options.Now.UTC(),
		Items:     items,
		Changes:   changes(items, options),
	}
}

func normalizeOptions(options Options) Options {
	options.Now = options.Now.UTC()
	if options.BoostStartPriority <= 0 {
		options.BoostStartPriority = DefaultBoostStartPriority
	}
	if options.BoostStartPriority > MaxPriority {
		options.BoostStartPriority = MaxPriority
	}
	if options.NormalStartPriority <= 0 {
		options.NormalStartPriority = DefaultNormalStartPriority
	}
	if options.NormalStartPriority > MaxPriority {
		options.NormalStartPriority = MaxPriority
	}
	if options.FreeStartPriority <= 0 {
		options.FreeStartPriority = DefaultFreeStartPriority
	}
	if options.FreeStartPriority > MaxPriority {
		options.FreeStartPriority = MaxPriority
	}
	if options.MinChange < 0 {
		options.MinChange = 0
	}
	return options
}

func planItems(credentials []core.Credential, evidence plannerEvidence, options Options) []PlanItem {
	freshByAuthIndex := quotaEvidenceByAuthIndex(evidence.Fresh)
	historicalByAuthIndex := quotaEvidenceByAuthIndex(evidence.Historical)
	items := make([]PlanItem, len(credentials))
	for index, credential := range credentials {
		item := PlanItem{
			Credential: credential,
			Priority:   credential.Priority,
			Disabled:   credential.Disabled,
			PlanType:   credential.PlanType,
			Reason:     ReasonKeepCurrentState,
			CustomTier: options.CustomTiers[credential.AuthIndex],
		}

		evidence, hasEvidence := freshByAuthIndex[credential.AuthIndex]
		if !hasEvidence {
			evidence, hasEvidence = historicalByAuthIndex[credential.AuthIndex]
		}
		if hasEvidence {
			item.hasQuotaEvidence = true
			if evidence.PlanType != "" && evidence.PlanType != core.PlanTypeUnknown {
				item.PlanType = evidence.PlanType
			}
			item.ResetAt = evidence.ResetAt
			item.Remaining = evidence.Remaining
			item.ShortWindowResetAt = evidence.ShortWindowResetAt
			item.ShortWindowRemaining = evidence.ShortWindowRemaining
			item.LongWindowResetAt = evidence.LongWindowResetAt
			item.LongWindowRemaining = evidence.LongWindowRemaining
			item.ArgonWindowResetAt = evidence.ArgonWindowResetAt
			item.ArgonWindowRemaining = evidence.ArgonWindowRemaining
			item.CycleBurnRate = evidence.CycleBurnRate

			metrics := ExtractQuotaMetrics(evidence, options.Now)
			item.R7d = metrics.R7d
			item.T7d = metrics.T7d
			item.R5h = metrics.R5h
			item.T5h = metrics.T5h
			item.CycleBurnRate = metrics.CycleBurnRate
			item.TRequired = metrics.TRequired
			item.Urgency = metrics.Urgency
			item.IsBoosted = metrics.IsBoosted
			if item.PlanType == core.PlanTypeFree {
				item.IsBoosted = false
			}

			if credential.Disabled {
				// Manually disabled on host: keep current priority and disabled state untouched.
				// Quota metrics are retained for UI display, but the credential must never be
				// planned with positive priority, modified, or written back.
				item.Reason = ReasonDisabledOnHost
				item.Priority = credential.Priority
				item.Disabled = true
				item.quotaState = quotaStateUnknown
			} else {
				// Active on host: the plugin manages dynamic priorities purely in memory and
				// NEVER mutates the host credential's disabled field.
				item.Disabled = false
				switch {
				case metrics.IsWeeklyDepleted:
					// Weekly depletion: soft downgrade priority to -1
					item.Priority = DepletedPriority
					item.Reason = ReasonFreshWeeklyDepleted
					item.quotaState = quotaStateWeeklyDepleted
				case metrics.IsShortDepleted:
					// Short-window soft depletion
					item.Priority = DepletedPriority
					item.Reason = ReasonFreshShortWindowDepleted
					item.quotaState = quotaStateShortDepleted
				default:
					// Healthy candidate
					if item.IsBoosted {
						item.Reason = ReasonFreshBoosted
					} else {
						item.Reason = ReasonFreshRemainingPositive
					}
					item.quotaState = quotaStateHealthy
				}
			}
		} else if credential.Disabled {
			item.Reason = ReasonDisabledOnHost
		}

		items[index] = item
	}
	return items
}

func quotaEvidenceByAuthIndex(evidence []QuotaEvidence) map[string]QuotaEvidence {
	result := make(map[string]QuotaEvidence, len(evidence))
	for _, item := range evidence {
		if item.AuthIndex != "" {
			result[item.AuthIndex] = item
		}
	}
	return result
}

func planPositive(items []PlanItem, options Options) {
	tolerance := options.UrgencyTolerance

	customIndicesByTier := make(map[int][]int)
	boostedIndices := make([]int, 0)
	regularIndices := make([]int, 0)
	freeIndices := make([]int, 0)

	for index, item := range items {
		if item.Disabled || !item.hasQuotaEvidence || item.quotaState != quotaStateHealthy {
			continue
		}
		// Check 429 Cooldown
		if cooldownUntil, inCooldown := options.CooldownAuthIndexes[item.Credential.AuthIndex]; inCooldown && options.Now.Before(cooldownUntil) {
			items[index].Priority = DepletedPriority
			items[index].Disabled = false
			reason := Reason429Cooldown
			if specific := options.CooldownReasons[item.Credential.AuthIndex]; specific != "" {
				reason = specific
			}
			items[index].Reason = reason
			continue
		}
		if item.CustomTier > 0 {
			customIndicesByTier[item.CustomTier] = append(customIndicesByTier[item.CustomTier], index)
			continue
		}
		if item.PlanType == core.PlanTypeFree {
			freeIndices = append(freeIndices, index)
		} else if item.IsBoosted {
			boostedIndices = append(boostedIndices, index)
		} else {
			regularIndices = append(regularIndices, index)
		}
	}

	// 1. Plan Custom Tiers: user-defined tiers take absolute precedence over auto tiers.
	// Candidates sharing the identical custom tier receive equal priority for balanced load-sharing.
	minCustomPriority := options.BoostStartPriority
	if len(customIndicesByTier) > 0 {
		customTiers := make([]int, 0, len(customIndicesByTier))
		for tier := range customIndicesByTier {
			customTiers = append(customTiers, tier)
		}
		slices.Sort(customTiers)

		for _, tier := range customTiers {
			p := max(990-(tier-1)*10, 200)
			minCustomPriority = min(minCustomPriority, p)
			indices := customIndicesByTier[tier]
			for _, idx := range indices {
				items[idx].Priority = p
				items[idx].Disabled = false
				items[idx].Reason = fmt.Sprintf("custom tier #%d", tier)
			}
		}
	}

	boostStart := options.BoostStartPriority
	normalStart := options.NormalStartPriority
	freeStart := options.FreeStartPriority

	if len(customIndicesByTier) > 0 {
		if boostStart >= minCustomPriority {
			boostStart = max(minCustomPriority-10, 150)
		}
		if normalStart >= boostStart {
			normalStart = max(boostStart-10, 100)
		}
		if freeStart >= normalStart {
			freeStart = max(normalStart-10, MinPriority)
		}
	}

	// 2. Plan Auto Tier 1 (Boosted) with Equal Priority Clustering
	if len(boostedIndices) > 0 {
		slices.SortStableFunc(boostedIndices, func(left, right int) int {
			return CompareHealthyCandidates(items[left], items[right])
		})
		currentPriority := boostStart
		anchorUrgency := items[boostedIndices[0]].Urgency
		for i, index := range boostedIndices {
			if i > 0 {
				diff := anchorUrgency - items[index].Urgency
				if diff < 0 {
					diff = -diff
				}
				if diff > tolerance {
					currentPriority--
					if currentPriority < MinPriority {
						currentPriority = MinPriority
					}
					anchorUrgency = items[index].Urgency
				}
			}
			items[index].Priority = currentPriority
			items[index].Disabled = false
			items[index].Reason = ReasonFreshBoosted
		}
	}

	// 3. Plan Auto Tier 2 (Regular PRO Active) with Equal Priority Clustering
	if len(regularIndices) > 0 {
		slices.SortStableFunc(regularIndices, func(left, right int) int {
			return CompareHealthyCandidates(items[left], items[right])
		})
		currentPriority := normalStart
		anchorUrgency := items[regularIndices[0]].Urgency
		floor := max(min(freeStart+1, normalStart), MinPriority)
		for i, index := range regularIndices {
			if i > 0 {
				diff := anchorUrgency - items[index].Urgency
				if diff < 0 {
					diff = -diff
				}
				if diff > tolerance {
					currentPriority--
					if currentPriority < floor {
						currentPriority = floor
					}
					anchorUrgency = items[index].Urgency
				}
			}
			items[index].Priority = currentPriority
			items[index].Disabled = false
			items[index].Reason = ReasonFreshRemainingPositive
		}
	}

	// 4. Plan Auto Tier 3 (FREE Active) with Equal Priority Clustering
	if len(freeIndices) > 0 {
		slices.SortStableFunc(freeIndices, func(left, right int) int {
			return CompareHealthyCandidates(items[left], items[right])
		})
		currentPriority := freeStart
		anchorUrgency := items[freeIndices[0]].Urgency
		for i, index := range freeIndices {
			if i > 0 {
				diff := anchorUrgency - items[index].Urgency
				if diff < 0 {
					diff = -diff
				}
				if diff > tolerance {
					currentPriority--
					if currentPriority < MinPriority {
						currentPriority = MinPriority
					}
					anchorUrgency = items[index].Urgency
				}
			}
			items[index].Priority = currentPriority
			items[index].Disabled = false
			items[index].Reason = ReasonFreshRemainingPositive
		}
	}
}

func changes(items []PlanItem, options Options) []Change {
	result := make([]Change, 0)
	for _, item := range items {
		if targetDiffers(item, options) {
			result = append(result, Change{
				Credential: item.Credential,
				Priority:   item.Priority,
				Disabled:   item.Disabled,
				Reason:     item.Reason,
				IsBoosted:  item.IsBoosted,
			})
		}
	}
	return result
}

func targetDiffers(item PlanItem, options Options) bool {
	if item.Credential.PriorityMissing {
		return true
	}
	if item.Priority == item.Credential.Priority && item.Disabled == item.Credential.Disabled {
		return false
	}
	if item.Disabled != item.Credential.Disabled {
		return true
	}
	if item.Priority == DepletedPriority && item.Disabled {
		return item.Credential.Priority != DepletedPriority || !item.Credential.Disabled
	}
	minChange := max(options.MinChange, 0)
	return abs(item.Priority-item.Credential.Priority) >= minChange
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
