package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

// ModelCatalogItem represents a discovered or built-in model registered for routing.
type ModelCatalogItem struct {
	ID          string                       `json:"id"`
	DisplayName string                       `json:"display_name"`
	QuotaPool   config.AntigravityModelGroup `json:"quota_pool"`
}

// Built-in standard Antigravity models matching CPA internal models.json catalog.
var builtInAntigravityModels = []ModelCatalogItem{
	// Gemini Pool
	{ID: "gemini-3-flash", DisplayName: "Gemini 3 Flash", QuotaPool: config.AntigravityModelGroupGemini},
	{ID: "gemini-3.8-flash-high", DisplayName: "Gemini 3.8 Flash High", QuotaPool: config.AntigravityModelGroupGemini},
	{ID: "gemini-3.5-flash-lite", DisplayName: "Gemini 3.5 Flash Lite", QuotaPool: config.AntigravityModelGroupGemini},
	{ID: "gemini-3.1-flash-lite", DisplayName: "Gemini 3.1 Flash Lite", QuotaPool: config.AntigravityModelGroupGemini},
	{ID: "gemini-3.1-flash-image", DisplayName: "Gemini 3.1 Flash Image", QuotaPool: config.AntigravityModelGroupGemini},
	// Argon Pool
	{ID: "gemini-4-argon-high", DisplayName: "Gemini 4 Argon High", QuotaPool: config.AntigravityModelGroupArgon},

	// Claude & GPT Pool
	{ID: "claude-sonnet-5-5-high", DisplayName: "Claude Sonnet 5.5 High", QuotaPool: config.AntigravityModelGroupClaudeGPT},
	{ID: "claude-opus-5-5-high", DisplayName: "Claude Opus 5.5 High", QuotaPool: config.AntigravityModelGroupClaudeGPT},
	{ID: "gpt-oss-120b-medium", DisplayName: "GPT OSS 120B Medium", QuotaPool: config.AntigravityModelGroupClaudeGPT},
}

// PrecomputedCandidate stores the fast O(1) scheduling attributes for a credential in a specific quota pool.
type PrecomputedCandidate struct {
	AuthIndex          string
	Priority           int
	Urgency            float64
	ShortWindowResetAt time.Time
	LongWindowResetAt  time.Time
	CooldownUntil      time.Time
	Reason             string
}

// PoolRoutingTable holds the precomputed routing state for a single quota pool.
type PoolRoutingTable struct {
	DecidedAt        time.Time
	NextTransitionAt time.Time
	ByAuthIndex      map[string]PrecomputedCandidate
	BlockedModels    map[string]map[string]time.Time
}

type sessionBinding struct {
	CandidateID string
	ExpiresAt   time.Time
}

const sessionAffinityTTL = 30 * time.Minute

type historicalEvidenceSource struct {
	historical []priority.QuotaEvidence
}

func (h historicalEvidenceSource) FreshQuotaEvidence() []priority.QuotaEvidence {
	return nil
}

func (h historicalEvidenceSource) HistoricalQuotaEvidence() []priority.QuotaEvidence {
	return h.historical
}

// resolveAuthIndex resolves candidate ID or attributes to the internal auth index.
func (r *Runtime) resolveAuthIndex(candidateID string, attributes map[string]string) string {
	r.routingMu.RLock()
	defer r.routingMu.RUnlock()

	cleanID := strings.TrimSpace(candidateID)
	if idx, ok := r.authIDToIndex[cleanID]; ok && idx != "" {
		return idx
	}
	if len(attributes) > 0 {
		for _, key := range []string{"auth_index", "path", "name", "email", "account"} {
			if val, ok := attributes[key]; ok && strings.TrimSpace(val) != "" {
				trimmed := strings.TrimSpace(val)
				if idx, found := r.authIDToIndex[trimmed]; found && idx != "" {
					return idx
				}
				if key == "path" {
					base := filepath.Base(trimmed)
					if idx, found := r.authIDToIndex[base]; found && idx != "" {
						return idx
					}
				}
				if key == "auth_index" {
					return trimmed
				}
			}
		}
	}
	return cleanID
}

// updateModelInventoryLocked updates the candidate ID lookup table and catalog from CPA host auth inventory.
func isAntigravityProviderName(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return p == "antigravity" || p == "google" || p == "gemini" || p == "google-antigravity"
}

