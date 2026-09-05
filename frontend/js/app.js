// ===== 核心：导航 / 标签页 / 下拉框 / 配置 / Toast / 窗口控制 =====

// 页面切换
var _currentPageId = 'overview';
function getPageTitle(pageId) {
  if (window.I18N && pageId) {
    var v = window.I18N.t('page.' + pageId);
    if (v && v !== 'page.' + pageId) return v;
  }
  var fallback = { overview: '概览', logs: '运行日志', register: '注册', accounts: '邮箱池', ip: 'IP 管理', info: '关于', settings: '设置' };
  return fallback[pageId] || pageId;
}
function switchPage(pageId) {
  _currentPageId = pageId;
  document.querySelectorAll('.page, .page-placeholder, .page-iframe').forEach(function(p) {
    p.classList.remove('active');
  });
  var target = document.getElementById('page-' + pageId);
  if (target) target.classList.add('active');
  document.querySelectorAll('.nav-item[data-page]').forEach(function(item) {
    item.classList.toggle('active', item.getAttribute('data-page') === pageId);
  });
  document.getElementById('titlebar-text').textContent = getPageTitle(pageId);
  if (pageId === 'overview') {
    startOverviewTimer();
  } else {
    stopOverviewTimer();
  }
  if (pageId === 'ip') {
    loadIpList();
  }
  if (pageId === 'accounts') {
    loadOutlookAccountsList();
    startOutlookAutoRefresh();
    if (typeof loadICloudAccountsList === 'function') loadICloudAccountsList();
  } else {
    stopOutlookAutoRefresh();
  }
  if (pageId === 'info') {
    loadInfoVersion();
  }
}

async function loadInfoVersion() {
  try {
    var data = await window.go.main.App.GetOverview();
    var ver = (data && data.version) ? data.version : '';
    if (ver) {
      ['info-version-detail', 'info-version-detail2'].forEach(function(id) {
        var el = document.getElementById(id);
        if (el) el.textContent = ver;
      });
    }
  } catch(e) {}

  // 从 GitHub 加载最新 release 信息
  var changelogEl = document.getElementById('info-changelog');
  var latestEl = document.getElementById('info-latest-version');
  var dateEl = document.getElementById('info-release-date');
  var tagEl = document.getElementById('info-changelog-version');
  if (changelogEl) changelogEl.innerHTML = '<span style="color:var(--text-muted);">' + tr('common.loading', '加载中...') + '</span>';
  try {
    var result = await window.go.main.App.CheckUpdate();
    if (result.error) {
      if (changelogEl) changelogEl.innerHTML = '<span style="color:var(--text-muted);">' + tr('common.loadFailed', '加载失败') + ': ' + result.error + '</span>';
      return;
    }
    if (latestEl) {
      latestEl.textContent = result.latestVersion || '-';
      latestEl.style.color = result.hasUpdate ? 'var(--success)' : 'var(--text)';
    }
    if (dateEl) dateEl.textContent = result.releaseDate || '-';
    if (tagEl) tagEl.textContent = result.latestVersion || '-';
    var banner = document.getElementById('info-update-banner');
    var bannerVer = document.getElementById('info-banner-version');
    if (banner) banner.style.display = result.hasUpdate ? 'block' : 'none';
    if (bannerVer) bannerVer.textContent = result.latestVersion || '';
    if (changelogEl) {
      var body = (result.changelog || '').trim();
      changelogEl.innerHTML = body ? renderChangelog(body) : '<span style="color:var(--text-muted);">' + tr('common.noData', '暂无更新说明') + '</span>';
    }
  } catch(e) {
    if (changelogEl) changelogEl.innerHTML = '<span style="color:var(--text-muted);">' + tr('common.loadFailed', '加载失败') + '</span>';
  }
}

// 翻译辅助：t() 返回 key 自身时回落到 fallback
function tr(key, fallback) {
  if (window.I18N && typeof window.I18N.t === 'function') {
    var v = window.I18N.t(key);
    if (v && v !== key) return v;
  }
  return fallback != null ? fallback : key;
}

// 存储目录设置
async function loadDataDir() {
  try {
    var dir = await window.go.main.App.GetDataDir();
    document.getElementById('cfg-data-dir').value = dir || '';
  } catch(e) {}
}

async function selectDataDir() {
  try {
    var path = await window.go.main.App.SelectDirectory();
    if (!path) return;
    var result = await window.go.main.App.SetDataDir(path);
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    document.getElementById('cfg-data-dir').value = result.path;
    showToast(tr('toast.dataDirSet', '存储目录已设置'));
  } catch(e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
  }
}

async function resetDataDir() {
  try {
    var result = await window.go.main.App.ResetDataDir();
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    document.getElementById('cfg-data-dir').value = result.path;
    showToast(tr('toast.dataDirReset', '已重置为默认存储目录'));
  } catch(e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
  }
}

