package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/promptguard"
)

type requestInterceptRequest struct {
	SourceFormat string `json:"SourceFormat"`
	ToFormat     string `json:"ToFormat"`
	Body         []byte `json:"Body"`
}

type requestInterceptResponse struct {
	Body []byte `json:"Body,omitempty"`
}

var emptyRequestInterceptEnvelope = mustMarshal(Envelope{
	OK:     true,
	Result: json.RawMessage(`{}`),
})

func envelopeRequestIntercept(resp requestInterceptResponse) []byte {
	if len(resp.Body) == 0 {
		return emptyRequestInterceptEnvelope
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		return emptyRequestInterceptEnvelope
	}
	return mustMarshal(Envelope{OK: true, Result: encoded})
}

func (r *Runtime) handleRequestInterceptBefore() []byte {
	return emptyRequestInterceptEnvelope
}

func (r *Runtime) handleRequestInterceptAfter(ctx context.Context, raw []byte) []byte {
	if ctx != nil && ctx.Err() != nil {
		return emptyRequestInterceptEnvelope
	}
	eng := r.promptGuardEngine.Load()
	if eng == nil || !eng.Enabled() || eng.ActiveRuleCount() == 0 {
		return emptyRequestInterceptEnvelope
	}
	if len(raw) == 0 {
		return emptyRequestInterceptEnvelope
	}
	var req requestInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return emptyRequestInterceptEnvelope
	}
	if !strings.EqualFold(strings.TrimSpace(req.ToFormat), "antigravity") || len(req.Body) == 0 {
		return emptyRequestInterceptEnvelope
	}
	rewritten, changed, hits := eng.RewriteRequest(promptguard.FormatAuto, req.Body)
	if !changed {
		return emptyRequestInterceptEnvelope
	}
	r.promptGuardRewrittenReqs.Add(1)
	r.promptGuardRewrittenHits.Add(int64(hits))
	return envelopeRequestIntercept(requestInterceptResponse{Body: rewritten})
}

// PreviewPromptGuard runs Prompt Guard dry-run rewriting against an input payload
// or prompt text using either caller-supplied draft rules or the active saved rules.
func (r *Runtime) PreviewPromptGuard(_ context.Context, req promptguard.PreviewRequest) (promptguard.PreviewResult, error) {
	cfg, err := r.Config()
	if err != nil {
		return promptguard.PreviewResult{}, err
	}
	pgCfg := cfg.PromptGuard.Clone()
	if req.Rules != nil {
		if len(req.Rules) > config.MaxPromptGuardRules {
			return promptguard.PreviewResult{}, config.ErrInvalidConfig
		}
		pgCfg.Rules = make([]config.PromptGuardRule, len(req.Rules))
		copy(pgCfg.Rules, req.Rules)
	}
	pgCfg.Enabled = true
	pgCfg.Normalize()
	return promptguard.Preview(pgCfg, req), nil
}

// SyncPromptGuard synchronizes Prompt Guard rules from the remote phrases.json repository.
// It downloads the remote phrases.json, applies DiffSyncRules (preserving custom user rules,
// deleting rules removed from remote, adding new remote rules, and updating modified rules),
// and hot-applies and persists the resulting configuration.
func (r *Runtime) SyncPromptGuard(ctx context.Context, syncURL string) ([]config.PromptGuardRule, promptguard.SyncSummary, error) {
	r.configMu.Lock()
	defer r.configMu.Unlock()

	baseCfg, err := r.Config()
	if err != nil {
		return nil, promptguard.SyncSummary{}, err
	}

	targetURL := strings.TrimSpace(syncURL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(baseCfg.PromptGuard.SyncURL)
	}
	if targetURL == "" {
		targetURL = promptguard.DefaultPromptGuardSyncURL
	}

	remoteFile, err := promptguard.FetchRemotePhrases(ctx, r.httpClient, targetURL)
	if err != nil {
		return nil, promptguard.SyncSummary{}, fmt.Errorf("fetch remote phrases: %w", err)
	}

	newRules, summary := promptguard.DiffSyncRules(baseCfg.PromptGuard.Rules, remoteFile.Phrases)

	dyn := baseCfg.Dynamic()
	dyn.PromptGuard.Rules = newRules
	if strings.TrimSpace(syncURL) != "" {
		dyn.PromptGuard.SyncURL = targetURL
	}

	newCfg, err := dyn.ApplyTo(baseCfg)
	if err != nil {
		return nil, summary, fmt.Errorf("apply synced prompt guard config: %w", err)
	}

	if err := r.state.SetDynamicConfig(ctx, dyn); err != nil {
		return nil, summary, fmt.Errorf("persist synced prompt guard config: %w", err)
	}

	if err := r.replaceConfig(ctx, newCfg); err != nil {
		return nil, summary, fmt.Errorf("replace config with synced prompt guard: %w", err)
	}

	return newRules, summary, nil
}

func (r *Runtime) startPromptGuardInitialSync() {
	if r.bgWorkersDisabled {
		return
	}
	r.promptGuardSyncOnce.Do(func() {
		go r.runPromptGuardInitialSync()
	})
}

func (r *Runtime) runPromptGuardInitialSync() {
	select {
	case <-time.After(3 * time.Second):
	case <-r.rootCtx.Done():
		return
	}

	cfg, err := r.Config()
	if err != nil {
		return
	}
	if cfg.PromptGuard.Enabled || len(cfg.PromptGuard.Rules) == 0 {
		ctx, cancel := context.WithTimeout(r.rootCtx, 20*time.Second)
		defer cancel()
		_, _, _ = r.SyncPromptGuard(ctx, "")
	}
}
