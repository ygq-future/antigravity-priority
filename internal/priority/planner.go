package priority

import (
	"slices"
	"time"

	"antigravity-priority/internal/core"
)

// EvidenceStatus indicates whether probe evidence is valid and ready for planning.
type EvidenceStatus string

const (
	// EvidenceStatusUnknown indicates no probe determination is available.
	EvidenceStatusUnknown EvidenceStatus = "unknown"
	// EvidenceStatusReady indicates evidence is valid and ready for planning.
	EvidenceStatusReady EvidenceStatus = "ready"
	// EvidenceStatusProbeFailed indicates probing failed.
	EvidenceStatusProbeFailed EvidenceStatus = "probe_failed"
	// EvidenceStatusUnsupported indicates unsupported provider or configuration.
	EvidenceStatusUnsupported EvidenceStatus = "unsupported"
	// EvidenceStatusUnavailable indicates the credential is currently unavailable.
	EvidenceStatusUnavailable EvidenceStatus = "unavailable"

	// Decision Reasons
	ReasonKeepCurrentState         = "keep current state"
	ReasonDisabledOnHost           = "disabled on host"
	ReasonFailedQuotaFetch         = "failedQuotaFetch"
	ReasonFreshWeeklyDepleted      = "fresh weekly depleted"
	ReasonFreshShortWindowDepleted = "fresh short window depleted"
	ReasonFreshBoosted             = "fresh boosted"
	ReasonFreshRemainingPositive   = "fresh remaining positive"
	Reason429Cooldown              = "429 rate limit cooldown"
)

// ProbeEvidence represents the per-credential quota evidence produced during a scheduling round.
type ProbeEvidence struct {
	AuthIndex            string
	Provider             core.Provider
	ObservedAt           time.Time
	ResetAt              *time.Time
	Remaining            *int64
	ShortWindowResetAt   *time.Time
	ShortWindowRemaining *int64
	LongWindowResetAt    *time.Time
	LongWindowRemaining  *int64
	Freshness            core.Freshness
	ProbeStatus          core.ProbeStatus
	Status               EvidenceStatus
	PlanType             core.PlanType
	EvidenceFresh        bool
	CycleBurnRate        float64
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

// PlanFreshOnly produces an immutable Plan based on fresh probe evidence and current credentials.
func PlanFreshOnly(credentials []core.Credential, evidence []ProbeEvidence, options Options) Plan {
	if options.Now.IsZero() {
		panic("priority: explicit decision time is required")
	}
	normalizedOptions := normalizeOptions(options)
	evidenceByAuthIndex := freshEvidenceByAuthIndex(evidence)
	items := initialItems(credentials, evidenceByAuthIndex, normalizedOptions)
	planFreshPositive(items, normalizedOptions)
	ensureUniquePriorities(items, normalizedOptions)
	SortPlanItems(items)
	return Plan{
		DecidedAt: normalizedOptions.Now.UTC(),
		Items:     items,
		Changes:   changes(items, normalizedOptions),
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

func freshEvidenceByAuthIndex(evidence []ProbeEvidence) map[string]ProbeEvidence {
	result := make(map[string]ProbeEvidence, len(evidence))
	for _, item := range evidence {
		if isFreshReadyEvidence(item) || item.Status == EvidenceStatusProbeFailed {
			result[item.AuthIndex] = item
		}
	}
	return result
}

func isFreshReadyEvidence(evidence ProbeEvidence) bool {
	return evidence.EvidenceFresh &&
		evidence.Freshness == core.FreshnessFresh &&
		evidence.ProbeStatus == core.ProbeStatusReady &&
		evidence.Status == EvidenceStatusReady
}

func initialItems(credentials []core.Credential, evidenceByAuthIndex map[string]ProbeEvidence, options Options) []PlanItem {
	items := make([]PlanItem, len(credentials))
	for index, credential := range credentials {
		item := PlanItem{
			Credential: credential,
			Priority:   credential.Priority,
			Disabled:   credential.Disabled,
			PlanType:   credential.PlanType,
			Reason:     ReasonKeepCurrentState,
		}

		evidence, hasFresh := evidenceByAuthIndex[credential.AuthIndex]
		if hasFresh {
			if evidence.Status == EvidenceStatusProbeFailed {
				item.Disabled = true
				item.EvidenceFresh = true
				if credential.Disabled {
					item.Priority = DepletedPriority
					item.Reason = ReasonDisabledOnHost
				} else {
					item.Reason = ReasonFailedQuotaFetch
				}
				items[index] = item
				continue
			}

			item.EvidenceFresh = true
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
		} else if credential.Disabled {
			item.Disabled = true
			item.Priority = DepletedPriority
			item.Reason = ReasonDisabledOnHost
		}

		items[index] = item
	}
	return items
}

func planFreshPositive(items []PlanItem, options Options) {
	tolerance := options.UrgencyTolerance

	boostedIndices := make([]int, 0)
	regularIndices := make([]int, 0)

	for index, item := range items {
		if item.Disabled || !item.EvidenceFresh || item.Priority == DepletedPriority {
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

func ensureUniquePriorities(items []PlanItem, options Options) {
	for index, item := range items {
		if item.Disabled || item.Priority < MinPriority {
			continue
		}
		// Cap unprobed peers so they don't linger at 999 boost tier
		if !item.EvidenceFresh && item.Priority > options.NormalStartPriority {
			items[index].Priority = options.NormalStartPriority
			items[index].ForceWrite = true
		}
	}
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
				EvidenceFresh: item.EvidenceFresh || item.ForceWrite,
				Reason:        item.Reason,
				IsBoosted:     item.IsBoosted,
			})
		}
	}
	return result
}

func shouldChange(item PlanItem, options Options) bool {
	if !item.EvidenceFresh && !item.ForceWrite {
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
