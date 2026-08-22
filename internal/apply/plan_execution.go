package apply

import (
	"context"
	"errors"
	"fmt"

	"antigravity-priority/internal/priority"
)

// ExecutePlan sends Planner changes through the single Host Transition seam.
// Planner gates remain in the Planner; this function only translates changes,
// executes them, and projects credential outcomes into the existing result
// shape used by Runtime and management.
func ExecutePlan(ctx context.Context, transition HostTransition, plan priority.Plan, reportSkippedPlan bool) (Result, error) {
	result := Result{
		Snapshot: newPlanSnapshot(plan),
		Changes:  make([]ChangeResult, 0, len(plan.Changes)+len(plan.Items)),
		Record:   RecordResult{Status: RecordNotAttempted},
	}
	intents := make([]TransitionIntent, 0, len(plan.Changes))
	activeChanges := make([]priority.Change, 0, len(plan.Changes))
	activePosition := make(map[int]int, len(plan.Changes))
	for index, change := range plan.Changes {
		if !change.EvidenceFresh {
			continue
		}
		activePosition[index] = len(activeChanges)
		intents = append(intents, IntentFromChange(change))
		activeChanges = append(activeChanges, change)
	}
	if len(intents) > 0 && transition == nil {
		return result, ErrMissingTransition
	}
	transitionResult := TransitionRoundResult{Details: []TransitionResult{}}
	var err error
	if transition != nil {
		transitionResult, err = transition.Execute(ctx, TransitionRound{Intents: intents})
		if err != nil {
			return result, fmt.Errorf("execute Host Transition round: %w", err)
		}
	}
	transitionResult.Totals = TransitionTotals{}
	for _, detail := range transitionResult.Details {
		transitionResult.Totals.add(detail.Outcome)
	}
	result.Transitions = transitionResult
	eventPlan := plan
	eventPlan.Changes = activeChanges
	result.Event = executionEvent(eventPlan, transitionResult)

	for planIndex, change := range plan.Changes {
		activeIndex, active := activePosition[planIndex]
		if !active {
			projected := skippedPlanChangeResult(change)
			result.Changes = append(result.Changes, projected)
			summarizeChange(&result, projected)
			continue
		}
		if activeIndex >= len(transitionResult.Details) {
			failure := FailureResult(priority.PlanItem{Credential: change.Credential, Priority: change.Priority, Disabled: change.Disabled, EvidenceFresh: change.EvidenceFresh, Reason: change.Reason}, errors.New("host transition omitted credential result"))
			result.Changes = append(result.Changes, failure)
			summarizeChange(&result, failure)
			continue
		}
		projected := transitionChangeResult(change, transitionResult.Details[activeIndex])
		result.Changes = append(result.Changes, projected)
		summarizeChange(&result, projected)
	}

	if reportSkippedPlan {
		changed := make(map[string]struct{}, len(plan.Changes))
		for _, change := range plan.Changes {
			changed[change.Credential.AuthIndex] = struct{}{}
		}
		for _, item := range plan.Items {
			if _, ok := changed[item.Credential.AuthIndex]; ok {
				continue
			}
			projected := skippedPlanItemResult(item)
			result.Changes = append(result.Changes, projected)
			summarizeChange(&result, projected)
		}
	}
	return result, nil
}

func skippedPlanChangeResult(change priority.Change) ChangeResult {
	result := newChangeResult(change)
	result.Status = ChangeStatusSkipped
	return result
}

// ExecuteRound runs explicit operational intents such as 429 Reactive
// Cooldown and priority reset and projects their details into a Result.
func ExecuteRound(ctx context.Context, transition HostTransition, round TransitionRound) (Result, error) {
	if transition == nil {
		return Result{Record: RecordResult{Status: RecordNotAttempted}}, ErrMissingTransition
	}
	transitionResult, err := transition.Execute(ctx, round)
	if err != nil {
		return Result{}, fmt.Errorf("execute Host Transition round: %w", err)
	}
	return ResultFromTransition(transitionResult), nil
}

