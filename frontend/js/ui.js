// ===== UI工具：Toast / 窗口控制 / 主题 / 健康检查 / 邮箱提供商 =====

// Toast 通知
function showToast(msg, type) {
  // 容器
  var container = document.getElementById('toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    document.body.appendChild(container);
  }

  var toast = document.createElement('div');
  toast.className = 'toast-item' + (type === 'error' ? ' toast-error' : ' toast-success');

  // 图标
  var icon = type === 'error'
    ? '<svg viewBox="0 0 24 24" class="toast-icon"><circle cx="12" cy="12" r="10"/><path d="M15 9l-6 6M9 9l6 6"/></svg>'
    : '<svg viewBox="0 0 24 24" class="toast-icon"><circle cx="12" cy="12" r="10"/><path d="M9 12l2 2 4-4"/></svg>';

  toast.innerHTML = icon + '<span class="toast-msg">' + msg + '</span>' +
    '<div class="toast-progress"><div class="toast-progress-bar"></div></div>';

  container.appendChild(toast);

  // 触发入场动画
  requestAnimationFrame(function() { toast.classList.add('show'); });

  // 自动消失
  setTimeout(function() {
    toast.classList.remove('show');
    toast.classList.add('hide');
    setTimeout(function() { toast.remove(); }, 400);
  }, 3000);
}

// 窗口控制
function closeApp() {
  try {
    if (window.runtime && window.runtime.Quit) { window.runtime.Quit(); }
    else { window.close(); }
  } catch (e) { console.error('关闭窗口失败:', e); }
}

function minimizeApp() {
  try {
    if (window.runtime && window.runtime.WindowMinimise) { window.runtime.WindowMinimise(); }
  } catch (e) { console.error('最小化窗口失败:', e); }
}

function maximizeApp() {
  try {
    if (window.runtime && window.runtime.WindowToggleMaximise) { window.runtime.WindowToggleMaximise(); }
  } catch (e) { console.error('最大化窗口失败:', e); }
}

// 主题切换（View Transition 圆形扩展动画）
function toggleTheme(e) {
  // 注入样式禁用所有 transition，防止主题切换闪烁
  var lockStyle = document.createElement('style');
  lockStyle.textContent = '*, *::before, *::after { transition-duration: 0s !important; }';
  document.head.appendChild(lockStyle);

  var applyTheme = function() {
    var html = document.documentElement;
    var isDark = html.getAttribute('data-theme') === 'dark';
    if (isDark) {
      html.removeAttribute('data-theme');
      if (window.appSettings) window.appSettings.theme = 'light';
      document.getElementById('theme-icon-light').style.display = '';
      document.getElementById('theme-icon-dark').style.display = 'none';
    } else {
      html.setAttribute('data-theme', 'dark');
      if (window.appSettings) window.appSettings.theme = 'dark';
      document.getElementById('theme-icon-light').style.display = 'none';
      document.getElementById('theme-icon-dark').style.display = '';
    }
    if (window.appSettings && window.go && window.go.main && window.go.main.App && window.go.main.App.SaveAppSettings) {
      window.go.main.App.SaveAppSettings(window.appSettings).catch(function() {});
    }
  };

  var unlockTransitions = function() {
    setTimeout(function() { lockStyle.remove(); }, 100);
  };

  // 不支持 View Transition 时直接切换
  if (!document.startViewTransition) {
    applyTheme();
    unlockTransitions();
    return;
  }

  var transition = document.startViewTransition(applyTheme);
  transition.finished.then(unlockTransitions);
  transition.ready.then(function() {
    var clientX = 0;
    var clientY = innerHeight;
    var radius = Math.hypot(
      Math.max(clientX, innerWidth - clientX),
      Math.max(clientY, innerHeight - clientY)
    );
    document.documentElement.animate(
      { clipPath: [
        'circle(0% at ' + clientX + 'px ' + clientY + 'px)',
        'circle(' + radius + 'px at ' + clientX + 'px ' + clientY + 'px)'
      ]},
      {
        duration: 500,
        easing: 'ease-in-out',
        pseudoElement: '::view-transition-new(root)'
      }
    );
  });
}

