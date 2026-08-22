package management

const templateScriptOverviewActionsCore = `        let latestSnapshot = null;
        let userSelectedModelGroup = false;

        function formatReason(reason, isBoosted, isDisabled) {
            if (!reason) {
                if (isDisabled) return currentLang === "zh-CN" ? "周额度耗尽" : "Weekly Depleted";
                if (isBoosted) return currentLang === "zh-CN" ? "🚀 优先提权" : "🚀 Boosted";
                return currentLang === "zh-CN" ? "正常活跃" : "Active";
            }
            var lower = reason.toLowerCase();
            if (lower.indexOf("boost") >= 0) return currentLang === "zh-CN" ? "🚀 优先提权" : "🚀 Boosted";
            if (lower.indexOf("remaining positive") >= 0) return currentLang === "zh-CN" ? "余量充足" : "Positive Balance";
            if (lower.indexOf("weekly depleted") >= 0) return currentLang === "zh-CN" ? "周额度耗尽" : "Weekly Depleted";
            if (lower.indexOf("short") >= 0 && lower.indexOf("depleted") >= 0) return currentLang === "zh-CN" ? "5h短窗口耗尽" : "5h Depleted";
            if (lower.indexOf("429") >= 0 || lower.indexOf("cooldown") >= 0) return currentLang === "zh-CN" ? "⏳ 429 冷却中" : "⏳ 429 Cooldown";
            if (lower.indexOf("predicted") >= 0) return currentLang === "zh-CN" ? "🔮 预测优先级" : "🔮 Predicted";
            if (lower.indexOf("in sync") >= 0 || lower.indexOf("optimal") >= 0) return currentLang === "zh-CN" ? "状态最优" : "In Sync";
            if (lower.indexOf("disabled on host") >= 0) return currentLang === "zh-CN" ? "已禁用" : "Disabled";
            return reason;
        }

        async function fetchSnapshot(options) {
            const settings = options || {};
            try {
                const data = await apiFetch(SNAPSHOT_PATH);
                latestSnapshot = data;
                if (!userSelectedModelGroup && data && data.active_model_group) {
                    const select = document.getElementById("modelGroupSelect");
                    if (select && select.value !== data.active_model_group) {
                        select.value = data.active_model_group;
                        updateCustomSelectDisplay();
                        const menu = document.getElementById("customSelectMenu");
                        if (menu) {
                            menu.querySelectorAll(".custom-select-option").forEach(opt => {
                                const isSelected = opt.getAttribute("data-value") === data.active_model_group;
                                opt.classList.toggle("selected", isSelected);
                            });
                        }
                    }
                }
                renderDashboard();
            } catch (err) {
                if (!settings.silent) showToast(err.message, "error");
            }
        }

        async function fetchDiagnostics(options) {
            const settings = options || {};
            try {
                const data = await apiFetch(DIAGNOSTICS_PATH);
                latestDiagnostics = data;
                renderDashboard();
                renderHistory();
                renderDiagnostics();
                renderScheduleStatus();
            } catch (err) {
                if (!settings.silent) showToast(err.message, "error");
            }
        }

        async function syncHost(options) {
            const settings = options || {};
            try {
				const data = await apiFetch(SYNC_PATH, { method: "POST" });
                latestSnapshot = data;
                renderDashboard();
                if (settings.notifySync) showToast(t("syncSuccess"), "success");
            } catch (err) {
                await fetchSnapshot(settings);
            }
        }

        async function refreshDashboard(options) {
            const settings = options || {};
            const btn = document.getElementById("btnRefresh");
            if (btn && settings.notifySync) btn.disabled = true;
            try {
                if (settings.withSync) {
                    await syncHost(settings);
                } else {
                    await fetchSnapshot(settings);
                }
                await fetchDiagnostics(settings);
            } finally {
                if (btn && settings.notifySync) btn.disabled = false;
            }
        }

        function extractChanges(result) {
            if (!result) return [];
            if (Array.isArray(result.changes) && result.changes.length > 0) {
                return result.changes;
            }
            if (result.snapshot && Array.isArray(result.snapshot.changes) && result.snapshot.changes.length > 0) {
                return result.snapshot.changes;
            }
            if (result.snapshot && Array.isArray(result.snapshot.items) && result.snapshot.items.length > 0) {
                return result.snapshot.items;
            }
            if (Array.isArray(result.items) && result.items.length > 0) {
                return result.items;
            }
            return [];
        }

        async function triggerApplyWithConfirm() {
            const groupSelect = document.getElementById("modelGroupSelect");
            const selectedGroup = groupSelect ? groupSelect.value : "gemini";
            const activeGroup = (dynamicConfig && dynamicConfig.antigravity_model_group) || (latestSnapshot && latestSnapshot.active_model_group) || "gemini";
            if (selectedGroup !== activeGroup) {
                showToast(currentLang === "zh-CN" ? "仅主控组可执行写回，当前主控组为 " + activeGroup : "Only active group can apply. Active: " + activeGroup, "info");
                return;
            }
            const groupData = (latestSnapshot && latestSnapshot.groups && latestSnapshot.groups[selectedGroup]) || {};
            const items = groupData.items || [];
            if (items.length === 0) {
                showToast(currentLang === "zh-CN" ? "未发现有效凭证，无需执行写回" : "No credentials found to apply", "info");
                return;
            }
            const changes = groupData.changes || [];

            // Show changes preview modal with direct confirm button
            showModal("apply-confirm", {
                snapshot: groupData,
                changes: changes,
                items: items
            }, false);
        }

        async function executeDirectApply() {
            closeModal();
            const groupSelect = document.getElementById("modelGroupSelect");
            const group = groupSelect ? groupSelect.value : "gemini";
            const btnApp = document.getElementById("btnApply");
            if (btnApp) btnApp.disabled = true;

            try {
                const path = RUN_PATH + "?mode=apply&antigravity_model_group=" + encodeURIComponent(group);
                const result = await apiFetch(path, { method: "POST" });
                const succeeded = (result && result.succeeded !== undefined) ? result.succeeded : 0;
                const attempted = (result && result.attempted !== undefined) ? result.attempted : 0;
                if (succeeded > 0 || attempted > 0) {
                    showToast(currentLang === "zh-CN" ? "已成功写回 CPA 宿主凭证优先级" : "Applied credential priorities to CPA host successfully", "success");
                } else {
                    showToast(t("noChangesToApply"), "info");
                }
                await refreshDashboard({ silent: true });
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (btnApp) btnApp.disabled = false;
            }
        }

`
