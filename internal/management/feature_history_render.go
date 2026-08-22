package management

const templateScriptHistory = `        function formatHistoryKind(kind) {
            var k = (kind || "").toLowerCase();
            if (k === "apply" || k === "auto_apply" || k === "manual_apply") return currentLang === "zh-CN" ? "立即写回" : "APPLY";
            if (k === "probe") return currentLang === "zh-CN" ? "配额探测" : "PROBE";
            if (k === "reset") return currentLang === "zh-CN" ? "重置优先级" : "RESET";
            return (kind || "RUN").toUpperCase();
        }

        function renderHistory() {
            const list = document.getElementById("historyList");
            if (!list) return;

            const entries = (latestDiagnostics && latestDiagnostics.run_history) || [];
            list.innerHTML = "";
            if (entries.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noHistory") + "</div>";
                return;
            }

            entries.forEach(function(entry, idx) {
                const item = document.createElement("div");
                item.className = "history-item";
                const dateStr = entry.at ? new Date(entry.at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                const kindText = formatHistoryKind(entry.kind);
                const msg = entry.message || "";
                const hasSnapshot = entry.snapshot && (entry.snapshot.items || entry.snapshot.changes);

                const succText = (currentLang === "zh-CN" ? "成功: " : "Succeeded: ") + (entry.succeeded || 0);
                const failText = (currentLang === "zh-CN" ? "失败: " : "Failed: ") + (entry.failed || 0);
                const skipText = (currentLang === "zh-CN" ? "跳过: " : "Skipped: ") + (entry.skipped || 0);

                item.innerHTML =
                    "<div class=\"history-head\">" +
                        "<div style=\"display:flex; align-items:center; gap:8px;\">" +
                            "<span class=\"badge badge-subtle\">" + escapeHTML(kindText) + "</span>" +
                            "<span style=\"font-size:13px; font-weight:600;\">" + escapeHTML(dateStr) + "</span>" +
                        "</div>" +
                        "<div class=\"history-stats\">" +
                            "<span class=\"badge badge-success\">" + succText + "</span>" +
                            "<span class=\"badge badge-danger\">" + failText + "</span>" +
                            "<span class=\"badge badge-subtle\">" + skipText + "</span>" +
                            (hasSnapshot ? "<button type=\"button\" class=\"btn-secondary\" style=\"min-height:24px; height:24px; padding:0 8px; font-size:11px;\" onclick=\"showHistoryDetails(" + idx + ")\">" + t("viewDetails") + "</button>" : "") +
                        "</div>" +
                    "</div>" +
                    (msg ? "<div style=\"font-size:11.5px; color:var(--text-muted); font-family:monospace; line-height:1.3;\">" + escapeHTML(msg) + "</div>" : "");
                list.appendChild(item);
            });
        }

`