// 快捷键
document.addEventListener('keydown', function(e) {
  // Ctrl+Enter 开始任务
  if (e.ctrlKey && e.key === 'Enter') {
    e.preventDefault();
    if (!document.getElementById('btn-start').disabled) startTask();
  }
  // Esc 停止任务
  if (e.key === 'Escape') {
    if (!document.getElementById('btn-stop').disabled) stopTask();
  }
});

// 当前选中的邮箱提供商
var selectedEmailProvider = 'outlook';
var selectedMoeMailDomains = [];
var allMoeMailDomains = []; // 存储所有可用域名及其配置映射
var selectedCloudMailDomains = [];
var allCloudMailDomains = []; // 存储所有 cloud-mail 域名及对应配置
var moeMailDomainViewState = 'idle';
var cloudMailDomainViewState = 'idle';

// HTML 转义函数
function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

function setInlineTestButton(btn, testing, testingKey) {
  if (!btn) return;
  btn.disabled = !!testing;
  var label = btn.querySelector('[data-i18n="accounts.testConnection"]');
  if (label) label.textContent = testing
    ? _uiT(testingKey || 'moemail.testing', '测试中...')
    : _uiT('accounts.testConnection', '测试连接');
}

function setLocalizedStatus(el, key, vars, fallback) {
  if (!el) return;
  el.dataset.i18nDynamic = key;
  el.dataset.i18nDynamicVars = JSON.stringify(vars || {});
  el.dataset.i18nDynamicFallback = fallback || '';
  el.textContent = _uiT(key, vars || {}, fallback || key);
}

function clearLocalizedStatus(el, text) {
  if (!el) return;
  delete el.dataset.i18nDynamic;
  delete el.dataset.i18nDynamicVars;
  delete el.dataset.i18nDynamicFallback;
  el.textContent = text || '';
}

// 初始化邮箱提供商选择（页面加载时调用）
function initEmailProviderSelection() {
  let provider = 'outlook';
  selectEmailProvider(provider);
}

// 选择邮箱提供商
function selectEmailProvider(provider) {
  selectedEmailProvider = provider;

  // 同步 dropdown 选中显示
  const providerWrap = document.getElementById('cfg-email-provider');
  if (providerWrap && typeof setDropdownValue === 'function') {
    setDropdownValue(providerWrap, provider);
  }

  // 显示/隐藏配置块
  const moemailConfigDiv = document.getElementById('moemail-config-select');
  const cloudmailConfigDiv = document.getElementById('cloudmail-config-select');

  if (moemailConfigDiv) moemailConfigDiv.style.display = (provider === 'moemail') ? 'block' : 'none';
  if (cloudmailConfigDiv) cloudmailConfigDiv.style.display = (provider === 'cloudmail') ? 'block' : 'none';

  // 右栏（邮箱具体选项）随是否有选项动态展开/收起，模态框宽度同步延展/缩回
  const optionsCol = document.getElementById('ntm-email-options');
  const modalContent = document.getElementById('ntm-modal-content');
  const hasRightCol = (provider === 'moemail' || provider === 'cloudmail');
  if (optionsCol) optionsCol.classList.toggle('show', hasRightCol);
  if (modalContent) modalContent.style.maxWidth = hasRightCol ? '520px' : '420px';

  // 选择有域名选项的邮箱类型时加载域名列表
  if (provider === 'moemail') {
    loadMoeMailDomainsToList();
  } else if (provider === 'cloudmail') {
    loadCloudMailDomainsToList();
  }
}

