package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/state"
)

type devRunner struct {
	mu             sync.Mutex
	geminiSnapshot apply.PlanSnapshot
	claudeSnapshot apply.PlanSnapshot
	runHistory     []map[string]any
	scheduleConfig state.ScheduleConfig
	dynamicConfig  state.DynamicConfig
	latestAudit    string
}

func buildDevChanges(items []apply.SnapshotItem) []apply.SnapshotChange {
	res := make([]apply.SnapshotChange, 0)
	for _, item := range items {
		if item.Current.Priority != item.Target.Priority || item.Current.Disabled != item.Target.Disabled || item.Current.PriorityMissing {
			res = append(res, apply.SnapshotChange{
				Name:          item.Name,
				AuthIndex:     item.AuthIndex,
				Current:       item.Current,
				Target:        item.Target,
				EvidenceFresh: item.EvidenceFresh,
				Reason:        item.Reason,
				IsBoosted:     item.IsBoosted,
			})
		}
	}
	return res
}

func newDevRunner() *devRunner {
	now := time.Now().UTC()

	// Rolling quota reset timestamps
	shortHealthy := now.Add(2*time.Hour + 35*time.Minute + 12*time.Second)
	longBoost := now.Add(18 * time.Hour) // Urgency = 0.85 / 18 = 0.047, within boost horizon
	longHealthy := now.Add(4*24*time.Hour + 12*time.Hour + 30*time.Minute)
	shortDepleted := now.Add(22*time.Minute + 18*time.Second)
	longDepleted := now.Add(1*24*time.Hour + 6*time.Hour)

	rem90 := int64(90)
	rem85 := int64(85)
	rem80 := int64(80)
	rem5 := int64(5)
	rem0 := int64(0)

	// 7 Representative Credentials for Gemini (Primary)
	geminiItems := []apply.SnapshotItem{
		// 1. Tier 1 (Boosted): Abundant weekly balance entering boost horizon -> Priority 999
		{
			Name:                 "work-gemini-pro@corp.com",
			AuthIndex:            "auth_ag_001",
			Provider:             "antigravity",
			Type:                 "antigravity",
			Status:               "active",
			PlanType:             "Antigravity Gemini Pro",
			Current:              apply.Target{Priority: 100, Disabled: false},
			Target:               apply.Target{Priority: 999, Disabled: false},
			EvidenceFresh:        true,
			Reason:               "fresh boosted",
			IsBoosted:            true,
			Urgency:              1.42,
			R7d:                  0.85,
			T7d:                  0.60,
			R5h:                  0.90,
			T5h:                  0.40,
			CycleBurnRate:        0.18,
			TRequired:            24.5,
			ShortWindowResetAt:   &shortHealthy,
			ShortWindowRemaining: &rem90,
			LongWindowResetAt:    &longBoost,
			LongWindowRemaining:  &rem85,
		},
		// 2. Tier 2 (Regular Active): Healthy account Alpha -> Priority 100 (Unset on host initially)
		{
			Name:                 "developer-account-01@gmail.com",
			AuthIndex:            "auth_ag_002",
			Provider:             "antigravity",
			Type:                 "antigravity",
			Status:               "active",
			PlanType:             "Antigravity Gemini Flash",
			Current:              apply.Target{Priority: 0, PriorityMissing: true, Disabled: false}, // Unset on host
			Target:               apply.Target{Priority: 100, Disabled: false},
			EvidenceFresh:        true,
			Reason:               "fresh remaining positive",
			IsBoosted:            false,
			Urgency:              0.88,
			R7d:                  0.80,
			T7d:                  0.62,
			R5h:                  0.85,
			T5h:                  0.80,
			CycleBurnRate:        0.15,
			TRequired:            30.0,
			ShortWindowResetAt:   &shortHealthy,
			ShortWindowRemaining: &rem85,
			LongWindowResetAt:    &longHealthy,
			LongWindowRemaining:  &rem80,
		},
		// 3. Tier 2 (Regular Active): Healthy account Beta -> Priority 98
		{
			Name:                 "developer-account-02@gmail.com",
			AuthIndex:            "auth_ag_003",
			Provider:             "antigravity",
			Type:                 "antigravity",
			Status:               "active",
			PlanType:             "Antigravity Gemini Flash",
			Current:              apply.Target{Priority: 80, Disabled: false},
			Target:               apply.Target{Priority: 98, Disabled: false},
			EvidenceFresh:        true,
			Reason:               "fresh remaining positive",
			IsBoosted:            false,
			Urgency:              0.86,
			R7d:                  0.78,
			T7d:                  0.62,
			R5h:                  0.80,
			T5h:                  0.80,
			CycleBurnRate:        0.15,
			TRequired:            30.0,
			ShortWindowResetAt:   &shortHealthy,
			ShortWindowRemaining: &rem80,
			LongWindowResetAt:    &longHealthy,
			LongWindowRemaining:  &rem80,
		},
		// 4. Tier 3 (Soft Depleted): 5h Short window exhausted -> Priority -1, Disabled false
		{
			Name:                 "ci-runner-short-depleted@ci.org",
			AuthIndex:            "auth_ag_004",
			Provider:             "antigravity",
			Type:                 "antigravity",
			Status:               "active",
			PlanType:             "Antigravity Gemini Flash",
			Current:              apply.Target{Priority: 100, Disabled: false},
			Target:               apply.Target{Priority: -1, Disabled: false},
			EvidenceFresh:        true,
			Reason:               "fresh short window depleted",
			IsBoosted:            false,
			Urgency:              0.45,
			R7d:                  0.75,
			T7d:                  0.70,
			R5h:                  0.05,
			T5h:                  0.10,
			CycleBurnRate:        0.22,
			TRequired:            18.0,
			ShortWindowResetAt:   &shortDepleted,
			ShortWindowRemaining: &rem5,
			LongWindowResetAt:    &longHealthy,
			LongWindowRemaining:  &rem80,
		},
		// 5. Tier 3 (Hard Depleted): 7d Weekly quota exhausted -> Priority -1, Disabled true
		{
			Name:                 "heavy-scraper-weekly-depleted@corp.io",
			AuthIndex:            "auth_ag_005",
			Provider:             "antigravity",
			Type:                 "antigravity",
			Status:               "active",
			PlanType:             "Antigravity Gemini Pro",
			Current:              apply.Target{Priority: 100, Disabled: false},
			Target:               apply.Target{Priority: -1, Disabled: true},
			EvidenceFresh:        true,
			Reason:               "fresh weekly depleted",
			IsBoosted:            false,
			Urgency:              0.0,
			R7d:                  0.0,
			T7d:                  0.20,
			R5h:                  0.0,
			T5h:                  0.20,
			CycleBurnRate:        0.28,
			TRequired:            0.0,
			ShortWindowResetAt:   &shortDepleted,
			ShortWindowRemaining: &rem0,
			LongWindowResetAt:    &longDepleted,
			LongWindowRemaining:  &rem0,
		},
		// 6. 429 Reactive Cooldown (REQ-11): Rate limited, temporary soft cooldown -> Priority -1, Disabled false
		{
			Name:                 "rate-limited-cooldown@api.com",
			AuthIndex:            "auth_ag_006",
			Provider:             "antigravity",
			Type:                 "antigravity",
			Status:               "active",
			PlanType:             "Antigravity Gemini Flash",
			Current:              apply.Target{Priority: 100, Disabled: false},
			Target:               apply.Target{Priority: -1, Disabled: false},
			EvidenceFresh:        true,
			Reason:               "429 rate limit cooldown",
			IsBoosted:            false,
			Urgency:              0.70,
			R7d:                  0.65,
			T7d:                  0.50,
			R5h:                  0.70,
			T5h:                  0.50,
			CycleBurnRate:        0.15,
			TRequired:            20.0,
			ShortWindowResetAt:   &shortHealthy,
			ShortWindowRemaining: &rem80,
			LongWindowResetAt:    &longHealthy,
			LongWindowRemaining:  &rem80,
		},
		// 7. Manually Disabled in CPA (Bug 2 Protection): Disabled true on host -> Priority -1, Disabled true
		{
			Name:                 "cpa-manually-disabled@admin.com",
			AuthIndex:            "auth_ag_007",
			Provider:             "antigravity",
			Type:                 "antigravity",
			Status:               "active",
			PlanType:             "Antigravity Gemini Pro",
			Current:              apply.Target{Priority: -1, Disabled: true},
			Target:               apply.Target{Priority: -1, Disabled: true},
			EvidenceFresh:        true,
			Reason:               "disabled on host",
			IsBoosted:            false,
			Urgency:              0.80,
			R7d:                  0.80,
			T7d:                  0.50,
			R5h:                  0.80,
			T5h:                  0.50,
			CycleBurnRate:        0.15,
			TRequired:            20.0,
			ShortWindowResetAt:   &shortHealthy,
			ShortWindowRemaining: &rem80,
			LongWindowResetAt:    &longHealthy,
			LongWindowRemaining:  &rem80,
		},
	}

	geminiChanges := buildDevChanges(geminiItems)

	geminiSnapshot := apply.PlanSnapshot{
		TotalItems:   len(geminiItems),
		TotalChanges: len(geminiChanges),
		Items:        geminiItems,
		Changes:      geminiChanges,
	}

	// Predicted snapshot for Claude & GPT Group (REQ-05)
	claudeItems := make([]apply.SnapshotItem, len(geminiItems))
	for i, item := range geminiItems {
		item.PlanType = "Antigravity Claude/GPT"
		item.IsPredicted = true
		item.Reason = "predicted: " + item.Reason
		if i == 0 {
			item.Target.Priority = 980
		} else if i == 1 || i == 2 {
			item.Target.Priority = 100
		}
		claudeItems[i] = item
	}
	claudeChanges := buildDevChanges(claudeItems)
	claudeSnapshot := apply.PlanSnapshot{
		TotalItems:   len(claudeItems),
		TotalChanges: len(claudeChanges),
		Items:        claudeItems,
		Changes:      claudeChanges,
	}

	// Execution History (REQ-07: includes embedded snapshots for inspection)
	history := []map[string]any{
		{
			"at":        now.Add(-10 * time.Minute),
			"kind":      "apply",
			"trigger":   "auto_apply",
			"attempted": 6,
			"succeeded": 6,
			"failed":    0,
			"skipped":   1,
			"message":   "auto_apply credentials=7 succeeded=6 failed=0 skipped=1",
			"snapshot":  geminiSnapshot,
		},
		{
			"at":        now.Add(-25 * time.Minute),
			"kind":      "apply",
			"trigger":   "manual_apply",
			"attempted": 6,
			"succeeded": 6,
			"failed":    0,
			"skipped":   1,
			"message":   "manual_apply credentials=7 succeeded=6 failed=0 skipped=1",
			"snapshot":  geminiSnapshot,
		},
		{
			"at":        now.Add(-40 * time.Minute),
			"kind":      "probe",
			"trigger":   "manual",
			"attempted": 7,
			"succeeded": 7,
			"failed":    0,
			"skipped":   0,
			"message":   "probe completed: 7 credentials probed",
		},
	}

	return &devRunner{
		geminiSnapshot: geminiSnapshot,
		claudeSnapshot: claudeSnapshot,
		runHistory:     history,
		latestAudit:    "all 7 credentials double-window monitored & quota paced",
		scheduleConfig: state.ScheduleConfig{
			Paused:        false,
			WindowEnabled: true,
			WindowStart:   "09:00",
			WindowEnd:     "23:00",
		},
		dynamicConfig: state.DynamicConfig{
			AutoApply:                true,
			Interval:                 "15m",
			AntigravityModelGroup:    "gemini",
			MaxConcurrency:           6,
			MinChange:                1,
			UrgencyTolerance:         0.05,
			QuotaSampleCapacity:      6,
			RateLimitCooldownMinutes: 5,
			PriorityRules: state.PriorityRulesConfig{
				BoostStartPriority:  999,
				NormalStartPriority: 100,
			},
			Schedule: state.ScheduleConfig{
				Paused:        false,
				WindowEnabled: true,
				WindowStart:   "09:00",
				WindowEnd:     "23:00",
			},
		},
	}
}

