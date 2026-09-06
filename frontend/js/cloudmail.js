// ===== Cloud-Mail 配置管理 =====

let cloudmailConfigs = [];
let cloudmailConfigStatus = {};

function _cmT(key, varsOrFallback, fallbackMaybe) {
  var vars = null, fallback = null;
  if (typeof varsOrFallback === 'string') {
    fallback = varsOrFallback;
  } else if (varsOrFallback && typeof varsOrFallback === 'object') {
    vars = varsOrFallback;
    if (typeof fallbackMaybe === 'string') fallback = fallbackMaybe;
  }
  if (window.I18N && typeof window.I18N.t === 'function') {
    var v = window.I18N.t(key, vars);
    if (v && v !== key) return v;
  }
  if (fallback != null) {
    if (vars) {
      return fallback.replace(/\{(\w+)\}/g, function(_, k) {
        return vars[k] != null ? vars[k] : '{' + k + '}';
      });
    }
    return fallback;
  }
  return key;
}

async function loadCloudMailConfigs() {
  try {
    const configs = await window.go.main.App.GetCloudMailConfigs();
    cloudmailConfigs = configs || [];
    loadCloudMailConfigStatus();
    updateCloudMailUI();
    return configs;
  } catch (e) {
    console.error('[CloudMail] 加载配置失败:', e);
    cloudmailConfigs = [];
    return [];
  }
}

function loadCloudMailConfigStatus() {
  try {
    const saved = localStorage.getItem('cloudmail-config-status');
    if (saved) {
      cloudmailConfigStatus = JSON.parse(saved);
    }
  } catch (e) {
    cloudmailConfigStatus = {};
  }
}

function saveCloudMailConfigStatus() {
  try {
    localStorage.setItem('cloudmail-config-status', JSON.stringify(cloudmailConfigStatus));
  } catch (e) {}
}

function updateCloudMailUI() {
  let activeCount = 0;
  cloudmailConfigs.forEach(cfg => {
    const status = cloudmailConfigStatus[cfg.name];
    if (status && status.tested && status.success) {
      activeCount++;
    }
  });

  const summaryEl = document.getElementById('settings-cloudmail-summary');
  if (summaryEl) {
    if (cloudmailConfigs.length === 0) {
      summaryEl.textContent = _cmT('cloudmail.summaryNone', '未配置');
    } else {
      summaryEl.textContent = _cmT('cloudmail.summaryActive', { n: cloudmailConfigs.length, m: activeCount }, '已配置 {n} 个，可用 {m} 个');
    }
  }

  renderCloudMailConfigList();
}

function renderCloudMailConfigList() {
  const inlineList = document.getElementById('cloudmail-inline-list');
  if (!inlineList) return;

  if (cloudmailConfigs.length === 0) {
    inlineList.innerHTML = `
      <div class="moemail-empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
          <polyline points="22,6 12,13 2,6"></polyline>
        </svg>
        <div>${_cmT('cloudmail.emptyInline', '暂无配置，请在上方添加 Cloud-Mail 配置')}</div>
      </div>
    `;
    return;
  }

  inlineList.innerHTML = cloudmailConfigs.map((cfg, idx) => {
    const status = cloudmailConfigStatus[cfg.name] || { tested: false };
    let dotClass = 'untested';
    let statusLabel = _cmT('status.untested', '未测试');
    let statusClass = 'untested';
    let domainsHtml = '';

    // 优先用服务器拉到的域名（自动发现），其次回退到配置里手填的
    const domains = (status.domains && status.domains.length > 0) ? status.domains : (cfg.domains || []);
    if (domains.length > 0) {
      domainsHtml = '<div class="moemail-domain-tags">' +
        domains.map(d => '<span class="moemail-domain-tag">' + escapeHtml(d) + '</span>').join('') +
        '</div>';
    }

    if (status.tested && status.success) {
      dotClass = 'success';
      statusLabel = _cmT('status.available', '可用');
      statusClass = 'success';
    } else if (status.tested) {
      dotClass = 'error';
      statusLabel = _cmT('status.unavailable', '不可用');
      statusClass = 'error';
    }

    return `
      <div class="moemail-config-item">
        <div class="moemail-config-main">
          <div class="moemail-status-dot ${dotClass}"></div>
          <div class="moemail-config-info">
            <div class="moemail-config-name">${escapeHtml(cfg.name)}</div>
            <div class="moemail-config-details">
              <span class="moemail-config-url">${escapeHtml(cfg.url)}</span>
              <span class="moemail-config-status ${statusClass}">${statusLabel}</span>
            </div>
            ${domainsHtml}
          </div>
        </div>
        <div class="moemail-config-actions">
          <button onclick="testCloudMailConfigByIndex(${idx})" class="btn btn-secondary btn-sm">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 11.08V12a10 10 0 11-5.93-9.14"/>
              <polyline points="22 4 12 14.01 9 11.01"/>
            </svg>
            ${_cmT('common.test', '测试')}
          </button>
          <button onclick="deleteCloudMailConfig(${idx})" class="btn btn-secondary btn-sm" style="color:var(--danger);">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="3 6 5 6 21 6"/>
              <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/>
            </svg>
            ${_cmT('common.delete', '删除')}
          </button>
        </div>
      </div>
    `;
  }).join('');
}

