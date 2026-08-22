package evidence_test

import (
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/provider/antigravity"
)

func TestClassify_CurrentRoundEvidenceAuthority(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(100 * time.Hour)
	credentials := []core.Credential{
		{AuthIndex: "fresh", Provider: core.ProviderAntigravity},
		{AuthIndex: "failed", Provider: core.ProviderAntigravity},
		{AuthIndex: "historical", Provider: core.ProviderAntigravity},
		{AuthIndex: "stale-round", Provider: core.ProviderAntigravity},
		{AuthIndex: "incomplete", Provider: core.ProviderAntigravity},
		{AuthIndex: "wrong-group", Provider: core.ProviderAntigravity},
	}

	result := evidence.Classify(evidence.Input{
		Round:       evidence.Round{ID: "round-current", ModelGroup: config.AntigravityModelGroupGemini},
		Credentials: credentials,
		Probes: []evidence.ProbeObservation{
			{RoundID: "round-current", Result: readyProbe("fresh", config.AntigravityModelGroupGemini, now, &reset5h, &reset7d)},
			{RoundID: "round-current", Result: failedProbe("failed", config.AntigravityModelGroupGemini, now)},
			{RoundID: "old-round", Result: readyProbe("stale-round", config.AntigravityModelGroupGemini, now.Add(-time.Hour), &reset5h, &reset7d)},
			{RoundID: "round-current", Result: incompleteProbe("incomplete", config.AntigravityModelGroupGemini, now)},
			{RoundID: "round-current", Result: readyProbe("wrong-group", config.AntigravityModelGroupClaudeGPT, now, &reset5h, &reset7d)},
		},
		Historical: []evidence.HistoricalObservation{{
			Evidence: historicalQuota("historical", config.AntigravityModelGroupGemini, now.Add(-2*time.Hour), reset5h, reset7d),
		}},
	})

	if len(result.Eligible) != 1 || result.Eligible[0].AuthIndex != "fresh" {
		t.Fatalf("eligible = %#v; want only current-round fresh evidence", result.Eligible)
	}
	if !result.Eligible[0].ObservedAt.Equal(now) {
		t.Fatalf("fresh observed_at = %s; want %s", result.Eligible[0].ObservedAt, now)
	}

	observations := make(map[string]evidence.Observation)
	for _, observation := range result.Observations {
		observations[observation.AuthIndex] = observation
	}
	checks := map[string]evidence.ObservationKind{
		"fresh":       evidence.ObservationFresh,
		"failed":      evidence.ObservationFailed,
		"historical":  evidence.ObservationHistorical,
		"stale-round": evidence.ObservationHistorical,
		"incomplete":  evidence.ObservationInvalid,
		"wrong-group": evidence.ObservationWrongGroup,
	}
	for authIndex, wantKind := range checks {
		observation, ok := observations[authIndex]
		if !ok {
			t.Fatalf("missing observation for %q: %#v", authIndex, result.Observations)
		}
		if observation.Kind != wantKind {
			t.Errorf("%s kind = %q; want %q", authIndex, observation.Kind, wantKind)
		}
	}
	if observations["historical"].Evidence == nil || !observations["historical"].Evidence.ObservedAt.Equal(now.Add(-2*time.Hour)) {
		t.Fatalf("historical observation lost its original observation time: %#v", observations["historical"])
	}
}

