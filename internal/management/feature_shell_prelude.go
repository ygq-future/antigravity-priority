package management

const templateScriptPrelude = `
    <script>
        const BASE_PATH = "/v0/management/plugins/antigravity-priority";
        const SNAPSHOT_PATH = BASE_PATH + "/snapshot/latest";
        const DIAGNOSTICS_PATH = BASE_PATH + "/diagnostics";
        const SCHEDULE_STATUS_PATH = BASE_PATH + "/schedule/status";
        const RUN_HISTORY_DETAIL_PATH = BASE_PATH + "/run-history/detail";
        const RUN_PATH = BASE_PATH + "/run";
        const SYNC_PATH = BASE_PATH + "/sync";
        const RESET_PATH = BASE_PATH + "/reset";
        const SCHEDULE_CONFIG_PATH = BASE_PATH + "/schedule/config";
        const CONFIG_PATH = BASE_PATH + "/runtime-config";
        const SAMPLES_PATH = BASE_PATH + "/samples";
        const REQUEST_HISTORY_CLEAR_PATH = BASE_PATH + "/request-history/clear";
        const REQUEST_HISTORY_PATH = BASE_PATH + "/request-history";
        const CUSTOM_TIER_PATH = BASE_PATH + "/custom-tier";
        const WARMUP_CREDENTIAL_PATH = BASE_PATH + "/warmup-credential";
        const EXCLUDED_MODELS_PATH = BASE_PATH + "/excluded-models";
        const COOLDOWNS_CLEAR_PATH = BASE_PATH + "/cooldowns/clear";
        const PROMPT_GUARD_PREVIEW_PATH = BASE_PATH + "/prompt-guard/preview";
        const PROMPT_GUARD_SYNC_PATH = BASE_PATH + "/prompt-guard/sync";
`
