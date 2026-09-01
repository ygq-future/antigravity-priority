package apply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
)

// Outcome is the only Host outcome vocabulary exposed by a Host Transition.
type Outcome string

const (
	OutcomeNoChange  Outcome = "no_change"
	OutcomeCommitted Outcome = "committed"
	OutcomeFailed    Outcome = "failed"
	OutcomeConflict  Outcome = "conflict"
	OutcomeUncertain Outcome = "uncertain"
)

// Reason values are stable, low-cardinality explanations for Host outcomes.
const (
	ReasonAlreadySatisfied       = "already_satisfied"
	ReasonCommitted              = "committed"
	ReasonContextCanceled        = "context_canceled"
	ReasonInvalidIntent          = "invalid_intent"
	ReasonReadFailed             = "read_failed"
	ReasonDecodeFailed           = "decode_failed"
	ReasonIdentityConflict       = "identity_conflict"
	ReasonPriorityConflict       = "priority_conflict"
	ReasonDisabledConflict       = "disabled_conflict"
	ReasonCommitFailed           = "commit_failed"
	ReasonVerificationFailed     = "verification_failed"
	ReasonVerificationUnreadable = "verification_unreadable"
)

// FieldOperation describes an explicit operation on one target field. The
// zero value is intentionally Unchanged: zero numeric and boolean values are
// never interpreted as an implicit write.
type FieldOperation string

const (
	FieldUnchanged FieldOperation = "unchanged"
	FieldSet       FieldOperation = "set"
	FieldUnset     FieldOperation = "unset"
)

// PriorityTarget is the requested operation for priority.
type PriorityTarget struct {
	Operation FieldOperation `json:"operation"`
	Value     int            `json:"value,omitempty"`
}

// DisabledTarget is the requested operation for disabled.
type DisabledTarget struct {
	Operation FieldOperation `json:"operation"`
	Value     bool           `json:"value,omitempty"`
}

// CredentialTarget contains independent operations for the two Host fields.
type CredentialTarget struct {
	Priority PriorityTarget `json:"priority"`
	Disabled DisabledTarget `json:"disabled"`
}

// KeepPriority, SetPriority, and UnsetPriority make the explicit operation
// contract readable at call sites.
func KeepPriority() PriorityTarget { return PriorityTarget{Operation: FieldUnchanged} }
func SetPriority(value int) PriorityTarget {
	return PriorityTarget{Operation: FieldSet, Value: value}
}
func UnsetPriority() PriorityTarget { return PriorityTarget{Operation: FieldUnset} }

// KeepDisabled, SetDisabled, and UnsetDisabled are the disabled equivalents.
func KeepDisabled() DisabledTarget { return DisabledTarget{Operation: FieldUnchanged} }
func SetDisabled(value bool) DisabledTarget {
	return DisabledTarget{Operation: FieldSet, Value: value}
}
func UnsetDisabled() DisabledTarget { return DisabledTarget{Operation: FieldUnset} }

// CredentialState is the decision-relevant state observed on a credential.
// DisabledKnown describes whether the field was present in the document; the
// semantic value of a missing disabled field is false.
type CredentialState struct {
	AuthIndex       string `json:"-"`
	Name            string `json:"-"`
	Priority        int    `json:"priority"`
	PriorityPresent bool   `json:"priority_present"`
	Disabled        bool   `json:"disabled"`
	DisabledKnown   bool   `json:"disabled_present"`
}

// TransitionIntent is an immutable request for one credential Host
// Transition. Expected is compared with the latest Host document immediately
// before commit; a decision-relevant mismatch produces conflict.
type TransitionIntent struct {
	AuthIndex string           `json:"-"`
	Name      string           `json:"-"`
	Expected  CredentialState  `json:"-"`
	Target    CredentialTarget `json:"target"`
	Cause     string           `json:"cause,omitempty"`
}

// ExpectedState creates an expected state from the domain credential snapshot.
func ExpectedState(credential core.Credential) CredentialState {
	return CredentialState{
		AuthIndex:       credential.AuthIndex,
		Name:            credential.Name,
		Priority:        credential.Priority,
		PriorityPresent: !credential.PriorityMissing,
		Disabled:        credential.Disabled,
		DisabledKnown:   true,
	}
}

