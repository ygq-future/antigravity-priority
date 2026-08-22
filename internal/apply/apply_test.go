package apply_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
)

type documentFixture struct {
	paths       map[string]string
	replaceErr  map[string]error
	replaceHook func(authIndex string, path string)
}

func newDocumentFixture(t *testing.T, documents map[string]string) (*documentFixture, host.API) {
	t.Helper()
	fixture := &documentFixture{paths: make(map[string]string), replaceErr: make(map[string]error)}
	for authIndex, contents := range documents {
		path := filepath.Join(t.TempDir(), authIndex+".json")
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		fixture.paths[authIndex] = path
	}
	return fixture, fixture
}

func (f *documentFixture) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	if err := ctx.Err(); err != nil {
		return host.AuthDocument{}, err
	}
	path, ok := f.paths[authIndex]
	if !ok {
		return host.AuthDocument{}, errors.New("document not found")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return host.AuthDocument{}, err
	}
	return host.AuthDocument{AuthIndex: authIndex, Name: "credential-" + authIndex, Path: path, JSON: raw}, nil
}

func (f *documentFixture) ListAuthFiles(context.Context) ([]host.AuthFile, error) { return nil, nil }
func (f *documentFixture) GetRuntime(context.Context, string) (host.RuntimeAuth, error) {
	return host.RuntimeAuth{}, nil
}
func (f *documentFixture) SaveAuth(context.Context, string, json.RawMessage) error { return nil }
func (f *documentFixture) HTTPDo(context.Context, host.HTTPRequest) (host.HTTPResponse, error) {
	return host.HTTPResponse{}, nil
}

type missingIdentityStore struct{ host.API }

func (s missingIdentityStore) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	document, err := s.API.GetAuth(ctx, authIndex)
	document.AuthIndex = ""
	return document, err
}

