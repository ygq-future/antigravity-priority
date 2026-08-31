package management

const templateScriptBoot = `        // Initialize application with deterministic dependency ordering and auth circuit-breaker
        async function initializeApp() {
            applyLanguage();
            if (isAuthBlocked) {
                openKeyModal();
                return;
            }
            await fetchDynamicConfig();
            if (isAuthBlocked) return;
            await fetchScheduleConfig();
            if (isAuthBlocked) return;
            await refreshDashboard({ withSync: true, silent: true });
            if (countdownInterval) clearInterval(countdownInterval);
            countdownInterval = setInterval(updateAllCountdowns, 1000);
            if (!isAuthBlocked) {
                if (dashboardRefreshInterval) clearInterval(dashboardRefreshInterval);
                dashboardRefreshInterval = setInterval(refreshOverviewSilently, 15000);
            }
        }
        initializeApp();
    </script>
`