// 注册结果输出目录设置
async function loadResultOutputDir() {
  try {
    var dir = await window.go.main.App.GetResultOutputDir();
    var el = document.getElementById('cfg-result-output-dir');
    if (el) el.value = dir || '';
  } catch(e) {}
}

async function selectResultOutputDir() {
  try {
    var path = await window.go.main.App.SelectDirectory();
    if (!path) return;
    var result = await window.go.main.App.SetResultOutputDir(path);
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    document.getElementById('cfg-result-output-dir').value = result.path;
    showToast(tr('toast.outputDirSet', '输出目录已设置') + ': ' + result.path);
  } catch(e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
  }
}

async function resetResultOutputDir() {
  try {
    var result = await window.go.main.App.ResetResultOutputDir();
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    document.getElementById('cfg-result-output-dir').value = result.path;
    showToast(tr('toast.outputDirReset', '已重置为默认输出目录'));
  } catch(e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
  }
}

// 代理设置
async function loadProxy() {
  try {
    var p = await window.go.main.App.GetProxy();
    var el = document.getElementById('cfg-proxy');
    if (el) el.value = p || '';
  } catch(e) {}
}

function renderProxyDetectCard(state, payload) {
  var box = document.getElementById('proxy-detect-card');
  if (!box) return;
  if (state === 'hidden') { box.style.display = 'none'; box.innerHTML = ''; return; }
  box.style.display = 'block';
  var base = 'border:1px solid var(--border);border-radius:8px;padding:10px 12px;font-size:12px;';
  if (state === 'loading') {
    box.style.cssText = base + 'background:var(--card-bg, transparent);color:var(--muted);';
    box.innerHTML = '正在检测代理出口…';
    return;
  }
  if (state === 'ok') {
    var loc = [payload.country, payload.region, payload.city].filter(Boolean).join(' · ');
    box.style.cssText = base + 'background:rgba(16,185,129,0.08);border-color:rgba(16,185,129,0.35);';
    box.innerHTML =
      '<div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">' +
        '<span style="font-weight:600;color:#10b981;">✓ 可用</span>' +
        '<span style="padding:1px 6px;border-radius:4px;background:rgba(16,185,129,0.15);color:#10b981;font-size:11px;font-weight:600;">' + (payload.scheme || '').toUpperCase() + '</span>' +
        '<span style="color:var(--text);font-weight:600;">' + (payload.ip || '') + '</span>' +
        (loc ? '<span style="color:var(--muted);">· ' + loc + '</span>' : '') +
      '</div>' +
      (payload.isp ? '<div style="margin-top:4px;color:var(--muted);font-size:11px;">' + payload.isp + '</div>' : '');
    return;
  }
  // error
  box.style.cssText = base + 'background:rgba(239,68,68,0.08);border-color:rgba(239,68,68,0.35);color:#ef4444;';
  box.innerHTML = '✗ 检测失败：' + (payload && payload.error ? payload.error : '未知错误');
}

async function saveProxy() {
  var el = document.getElementById('cfg-proxy');
  if (!el) return;
  try {
    if (el.value.trim()) renderProxyDetectCard('loading');
    else renderProxyDetectCard('hidden');
    var result = await window.go.main.App.SetProxy(el.value.trim());
    if (result.error) {
      showToast(result.error, 'error');
      renderProxyDetectCard('hidden');
      return;
    }
    el.value = result.proxy || '';
    if (!result.proxy) {
      renderProxyDetectCard('hidden');
      showToast(tr('toast.proxyCleared', '代理已清除'));
      return;
    }
    showToast(tr('toast.proxySaved', '代理已保存'));
    var d = result.detect;
    if (d && d.ok) renderProxyDetectCard('ok', d);
    else renderProxyDetectCard('error', d || {});
  } catch(e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
    renderProxyDetectCard('error', { error: e.message });
  }
}

async function resetProxy() {
  try {
    await window.go.main.App.ResetProxy();
    var el = document.getElementById('cfg-proxy');
    if (el) el.value = '';
    renderProxyDetectCard('hidden');
    showToast(tr('toast.proxyCleared', '代理已清除'));
  } catch(e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
  }
}

// UI 状态（概览页按钮 + 新建任务模态框按钮）
function updateUIStatus(running) {
  // 运行中：禁用所有「开始」入口（概览新建任务 + 模态框开始）；停止入口只保留在概览页
  ['btn-start', 'ntm-start'].forEach(function(id) {
    var b = document.getElementById(id);
    if (b) b.disabled = running;
  });
  ['btn-stop'].forEach(function(id) {
    var b = document.getElementById(id);
    if (b) b.disabled = !running;
  });
}

