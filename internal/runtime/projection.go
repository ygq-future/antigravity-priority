package runtime

import (
	"fmt"
	"strings"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/priority"
)

// ProjectionInput contains the complete in-process input for one dual-group
// projection. Evidence is already classified by the caller's evidence
// authority; projection only applies the same planning inputs to each group.
// None of the supplied slices or maps are mutated.
type ProjectionInput struct {
	ControlModelGroup config.AntigravityModelGroup
	Credentials       []core.Credential
	EvidenceByGroup   map[config.AntigravityModelGroup][]priority.ProbeEvidence
	PlanningOptions   priority.Options
	ProjectionTime    time.Time
}

// DualModelGroupProjection is the only projection result exposed to Runtime
// callers. The predicted Plan is intentionally not returned, so it cannot be
// passed to Host Transition by an Apply use case.
type DualModelGroupProjection struct {
	ControlPlan     priority.Plan
	ControlSnapshot apply.PlanSnapshot
	Snapshot        apply.DualGroupSnapshot
}

// ProjectDualModelGroups calculates the configured control plan and both
// model-group snapshots from one immutable input set.
func ProjectDualModelGroups(input ProjectionInput) (DualModelGroupProjection, error) {
	controlGroup, err := canonicalModelGroup(input.ControlModelGroup)
	if err != nil {
		return DualModelGroupProjection{}, err
	}
	if input.ProjectionTime.IsZero() {
		return DualModelGroupProjection{}, fmt.Errorf("runtime: projection time is required")
	}

	projectionTime := input.ProjectionTime.UTC()
	alternateGroup := alternateProjectionGroup(controlGroup)
	options := clonePlanningOptions(input.PlanningOptions)
	options.Now = projectionTime

	controlPlan := priority.PlanFreshOnly(
		cloneCredentials(input.Credentials),
		cloneEvidence(input.EvidenceByGroup[controlGroup]),
		options,
	)
	predictedPlan := priority.PlanFreshOnly(
		cloneCredentials(input.Credentials),
		cloneEvidence(input.EvidenceByGroup[alternateGroup]),
		options,
	)

	controlSnapshot := snapshotForProjectionRole(controlPlan, false)
	predictedSnapshot := snapshotForProjectionRole(predictedPlan, true)

	return DualModelGroupProjection{
		ControlPlan:     controlPlan,
		ControlSnapshot: controlSnapshot,
		Snapshot: apply.DualGroupSnapshot{
			ActiveModelGroup: string(controlGroup),
			ObservedAt:       projectionTime,
			Groups: map[string]apply.GroupSnapshot{
				string(controlGroup): {
					Items:   controlSnapshot.Items,
					Changes: controlSnapshot.Changes,
				},
				string(alternateGroup): {
					Items:   predictedSnapshot.Items,
					Changes: predictedSnapshot.Changes,
				},
			},
		},
	}, nil
}

func canonicalModelGroup(group config.AntigravityModelGroup) (config.AntigravityModelGroup, error) {
	parsed, err := config.ParseAntigravityModelGroup(string(group))
	if err != nil {
		return "", err
	}
	return parsed, nil
}

func alternateProjectionGroup(group config.AntigravityModelGroup) config.AntigravityModelGroup {
	if group == config.AntigravityModelGroupClaudeGPT {
		return config.AntigravityModelGroupGemini
	}
	return config.AntigravityModelGroupClaudeGPT
}

func snapshotForProjectionRole(plan priority.Plan, predicted bool) apply.PlanSnapshot {
	return snapshotForProjectionRoleSnapshot(apply.Snapshot(plan), predicted)
}

func snapshotForProjectionRoleSnapshot(snapshot apply.PlanSnapshot, predicted bool) apply.PlanSnapshot {
	for index := range snapshot.Items {
		snapshot.Items[index].IsPredicted = predicted
		snapshot.Items[index].Reason = roleReason(snapshot.Items[index].Reason, predicted)
	}
	for index := range snapshot.Changes {
		snapshot.Changes[index].Reason = roleReason(snapshot.Changes[index].Reason, predicted)
	}
	return snapshot
}

func roleReason(reason string, predicted bool) string {
	reason = strings.TrimPrefix(reason, "predicted: ")
	if predicted && reason != "" && reason != priority.ReasonKeepCurrentState {
		return "predicted: " + reason
	}
	return reason
}

func clonePlanningOptions(options priority.Options) priority.Options {
	options.CooldownAuthIndexes = cloneCooldowns(options.CooldownAuthIndexes)
	return options
}

func cloneCooldowns(cooldowns map[string]time.Time) map[string]time.Time {
	if len(cooldowns) == 0 {
		return nil
	}
	cloned := make(map[string]time.Time, len(cooldowns))
	for authIndex, until := range cooldowns {
		cloned[authIndex] = until
	}
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

func cloneEvidence(evidence []priority.ProbeEvidence) []priority.ProbeEvidence {
	if evidence == nil {
		return nil
	}
	cloned := make([]priority.ProbeEvidence, len(evidence))
	for index, item := range evidence {
		cloned[index] = item
		cloned[index].ResetAt = cloneTimePointer(item.ResetAt)
		cloned[index].ShortWindowResetAt = cloneTimePointer(item.ShortWindowResetAt)
		cloned[index].LongWindowResetAt = cloneTimePointer(item.LongWindowResetAt)
		cloned[index].Remaining = cloneInt64Pointer(item.Remaining)
		cloned[index].ShortWindowRemaining = cloneInt64Pointer(item.ShortWindowRemaining)
		cloned[index].LongWindowRemaining = cloneInt64Pointer(item.LongWindowRemaining)
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

func cloneDualGroupSnapshot(snapshot apply.DualGroupSnapshot) apply.DualGroupSnapshot {
	cloned := snapshot
	cloned.Groups = make(map[string]apply.GroupSnapshot, len(snapshot.Groups))
	for group, groupSnapshot := range snapshot.Groups {
		clonedGroup := groupSnapshot
		items := make([]apply.SnapshotItem, len(groupSnapshot.Items))
		for index, item := range groupSnapshot.Items {
			items[index] = item
			items[index].ResetAt = cloneTimePointer(item.ResetAt)
			items[index].ShortWindowResetAt = cloneTimePointer(item.ShortWindowResetAt)
			items[index].LongWindowResetAt = cloneTimePointer(item.LongWindowResetAt)
			items[index].ShortWindowRemaining = cloneInt64Pointer(item.ShortWindowRemaining)
			items[index].LongWindowRemaining = cloneInt64Pointer(item.LongWindowRemaining)
		}
		clonedGroup.Items = items
		clonedGroup.Changes = make([]apply.SnapshotChange, len(groupSnapshot.Changes))
		copy(clonedGroup.Changes, groupSnapshot.Changes)
		cloned.Groups[group] = clonedGroup
	}
	return cloned
}
