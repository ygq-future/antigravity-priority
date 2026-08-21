package management

const templateScriptModals = `        // Show Modal: Apply Confirm vs History Details Snapshot
        function showModal(mode, result, isFromHistory) {
            const modal = document.getElementById("diffModal");
            const title = document.getElementById("modalTitle");
            const summary = document.getElementById("modalSummary");
            const list = document.getElementById("modalDiffList");
            const btnApply = document.getElementById("btnModalApply");

            const isApplyConfirm = (mode === "apply-confirm");
            const changes = extractChanges(result);

            if (isApplyConfirm) {
                title.textContent = t("confirmApplyTitle");
                btnApply.hidden = false;
                btnApply.textContent = t("btnConfirmApply");
                btnApply.onclick = executeDirectApply;
                summary.textContent = (currentLang === "zh-CN" ? "待写回凭证数量: " : "Credentials to update: ") + changes.length;
            } else {
                title.textContent = currentLang === "zh-CN" ? "执行明细快照" : "Execution Details Snapshot";
                btnApply.hidden = true;
                summary.textContent = (currentLang === "zh-CN" ? "成功: " : "Succeeded: ") + (result.succeeded || 0) + ", " +
                    (currentLang === "zh-CN" ? "失败: " : "Failed: ") + (result.failed || 0) + ", " +
                    (currentLang === "zh-CN" ? "跳过: " : "Skipped: ") + (result.skipped || 0);
            }

            list.innerHTML = "";
            if (changes.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noChanges") + "</div>";
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

                    const name = c.name || c.auth_index || "Credential";
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

        async function openSamplesModal(authIndex, name) {
            const modal = document.getElementById("samplesModal");
            const title = document.getElementById("samplesModalTitle");
            const sub = document.getElementById("samplesModalSubtitle");
            const body = document.getElementById("samplesModalBody");
            if (!modal || !body) return;

            title.textContent = (name || authIndex) + " - " + t("samplesModalTitle");
            sub.textContent = "ID: " + authIndex;
            body.innerHTML = "<div class=\"empty-state\">" + t("loading") + "</div>";
            modal.hidden = false;

            try {
                const path = SAMPLES_PATH + "?auth_index=" + encodeURIComponent(authIndex);
                const data = await apiFetch(path);
                const groups = (data && data.groups) || {};
                const geminiData = (groups.gemini && groups.gemini.samples) || [];
                const claudeData = (groups.claude_gpt && groups.claude_gpt.samples) || [];

                const maxLen = Math.max(geminiData.length, claudeData.length);
                if (maxLen === 0) {
                    body.innerHTML = "<div class=\"empty-state\">" + t("noSamples") + "</div>";
                    return;
                }

                function renderMiniMeter(val) {
                    if (val === undefined || val === null || val === "-") return "<span style=\"color:var(--text-muted);\">-</span>";
                    const num = Math.max(0, Math.min(100, parseInt(val, 10) || 0));
                    let fillClass = "meter-fill-healthy";
                    if (num <= 10) fillClass = "meter-fill-danger";
                    else if (num <= 30) fillClass = "meter-fill-warning";

                    return "<div style=\"display:flex; align-items:center; gap:8px; width:100%; min-width:85px; max-width:140px;\">" +
                        "<div style=\"flex:1; height:5px; background:var(--meter-bg); border-radius:999px; overflow:hidden;\">" +
                            "<div class=\"meter-fill " + fillClass + "\" style=\"width:" + num + "%; height:100%; border-radius:999px;\"></div>" +
                        "</div>" +
                        "<strong style=\"font-size:11px; font-family:monospace; min-width:32px; text-align:right;\">" + num + "%</strong>" +
                    "</div>";
                }

                let html = "<table class=\"sample-table\">" +
                    "<thead><tr>" +
                        "<th style=\"width:36px; text-align:center;\">#</th>" +
                        "<th style=\"width:150px;\">" + t("colObservedAt") + "</th>" +
                        "<th style=\"width:110px;\">" + (currentLang === "zh-CN" ? "模型组" : "Group") + "</th>" +
                        "<th>" + t("shortWindow") + "</th>" +
                        "<th>" + t("longWindow") + "</th>" +
                    "</tr></thead>";

                for (let i = 0; i < maxLen; i++) {
                    const g = geminiData[i];
                    const c = claudeData[i];
                    const timeRaw = (g && g.observed_at) || (c && c.observed_at);
                    const timeStr = timeRaw ? new Date(timeRaw).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";

                    const g5hMeter = g ? renderMiniMeter(g.short_window_rem) : "-";
                    const g7dMeter = g ? renderMiniMeter(g.long_window_rem) : "-";
                    const c5hMeter = c ? renderMiniMeter(c.short_window_rem) : "-";
                    const c7dMeter = c ? renderMiniMeter(c.long_window_rem) : "-";

                    html += "<tbody class=\"sample-group\">" +
                        "<tr>" +
                            "<td rowspan=\"2\" class=\"sample-group-bottom\" style=\"font-weight:700; text-align:center; vertical-align:middle; border-bottom:1px solid var(--border-color);\">" + (i + 1) + "</td>" +
                            "<td rowspan=\"2\" class=\"sample-group-bottom\" style=\"vertical-align:middle; border-bottom:1px solid var(--border-color); font-size:12px; color:var(--text-secondary); font-family:monospace;\">" + escapeHTML(timeStr) + "</td>" +
                            "<td style=\"border-bottom:1px dashed var(--border-subtle); padding:6px 10px;\"><span class=\"badge badge-subtle\" style=\"font-size:10px;\">🔵 Gemini</span></td>" +
                            "<td style=\"border-bottom:1px dashed var(--border-subtle); padding:6px 10px;\">" + g5hMeter + "</td>" +
                            "<td style=\"border-bottom:1px dashed var(--border-subtle); padding:6px 10px;\">" + g7dMeter + "</td>" +
                        "</tr>" +
                        "<tr>" +
                            "<td class=\"sample-group-bottom\" style=\"border-bottom:1px solid var(--border-color); padding:6px 10px;\"><span class=\"badge badge-predicted\" style=\"font-size:10px;\">🟣 Claude/GPT</span></td>" +
                            "<td class=\"sample-group-bottom\" style=\"border-bottom:1px solid var(--border-color); padding:6px 10px;\">" + c5hMeter + "</td>" +
                            "<td class=\"sample-group-bottom\" style=\"border-bottom:1px solid var(--border-color); padding:6px 10px;\">" + c7dMeter + "</td>" +
                        "</tr>" +
                    "</tbody>";
                }

                html += "</table>";
                body.innerHTML = html;
            } catch (err) {
                body.innerHTML = "<div class=\"empty-state\" style=\"color:var(--accent-red-text);\">" + escapeHTML(err.message) + "</div>";
            }
        }

        function closeSamplesModal() {
            const modal = document.getElementById("samplesModal");
            if (modal) modal.hidden = true;
        }

`
