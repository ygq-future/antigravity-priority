package management

// overviewMarkup contains the HTML owned by the Overview feature asset.
const overviewMarkup = `
        <section id="panelOverview">
            <div class="summary-grid">
                <div class="kpi-card">
                    <p class="kpi-title" data-i18n="kpiTotal">总凭证数</p>
                    <div id="valTotalCreds" class="kpi-value">0</div>
                    <div id="valTotalDesc" class="kpi-desc">0 活跃可用</div>
                </div>
                <div class="kpi-card">
                    <p class="kpi-title" data-i18n="kpiBoosted">🚀 动态 Boost</p>
                    <div id="valBoosted" class="kpi-value">0</div>
                    <div class="kpi-desc" data-i18n="kpiBoostedDesc">高配额充裕候选</div>
                </div>
                <div class="kpi-card">
                    <p class="kpi-title" data-i18n="kpiDepleted">耗尽 / 降级</p>
                    <div id="valDepleted" class="kpi-value">0</div>
                    <div class="kpi-desc" data-i18n="kpiDepletedDesc">等待窗口重置</div>
                </div>
                <div class="kpi-card">
                    <p class="kpi-title" data-i18n="kpiLastAudit">最新调度审计</p>
                    <div id="valLastAudit" class="kpi-value" style="font-size:14px; word-break:break-word;">-</div>
                    <div id="valNextProbe" class="kpi-desc">-</div>
                </div>
            </div>

            <div class="control-bar">
                <input type="hidden" id="modelSelect" value="gemini-3.8-flash-high">
                <div id="customModelSelect" class="custom-select-wrapper" style="min-width: 290px;">
                    <button type="button" class="custom-select-trigger" onclick="toggleCustomModelSelect(event)" aria-haspopup="listbox" aria-expanded="false" id="customModelSelectTrigger">
                        <span id="customModelSelectBadge" class="model-pool-badge badge-gemini">Gemini</span>
                        <span id="customModelSelectLabel" class="model-select-name">gemini-3.8-flash-high</span>
                        <span class="custom-select-arrow">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"></path></svg>
                        </span>
                    </button>
                    <div id="customModelSelectMenu" class="custom-select-options model-select-menu" hidden role="listbox">
                        <div class="model-select-group-header" data-i18n="poolGemini">Gemini 额度池</div>
                        <div class="custom-select-option selected" data-value="gemini-3.8-flash-high" data-pool="gemini" onclick="selectModel('gemini-3.8-flash-high', event)" role="option" aria-selected="true">
                            <span class="model-pool-badge badge-gemini">Gemini</span>
                            <span class="model-option-name">gemini-3.8-flash-high</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                        <div class="custom-select-option" data-value="gemini-3-flash" data-pool="gemini" onclick="selectModel('gemini-3-flash', event)" role="option" aria-selected="false">
                            <span class="model-pool-badge badge-gemini">Gemini</span>
                            <span class="model-option-name">gemini-3-flash</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                        <div class="custom-select-option" data-value="gemini-3.5-flash-lite" data-pool="gemini" onclick="selectModel('gemini-3.5-flash-lite', event)" role="option" aria-selected="false">
                            <span class="model-pool-badge badge-gemini">Gemini</span>
                            <span class="model-option-name">gemini-3.5-flash-lite</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                        <div class="custom-select-option" data-value="gemini-3.1-flash-lite" data-pool="gemini" onclick="selectModel('gemini-3.1-flash-lite', event)" role="option" aria-selected="false">
                            <span class="model-pool-badge badge-gemini">Gemini</span>
                            <span class="model-option-name">gemini-3.1-flash-lite</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                        <div class="custom-select-option" data-value="gemini-3.1-flash-image" data-pool="gemini" onclick="selectModel('gemini-3.1-flash-image', event)" role="option" aria-selected="false">
                            <span class="model-pool-badge badge-gemini">Gemini</span>
                            <span class="model-option-name">gemini-3.1-flash-image</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                        <div class="model-select-group-header" data-i18n="poolArgon">Argon 额度池</div>
                        <div class="custom-select-option" data-value="gemini-4-argon-high" data-pool="argon" onclick="selectModel('gemini-4-argon-high', event)" role="option" aria-selected="false">
                            <span class="model-pool-badge badge-argon">Argon</span>
                            <span class="model-option-name">gemini-4-argon-high</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                        <div class="model-select-group-header" data-i18n="poolClaudeGPT">Claude & GPT 额度池</div>
                        <div class="custom-select-option" data-value="claude-sonnet-5-5-high" data-pool="claude_gpt" onclick="selectModel('claude-sonnet-5-5-high', event)" role="option" aria-selected="false">
                            <span class="model-pool-badge badge-claude">Claude/GPT</span>
                            <span class="model-option-name">claude-sonnet-5-5-high</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                        <div class="custom-select-option" data-value="claude-opus-5-5-high" data-pool="claude_gpt" onclick="selectModel('claude-opus-5-5-high', event)" role="option" aria-selected="false">
                            <span class="model-pool-badge badge-claude">Claude/GPT</span>
                            <span class="model-option-name">claude-opus-5-5-high</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                        <div class="custom-select-option" data-value="gpt-oss-120b-medium" data-pool="claude_gpt" onclick="selectModel('gpt-oss-120b-medium', event)" role="option" aria-selected="false">
                            <span class="model-pool-badge badge-claude">Claude/GPT</span>
                            <span class="model-option-name">gpt-oss-120b-medium</span>
                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                        </div>
                    </div>
                </div>
                <button type="button" class="btn-secondary" onclick="refreshDashboard({ withSync: true, notifySync: true })" id="btnRefresh">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
                    <span data-i18n="btnRefresh">刷新同步</span>
                </button>
                <button type="button" class="btn-secondary" onclick="triggerProbe()" id="btnProbe">
                    <span><span class="btn-probe-icon">📡</span> <span data-i18n="btnProbe">刷新配额</span></span>
                </button>
                <button type="button" class="btn-secondary" onclick="triggerReset()" id="btnReset" title="重置静态优先级">
                    <span>🔄 <span data-i18n="btnReset">重置静态优先级</span></span>
                </button>
                <span id="scheduleStatusBadge" class="schedule-status active" onclick="toggleSchedulePause()" title="点击切换暂停状态">
                    🟢 <span id="scheduleStatusText" data-i18n="scheduleActive">动态调度运行中</span>
                </span>
            </div>

            <div class="credentials-wrapper">
                <div id="credentialsLoadingOverlay" class="credentials-loading-overlay" hidden aria-live="polite">
                    <div class="credentials-loading-card">
                        <div class="quota-probe-radar">
                            <div class="radar-ping"></div>
                            <div class="radar-beacon">📡</div>
                        </div>
                        <div class="quota-loading-text">
                            <div class="quota-loading-title" data-i18n="probingQuotaTitle">正在刷新全局配额...</div>
                            <div class="quota-loading-desc" data-i18n="probingQuotaDesc">正在向 Google Antigravity 服务探测最新 5h/7d 额度窗口</div>
                        </div>
                        <div class="quota-loading-bar">
                            <div class="quota-loading-bar-fill"></div>
                        </div>
                    </div>
                </div>
                <div id="credentialsContainer" class="credentials-grid scroll-container">
                    <div class="empty-state" data-i18n="loading">加载中...</div>
                </div>
            </div>
        </section>
`

var overviewPageAsset = managementPageAsset{
	name:            "Overview",
	markup:          overviewMarkup + overviewSamplesMarkup + overviewTierMarkup,
	styles:          templateStyleOverviewCards,
	scripts:         templateScriptOverviewActionsCore + templateScriptOverviewReset + templateScriptOverviewRender + templateScriptProbeSchedule + templateScriptOverviewSamples + templateScriptOverviewTier,
	translationKeys: overviewTranslationKeys,
}
