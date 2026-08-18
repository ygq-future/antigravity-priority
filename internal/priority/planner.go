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
}

// Plan encapsulates the complete immutable priority scheduling decision and change set.
type Plan struct {
	Items   []PlanItem
	Changes []Change
}

// PlanFreshOnly produces an immutable Plan based on fresh probe evidence and current credentials.
func PlanFreshOnly(credentials []core.Credential, evidence []ProbeEvidence, options Options) Plan {
	normalizedOptions := normalizeOptions(options)
	evidenceByAuthIndex := freshEvidenceByAuthIndex(evidence)
	items := initialItems(credentials, evidenceByAuthIndex, normalizedOptions)
	planFreshPositive(items, normalizedOptions)
	ensureUniquePriorities(items, normalizedOptions)
	SortPlanItems(items)
	return Plan{
		Items:   items,
		Changes: changes(items, normalizedOptions),
	}
}

func normalizeOptions(options Options) Options {
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
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
		if isFreshReadyEvidence(item) {
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
			Reason:     "keep current state",
		}

		evidence, hasFresh := evidenceByAuthIndex[credential.AuthIndex]
		if hasFresh {
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

			switch {
			case metrics.IsWeeklyDepleted:
				// Tier 3: Weekly hard depletion has highest precedence
				item.Priority = DepletedPriority
				item.Disabled = true
				item.Reason = "fresh weekly depleted"
			case metrics.IsShortDepleted:
				// Tier 3: Short-window soft depletion
				item.Priority = DepletedPriority
				item.Disabled = false
				item.Reason = "fresh short window depleted"
			default:
				// Healthy candidate
				item.Disabled = false
				if item.IsBoosted {
					item.Reason = "fresh boosted"
				} else {
					item.Reason = "fresh remaining positive"
				}
			}
		}

		items[index] = item
	}
	return items
}

func planFreshPositive(items []PlanItem, options Options) {
	boostedIndices := make([]int, 0)
	regularIndices := make([]int, 0)

	for index, item := range items {
		if !item.EvidenceFresh || item.Priority == DepletedPriority {
			continue
		}
		if item.IsBoosted {
			boostedIndices = append(boostedIndices, index)
		} else {
			regularIndices = append(regularIndices, index)
		}
	}

	// 1. Plan Tier 1 (Boosted)
	slices.SortStableFunc(boostedIndices, func(left, right int) int {
		return CompareHealthyCandidates(items[left], items[right])
	})
	boostPriority := options.BoostStartPriority
	for _, index := range boostedIndices {
		items[index].Priority = boostPriority
		items[index].Disabled = false
		items[index].Reason = "fresh boosted"
		boostPriority--
		if boostPriority < MinPriority {
			boostPriority = MinPriority
		}
	}

	// 2. Plan Tier 2 (Regular Active)
	slices.SortStableFunc(regularIndices, func(left, right int) int {
		return CompareHealthyCandidates(items[left], items[right])
	})
	normalPriority := options.NormalStartPriority
	for _, index := range regularIndices {
		items[index].Priority = normalPriority
		items[index].Disabled = false
		items[index].Reason = "fresh remaining positive"
		normalPriority--
		if normalPriority < MinPriority {
			normalPriority = MinPriority
		}
	}
}

func ensureUniquePriorities(items []PlanItem, options Options) {
	activeIndices := make([]int, 0)
	hasFreshPositive := false

	for index, item := range items {
		if item.Disabled || item.Priority < MinPriority {
			continue
		}
		if item.EvidenceFresh && isPositiveRemaining(item) {
			hasFreshPositive = true
		}
		activeIndices = append(activeIndices, index)
	}

	if len(activeIndices) == 0 || !hasFreshPositive {
		return
	}

	slices.SortStableFunc(activeIndices, func(left, right int) int {
		return CompareUniquenessCandidates(items[left], items[right])
	})

	used := make(map[int]struct{}, len(activeIndices))
	assigned := make(map[int]int, len(activeIndices))

	// Pass 1: Assign fresh boosted items
	boostPriority := options.BoostStartPriority
	for _, index := range activeIndices {
		if !items[index].EvidenceFresh || !items[index].IsBoosted {
			continue
		}
		slot := nextAvailablePriority(boostPriority, used)
		assigned[index] = slot
		used[slot] = struct{}{}
		boostPriority = slot - 1
	}

	// Pass 2: Assign fresh regular items
	normalPriority := options.NormalStartPriority
	for _, index := range activeIndices {
		if !items[index].EvidenceFresh || items[index].IsBoosted {
			continue
		}
		slot := nextAvailablePriority(normalPriority, used)
		assigned[index] = slot
		used[slot] = struct{}{}
		normalPriority = slot - 1
	}

	// Pass 3: Assign unprobed peers
	for _, index := range activeIndices {
		if items[index].EvidenceFresh {
			continue
		}
		pref := items[index].Priority
		if pref > options.NormalStartPriority {
			pref = options.NormalStartPriority
		}
		slot := nextAvailablePriority(pref, used)
		assigned[index] = slot
		used[slot] = struct{}{}
	}

	// Apply unique assignments and tag shifted peers with ForceWrite
	for _, index := range activeIndices {
		newPriority := assigned[index]
		if items[index].Priority != newPriority {
			if !items[index].EvidenceFresh {
				items[index].ForceWrite = true
				items[index].Reason = "priority uniqueness"
			} else if items[index].Reason == "keep current state" || items[index].Reason == "" {
				items[index].Reason = "priority uniqueness"
			}
			items[index].Priority = newPriority
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
			})
		}
	}
	return result
}

func shouldChange(item PlanItem, options Options) bool {
	if !item.EvidenceFresh && !item.ForceWrite {
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
