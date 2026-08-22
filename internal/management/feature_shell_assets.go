package management

// managementPageShellMarkup contains the document-level controls shared by
// every feature panel.
const managementPageShellMarkup = `
</head>
<body>
    <div class="container">
        <header class="topbar">
            <div class="brand-zone">
                <h1>
                    <span data-i18n="title">Antigravity Priority</span>
                    <span class="version-badge">v1.2.4</span>
                </h1>
            </div>
            <div class="topbar-actions">
                <div id="toastRoot" class="toast-root" aria-live="polite"></div>
                <button type="button" class="btn-secondary" onclick="openKeyModal()" aria-label="Set Management Key">
                    <span>🔑 <span data-i18n="btnKey">密钥</span></span>
                </button>
                <button type="button" class="btn-secondary" onclick="toggleLanguage()" aria-label="Toggle Language">
                    <span id="langLabel">EN / 中文</span>
                </button>
                <button type="button" class="btn-secondary" onclick="toggleTheme()" aria-label="Toggle Theme" id="btnThemeToggle" title="Toggle Theme">
                    <span id="themeIcon">🌓</span>
                </button>
            </div>
        </header>

        <nav class="tabs" aria-label="Dashboard Navigation">
            <button type="button" class="tab active" data-tab="overview" onclick="switchTab('overview')" data-i18n="tabOverview">概览与仪表盘</button>
            <button type="button" class="tab" data-tab="history" onclick="switchTab('history')" data-i18n="tabHistory">执行历史</button>
            <button type="button" class="tab" data-tab="diagnostics" onclick="switchTab('diagnostics')" data-i18n="tabDiagnostics">系统诊断</button>
            <button type="button" class="tab" data-tab="config" onclick="switchTab('config')" data-i18n="tabConfig">⚙️ 配置中心</button>
            <button type="button" class="tab" data-tab="help" onclick="switchTab('help')" data-i18n="tabHelp">使用帮助</button>
        </nav>`

const managementPageShellStyles = templateStyleTokens +
	templateStyleShell +
	templateStyleControls

const managementPageShellStylesTail = templateStyleOverlays + templateStyleResponsive

const managementPageShellScripts = templateScriptPrelude +
	templateScriptI18N +
	templateScriptShellCore +
	templateScriptControls +
	templateScriptModals +
	templateScriptLiveUI
