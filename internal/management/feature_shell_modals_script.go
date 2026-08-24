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

        function historyPriorityValue(target) {
            if (!target) return "";
            if (target.disabled) return currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]";
            if (target.priority_missing) return t("unsetPriority");
            return target.priority !== undefined && target.priority !== null ? String(target.priority) : "";
        }

        function historyPriorityCell(change) {
            if (!change) return "<span class=\"history-empty-cell\"></span>";
            var from = historyPriorityValue(change.current);
            var to = historyPriorityValue(change.target);
            if (!from && !to) return "<span class=\"history-empty-cell\"></span>";
            var reason = formatReason(change.reason, Boolean(change.is_boosted), Boolean(change.target && change.target.disabled));
            return "<div class=\"history-priority-value\"><span>" + escapeHTML(from || "-") + "</span><b>&rarr;</b><span>" + escapeHTML(to || "-") + "</span></div>" +
                (reason ? "<div class=\"history-priority-reason\">" + escapeHTML(reason) + "</div>" : "");
        }

        function renderHistoryComparison(list, result, includePriority) {
            var groups = result && result.groups && typeof result.groups === "object" ? result.groups : {};
            var changes = result && Array.isArray(result.changes) ? result.changes : [];
            var rows = {};
            var quotaRecordCount = 0;
            function ensure(value) {
                var key = historyIdentity(value);
                if (!rows[key]) rows[key] = {email: value && value.email ? String(value.email) : "", gemini: null, claude_gpt: null, change: null};
                if (!rows[key].email && value && value.email) rows[key].email = String(value.email);
                return rows[key];
            }
            ["gemini", "claude_gpt"].forEach(function(modelGroup) {
                (Array.isArray(groups[modelGroup]) ? groups[modelGroup] : []).forEach(function(record) {
                    var row = ensure(record);
                    var sample = record.sample || {};
                    var existing = row[modelGroup];
                    if (!existing || String(sample.observed_at || "") > String(existing.observed_at || "")) row[modelGroup] = sample;
                    quotaRecordCount++;
                });
            });
            changes.forEach(function(change) {
                var row = ensure(change);
                row.change = change;
            });

            var rowKeys = Object.keys(rows).sort(function(left, right) {
                return String(rows[left].email || left).localeCompare(String(rows[right].email || right));
            });
            list.innerHTML = "";
            if (rowKeys.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + (currentLang === "zh-CN" ? "本轮没有额度或优先级变化" : "No quota or priority changes in this run") + "</div>";
                return {quotaRecordCount: quotaRecordCount, rowCount: 0};
            }

            var tableClass = includePriority ? "history-comparison-table" : "history-comparison-table history-comparison-quota-only";
            var html = "<div class=\"" + tableClass + "\">" +
                "<div class=\"history-comparison-row history-comparison-header\">" +
                    "<div>" + (currentLang === "zh-CN" ? "账号" : "Account") + "</div>" +
                    "<div>" + (currentLang === "zh-CN" ? "本次探测 Gemini 额度" : "Gemini Quota This Probe") + "</div>" +
                    "<div>" + (currentLang === "zh-CN" ? "本次探测 Claude/GPT 额度" : "Claude/GPT Quota This Probe") + "</div>" +
                    (includePriority ? "<div>" + (currentLang === "zh-CN" ? "优先级变化" : "Priority Change") + "</div>" : "") +
                "</div>";
            rowKeys.forEach(function(key) {
                var row = rows[key];
                html += "<div class=\"history-comparison-row\">" +
                    "<div class=\"history-account-cell\">" + escapeHTML(row.email || "Credential") + "</div>" +
                    "<div>" + historyQuotaCell(row.gemini) + "</div>" +
                    "<div>" + historyQuotaCell(row.claude_gpt) + "</div>" +
                    (includePriority ? "<div>" + historyPriorityCell(row.change) + "</div>" : "") +
                "</div>";
            });
            list.innerHTML = html + "</div>";
            return {quotaRecordCount: quotaRecordCount, rowCount: rowKeys.length};
        }

        // Show Modal: Apply Confirm vs History Details Snapshot
        function showModal(mode, result, isFromHistory) {
            const modal = document.getElementById("diffModal");
            const modalPanel = document.getElementById("diffModalPanel");
            const title = document.getElementById("modalTitle");
            const summary = document.getElementById("modalSummary");
            const list = document.getElementById("modalDiffList");
            const btnApply = document.getElementById("btnModalApply");

			const isApplyConfirm = (mode === "apply-confirm");
			const isProbeHistory = (mode === "probe-history");
			const isAutoHistory = (mode === "auto-history");
			if (modalPanel) modalPanel.classList.toggle("modal-history-comparison", isProbeHistory || isAutoHistory);

			if (isProbeHistory || isAutoHistory) {
				title.textContent = isAutoHistory
					? (currentLang === "zh-CN" ? "自动调度明细" : "Automatic Schedule Details")
					: (currentLang === "zh-CN" ? "配额探测明细" : "Quota Probe Details");
                btnApply.hidden = true;
                const groups = result && result.groups && typeof result.groups === "object" ? result.groups : {};
				const historyChanges = isAutoHistory && result && Array.isArray(result.changes) ? result.changes : [];
				const comparison = renderHistoryComparison(list, {groups: groups, changes: historyChanges}, isAutoHistory);
				const summaryParts = [(currentLang === "zh-CN" ? "本轮新增额度样本: " : "New quota samples in this round: ") + comparison.quotaRecordCount];
				if (isAutoHistory && historyChanges.length > 0) summaryParts.push((currentLang === "zh-CN" ? "优先级变化: " : "Priority changes: ") + historyChanges.length);
				summary.textContent = summaryParts.join(" · ");
				modal.hidden = false;
                return;
            }

            const changes = isApplyConfirm && Array.isArray(result.changes) ? result.changes : extractChanges(result);
            const isProjectedPreview = isApplyConfirm && result.preview_mode === "projected";

            if (isApplyConfirm) {
                title.textContent = t("confirmApplyTitle");
                btnApply.hidden = false;
                btnApply.textContent = t("btnConfirmApply");
                btnApply.onclick = executeDirectApply;
                summary.textContent = isProjectedPreview
                    ? t("projectedApplyPreview") + ": " + changes.length + " · " + t("manualApplyRecheck")
                    : t("pendingApplyPreview") + ": " + changes.length;
            } else {
                title.textContent = currentLang === "zh-CN" ? "执行明细快照" : "Execution Details Snapshot";
                btnApply.hidden = true;
                summary.textContent = (currentLang === "zh-CN" ? "成功: " : "Succeeded: ") + (result.succeeded || 0) + ", " +
                    (currentLang === "zh-CN" ? "失败: " : "Failed: ") + (result.failed || 0) + ", " +
                    (currentLang === "zh-CN" ? "跳过: " : "Skipped: ") + (result.skipped || 0);
            }

            list.innerHTML = "";
            if (changes.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + (isApplyConfirm ? t("manualApplyRecheck") : t("noChanges")) + "</div>";
            } else {
                changes.forEach(c => {
                    const row = document.createElement("div");
                    row.className = "diff-card";
                    let fromP = "-";
                    if (c.current && c.current.priority !== undefined && c.current.priority !== null) {
                        fromP = c.current.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.current.priority_missing ? t("unsetPriority") : c.current.priority);
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