// 新建任务模态框：打开 / 关闭
function openNewTaskModal() {
  // 重置并加载域名列表、恢复上次选中的邮箱提供商
  if (typeof initEmailProviderSelection === 'function') initEmailProviderSelection();
  // 刷新代理下拉，保证新增代理后选项最新
  if (typeof loadProxyOptions === 'function') loadProxyOptions();
  var m = document.getElementById('new-task-modal');
  if (m) m.classList.add('show');
}
function closeNewTaskModal() {
  var m = document.getElementById('new-task-modal');
  if (m) m.classList.remove('show');
}

// 配置读写
function getFormConfig() {
  const config = {
    count: parseInt(document.getElementById('cfg-count').value) || 1,
    concurrency: parseInt(document.getElementById('cfg-concurrency').value) || 1,
    delay: parseInt(document.getElementById('cfg-delay').value) || 3,
    emailProvider: selectedEmailProvider || 'outlook',
    proxy: ((document.getElementById('cfg-proxy-select') || {}).dataset || {}).value || '',
    proxyConfigured: true
  };

  // 如果选择了 MoeMail，添加域名信息和前缀配置
  if (config.emailProvider === 'moemail') {
    if (!selectedMoeMailDomains || selectedMoeMailDomains.length === 0) {
      throw new Error('请选择至少一个域名或选择随机/全部');
    }

    // 如果选择了随机或全部，传递所有可用域名和配置
    if (selectedMoeMailDomains.includes('__random__') || selectedMoeMailDomains.includes('__all__')) {
      config.moemailDomains = allMoeMailDomains.map(item => item.domain);
      config.moemailConfigs = {};
      allMoeMailDomains.forEach(item => {
        config.moemailConfigs[item.domain] = item.configs;
      });
      // 标记是否为随机模式
      config.moemailRandomMode = selectedMoeMailDomains.includes('__random__');
    } else {
      // 传递选中的域名和对应的配置
      config.moemailDomains = selectedMoeMailDomains;
      config.moemailConfigs = {};
      selectedMoeMailDomains.forEach(domain => {
        const item = allMoeMailDomains.find(d => d.domain === domain);
        if (item) {
          config.moemailConfigs[domain] = item.configs;
        }
      });
      config.moemailRandomMode = false;
    }
  }

  // 如果选择了 Cloud-Mail，添加域名信息和配置
  if (config.emailProvider === 'cloudmail') {
    if (!selectedCloudMailDomains || selectedCloudMailDomains.length === 0) {
      throw new Error('请选择至少一个 Cloud-Mail 域名');
    }

    if (selectedCloudMailDomains.includes('__random__') || selectedCloudMailDomains.includes('__all__')) {
      config.cloudmailDomains = allCloudMailDomains.map(item => item.domain);
      config.cloudmailConfigs = {};
      allCloudMailDomains.forEach(item => {
        config.cloudmailConfigs[item.domain] = item.configs;
      });
      config.cloudmailRandomMode = selectedCloudMailDomains.includes('__random__');
    } else {
      config.cloudmailDomains = selectedCloudMailDomains;
      config.cloudmailConfigs = {};
      selectedCloudMailDomains.forEach(domain => {
        const item = allCloudMailDomains.find(d => d.domain === domain);
        if (item) {
          config.cloudmailConfigs[domain] = item.configs;
        }
      });
      config.cloudmailRandomMode = false;
    }
  }
  if (config.emailProvider === 'mailnest') {
    config.mailNestConfig = {
      apiKey: document.getElementById('mailnest-inline-apikey').value,
      projectCode: document.getElementById('mailnest-inline-project-code').value
    };
  }
  return config;
}

window.appSettings = null;

function settingValue(id, fallback) {
  var el = document.getElementById(id);
  if (!el) return fallback;
  if (el.classList && el.classList.contains('custom-dropdown')) return el.dataset.value !== undefined ? el.dataset.value : fallback;
  return el.value;
}

function settingChecked(id, fallback) {
  var el = document.getElementById(id);
  return el ? el.checked : fallback;
}

function setSettingValue(id, value) {
  var el = document.getElementById(id);
  if (!el || value === undefined || value === null) return;
  if (el.classList && el.classList.contains('custom-dropdown') && typeof setDropdownValue === 'function') setDropdownValue(el, String(value));
  else el.value = value;
}

function setSettingChecked(id, value) {
  var el = document.getElementById(id);
  if (el) el.checked = !!value;
}

function applyThemePreference(theme) {
  var resolved = theme;
  if (theme === 'system') resolved = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  document.documentElement.toggleAttribute('data-theme', resolved === 'dark');
  if (resolved === 'dark') document.documentElement.setAttribute('data-theme', 'dark');
  var light = document.getElementById('theme-icon-light');
  var dark = document.getElementById('theme-icon-dark');
  if (light) light.style.display = resolved === 'dark' ? 'none' : '';
  if (dark) dark.style.display = resolved === 'dark' ? '' : 'none';
}

