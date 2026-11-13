package management

const templateScriptProbeSchedule = `        function setListProbeLoading(loading) {
            if (typeof setListLoading === "function") {
                if (loading) {
                    setListLoading({
                        title: t("probingQuotaTitle"),
                        desc: t("probingQuotaDesc"),
                        icon: "📡"
                    });
                } else {
                    setListLoading(false);
                }
                return;
            }
            var overlay = document.getElementById("credentialsLoadingOverlay");
            var container = document.getElementById("credentialsContainer");
            if (loading) {
                if (overlay) overlay.hidden = false;
                if (container && container.classList && container.classList.add) {
                    container.classList.add("credentials-probing");
                }
            } else {
                if (overlay) overlay.hidden = true;
                if (container && container.classList && container.classList.remove) {
                    container.classList.remove("credentials-probing");
                }
            }
        }

        async function triggerProbe() {
            var btn = document.getElementById("btnProbe");
            if (!btn || btn.classList.contains("btn-cooldown") || btn.classList.contains("btn-probing")) return;

            btn.disabled = true;
            btn.classList.add("btn-probing");
            var label = btn.querySelector("[data-i18n='btnProbe']") || btn.querySelector("span");
            var origText = label ? label.textContent : "";
            if (label) label.textContent = t("probing");

            setListProbeLoading(true);
            try {
                var path = RUN_PATH + "?mode=probe";
                var data = await apiFetch(path, { method: "POST" });
                var outcomes = (data && Array.isArray(data.probe_outcomes)) ? data.probe_outcomes : [];
                var failedCount = 0;
                var successCount = 0;
                var unavailableCount = 0;
                for (var i = 0; i < outcomes.length; i++) {
                    var o = outcomes[i];
                    if (!o) continue;
                    if (o.status === "success") {
                        successCount++;
                    } else if (o.status === "unavailable") {
                        unavailableCount++;
                    } else {
                        failedCount++;
                    }
                }
                if (failedCount > 0) {
                    showToast(t("probeBatchFailed") + " (" + failedCount + ")", "error");
                } else if (successCount === 0 && unavailableCount > 0) {
                    showToast(t("probeUnavailable"), "info");
                } else {
                    showToast(t("probeSuccess"), "success");
                }
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                setListProbeLoading(false);
                btn.classList.remove("btn-probing");
            }

            btn.classList.add("btn-cooldown");
            var remaining = 10;
            if (label) label.textContent = t("probeCooldown") + " (" + remaining + "s)";
            probeCooldownTimer = setInterval(function() {
                remaining--;
                if (label) label.textContent = t("probeCooldown") + " (" + remaining + "s)";
                if (remaining <= 0) {
                    clearInterval(probeCooldownTimer);
                    probeCooldownTimer = null;
                    btn.disabled = false;
                    btn.classList.remove("btn-cooldown");
                    if (label) label.textContent = origText;
                }
            }, 1000);
        }

        async function fetchScheduleConfig() {
            if (isAuthBlocked) return;
            try {
                scheduleConfig = await apiFetch(SCHEDULE_CONFIG_PATH);
                renderScheduleStatus();
            } catch (_) {}
        }

        function renderScheduleStatus() {
            var badge = document.getElementById("scheduleStatusBadge");
            var textEl = document.getElementById("scheduleStatusText");
            if (!badge || !textEl) return;

            var isAutoApplyEnabled = false;
            if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
            } else if (latestDiagnostics && latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
            }

            if (!isAutoApplyEnabled) {
                badge.className = "schedule-status paused";
                badge.title = currentLang === "zh-CN" ? "自动定时调度已关闭，点击前往配置中心开启" : "Auto-scheduling disabled. Click to open Config Center.";
                badge.innerHTML = "⚪ <span id=\"scheduleStatusText\">" + t("scheduleDisabled") + "</span>";
                return;
            }

            var sched = (typeof scheduleConfig !== "undefined" && scheduleConfig) ? scheduleConfig : ((typeof latestDiagnostics !== "undefined" && latestDiagnostics && latestDiagnostics.scheduler) ? latestDiagnostics.scheduler : null);
            if (!sched) return;

            if (sched.paused) {
                badge.className = "schedule-status paused";
                badge.title = currentLang === "zh-CN" ? "点击恢复自动调度" : "Click to resume";
                badge.innerHTML = "⏸ <span id=\"scheduleStatusText\">" + t("schedulePaused") + "</span>";
                return;
            }

            var modeTip = "";
            if (typeof latestDiagnostics !== "undefined" && latestDiagnostics && latestDiagnostics.scheduler && latestDiagnostics.scheduler.mode) {
                var ds = latestDiagnostics.scheduler;
                modeTip = currentLang === "zh-CN"
                    ? (" [" + ds.mode + " · 近窗口 " + (ds.curr_window_requests || 0) + "/" + (ds.prev_window_requests || 0) + "]")
                    : (" [" + ds.mode + " · window " + (ds.curr_window_requests || 0) + "/" + (ds.prev_window_requests || 0) + "]");
            }
            badge.className = "schedule-status active";
            badge.title = (currentLang === "zh-CN" ? "点击暂停自动调度" : "Click to pause") + modeTip;
            badge.innerHTML = "🟢 <span id=\"scheduleStatusText\">" + t("scheduleActive") + "</span>";
        }

        async function toggleSchedulePause() {
            var isAutoApplyEnabled = false;
            if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
            } else if (latestDiagnostics && latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
            }

            if (!isAutoApplyEnabled) {
                switchTab("config");
                showToast(currentLang === "zh-CN" ? "请在配置中心开启“自动定时调度”开关" : "Please enable 'Auto Periodic Scheduling' in Config Center", "info");
                return;
            }

            if (!scheduleConfig) {
                await fetchScheduleConfig();
                if (!scheduleConfig) return;
            }

            var newConfig = {
                paused: !scheduleConfig.paused
            };

            try {
                scheduleConfig = await apiFetch(SCHEDULE_CONFIG_PATH, {
                    method: "POST",
                    body: JSON.stringify(newConfig)
                });
                renderScheduleStatus();
                renderDashboard();
                renderDiagnostics();
                showToast(scheduleConfig.paused ? t("schedulePaused") : t("scheduleActive"), "info");
            } catch (err) {
                showToast(err.message, "error");
            }
        }

`
