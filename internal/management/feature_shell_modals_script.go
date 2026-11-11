package management

const templateScriptModals = `        function historyIdentity(value) {
            var email = value && value.email ? String(value.email).trim() : "";
            if (email) return "email:" + email.toLowerCase();
            var authIndex = value && value.auth_index ? String(value.auth_index).trim() : "";
            return authIndex ? "auth:" + authIndex : "unknown";
        }

        function historyQuotaMeter(value) {
            var parsed = Number(value);
            if (!Number.isFinite(parsed)) return "";
            var percent = Math.max(0, Math.min(100, Math.trunc(parsed)));
            return "<div class=\"history-quota-meter\"><span>" + percent + "%</span><div class=\"history-meter-track\"><div class=\"history-meter-fill\" style=\"width:" + percent + "%\"></div></div></div>";
        }

        function historyQuotaCell(sample) {
            if (!sample) return "<span class=\"history-empty-cell\"></span>";
            var meters = [];
            if (sample.short_window_rem !== undefined && sample.short_window_rem !== null) {
                meters.push("<div><b>5h</b>" + historyQuotaMeter(sample.short_window_rem) + "</div>");
            }
            if (sample.long_window_rem !== undefined && sample.long_window_rem !== null) {
                meters.push("<div><b>7d</b>" + historyQuotaMeter(sample.long_window_rem) + "</div>");
            }
            return meters.length ? "<div class=\"history-quota-values\">" + meters.join("") + "</div>" : "<span class=\"history-empty-cell\"></span>";
        }

        function renderHistoryComparison(list, result) {
            var groups = result && result.groups && typeof result.groups === "object" ? result.groups : {};
            var rows = {};
            var quotaRecordCount = 0;
            function ensure(value) {
                var key = historyIdentity(value);
                if (!rows[key]) rows[key] = {email: value && value.email ? String(value.email) : "", gemini: null, argon: null, claude_gpt: null};
                if (!rows[key].email && value && value.email) rows[key].email = String(value.email);
                return rows[key];
            }
            ["gemini", "argon", "claude_gpt"].forEach(function(modelGroup) {
                (Array.isArray(groups[modelGroup]) ? groups[modelGroup] : []).forEach(function(record) {
                    var row = ensure(record);
                    var sample = record.sample || {};
                    var existing = row[modelGroup];
                    if (!existing || String(sample.observed_at || "") > String(existing.observed_at || "")) row[modelGroup] = sample;
                    quotaRecordCount++;
                });
            });

            var rowKeys = Object.keys(rows).sort(function(left, right) {
                return String(rows[left].email || left).localeCompare(String(rows[right].email || right));
            });
            list.innerHTML = "";
            if (rowKeys.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + (currentLang === "zh-CN" ? "本轮没有额度样本" : "No quota samples in this run") + "</div>";
                return {quotaRecordCount: quotaRecordCount, rowCount: 0};
            }

            var html = "<div class=\"history-comparison-table history-comparison-quota-only\">" +
                "<div class=\"history-comparison-row history-comparison-header\">" +
                    "<div>" + (currentLang === "zh-CN" ? "账号" : "Account") + "</div>" +
                    "<div>" + (currentLang === "zh-CN" ? "本次探测 Gemini 额度" : "Gemini Quota This Probe") + "</div>" +
                    "<div>" + (currentLang === "zh-CN" ? "本次探测 Argon 额度" : "Argon Quota This Probe") + "</div>" +
                    "<div>" + (currentLang === "zh-CN" ? "本次探测 Claude/GPT 额度" : "Claude/GPT Quota This Probe") + "</div>" +
                "</div>";
            rowKeys.forEach(function(key) {
                var row = rows[key];
                html += "<div class=\"history-comparison-row\">" +
                    "<div class=\"history-account-cell\">" + escapeHTML(row.email || "Credential") + "</div>" +
                    "<div>" + historyQuotaCell(row.gemini) + "</div>" +
                    "<div>" + historyQuotaCell(row.argon) + "</div>" +
                    "<div>" + historyQuotaCell(row.claude_gpt) + "</div>" +
                "</div>";
            });
            list.innerHTML = html + "</div>";
            return {quotaRecordCount: quotaRecordCount, rowCount: rowKeys.length};
        }

        // Show Modal: Probe/Auto History vs Execution Details Snapshot
        function showModal(mode, result) {
            const modal = document.getElementById("diffModal");
            const modalPanel = document.getElementById("diffModalPanel");
            const title = document.getElementById("modalTitle");
            const summary = document.getElementById("modalSummary");
            const list = document.getElementById("modalDiffList");
			const isProbeHistory = (mode === "probe-history");
			const isAutoHistory = (mode === "auto-history");
			if (modalPanel) modalPanel.classList.toggle("modal-history-comparison", isProbeHistory || isAutoHistory);

			if (isProbeHistory || isAutoHistory) {
				title.textContent = isAutoHistory
					? (currentLang === "zh-CN" ? "自动调度明细" : "Automatic Schedule Details")
					: (currentLang === "zh-CN" ? "配额探测明细" : "Quota Probe Details");
                const groups = result && result.groups && typeof result.groups === "object" ? result.groups : {};
				const comparison = renderHistoryComparison(list, {groups: groups});
				summary.textContent = (currentLang === "zh-CN" ? "本轮新增配额样本: " : "New quota samples in this round: ") + comparison.quotaRecordCount;
				modal.hidden = false;
                return;
            }

            const changes = extractChanges(result);
            title.textContent = currentLang === "zh-CN" ? "执行明细快照" : "Execution Details Snapshot";
            summary.textContent = (currentLang === "zh-CN" ? "成功: " : "Succeeded: ") + (result.succeeded || 0) + ", " +
                (currentLang === "zh-CN" ? "失败: " : "Failed: ") + (result.failed || 0) + ", " +
                (currentLang === "zh-CN" ? "跳过: " : "Skipped: ") + (result.skipped || 0);

            list.innerHTML = "";
            if (changes.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noChanges") + "</div>";
            } else {
                changes.forEach(c => {
                    const row = document.createElement("div");
                    row.className = "diff-card";
                    let fromP = "-";
                    if (c.host && c.host.priority !== undefined && c.host.priority !== null) {
                        fromP = c.host.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.host.priority_missing ? t("unsetPriority") : c.host.priority);
                    } else if (c.priority_from !== undefined && c.priority_from !== null) {
                        fromP = c.disabled_from ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.priority_missing ? t("unsetPriority") : c.priority_from);
                    }

                    let toP = "-";
                    if (c.target && c.target.priority !== undefined && c.target.priority !== null) {
                        toP = c.target.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.target.priority_missing ? t("unsetPriority") : c.target.priority);
                    } else if (c.priority_to !== undefined && c.priority_to !== null) {
                        toP = c.disabled_to ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.priority_missing_to ? t("unsetPriority") : c.priority_to);
                    } else if (c.priority !== undefined && c.priority !== null) {
                        toP = (c.disabled || (c.target && c.target.disabled)) ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.priority;
                    }

                    const name = c.email || "Credential";
                    const isBoost = Boolean(c.is_boosted || (c.reason && c.reason.indexOf("boost") >= 0));
                    const reasonText = formatReason(c.reason, isBoost, c.disabled_to || (c.target && c.target.disabled));

                    row.innerHTML = "<div>" +
                        "<div style=\"font-weight:700; font-size:13px; display:flex; align-items:center; gap:6px;\">" +
                            "<span>" + escapeHTML(name) + "</span>" +
                            (isBoost ? "<span class=\"badge badge-boost\">🚀 Boost</span>" : "") +
                        "</div>" +
                        "<div style=\"font-size:12px; color:var(--text-muted);\">" + escapeHTML(reasonText) + "</div>" +
                    "</div>" +
                    "<div class=\"diff-value-box\">" +
                        "<span class=\"diff-from\">" + fromP + "</span>" +
                        "<span>&rarr;</span>" +
                        "<span class=\"diff-to\">" + toP + "</span>" +
                    "</div>";
                    list.appendChild(row);
                });
            }
            modal.hidden = false;
        }

        function closeModal() {
            document.getElementById("diffModal").hidden = true;
        }

        function closeModalFromBackdrop(event) {
            if (event && event.target === event.currentTarget) closeModal();
        }

`
