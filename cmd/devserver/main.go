package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/management"
)

type devRunner struct {
	mu         sync.Mutex
	snapshot   apply.PlanSnapshot
	runHistory []map[string]any
}

func newDevRunner() *devRunner {
	now := time.Now().UTC()
	short1 := now.Add(2*time.Hour + 35*time.Minute + 12*time.Second)
	long1 := now.Add(4*24*time.Hour + 12*time.Hour + 30*time.Minute)
	remShort1 := int64(90)
	remLong1 := int64(85)

	short2 := now.Add(4*time.Hour + 10*time.Minute + 45*time.Second)
	long2 := now.Add(5*24*time.Hour + 8*time.Hour)
	remShort2 := int64(70)
	remLong2 := int64(55)

	short3 := now.Add(22*time.Minute + 18*time.Second)
	long3 := now.Add(6*24*time.Hour + 2*time.Hour)
	remShort3 := int64(5)
	remLong3 := int64(75)

	short4 := now.Add(1*time.Hour + 5*time.Minute)
	long4 := now.Add(1*24*time.Hour + 6*time.Hour)
	remShort4 := int64(0)
	remLong4 := int64(0)

	snapshot := apply.PlanSnapshot{
		TotalItems:   4,
		TotalChanges: 3,
		Items: []apply.SnapshotItem{
			{
				Name:                 "work-pro-gemini@corp.com",
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
				ShortWindowResetAt:   &short1,
				ShortWindowRemaining: &remShort1,
				LongWindowResetAt:    &long1,
				LongWindowRemaining:  &remLong1,
			},
			{
				Name:                 "personal-antigravity-dev",
				AuthIndex:            "auth_ag_002",
				Provider:             "antigravity",
				Type:                 "antigravity",
				Status:               "active",
				PlanType:             "Antigravity Claude/GPT",
				Current:              apply.Target{Priority: 100, Disabled: false},
				Target:               apply.Target{Priority: 899, Disabled: false},
				EvidenceFresh:        true,
				Reason:               "fresh remaining positive",
				IsBoosted:            false,
				Urgency:              0.88,
				R7d:                  0.55,
				T7d:                  0.62,
				R5h:                  0.70,
				T5h:                  0.80,
				CycleBurnRate:        0.15,
				TRequired:            30.0,
				ShortWindowResetAt:   &short2,
				ShortWindowRemaining: &remShort2,
				LongWindowResetAt:    &long2,
				LongWindowRemaining:  &remLong2,
			},
			{
				Name:                 "ci-runner-test-account",
				AuthIndex:            "auth_ag_003",
				Provider:             "antigravity",
				Type:                 "antigravity",
				Status:               "active",
				PlanType:             "Antigravity Gemini Flash",
				Current:              apply.Target{Priority: 850, Disabled: false},
				Target:               apply.Target{Priority: 1, Disabled: false},
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
				ShortWindowResetAt:   &short3,
				ShortWindowRemaining: &remShort3,
				LongWindowResetAt:    &long3,
				LongWindowRemaining:  &remLong3,
			},
			{
				Name:                 "heavy-batch-scraper@org.io",
				AuthIndex:            "auth_ag_004",
				Provider:             "antigravity",
				Type:                 "antigravity",
				Status:               "active",
				PlanType:             "Antigravity Pro",
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
				ShortWindowResetAt:   &short4,
				ShortWindowRemaining: &remShort4,
				LongWindowResetAt:    &long4,
				LongWindowRemaining:  &remLong4,
			},
		},
		Changes: []apply.SnapshotChange{
			{
				Name:          "work-pro-gemini@corp.com",
				AuthIndex:     "auth_ag_001",
				Current:       apply.Target{Priority: 100, Disabled: false},
				Target:        apply.Target{Priority: 999, Disabled: false},
				EvidenceFresh: true,
				Reason:        "fresh boosted",
				IsBoosted:     true,
			},
			{
				Name:          "personal-antigravity-dev",
				AuthIndex:     "auth_ag_002",
				Current:       apply.Target{Priority: 100, Disabled: false},
				Target:        apply.Target{Priority: 899, Disabled: false},
				EvidenceFresh: true,
				Reason:        "fresh remaining positive",
				IsBoosted:     false,
			},
			{
				Name:          "ci-runner-test-account",
				AuthIndex:     "auth_ag_003",
				Current:       apply.Target{Priority: 850, Disabled: false},
				Target:        apply.Target{Priority: 1, Disabled: false},
				EvidenceFresh: true,
				Reason:        "fresh short window depleted",
				IsBoosted:     false,
			},
		},
	}

	history := []map[string]any{
		{
			"at":        now.Add(-15 * time.Minute),
			"kind":      "auto_apply",
			"trigger":   "auto_apply",
			"attempted": 4,
			"succeeded": 4,
			"failed":    0,
			"skipped":   0,
			"message":   "auto_apply credentials=4 succeeded=4 failed=0 skipped=0",
		},
	}

	return &devRunner{
		snapshot:   snapshot,
		runHistory: history,
	}
}

