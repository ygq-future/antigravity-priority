package management

const templateScriptOverviewReset = `        async function triggerReset() {
            var confirmed = await showThemedConfirm({
                title: t("confirmResetTitle"),
                message: t("confirmResetMessage"),
                confirmText: currentLang === "zh-CN" ? "确认重置" : "Reset",
                cancelText: currentLang === "zh-CN" ? "取消" : "Cancel",
                isDanger: true
            });
            if (!confirmed) return;

            const btn = document.getElementById("btnReset");
            if (btn) btn.disabled = true;
            if (typeof setListLoading === "function") {
                setListLoading({
                    title: t("resettingTitle"),
                    desc: t("resettingDesc"),
                    icon: "🔄"
                });
            }
            try {
                const result = await apiFetch(RESET_PATH, { method: "POST" });
                showToast(result.message || t("resetSuccess"), "success");
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (typeof setListLoading === "function") setListLoading(false);
                if (btn) btn.disabled = false;
            }
        }

        // quotaWindowExpiryMode decides how an elapsed reset timestamp is read.
        // Weekly quotas roll on a fixed cadence, so an elapsed weekly reset can
        // only be stale evidence. A 5h window may legitimately stay idle at full
        // quota with an elapsed reset; once any quota is consumed the window is
        // active and an elapsed reset is stale evidence as well.
        function quotaWindowExpiryMode(windowKind, remaining) {
            if (windowKind === "weekly") return "stale";
            if (remaining === null || remaining === undefined) return "ready";
            var value = Number(remaining);
            if (!Number.isFinite(value)) return "ready";
            return value >= 100 ? "ready" : "stale";
        }

        function formatCountdown(targetDateStr, expiryMode) {
            if (!targetDateStr) return "-";
            var target = new Date(targetDateStr).getTime();
            var offset = (typeof serverClockOffsetMs === "number" && Number.isFinite(serverClockOffsetMs)) ? serverClockOffsetMs : 0;
            var now = Date.now() + offset;
            var diff = target - now;
            if (diff <= 0) {
                if (expiryMode === "stale") return currentLang === "zh-CN" ? "待刷新" : "Stale";
                return currentLang === "zh-CN" ? "就绪" : "Ready";
            }

            var days = Math.floor(diff / (1000 * 60 * 60 * 24));
            var hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
            var minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
            var seconds = Math.floor((diff % (1000 * 60)) / 1000);

            if (days > 0) {
                return days + "d " + pad(hours) + "h " + pad(minutes) + "m";
            }
            if (hours > 0) {
                return pad(hours) + "h " + pad(minutes) + "m " + pad(seconds) + "s";
            }
            if (minutes > 0) {
                return pad(minutes) + "m " + pad(seconds) + "s";
            }
            return pad(seconds) + "s";
        }

        function pad(n) { return n < 10 ? "0" + n : n; }

`
