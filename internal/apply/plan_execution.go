package apply

import (
	"context"
	"fmt"
)

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

func eventFromTransitionDetails(details []TransitionResult) AuditEvent {
	event := AuditEvent{
		Action:       "host.transition",
		TotalChanges: len(details),
		Changes:      make([]AuditChange, 0, len(details)),
	}
	for _, detail := range details {
		event.Changes = append(event.Changes, AuditChange{
			Name:      detail.Name,
			AuthIndex: detail.AuthIndex,
			Reason:    detail.Reason,
			Outcome:   detail.Outcome,
			Cause:     detail.Cause,
		})
	}
	return event
}
