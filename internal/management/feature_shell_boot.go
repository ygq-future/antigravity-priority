package management

const templateScriptBoot = `        // Initialize application with deterministic dependency ordering
        async function initializeApp() {
            applyLanguage();
            await fetchDynamicConfig();
            await fetchScheduleConfig();
            await refreshDashboard(true);
            countdownInterval = setInterval(updateAllCountdowns, 1000);
        }
        initializeApp();
    </script>
`
