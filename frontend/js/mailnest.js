// ===== MailNest 配置管理 =====
let mailnestConfig = {};


// 内联测试 MailNest 配置
async function inlineTestMailNest() {
    var apiKey = document.getElementById('mailnest-inline-apikey').value.trim();
    var projectCode = document.getElementById('mailnest-inline-project-code').value.trim();
    if (!projectCode || !apiKey) {
        showToast(_mmT('mailnest.requiredKeyProjectCodeShort', '请填写 Key 和 项目代码'), 'error');
        return;
    }
    var btn = document.getElementById('mailnest-inline-test-btn');
    var statusEl = document.getElementById('mailnest-inline-status');
    setInlineTestButton(btn, true, 'moemail.testing');
    clearLocalizedStatus(statusEl);
    try {
        var result = await window.go.main.App.TestMailNestConnection(JSON.stringify({
            projectCode: projectCode, apiKey: apiKey
        }));
        if (result.success) {
            statusEl.style.color = 'var(--success)';
            setLocalizedStatus(statusEl, 'mailnest.balance', {n: (result.balance || 0)}, '测试成功，余额为 {n}');
        } else {
            statusEl.style.color = 'var(--danger)';
            if (result.error) clearLocalizedStatus(statusEl, result.error);
            else setLocalizedStatus(statusEl, 'moemail.testFailed', {}, '连接失败');
        }
    } catch (e) {
        statusEl.style.color = 'var(--danger)';
        setLocalizedStatus(statusEl, 'moemail.testFailedShort', {}, '测试失败');
    }
    setInlineTestButton(btn, false, 'moemail.testing');
}

// 内联添加 MailNest 配置
async function inlineAddMailNest() {
    var apiKey = document.getElementById('mailnest-inline-apikey').value.trim();
    var projectCode = document.getElementById('mailnest-inline-project-code').value.trim();
    if (!projectCode || !apiKey) {
        showToast(_mmT('mailnest.requiredKeyProjectCodeShort', '请填写 Key 和 项目代码'), 'error');
        return;
    }

    // 先测试连接，成功后才保存
    var btn = document.getElementById('mailnest-inline-test-btn');
    var statusEl = document.getElementById('mailnest-inline-status');
    setInlineTestButton(btn, true, 'moemail.testing');
    clearLocalizedStatus(statusEl);
    statusEl.style.color = '';
    clearLocalizedStatus(statusEl);
    var testResult;
    try {
        testResult = await window.go.main.App.TestMailNestConnection(JSON.stringify({
            projectCode: projectCode, apiKey: apiKey
        }));
    } catch (e) {
        testResult = {error: String(e)};
    } finally {
        if (btn) {
            setInlineTestButton(btn, false, 'moemail.testing');
        }
    }

    if (!testResult || testResult.error) {
        var errMsg = (testResult && testResult.error) || _mmT('moemail.testFailedShort', '测试失败');
        if (statusEl) {
            statusEl.style.color = 'var(--danger)';
            clearLocalizedStatus(statusEl, errMsg);
        }
        showToast(_mmT('moemail.cannotSaveUntilOk', '连接测试未通过，未保存配置：') + errMsg, 'error');
        return;
    }

    mailnestConfig['apiKey'] = apiKey;
    mailnestConfig['projectCode'] = projectCode;
    const saveResult = await window.go.main.App.SaveMailNestConfig(JSON.stringify(mailnestConfig));
    if (saveResult.error) {
        showToast(_mmT('toast.operationFailed', '保存失败') + ': ' + saveResult.error, 'error');
        return;
    }
    // 更新设置页摘要
    const summaryEl = document.getElementById('settings-mailnest-summary');
    if (summaryEl) {
        summaryEl.textContent = _mmT('mailnest.summaryActive', "已配置");
    }
}

// 加载 MailNest 配置
async function loadMailNestConfig() {
    try {
        mailnestConfig = await window.go.main.App.GetMailNestConfig()
        updateMailNestUI();
    } catch (e) {
        console.error('[MailNest] 加载配置失败:', e);
        mailnestConfig = {}
    }
}

function updateMailNestUI() {
    const summaryEl = document.getElementById('settings-mailnest-summary');
    if (mailnestConfig.projectCode) {
        document.getElementById('mailnest-inline-apikey').value = mailnestConfig.apiKey;
        document.getElementById('mailnest-inline-project-code').value = mailnestConfig.projectCode;
        summaryEl.textContent = _mmT('mailnest.summaryActive', "已配置");
    } else {
        document.getElementById('mailnest-inline-apikey').value = '';
        document.getElementById('mailnest-inline-project-code').value = '';
        summaryEl.textContent = _mmT('mailnest.summaryNone', "未配置");
    }
}

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', async function () {
    await loadMailNestConfig();
});

// 语言切换后重新渲染 MailNest UI（状态/摘要/空态等动态文本）
window.addEventListener('i18n:changed', function () {
    try {
        if (typeof updateMailNestUI === 'function') updateMailNestUI();
    } catch (e) {
    }
    var btn = document.getElementById('mailnest-inline-test-btn');
    if (btn) setInlineTestButton(btn, btn.disabled, 'moemail.testing');
});
