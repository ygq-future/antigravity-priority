package management

// shellModalMarkup contains modal foundations shared by multiple features.
const shellModalMarkup = `
    <!-- Management Key Modal -->
    <div id="keyModal" class="modal-backdrop" hidden>
        <div class="modal" role="dialog" aria-modal="true" aria-labelledby="keyModalTitle" style="max-width: 460px;">
            <div style="display:flex; justify-content:space-between; align-items:center;">
                <h2 id="keyModalTitle" style="margin:0; font-size:17px;" data-i18n="keyModalTitle">CPA 管理密钥认证</h2>
                <button type="button" class="btn-secondary" onclick="closeKeyModal()" style="min-height:28px; padding:3px 8px;">✕</button>
            </div>
            <p style="font-size:13px; color:var(--text-secondary); margin:0; line-height:1.5;" data-i18n="keyModalDesc">CPA 原生管理界面下请输入 config.yaml 中的 Management Key；若在 CPA-Plus 等增强面板中，请输入 CPA-Plus 登录密码（通常为 cpamp_... 格式）。</p>
            <div>
                <input type="password" id="manualKeyInput" placeholder="Management Key" autocomplete="off" style="width:100%;">
            </div>
            <div style="display:flex; justify-content:flex-end; gap:8px; margin-top:6px;">
                <button type="button" class="btn-secondary" onclick="closeKeyModal()" data-i18n="btnClose">取消</button>
                <button type="button" class="btn-primary" onclick="saveKeyAndRefresh()" data-i18n="btnSaveKey">保存并验证</button>
            </div>
        </div>
    </div>

    <!-- Diff & Run Result Modal -->
    <div id="diffModal" class="modal-backdrop" hidden onclick="closeModalFromBackdrop(event)">
        <div id="diffModalPanel" class="modal" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
            <div style="display:flex; justify-content:space-between; align-items:center;">
                <h2 id="modalTitle" style="margin:0; font-size:17px;">调度变更预览</h2>
                <button type="button" class="btn-secondary" onclick="closeModal()" style="min-height:28px; padding:3px 8px;">✕</button>
            </div>
            <div id="modalSummary" style="font-size:12px; color:var(--text-secondary);"></div>
            <div id="modalDiffList" class="modal-body"></div>
            <div style="display:flex; justify-content:flex-end; gap:8px; margin-top:6px;">
                <button type="button" class="btn-secondary" onclick="closeModal()" data-i18n="btnClose">关闭</button>
                <button type="button" class="btn-primary" id="btnModalApply" onclick="executeDirectApply()" data-i18n="btnConfirmApply">确认写回</button>
            </div>
        </div>
    </div>

    <!-- Themed Confirm Modal -->
    <div id="confirmModal" class="modal-backdrop" hidden>
        <div class="modal" role="dialog" aria-modal="true" style="max-width: 440px;">
            <div class="confirm-title" id="confirmTitle"></div>
            <p class="confirm-message" id="confirmMessage"></p>
            <div class="confirm-actions">
                <button type="button" class="btn-secondary" id="confirmCancelBtn">取消</button>
                <button type="button" class="btn-danger" id="confirmOkBtn">确认</button>
            </div>
        </div>
    </div>

`
