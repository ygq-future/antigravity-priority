package management

const templateScriptProbeSchedule = `        async function triggerProbe() {
            var btn = document.getElementById("btnProbe");
            if (!btn || btn.classList.contains("btn-cooldown")) return;

            btn.disabled = true;
            btn.classList.add("btn-cooldown");

            try {
                var groupSelect = document.getElementById("modelGroupSelect");
                var group = groupSelect ? groupSelect.value : "gemini";
                var path = RUN_PATH + "?mode=probe&antigravity_model_group=" + encodeURIComponent(group);
                await apiFetch(path, { method: "POST" });
                showToast(t("probeSuccess"), "success");
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            }

            var remaining = 10;
            var label = btn.querySelector("[data-i18n='btnProbe']") || btn.querySelector("span");
            var origText = label ? label.textContent : "";
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

            if (!scheduleConfig) return;

            if (scheduleConfig.paused) {
                badge.className = "schedule-status paused";
                badge.title = currentLang === "zh-CN" ? "点击恢复自动调度" : "Click to resume";
                badge.innerHTML = "⏸ <span id=\"scheduleStatusText\">" + t("schedulePaused") + "</span>";
                return;
            }

            var windowActive = true;
            if (scheduleConfig.window_enabled && scheduleConfig.window_start && scheduleConfig.window_end) {
                windowActive = isCurrentTimeInScheduleWindow(scheduleConfig.window_start, scheduleConfig.window_end);
            }

            if (scheduleConfig.window_enabled && !windowActive) {
                badge.className = "schedule-status sleeping";
                badge.title = currentLang === "zh-CN" ? "当前时间不在生效时段内" : "Outside active schedule window";
                badge.innerHTML = "🌙 <span id=\"scheduleStatusText\">" + t("scheduleSleeping") + "</span>";
                return;
            }

            var windowStr = "";
            if (scheduleConfig.window_enabled && scheduleConfig.window_start && scheduleConfig.window_end) {
                windowStr = " (" + scheduleConfig.window_start + "-" + scheduleConfig.window_end + ")";
            }
            badge.className = "schedule-status active";
            badge.title = currentLang === "zh-CN" ? "点击暂停自动调度" : "Click to pause";
            badge.innerHTML = "🟢 <span id=\"scheduleStatusText\">" + t("scheduleActive") + windowStr + "</span>";
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
                paused: !scheduleConfig.paused,
                window_enabled: scheduleConfig.window_enabled || false,
                window_start: scheduleConfig.window_start || "00:00",
                window_end: scheduleConfig.window_end || "23:59"
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
