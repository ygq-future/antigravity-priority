package management

const overviewSamplesMarkup = `
    <!-- Quota Samples History Modal -->
    <div id="samplesModal" class="modal-backdrop" hidden>
        <div class="modal" role="dialog" aria-modal="true" aria-labelledby="samplesModalTitle" style="max-width: 620px;">
            <div style="display:flex; justify-content:space-between; align-items:center;">
                <div style="display:flex; align-items:center; gap:8px;">
                    <span style="font-size:18px;">📊</span>
                    <h2 id="samplesModalTitle" style="margin:0; font-size:16px;">自适应时序采样明细</h2>
                </div>
                <button type="button" class="btn-secondary" onclick="closeSamplesModal()" style="min-height:28px; padding:3px 8px;">✕</button>
            </div>
            <div id="samplesModalSubtitle" style="font-size:12px; color:var(--text-muted); font-family:monospace;"></div>
            <div id="samplesModalBody" class="modal-body" style="max-height:360px; overflow-y:auto; padding:4px 0;"></div>
            <div style="display:flex; justify-content:flex-end; gap:8px; margin-top:6px;">
                <button type="button" class="btn-secondary" onclick="closeSamplesModal()" data-i18n="btnClose">关闭</button>
            </div>
        </div>
    </div>
`

const templateScriptOverviewSamples = `        async function openSamplesModal(authIndex, name) {
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
