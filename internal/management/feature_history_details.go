package management

const templateScriptHistoryDetails = `        function formatHistoryKind(entry) {
            var kind = entry && entry.kind ? entry.kind : "";
            var trigger = entry && entry.trigger ? entry.trigger : "";
            var k = kind.toLowerCase();
            if (k === "auto_apply" || trigger.toLowerCase() === "auto_apply") return currentLang === "zh-CN" ? "自动调度" : "AUTO SCHEDULE";
            if (k === "probe") return currentLang === "zh-CN" ? "配额探测" : "PROBE";
            if (k === "reset") return currentLang === "zh-CN" ? "重置优先级" : "RESET";
            return (kind || "RUN").toUpperCase();
        }

        var historyDetailRequestId = 0;

        async function showHistoryDetails(index) {
            if (!latestDiagnostics || !latestDiagnostics.run_history) return;
            var entry = latestDiagnostics.run_history[index];
            if (!entry) return;
            var isAutoSchedule = entry.kind === "auto_apply" || entry.trigger === "auto_apply";
            var isProbeOrAuto = entry.kind === "probe" || isAutoSchedule;
            if (isProbeOrAuto && !entry.probe_round_id) {
                showToast(currentLang === "zh-CN" ? "该探测记录没有可追溯的新样本" : "This probe record has no attributable new samples", "info");
                return;
            }
            var requestId = ++historyDetailRequestId;
            var loadingMode = isAutoSchedule ? "auto-history" : (entry.kind === "probe" ? "probe-history" : "history");
            if (isProbeOrAuto || entry.has_snapshot || (entry.snapshot && (entry.snapshot.items || entry.snapshot.changes))) {
                showModal(loadingMode, { loading: true });
            }
            if (!entry.snapshot && entry.has_snapshot && entry.at) {
                try {
                    var detailResp = await apiFetch(RUN_HISTORY_DETAIL_PATH + "?at=" + encodeURIComponent(entry.at));
                    if (requestId !== historyDetailRequestId || document.getElementById("diffModal").hidden) return;
                    if (detailResp && detailResp.snapshot) {
                        entry.snapshot = detailResp.snapshot;
                    }
                } catch (err) {
                    if (requestId === historyDetailRequestId) closeModal();
                    showToast(err.message || (currentLang === "zh-CN" ? "读取快照详情失败" : "Failed to load snapshot details"), "error");
                    return;
                }
            }

            if (isProbeOrAuto) {
                const probeModelGroups = ["gemini", "claude_gpt"];
                Promise.all(probeModelGroups.map(function(modelGroup) {
                    var probePath = SAMPLES_PATH + "?probe_round_id=" + encodeURIComponent(entry.probe_round_id) + "&model_group=" + encodeURIComponent(modelGroup);
                    return apiFetch(probePath).then(function(data) {
                        return {
                            model_group: modelGroup,
                            records: Array.isArray(data && data.records) ? data.records : []
                        };
                    });
                })).then(function(groupResults) {
                    if (requestId !== historyDetailRequestId || document.getElementById("diffModal").hidden) return;
                    var groups = {};
                    groupResults.forEach(function(groupResult) {
                        groups[groupResult.model_group] = groupResult.records;
                    });
					var detailResult = {
                        groups: groups,
						probe_round_id: entry.probe_round_id,
						snapshot: entry.snapshot,
						items: entry.snapshot && entry.snapshot.items ? entry.snapshot.items : [],
						changes: entry.snapshot && entry.snapshot.changes ? entry.snapshot.changes : [],
						attempted: entry.attempted || 0,
						succeeded: entry.succeeded || 0,
						failed: entry.failed || 0,
						skipped: entry.skipped || 0
					};
					if (isAutoSchedule) {
						showModal("auto-history", detailResult);
					} else {
						showModal("probe-history", detailResult);
					}
                }).catch(function(err) {
                    if (requestId === historyDetailRequestId) closeModal();
                    showToast(err.message || (currentLang === "zh-CN" ? "读取探测明细失败" : "Failed to load probe details"), "error");
                });
                return;
            }

            var snap = entry.snapshot;
            if (!snap || (!snap.items && !snap.changes)) {
                showToast(currentLang === "zh-CN" ? "该记录无详细快照数据" : "No snapshot data for this entry", "info");
                return;
            }

            var result = {
                snapshot: snap,
                items: snap.items || [],
                changes: snap.changes || [],
                attempted: entry.attempted || 0,
                succeeded: entry.succeeded || 0,
                failed: entry.failed || 0,
                skipped: entry.skipped || 0
            };

            // Strictly read-only for historical detail modal
            showModal("history", result);
        }

`
