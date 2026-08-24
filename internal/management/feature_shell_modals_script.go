package management

const templateScriptModals = `        // Show Modal: Apply Confirm vs History Details Snapshot
        function showModal(mode, result, isFromHistory) {
            const modal = document.getElementById("diffModal");
            const title = document.getElementById("modalTitle");
            const summary = document.getElementById("modalSummary");
            const list = document.getElementById("modalDiffList");
            const btnApply = document.getElementById("btnModalApply");

            const isApplyConfirm = (mode === "apply-confirm");
            const isProbeHistory = (mode === "probe-history");

            if (isProbeHistory) {
                title.textContent = currentLang === "zh-CN" ? "配额探测明细" : "Quota Probe Details";
                btnApply.hidden = true;
                const groupOrder = ["gemini", "claude_gpt"];
                const groups = result && result.groups && typeof result.groups === "object" ? result.groups : {};
                const totalRecords = groupOrder.reduce(function(total, modelGroup) {
                    return total + (Array.isArray(groups[modelGroup]) ? groups[modelGroup].length : 0);
                }, 0);
                summary.textContent = (currentLang === "zh-CN" ? "本轮新增额度样本: " : "New quota samples in this round: ") + totalRecords;
                list.innerHTML = "";
                if (totalRecords === 0) {
                    list.innerHTML = "<div class=\"empty-state\">" + (currentLang === "zh-CN" ? "本轮没有新增额度变化样本" : "No quota changes were appended in this probe round") + "</div>";
                } else {
                    groupOrder.forEach(function(modelGroup) {
                        const records = Array.isArray(groups[modelGroup]) ? groups[modelGroup] : [];
                        if (records.length === 0) return;

                        const groupSection = document.createElement("div");
                        groupSection.className = "probe-history-group";
                        groupSection.innerHTML = "<div class=\"probe-history-group-title\">" +
                            (modelGroup === "gemini" ? "Gemini" : "Claude/GPT") + "</div>";
                        list.appendChild(groupSection);

                        records.forEach(function(record) {
                            const sample = record.sample || {};
                            const email = record.email || "Credential";
                            const observedAt = sample.observed_at ? new Date(sample.observed_at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                            function quotaMeter(value) {
                                const parsed = Number(value);
                                if (!Number.isFinite(parsed)) return "-";
                                const percent = Math.max(0, Math.min(100, Math.trunc(parsed)));
                                return "<div style=\"display:flex; align-items:center; gap:6px; min-width:150px;\"><div style=\"flex:1; height:7px; background:var(--meter-track-bg); border:1px solid var(--meter-track-border); border-radius:999px; overflow:hidden;\"><div class=\"meter-fill meter-fill-healthy\" style=\"width:" + percent + "%; height:100%;\"></div></div><strong style=\"font-size:11px; min-width:32px; text-align:right;\">" + percent + "%</strong></div>";
                            }
                            const row = document.createElement("div");
                            row.className = "diff-card";
                            row.innerHTML = "<div style=\"min-width:0; flex:1;\"><div style=\"font-weight:700; font-size:13px;\">" + escapeHTML(email) + "</div><div style=\"font-size:11px; color:var(--text-muted);\">" + escapeHTML(observedAt) + "</div></div>" +
                                "<div style=\"display:flex; flex-direction:column; gap:5px; min-width:190px; font-size:11px;\">" +
                                    "<div style=\"display:flex; align-items:center; gap:6px;\"><span style=\"min-width:18px;\">5h</span>" + quotaMeter(sample.short_window_rem) + "</div>" +
                                    "<div style=\"display:flex; align-items:center; gap:6px;\"><span style=\"min-width:18px;\">7d</span>" + quotaMeter(sample.long_window_rem) + "</div>" +
                                "</div>";
                            groupSection.appendChild(row);
                        });
                    });
                }
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

`