func (d *devRunner) Run(ctx context.Context, request management.RunRequest) (apply.Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UTC()
	kind := "dry_run"
	if request.Mode == "apply" {
		kind = "apply"
		// Simulate apply updating current priorities
		for i := range d.snapshot.Items {
			d.snapshot.Items[i].Current = d.snapshot.Items[i].Target
		}
	}

	result := apply.Result{
		Attempted: len(d.snapshot.Changes),
		Succeeded: len(d.snapshot.Changes),
		Failed:    0,
		Skipped:   0,
		Snapshot:  d.snapshot,
		Changes: []apply.ChangeResult{
			{
				Name:              "work-pro-gemini@corp.com",
				AuthIndex:         "auth_ag_001",
				Provider:          "antigravity",
				Status:            "success",
				PriorityAttempted: true,
				PriorityFrom:      100,
				PriorityTo:        999,
				Reason:            "fresh boosted",
			},
			{
				Name:              "personal-antigravity-dev",
				AuthIndex:         "auth_ag_002",
				Provider:          "antigravity",
				Status:            "success",
				PriorityAttempted: true,
				PriorityFrom:      100,
				PriorityTo:        899,
				Reason:            "fresh remaining positive",
			},
			{
				Name:              "ci-runner-test-account",
				AuthIndex:         "auth_ag_003",
				Provider:          "antigravity",
				Status:            "success",
				PriorityAttempted: true,
				PriorityFrom:      850,
				PriorityTo:        1,
				Reason:            "fresh short window depleted",
			},
		},
	}

	d.runHistory = append([]map[string]any{
		{
			"at":        now,
			"kind":      kind,
			"trigger":   "manual",
			"attempted": result.Attempted,
			"succeeded": result.Succeeded,
			"failed":    0,
			"skipped":   0,
			"message":   fmt.Sprintf("%s credentials=%d succeeded=%d model_group=%s", kind, result.Attempted, result.Succeeded, request.AntigravityModelGroup),
		},
	}, d.runHistory...)

	return result, nil
}

func (d *devRunner) Status(ctx context.Context) (management.StatusInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return management.StatusInfo{
		TotalCredentials: 4,
		FreshCount:       4,
		LatestAudit:      "scheduling calculation completed (dev mock)",
	}, nil
}

func (d *devRunner) LatestSnapshot(ctx context.Context) (apply.PlanSnapshot, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.snapshot, nil
}

func (d *devRunner) Diagnostics(ctx context.Context) (map[string]any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[string]any{
		"management_api": map[string]any{
			"status":     "ready",
			"auto_apply": true,
			"enabled":    true,
		},
		"scheduler": map[string]any{
			"interval":           "15m0s",
			"last_auto_apply_at": time.Now().Add(-5 * time.Minute),
			"next_wait":          "10m0s",
			"worker_active":      true,
		},
		"latest_audit": "all 4 credentials healthy & double-window monitored",
		"run_history":  d.runHistory,
	}, nil
}

func main() {
	runner := newDevRunner()
	handler := management.NewHandler(runner)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	fmt.Println("=========================================================")
	fmt.Println(" Antigravity Priority - Embedded WebUI Dev Server")
	fmt.Println("=========================================================")
	fmt.Println(" Server listening on: http://localhost:8080/status")
	fmt.Println(" Open the link above in your browser to interact with the UI!")
	fmt.Println(" Press Ctrl+C to stop the server.")
	fmt.Println("=========================================================")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