// ResultFromTransition derives all legacy-compatible counters from transition
// details. It is also the projection seam for operational transitions that do
// not originate from a Planner Change.
func ResultFromTransition(round TransitionRoundResult) Result {
	derived := TransitionTotals{}
	for _, detail := range round.Details {
		derived.add(detail.Outcome)
	}
	round.Totals = derived
	result := Result{
		Transitions: round,
		Changes:     make([]ChangeResult, 0, len(round.Details)),
		Record:      RecordResult{Status: RecordNotAttempted},
	}
	result.Event = eventFromTransitionDetails(round.Details)
	for _, detail := range round.Details {
		change := ChangeResult{
			Name:              detail.Name,
			AuthIndex:         detail.AuthIndex,
			Status:            ChangeStatus(detail.Outcome),
			HostOutcome:       detail.Outcome,
			Success:           detail.Outcome == OutcomeCommitted || detail.Outcome == OutcomeNoChange,
			Reason:            detail.Reason,
			PriorityAttempted: detail.PriorityAttempted,
			DisabledAttempted: detail.DisabledAttempted,
			Error:             detail.Error,
			PriorityFrom:      detail.Before.Priority,
			PriorityMissing:   !detail.Before.PriorityPresent,
			PriorityTo:        detail.Target.Priority,
			DisabledFrom:      detail.Before.Disabled,
			DisabledTo:        detail.Target.Disabled,
		}
		result.Changes = append(result.Changes, change)
		summarizeChange(&result, change)
	}
	return result
}

func transitionChangeResult(change priority.Change, detail TransitionResult) ChangeResult {
	result := newChangeResult(change)
	result.Name = redactIdentifier(resultName(change.Credential))
	result.AuthIndex = redactIdentifier(change.Credential.AuthIndex)
	result.Account = redactIdentifier(change.Credential.Account)
	result.Email = redactIdentifier(change.Credential.Email)
	result.Status = ChangeStatus(detail.Outcome)
	result.HostOutcome = detail.Outcome
	result.Success = detail.Outcome == OutcomeCommitted || detail.Outcome == OutcomeNoChange
	result.PriorityAttempted = detail.PriorityAttempted
	result.DisabledAttempted = detail.DisabledAttempted
	result.Error = detail.Error
	result.Reason = detail.Reason
	return result
}

func eventFromTransitionDetails(details []TransitionResult) AuditEvent {
	event := AuditEvent{
		Action:       "host.transition",
		TotalChanges: len(details),
		Changes:      make([]AuditChange, 0, len(details)),
	}
	for _, detail := range details {
		if detail.Outcome == OutcomeCommitted || detail.Outcome == OutcomeNoChange {
			event.FreshChanges++
		} else {
			event.SkippedChanges++
		}
		event.Changes = append(event.Changes, AuditChange{
			Name:          detail.Name,
			AuthIndex:     detail.AuthIndex,
			EvidenceFresh: true,
			Reason:        detail.Reason,
			Outcome:       detail.Outcome,
			Cause:         detail.Cause,
		})
	}
	return event
}

func executionEvent(plan priority.Plan, round TransitionRoundResult) AuditEvent {
	event := AuditEvent{
		Action:       "host.transition",
		TotalChanges: len(plan.Changes),
		Changes:      make([]AuditChange, 0, len(round.Details)),
	}
	for index, detail := range round.Details {
		if detail.Outcome == OutcomeCommitted || detail.Outcome == OutcomeNoChange {
			event.FreshChanges++
		} else {
			event.SkippedChanges++
		}
		change := AuditChange{
			EvidenceFresh: index < len(plan.Changes) && plan.Changes[index].EvidenceFresh,
			Outcome:       detail.Outcome,
			Reason:        detail.Reason,
			Cause:         detail.Cause,
		}
		if index < len(plan.Changes) {
			change.Name = redactIdentifier(resultName(plan.Changes[index].Credential))
			change.AuthIndex = redactIdentifier(plan.Changes[index].Credential.AuthIndex)
		}
		event.Changes = append(event.Changes, change)
	}
	return event
}
