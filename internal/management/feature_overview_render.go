package management

const templateScriptOverviewRender = `        function resolveModelPool(modelId) {
            if (!modelId) return "gemini";
            let m = String(modelId).toLowerCase().trim();
            const slashIdx = m.lastIndexOf("/");
            if (slashIdx >= 0) m = m.substring(slashIdx + 1);
            const parenIdx = m.indexOf("(");
            if (parenIdx >= 0) m = m.substring(0, parenIdx).trim();
            if (m.indexOf("argon") >= 0) return "argon";
            if (m.indexOf("claude") >= 0 || m.indexOf("opus") >= 0 || m.indexOf("sonnet") >= 0 ||
                m.indexOf("haiku") >= 0 || m.indexOf("fable") >= 0 || m.indexOf("gpt") >= 0 ||
                m.indexOf("o1") >= 0 || m.indexOf("o3") >= 0 || m.indexOf("o4") >= 0 || m.indexOf("openai") >= 0) {
                return "claude_gpt";
            }
            return "gemini";
        }

        function isModelEligible(item, selectedModel, pool) {
            if (item && item.host && item.host.disabled) {
                return { eligible: false, reason: "disabled" };
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
            if (pool === "claude_gpt" && item) {
                const qw = item.quota_windows || {};
                const c5h = qw.claude_5h || {};
                const c7d = qw.claude_7d || {};
                const g5h = qw.gemini_5h || {};
                const g7d = qw.gemini_7d || {};
                const hasGemini = (g5h.remaining !== null && g5h.remaining !== undefined) ||
                                  (g7d.remaining !== null && g7d.remaining !== undefined);
                const hasClaude = (c5h.remaining !== null && c5h.remaining !== undefined) ||
                                  (c7d.remaining !== null && c7d.remaining !== undefined);
                if (!hasClaude && hasGemini) {
                    return { eligible: false, reason: "unsupported_model" };
                }
            }
            if (pool === "argon" && item) {
                const qw = item.quota_windows || {};
                const a7d = qw.argon_7d || {};
                const g7d = qw.gemini_7d || {};
                const hasGemini = (g7d.remaining !== null && g7d.remaining !== undefined);
                const hasArgon = (a7d.remaining !== null && a7d.remaining !== undefined);
                if (!hasArgon && hasGemini) {
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
                var isSleeping = false;
                if (scheduleConfig && scheduleConfig.window_enabled && scheduleConfig.window_start && scheduleConfig.window_end) {
                    isSleeping = !isCurrentTimeInScheduleWindow(scheduleConfig.window_start, scheduleConfig.window_end);
                } else if (sched.window_enabled && sched.window_start && sched.window_end) {
                    isSleeping = !isCurrentTimeInScheduleWindow(sched.window_start, sched.window_end);
                }

                var el = document.getElementById("valNextProbe");
                if (el) {
                    if (!isAutoApplyEnabled) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--text-muted); font-weight:600;\">" + t("scheduleDisabled") + "</span>";
                    } else if (isPaused) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--accent-yellow-text); font-weight:600;\">" + t("schedulePaused") + "</span>";
                    } else if (isSleeping) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--accent-purple-text); font-weight:600;\">" + t("scheduleSleeping") + "</span>";
                    } else {
                        var nextRunAt = sched.next_run_at;
                        var nextStr = nextRunAt ? formatCountdown(nextRunAt) : (sched.next_wait || "-");
                        el.innerHTML = (currentLang === "zh-CN" ? "下次调度: " : "Next run: ") + "<span class=\"meter-countdown\" data-scheduler-countdown=\"" + (nextRunAt || "") + "\">" + nextStr + "</span>";
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
                const eligibility = isModelEligible(item, selectedModel, pool);
                const isCooldown = Boolean(item.reason && (item.reason.indexOf("rate limit cooldown") >= 0 || item.reason.indexOf("cooldown") >= 0));
                const targetP = (item.target && item.target.priority !== undefined) ? item.target.priority : (item.priority !== undefined ? item.priority : 0);
                const isManuallyDisabled = item.reason && item.reason.indexOf("disabled on host") >= 0;
                const isWeeklyDepleted = (item.target && item.target.disabled && !isManuallyDisabled) || targetP <= -2;
                const isSoftDepleted = targetP === -1;

                if (!eligibility.eligible) {
                    bypassedItems.push({
                        item: item,
                        reason: eligibility.reason,
                        sortGroup: eligibility.reason === "unsupported_model" ? 3 : 4
                    });
                } else if (isCooldown) {
                    bypassedItems.push({
                        item: item,
                        reason: "cooldown",
                        sortGroup: 1
                    });
                } else if (isSoftDepleted || isWeeklyDepleted) {
                    bypassedItems.push({
                        item: item,
                        reason: isWeeklyDepleted ? "weekly_depleted" : "soft_depleted",
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
                    const isCooldown = Boolean(item.reason && (item.reason.indexOf("rate limit cooldown") >= 0 || item.reason.indexOf("cooldown") >= 0));
                    const targetP = (item.target && item.target.priority !== undefined) ? item.target.priority : (item.priority !== undefined ? item.priority : 0);
                    const isWeeklyDepleted = (item.target && item.target.disabled) || targetP <= -2;
                    const isSoftDepleted = targetP === -1;
                    let reason = "";
                    if (isCooldown) reason = "cooldown";
                    else if (isWeeklyDepleted) reason = "weekly_depleted";
                    else if (isSoftDepleted) reason = "soft_depleted";
                    return { item: item, tier: idx + 1, isBypassed: (isCooldown || isWeeklyDepleted || isSoftDepleted), reason: reason };
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

                const isManuallyDisabled = Boolean(item.reason && item.reason.indexOf("disabled on host") >= 0);
                const isWeeklyDepleted = Boolean(item.target && item.target.disabled);
                const isDisabled = isManuallyDisabled || isWeeklyDepleted;

                const card = document.createElement("div");
                card.className = "cred-card" + (isDisabled ? " cred-card-disabled" : "");

                const isBoosted = Boolean(item.is_boosted);
                const urgency = (item.urgency || 0).toFixed(2);
                const burnRate = (item.cycle_burn_rate || 0).toFixed(2);

                let priorityDisplay = "-";
                if (isDisabled) {
                    priorityDisplay = "-1";
                } else if (item.target) {
                    if (item.target.priority_missing || item.target.priority === undefined || item.target.priority === null) {
                        priorityDisplay = t("unsetPriority");
                    } else {
                        priorityDisplay = item.target.priority;
                    }
                } else if (item.host && item.host.priority !== undefined && item.host.priority !== null) {
                    priorityDisplay = item.host.priority;
                }
                const authIdx = item.auth_index || "";
                const isProbing = probingAuthIndexes.has(String(authIdx));
                card.setAttribute("data-auth-index", String(authIdx));
                if (isProbing) card.classList.add("cred-card-probing");
                const credDisplayName = item.email || "Credential";
                const maskedEmail = formatMaskedEmail(credDisplayName);

                let callOrderBadge = "";
                const customTier = Number(item.custom_tier) || 0;
                if (!isBypassed) {
                    let tierText = "";
                    if (customTier > 0) {
                        tierText = "🎯 Tier #" + customTier + (currentLang === "zh-CN" ? " · 自定义" : " · Custom");
                    } else if (isBoosted) {
                        tierText = "🚀 Tier #" + tier + " · Boost";
                    } else {
                        tierText = "Tier #" + tier;
                    }
                    const badgeClass = customTier > 0 ? "badge-boost" : (isBoosted ? "badge-boost" : "badge-tier");
                    callOrderBadge = "<span class=\"badge " + badgeClass + "\" style=\"cursor:pointer;\" onclick=\"openCustomTierModal('" + escapeHTML(authIdx) + "', '" + escapeHTML(credDisplayName) + "', " + customTier + ")\" title=\"" + (currentLang === "zh-CN" ? "点击修改自定义 Tier" : "Click to edit Custom Tier") + "\">" + escapeHTML(tierText) + "</span>";
                } else if (bypassedReason === "cooldown") {
                    callOrderBadge = "<span class=\"badge badge-warning\">" + (currentLang === "zh-CN" ? "⏳ 健康冷却避让" : "⏳ Health Cooldown") + "</span>";
                } else if (bypassedReason === "unsupported_model") {
                    callOrderBadge = "<span class=\"badge badge-secondary\">" + (currentLang === "zh-CN" ? "⛔ 不支持该模型" : "⛔ Unsupported") + "</span>";
                } else if (bypassedReason === "disabled") {
                    callOrderBadge = "<span class=\"badge badge-secondary\">" + (currentLang === "zh-CN" ? "⛔ 不可用" : "⛔ Unavailable") + "</span>";
                } else {
                    callOrderBadge = "<span class=\"badge badge-danger\">" + (currentLang === "zh-CN" ? "🚫 耗尽避让" : "🚫 Depleted") + "</span>";
                }

                const isCooldown = Boolean(item.reason && (item.reason.indexOf("rate limit cooldown") >= 0 || item.reason.indexOf("cooldown") >= 0));
                const probeFailed = item.reason && (item.reason.indexOf("probe failed") >= 0 || item.reason.indexOf("probe invalid") >= 0);
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
                const formattedReason = formatReason(item.reason, isBoosted, item.target && item.target.disabled);

                let cardHtml = "<div class=\"cred-info\">" +
                    "<div class=\"cred-name\">" +
                        callOrderBadge +
                        "<span class=\"cred-email\" title=\"" + escapeHTML(credDisplayName) + "\">" + escapeHTML(maskedEmail) + "</span>" +
                        "<span class=\"cred-meta\">" + formatPlanBadge(item.plan_type) + "</span>" +
                    "</div>" +
                    "<div class=\"cred-badge-row\">" +
                        "<div class=\"metric-pill metric-pill-priority\">" + (currentLang === "zh-CN" ? "动态优先级: " : "Priority: ") + "<strong>" + priorityDisplay + "</strong></div>" +
                        "<div class=\"metric-pill metric-pill-urgency\">" + t("urgencyLabel") + "<strong>" + urgency + "</strong></div>" +
                        "<div class=\"metric-pill metric-pill-burn\">" + t("burnLabel") + "<strong>" + burnRate + "</strong></div>" +
                    "</div>" +
                "</div>";

                // 5 Quota Progress Bars
                const qw = item.quota_windows || {};
                const argonData = (qw.argon_7d && qw.argon_7d.remaining !== null && qw.argon_7d.remaining !== undefined)
                    ? qw.argon_7d
                    : ((item.argon_window_remaining !== null && item.argon_window_remaining !== undefined)
                        ? { remaining: item.argon_window_remaining, reset_at: item.argon_window_reset_at }
                        : qw.argon_7d);
                const bars = [
                    { id: "gemini_5h", kind: "short", label: "Gemini 5h", windowType: t("shortWindow"), data: qw.gemini_5h || { remaining: item.short_window_remaining, reset_at: item.short_window_reset_at }, isActive: (pool === "gemini" || pool === "argon") },
                    { id: "gemini_7d", kind: "weekly", label: "Gemini 7d", windowType: t("longWindow"), data: qw.gemini_7d || { remaining: item.long_window_remaining, reset_at: item.long_window_reset_at }, isActive: (pool === "gemini") },
                    { id: "claude_5h", kind: "short", label: "Claude 5h", windowType: t("shortWindow"), data: qw.claude_5h, isActive: (pool === "claude_gpt") },
                    { id: "claude_7d", kind: "weekly", label: "Claude 7d", windowType: t("longWindow"), data: qw.claude_7d, isActive: (pool === "claude_gpt") },
                    { id: "argon_7d", kind: "weekly", label: "Argon 7d", windowType: t("argonWindow"), data: argonData, isActive: (pool === "argon") }
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