func (r *Runtime) updateModelInventoryLocked(files []host.AuthFile, models []host.AuthModelsEntry) {
	r.authIDToIndex = make(map[string]string)
	r.supportedModelsByAuth = make(map[string]map[string]struct{})

	antigravityAuthIDs := make(map[string]struct{})
	for _, file := range files {
		if !isAntigravityAuthFile(file) {
			continue
		}
		idx := file.AuthIndex
		if idx == "" {
			continue
		}
		antigravityAuthIDs[idx] = struct{}{}
		r.authIDToIndex[idx] = idx
		if file.ID != "" {
			antigravityAuthIDs[file.ID] = struct{}{}
			r.authIDToIndex[file.ID] = idx
		}
		if file.Name != "" {
			antigravityAuthIDs[file.Name] = struct{}{}
			r.authIDToIndex[file.Name] = idx
		}
		if file.Email != "" {
			antigravityAuthIDs[file.Email] = struct{}{}
			r.authIDToIndex[file.Email] = idx
		}
		if file.Account != "" {
			antigravityAuthIDs[file.Account] = struct{}{}
			r.authIDToIndex[file.Account] = idx
		}
		if file.Path != "" {
			antigravityAuthIDs[file.Path] = struct{}{}
			antigravityAuthIDs[filepath.Base(file.Path)] = struct{}{}
			antigravityAuthIDs[filepath.ToSlash(file.Path)] = struct{}{}
			r.authIDToIndex[file.Path] = idx
			r.authIDToIndex[filepath.Base(file.Path)] = idx
			r.authIDToIndex[filepath.ToSlash(file.Path)] = idx
		}
	}

	catalogMap := make(map[string]ModelCatalogItem)
	var discoveredAntigravityModels int

	for _, entry := range models {
		// Ignore models from explicit non-antigravity providers (e.g. vertex, codex, openai).
		if p := strings.TrimSpace(entry.Provider); p != "" && !isAntigravityProviderName(p) {
			continue
		}

		authIdx := entry.AuthIndex
		if authIdx == "" {
			authIdx = entry.ID
		}

		// When Antigravity credentials exist, ensure the model entry belongs to one of them.
		if len(antigravityAuthIDs) > 0 {
			_, byAuthIdx := antigravityAuthIDs[authIdx]
			_, byID := antigravityAuthIDs[entry.ID]
			if !byAuthIdx && !byID {
				continue
			}
		}

		if authIdx != "" {
			if entry.ID != "" {
				r.authIDToIndex[entry.ID] = authIdx
			}
			r.authIDToIndex[authIdx] = authIdx
		}

		if len(entry.Models) > 0 && authIdx != "" {
			set := r.supportedModelsByAuth[authIdx]
			if set == nil {
				set = make(map[string]struct{})
				r.supportedModelsByAuth[authIdx] = set
			}
			for _, m := range entry.Models {
				trimmed := strings.TrimSpace(m)
				if trimmed == "" {
					continue
				}
				set[trimmed] = struct{}{}
				discoveredAntigravityModels++
				if _, exists := catalogMap[trimmed]; !exists {
					catalogMap[trimmed] = ModelCatalogItem{
						ID:          trimmed,
						DisplayName: trimmed,
						QuotaPool:   config.ClassifyModelQuotaPool(trimmed),
					}
				}
			}
		}
	}

	// Fall back to built-in models only when host reports no callable Antigravity models.
	if discoveredAntigravityModels == 0 {
		for _, item := range builtInAntigravityModels {
			catalogMap[item.ID] = item
		}
	}

	mergedCatalog := make([]ModelCatalogItem, 0, len(catalogMap))
	for _, item := range catalogMap {
		mergedCatalog = append(mergedCatalog, item)
	}
	sort.Slice(mergedCatalog, func(i, j int) bool {
		return mergedCatalog[i].ID < mergedCatalog[j].ID
	})
	r.modelCatalog = mergedCatalog
}

