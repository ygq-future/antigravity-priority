package management

// TemplateConfig contains the HTML structure for the Configuration Center tab.
const TemplateConfig = `
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
                                <span class="form-title" data-i18n="cfgAutoApply">自动定时调度</span>
                                <span class="form-hint" data-i18n="cfgAutoApplyHint">周期性自动探测并智能更新宿主凭证优先级</span>
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
                                <span class="form-title" data-i18n="cfgInterval">调度执行周期</span>
                                <span class="form-hint" data-i18n="cfgIntervalHint">自动调度与探测的执行间隔（如 15m, 30m, 1h）</span>
                            </div>
                            <div class="form-input-group">
                                <select id="cfgIntervalSelect" hidden>
                                    <option value="5m" data-i18n="optInterval5m">5 分钟</option>
                                    <option value="15m" selected data-i18n="optInterval15m">15 分钟 (推荐)</option>
                                    <option value="30m" data-i18n="optInterval30m">30 分钟</option>
                                    <option value="1h" data-i18n="optInterval1h">1 小时</option>
                                    <option value="custom" data-i18n="optIntervalCustom">自定义</option>
                                </select>
                                <div id="customIntervalSelect" class="custom-select-wrapper" style="min-width:140px;">
                                    <button type="button" class="custom-select-trigger" onclick="toggleCustomIntervalSelect(event)" aria-haspopup="listbox" aria-expanded="false">
                                        <span id="customIntervalLabel">15 分钟 (推荐)</span>
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
                                            <span data-i18n="optInterval15m">15 分钟 (推荐)</span>
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

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgModelGroup">配额主控模型组</span>
                                <span class="form-hint" data-i18n="cfgModelGroupHint">作为主要写回依据的模型组</span>
                            </div>
                            <div class="form-input-group">
                                <select id="cfgModelGroup" hidden>
                                    <option value="gemini" data-i18n="optGemini">Gemini 模型</option>
                                    <option value="claude_gpt" data-i18n="optClaudeGPT">Claude 与 GPT 模型</option>
                                </select>
                                <div id="customCfgModelGroupSelect" class="custom-select-wrapper" style="min-width:160px;">
                                    <button type="button" class="custom-select-trigger" onclick="toggleCustomCfgModelSelect(event)" aria-haspopup="listbox" aria-expanded="false">
                                        <span id="customCfgModelLabel">Gemini 模型</span>
                                        <span class="custom-select-arrow">
                                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"></path></svg>
                                        </span>
                                    </button>
                                    <div id="customCfgModelMenu" class="custom-select-options" hidden role="listbox">
                                        <div class="custom-select-option selected" data-value="gemini" onclick="selectCfgModelOption('gemini', event)">
                                            <span data-i18n="optGemini">Gemini 模型</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                        <div class="custom-select-option" data-value="claude_gpt" onclick="selectCfgModelOption('claude_gpt', event)">
                                            <span data-i18n="optClaudeGPT">Claude 与 GPT 模型</span>
                                            <span class="custom-select-check"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgScheduleWindow">生效时间区间</span>
                                <span class="form-hint" data-i18n="cfgScheduleWindowHint">仅在每日指定时段运行 (支持跨午夜)</span>
                            </div>
                            <div class="form-input-group" style="flex-direction:column; align-items:flex-end; gap:4px;">
                                <label class="toggle-label" style="font-size:12px;">
                                    <input type="checkbox" id="cfgWindowEnabled" onchange="onWindowEnabledChange()">
                                    <span data-i18n="cfgWindowEnabledLabel">仅在指定时段运行</span>
                                </label>
                                <div id="cfgWindowInputs" style="display:flex; align-items:center; gap:6px;">
                                    <input type="text" id="cfgWindowStart" class="config-time-input" placeholder="09:00">
                                    <span style="color:var(--text-muted); font-weight:700;">-</span>
                                    <input type="text" id="cfgWindowEnd" class="config-time-input" placeholder="23:00">
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Card 2: Probing & Performance -->
                <div class="config-card">
                    <h3 class="config-card-title">
                        <span>⚡</span>
                        <span data-i18n="cfgCardPerfTitle">探测与写回性能</span>
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
                                <span class="form-title" data-i18n="cfgMinChange">优先级变动写入阈值</span>
                                <span class="form-hint" data-i18n="cfgMinChangeHint">优先级变化绝对值达到该阈值才写入宿主 (0-100)</span>
                            </div>
                            <div class="form-input-group">
                                <input type="number" id="cfgMinChange" class="config-num-input" min="0" max="100" value="1">
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

                <!-- Card 3: Priority Rules -->
                <div class="config-card">
                    <h3 class="config-card-title">
                        <span>🎯</span>
                        <span data-i18n="cfgCardRulesTitle">优先级分值规则 (Priority Rules)</span>
                    </h3>

                    <div class="config-form-grid">
                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgBoostStart">🚀 动态 Boost 起始优先级</span>
                                <span class="form-hint" data-i18n="cfgBoostStartHint">充裕且即将重置的第一梯队凭证起始优先级 (1-999)</span>
                            </div>
                            <div class="form-input-group">
                                <input type="number" id="cfgBoostStartPriority" class="config-num-input" min="1" max="999" value="999">
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-label-box">
                                <span class="form-title" data-i18n="cfgNormalStart">常规健康凭证起始优先级</span>
                                <span class="form-hint" data-i18n="cfgNormalStartHint">常规可用梯队凭证的起始基准优先级 (1-999)</span>
                            </div>
                            <div class="form-input-group">
                                <input type="number" id="cfgNormalStartPriority" class="config-num-input" min="1" max="999" value="100">
                            </div>
                        </div>
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