function generateCloudMailName() {
  var prefix = _cmT('cloudmail.autoNamePrefix', '配置');
  let idx = cloudmailConfigs.length + 1;
  let name = prefix + ' ' + idx;
  while (cloudmailConfigs.some(c => c.name === name)) {
    idx++;
    name = prefix + ' ' + idx;
  }
  return name;
}

async function inlineAddCloudMail() {
  var name = (document.getElementById('cloudmail-inline-name').value || '').trim();
  var url = (document.getElementById('cloudmail-inline-url').value || '').trim();
  var em = (document.getElementById('cloudmail-inline-email').value || '').trim();
  var pwd = (document.getElementById('cloudmail-inline-password').value || '').trim();

  if (!url || !em || !pwd) {
    showToast(_cmT('cloudmail.requiredFields', '请填写 URL、管理员邮箱、密码'), 'error');
    return;
  }
  // 域名在服务器端通过 /api/setting/websiteConfig 自动获取，无需用户输入
  if (!name) name = generateCloudMailName();
  if (cloudmailConfigs.some(c => c.name === name)) {
    showToast(_cmT('cloudmail.nameExists', '配置名称已存在'), 'error');
    return;
  }

  // 先测试连接，成功后才保存
  var btn = document.getElementById('cloudmail-inline-test-btn');
  var statusEl = document.getElementById('cloudmail-inline-status');
  setInlineTestButton(btn, true, 'cloudmail.testing');
  if (statusEl) { statusEl.style.color = ''; clearLocalizedStatus(statusEl); }

  var testPayload = { name: name, url: url, email: em, password: pwd, domains: [] };
  var testResult;
  try {
    testResult = await window.go.main.App.TestCloudMailConnection(JSON.stringify(testPayload));
  } catch (e) {
    testResult = { error: String(e) };
  } finally {
    setInlineTestButton(btn, false, 'cloudmail.testing');
  }

  if (!testResult || testResult.error) {
    var errMsg = (testResult && testResult.error) || _cmT('cloudmail.testFailedShort', '测试失败');
    if (statusEl) { statusEl.style.color = 'var(--danger)'; clearLocalizedStatus(statusEl, errMsg); }
    showToast(_cmT('cloudmail.cannotSaveUntilOk', '连接测试未通过，未保存配置：') + errMsg, 'error');
    return;
  }

  var fetchedDomains = testResult.domains || [];
  const newConfig = { name: name, url: url, email: em, password: pwd, domains: fetchedDomains };
  cloudmailConfigs.push(newConfig);
  const saveResult = await window.go.main.App.SaveCloudMailConfigs(JSON.stringify(cloudmailConfigs));
  if (saveResult.error) {
    cloudmailConfigs.pop();
    showToast(_cmT('toast.operationFailed', '保存失败') + ': ' + saveResult.error, 'error');
    return;
  }

  // 标记为已测试通过
  cloudmailConfigStatus[name] = { tested: true, success: true, domains: fetchedDomains };
  saveCloudMailConfigStatus();

  document.getElementById('cloudmail-inline-name').value = '';
  document.getElementById('cloudmail-inline-url').value = '';
  document.getElementById('cloudmail-inline-email').value = '';
  document.getElementById('cloudmail-inline-password').value = '';
  if (statusEl) { statusEl.style.color = 'var(--success)'; clearLocalizedStatus(statusEl); }

  if (fetchedDomains.length > 0) {
    showToast(_cmT('cloudmail.addedWithDomains', { name: name, n: fetchedDomains.length }, '已添加 {name}，{n} 个域名'));
  } else {
    showToast(_cmT('cloudmail.addedNamed', { name: name }, '已添加: {name}'));
  }
  renderCloudMailConfigList();
  updateCloudMailUI();
}