// recomputePoolRoutingTableLocked computes a PoolRoutingTable for a single pool in pure memory.
func (r *Runtime) recomputePoolRoutingTableLocked(pool config.AntigravityModelGroup, now time.Time, store *state.Store, credentials []core.Credential) PoolRoutingTable {
	if store == nil {
		return PoolRoutingTable{
			DecidedAt:     now,
			ByAuthIndex:   make(map[string]PrecomputedCandidate),
			BlockedModels: make(map[string]map[string]time.Time),
		}
	}

	historical := store.BuildHistoricalEvidence(credentials, string(pool))
	cooldownEntries := store.GetActiveCooldownEntriesForGroup(now, string(pool))
	cooldowns := make(map[string]time.Time, len(cooldownEntries))
	cooldownReasons := make(map[string]string, len(cooldownEntries))
	for authIndex, entry := range cooldownEntries {
		cooldowns[authIndex] = entry.CooldownUntil
		if entry.Reason != "" {
			cooldownReasons[authIndex] = entry.Reason
		}
	}
	blockedModels := store.GetActiveModelCooldownsForGroup(now, string(pool))

	customTiers := store.GetCustomTiers()
	if customTiers == nil && r.cfg.CustomTiers != nil {
		customTiers = r.cfg.CustomTiers
	}

	options := priority.Options{
		Now:                 now,
		BoostStartPriority:  r.cfg.PriorityRules.BoostStartPriority,
		NormalStartPriority: r.cfg.PriorityRules.NormalStartPriority,
		FreeStartPriority:   r.cfg.PriorityRules.FreeStartPriority,
		MinChange:           r.cfg.MinChange,
		UrgencyTolerance:    r.cfg.UrgencyTolerance,
		CooldownAuthIndexes: cooldowns,
		CooldownReasons:     cooldownReasons,
		CustomTiers:         customTiers,
	}

	plan := priority.PlanWithEvidence(credentials, historicalEvidenceSource{historical: historical}, options)

	candidates := make(map[string]PrecomputedCandidate, len(plan.Items))
	var nextTransition time.Time

	for _, item := range plan.Items {
		authIdx := item.Credential.AuthIndex
		cdUntil, isCooling := cooldowns[authIdx]

		var shortReset, longReset time.Time
		if item.ShortWindowResetAt != nil {
			shortReset = *item.ShortWindowResetAt
		}
		if item.LongWindowResetAt != nil {
			longReset = *item.LongWindowResetAt
		}

		score := item.Priority
		reason := item.Reason

		if isCooling {
			score = -1
			reason = priority.Reason429Cooldown
			if specific := cooldownReasons[authIdx]; specific != "" {
				reason = specific
			}
		} else if item.Disabled && item.Priority == -1 {
			score = -2
		}

		candidates[authIdx] = PrecomputedCandidate{
			AuthIndex:          authIdx,
			Priority:           score,
			Urgency:            item.Urgency,
			ShortWindowResetAt: shortReset,
			LongWindowResetAt:  longReset,
			CooldownUntil:      cdUntil,
			Reason:             reason,
		}

		// Track earliest transition timestamp
		if !shortReset.IsZero() && shortReset.After(now) {
			nextTransition = minFutureTime(nextTransition, shortReset)
		}
		if !longReset.IsZero() && longReset.After(now) {
			nextTransition = minFutureTime(nextTransition, longReset)
		}
		if isCooling && cdUntil.After(now) {
			nextTransition = minFutureTime(nextTransition, cdUntil)
		}
	}
	for _, models := range blockedModels {
		for _, until := range models {
			if until.After(now) {
				nextTransition = minFutureTime(nextTransition, until)
			}
		}
	}

	return PoolRoutingTable{
		DecidedAt:        now,
		NextTransitionAt: nextTransition,
		ByAuthIndex:      candidates,
		BlockedModels:    blockedModels,
	}
}

// recomputeRoutingTablesLocked updates all 3 pool routing tables in memory.
func (r *Runtime) recomputeRoutingTablesLocked(now time.Time, existingCreds ...[]core.Credential) {
	cachePath := r.cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, _ := state.Load(context.Background(), cachePath)
	var credentials []core.Credential
	if len(existingCreds) > 0 {
		credentials = existingCreds[0]
	} else {
		credentials = r.currentCredentialsLocked()
	}
	for _, pool := range config.AllAntigravityModelGroups {
		r.plansByPool[pool] = r.recomputePoolRoutingTableLocked(pool, now, store, credentials)
	}
}

