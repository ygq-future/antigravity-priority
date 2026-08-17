package runtime

import (
	"context"
	"fmt"
	"sync"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
)

// Runtime manages plugin lifecycle, configuration, and host interaction.
type Runtime struct {
	mu            sync.Mutex
	cfg           config.Config
	hostCallbacks host.HostCallbacks
	shutdown      bool
}

// New creates a new Runtime instance with the provided options.
func New(options Options) *Runtime {
	return &Runtime{
		cfg:           config.Default(),
		hostCallbacks: options.Host,
	}
}

// Handle routes CPA method calls to their corresponding handlers.
func (r *Runtime) Handle(ctx context.Context, method string, request []byte) []byte {
	switch method {
	case "plugin.register":
		parsed, err := decodeRegisterRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Register(ctx, parsed)
		return envelopeRegister(result, err)
	case "plugin.reconfigure":
		parsed, err := decodeReconfigureRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Reconfigure(ctx, parsed)
		return envelopeRegister(result, err)
	case "plugin.shutdown":
		return envelopeStatus(r.Shutdown(ctx))
	default:
		return failure(fmt.Errorf("%w: method %q", ErrInvalidRequest, method))
	}
}

// Register initializes the plugin with configuration received from CPA.
func (r *Runtime) Register(ctx context.Context, req RegisterRequest) (RegisterResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.applyConfigLocked(req.ConfigYAML)
}

// Reconfigure updates runtime configuration dynamically.
func (r *Runtime) Reconfigure(ctx context.Context, req ReconfigureRequest) (RegisterResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.applyConfigLocked(req.ConfigYAML)
}

func (r *Runtime) applyConfigLocked(configYAML string) (RegisterResult, error) {
	if r.shutdown {
		return RegisterResult{}, ErrShutdown
	}

	cfg, err := config.LoadBytes([]byte(configYAML))
	if err != nil {
		return RegisterResult{}, err
	}
	r.cfg = cfg

	return RegisterResult{
		SchemaVersion: 1,
		Metadata:      buildMetadata(),
		Capabilities: map[string]bool{
			"management": true,
		},
	}, nil
}

// Shutdown terminates the runtime.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.shutdown = true
	return nil
}

func buildMetadata() Metadata {
	return Metadata{
		Name:             "Antigravity Priority",
		Version:          "1.0.0",
		Author:           "sheepyu",
		GitHubRepository: "https://github.com/sheepyu/antigravity-priority",
		Description:      "Intelligent quota pacing and adaptive burn-rate priority scheduler exclusively for Google Antigravity in CLIProxyAPI.",
		ConfigFields: []ConfigField{
			{
				Name:         "enabled",
				Type:         "bool",
				Description:  "Enable or disable the Antigravity Priority plugin.",
				DefaultValue: true,
			},
			{
				Name:         "auto_apply",
				Type:         "bool",
				Description:  "Automatically write calculated priorities back to host auth files.",
				DefaultValue: false,
			},
			{
				Name:         "interval",
				Type:         "string",
				Description:  "Probe and scheduling calculation interval.",
				DefaultValue: "15m",
			},
			{
				Name:         "antigravity_model_group",
				Type:         "select",
				Description:  "Antigravity quota model group for priority scheduling.",
				EnumValues:   []string{"gemini", "claude_gpt"},
				DefaultValue: "gemini",
			},
			{
				Name:         "max_concurrency",
				Type:         "int",
				Description:  "Maximum concurrent quota probe requests.",
				DefaultValue: 6,
			},
			{
				Name:         "min_change",
				Type:         "int",
				Description:  "Minimum priority change required to trigger a write-back.",
				DefaultValue: 1,
			},
		},
	}
}
