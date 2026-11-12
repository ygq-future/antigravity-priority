package runtime

import (
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/priority"
)

// ProjectionInput contains the complete in-process input for one three-pool
// projection. Evidence is already classified by the caller's evidence
// authority; projection only applies the same planning inputs to each pool.
// None of the supplied slices or maps are mutated.
type ProjectionInput struct {
	Credentials     []core.Credential
	EvidenceByGroup map[config.AntigravityModelGroup]evidence.Result
	PlanningOptions priority.Options
	ProjectionTime  time.Time
	// Warmup carries the global pool toggles and the card-level switches so
	// every pool snapshot renders the same effective warm-up state.
	Warmup config.WarmupConfig
}

// ModelGroupProjection is the projection result exposed to Runtime callers.
type ModelGroupProjection struct {
	Snapshot apply.SnapshotSet
}

// ProjectModelGroups calculates all three model-group snapshots from one
// immutable input set.
func ProjectModelGroups(input ProjectionInput) (ModelGroupProjection, error) {
	if input.ProjectionTime.IsZero() {
		return ModelGroupProjection{}, fmt.Errorf("runtime: projection time is required")
	}

	projectionTime := input.ProjectionTime.UTC()
	options := clonePlanningOptions(input.PlanningOptions)
	options.Now = projectionTime

	snapshotsByGroup := make(map[config.AntigravityModelGroup]apply.PlanSnapshot, len(config.AllAntigravityModelGroups))

	for _, group := range config.AllAntigravityModelGroups {
		grpEvidence := cloneEvidence(input.EvidenceByGroup[group])
		grpOptions := clonePlanningOptions(options)
		grpOptions.CooldownAuthIndexes = filterCooldownsForGroup(options.CooldownAuthIndexes, group)
		grpOptions.CooldownReasons = filterCooldownReasonsForGroup(options.CooldownReasonsByKey, group)
		plan := priority.PlanWithEvidence(
			cloneCredentials(input.Credentials),
			grpEvidence,
			grpOptions,
		)
		annotatePlanObservations(&plan, grpEvidence.Observations)
		snapshotsByGroup[group] = apply.Snapshot(plan)
	}

	// Cross-reference 5 quota windows across all groups for every credential.
	// SnapshotItem.AuthIndex is redacted for display, so two credentials can
	// share it (for example every "dev-auth-*" becomes "dev-***"). Internal
	// correlation must use the trusted in-memory identity instead.
	quotaWindowsByAuth := buildAllQuotaWindows(snapshotsByGroup)
	for _, group := range config.AllAntigravityModelGroups {
		snap := snapshotsByGroup[group]
		for index := range snap.Items {
			authIndex := snapshotAuthIndex(snap.Items[index])
			if qw, ok := quotaWindowsByAuth[authIndex]; ok {
				snap.Items[index].QuotaWindows = qw
			}
			_, overridden := input.Warmup.WarmupOverride(authIndex)
			isFree := snap.Items[index].PlanType == string(core.PlanTypeFree)
			snap.Items[index].WarmupOverridden = overridden && !isFree
			snap.Items[index].WarmupActive = !isFree && warmupEffectiveEnabled(input.Warmup, authIndex, group)
		}
		snapshotsByGroup[group] = snap
	}

	groups := make(map[string]apply.PoolSnapshot, len(config.AllAntigravityModelGroups))
	for _, group := range config.AllAntigravityModelGroups {
		snap := snapshotsByGroup[group]
		groups[string(group)] = apply.PoolSnapshot{
			Items: snap.Items,
		}
	}

	return ModelGroupProjection{
		Snapshot: apply.SnapshotSet{
			ObservedAt: projectionTime,
			Groups:     groups,
		},
	}, nil
}

// warmupEnabledForGroup reports whether the global warm-up toggle covers this
// pool. Argon shares Gemini's physical 5h window, so it follows the Gemini
// toggle.
func warmupEnabledForGroup(warmup config.WarmupConfig, group config.AntigravityModelGroup) bool {
	switch group {
	case config.AntigravityModelGroupClaudeGPT:
		return warmup.ClaudeEnabled
	case config.AntigravityModelGroupGemini, config.AntigravityModelGroupArgon:
		return warmup.GeminiEnabled
	default:
		return false
	}
}

