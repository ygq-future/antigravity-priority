package management

// TemplateOverview contains the HTML structure for the Overview & Meters tab.
const TemplateOverview = `
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
                <select id="modelGroupSelect" hidden>
                    <option value="gemini" data-i18n="optGemini">Gemini 模型</option>
                    <option value="claude_gpt" data-i18n="optClaudeGPT">Claude 与 GPT 模型</option>
                </select>
                <div id="customModelGroupSelect" class="custom-select-wrapper">
                    <button type="button" class="custom-select-trigger" onclick="toggleCustomSelect(event)" aria-haspopup="listbox" aria-expanded="false">
                        <span id="customSelectLabel" data-i18n="optGemini">Gemini 模型</span>
                        <span class="custom-select-arrow">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"></path></svg>
                        </span>
                    </button>
                    <div id="customSelectMenu" class="custom-select-options" hidden role="listbox">
                        <div class="custom-select-option selected" data-value="gemini" onclick="selectModelGroup('gemini', event)" role="option" aria-selected="true">
                            <span data-i18n="optGemini">Gemini 模型</span>
                            <span class="custom-select-check">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg>
                            </span>
                        </div>
                        <div class="custom-select-option" data-value="claude_gpt" onclick="selectModelGroup('claude_gpt', event)" role="option" aria-selected="false">
                            <span data-i18n="optClaudeGPT">Claude 与 GPT 模型</span>
                            <span class="custom-select-check">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg>
                            </span>
                        </div>
                    </div>
                </div>

                <button type="button" class="btn-secondary" onclick="refreshDashboard(true)" id="btnRefresh">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
                    <span data-i18n="btnRefresh">刷新</span>
                </button>
                <button type="button" class="btn-secondary" onclick="triggerProbe()" id="btnProbe">
                    <span>📡 <span data-i18n="btnProbe">刷新配额</span></span>
                </button>
                <button type="button" class="btn-secondary" onclick="triggerRun('dry-run')" id="btnDryRun">
                    <span>🔍 <span data-i18n="btnDryRun">试运行</span></span>
                </button>
                <button type="button" class="btn-primary" onclick="triggerApplyWithConfirm()" id="btnApply">
                    <span>⚡ <span data-i18n="btnApply">立即写回</span></span>
                </button>
                <button type="button" class="btn-danger" onclick="triggerReset()" id="btnReset">
                    <span>🔄 <span data-i18n="btnReset">重置优先级</span></span>
                </button>
                <span id="scheduleStatusBadge" class="schedule-status active" onclick="toggleSchedulePause()" title="Click to toggle pause">
                    🟢 <span id="scheduleStatusText" data-i18n="scheduleActive">自动调度运行中</span>
                </span>
            </div>

            <div id="credentialsContainer" class="credentials-grid scroll-container">
                <div class="empty-state" data-i18n="loading">加载中...</div>
            </div>
        </section>
`
