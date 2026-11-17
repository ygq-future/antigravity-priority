package promptguard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"antigravity-priority/internal/config"
)

// DefaultPromptGuardSyncURL is the default remote phrases.json endpoint.
const DefaultPromptGuardSyncURL = "https://raw.githubusercontent.com/ygq-future/magpie-prompt-guard/main/phrases.json"

// RemotePhraseFile models the phrases.json structure from magpie-prompt-guard.
type RemotePhraseFile struct {
	Version int            `json:"version"`
	Updated string         `json:"updated,omitempty"`
	About   string         `json:"about,omitempty"`
	Phrases []RemotePhrase `json:"phrases"`
}

// RemotePhrase represents a single phrase entry in phrases.json.
type RemotePhrase struct {
	Find    string `json:"find"`
	Replace string `json:"replace"`
	Mode    string `json:"mode,omitempty"`
	Note    string `json:"note,omitempty"`
}

// SyncSummary records statistics about a DiffSyncRules operation.
type SyncSummary struct {
	TotalBefore     int `json:"total_before"`
	TotalAfter      int `json:"total_after"`
	Added           int `json:"added"`
	Updated         int `json:"updated"`
	Deleted         int `json:"deleted"`
	PreservedCustom int `json:"preserved_custom"`
}

// FetchRemotePhrases downloads and parses phrases.json from the given URL.
func FetchRemotePhrases(ctx context.Context, client *http.Client, url string) (*RemotePhraseFile, error) {
	targetURL := strings.TrimSpace(url)
	if targetURL == "" {
		targetURL = DefaultPromptGuardSyncURL
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create sync request: %w", err)
	}
	req.Header.Set("User-Agent", "antigravity-priority/prompt-guard")
	req.Header.Set("Accept", "application/json, text/plain")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch remote phrases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch remote phrases returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read remote phrases response: %w", err)
	}

	var file RemotePhraseFile
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("decode remote phrases JSON: %w", err)
	}
	return &file, nil
}

// DiffSyncRules performs differential synchronization between existing rules and remote phrases:
// 1. User custom rules (Custom == true) are completely preserved and unaffected.
// 2. Remote rules missing from remotePhrases are removed ("少了的就删").
// 3. New phrases from remotePhrases are added ("新增的就加").
// 4. Existing remote rules matching remotePhrases have their Replace and Mode updated, preserving their On state.
// 5. Preserves custom rules first, followed by remote rules ordered according to remotePhrases.
func DiffSyncRules(currentRules []config.PromptGuardRule, remotePhrases []RemotePhrase) ([]config.PromptGuardRule, SyncSummary) {
	summary := SyncSummary{
		TotalBefore: len(currentRules),
	}

	var customRules []config.PromptGuardRule
	existingRemoteByFind := make(map[string]config.PromptGuardRule)
	customFinds := make(map[string]struct{})

	for _, rule := range currentRules {
		cleanFind := strings.TrimSpace(rule.Find)
		if rule.Custom {
			customRules = append(customRules, rule)
			if cleanFind != "" {
				customFinds[cleanFind] = struct{}{}
			}
			summary.PreservedCustom++
		} else {
			if cleanFind != "" {
				existingRemoteByFind[cleanFind] = rule
			}
		}
	}

	seenRemoteFinds := make(map[string]struct{})
	var syncedRemoteRules []config.PromptGuardRule

	for _, p := range remotePhrases {
		cleanFind := strings.TrimSpace(p.Find)
		if cleanFind == "" {
			continue
		}
		if _, seen := seenRemoteFinds[cleanFind]; seen {
			continue
		}
		seenRemoteFinds[cleanFind] = struct{}{}

		// If user already has a custom rule with this find, the custom rule takes precedence.
		if _, isCustom := customFinds[cleanFind]; isCustom {
			continue
		}

		mode := strings.ToLower(strings.TrimSpace(p.Mode))
		if mode == "" {
			mode = config.PromptGuardModeText
		}

		if existing, exists := existingRemoteByFind[cleanFind]; exists {
			if existing.Replace != p.Replace || existing.Mode != mode {
				summary.Updated++
			}
			syncedRemoteRules = append(syncedRemoteRules, config.PromptGuardRule{
				On:      existing.On,
				Find:    p.Find,
				Replace: p.Replace,
				Scope:   config.PromptGuardScopeSystem,
				Mode:    mode,
				Custom:  false,
			})
		} else {
			summary.Added++
			syncedRemoteRules = append(syncedRemoteRules, config.PromptGuardRule{
				On:      true,
				Find:    p.Find,
				Replace: p.Replace,
				Scope:   config.PromptGuardScopeSystem,
				Mode:    mode,
				Custom:  false,
			})
		}
	}

	for find := range existingRemoteByFind {
		if _, inRemote := seenRemoteFinds[find]; !inRemote {
			summary.Deleted++
		}
	}

	out := make([]config.PromptGuardRule, 0, len(customRules)+len(syncedRemoteRules))
	out = append(out, customRules...)
	out = append(out, syncedRemoteRules...)

	if len(out) > config.MaxPromptGuardRules {
		out = out[:config.MaxPromptGuardRules]
	}

	summary.TotalAfter = len(out)
	return out, summary
}