// IntentFromChange converts an immutable Planner change into an explicit
// transition intent without moving any scheduling decisions into Apply.
func IntentFromChange(change priority.Change) TransitionIntent {
	target := CredentialTarget{
		Priority: PriorityTarget{Operation: FieldUnchanged},
		Disabled: DisabledTarget{Operation: FieldUnchanged},
	}
	if change.Priority != change.Credential.Priority || change.Credential.PriorityMissing {
		target.Priority = PriorityTarget{Operation: FieldSet, Value: change.Priority}
	}
	if change.Disabled != change.Credential.Disabled {
		target.Disabled = DisabledTarget{Operation: FieldSet, Value: change.Disabled}
	}
	return TransitionIntent{
		AuthIndex: change.Credential.AuthIndex,
		Name:      firstNonEmpty(change.Credential.Name, change.Credential.Account, change.Credential.Email, change.Credential.AuthIndex),
		Expected:  ExpectedState(change.Credential),
		Target:    target,
		Cause:     change.Reason,
	}
}

// CooldownIntent creates the non-destructive 429 Reactive Cooldown target.
func CooldownIntent(credential core.Credential, cause string) TransitionIntent {
	return TransitionIntent{
		AuthIndex: credential.AuthIndex,
		Name:      credential.Name,
		Expected:  ExpectedState(credential),
		Target: CredentialTarget{
			Priority: PriorityTarget{Operation: FieldSet, Value: -1},
			Disabled: DisabledTarget{Operation: FieldUnchanged},
		},
		Cause: cause,
	}
}

// CooldownRecoveryIntent restores the exact Host state captured before a 429 cooldown.
func CooldownRecoveryIntent(credential core.Credential, priorityValue int, priorityMissing, disabled bool, cause string) TransitionIntent {
	priorityTarget := SetPriority(priorityValue)
	if priorityMissing {
		priorityTarget = UnsetPriority()
	}
	return TransitionIntent{
		AuthIndex: credential.AuthIndex,
		Name:      credential.Name,
		Expected:  ExpectedState(credential),
		Target: CredentialTarget{
			Priority: priorityTarget,
			Disabled: SetDisabled(disabled),
		},
		Cause: cause,
	}
}

// ResetIntent creates a priority reset target that preserves disabled state.
func ResetIntent(credential core.Credential, cause string) TransitionIntent {
	return TransitionIntent{
		AuthIndex: credential.AuthIndex,
		Name:      credential.Name,
		Expected:  ExpectedState(credential),
		Target: CredentialTarget{
			Priority: PriorityTarget{Operation: FieldUnset},
			Disabled: DisabledTarget{Operation: FieldUnchanged},
		},
		Cause: cause,
	}
}

// TransitionRound is the batch boundary. Each intent is attempted once and
// failures do not prevent later credentials from being processed.
type TransitionRound struct {
	Intents []TransitionIntent
}

// TransitionTotals are derived exclusively from Details.
type TransitionTotals struct {
	Attempted int `json:"attempted"`
	NoChange  int `json:"no_change"`
	Committed int `json:"committed"`
	Failed    int `json:"failed"`
	Conflicts int `json:"conflicts"`
	Uncertain int `json:"uncertain"`
}

// TransitionResult is a redacted credential-level Host outcome. The raw
// credential identity is retained only in the unexported authIndex field for
// internal projection and is never serialized.
type TransitionResult struct {
	Name              string          `json:"name"`
	AuthIndex         string          `json:"auth_index"`
	Outcome           Outcome         `json:"outcome"`
	Cause             string          `json:"cause,omitempty"`
	Reason            string          `json:"reason"`
	Before            CredentialState `json:"before"`
	After             CredentialState `json:"after"`
	Target            CredentialState `json:"target"`
	PriorityAttempted bool            `json:"priority_attempted"`
	DisabledAttempted bool            `json:"disabled_attempted"`
	Error             string          `json:"error,omitempty"`
	authIndex         string
}

