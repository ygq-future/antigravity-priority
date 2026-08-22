package apply

import (
	"context"
	"errors"

	"antigravity-priority/internal/priority"
)

// ChangeStatus represents the execution status of an individual planned change.
type ChangeStatus string

const (
	// ChangeStatusSkipped indicates the change lacked fresh evidence or difference, skipping write-back.
	ChangeStatusSkipped ChangeStatus = "skipped"
	// ChangeStatusNoChange indicates the requested target was already satisfied.
	ChangeStatusNoChange ChangeStatus = ChangeStatus(OutcomeNoChange)
	// ChangeStatusCommitted indicates the resulting Host state was verified.
	ChangeStatusCommitted ChangeStatus = ChangeStatus(OutcomeCommitted)
	// ChangeStatusSuccess is retained as a source-compatible alias for committed.
	ChangeStatusSuccess ChangeStatus = ChangeStatusCommitted
	// ChangeStatusFailed indicates the Host transition did not commit.
	ChangeStatusFailed ChangeStatus = ChangeStatus(OutcomeFailed)
	// ChangeStatusConflict indicates newer decision-relevant Host state won.
	ChangeStatusConflict ChangeStatus = ChangeStatus(OutcomeConflict)
	// ChangeStatusUncertain indicates the post-commit Host state could not be proven.
	ChangeStatusUncertain ChangeStatus = ChangeStatus(OutcomeUncertain)
)

// ErrMissingTransition indicates that a Host-changing execution did not
// provide the single Host Transition seam.
var ErrMissingTransition = errors.New("apply: host transition is required")

// Request holds the input required to execute an apply round.
type Request struct {
	Transition        HostTransition
	Plan              priority.Plan
	ReportSkippedPlan bool
}

// Result summarizes the execution result of an apply round.
type Result struct {
	Snapshot    PlanSnapshot          `json:"snapshot"`
	Event       AuditEvent            `json:"event"`
	Changes     []ChangeResult        `json:"changes"`
	Transitions TransitionRoundResult `json:"transitions"`
	Record      RecordResult          `json:"record"`
	Attempted   int                   `json:"attempted"`
	Succeeded   int                   `json:"succeeded"`
	Failed      int                   `json:"failed"`
	Skipped     int                   `json:"skipped"`
	NoChange    int                   `json:"no_change"`
	Conflicts   int                   `json:"conflicts"`
	Uncertain   int                   `json:"uncertain"`
}

// RecordStatus describes persistence of execution history separately from the
// Host outcome that produced it.
type RecordStatus string

const (
	RecordNotAttempted RecordStatus = "not_attempted"
	RecordPersisted    RecordStatus = "persisted"
	RecordFailed       RecordStatus = "failed"
)

// RecordResult keeps record-persistence health independent from Host truth.
type RecordResult struct {
	Status RecordStatus `json:"status"`
	Error  string       `json:"error,omitempty"`
}

// ChangeResult contains the redacted execution result for a single change.
type ChangeResult struct {
	Name              string       `json:"name"`
	AuthIndex         string       `json:"auth_index"`
	RetryAuthIndex    string       `json:"-"`
	Provider          string       `json:"provider"`
	Account           string       `json:"account,omitempty"`
	Email             string       `json:"email,omitempty"`
	Status            ChangeStatus `json:"status"`
	HostOutcome       Outcome      `json:"host_outcome"`
	Success           bool         `json:"success"`
	EvidenceFresh     bool         `json:"evidence_fresh"`
	Reason            string       `json:"reason"`
	PriorityAttempted bool         `json:"priority_attempted"`
	DisabledAttempted bool         `json:"disabled_attempted"`
	PriorityFrom      int          `json:"priority_from"`
	PriorityMissing   bool         `json:"priority_missing,omitempty"`
	PriorityTo        int          `json:"priority_to"`
	DisabledFrom      bool         `json:"disabled_from"`
	DisabledTo        bool         `json:"disabled_to"`
	Error             string       `json:"error,omitempty"`
}

// FailureResult constructs a ChangeResult for an unwritten credential failure suitable for UI/management reporting.
func FailureResult(credential priority.PlanItem, err error) ChangeResult {
	result := newChangeResult(priority.Change{
		Credential:    credential.Credential,
		Priority:      credential.Priority,
		Disabled:      credential.Disabled,
		EvidenceFresh: credential.EvidenceFresh || credential.ForceWrite,
		Reason:        credential.Reason,
	})
	result.Status = ChangeStatusFailed
	result.HostOutcome = OutcomeFailed
	result.Error = redactedError(err)
	return result
}

// Apply executes Planner changes through the single Host Transition seam.
func Apply(ctx context.Context, request Request) (Result, error) {
	return ExecutePlan(ctx, request.Transition, request.Plan, request.ReportSkippedPlan)
}

func skippedPlanItemResult(item priority.PlanItem) ChangeResult {
	result := newChangeResult(priority.Change{
		Credential:    item.Credential,
		Priority:      item.Priority,
		Disabled:      item.Disabled,
		EvidenceFresh: item.EvidenceFresh || item.ForceWrite,
		Reason:        item.Reason,
	})
	result.Status = ChangeStatusSkipped
	return result
}

func newChangeResult(change priority.Change) ChangeResult {
	return ChangeResult{
		Name:            redactIdentifier(resultName(change.Credential)),
		AuthIndex:       redactIdentifier(change.Credential.AuthIndex),
		RetryAuthIndex:  change.Credential.AuthIndex,
		Provider:        string(change.Credential.Provider),
		Account:         redactIdentifier(change.Credential.Account),
		Email:           redactIdentifier(change.Credential.Email),
		EvidenceFresh:   change.EvidenceFresh,
		Reason:          redactString(change.Reason),
		PriorityFrom:    change.Credential.Priority,
		PriorityMissing: change.Credential.PriorityMissing,
		PriorityTo:      change.Priority,
		DisabledFrom:    change.Credential.Disabled,
		DisabledTo:      change.Disabled,
	}
}

func summarizeChange(result *Result, change ChangeResult) {
	switch change.Status {
	case ChangeStatusCommitted:
		result.Attempted++
		result.Succeeded++
	case ChangeStatusNoChange:
		result.Attempted++
		result.Skipped++
		result.NoChange++
	case ChangeStatusFailed:
		result.Attempted++
		result.Failed++
	case ChangeStatusConflict:
		result.Attempted++
		result.Conflicts++
	case ChangeStatusUncertain:
		result.Attempted++
		result.Uncertain++
	case ChangeStatusSkipped:
		result.Skipped++
	}
}
