package management

const templateScriptOverviewActionsCore = `        let latestSnapshot = null;
        const probingAuthIndexes = new Set();
        let activeManualLoadingCount = 0;
        let autoScheduleLoadingActive = false;
        let autoSchedulePollTimer = null;
        let autoScheduleProbeInFlight = false;
        let autoScheduleDueHandledForRunAt = "";
        let autoSchedulePendingSince = 0;
        let autoScheduleLastObservedRunAt = null;
        let serverClockOffsetMs = 0;

        function credentialProbeButtonContent(probing) {
            return probing
                ? "<span class=\"probe-spinner\"></span>" + t("probing")
                : "⚡ " + t("btnProbeSingle");
        }

        function applyCredentialProbing(authIndex, probing, btn) {
            const content = credentialProbeButtonContent(probing);
            if (btn) {
                btn.disabled = probing;
                btn.innerHTML = content;
            }
            const container = document.getElementById("credentialsContainer");
            if (!container || typeof container.querySelectorAll !== "function") return;
            const cards = container.querySelectorAll(".cred-card");
            for (let i = 0; i < cards.length; i++) {
                const card = cards[i];
                if (!card || typeof card.getAttribute !== "function") continue;
                if (String(card.getAttribute("data-auth-index") || "") !== String(authIndex)) continue;
                if (probing) {
                    card.classList.add("cred-card-probing");
                } else {
                    card.classList.remove("cred-card-probing");
                }
                const probeBtn = (typeof card.querySelector === "function") ? card.querySelector("[data-probe-action]") : null;
                if (!probeBtn || probeBtn === btn) continue;
                probeBtn.disabled = probing;
                probeBtn.innerHTML = content;
            }
        }

        function formatReason(reason, isBoosted, isDisabled, routingStatus, probeFailed) {
            if (probeFailed && reason) {
                var lowerProbe = reason.toLowerCase();
                var prefix = lowerProbe.indexOf("probe invalid") >= 0
                    ? (currentLang === "zh-CN" ? "探测异常: " : "Probe invalid: ")
                    : (currentLang === "zh-CN" ? "探测失败: " : "Probe failed: ");
                var detail = reason;
                var colonIdx = reason.indexOf(":");
                if (colonIdx >= 0) {
                    detail = reason.substring(colonIdx + 1).trim();
                }
                return detail ? prefix + detail : (currentLang === "zh-CN" ? "探测失败" : "Probe failed");
            }
            if (routingStatus) {
                if (routingStatus === "disabled") return t("statusDisabled");
                if (routingStatus === "pool_unavailable") return currentLang === "zh-CN" ? "未开通该模型组" : "Model group unavailable";
                if (routingStatus === "weekly_depleted") return currentLang === "zh-CN" ? "周额度耗尽" : "Weekly Depleted";
                if (routingStatus === "short_depleted") return currentLang === "zh-CN" ? "5h短窗口耗尽" : "5h Depleted";
                if (routingStatus === "cold_start") return currentLang === "zh-CN" ? "待探测 (冷启动)" : "Cold Start (Pending Probe)";
                if (routingStatus === "cooldown") {
                    if (!reason || reason.toLowerCase() === "429 rate limit cooldown") {
                        return currentLang === "zh-CN" ? "⏳ 429 冷却中" : "⏳ 429 Cooldown";
                    }
                    return reason;
                }
                if (routingStatus === "active") {
                    if (isBoosted) return currentLang === "zh-CN" ? "🚀 优先提权" : "🚀 Boosted";
                    if (reason && reason.toLowerCase().indexOf("custom tier") === 0) return reason;
                    return currentLang === "zh-CN" ? "余量充足" : "Positive Balance";
                }
            }
            if (!reason) {
                if (isDisabled) return currentLang === "zh-CN" ? "周额度耗尽" : "Weekly Depleted";
                if (isBoosted) return currentLang === "zh-CN" ? "🚀 优先提权" : "🚀 Boosted";
                return currentLang === "zh-CN" ? "正常活跃" : "Active";
            }
            return reason;
        }
        function formatPlanBadge(planType) {
            var raw = (planType || "").trim().toLowerCase();
            if (raw === "pro") {
                return "<span class=\"badge badge-plan badge-plan-pro\">PRO</span>";
            }
            if (raw === "free") {
                return "<span class=\"badge badge-plan badge-plan-free\">FREE</span>";
            }
            if (raw === "ultra") {
                return "<span class=\"badge badge-plan badge-plan-ultra\">ULTRA</span>";
            }
            if (raw === "ultra_lite" || raw === "ultra-lite") {
                return "<span class=\"badge badge-plan badge-plan-ultra-lite\">ULTRA-LITE</span>";
            }
            if (raw === "team") {
                return "<span class=\"badge badge-plan badge-plan-default\">TEAM</span>";
            }
            if (raw === "plus") {
                return "<span class=\"badge badge-plan badge-plan-default\">PLUS</span>";
            }
            var label = planType ? planType.toUpperCase() : "ANTIGRAVITY";
            return "<span class=\"badge badge-plan badge-plan-default\">" + escapeHTML(label) + "</span>";
        }
        function autoScheduleLoadingOptions() {
            return {
                title: t("autoSchedulingTitle"),
                desc: t("autoSchedulingDesc"),
                icon: "⚡"
            };
        }

        function renderListLoadingOverlay(options) {
            var overlay = document.getElementById("credentialsLoadingOverlay");
            var container = document.getElementById("credentialsContainer");
            var isLoading = typeof options === "object" ? (options && options.loading !== false) : Boolean(options);

            if (isLoading) {
                if (overlay) {
                    if (typeof options === "object" && options && typeof overlay.querySelector === "function") {
                        var titleEl = overlay.querySelector(".quota-loading-title");
                        var descEl = overlay.querySelector(".quota-loading-desc");
                        var iconEl = overlay.querySelector(".radar-beacon");
                        if (titleEl && options.title) titleEl.textContent = options.title;
                        if (descEl && options.desc) descEl.textContent = options.desc;
                        if (iconEl && options.icon) iconEl.textContent = options.icon;
                    }
                    overlay.hidden = false;
                }
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

        function setListLoading(options) {
            var isLoading = typeof options === "object" ? (options && options.loading !== false) : Boolean(options);
            if (isLoading) {
                activeManualLoadingCount++;
                renderListLoadingOverlay(options);
                return;
            }
            if (activeManualLoadingCount > 0) {
                activeManualLoadingCount--;
            }
            if (activeManualLoadingCount > 0) {
                return;
            }
            if (autoScheduleLoadingActive && (typeof activeTab === "undefined" || activeTab === "overview")) {
                renderListLoadingOverlay(autoScheduleLoadingOptions());
            } else {
                renderListLoadingOverlay(false);
            }
        }

        function isOverviewActiveAndVisible() {
            var onOverview = (typeof activeTab === "undefined") || activeTab === "overview";
            var visible = (typeof document === "undefined") || !document.hidden;
            return onOverview && visible;
        }

        function isAutoScheduleEnabledInUI() {
            var isAutoApplyEnabled = false;
            if (typeof dynamicConfig !== "undefined" && dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
            } else if (typeof latestDiagnostics !== "undefined" && latestDiagnostics && latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
            }
            var sched = (typeof latestDiagnostics !== "undefined" && latestDiagnostics && latestDiagnostics.scheduler) ? latestDiagnostics.scheduler : null;
            var isPaused = (typeof scheduleConfig !== "undefined" && scheduleConfig)
                ? Boolean(scheduleConfig.paused)
                : Boolean(sched && sched.paused);
            return isAutoApplyEnabled && !isPaused;
        }

        function clearAutoSchedulePollTimer() {
            if (autoSchedulePollTimer) {
                clearTimeout(autoSchedulePollTimer);
                autoSchedulePollTimer = null;
            }
        }

        function updateServerClockOffset(sched) {
            if (!sched || !sched.server_now) return;
            var parsed = Date.parse(sched.server_now);
            if (Number.isFinite(parsed)) {
                serverClockOffsetMs = parsed - Date.now();
            }
        }

        function setAutoScheduleListLoading(active, options) {
            var next = Boolean(active);
            var changed = autoScheduleLoadingActive !== next;
            autoScheduleLoadingActive = next;
            if (activeManualLoadingCount <= 0) {
                if (next) {
                    renderListLoadingOverlay(autoScheduleLoadingOptions());
                } else {
                    renderListLoadingOverlay(false);
                }
            }
            if (changed && !(options && options.skipRender) && typeof renderDashboard === "function") {
                renderDashboard();
            }
        }

        function resetAutoScheduleLoading(options) {
            clearAutoSchedulePollTimer();
            autoSchedulePendingSince = 0;
            autoScheduleLastObservedRunAt = null;
            setAutoScheduleListLoading(false, options);
        }

        function scheduleAutoSchedulePoll(delayMs) {
            clearAutoSchedulePollTimer();
            autoSchedulePollTimer = setTimeout(pollAutoScheduleProgress, delayMs);
        }

        function syncScheduleConfigFromDiagnostics(data) {
            if (data && data.scheduler && typeof scheduleConfig !== "undefined") {
                if (!scheduleConfig) {
                    scheduleConfig = {
                        paused: Boolean(data.scheduler.paused)
                    };
                } else if (data.scheduler.paused !== undefined) {
                    scheduleConfig.paused = Boolean(data.scheduler.paused);
                }
            }
        }

        function hasAutoScheduleSettled(sched, prevLastRun, prevNextRun) {
            if (!sched) return false;
            var currLastRun = String(sched.last_auto_apply_at || "");
            var currNextRun = String(sched.next_run_at || "");
            var completedNewRun = prevLastRun !== null && currLastRun !== prevLastRun;
            var movedNextRun = Boolean(currNextRun) && currNextRun !== prevNextRun;
            return completedNewRun || movedNextRun || sched.mode === "idle";
        }

        function syncAutoScheduleFromDiagnostics(data, options) {
            var opts = options || {};
            var sched = (data && data.scheduler) ? data.scheduler : null;
            if (!sched) return false;
            updateServerClockOffset(sched);

            if (!isOverviewActiveAndVisible() || !isAutoScheduleEnabledInUI()) {
                resetAutoScheduleLoading(opts);
                return false;
            }

            if (sched.running) {
                autoSchedulePendingSince = 0;
                if (sched.next_run_at) {
                    autoScheduleDueHandledForRunAt = String(sched.next_run_at);
                }
                if (autoScheduleLastObservedRunAt === null) {
                    autoScheduleLastObservedRunAt = String(sched.last_auto_apply_at || "");
                }
                setAutoScheduleListLoading(true, opts);
                if (!autoSchedulePollTimer && !autoScheduleProbeInFlight) {
                    scheduleAutoSchedulePoll(1500);
                }
                return true;
            }

            if (autoScheduleLoadingActive) {
                if (hasAutoScheduleSettled(sched, autoScheduleLastObservedRunAt, autoScheduleDueHandledForRunAt)) {
                    resetAutoScheduleLoading(opts);
                    return false;
                }
                if (autoSchedulePendingSince > 0 && (Date.now() - autoSchedulePendingSince) < 5000) {
                    if (!autoSchedulePollTimer && !autoScheduleProbeInFlight) {
                        scheduleAutoSchedulePoll(1200);
                    }
                    return true;
                }
                resetAutoScheduleLoading(opts);
            }
            return false;
        }

        function checkAutoScheduleReady() {
            if (!isOverviewActiveAndVisible() || !isAutoScheduleEnabledInUI()) {
                if (autoScheduleLoadingActive) {
                    resetAutoScheduleLoading();
                }
                return false;
            }
            if (typeof apiFetch !== "function") return false;
            if (activeManualLoadingCount > 0 || autoScheduleLoadingActive) return false;

            var sched = (typeof latestDiagnostics !== "undefined" && latestDiagnostics && latestDiagnostics.scheduler)
                ? latestDiagnostics.scheduler
                : null;
            if (!sched || sched.running || sched.worker_active === false) return false;
            if (sched.mode === "idle" || !sched.next_run_at) return false;

            var nextRunStr = String(sched.next_run_at);
            if (autoScheduleDueHandledForRunAt === nextRunStr) return false;

            var targetMs = Date.parse(nextRunStr);
            if (!Number.isFinite(targetMs)) return false;

            var estNowMs = Date.now() + serverClockOffsetMs;
            if (targetMs > estNowMs) return false;

            autoScheduleDueHandledForRunAt = nextRunStr;
            if (estNowMs - targetMs > 15000) {
                return false;
            }

            autoScheduleLastObservedRunAt = String(sched.last_auto_apply_at || "");
            autoSchedulePendingSince = Date.now();
            setAutoScheduleListLoading(true);
            scheduleAutoSchedulePoll(600);
            return true;
        }

        async function pollAutoScheduleProgress() {
            autoSchedulePollTimer = null;
            if (typeof isAuthBlocked !== "undefined" && isAuthBlocked) {
                resetAutoScheduleLoading();
                return;
            }
            if (!isOverviewActiveAndVisible() || !isAutoScheduleEnabledInUI()) {
                resetAutoScheduleLoading();
                return;
            }
            if (autoScheduleProbeInFlight) return;
            autoScheduleProbeInFlight = true;
            try {
                var wasActive = autoScheduleLoadingActive;
                var prevLastRun = autoScheduleLastObservedRunAt;
                var prevNextRun = autoScheduleDueHandledForRunAt;
                var data = await apiFetch(DIAGNOSTICS_PATH);
                latestDiagnostics = data;
                syncScheduleConfigFromDiagnostics(data);
                var sched = (data && data.scheduler) || {};
                updateServerClockOffset(sched);
                if (sched.running) {
                    if (syncAutoScheduleFromDiagnostics(data, { skipRender: true })) {
                        scheduleAutoSchedulePoll(1500);
                    }
                } else if (wasActive) {
                    if (hasAutoScheduleSettled(sched, prevLastRun, prevNextRun)) {
                        clearAutoSchedulePollTimer();
                        autoSchedulePendingSince = 0;
                        autoScheduleLastObservedRunAt = null;
                        await fetchSnapshot({ silent: true });
                        setAutoScheduleListLoading(false, { skipRender: true });
                    } else if (autoSchedulePendingSince > 0 && (Date.now() - autoSchedulePendingSince) < 5000) {
                        scheduleAutoSchedulePoll(1200);
                    } else {
                        resetAutoScheduleLoading({ skipRender: true });
                    }
                }
                if (typeof renderDashboard === "function") renderDashboard();
                if (typeof renderHistory === "function") renderHistory();
                if (typeof renderDiagnostics === "function") renderDiagnostics();
                if (typeof renderScheduleStatus === "function") renderScheduleStatus();
            } catch (_) {
                if (autoScheduleLoadingActive && isOverviewActiveAndVisible() && isAutoScheduleEnabledInUI()) {
                    if (autoSchedulePendingSince === 0 || (Date.now() - autoSchedulePendingSince) < 5000) {
                        scheduleAutoSchedulePoll(2000);
                    } else {
                        resetAutoScheduleLoading();
                    }
                }
            } finally {
                autoScheduleProbeInFlight = false;
            }
        }

        async function fetchSnapshot(options) {
            if (isAuthBlocked) return;
            const settings = options || {};
            try {
                const data = await apiFetch(SNAPSHOT_PATH);
                latestSnapshot = data;
                if (data && Array.isArray(data.models)) {
                    updateModelDropdownFromSnapshot(data.models);
                }
                renderDashboard();
            } catch (err) {
                if (!settings.silent && !isAuthBlocked) showToast(err.message, "error");
            }
        }

        async function fetchDiagnostics(options) {
            if (isAuthBlocked) return;
            const settings = options || {};
            try {
                const data = await apiFetch(DIAGNOSTICS_PATH);
                latestDiagnostics = data;
                if (!latestSnapshot) {
                    try {
                        const snapshot = await apiFetch(SNAPSHOT_PATH);
                        latestSnapshot = snapshot;
                        if (snapshot && Array.isArray(snapshot.models)) {
                            updateModelDropdownFromSnapshot(snapshot.models);
                        }
                    } catch (_) {}
                }
                syncScheduleConfigFromDiagnostics(data);
                syncAutoScheduleFromDiagnostics(data, { skipRender: true });
                renderDashboard();
                renderHistory();
                renderDiagnostics();
                renderScheduleStatus();
            } catch (err) {
                if (!settings.silent && !isAuthBlocked) showToast(err.message, "error");
            }
        }

        async function syncHost(options) {
            if (isAuthBlocked) return;
            const settings = options || {};
            try {
				const data = await apiFetch(SYNC_PATH, { method: "POST" });
                latestSnapshot = data;
                if (data && Array.isArray(data.models)) {
                    updateModelDropdownFromSnapshot(data.models);
                }
                if (settings.notifySync) showToast(t("syncSuccess"), "success");
            } catch (err) {
                if (isAuthBlocked) throw err;
                await fetchSnapshot(settings);
            }
        }

        async function refreshDashboard(options) {
            if (isAuthBlocked) return;
            const settings = options || {};
            const btn = document.getElementById("btnRefresh");
            const isManualSync = Boolean(btn && settings.notifySync);
            const isInitialBoot = !latestSnapshot;
            const shouldShowListLoading = isManualSync || isInitialBoot || Boolean(settings.showLoading);

            if (isManualSync) {
                btn.disabled = true;
                btn.classList.add("btn-refreshing");
            }
            if (shouldShowListLoading) {
                setListLoading({
                    title: t("syncingQuotaTitle"),
                    desc: t("syncingQuotaDesc"),
                    icon: "🔄"
                });
            }
            try {
                if (settings.withSync) {
                    await syncHost(settings);
                } else {
                    await fetchSnapshot(settings);
                }
                if (!isAuthBlocked) {
                    await fetchDiagnostics(settings);
                }
            } finally {
                if (shouldShowListLoading) {
                    setListLoading(false);
                }
                if (isManualSync) {
                    btn.disabled = false;
                    btn.classList.remove("btn-refreshing");
                }
            }
        }

        async function probeSingleCredential(authIndex, btn) {
            if (isAuthBlocked || !authIndex) return;
            const probeKey = String(authIndex);
            probingAuthIndexes.add(probeKey);
            applyCredentialProbing(probeKey, true, btn);
            try {
                const url = RUN_PATH + "?mode=probe&auth_index=" + encodeURIComponent(authIndex);
                const data = await apiFetch(url, { method: "POST" });
                await fetchSnapshot({ silent: true });

                const outcomes = (data && Array.isArray(data.probe_outcomes)) ? data.probe_outcomes : [];
                const targetOutcome = outcomes.find(function(o) { return o && o.auth_index === authIndex; }) || null;

                if (targetOutcome && targetOutcome.status === "unavailable") {
                    var unavailableDetail = targetOutcome.error ? " · " + targetOutcome.error : "";
                    showToast(t("probeUnavailable") + unavailableDetail, "info");
                } else if (targetOutcome && targetOutcome.status !== "success") {
                    let detail = targetOutcome.error || (currentLang === "zh-CN" ? "探测未成功" : "Probe unsuccessful");
                    if (targetOutcome.http_status_code && detail.indexOf(String(targetOutcome.http_status_code)) < 0) {
                        detail += " (HTTP " + targetOutcome.http_status_code + ")";
                    }
                    showToast(t("probeSingleFailed") + " · " + detail, "error");
                } else if (targetOutcome && targetOutcome.status === "success") {
                    showToast(t("probeSingleSuccess"), "success");
                } else {
                    showToast(t("probeSingleNoResult"), "error");
                }
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                probingAuthIndexes.delete(probeKey);
                applyCredentialProbing(probeKey, false, btn);
            }
        }

        async function toggleCredentialWarmup(authIndex, group, enabled, button) {
            if (isAuthBlocked || !authIndex || !group || !button) return;
            const wasOn = button.classList.contains("is-on");
            button.classList.add("is-pending");
            button.disabled = true;
            button.classList.toggle("is-on", enabled);
            button.classList.toggle("is-off", !enabled);
            button.setAttribute("aria-pressed", enabled ? "true" : "false");
            try {
                await apiFetch(WARMUP_CREDENTIAL_PATH, {
                    method: "POST",
                    body: JSON.stringify({ auth_index: authIndex, group: group, enabled: enabled })
                });
                showToast(t(enabled ? "warmupCardEnabled" : "warmupCardDisabled"), "success");
                await refreshDashboard({ withSync: true, silent: true });
            } catch (err) {
                button.classList.toggle("is-on", wasOn);
                button.classList.toggle("is-off", !wasOn);
                button.setAttribute("aria-pressed", wasOn ? "true" : "false");
                showToast(err.message, "error");
            } finally {
                button.classList.remove("is-pending");
                button.disabled = false;
            }
        }

        function extractChanges(result) {
            if (!result) return [];
            if (Array.isArray(result.changes) && result.changes.length > 0) {
                return result.changes;
            }
            if (result.snapshot && Array.isArray(result.snapshot.items) && result.snapshot.items.length > 0) {
                return result.snapshot.items;
            }
            if (Array.isArray(result.items) && result.items.length > 0) {
                return result.items;
            }
            return [];
        }

`
