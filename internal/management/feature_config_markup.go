package management

// configMarkup contains the HTML owned by the Config feature asset.
const configMarkup = `
        <section id="panelConfig" hidden>
            <div class="config-scroll scroll-container">
                <!-- Card 1: Scheduling & Active Time Window -->
                <div class="config-card">
                    <h3 class="config-card-title">
                        <span>⏰</span>
                        <span data-i18n="cfgCardScheduleTitle">自动调度与时段配置</span>
                    </h3>

                    <div class="config-form-grid">
                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgAutoApply">自适应配额校准</span>
                                <span class="form-hint" data-i18n="cfgAutoApplyHint">根据历史请求量与频率自适应向 Google 发起配额探测，高频必校准、低频延后、空闲跳过</span>
                            </div>
                            <div class="form-input-group">
                                <label class="toggle-label">
                                    <span class="toggle-switch">
                                        <input type="checkbox" id="cfgAutoApply">
                                        <span class="toggle-slider"></span>
                                    </span>
                                    <span id="cfgAutoApplyStatusText">已开启</span>
                                </label>
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgInterval">高频校准周期上限</span>
                                <span class="form-hint" data-i18n="cfgIntervalHint">频繁使用时的最大校准间隔（含随机抖动，如 15m, 30m, 1h）；低频自动拉长，空闲不触发</span>
                            </div>
                            <div class="form-input-group">
                                <select id="cfgIntervalSelect" hidden>
                                    <option value="5m" data-i18n="optInterval5m">5 分钟</option>
                                    <option value="15m" selected data-i18n="optInterval15m">15 分钟</option>
                                    <option value="30m" data-i18n="optInterval30m">30 分钟</option>
                                    <option value="1h" data-i18n="optInterval1h">1 小时</option>
                                    <option value="custom" data-i18n="optIntervalCustom">自定义</option>
                                </select>
                                <div id="customIntervalSelect" class="custom-select-wrapper" style="min-width:140px;">
                                    <button type="button" class="custom-select-trigger" onclick="toggleCustomIntervalSelect(event)" aria-haspopup="listbox" aria-expanded="false">
                                        <span id="customIntervalLabel">15 分钟</span>
                                        <span class="custom-select-arrow">
                                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"></path></svg>
                                        </span>
                                    </button>
                                    <div id="customIntervalMenu" class="custom-select-options" hidden role="listbox">
                                        <div class="custom-select-option" data-value="5m" onclick="selectIntervalOption('5m', event)">
                                            <span data-i18n="optInterval5m">5 分钟</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option selected" data-value="15m" onclick="selectIntervalOption('15m', event)">
                                            <span data-i18n="optInterval15m">15 分钟</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="30m" onclick="selectIntervalOption('30m', event)">
                                            <span data-i18n="optInterval30m">30 分钟</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="1h" onclick="selectIntervalOption('1h', event)">
                                            <span data-i18n="optInterval1h">1 小时</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="custom" onclick="selectIntervalOption('custom', event)">
                                            <span data-i18n="optIntervalCustom">自定义</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                    </div>
                                </div>
                                <input type="text" id="cfgIntervalCustom" class="config-time-input" placeholder="15m" hidden>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Card 2: Probing & Performance -->
                <div class="config-card">
                    <h3 class="config-card-title">
                        <span>⚡</span>
                        <span data-i18n="cfgCardPerfTitle">探测与调度性能</span>
                    </h3>

                    <div class="config-form-grid">
                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgMaxConcurrency">最大探测并发数</span>
                                <span class="form-hint" data-i18n="cfgMaxConcurrencyHint">向 Google API 并发探测的最大协程数 (1-32)</span>
                            </div>
                            <div class="form-input-group">
                                <input type="number" id="cfgMaxConcurrency" class="config-num-input" min="1" max="32" value="6">
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgUrgencyTolerance">紧迫度分档容差</span>
                                <span class="form-hint" data-i18n="cfgUrgencyToleranceHint">紧迫度差距在此容差内分配相同优先级 (0.00-0.50)</span>
                            </div>
                            <div class="form-input-group">
                                <input type="number" id="cfgUrgencyTolerance" class="config-num-input" min="0.00" max="0.50" step="0.01" value="0.05">
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgSampleCapacity">自适应时序样本容量</span>
                                <span class="form-hint" data-i18n="cfgSampleCapacityHint">滑动窗口保留的历史探测样本数，用于平滑估计燃尽率 (2-30)</span>
                            </div>
                            <div class="form-input-group">
                                <input type="number" id="cfgSampleCapacity" class="config-num-input" min="2" max="30" value="6">
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgCooldownMinutes">429 熔断冷却时长 (分钟)</span>
                                <span class="form-hint" data-i18n="cfgCooldownMinutesHint">遭遇 429 限流时临时降级至 -1 的冷却期 (1-1440)</span>
                            </div>
                            <div class="form-input-group">
                                <input type="number" id="cfgCooldownMinutes" class="config-num-input" min="1" max="1440" value="5">
                            </div>
                        </div>

                    </div>
                </div>

                <!-- Card 3: 5-Hour Window Warm-Up -->
                <div class="config-card">
                    <h3 class="config-card-title">
                        <span>🔥</span>
                        <span data-i18n="cfgCardWarmupTitle">5小时短窗预热</span>
                    </h3>

                    <div class="config-form-grid">
                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgWarmupGemini">Gemini 5h 满额自动预热</span>
                                <span class="form-hint" data-i18n="cfgWarmupGeminiHint" data-i18n-title="cfgWarmupGeminiHintTitle">短窗 100% 满额时自动触发 5h 重置</span>
                            </div>
                            <div class="form-input-group">
                                <label class="toggle-label">
                                    <span class="toggle-switch">
                                        <input type="checkbox" id="cfgWarmupGeminiEnabled">
                                        <span class="toggle-slider"></span>
                                    </span>
                                </label>
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgWarmupGeminiModel">Gemini 预热目标模型</span>
                                <span class="form-hint" data-i18n="cfgWarmupGeminiModelHint">用于触发 Gemini 5h 重置倒计时的模型</span>
                            </div>
                            <div class="form-input-group">
                                <select id="cfgWarmupGeminiModelSelect" hidden></select>
                                <div id="customWarmupGeminiModelSelect" class="custom-select-wrapper" style="min-width:210px;">
                                    <button type="button" class="custom-select-trigger" onclick="toggleCustomWarmupGeminiSelect(event)" aria-haspopup="listbox" aria-expanded="false">
                                        <span id="customWarmupGeminiLabel">gemini-3-flash</span>
                                        <span class="custom-select-arrow">
                                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"></path></svg>
                                        </span>
                                    </button>
                                    <div id="customWarmupGeminiMenu" class="custom-select-options" hidden role="listbox"></div>
                                </div>
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgWarmupClaude">Claude / GPT 5h 满额自动预热</span>
                                <span class="form-hint" data-i18n="cfgWarmupClaudeHint" data-i18n-title="cfgWarmupClaudeHintTitle">短窗 100% 满额时自动触发 5h 重置</span>
                            </div>
                            <div class="form-input-group">
                                <label class="toggle-label">
                                    <span class="toggle-switch">
                                        <input type="checkbox" id="cfgWarmupClaudeEnabled">
                                        <span class="toggle-slider"></span>
                                    </span>
                                </label>
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgWarmupClaudeModel">Claude 预热目标模型</span>
                                <span class="form-hint" data-i18n="cfgWarmupClaudeModelHint">用于触发 Claude 5h 重置倒计时的模型</span>
                            </div>
                            <div class="form-input-group">
                                <select id="cfgWarmupClaudeModelSelect" hidden></select>
                                <div id="customWarmupClaudeModelSelect" class="custom-select-wrapper" style="min-width:210px;">
                                    <button type="button" class="custom-select-trigger" onclick="toggleCustomWarmupClaudeSelect(event)" aria-haspopup="listbox" aria-expanded="false">
                                        <span id="customWarmupClaudeLabel">claude-sonnet-5-5-high</span>
                                        <span class="custom-select-arrow">
                                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"></path></svg>
                                        </span>
                                    </button>
                                    <div id="customWarmupClaudeMenu" class="custom-select-options" hidden role="listbox"></div>
                                </div>
                            </div>
                        </div>

                    </div>
                </div>

                <!-- Card 4: Prompt Guard (Antigravity Request Filtering) -->
                <div class="config-card">
                    <div class="pg-card-header">
                        <h3 class="config-card-title">
                            <span>🛡️</span>
                            <span data-i18n="cfgCardPromptGuardTitle">Prompt Guard 提示词防护</span>
                        </h3>
                        <button type="button" class="btn-secondary pg-copy-bisect-btn" id="btnPgCopyBisect" onclick="copyPromptGuardBisectPrompt()" data-i18n="cfgPgBtnCopyBisect">📋 复制二分排查提示词</button>
                    </div>
                    <div class="form-row full-width">
                        <div class="form-label-box">
                            <span class="form-title" data-i18n="cfgPgEnabled">启用请求体关键字改写</span>
                            <span class="form-hint" data-i18n="cfgPgEnabledHint">请求路由至 Antigravity 后、发往上游前按规则改写系统提示词中的敏感短语，减少误触发无额度 429</span>
                        </div>
                        <div class="form-input-group">
                            <label class="toggle-label">
                                <span class="toggle-switch">
                                    <input type="checkbox" id="cfgPromptGuardEnabled">
                                    <span class="toggle-slider"></span>
                                </span>
                                <span id="cfgPromptGuardStatusText">已关闭</span>
                            </label>
                        </div>
                    </div>

                    <div class="pg-toolbar">
                        <div class="pg-toolbar-left">
                            <input type="text" id="cfgPgSearch" class="pg-search-input" data-i18n-placeholder="cfgPgSearchPlaceholder" placeholder="筛选查找或替换文本..." oninput="renderPromptGuardRules()">
                            <span id="cfgPgRuleCount" class="pg-count-pill">0 / 200</span>
                        </div>
                        <div class="pg-toolbar-right">
                            <button type="button" class="btn-secondary" id="btnPgAddRule" onclick="addPromptGuardRule()" data-i18n="cfgPgBtnAddRule">+ 新增规则</button>
                            <button type="button" class="btn-secondary" id="btnPgSyncRemote" onclick="syncPromptGuardRemoteRules()" data-i18n="cfgPgBtnSyncRemote">🔄 同步远程规则</button>
                            <button type="button" class="btn-secondary" id="btnPgRestoreDefaults" style="display:none;" onclick="restoreDefaultPromptGuardRules()"></button>
                        </div>
                    </div>

                    <div id="cfgPgRulesList" class="pg-rules-list"></div>

                    <div class="pg-dryrun-box">
                        <div class="pg-dryrun-header">
                            <span class="pg-dryrun-title" data-i18n="cfgPgDryRunTitle">试运行预览 (Dry-Run)</span>
                            <div class="pg-dryrun-controls">
                                <select id="cfgPgPreviewFormat" hidden>
                                    <option value="auto" selected data-i18n="cfgPgFormatAuto">自动识别 (JSON / 纯文本)</option>
                                    <option value="text:system" data-i18n="cfgPgFormatTextSystem">纯文本 (仅系统范围)</option>
                                    <option value="text:all" data-i18n="cfgPgFormatTextAll">纯文本 (全部消息)</option>
                                    <option value="claude" data-i18n="cfgPgFormatClaude">Claude / Anthropic JSON</option>
                                    <option value="openai" data-i18n="cfgPgFormatOpenAI">OpenAI Chat JSON</option>
                                    <option value="openai-response" data-i18n="cfgPgFormatResponses">OpenAI Responses JSON</option>
                                    <option value="gemini" data-i18n="cfgPgFormatGemini">Gemini / Antigravity JSON</option>
                                </select>
                                <div id="customPgFormatSelect" class="custom-select-wrapper" style="min-width:205px;">
                                    <button type="button" class="custom-select-trigger" onclick="toggleCustomPgFormatSelect(event)" aria-haspopup="listbox" aria-expanded="false">
                                        <span id="customPgFormatLabel">自动识别 (JSON / 纯文本)</span>
                                        <span class="custom-select-arrow">
                                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"></path></svg>
                                        </span>
                                    </button>
                                    <div id="customPgFormatMenu" class="custom-select-options" hidden role="listbox">
                                        <div class="custom-select-option selected" data-value="auto" onclick="selectPgFormatOption('auto', event)">
                                            <span data-i18n="cfgPgFormatAuto">自动识别 (JSON / 纯文本)</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="text:system" onclick="selectPgFormatOption('text:system', event)">
                                            <span data-i18n="cfgPgFormatTextSystem">纯文本 (仅系统范围)</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="text:all" onclick="selectPgFormatOption('text:all', event)">
                                            <span data-i18n="cfgPgFormatTextAll">纯文本 (全部消息)</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="claude" onclick="selectPgFormatOption('claude', event)">
                                            <span data-i18n="cfgPgFormatClaude">Claude / Anthropic JSON</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="openai" onclick="selectPgFormatOption('openai', event)">
                                            <span data-i18n="cfgPgFormatOpenAI">OpenAI Chat JSON</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="openai-response" onclick="selectPgFormatOption('openai-response', event)">
                                            <span data-i18n="cfgPgFormatResponses">OpenAI Responses JSON</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="gemini" onclick="selectPgFormatOption('gemini', event)">
                                            <span data-i18n="cfgPgFormatGemini">Gemini / Antigravity JSON</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                    </div>
                                </div>
                                <button type="button" class="btn-secondary" id="btnPgRunPreview" onclick="runPromptGuardPreview()" data-i18n="cfgPgBtnRunPreview">执行试运行</button>
                            </div>
                        </div>
                        <textarea id="cfgPgPreviewInput" class="pg-textarea" data-i18n-placeholder="cfgPgDryRunPlaceholder" placeholder="粘贴提示词文本或完整 API 请求 JSON，验证规则命中与改写结果（纯本地试运行，不发送上游请求）..."></textarea>
                        <div id="cfgPgPreviewResult" class="pg-preview-result" hidden></div>
                    </div>
                </div>
            </div>

            <!-- Floating Capsule Buttons at Bottom Right -->
            <div class="config-floating-actions">
                <button type="button" class="btn-secondary" onclick="resetDynamicConfigToDefaults()" id="btnResetConfig">
                    <span>🔄 <span data-i18n="btnResetToDefaults">恢复默认配置</span></span>
                </button>
                <button type="button" class="btn-primary" onclick="saveDynamicConfig()" id="btnSaveConfig" disabled>
                    <span>💾 <span data-i18n="btnSaveConfig">保存并立即生效</span></span>
                </button>
            </div>
        </section>
`

var configPageAsset = managementPageAsset{
	name:            "Config",
	markup:          configMarkup,
	styles:          templateStyleConfig,
	scripts:         templateScriptConfig,
	translationKeys: configTranslationKeys,
}