// TransitionRoundResult contains every credential result and derived totals.
type TransitionRoundResult struct {
	Details []TransitionResult `json:"details"`
	Totals  TransitionTotals   `json:"totals"`
}

// HostTransition is the executable seam for all physical credential changes.
type HostTransition interface {
	Execute(ctx context.Context, round TransitionRound) (TransitionRoundResult, error)
}

// Transitioner implements HostTransition using a complete-document store.
type Transitioner struct {
	store host.API
}

// NewHostTransition creates the Apply-layer Host Transition module.
func NewHostTransition(store host.API) *Transitioner {
	return &Transitioner{store: store}
}

// NewTransitioner is an explicit alias for callers that prefer the concrete
// implementation name.
func NewTransitioner(store host.API) *Transitioner {
	return NewHostTransition(store)
}

// Execute applies each intent at most once and derives all round statistics
// from the returned credential details.
func (t *Transitioner) Execute(ctx context.Context, round TransitionRound) (TransitionRoundResult, error) {
	if t == nil || t.store == nil {
		return TransitionRoundResult{}, errors.New("host transition: document store is required")
	}
	result := TransitionRoundResult{Details: make([]TransitionResult, 0, len(round.Intents))}
	for _, intent := range round.Intents {
		detail := t.executeOne(ctx, intent)
		result.Details = append(result.Details, detail)
		result.Totals.add(detail.Outcome)
	}
	return result, nil
}

func (t *Transitioner) executeOne(ctx context.Context, intent TransitionIntent) TransitionResult {
	intent = normalizeIntent(intent)
	result := newTransitionResult(intent)
	if strings.TrimSpace(intent.AuthIndex) == "" {
		return result.withOutcome(OutcomeFailed, ReasonInvalidIntent, errors.New("auth index is required"))
	}
	if err := ctx.Err(); err != nil {
		return result.withOutcome(OutcomeFailed, ReasonContextCanceled, err)
	}

	document, err := t.store.GetAuth(ctx, intent.AuthIndex)
	if err != nil {
		return result.withOutcome(OutcomeFailed, reasonForReadError(ctx, err), err)
	}
	current, fields, err := decodeCredentialDocument(document)
	if err != nil {
		return result.withOutcome(OutcomeFailed, ReasonDecodeFailed, err)
	}
	result.Before = current.redacted()
	if conflictReason := expectedConflict(intent.Expected, current); conflictReason != "" {
		return result.withOutcome(OutcomeConflict, conflictReason, nil)
	}

	target, changed := applyTarget(current, fields, intent.Target)
	result.Target = target.redacted()
	result.PriorityAttempted = intent.Target.Priority.Operation != FieldUnchanged
	result.DisabledAttempted = intent.Target.Disabled.Operation != FieldUnchanged
	if !changed {
		result.After = current.redacted()
		return result.withOutcome(OutcomeNoChange, ReasonAlreadySatisfied, nil)
	}
	if err := ctx.Err(); err != nil {
		return result.withOutcome(OutcomeFailed, ReasonContextCanceled, err)
	}

	encoded, err := json.Marshal(fields)
	if err != nil {
		return result.withOutcome(OutcomeFailed, ReasonDecodeFailed, err)
	}
	if err := t.store.ReplaceAuth(ctx, document, encoded); err != nil {
		return t.resolveAfterCommitError(ctx, intent, document, current, target, result, err)
	}

	return t.verifyCommitted(intent, document, target, result)
}

func (t *Transitioner) resolveAfterCommitError(ctx context.Context, intent TransitionIntent, document host.AuthDocument, before, target credentialDocumentState, result TransitionResult, commitErr error) TransitionResult {
	final, err := t.readFinal(document, intent.AuthIndex)
	if err != nil {
		result.After = before.redacted()
		return result.withOutcome(OutcomeUncertain, ReasonVerificationUnreadable, errors.Join(commitErr, err))
	}
	result.After = final.redacted()
	if target.matches(final) && identityMatches(intent, final) {
		return result.withOutcome(OutcomeCommitted, ReasonCommitted, commitErr)
	}
	if expectedConflict(intent.Expected, final) == "" && final.sameDecisionState(before) {
		return result.withOutcome(OutcomeFailed, ReasonCommitFailed, commitErr)
	}
	return result.withOutcome(OutcomeUncertain, ReasonVerificationFailed, commitErr)
}