// warmupEffectiveEnabled reports the effective warm-up state for one pool:
// a card-level override wins over the global pool toggle.
func warmupEffectiveEnabled(warmup config.WarmupConfig, authIndex string, group config.AntigravityModelGroup) bool {
	if enabled, ok := warmup.WarmupOverride(authIndex); ok {
		return enabled
	}
	return warmupEnabledForGroup(warmup, group)
}

func filterCooldownsForGroup(cooldowns map[string]time.Time, group config.AntigravityModelGroup) map[string]time.Time {
	if len(cooldowns) == 0 {
		return nil
	}
	normGroup := strings.ToLower(string(group))
	filtered := make(map[string]time.Time)
	for k, v := range cooldowns {
		authIndex := k
		if prefix, suffix, found := strings.Cut(k, "::"); found {
			if strings.ToLower(strings.TrimSpace(suffix)) != normGroup {
				continue
			}
			authIndex = prefix
		}
		if existing, ok := filtered[authIndex]; !ok || v.After(existing) {
			filtered[authIndex] = v
		}
	}
	return filtered
}

func filterCooldownReasonsForGroup(reasons map[string]string, group config.AntigravityModelGroup) map[string]string {
	if len(reasons) == 0 {
		return nil
	}
	normGroup := strings.ToLower(string(group))
	filtered := make(map[string]string)
	for k, v := range reasons {
		authIndex := k
		if prefix, suffix, found := strings.Cut(k, "::"); found {
			if strings.ToLower(strings.TrimSpace(suffix)) != normGroup {
				continue
			}
			authIndex = prefix
		}
		filtered[authIndex] = v
	}
	return filtered
}

func buildAllQuotaWindows(snapshots map[config.AntigravityModelGroup]apply.PlanSnapshot) map[string]apply.AllQuotaWindows {
	result := make(map[string]apply.AllQuotaWindows)

	geminiSnap := snapshots[config.AntigravityModelGroupGemini]
	for _, item := range geminiSnap.Items {
		authIndex := snapshotAuthIndex(item)
		qw := result[authIndex]
		qw.Gemini5h = itemShortWindowBar(item)
		qw.Gemini7d = itemLongWindowBar(item)
		if item.ArgonWindowRemaining != nil && item.PlanType != string(core.PlanTypeFree) {
			qw.Argon7d = apply.QuotaWindowBar{
				Remaining: cloneInt64Pointer(item.ArgonWindowRemaining),
				ResetAt:   cloneTimePointer(item.ArgonWindowResetAt),
			}
		}
		result[authIndex] = qw
	}

	claudeSnap := snapshots[config.AntigravityModelGroupClaudeGPT]
	for _, item := range claudeSnap.Items {
		authIndex := snapshotAuthIndex(item)
		qw := result[authIndex]
		qw.Claude5h = itemShortWindowBar(item)
		qw.Claude7d = itemLongWindowBar(item)
		result[authIndex] = qw
	}

	argonSnap := snapshots[config.AntigravityModelGroupArgon]
	for _, item := range argonSnap.Items {
		authIndex := snapshotAuthIndex(item)
		qw := result[authIndex]
		argonBar := itemArgonWindowBar(item)
		if argonBar.Remaining != nil || qw.Argon7d.Remaining == nil {
			qw.Argon7d = argonBar
		}
		if item.ArgonWindowRemaining == nil && item.LongWindowRemaining != nil && item.PlanType != string(core.PlanTypeFree) {
			qw.ArgonSharedWithGemini = true
		}
		result[authIndex] = qw
	}

	return result
}

// snapshotAuthIndex returns the unredacted credential identity for internal
// correlation. Display fields are redacted and may collide across credentials.
func snapshotAuthIndex(item apply.SnapshotItem) string {
	if item.Identity.AuthIndex != "" {
		return item.Identity.AuthIndex
	}
	return item.AuthIndex
}

