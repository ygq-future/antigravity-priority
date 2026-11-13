package priority

import (
	"reflect"
	"testing"
	"time"

	"antigravity-priority/internal/core"
)

func int64Ptr(v int64) *int64 {
	return new(v)
}

func planFreshOnly(credentials []core.Credential, evidence []QuotaEvidence, options Options) Plan {
	if options.Now.IsZero() {
		panic("priority: explicit decision time is required")
	}
	options.Now = options.Now.UTC()
	return planWithEvidence(credentials, plannerEvidence{Fresh: evidence}, options)
}

func TestPlanFreshOnlyRequiresExplicitDecisionTime(t *testing.T) {
	t.Run("zero decision time is rejected", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected zero decision time to be rejected")
			}
		}()
		planFreshOnly(nil, nil, Options{})
	})

	t.Run("identical inputs produce identical replayable plans", func(t *testing.T) {
		now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
		options := Options{Now: now}
		first := planFreshOnly(nil, nil, options)
		second := planFreshOnly(nil, nil, options)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("plans differ: first=%#v second=%#v", first, second)
		}
		if !first.DecidedAt.Equal(now) {
			t.Fatalf("DecidedAt = %v; want %v", first.DecidedAt, now)
		}
	})
}

func TestPlanFreshOnlyZeroToleranceClustersOnlyExactlyEqualScores(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(100 * time.Hour)
	credentials := []core.Credential{{AuthIndex: "equal-a"}, {AuthIndex: "equal-b"}, {AuthIndex: "different"}}
	evidence := []QuotaEvidence{
		{AuthIndex: "equal-a", ShortWindowRemaining: int64Ptr(80), ShortWindowResetAt: &reset5h, LongWindowRemaining: int64Ptr(80), LongWindowResetAt: &reset7d},
		{AuthIndex: "equal-b", ShortWindowRemaining: int64Ptr(80), ShortWindowResetAt: &reset5h, LongWindowRemaining: int64Ptr(80), LongWindowResetAt: &reset7d},
		{AuthIndex: "different", ShortWindowRemaining: int64Ptr(80), ShortWindowResetAt: &reset5h, LongWindowRemaining: int64Ptr(79), LongWindowResetAt: &reset7d},
	}
	plan := planFreshOnly(credentials, evidence, Options{Now: now, UrgencyTolerance: 0})
	priorities := map[string]int{}
	for _, item := range plan.Items {
		priorities[item.Credential.AuthIndex] = item.Priority
	}
	if priorities["equal-a"] != priorities["equal-b"] {
		t.Fatalf("equal scores did not share tier: %#v", priorities)
	}
	if priorities["different"] == priorities["equal-a"] {
		t.Fatalf("positive score difference shared zero-tolerance tier: %#v", priorities)
	}
}

