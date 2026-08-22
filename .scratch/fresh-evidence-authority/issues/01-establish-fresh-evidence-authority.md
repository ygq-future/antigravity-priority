# 01: Establish the Fresh Evidence authority

**What to build:** Introduce one in-process Evidence module interface that classifies a scheduling round into quota evidence eligible for planning and observations that are read-only. Through that interface and Planner, prove that only a verified current-round success can authorize a quota-driven Change, while failed or historical observations preserve the credential's current Host target.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] A verified successful observation from the current scheduling round is eligible for quota planning.
- [ ] Current failures, historical failures, historical successes, incomplete observations, and wrong-group observations are never scheduling-eligible.
- [ ] Historical observations retain their real observation time and remain available separately for display or prediction.
- [ ] A credential without Fresh Evidence keeps its current priority and disabled state and produces no quota-driven Change.
- [ ] A mixed input can still produce Changes for credentials with Fresh Evidence while leaving other credentials unchanged.
- [ ] One table-driven contract suite exercises the Evidence module interface without asserting on private helpers or caller-assembled freshness flags.