func itemShortWindowBar(item apply.SnapshotItem) apply.QuotaWindowBar {
	rem := item.ShortWindowRemaining
	resetAt := item.ShortWindowResetAt
	if rem == nil && item.R5h > 0 {
		val := int64(math.Round(item.R5h * 100.0))
		rem = &val
	}
	if resetAt == nil {
		resetAt = item.ResetAt
	}
	return apply.QuotaWindowBar{
		Remaining: cloneInt64Pointer(rem),
		ResetAt:   cloneTimePointer(resetAt),
	}
}

func itemLongWindowBar(item apply.SnapshotItem) apply.QuotaWindowBar {
	rem := item.LongWindowRemaining
	resetAt := item.LongWindowResetAt
	if rem == nil && item.R7d > 0 {
		val := int64(math.Round(item.R7d * 100.0))
		rem = &val
	}
	if resetAt == nil {
		resetAt = item.ResetAt
	}
	return apply.QuotaWindowBar{
		Remaining: cloneInt64Pointer(rem),
		ResetAt:   cloneTimePointer(resetAt),
	}
}

func itemArgonWindowBar(item apply.SnapshotItem) apply.QuotaWindowBar {
	if item.PlanType == string(core.PlanTypeFree) {
		return apply.QuotaWindowBar{}
	}
	return apply.QuotaWindowBar{
		Remaining: cloneInt64Pointer(item.ArgonWindowRemaining),
		ResetAt:   cloneTimePointer(item.ArgonWindowResetAt),
	}
}

func annotatePlanObservations(plan *priority.Plan, observations []evidence.Observation) {
	for _, observation := range observations {
		var reason string
		switch observation.Kind {
		case evidence.ObservationFailed:
			reason = "probe failed"
		case evidence.ObservationInvalid:
			reason = "probe invalid"
		case evidence.ObservationUnavailable:
			reason = priority.ReasonQuotaUnavailable
		default:
			continue
		}
		if failure := strings.TrimSpace(observation.Failure); failure != "" {
			reason += ": " + failure
		}
		for index := range plan.Items {
			if plan.Items[index].Credential.AuthIndex != observation.AuthIndex {
				continue
			}
			if observation.Kind == evidence.ObservationUnavailable {
				plan.Items[index].Priority = priority.DepletedPriority
			}
			if plan.Items[index].Reason != priority.ReasonKeepCurrentState && plan.Items[index].Reason != priority.ReasonDisabledOnHost {
				reason += "; " + plan.Items[index].Reason
			}
			plan.Items[index].Reason = reason
		}
	}
}

func clonePlanningOptions(options priority.Options) priority.Options {
	options.CooldownAuthIndexes = cloneCooldowns(options.CooldownAuthIndexes)
	options.CooldownReasons = cloneStringMap(options.CooldownReasons)
	options.CooldownReasonsByKey = cloneStringMap(options.CooldownReasonsByKey)
	if len(options.CustomTiers) > 0 {
		cloned := make(map[string]int, len(options.CustomTiers))
		maps.Copy(cloned, options.CustomTiers)
		options.CustomTiers = cloned
	}
	return options
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	maps.Copy(cloned, values)
	return cloned
}

func cloneCooldowns(cooldowns map[string]time.Time) map[string]time.Time {
	if len(cooldowns) == 0 {
		return nil
	}
	cloned := make(map[string]time.Time, len(cooldowns))
	maps.Copy(cloned, cooldowns)
	return cloned
}

func cloneCredentials(credentials []core.Credential) []core.Credential {
	if credentials == nil {
		return nil
	}
	cloned := make([]core.Credential, len(credentials))
	for index, credential := range credentials {
		cloned[index] = credential
		cloned[index].RawJSON = append([]byte(nil), credential.RawJSON...)
	}
	return cloned
}

func cloneEvidence(result evidence.Result) evidence.Result {
	cloned := evidence.Result{
		Eligible:     cloneQuotaEvidence(result.Eligible),
		Observations: make([]evidence.Observation, len(result.Observations)),
	}
	for index, observation := range result.Observations {
		cloned.Observations[index] = observation
		if observation.Evidence != nil {
			copy := cloneQuotaEvidence([]priority.QuotaEvidence{*observation.Evidence})[0]
			cloned.Observations[index].Evidence = &copy
		}
	}
	return cloned
}

