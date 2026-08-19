package management

// TemplateDiagnostics contains the HTML structure for the System Diagnostics tab.
const TemplateDiagnostics = `
        <section id="panelDiagnostics" hidden>
            <div class="card" style="display:flex; flex-direction:column; height:100%; gap:12px; box-sizing:border-box;">
                <h2 style="margin:0; font-size:15px; flex-shrink:0;" data-i18n="diagnosticsTitle">系统运行诊断与调度状态</h2>
                <div id="schedulerInfo" class="diag-scheduler-box" style="flex-shrink:0;"></div>
                <div class="diag-header" style="flex-shrink:0;">
                    <span style="font-size:13px; font-weight:600; color:var(--text-muted);">Raw JSON</span>
                    <button type="button" class="btn-secondary" style="min-height:28px; padding:3px 8px; font-size:12px;" onclick="copyDiagnosticsJSON()">📋 <span data-i18n="btnCopy">复制 JSON</span></button>
                </div>
                <pre id="rawDiagnostics" class="code-block diag-scroll scroll-container"></pre>
            </div>
        </section>
`
