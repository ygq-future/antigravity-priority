package runtime

import (
	"maps"
	"strings"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

// RoutingEngineInput contains the complete immutable input for evaluating all
// three Antigravity quota pools into both O(1) scheduler tables and UI/audit snapshots.
type RoutingEngineInput struct {
	Credentials           []core.Credential
	EvidenceByGroup       map[config.AntigravityModelGroup]evidence.Result
	Store                 *state.Store
	Config                config.Config
	PlanningOptions       *priority.Options
	Warmup                *config.WarmupConfig
	Now                   time.Time
	Models                []apply.ModelOption
	SupportedModelsByAuth map[string][]string
}

// RoutingEngineOutput is the atomic three-pool routing and snapshot projection
// produced by EvaluateRoutingAndSnapshot.
type RoutingEngineOutput struct {
	Tables   map[config.AntigravityModelGroup]PoolRoutingTable
	Snapshot apply.SnapshotSet
}

type poolEvaluation struct {
	table    PoolRoutingTable
	snapshot apply.PlanSnapshot
}

type poolCooldownInfo struct {
	until  time.Time
	reason string
}

// EvaluateRoutingAndSnapshot computes all three pool routing tables and the
// corresponding SnapshotSet from a single evaluation pass. Both scheduler.pick
// and the Management UI consume this identical output.
func EvaluateRoutingAndSnapshot(input RoutingEngineInput) (RoutingEngineOutput, error) {
	if input.Now.IsZero() {
		return RoutingEngineOutput{}, errProjectionTimeRequired
	}
	now := input.Now.UTC()

	credentials := enrichCredentialsPlanTypeFromStore(input.Store, cloneCredentials(input.Credentials))

	evidenceByGroup := input.EvidenceByGroup
	if evidenceByGroup == nil {
		evidenceByGroup = buildProjectionEvidence(input.Store, credentials)
	}

	var baseOptions priority.Options
	if input.PlanningOptions != nil {
		baseOptions = clonePlanningOptions(*input.PlanningOptions)
	} else {
		baseOptions = planningOptionsForEngine(input.Config, input.Store, now)
	}
	baseOptions.Now = now

	warmup := input.Config.Warmup
	if input.Warmup != nil {
		warmup = *input.Warmup
	}

	tables := make(map[config.AntigravityModelGroup]PoolRoutingTable, len(config.AllAntigravityModelGroups))
	snapshotsByGroup := make(map[config.AntigravityModelGroup]apply.PlanSnapshot, len(config.AllAntigravityModelGroups))

	for _, group := range config.AllAntigravityModelGroups {
		eval := evaluateSinglePool(group, credentials, evidenceByGroup[group], input.Store, baseOptions, now)
		tables[group] = eval.table
		snapshotsByGroup[group] = eval.snapshot
	}

	quotaWindowsByAuth := buildAllQuotaWindows(snapshotsByGroup)
	groups := make(map[string]apply.PoolSnapshot, len(config.AllAntigravityModelGroups))

	for _, group := range config.AllAntigravityModelGroups {
		snap := snapshotsByGroup[group]
		for index := range snap.Items {
			authIndex := snapshotAuthIndex(snap.Items[index])
			if qw, ok := quotaWindowsByAuth[authIndex]; ok {
				snap.Items[index].QuotaWindows = qw
			}
			_, overridden := warmup.WarmupOverride(authIndex)
			isFree := snap.Items[index].PlanType == string(core.PlanTypeFree)
			snap.Items[index].WarmupOverridden = overridden && !isFree
			snap.Items[index].WarmupActive = !isFree && warmupEffectiveEnabled(warmup, authIndex, group)
			if supported, ok := input.SupportedModelsByAuth[authIndex]; ok && len(supported) > 0 {
				snap.Items[index].SupportedModels = append([]string(nil), supported...)
			}
		}
		groups[string(group)] = apply.PoolSnapshot{
			Items: snap.Items,
		}
	}

	var models []apply.ModelOption
	if len(input.Models) > 0 {
		models = append([]apply.ModelOption(nil), input.Models...)
	}

	return RoutingEngineOutput{
		Tables: tables,
		Snapshot: apply.SnapshotSet{
			ObservedAt: now,
			Groups:     groups,
			Models:     models,
		},
	}, nil
}

func evaluateSinglePool(
	pool config.AntigravityModelGroup,
	credentials []core.Credential,
	rawEvidence evidence.Result,
	store *state.Store,
	baseOptions priority.Options,
	now time.Time,
) poolEvaluation {
	grpEvidence := cloneEvidence(rawEvidence)
	cooldownMap := resolveCooldownsForPool(pool, store, baseOptions, now)

	grpOptions := clonePlanningOptions(baseOptions)
	grpOptions.Now = now
	if len(cooldownMap) > 0 {
		grpOptions.CooldownAuthIndexes = make(map[string]time.Time, len(cooldownMap))
		grpOptions.CooldownReasons = make(map[string]string, len(cooldownMap))
		for authIdx, info := range cooldownMap {
			grpOptions.CooldownAuthIndexes[authIdx] = info.until
			if info.reason != "" {
				grpOptions.CooldownReasons[authIdx] = info.reason
			}
		}
	} else {
		grpOptions.CooldownAuthIndexes = nil
		grpOptions.CooldownReasons = nil
	}

	plan := priority.PlanWithEvidence(cloneCredentials(credentials), grpEvidence, grpOptions)

	failedObsByAuth := make(map[string]evidence.Observation)
	unavailableObsByAuth := make(map[string]evidence.Observation)
	for _, obs := range grpEvidence.Observations {
		switch obs.Kind {
		case evidence.ObservationFailed, evidence.ObservationInvalid:
			failedObsByAuth[obs.AuthIndex] = obs
		case evidence.ObservationUnavailable:
			unavailableObsByAuth[obs.AuthIndex] = obs
		}
	}

	var blockedModels map[string]map[string]time.Time
	if store != nil {
		blockedModels = store.GetActiveModelCooldownsForGroup(now, string(pool))
	} else {
		blockedModels = make(map[string]map[string]time.Time)
	}

	snap := apply.Snapshot(plan)
	candidates := make(map[string]PrecomputedCandidate, len(plan.Items))
	var nextTransition time.Time

	for index, item := range plan.Items {
		authIdx := item.Credential.AuthIndex
		snapItem := &snap.Items[index]

		unavailable := false
		unavailableReason := ""
		if obs, ok := unavailableObsByAuth[authIdx]; ok {
			unavailable = true
			unavailableReason = strings.TrimSpace(obs.Failure)
		} else if store != nil {
			if entry, ok := store.GetEntry(authIdx, string(pool)); ok && entry.Unavailable {
				unavailable = true
				unavailableReason = strings.TrimSpace(entry.UnavailableReason)
			}
		}

		probeObs, hasProbeFailure := failedObsByAuth[authIdx]
		cooldownInfo, isCooling := cooldownMap[authIdx]

		classification := classifyPoolCandidate(
			item,
			isCooling,
			cooldownInfo,
			unavailable,
			unavailableReason,
			hasProbeFailure,
			probeObs,
		)

		snapItem.Target.Priority = classification.score
		snapItem.Target.Disabled = classification.disabled
		snapItem.RoutingStatus = classification.status
		eligible := classification.poolEligible
		snapItem.PoolEligible = &eligible
		snapItem.ProbeFailed = hasProbeFailure
		snapItem.IsBoosted = classification.isBoosted
		snapItem.CooldownUntil = cloneTimePointer(classification.cooldownUntil)
		snapItem.Reason = redactCandidateReason(classification.reason)

		var shortReset, longReset time.Time
		if item.ShortWindowResetAt != nil {
			shortReset = *item.ShortWindowResetAt
		}
		if item.LongWindowResetAt != nil {
			longReset = *item.LongWindowResetAt
		}
		var cdUntil time.Time
		if classification.cooldownUntil != nil {
			cdUntil = *classification.cooldownUntil
		}

		candidates[authIdx] = PrecomputedCandidate{
			AuthIndex:          authIdx,
			Priority:           classification.score,
			Disabled:           classification.disabled,
			RoutingStatus:      classification.status,
			PoolEligible:       classification.poolEligible,
			IsBoosted:          classification.isBoosted,
			Urgency:            item.Urgency,
			ShortWindowResetAt: shortReset,
			LongWindowResetAt:  longReset,
			CooldownUntil:      cdUntil,
			Reason:             classification.reason,
		}

		if !shortReset.IsZero() && shortReset.After(now) {
			nextTransition = minFutureTime(nextTransition, shortReset)
		}
		if !longReset.IsZero() && longReset.After(now) {
			nextTransition = minFutureTime(nextTransition, longReset)
		}
		if isCooling && cdUntil.After(now) {
			nextTransition = minFutureTime(nextTransition, cdUntil)
		}
	}

	for _, models := range blockedModels {
		for _, until := range models {
			if until.After(now) {
				nextTransition = minFutureTime(nextTransition, until)
			}
		}
	}

	return poolEvaluation{
		table: PoolRoutingTable{
			DecidedAt:        now,
			NextTransitionAt: nextTransition,
			ByAuthIndex:      candidates,
			BlockedModels:    blockedModels,
		},
		snapshot: snap,
	}
}

type candidateClassification struct {
	score         int
	disabled      bool
	status        apply.RoutingStatus
	poolEligible  bool
	isBoosted     bool
	reason        string
	cooldownUntil *time.Time
}

func classifyPoolCandidate(
	item priority.PlanItem,
	isCooling bool,
	cooldown poolCooldownInfo,
	unavailable bool,
	unavailableReason string,
	hasProbeFailure bool,
	probeObs evidence.Observation,
) candidateClassification {
	if item.Disabled {
		return candidateClassification{
			score:        disabledRoutingTier,
			disabled:     true,
			status:       apply.RoutingStatusDisabledOnHost,
			poolEligible: false,
			reason:       priority.ReasonDisabledOnHost,
		}
	}

	if isCooling {
		until := cooldown.until.UTC()
		reason := cooldown.reason
		if reason == "" {
			reason = priority.Reason429Cooldown
		}
		return candidateClassification{
			score:         priority.DepletedPriority,
			status:        apply.RoutingStatusCooldown,
			poolEligible:  true,
			reason:        reason,
			cooldownUntil: &until,
		}
	}

	if unavailable {
		reason := priority.ReasonQuotaUnavailable
		if unavailableReason != "" {
			reason += ": " + unavailableReason
		}
		return candidateClassification{
			score:        priority.DepletedPriority,
			status:       apply.RoutingStatusPoolUnavailable,
			poolEligible: false,
			reason:       reason,
		}
	}

	if item.HasQuotaEvidence() {
		switch item.Reason {
		case priority.ReasonFreshWeeklyDepleted:
			return candidateClassification{
				score:        priority.DepletedPriority,
				status:       apply.RoutingStatusWeeklyDepleted,
				poolEligible: true,
				reason:       formatReasonWithProbeFailure(item.Reason, probeObs, hasProbeFailure),
			}
		case priority.ReasonFreshShortWindowDepleted:
			return candidateClassification{
				score:        priority.DepletedPriority,
				status:       apply.RoutingStatusShortDepleted,
				poolEligible: true,
				reason:       formatReasonWithProbeFailure(item.Reason, probeObs, hasProbeFailure),
			}
		default:
			return candidateClassification{
				score:        item.Priority,
				status:       apply.RoutingStatusActive,
				poolEligible: true,
				isBoosted:    item.IsBoosted,
				reason:       formatReasonWithProbeFailure(item.Reason, probeObs, hasProbeFailure),
			}
		}
	}

	return candidateClassification{
		score:        coldStartRoutingTier,
		status:       apply.RoutingStatusColdStart,
		poolEligible: true,
		reason:       formatReasonWithProbeFailure(item.Reason, probeObs, hasProbeFailure),
	}
}

func formatReasonWithProbeFailure(baseReason string, obs evidence.Observation, hasProbeFailure bool) string {
	if !hasProbeFailure {
		return baseReason
	}
	reason := "probe failed"
	if obs.Kind == evidence.ObservationInvalid {
		reason = "probe invalid"
	}
	if failure := strings.TrimSpace(obs.Failure); failure != "" {
		reason += ": " + failure
	}
	if baseReason != "" && baseReason != priority.ReasonKeepCurrentState && baseReason != priority.ReasonDisabledOnHost {
		reason += "; " + baseReason
	}
	return reason
}

func redactCandidateReason(reason string) string {
	if reason == "" {
		return ""
	}
	return host.RedactBytes([]byte(reason))
}

func resolveCooldownsForPool(
	pool config.AntigravityModelGroup,
	store *state.Store,
	options priority.Options,
	now time.Time,
) map[string]poolCooldownInfo {
	if store != nil {
		entries := store.GetActiveCooldownEntriesForGroup(now, string(pool))
		if len(entries) == 0 {
			return nil
		}
		resolved := make(map[string]poolCooldownInfo, len(entries))
		for authIndex, entry := range entries {
			resolved[authIndex] = poolCooldownInfo{
				until:  entry.CooldownUntil,
				reason: entry.Reason,
			}
		}
		return resolved
	}

	if len(options.CooldownAuthIndexes) == 0 {
		return nil
	}

	normGroup := strings.ToLower(string(pool))
	resolved := make(map[string]poolCooldownInfo)
	isAccountLevel := make(map[string]bool)

	for key, until := range options.CooldownAuthIndexes {
		if !now.Before(until) {
			continue
		}
		authIndex := key
		entryAccountLevel := true
		if prefix, suffix, found := strings.Cut(key, "::"); found {
			if strings.ToLower(strings.TrimSpace(suffix)) != normGroup {
				continue
			}
			authIndex = prefix
			entryAccountLevel = false
		}
		reason := options.CooldownReasonsByKey[key]
		if reason == "" {
			reason = options.CooldownReasons[authIndex]
		}
		existing, exists := resolved[authIndex]
		if !exists {
			resolved[authIndex] = poolCooldownInfo{until: until, reason: reason}
			isAccountLevel[authIndex] = entryAccountLevel
			continue
		}
		if isAccountLevel[authIndex] {
			continue
		}
		if entryAccountLevel || until.After(existing.until) {
			resolved[authIndex] = poolCooldownInfo{until: until, reason: reason}
			isAccountLevel[authIndex] = entryAccountLevel
		}
	}
	return resolved
}

func planningOptionsForEngine(cfg config.Config, store *state.Store, now time.Time) priority.Options {
	var customTiers map[string]int
	if store != nil {
		customTiers = store.GetCustomTiers()
	}
	if len(customTiers) == 0 && len(cfg.CustomTiers) > 0 {
		customTiers = make(map[string]int, len(cfg.CustomTiers))
		maps.Copy(customTiers, cfg.CustomTiers)
	}
	return priority.Options{
		Now:                 now,
		BoostStartPriority:  cfg.PriorityRules.BoostStartPriority,
		NormalStartPriority: cfg.PriorityRules.NormalStartPriority,
		FreeStartPriority:   cfg.PriorityRules.FreeStartPriority,
		UrgencyTolerance:    cfg.UrgencyTolerance,
		CustomTiers:         customTiers,
	}
}
