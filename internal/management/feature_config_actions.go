package management

const templateScriptConfig = `        let originalConfigState = null;

        async function fetchDynamicConfig() {
            if (isAuthBlocked) return;
            try {
                dynamicConfig = await apiFetch(CONFIG_PATH);
                if (!userSelectedModelGroup && dynamicConfig && dynamicConfig.antigravity_model_group) {
                    const select = document.getElementById("modelGroupSelect");
                    if (select && select.value !== dynamicConfig.antigravity_model_group) {
                        select.value = dynamicConfig.antigravity_model_group;
                        updateCustomSelectDisplay();
                        const menu = document.getElementById("customSelectMenu");
                        if (menu) {
                            menu.querySelectorAll(".custom-select-option").forEach(opt => {
                                const isSelected = opt.getAttribute("data-value") === dynamicConfig.antigravity_model_group;
                                opt.classList.toggle("selected", isSelected);
                            });
                        }
                    }
                }
                renderDynamicConfigForm(dynamicConfig);
                renderScheduleStatus();
            } catch (err) {
                if (!isAuthBlocked) showToast(err.message, "error");
            }
        }

        function getDynamicConfigFormData() {
            var autoApply = Boolean(document.getElementById("cfgAutoApply") && document.getElementById("cfgAutoApply").checked);
            var intervalSelect = document.getElementById("cfgIntervalSelect");
            var intervalCustom = document.getElementById("cfgIntervalCustom");
            var interval = "15m";
            if (intervalSelect) {
                interval = intervalSelect.value === "custom" ? ((intervalCustom && intervalCustom.value.trim()) || "") : intervalSelect.value;
            }
            var modelGroup = (document.getElementById("cfgModelGroup") && document.getElementById("cfgModelGroup").value) || "gemini";
            var ignoreDisabledHost = Boolean(document.getElementById("cfgIgnoreDisabledHost") && document.getElementById("cfgIgnoreDisabledHost").checked);
            var windowEnabled = Boolean(document.getElementById("cfgWindowEnabled") && document.getElementById("cfgWindowEnabled").checked);
            var windowStart = (document.getElementById("cfgWindowStart") && document.getElementById("cfgWindowStart").value.trim()) || "";
            var windowEnd = (document.getElementById("cfgWindowEnd") && document.getElementById("cfgWindowEnd").value.trim()) || "";
            var maxConcurrency = (document.getElementById("cfgMaxConcurrency") && document.getElementById("cfgMaxConcurrency").value) || "";
            var minChange = (document.getElementById("cfgMinChange") && document.getElementById("cfgMinChange").value) || "";
            var urgencyTol = (document.getElementById("cfgUrgencyTolerance") && document.getElementById("cfgUrgencyTolerance").value) || "";
            var sampleCapacity = (document.getElementById("cfgSampleCapacity") && document.getElementById("cfgSampleCapacity").value) || "";
            var cooldownMin = (document.getElementById("cfgCooldownMinutes") && document.getElementById("cfgCooldownMinutes").value) || "";
            var boostStart = (document.getElementById("cfgBoostStartPriority") && document.getElementById("cfgBoostStartPriority").value) || "";
            var normalStart = (document.getElementById("cfgNormalStartPriority") && document.getElementById("cfgNormalStartPriority").value) || "";

            return JSON.stringify({
                autoApply, interval, modelGroup, windowEnabled, windowStart, windowEnd,
                maxConcurrency, minChange, urgencyTol, sampleCapacity, cooldownMin, boostStart, normalStart, ignoreDisabledHost
            });
        }

        function updateSaveButtonState() {
            var btn = document.getElementById("btnSaveConfig");
            if (!btn) return;
            if (!originalConfigState) {
                btn.disabled = true;
                return;
            }
            var current = getDynamicConfigFormData();
            btn.disabled = current === originalConfigState;
        }

        function renderDynamicConfigForm(cfg) {
            if (!cfg) return;

            var autoApply = document.getElementById("cfgAutoApply");
            var autoText = document.getElementById("cfgAutoApplyStatusText");
            if (autoApply) {
                autoApply.checked = Boolean(cfg.auto_apply);
                if (autoText) autoText.textContent = cfg.auto_apply ? (currentLang === "zh-CN" ? "已开启" : "Enabled") : (currentLang === "zh-CN" ? "已关闭" : "Disabled");
            }

            var intervalSelect = document.getElementById("cfgIntervalSelect");
            var intervalCustom = document.getElementById("cfgIntervalCustom");
            var intervalVal = cfg.interval || "15m";
            if (intervalSelect) {
                if (["5m", "15m", "30m", "1h"].indexOf(intervalVal) >= 0) {
                    intervalSelect.value = intervalVal;
                    if (intervalCustom) intervalCustom.hidden = true;
                } else {
                    intervalSelect.value = "custom";
                    if (intervalCustom) {
                        intervalCustom.value = intervalVal;
                        intervalCustom.hidden = false;
                    }
                }
                updateCustomIntervalDisplay();
            }

            var modelGroup = document.getElementById("cfgModelGroup");
            if (modelGroup) {
                modelGroup.value = cfg.antigravity_model_group || "gemini";
                updateCustomCfgModelDisplay();
            }

            var ignoreDisabledHost = document.getElementById("cfgIgnoreDisabledHost");
            var ignoreDisabledHostText = document.getElementById("cfgIgnoreDisabledHostStatusText");
            if (ignoreDisabledHost) {
                ignoreDisabledHost.checked = cfg.ignore_disabled_host !== undefined ? Boolean(cfg.ignore_disabled_host) : true;
                if (ignoreDisabledHostText) ignoreDisabledHostText.textContent = ignoreDisabledHost.checked ? (currentLang === "zh-CN" ? "已开启" : "Enabled") : (currentLang === "zh-CN" ? "已关闭" : "Disabled");
            }

            var windowEnabled = document.getElementById("cfgWindowEnabled");
            var windowInputs = document.getElementById("cfgWindowInputs");
            var windowStart = document.getElementById("cfgWindowStart");
            var windowEnd = document.getElementById("cfgWindowEnd");
            var sched = cfg.schedule || {};
            if (windowEnabled) {
                windowEnabled.checked = Boolean(sched.window_enabled);
                if (windowInputs) windowInputs.style.opacity = sched.window_enabled ? "1" : "0.5";
            }
            if (windowStart) windowStart.value = sched.window_start || "09:00";
            if (windowEnd) windowEnd.value = sched.window_end || "23:00";

            var maxConcurrency = document.getElementById("cfgMaxConcurrency");
            if (maxConcurrency) maxConcurrency.value = cfg.max_concurrency || 6;

            var minChange = document.getElementById("cfgMinChange");
            if (minChange) minChange.value = cfg.min_change !== undefined ? cfg.min_change : 1;

            var urgencyTol = document.getElementById("cfgUrgencyTolerance");
            if (urgencyTol) urgencyTol.value = cfg.urgency_tolerance !== undefined ? cfg.urgency_tolerance : 0.05;

            var sampleCapacity = document.getElementById("cfgSampleCapacity");
            if (sampleCapacity) sampleCapacity.value = cfg.quota_sample_capacity !== undefined ? cfg.quota_sample_capacity : 6;

            var cooldownMin = document.getElementById("cfgCooldownMinutes");
            if (cooldownMin) cooldownMin.value = cfg.rate_limit_cooldown_minutes !== undefined ? cfg.rate_limit_cooldown_minutes : 5;

            var rules = cfg.priority_rules || {};
            var boostStart = document.getElementById("cfgBoostStartPriority");
            if (boostStart) boostStart.value = rules.boost_start_priority || 999;

            var normalStart = document.getElementById("cfgNormalStartPriority");
            if (normalStart) normalStart.value = rules.normal_start_priority || 100;

            // Sync original snapshot and disable save button
            originalConfigState = getDynamicConfigFormData();
            updateSaveButtonState();
        }

        function onIntervalSelectChange() {
            var select = document.getElementById("cfgIntervalSelect");
            var custom = document.getElementById("cfgIntervalCustom");
            if (!select || !custom) return;
            custom.hidden = select.value !== "custom";
            if (select.value === "custom" && !custom.value) {
                custom.value = "20m";
            }
            updateSaveButtonState();
        }

        function onWindowEnabledChange() {
            var chk = document.getElementById("cfgWindowEnabled");
            var inputs = document.getElementById("cfgWindowInputs");
            if (chk && inputs) {
                inputs.style.opacity = chk.checked ? "1" : "0.5";
            }
            updateSaveButtonState();
        }

        document.addEventListener("change", function(e) {
            if (e.target && e.target.id === "cfgAutoApply") {
                var autoText = document.getElementById("cfgAutoApplyStatusText");
                if (autoText) {
                    autoText.textContent = e.target.checked ? (currentLang === "zh-CN" ? "已开启" : "Enabled") : (currentLang === "zh-CN" ? "已关闭" : "Disabled");
                }
            }
            if (e.target && e.target.id === "cfgIgnoreDisabledHost") {
                var ignoreText = document.getElementById("cfgIgnoreDisabledHostStatusText");
                if (ignoreText) {
                    ignoreText.textContent = e.target.checked ? (currentLang === "zh-CN" ? "已开启" : "Enabled") : (currentLang === "zh-CN" ? "已关闭" : "Disabled");
                }
            }
            if (e.target && e.target.closest("#panelConfig")) {
                updateSaveButtonState();
            }
        });

        document.addEventListener("input", function(e) {
            if (e.target && e.target.closest("#panelConfig")) {
                updateSaveButtonState();
            }
        });

        // Comprehensive Frontend Value Validation
        function parseGoDurationMillis(value) {
            var text = String(value || "").trim();
            if (!text) return NaN;
			if (text.charAt(0) === "+") text = text.slice(1);
            var units = { ns: 0.000001, us: 0.001, "µs": 0.001, "μs": 0.001, ms: 1, s: 1000, m: 60000, h: 3600000 };
            var token = /([0-9]+(?:\.[0-9]*)?|\.[0-9]+)(ns|us|µs|μs|ms|s|m|h)/g;
            var total = 0;
            var consumed = "";
            var match;
            while ((match = token.exec(text)) !== null) {
                consumed += match[0];
                total += Number(match[1]) * units[match[2]];
            }
            return consumed === text ? total : NaN;
        }

        function parseStrictInteger(value) {
            var text = String(value || "").trim();
            return /^-?\d+$/.test(text) ? Number(text) : NaN;
        }

        async function saveDynamicConfig() {
            var btn = document.getElementById("btnSaveConfig");
            if (btn) btn.disabled = true;

            try {
                var autoApply = Boolean(document.getElementById("cfgAutoApply") && document.getElementById("cfgAutoApply").checked);
                var intervalSelect = document.getElementById("cfgIntervalSelect");
                var intervalCustom = document.getElementById("cfgIntervalCustom");
                var interval = "15m";
                if (intervalSelect) {
                    if (intervalSelect.value === "custom") {
                        interval = (intervalCustom && intervalCustom.value.trim()) || "";
						if (!interval || parseGoDurationMillis(interval) < 60000) {
                            showToast(t("valErrInterval"), "error");
                            if (intervalCustom) intervalCustom.focus();
                            updateSaveButtonState();
                            return;
                        }
                    } else {
                        interval = intervalSelect.value;
                    }
                }

                var modelGroup = (document.getElementById("cfgModelGroup") && document.getElementById("cfgModelGroup").value) || "gemini";
                var ignoreDisabledHost = Boolean(document.getElementById("cfgIgnoreDisabledHost") && document.getElementById("cfgIgnoreDisabledHost").checked);
                var windowEnabled = Boolean(document.getElementById("cfgWindowEnabled") && document.getElementById("cfgWindowEnabled").checked);
                var windowStart = (document.getElementById("cfgWindowStart") && document.getElementById("cfgWindowStart").value.trim()) || "09:00";
                var windowEnd = (document.getElementById("cfgWindowEnd") && document.getElementById("cfgWindowEnd").value.trim()) || "23:00";

                if (windowEnabled) {
                    var timeRegex = /^([01]\d|2[0-3]):[0-5]\d$/;
                    if (!timeRegex.test(windowStart) || !timeRegex.test(windowEnd)) {
                        showToast(t("valErrWindow"), "error");
                        updateSaveButtonState();
                        return;
                    }
                }

				var maxConcurrency = parseStrictInteger((document.getElementById("cfgMaxConcurrency") && document.getElementById("cfgMaxConcurrency").value) || "6");
                if (isNaN(maxConcurrency) || maxConcurrency < 1 || maxConcurrency > 32) {
                    showToast(t("valErrConcurrency"), "error");
                    updateSaveButtonState();
                    return;
                }

				var minChange = parseStrictInteger((document.getElementById("cfgMinChange") && document.getElementById("cfgMinChange").value) || "1");
                if (isNaN(minChange) || minChange < 0 || minChange > 100) {
                    showToast(t("valErrMinChange"), "error");
                    updateSaveButtonState();
                    return;
                }

                var urgencyTolRaw = (document.getElementById("cfgUrgencyTolerance") && document.getElementById("cfgUrgencyTolerance").value) || "0.05";
				var urgencyTol = Number(urgencyTolRaw);
				if (urgencyTolRaw.trim() === "" || isNaN(urgencyTol) || urgencyTol < 0.0 || urgencyTol > 0.50) {
                    showToast(t("valErrUrgencyTol"), "error");
                    updateSaveButtonState();
                    return;
                }

				var sampleCapacity = parseStrictInteger((document.getElementById("cfgSampleCapacity") && document.getElementById("cfgSampleCapacity").value) || "6");
                if (isNaN(sampleCapacity) || sampleCapacity < 2 || sampleCapacity > 30) {
                    showToast(t("valErrSampleCapacity"), "error");
                    updateSaveButtonState();
                    return;
                }

				var cooldownMin = parseStrictInteger((document.getElementById("cfgCooldownMinutes") && document.getElementById("cfgCooldownMinutes").value) || "5");
                if (isNaN(cooldownMin) || cooldownMin < 1 || cooldownMin > 1440) {
                    showToast(t("valErrCooldown"), "error");
                    updateSaveButtonState();
                    return;
                }

				var boostStart = parseStrictInteger((document.getElementById("cfgBoostStartPriority") && document.getElementById("cfgBoostStartPriority").value) || "999");
				var normalStart = parseStrictInteger((document.getElementById("cfgNormalStartPriority") && document.getElementById("cfgNormalStartPriority").value) || "100");

                if (isNaN(boostStart) || boostStart < 1 || boostStart > 999 || isNaN(normalStart) || normalStart < 1 || normalStart > 999) {
                    showToast(t("valErrPriorityRange"), "error");
                    updateSaveButtonState();
                    return;
                }
                if (normalStart > boostStart) {
                    showToast(t("valErrPriorityOrder"), "error");
                    updateSaveButtonState();
                    return;
                }

                var reqBody = {
                    auto_apply: autoApply,
                    interval: interval,
                    antigravity_model_group: modelGroup,
                    ignore_disabled_host: ignoreDisabledHost,
                    max_concurrency: maxConcurrency,
                    min_change: minChange,
                    urgency_tolerance: urgencyTol,
                    rate_limit_cooldown_minutes: cooldownMin,
                    quota_sample_capacity: sampleCapacity,
                    priority_rules: {
                        boost_start_priority: boostStart,
                        normal_start_priority: normalStart
                    },
                    schedule: {
                        paused: Boolean(scheduleConfig && scheduleConfig.paused),
                        window_enabled: windowEnabled,
                        window_start: windowStart,
                        window_end: windowEnd
                    }
                };

                dynamicConfig = await apiFetch(CONFIG_PATH, {
                    method: "POST",
                    body: JSON.stringify(reqBody)
                });
                renderDynamicConfigForm(dynamicConfig);
                await fetchScheduleConfig();
                await refreshDashboard();
                showToast(t("configSaveSuccess"), "success");
            } catch (err) {
                showToast(err.message, "error");
                updateSaveButtonState();
            }
        }

        async function resetDynamicConfigToDefaults() {
            var confirmed = await showThemedConfirm({
                title: t("confirmResetConfigTitle"),
                message: t("confirmResetConfigMsg"),
                confirmText: currentLang === "zh-CN" ? "确认恢复默认" : "Reset to Defaults",
                cancelText: currentLang === "zh-CN" ? "取消" : "Cancel",
                isDanger: false
            });
            if (!confirmed) return;

            var defaultCfg = {
                auto_apply: false,
                interval: "15m",
                antigravity_model_group: "gemini",
                ignore_disabled_host: true,
                max_concurrency: 6,
                min_change: 1,
                urgency_tolerance: 0.05,
                quota_sample_capacity: 6,
                rate_limit_cooldown_minutes: 5,
                priority_rules: {
                    boost_start_priority: 999,
                    normal_start_priority: 100
                },
                schedule: {
                    paused: false,
                    window_enabled: false,
                    window_start: "00:00",
                    window_end: "23:59"
                }
            };
            renderDynamicConfigForm(defaultCfg);
            updateSaveButtonState();
            await saveDynamicConfig();
        }

`
