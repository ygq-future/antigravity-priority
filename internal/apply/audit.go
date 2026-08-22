package apply

import (
	"encoding/json"
	"strconv"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
)

// PlanSnapshot is the safe redacted plan snapshot for audit and diagnostics.
type PlanSnapshot struct {
	DecidedAt    time.Time        `json:"decided_at"`
	TotalItems   int              `json:"total_items"`
	TotalChanges int              `json:"total_changes"`
	Items        []SnapshotItem   `json:"items"`
	Changes      []SnapshotChange `json:"changes"`
}

// Snapshot returns the redacted snapshot of a plan for snapshot and diagnostics reuse.
func Snapshot(plan priority.Plan) PlanSnapshot {
	return newPlanSnapshot(plan)
}

// SnapshotItem is the redacted audit view of a single planned item.
type SnapshotItem struct {
	Name                 string     `json:"name"`
	AuthIndex            string     `json:"auth_index"`
	Account              string     `json:"account,omitempty"`
	Email                string     `json:"email,omitempty"`
	Provider             string     `json:"provider"`
	Type                 string     `json:"type"`
	Status               string     `json:"status"`
	PlanType             string     `json:"plan_type,omitempty"`
	Current              Target     `json:"current"`
	Target               Target     `json:"target"`
	EvidenceFresh        bool       `json:"evidence_fresh"`
	Reason               string     `json:"reason"`
	IsBoosted            bool       `json:"is_boosted"`
	IsPredicted          bool       `json:"is_predicted"`
	Urgency              float64    `json:"urgency"`
	R7d                  float64    `json:"r7d"`
	T7d                  float64    `json:"t7d"`
	R5h                  float64    `json:"r5h"`
	T5h                  float64    `json:"t5h"`
	CycleBurnRate        float64    `json:"cycle_burn_rate"`
	TRequired            float64    `json:"t_required"`
	ResetAt              *time.Time `json:"reset_at,omitempty"`
	ShortWindowResetAt   *time.Time `json:"short_window_reset_at,omitempty"`
	ShortWindowRemaining *int64     `json:"short_window_remaining,omitempty"`
	LongWindowResetAt    *time.Time `json:"long_window_reset_at,omitempty"`
	LongWindowRemaining  *int64     `json:"long_window_remaining,omitempty"`
}

// SnapshotChange is the redacted audit view of a single candidate change.
type SnapshotChange struct {
	Name          string `json:"name"`
	AuthIndex     string `json:"auth_index"`
	Account       string `json:"account,omitempty"`
	Email         string `json:"email,omitempty"`
	Current       Target `json:"current"`
	Target        Target `json:"target"`
	EvidenceFresh bool   `json:"evidence_fresh"`
	Reason        string `json:"reason"`
	IsBoosted     bool   `json:"is_boosted"`
}

// Target represents the priority and disabled target state.
type Target struct {
	Priority        int  `json:"priority"`
	PriorityMissing bool `json:"priority_missing,omitempty"`
	Disabled        bool `json:"disabled"`
}

// AuditEvent is the redacted audit event for a Host transition round.
type AuditEvent struct {
	Action         string        `json:"action"`
	TotalChanges   int           `json:"total_changes"`
	FreshChanges   int           `json:"fresh_changes"`
	SkippedChanges int           `json:"skipped_changes"`
	Changes        []AuditChange `json:"changes"`
}

// AuditChange is a single change summary within an AuditEvent.
type AuditChange struct {
	Name          string  `json:"name"`
	AuthIndex     string  `json:"auth_index"`
	EvidenceFresh bool    `json:"evidence_fresh"`
	Reason        string  `json:"reason"`
	Outcome       Outcome `json:"outcome,omitempty"`
	Cause         string  `json:"cause,omitempty"`
}

