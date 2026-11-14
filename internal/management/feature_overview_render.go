package management

const templateScriptOverviewRender = `        function resolveModelPool(modelId) {
            if (!modelId) return "gemini";
            const catalog = (typeof latestSnapshot !== "undefined" && latestSnapshot && Array.isArray(latestSnapshot.models))
                ? latestSnapshot.models
                : ((typeof window !== "undefined" && Array.isArray(window.cpaAvailableModels)) ? window.cpaAvailableModels : null);
            if (catalog) {
                for (let i = 0; i < catalog.length; i++) {
                    const m = catalog[i];
                    if (m && m.id === modelId && m.quota_pool) {
                        return m.quota_pool;
                    }
                }
            }
            if (typeof document !== "undefined" && typeof document.getElementById === "function") {
                const menu = document.getElementById("customModelSelectMenu");
                if (menu && typeof menu.querySelectorAll === "function") {
                    const opts = menu.querySelectorAll(".custom-select-option");
                    for (let i = 0; i < opts.length; i++) {
                        if (opts[i] && typeof opts[i].getAttribute === "function" && opts[i].getAttribute("data-value") === modelId) {
                            const poolAttr = opts[i].getAttribute("data-pool");
                            if (poolAttr) return poolAttr;
                        }
                    }
                }
            }
            return "gemini";
        }

        function resolveItemRoutingStatus(item) {
            if (item && item.routing_status) {
                return item.routing_status;
            }
            if (item && item.host && item.host.disabled) {
                return "disabled";
            }
            if (item && item.cooldown_until) {
                return "cooldown";
            }
            const targetP = (item && item.target && item.target.priority !== undefined)
                ? item.target.priority
                : ((item && item.priority !== undefined) ? item.priority : 0);
            if (targetP <= -2 || (item && item.target && item.target.disabled)) {
                return "weekly_depleted";
            }
            if (targetP === -1) {
                return "short_depleted";
            }
            return "active";
        }

        function isModelEligible(item, selectedModel) {
            const status = resolveItemRoutingStatus(item);
            if (status === "disabled") {
                return { eligible: false, reason: "disabled" };
            }
            if (typeof isModelExcluded === "function" && isModelExcluded(selectedModel)) {
                return { eligible: false, reason: "excluded_model" };
            }
            if (status === "pool_unavailable" || (item && item.pool_eligible === false)) {
                return { eligible: false, reason: "unsupported_model" };
            }
            if (item && Array.isArray(item.supported_models) && item.supported_models.length > 0) {
                const needle = String(selectedModel || "").toLowerCase().trim();
                const found = item.supported_models.some(function(m) {
                    return String(m || "").toLowerCase().trim() === needle;
                });
                if (!found) {
                    return { eligible: false, reason: "unsupported_model" };
                }
            }
            return { eligible: true, reason: "" };
        }
        function collectCredentialCards(container) {
            if (!container) return [];
            if (typeof container.querySelectorAll === "function") {
                const matched = container.querySelectorAll(".cred-card");
                if (matched && matched.length > 0) return matched;
            }
            return container.children || [];
        }

        function captureCredentialCardRects(container) {
            const rects = {};
            const cards = collectCredentialCards(container);
            for (let i = 0; i < cards.length; i++) {
                const card = cards[i];
                if (!card || typeof card.getBoundingClientRect !== "function") continue;
                const key = (typeof card.getAttribute === "function" && card.getAttribute("data-auth-index")) || "";
                if (!key) continue;
                const rect = card.getBoundingClientRect();
                rects[key] = { left: rect.left, top: rect.top };
            }
            return rects;
        }

        function animateCredentialReorder(container, previousRects) {
            if (!container || !previousRects) return;
            const cards = collectCredentialCards(container);
            const hasPreviousLayout = Object.keys(previousRects).length > 0;
            for (let i = 0; i < cards.length; i++) {
                const card = cards[i];
                if (!card || !card.style) continue;
                if (!hasPreviousLayout) continue;
                const key = (typeof card.getAttribute === "function" && card.getAttribute("data-auth-index")) || "";
                if (!previousRects[key]) {
                    const enterDelay = i * 45;
                    card.classList.add("cred-card-enter");
                    card.style.animationDelay = enterDelay + "ms";
                    setTimeout(function(target) {
                        target.classList.remove("cred-card-enter");
                        target.style.animationDelay = "";
                    }.bind(null, card), enterDelay + 460);
                    continue;
                }
                if (typeof card.getBoundingClientRect !== "function") continue;
                const previous = previousRects[key];
                const next = card.getBoundingClientRect();
                const dx = previous.left - next.left;
                const dy = previous.top - next.top;
                if (Math.abs(dx) < 0.5 && Math.abs(dy) < 0.5) continue;

                const moveDelay = i * 25;
                card.style.transition = "none";
                card.style.transform = "translate(" + dx + "px, " + dy + "px)";
                void card.offsetWidth;
                card.style.transition = "transform 0.48s cubic-bezier(0.22, 1, 0.36, 1) " + moveDelay + "ms";
                card.style.transform = "translate(0px, 0px)";
                setTimeout(function(target) {
                    target.style.transition = "";
                    target.style.transform = "";
                }.bind(null, card), moveDelay + 560);
            }
        }

        function renderDashboard() {
            if (latestDiagnostics) {
                var auditEl = document.getElementById("valLastAudit");
                if (auditEl) auditEl.textContent = latestDiagnostics.latest_audit || "-";

                var sched = latestDiagnostics.scheduler || {};
                var isAutoApplyEnabled = false;
                if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                    isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
                } else if (latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                    isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
                }

                var isPaused = scheduleConfig ? Boolean(scheduleConfig.paused) : Boolean(sched.paused);

                var el = document.getElementById("valNextProbe");
                if (el) {
                    if (!isAutoApplyEnabled) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--text-muted); font-weight:600;\">" + t("scheduleDisabled") + "</span>";
                    } else if (isPaused) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--accent-yellow-text); font-weight:600;\">" + t("schedulePaused") + "</span>";
                    } else if (sched.running || (typeof autoScheduleLoadingActive !== "undefined" && autoScheduleLoadingActive)) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--accent-blue); font-weight:600;\">" + t("scheduleExecuting") + "</span>";
                    } else if (sched.mode === "idle") {
                        var prevReq = Number(sched.prev_window_requests || 0);
                        var currReq = Number(sched.curr_window_requests || 0);
                        var idleTitle = currentLang === "zh-CN"
                            ? ("前窗口请求 " + prevReq + " 次，当前窗口 " + currReq + " 次；无活跃请求时自动跳过")
                            : ("Prev window: " + prevReq + " reqs, current: " + currReq + " reqs; skipped while idle");
                        el.innerHTML = (currentLang === "zh-CN" ? "调度模式: " : "Schedule: ") + "<span title=\"" + escapeHTML(idleTitle) + "\" style=\"color:var(--text-muted); font-weight:600;\">" + t("scheduleModeIdle") + "</span>";
                    } else {
                        var nextRunAt = sched.next_run_at;
                        var nextStr = nextRunAt ? formatCountdown(nextRunAt) : (sched.next_wait || "-");
                        var modeSuffix = sched.mode === "low_frequency" ? (" <span style=\"color:var(--text-muted); font-size:11px;\">(" + t("scheduleModeLowFreq") + ")</span>") : "";
                        el.innerHTML = (currentLang === "zh-CN" ? "下次调度: " : "Next run: ") + "<span class=\"meter-countdown\" data-scheduler-countdown=\"" + (nextRunAt || "") + "\">" + nextStr + "</span>" + modeSuffix;
                    }
                }
            }

            const container = document.getElementById("credentialsContainer");
            if (!container || !latestSnapshot) return;

            const modelInput = document.getElementById("modelSelect");
            const selectedModel = (modelInput && modelInput.value) || "gemini-3.8-flash-high";
            const pool = resolveModelPool(selectedModel);

            const groups = (latestSnapshot && latestSnapshot.groups) || {};
            const groupData = groups[pool] || {};
            const rawItems = groupData.items || [];

            const activeItems = [];
            const bypassedItems = [];

            rawItems.forEach(function(item) {
                const status = resolveItemRoutingStatus(item);
                const eligibility = isModelEligible(item, selectedModel);

                if (!eligibility.eligible) {
                    bypassedItems.push({
                        item: item,
                        reason: eligibility.reason,
                        sortGroup: (eligibility.reason === "unsupported_model" || eligibility.reason === "excluded_model") ? 3 : 4
                    });
                } else if (status === "cooldown") {
                    bypassedItems.push({
                        item: item,
                        reason: "cooldown",
                        sortGroup: 1
                    });
                } else if (status === "weekly_depleted" || status === "short_depleted") {
                    bypassedItems.push({
                        item: item,
                        reason: status,
                        sortGroup: 2
                    });
                } else {
                    activeItems.push(item);
                }
            });

            activeItems.sort(function(a, b) {
                const pa = (a.target && a.target.priority !== undefined) ? a.target.priority : (a.priority || 0);
                const pb = (b.target && b.target.priority !== undefined) ? b.target.priority : (b.priority || 0);
                if (pa !== pb) return pb - pa;

                const ua = a.urgency || 0;
                const ub = b.urgency || 0;
                if (Math.abs(ua - ub) > 1e-4) return ub - ua;

                const ta = a.t5h || (a.short_window_reset_at ? Date.parse(a.short_window_reset_at) : 0);
                const tb = b.t5h || (b.short_window_reset_at ? Date.parse(b.short_window_reset_at) : 0);
                if (ta !== tb) return ta - tb;

                return String(a.auth_index || "").localeCompare(String(b.auth_index || ""));
            });

            let currentTier = 0;
            let lastPriority = null;
            const tieredActive = activeItems.map(function(item) {
                const p = (item.target && item.target.priority !== undefined) ? item.target.priority : (item.priority || 0);
                if (lastPriority === null || p !== lastPriority) {
                    currentTier++;
                    lastPriority = p;
                }
                return { item: item, tier: currentTier, isBypassed: false, reason: "" };
            });

            bypassedItems.sort(function(a, b) {
                if (a.sortGroup !== b.sortGroup) return a.sortGroup - b.sortGroup;
                return String(a.item.auth_index || "").localeCompare(String(b.item.auth_index || ""));
            });
            const tieredBypassed = bypassedItems.map(function(b) {
                return { item: b.item, tier: 0, isBypassed: true, reason: b.reason };
            });

            let renderItems;
            if (!modelInput) {
                renderItems = rawItems.map(function(item, idx) {
                    const status = resolveItemRoutingStatus(item);
                    const isBypassed = status !== "active" && status !== "cold_start";
                    const reason = isBypassed ? (status === "pool_unavailable" ? "unsupported_model" : status) : "";
                    return { item: item, tier: idx + 1, isBypassed: isBypassed, reason: reason };
                });
            } else {
                renderItems = tieredActive.concat(tieredBypassed);
            }

            let boostedCount = 0;
            activeItems.forEach(function(item) {
                if (item.is_boosted) boostedCount++;
            });
            const activeCount = activeItems.length;
            const depletedCount = bypassedItems.length;

            const elTotal = document.getElementById("valTotalCreds");
            if (elTotal) elTotal.textContent = rawItems.length;
            const elDesc = document.getElementById("valTotalDesc");
            if (elDesc) elDesc.textContent = activeCount + " " + (currentLang === "zh-CN" ? "活跃可用" : "active");
            const elBoosted = document.getElementById("valBoosted");
            if (elBoosted) elBoosted.textContent = boostedCount;
            const elDepleted = document.getElementById("valDepleted");
            if (elDepleted) elDepleted.textContent = depletedCount;

            const previousCardRects = captureCredentialCardRects(container);
            container.innerHTML = "";
            if (renderItems.length === 0) {
                container.innerHTML = "<div class=\"empty-state\">" + t("noCreds") + "</div>";
                return;
            }

            renderItems.forEach(function(entry) {
                const item = entry.item;
                const isBypassed = entry.isBypassed;
                const bypassedReason = entry.reason;
                const tier = entry.tier;

                const itemStatus = resolveItemRoutingStatus(item);
                const isManuallyDisabled = itemStatus === "disabled";
                const isWeeklyDepleted = itemStatus === "weekly_depleted";
                const isDisabled = isManuallyDisabled || isWeeklyDepleted;

                const card = document.createElement("div");
                card.className = "cred-card" + (isDisabled ? " cred-card-disabled" : "");

                const isBoosted = Boolean(item.is_boosted);
                const urgency = (item.urgency || 0).toFixed(2);
                const burnRate = (item.cycle_burn_rate || 0).toFixed(2);

                let priorityDisplay = "-";
                if (item.target && item.target.priority !== undefined && item.target.priority !== null) {
                    priorityDisplay = item.target.priority;
                } else if (item.host && item.host.priority !== undefined && item.host.priority !== null) {
                    priorityDisplay = item.host.priority;
                }
                const authIdx = item.auth_index || "";
                const isProbing = probingAuthIndexes.has(String(authIdx));
                card.setAttribute("data-auth-index", String(authIdx));
                if (isProbing) card.classList.add("cred-card-probing");
                const credDisplayName = item.email || "Credential";
                const displayName = formatAccountName(credDisplayName);

                let callOrderBadge = "";
                const customTier = Number(item.custom_tier) || 0;
                const warmupOverridden = Boolean(item.warmup_overridden);
                const warmupActive = Boolean(item.warmup_active);
                const isFreePlan = String(item.plan_type || "").trim().toLowerCase() === "free";
                const warmupIcon = isFreePlan ? "" :
                    "<button type=\"button\" class=\"cred-warmup-icon " + (warmupActive ? "is-on" : "is-off") + "\" " +
                    "aria-pressed=\"" + (warmupActive ? "true" : "false") + "\" " +
                    "title=\"" + escapeHTML(t(warmupOverridden ? "warmupIconOverrideTitle" : "warmupIconFollowTitle")) + "\" " +
                    "onclick=\"toggleCredentialWarmup('" + escapeHTML(authIdx) + "', '" + escapeHTML(pool) + "', " + (warmupActive ? "false" : "true") + ", this)\">♨️</button>";
                if (!isBypassed) {
                    let tierText = "";
                    if (customTier > 0) {
                        tierText = "🎯 Tier #" + customTier + (currentLang === "zh-CN" ? " · 自定义" : " · Custom");
                    } else if (isBoosted) {
                        tierText = "🚀 Tier #" + tier + " · Boost";
                    } else if (itemStatus === "cold_start") {
                        tierText = "Tier #" + tier + (currentLang === "zh-CN" ? " · 待探测" : " · Cold Start");
                    } else {
                        tierText = "Tier #" + tier;
                    }
                    const badgeClass = customTier > 0 ? "badge-boost" : (isBoosted ? "badge-boost" : "badge-tier");
                    callOrderBadge = "<span class=\"badge " + badgeClass + "\" style=\"cursor:pointer;\" onclick=\"openCustomTierModal('" + escapeHTML(authIdx) + "', '" + escapeHTML(credDisplayName) + "', " + customTier + ")\" title=\"" + (currentLang === "zh-CN" ? "点击修改自定义 Tier" : "Click to edit Custom Tier") + "\">" + escapeHTML(tierText) + "</span>";
                } else if (bypassedReason === "cooldown") {
                    callOrderBadge = "<span class=\"badge badge-warning\">" + (currentLang === "zh-CN" ? "⏳ 健康冷却避让" : "⏳ Health Cooldown") + "</span>";
                } else if (bypassedReason === "excluded_model") {
                    callOrderBadge = "<span class=\"badge badge-danger\" style=\"cursor:help;\" title=\"" + escapeHTML(t("excludedModelBadgeTitle")) + "\">" + escapeHTML(t("statusExcludedModel")) + "</span>";
                } else if (bypassedReason === "unsupported_model") {
                    callOrderBadge = "<span class=\"badge badge-secondary\">" + (currentLang === "zh-CN" ? "⛔ 不支持该模型" : "⛔ Unsupported") + "</span>";
                } else if (bypassedReason === "disabled") {
                    callOrderBadge = "<span class=\"badge badge-secondary\">⛔ " + escapeHTML(t("statusDisabled")) + "</span>";
                } else {
                    callOrderBadge = "<span class=\"badge badge-danger\">" + (currentLang === "zh-CN" ? "🚫 耗尽避让" : "🚫 Depleted") + "</span>";
                }

                const isCooldown = itemStatus === "cooldown";
                const probeFailed = Boolean(item.probe_failed);
                let reasonColorClass = "cred-reason-normal";
                if (isCooldown) {
                    reasonColorClass = "cred-reason-warning";
                } else if (probeFailed) {
                    reasonColorClass = "cred-reason-danger";
                } else if (isManuallyDisabled || isWeeklyDepleted) {
                    reasonColorClass = "cred-reason-muted";
                } else if (isBypassed) {
                    reasonColorClass = "cred-reason-danger";
                } else if (isBoosted) {
                    reasonColorClass = "cred-reason-boost";
                }
                const formattedReason = formatReason(item.reason, isBoosted, isDisabled, itemStatus, probeFailed);
                let cardHtml = "<div class=\"cred-info\">" +
                    "<div class=\"cred-name\">" +
                        callOrderBadge +
                        "<span class=\"cred-email\" title=\"" + escapeHTML(displayName) + "\">" + escapeHTML(displayName) + "</span>" +
                        "<span class=\"cred-meta\">" + formatPlanBadge(item.plan_type) + "</span>" +
                        warmupIcon +
                    "</div>" +
                    "<div class=\"cred-badge-row\">" +
                        "<div class=\"metric-pill metric-pill-priority\">" + (currentLang === "zh-CN" ? "动态优先级: " : "Priority: ") + "<strong>" + priorityDisplay + "</strong></div>" +
                        "<div class=\"metric-pill metric-pill-urgency\">" + t("urgencyLabel") + "<strong>" + urgency + "</strong></div>" +
                        "<div class=\"metric-pill metric-pill-burn\">" + t("burnLabel") + "<strong>" + burnRate + "</strong></div>" +
                    "</div>" +
                "</div>";

                // 5 Quota Progress Bars
                const qw = item.quota_windows || {};
                const sharedArgon = qw.argon_shared_with_gemini === true;
                const argonData = (qw.argon_7d && qw.argon_7d.remaining !== null && qw.argon_7d.remaining !== undefined)
                    ? qw.argon_7d
                    : ((item.argon_window_remaining !== null && item.argon_window_remaining !== undefined)
                        ? { remaining: item.argon_window_remaining, reset_at: item.argon_window_reset_at }
                        : qw.argon_7d);
                const bars = [
                    { id: "gemini_5h", kind: "short", label: "Gemini 5h", windowType: t("shortWindow"), data: qw.gemini_5h || { remaining: item.short_window_remaining, reset_at: item.short_window_reset_at }, isActive: (pool === "gemini" || pool === "argon") },
                    { id: "gemini_7d", kind: "weekly", label: "Gemini 7d", windowType: t("longWindow"), data: qw.gemini_7d || { remaining: item.long_window_remaining, reset_at: item.long_window_reset_at }, isActive: (pool === "gemini" || (pool === "argon" && sharedArgon)) },
                    { id: "claude_5h", kind: "short", label: "Claude 5h", windowType: t("shortWindow"), data: qw.claude_5h, isActive: (pool === "claude_gpt") },
                    { id: "claude_7d", kind: "weekly", label: "Claude 7d", windowType: t("longWindow"), data: qw.claude_7d, isActive: (pool === "claude_gpt") },
                    { id: "argon_7d", kind: "weekly", label: "Argon 7d", windowType: t("argonWindow"), data: argonData, isActive: (pool === "argon" && !sharedArgon) }
                ];

                cardHtml += "<div class=\"quota-bars-grid\">";
                bars.forEach(function(b) {
                    const d = b.data || {};
                    const hasVal = d.remaining !== null && d.remaining !== undefined;
                    const pct = hasVal ? Math.max(0, Math.min(100, Math.round(Number(d.remaining)))) : 0;
                    let fillClass = "meter-fill-healthy";
                    if (pct <= 10) fillClass = "meter-fill-danger";
                    else if (pct <= 30) fillClass = "meter-fill-warning";

                    const resetTime = d.reset_at || "";
                    const expiryMode = quotaWindowExpiryMode(b.kind, hasVal ? Number(d.remaining) : null);
                    const resetStr = resetTime ? formatCountdown(resetTime, expiryMode) : "-";

                    let estBurnTag = "";
                    if (b.isActive) {
                        const isShort = b.id.indexOf("5h") >= 0;
                        const estVal = isShort ? (item.estimated_short_burn_pct || 0) : (item.estimated_long_burn_pct || 0);
                        if (estVal > 0) {
                            estBurnTag = " <span class=\"meter-est-burn\">(" + (currentLang === "zh-CN" ? "预估 -" : "est. -") + Math.round(estVal) + "%)</span>";
                        }
                    }

                    const activeClass = b.isActive ? " active-pool-meter" : " inactive-pool-meter";
                    const containerClass = "meter-container" + activeClass;

                    const pctDisplay = hasVal
                        ? " <span class=\"meter-pct\">" + pct + "%</span>"
                        : " <span class=\"meter-na\">(N/A)</span>";

                    cardHtml += "<div class=\"" + containerClass + "\">" +
                        "<div class=\"meter-label-row\">" +
                            "<span class=\"meter-name\" title=\"" + escapeHTML(b.windowType) + "\">" +
                                "<span class=\"meter-pool-name\">" + escapeHTML(b.label) + "</span>" +
                                pctDisplay + estBurnTag +
                            "</span>" +
                            "<span class=\"meter-countdown\" data-reset-time=\"" + escapeHTML(resetTime) + "\" data-expiry-mode=\"" + expiryMode + "\">" + resetStr + "</span>" +
                        "</div>" +
                        "<div class=\"meter-track\">" +
                            "<div class=\"meter-fill " + (hasVal ? fillClass : "") + "\" style=\"width: " + (hasVal ? pct : 0) + "%\"></div>" +
                        "</div>" +
                    "</div>";
                });
                cardHtml += "</div>";

                cardHtml += "<div class=\"cred-footer\">" +
                    "<div class=\"cred-reason " + reasonColorClass + "\" title=\"" + escapeHTML(formattedReason) + "\">" + escapeHTML(formattedReason) + "</div>" +
                    "<div class=\"cred-actions\">" +
                        "<button type=\"button\" class=\"btn-secondary btn-card-action\" onclick=\"openCustomTierModal('" + escapeHTML(authIdx) + "', '" + escapeHTML(credDisplayName) + "', " + customTier + ")\">🎯 " + t("btnTier") + "</button>" +
                        "<button type=\"button\" class=\"btn-secondary btn-card-action\" data-probe-action=\"1\" onclick=\"probeSingleCredential('" + escapeHTML(authIdx) + "', this)\">" + credentialProbeButtonContent(isProbing) + "</button>" +
                        "<button type=\"button\" class=\"btn-secondary btn-card-action\" onclick=\"openSamplesModal('" + escapeHTML(authIdx) + "', '" + escapeHTML(credDisplayName) + "')\">📊 " + t("btnSamples") + "</button>" +
                    "</div>" +
                "</div>";
                card.innerHTML = cardHtml;
                container.appendChild(card);
            });

            animateCredentialReorder(container, previousCardRects);
        }

`
