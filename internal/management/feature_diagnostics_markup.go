package management

// diagnosticsMarkup contains the HTML owned by the Diagnostics feature asset.
const diagnosticsMarkup = `
        <section id="panelDiagnostics" hidden>
            <div class="diag-scroll-container scroll-container">
                <div class="diag-top-bar">
                    <div class="diag-title-group">
                        <h2 class="diag-title" data-i18n="diagnosticsTitle">运行诊断与调度决策</h2>
                        <span id="diagHealthBadge" class="badge badge-subtle">-</span>
                        <span id="diagHealthText" class="diag-health-text">-</span>
                    </div>
                    <button type="button" class="btn-secondary diag-copy-btn" onclick="copyDiagnosticsJSON()">
                        📋 <span data-i18n="btnCopyDiagnostics">复制诊断 JSON</span>
                    </button>
                </div>

                <div class="diag-kpi-grid">
                    <div class="diag-kpi-card">
                        <div class="diag-kpi-head">
                            <span class="diag-kpi-title" data-i18n="diagKpiScheduler">⏰ 调度循环</span>
                            <span id="diagSchedBadge" class="badge badge-subtle">-</span>
                        </div>
                        <div id="diagSchedInterval" class="diag-kpi-value">-</div>
                        <div id="diagSchedCountdown" class="diag-kpi-desc">-</div>
                    </div>
                    <div class="diag-kpi-card">
                        <div class="diag-kpi-head">
                            <span class="diag-kpi-title" data-i18n="diagKpiCooldown">🛡️ 隔离与冷却</span>
                            <span id="diagCooldownBadge" class="badge badge-success">-</span>
                        </div>
                        <div id="diagCooldownCount" class="diag-kpi-value">0</div>
                        <div id="diagCooldownSub" class="diag-kpi-desc">-</div>
                    </div>
                    <div class="diag-kpi-card">
                        <div class="diag-kpi-head">
                            <span class="diag-kpi-title" data-i18n="diagKpiLastApply">📊 最近运行</span>
                            <span id="diagApplyBadge" class="badge badge-subtle">-</span>
                        </div>
                        <div id="diagApplyStats" class="diag-kpi-value">-</div>
                        <div id="diagApplyTime" class="diag-kpi-desc">-</div>
                    </div>
                </div>

                <div class="diag-lower-grid diag-pool-grid">
                    <div class="diag-card">
                        <h3 class="diag-card-title">
                            <span>🧩</span>
                            <span data-i18n="diagSectionPools">配额池矩阵</span>
                        </h3>
                        <div class="diag-table-wrap">
                            <table class="diag-table">
                                <thead>
                                    <tr>
                                        <th data-i18n="diagPoolLabel">池</th>
                                        <th data-i18n="diagColActive">活跃</th>
                                        <th data-i18n="diagColBoost">Boost</th>
                                        <th data-i18n="diagColDepleted">耗尽</th>
                                        <th data-i18n="diagColCooldown">冷却</th>
                                        <th data-i18n="diagColMin5h">最低 5h</th>
                                        <th data-i18n="diagColMin7d">最低 7d</th>
                                        <th data-i18n="diagColNearestReset">最近重置</th>
                                        <th data-i18n="diagColAvgBurn">平均 C_cycle</th>
                                    </tr>
                                </thead>
                                <tbody id="diagPoolMatrix"></tbody>
                            </table>
                        </div>
                    </div>
                    <div class="diag-card">
                        <h3 class="diag-card-title">
                            <span>💾</span>
                            <span data-i18n="diagSectionStorage">缓存文件</span>
                        </h3>
                        <div id="diagStorageList" class="diag-storage-list">
                            <div class="empty-state" data-i18n="diagStorageEmpty">暂无缓存文件信息</div>
                        </div>
                    </div>
                </div>

                <div class="diag-card">
                    <h3 class="diag-card-title">
                        <span>🧠</span>
                        <span data-i18n="diagSectionPlan">调度决策解释</span>
                    </h3>
                    <div class="diag-table-wrap diag-table-scroll">
                        <table class="diag-table diag-plan-table">
                            <thead>
                                <tr>
                                    <th data-i18n="diagColCredential">凭证</th>
                                    <th data-i18n="diagColPriority">宿主 → 规划</th>
                                    <th data-i18n="diagColDecision">决策</th>
                                    <th data-i18n="diagColUrgency">Urgency</th>
                                    <th data-i18n="diagColHorizon">T7d / T_req</th>
                                    <th data-i18n="diagColCycle">C_cycle</th>
                                    <th data-i18n="diagColReason">原因</th>
                                </tr>
                            </thead>
                            <tbody id="diagPlanTable"></tbody>
                        </table>
                    </div>
                </div>

                <div class="diag-lower-grid">
                    <div class="diag-card">
                        <h3 class="diag-card-title">
                            <span>🛡️</span>
                            <span data-i18n="diagSectionCooldown">健康与隔离</span>
                        </h3>
                        <div id="diagStormBanner" class="diag-storm-banner" hidden></div>
                        <div id="diagPending429" class="diag-health-block"></div>
                        <div id="diagCooldownContent" class="diag-health-block"></div>
                    </div>
                    <div class="diag-card">
                        <h3 class="diag-card-title">
                            <span>🕒</span>
                            <span data-i18n="diagSectionTimeline">运行时间线</span>
                        </h3>
                        <div id="diagRunTimeline" class="diag-timeline"></div>
                    </div>
                </div>
            </div>
        </section>
`

var diagnosticsPageAsset = managementPageAsset{
	name:            "Diagnostics",
	markup:          diagnosticsMarkup,
	styles:          templateStyleDiagnostics,
	scripts:         templateScriptDiagnostics,
	translationKeys: diagnosticsTranslationKeys,
}
