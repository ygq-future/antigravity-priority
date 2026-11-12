package management

const overviewSamplesMarkup = `
    <!-- Quota Samples History Modal -->
    <div id="samplesModal" class="modal-backdrop" hidden>
        <div class="modal samples-modal" role="dialog" aria-modal="true" aria-labelledby="samplesModalTitle">
            <div class="samples-modal-head">
                <div class="samples-modal-title">
                    <span style="font-size:16px;">📊</span>
                    <h2 id="samplesModalTitle" style="margin:0; font-size:15px;">自适应时序采样明细</h2>
                </div>
                <button type="button" class="btn-secondary samples-modal-close" onclick="closeSamplesModal()">✕</button>
            </div>
            <div class="samples-modal-bar">
                <div id="samplesModalSubtitle" class="samples-modal-subtitle"></div>
                <div id="samplesPoolSwitcher" class="samples-pool-switcher">
                    <button type="button" class="btn-pool-pill active" id="btnSamplePoolGemini" onclick="switchSamplesPool('gemini')">Gemini</button>
                    <button type="button" class="btn-pool-pill" id="btnSamplePoolArgon" onclick="switchSamplesPool('argon')">Argon</button>
                    <button type="button" class="btn-pool-pill" id="btnSamplePoolClaude" onclick="switchSamplesPool('claude_gpt')">Claude &amp; GPT</button>
                </div>
            </div>
            <div id="samplesModalBody" class="modal-body samples-modal-body"></div>
        </div>
    </div>
`

const templateScriptOverviewSamples = `        let cachedSamplesData = null;
        let cachedSamplesEmail = "";
        let activeSamplePool = "gemini";

        async function openSamplesModal(authIndex, email) {
            const modal = document.getElementById("samplesModal");
            const title = document.getElementById("samplesModalTitle");
            const sub = document.getElementById("samplesModalSubtitle");
            const body = document.getElementById("samplesModalBody");
            if (!modal || !body) return;

            cachedSamplesEmail = email || "";
            const displayName = formatAccountName(email);
            title.textContent = t("samplesModalTitle");
            if (sub) sub.textContent = email ? "Email: " + displayName : "";
            body.innerHTML = "<div class=\"empty-state\">" + t("loading") + "</div>";
            modal.hidden = false;

            const modelInput = document.getElementById("modelSelect");
            let initialPool = "gemini";
            if (modelInput && modelInput.value) {
                initialPool = resolveModelPool(modelInput.value);
            }
            activeSamplePool = initialPool;

            try {
                const path = SAMPLES_PATH + "?auth_index=" + encodeURIComponent(authIndex);
                const data = await apiFetch(path);
                cachedSamplesData = data;
                renderSamplesModalGroup(activeSamplePool, cachedSamplesEmail);
            } catch (err) {
                body.innerHTML = "<div class=\"empty-state\" style=\"color:var(--accent-red-text);\">" + escapeHTML(err.message) + "</div>";
            }
        }

        function switchSamplesPool(pool) {
            activeSamplePool = pool;
            renderSamplesModalGroup(pool, cachedSamplesEmail);
        }

        function renderSamplesModalGroup(selectedGroup, email) {
            const body = document.getElementById("samplesModalBody");
            const sub = document.getElementById("samplesModalSubtitle");
            if (!body) return;

            const btnG = document.getElementById("btnSamplePoolGemini");
            const btnA = document.getElementById("btnSamplePoolArgon");
            const btnC = document.getElementById("btnSamplePoolClaude");
            if (btnG) btnG.classList.toggle("active", selectedGroup === "gemini");
            if (btnA) btnA.classList.toggle("active", selectedGroup === "argon");
            if (btnC) btnC.classList.toggle("active", selectedGroup === "claude_gpt");

            const groups = (cachedSamplesData && cachedSamplesData.groups) || {};
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
            if (sub) sub.textContent = email ? "Email: " + formatAccountName(email) : "";

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

                return "<div class=\"sample-meter\">" +
                    "<div class=\"sample-meter-track\">" +
                        "<div class=\"meter-fill " + fillClass + "\" style=\"width:" + num + "%; height:100%; border-radius:999px;\"></div>" +
                    "</div>" +
                    "<strong class=\"sample-meter-value\">" + num + "%</strong>" +
                "</div>";
            }

            let html = "<table class=\"sample-table\">" +
                "<thead><tr>" +
                    "<th style=\"width:32px; text-align:center;\">#</th>" +
                    "<th style=\"width:146px;\">" + t("colObservedAt") + "</th>" +
                    "<th>" + t("shortWindow") + "</th>" +
                    "<th>" + t("longWindow") + "</th>" +
                "</tr></thead>";

            for (let i = 0; i < samples.length; i++) {
                const sample = samples[i] || {};
                const timeRaw = sample.observed_at;
                const timeStr = timeRaw ? new Date(timeRaw).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";

                html += "<tbody class=\"sample-group\">" +
                    "<tr>" +
                        "<td class=\"sample-index\">" + (i + 1) + "</td>" +
                        "<td class=\"sample-time\">" + escapeHTML(timeStr) + "</td>" +
                        "<td>" + renderMiniMeter(sample.short_window_rem) + "</td>" +
                        "<td>" + renderMiniMeter(sample.long_window_rem) + "</td>" +
                    "</tr>" +
                "</tbody>";
            }

            html += "</table>";
            body.innerHTML = html;
        }

        function closeSamplesModal() {
            const modal = document.getElementById("samplesModal");
            if (modal) modal.hidden = true;
        }
`