func newPlanSnapshot(plan priority.Plan) PlanSnapshot {
	snapshot := PlanSnapshot{
		DecidedAt:    plan.DecidedAt,
		TotalItems:   len(plan.Items),
		TotalChanges: len(plan.Changes),
		Items:        make([]SnapshotItem, 0, len(plan.Items)),
		Changes:      make([]SnapshotChange, 0, len(plan.Changes)),
	}
	for _, item := range plan.Items {
		snapshot.Items = append(snapshot.Items, snapshotItem(item))
	}
	for _, change := range plan.Changes {
		snapshot.Changes = append(snapshot.Changes, snapshotChange(change))
	}
	return snapshot
}

func snapshotItem(item priority.PlanItem) SnapshotItem {
	credential := item.Credential
	return SnapshotItem{
		Name:                 redactIdentifier(credential.Name),
		AuthIndex:            redactIdentifier(credential.AuthIndex),
		Account:              redactIdentifier(credential.Account),
		Email:                redactIdentifier(credential.Email),
		Provider:             redactString(string(credential.Provider)),
		Type:                 redactString(string(credential.Type)),
		Status:               redactString(string(credential.Status)),
		PlanType:             redactString(string(item.PlanType)),
		Current:              currentTarget(credential.Priority, credential.PriorityMissing, credential.Disabled),
		Target:               target(item.Priority, item.Disabled),
		EvidenceFresh:        item.EvidenceFresh || item.ForceWrite,
		Reason:               redactString(item.Reason),
		IsBoosted:            item.IsBoosted,
		Urgency:              item.Urgency,
		R7d:                  item.R7d,
		T7d:                  item.T7d,
		R5h:                  item.R5h,
		T5h:                  item.T5h,
		CycleBurnRate:        item.CycleBurnRate,
		TRequired:            item.TRequired,
		ResetAt:              item.ResetAt,
		ShortWindowResetAt:   item.ShortWindowResetAt,
		ShortWindowRemaining: item.ShortWindowRemaining,
		LongWindowResetAt:    item.LongWindowResetAt,
		LongWindowRemaining:  item.LongWindowRemaining,
	}
}

func snapshotChange(change priority.Change) SnapshotChange {
	credential := change.Credential
	return SnapshotChange{
		Name:          redactIdentifier(credential.Name),
		AuthIndex:     redactIdentifier(credential.AuthIndex),
		Account:       redactIdentifier(credential.Account),
		Email:         redactIdentifier(credential.Email),
		Current:       currentTarget(credential.Priority, credential.PriorityMissing, credential.Disabled),
		Target:        target(change.Priority, change.Disabled),
		EvidenceFresh: change.EvidenceFresh,
		Reason:        redactString(change.Reason),
		IsBoosted:     change.IsBoosted,
	}
}

func resultName(credential core.Credential) string {
	for _, value := range []string{credential.Account, credential.Email, credential.Name, credential.AuthIndex} {
		if value != "" {
			return redactString(value)
		}
	}
	return ""
}

func currentTarget(priority int, priorityMissing bool, disabled bool) Target {
	return Target{Priority: priority, PriorityMissing: priorityMissing, Disabled: disabled}
}

func target(priority int, disabled bool) Target {
	return Target{Priority: priority, Disabled: disabled}
}

func redactString(value string) string {
	if value == "" {
		return ""
	}
	return host.RedactBytes([]byte(value))
}

func redactedError(err error) string {
	if err == nil {
		return ""
	}
	return redactString(err.Error())
}

func redactedErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	encoded, err := json.Marshal(errStrings(errs))
	if err != nil {
		return host.RedactedValue
	}
	return redactString(string(encoded))
}

func errStrings(errs []error) []string {
	values := make([]string, 0, len(errs))
	for index, err := range errs {
		values = append(values, strconv.Itoa(index+1)+": "+redactedError(err))
	}
	return values
}

// GroupSnapshot holds the snapshot for a single model group within a dual-group response.
type GroupSnapshot struct {
	Items   []SnapshotItem   `json:"items"`
	Changes []SnapshotChange `json:"changes"`
}

// DualGroupSnapshot wraps snapshots for both model groups, enabling instant client-side switching.
type DualGroupSnapshot struct {
	ActiveModelGroup string                   `json:"active_model_group"`
	ObservedAt       time.Time                `json:"observed_at,omitempty"`
	Groups           map[string]GroupSnapshot `json:"groups"`
}
