// Package evidence owns the Fresh Evidence boundary for quota scheduling.
// Callers provide probe outcomes and persisted observations; this package is
// the only place that classifies round membership, validation, and history.
package evidence

import (
	"strings"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/provider/antigravity"
)

// ObservationKind describes the diagnostic truth of an observation without
// granting it Host write authority.
type ObservationKind string

const (
	ObservationFresh      ObservationKind = "fresh"
	ObservationHistorical ObservationKind = "historical"
	ObservationFailed     ObservationKind = "failed"
	ObservationInvalid    ObservationKind = "invalid"
	ObservationWrongGroup ObservationKind = "wrong_group"
	ObservationUnknown    ObservationKind = "unknown"
)

// Round identifies one scheduling round and the model group being classified.
// The opaque ID, rather than timestamp equality, owns round membership.
type Round struct {
	ID         string
	ModelGroup config.AntigravityModelGroup
}

// ProbeObservation binds one normalized provider result to its scheduling
// round. The provider result itself remains an observation, not an authority.
type ProbeObservation struct {
	RoundID string
	Result  antigravity.ProbeResult
}

// HistoricalObservation is a persisted successful observation plus the
// independent diagnostic state of its most recent failure.
type HistoricalObservation struct {
	AuthIndex  string
	ModelGroup config.AntigravityModelGroup
	Evidence   priority.QuotaEvidence
	LastError  string
	FailureAt  time.Time
}

// Observation is the read-only diagnostic projection returned by Classify.
// Evidence is present for valid fresh or historical quota observations and is
// never implied by a failure or invalid result.
type Observation struct {
	AuthIndex  string
	ModelGroup config.AntigravityModelGroup
	Kind       ObservationKind
	ObservedAt time.Time
	Evidence   *priority.QuotaEvidence
	Failure    string
}

// Result is the complete classification for one model group in one round.
// Eligible contains only validated successful results from the exact round.
type Result struct {
	Eligible     []priority.QuotaEvidence
	Observations []Observation
}

// FreshQuotaEvidence exposes only the validated current-round quota set to
// the Planner contract. The returned slice is detached from the result.
func (result Result) FreshQuotaEvidence() []priority.QuotaEvidence {
	return cloneQuotaEvidence(result.Eligible)
}

// HistoricalQuotaEvidence exposes only read-only historical quota to the
// Planner contract. Failed and invalid observations are intentionally absent.
func (result Result) HistoricalQuotaEvidence() []priority.QuotaEvidence {
	historical := make([]priority.QuotaEvidence, 0)
	for _, observation := range result.Observations {
		if observation.Kind == ObservationHistorical && observation.Evidence != nil {
			historical = append(historical, cloneEvidence(*observation.Evidence))
		}
	}
	return historical
}

// Input contains all facts needed to classify one model group. Credentials
// constrain the known Host inventory; probes and history are never inferred
// from one another.
type Input struct {
	Round       Round
	Credentials []core.Credential
	Probes      []ProbeObservation
	Historical  []HistoricalObservation
}

// Classify is the single Fresh Evidence authority. A successful, validated
// probe is eligible only when its round ID and model group match the input
// round. Historical, failed, malformed, and wrong-group results remain
// diagnostic observations only.
func Classify(input Input) Result {
	knownCredentials := make(map[string]core.Credential, len(input.Credentials))
	for _, credential := range input.Credentials {
		knownCredentials[credential.AuthIndex] = credential
	}

	result := Result{
		Eligible:     make([]priority.QuotaEvidence, 0),
		Observations: make([]Observation, 0),
	}
	for _, historical := range input.Historical {
		authIndex, modelGroup := historicalIdentity(historical)
		if authIndex == "" || modelGroup != input.Round.ModelGroup {
			continue
		}
		if _, known := knownCredentials[authIndex]; !known {
			continue
		}
		if !validQuotaEvidence(historical.Evidence, input.Round.ModelGroup) {
			continue
		}
		evidence := cloneEvidence(historical.Evidence)
		result.Observations = append(result.Observations, Observation{
			AuthIndex:  evidence.AuthIndex,
			ModelGroup: input.Round.ModelGroup,
			Kind:       ObservationHistorical,
			ObservedAt: evidence.ObservedAt,
			Evidence:   &evidence,
		})
	}

	currentAuthIndexes := make(map[string]ObservationKind)
	for _, probe := range input.Probes {
		observed := probe.Result
		if observed.AuthIndex == "" {
			continue
		}
		if _, known := knownCredentials[observed.AuthIndex]; !known {
			continue
		}
		if observed.ModelGroup != input.Round.ModelGroup {
			result.Observations = append(result.Observations, Observation{
				AuthIndex:  observed.AuthIndex,
				ModelGroup: observed.ModelGroup,
				Kind:       ObservationWrongGroup,
				ObservedAt: observed.ObservedAt,
				Failure:    "probe result belongs to another model group",
			})
			currentAuthIndexes[observed.AuthIndex] = ObservationWrongGroup
			continue
		}

		if probe.RoundID != input.Round.ID {
			if observed.Status == antigravity.StatusReady {
				if quota, ok := quotaEvidenceFromProbe(observed, input.Round.ModelGroup); ok {
					result.Observations = append(result.Observations, historicalObservation(quota))
					currentAuthIndexes[observed.AuthIndex] = ObservationHistorical
					continue
				}
			}
			kind := ObservationFailed
			if observed.Status == antigravity.StatusReady {
				kind = ObservationInvalid
			}
			result.Observations = append(result.Observations, failedObservation(observed, kind))
			currentAuthIndexes[observed.AuthIndex] = kind
			continue
		}

		quota, valid := quotaEvidenceFromProbe(observed, input.Round.ModelGroup)
		if observed.Status == antigravity.StatusReady && valid {
			removeObservationForAuth(&result.Observations, observed.AuthIndex, ObservationHistorical)
			result.Eligible = append(result.Eligible, quota)
			result.Observations = append(result.Observations, freshObservation(quota))
			currentAuthIndexes[observed.AuthIndex] = ObservationFresh
			continue
		}

		kind := ObservationFailed
		if observed.Status == antigravity.StatusReady {
			kind = ObservationInvalid
		}
		result.Observations = append(result.Observations, failedObservation(observed, kind))
		currentAuthIndexes[observed.AuthIndex] = kind
	}

	for _, historical := range input.Historical {
		authIndex, modelGroup := historicalIdentity(historical)
		if historical.LastError == "" || authIndex == "" || modelGroup != input.Round.ModelGroup {
			continue
		}
		if _, known := knownCredentials[authIndex]; !known {
			continue
		}
		if _, hasCurrent := currentAuthIndexes[authIndex]; hasCurrent {
			continue
		}
		result.Observations = append(result.Observations, Observation{
			AuthIndex:  authIndex,
			ModelGroup: modelGroup,
			Kind:       ObservationFailed,
			ObservedAt: historical.FailureAt,
			Failure:    historical.LastError,
		})
	}

	return result
}

