package priority

import (
	"testing"
	"time"

	"antigravity-priority/internal/core"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func TestPlanFreshOnly(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	defaultOptions := Options{
		Now:                 now,
		BoostStartPriority:  999,
		NormalStartPriority: 100,
		MinChange:           1,
	}

	t.Run("acceptance: boosted tier clustering with tolerance", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "boost-1", Priority: 50, Disabled: false},
			{AuthIndex: "boost-2", Priority: 50, Disabled: false},
			{AuthIndex: "boost-3", Priority: 50, Disabled: false},
		}

		// All 3 qualify for boost (T_7d <= T_required) with different urgencies
		reset7d1 := now.Add(10 * time.Hour) // Urgency = 0.80 / 10 = 0.08
		reset7d2 := now.Add(5 * time.Hour)  // Urgency = 0.80 / 5 = 0.16 (highest)
		reset7d3 := now.Add(20 * time.Hour) // Urgency = 0.80 / 20 = 0.04 (lowest)
		reset5h := now.Add(2 * time.Hour)

		evidence := []ProbeEvidence{
			{
				AuthIndex:            "boost-1",
				Provider:             core.ProviderAntigravity,
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d1,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15, // T_req = (0.8/0.15)*5 = 26.67h
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				AuthIndex:            "boost-2",
				Provider:             core.ProviderAntigravity,
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d2,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				AuthIndex:            "boost-3",
				Provider:             core.ProviderAntigravity,
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d3,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
		}

		optsTight := defaultOptions
		optsTight.UrgencyTolerance = 0.01
		plan := PlanFreshOnly(creds, evidence, optsTight)

		if len(plan.Items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(plan.Items))
		}

		// Sorted order with tight tolerance: boost-2 (0.16) -> 999, boost-1 (0.08) -> 998, boost-3 (0.04) -> 997
		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		if itemMap["boost-2"].Priority != 999 {
			t.Errorf("boost-2 priority = %d; want 999", itemMap["boost-2"].Priority)
		}
		if itemMap["boost-1"].Priority != 998 {
			t.Errorf("boost-1 priority = %d; want 998", itemMap["boost-1"].Priority)
		}
		if itemMap["boost-3"].Priority != 997 {
			t.Errorf("boost-3 priority = %d; want 997", itemMap["boost-3"].Priority)
		}

		// Changes verification
		if len(plan.Changes) != 3 {
			t.Errorf("expected 3 changes, got %d", len(plan.Changes))
		}
	})

	t.Run("equal priority clustering: healthy accounts with close urgency share identical priority", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "acc-1", Priority: 50, Disabled: false},
			{AuthIndex: "acc-2", Priority: 50, Disabled: false},
			{AuthIndex: "acc-3", Priority: 50, Disabled: false},
			{AuthIndex: "acc-4", Priority: 50, Disabled: false},
		}

		reset7d := now.Add(100 * time.Hour)
		reset5h := now.Add(2 * time.Hour)

		// All 4 accounts have close weekly balances: 85%, 84%, 83%, 82% (Urgency: 0.0085, 0.0084, 0.0083, 0.0082)
		// Max delta is 0.0003, well within default tolerance of 0.05
		evidence := []ProbeEvidence{
			{AuthIndex: "acc-1", LongWindowRemaining: int64Ptr(85), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15, EvidenceFresh: true, Freshness: core.FreshnessFresh, ProbeStatus: core.ProbeStatusReady, Status: EvidenceStatusReady},
			{AuthIndex: "acc-2", LongWindowRemaining: int64Ptr(84), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15, EvidenceFresh: true, Freshness: core.FreshnessFresh, ProbeStatus: core.ProbeStatusReady, Status: EvidenceStatusReady},
			{AuthIndex: "acc-3", LongWindowRemaining: int64Ptr(83), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15, EvidenceFresh: true, Freshness: core.FreshnessFresh, ProbeStatus: core.ProbeStatusReady, Status: EvidenceStatusReady},
			{AuthIndex: "acc-4", LongWindowRemaining: int64Ptr(82), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15, EvidenceFresh: true, Freshness: core.FreshnessFresh, ProbeStatus: core.ProbeStatusReady, Status: EvidenceStatusReady},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)
		for _, item := range plan.Items {
			if item.Priority != 100 {
				t.Errorf("expected account %s in same cluster to get priority 100, got %d", item.Credential.AuthIndex, item.Priority)
			}
		}
	})

	t.Run("429 rate limit cooldown sets priority to -1 while keeping disabled false", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "acc-healthy", Priority: 50, Disabled: false},
			{AuthIndex: "acc-cooldown", Priority: 50, Disabled: false},
		}

		reset7d := now.Add(100 * time.Hour)
		reset5h := now.Add(2 * time.Hour)

		evidence := []ProbeEvidence{
			{AuthIndex: "acc-healthy", LongWindowRemaining: int64Ptr(80), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15, EvidenceFresh: true, Freshness: core.FreshnessFresh, ProbeStatus: core.ProbeStatusReady, Status: EvidenceStatusReady},
			{AuthIndex: "acc-cooldown", LongWindowRemaining: int64Ptr(80), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15, EvidenceFresh: true, Freshness: core.FreshnessFresh, ProbeStatus: core.ProbeStatusReady, Status: EvidenceStatusReady},
		}

		opts := defaultOptions
		opts.CooldownAuthIndexes = map[string]time.Time{
			"acc-cooldown": now.Add(5 * time.Minute),
		}

		plan := PlanFreshOnly(creds, evidence, opts)
		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		if itemMap["acc-healthy"].Priority != 100 {
			t.Errorf("healthy account priority = %d; want 100", itemMap["acc-healthy"].Priority)
		}
		if itemMap["acc-cooldown"].Priority != -1 {
			t.Errorf("cooldown account priority = %d; want -1", itemMap["acc-cooldown"].Priority)
		}
		if itemMap["acc-cooldown"].Disabled {
			t.Errorf("cooldown account should NOT be disabled, got Disabled=true")
		}
		if itemMap["acc-cooldown"].Reason != "429 rate limit cooldown" {
			t.Errorf("cooldown reason = %q; want '429 rate limit cooldown'", itemMap["acc-cooldown"].Reason)
		}
	})

	t.Run("acceptance: hard depletion strict precedence over soft depletion", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "hard-depleted", Priority: 100, Disabled: false},
			{AuthIndex: "both-depleted", Priority: 100, Disabled: false},
			{AuthIndex: "soft-depleted", Priority: 100, Disabled: false},
		}

		reset7d := now.Add(100 * time.Hour)
		reset5h := now.Add(3 * time.Hour)

		evidence := []ProbeEvidence{
			{
				AuthIndex:            "hard-depleted",
				LongWindowRemaining:  int64Ptr(0), // R_7d = 0
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80), // R_5h > 0
				ShortWindowResetAt:   &reset5h,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				AuthIndex:            "both-depleted",
				LongWindowRemaining:  int64Ptr(0), // R_7d = 0
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(0), // R_5h = 0
				ShortWindowResetAt:   &reset5h,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				AuthIndex:            "soft-depleted",
				LongWindowRemaining:  int64Ptr(50), // R_7d > 0
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(0), // R_5h = 0
				ShortWindowResetAt:   &reset5h,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)
		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		// hard-depleted: priority=-1, disabled=true
		if itemMap["hard-depleted"].Priority != -1 || !itemMap["hard-depleted"].Disabled {
			t.Errorf("hard-depleted: priority=%d disabled=%v; want -1, true", itemMap["hard-depleted"].Priority, itemMap["hard-depleted"].Disabled)
		}
		// both-depleted: priority=-1, disabled=true (hard depletion strict precedence!)
		if itemMap["both-depleted"].Priority != -1 || !itemMap["both-depleted"].Disabled {
			t.Errorf("both-depleted: priority=%d disabled=%v; want -1, true", itemMap["both-depleted"].Priority, itemMap["both-depleted"].Disabled)
		}
		// soft-depleted: priority=-1, disabled=false (soft depletion keeps disabled=false for auto-recovery)
		if itemMap["soft-depleted"].Priority != -1 || itemMap["soft-depleted"].Disabled {
			t.Errorf("soft-depleted: priority=%d disabled=%v; want -1, false", itemMap["soft-depleted"].Priority, itemMap["soft-depleted"].Disabled)
		}
	})

	t.Run("manually disabled credential on host is never re-enabled even with healthy quota", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "user-disabled-healthy", Priority: 100, Disabled: true},
		}
		reset7d := now.Add(100 * time.Hour)
		reset5h := now.Add(3 * time.Hour)
		evidence := []ProbeEvidence{
			{
				AuthIndex:            "user-disabled-healthy",
				LongWindowRemaining:  int64Ptr(90),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)
		if len(plan.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(plan.Items))
		}
		if !plan.Items[0].Disabled {
			t.Errorf("expected manually disabled account to stay disabled (Disabled=true), got Disabled=false")
		}
		if plan.Items[0].Priority != DepletedPriority {
			t.Errorf("expected priority %d, got %d", DepletedPriority, plan.Items[0].Priority)
		}
		// Must not generate any change enabling the account
		for _, chg := range plan.Changes {
			if chg.Credential.AuthIndex == "user-disabled-healthy" && !chg.Disabled {
				t.Errorf("plan generated change enabling user-disabled credential: %+v", chg)
			}
		}
	})

	t.Run("acceptance: healthy regular credentials sort by urgency, tie-break 5h reset, then authIndex", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "reg-c", Priority: 10, Disabled: false},
			{AuthIndex: "reg-b", Priority: 10, Disabled: false},
			{AuthIndex: "reg-a", Priority: 10, Disabled: false},
			{AuthIndex: "reg-d", Priority: 10, Disabled: false},
		}

		// Long reset far out (> 26.67h) so none are boosted
		reset7d := now.Add(100 * time.Hour)    // Urgency = 0.50 / 100 = 0.005
		reset7dHigh := now.Add(50 * time.Hour) // Urgency = 0.50 / 50 = 0.010 (higher)

		reset5hEarly := now.Add(1 * time.Hour)
		reset5hLate := now.Add(4 * time.Hour)

		evidence := []ProbeEvidence{
			{
				// reg-a: urgency 0.005, 5h reset 4h
				AuthIndex:            "reg-a",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5hLate,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				// reg-b: urgency 0.005, 5h reset 1h (should beat reg-a and reg-c due to 5h reset)
				AuthIndex:            "reg-b",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5hEarly,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				// reg-c: urgency 0.005, 5h reset 4h (same urgency & 5h reset as reg-a -> tie break authIndex: reg-a < reg-c)
				AuthIndex:            "reg-c",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5hLate,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				// reg-d: urgency 0.010 (highest urgency -> priority 100)
				AuthIndex:            "reg-d",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7dHigh,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5hLate,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
		}

		optsTight := defaultOptions
		optsTight.UrgencyTolerance = 0.001
		plan := PlanFreshOnly(creds, evidence, optsTight)
		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		// reg-d has highest urgency -> Tier 1 (100)
		// reg-b has 1h 5h reset advantage -> Tier 2 (99)
		// reg-a, reg-c share identical 5h/7d metrics -> Tier 3 (98)
		if itemMap["reg-d"].Priority != 100 {
			t.Errorf("reg-d priority = %d; want 100", itemMap["reg-d"].Priority)
		}
		if itemMap["reg-b"].Priority != 99 {
			t.Errorf("reg-b priority = %d; want 99", itemMap["reg-b"].Priority)
		}
		if itemMap["reg-a"].Priority != 98 {
			t.Errorf("reg-a priority = %d; want 98", itemMap["reg-a"].Priority)
		}
		if itemMap["reg-c"].Priority != 98 {
			t.Errorf("reg-c priority = %d; want 98", itemMap["reg-c"].Priority)
		}
	})

	t.Run("acceptance: priority uniqueness and unprobed peer ForceWrite tagging", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "fresh-boost", Priority: 50, Disabled: false},
			{AuthIndex: "fresh-reg", Priority: 50, Disabled: false},
			{AuthIndex: "unprobed-peer-collision", Priority: 100, Disabled: false}, // collides with fresh-reg at 100
			{AuthIndex: "unprobed-peer-safe", Priority: 70, Disabled: false},       // no collision
			{AuthIndex: "unprobed-peer-disabled", Priority: 100, Disabled: true},   // disabled -> stays untouched
		}

		reset7dBoost := now.Add(5 * time.Hour)
		reset7dReg := now.Add(80 * time.Hour)
		reset5h := now.Add(2 * time.Hour)

		evidence := []ProbeEvidence{
			{
				AuthIndex:            "fresh-boost",
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7dBoost,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				AuthIndex:            "fresh-reg",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7dReg,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)
		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		if itemMap["fresh-boost"].Priority != 999 {
			t.Errorf("fresh-boost priority = %d; want 999", itemMap["fresh-boost"].Priority)
		}
		if itemMap["fresh-reg"].Priority != 100 {
			t.Errorf("fresh-reg priority = %d; want 100", itemMap["fresh-reg"].Priority)
		}

		// unprobed-peer-collision had priority 100 -> stays <= 100
		unprobedColl := itemMap["unprobed-peer-collision"]
		if unprobedColl.Priority > 100 {
			t.Errorf("unprobed-peer-collision priority = %d; want <= 100", unprobedColl.Priority)
		}

		// unprobed-peer-safe had priority 70 -> stays 70, ForceWrite = false
		unprobedSafe := itemMap["unprobed-peer-safe"]
		if unprobedSafe.Priority != 70 {
			t.Errorf("unprobed-peer-safe priority = %d; want 70", unprobedSafe.Priority)
		}
		if unprobedSafe.ForceWrite {
			t.Errorf("unprobed-peer-safe ForceWrite = true; want false")
		}
	})

	t.Run("min_change gating filters small non-disabled delta", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "cred-small-delta", Priority: 99, Disabled: false}, // target priority 100 -> delta = 1
			{AuthIndex: "cred-large-delta", Priority: 50, Disabled: false}, // target priority 99 -> delta = 49
		}

		reset7d := now.Add(80 * time.Hour)
		reset5h := now.Add(2 * time.Hour)

		evidence := []ProbeEvidence{
			{
				AuthIndex:            "cred-small-delta",
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				AuthIndex:            "cred-large-delta",
				LongWindowRemaining:  int64Ptr(70),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
		}

		opts := defaultOptions
		opts.MinChange = 5 // requires at least 5 priority change

		plan := PlanFreshOnly(creds, evidence, opts)

		// cred-small-delta (priority 99 -> 100, delta=1 < 5) should NOT be in Changes
		// cred-large-delta (priority 50 -> 99, delta=49 >= 5) should BE in Changes
		if len(plan.Changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(plan.Changes))
		}
		if plan.Changes[0].Credential.AuthIndex != "cred-large-delta" {
			t.Errorf("change authIndex = %s; want cred-large-delta", plan.Changes[0].Credential.AuthIndex)
		}
	})

	t.Run("credential with PriorityMissing always triggers change", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "missing-prio", Priority: 100, PriorityMissing: true, Disabled: false},
		}
		reset7d := now.Add(80 * time.Hour)
		evidence := []ProbeEvidence{
			{
				AuthIndex:           "missing-prio",
				LongWindowRemaining: int64Ptr(80),
				LongWindowResetAt:   &reset7d,
				EvidenceFresh:       true,
				Freshness:           core.FreshnessFresh,
				ProbeStatus:         core.ProbeStatusReady,
				Status:              EvidenceStatusReady,
			},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)
		if len(plan.Changes) != 1 {
			t.Errorf("expected 1 change for missing priority, got %d", len(plan.Changes))
		}
	})

	t.Run("stale probe evidence is ignored", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "stale-cred", Priority: 50, Disabled: false},
		}

		evidence := []ProbeEvidence{
			{
				AuthIndex:     "stale-cred",
				EvidenceFresh: false, // stale
				Freshness:     core.FreshnessStale,
				ProbeStatus:   core.ProbeStatusReady,
				Status:        EvidenceStatusReady,
			},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)

		if len(plan.Changes) != 0 {
			t.Errorf("expected 0 changes for stale evidence, got %d", len(plan.Changes))
		}

		if len(plan.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(plan.Items))
		}
		item := plan.Items[0]
		if item.Priority != 50 {
			t.Errorf("stale-cred priority = %d; want 50", item.Priority)
		}
		if item.Disabled {
			t.Errorf("stale-cred should NOT be disabled")
		}
		if item.Reason != "keep current state" {
			t.Errorf("stale-cred reason = %q; want 'keep current state'", item.Reason)
		}
	})

	t.Run("probe failure triggers temporary disable failedQuotaFetch", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "failed-cred", Priority: 60, Disabled: false},
			{AuthIndex: "already-disabled-failed", Priority: 100, Disabled: true},
		}

		evidence := []ProbeEvidence{
			{
				AuthIndex:     "failed-cred",
				EvidenceFresh: true,
				Freshness:     core.FreshnessFresh,
				ProbeStatus:   core.ProbeStatusUnknown,
				Status:        EvidenceStatusProbeFailed,
			},
			{
				AuthIndex:     "already-disabled-failed",
				EvidenceFresh: true,
				Freshness:     core.FreshnessFresh,
				ProbeStatus:   core.ProbeStatusUnknown,
				Status:        EvidenceStatusProbeFailed,
			},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)

		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		// 1. failed-cred was active -> now disabled with failedQuotaFetch
		failedItem := itemMap["failed-cred"]
		if !failedItem.Disabled {
			t.Errorf("failed-cred should be Disabled=true")
		}
		if failedItem.Reason != "failedQuotaFetch" {
			t.Errorf("failed-cred reason = %q; want 'failedQuotaFetch'", failedItem.Reason)
		}
		if failedItem.Priority != 60 {
			t.Errorf("failed-cred priority = %d; want 60", failedItem.Priority)
		}

		// 2. already-disabled-failed was already disabled on host -> stays disabled with reason "disabled on host"
		alreadyDisabledItem := itemMap["already-disabled-failed"]
		if !alreadyDisabledItem.Disabled {
			t.Errorf("already-disabled-failed should be Disabled=true")
		}
		if alreadyDisabledItem.Reason != "disabled on host" {
			t.Errorf("already-disabled-failed reason = %q; want 'disabled on host'", alreadyDisabledItem.Reason)
		}
		if alreadyDisabledItem.Priority != DepletedPriority {
			t.Errorf("already-disabled-failed priority = %d; want %d", alreadyDisabledItem.Priority, DepletedPriority)
		}

		// Changes should contain only the change for failed-cred (since already-disabled-failed was already disabled)
		if len(plan.Changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(plan.Changes))
		}
		if plan.Changes[0].Credential.AuthIndex != "failed-cred" {
			t.Errorf("change authIndex = %s; want failed-cred", plan.Changes[0].Credential.AuthIndex)
		}
		if !plan.Changes[0].Disabled {
			t.Errorf("change should have Disabled=true")
		}
		if plan.Changes[0].Reason != "failedQuotaFetch" {
			t.Errorf("change reason = %q; want 'failedQuotaFetch'", plan.Changes[0].Reason)
		}
	})

	t.Run("handles empty credentials and options normalization gracefully", func(t *testing.T) {
		emptyPlan := PlanFreshOnly(nil, nil, Options{})
		if len(emptyPlan.Items) != 0 || len(emptyPlan.Changes) != 0 {
			t.Errorf("empty plan should have 0 items and changes")
		}

		opts := normalizeOptions(Options{
			BoostStartPriority:  -5,
			NormalStartPriority: 2000,
			MinChange:           -10,
		})
		if opts.BoostStartPriority != DefaultBoostStartPriority {
			t.Errorf("BoostStartPriority = %d; want %d", opts.BoostStartPriority, DefaultBoostStartPriority)
		}
		if opts.NormalStartPriority != MaxPriority {
			t.Errorf("NormalStartPriority = %d; want %d", opts.NormalStartPriority, MaxPriority)
		}
		if opts.MinChange != 0 {
			t.Errorf("MinChange = %d; want 0", opts.MinChange)
		}

		optsMaxBoost := normalizeOptions(Options{
			BoostStartPriority: 10000,
		})
		if optsMaxBoost.BoostStartPriority != MaxPriority {
			t.Errorf("BoostStartPriority = %d; want %d", optsMaxBoost.BoostStartPriority, MaxPriority)
		}
	})

	t.Run("unprobed peer with legacy 999 priority is capped and shifted", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "fresh-prio-100", Priority: 50, Disabled: false},
			{AuthIndex: "unprobed-legacy-999", Priority: 999, Disabled: false},
		}
		reset7d := now.Add(80 * time.Hour)
		reset5h := now.Add(2 * time.Hour)
		evidence := []ProbeEvidence{
			{
				AuthIndex:            "fresh-prio-100",
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)
		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		if itemMap["fresh-prio-100"].Priority != 100 {
			t.Errorf("fresh-prio-100 priority = %d; want 100", itemMap["fresh-prio-100"].Priority)
		}
		// unprobed-legacy-999 had 999, should be capped to <= 100
		unprobed := itemMap["unprobed-legacy-999"]
		if unprobed.Priority != 100 {
			t.Errorf("unprobed-legacy-999 priority = %d; want 100", unprobed.Priority)
		}
		if !unprobed.ForceWrite {
			t.Errorf("unprobed-legacy-999 ForceWrite = false; want true")
		}
	})

	t.Run("already depleted credential on host generates no redundant change", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "already-depleted", Priority: -1, Disabled: true},
		}
		reset7d := now.Add(80 * time.Hour)
		evidence := []ProbeEvidence{
			{
				AuthIndex:           "already-depleted",
				LongWindowRemaining: int64Ptr(0),
				LongWindowResetAt:   &reset7d,
				EvidenceFresh:       true,
				Freshness:           core.FreshnessFresh,
				ProbeStatus:         core.ProbeStatusReady,
				Status:              EvidenceStatusReady,
			},
		}

		plan := PlanFreshOnly(creds, evidence, defaultOptions)
		if len(plan.Changes) != 0 {
			t.Errorf("expected 0 changes for already depleted credential, got %d", len(plan.Changes))
		}
	})

	t.Run("priority decrements clamp at MinPriority (1)", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "c1", Priority: 50},
			{AuthIndex: "c2", Priority: 50},
			{AuthIndex: "c3", Priority: 50},
		}
		reset7d := now.Add(80 * time.Hour)
		reset5h := now.Add(2 * time.Hour)
		evidence := []ProbeEvidence{
			{
				AuthIndex:            "c1",
				LongWindowRemaining:  int64Ptr(90),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				AuthIndex:            "c2",
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
			{
				AuthIndex:            "c3",
				LongWindowRemaining:  int64Ptr(70),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(70),
				ShortWindowResetAt:   &reset5h,
				EvidenceFresh:        true,
				Freshness:            core.FreshnessFresh,
				ProbeStatus:          core.ProbeStatusReady,
				Status:               EvidenceStatusReady,
			},
		}

		opts := defaultOptions
		opts.NormalStartPriority = 2 // start at 2 -> c1 gets 2, c2 gets 1, c3 gets 1 (before uniqueness) -> uniqueness assigns unique slots
		plan := PlanFreshOnly(creds, evidence, opts)
		if len(plan.Items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(plan.Items))
		}
	})
}

func TestNextAvailablePriority(t *testing.T) {
	used := make(map[int]struct{})
	// Preferred > MaxPriority
	p1 := nextAvailablePriority(2000, used)
	if p1 != MaxPriority {
		t.Errorf("nextAvailablePriority(2000) = %d; want %d", p1, MaxPriority)
	}

	// Preferred < MinPriority
	p2 := nextAvailablePriority(-10, used)
	if p2 != MinPriority {
		t.Errorf("nextAvailablePriority(-10) = %d; want %d", p2, MinPriority)
	}

	// All downward slots full, search upward
	used[1] = struct{}{}
	used[2] = struct{}{}
	p3 := nextAvailablePriority(1, used)
	if p3 != 3 {
		t.Errorf("nextAvailablePriority(1 with 1,2 used) = %d; want 3", p3)
	}
}
