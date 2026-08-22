package management

const templateScriptPrelude = `
    <script>
        const BASE_PATH = "/v0/management/plugins/antigravity-priority";
        const SNAPSHOT_PATH = BASE_PATH + "/snapshot/latest";
        const DIAGNOSTICS_PATH = BASE_PATH + "/diagnostics";
        const RUN_PATH = BASE_PATH + "/run";
        const SYNC_PATH = BASE_PATH + "/sync";
        const RESET_PATH = BASE_PATH + "/reset";
        const SCHEDULE_CONFIG_PATH = BASE_PATH + "/schedule/config";
        const CONFIG_PATH = BASE_PATH + "/config";
        const SAMPLES_PATH = BASE_PATH + "/samples";

`
