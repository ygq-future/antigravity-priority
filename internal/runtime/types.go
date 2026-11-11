package runtime

import (
	"context"
	"errors"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
)

// ErrRunInProgress indicates that a priority scheduling run is already active.
var ErrRunInProgress = errors.New("runtime: run already in progress")

// ErrShutdown indicates that the runtime has been shut down.
var ErrShutdown = errors.New("runtime: shutdown")

// ErrInvalidRequest indicates that the CPA JSON envelope request is invalid.
var ErrInvalidRequest = errors.New("runtime: invalid request")

// ErrUnknownMethod indicates that CPA requested a plugin method this runtime does not implement.
var ErrUnknownMethod = errors.New("runtime: unknown method")

const (
	// Plugin JSON-RPC Methods
	MethodPluginRegister     = "plugin.register"
	MethodPluginQuiesce      = "plugin.quiesce"
	MethodPluginReconfigure  = "plugin.reconfigure"
	MethodPluginShutdown     = "plugin.shutdown"
	MethodManagementRegister = "management.register"
	MethodManagementHandle   = "management.handle"
	MethodUsageHandle        = "usage.handle"
	MethodSchedulerPick      = "scheduler.pick"
	MethodFilterResponse     = "filter.response"
	MethodFilterComplete     = "filter.complete"
	MethodFilterError        = "filter.error"
	MethodFilterOutbound     = "filter.outbound"
	MethodFilterInbound      = "filter.inbound"

	// Run History Kinds
	KindProbe            = "probe"
	KindReset            = "reset"
	KindAutoApply        = "auto_apply"
	KindCooldown         = "cooldown"
	KindCooldownRecovery = "cooldown_recovery"
)

// Trigger represents the initiator of a scheduling execution.
type Trigger string

const (
	// TriggerAutoApply indicates a background scheduled auto-apply execution.
	TriggerAutoApply Trigger = "auto_apply"
	// TriggerProbe indicates a probe-only execution that fetches fresh quota without planning or applying.
	TriggerProbe Trigger = "probe"
	// TriggerManualReset indicates a user-triggered static priority reset.
	TriggerManualReset Trigger = "manual_reset"
)

// TaskRequest holds parameters for an internal scheduling run.
type TaskRequest struct {
	Config      config.Config
	Trigger     Trigger
	AuthIndexes []string
}

// TaskRunner executes a scheduling task.
type TaskRunner func(ctx context.Context, request TaskRequest) error

// Clock provides time abstraction for runtime testing.
type Clock interface {
	Now() time.Time
}

// Sleeper provides interruptible sleep abstraction.
type Sleeper interface {
	Sleep(ctx context.Context, duration time.Duration) error
}

// RunHistoryEntry records the execution outcome of a scheduling run.
type RunHistoryEntry struct {
	Kind         string              `json:"kind"`
	Trigger      string              `json:"trigger"`
	At           time.Time           `json:"at"`
	Attempted    int                 `json:"attempted"`
	Succeeded    int                 `json:"succeeded"`
	Failed       int                 `json:"failed"`
	Skipped      int                 `json:"skipped"`
	NoChange     int                 `json:"no_change"`
	Conflicts    int                 `json:"conflicts"`
	Uncertain    int                 `json:"uncertain"`
	Message      string              `json:"message"`
	ProbeRoundID string              `json:"probe_round_id,omitempty"`
	Snapshot     *apply.PlanSnapshot `json:"snapshot,omitempty"`
	Record       apply.RecordResult  `json:"record"`
}

// RegisterRequest is the JSON request payload for plugin.register.
type RegisterRequest struct {
	ConfigYAML string `json:"config_yaml"`
}

// ReconfigureRequest is the JSON request payload for plugin.reconfigure.
type ReconfigureRequest struct {
	ConfigYAML string `json:"config_yaml"`
}

// RegisterResult is the metadata returned to CPA on registration.
type RegisterResult struct {
	SchemaVersion int             `json:"schema_version"`
	Metadata      Metadata        `json:"metadata"`
	Capabilities  map[string]bool `json:"capabilities"`
}

// ConfigField describes configuration fields for CPA UI rendering.
type ConfigField struct {
	Name         string   `json:"Name"`
	Type         string   `json:"Type"`
	Description  string   `json:"Description"`
	EnumValues   []string `json:"EnumValues,omitempty"`
	DefaultValue any      `json:"DefaultValue"`
}

// SchedulerPickRequest represents the JSON-RPC request for scheduler.pick.
type SchedulerPickRequest struct {
	Provider   string                   `json:"Provider"`
	Providers  []string                 `json:"Providers"`
	Model      string                   `json:"Model"`
	Stream     bool                     `json:"Stream"`
	Options    SchedulerOptions         `json:"Options"`
	Candidates []SchedulerAuthCandidate `json:"Candidates"`
}

// SchedulerOptions represents the options passed to scheduler.pick.
type SchedulerOptions struct {
	Headers  map[string][]string `json:"Headers"`
	Metadata map[string]any      `json:"Metadata"` // includes "canonical_session_id"
}

// SchedulerAuthCandidate represents a candidate credential evaluated by scheduler.pick.
type SchedulerAuthCandidate struct {
	ID         string            `json:"ID"`
	Provider   string            `json:"Provider"`
	Priority   int               `json:"Priority"`
	Status     string            `json:"Status"`
	Attributes map[string]string `json:"Attributes"`
	Metadata   map[string]any    `json:"Metadata,omitempty"`
}

// SchedulerPickResponse represents the response payload for scheduler.pick.
type SchedulerPickResponse struct {
	AuthID          string `json:"auth_id,omitempty"`
	DelegateBuiltin string `json:"delegate_builtin,omitempty"`
	Handled         bool   `json:"handled"`
	Reject          bool   `json:"reject,omitempty"`
	RejectReason    string `json:"reject_reason,omitempty"`
	RejectCode      string `json:"reject_code,omitempty"`
}

// Metadata describes non-sensitive plugin information displayed in CPA.
type Metadata struct {
	Name             string        `json:"Name"`
	Version          string        `json:"Version"`
	Author           string        `json:"Author"`
	GitHubRepository string        `json:"GitHubRepository"`
	Description      string        `json:"Description"`
	ConfigFields     []ConfigField `json:"ConfigFields,omitempty"`
}

// Options provides injectable dependencies for the runtime.
type Options struct {
	Host    host.HostCallbacks
	Clock   Clock
	Sleeper Sleeper
	Runner  TaskRunner
	// StateCachePath overrides the startup cache path. It is primarily used
	// to isolate runtime instances in tests; production callers may leave it
	// empty to use the configured default path.
	StateCachePath string
}