func TestPlanFreshOnly(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	defaultOptions := Options{
		Now:                 now,
		UrgencyTolerance:    0.05,
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

		evidence := []QuotaEvidence{
			{
				AuthIndex:            "boost-1",
				Provider:             core.ProviderAntigravity,
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d1,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15, // T_req = (0.8/0.15)*5 = 26.67h
			},
			{
				AuthIndex:            "boost-2",
				Provider:             core.ProviderAntigravity,
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d2,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
			},
			{
				AuthIndex:            "boost-3",
				Provider:             core.ProviderAntigravity,
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d3,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
			},
		}

		optsTight := defaultOptions
		optsTight.UrgencyTolerance = 0.01
		plan := planFreshOnly(creds, evidence, optsTight)

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
		evidence := []QuotaEvidence{
			{AuthIndex: "acc-1", LongWindowRemaining: int64Ptr(85), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15},
			{AuthIndex: "acc-2", LongWindowRemaining: int64Ptr(84), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15},
			{AuthIndex: "acc-3", LongWindowRemaining: int64Ptr(83), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15},
			{AuthIndex: "acc-4", LongWindowRemaining: int64Ptr(82), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15},
		}

		plan := planFreshOnly(creds, evidence, defaultOptions)
		for _, item := range plan.Items {
			if item.Priority != 100 {
				t.Errorf("expected account %s in same cluster to get priority 100, got %d", item.Credential.AuthIndex, item.Priority)
			}
		}
	})

	t.Run("pool cooldown sets priority to -1 while keeping disabled false", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "acc-healthy", Priority: 50, Disabled: false},
			{AuthIndex: "acc-cooldown", Priority: 50, Disabled: false},
		}

		reset7d := now.Add(100 * time.Hour)
		reset5h := now.Add(2 * time.Hour)

		evidence := []QuotaEvidence{
			{AuthIndex: "acc-healthy", LongWindowRemaining: int64Ptr(80), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15},
			{AuthIndex: "acc-cooldown", LongWindowRemaining: int64Ptr(80), LongWindowResetAt: &reset7d, ShortWindowRemaining: int64Ptr(90), ShortWindowResetAt: &reset5h, CycleBurnRate: 0.15},
		}

		opts := defaultOptions
		cooldownUntil := now.Add(5 * time.Minute)
		opts.CooldownAuthIndexes = map[string]time.Time{
			"acc-cooldown": cooldownUntil,
		}
		opts.CooldownReasons = map[string]string{
			"acc-cooldown": "health probe HTTP 404",
		}

		plan := planFreshOnly(creds, evidence, opts)
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
		if itemMap["acc-cooldown"].Reason != "health probe HTTP 404" {
			t.Errorf("cooldown reason = %q; want 'health probe HTTP 404'", itemMap["acc-cooldown"].Reason)
		}
		if itemMap["acc-cooldown"].CooldownUntil == nil || !itemMap["acc-cooldown"].CooldownUntil.Equal(cooldownUntil) {
			t.Errorf("cooldown deadline = %v; want %v", itemMap["acc-cooldown"].CooldownUntil, cooldownUntil)
		}
		if itemMap["acc-healthy"].CooldownUntil != nil {
			t.Errorf("healthy account should not carry a cooldown deadline, got %v", itemMap["acc-healthy"].CooldownUntil)
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

		evidence := []QuotaEvidence{
			{
				AuthIndex:            "hard-depleted",
				LongWindowRemaining:  int64Ptr(0), // R_7d = 0
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80), // R_5h > 0
				ShortWindowResetAt:   &reset5h,
			},
			{
				AuthIndex:            "both-depleted",
				LongWindowRemaining:  int64Ptr(0), // R_7d = 0
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(0), // R_5h = 0
				ShortWindowResetAt:   &reset5h,
			},
			{
				AuthIndex:            "soft-depleted",
				LongWindowRemaining:  int64Ptr(50), // R_7d > 0
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(0), // R_5h = 0
				ShortWindowResetAt:   &reset5h,
			},
		}

		plan := planFreshOnly(creds, evidence, defaultOptions)
		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		// hard-depleted: priority=-1, disabled=false (soft depletion never mutates host disabled)
		if itemMap["hard-depleted"].Priority != -1 || itemMap["hard-depleted"].Disabled || itemMap["hard-depleted"].Reason != ReasonFreshWeeklyDepleted {
			t.Errorf("hard-depleted: priority=%d disabled=%v reason=%s; want -1, false, %s", itemMap["hard-depleted"].Priority, itemMap["hard-depleted"].Disabled, itemMap["hard-depleted"].Reason, ReasonFreshWeeklyDepleted)
		}
		// both-depleted: priority=-1, disabled=false (weekly depletion takes strict precedence over short depletion)
		if itemMap["both-depleted"].Priority != -1 || itemMap["both-depleted"].Disabled || itemMap["both-depleted"].Reason != ReasonFreshWeeklyDepleted {
			t.Errorf("both-depleted: priority=%d disabled=%v reason=%s; want -1, false, %s", itemMap["both-depleted"].Priority, itemMap["both-depleted"].Disabled, itemMap["both-depleted"].Reason, ReasonFreshWeeklyDepleted)
		}
		// soft-depleted: priority=-1, disabled=false (short depletion keeps disabled=false for auto-recovery)
		if itemMap["soft-depleted"].Priority != -1 || itemMap["soft-depleted"].Disabled || itemMap["soft-depleted"].Reason != ReasonFreshShortWindowDepleted {
			t.Errorf("soft-depleted: priority=%d disabled=%v reason=%s; want -1, false, %s", itemMap["soft-depleted"].Priority, itemMap["soft-depleted"].Disabled, itemMap["soft-depleted"].Reason, ReasonFreshShortWindowDepleted)
		}
	})

	t.Run("manually disabled credential keeps host state and retains quota meters without host write", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "user-disabled-healthy", Priority: 100, Disabled: true},
		}
		reset7d := now.Add(100 * time.Hour)
		reset5h := now.Add(3 * time.Hour)
		evidence := []QuotaEvidence{
			{
				AuthIndex:            "user-disabled-healthy",
				LongWindowRemaining:  int64Ptr(90),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(85),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
			},
		}

		plan := planFreshOnly(creds, evidence, defaultOptions)
		if len(plan.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(plan.Items))
		}
		item := plan.Items[0]
		if !item.Disabled || item.Priority != 100 {
			t.Errorf("expected manually disabled account to keep host state, got %+v", item)
		}
		if item.Reason != ReasonDisabledOnHost {
			t.Errorf("expected reason %q, got %q", ReasonDisabledOnHost, item.Reason)
		}
		if item.R7d != 0.90 || item.R5h != 0.85 {
			t.Errorf("expected quota metrics preserved (R7d=0.9, R5h=0.85), got R7d=%v, R5h=%v", item.R7d, item.R5h)
		}
		if item.ShortWindowResetAt == nil || !item.ShortWindowResetAt.Equal(reset5h) {
			t.Errorf("expected reset timestamp preserved, got %+v", item.ShortWindowResetAt)
		}
	})

	t.Run("weekly depleted credential receives soft downgrade priority = -1 and disabled = false", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "weekly-below-threshold", Priority: 100, Disabled: false},
			{AuthIndex: "weekly-healthy", Priority: -1, Disabled: false},
			{AuthIndex: "weekly-short-depleted", Priority: 100, Disabled: false},
		}
		reset7d := now.Add(100 * time.Hour)
		reset5h := now.Add(3 * time.Hour)
		evidence := []QuotaEvidence{
			{
				// 0% 7d quota: weekly depleted, priority=-1, disabled=false
				AuthIndex:           "weekly-below-threshold",
				LongWindowRemaining: int64Ptr(0),
				LongWindowResetAt:   &reset7d,
				ShortWindowResetAt:  &reset5h,
				CycleBurnRate:       0.15,
			},
			{
				// 15% 7d quota (>= 10% threshold) & 80% 5h: healthy, priority=100, disabled=false
				AuthIndex:            "weekly-healthy",
				LongWindowRemaining:  int64Ptr(15),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
			},
			{
				// 15% 7d quota (>= 10% threshold) but 0% 5h: short-depleted, priority=-1, disabled=false
				AuthIndex:            "weekly-short-depleted",
				LongWindowRemaining:  int64Ptr(15),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(0),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
			},
		}

		plan := planFreshOnly(creds, evidence, defaultOptions)
		if len(plan.Items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(plan.Items))
		}

		itemMap := make(map[string]PlanItem, 3)
		for _, it := range plan.Items {
			itemMap[it.Credential.AuthIndex] = it
		}

		// 1. Below threshold: stay depleted with disabled=false (never mutate host disabled)
		below := itemMap["weekly-below-threshold"]
		if below.Disabled || below.Priority != -1 || below.Reason != ReasonFreshWeeklyDepleted {
			t.Errorf("below threshold should be weekly depleted: %+v", below)
		}

		// 2. Healthy recovery: positive priority, disabled=false
		healthy := itemMap["weekly-healthy"]
		if healthy.Disabled || healthy.Priority != 100 || healthy.Reason != ReasonFreshRemainingPositive {
			t.Errorf("healthy recovery failed: %+v", healthy)
		}

		// 3. Short depleted: priority=-1, disabled=false
		shortDep := itemMap["weekly-short-depleted"]
		if shortDep.Disabled || shortDep.Priority != -1 || shortDep.Reason != ReasonFreshShortWindowDepleted {
			t.Errorf("short depleted failed: %+v", shortDep)
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

		evidence := []QuotaEvidence{
			{
				// reg-a: urgency 0.005, 5h reset 4h
				AuthIndex:            "reg-a",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5hLate,
				CycleBurnRate:        0.15,
			},
			{
				// reg-b: urgency 0.005, 5h reset 1h (should beat reg-a and reg-c due to 5h reset)
				AuthIndex:            "reg-b",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5hEarly,
				CycleBurnRate:        0.15,
			},
			{
				// reg-c: urgency 0.005, 5h reset 4h (same urgency & 5h reset as reg-a -> tie break authIndex: reg-a < reg-c)
				AuthIndex:            "reg-c",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5hLate,
				CycleBurnRate:        0.15,
			},
			{
				// reg-d: urgency 0.010 (highest urgency -> priority 100)
				AuthIndex:            "reg-d",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7dHigh,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5hLate,
				CycleBurnRate:        0.15,
			},
		}

		optsTight := defaultOptions
		optsTight.UrgencyTolerance = 0.001
		plan := planFreshOnly(creds, evidence, optsTight)
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

	t.Run("acceptance: priority uniqueness and unprobed peer stability", func(t *testing.T) {
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

		evidence := []QuotaEvidence{
			{
				AuthIndex:            "fresh-boost",
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7dBoost,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
			},
			{
				AuthIndex:            "fresh-reg",
				LongWindowRemaining:  int64Ptr(50),
				LongWindowResetAt:    &reset7dReg,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
				CycleBurnRate:        0.15,
			},
		}

		plan := planFreshOnly(creds, evidence, defaultOptions)
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

		// unprobed-peer-safe had priority 70 -> stays 70
		unprobedSafe := itemMap["unprobed-peer-safe"]
		if unprobedSafe.Priority != 70 {
			t.Errorf("unprobed-peer-safe priority = %d; want 70", unprobedSafe.Priority)
		}
	})

	t.Run("persisted historical evidence plans from stored quota", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "stored-cred", Priority: 50, Disabled: false},
		}
		reset7d := now.Add(100 * time.Hour)
		reset5h := now.Add(3 * time.Hour)
		evidence := []QuotaEvidence{
			{
				AuthIndex:            "stored-cred",
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
			},
		}

		plan := PlanWithEvidence(creds, plannerEvidence{Historical: evidence}, defaultOptions)

		if len(plan.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(plan.Items))
		}
		item := plan.Items[0]
		if item.Disabled || item.Priority != 100 {
			t.Errorf("historical evidence should plan a healthy target: %#v", item)
		}
		if item.Reason != ReasonFreshRemainingPositive {
			t.Errorf("historical evidence reason = %q; want %q", item.Reason, ReasonFreshRemainingPositive)
		}
	})

	t.Run("missing evidence preserves current Host state", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "failed-cred", Priority: 60, Disabled: false},
			{AuthIndex: "already-disabled-failed", Priority: 100, Disabled: true},
		}

		plan := planFreshOnly(creds, nil, defaultOptions)

		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		// 1. failed-cred remains active and keeps its current priority
		failedItem := itemMap["failed-cred"]
		if failedItem.Disabled || failedItem.Priority != 60 {
			t.Errorf("failed-cred changed without evidence: %#v", failedItem)
		}

		// 2. already-disabled-failed remains exactly as observed on Host
		alreadyDisabledItem := itemMap["already-disabled-failed"]
		if !alreadyDisabledItem.Disabled || alreadyDisabledItem.Priority != 100 {
			t.Errorf("already-disabled-failed changed without evidence: %#v", alreadyDisabledItem)
		}
	})

	t.Run("handles empty credentials gracefully", func(t *testing.T) {
		emptyPlan := planFreshOnly(nil, nil, Options{Now: now})
		if len(emptyPlan.Items) != 0 {
			t.Errorf("empty plan should have 0 items")
		}
	})

	t.Run("unprobed peer keeps legacy priority without write authority", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "fresh-prio-100", Priority: 50, Disabled: false},
			{AuthIndex: "unprobed-legacy-999", Priority: 999, Disabled: false},
		}
		reset7d := now.Add(80 * time.Hour)
		reset5h := now.Add(2 * time.Hour)
		evidence := []QuotaEvidence{
			{
				AuthIndex:            "fresh-prio-100",
				LongWindowRemaining:  int64Ptr(80),
				LongWindowResetAt:    &reset7d,
				ShortWindowRemaining: int64Ptr(80),
				ShortWindowResetAt:   &reset5h,
			},
		}

		plan := planFreshOnly(creds, evidence, defaultOptions)
		itemMap := make(map[string]PlanItem)
		for _, item := range plan.Items {
			itemMap[item.Credential.AuthIndex] = item
		}

		if itemMap["fresh-prio-100"].Priority != 100 {
			t.Errorf("fresh-prio-100 priority = %d; want 100", itemMap["fresh-prio-100"].Priority)
		}
		// An unprobed credential must not be rewritten just to make room for a peer.
		unprobed := itemMap["unprobed-legacy-999"]
		if unprobed.Priority != 999 {
			t.Errorf("unprobed-legacy-999 priority = %d; want 999", unprobed.Priority)
		}
	})

	t.Run("already depleted credential on host generates no redundant change", func(t *testing.T) {
		creds := []core.Credential{
			{AuthIndex: "already-depleted", Priority: -1, Disabled: true},
		}
		reset7d := now.Add(80 * time.Hour)
		evidence := []QuotaEvidence{
			{
				AuthIndex:           "already-depleted",
				LongWindowRemaining: int64Ptr(0),
				LongWindowResetAt:   &reset7d,
			},
		}

		plan := planFreshOnly(creds, evidence, defaultOptions)
		if len(plan.Items) != 1 || plan.Items[0].Priority != DepletedPriority {
			t.Errorf("already depleted plan = %#v; want priority -1", plan.Items)
		}
	})

	t.Run("priority decrements clamp at MinPriority (1)", func(t *testing.T) {
		const count = DefaultNormalStartPriority + 2
		creds := make([]core.Credential, count)
		evidence := make([]QuotaEvidence, count)
		reset5h := now.Add(4 * time.Hour)
		reset7d := now.Add(1000 * time.Hour)
		for i := range count {
			authIndex := "c" + string(rune('a'+i/26)) + string(rune('a'+i%26))
			creds[i] = core.Credential{AuthIndex: authIndex, Priority: 50}
			rem := int64(100 - i%99)
			longReset := reset7d.Add(time.Duration(i) * time.Hour)
			evidence[i] = QuotaEvidence{
				AuthIndex:            authIndex,
				LongWindowRemaining:  &rem,
				LongWindowResetAt:    &longReset,
				ShortWindowRemaining: int64Ptr(90),
				ShortWindowResetAt:   &reset5h,
			}
		}

		opts := defaultOptions
		opts.UrgencyTolerance = 0
		plan := planFreshOnly(creds, evidence, opts)
		if len(plan.Items) != count {
			t.Fatalf("expected %d items, got %d", count, len(plan.Items))
		}
		for _, item := range plan.Items {
			if item.Priority < MinPriority {
				t.Fatalf("priority %d dropped below MinPriority (%d)", item.Priority, MinPriority)
			}
		}
	})
}

