package management

const templateScriptOverviewRender = `        function renderDashboard() {
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

            const groupSelect = document.getElementById("modelGroupSelect");
            const selectedGroup = groupSelect ? groupSelect.value : "gemini";
            const activeGroup = (latestSnapshot && latestSnapshot.active_model_group) || "gemini";
            const isPredictedView = selectedGroup !== activeGroup;
            const groupData = (latestSnapshot && latestSnapshot.groups && latestSnapshot.groups[selectedGroup]) || {};
            const items = groupData.items || [];
            const pendingAuthIndexes = new Set((groupData.changes || []).map(change => change.auth_index));
            let boostedCount = 0;
            let depletedCount = 0;
            let activeCount = 0;

            items.forEach(item => {
                if (item.is_boosted) boostedCount++;
                if (item.target && item.target.disabled) depletedCount++;
                if (item.target && !item.target.disabled) activeCount++;
            });

            document.getElementById("valTotalCreds").textContent = items.length;
            document.getElementById("valTotalDesc").textContent = activeCount + " " + (currentLang === "zh-CN" ? "活跃可用" : "active");
            document.getElementById("valBoosted").textContent = boostedCount;
            document.getElementById("valDepleted").textContent = depletedCount;

            container.innerHTML = "";
            if (items.length === 0) {
                container.innerHTML = "<div class=\"empty-state\">" + t("noCreds") + "</div>";
                return;
            }

            items.forEach(item => {
                const card = document.createElement("div");
                card.className = "cred-card";

                const isBoosted = item.is_boosted;
                const r5hPercent = Math.max(0, Math.min(100, Math.round((item.r5h || 0) * 100)));
                const r7dPercent = Math.max(0, Math.min(100, Math.round((item.r7d || 0) * 100)));

                let fill5hClass = "meter-fill-healthy";
                if (r5hPercent <= 10) fill5hClass = "meter-fill-danger";
                else if (r5hPercent <= 30) fill5hClass = "meter-fill-warning";

                let fill7dClass = "meter-fill-healthy";
                if (r7dPercent <= 10) fill7dClass = "meter-fill-danger";
                else if (r7dPercent <= 30) fill7dClass = "meter-fill-warning";

                const urgency = (item.urgency || 0).toFixed(2);
                const burnRate = (item.cycle_burn_rate || 0).toFixed(2);

                let actualP = "-";
                if (item.current) {
                    if (item.current.disabled) actualP = currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]";
                    else if (item.current.priority_missing || item.current.priority === undefined || item.current.priority === null) actualP = t("unsetPriority");
                    else actualP = item.current.priority;
                } else if (item.priority_missing || item.priority === undefined || item.priority === null) {
                    actualP = t("unsetPriority");
                } else if (item.priority !== undefined && item.priority !== null) {
                    actualP = item.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : item.priority;
                }

                let targetP = "-";
                if (item.target) {
                    if (item.target.disabled) targetP = currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]";
                    else if (item.target.priority_missing || item.target.priority === undefined || item.target.priority === null) targetP = t("unsetPriority");
                    else targetP = item.target.priority;
                } else {
                    targetP = actualP;
                }

                let tagBadge = "";
                if (isPredictedView) {
                    tagBadge = "<span class=\"badge badge-predicted\">" + t("predictedBadge") + "</span>";
                } else if (pendingAuthIndexes.has(item.auth_index)) {
                    tagBadge = "<span class=\"badge badge-pending\">" + t("pendingApply") + "</span>";
                } else if (String(actualP) !== String(targetP)) {
                    tagBadge = "<span class=\"badge badge-warning\">" + t("targetChanged") + "</span>";
                }

                const probeFailed = item.reason && (item.reason.indexOf("probe failed") >= 0 || item.reason.indexOf("probe invalid") >= 0);
                let statusBadge = "<span class=\"badge badge-success\">" + t("statusActive") + "</span>";
                if (probeFailed) {
                    statusBadge = "<span class=\"badge badge-warning\">" + t("statusFailed") + "</span>";
                } else if (item.target && item.target.disabled) {
                    statusBadge = "<span class=\"badge badge-danger\">" + t("statusWeeklyDepleted") + "</span>";
                } else if (item.reason && item.reason.indexOf("429") >= 0) {
                    statusBadge = "<span class=\"badge badge-warning\">" + t("statusCooldown") + "</span>";
                } else if (isBoosted) {
                    statusBadge = "<span class=\"badge badge-boost\">" + t("statusBoosted") + "</span>";
                }

                const formattedReason = formatReason(item.reason, isBoosted, item.target && item.target.disabled);
                const authIdx = item.auth_index || "";
                const credDisplayName = item.email || "Credential";

                card.innerHTML =
					"<div class=\"cred-info\">" +
						"<div class=\"cred-name\">" +
							"<span class=\"cred-email\">" + escapeHTML(credDisplayName) + "</span>" +
							"<span class=\"cred-meta\"> · " + escapeHTML(item.plan_type || "Antigravity") + "</span>" +
						"</div>" +
                        "<div class=\"cred-badge-row\">" +
                            statusBadge +
                            "<div class=\"metric-pill metric-pill-urgency\">" + t("urgencyLabel") + "<strong>" + urgency + "</strong></div>" +
                            "<div class=\"metric-pill metric-pill-burn\">" + t("burnLabel") + "<strong>" + burnRate + "</strong></div>" +
                            "<button type=\"button\" class=\"btn-secondary\" style=\"min-height:20px; height:20px; padding:0 6px; font-size:11px; margin-left:auto; border-radius:4px;\" onclick=\"openSamplesModal('" + escapeHTML(authIdx) + "', '" + escapeHTML(credDisplayName) + "')\">📊 " + t("btnSamples") + "</button>" +
                        "</div>" +
                    "</div>" +

                    "<div class=\"meter-container\">" +
                        "<div class=\"meter-label-row\">" +
                            "<span>" + t("shortWindow") + " (" + r5hPercent + "%)</span>" +
                            "<span class=\"meter-countdown\" data-reset-time=\"" + (item.short_window_reset_at || "") + "\">" + formatCountdown(item.short_window_reset_at) + "</span>" +
                        "</div>" +
                        "<div class=\"meter-track\">" +
                            "<div class=\"meter-fill " + fill5hClass + "\" style=\"width: " + r5hPercent + "%\"></div>" +
                        "</div>" +
                    "</div>" +

                    "<div class=\"meter-container\">" +
                        "<div class=\"meter-label-row\">" +
                            "<span>" + t("longWindow") + " (" + r7dPercent + "%)</span>" +
                            "<span class=\"meter-countdown\" data-reset-time=\"" + (item.long_window_reset_at || "") + "\">" + formatCountdown(item.long_window_reset_at) + "</span>" +
                        "</div>" +
                        "<div class=\"meter-track\">" +
                            "<div class=\"meter-fill " + fill7dClass + "\" style=\"width: " + r7dPercent + "%\"></div>" +
                        "</div>" +
                    "</div>" +

                    "<div class=\"cred-priority\">" +
                        "<div class=\"priority-score-box\" style=\"flex-wrap:wrap;\">" +
                            "<span style=\"font-size:11.5px; color:var(--text-muted); font-weight:600;\">" + t("actualPriority") + ":</span>" +
                            "<span style=\"font-size:12.5px; font-weight:700; color:var(--text-secondary); font-family:SFMono-Regular, Consolas, Menlo, monospace;\">" + actualP + "</span>" +
                            "<span style=\"color:var(--border-color); margin:0 1px;\">|</span>" +
                            "<span style=\"font-size:11.5px; color:var(--text-muted); font-weight:600;\">" + (isPredictedView ? t("predictedPriority") : t("targetPriority")) + ":</span>" +
                            "<span class=\"priority-score" + (isPredictedView ? " priority-predicted" : "") + "\">" + targetP + "</span>" +
                            tagBadge +
                        "</div>" +
                        "<div style=\"font-size:11px; color:var(--text-secondary);\">" + escapeHTML(formattedReason) + "</div>" +
                    "</div>";

                container.appendChild(card);
            });
        }

`