// currentCredentialsLocked extracts current known credentials from host or state cache.
func (r *Runtime) currentCredentialsLocked() []core.Credential {
	if r.hostCallbacks != nil {
		client := host.NewClient(r.hostCallbacks)
		if files, err := client.ListAuthFiles(context.Background()); err == nil && len(files) > 0 {
			creds := credentialsFromAuthFiles(files)
			return creds
		}
	}

	cachePath := r.cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	if store, err := state.Load(context.Background(), cachePath); err == nil {
		authIndexes := store.GetKnownAuthIndexes()
		var creds []core.Credential
		for _, idx := range authIndexes {
			creds = append(creds, core.Credential{
				AuthIndex: idx,
				Name:      idx,
				Provider:  core.ProviderAntigravity,
				Status:    core.CredentialStatusActive,
				PlanType:  store.GetPlanType(idx, string(config.AntigravityModelGroupGemini)),
			})
		}
		return creds
	}
	return nil
}

// getOrRefreshPoolTable returns the cached PoolRoutingTable or lazily re-plans if expired.
func (r *Runtime) getOrRefreshPoolTable(pool config.AntigravityModelGroup, now time.Time) PoolRoutingTable {
	r.routingMu.RLock()
	table, ok := r.plansByPool[pool]
	needsRefresh := !ok || (!table.NextTransitionAt.IsZero() && !now.Before(table.NextTransitionAt)) || now.Sub(table.DecidedAt) > 60*time.Second
	r.routingMu.RUnlock()

	if !needsRefresh {
		return table
	}

	r.routingMu.Lock()
	defer r.routingMu.Unlock()
	// Double check under write lock
	table, ok = r.plansByPool[pool]
	if ok && (table.NextTransitionAt.IsZero() || now.Before(table.NextTransitionAt)) && now.Sub(table.DecidedAt) <= 60*time.Second {
		return table
	}

	cachePath := r.cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, _ := state.Load(context.Background(), cachePath)
	credentials := r.currentCredentialsLocked()
	refreshed := r.recomputePoolRoutingTableLocked(pool, now, store, credentials)
	r.plansByPool[pool] = refreshed
	return refreshed
}

func minFutureTime(current, candidate time.Time) time.Time {
	if current.IsZero() {
		return candidate
	}
	if candidate.Before(current) {
		return candidate
	}
	return current
}

