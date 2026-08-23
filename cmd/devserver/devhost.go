package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/host"
	"antigravity-priority/internal/provider/antigravity"
)

const (
	defaultDevAccountCount = 10
	defaultDevAuthDir      = "data/devserver/auth-files"
	defaultDevQuotaState   = "data/devserver/quota-state.json"
	defaultDevCachePath    = "data/devserver/refresh-cache.json"
)

var simulatedModelGroups = []string{
	string(antigravity.ModelGroupGemini),
	string(antigravity.ModelGroupClaudeGPT),
}

type devHostOptions struct {
	AuthDir        string
	QuotaStatePath string
	AccountCount   int
	Seed           int64
	NowFn          func() time.Time
}

// devHost is a file-backed implementation of CPA's HostCallbacks contract.
// Auth documents are deliberately kept separate from quota state: CPA owns
// credential metadata while Antigravity owns the quota response.
type devHost struct {
	mu             sync.Mutex
	authDir        string
	quotaStatePath string
	rng            *rand.Rand
	nowFn          func() time.Time
	quota          map[string]devQuotaState
}

type devQuotaState struct {
	Probed bool                          `json:"probed"`
	Groups map[string]devQuotaGroupState `json:"groups"`
}

type devQuotaGroupState struct {
	Short        int64     `json:"short_window_remaining"`
	Long         int64     `json:"long_window_remaining"`
	ShortResetAt time.Time `json:"short_window_reset_at"`
	LongResetAt  time.Time `json:"long_window_reset_at"`
}

type devAuthDocument struct {
	AccessToken  string `json:"access_token"`
	Disabled     bool   `json:"disabled"`
	Email        string `json:"email"`
	Expired      string `json:"expired"`
	ExpiresIn    int    `json:"expires_in"`
	Priority     int    `json:"priority"`
	ProjectID    string `json:"project_id"`
	RefreshToken string `json:"refresh_token"`
	Timestamp    int64  `json:"timestamp"`
	Type         string `json:"type"`
}

func newDevHost(options devHostOptions) (*devHost, error) {
	if strings.TrimSpace(options.AuthDir) == "" {
		return nil, errors.New("dev host auth directory is required")
	}
	if options.AccountCount <= 0 {
		options.AccountCount = defaultDevAccountCount
	}
	if options.NowFn == nil {
		options.NowFn = time.Now
	}
	if options.Seed == 0 {
		options.Seed = time.Now().UnixNano()
	}
	if err := os.MkdirAll(options.AuthDir, 0o700); err != nil {
		return nil, fmt.Errorf("create dev host auth directory: %w", err)
	}

	dev := &devHost{
		authDir:        options.AuthDir,
		quotaStatePath: strings.TrimSpace(options.QuotaStatePath),
		rng:            rand.New(rand.NewSource(options.Seed)),
		nowFn:          options.NowFn,
		quota:          make(map[string]devQuotaState),
	}
	if err := seedDevAuthFiles(dev.authDir, options.AccountCount, dev.now()); err != nil {
		return nil, err
	}
	if err := dev.loadQuotaState(); err != nil {
		return nil, err
	}
	return dev, nil
}

func (d *devHost) now() time.Time {
	if d.nowFn == nil {
		return time.Now()
	}
	return d.nowFn()
}

func seedDevAuthFiles(dir string, count int, now time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("list dev host auth directory: %w", err)
	}
	existing := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			existing++
		}
	}
	for index := 1; existing < count; index++ {
		name := fmt.Sprintf("dev-auth-%03d.json", index)
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect dev auth file %s: %w", name, err)
		}
		document := devAuthDocument{
			Email:     fmt.Sprintf("dev-account-%03d@gmail.com", index),
			Expired:   now.Add(time.Hour).Format(time.RFC3339),
			ExpiresIn: 3599,
			Priority:  98,
			ProjectID: fmt.Sprintf("dev-project-%03d", index),
			Timestamp: now.UnixMilli(),
			Type:      "antigravity",
		}
		data, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return fmt.Errorf("encode dev auth file %s: %w", name, err)
		}
		if err := writeDevFileAtomic(path, data); err != nil {
			return fmt.Errorf("seed dev auth file %s: %w", name, err)
		}
		existing++
	}
	return nil
}