func historicalIdentity(historical HistoricalObservation) (string, config.AntigravityModelGroup) {
	authIndex := historical.AuthIndex
	modelGroup := historical.ModelGroup
	if authIndex == "" {
		authIndex = historical.Evidence.AuthIndex
	}
	if modelGroup == "" {
		modelGroup = historical.Evidence.ModelGroup
	}
	return authIndex, modelGroup
}

func quotaEvidenceFromProbe(result antigravity.ProbeResult, group config.AntigravityModelGroup) (priority.QuotaEvidence, bool) {
	if result.Provider != core.ProviderAntigravity || result.ModelGroup != group || result.AuthIndex == "" || result.ObservedAt.IsZero() {
		return priority.QuotaEvidence{}, false
	}
	if result.ResetAt == nil || result.Remaining == nil {
		return priority.QuotaEvidence{}, false
	}
	return priority.QuotaEvidence{
		Provider:             result.Provider,
		AuthIndex:            result.AuthIndex,
		ModelGroup:           result.ModelGroup,
		ObservedAt:           result.ObservedAt.UTC(),
		ResetAt:              cloneTime(result.ResetAt),
		Remaining:            cloneInt64(result.Remaining),
		ShortWindowResetAt:   cloneTime(result.ShortWindowResetAt),
		ShortWindowRemaining: cloneInt64(result.ShortWindowRemaining),
		LongWindowResetAt:    cloneTime(result.LongWindowResetAt),
		LongWindowRemaining:  cloneInt64(result.LongWindowRemaining),
		PlanType:             result.PlanType,
	}, true
}

func validQuotaEvidence(evidence priority.QuotaEvidence, group config.AntigravityModelGroup) bool {
	return evidence.Provider == core.ProviderAntigravity &&
		evidence.AuthIndex != "" &&
		evidence.ModelGroup == group &&
		!evidence.ObservedAt.IsZero() &&
		evidence.ResetAt != nil &&
		evidence.Remaining != nil
}

func freshObservation(evidence priority.QuotaEvidence) Observation {
	copy := cloneEvidence(evidence)
	return Observation{
		AuthIndex:  copy.AuthIndex,
		ModelGroup: copy.ModelGroup,
		Kind:       ObservationFresh,
		ObservedAt: copy.ObservedAt,
		Evidence:   &copy,
	}
}

func historicalObservation(evidence priority.QuotaEvidence) Observation {
	copy := cloneEvidence(evidence)
	return Observation{
		AuthIndex:  copy.AuthIndex,
		ModelGroup: copy.ModelGroup,
		Kind:       ObservationHistorical,
		ObservedAt: copy.ObservedAt,
		Evidence:   &copy,
	}
}

func failedObservation(result antigravity.ProbeResult, kind ObservationKind) Observation {
	return Observation{
		AuthIndex:  result.AuthIndex,
		ModelGroup: result.ModelGroup,
		Kind:       kind,
		ObservedAt: result.ObservedAt,
		Failure:    strings.TrimSpace(result.Error),
	}
}

func removeObservationForAuth(observations *[]Observation, authIndex string, kind ObservationKind) {
	filtered := (*observations)[:0]
	for _, observation := range *observations {
		if observation.AuthIndex == authIndex && observation.Kind == kind {
			continue
		}
		filtered = append(filtered, observation)
	}
	*observations = filtered
}

func cloneEvidence(value priority.QuotaEvidence) priority.QuotaEvidence {
	value.ResetAt = cloneTime(value.ResetAt)
	value.Remaining = cloneInt64(value.Remaining)
	value.ShortWindowResetAt = cloneTime(value.ShortWindowResetAt)
	value.ShortWindowRemaining = cloneInt64(value.ShortWindowRemaining)
	value.LongWindowResetAt = cloneTime(value.LongWindowResetAt)
	value.LongWindowRemaining = cloneInt64(value.LongWindowRemaining)
	return value
}

func cloneQuotaEvidence(values []priority.QuotaEvidence) []priority.QuotaEvidence {
	if values == nil {
		return nil
	}
	cloned := make([]priority.QuotaEvidence, len(values))
	for index, value := range values {
		cloned[index] = cloneEvidence(value)
	}
	return cloned
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
