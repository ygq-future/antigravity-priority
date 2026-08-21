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
            try {
                const result = await apiFetch(RESET_PATH, { method: "POST" });
                showToast(result.message || t("resetSuccess"), "success");
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (btn) btn.disabled = false;
            }
        }

        function formatCountdown(targetDateStr) {
            if (!targetDateStr) return "-";
            var target = new Date(targetDateStr).getTime();
            var now = Date.now();
            var diff = target - now;
            if (diff <= 0) return currentLang === "zh-CN" ? "就绪" : "Ready";

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
