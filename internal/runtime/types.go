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

const (
	// Plugin JSON-RPC Methods
	MethodPluginRegister     = "plugin.register"
	MethodPluginReconfigure  = "plugin.reconfigure"
	MethodPluginShutdown     = "plugin.shutdown"
	MethodManagementRegister = "management.register"
	MethodManagementHandle   = "management.handle"
	MethodFilterResponse     = "filter.response"
	MethodFilterComplete     = "filter.complete"
	MethodFilterError        = "filter.error"
	MethodFilterOutbound     = "filter.outbound"
	MethodFilterInbound      = "filter.inbound"

	// Run History Kinds
	KindApply     = "apply"
	KindProbe     = "probe"
	KindReset     = "reset"
	KindAutoApply = "auto_apply"
	KindCooldown  = "cooldown"
)

// Trigger represents the initiator of a scheduling execution.
type Trigger string

const (
	// TriggerManualApply indicates a manual write-back execution.
	TriggerManualApply Trigger = "manual_apply"
	// TriggerAutoApply indicates a background scheduled auto-apply execution.
	TriggerAutoApply Trigger = "auto_apply"
	// TriggerProbe indicates a probe-only execution that fetches fresh quota without planning or applying.
	TriggerProbe Trigger = "probe"
)

// TaskRequest holds parameters for an internal scheduling run.
type TaskRequest struct {
	Config          config.Config
	Trigger         Trigger
	AuthIndexes     []string
	PreviewID       string
	PreviewRequired bool
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

// Ticker provides periodic time tick abstraction.
type Ticker interface {
	Chan() <-chan time.Time
	Stop()
}

// TickerFactory creates Ticker instances.
type TickerFactory interface {
	NewTicker(interval time.Duration) Ticker
}

// RunHistoryEntry records the execution outcome of a scheduling run.
type RunHistoryEntry struct {
	Kind      string              `json:"kind"`
	Trigger   string              `json:"trigger"`
	At        time.Time           `json:"at"`
	Attempted int                 `json:"attempted"`
	Succeeded int                 `json:"succeeded"`
	Failed    int                 `json:"failed"`
	Skipped   int                 `json:"skipped"`
	NoChange  int                 `json:"no_change"`
	Conflicts int                 `json:"conflicts"`
	Uncertain int                 `json:"uncertain"`
	Message   string              `json:"message"`
	Snapshot  *apply.PlanSnapshot `json:"snapshot,omitempty"`
	Record    apply.RecordResult  `json:"record"`
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
	Host          host.HostCallbacks
	Clock         Clock
	Sleeper       Sleeper
	TickerFactory TickerFactory
	Runner        TaskRunner
	// StateCachePath overrides the startup cache path. It is primarily used
	// to isolate runtime instances in tests; production callers may leave it
	// empty to use the configured default path.
	StateCachePath string
}
