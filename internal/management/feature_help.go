package management

// helpMarkup contains the HTML owned by the Help feature asset.
const helpMarkup = `
        <section id="panelHelp" hidden>
            <div class="card scroll-container help-panel">
                <div class="help-header">
                    <h2 class="help-title" data-i18n="helpTitle">Antigravity Priority 原理与核心特色说明</h2>
                    <a href="//github.com/ygq-future/antigravity-priority" target="_blank" rel="noopener noreferrer" class="btn-secondary help-github-btn" title="GitHub Repository">
                        <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
                        <span data-i18n="helpGithub">GitHub 主页</span>
                    </a>
                </div>
                <p class="help-intro" data-i18n="helpP1">本插件专为 CLIProxyAPI 的 Google Antigravity 设计，围绕 Gemini、Argon 与 Claude &amp; GPT 三大独立配额池执行自适应优先级调度与纯内存动态分流：</p>
                <div class="help-feature-list">
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="help5hTitle">⚡ 5 小时短窗 (5h) 自愈软降级</div>
                        <div data-i18n="help5hDesc">实时跟踪 Gemini 与 Claude &amp; GPT 的高频配额与重置倒计时。短窗耗尽时仅在内存中将优先级降至 -1（保持 disabled=false），窗口刷新后自动恢复调度，绝不改写宿主凭证的禁用状态。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="help7dTitle">📅 7 天周长窗 (7d) 全局把控</div>
                        <div data-i18n="help7dDesc">同时跟踪 Gemini、Argon 与 Claude &amp; GPT 的周配额剩余量与重置进度。周额度耗尽同样执行纯内存软降级（priority=-1, disabled=false），避免单张凭证在周期前半段过早透支。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpBurnTitle">📈 自适应燃尽学习 (C_cycle / K_5h)</div>
                        <div data-i18n="helpBurnDesc">结合后台探测增量与实时请求用量，自动学习每个账号的周期燃尽率 C_cycle 与 5h 消耗灵敏度 K_5h，并通过 EMA 平滑收敛，无需手动配置数学系数。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpUrgencyTitle">⚖️ 配额燃尽紧迫度 (Weekly Urgency) 与平滑轮换</div>
                        <div data-i18n="helpUrgencyDesc">量化单位时间内的周额度消耗压力，配合容差分档将紧迫度相近的账号聚为同一优先级，并结合会话粘性与轮询指针实现均衡分流。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpBoostTitle">🚀 动态提前提权 (Dynamic Boost Horizon)</div>
                        <div data-i18n="helpBoostDesc">当剩余周额度无法在重置前自然耗尽时（周重置临近），自动进入 Boost 第一梯队，从默认 999 起按紧迫度梯次递减；Free 账号不参与提权，确保 Pro 优先。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpWarmupTitle">♨️ 5 小时短窗满额自动预热</div>
                        <div data-i18n="helpWarmupDesc">Gemini 或 Claude &amp; GPT 短窗 100% 满额且重置倒计时未激活时，自动发送一条轻量自然请求提前开启 5 小时周期；重置到点后依据持久化配额证据补发，内置防抖、随机抖动与独立开关。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpTierTitle">🎯 梯队分层与自定义覆盖</div>
                        <div data-i18n="helpTierDesc">调用次序为 自定义梯队 &gt; Boost &gt; 常规 Pro &gt; Free 兜底；同梯队凭证共享相同优先级并轮询分流，支持在概览卡片上为单个账号指定自定义梯队。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="help429Title">🛡️ 健康故障转移与池级自愈</div>
                        <div data-i18n="help429Desc">401/403 视为账号级异常直接隔离并软降级；429 与 5xx 精准隔离至受影响配额池，并进入带抖动的指数退避探测队列，探测成功后自动恢复原梯队，同账号其他配额池不受影响。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpUITitle">🖥️ 可视化调度与观测</div>
                        <div data-i18n="helpUIDesc">概览看板展示模型路由与 5 条配额进度条，请求历史记录模型、Token 消耗与状态，系统诊断透出调度、熔断与脱敏审计；配置中心修改后即时热生效。</div>
                    </div>
                </div>
            </div>
        </section>
`

const helpStyles = `        .help-panel {
            line-height: 1.6;
            font-size: 13px;
            color: var(--text-secondary);
            padding: 16px 18px !important;
            box-sizing: border-box;
        }

        .help-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 12px;
            gap: 12px;
            flex-wrap: wrap;
        }

        .help-title {
            margin: 0;
            font-size: 15px;
            color: var(--text-primary);
        }

        .help-github-btn {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            text-decoration: none;
            font-size: 12px;
            font-weight: 650;
            padding: 4px 12px;
            min-height: 28px;
            height: 28px;
            border-radius: 6px;
            color: var(--text-primary);
            border: 1px solid var(--border-color);
            background: var(--bg-card);
            transition: all 0.15s ease;
        }

        .help-github-btn:hover {
            background: var(--bg-hover, var(--bg-subtle));
            border-color: var(--border-focus, var(--accent-blue));
            color: var(--accent-blue-text);
        }
        .help-intro {
            margin: 0 0 12px;
        }

        .help-feature-list {
            display: grid;
            gap: 10px;
        }

        .help-feature-item {
            background: var(--bg-subtle);
            padding: 10px 14px;
            border-radius: 8px;
            border: 1px solid var(--border-subtle);
        }

        .help-feature-title {
            font-weight: 700;
            color: var(--text-primary);
            margin-bottom: 2px;
        }
`

var helpPageAsset = managementPageAsset{
	name:            "Help",
	markup:          helpMarkup,
	styles:          helpStyles,
	translationKeys: helpTranslationKeys,
}
