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
			Credential: core.Credential{AuthIndex: "unprobed-2"},
			Priority:   50,
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

}
