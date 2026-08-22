package management

// helpMarkup contains the HTML owned by the Help feature asset.
const helpMarkup = `
        <section id="panelHelp" hidden>
            <div class="card scroll-container help-panel">
                <h2 class="help-title" data-i18n="helpTitle">Antigravity Priority 原理与核心特色说明</h2>
                <p class="help-intro" data-i18n="helpP1">本插件专为 CLIProxyAPI 的 Google Antigravity 设计，实现双窗口自适应优先级调度与智能写回：</p>
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
        }

        .help-title {
            margin: 0 0 10px;
            font-size: 15px;
            color: var(--text-primary);
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