func TestPlanWithEvidence_FreshEvidenceTakesPrecedenceOverHistorical(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(100 * time.Hour)
	credentials := []core.Credential{
		{AuthIndex: "fresh", Priority: 50},
		{AuthIndex: "historical", Priority: 50},
		{AuthIndex: "both", Priority: 50},
		{AuthIndex: "failed", Priority: 70},
	}
	healthy := QuotaEvidence{
		LongWindowRemaining:  int64Ptr(80),
		LongWindowResetAt:    &reset7d,
		ShortWindowRemaining: int64Ptr(80),
		ShortWindowResetAt:   &reset5h,
	}
	fresh := healthy
	fresh.AuthIndex = "fresh"
	historical := healthy
	historical.AuthIndex = "historical"
	depleted := QuotaEvidence{
		AuthIndex:            "both",
		LongWindowRemaining:  int64Ptr(80),
		LongWindowResetAt:    &reset7d,
		ShortWindowRemaining: int64Ptr(0),
		ShortWindowResetAt:   &reset5h,
	}
	preferred := healthy
	preferred.AuthIndex = "both"

	plan := PlanWithEvidence(credentials, plannerEvidence{
		Fresh:      []QuotaEvidence{fresh, preferred},
		Historical: []QuotaEvidence{historical, depleted},
	}, Options{Now: now})

	items := make(map[string]PlanItem, len(plan.Items))
	for _, item := range plan.Items {
		items[item.Credential.AuthIndex] = item
	}
	if items["fresh"].Priority != 100 {
		t.Fatalf("fresh item = %#v; want scheduled target 100", items["fresh"])
	}
	if items["historical"].Priority != 100 {
		t.Fatalf("historical item = %#v; want stored-quota target 100", items["historical"])
	}
	if items["both"].Priority != 100 {
		t.Fatalf("precedence item = %#v; fresh evidence must override depleted historical evidence", items["both"])
	}
	if items["failed"].Priority != 70 || items["failed"].Disabled {
		t.Fatalf("failed item changed without evidence: %#v", items["failed"])
	}
}