func (t *Transitioner) verifyCommitted(intent TransitionIntent, document host.AuthDocument, target credentialDocumentState, result TransitionResult) TransitionResult {
	final, err := t.readFinal(document, intent.AuthIndex)
	if err != nil {
		return result.withOutcome(OutcomeUncertain, ReasonVerificationUnreadable, err)
	}
	result.After = final.redacted()
	if !target.matches(final) || !identityMatches(intent, final) {
		return result.withOutcome(OutcomeUncertain, ReasonVerificationFailed, errors.New("final Host state did not match target"))
	}
	return result.withOutcome(OutcomeCommitted, ReasonCommitted, nil)
}

func (t *Transitioner) readFinal(document host.AuthDocument, authIndex string) (credentialDocumentState, error) {
	// A successful replacement is the commit point. Verification is therefore
	// intentionally detached from caller cancellation so a post-commit cancel
	// cannot turn a committed Host change into a failed outcome.
	verifyCtx := context.WithoutCancel(context.Background())
	latest, err := t.store.GetAuth(verifyCtx, authIndex)
	if err != nil {
		return credentialDocumentState{}, err
	}
	state, _, err := decodeCredentialDocument(latest)
	return state, err
}

func normalizeIntent(intent TransitionIntent) TransitionIntent {
	intent.AuthIndex = strings.TrimSpace(firstNonEmpty(intent.AuthIndex, intent.Expected.AuthIndex))
	intent.Name = strings.TrimSpace(firstNonEmpty(intent.Name, intent.Expected.Name))
	intent.Expected.AuthIndex = strings.TrimSpace(firstNonEmpty(intent.Expected.AuthIndex, intent.AuthIndex))
	intent.Expected.Name = strings.TrimSpace(firstNonEmpty(intent.Expected.Name, intent.Name))
	if intent.Target.Priority.Operation == "" {
		intent.Target.Priority.Operation = FieldUnchanged
	}
	if intent.Target.Disabled.Operation == "" {
		intent.Target.Disabled.Operation = FieldUnchanged
	}
	return intent
}

func newTransitionResult(intent TransitionIntent) TransitionResult {
	return TransitionResult{
		Name:      redactIdentifier(intent.Name),
		AuthIndex: redactIdentifier(intent.AuthIndex),
		Cause:     redactString(intent.Cause),
		authIndex: intent.AuthIndex,
	}
}

func (r TransitionResult) withOutcome(outcome Outcome, reason string, err error) TransitionResult {
	r.Outcome = outcome
	r.Reason = reason
	if err != nil {
		r.Error = redactedError(err)
	}
	return r
}

func (t *TransitionTotals) add(outcome Outcome) {
	t.Attempted++
	switch outcome {
	case OutcomeNoChange:
		t.NoChange++
	case OutcomeCommitted:
		t.Committed++
	case OutcomeFailed:
		t.Failed++
	case OutcomeConflict:
		t.Conflicts++
	case OutcomeUncertain:
		t.Uncertain++
	}
}

type credentialDocumentState struct {
	AuthIndex       string
	Name            string
	Priority        int
	PriorityPresent bool
	Disabled        bool
	DisabledKnown   bool
}

func (s credentialDocumentState) redacted() CredentialState {
	return CredentialState{
		Priority:        s.Priority,
		PriorityPresent: s.PriorityPresent,
		Disabled:        s.Disabled,
		DisabledKnown:   s.DisabledKnown,
	}
}

func (s credentialDocumentState) sameDecisionState(other credentialDocumentState) bool {
	return s.Priority == other.Priority && s.PriorityPresent == other.PriorityPresent && s.Disabled == other.Disabled
}

func (s credentialDocumentState) matches(other credentialDocumentState) bool {
	return s.Priority == other.Priority && s.PriorityPresent == other.PriorityPresent &&
		s.Disabled == other.Disabled && s.DisabledKnown == other.DisabledKnown
}

