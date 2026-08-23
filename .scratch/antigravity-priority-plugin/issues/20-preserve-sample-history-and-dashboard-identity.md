# 20 — Preserve Sample History and Dashboard Identity

**Status:** completed

**What to fix:** Keep quota observations as a bounded history for both adaptive learning and the samples panel, and expose the CPA Host credential email and auth index in the authenticated management dashboard.

## Confirmed Semantics
- Each credential/model-group entry has one `samples` FIFO history bounded by `quota_sample_capacity`.
- Every distinct quota/reset observation receives a monotonically increasing sequence number.
- A successful probe matching the latest quota and reset values updates only that sample's observation time.
- Adaptive learning records a `learning_baseline_sequence`; learning advances this cursor without deleting history.
- A short-window reset or replenishment preserves history and advances the learning baseline to the current sample.
- If FIFO rotation evicts the recorded baseline, estimation resumes from the oldest retained sample.
- Dashboard cards display the full CPA Host `email` and full `authIndex`; runtime identity and sample lookup continue to use `authIndex`.
- Persisted audit payloads remain redacted and do not gain raw credential identifiers.

## Compatibility
- Existing cache entries without sample sequences or a learning baseline are normalized on load.
- Existing quota history remains readable, with deterministic sequence assignment from oldest to newest.

## Acceptance Criteria
- [x] Learning and window resets no longer clear `samples`.
- [x] FIFO capacity remains the only sample-history eviction mechanism.
- [x] Unchanged probes refresh the latest timestamp without appending a duplicate.
- [x] Learning continues correctly after its baseline is rotated out.
- [x] The samples API receives the full `authIndex` shown by the management snapshot.
- [x] Dashboard cards show the full CPA Host email and auth index.
- [x] Audit serialization remains redacted.
- [x] Devserver exercises deduplication, quota changes, window rebasing, and FIFO rotation through the production estimator.