func (d *devRunner) Run(ctx context.Context, request management.RunRequest) (apply.Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UTC()

	// Probe mode (REQ-04)
	if request.Mode == "probe" {
		d.latestAudit = fmt.Sprintf("probe completed: %d credentials refreshed at %s", len(d.geminiSnapshot.Items), now.Format("15:04:05"))
		d.runHistory = append([]map[string]any{
			{
				"at":        now,
				"kind":      "probe",
				"trigger":   "probe",
				"attempted": len(d.geminiSnapshot.Items),
				"succeeded": len(d.geminiSnapshot.Items),
				"failed":    0,
				"skipped":   0,
				"message":   d.latestAudit,
			},
		}, d.runHistory...)
		return apply.Result{
			Attempted: len(d.geminiSnapshot.Items),
			Succeeded: len(d.geminiSnapshot.Items),
			Snapshot:  d.geminiSnapshot,
		}, nil
	}

	if request.Mode != "apply" {
		return apply.Result{}, fmt.Errorf("unsupported mode: %s", request.Mode)
	}

	activeChangesBefore := d.geminiSnapshot.Changes
	activeItems := d.geminiSnapshot.Items
	if d.dynamicConfig.AntigravityModelGroup == "claude_gpt" {
		activeChangesBefore = d.claudeSnapshot.Changes
		activeItems = d.claudeSnapshot.Items
	}

	if len(activeChangesBefore) == 0 {
		d.latestAudit = fmt.Sprintf("all %d credentials in sync, no changes required", len(activeItems))
		return apply.Result{
			Attempted: 0,
			Succeeded: 0,
			Skipped:   len(activeItems),
			Snapshot:  d.geminiSnapshot,
		}, nil
	}

	for i := range d.geminiSnapshot.Items {
		d.geminiSnapshot.Items[i].Current = d.geminiSnapshot.Items[i].Target
		d.geminiSnapshot.Items[i].Current.PriorityMissing = false
	}
	for i := range d.claudeSnapshot.Items {
		d.claudeSnapshot.Items[i].Current = d.claudeSnapshot.Items[i].Target
		d.claudeSnapshot.Items[i].Current.PriorityMissing = false
	}
	d.geminiSnapshot.Changes = buildDevChanges(d.geminiSnapshot.Items)
	d.claudeSnapshot.Changes = buildDevChanges(d.claudeSnapshot.Items)

	result := apply.Result{
		Attempted: len(activeChangesBefore),
		Succeeded: len(activeChangesBefore),
		Failed:    0,
		Skipped:   len(activeItems) - len(activeChangesBefore),
		Snapshot:  d.geminiSnapshot,
		Changes:   make([]apply.ChangeResult, 0),
	}
	if d.dynamicConfig.AntigravityModelGroup == "claude_gpt" {
		result.Snapshot = d.claudeSnapshot
	}

	for _, c := range activeChangesBefore {
		result.Changes = append(result.Changes, apply.ChangeResult{
			Name:            c.Name,
			AuthIndex:       c.AuthIndex,
			Status:          apply.ChangeStatusSuccess,
			Success:         true,
			PriorityFrom:    c.Current.Priority,
			PriorityMissing: c.Current.PriorityMissing,
			PriorityTo:      c.Target.Priority,
			DisabledFrom:    c.Current.Disabled,
			DisabledTo:      c.Target.Disabled,
		})
	}

	d.latestAudit = fmt.Sprintf("apply completed: %d succeeded, 0 failed, %d skipped", len(activeChangesBefore), result.Skipped)
	d.runHistory = append([]map[string]any{
		{
			"at":        now,
			"kind":      "apply",
			"trigger":   "manual_apply",
			"attempted": result.Attempted,
			"succeeded": result.Succeeded,
			"failed":    0,
			"skipped":   result.Skipped,
			"message":   d.latestAudit,
			"snapshot":  result.Snapshot,
		},
	}, d.runHistory...)

	return result, nil
}

