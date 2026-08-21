package management

const templateScriptHistoryDetails = `        function showHistoryDetails(index) {
            if (!latestDiagnostics || !latestDiagnostics.run_history) return;
            var entry = latestDiagnostics.run_history[index];
            if (!entry) return;

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
