package management

const overviewTierMarkup = `
    <!-- Custom Tier Modal -->
    <div id="customTierModal" class="modal-backdrop" hidden>
        <div class="modal" role="dialog" aria-modal="true" aria-labelledby="customTierModalTitle" style="max-width: 420px; padding: 12px 16px;">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom: 8px;">
                <div style="display:flex; align-items:baseline; gap:6px;">
                    <span style="font-size:15px;">🎯</span>
                    <h2 id="customTierModalTitle" style="margin:0; font-size:15px; font-weight:600;" data-i18n="customTierModalTitle">设置自定义 Tier</h2>
                    <span id="customTierModalSubtitle" style="font-size:11px; color:var(--text-muted); font-family:monospace;"></span>
                </div>
                <div style="display:flex; align-items:center; gap:6px;">
                    <button type="button" class="btn-primary" onclick="submitCustomTier()" id="btnSaveCustomTier" title="保存并生效" disabled style="min-height:26px; height:26px; padding:0 8px; font-size:13px; display:inline-flex; align-items:center; justify-content:center; opacity:0.45; cursor:not-allowed;">💾</button>
                    <button type="button" class="btn-secondary" onclick="closeCustomTierModal()" title="关闭" style="min-height:26px; height:26px; padding:0 8px; font-size:12px; display:inline-flex; align-items:center; justify-content:center;">✕</button>
                </div>
            </div>

            <div style="margin-bottom: 8px;">
                <div style="display:flex; align-items:center; gap:6px; white-space:nowrap;">
                    <button type="button" class="btn-pool-pill" id="tierBtn0" onclick="setTierPreset(0)" style="padding:3px 9px; font-size:12px;" data-i18n="customTierAutoOption">自动</button>
                    <button type="button" class="btn-pool-pill" id="tierBtn1" onclick="setTierPreset(1)" style="padding:3px 9px; font-size:12px;">Tier 1</button>
                    <button type="button" class="btn-pool-pill" id="tierBtn2" onclick="setTierPreset(2)" style="padding:3px 9px; font-size:12px;">Tier 2</button>
                    <button type="button" class="btn-pool-pill" id="tierBtn3" onclick="setTierPreset(3)" style="padding:3px 9px; font-size:12px;">Tier 3</button>
                    <div style="display:flex; align-items:center; gap:4px; margin-left:auto;">
                        <span style="font-size:11px; color:var(--text-secondary);" data-i18n="customTierCustomInput">其他:</span>
                        <input type="number" id="customTierInput" min="0" max="99" value="0" class="config-num-input" style="width:48px; height:26px; padding:2px 4px; text-align:center; font-size:12px;" oninput="onCustomTierInputChange()" onkeydown="onCustomTierKeyDown(event)">
                    </div>
                </div>
            </div>

            <div style="font-size:11px; color:var(--text-muted); line-height:1.4;" data-i18n="customTierExplanation">
                💡 Tier 1 优先级高于 Tier 2，同级自动轮询均衡负载；设为 0 恢复自动计算。
            </div>
        </div>
    </div>
`

const templateScriptOverviewTier = `        let currentEditingAuthIndex = "";
        let initialTierValue = 0;

        function openCustomTierModal(authIndex, email, currentTier) {
            currentEditingAuthIndex = authIndex;
            initialTierValue = Number(currentTier) || 0;
            const modal = document.getElementById("customTierModal");
            const sub = document.getElementById("customTierModalSubtitle");
            const input = document.getElementById("customTierInput");
            if (!modal) return;
            const masked = formatMaskedEmail(email);
            if (sub) sub.textContent = email ? "(" + masked + ")" : (authIndex ? "(" + authIndex + ")" : "");
            if (input) input.value = initialTierValue;
            updateCustomTierSaveState();
            modal.hidden = false;
        }

        function closeCustomTierModal() {
            const modal = document.getElementById("customTierModal");
            if (modal) modal.hidden = true;
            currentEditingAuthIndex = "";
            initialTierValue = 0;
        }

        function setTierPreset(tier) {
            const input = document.getElementById("customTierInput");
            if (input) input.value = tier;
            updateCustomTierSaveState();
        }

        function onCustomTierKeyDown(e) {
            if (e.key === "-" || e.key === "+" || e.key === "." || e.key === "e" || e.key === "E") {
                e.preventDefault();
            }
        }

        function parseCustomTierInput() {
            const input = document.getElementById("customTierInput");
            if (!input) return { valid: false, value: 0 };
            const raw = input.value.trim();
            if (raw === "" || !/^\d+$/.test(raw)) {
                return { valid: false, value: 0 };
            }
            const num = parseInt(raw, 10);
            if (isNaN(num) || num < 0 || num > 99) {
                return { valid: false, value: num };
            }
            return { valid: true, value: num };
        }

        function onCustomTierInputChange() {
            const input = document.getElementById("customTierInput");
            if (input && input.value !== "") {
                const cleaned = input.value.replace(/[^\d]/g, "");
                if (cleaned !== input.value) {
                    input.value = cleaned;
                }
            }
            updateCustomTierSaveState();
        }

        function updateTierPresetButtons(val) {
            [0, 1, 2, 3].forEach(function(i) {
                const btn = document.getElementById("tierBtn" + i);
                if (btn) {
                    if (val === i) btn.classList.add("active");
                    else btn.classList.remove("active");
                }
            });
        }

        function updateCustomTierSaveState() {
            const saveBtn = document.getElementById("btnSaveCustomTier");
            const parsed = parseCustomTierInput();
            updateTierPresetButtons(parsed.valid ? parsed.value : -1);
            if (!saveBtn) return;
            const canSave = parsed.valid && parsed.value !== initialTierValue;
            saveBtn.disabled = !canSave;
            saveBtn.style.opacity = canSave ? "1" : "0.45";
            saveBtn.style.cursor = canSave ? "pointer" : "not-allowed";
        }

        async function submitCustomTier() {
            if (!currentEditingAuthIndex) return;
            const parsed = parseCustomTierInput();
            if (!parsed.valid || parsed.value === initialTierValue) return;

            const btn = document.getElementById("btnSaveCustomTier");
            if (btn) btn.disabled = true;

            try {
                await apiFetch(CUSTOM_TIER_PATH, {
                    method: "POST",
                    body: JSON.stringify({ auth_index: currentEditingAuthIndex, tier: parsed.value })
                });
                closeCustomTierModal();
                showToast(currentLang === "zh-CN" ? "Tier 设置已更新并立即生效" : "Tier setting updated and applied", "success");
                await refreshDashboard({ withSync: true, showLoading: true });
            } catch (e) {
                showToast((currentLang === "zh-CN" ? "保存失败: " : "Save failed: ") + e.message, "error");
            } finally {
                updateCustomTierSaveState();
            }
        }
`
