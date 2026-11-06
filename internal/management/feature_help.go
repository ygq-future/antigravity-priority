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
                <p class="help-intro" data-i18n="helpP1">本插件专为 CLIProxyAPI 的 Google Antigravity 设计，实现三大配额池自适应优先级调度与纯内存动态分流：</p>
                <div class="help-feature-list">
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="help5hTitle">⚡ 5小时短窗口 (5h) 自愈降级</div>
                        <div data-i18n="help5hDesc">实时监控高频请求配额与重置倒计时。短窗耗尽仅执行软降级（priority=-1, disabled=false），短窗刷新后自动恢复调度，有效防止触发 429 速率限制。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="help7dTitle">📅 7天长窗口 (7d) 全局把控</div>
                        <div data-i18n="help7dDesc">跟踪周配额剩余量及重置进度。周额度耗尽执行硬禁用（disabled=true），避免单张凭证在周期前半段过早耗尽。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpBurnTitle">📈 自适应燃尽学习 (C_cycle)</div>
                        <div data-i18n="helpBurnDesc">基于连续探测增量自动学习推算周期消耗能力并通过 EMA 平滑，杜绝人工手动配置负担，实时计算保底所需剩余额度。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpUrgencyTitle">⚖️ 配额燃尽紧迫度 (Weekly Urgency) 与平滑轮换</div>
                        <div data-i18n="helpUrgencyDesc">量化单周期内的额度使用压力，配合容差分档算法，将紧迫度相近的账号自动赋予相同优先级实现轮询均衡。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="helpBoostTitle">🚀 动态提前提权 (Dynamic Boost Horizon)</div>
                        <div data-i18n="helpBoostDesc">对双窗口余量充裕且周重置临近的凭证自动赋予第一梯队超高优先级（900-999），彻底消除大额度账号撑死溢出与浪费痛点。</div>
                    </div>
                    <div class="help-feature-item">
                        <div class="help-feature-title" data-i18n="help429Title">⏳ 429 熔断冷却与自动自愈</div>
                        <div data-i18n="help429Desc">遭遇 Google 429 速率限制时自动临时降级为 -1，冷却期结束后在下次调度中自动探测自愈，无需人工介入。</div>
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