func TestClassify_FailedProbePreservesHistoricalObservationSeparately(t *testing.T) {
	now := time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(100 * time.Hour)
	result := evidence.Classify(evidence.Input{
		Round:       evidence.Round{ID: "round-current", ModelGroup: config.AntigravityModelGroupGemini},
		Credentials: []core.Credential{{AuthIndex: "account", Provider: core.ProviderAntigravity}},
		Probes: []evidence.ProbeObservation{{
			RoundID: "round-current",
			Result:  failedProbe("account", config.AntigravityModelGroupGemini, now),
		}},
		Historical: []evidence.HistoricalObservation{{
			Evidence:  historicalQuota("account", config.AntigravityModelGroupGemini, now.Add(-time.Hour), reset5h, reset7d),
			LastError: "previous failure",
			FailureAt: now.Add(-30 * time.Minute),
		}},
	})

	if len(result.Eligible) != 0 {
		t.Fatalf("failed probe produced eligible evidence: %#v", result.Eligible)
	}
	var failed, historical int
	for _, observation := range result.Observations {
		if observation.Kind == evidence.ObservationFailed {
			failed++
		}
		if observation.Kind == evidence.ObservationHistorical {
			historical++
		}
	}
	if failed != 1 || historical != 1 {
		t.Fatalf("observations = %#v; want one current failure and one historical observation", result.Observations)
	}
}

func TestClassify_StaleFailedProbeIsNeverHistoricalQuota(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(100 * time.Hour)
	result := evidence.Classify(evidence.Input{
		Round:       evidence.Round{ID: "round-current", ModelGroup: config.AntigravityModelGroupGemini},
		Credentials: []core.Credential{{AuthIndex: "stale-failed", Provider: core.ProviderAntigravity}},
		Probes: []evidence.ProbeObservation{{
			RoundID: "old-round",
			Result: func() antigravity.ProbeResult {
				probe := readyProbe("stale-failed", config.AntigravityModelGroupGemini, now.Add(-time.Hour), &reset5h, &reset7d)
				probe.Status = antigravity.StatusProbeFailed
				probe.Error = "quota unavailable"
				return probe
			}(),
		}},
	})

	if len(result.Eligible) != 0 {
		t.Fatalf("stale failed probe produced eligible evidence: %#v", result.Eligible)
	}
	if len(result.Observations) != 1 || result.Observations[0].Kind != evidence.ObservationFailed {
		t.Fatalf("stale failed observations = %#v; want one failed observation", result.Observations)
	}
}

func readyProbe(authIndex string, group config.AntigravityModelGroup, observedAt time.Time, reset5h, reset7d *time.Time) antigravity.ProbeResult {
	short := int64(80)
	long := int64(90)
	remaining := int64(80)
	return antigravity.ProbeResult{
		Provider:             core.ProviderAntigravity,
		AuthIndex:            authIndex,
		ModelGroup:           group,
		ObservedAt:           observedAt,
		ResetAt:              reset5h,
		Remaining:            &remaining,
		ShortWindowResetAt:   reset5h,
		ShortWindowRemaining: &short,
		LongWindowResetAt:    reset7d,
		LongWindowRemaining:  &long,
		Status:               antigravity.StatusReady,
		PlanType:             core.PlanTypePro,
	}
}

func failedProbe(authIndex string, group config.AntigravityModelGroup, observedAt time.Time) antigravity.ProbeResult {
	return antigravity.ProbeResult{
		Provider:   core.ProviderAntigravity,
		AuthIndex:  authIndex,
		ModelGroup: group,
		ObservedAt: observedAt,
		Status:     antigravity.StatusProbeFailed,
		Error:      "quota unavailable",
	}
}

func incompleteProbe(authIndex string, group config.AntigravityModelGroup, observedAt time.Time) antigravity.ProbeResult {
	result := readyProbe(authIndex, group, observedAt, nil, nil)
	result.ResetAt = nil
	return result
}

func historicalQuota(authIndex string, group config.AntigravityModelGroup, observedAt, reset5h, reset7d time.Time) priority.QuotaEvidence {
	short := int64(70)
	long := int64(85)
	remaining := int64(70)
	return priority.QuotaEvidence{
		Provider:             core.ProviderAntigravity,
		AuthIndex:            authIndex,
		ModelGroup:           group,
		ObservedAt:           observedAt,
		ResetAt:              &reset5h,
		Remaining:            &remaining,
		ShortWindowResetAt:   &reset5h,
		ShortWindowRemaining: &short,
		LongWindowResetAt:    &reset7d,
		LongWindowRemaining:  &long,
		PlanType:             core.PlanTypePro,
	}
}