function _uiT(key, varsOrFallback, fallbackMaybe) {
  var vars = null;
  var fallback = fallbackMaybe;
  if (typeof varsOrFallback === 'string') fallback = varsOrFallback;
  else if (varsOrFallback && typeof varsOrFallback === 'object') vars = varsOrFallback;
  if (window.I18N && typeof window.I18N.t === 'function') {
    var v = window.I18N.t(key, vars);
    if (v && v !== key) return v;
  }
  if (fallback == null) return key;
  return vars ? fallback.replace(/\{(\w+)\}/g, function(_, k) { return vars[k] != null ? vars[k] : '{' + k + '}'; }) : fallback;
}

function renderMoeMailDomains() {
  const listDiv = document.getElementById('cfg-moemail-domains-list');
  if (!listDiv) return;
  if (moeMailDomainViewState === 'loading' || moeMailDomainViewState === 'idle') {
    listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">' + _uiT('common.loading', '加载中...') + '</div>';
    return;
  }
  if (moeMailDomainViewState === 'empty') {
    listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">' + _uiT('moemail.noDomainsHint', '暂无配置，请先在设置页添加') + '</div>';
    return;
  }
  if (moeMailDomainViewState === 'unavailable') {
    listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">' + _uiT('moemail.noActiveDomain', '暂无可用域名，请先测试配置') + '</div>';
    return;
  }
  if (moeMailDomainViewState === 'error') {
    listDiv.innerHTML = '<div style="text-align:center;color:var(--danger);font-size:12px;padding:12px;">' + _uiT('common.loadFailed', '加载失败') + '</div>';
    return;
  }
  if (!selectedMoeMailDomains.length) selectedMoeMailDomains = ['__random__'];
  var html = '<div class="domain-mode-row">'
    + '<div class="domain-mode-btn" data-domain="__random__" onclick="toggleMoeMailDomain(\'__random__\')">'
    + '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/><line x1="4" y1="4" x2="9" y2="9"/></svg>'
    + _uiT('register.modeRandom', '随机') + '</div>'
    + '<div class="domain-mode-btn" data-domain="__all__" onclick="toggleMoeMailDomain(\'__all__\')">'
    + '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 014-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 01-4 4H3"/></svg>'
    + _uiT('register.modeRoundRobin', '轮询') + '</div></div><div class="domain-chips-wrap">';
  html += allMoeMailDomains.map(function(item) {
    return '<div class="domain-chip" data-domain="' + escapeHtml(item.domain) + '" onclick="toggleMoeMailDomain(\'' + escapeHtml(item.domain) + '\')" title="' + _uiT('register.configCount', { n: item.configs.length }, '{n} 个配置') + '">' + escapeHtml(item.domain) + '</div>';
  }).join('');
  listDiv.innerHTML = html + '</div>';
  updateDomainOptionStyles();
}

// 加载 MoeMail 域名到列表
async function loadMoeMailDomainsToList() {
  const listDiv = document.getElementById('cfg-moemail-domains-list');
  if (!listDiv) return;

  moeMailDomainViewState = 'loading';
  renderMoeMailDomains();

  try {
    const configs = await window.go.main.App.GetMoeMailConfigs();

    if (!configs || configs.length === 0) {
      allMoeMailDomains = [];
      moeMailDomainViewState = 'empty';
      renderMoeMailDomains();
      return;
    }

    let configStatus = {};
    try {
      const saved = localStorage.getItem('moemail-config-status');
      if (saved) configStatus = JSON.parse(saved);
    } catch (e) {}

    allMoeMailDomains = [];
    const domainConfigMap = {};

    for (const cfg of configs) {
      const status = configStatus[cfg.name];
      if (status && status.tested && status.success && status.domains && status.domains.length > 0) {
        for (const domain of status.domains) {
          if (!domainConfigMap[domain]) domainConfigMap[domain] = [];
          domainConfigMap[domain].push(cfg);
        }
      }
    }

    allMoeMailDomains = Object.keys(domainConfigMap).map(domain => ({
      domain: domain,
      configs: domainConfigMap[domain]
    }));

    if (allMoeMailDomains.length === 0) {
      moeMailDomainViewState = 'unavailable';
      renderMoeMailDomains();
      return;
    }
    moeMailDomainViewState = 'ready';
    renderMoeMailDomains();

  } catch (e) {
    console.error('加载 MoeMail 域名失败:', e);
    moeMailDomainViewState = 'error';
    renderMoeMailDomains();
  }
}