func TestPlanFreshOnly_DepletedPriorityUsesExplicitRuntimeState(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(100 * time.Hour)
	healthy := QuotaEvidence{
		AuthIndex:            "recovered",
		ShortWindowRemaining: int64Ptr(100),
		ShortWindowResetAt:   &reset5h,
		LongWindowRemaining:  int64Ptr(15),
		LongWindowResetAt:    &reset7d,
		CycleBurnRate:        0.15,
	}
	options := Options{
		Now:                 now,
	}

	t.Run("healthy credential recovers from current minus one", func(t *testing.T) {
		plan := planFreshOnly([]core.Credential{{AuthIndex: "recovered", Priority: -1}}, []QuotaEvidence{healthy}, options)
		if len(plan.Items) != 1 || plan.Items[0].Priority != 100 || plan.Items[0].Disabled {
			t.Fatalf("recovered item = %#v; want priority=100 disabled=false", plan.Items)
		}
	})

	t.Run("active 429 cooldown keeps current minus one", func(t *testing.T) {
		cooldownOptions := options
		cooldownOptions.CooldownAuthIndexes = map[string]time.Time{"recovered": now.Add(5 * time.Minute)}
		plan := planFreshOnly([]core.Credential{{AuthIndex: "recovered", Priority: -1}}, []QuotaEvidence{healthy}, cooldownOptions)
		if len(plan.Items) != 1 || plan.Items[0].Priority != -1 || plan.Items[0].Disabled || plan.Items[0].Reason != Reason429Cooldown {
			t.Fatalf("cooldown item = %#v; want active soft cooldown", plan.Items)
		}
	})

	t.Run("expired 429 cooldown recovers current minus one", func(t *testing.T) {
		cooldownOptions := options
		cooldownOptions.CooldownAuthIndexes = map[string]time.Time{"recovered": now.Add(-time.Second)}
		plan := planFreshOnly([]core.Credential{{AuthIndex: "recovered", Priority: -1}}, []QuotaEvidence{healthy}, cooldownOptions)
		if len(plan.Items) != 1 || plan.Items[0].Priority != 100 {
			t.Fatalf("expired cooldown plan = %#v; want recovered positive priority", plan)
		}
	})

	t.Run("short-window depletion remains soft depleted", func(t *testing.T) {
		depleted := healthy
		depleted.ShortWindowRemaining = int64Ptr(0)
		plan := planFreshOnly([]core.Credential{{AuthIndex: "recovered", Priority: -1}}, []QuotaEvidence{depleted}, options)
		if len(plan.Items) != 1 || plan.Items[0].Priority != -1 || plan.Items[0].Disabled || plan.Items[0].Reason != ReasonFreshShortWindowDepleted {
			t.Fatalf("short-depleted item = %#v; want soft depletion", plan.Items)
		}
	})
}