func (d *devRunner) Reset(ctx context.Context) (map[string]any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.geminiSnapshot.Items {
		if !d.geminiSnapshot.Items[i].Current.Disabled {
			d.geminiSnapshot.Items[i].Current.Priority = 0
			d.geminiSnapshot.Items[i].Current.PriorityMissing = true
		}
	}
	for i := range d.claudeSnapshot.Items {
		if !d.claudeSnapshot.Items[i].Current.Disabled {
			d.claudeSnapshot.Items[i].Current.Priority = 0
			d.claudeSnapshot.Items[i].Current.PriorityMissing = true
		}
	}
	d.geminiSnapshot.Changes = buildDevChanges(d.geminiSnapshot.Items)
	d.claudeSnapshot.Changes = buildDevChanges(d.claudeSnapshot.Items)

	d.latestAudit = fmt.Sprintf("reset %d credential priorities to default", len(d.geminiSnapshot.Items))
	now := time.Now().UTC()
	d.runHistory = append([]map[string]any{
		{
			"at":        now,
			"kind":      "reset",
			"trigger":   "manual",
			"attempted": len(d.geminiSnapshot.Items),
			"succeeded": len(d.geminiSnapshot.Items),
			"failed":    0,
			"skipped":   0,
			"message":   d.latestAudit,
			"snapshot":  d.geminiSnapshot,
		},
	}, d.runHistory...)
	return map[string]any{
		"ok":          true,
		"message":     d.latestAudit,
		"reset_count": len(d.geminiSnapshot.Items),
	}, nil
}