func (d *devHost) loadQuotaState() error {
	if d.quotaStatePath == "" {
		return nil
	}
	data, err := os.ReadFile(d.quotaStatePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read dev quota state: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &d.quota); err != nil {
		return fmt.Errorf("decode dev quota state: %w", err)
	}
	if d.quota == nil {
		d.quota = make(map[string]devQuotaState)
	}
	return nil
}

func (d *devHost) saveQuotaStateLocked() error {
	if d.quotaStatePath == "" {
		return nil
	}
	data, err := json.MarshalIndent(d.quota, "", "  ")
	if err != nil {
		return fmt.Errorf("encode dev quota state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(d.quotaStatePath), 0o700); err != nil {
		return fmt.Errorf("create dev quota state directory: %w", err)
	}
	return writeDevFileAtomic(d.quotaStatePath, data)
}

func (d *devHost) ListAuthFiles(ctx context.Context) ([]host.AuthFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	entries, err := os.ReadDir(d.authDir)
	if err != nil {
		return nil, fmt.Errorf("list dev auth files: %w", err)
	}
	files := make([]host.AuthFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(d.authDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read dev auth file %s: %w", entry.Name(), err)
		}
		var file host.AuthFile
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("decode dev auth file %s: %w", entry.Name(), err)
		}
		file.AuthIndex = authIndexForPath(path)
		if strings.TrimSpace(file.Name) == "" {
			file.Name = firstDevAuthName(file.Email, entry.Name())
		}
		if strings.TrimSpace(file.Status) == "" {
			file.Status = "active"
			if file.Disabled {
				file.Status = "disabled"
			}
		}
		file.RawJSON = append(json.RawMessage(nil), data...)
		files = append(files, file)
	}
	return files, nil
}