// handleSchedulerPick executes O(1) dynamic request scheduling across Antigravity credentials.
func (r *Runtime) handleSchedulerPick(ctx context.Context, request []byte) []byte {
	var req SchedulerPickRequest
	if err := json.Unmarshal(request, &req); err != nil {
		return failure(err)
	}

	r.mu.Lock()
	enabled := r.cfg.Enabled
	r.mu.Unlock()

	if !enabled {
		return envelopeSchedulerPick(SchedulerPickResponse{Handled: false})
	}

	var agCandidates []SchedulerAuthCandidate
	var otherCandidates []SchedulerAuthCandidate

	for _, c := range req.Candidates {
		if strings.TrimSpace(c.ID) == "" {
			continue
		}
		if isAntigravityUsageProvider(c.Provider) {
			agCandidates = append(agCandidates, c)
		} else {
			otherCandidates = append(otherCandidates, c)
		}
	}

	if len(agCandidates) == 0 {
		return envelopeSchedulerPick(SchedulerPickResponse{Handled: false})
	}

	now := r.clock.Now().UTC()
	pool := config.ClassifyModelQuotaPool(req.Model)
	poolTable := r.getOrRefreshPoolTable(pool, now)

	// Score Antigravity candidates
	type candidateScore struct {
		candidate SchedulerAuthCandidate
		authIndex string
		score     int
	}

	scores := make([]candidateScore, 0, len(agCandidates))
	bestScore := -1 << 31

	for _, c := range agCandidates {
		authIdx := r.resolveAuthIndex(c.ID, c.Attributes)
		candScore := 0 // default cold-start / unprobed

		pre, found := poolTable.ByAuthIndex[authIdx]
		if found {
			candScore = pre.Priority
		}
		if blocked := poolTable.BlockedModels[authIdx]; len(blocked) > 0 {
			if until, isBlocked := blocked[normalizeModelKey(req.Model)]; isBlocked && now.Before(until) {
				if candScore > priority.DepletedPriority {
					candScore = priority.DepletedPriority
				}
			}
		}
		scores = append(scores, candidateScore{
			candidate: c,
			authIndex: authIdx,
			score:     candScore,
		})
		if candScore > bestScore {
			bestScore = candScore
		}
	}

	// Branch A: Healthy or cold-start Antigravity candidate exists (bestScore >= 0)
	if bestScore >= 0 {
		var winners []SchedulerAuthCandidate
		for _, s := range scores {
			if s.score == bestScore {
				winners = append(winners, s.candidate)
			}
		}

		sort.Slice(winners, func(i, j int) bool {
			return winners[i].ID < winners[j].ID
		})

		sessionID := extractSessionID(req.Options)
		r.routingMu.Lock()
		defer r.routingMu.Unlock()

		// 1. Session Affinity Check
		if sessionID != "" {
			affKey := string(pool) + "::" + sessionID
			if binding, bound := r.sessionAffinity[affKey]; bound && now.Before(binding.ExpiresAt) {
				for _, w := range winners {
					if w.ID == binding.CandidateID {
						binding.ExpiresAt = now.Add(sessionAffinityTTL)
						r.sessionAffinity[affKey] = binding
						return envelopeSchedulerPick(SchedulerPickResponse{
							AuthID:  binding.CandidateID,
							Handled: true,
						})
					}
				}
			}
		}

		// 2. Round-Robin Rotation
		cursor := r.rrCursorByPool[pool]
		picked := pickSuccessor(winners, cursor)
		r.rrCursorByPool[pool] = picked.ID

		if sessionID != "" {
			affKey := string(pool) + "::" + sessionID
			r.sessionAffinity[affKey] = sessionBinding{
				CandidateID: picked.ID,
				ExpiresAt:   now.Add(sessionAffinityTTL),
			}
		}

		return envelopeSchedulerPick(SchedulerPickResponse{
			AuthID:  picked.ID,
			Handled: true,
		})
	}

	// Branch B: All Antigravity candidates are depleted or in cooldown (bestScore < 0)
	// Subcase B1: Mixed route with other healthy provider candidates available
	if len(otherCandidates) > 0 {
		maxOtherPriority := -1 << 31
		for _, oc := range otherCandidates {
			if oc.Priority > maxOtherPriority {
				maxOtherPriority = oc.Priority
			}
		}

		var bestOthers []SchedulerAuthCandidate
		for _, oc := range otherCandidates {
			if oc.Priority == maxOtherPriority {
				bestOthers = append(bestOthers, oc)
			}
		}

		sort.Slice(bestOthers, func(i, j int) bool {
			return bestOthers[i].ID < bestOthers[j].ID
		})

		r.routingMu.Lock()
		cursor := r.rrCursorByPool[pool]
		picked := pickSuccessor(bestOthers, cursor)
		r.rrCursorByPool[pool] = picked.ID
		r.routingMu.Unlock()

		return envelopeSchedulerPick(SchedulerPickResponse{
			AuthID:  picked.ID,
			Handled: true,
		})
	}

	// Subcase B2: Antigravity-only route and all credentials are depleted/cooling
	return envelopeSchedulerPick(SchedulerPickResponse{
		Handled: false,
	})
}

// pickSuccessor picks the next item in a sorted slice following cursor in a round-robin cycle.
func pickSuccessor(candidates []SchedulerAuthCandidate, cursor string) SchedulerAuthCandidate {
	if len(candidates) == 0 {
		return SchedulerAuthCandidate{}
	}
	if cursor == "" {
		return candidates[0]
	}
	for i, c := range candidates {
		if c.ID == cursor {
			return candidates[(i+1)%len(candidates)]
		}
	}
	return candidates[0]
}

// normalizeModelKey canonicalizes a model identifier for health-state lookups.
func normalizeModelKey(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

// extractSessionID extracts canonical session ID from Options metadata or headers.
func extractSessionID(options SchedulerOptions) string {
	if options.Metadata != nil {
		if sid, ok := options.Metadata["canonical_session_id"].(string); ok && strings.TrimSpace(sid) != "" {
			return strings.TrimSpace(sid)
		}
		if sid, ok := options.Metadata["session_id"].(string); ok && strings.TrimSpace(sid) != "" {
			return strings.TrimSpace(sid)
		}
	}
	if options.Headers != nil {
		for _, k := range []string{"x-session-id", "session-id", "x-conversation-id"} {
			for headerKey, vals := range options.Headers {
				if strings.EqualFold(headerKey, k) && len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
					return strings.TrimSpace(vals[0])
				}
			}
		}
	}
	return ""
}
