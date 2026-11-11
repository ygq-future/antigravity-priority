package management

const templateScriptOverviewActionsCore = `        let latestSnapshot = null;
        let userSelectedModelGroup = false;
        const probingAuthIndexes = new Set();

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

        function formatReason(reason, isBoosted, isDisabled) {
            if (!reason) {
                if (isDisabled) return currentLang === "zh-CN" ? "周额度耗尽" : "Weekly Depleted";
                if (isBoosted) return currentLang === "zh-CN" ? "🚀 优先提权" : "🚀 Boosted";
                return currentLang === "zh-CN" ? "正常活跃" : "Active";
            }
            var lower = reason.toLowerCase();
            if (lower.indexOf("probe failed") >= 0 || lower.indexOf("probe invalid") >= 0) {
                var prefix = lower.indexOf("probe invalid") >= 0
                    ? (currentLang === "zh-CN" ? "探测异常: " : "Probe invalid: ")
                    : (currentLang === "zh-CN" ? "探测失败: " : "Probe failed: ");
                var detail = reason;
                var colonIdx = reason.indexOf(":");
                if (colonIdx >= 0) {
                    detail = reason.substring(colonIdx + 1).trim();
                }
                return detail ? prefix + detail : (currentLang === "zh-CN" ? "探测失败" : "Probe failed");
            }
            if (lower.indexOf("cooldown") >= 0 || (lower.indexOf("429") >= 0 && lower.indexOf("status 429") < 0)) return currentLang === "zh-CN" ? "⏳ 429 冷却中" : "⏳ 429 Cooldown";
            if (lower.indexOf("boost") >= 0) return currentLang === "zh-CN" ? "🚀 优先提权" : "🚀 Boosted";
            if (lower.indexOf("remaining positive") >= 0) return currentLang === "zh-CN" ? "余量充足" : "Positive Balance";
            if (lower.indexOf("weekly depleted") >= 0) return currentLang === "zh-CN" ? "周额度耗尽" : "Weekly Depleted";
            if (lower.indexOf("short") >= 0 && lower.indexOf("depleted") >= 0) return currentLang === "zh-CN" ? "5h短窗口耗尽" : "5h Depleted";
            if (lower.indexOf("predicted") >= 0) return currentLang === "zh-CN" ? "🔮 预测优先级" : "🔮 Predicted";
            if (lower.indexOf("in sync") >= 0 || lower.indexOf("optimal") >= 0) return currentLang === "zh-CN" ? "状态最优" : "In Sync";
            if (lower.indexOf("disabled on host") >= 0) return t("statusDisabled");
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
            if (raw === "team") {
                return "<span class=\"badge badge-plan badge-plan-default\">TEAM</span>";
            }
            if (raw === "plus") {
                return "<span class=\"badge badge-plan badge-plan-default\">PLUS</span>";
            }
            var label = planType ? planType.toUpperCase() : "ANTIGRAVITY";
            return "<span class=\"badge badge-plan badge-plan-default\">" + escapeHTML(label) + "</span>";
        }
        function formatMaskedEmail(email) {
            if (!email || email === "Credential") return "Credential";
            var s = String(email).trim();
            if (s.length <= 4) return s + "***";
            return s.substring(0, 4) + "***";
        }

        function setListLoading(options) {
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
                if (data && data.scheduler && typeof scheduleConfig !== "undefined") {
                    if (!scheduleConfig) {
                        scheduleConfig = {
                            paused: Boolean(data.scheduler.paused),
                            window_enabled: Boolean(data.scheduler.window_enabled),
                            window_start: data.scheduler.window_start || "00:00",
                            window_end: data.scheduler.window_end || "23:59"
                        };
                    } else {
                        if (data.scheduler.paused !== undefined) {
                            scheduleConfig.paused = Boolean(data.scheduler.paused);
                        }
                        if (data.scheduler.window_enabled !== undefined) {
                            scheduleConfig.window_enabled = Boolean(data.scheduler.window_enabled);
                        }
                        if (data.scheduler.window_start) {
                            scheduleConfig.window_start = data.scheduler.window_start;
                        }
                        if (data.scheduler.window_end) {
                            scheduleConfig.window_end = data.scheduler.window_end;
                        }
                    }
                }
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
                const groupSelect = document.getElementById("modelGroupSelect");
                const modelGroup = groupSelect ? groupSelect.value : "gemini";
                const url = RUN_PATH + "?mode=probe&antigravity_model_group=" + encodeURIComponent(modelGroup) + "&auth_index=" + encodeURIComponent(authIndex);
                const data = await apiFetch(url, { method: "POST" });
                await fetchSnapshot({ silent: true });

                const outcomes = (data && Array.isArray(data.probe_outcomes)) ? data.probe_outcomes : [];
                const targetOutcome = outcomes.find(function(o) { return o && o.auth_index === authIndex && (!modelGroup || o.model_group === modelGroup); }) ||
                    outcomes.find(function(o) { return o && o.auth_index === authIndex; }) || null;

                if (targetOutcome && targetOutcome.status !== "success") {
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

`
