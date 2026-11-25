package management

const templateScriptHistory = `        function renderHistory() {
            if (!isRequestErrorTooltipPinned()) {
                hideRequestErrorTooltip();
            }
            const list = document.getElementById("historyList");
            if (!list) return;

            const records = (latestDiagnostics && latestDiagnostics.request_history) || [];
            const badge = document.getElementById("requestHistoryCountBadge");
            if (badge) {
                badge.textContent = records.length + (currentLang === "zh-CN" ? " 条记录" : " records");
            }
            const limitInput = document.getElementById("historyLimitInput");
            if (limitInput && latestDiagnostics && latestDiagnostics.request_history_limit !== undefined && document.activeElement !== limitInput) {
                limitInput.value = latestDiagnostics.request_history_limit;
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
                if (pool === "claude_gpt") {
                    modelClass = "req-model-claude";
                    poolLabel = "Claude";
                }

                const inTok = (r.input_tokens || 0).toLocaleString();
                const outTok = (r.output_tokens || 0).toLocaleString();
                const totalTok = (r.total_tokens || 0).toLocaleString();
                const reasonTok = (r.reasoning_tokens || 0).toLocaleString();
                const cacheTok = (r.cache_read_tokens || r.cached_tokens || 0).toLocaleString();

                const tokenLabels = currentLang === "zh-CN"
                    ? { input: "输入", output: "输出", think: "思考", cache: "缓存", total: "总计" }
                    : { input: "in", output: "out", think: "think", cache: "cache", total: "total" };
                let tokenBreakdown = inTok + " " + tokenLabels.input + " / " + outTok + " " + tokenLabels.output;
                if (r.reasoning_tokens > 0) {
                    tokenBreakdown += " / " + reasonTok + " " + tokenLabels.think;
                }
                if (r.cache_read_tokens > 0 || r.cached_tokens > 0) {
                    tokenBreakdown += " / " + cacheTok + " " + tokenLabels.cache;
                }

                let statusBadge = "<span class=\"req-status-ok\">● 200 OK</span>";
                if (r.failed) {
                    const isRequestScoped429 = r.status_code === 429 && r.failure_scope === "request_scoped";
                    const statusClass = isRequestScoped429 ? "req-status-exempt" : (r.status_code === 429 ? "req-status-429" : "req-status-fail");
                    const statusText = isRequestScoped429
                        ? "● 429 " + t("statusRequestScoped429")
                        : (r.status_code === 429
                            ? "● 429 " + t("statusRateLimited")
                            : "● " + (r.status_code || 500) + " " + t("statusError"));
                    const errorDetail = r.error_message || ("HTTP " + (r.status_code || 500));
                    statusBadge = "<span class=\"req-status-cell\">" +
                        "<span class=\"" + statusClass + "\">" + statusText + "</span>" +
                        "<span class=\"req-error-info\" tabindex=\"0\" role=\"button\" aria-label=\"" + escapeHTML(t("historyErrorDetails")) + "\" data-error=\"" + escapeHTML(errorDetail) + "\">i</span>" +
                        "</span>";
                }

                html += "<tr>" +
                    "<td class=\"req-time\" title=\"" + escapeHTML(fullDateStr) + "\">" + escapeHTML(timeStr) + "</td>" +
                    "<td><span class=\"req-model-badge " + modelClass + "\">[" + poolLabel + "] " + escapeHTML(r.model || "-") + "</span></td>" +
                    "<td class=\"req-cred\">" + escapeHTML(formatAccountName(r.email || r.auth_index || "-")) + "</td>" +
                    "<td class=\"req-tokens\">" + tokenBreakdown + " <span class=\"req-tokens-total\">(" + totalTok + " " + tokenLabels.total + ")</span></td>" +
                    "<td>" + statusBadge + "</td>" +
                "</tr>";
            });

            html += "</tbody></table>";
            list.innerHTML = html;
        }

        async function saveRequestHistoryLimit() {
            const input = document.getElementById("historyLimitInput");
            if (!input) return;
            const limit = Number(input.value);
            if (!Number.isInteger(limit) || limit < 1 || limit > 1000) {
                showToast(t("valErrRequestHistoryLimit"), "error");
                input.focus();
                return;
            }
            try {
                const cfg = await apiFetch(CONFIG_PATH);
                cfg.request_history_limit = limit;
                dynamicConfig = await apiFetch(CONFIG_PATH, { method: "POST", body: JSON.stringify(cfg) });
                showToast(t("historyLimitSaved"), "success");
                await fetchDiagnostics({ silent: true });
            } catch (err) {
                showToast(err.message || t("valErrRequestHistoryLimit"), "error");
            }
        }

        let requestErrorTooltipTimer = null;
        let requestErrorTooltipHovered = false;
        let requestErrorTooltipSelecting = false;

        function clearRequestErrorTooltipTimer() {
            if (requestErrorTooltipTimer) {
                clearTimeout(requestErrorTooltipTimer);
                requestErrorTooltipTimer = null;
            }
        }

        function hasRequestErrorTooltipSelection() {
            const tip = document.getElementById("reqErrorTooltip");
            if (!tip || !window.getSelection) return false;
            const sel = window.getSelection();
            if (!sel || sel.isCollapsed || !sel.rangeCount) return false;
            return tip.contains(sel.anchorNode) || tip.contains(sel.focusNode);
        }

        function isRequestErrorTooltipPinned() {
            return requestErrorTooltipHovered || requestErrorTooltipSelecting || hasRequestErrorTooltipSelection();
        }

        function formatRequestErrorTooltipText(raw) {
            let text = String(raw || "").trim();
            if (!text) return "";
            if (text.indexOf("&#34;") >= 0 || text.indexOf("&quot;") >= 0 || text.indexOf("&#39;") >= 0 || text.indexOf("&lt;") >= 0 || text.indexOf("&gt;") >= 0 || text.indexOf("&amp;") >= 0) {
                text = text
                    .replace(/&#34;/g, "\"")
                    .replace(/&quot;/g, "\"")
                    .replace(/&#39;/g, "'")
                    .replace(/&lt;/g, "<")
                    .replace(/&gt;/g, ">")
                    .replace(/&amp;/g, "&");
            }
            if ((text[0] === "{" && text[text.length - 1] === "}") || (text[0] === "[" && text[text.length - 1] === "]")) {
                try {
                    return JSON.stringify(JSON.parse(text), null, 2);
                } catch (_) {}
            }
            return text;
        }

        function positionRequestErrorTooltip(target) {
            const tip = document.getElementById("reqErrorTooltip");
            if (!tip) return;
            const message = formatRequestErrorTooltipText(target.getAttribute("data-error") || "");
            if (!message) return;
            clearRequestErrorTooltipTimer();
            tip.textContent = message;
            const rect = target.getBoundingClientRect();
            const tipRect = tip.getBoundingClientRect();
            const margin = 8;
            let left = rect.left + rect.width / 2 - tipRect.width / 2;
            left = Math.max(margin, Math.min(left, window.innerWidth - tipRect.width - margin));
            let top = rect.bottom + 6;
            let placement = "bottom";
            if (top + tipRect.height > window.innerHeight - margin) {
                placement = "top";
                top = rect.top - tipRect.height - 6;
            }
            tip.dataset.placement = placement;
            tip.style.left = left + "px";
            tip.style.top = Math.max(margin, top) + "px";
            tip.classList.add("visible");
        }

        function scheduleHideRequestErrorTooltip() {
            if (isRequestErrorTooltipPinned()) return;
            clearRequestErrorTooltipTimer();
            requestErrorTooltipTimer = setTimeout(function() {
                if (!isRequestErrorTooltipPinned()) {
                    hideRequestErrorTooltip();
                }
            }, 300);
        }

        function hideRequestErrorTooltip() {
            clearRequestErrorTooltipTimer();
            requestErrorTooltipHovered = false;
            requestErrorTooltipSelecting = false;
            const tip = document.getElementById("reqErrorTooltip");
            if (tip) tip.classList.remove("visible");
        }

        const requestErrorTooltipElement = document.getElementById("reqErrorTooltip");
        if (requestErrorTooltipElement) {
            requestErrorTooltipElement.addEventListener("mouseenter", function() {
                requestErrorTooltipHovered = true;
                clearRequestErrorTooltipTimer();
            });
            requestErrorTooltipElement.addEventListener("mouseleave", function() {
                requestErrorTooltipHovered = false;
                scheduleHideRequestErrorTooltip();
            });
            requestErrorTooltipElement.addEventListener("mousedown", function() {
                requestErrorTooltipSelecting = true;
                clearRequestErrorTooltipTimer();
            });
            requestErrorTooltipElement.addEventListener("focusin", clearRequestErrorTooltipTimer);
            requestErrorTooltipElement.addEventListener("focusout", scheduleHideRequestErrorTooltip);
        }

        document.addEventListener("mouseup", function() {
            if (requestErrorTooltipSelecting) {
                requestErrorTooltipSelecting = false;
            }
        });
        document.addEventListener("mousedown", function(e) {
            const tip = document.getElementById("reqErrorTooltip");
            if (!tip || !tip.classList.contains("visible")) return;
            if (tip.contains(e.target)) return;
            if (e.target && e.target.closest && e.target.closest(".req-error-info")) return;
            hideRequestErrorTooltip();
        });
        document.addEventListener("mouseover", function(e) {
            const target = e.target && e.target.closest ? e.target.closest(".req-error-info") : null;
            if (target) positionRequestErrorTooltip(target);
        });
        document.addEventListener("mouseout", function(e) {
            const target = e.target && e.target.closest ? e.target.closest(".req-error-info") : null;
            if (target) scheduleHideRequestErrorTooltip();
        });
        document.addEventListener("focusin", function(e) {
            const target = e.target && e.target.closest ? e.target.closest(".req-error-info") : null;
            if (target) positionRequestErrorTooltip(target);
        });
        document.addEventListener("focusout", function(e) {
            const target = e.target && e.target.closest ? e.target.closest(".req-error-info") : null;
            if (target) scheduleHideRequestErrorTooltip();
        });
        window.addEventListener("scroll", function(e) {
            const tip = document.getElementById("reqErrorTooltip");
            if (tip && e && e.target && (e.target === tip || tip.contains(e.target))) {
                return;
            }
            if (!isRequestErrorTooltipPinned()) {
                hideRequestErrorTooltip();
            }
        }, true);
        window.addEventListener("resize", hideRequestErrorTooltip);

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