func TestHostTransition_AtomicTargetPreservesUnrelatedFields(t *testing.T) {
	fixture, client := newDocumentFixture(t, map[string]string{
		"idx-1": `{"access_token":"secret","priority":10,"disabled":false,"metadata":{"keep":true}}`,
	})
	result, err := apply.NewHostTransition(client).Execute(context.Background(), apply.TransitionRound{Intents: []apply.TransitionIntent{{
		AuthIndex: "idx-1",
		Expected:  apply.CredentialState{AuthIndex: "idx-1", Priority: 10, PriorityPresent: true, Disabled: false, DisabledKnown: true},
		Target: apply.CredentialTarget{
			Priority: apply.PriorityTarget{Operation: apply.FieldSet, Value: 90},
			Disabled: apply.DisabledTarget{Operation: apply.FieldSet, Value: true},
		},
		Cause: "ordinary apply",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Totals.Committed != 1 || len(result.Details) != 1 || result.Details[0].Outcome != apply.OutcomeCommitted {
		t.Fatalf("unexpected transition result: %#v", result)
	}
	data, err := os.ReadFile(fixture.paths["idx-1"])
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document["priority"] != float64(90) || document["disabled"] != true || document["metadata"].(map[string]any)["keep"] != true {
		t.Fatalf("atomic transition did not preserve document: %s", data)
	}
}

func TestHostTransition_NoChangeAndIndependentLaterCredential(t *testing.T) {
	fixture, client := newDocumentFixture(t, map[string]string{
		"same":  `{"priority":10,"disabled":false}`,
		"later": `{"priority":20,"disabled":false}`,
	})
	fixture.replaceErr["later"] = errors.New("disk unavailable token=secret")
	result, err := apply.NewHostTransition(client).Execute(context.Background(), apply.TransitionRound{Intents: []apply.TransitionIntent{
		{AuthIndex: "same", Expected: apply.CredentialState{AuthIndex: "same", Priority: 10, PriorityPresent: true, Disabled: false, DisabledKnown: true}, Target: apply.CredentialTarget{Priority: apply.PriorityTarget{Operation: apply.FieldSet, Value: 10}}},
		{AuthIndex: "later", Expected: apply.CredentialState{AuthIndex: "later", Priority: 20, PriorityPresent: true, Disabled: false, DisabledKnown: true}, Target: apply.CredentialTarget{Priority: apply.PriorityTarget{Operation: apply.FieldSet, Value: 30}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Totals.NoChange != 1 || result.Totals.Failed != 1 || result.Totals.Attempted != 2 {
		t.Fatalf("totals were not derived from independent details: %#v", result.Totals)
	}
	if strings.Contains(result.Details[1].Error, "secret") || !strings.Contains(result.Details[1].Error, host.RedactedValue) {
		t.Fatalf("transition error was not redacted: %#v", result.Details[1])
	}
}

func (f *documentFixture) ReplaceAuth(ctx context.Context, document host.AuthDocument, doc json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := f.replaceErr[document.AuthIndex]; err != nil {
		return err
	}
	if f.replaceHook != nil {
		f.replaceHook(document.AuthIndex, document.Path)
	}
	return os.WriteFile(document.Path, doc, 0o600)
}

func TestHostTransition_ConflictDoesNotOverwriteNewerHostState(t *testing.T) {
	fixture, client := newDocumentFixture(t, map[string]string{
		"idx-1": `{"priority":25,"disabled":false,"metadata":{"new":true}}`,
	})
	result, err := apply.NewHostTransition(client).Execute(context.Background(), apply.TransitionRound{Intents: []apply.TransitionIntent{{
		AuthIndex: "idx-1",
		Expected:  apply.CredentialState{AuthIndex: "idx-1", Priority: 10, PriorityPresent: true, Disabled: false, DisabledKnown: true},
		Target:    apply.CredentialTarget{Priority: apply.PriorityTarget{Operation: apply.FieldSet, Value: 90}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Details[0].Outcome != apply.OutcomeConflict || result.Details[0].Reason != "priority_conflict" {
		t.Fatalf("expected priority conflict, got %#v", result.Details[0])
	}
	data, _ := os.ReadFile(fixture.paths["idx-1"])
	if strings.Contains(string(data), "90") {
		t.Fatalf("conflicting transition overwrote Host state: %s", data)
	}
}

func TestHostTransition_MissingIdentityCannotCommit(t *testing.T) {
	_, client := newDocumentFixture(t, map[string]string{
		"idx-1": `{"priority":10,"disabled":false}`,
	})
	result, err := apply.NewHostTransition(missingIdentityStore{API: client}).Execute(context.Background(), apply.TransitionRound{Intents: []apply.TransitionIntent{{
		AuthIndex: "idx-1",
		Expected:  apply.CredentialState{AuthIndex: "idx-1", Priority: 10, PriorityPresent: true, Disabled: false, DisabledKnown: true},
		Target:    apply.CredentialTarget{Priority: apply.PriorityTarget{Operation: apply.FieldSet, Value: 20}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Details[0].Outcome != apply.OutcomeConflict || result.Details[0].Reason != apply.ReasonIdentityConflict {
		t.Fatalf("missing Host identity was not rejected: %#v", result.Details[0])
	}
}

func TestHostTransition_SetUnsetAndCancellationSemantics(t *testing.T) {
	fixture, client := newDocumentFixture(t, map[string]string{
		"reset":  `{"priority":90,"disabled":true,"metadata":{"keep":1}}`,
		"cancel": `{"priority":10,"disabled":false}`,
	})
	result, err := apply.NewHostTransition(client).Execute(context.Background(), apply.TransitionRound{Intents: []apply.TransitionIntent{{
		AuthIndex: "reset",
		Expected:  apply.CredentialState{AuthIndex: "reset", Priority: 90, PriorityPresent: true, Disabled: true, DisabledKnown: true},
		Target: apply.CredentialTarget{
			Priority: apply.PriorityTarget{Operation: apply.FieldUnset},
			Disabled: apply.DisabledTarget{Operation: apply.FieldUnchanged},
		},
	}}})
	if err != nil || result.Details[0].Outcome != apply.OutcomeCommitted {
		t.Fatalf("reset transition failed: %#v, %v", result, err)
	}
	var reset map[string]any
	data, _ := os.ReadFile(fixture.paths["reset"])
	_ = json.Unmarshal(data, &reset)
	if _, ok := reset["priority"]; ok || reset["disabled"] != true || reset["metadata"].(map[string]any)["keep"] != float64(1) {
		t.Fatalf("reset changed unrelated or disabled state: %s", data)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = apply.NewHostTransition(client).Execute(canceled, apply.TransitionRound{Intents: []apply.TransitionIntent{{
		AuthIndex: "cancel",
		Expected:  apply.CredentialState{AuthIndex: "cancel", Priority: 10, PriorityPresent: true, Disabled: false, DisabledKnown: false},
		Target:    apply.CredentialTarget{Priority: apply.PriorityTarget{Operation: apply.FieldSet, Value: 20}},
	}}})
	if err != nil || result.Details[0].Outcome != apply.OutcomeFailed || result.Details[0].Reason != "context_canceled" {
		t.Fatalf("pre-commit cancellation was not failed: %#v, %v", result, err)
	}

	fixture.replaceHook = func(_ string, _ string) { cancel() }
	result, err = apply.NewHostTransition(client).Execute(context.Background(), apply.TransitionRound{Intents: []apply.TransitionIntent{{
		AuthIndex: "cancel",
		Expected:  apply.CredentialState{AuthIndex: "cancel", Priority: 10, PriorityPresent: true, Disabled: false, DisabledKnown: false},
		Target:    apply.CredentialTarget{Priority: apply.PriorityTarget{Operation: apply.FieldSet, Value: 20}},
	}}})
	if err != nil || result.Details[0].Outcome != apply.OutcomeCommitted {
		t.Fatalf("post-commit cancellation negated committed state: %#v, %v", result, err)
	}
}

func TestExecutePlan_UsesTransitionAndDerivedCounters(t *testing.T) {
	fixture, client := newDocumentFixture(t, map[string]string{"idx-1": `{"priority":10,"disabled":false}`})
	plan := priority.Plan{
		Items: []priority.PlanItem{
			{Credential: core.Credential{AuthIndex: "idx-1", Name: "credential-idx-1", Priority: 10, Disabled: false}, Priority: 20, Disabled: false, EvidenceFresh: true, Reason: "fresh"},
			{Credential: core.Credential{AuthIndex: "idx-2", Name: "skipped", Priority: 30}, Priority: 30, Disabled: false, EvidenceFresh: false, Reason: priority.ReasonKeepCurrentState},
		},
		Changes: []priority.Change{{Credential: core.Credential{AuthIndex: "idx-1", Name: "credential-idx-1", Priority: 10, Disabled: false}, Priority: 20, Disabled: false, EvidenceFresh: true, Reason: "fresh"}},
	}
	result, err := apply.Apply(context.Background(), apply.Request{Transition: apply.NewHostTransition(client), Plan: plan, ReportSkippedPlan: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Succeeded != 1 || result.Skipped != 1 || result.Transitions.Totals.Committed != 1 {
		t.Fatalf("unexpected plan projection: %#v", result)
	}
	if result.Record.Status != apply.RecordNotAttempted || strings.Contains(string(mustJSON(t, result)), "token") {
		t.Fatalf("unexpected result projection: %#v", result)
	}
	_ = fixture
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
