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
            var panel = document.getElementById("panelDiagnostics");
            if (panel && panel.hidden) return;

            renderHealthBanner();
            renderSchedulerKpi();
            renderIsolationKpi();
            renderLastRunKpi();
            renderPoolMatrix();
            renderPlanTable();
            renderHealthSection();
            renderRunTimeline();
            renderStorage();
        }

        function diagGroupKeys() {
            return ["gemini", "argon", "claude_gpt"];
        }

        function diagPoolLabel(key) {
            if (key === "argon") return t("poolArgon");
            if (key === "claude_gpt") return t("poolClaudeGPT");
            return t("poolGemini");
        }

        function diagPoolBadgeClass(key) {
            if (key === "argon") return "badge-argon";
            if (key === "claude_gpt") return "badge-claude";
            return "badge-gemini";
        }

        function diagGroupItems(key) {
            if (!latestSnapshot || !latestSnapshot.groups) return [];
            var group = latestSnapshot.groups[key];
            return group && Array.isArray(group.items) ? group.items : [];
        }

        function diagAllItems() {
            var out = [];
            diagGroupKeys().forEach(function(key) {
                diagGroupItems(key).forEach(function(item) {
                    out.push({ group: key, item: item });
                });
            });
            return out;
        }

        function diagTargetPriority(item) {
            var target = item && item.target ? item.target : {};
            if (target.priority !== undefined && target.priority !== null) return target.priority;
            if (item && item.priority !== undefined && item.priority !== null) return item.priority;
            return 0;
        }

        function diagIsDepleted(item) {
            var target = item && item.target ? item.target : {};
            return Boolean(target.disabled) || diagTargetPriority(item) === -1;
        }

        function diagIsCooldown(item) {
            return String((item && item.reason) || "").toLowerCase().indexOf("cooldown") >= 0;
        }

        function diagDecision(item) {
            if (diagIsCooldown(item)) return { key: "cooldown", label: t("statusBypassedCooldown"), cls: "badge-warning" };
            if (diagIsDepleted(item)) return { key: "depleted", label: t("statusBypassedDepleted"), cls: "badge-danger" };
            if (item && item.is_boosted) return { key: "boost", label: t("statusBoosted"), cls: "badge-boost" };
            var customTier = Number(item && item.custom_tier) || 0;
            if (customTier > 0) return { key: "custom", label: t("tierPrefix") + customTier, cls: "badge-boost" };
            return { key: "normal", label: t("statusActive"), cls: "badge-tier" };
        }

        function diagFormatHours(value) {
            var num = Number(value);
            if (!Number.isFinite(num) || num <= 0) return "-";
            if (num >= 48) return (num / 24).toFixed(1) + "d";
            return num.toFixed(1) + "h";
        }

        function diagFormatPct(value) {
            if (value === null || value === undefined) return "-";
            var num = Number(value);
            if (!Number.isFinite(num)) return "-";
            return Math.round(num) + "%";
        }

        function diagMinRemaining(current, bar) {
            if (!bar || bar.remaining === null || bar.remaining === undefined) return current;
            var value = Number(bar.remaining);
            if (!Number.isFinite(value)) return current;
            return current === null ? value : Math.min(current, value);
        }

        function diagNearestReset(current, bar) {
            if (!bar || !bar.reset_at) return current;
            var ts = Date.parse(bar.reset_at);
            if (!Number.isFinite(ts)) return current;
            return current === null ? ts : Math.min(current, ts);
        }

        function renderHealthBanner() {
            var badge = document.getElementById("diagHealthBadge");
            var text = document.getElementById("diagHealthText");
            if (!badge || !text) return;

            var cooldowns = (latestDiagnostics.active_cooldowns || []).length;
            var pending = (latestDiagnostics.pending_429 || []).length;
            var storm = latestDiagnostics.rate_limit_storm || {};
            var runs = latestDiagnostics.run_history || [];
            var lastRun = latestDiagnostics.latest_apply || (runs.length > 0 ? runs[0] : null);
            var failures = lastRun ? (lastRun.failed || 0) + (lastRun.uncertain || 0) + (lastRun.conflicts || 0) : 0;
            var storageErrors = latestDiagnostics.storage && Array.isArray(latestDiagnostics.storage.load_errors) ? latestDiagnostics.storage.load_errors.length : 0;

            var critical = Boolean(storm.active) || failures > 0 || storageErrors > 0;
            var warning = cooldowns > 0 || pending > 0;
            var details = [];
            if (storm.active) details.push(t("diagHealthStorm"));
            if (failures > 0) details.push(failures + " " + t("diagFailedPill"));
            if (cooldowns > 0) details.push(cooldowns + " " + t("diagCoolingDownCount"));
            if (pending > 0) details.push(pending + " " + t("diagPending429Title"));
            if (storageErrors > 0) details.push(t("diagHealthStorageError"));

            if (critical) {
                badge.className = "badge badge-danger";
                badge.textContent = t("diagHealthCritical");
            } else if (warning) {
                badge.className = "badge badge-warning";
                badge.textContent = t("diagHealthWarning");
            } else {
                badge.className = "badge badge-success";
                badge.textContent = t("diagHealthOk");
            }
            text.textContent = details.length > 0 ? details.join(" · ") : t("diagHealthOk");
        }

        function renderSchedulerKpi() {
            var sched = latestDiagnostics.scheduler || {};
            var autoApply = dynamicConfig && dynamicConfig.auto_apply !== undefined
                ? Boolean(dynamicConfig.auto_apply)
                : Boolean(latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply);
            var paused = scheduleConfig ? Boolean(scheduleConfig.paused) : Boolean(sched.paused);
            var windowEnabled = scheduleConfig ? Boolean(scheduleConfig.window_enabled) : Boolean(sched.window_enabled);
            var windowStart = scheduleConfig ? scheduleConfig.window_start : sched.window_start;
            var windowEnd = scheduleConfig ? scheduleConfig.window_end : sched.window_end;
            var sleeping = windowEnabled && windowStart && windowEnd && !isCurrentTimeInScheduleWindow(windowStart, windowEnd);

            var badge = document.getElementById("diagSchedBadge");
            var value = document.getElementById("diagSchedInterval");
            var desc = document.getElementById("diagSchedCountdown");
            if (value) value.textContent = sched.interval || "-";
            if (!badge || !desc) return;

            if (!autoApply) {
                badge.className = "badge badge-subtle";
                badge.textContent = t("diagStatusDisabled");
                desc.textContent = t("scheduleDisabled");
            } else if (paused) {
                badge.className = "badge badge-warning";
                badge.textContent = t("diagStatusPaused");
                desc.textContent = t("schedulePaused");
            } else if (sleeping) {
                badge.className = "badge badge-predicted";
                badge.textContent = t("diagStatusSleeping");
                desc.textContent = t("scheduleSleeping");
            } else {
                badge.className = "badge badge-success";
                badge.textContent = t("diagStatusRunning");
                var next = sched.next_run_at || "";
                desc.innerHTML = (currentLang === "zh-CN" ? "下次运行: " : "Next run: ") +
                    "<span class=\"meter-countdown\" data-scheduler-countdown=\"" + escapeHTML(next) + "\">" + (next ? formatCountdown(next) : "-") + "</span>";
            }
        }

        function renderIsolationKpi() {
            var cooldowns = latestDiagnostics.active_cooldowns || [];
            var pending = latestDiagnostics.pending_429 || [];
            var storm = latestDiagnostics.rate_limit_storm || {};
            var badge = document.getElementById("diagCooldownBadge");
            var count = document.getElementById("diagCooldownCount");
            var sub = document.getElementById("diagCooldownSub");
            if (!badge || !count || !sub) return;

            count.textContent = String(cooldowns.length);
            if (storm.active) {
                badge.className = "badge badge-danger";
                badge.textContent = t("diagTripped");
                count.style.color = "var(--accent-red-text)";
                sub.textContent = t("diagHealthStorm");
                return;
            }
            if (cooldowns.length > 0) {
                badge.className = "badge badge-warning";
                badge.textContent = t("diagTripped");
                count.style.color = "var(--accent-yellow-text)";
            } else {
                badge.className = "badge badge-success";
                badge.textContent = t("diagHealthy");
                count.style.color = "var(--text-primary)";
            }
            sub.textContent = pending.length > 0
                ? pending.length + " " + t("diagPending429Title")
                : t("diagCooldownSubText");
        }

        function renderLastRunKpi() {
            var runs = latestDiagnostics.run_history || [];
            var run = latestDiagnostics.latest_apply || (runs.length > 0 ? runs[0] : null);
            var badge = document.getElementById("diagApplyBadge");
            var stats = document.getElementById("diagApplyStats");
            var time = document.getElementById("diagApplyTime");
            if (!badge || !stats || !time) return;
            if (!run) {
                badge.className = "badge badge-subtle";
                badge.textContent = t("diagNoLastRun");
                stats.textContent = "-";
                time.textContent = t("diagNoLastRun");
                return;
            }
            var succeeded = run.succeeded || 0;
            var failed = run.failed || 0;
            var attempted = run.attempted || 0;
            stats.textContent = succeeded + " " + t("diagSuccessPill") + " / " + attempted + " " + t("diagAttemptedPill");
            time.textContent = (run.at ? new Date(run.at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-") + " · " + formatHistoryKind(run);
            if (failed > 0) {
                badge.className = "badge badge-danger";
                badge.textContent = t("diagHasFailed");
            } else if (succeeded > 0) {
                badge.className = "badge badge-success";
                badge.textContent = t("diagAllPassed");
            } else {
                badge.className = "badge badge-subtle";
                badge.textContent = t("diagNoChanges");
            }
        }

        function renderPoolMatrix() {
            var tbody = document.getElementById("diagPoolMatrix");
            if (!tbody) return;
            var html = "";
            diagGroupKeys().forEach(function(key) {
                var items = diagGroupItems(key);
                var active = 0;
                var boosted = 0;
                var depleted = 0;
                var cooling = 0;
                var min5h = null;
                var min7d = null;
                var nearestReset = null;
                var burnSum = 0;
                var burnCount = 0;

                items.forEach(function(item) {
                    var decision = diagDecision(item);
                    if (decision.key === "cooldown") cooling++;
                    else if (decision.key === "depleted") depleted++;
                    else {
                        active++;
                        if (item.is_boosted) boosted++;
                    }
                    var qw = item.quota_windows || {};
                    var shortBar = key === "claude_gpt" ? qw.claude_5h : (key === "gemini" ? qw.gemini_5h : null);
                    var longBar = key === "claude_gpt" ? qw.claude_7d : (key === "argon" ? qw.argon_7d : qw.gemini_7d);
                    min5h = diagMinRemaining(min5h, shortBar);
                    min7d = diagMinRemaining(min7d, longBar);
                    nearestReset = diagNearestReset(nearestReset, shortBar);
                    nearestReset = diagNearestReset(nearestReset, longBar);
                    var burn = Number(item.cycle_burn_rate);
                    if (Number.isFinite(burn) && burn > 0) {
                        burnSum += burn;
                        burnCount++;
                    }
                });

                html += "<tr>" +
                    "<td><span class=\"badge " + diagPoolBadgeClass(key) + "\">" + escapeHTML(diagPoolLabel(key)) + "</span></td>" +
                    "<td class=\"diag-mono\">" + active + "</td>" +
                    "<td class=\"diag-mono\">" + boosted + "</td>" +
                    "<td class=\"diag-mono\">" + depleted + "</td>" +
                    "<td class=\"diag-mono\">" + cooling + "</td>" +
                    "<td class=\"diag-mono\">" + diagFormatPct(min5h) + "</td>" +
                    "<td class=\"diag-mono\">" + diagFormatPct(min7d) + "</td>" +
                    "<td class=\"diag-mono\">" + (nearestReset ? formatCountdown(nearestReset) : "-") + "</td>" +
                    "<td class=\"diag-mono\">" + (burnCount > 0 ? (burnSum / burnCount).toFixed(2) : "-") + "</td>" +
                "</tr>";
            });
            tbody.innerHTML = html || "<tr><td colspan=\"9\" class=\"diag-empty\">" + t("loading") + "</td></tr>";
        }

        function renderPlanTable() {
            var tbody = document.getElementById("diagPlanTable");
            if (!tbody) return;
            var entries = diagAllItems();
            entries.sort(function(left, right) {
                var lp = diagTargetPriority(left.item);
                var rp = diagTargetPriority(right.item);
                if (lp !== rp) return rp - lp;
                var lu = Number(left.item.urgency) || 0;
                var ru = Number(right.item.urgency) || 0;
                if (Math.abs(lu - ru) > 1e-4) return ru - lu;
                return String(left.item.auth_index || "").localeCompare(String(right.item.auth_index || ""));
            });
            if (entries.length === 0) {
                tbody.innerHTML = "<tr><td colspan=\"7\" class=\"diag-empty\">" + t("loading") + "</td></tr>";
                return;
            }

            var html = "";
            entries.forEach(function(entry) {
                var item = entry.item;
                var decision = diagDecision(item);
                var host = item.host && item.host.priority !== undefined && item.host.priority !== null && !item.host.priority_missing
                    ? item.host.priority
                    : "-";
                var target = diagTargetPriority(item);
                var reason = String(item.reason || "-");
                var credDisplayName = item.email || "Credential";
                var maskedCredential = formatMaskedEmail(credDisplayName);
                html += "<tr>" +
                    "<td><div class=\"diag-cred\">" +
                        "<span class=\"diag-cred-name\" title=\"" + escapeHTML(maskedCredential) + "\">" + escapeHTML(maskedCredential) + "</span>" +
                        "<span class=\"badge " + diagPoolBadgeClass(entry.group) + " diag-cred-pool\">" + escapeHTML(diagPoolLabel(entry.group)) + "</span>" +
                    "</div></td>" +
                    "<td class=\"diag-mono\">" + escapeHTML(host + " → " + target) + "</td>" +
                    "<td><span class=\"badge " + decision.cls + "\">" + escapeHTML(decision.label) + "</span></td>" +
                    "<td class=\"diag-mono\">" + (Number(item.urgency) || 0).toFixed(2) + "</td>" +
                    "<td class=\"diag-mono\">" + diagFormatHours(item.t7d) + " / " + diagFormatHours(item.t_required) + "</td>" +
                    "<td class=\"diag-mono\">" + (Number(item.cycle_burn_rate) || 0).toFixed(2) + "</td>" +
                    "<td class=\"diag-reason\" title=\"" + escapeHTML(reason) + "\">" + escapeHTML(reason) + "</td>" +
                "</tr>";
            });
            tbody.innerHTML = html;
        }

        function renderHealthSection() {
            var storm = latestDiagnostics.rate_limit_storm || {};
            var banner = document.getElementById("diagStormBanner");
            if (banner) {
                if (storm.active) {
                    banner.hidden = false;
                    banner.innerHTML = "<strong>" + escapeHTML(t("diagStormTitle")) + "</strong>" +
                        "<span>" + escapeHTML(t("diagStormUntil")) + ": " + escapeHTML(storm.until ? new Date(storm.until).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-") +
                        " · " + escapeHTML(t("diagStormAccounts")) + ": " + (storm.credentials || 0) +
                        " · " + escapeHTML(t("diagStormRecovered")) + ": " + (storm.recovered || 0) + "</span>";
                } else {
                    banner.hidden = true;
                    banner.innerHTML = "";
                }
            }
            renderPending429();
            renderCooldowns();
        }

        function renderPending429() {
            var root = document.getElementById("diagPending429");
            if (!root) return;
            var pending = latestDiagnostics.pending_429 || [];
            if (!Array.isArray(pending) || pending.length === 0) {
                root.innerHTML = "<div class=\"diag-subhead\">" + escapeHTML(t("diagPending429Title")) + "</div>" +
                    "<div class=\"diag-empty-line\">" + escapeHTML(t("diagPending429Empty")) + "</div>";
                return;
            }
            var html = "<div class=\"diag-subhead\">" + escapeHTML(t("diagPending429Title")) +
                " <span class=\"badge badge-warning\">" + pending.length + "</span></div><div class=\"diag-mini-list\">";
            pending.forEach(function(item) {
                var identity = formatMaskedEmail(item.auth_index || "-");
                html += "<div class=\"diag-mini-row\">" +
                    "<div class=\"diag-mini-main\"><strong class=\"diag-mono\">" + escapeHTML(identity) + "</strong>" +
                    "<span class=\"diag-mini-meta\">" + escapeHTML((item.model_group || "-") + (item.model ? " · " + item.model : "")) + "</span></div>" +
                    "<span class=\"badge badge-warning\">×" + (item.count || 0) + "</span>" +
                "</div>";
            });
            html += "</div>";
            root.innerHTML = html;
        }

        function renderCooldowns() {
            var root = document.getElementById("diagCooldownContent");
            if (!root) return;
            var cooldowns = latestDiagnostics.active_cooldowns || [];
            if (cooldowns.length === 0) {
                root.innerHTML = "<div class=\"diag-cooldown-empty\"><span>✅</span><span>" + escapeHTML(t("diagCooldownEmptyText")) + "</span></div>";
                return;
            }
            var html = "<div class=\"diag-subhead\">" + escapeHTML(t("diagSectionCooldown")) +
                " <span class=\"badge badge-danger\">" + cooldowns.length + "</span></div><div class=\"diag-mini-list\">";
            cooldowns.forEach(function(item) {
                var until = item.cooldown_until || "";
                var kind = item.failure_kind || (item.status_code ? String(item.status_code) : "cooldown");
                var probe = item.probe_enabled ? " · probe" : "";
                var identity = formatMaskedEmail(item.email || item.auth_index || "Credential");
                html += "<div class=\"diag-mini-row\">" +
                    "<div class=\"diag-mini-main\"><strong>" + escapeHTML(identity) + "</strong>" +
                    "<span class=\"diag-mini-meta\">" + escapeHTML((item.model_group || "-") + (item.model ? " · " + item.model : "") + " · " + kind + probe) + "</span></div>" +
                    "<div class=\"diag-mini-countdown\"><span>" + escapeHTML(t("diagRemainingSec")) + "</span>" +
                    "<strong class=\"meter-countdown\" data-cooldown-countdown=\"" + escapeHTML(until) + "\">" + (until ? formatCountdown(until) : "-") + "</strong></div>" +
                "</div>";
            });
            html += "</div>";
            root.innerHTML = html;
        }

        function renderRunTimeline() {
            var root = document.getElementById("diagRunTimeline");
            if (!root) return;
            var runs = latestDiagnostics.run_history || [];
            if (!Array.isArray(runs) || runs.length === 0) {
                root.innerHTML = "<div class=\"diag-empty-line\">" + escapeHTML(t("diagTimelineEmpty")) + "</div>";
                return;
            }
            var html = "";
            runs.forEach(function(run, index) {
                var at = run.at ? new Date(run.at) : null;
                var time = at ? at.toLocaleTimeString(currentLang === "zh-CN" ? "zh-CN" : "en-US", { hour12: false }) : "-";
                var record = run.record && run.record.status ? run.record.status : "";
                var recordClass = record === "persisted" ? "badge-success" : (record === "failed" ? "badge-danger" : "badge-subtle");
                var recordLabel = record === "persisted" ? t("diagRunRecordPersisted") : (record === "failed" ? t("diagRunRecordFailed") : t("diagRunRecordNotAttempted"));
                html += "<button type=\"button\" class=\"diag-timeline-item\" onclick=\"showHistoryDetails(" + index + ")\">" +
                    "<span class=\"diag-timeline-time\">" + escapeHTML(time) + "</span>" +
                    "<span class=\"diag-timeline-kind\">" + escapeHTML(formatHistoryKind(run)) + "</span>" +
                    "<span class=\"diag-timeline-msg\" title=\"" + escapeHTML(run.message || "") + "\">" + escapeHTML(run.message || "-") + "</span>" +
                    "<span class=\"diag-timeline-outcome\">" + (run.succeeded || 0) + " / " + (run.attempted || 0) + "</span>" +
                    "<span class=\"badge " + recordClass + "\">" + escapeHTML(recordLabel) + "</span>" +
                "</button>";
            });
            root.innerHTML = html;
        }

        function formatBytes(bytes) {
            var value = Number(bytes);
            if (!Number.isFinite(value) || value < 0) value = 0;
            if (value < 1024) return Math.round(value) + " B";
            var units = ["KB", "MB", "GB"];
            var unit = 0;
            value = value / 1024;
            while (value >= 1024 && unit < units.length - 1) {
                value = value / 1024;
                unit++;
            }
            return (value >= 10 ? Math.round(value) : value.toFixed(1)) + " " + units[unit];
        }

        function renderStorage() {
            var list = document.getElementById("diagStorageList");
            if (!list) return;
            var storage = latestDiagnostics ? latestDiagnostics.storage : null;
            if (!storage || !Array.isArray(storage.files) || storage.files.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + escapeHTML(t("diagStorageEmpty")) + "</div>";
                return;
            }

            var roleLabels = {
                state: t("diagStorageRoleState"),
                runtime: t("diagStorageRoleRuntime"),
                runs: t("diagStorageRoleRuns"),
                requests: t("diagStorageRoleRequests")
            };
            var historyCount = Array.isArray(latestDiagnostics.request_history) ? latestDiagnostics.request_history.length : 0;
            var historyMeta = " · " + historyCount + " / " + (latestDiagnostics.request_history_limit || 0);

            var html = "";
            if (Array.isArray(storage.load_errors) && storage.load_errors.length > 0) {
                html += "<div class=\"diag-storage-warning\">" + escapeHTML(storage.load_errors.join("; ")) + "</div>";
            }
            storage.files.forEach(function(file) {
                var exists = Boolean(file.exists);
                var role = file.role || "";
                var label = roleLabels[role] || role;
                if (role === "requests") label += historyMeta;
                var sizeText = exists ? formatBytes(file.size_bytes) : t("diagStorageMissing");
                var badgeClass = exists ? "badge badge-subtle" : "badge badge-danger";
                html += "<div class=\"diag-storage-item\">" +
                    "<div class=\"diag-storage-info\">" +
                        "<div class=\"diag-storage-role\">" + escapeHTML(label) + "</div>" +
                        "<div class=\"diag-storage-path\" title=\"" + escapeHTML(file.path || "") + "\">" + escapeHTML(file.path || "-") + "</div>" +
                    "</div>" +
                    "<span class=\"" + badgeClass + " diag-storage-size\" style=\"font-size:11px;\">" + escapeHTML(sizeText) + "</span>" +
                "</div>";
            });
            list.innerHTML = html;
        }

`
