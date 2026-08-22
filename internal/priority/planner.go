package priority

import (
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
)

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
	MinChange           int
	UrgencyTolerance    float64
	CooldownAuthIndexes map[string]time.Time
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
	EvidenceFresh        bool
	ForceWrite           bool
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
}

// Change represents a required host write-back modification.
type Change struct {
	Credential    core.Credential
	Priority      int
	Disabled      bool
	EvidenceFresh bool
	Reason        string
	IsBoosted     bool
}

// Plan encapsulates the complete immutable priority scheduling decision and change set.
type Plan struct {
	DecidedAt time.Time
	Items     []PlanItem
	Changes   []Change
}

// PlanWithEvidence produces a plan from the Evidence authority's two
// semantically separate sets. Historical observations can shape targets for
// display or prediction, but their changes are never marked write-qualified.
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
	markHistoricalReasons(items)
	ensureUniquePriorities(items, options)
	SortPlanItems(items)
	return Plan{
		DecidedAt: options.Now.UTC(),
		Items:     items,
		Changes:   changes(items, options),
	}
}

func markHistoricalReasons(items []PlanItem) {
	for index, item := range items {
		if item.EvidenceFresh || !item.hasQuotaEvidence || item.Reason == ReasonDisabledOnHost {
			continue
		}
		items[index].Reason = "historical: " + item.Reason
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
		}

		evidence, hasFresh := freshByAuthIndex[credential.AuthIndex]
		evidenceFresh := true
		if !hasFresh {
			evidence, hasFresh = historicalByAuthIndex[credential.AuthIndex]
			evidenceFresh = false
		}
		if hasFresh {
			item.EvidenceFresh = evidenceFresh
			item.hasQuotaEvidence = true
			item.PlanType = evidence.PlanType
			item.ResetAt = evidence.ResetAt
			item.Remaining = evidence.Remaining
			item.ShortWindowResetAt = evidence.ShortWindowResetAt
			item.ShortWindowRemaining = evidence.ShortWindowRemaining
			item.LongWindowResetAt = evidence.LongWindowResetAt
			item.LongWindowRemaining = evidence.LongWindowRemaining
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

			if credential.Disabled {
				item.Disabled = true
				item.Priority = DepletedPriority
				item.Reason = ReasonDisabledOnHost
				items[index] = item
				continue
			}

			switch {
			case metrics.IsWeeklyDepleted:
				// Tier 3: Weekly hard depletion has highest precedence
				item.Priority = DepletedPriority
				item.Disabled = true
				item.Reason = ReasonFreshWeeklyDepleted
			case metrics.IsShortDepleted:
				// Tier 3: Short-window soft depletion
				item.Priority = DepletedPriority
				item.Disabled = false
				item.Reason = ReasonFreshShortWindowDepleted
			default:
				// Healthy candidate
				item.Disabled = false
				if item.IsBoosted {
					item.Reason = ReasonFreshBoosted
				} else {
					item.Reason = ReasonFreshRemainingPositive
				}
			}
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

	boostedIndices := make([]int, 0)
	regularIndices := make([]int, 0)

	for index, item := range items {
		if item.Disabled || !item.hasQuotaEvidence || item.Priority == DepletedPriority {
			continue
		}
		// Check 429 Cooldown
		if cooldownUntil, inCooldown := options.CooldownAuthIndexes[item.Credential.AuthIndex]; inCooldown && options.Now.Before(cooldownUntil) {
			items[index].Priority = DepletedPriority
			items[index].Disabled = false
			items[index].Reason = Reason429Cooldown
			continue
		}
		if item.IsBoosted {
			boostedIndices = append(boostedIndices, index)
		} else {
			regularIndices = append(regularIndices, index)
		}
	}

	// 1. Plan Tier 1 (Boosted) with Equal Priority Clustering
	if len(boostedIndices) > 0 {
		slices.SortStableFunc(boostedIndices, func(left, right int) int {
			return CompareHealthyCandidates(items[left], items[right])
		})
		currentPriority := options.BoostStartPriority
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

	// 2. Plan Tier 2 (Regular Active) with Equal Priority Clustering
	if len(regularIndices) > 0 {
		slices.SortStableFunc(regularIndices, func(left, right int) int {
			return CompareHealthyCandidates(items[left], items[right])
		})
		currentPriority := options.NormalStartPriority
		anchorUrgency := items[regularIndices[0]].Urgency
		for i, index := range regularIndices {
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

func ensureUniquePriorities(_ []PlanItem, _ Options) {
	// The planner does not rewrite an unprobed credential to make room for a
	// probed peer. Such a rewrite would turn a derived uniqueness adjustment
	// into a quota-driven Host change without Fresh Evidence.
}

func nextAvailablePriority(preferred int, used map[int]struct{}) int {
	if preferred > MaxPriority {
		preferred = MaxPriority
	}
	if preferred < MinPriority {
		preferred = MinPriority
	}
	for p := preferred; p >= MinPriority; p-- {
		if _, exists := used[p]; !exists {
			return p
		}
	}
	for p := preferred + 1; p <= MaxPriority; p++ {
		if _, exists := used[p]; !exists {
			return p
		}
	}
	return MinPriority
}

func changes(items []PlanItem, options Options) []Change {
	result := make([]Change, 0)
	for _, item := range items {
		if shouldChange(item, options) {
			result = append(result, Change{
				Credential:    item.Credential,
				Priority:      item.Priority,
				Disabled:      item.Disabled,
				EvidenceFresh: item.EvidenceFresh,
				Reason:        item.Reason,
				IsBoosted:     item.IsBoosted,
			})
		}
	}
	return result
}

func shouldChange(item PlanItem, options Options) bool {
	if !item.EvidenceFresh {
		return false
	}
	if item.Credential.Disabled && item.Disabled {
		return false
	}
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
	minChange := options.MinChange
	if minChange < 0 {
		minChange = 0
	}
	return abs(item.Priority-item.Credential.Priority) >= minChange
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
