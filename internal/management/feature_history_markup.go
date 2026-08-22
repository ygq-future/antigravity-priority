package management

// historyMarkup contains the HTML owned by the History feature asset.
const historyMarkup = `
        <section id="panelHistory" hidden>
            <div class="card" style="display:flex; flex-direction:column; height:100%; box-sizing:border-box; overflow:hidden;">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px; flex-shrink:0;">
                    <h2 style="margin:0; font-size:15px;" data-i18n="historyTitle">最近执行记录 (最近 10 次)</h2>
                    <button type="button" class="btn-secondary" onclick="fetchDiagnostics()" data-i18n="btnRefresh">刷新</button>
                </div>
                <div id="historyList" class="scroll-container">
                    <div class="empty-state" data-i18n="noHistory">暂无执行历史</div>
                </div>
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
