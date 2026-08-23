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
                const groupSelect = document.getElementById("modelGroupSelect");
                const selectedGroup = (groupSelect && groupSelect.value) || "gemini";
                const selectedGroupData = groups[selectedGroup] || {};
                const samples = (Array.isArray(selectedGroupData.samples) ? selectedGroupData.samples : []).slice().sort(function(left, right) {
                    const leftTime = Date.parse(left && left.observed_at ? left.observed_at : "");
                    const rightTime = Date.parse(right && right.observed_at ? right.observed_at : "");
                    const leftValid = Number.isFinite(leftTime);
                    const rightValid = Number.isFinite(rightTime);
                    if (leftValid && rightValid && leftTime !== rightTime) return rightTime - leftTime;
                    if (leftValid !== rightValid) return rightValid ? 1 : -1;
                    return Number(right && right.sequence || 0) - Number(left && left.sequence || 0);
                });
                const groupLabel = selectedGroup === "claude_gpt" ? "Claude/GPT" : "Gemini";

                if (samples.length === 0) {
                    body.innerHTML = "<div class=\"empty-state\">" + t("noSamples") + "</div>";
                    return;
                }

                function renderMiniMeter(val) {
                    if (val === undefined || val === null || val === "") return "<span style=\"color:var(--text-muted);\">-</span>";
                    const parsed = Number(val);
                    if (!Number.isFinite(parsed)) return "<span style=\"color:var(--text-muted);\">-</span>";
                    const num = Math.max(0, Math.min(100, Math.trunc(parsed)));
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
                        "<th>" + t("shortWindow") + "</th>" +
                        "<th>" + t("longWindow") + "</th>" +
                    "</tr></thead>";

                for (let i = 0; i < samples.length; i++) {
                    const sample = samples[i] || {};
                    const timeRaw = sample.observed_at;
                    const timeStr = timeRaw ? new Date(timeRaw).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";

                    html += "<tbody class=\"sample-group\">" +
                        "<tr>" +
                            "<td style=\"font-weight:700; text-align:center; border-bottom:1px solid var(--border-color);\">" + (i + 1) + "</td>" +
                            "<td style=\"border-bottom:1px solid var(--border-color); font-size:12px; color:var(--text-secondary); font-family:monospace;\">" + escapeHTML(timeStr) + "</td>" +
                            "<td style=\"border-bottom:1px solid var(--border-color); padding:6px 10px;\">" + renderMiniMeter(sample.short_window_rem) + "</td>" +
                            "<td style=\"border-bottom:1px solid var(--border-color); padding:6px 10px;\">" + renderMiniMeter(sample.long_window_rem) + "</td>" +
                        "</tr>" +
                    "</tbody>";
                }

                html += "</table>";
                body.innerHTML = html;
                sub.textContent = "ID: " + authIndex + " · " + groupLabel;
            } catch (err) {
                body.innerHTML = "<div class=\"empty-state\" style=\"color:var(--accent-red-text);\">" + escapeHTML(err.message) + "</div>";
            }
        }

        function closeSamplesModal() {
            const modal = document.getElementById("samplesModal");
            if (modal) modal.hidden = true;
        }

`
