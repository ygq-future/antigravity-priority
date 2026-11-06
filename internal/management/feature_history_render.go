package management

const templateScriptHistory = `        function renderHistory() {
            const list = document.getElementById("historyList");
            if (!list) return;

            const records = (latestDiagnostics && latestDiagnostics.request_history) || [];
            const badge = document.getElementById("requestHistoryCountBadge");
            if (badge) {
                badge.textContent = records.length + (currentLang === "zh-CN" ? " 条记录" : " records");
            }

            if (records.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noHistory") + "</div>";
                return;
            }

            let html = "<table class=\"request-history-table\">" +
                "<thead>" +
                    "<tr>" +
                        "<th>" + t("thTime") + "</th>" +
                        "<th>" + t("thModel") + "</th>" +
                        "<th>" + t("thCredential") + "</th>" +
                        "<th>" + t("thTokens") + "</th>" +
                        "<th>" + t("thStatus") + "</th>" +
                    "</tr>" +
                "</thead>" +
                "<tbody>";

            records.forEach(function(r) {
                const date = r.at ? new Date(r.at) : new Date();
                const timeStr = date.toLocaleTimeString(currentLang === "zh-CN" ? "zh-CN" : "en-US", { hour12: false }) + "." + String(date.getMilliseconds()).padStart(3, "0");
                const fullDateStr = date.toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US");

                const pool = r.model_pool || "";
                let modelClass = "req-model-gemini";
                let poolLabel = "Gemini";
                if (pool === "argon") {
                    modelClass = "req-model-argon";
                    poolLabel = "Argon";
                } else if (pool === "claude_gpt") {
                    modelClass = "req-model-claude";
                    poolLabel = "Claude";
                }

                const inTok = (r.input_tokens || 0).toLocaleString();
                const outTok = (r.output_tokens || 0).toLocaleString();
                const totalTok = (r.total_tokens || 0).toLocaleString();
                const reasonTok = (r.reasoning_tokens || 0).toLocaleString();
                const cacheTok = (r.cache_read_tokens || r.cached_tokens || 0).toLocaleString();

                let tokenBreakdown = inTok + " in / " + outTok + " out";
                if (r.reasoning_tokens > 0) {
                    tokenBreakdown += " / " + reasonTok + (currentLang === "zh-CN" ? " 思考" : " think");
                }
                if (r.cache_read_tokens > 0 || r.cached_tokens > 0) {
                    tokenBreakdown += " / " + cacheTok + (currentLang === "zh-CN" ? " 缓存" : " cache");
                }

                let statusBadge = "<span class=\"req-status-ok\">● 200 OK</span>";
                if (r.failed) {
                    if (r.status_code === 429) {
                        statusBadge = "<span class=\"req-status-429\" title=\"" + escapeHTML(r.error_message || "") + "\">● 429 " + t("statusRateLimited") + "</span>";
                    } else {
                        statusBadge = "<span class=\"req-status-fail\" title=\"" + escapeHTML(r.error_message || "") + "\">● " + (r.status_code || 500) + " " + t("statusError") + "</span>";
                    }
                }

                html += "<tr>" +
                    "<td class=\"req-time\" title=\"" + escapeHTML(fullDateStr) + "\">" + escapeHTML(timeStr) + "</td>" +
                    "<td><span class=\"req-model-badge " + modelClass + "\">[" + poolLabel + "] " + escapeHTML(r.model || "-") + "</span></td>" +
                    "<td class=\"req-cred\">" + escapeHTML(r.email || r.auth_index || "-") + "</td>" +
                    "<td class=\"req-tokens\">" + tokenBreakdown + " <span class=\"req-tokens-total\">(" + totalTok + ")</span></td>" +
                    "<td>" + statusBadge + "</td>" +
                "</tr>";
            });

            html += "</tbody></table>";
            list.innerHTML = html;
        }

        async function clearRequestHistoryAction() {
            if (!confirm(currentLang === "zh-CN" ? "确认清空所有已记录的请求历史吗？" : "Clear all recorded request history?")) {
                return;
            }
            try {
                await apiFetch(REQUEST_HISTORY_CLEAR_PATH, { method: "POST" });
                showToast(currentLang === "zh-CN" ? "已清空请求历史" : "Request history cleared");
                if (latestDiagnostics) {
                    latestDiagnostics.request_history = [];
                }
                renderHistory();
            } catch (err) {
                showToast(err.message || (currentLang === "zh-CN" ? "清空失败" : "Failed to clear"), "error");
            }
        }
`
