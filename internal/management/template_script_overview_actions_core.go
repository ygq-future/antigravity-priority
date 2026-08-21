package management

const templateScriptOverviewActionsCore = `        async function fetchSnapshot() {
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
                showToast(err.message, "error");
            }
        }

        async function fetchDiagnostics() {
            try {
                const data = await apiFetch(DIAGNOSTICS_PATH);
                latestDiagnostics = data;
                renderDashboard();
                renderHistory();
                renderDiagnostics();
                renderScheduleStatus();
            } catch (err) {
                showToast(err.message, "error");
            }
        }

        async function syncHost() {
            try {
				const data = await apiFetch(SYNC_PATH, { method: "POST" });
                latestSnapshot = data;
                renderDashboard();
                showToast(t("syncSuccess"), "success");
            } catch (err) {
                await fetchSnapshot();
            }
        }

        async function refreshDashboard(withSync) {
            const btn = document.getElementById("btnRefresh");
            if (btn) btn.disabled = true;
            try {
                if (withSync) {
                    await syncHost();
                } else {
                    await fetchSnapshot();
                }
                await fetchDiagnostics();
            } finally {
                if (btn) btn.disabled = false;
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
            if (changes.length === 0) {
                showToast(t("noChangesToApply"), "info");
                return;
            }

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
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (btnApp) btnApp.disabled = false;
            }
        }

`