func (d *devRunner) Status(ctx context.Context) (management.StatusInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return management.StatusInfo{
		TotalCredentials: len(d.geminiSnapshot.Items),
		FreshCount:       len(d.geminiSnapshot.Items),
		LatestAudit:      d.latestAudit,
	}, nil
}

func (d *devRunner) LatestSnapshot(ctx context.Context) (apply.DualGroupSnapshot, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	activeGroup := d.dynamicConfig.AntigravityModelGroup
	if activeGroup == "" {
		activeGroup = "gemini"
	}
	return d.dualSnapshot(activeGroup), nil
}

func (d *devRunner) SyncHost(ctx context.Context, modelGroup config.AntigravityModelGroup) (apply.DualGroupSnapshot, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	activeGroup := d.dynamicConfig.AntigravityModelGroup
	if activeGroup == "" {
		activeGroup = "gemini"
	}
	d.latestAudit = fmt.Sprintf("synchronized %d credentials from mock host at %s", len(d.geminiSnapshot.Items), time.Now().UTC().Format("15:04:05"))
	return d.dualSnapshot(activeGroup), nil
}

func (d *devRunner) dualSnapshot(activeGroup string) apply.DualGroupSnapshot {
	primary := devSnapshotRole(d.geminiSnapshot, false)
	predicted := devSnapshotRole(d.claudeSnapshot, true)
	if activeGroup == "claude_gpt" {
		primary = devSnapshotRole(d.claudeSnapshot, false)
		predicted = devSnapshotRole(d.geminiSnapshot, true)
	}
	groups := map[string]apply.GroupSnapshot{}
	if activeGroup == "claude_gpt" {
		groups["claude_gpt"] = apply.GroupSnapshot{Items: primary.Items, Changes: primary.Changes}
		groups["gemini"] = apply.GroupSnapshot{Items: predicted.Items, Changes: predicted.Changes}
	} else {
		groups["gemini"] = apply.GroupSnapshot{Items: primary.Items, Changes: primary.Changes}
		groups["claude_gpt"] = apply.GroupSnapshot{Items: predicted.Items, Changes: predicted.Changes}
	}
	return apply.DualGroupSnapshot{ActiveModelGroup: activeGroup, ObservedAt: time.Now().UTC(), Groups: groups}
}

