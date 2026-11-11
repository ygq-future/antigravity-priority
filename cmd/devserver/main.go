package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"antigravity-priority/internal/runtime"
	"antigravity-priority/internal/state"
)

type devServerOptions struct {
	AuthDir        string
	QuotaStatePath string
	StateCachePath string
	AccountCount   int
	Seed           int64
	Clock          runtime.Clock
	AutoApply      bool
	Delay          time.Duration
}

type devServer struct {
	host    *devHost
	runtime *runtime.Runtime
	delay   time.Duration
}

func newDevServer(options devServerOptions) (*devServer, error) {
	if strings.TrimSpace(options.AuthDir) == "" {
		options.AuthDir = defaultDevAuthDir
	}
	if strings.TrimSpace(options.QuotaStatePath) == "" {
		options.QuotaStatePath = defaultDevQuotaState
	}
	if strings.TrimSpace(options.StateCachePath) == "" {
		options.StateCachePath = defaultDevCachePath
	}
	if options.AccountCount <= 0 {
		options.AccountCount = defaultDevAccountCount
	}

	var nowFn func() time.Time
	if options.Clock != nil {
		nowFn = options.Clock.Now
	}
	fakeHost, err := newDevHost(devHostOptions{
		AuthDir:        options.AuthDir,
		QuotaStatePath: options.QuotaStatePath,
		AccountCount:   options.AccountCount,
		Seed:           options.Seed,
		NowFn:          nowFn,
	})
	if err != nil {
		return nil, err
	}
	if store, err := state.Load(context.Background(), options.StateCachePath); err == nil {
		cooldowns := store.GetCooldowns()
		if len(cooldowns) == 0 {
			nowTime := time.Now().UTC()
			if options.Clock != nil {
				nowTime = options.Clock.Now().UTC()
			}
			store.SetCooldown(state.CooldownEntry{
				AuthIndex:     "dev-auth-005",
				ModelGroup:    "gemini",
				TriggeredAt:   nowTime,
				CooldownUntil: nowTime.Add(30 * time.Minute),
				Reason:        "429 rate limit cooldown",
			})
			_ = store.SaveStateAtomic(context.Background())
		}
		if len(store.GetRequestHistory()) == 0 {
			nowTime := time.Now().UTC()
			if options.Clock != nil {
				nowTime = options.Clock.Now().UTC()
			}
			store.SetRequestHistory([]state.RequestHistoryEntry{
				{
					ID:           "req-seed-001",
					At:           nowTime.Add(-12 * time.Second),
					Model:        "claude-opus-4-6-thinking",
					ModelPool:    "claude_gpt",
					AuthIndex:    "dev-auth-001",
					Email:        "mojs***",
					InputTokens:  1850,
					OutputTokens: 620,
					TotalTokens:  2470,
					Failed:       false,
					StatusCode:   200,
				},
				{
					ID:              "req-seed-002",
					At:              nowTime.Add(-48 * time.Second),
					Model:           "gemini-3.8-flash-high",
					ModelPool:       "gemini",
					AuthIndex:       "dev-auth-002",
					Email:           "arno***",
					InputTokens:     259,
					OutputTokens:    445,
					ReasoningTokens: 938,
					TotalTokens:     1642,
					Failed:          false,
					StatusCode:      200,
				},
				{
					ID:           "req-seed-003",
					At:           nowTime.Add(-2 * time.Minute),
					Model:        "gemini-4-argon-high",
					ModelPool:    "argon",
					AuthIndex:    "dev-auth-003",
					Email:        "mojs***",
					InputTokens:  2100,
					OutputTokens: 980,
					TotalTokens:  3080,
					Failed:       false,
					StatusCode:   200,
				},
				{
					ID:           "req-seed-004",
					At:           nowTime.Add(-5 * time.Minute),
					Model:        "gemini-3.8-flash-high",
					ModelPool:    "gemini",
					AuthIndex:    "dev-auth-005",
					Email:        "dev-***",
					InputTokens:  120,
					OutputTokens: 0,
					TotalTokens:  120,
					Failed:       true,
					StatusCode:   429,
					ErrorMessage: "RESOURCE_EXHAUSTED: rate limit exceeded",
				},
				{
					ID:           "req-seed-005",
					At:           nowTime.Add(-8 * time.Minute),
					Model:        "claude-sonnet-5-5-high",
					ModelPool:    "claude_gpt",
					AuthIndex:    "dev-auth-002",
					Email:        "arno***",
					InputTokens:  940,
					OutputTokens: 0,
					TotalTokens:  940,
					Failed:       true,
					StatusCode:   500,
					ErrorMessage: defaultSimulatedErrorMessage,
				},
				{
					ID:           "req-seed-006",
					At:           nowTime.Add(-11 * time.Minute),
					Model:        "gemini-4-argon-high",
					ModelPool:    "argon",
					AuthIndex:    "dev-auth-003",
					Email:        "mojs***",
					InputTokens:  320,
					OutputTokens: 0,
					TotalTokens:  320,
					Failed:       true,
					StatusCode:   403,
					ErrorMessage: "PERMISSION_DENIED: caller does not have permission to access the requested resource",
				},
			})
			_ = store.SaveRequestsAtomic(context.Background())
		}
	}
	rt := runtime.New(runtime.Options{
		Host:           fakeHost,
		Clock:          options.Clock,
		StateCachePath: options.StateCachePath,
	})
	configJSON := fmt.Sprintf(`{"enabled":true,"state_cache_path":%q}`, options.StateCachePath)
	if _, err := rt.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: configJSON}); err != nil {
		_ = rt.Shutdown(context.Background())
		return nil, fmt.Errorf("register dev runtime: %w", err)
	}
	if options.AutoApply {
		dynamic, err := rt.GetDynamicConfig(context.Background())
		if err != nil {
			_ = rt.Shutdown(context.Background())
			return nil, fmt.Errorf("read dev runtime config: %w", err)
		}
		dynamic.AutoApply = true
		if err := rt.SetDynamicConfig(context.Background(), dynamic); err != nil {
			_ = rt.Shutdown(context.Background())
			return nil, fmt.Errorf("enable dev runtime scheduler: %w", err)
		}
	}
	return &devServer{host: fakeHost, runtime: rt, delay: options.Delay}, nil
}