// 更新域名选项的视觉状态
function updateDomainOptionStyles() {
  const container = document.getElementById('cfg-moemail-domains-list');
  if (!container) return;
  container.querySelectorAll('.domain-mode-btn').forEach(el => {
    const domain = el.getAttribute('data-domain');
    el.classList.toggle('selected', selectedMoeMailDomains.includes(domain));
  });
  container.querySelectorAll('.domain-chip').forEach(el => {
    const domain = el.getAttribute('data-domain');
    el.classList.toggle('selected', selectedMoeMailDomains.includes(domain));
  });
}

// 切换域名选择
function toggleMoeMailDomain(domain, el) {
  const isSelected = selectedMoeMailDomains.includes(domain);

  if (domain === '__random__' || domain === '__all__') {
    if (isSelected) {
      selectedMoeMailDomains = selectedMoeMailDomains.filter(d => d !== domain);
    } else {
      selectedMoeMailDomains = [domain];
    }
  } else {
    // 点击具体域名：先清除 __random__ 和 __all__
    selectedMoeMailDomains = selectedMoeMailDomains.filter(d => d !== '__random__' && d !== '__all__');
    if (isSelected) {
      selectedMoeMailDomains = selectedMoeMailDomains.filter(d => d !== domain);
    } else {
      selectedMoeMailDomains.push(domain);
    }
  }

  updateDomainOptionStyles();
}

// 全选域名
function selectAllMoeMailDomains() {
  selectedMoeMailDomains = allMoeMailDomains.map(item => item.domain);
  updateDomainOptionStyles();
}

// ===== Cloud-Mail 域名加载/选择 =====
function renderCloudMailDomains() {
  const listDiv = document.getElementById('cfg-cloudmail-domains-list');
  if (!listDiv) return;
  if (cloudMailDomainViewState === 'loading' || cloudMailDomainViewState === 'idle') {
    listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">' + _uiT('common.loading', '加载中...') + '</div>';
    return;
  }
  if (cloudMailDomainViewState === 'empty') {
    listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">' + _uiT('cloudmail.noDomainsHint', '暂无配置，请先在邮箱池页添加') + '</div>';
    return;
  }
  if (cloudMailDomainViewState === 'unavailable') {
    listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">' + _uiT('cloudmail.noActiveDomain', '暂无可用域名，请先测试 Cloud-Mail 配置') + '</div>';
    return;
  }
  if (cloudMailDomainViewState === 'error') {
    listDiv.innerHTML = '<div style="text-align:center;color:var(--danger);font-size:12px;padding:12px;">' + _uiT('common.loadFailed', '加载失败') + '</div>';
    return;
  }
  if (!selectedCloudMailDomains.length) selectedCloudMailDomains = ['__random__'];
  var html = '<div class="domain-mode-row">'
    + '<div class="domain-mode-btn" data-domain="__random__" onclick="toggleCloudMailDomain(\'__random__\')">'
    + '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/><line x1="4" y1="4" x2="9" y2="9"/></svg>'
    + _uiT('register.modeRandom', '随机') + '</div>'
    + '<div class="domain-mode-btn" data-domain="__all__" onclick="toggleCloudMailDomain(\'__all__\')">'
    + '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 014-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 01-4 4H3"/></svg>'
    + _uiT('register.modeRoundRobin', '轮询') + '</div></div><div class="domain-chips-wrap">';
  html += allCloudMailDomains.map(function(item) {
    return '<div class="domain-chip" data-domain="' + escapeHtml(item.domain) + '" onclick="toggleCloudMailDomain(\'' + escapeHtml(item.domain) + '\')" title="' + _uiT('register.configCount', { n: item.configs.length }, '{n} 个配置') + '">' + escapeHtml(item.domain) + '</div>';
  }).join('');
  listDiv.innerHTML = html + '</div>';
  updateCloudMailDomainStyles();
}