func TestPlanWithEvidence_FreeTierIsolatedBelowPro(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset7dFar := now.Add(100 * time.Hour)
	reset7dNear := now.Add(5 * time.Hour) // resets in 5h

	creds := []core.Credential{
		{AuthIndex: "pro-boost", PlanType: core.PlanTypePro, Priority: 50, Disabled: false},
		{AuthIndex: "pro-regular", PlanType: core.PlanTypePro, Priority: 50, Disabled: false},
		{AuthIndex: "free-near-reset", PlanType: core.PlanTypeFree, Priority: 50, Disabled: false},
		{AuthIndex: "free-far-reset", PlanType: core.PlanTypeFree, Priority: 50, Disabled: false},
	}

	evidence := []QuotaEvidence{
		{
			AuthIndex:            "pro-boost",
			PlanType:             core.PlanTypePro,
			ShortWindowRemaining: int64Ptr(100),
			ShortWindowResetAt:   &reset5h,
			LongWindowRemaining:  int64Ptr(80),
			LongWindowResetAt:    &reset7dNear, // qualifies for boost
			CycleBurnRate:        0.15,
		},
		{
			AuthIndex:            "pro-regular",
			PlanType:             core.PlanTypePro,
			ShortWindowRemaining: int64Ptr(100),
			ShortWindowResetAt:   &reset5h,
			LongWindowRemaining:  int64Ptr(80),
			LongWindowResetAt:    &reset7dFar, // regular healthy
			CycleBurnRate:        0.15,
		},
		{
			AuthIndex:           "free-near-reset",
			PlanType:            core.PlanTypeFree,
			LongWindowRemaining: int64Ptr(90),
			LongWindowResetAt:   &reset7dNear, // near reset, but Free plan -> MUST NOT boost!
			CycleBurnRate:       0.15,
		},
		{
			AuthIndex:           "free-far-reset",
			PlanType:            core.PlanTypeFree,
			LongWindowRemaining: int64Ptr(30),
			LongWindowResetAt:   &reset7dFar,
			CycleBurnRate:       0.15,
		},
	}

	opts := Options{
		Now:                 now,
	}

	plan := planFreshOnly(creds, evidence, opts)
	itemMap := make(map[string]PlanItem, len(plan.Items))
	for _, it := range plan.Items {
		itemMap[it.Credential.AuthIndex] = it
	}

	// 1. pro-boost gets Tier 1 (999)
	proBoost := itemMap["pro-boost"]
	if proBoost.Priority != 999 || !proBoost.IsBoosted || proBoost.Reason != ReasonFreshBoosted {
		t.Errorf("pro-boost priority = %d, boosted = %v, reason = %s; want 999, true, %s",
			proBoost.Priority, proBoost.IsBoosted, proBoost.Reason, ReasonFreshBoosted)
	}

	// 2. pro-regular gets Tier 2 (100)
	proReg := itemMap["pro-regular"]
	if proReg.Priority != 100 || proReg.IsBoosted || proReg.Reason != ReasonFreshRemainingPositive {
		t.Errorf("pro-regular priority = %d, boosted = %v; want 100, false", proReg.Priority, proReg.IsBoosted)
	}

	// 3. free-near-reset MUST NOT be boosted, gets Tier 3 (10)
	freeNear := itemMap["free-near-reset"]
	if freeNear.Priority != 10 || freeNear.IsBoosted {
		t.Errorf("free-near-reset priority = %d, boosted = %v; want 10, false", freeNear.Priority, freeNear.IsBoosted)
	}

	// 4. free-far-reset gets Tier 3 (less urgency than free-near-reset -> 9)
	freeFar := itemMap["free-far-reset"]
	if freeFar.Priority > 10 || freeFar.Priority < 1 || freeFar.Priority >= freeNear.Priority {
		t.Errorf("free-far-reset priority = %d; want < %d and >= 1", freeFar.Priority, freeNear.Priority)
	}

	// 5. Ensure all PRO priorities are strictly greater than FREE priorities
	if proReg.Priority <= freeNear.Priority {
		t.Errorf("pro-regular priority (%d) should be strictly greater than free-near-reset priority (%d)",
			proReg.Priority, freeNear.Priority)
	}
}

