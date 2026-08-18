package priority

import (
	"testing"

	"antigravity-priority/internal/core"
)

func TestCompareHealthyCandidates(t *testing.T) {
	t.Run("urgency descending takes first precedence", func(t *testing.T) {
		left := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-b"},
			Urgency:    1.2,
			T5h:        4.0,
		}
		right := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-a"},
			Urgency:    0.8,
			T5h:        1.0,
		}

		// left has higher urgency -> should return -1
		res := CompareHealthyCandidates(left, right)
		if res != -1 {
			t.Errorf("CompareHealthyCandidates() = %v; want -1 (higher urgency first)", res)
		}
		resRev := CompareHealthyCandidates(right, left)
		if resRev != 1 {
			t.Errorf("CompareHealthyCandidates() reverse = %v; want 1", resRev)
		}
	})

	t.Run("5h reset countdown ascending breaks tie on equal urgency", func(t *testing.T) {
		left := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-z"},
			Urgency:    1.0,
			T5h:        1.5,
		}
		right := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-a"},
			Urgency:    1.0,
			T5h:        3.5,
		}

		// left resets sooner (1.5h vs 3.5h) -> should return -1
		res := CompareHealthyCandidates(left, right)
		if res != -1 {
			t.Errorf("CompareHealthyCandidates() = %v; want -1 (earlier 5h reset first)", res)
		}
	})

	t.Run("authIndex ascending breaks tie on equal urgency and 5h reset", func(t *testing.T) {
		left := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-01"},
			Urgency:    1.0,
			T5h:        2.0,
		}
		right := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-02"},
			Urgency:    1.0,
			T5h:        2.0,
		}

		// auth-01 < auth-02 -> should return -1
		res := CompareHealthyCandidates(left, right)
		if res != -1 {
			t.Errorf("CompareHealthyCandidates() = %v; want -1 (auth-01 before auth-02)", res)
		}
	})
}

func TestCompareUniquenessCandidates(t *testing.T) {
	rem100 := int64(100)
	t.Run("fresh positive comes before unprobed peer", func(t *testing.T) {
		fresh := PlanItem{
			Credential:    core.Credential{AuthIndex: "auth-fresh", Priority: 50},
			EvidenceFresh: true,
			Remaining:     &rem100,
			R7d:           1.0,
		}
		unprobed := PlanItem{
			Credential:    core.Credential{AuthIndex: "auth-unprobed", Priority: 100},
			EvidenceFresh: false,
		}

		if res := CompareUniquenessCandidates(fresh, unprobed); res != -1 {
			t.Errorf("CompareUniquenessCandidates(fresh, unprobed) = %v; want -1", res)
		}
		if res := CompareUniquenessCandidates(unprobed, fresh); res != 1 {
			t.Errorf("CompareUniquenessCandidates(unprobed, fresh) = %v; want 1", res)
		}
	})

	t.Run("boosted fresh comes before regular fresh", func(t *testing.T) {
		boosted := PlanItem{
			Credential:    core.Credential{AuthIndex: "auth-boost"},
			EvidenceFresh: true,
			Remaining:     &rem100,
			IsBoosted:     true,
			Urgency:       0.5,
		}
		regular := PlanItem{
			Credential:    core.Credential{AuthIndex: "auth-reg"},
			EvidenceFresh: true,
			Remaining:     &rem100,
			IsBoosted:     false,
			Urgency:       1.5,
		}

		if res := CompareUniquenessCandidates(boosted, regular); res != -1 {
			t.Errorf("CompareUniquenessCandidates(boosted, regular) = %v; want -1", res)
		}
	})

	t.Run("unprobed peers sort by existing priority then AuthIndex", func(t *testing.T) {
		peerA := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-a", Priority: 80},
			Priority:   80,
		}
		peerB := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-b", Priority: 90},
			Priority:   90,
		}
		peerC := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-c", Priority: 80},
			Priority:   80,
		}

		// peerB (90) > peerA (80) -> CompareUniquenessCandidates(peerA, peerB) > 0
		if res := CompareUniquenessCandidates(peerA, peerB); res <= 0 {
			t.Errorf("CompareUniquenessCandidates(peerA, peerB) = %v; want >0", res)
		}
		// peerA (80, "auth-a") vs peerC (80, "auth-c") -> -1
		if res := CompareUniquenessCandidates(peerA, peerC); res != -1 {
			t.Errorf("CompareUniquenessCandidates(peerA, peerC) = %v; want -1", res)
		}
	})
}

func TestSortPlanItems(t *testing.T) {
	items := []PlanItem{
		{
			Credential:    core.Credential{AuthIndex: "unprobed-2"},
			Priority:      50,
			EvidenceFresh: false,
		},
		{
			Credential:    core.Credential{AuthIndex: "fresh-depleted"},
			Priority:      -1,
			Disabled:      true,
			EvidenceFresh: true,
		},
		{
			Credential:    core.Credential{AuthIndex: "fresh-boost-1"},
			Priority:      999,
			EvidenceFresh: true,
		},
		{
			Credential:    core.Credential{AuthIndex: "fresh-boost-2"},
			Priority:      998,
			EvidenceFresh: true,
		},
		{
			Credential:    core.Credential{AuthIndex: "fresh-soft-depleted"},
			Priority:      -1,
			Disabled:      false,
			EvidenceFresh: true,
		},
	}

	SortPlanItems(items)

	// Expected order:
	// 0: fresh-boost-1 (999, fresh)
	// 1: fresh-boost-2 (998, fresh)
	// 2: fresh-soft-depleted (-1, fresh, disabled=false)
	// 3: fresh-depleted (-1, fresh, disabled=true)
	// 4: unprobed-2 (50, non-fresh)
	if items[0].Credential.AuthIndex != "fresh-boost-1" {
		t.Errorf("item[0] = %s; want fresh-boost-1", items[0].Credential.AuthIndex)
	}
	if items[1].Credential.AuthIndex != "fresh-boost-2" {
		t.Errorf("item[1] = %s; want fresh-boost-2", items[1].Credential.AuthIndex)
	}
	if items[2].Credential.AuthIndex != "fresh-soft-depleted" {
		t.Errorf("item[2] = %s; want fresh-soft-depleted", items[2].Credential.AuthIndex)
	}
	if items[3].Credential.AuthIndex != "fresh-depleted" {
		t.Errorf("item[3] = %s; want fresh-depleted", items[3].Credential.AuthIndex)
	}
	if items[4].Credential.AuthIndex != "unprobed-2" {
		t.Errorf("item[4] = %s; want unprobed-2", items[4].Credential.AuthIndex)
	}

	t.Run("isPositiveRemaining helper variants", func(t *testing.T) {
		rem50 := int64(50)
		remZero := int64(0)
		if !isPositiveRemaining(PlanItem{LongWindowRemaining: &rem50}) {
			t.Errorf("isPositiveRemaining with LongWindowRemaining > 0 should be true")
		}
		if isPositiveRemaining(PlanItem{LongWindowRemaining: &remZero}) {
			t.Errorf("isPositiveRemaining with LongWindowRemaining == 0 should be false")
		}
		if !isPositiveRemaining(PlanItem{Remaining: &rem50}) {
			t.Errorf("isPositiveRemaining with Remaining > 0 should be true")
		}
		if !isPositiveRemaining(PlanItem{R7d: 0.5}) {
			t.Errorf("isPositiveRemaining with R7d > 0 should be true")
		}
		if isPositiveRemaining(PlanItem{}) {
			t.Errorf("isPositiveRemaining with empty PlanItem should be false")
		}
	})
}
