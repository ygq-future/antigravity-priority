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

	t.Run("pro candidate takes precedence over free candidate regardless of urgency", func(t *testing.T) {
		proItem := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-pro"},
			PlanType:   core.PlanTypePro,
			Urgency:    0.5,
		}
		freeItem := PlanItem{
			Credential: core.Credential{AuthIndex: "auth-free"},
			PlanType:   core.PlanTypeFree,
			Urgency:    5.0, // higher urgency, but still Free
		}

		if res := CompareHealthyCandidates(proItem, freeItem); res != -1 {
			t.Errorf("CompareHealthyCandidates(pro, free) = %v; want -1", res)
		}
		if res := CompareHealthyCandidates(freeItem, proItem); res != 1 {
			t.Errorf("CompareHealthyCandidates(free, pro) = %v; want 1", res)
		}
	})
}

func TestSortPlanItems(t *testing.T) {
	items := []PlanItem{
		{
			Credential: core.Credential{AuthIndex: "unprobed"},
			Priority:   50,
		},
		{
			Credential: core.Credential{AuthIndex: "hard-depleted"},
			Priority:   -1,
			Disabled:   true,
		},
		{
			Credential: core.Credential{AuthIndex: "boost-1"},
			Priority:   999,
		},
		{
			Credential: core.Credential{AuthIndex: "boost-2"},
			Priority:   998,
		},
		{
			Credential: core.Credential{AuthIndex: "soft-depleted"},
			Priority:   -1,
			Disabled:   false,
		},
	}

	SortPlanItems(items)

	// Expected order: priority descending, then enabled before disabled,
	// then AuthIndex ascending.
	want := []string{"boost-1", "boost-2", "unprobed", "soft-depleted", "hard-depleted"}
	for index, authIndex := range want {
		if items[index].Credential.AuthIndex != authIndex {
			t.Errorf("item[%d] = %s; want %s", index, items[index].Credential.AuthIndex, authIndex)
		}
	}
}