func TestPlanWithEvidence_CustomTierOverridesAutoAndBalancesLoad(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(72 * time.Hour)

	// Setup 3 accounts: a, b, c
	// Without custom tier, urgency would determine priorities:
	// a has highest urgency -> auto Tier 1
	// b has medium urgency -> auto Tier 2
	// c has low urgency -> auto Tier 3
	remHigh := int64(80)
	remMed := int64(50)
	remLow := int64(20)

	creds := []core.Credential{
		{AuthIndex: "account-a", PlanType: core.PlanTypePro, Priority: 50, Disabled: false},
		{AuthIndex: "account-b", PlanType: core.PlanTypePro, Priority: 50, Disabled: false},
		{AuthIndex: "account-c", PlanType: core.PlanTypePro, Priority: 50, Disabled: false},
	}

	evidence := plannerEvidence{
		Fresh: []QuotaEvidence{
			{
				AuthIndex:            "account-a",
				PlanType:             core.PlanTypePro,
				ShortWindowRemaining: &remHigh,
				ShortWindowResetAt:   &reset5h,
				LongWindowRemaining:  &remHigh,
				LongWindowResetAt:    &reset7d,
				CycleBurnRate:        0.9, // Highest urgency -> normally Tier 1
			},
			{
				AuthIndex:            "account-b",
				PlanType:             core.PlanTypePro,
				ShortWindowRemaining: &remMed,
				ShortWindowResetAt:   &reset5h,
				LongWindowRemaining:  &remMed,
				LongWindowResetAt:    &reset7d,
				CycleBurnRate:        0.5, // Medium urgency -> normally Tier 2
			},
			{
				AuthIndex:            "account-c",
				PlanType:             core.PlanTypePro,
				ShortWindowRemaining: &remLow,
				ShortWindowResetAt:   &reset5h,
				LongWindowRemaining:  &remLow,
				LongWindowResetAt:    &reset7d,
				CycleBurnRate:        0.1, // Lowest urgency -> normally Tier 3
			},
		},
	}

	// User sets: C: custom tier 1, a: 2, b: 2
	options := Options{
		Now:                 now,
		UrgencyTolerance:    0.05,
		CustomTiers: map[string]int{
			"account-c": 1,
			"account-a": 2,
			"account-b": 2,
		},
	}

	plan := planWithEvidence(creds, evidence, options)
	itemMap := make(map[string]PlanItem)
	for _, item := range plan.Items {
		itemMap[item.Credential.AuthIndex] = item
	}

	itemC := itemMap["account-c"]
	itemA := itemMap["account-a"]
	itemB := itemMap["account-b"]

	// 1. Account C has Custom Tier 1 -> gets higher priority (990)
	if itemC.Priority != 990 {
		t.Errorf("account-c priority = %d, want 990", itemC.Priority)
	}
	if itemC.Reason != "custom tier #1" {
		t.Errorf("account-c reason = %s, want 'custom tier #1'", itemC.Reason)
	}

	// 2. Account a and b have Custom Tier 2 -> get equal priority (980) for load balancing
	if itemA.Priority != 980 {
		t.Errorf("account-a priority = %d, want 980", itemA.Priority)
	}
	if itemB.Priority != 980 {
		t.Errorf("account-b priority = %d, want 980", itemB.Priority)
	}
	if itemA.Priority != itemB.Priority {
		t.Errorf("account-a (%d) and account-b (%d) must have equal priority for load balancing",
			itemA.Priority, itemB.Priority)
	}
	if itemA.Reason != "custom tier #2" || itemB.Reason != "custom tier #2" {
		t.Errorf("account-a/b reasons: want 'custom tier #2', got %q and %q", itemA.Reason, itemB.Reason)
	}

	// 3. Priority of C > Priority of a/b
	if itemC.Priority <= itemA.Priority {
		t.Errorf("account-c priority (%d) must be strictly greater than account-a/b priority (%d)",
			itemC.Priority, itemA.Priority)
	}
}
