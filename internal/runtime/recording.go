package runtime

import (
	"fmt"

	"antigravity-priority/internal/apply"
)
func withTransitionStats(result apply.Result, entry RunHistoryEntry) RunHistoryEntry {
	entry.Attempted = result.Attempted
	entry.Succeeded = result.Succeeded
	entry.Failed = result.Failed
	entry.Skipped = result.Skipped
	entry.NoChange = result.NoChange
	entry.Conflicts = result.Conflicts
	entry.Uncertain = result.Uncertain
	return entry
}

func resultSummary(prefix string, result apply.Result) string {
	return fmt.Sprintf("%s attempted=%d committed=%d no_change=%d failed=%d conflict=%d uncertain=%d skipped=%d",
		prefix,
		result.Transitions.Totals.Attempted,
		result.Transitions.Totals.Committed,
		result.Transitions.Totals.NoChange,
		result.Transitions.Totals.Failed,
		result.Transitions.Totals.Conflicts,
		result.Transitions.Totals.Uncertain,
		result.Skipped)
}