func devSnapshotRole(snapshot apply.PlanSnapshot, predicted bool) apply.PlanSnapshot {
	copy := snapshot
	copy.Items = append([]apply.SnapshotItem(nil), snapshot.Items...)
	copy.Changes = append([]apply.SnapshotChange(nil), snapshot.Changes...)
	for index := range copy.Items {
		copy.Items[index].IsPredicted = predicted
		copy.Items[index].Reason = devReasonRole(copy.Items[index].Reason, predicted)
	}
	for index := range copy.Changes {
		copy.Changes[index].Reason = devReasonRole(copy.Changes[index].Reason, predicted)
	}
	return copy
}

func devReasonRole(reason string, predicted bool) string {
	reason = strings.TrimPrefix(reason, "predicted: ")
	if predicted && reason != "" {
		return "predicted: " + reason
	}
	return reason
}

func (d *devRunner) GetSamples(ctx context.Context, authIndex, modelGroup string) ([]state.QuotaSample, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now().UTC()
	if modelGroup == "claude_gpt" {
		return []state.QuotaSample{
			{
				ObservedAt:     now.Add(-45 * time.Minute),
				ShortWindowRem: 90,
				LongWindowRem:  80,
			},
			{
				ObservedAt:     now.Add(-30 * time.Minute),
				ShortWindowRem: 85,
				LongWindowRem:  78,
			},
			{
				ObservedAt:     now.Add(-15 * time.Minute),
				ShortWindowRem: 80,
				LongWindowRem:  75,
			},
			{
				ObservedAt:     now,
				ShortWindowRem: 75,
				LongWindowRem:  72,
			},
		}, nil
	}
	return []state.QuotaSample{
		{
			ObservedAt:     now.Add(-45 * time.Minute),
			ShortWindowRem: 95,
			LongWindowRem:  85,
		},
		{
			ObservedAt:     now.Add(-30 * time.Minute),
			ShortWindowRem: 90,
			LongWindowRem:  82,
		},
		{
			ObservedAt:     now.Add(-15 * time.Minute),
			ShortWindowRem: 85,
			LongWindowRem:  80,
		},
		{
			ObservedAt:     now,
			ShortWindowRem: 80,
			LongWindowRem:  78,
		},
	}, nil
}

