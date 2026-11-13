package runtime

import (
	"context"
	"encoding/json"
	"maps"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
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
	Disabled           bool
	RoutingStatus      apply.RoutingStatus
	PoolEligible       bool
	IsBoosted          bool
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

const (
	sessionAffinityTTL = 30 * time.Minute

	// coldStartRoutingTier is the routing score for an active credential that
	// has no quota evidence yet. It ranks below every healthy planned tier but
	// above depleted or cooling candidates, so a new credential can serve
	// traffic only after healthy peers are exhausted.
	coldStartRoutingTier = 0

	// disabledRoutingTier keeps a Host-disabled credential out of routing
	// regardless of the static priority left on the Host.
	disabledRoutingTier = -2
)

// poolRouter owns the in-memory three-pool routing tables, host model/auth
// inventory lookup maps, round-robin cursors, and session affinity state under
// a single RWMutex.
type poolRouter struct {
	mu                    sync.RWMutex
	plansByPool           map[config.AntigravityModelGroup]PoolRoutingTable
	authIDToIndex         map[string]string
	supportedModelsByAuth map[string]map[string]struct{}
	modelCatalog          []ModelCatalogItem
	rrCursorByPool        map[config.AntigravityModelGroup]string
	sessionAffinity       map[string]sessionBinding
}

func newPoolRouter() poolRouter {
	return poolRouter{
		plansByPool:           make(map[config.AntigravityModelGroup]PoolRoutingTable),
		authIDToIndex:         make(map[string]string),
		supportedModelsByAuth: make(map[string]map[string]struct{}),
		modelCatalog:          append([]ModelCatalogItem(nil), builtInAntigravityModels...),
		rrCursorByPool:        make(map[config.AntigravityModelGroup]string),
		sessionAffinity:       make(map[string]sessionBinding),
	}
}

func (pr *poolRouter) ensureMapsLocked() {
	if pr.plansByPool == nil {
		pr.plansByPool = make(map[config.AntigravityModelGroup]PoolRoutingTable)
	}
	if pr.authIDToIndex == nil {
		pr.authIDToIndex = make(map[string]string)
	}
	if pr.supportedModelsByAuth == nil {
		pr.supportedModelsByAuth = make(map[string]map[string]struct{})
	}
	if pr.rrCursorByPool == nil {
		pr.rrCursorByPool = make(map[config.AntigravityModelGroup]string)
	}
	if pr.sessionAffinity == nil {
		pr.sessionAffinity = make(map[string]sessionBinding)
	}
}

// ResolveAuthIndex resolves candidate ID or attributes to the internal auth index.
func (pr *poolRouter) ResolveAuthIndex(candidateID string, attributes map[string]string) string {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	cleanID := strings.TrimSpace(candidateID)
	if idx, ok := pr.authIDToIndex[cleanID]; ok && idx != "" {
		return idx
	}
	if len(attributes) > 0 {
		for _, key := range []string{"auth_index", "path", "name", "email", "account"} {
			if val, ok := attributes[key]; ok && strings.TrimSpace(val) != "" {
				trimmed := strings.TrimSpace(val)
				if idx, found := pr.authIDToIndex[trimmed]; found && idx != "" {
					return idx
				}
				if key == "path" {
					base := filepath.Base(trimmed)
					if idx, found := pr.authIDToIndex[base]; found && idx != "" {
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

func isAntigravityProviderName(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return p == "antigravity" || p == "google" || p == "gemini" || p == "google-antigravity"
}

// UpdateInventory refreshes auth/model lookup tables from host inventory under a write lock.
func (pr *poolRouter) UpdateInventory(files []host.AuthFile, models []host.AuthModelsEntry) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.updateInventoryLocked(files, models)
}

func (pr *poolRouter) updateInventoryLocked(files []host.AuthFile, models []host.AuthModelsEntry) {
	pr.authIDToIndex = make(map[string]string)
	pr.supportedModelsByAuth = make(map[string]map[string]struct{})
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
		pr.authIDToIndex[idx] = idx
		if file.ID != "" {
			antigravityAuthIDs[file.ID] = struct{}{}
			pr.authIDToIndex[file.ID] = idx
		}
		if file.Name != "" {
			antigravityAuthIDs[file.Name] = struct{}{}
			pr.authIDToIndex[file.Name] = idx
		}
		if file.Email != "" {
			antigravityAuthIDs[file.Email] = struct{}{}
			pr.authIDToIndex[file.Email] = idx
		}
		if file.Account != "" {
			antigravityAuthIDs[file.Account] = struct{}{}
			pr.authIDToIndex[file.Account] = idx
		}
		if file.Path != "" {
			antigravityAuthIDs[file.Path] = struct{}{}
			antigravityAuthIDs[filepath.Base(file.Path)] = struct{}{}
			antigravityAuthIDs[filepath.ToSlash(file.Path)] = struct{}{}
			pr.authIDToIndex[file.Path] = idx
			pr.authIDToIndex[filepath.Base(file.Path)] = idx
			pr.authIDToIndex[filepath.ToSlash(file.Path)] = idx
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
				pr.authIDToIndex[entry.ID] = authIdx
			}
			pr.authIDToIndex[authIdx] = authIdx
		}

		if len(entry.Models) > 0 && authIdx != "" {
			set := pr.supportedModelsByAuth[authIdx]
			if set == nil {
				set = make(map[string]struct{})
				pr.supportedModelsByAuth[authIdx] = set
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
	pr.modelCatalog = mergedCatalog
}

// routingConfigSnapshot returns an immutable snapshot of the configuration fields consumed by routing.
func (r *Runtime) routingConfigSnapshot() config.Config {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg := r.cfg
	cfg.CustomTiers = maps.Clone(r.cfg.CustomTiers)
	cfg.Warmup = r.cfg.Warmup.Clone()
	cfg.ExcludedModels = append([]string(nil), r.cfg.ExcludedModels...)
	return cfg
}

// CatalogSnapshot returns an immutable copy of the registered model catalog and per-credential capabilities.
func (pr *poolRouter) CatalogSnapshot() ([]apply.ModelOption, map[string][]string) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.catalogSnapshotLocked()
}

// catalogSnapshotLocked reads model catalog and per-credential supported models while pr.mu is held.
func (pr *poolRouter) catalogSnapshotLocked() ([]apply.ModelOption, map[string][]string) {
	catalog := pr.modelCatalog
	if len(catalog) == 0 {
		catalog = builtInAntigravityModels
	}
	models := make([]apply.ModelOption, len(catalog))
	for i, item := range catalog {
		models[i] = apply.ModelOption{
			ID:          item.ID,
			DisplayName: item.DisplayName,
			QuotaPool:   string(item.QuotaPool),
		}
	}
	supportedMap := make(map[string][]string, len(pr.supportedModelsByAuth))
	for authIdx, set := range pr.supportedModelsByAuth {
		if len(set) > 0 {
			supported := make([]string, 0, len(set))
			for m := range set {
				supported = append(supported, m)
			}
			sort.Strings(supported)
			supportedMap[authIdx] = supported
		}
	}
	return models, supportedMap
}

func (pr *poolRouter) ResetTables() {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.plansByPool = make(map[config.AntigravityModelGroup]PoolRoutingTable)
}

func (pr *poolRouter) EvaluateAndCommit(
	inventoryFiles []host.AuthFile,
	inventoryModels []host.AuthModelsEntry,
	evalFn func(models []apply.ModelOption, supportedMap map[string][]string) (RoutingEngineOutput, error),
) (RoutingEngineOutput, error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.ensureMapsLocked()
	if inventoryFiles != nil || inventoryModels != nil {
		pr.updateInventoryLocked(inventoryFiles, inventoryModels)
	}
	models, supportedMap := pr.catalogSnapshotLocked()
	out, err := evalFn(models, supportedMap)
	if err != nil {
		return RoutingEngineOutput{}, err
	}
	pr.plansByPool = out.Tables
	return out, nil
}

func (pr *poolRouter) EvaluateAndCapture(
	authIndex string,
	evalFn func(models []apply.ModelOption, supportedMap map[string][]string) (RoutingEngineOutput, error),
) map[string]PrecomputedCandidate {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.ensureMapsLocked()
	models, supportedMap := pr.catalogSnapshotLocked()
	out, err := evalFn(models, supportedMap)
	if err == nil {
		pr.plansByPool = out.Tables
	}
	captured := make(map[string]PrecomputedCandidate, len(pr.plansByPool))
	for pool, table := range pr.plansByPool {
		if cand, ok := table.ByAuthIndex[authIndex]; ok {
			captured[string(pool)] = cand
		}
	}
	return captured
}

// resolveCurrentCredentials extracts current known credentials from host or state cache without holding r.router.mu.
func (r *Runtime) resolveCurrentCredentials() []core.Credential {
	if r.hostCallbacks != nil {
		client := host.NewClient(r.hostCallbacks)
		if files, err := client.ListAuthFiles(context.Background()); err == nil && len(files) > 0 {
			return credentialsFromAuthFiles(files)
		}
	}

	if store := r.state.StoreOrNil(); store != nil {
		authIndexes := store.GetKnownAuthIndexes()
		creds := make([]core.Credential, 0, len(authIndexes))
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

func isPoolTableFresh(table PoolRoutingTable, ok bool, now time.Time) bool {
	return ok && (table.NextTransitionAt.IsZero() || now.Before(table.NextTransitionAt)) && now.Sub(table.DecidedAt) <= 60*time.Second
}

// getOrRefreshPoolTable returns the cached PoolRoutingTable or lazily re-plans if expired.
func (r *Runtime) getOrRefreshPoolTable(pool config.AntigravityModelGroup, now time.Time) PoolRoutingTable {
	r.router.mu.RLock()
	table, ok := r.router.plansByPool[pool]
	needsRefresh := !isPoolTableFresh(table, ok, now)
	r.router.mu.RUnlock()

	if !needsRefresh {
		return table
	}

	cfg := r.routingConfigSnapshot()
	credentials := enrichCredentialsPlanTypeFromStore(r.state.StoreOrNil(), r.resolveCurrentCredentials())

	r.router.mu.Lock()
	defer r.router.mu.Unlock()
	r.router.ensureMapsLocked()
	// Double check under write lock
	table, ok = r.router.plansByPool[pool]
	if isPoolTableFresh(table, ok, now) {
		return table
	}

	r.router.pruneExpiredSessionAffinityLocked(now)
	store := r.state.StoreOrNil()
	poolEvidence := buildPoolProjectionEvidence(store, credentials, pool)
	baseOptions := planningOptionsForEngine(cfg, store, now)
	eval := evaluateSinglePool(pool, credentials, poolEvidence, store, baseOptions, now)
	r.router.plansByPool[pool] = eval.table
	return eval.table
}

// pruneExpiredSessionAffinityLocked drops bindings that can no longer be
// honored. It runs only while pr.mu is held for writing, so the map cannot
// change underneath the sweep.
func (pr *poolRouter) pruneExpiredSessionAffinityLocked(now time.Time) {
	for key, binding := range pr.sessionAffinity {
		if !now.Before(binding.ExpiresAt) {
			delete(pr.sessionAffinity, key)
		}
	}
}

func (pr *poolRouter) SelectCandidate(pool config.AntigravityModelGroup, winners []SchedulerAuthCandidate, sessionID string, now time.Time) string {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.ensureMapsLocked()

	if sessionID != "" {
		affKey := string(pool) + "::" + sessionID
		if binding, bound := pr.sessionAffinity[affKey]; bound && now.Before(binding.ExpiresAt) {
			for _, w := range winners {
				if w.ID == binding.CandidateID {
					binding.ExpiresAt = now.Add(sessionAffinityTTL)
					pr.sessionAffinity[affKey] = binding
					return binding.CandidateID
				}
			}
		}
	}

	cursor := pr.rrCursorByPool[pool]
	picked := pickSuccessor(winners, cursor)
	pr.rrCursorByPool[pool] = picked.ID

	if sessionID != "" {
		affKey := string(pool) + "::" + sessionID
		pr.sessionAffinity[affKey] = sessionBinding{
			CandidateID: picked.ID,
			ExpiresAt:   now.Add(sessionAffinityTTL),
		}
	}
	return picked.ID
}

func (pr *poolRouter) NextCandidate(pool config.AntigravityModelGroup, candidates []SchedulerAuthCandidate) string {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.ensureMapsLocked()
	cursor := pr.rrCursorByPool[pool]
	picked := pickSuccessor(candidates, cursor)
	pr.rrCursorByPool[pool] = picked.ID
	return picked.ID
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
func (r *Runtime) handleSchedulerPick(_ context.Context, request []byte) []byte {
	var req SchedulerPickRequest
	if err := json.Unmarshal(request, &req); err != nil {
		return failure(err)
	}

	r.mu.Lock()
	enabled := r.cfg.Enabled
	excluded := config.IsModelExcluded(r.cfg.ExcludedModels, req.Model)
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

	pool := config.ClassifyModelQuotaPool(req.Model)

	if excluded {
		return envelopeSchedulerPick(r.pickFallbackOtherCandidate(pool, otherCandidates))
	}

	now := r.clock.Now().UTC()
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
		authIdx := r.router.ResolveAuthIndex(c.ID, c.Attributes)
		candScore := coldStartRoutingTier

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
		pickedID := r.router.SelectCandidate(pool, winners, sessionID, now)
		return envelopeSchedulerPick(SchedulerPickResponse{
			AuthID:  pickedID,
			Handled: true,
		})
	}

	// Branch B: All Antigravity candidates are depleted or in cooldown (bestScore < 0)
	return envelopeSchedulerPick(r.pickFallbackOtherCandidate(pool, otherCandidates))
}

// pickFallbackOtherCandidate routes mixed-provider requests to the highest-priority
// non-Antigravity candidate when Antigravity is depleted, cooling, or excluded.
func (r *Runtime) pickFallbackOtherCandidate(pool config.AntigravityModelGroup, otherCandidates []SchedulerAuthCandidate) SchedulerPickResponse {
	if len(otherCandidates) == 0 {
		return SchedulerPickResponse{Handled: false}
	}
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

	pickedID := r.router.NextCandidate(pool, bestOthers)
	return SchedulerPickResponse{
		AuthID:  pickedID,
		Handled: true,
	}
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