func cloneQuotaEvidence(evidence []priority.QuotaEvidence) []priority.QuotaEvidence {
	if evidence == nil {
		return nil
	}
	cloned := make([]priority.QuotaEvidence, len(evidence))
	for index, item := range evidence {
		cloned[index] = item
		cloned[index].ResetAt = cloneTimePointer(item.ResetAt)
		cloned[index].ShortWindowResetAt = cloneTimePointer(item.ShortWindowResetAt)
		cloned[index].LongWindowResetAt = cloneTimePointer(item.LongWindowResetAt)
		cloned[index].ArgonWindowResetAt = cloneTimePointer(item.ArgonWindowResetAt)
		cloned[index].Remaining = cloneInt64Pointer(item.Remaining)
		cloned[index].ShortWindowRemaining = cloneInt64Pointer(item.ShortWindowRemaining)
		cloned[index].LongWindowRemaining = cloneInt64Pointer(item.LongWindowRemaining)
		cloned[index].ArgonWindowRemaining = cloneInt64Pointer(item.ArgonWindowRemaining)
	}
	return cloned
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneSnapshotSet(snapshot apply.SnapshotSet) apply.SnapshotSet {
	cloned := snapshot
	if snapshot.Models != nil {
		cloned.Models = make([]apply.ModelOption, len(snapshot.Models))
		copy(cloned.Models, snapshot.Models)
	}
	cloned.Groups = make(map[string]apply.PoolSnapshot, len(snapshot.Groups))
	for group, groupSnapshot := range snapshot.Groups {
		clonedGroup := groupSnapshot
		items := make([]apply.SnapshotItem, len(groupSnapshot.Items))
		for index, item := range groupSnapshot.Items {
			items[index] = item
			items[index].ResetAt = cloneTimePointer(item.ResetAt)
			items[index].ShortWindowResetAt = cloneTimePointer(item.ShortWindowResetAt)
			items[index].LongWindowResetAt = cloneTimePointer(item.LongWindowResetAt)
			items[index].ArgonWindowResetAt = cloneTimePointer(item.ArgonWindowResetAt)
			items[index].ShortWindowRemaining = cloneInt64Pointer(item.ShortWindowRemaining)
			items[index].LongWindowRemaining = cloneInt64Pointer(item.LongWindowRemaining)
			items[index].ArgonWindowRemaining = cloneInt64Pointer(item.ArgonWindowRemaining)
			items[index].QuotaWindows.Gemini5h.Remaining = cloneInt64Pointer(item.QuotaWindows.Gemini5h.Remaining)
			items[index].QuotaWindows.Gemini5h.ResetAt = cloneTimePointer(item.QuotaWindows.Gemini5h.ResetAt)
			items[index].QuotaWindows.Gemini7d.Remaining = cloneInt64Pointer(item.QuotaWindows.Gemini7d.Remaining)
			items[index].QuotaWindows.Gemini7d.ResetAt = cloneTimePointer(item.QuotaWindows.Gemini7d.ResetAt)
			items[index].QuotaWindows.Claude5h.Remaining = cloneInt64Pointer(item.QuotaWindows.Claude5h.Remaining)
			items[index].QuotaWindows.Claude5h.ResetAt = cloneTimePointer(item.QuotaWindows.Claude5h.ResetAt)
			items[index].QuotaWindows.Claude7d.Remaining = cloneInt64Pointer(item.QuotaWindows.Claude7d.Remaining)
			items[index].QuotaWindows.Claude7d.ResetAt = cloneTimePointer(item.QuotaWindows.Claude7d.ResetAt)
			items[index].QuotaWindows.Argon7d.Remaining = cloneInt64Pointer(item.QuotaWindows.Argon7d.Remaining)
			items[index].QuotaWindows.Argon7d.ResetAt = cloneTimePointer(item.QuotaWindows.Argon7d.ResetAt)
			if item.SupportedModels != nil {
				items[index].SupportedModels = make([]string, len(item.SupportedModels))
				copy(items[index].SupportedModels, item.SupportedModels)
			}
		}
		clonedGroup.Items = items
		cloned.Groups[group] = clonedGroup
	}
	return cloned
}