func (d *devHost) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	if err := ctx.Err(); err != nil {
		return host.AuthDocument{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	path, name, err := d.findAuthPathLocked(authIndex)
	if err != nil {
		return host.AuthDocument{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return host.AuthDocument{}, fmt.Errorf("read dev auth document: %w", err)
	}
	return host.AuthDocument{
		AuthIndex: authIndexForPath(path),
		Name:      name,
		Path:      path,
		JSON:      append(json.RawMessage(nil), data...),
	}, nil
}

func (d *devHost) GetRuntime(ctx context.Context, authIndex string) (host.RuntimeAuth, error) {
	files, err := d.ListAuthFiles(ctx)
	if err != nil {
		return host.RuntimeAuth{}, err
	}
	for _, file := range files {
		if file.AuthIndex == authIndex {
			return host.RuntimeAuth{
				AuthIndex: authIndex,
				Name:      file.Name,
				Provider:  file.Type,
				Disabled:  file.Disabled,
				Metadata:  append(json.RawMessage(nil), file.RawJSON...),
			}, nil
		}
	}
	return host.RuntimeAuth{}, fmt.Errorf("dev auth runtime %q not found", authIndex)
}

func (d *devHost) SaveAuth(ctx context.Context, name string, document json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	path, _, err := d.findAuthPathLocked(name)
	if err != nil {
		return err
	}
	if !json.Valid(document) {
		return errors.New("dev auth document is invalid JSON")
	}
	return writeDevFileAtomic(path, document)
}

func (d *devHost) HTTPDo(ctx context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
	if err := ctx.Err(); err != nil {
		return host.HTTPResponse{}, err
	}
	if request.Method != http.MethodPost || !strings.Contains(request.URL, "retrieveUserQuotaSummary") {
		return host.HTTPResponse{StatusCode: http.StatusNotFound}, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, _, err := d.findAuthPathLocked(request.AuthIndex); err != nil {
		return host.HTTPResponse{StatusCode: http.StatusUnauthorized}, nil
	}
	now := d.now().UTC()
	state := d.quota[request.AuthIndex]
	if !state.Probed || !state.hasAllGroups() {
		state = fullDevQuotaState(now)
		state.Probed = true
	} else if state.exhausted() {
		state = fullDevQuotaState(now)
		state.Probed = true
	} else {
		state.advance(now, d.rng)
	}
	d.quota[request.AuthIndex] = state
	if err := d.saveQuotaStateLocked(); err != nil {
		return host.HTTPResponse{}, err
	}
	body, err := marshalDevQuotaResponse(state)
	if err != nil {
		return host.HTTPResponse{}, err
	}
	return host.HTTPResponse{
		StatusCode: http.StatusOK,
		Headers:    host.Header{"Content-Type": []string{"application/json"}},
		Body:       body,
	}, nil
}

func (s devQuotaState) hasAllGroups() bool {
	if s.Groups == nil {
		return false
	}
	for _, group := range simulatedModelGroups {
		if _, ok := s.Groups[group]; !ok {
			return false
		}
	}
	return true
}

func (s devQuotaState) exhausted() bool {
	for _, group := range simulatedModelGroups {
		quota := s.Groups[group]
		if quota.Short <= 0 || quota.Long <= 0 {
			return true
		}
	}
	return false
}

func fullDevQuotaState(now time.Time) devQuotaState {
	groups := make(map[string]devQuotaGroupState, len(simulatedModelGroups))
	for _, group := range simulatedModelGroups {
		groups[group] = devQuotaGroupState{
			Short:        100,
			Long:         100,
			ShortResetAt: now.Add(5 * time.Hour),
			LongResetAt:  now.Add(7 * 24 * time.Hour),
		}
	}
	return devQuotaState{Groups: groups}
}

func (s *devQuotaState) advance(now time.Time, rng *rand.Rand) {
	for _, group := range simulatedModelGroups {
		quota := s.Groups[group]
		if !now.Before(quota.ShortResetAt) {
			quota.Short = 100
			quota.ShortResetAt = now.Add(5 * time.Hour)
		}
		if !now.Before(quota.LongResetAt) {
			quota.Long = 100
			quota.LongResetAt = now.Add(7 * 24 * time.Hour)
		}
		quota.Short = consumeDevQuota(quota.Short, rng)
		quota.Long = consumeDevQuota(quota.Long, rng)
		s.Groups[group] = quota
	}
}

func consumeDevQuota(value int64, rng *rand.Rand) int64 {
	if value <= 1 {
		return 0
	}
	if rng.Intn(8) == 0 {
		return 0
	}
	maxDelta := value / 3
	if maxDelta < 1 {
		maxDelta = 1
	}
	delta := int64(1 + rng.Intn(int(maxDelta)))
	if delta >= value {
		return 0
	}
	return value - delta
}

func marshalDevQuotaResponse(state devQuotaState) ([]byte, error) {
	type window struct {
		Name              string  `json:"name"`
		RemainingFraction float64 `json:"remainingFraction"`
		ResetTime         string  `json:"resetTime"`
	}
	type model struct {
		ModelProvider string `json:"modelProvider"`
		QuotaInfo     struct {
			Windows []window `json:"windows"`
		} `json:"quotaInfo"`
	}
	makeModel := func(provider string, quota devQuotaGroupState) model {
		result := model{ModelProvider: provider}
		result.QuotaInfo.Windows = []window{
			{Name: "5h", RemainingFraction: float64(quota.Short) / 100, ResetTime: quota.ShortResetAt.UTC().Format(time.RFC3339)},
			{Name: "7d", RemainingFraction: float64(quota.Long) / 100, ResetTime: quota.LongResetAt.UTC().Format(time.RFC3339)},
		}
		return result
	}
	response := struct {
		Models map[string]model `json:"models"`
	}{Models: map[string]model{
		"gemini-2.5-flash":  makeModel("google", state.Groups[string(antigravity.ModelGroupGemini)]),
		"claude-3-5-sonnet": makeModel("anthropic", state.Groups[string(antigravity.ModelGroupClaudeGPT)]),
		"gpt-4o":            makeModel("openai", state.Groups[string(antigravity.ModelGroupClaudeGPT)]),
	}}
	return json.Marshal(response)
}

func (d *devHost) findAuthPathLocked(identifier string) (string, string, error) {
	needle := strings.TrimSpace(identifier)
	if needle == "" {
		return "", "", errors.New("dev auth identifier is required")
	}
	entries, err := os.ReadDir(d.authDir)
	if err != nil {
		return "", "", fmt.Errorf("list dev auth files: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(d.authDir, entry.Name())
		if authIndexForPath(path) == needle || entry.Name() == needle {
			return path, entry.Name(), nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", "", readErr
		}
		var file struct {
			Email string `json:"email"`
		}
		if json.Unmarshal(data, &file) == nil && file.Email == needle {
			return path, entry.Name(), nil
		}
	}
	return "", "", fmt.Errorf("dev auth %q not found", needle)
}

func authIndexForPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func firstDevAuthName(email, fallback string) string {
	if strings.TrimSpace(email) != "" {
		return strings.TrimSpace(email)
	}
	return strings.TrimSuffix(fallback, filepath.Ext(fallback))
}

func writeDevFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".devserver-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
