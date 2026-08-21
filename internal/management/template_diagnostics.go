package management

// TemplateDiagnostics contains the HTML structure for the System Diagnostics tab.
const TemplateDiagnostics = `
        <section id="panelDiagnostics" hidden>
            <div class="diag-scroll-container scroll-container">
                <!-- Diagnostics Top Header -->
                <div class="diag-top-bar">
                    <h2 style="margin:0; font-size:15px; font-weight:700; color:var(--text-primary);" data-i18n="diagnosticsTitle">系统运行诊断与调度状态</h2>
                    <button type="button" class="btn-secondary" style="min-height:30px; height:30px; padding:0 12px; font-size:12px;" onclick="copyDiagnosticsJSON()">
                        📋 <span data-i18n="btnCopyDiagnostics">复制诊断 JSON</span>
                    </button>
                </div>

                <!-- 3-Column KPI Grid -->
                <div class="diag-kpi-grid">
                    <!-- KPI 1: Scheduler Engine -->
                    <div class="diag-kpi-card">
                        <div class="diag-kpi-head">
                            <span class="diag-kpi-title" data-i18n="diagKpiScheduler">⏰ 调度引擎</span>
                            <span id="diagSchedBadge" class="badge badge-subtle">-</span>
                        </div>
                        <div id="diagSchedInterval" class="diag-kpi-value">-</div>
                        <div id="diagSchedCountdown" class="diag-kpi-desc">-</div>
                    </div>

                    <!-- KPI 2: 429 Circuit Breakers -->
                    <div class="diag-kpi-card">
                        <div class="diag-kpi-head">
                            <span class="diag-kpi-title" data-i18n="diagKpiCooldown">🛡️ 429 熔断监控</span>
                            <span id="diagCooldownBadge" class="badge badge-success">正常</span>
                        </div>
                        <div id="diagCooldownCount" class="diag-kpi-value">0</div>
                        <div id="diagCooldownSub" class="diag-kpi-desc" data-i18n="diagCooldownSubText">自动降权 -1 机制就绪</div>
                    </div>

                    <!-- KPI 3: Latest Apply Health -->
                    <div class="diag-kpi-card">
                        <div class="diag-kpi-head">
                            <span class="diag-kpi-title" data-i18n="diagKpiLastApply">📊 最近写入体征</span>
                            <span id="diagApplyBadge" class="badge badge-subtle">-</span>
                        </div>
                        <div id="diagApplyStats" class="diag-kpi-value">-</div>
                        <div id="diagApplyTime" class="diag-kpi-desc">-</div>
                    </div>
                </div>

                <!-- Section Card 1: Scheduling Details & Active Time Window -->
                <div class="diag-card">
                    <h3 class="diag-card-title">
                        <span>⏰</span>
                        <span data-i18n="diagSectionScheduler">调度运行详情与时间窗口</span>
                    </h3>
                    <div class="diag-detail-grid">
                        <div class="diag-detail-item">
                            <span class="diag-detail-label" data-i18n="diagSchedIntervalLabel">基础执行周期</span>
                            <span id="diagSchedDetailInterval" class="diag-detail-val">-</span>
                        </div>
                        <div class="diag-detail-item">
                            <span class="diag-detail-label" data-i18n="diagSchedWorkerLabel">后台 Worker 协程</span>
                            <span id="diagSchedDetailWorker" class="diag-detail-val">-</span>
                        </div>
                        <div class="diag-detail-item">
                            <span class="diag-detail-label" data-i18n="diagSchedLastRunLabel">上次自动调度</span>
                            <span id="diagSchedDetailLastRun" class="diag-detail-val">-</span>
                        </div>
                        <div class="diag-detail-item">
                            <span class="diag-detail-label" data-i18n="diagSchedNextRunLabel">下次调度预计时间</span>
                            <span id="diagSchedDetailNextRun" class="diag-detail-val">-</span>
                        </div>
                    </div>
                    <div class="diag-window-box">
                        <div style="display:flex; align-items:center; justify-content:space-between; gap:8px; flex-wrap:wrap;">
                            <div style="display:flex; align-items:center; gap:8px;">
                                <span style="font-size:13px; font-weight:600;" data-i18n="diagWindowPolicyLabel">调度时段策略:</span>
                                <span id="diagSchedDetailWindowText" style="font-size:13px; color:var(--text-secondary); font-family:monospace;">-</span>
                            </div>
                            <span id="diagSchedDetailWindowBadge" class="badge badge-subtle">-</span>
                        </div>
                    </div>
                </div>

                <!-- Section Card 2: 429 Rate Limit Cooldowns -->
                <div class="diag-card">
                    <h3 class="diag-card-title">
                        <span>🛡️</span>
                        <span data-i18n="diagSectionCooldown">429 熔断与冷却凭证监控</span>
                    </h3>
                    <div id="diagCooldownContent">
                        <div class="diag-cooldown-empty">
                            <span>✅</span>
                            <span data-i18n="diagCooldownEmptyText">当前无处于 429 冷却中的凭证，所有账号运行正常</span>
                        </div>
                    </div>
                </div>

                <!-- Section Card 3: Latest Execution & Desensitized Audit Stream -->
                <div class="diag-card">
                    <h3 class="diag-card-title">
                        <span>📊</span>
                        <span data-i18n="diagSectionAudit">最近写入执行与脱敏审计</span>
                    </h3>
                    <div class="diag-audit-box">
                        <div style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.5px; margin-bottom:4px;" data-i18n="diagAuditSummaryLabel">Audit Stream</div>
                        <div id="diagAuditText" class="diag-audit-text">-</div>
                    </div>
                    <div id="diagAuditPills" class="diag-pills-row" style="margin-top:10px;">
                        <!-- Succeeded, Failed, Skipped badges -->
                    </div>
                </div>
            </div>
        </section>
`
