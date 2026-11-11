package antigravity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"antigravity-priority/internal/core"
)

const (
	// LoadCodeAssistDailyURL is the Antigravity CLI host used for subscription
	// entitlement. The production host can report free-tier for individual paid
	// accounts, so daily is the authoritative source for plan tier discovery.
	LoadCodeAssistDailyURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
)

type loadCodeAssistResponse struct {
	CurrentTier *tierDescriptor `json:"currentTier"`
	PaidTier    *tierDescriptor `json:"paidTier"`
}

type tierDescriptor struct {
	ID string `json:"id"`
}

// ParseLoadCodeAssist resolves the effective Antigravity subscription tier.
//
// paidTier is authoritative: consumer accounts often keep currentTier at
// free-tier even while paying. allowedTiers is deliberately ignored because it
// describes eligibility, not the entitlement currently applied to the account.
// Non-free identifiers without a fixed mapping are preserved as-is so a future
// tier is displayed rather than silently mislabeled.
func ParseLoadCodeAssist(raw []byte) (core.PlanType, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return core.PlanTypeUnknown, errors.New("loadCodeAssist response is empty")
	}

	var response loadCodeAssistResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return core.PlanTypeUnknown, fmt.Errorf("decode loadCodeAssist response: %w", err)
	}

	if paidID := tierID(response.PaidTier); paidID != "" {
		return planTypeForTierID(paidID), nil
	}
	if currentID := tierID(response.CurrentTier); currentID != "" {
		return planTypeForTierID(currentID), nil
	}

	return core.PlanTypeUnknown, errors.New("loadCodeAssist response has no tier")
}

func planTypeForTierID(id string) core.PlanType {
	switch id {
	case "free-tier":
		return core.PlanTypeFree
	case "g1-pro-tier":
		return core.PlanTypePro
	case "g1-ultra-tier":
		return core.PlanTypeUltra
	case "g1-ultra-lite-tier":
		return core.PlanTypeUltraLite
	case "standard-tier":
		return core.PlanTypeTeam
	default:
		return core.PlanType(id)
	}
}

func tierID(tier *tierDescriptor) string {
	if tier == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(tier.ID))
}
