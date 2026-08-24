package management

const templateScriptDiagnostics = `        let latestDiagnostics = null;

        function copyDiagnosticsJSON() {
            if (!latestDiagnostics) {
                showToast(currentLang === "zh-CN" ? "暂无诊断数据可复制" : "No diagnostics data to copy", "info");
                return;
            }
            var text = JSON.stringify(latestDiagnostics, null, 2);
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).then(function() {
                    showToast(t("copiedSuccess") || (currentLang === "zh-CN" ? "诊断数据已成功复制到剪贴板" : "Diagnostics JSON copied to clipboard"), "success");
                }).catch(function() {
                    fallbackCopyText(text);
                });
            } else {
                fallbackCopyText(text);
            }
        }

        function fallbackCopyText(text) {
            var textarea = document.createElement("textarea");
            textarea.value = text;
            textarea.style.position = "fixed";
            textarea.style.left = "-9999px";
            textarea.style.top = "0";
            document.body.appendChild(textarea);
            textarea.focus();
            textarea.select();
            try {
                var successful = document.execCommand("copy");
                if (successful) {
                    showToast(t("copiedSuccess") || (currentLang === "zh-CN" ? "诊断数据已成功复制到剪贴板" : "Diagnostics JSON copied to clipboard"), "success");
                } else {
                    showToast("Copy failed", "error");
                }
            } catch (_) {
                showToast("Copy failed", "error");
            }
            document.body.removeChild(textarea);
        }

        function renderDiagnostics() {
            if (!latestDiagnostics) return;

            var sched = latestDiagnostics.scheduler || {};
            var isAutoApplyEnabled = false;
            if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
            } else if (latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
            }

            var isPaused = scheduleConfig ? Boolean(scheduleConfig.paused) : Boolean(sched.paused);
            var isSleeping = false;
            var windowStart = scheduleConfig && scheduleConfig.window_start ? scheduleConfig.window_start : sched.window_start;
            var windowEnd = scheduleConfig && scheduleConfig.window_end ? scheduleConfig.window_end : sched.window_end;
            var windowEnabled = scheduleConfig && scheduleConfig.window_enabled !== undefined ? scheduleConfig.window_enabled : sched.window_enabled;

            if (windowEnabled && windowStart && windowEnd) {
                isSleeping = !isCurrentTimeInScheduleWindow(windowStart, windowEnd);
            }

            // --- 1. KPI Card 1: Scheduler Engine ---
            var schedBadge = document.getElementById("diagSchedBadge");
            var schedInterval = document.getElementById("diagSchedInterval");
            var schedCountdown = document.getElementById("diagSchedCountdown");
            if (schedBadge && schedInterval && schedCountdown) {
                var nextRunAt = sched.next_run_at;
                var nextStr = nextRunAt ? formatCountdown(nextRunAt) : (sched.next_wait || "-");

                if (!isAutoApplyEnabled) {
                    schedBadge.className = "badge badge-subtle";
                    schedBadge.textContent = t("diagStatusDisabled");
                    schedCountdown.innerHTML = "<span style=\"color:var(--text-muted); font-weight:600;\">" + t("scheduleDisabled") + "</span>";
                } else if (isPaused) {
                    schedBadge.className = "badge badge-warning";
                    schedBadge.textContent = t("diagStatusPaused");
                    schedCountdown.innerHTML = "<span style=\"color:var(--accent-yellow-text); font-weight:600;\">" + t("schedulePaused") + "</span>";
                } else if (isSleeping) {
                    schedBadge.className = "badge badge-predicted";
                    schedBadge.textContent = t("diagStatusSleeping");
                    schedCountdown.innerHTML = "<span style=\"color:var(--accent-purple-text); font-weight:600;\">" + t("scheduleSleeping") + "</span>";
                } else {
                    schedBadge.className = "badge badge-success";
                    schedBadge.textContent = t("diagStatusRunning");
                    schedCountdown.innerHTML = (currentLang === "zh-CN" ? "下次运行: " : "Next run: ") + "<span class=\"meter-countdown\" data-scheduler-countdown=\"" + (nextRunAt || "") + "\">" + nextStr + "</span>";
                }
                schedInterval.textContent = sched.interval || "-";
            }

            // --- 2. KPI Card 2: 429 Cooldowns ---
            var cooldowns = (latestDiagnostics.active_cooldowns || []).slice();
            if (cooldowns.length === 0 && latestSnapshot && latestSnapshot.groups) {
                // Fallback scan across snapshot items in case active_cooldowns was not populated
                var groupKey = (document.getElementById("modelGroupSelect") && document.getElementById("modelGroupSelect").value) || (latestSnapshot.active_model_group || "gemini");
                var gData = latestSnapshot.groups[groupKey] || {};
                (gData.items || []).forEach(function(it) {
                    var r = (it.reason || "").toLowerCase();
                    if (r.indexOf("429") >= 0 || r.indexOf("cooldown") >= 0) {
                        cooldowns.push({
                            email: it.email || "Credential",
                            model_group: groupKey,
                            reason: it.reason || "429 rate limit cooldown",
                            cooldown_until: (it.short_window_reset_at || it.reset_at || null)
                        });
                    }
                });
            }
            var cdBadge = document.getElementById("diagCooldownBadge");
            var cdCount = document.getElementById("diagCooldownCount");
            var cdSub = document.getElementById("diagCooldownSub");
            if (cdBadge && cdCount && cdSub) {
                if (cooldowns.length === 0) {
                    cdBadge.className = "badge badge-success";
                    cdBadge.textContent = t("diagHealthy");
                    cdCount.textContent = "0";
                    cdCount.style.color = "var(--text-primary)";
                    cdSub.textContent = t("diagCooldownSubText");
                } else {
                    cdBadge.className = "badge badge-danger";
                    cdBadge.textContent = t("diagTripped");
                    cdCount.textContent = cooldowns.length + " " + t("diagCoolingDownCount");
                    cdCount.style.color = "var(--accent-red-text)";
                    cdSub.textContent = (currentLang === "zh-CN" ? "已自动降权至 -1 保护中" : "Demoted to -1 for protection");
                }
            }

            // --- 3. KPI Card 3: Latest Apply Health ---
			var latestApply = latestDiagnostics.latest_apply || null;
			var succ = latestApply ? (latestApply.succeeded || 0) : 0;
			var fail = latestApply ? (latestApply.failed || 0) : 0;
			var skip = latestApply ? (latestApply.skipped || 0) : 0;
			var attempted = latestApply ? (latestApply.attempted || 0) : 0;
            var lastRunTime = (latestApply && latestApply.at) || null;

            var applyBadge = document.getElementById("diagApplyBadge");
            var applyStats = document.getElementById("diagApplyStats");
            var applyTime = document.getElementById("diagApplyTime");
            if (applyBadge && applyStats && applyTime) {
                if (!lastRunTime && (attempted === 0 && succ === 0 && fail === 0 && skip === 0)) {
                    applyBadge.className = "badge badge-subtle";
                    applyBadge.textContent = t("diagNoLastRun");
                    applyStats.textContent = "-";
                    applyTime.textContent = t("diagNoLastRun");
                } else if (fail > 0) {
                    applyBadge.className = "badge badge-danger";
                    applyBadge.textContent = t("diagHasFailed");
                    applyStats.textContent = succ + " " + t("diagSuccessPill") + " · " + fail + " " + t("diagFailedPill");
                    applyTime.textContent = lastRunTime ? new Date(lastRunTime).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                } else if (succ > 0) {
                    applyBadge.className = "badge badge-success";
                    applyBadge.textContent = t("diagAllPassed");
                    applyStats.textContent = succ + " " + t("diagSuccessPill") + " · " + skip + " " + t("diagSkippedPill");
                    applyTime.textContent = lastRunTime ? new Date(lastRunTime).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                } else {
                    applyBadge.className = "badge badge-subtle";
                    applyBadge.textContent = t("diagNoChanges");
                    applyStats.textContent = skip + " " + t("diagSkippedPill");
                    applyTime.textContent = lastRunTime ? new Date(lastRunTime).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                }
            }

            // --- 4. Section 1: Scheduling Details & Window Panel ---
            var detInterval = document.getElementById("diagSchedDetailInterval");
            var detWorker = document.getElementById("diagSchedDetailWorker");
            var detLastRun = document.getElementById("diagSchedDetailLastRun");
            var detNextRun = document.getElementById("diagSchedDetailNextRun");
            var detWinText = document.getElementById("diagSchedDetailWindowText");
            var detWinBadge = document.getElementById("diagSchedDetailWindowBadge");

            if (detInterval) detInterval.textContent = sched.interval || "-";
            if (detWorker) {
                var workerActive = Boolean(sched.worker_active);
                detWorker.innerHTML = workerActive
                    ? "<span class=\"badge badge-success\" style=\"font-size:11px;\">" + t("diagWorkerActive") + "</span>"
                    : "<span class=\"badge badge-subtle\" style=\"font-size:11px;\">" + t("diagWorkerInactive") + "</span>";
            }
            if (detLastRun) {
                detLastRun.textContent = sched.last_auto_apply_at ? new Date(sched.last_auto_apply_at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : t("diagNoLastRun");
            }
            if (detNextRun) {
                detNextRun.textContent = sched.next_run_at ? new Date(sched.next_run_at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
            }
            if (detWinText && detWinBadge) {
                if (!windowEnabled || !windowStart || !windowEnd) {
                    detWinText.textContent = t("diagWindowContinuous");
                    detWinBadge.className = "badge badge-success";
                    detWinBadge.textContent = "24/7";
                } else {
                    detWinText.textContent = windowStart + " - " + windowEnd;
                    if (isSleeping) {
                        detWinBadge.className = "badge badge-predicted";
                        detWinBadge.textContent = t("diagWindowSleeping");
                    } else {
                        detWinBadge.className = "badge badge-success";
                        detWinBadge.textContent = t("diagWindowActive");
                    }
                }
            }

            // --- 5. Section 2: 429 Cooldown Content ---
            var cdContent = document.getElementById("diagCooldownContent");
            if (cdContent) {
                if (cooldowns.length === 0) {
                    cdContent.innerHTML = "<div class=\"diag-cooldown-empty\">" +
                        "<span>✅</span>" +
                        "<span>" + t("diagCooldownEmptyText") + "</span>" +
                    "</div>";
                } else {
                    var html = "<div class=\"diag-cooldown-list\">";
                    cooldowns.forEach(function(c) {
                        var cdUntil = c.cooldown_until;
                        var cdCountdownStr = cdUntil ? formatCountdown(cdUntil) : "-";
                        var groupTag = c.model_group ? "<span class=\"badge badge-subtle\" style=\"font-size:10px;\">" + escapeHTML(c.model_group) + "</span>" : "";
                        html += "<div class=\"diag-cooldown-item\">" +
                            "<div style=\"display:flex; flex-direction:column; gap:4px; min-width:0;\">" +
                                "<div style=\"display:flex; align-items:center; gap:6px;\">" +
                                    "<strong style=\"font-size:13px; color:var(--text-primary);\">" + escapeHTML(c.email || "Credential") + "</strong>" +
                                    groupTag +
                                    "<span class=\"badge badge-danger\" style=\"font-size:10px;\">429 Cooldown</span>" +
                                "</div>" +
                                "<div style=\"font-size:11.5px; color:var(--text-muted); font-family:monospace;\">" + escapeHTML(c.reason || "429 rate limit") + "</div>" +
                            "</div>" +
                            "<div style=\"text-align:right; font-size:12px;\">" +
                                "<div style=\"color:var(--text-muted); font-size:11px;\">" + t("diagRemainingSec") + "</div>" +
                                "<strong class=\"meter-countdown\" data-cooldown-countdown=\"" + (cdUntil || "") + "\" style=\"font-family:monospace; color:var(--accent-yellow-text); font-size:13px;\">" + cdCountdownStr + "</strong>" +
                            "</div>" +
                        "</div>";
                    });
                    html += "</div>";
                    cdContent.innerHTML = html;
                }
            }

            // --- 6. Section 3: Latest Audit & Metrics ---
            var auditText = document.getElementById("diagAuditText");
            var auditPills = document.getElementById("diagAuditPills");
            if (auditText) {
				auditText.textContent = latestApply ? (latestApply.message || "-") : (currentLang === "zh-CN" ? "尚无 Apply 写入记录" : "No Apply write recorded yet");
            }
            if (auditPills) {
                var succCount = succ;
                var failCount = fail;
                var skipCount = skip;
                var attCount = attempted;

                auditPills.innerHTML =
                    "<span class=\"badge badge-success\">" + succCount + " " + t("diagSuccessPill") + "</span>" +
                    "<span class=\"badge badge-danger\">" + failCount + " " + t("diagFailedPill") + "</span>" +
                    "<span class=\"badge badge-subtle\">" + skipCount + " " + t("diagSkippedPill") + "</span>" +
                    "<span class=\"badge badge-subtle\" style=\"font-weight:700;\">" + attCount + " " + t("diagAttemptedPill") + "</span>";
            }
        }

`