async function loadCloudMailDomainsToList() {
  const listDiv = document.getElementById('cfg-cloudmail-domains-list');
  if (!listDiv) return;
  cloudMailDomainViewState = 'loading';
  renderCloudMailDomains();
  try {
    const configs = await window.go.main.App.GetCloudMailConfigs();
    if (!configs || configs.length === 0) {
      allCloudMailDomains = [];
      cloudMailDomainViewState = 'empty';
      renderCloudMailDomains();
      return;
    }
    let configStatus = {};
    try {
      const saved = localStorage.getItem('cloudmail-config-status');
      if (saved) configStatus = JSON.parse(saved);
    } catch (e) {}
    const domainConfigMap = {};
    for (const cfg of configs) {
      const status = configStatus[cfg.name];
      if (!status || !status.tested || !status.success) continue;
      const domains = (status.domains && status.domains.length > 0) ? status.domains : (cfg.domains || []);
      for (const domain of domains) {
        if (!domainConfigMap[domain]) domainConfigMap[domain] = [];
        domainConfigMap[domain].push(cfg);
      }
    }
    allCloudMailDomains = Object.keys(domainConfigMap).map(domain => ({ domain: domain, configs: domainConfigMap[domain] }));
    cloudMailDomainViewState = allCloudMailDomains.length ? 'ready' : 'unavailable';
    renderCloudMailDomains();
  } catch (e) {
    console.error('加载 Cloud-Mail 域名失败:', e);
    cloudMailDomainViewState = 'error';
    renderCloudMailDomains();
  }
}

function updateCloudMailDomainStyles() {
  const container = document.getElementById('cfg-cloudmail-domains-list');
  if (!container) return;
  container.querySelectorAll('.domain-mode-btn').forEach(el => {
    const d = el.getAttribute('data-domain');
    el.classList.toggle('selected', selectedCloudMailDomains.includes(d));
  });
  container.querySelectorAll('.domain-chip').forEach(el => {
    const d = el.getAttribute('data-domain');
    el.classList.toggle('selected', selectedCloudMailDomains.includes(d));
  });
}

function toggleCloudMailDomain(domain) {
  const isSelected = selectedCloudMailDomains.includes(domain);
  if (domain === '__random__' || domain === '__all__') {
    if (isSelected) {
      selectedCloudMailDomains = selectedCloudMailDomains.filter(d => d !== domain);
    } else {
      selectedCloudMailDomains = [domain];
    }
  } else {
    selectedCloudMailDomains = selectedCloudMailDomains.filter(d => d !== '__random__' && d !== '__all__');
    if (isSelected) {
      selectedCloudMailDomains = selectedCloudMailDomains.filter(d => d !== domain);
    } else {
      selectedCloudMailDomains.push(domain);
    }
  }
  updateCloudMailDomainStyles();
}

function selectAllCloudMailDomains() {
  selectedCloudMailDomains = allCloudMailDomains.map(item => item.domain);
  updateCloudMailDomainStyles();
}

window.addEventListener('i18n:changed', function() {
  renderMoeMailDomains();
  renderCloudMailDomains();
  document.querySelectorAll('[data-i18n-dynamic]').forEach(function(el) {
    var vars = {};
    try { vars = JSON.parse(el.dataset.i18nDynamicVars || '{}'); } catch (e) {}
    el.textContent = _uiT(el.dataset.i18nDynamic, vars, el.dataset.i18nDynamicFallback || el.dataset.i18nDynamic);
  });
});