async function populateDefaultProxySetting(selected) {
  var wrap = document.getElementById('setting-default-proxy');
  if (!wrap) return;
  var options = wrap.querySelector('.dropdown-options');
  if (!options) return;
  var list = [];
  try { list = await window.go.main.App.ListProxyPool(); } catch (e) {}
  options.innerHTML = '<div class="dropdown-option" data-value="" data-i18n="ip.direct">' + tr('ip.direct', '直连') + '</div>';
  (list || []).filter(function(p) { return p.enabled; }).forEach(function(p) {
    var option = document.createElement('div');
    option.className = 'dropdown-option';
    option.setAttribute('data-value', p.url);
    option.textContent = p.name || p.probeIp || p.url;
    options.appendChild(option);
  });
  setDropdownValue(wrap, selected || '');
}

function renderAppSettings(s) {
  window.appSettings = s;
  setSettingValue('setting-default-count', s.defaultCount);
  setSettingValue('setting-default-concurrency', s.defaultConcurrency);
  setSettingValue('setting-default-delay', s.defaultDelay);
  setSettingValue('setting-default-provider', s.defaultEmailProvider);
  setSettingValue('setting-domain-mode', s.defaultDomainMode);
  setSettingValue('setting-email-proxy-mode', s.emailProxyMode);
  setSettingValue('setting-email-proxy', s.emailProxy);
  setSettingValue('setting-otp-timeout', s.otpTimeoutSeconds);
  setSettingValue('setting-retry-profile', s.retryProfile);
  setSettingChecked('setting-stop-on-risk', s.stopOnRisk);
  setSettingChecked('cfg-sound', s.soundEnabled);
  setSettingChecked('setting-desktop-notification', s.desktopNotifications);
  setSettingValue('setting-sound-volume', s.soundVolume);
  setSettingChecked('setting-auto-update', s.autoCheckUpdates);
  setSettingValue('setting-theme', s.theme);
  setSettingValue('setting-language', s.language || 'zh');
  setSettingChecked('setting-persistent-logs', s.persistentLogs);
  setSettingValue('setting-log-retention', s.logRetentionDays);
  setSettingChecked('setting-auto-probe', s.autoProbeProxies);
  setSettingValue('setting-moe-expiry', s.moeMailExpiryMinutes);
  setSettingValue('setting-aws-region', s.awsRegion);
  setSettingValue('setting-request-timeout', s.requestTimeoutSeconds);
  setSettingValue('setting-fingerprint-ttl', s.fingerprintTTLHours);
  setFingerprintCurveValues(s.fingerprintOffsets, s.fingerprintAlgorithm || 'balanced');
  setSettingChecked('setting-telemetry', s.telemetryEnabled);
  setSettingValue('setting-oidc-base', s.oidcBase); setSettingValue('setting-signin-base', s.signinBase);
  setSettingValue('setting-profile-base', s.profileBase); setSettingValue('setting-view-base', s.viewBase);
  setSettingValue('setting-portal-base', s.portalBase); setSettingValue('setting-start-url', s.startURL);
  setSettingValue('setting-kiro-base', s.kiroBase); setSettingValue('setting-kiro-redirect', s.kiroRedirectURI);
  setSettingValue('setting-directory-id', s.directoryID);
  document.getElementById('cfg-count').value = s.defaultCount;
  document.getElementById('cfg-concurrency').value = s.defaultConcurrency;
  document.getElementById('cfg-delay').value = s.defaultDelay;
  populateDefaultProxySetting(s.defaultTaskProxy);
  applyThemePreference(s.theme);
  syncEmailProxyField();
  syncVolumeLabel();
}

async function loadAppSettings() {
  try {
    var settings = await window.go.main.App.GetAppSettings();
    renderAppSettings(settings);
    try { localStorage.removeItem('kiro-config'); localStorage.removeItem('kiro-sound'); localStorage.removeItem('kiro-theme'); } catch (e) {}
  } catch (e) {
    console.error('[设置] 加载失败:', e);
  }
}

