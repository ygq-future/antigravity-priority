package management

// templateScriptAccountDisplay owns account-label masking for every feature.
// The preference persists per browser alongside the language setting.
const templateScriptAccountDisplay = `        const ACCOUNT_MASK_STORAGE_KEY = "agp_account_masking";
        let accountMaskingEnabled = true;
        try {
            if (typeof localStorage !== "undefined" && localStorage.getItem(ACCOUNT_MASK_STORAGE_KEY) === "0") {
                accountMaskingEnabled = false;
            }
        } catch (_) {}

        // maskAccountPart hides only the middle characters while keeping both
        // ends visible, so accounts sharing a prefix stay distinguishable.
        function maskAccountPart(value) {
            var chars = Array.from(String(value || ""));
            var len = chars.length;
            if (len === 0) return "";
            if (len <= 2) return chars[0] + "***";
            if (len <= 5) return chars[0] + "***" + chars[len - 1];
            if (len <= 8) return chars.slice(0, 2).join("") + "***" + chars.slice(len - 2).join("");
            if (len <= 11) return chars.slice(0, 3).join("") + "***" + chars.slice(len - 3).join("");
            return chars.slice(0, 4).join("") + "***" + chars.slice(len - 4).join("");
        }

        // maskAccountName masks the email local part only: the domain stays
        // readable because it identifies the provider, not the account owner.
        function maskAccountName(value) {
            var s = String(value == null ? "" : value).trim();
            if (!s || s === "Credential" || s === "-") return s;
            var at = s.lastIndexOf("@");
            if (at > 0 && at < s.length - 1) {
                return maskAccountPart(s.substring(0, at)) + s.substring(at);
            }
            return maskAccountPart(s);
        }

        function formatAccountName(value) {
            var s = String(value == null ? "" : value).trim();
            if (!s || s === "Credential") return "Credential";
            if (s === "-") return "-";
            return accountMaskingEnabled ? maskAccountName(s) : s;
        }

        function updateAccountMaskToggle() {
            var btn = document.getElementById("btnAccountMask");
            if (!btn) return;
            btn.setAttribute("aria-pressed", accountMaskingEnabled ? "true" : "false");
            btn.title = t("accountMaskToggleTitle");
            btn.innerHTML = (accountMaskingEnabled ? "🙈 " : "👁 ") + t(accountMaskingEnabled ? "accountMaskOn" : "accountMaskOff");
        }

        function toggleAccountMasking() {
            accountMaskingEnabled = !accountMaskingEnabled;
            try {
                if (typeof localStorage !== "undefined") {
                    localStorage.setItem(ACCOUNT_MASK_STORAGE_KEY, accountMaskingEnabled ? "1" : "0");
                }
            } catch (_) {}
            updateAccountMaskToggle();
            renderDashboard();
            renderHistory();
            renderDiagnostics();
        }
`
