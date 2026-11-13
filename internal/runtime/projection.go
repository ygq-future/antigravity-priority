package runtime

import (
	"errors"
	"maps"
	"math"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/priority"
)

var errProjectionTimeRequired = errors.New("runtime: projection time is required")

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
// immutable input set via the unified routing and snapshot engine.
func ProjectModelGroups(input ProjectionInput) (ModelGroupProjection, error) {
	if input.ProjectionTime.IsZero() {
		return ModelGroupProjection{}, errProjectionTimeRequired
	}
	evidenceByGroup := input.EvidenceByGroup
	if evidenceByGroup == nil {
		evidenceByGroup = map[config.AntigravityModelGroup]evidence.Result{}
	}
	options := input.PlanningOptions
	warmup := input.Warmup
	out, err := EvaluateRoutingAndSnapshot(RoutingEngineInput{
		Credentials:     input.Credentials,
		EvidenceByGroup: evidenceByGroup,
		PlanningOptions: &options,
		Warmup:          &warmup,
		Now:             input.ProjectionTime,
	})
	if err != nil {
		return ModelGroupProjection{}, err
	}
	return ModelGroupProjection{Snapshot: out.Snapshot}, nil
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

func cloneBoolPointer(value *bool) *bool {
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
			items[index].CooldownUntil = cloneTimePointer(item.CooldownUntil)
			items[index].PoolEligible = cloneBoolPointer(item.PoolEligible)
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