function collectAppSettings() {
  var s = Object.assign({}, window.appSettings || {});
  s.defaultCount = parseInt(settingValue('setting-default-count', 1)) || 1;
  s.defaultConcurrency = parseInt(settingValue('setting-default-concurrency', 1)) || 1;
  s.defaultDelay = Math.max(0, parseInt(settingValue('setting-default-delay', 1)) || 0);
  s.defaultEmailProvider = settingValue('setting-default-provider', 'outlook');
  s.defaultTaskProxy = settingValue('setting-default-proxy', '');
  s.defaultDomainMode = settingValue('setting-domain-mode', 'random');
  s.emailProxyMode = settingValue('setting-email-proxy-mode', 'follow-task');
  s.emailProxy = settingValue('setting-email-proxy', '').trim();
  s.otpTimeoutSeconds = parseInt(settingValue('setting-otp-timeout', 120));
  s.retryProfile = settingValue('setting-retry-profile', 'standard');
  s.stopOnRisk = settingChecked('setting-stop-on-risk', true);
  s.soundEnabled = settingChecked('cfg-sound', true);
  s.desktopNotifications = settingChecked('setting-desktop-notification', true);
  s.soundVolume = parseInt(settingValue('setting-sound-volume', 70));
  s.autoCheckUpdates = settingChecked('setting-auto-update', true);
  s.theme = settingValue('setting-theme', 'system');
  s.language = settingValue('setting-language', 'zh');
  s.persistentLogs = settingChecked('setting-persistent-logs', false);
  s.logRetentionDays = parseInt(settingValue('setting-log-retention', 7));
  s.autoProbeProxies = settingChecked('setting-auto-probe', true);
  s.moeMailExpiryMinutes = parseInt(settingValue('setting-moe-expiry', 60));
  s.awsRegion = settingValue('setting-aws-region', 'us-east-1').trim();
  s.requestTimeoutSeconds = parseInt(settingValue('setting-request-timeout', 60));
  s.fingerprintTTLHours = parseInt(settingValue('setting-fingerprint-ttl', 6));
  s.fingerprintAlgorithm = settingValue('setting-fingerprint-algorithm', 'balanced');
  s.fingerprintOffsets = getFingerprintCurveValues();
  s.telemetryEnabled = settingChecked('setting-telemetry', true);
  s.oidcBase = settingValue('setting-oidc-base', '').trim(); s.signinBase = settingValue('setting-signin-base', '').trim();
  s.profileBase = settingValue('setting-profile-base', '').trim(); s.viewBase = settingValue('setting-view-base', '').trim();
  s.portalBase = settingValue('setting-portal-base', '').trim(); s.startURL = settingValue('setting-start-url', '').trim();
  s.kiroBase = settingValue('setting-kiro-base', '').trim(); s.kiroRedirectURI = settingValue('setting-kiro-redirect', '').trim();
  s.directoryID = settingValue('setting-directory-id', '').trim();
  return s;
}

