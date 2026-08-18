package apply_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
)

type mockHost struct {
	patchPriorityFunc func(ctx context.Context, authIndex string, p int) error
	patchDisabledFunc func(ctx context.Context, name string, disabled bool) error

	patchedPriorities map[string]int
	patchedDisabled   map[string]bool
}

func newMockHost() *mockHost {
	return &mockHost{
		patchedPriorities: make(map[string]int),
		patchedDisabled:   make(map[string]bool),
	}
}

func (m *mockHost) PatchPriority(ctx context.Context, authIndex string, p int) error {
	m.patchedPriorities[authIndex] = p
	if m.patchPriorityFunc != nil {
		return m.patchPriorityFunc(ctx, authIndex, p)
	}
	return nil
}

func (m *mockHost) PatchDisabled(ctx context.Context, name string, disabled bool) error {
	m.patchedDisabled[name] = disabled
	if m.patchDisabledFunc != nil {
		return m.patchDisabledFunc(ctx, name, disabled)
	}
	return nil
}

type mockAuditor struct {
	saveSnapshotFunc func(ctx context.Context, snapshot apply.PlanSnapshot) error
	recordEventFunc  func(ctx context.Context, event apply.AuditEvent) error

	snapshots []apply.PlanSnapshot
	events    []apply.AuditEvent
}

func (m *mockAuditor) SaveSnapshot(ctx context.Context, snapshot apply.PlanSnapshot) error {
	m.snapshots = append(m.snapshots, snapshot)
	if m.saveSnapshotFunc != nil {
		return m.saveSnapshotFunc(ctx, snapshot)
	}
	return nil
}

func (m *mockAuditor) RecordEvent(ctx context.Context, event apply.AuditEvent) error {
	m.events = append(m.events, event)
	if m.recordEventFunc != nil {
		return m.recordEventFunc(ctx, event)
	}
	return nil
}

func TestApply_MissingAuditor(t *testing.T) {
	ctx := context.Background()
	_, err := apply.Apply(ctx, apply.Request{
		Host:    newMockHost(),
		Auditor: nil,
		Plan:    priority.Plan{},
	})
	if !errors.Is(err, apply.ErrMissingAuditor) {
		t.Fatalf("expected ErrMissingAuditor, got %v", err)
	}
}

func TestApply_AuditorErrors(t *testing.T) {
	ctx := context.Background()

	// SaveSnapshot error
	auditorErr := errors.New("auditor save failed")
	auditor := &mockAuditor{
		saveSnapshotFunc: func(ctx context.Context, snapshot apply.PlanSnapshot) error {
			return auditorErr
		},
	}
	_, err := apply.Apply(ctx, apply.Request{
		Host:    newMockHost(),
		Auditor: auditor,
		Plan:    priority.Plan{},
	})
	if err == nil || !strings.Contains(err.Error(), "save apply snapshot") {
		t.Fatalf("expected save apply snapshot error, got %v", err)
	}

	// RecordEvent error
	auditor = &mockAuditor{
		recordEventFunc: func(ctx context.Context, event apply.AuditEvent) error {
			return errors.New("record event failed")
		},
	}
	_, err = apply.Apply(ctx, apply.Request{
		Host:    newMockHost(),
		Auditor: auditor,
		Plan:    priority.Plan{},
	})
	if err == nil || !strings.Contains(err.Error(), "record apply audit event") {
		t.Fatalf("expected record apply audit event error, got %v", err)
	}
}