func decodeCredentialDocument(document host.AuthDocument) (credentialDocumentState, map[string]json.RawMessage, error) {
	if len(strings.TrimSpace(string(document.JSON))) == 0 {
		return credentialDocumentState{}, nil, errors.New("credential document is empty")
	}
	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(document.JSON, &fields); err != nil {
		return credentialDocumentState{}, nil, fmt.Errorf("decode credential document: %w", err)
	}
	state := credentialDocumentState{AuthIndex: strings.TrimSpace(document.AuthIndex), Name: strings.TrimSpace(document.Name)}
	if value, ok := fields[host.FieldAuthIndex]; ok {
		var authIndex string
		if err := json.Unmarshal(value, &authIndex); err != nil {
			return credentialDocumentState{}, nil, fmt.Errorf("decode credential auth index: %w", err)
		}
		if strings.TrimSpace(authIndex) != "" {
			state.AuthIndex = strings.TrimSpace(authIndex)
		}
	}
	if value, ok := fields[host.FieldName]; ok {
		var name string
		if err := json.Unmarshal(value, &name); err != nil {
			return credentialDocumentState{}, nil, fmt.Errorf("decode credential name: %w", err)
		}
		if state.Name == "" {
			state.Name = strings.TrimSpace(name)
		}
	}
	if value, ok := fields[host.FieldPriority]; ok {
		if string(value) != "null" {
			if err := json.Unmarshal(value, &state.Priority); err != nil {
				return credentialDocumentState{}, nil, fmt.Errorf("decode credential priority: %w", err)
			}
			state.PriorityPresent = true
		}
	}
	if value, ok := fields[host.FieldDisabled]; ok {
		if err := json.Unmarshal(value, &state.Disabled); err != nil {
			return credentialDocumentState{}, nil, fmt.Errorf("decode credential disabled: %w", err)
		}
		state.DisabledKnown = true
	}
	return state, fields, nil
}

func expectedConflict(expected CredentialState, actual credentialDocumentState) string {
	if expected.AuthIndex != "" {
		if actual.AuthIndex == "" || expected.AuthIndex != actual.AuthIndex {
			return ReasonIdentityConflict
		}
	}
	if expected.PriorityPresent != actual.PriorityPresent || (expected.PriorityPresent && expected.Priority != actual.Priority) {
		return ReasonPriorityConflict
	}
	if expected.DisabledKnown && expected.Disabled != actual.Disabled {
		return ReasonDisabledConflict
	}
	return ""
}

func identityMatches(intent TransitionIntent, actual credentialDocumentState) bool {
	return intent.AuthIndex == "" || (actual.AuthIndex != "" && intent.AuthIndex == actual.AuthIndex)
}

func applyTarget(current credentialDocumentState, fields map[string]json.RawMessage, target CredentialTarget) (credentialDocumentState, bool) {
	result := current
	changed := false
	switch target.Priority.Operation {
	case FieldSet:
		if !current.PriorityPresent || current.Priority != target.Priority.Value {
			encoded, _ := json.Marshal(target.Priority.Value)
			fields[host.FieldPriority] = encoded
			result.Priority = target.Priority.Value
			result.PriorityPresent = true
			changed = true
		}
	case FieldUnset:
		if current.PriorityPresent {
			delete(fields, host.FieldPriority)
			result.Priority = 0
			result.PriorityPresent = false
			changed = true
		}
	}
	switch target.Disabled.Operation {
	case FieldSet:
		if !current.DisabledKnown || current.Disabled != target.Disabled.Value {
			encoded, _ := json.Marshal(target.Disabled.Value)
			fields[host.FieldDisabled] = encoded
			result.Disabled = target.Disabled.Value
			result.DisabledKnown = true
			changed = true
		}
	case FieldUnset:
		if current.DisabledKnown {
			delete(fields, host.FieldDisabled)
			result.Disabled = false
			result.DisabledKnown = false
			changed = true
		}
	}
	return result, changed
}

func reasonForReadError(ctx context.Context, err error) string {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ReasonContextCanceled
	}
	return ReasonReadFailed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func redactIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}
