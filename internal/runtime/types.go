package runtime

import (
	"errors"

	"antigravity-priority/internal/host"
)

// ErrRunInProgress indicates that a priority scheduling run is already active.
var ErrRunInProgress = errors.New("runtime: run already in progress")

// ErrShutdown indicates that the runtime has been shut down.
var ErrShutdown = errors.New("runtime: shutdown")

// ErrInvalidRequest indicates that the CPA JSON envelope request is invalid.
var ErrInvalidRequest = errors.New("runtime: invalid request")

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
	Host host.HostCallbacks
}