func (d *devServer) resolveAuthIndex(r *http.Request) (string, error) {
	queryIndex := strings.TrimSpace(r.URL.Query().Get("auth_index"))
	if queryIndex == "" {
		var body struct {
			AuthIndex string `json:"auth_index"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		queryIndex = strings.TrimSpace(body.AuthIndex)
	}
	files, err := d.host.ListAuthFiles(r.Context())
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("no accounts available")
	}
	if queryIndex == "" {
		return files[0].AuthIndex, nil
	}
	for _, f := range files {
		if strings.EqualFold(f.AuthIndex, queryIndex) || strings.EqualFold(f.Name, queryIndex) || strings.EqualFold(f.Email, queryIndex) {
			return f.AuthIndex, nil
		}
	}
	for _, f := range files {
		if strings.Contains(strings.ToLower(f.AuthIndex), strings.ToLower(queryIndex)) ||
			strings.Contains(strings.ToLower(f.Name), strings.ToLower(queryIndex)) ||
			strings.Contains(strings.ToLower(f.Email), strings.ToLower(queryIndex)) {
			return f.AuthIndex, nil
		}
	}
	return queryIndex, nil
}

func (d *devServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/dev/simulate-429", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authIndex, err := d.resolveAuthIndex(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := fmt.Sprintf(`{"Provider":"antigravity","Model":"gemini-3.7-flash","AuthIndex":%q,"Failed":true,"Failure":{"StatusCode":429,"Body":"RESOURCE_EXHAUSTED"}}`, authIndex)
		resp := d.runtime.Handle(r.Context(), runtime.MethodUsageHandle, []byte(payload))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})

	mux.HandleFunc("/dev/simulate-success", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authIndex, err := d.resolveAuthIndex(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload := fmt.Sprintf(`{"Provider":"antigravity","Model":"gemini-3.7-flash","AuthIndex":%q,"Failed":false}`, authIndex)
		resp := d.runtime.Handle(r.Context(), runtime.MethodUsageHandle, []byte(payload))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})
	mux.HandleFunc("/dev/simulate-error", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authIndex, err := d.resolveAuthIndex(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		statusCode := http.StatusInternalServerError
		if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
			parsed, parseErr := strconv.Atoi(rawStatus)
			if parseErr != nil || parsed < 100 || parsed > 599 {
				http.Error(w, "status must be an integer between 100 and 599", http.StatusBadRequest)
				return
			}
			statusCode = parsed
		}
		message := strings.TrimSpace(r.URL.Query().Get("message"))
		if message == "" {
			message = defaultSimulatedErrorMessage
		}
		model := strings.TrimSpace(r.URL.Query().Get("model"))
		if model == "" {
			model = "gemini-3.8-flash-high"
		}
		payload, err := json.Marshal(map[string]any{
			"Provider":  "antigravity",
			"Model":     model,
			"AuthIndex": authIndex,
			"Failed":    true,
			"Failure": map[string]any{
				"StatusCode": statusCode,
				"Body":       message,
			},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := d.runtime.Handle(r.Context(), runtime.MethodUsageHandle, payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})

	mux.HandleFunc("/dev/simulate-usage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := d.runtime.Handle(r.Context(), runtime.MethodUsageHandle, raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})

	managementHandler := d.runtime.ManagementHandler()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if d.delay > 0 && r.Method == http.MethodPost {
			path := r.URL.Path
			if strings.HasSuffix(path, "/sync") ||
				strings.HasSuffix(path, "/run") ||
				strings.HasSuffix(path, "/reset") ||
				strings.HasSuffix(path, "/custom-tier") {
				select {
				case <-time.After(d.delay):
				case <-r.Context().Done():
					return
				}
			}
		}
		managementHandler.ServeHTTP(w, r)
	})
	return mux
}

func main() {
	defaultAddress := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DEVSERVER_ADDR"))
	if defaultAddress == "" {
		defaultAddress = "127.0.0.1:8080"
	}
	defaultAccounts := envInt("ANTIGRAVITY_DEVSERVER_ACCOUNTS", defaultDevAccountCount)
	defaultSeed := envInt64("ANTIGRAVITY_DEVSERVER_SEED", 0)
	defaultAuthDir := envString("ANTIGRAVITY_DEVSERVER_AUTH_DIR", defaultDevAuthDir)
	defaultQuotaState := envString("ANTIGRAVITY_DEVSERVER_QUOTA_STATE", defaultDevQuotaState)
	defaultCachePath := envString("ANTIGRAVITY_DEVSERVER_CACHE", defaultDevCachePath)
	defaultDelay := envDuration("ANTIGRAVITY_DEVSERVER_DELAY", 800*time.Millisecond)
	address := flag.String("addr", defaultAddress, "HTTP listen address")
	authDir := flag.String("auth-dir", defaultAuthDir, "directory containing simulated CPA auth JSON files")
	quotaState := flag.String("quota-state", defaultQuotaState, "persistent state for simulated Antigravity quota")
	cachePath := flag.String("state-cache", defaultCachePath, "Runtime state cache path")
	accounts := flag.Int("accounts", defaultAccounts, "minimum number of simulated Antigravity auth files")
	seed := flag.Int64("seed", defaultSeed, "random seed for deterministic quota simulation; zero selects a time-based seed")
	autoApply := flag.Bool("auto-apply", false, "enable the Runtime scheduler at startup")
	delay := flag.Duration("delay", defaultDelay, "simulated network latency for sync/probe/reset operations (e.g. 800ms, 1s; set to 0 to disable)")
	flag.Parse()

	dev, err := newDevServer(devServerOptions{
		AuthDir:        *authDir,
		QuotaStatePath: *quotaState,
		StateCachePath: *cachePath,
		AccountCount:   *accounts,
		Seed:           *seed,
		AutoApply:      *autoApply,
		Delay:          *delay,
	})
	if err != nil {
		log.Fatalf("start devserver runtime: %v", err)
	}

	server := &http.Server{
		Addr:              *address,
		Handler:           dev.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("=========================================================")
	fmt.Println(" Antigravity Priority - CPA Runtime Dev Server")
	fmt.Println("=========================================================")
	displayAddress := *address
	if strings.HasPrefix(displayAddress, ":") {
		displayAddress = "localhost" + displayAddress
	}
	fmt.Printf(" Server listening on: http://%s/status\n", displayAddress)
	fmt.Printf(" Simulated CPA auth files: %s (%d minimum accounts)\n", filepath.Clean(*authDir), *accounts)
	fmt.Printf(" Simulated network delay: %v (configurable via -delay or ANTIGRAVITY_DEVSERVER_DELAY)\n", *delay)
	fmt.Println(" Simulated Antigravity quota requests stay inside the devserver.")
	fmt.Println(" Simulation Hooks for Manual 429 Testing:")
	fmt.Printf("   POST http://%s/dev/simulate-429?auth_index=<authIndex>\n", displayAddress)
	fmt.Printf("   POST http://%s/dev/simulate-success?auth_index=<authIndex>\n", displayAddress)
	fmt.Printf("   POST http://%s/dev/simulate-error?auth_index=<authIndex>&status=500&message=<detail>\n", displayAddress)
	fmt.Println(" Press Ctrl+C to stop the server.")
	fmt.Println("=========================================================")

	serverError := make(chan error, 1)
	go func() {
		serverError <- server.ListenAndServe()
	}()
	select {
	case err := <-serverError:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("devserver error: %v", err)
		}
	case <-stopContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("devserver HTTP shutdown: %v", err)
		}
		if err := dev.runtime.Shutdown(shutdownContext); err != nil {
			log.Printf("devserver runtime shutdown: %v", err)
		}
		<-serverError
	}
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