async function inlineTestCloudMail() {
  var url = (document.getElementById('cloudmail-inline-url').value || '').trim();
  var em = (document.getElementById('cloudmail-inline-email').value || '').trim();
  var pwd = (document.getElementById('cloudmail-inline-password').value || '').trim();

  if (!url || !em || !pwd) {
    showToast(_cmT('cloudmail.requiredFields', '请填写 URL、管理员邮箱、密码'), 'error');
    return;
  }
  var btn = document.getElementById('cloudmail-inline-test-btn');
  var statusEl = document.getElementById('cloudmail-inline-status');
  setInlineTestButton(btn, true, 'cloudmail.testing');
  if (statusEl) clearLocalizedStatus(statusEl);
  try {
    var result = await window.go.main.App.TestCloudMailConnection(JSON.stringify({
      name: 'inline-test', url: url, email: em, password: pwd, domains: []
    }));
    if (result.success) {
      var fetched = result.domains || [];
      if (statusEl) {
        statusEl.style.color = 'var(--success)';
        if (fetched.length > 0) {
          setLocalizedStatus(statusEl, 'cloudmail.connectedDomainsList', { d: fetched.join(', ') }, '连接成功，域名: {d}');
        } else {
          setLocalizedStatus(statusEl, 'cloudmail.connectedNoDomain', {}, '连接成功，但服务器未返回域名（可能开启了 loginDomain 隐私开关）');
        }
      }
    } else {
      if (statusEl) {
        statusEl.style.color = 'var(--danger)';
        if (result.error) clearLocalizedStatus(statusEl, result.error);
        else setLocalizedStatus(statusEl, 'cloudmail.testFailed', {}, '连接失败');
      }
    }
  } catch(e) {
    if (statusEl) {
      statusEl.style.color = 'var(--danger)';
      setLocalizedStatus(statusEl, 'cloudmail.testFailedShort', {}, '测试失败');
    }
  }
  setInlineTestButton(btn, false, 'cloudmail.testing');
}

async function testCloudMailConfigByIndex(index) {
  if (index < 0 || index >= cloudmailConfigs.length) return;
  const config = cloudmailConfigs[index];
  try {
    const result = await window.go.main.App.TestCloudMailConnection(JSON.stringify(config));
    if (result.error) {
      cloudmailConfigStatus[config.name] = { tested: true, success: false };
      saveCloudMailConfigStatus();
      renderCloudMailConfigList();
      updateCloudMailUI();
      showToast(config.name + ': ' + result.error, 'error');
    } else {
      const domains = result.domains || [];
      cloudmailConfigStatus[config.name] = { tested: true, success: true, domains: domains };
      saveCloudMailConfigStatus();
      renderCloudMailConfigList();
      updateCloudMailUI();
      if (domains.length > 0) {
        showToast(config.name + ': ' + _cmT('cloudmail.testOkWithDomains', { n: domains.length }, '连接成功，{n} 个域名'), 'success');
      } else {
        showToast(config.name + ': ' + _cmT('cloudmail.testOkNoDomain', '连接成功，但服务器未返回域名'), 'success');
      }
    }
  } catch (e) {
    cloudmailConfigStatus[config.name] = { tested: true, success: false };
    saveCloudMailConfigStatus();
    renderCloudMailConfigList();
    updateCloudMailUI();
    showToast(config.name + ': ' + _cmT('cloudmail.testFailedShort', '测试失败'), 'error');
  }
}

async function deleteCloudMailConfig(index) {
  if (index < 0 || index >= cloudmailConfigs.length) return;
  const configName = cloudmailConfigs[index].name;
  showConfirmModal(
    _cmT('cloudmail.deleteConfigTitle', '删除配置'),
    _cmT('cloudmail.deleteConfigMsg', { name: configName }, '确认删除配置 "{name}" 吗？'),
    _cmT('accounts.deleteConfirm', '确认删除'),
    async function() {
      cloudmailConfigs.splice(index, 1);
      try {
        const result = await window.go.main.App.SaveCloudMailConfigs(JSON.stringify(cloudmailConfigs));
        if (result.error) {
          showToast(_cmT('toast.deleteFailed', '删除失败') + ': ' + result.error, 'error');
          await loadCloudMailConfigs();
          return;
        }
        delete cloudmailConfigStatus[configName];
        saveCloudMailConfigStatus();
        updateCloudMailUI();
        showToast(_cmT('toast.deleteOk', '删除成功'), 'success');
      } catch (e) {
        showToast(_cmT('toast.deleteFailed', '删除失败') + ': ' + e, 'error');
        await loadCloudMailConfigs();
      }
    }
  );
}

async function autoTestAllCloudMailConfigs() {
  if (cloudmailConfigs.length === 0) return;
  console.log('[CloudMail] 启动自动测试，共 ' + cloudmailConfigs.length + ' 个配置');
  for (let i = 0; i < cloudmailConfigs.length; i++) {
    const config = cloudmailConfigs[i];
    try {
      const result = await window.go.main.App.TestCloudMailConnection(JSON.stringify(config));
      if (result.error) {
        cloudmailConfigStatus[config.name] = { tested: true, success: false };
      } else {
        const domains = result.domains || [];
        cloudmailConfigStatus[config.name] = { tested: true, success: true, domains: domains };
      }
    } catch (e) {
      cloudmailConfigStatus[config.name] = { tested: true, success: false };
    }
  }
  saveCloudMailConfigStatus();
  updateCloudMailUI();
  console.log('[CloudMail] 自动测试完成');
}

document.addEventListener('DOMContentLoaded', async function() {
  await loadCloudMailConfigs();
  autoTestAllCloudMailConfigs();
});

window.addEventListener('i18n:changed', function() {
  try { if (typeof updateCloudMailUI === 'function') updateCloudMailUI(); } catch (e) {}
  var btn = document.getElementById('cloudmail-inline-test-btn');
  if (btn) setInlineTestButton(btn, btn.disabled, 'cloudmail.testing');
});