async function saveAppSettings() {
  try {
    var result = await window.go.main.App.SaveAppSettings(collectAppSettings());
    if (result.error) { showToast(result.error, 'error'); return; }
    renderAppSettings(result.settings);
    if (window.I18N) window.I18N.setLanguage(result.settings.language || 'zh');
    showToast(tr('settings.saved', '设置已保存'));
  } catch (e) { showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error'); }
}

function syncEmailProxyField() {
  var field = document.getElementById('setting-email-proxy');
  if (field) field.disabled = settingValue('setting-email-proxy-mode', 'follow-task') !== 'custom';
}

function syncVolumeLabel() {
  var output = document.getElementById('setting-sound-volume-label');
  if (output) output.textContent = settingValue('setting-sound-volume', 70) + '%';
}

function syncAWSRegionEndpoints() {
  var region = settingValue('setting-aws-region', 'us-east-1').trim() || 'us-east-1';
  setSettingValue('setting-oidc-base', 'https://oidc.' + region + '.amazonaws.com');
  setSettingValue('setting-signin-base', 'https://' + region + '.signin.aws');
  setSettingValue('setting-portal-base', 'https://portal.sso.' + region + '.amazonaws.com');
}

var fingerprintCurvePresets = {
  stable: [0, 0, 0, 0, 0],
  balanced: [0, 0, 0, 15, 100],
  fresh: [100, 100, 100, 100, 100]
};
var fingerprintCurveValues = fingerprintCurvePresets.balanced.slice();
var fingerprintCurveDrag = null;
var fingerprintCurveX = [56, 184, 312, 440, 568];
var fingerprintCurveLabelKeys = ['fpBrowser', 'fpHardware', 'fpDisplay', 'fpRendering', 'fpSession'];

function normalizeFingerprintCurve(values, legacyAlgorithm) {
  if (!Array.isArray(values) || values.length !== 5) values = fingerprintCurvePresets[legacyAlgorithm] || fingerprintCurvePresets.balanced;
  return values.map(function(value) { return Math.max(0, Math.min(100, Math.round(Number(value) || 0))); });
}

function getFingerprintCurveValues() {
  return fingerprintCurveValues.slice();
}

function fingerprintCurvePoints(values) {
  return values.map(function(value, index) {
    return { x: fingerprintCurveX[index], y: 224 - value * 2 };
  });
}

function fingerprintCurvePath(points) {
  if (!points.length) return '';
  var path = 'M ' + points[0].x + ' ' + points[0].y;
  for (var i = 0; i < points.length - 1; i++) {
    var p1 = points[i];
    var p2 = points[i + 1];
    var control = (p2.x - p1.x) * 0.42;
    path += ' C ' + (p1.x + control) + ' ' + p1.y + ', ' + (p2.x - control) + ' ' + p2.y + ', ' + p2.x + ' ' + p2.y;
  }
  return path;
}

function matchingFingerprintPreset(values) {
  return Object.keys(fingerprintCurvePresets).find(function(name) {
    return fingerprintCurvePresets[name].every(function(value, index) { return values[index] === value; });
  }) || 'custom';
}

function ensureFingerprintCurveHandles() {
  var handles = document.getElementById('fp-curve-handles');
  if (!handles || handles.childElementCount) return;
  var svgNS = 'http://www.w3.org/2000/svg';
  fingerprintCurveX.forEach(function(_, index) {
    var group = document.createElementNS(svgNS, 'g');
    group.setAttribute('class', 'fp-curve-handle');
    group.setAttribute('data-fp-index', String(index));
    group.setAttribute('tabindex', '0');
    group.setAttribute('role', 'slider');
    group.setAttribute('aria-valuemin', '0');
    group.setAttribute('aria-valuemax', '100');
    group.innerHTML = '<line class="fp-curve-guide" x1="0" y1="0" x2="0" y2="0"></line><circle class="fp-curve-handle-hit" r="16"></circle><circle class="fp-curve-handle-ring" r="6"></circle><circle class="fp-curve-handle-core" r="2"></circle><text class="fp-curve-handle-value" x="0" y="-13"></text>';
    handles.appendChild(group);
  });
}

function renderFingerprintCurve() {
  var line = document.getElementById('fp-curve-line');
  var area = document.getElementById('fp-curve-area');
  if (!line || !area) return;
  ensureFingerprintCurveHandles();
  var points = fingerprintCurvePoints(fingerprintCurveValues);
  var path = fingerprintCurvePath(points);
  line.setAttribute('d', path);
  area.setAttribute('d', path + ' L 568 224 L 56 224 Z');
  document.querySelectorAll('.fp-curve-handle').forEach(function(handle, index) {
    var point = points[index];
    handle.setAttribute('transform', 'translate(' + point.x + ' ' + point.y + ')');
    handle.setAttribute('aria-label', tr('settings.' + fingerprintCurveLabelKeys[index], fingerprintCurveLabelKeys[index]));
    handle.setAttribute('aria-valuenow', String(fingerprintCurveValues[index]));
    handle.setAttribute('aria-valuetext', fingerprintCurveValues[index] + '%');
    handle.querySelector('.fp-curve-guide').setAttribute('y2', String(224 - point.y));
    handle.querySelector('.fp-curve-handle-value').textContent = fingerprintCurveValues[index] + '%';
  });
  var average = Math.round(fingerprintCurveValues.reduce(function(total, value) { return total + value; }, 0) / fingerprintCurveValues.length);
  var meter = document.getElementById('fp-curve-average');
  if (meter) meter.textContent = average + '%';
  var preset = matchingFingerprintPreset(fingerprintCurveValues);
  setSettingValue('setting-fingerprint-algorithm', preset);
  setSettingValue('setting-fingerprint-offsets', fingerprintCurveValues.join(','));
  document.querySelectorAll('[data-fp-preset]').forEach(function(button) {
    button.classList.toggle('selected', button.dataset.fpPreset === preset);
  });
}

function setFingerprintCurveValues(values, legacyAlgorithm) {
  fingerprintCurveValues = normalizeFingerprintCurve(values, legacyAlgorithm);
  initFingerprintCurve();
  renderFingerprintCurve();
}

function applyFingerprintPreset(name) {
  if (!fingerprintCurvePresets[name]) return;
  setFingerprintCurveValues(fingerprintCurvePresets[name], name);
}

function updateFingerprintCurveFromPointer(event) {
  if (!fingerprintCurveDrag || fingerprintCurveDrag.pointerId !== event.pointerId) return;
  var svg = document.getElementById('fp-curve-svg');
  var rect = svg.getBoundingClientRect();
  var svgY = (event.clientY - rect.top) * 260 / rect.height;
  fingerprintCurveValues[fingerprintCurveDrag.index] = Math.round(Math.max(0, Math.min(100, (224 - svgY) / 2)));
  renderFingerprintCurve();
}

function finishFingerprintCurveDrag(event) {
  if (!fingerprintCurveDrag || (event.pointerId !== undefined && fingerprintCurveDrag.pointerId !== event.pointerId)) return;
  var handle = document.querySelector('.fp-curve-handle[data-fp-index="' + fingerprintCurveDrag.index + '"]');
  if (handle) handle.classList.remove('dragging');
  fingerprintCurveDrag = null;
}

function initFingerprintCurve() {
  var svg = document.getElementById('fp-curve-svg');
  if (!svg) return;
  ensureFingerprintCurveHandles();
  if (svg.dataset.ready === 'true') return;
  svg.dataset.ready = 'true';
  svg.addEventListener('pointerdown', function(event) {
    var handle = event.target.closest && event.target.closest('.fp-curve-handle');
    if (!handle) return;
    event.preventDefault();
    fingerprintCurveDrag = { index: Number(handle.dataset.fpIndex), pointerId: event.pointerId };
    handle.classList.add('dragging');
    updateFingerprintCurveFromPointer(event);
  });
  svg.addEventListener('keydown', function(event) {
    var handle = event.target.closest && event.target.closest('.fp-curve-handle');
    if (!handle) return;
    var index = Number(handle.dataset.fpIndex);
    var step = event.shiftKey ? 5 : 1;
    if (event.key === 'ArrowUp' || event.key === 'ArrowRight') fingerprintCurveValues[index] += step;
    else if (event.key === 'ArrowDown' || event.key === 'ArrowLeft') fingerprintCurveValues[index] -= step;
    else if (event.key === 'Home') fingerprintCurveValues[index] = 0;
    else if (event.key === 'End') fingerprintCurveValues[index] = 100;
    else return;
    event.preventDefault();
    fingerprintCurveValues[index] = Math.max(0, Math.min(100, fingerprintCurveValues[index]));
    renderFingerprintCurve();
  });
  window.addEventListener('pointermove', updateFingerprintCurveFromPointer);
  window.addEventListener('pointerup', finishFingerprintCurveDrag);
  window.addEventListener('pointercancel', finishFingerprintCurveDrag);
  renderFingerprintCurve();
}

function requestAdvancedSettings() {
  var content = document.getElementById('advanced-settings-content');
  var trigger = document.getElementById('advanced-settings-trigger');
  if (!content || !trigger) return;
  if (!content.hidden) { content.hidden = true; trigger.setAttribute('aria-expanded', 'false'); return; }
  showConfirmModal(tr('settings.advancedWarningTitle', '打开高级配置？'), tr('settings.advancedWarning', '修改服务端点或底层网络参数可能导致注册失败、账号风控或接口不可用。仅在明确知道参数用途时继续。'), tr('settings.continueOpen', '继续打开'), function() {
    content.hidden = false; trigger.setAttribute('aria-expanded', 'true'); initFingerprintCurve(); requestAnimationFrame(renderFingerprintCurve);
  });
}

async function openLogsDirectory() { try { await window.go.main.App.OpenLogsDir(); } catch (e) {} }
function clearPersistentLogs() { showConfirmModal(tr('settings.clearLogs', '清理日志'), tr('settings.clearLogsConfirm', '确定删除全部持久化日志吗？'), tr('common.confirm', '确认'), async function() { var r = await window.go.main.App.ClearLogs(); showToast(r.error || tr('settings.logsCleared', '日志已清理'), r.error ? 'error' : 'success'); }); }
function clearFingerprintCache() { showConfirmModal(tr('settings.clearFingerprint', '清理指纹缓存'), tr('settings.clearFingerprintConfirm', '确定清理全部指纹缓存吗？'), tr('common.confirm', '确认'), async function() { var r = await window.go.main.App.ResetFingerprintCache(); showToast(r.error || tr('settings.fingerprintCleared', '指纹缓存已清理'), r.error ? 'error' : 'success'); }); }

// 初始化加载
async function loadConfig() {
  console.log('[启动] 开始初始化...');
  
  // 默认禁用所有功能，等待卡密验证
  
  let retries = 0;
  while ((!window.go || !window.go.main || !window.go.main.App) && retries < 100) {
    await new Promise(resolve => setTimeout(resolve, 50));
    retries++;
  }
  if (!window.go || !window.go.main || !window.go.main.App) {
    console.error('[启动] Wails runtime 加载失败');
    // 即使失败也显示界面
    document.getElementById('main-container').style.display = 'block';
    return;
  }
  console.log('[启动] Wails runtime 已就绪');

  // 检测平台，macOS 使用原生窗口控件
  try {
    const env = await window.runtime.Environment();
    if (env && env.platform === 'darwin') {
      document.body.classList.add('platform-darwin');
    }
  } catch(e) {}

  // 直接显示主界面
  console.log('[启动] 显示主界面');
  const mainContainer = document.getElementById('main-container');
  if (mainContainer) {
    mainContainer.style.display = 'block';
    mainContainer.style.height = '100vh';
    mainContainer.style.width = '100vw';
    mainContainer.style.position = 'fixed';
    mainContainer.style.top = '0';
    mainContainer.style.left = '0';
    mainContainer.style.zIndex = '1';
    
    // 隐藏骨架屏
    const skeleton = document.getElementById('skeleton-loader');
    if (skeleton) {
      skeleton.style.display = 'none';
    }
    
    console.log('[启动] main-container 已显示');
  } else {
    console.error('[启动] 找不到 main-container 元素');
  }
  
  await loadAppSettings();
  loadOutlookAccountsList();
  loadDataDir();
  loadResultOutputDir();
  loadProxy();
  if (typeof loadProxyOptions === 'function') loadProxyOptions();
  startOverviewTimer();
  console.log('[启动] 初始化完成');
}

// 页面加载时自动初始化
window.addEventListener('DOMContentLoaded', async function() {
  await loadConfig();
  initEmailProviderSelection();
  // 初始化 i18n（在 Wails runtime 就绪后），失败时不阻塞主流程
  try {
    if (window.I18N && typeof window.I18N.init === 'function') {
      await window.I18N.init();
      if (window.appSettings) window.appSettings.language = window.I18N.getLanguage();
      setSettingValue('setting-language', window.I18N.getLanguage());
      refreshLanguageNavLabel();
      // 重新渲染依赖 i18n 的动态文本
      var tb = document.getElementById('titlebar-text');
      if (tb) tb.textContent = getPageTitle(_currentPageId);
    }
  } catch(e) {}
  // 语言切换时刷新动态文本
  window.addEventListener('i18n:changed', function() {
    var tb = document.getElementById('titlebar-text');
    if (tb) tb.textContent = getPageTitle(_currentPageId);
    refreshLanguageNavLabel();
  });
  // 启动时静默检查更新
  if (!window.appSettings || window.appSettings.autoCheckUpdates !== false) setTimeout(checkUpdateOnStartup, 2000);
});

// 语言循环切换（侧栏点击）：zh → en → ja → zh
var _langOrder = ['zh', 'en', 'ja'];
var _langLabel = { zh: '中', en: 'EN', ja: 'あ' };
var _langFlag = { zh: 'cn', en: 'us', ja: 'jp' };
function cycleLanguage() {
  if (!window.I18N) return;
  var cur = window.I18N.getLanguage();
  var idx = _langOrder.indexOf(cur);
  var next = _langOrder[(idx + 1) % _langOrder.length];
  try {
    window.I18N.setLanguage(next);
    showToast(tr('toast.languageChanged', '已切换语言'));
  } catch(e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
  }
}
function refreshLanguageNavLabel() {
  var el = document.getElementById('nav-language-label');
  if (!el || !window.I18N) return;
  var cur = window.I18N.getLanguage();
  if (el.tagName === 'IMG') {
    el.src = 'https://flagcdn.com/w40/' + (_langFlag[cur] || 'cn') + '.png';
    el.alt = cur;
  } else {
    el.textContent = _langLabel[cur] || cur;
  }
}

async function checkUpdateOnStartup() {
  try {
    var result = await window.go.main.App.CheckUpdate();
    if (result && result.hasUpdate) {
      if (typeof showUpdateModal === 'function') showUpdateModal(result);
    }
  } catch(e) {}
}

function renderChangelog(md) {
  var esc = function(s) {
    return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
  };
  var inline = function(s) {
    return esc(s)
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/`(.+?)`/g, '<code style="background:var(--bg-subtle);padding:1px 5px;border-radius:4px;font-family:var(--font-mono);font-size:12px;">$1</code>');
  };

  var lines = md.split('\n');
  var html = '';
  var inList = false;

  for (var i = 0; i < lines.length; i++) {
    var line = lines[i];
    var h2 = line.match(/^##\s+(.+)/);
    var h3 = line.match(/^###\s+(.+)/);
    var li = line.match(/^[-*]\s+(.+)/);
    var blank = line.trim() === '';

    if (h2) {
      if (inList) { html += '</ul>'; inList = false; }
      html += '<div class="cl-h2">' + inline(h2[1]) + '</div>';
    } else if (h3) {
      if (inList) { html += '</ul>'; inList = false; }
      html += '<div class="cl-h3">' + inline(h3[1]) + '</div>';
    } else if (li) {
      if (!inList) { html += '<ul class="cl-list">'; inList = true; }
      html += '<li>' + inline(li[1]) + '</li>';
    } else if (blank) {
      if (inList) { html += '</ul>'; inList = false; }
    } else {
      if (inList) { html += '</ul>'; inList = false; }
      html += '<p class="cl-p">' + inline(line) + '</p>';
    }
  }
  if (inList) html += '</ul>';
  return html;
}
