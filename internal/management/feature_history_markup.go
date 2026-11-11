package management

// historyMarkup contains the HTML owned by the Request History feature asset.
const historyMarkup = `
        <section id="panelHistory" hidden>
            <div class="card" style="display:flex; flex-direction:column; height:100%; box-sizing:border-box; overflow:hidden;">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px; flex-shrink:0; gap:8px; flex-wrap:wrap;">
                    <div style="display:flex; align-items:baseline; gap:8px;">
                        <h2 style="margin:0; font-size:15px;" data-i18n="historyTitle">最近请求历史</h2>
                        <span id="requestHistoryCountBadge" class="badge badge-subtle" style="font-size:11px;">0 条记录</span>
                    </div>
                    <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
                        <div class="history-limit-control">
                            <span data-i18n="historyRetainLabel">保留</span>
                            <input type="number" id="historyLimitInput" class="history-limit-input" min="1" max="1000" step="1" value="100" aria-label="Request history retention">
                            <span data-i18n="historyRetainUnit">条</span>
                            <button type="button" class="btn-secondary history-limit-save" onclick="saveRequestHistoryLimit()" data-i18n="btnSaveHistoryLimit">保存</button>
                        </div>
                        <button type="button" class="btn-secondary" onclick="fetchDiagnostics()" data-i18n="btnRefresh">🔄 刷新</button>
                        <button type="button" class="btn-secondary" onclick="clearRequestHistoryAction()" data-i18n="btnClearHistory" style="color:var(--accent-red-text);">🧹 清空记录</button>
                    </div>
                </div>
                <div id="historyList" class="scroll-container" style="flex:1 1 auto; overflow-y:auto; overflow-x:auto;">
                    <div class="empty-state" data-i18n="noHistory">暂无请求历史记录</div>
                </div>
                <div id="reqErrorTooltip" class="req-error-tooltip" role="tooltip" tabindex="0"></div>
            </div>
        </section>
`

var historyPageAsset = managementPageAsset{
	name:            "History",
	markup:          historyMarkup,
	styles:          templateStyleHistory,
	scripts:         templateScriptHistory + templateScriptHistoryDetails,
	translationKeys: historyTranslationKeys,
}
