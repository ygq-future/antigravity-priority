# 0005: Atomic Host Transitions and Truthful Outcomes

All target fields for one CPA Host credential form a single Host Transition, committed through one verified file replacement rather than independent priority and disabled writes. Scheduled Apply, 429 Reactive Cooldown, and priority reset share this lifecycle; outcomes describe the resulting Host state, credential failures remain local, later scheduled rounds provide compensation without same-round retries, and execution history is derived from transition facts rather than hand-maintained counters.

## Consequences

- The current Host state wins when a stale intent conflicts with an external priority or disabled change.
- Reset removes the priority field while preserving disabled state; 429 Cooldown persists even when its Host demotion fails.
- Exact crash recovery, write-ahead journals, replay protocols, and additional persistence schemas are deliberately excluded until a demonstrated requirement justifies them.
