package runtime

import (
	"context"
	"encoding/json"
	"strings"

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
	rewritten, changed, hits := eng.RewriteRequest(req.SourceFormat, req.Body)
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
