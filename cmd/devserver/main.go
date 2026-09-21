package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
)

type devServerOptions struct {
	AuthDir        string
	QuotaStatePath string
	StateCachePath string
	AccountCount   int
	Seed           int64
	Clock          runtime.Clock
	AutoApply      bool
}

type devServer struct {
	host    *devHost
	runtime *runtime.Runtime
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
	return &devServer{host: fakeHost, runtime: rt}, nil
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

	mux.Handle("/", d.runtime.ManagementHandler())
	return mux
}

func main() {
	defaultAddress := strings.TrimSpace(os.Getenv("ANTIGRAVITY_DEVSERVER_ADDR"))
	if defaultAddress == "" {
		defaultAddress = ":8080"
	}
	defaultAccounts := envInt("ANTIGRAVITY_DEVSERVER_ACCOUNTS", defaultDevAccountCount)
	defaultSeed := envInt64("ANTIGRAVITY_DEVSERVER_SEED", 0)
	defaultAuthDir := envString("ANTIGRAVITY_DEVSERVER_AUTH_DIR", defaultDevAuthDir)
	defaultQuotaState := envString("ANTIGRAVITY_DEVSERVER_QUOTA_STATE", defaultDevQuotaState)
	defaultCachePath := envString("ANTIGRAVITY_DEVSERVER_CACHE", defaultDevCachePath)

	address := flag.String("addr", defaultAddress, "HTTP listen address")
	authDir := flag.String("auth-dir", defaultAuthDir, "directory containing simulated CPA auth JSON files")
	quotaState := flag.String("quota-state", defaultQuotaState, "persistent state for simulated Antigravity quota")
	cachePath := flag.String("state-cache", defaultCachePath, "Runtime state cache path")
	accounts := flag.Int("accounts", defaultAccounts, "minimum number of simulated Antigravity auth files")
	seed := flag.Int64("seed", defaultSeed, "random seed for deterministic quota simulation; zero selects a time-based seed")
	autoApply := flag.Bool("auto-apply", false, "enable the Runtime scheduler at startup")
	flag.Parse()

	dev, err := newDevServer(devServerOptions{
		AuthDir:        *authDir,
		QuotaStatePath: *quotaState,
		StateCachePath: *cachePath,
		AccountCount:   *accounts,
		Seed:           *seed,
		AutoApply:      *autoApply,
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
	fmt.Println(" Simulated Antigravity quota requests stay inside the devserver.")
	fmt.Println(" Simulation Hooks for Manual 429 Testing:")
	fmt.Printf("   POST http://%s/dev/simulate-429?auth_index=<authIndex>\n", displayAddress)
	fmt.Printf("   POST http://%s/dev/simulate-success?auth_index=<authIndex>\n", displayAddress)
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
