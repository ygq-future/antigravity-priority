package management

const templateScriptHistoryDetails = `        function showHistoryDetails(index) {
            if (!latestDiagnostics || !latestDiagnostics.run_history) return;
            var entry = latestDiagnostics.run_history[index];
            if (!entry) return;

            if (entry.kind === "probe") {
                if (!entry.probe_round_id) {
                    showToast(currentLang === "zh-CN" ? "该探测记录没有可追溯的新样本" : "This probe record has no attributable new samples", "info");
                    return;
                }
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
                    var groups = {};
                    groupResults.forEach(function(groupResult) {
                        groups[groupResult.model_group] = groupResult.records;
                    });
                    showModal("probe-history", {
                        groups: groups,
                        probe_round_id: entry.probe_round_id
                    }, true);
                }).catch(function(err) {
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
            showModal("history", result, true);
        }

`
