package management

const templateScriptBoot = `        // Initialize application with deterministic dependency ordering
        async function initializeApp() {
            applyLanguage();
            await fetchDynamicConfig();
            await fetchScheduleConfig();
            await refreshDashboard({ withSync: true, silent: true });
            countdownInterval = setInterval(updateAllCountdowns, 1000);
            dashboardRefreshInterval = setInterval(refreshOverviewSilently, 15000);
        }
        initializeApp();
    </script>
`