func TestApply_FreshEvidenceGating(t *testing.T) {
	ctx := context.Background()
	hostMock := newMockHost()
	auditorMock := &mockAuditor{}

	plan := priority.Plan{
		Changes: []priority.Change{
			{
				Credential: core.Credential{
					Name:      "stale-cred",
					AuthIndex: "idx-stale",
					Priority:  10,
					Disabled:  false,
				},
				Priority:      50,
				Disabled:      false,
				EvidenceFresh: false, // NOT fresh and NOT forced
				Reason:        "stale calculation",
			},
			{
				Credential: core.Credential{
					Name:      "fresh-cred",
					AuthIndex: "idx-fresh",
					Priority:  10,
					Disabled:  false,
				},
				Priority:      50,
				Disabled:      false,
				EvidenceFresh: true, // Fresh
				Reason:        "fresh positive",
			},
		},
	}

	res, err := apply.Apply(ctx, apply.Request{
		Host:    hostMock,
		Auditor: auditorMock,
		Plan:    plan,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Attempted != 1 {
		t.Errorf("expected Attempted=1, got %d", res.Attempted)
	}
	if res.Succeeded != 1 {
		t.Errorf("expected Succeeded=1, got %d", res.Succeeded)
	}
	if res.Skipped != 1 {
		t.Errorf("expected Skipped=1, got %d", res.Skipped)
	}

	if _, exists := hostMock.patchedPriorities["idx-stale"]; exists {
		t.Errorf("stale credential should not be patched in host")
	}
	if p, exists := hostMock.patchedPriorities["idx-fresh"]; !exists || p != 50 {
		t.Errorf("fresh credential expected priority 50, got %d (exists=%v)", p, exists)
	}

	if res.Changes[0].Status != apply.ChangeStatusSkipped {
		t.Errorf("expected first change status skipped, got %v", res.Changes[0].Status)
	}
	if res.Changes[1].Status != apply.ChangeStatusSuccess {
		t.Errorf("expected second change status success, got %v", res.Changes[1].Status)
	}
}

func TestApply_MinChangeAndNoDifferenceSkipping(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	resetAt := now.Add(200 * time.Hour) // Far future so not boosted

	// Plan with min_change threshold test
	creds := []core.Credential{
		{
			Name:      "cred-1",
			AuthIndex: "idx-1",
			Priority:  100,
			Disabled:  false,
			Status:    core.CredentialStatusActive,
		},
	}

	rem := int64(100)
	evidence := []priority.ProbeEvidence{
		{
			AuthIndex:     "idx-1",
			Remaining:     &rem,
			ResetAt:       &resetAt,
			ObservedAt:    now,
			Freshness:     core.FreshnessFresh,
			ProbeStatus:   core.ProbeStatusReady,
			Status:        priority.EvidenceStatusReady,
			EvidenceFresh: true,
		},
	}

	// With NormalStartPriority = 98 (diff = 2) and MinChange = 5, changes should be empty
	plan := priority.PlanFreshOnly(creds, evidence, priority.Options{
		Now:                 now,
		NormalStartPriority: 98,
		MinChange:           5,
	})

	if len(plan.Changes) != 0 {
		t.Fatalf("expected 0 changes due to diff < min_change, got %d", len(plan.Changes))
	}

	hostMock := newMockHost()
	auditorMock := &mockAuditor{}

	res, err := apply.Apply(ctx, apply.Request{
		Host:              hostMock,
		Auditor:           auditorMock,
		Plan:              plan,
		ReportSkippedPlan: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Attempted != 0 {
		t.Errorf("expected 0 attempted, got %d", res.Attempted)
	}
	if res.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", res.Skipped)
	}
	if len(res.Changes) != 1 || res.Changes[0].Status != apply.ChangeStatusSkipped {
		t.Errorf("expected 1 skipped change result, got %+v", res.Changes)
	}

	// Change with identical priority and disabled should also be skipped in applyChange
	identPlan := priority.Plan{
		Items: []priority.PlanItem{
			{
				Credential: core.Credential{
					Name:      "cred-1",
					AuthIndex: "idx-1",
					Priority:  100,
				},
				Priority:      90,
				EvidenceFresh: true,
			},
			{
				Credential: core.Credential{
					Name:      "cred-2",
					AuthIndex: "idx-2",
					Priority:  50,
				},
				Priority:      50,
				EvidenceFresh: false,
			},
		},
		Changes: []priority.Change{
			{
				Credential: core.Credential{
					Name:      "cred-1",
					AuthIndex: "idx-1",
					Priority:  100,
				},
				Priority:      90,
				EvidenceFresh: true,
				Reason:        "priority change",
			},
			{
				Credential:    creds[0],
				Priority:      100, // same
				Disabled:      false, // same
				EvidenceFresh: true,
				Reason:        "no diff",
			},
		},
	}
	res2, err := apply.Apply(ctx, apply.Request{
		Host:              hostMock,
		Auditor:           auditorMock,
		Plan:              identPlan,
		ReportSkippedPlan: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.Skipped < 1 || res2.Attempted < 1 {
		t.Errorf("expected mixed skipped and attempted, got res: %+v", res2)
	}
}

func TestApply_PriorityAndDisabledPatches(t *testing.T) {
	ctx := context.Background()
	hostMock := newMockHost()
	auditorMock := &mockAuditor{}

	plan := priority.Plan{
		Changes: []priority.Change{
			{
				Credential: core.Credential{
					Name:      "cred-pri",
					AuthIndex: "idx-pri",
					Priority:  10,
					Disabled:  false,
				},
				Priority:      80,
				Disabled:      false,
				EvidenceFresh: true,
				Reason:        "priority only change",
			},
			{
				Credential: core.Credential{
					Name:      "cred-dis",
					AuthIndex: "idx-dis",
					Priority:  50,
					Disabled:  false,
				},
				Priority:      50,
				Disabled:      true,
				EvidenceFresh: true,
				Reason:        "disabled only change",
			},
			{
				Credential: core.Credential{
					Name:      "",
					AuthIndex: "idx-both",
					Priority:  20,
					Disabled:  false,
				},
				Priority:      90,
				Disabled:      true,
				EvidenceFresh: true,
				Reason:        "both priority and disabled change without name",
			},
		},
	}

	res, err := apply.Apply(ctx, apply.Request{
		Host:    hostMock,
		Auditor: auditorMock,
		Plan:    plan,
	})
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}

	if res.Attempted != 3 || res.Succeeded != 3 || res.Failed != 0 {
		t.Fatalf("expected 3 succeeded changes, got %+v", res)
	}

	// Verify host calls
	if hostMock.patchedPriorities["idx-pri"] != 80 {
		t.Errorf("expected idx-pri priority 80, got %d", hostMock.patchedPriorities["idx-pri"])
	}
	if _, disabledPatched := hostMock.patchedDisabled["cred-pri"]; disabledPatched {
		t.Errorf("cred-pri should not have disabled patched")
	}

	if hostMock.patchedDisabled["cred-dis"] != true {
		t.Errorf("expected cred-dis disabled true, got %v", hostMock.patchedDisabled["cred-dis"])
	}

	if hostMock.patchedPriorities["idx-both"] != 90 {
		t.Errorf("expected idx-both priority 90, got %d", hostMock.patchedPriorities["idx-both"])
	}
	if hostMock.patchedDisabled["idx-both"] != true {
		t.Errorf("expected idx-both (fallback name) disabled true, got %v", hostMock.patchedDisabled["idx-both"])
	}
}

func TestApply_HostErrors(t *testing.T) {
	ctx := context.Background()

	// Missing host when changes need to be applied
	plan := priority.Plan{
		Changes: []priority.Change{
			{
				Credential: core.Credential{
					Name:      "cred-1",
					AuthIndex: "idx-1",
					Priority:  10,
				},
				Priority:      50,
				EvidenceFresh: true,
			},
		},
	}

	res, err := apply.Apply(ctx, apply.Request{
		Host:    nil,
		Auditor: &mockAuditor{},
		Plan:    plan,
	})
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if res.Failed != 1 || res.Changes[0].Status != apply.ChangeStatusFailed {
		t.Fatalf("expected change failed when host is nil, got res: %+v", res)
	}
	if !strings.Contains(res.Changes[0].Error, "host is required") {
		t.Errorf("expected error message to mention host is required, got %s", res.Changes[0].Error)
	}

	// PatchPriority error
	hostMock := newMockHost()
	hostMock.patchPriorityFunc = func(ctx context.Context, authIndex string, p int) error {
		return errors.New("database locked with secret-token=xyz")
	}
	res, err = apply.Apply(ctx, apply.Request{
		Host:    hostMock,
		Auditor: &mockAuditor{},
		Plan:    plan,
	})
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if res.Failed != 1 || res.Changes[0].Status != apply.ChangeStatusFailed {
		t.Fatalf("expected change failed on PatchPriority error, got res: %+v", res)
	}
	if strings.Contains(res.Changes[0].Error, "xyz") {
		t.Errorf("error should redact sensitive token, got %s", res.Changes[0].Error)
	}
	if !strings.Contains(res.Changes[0].Error, host.RedactedValue) {
		t.Errorf("error should contain REDACTED, got %s", res.Changes[0].Error)
	}

	// PatchDisabled error
	disPlan := priority.Plan{
		Changes: []priority.Change{
			{
				Credential: core.Credential{
					Name:      "cred-2",
					AuthIndex: "idx-2",
					Disabled:  false,
				},
				Disabled:      true,
				EvidenceFresh: true,
			},
		},
	}
	hostMock = newMockHost()
	hostMock.patchDisabledFunc = func(ctx context.Context, name string, disabled bool) error {
		return errors.New("host patch disabled failed with Bearer secret-auth-header")
	}
	res, err = apply.Apply(ctx, apply.Request{
		Host:    hostMock,
		Auditor: &mockAuditor{},
		Plan:    disPlan,
	})
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if res.Failed != 1 || res.Changes[0].Status != apply.ChangeStatusFailed {
		t.Fatalf("expected change failed on PatchDisabled error, got res: %+v", res)
	}
	if strings.Contains(res.Changes[0].Error, "secret-auth-header") {
		t.Errorf("error should redact sensitive token, got %s", res.Changes[0].Error)
	}
}

func TestFailureResult(t *testing.T) {
	item := priority.PlanItem{
		Credential: core.Credential{
			Name:      "cred-test",
			AuthIndex: "idx-test",
			Priority:  10,
		},
		Priority: 50,
		Reason:   "test failure",
	}

	err := errors.New("token=abc-123 failed")
	res := apply.FailureResult(item, err)

	if res.Status != apply.ChangeStatusFailed {
		t.Errorf("expected Status Failed, got %v", res.Status)
	}
	if res.Success {
		t.Errorf("expected Success false, got true")
	}
	if strings.Contains(res.Error, "abc-123") {
		t.Errorf("error was not redacted: %s", res.Error)
	}
	if !strings.Contains(res.Error, host.RedactedValue) {
		t.Errorf("error should contain REDACTED: %s", res.Error)
	}
}