func (d *devRunner) Diagnostics(ctx context.Context) (map[string]any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	interval, err := time.ParseDuration(d.dynamicConfig.Interval)
	if err != nil || interval <= 0 {
		interval = 15 * time.Minute
	}
	lastAuto := time.Now().Add(-10 * time.Second)
	nextWaitDuration := interval - 10*time.Second
	if nextWaitDuration <= 0 {
		nextWaitDuration = interval
	}
	nextRunAt := time.Now().UTC().Add(nextWaitDuration)
	var latestApply map[string]any
	for _, entry := range d.runHistory {
		if entry["kind"] == "apply" {
			latestApply = entry
			break
		}
	}

	return map[string]any{
		"management_api": map[string]any{
			"status":     "ready",
			"auto_apply": d.dynamicConfig.AutoApply,
			"enabled":    true,
		},
		"scheduler": map[string]any{
			"interval":           d.dynamicConfig.Interval,
			"last_auto_apply_at": lastAuto,
			"next_wait":          nextWaitDuration.String(),
			"next_run_at":        nextRunAt.Format(time.RFC3339),
			"worker_active":      true,
			"paused":             d.scheduleConfig.Paused,
			"window_enabled":     d.scheduleConfig.WindowEnabled,
			"window_start":       d.scheduleConfig.WindowStart,
			"window_end":         d.scheduleConfig.WindowEnd,
		},
		"active_cooldowns": []map[string]any{
			{
				"auth_index":     "rate-limited-cooldown@api.com",
				"model_group":    "gemini",
				"triggered_at":   time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339),
				"cooldown_until": time.Now().UTC().Add(3*time.Minute + 25*time.Second).Format(time.RFC3339),
				"reason":         "429 rate limit cooldown",
			},
		},
		"latest_audit": d.latestAudit,
		"latest_apply": latestApply,
		"last_result": apply.Result{
			Attempted: 6,
			Succeeded: 6,
			Failed:    0,
			Skipped:   1,
		},
		"run_history": d.runHistory,
	}, nil
}

func (d *devRunner) GetScheduleConfig(ctx context.Context) (state.ScheduleConfig, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.scheduleConfig, nil
}

func (d *devRunner) SetScheduleConfig(ctx context.Context, cfg state.ScheduleConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.scheduleConfig = cfg
	d.dynamicConfig.Schedule = cfg
	return nil
}

func (d *devRunner) GetDynamicConfig(ctx context.Context) (state.DynamicConfig, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dynamicConfig, nil
}

func (d *devRunner) SetDynamicConfig(ctx context.Context, cfg state.DynamicConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dynamicConfig = cfg
	d.scheduleConfig = cfg.Schedule
	return nil
}

func main() {
	runner := newDevRunner()
	handler := management.NewHandler(runner)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	fmt.Println("=========================================================")
	fmt.Println(" Antigravity Priority - Embedded WebUI Dev Server (v1.1.0)")
	fmt.Println("=========================================================")
	fmt.Println(" Server listening on: http://localhost:8080/status")
	fmt.Println(" Open the link above in your browser to interact with the UI!")
	fmt.Println(" Supports full verification of REQ-01 through REQ-11 & Bug Fixes")
	fmt.Println(" Press Ctrl+C to stop the server.")
	fmt.Println("=========================================================")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("devserver error: %v", err)
	}
}
