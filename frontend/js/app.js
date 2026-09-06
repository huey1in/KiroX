// ===== 核心：导航 / 标签页 / 下拉框 / 配置 / Toast / 窗口控制 =====

// 页面切换
var _currentPageId = 'overview';
var infoChangelogView = { state: 'idle', error: '' };

function renderInfoChangelogState() {
  var el = document.getElementById('info-changelog');
  if (!el || infoChangelogView.state === 'idle' || infoChangelogView.state === 'ready') return;
  var key = infoChangelogView.state === 'loading' ? 'common.loading'
    : (infoChangelogView.state === 'empty' ? 'common.noData' : 'common.loadFailed');
  var fallback = infoChangelogView.state === 'loading' ? '加载中...'
    : (infoChangelogView.state === 'empty' ? '暂无更新说明' : '加载失败');
  var suffix = infoChangelogView.error ? ': ' + infoChangelogView.error : '';
  el.textContent = tr(key, fallback) + suffix;
  el.style.color = 'var(--text-muted)';
}

function setInfoChangelogState(state, error) {
  infoChangelogView = { state: state, error: error || '' };
  renderInfoChangelogState();
}

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
  updateSettingsDirtyState();
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
  setInfoChangelogState('loading');
  try {
    var result = await window.go.main.App.CheckUpdate();
    if (result.error) {
      setInfoChangelogState('error', result.error);
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
      if (body) {
        infoChangelogView = { state: 'ready', error: '' };
        changelogEl.style.color = '';
        changelogEl.innerHTML = renderChangelog(body);
      } else {
        setInfoChangelogState('empty');
      }
    }
  } catch(e) {
    setInfoChangelogState('error');
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

var proxyDetectView = { state: 'hidden', payload: null };

function renderProxyDetectCard(state, payload) {
  proxyDetectView = { state: state, payload: payload || null };
  var box = document.getElementById('proxy-detect-card');
  if (!box) return;
  if (state === 'hidden') { box.style.display = 'none'; box.innerHTML = ''; return; }
  box.style.display = 'block';
  var base = 'border:1px solid var(--border);border-radius:8px;padding:10px 12px;font-size:12px;';
  if (state === 'loading') {
    box.style.cssText = base + 'background:var(--card-bg, transparent);color:var(--muted);';
    box.textContent = tr('settings.proxyDetecting', '正在检测代理出口…');
    return;
  }
  if (state === 'ok') {
    var loc = [payload.country, payload.region, payload.city].filter(Boolean).join(' · ');
    box.style.cssText = base + 'background:rgba(16,185,129,0.08);border-color:rgba(16,185,129,0.35);';
    box.innerHTML =
      '<div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">' +
        '<span style="font-weight:600;color:#10b981;">✓ ' + tr('settings.proxyAvailable', '可用') + '</span>' +
        '<span style="padding:1px 6px;border-radius:4px;background:rgba(16,185,129,0.15);color:#10b981;font-size:11px;font-weight:600;">' + (payload.scheme || '').toUpperCase() + '</span>' +
        '<span style="color:var(--text);font-weight:600;">' + (payload.ip || '') + '</span>' +
        (loc ? '<span style="color:var(--muted);">· ' + loc + '</span>' : '') +
      '</div>' +
      (payload.isp ? '<div style="margin-top:4px;color:var(--muted);font-size:11px;">' + payload.isp + '</div>' : '');
    return;
  }
  // error
  box.style.cssText = base + 'background:rgba(239,68,68,0.08);border-color:rgba(239,68,68,0.35);color:#ef4444;';
  box.textContent = '✗ ' + tr('settings.proxyDetectFailed', '检测失败') + ': ' + (payload && payload.error ? payload.error : tr('ip.unknownError', '未知错误'));
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
var savedSettingsSnapshot = null;
var settingsSaving = false;

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

function snapshotAppSettings(settings) {
  return JSON.stringify(settings || {});
}

function settingsHaveChanges() {
  if (savedSettingsSnapshot === null || !window.appSettings) return false;
  return snapshotAppSettings(collectAppSettings()) !== savedSettingsSnapshot;
}

function updateFloatingSaveButton(hasChanges) {
  var button = document.getElementById('settings-floating-save');
  var headerButton = document.getElementById('settings-save-button');
  var scrollRoot = document.querySelector('.app-content');
  if (!button || !headerButton || !scrollRoot) return;
  var page = document.getElementById('page-settings');
  var rootRect = scrollRoot.getBoundingClientRect();
  var headerRect = headerButton.getBoundingClientRect();
  var headerHasScrolledAway = headerRect.bottom < rootRect.top + 12;
  button.classList.toggle('is-visible', !!hasChanges && !!page && page.classList.contains('active') && headerHasScrolledAway);
}

function updateSettingsDirtyState() {
  var hasChanges = settingsHaveChanges();
  var headerButton = document.getElementById('settings-save-button');
  var floatingButton = document.getElementById('settings-floating-save');
  if (headerButton) headerButton.disabled = !hasChanges || settingsSaving;
  if (floatingButton) floatingButton.disabled = !hasChanges || settingsSaving;
  updateFloatingSaveButton(hasChanges);
}

function initSettingsChangeTracking() {
  var page = document.getElementById('page-settings');
  var scrollRoot = document.querySelector('.app-content');
  if (!page || page.dataset.changeTracking === 'true') return;
  page.dataset.changeTracking = 'true';
  page.addEventListener('input', updateSettingsDirtyState);
  page.addEventListener('change', updateSettingsDirtyState);
  if (scrollRoot) scrollRoot.addEventListener('scroll', updateSettingsDirtyState, { passive: true });
  window.addEventListener('resize', updateSettingsDirtyState);
  updateSettingsDirtyState();
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

function renderAppSettings(s) {
  window.appSettings = s;
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
  setFingerprintCurveValues(s.fingerprintOffsets, s.fingerprintCurvePositions);
  setSettingChecked('setting-telemetry', s.telemetryEnabled);
  setSettingChecked('setting-waf-enabled', s.wafEnabled);
  setSettingValue('setting-two-captcha-api-key', s.twoCaptchaAPIKey);
  setSettingValue('setting-waf-website-url', s.wafWebsiteURL);
  setSettingValue('setting-waf-website-key', s.wafWebsiteKey);
  setSettingValue('setting-waf-iv', s.wafIV);
  setSettingValue('setting-waf-context', s.wafContext);
  setSettingValue('setting-waf-jsapi-script', s.wafJSAPIScript);
  setSettingValue('setting-waf-challenge-script', s.wafChallengeScript);
  setSettingValue('setting-waf-captcha-script', s.wafCaptchaScript);
  setSettingValue('setting-oidc-base', s.oidcBase); setSettingValue('setting-signin-base', s.signinBase);
  setSettingValue('setting-profile-base', s.profileBase); setSettingValue('setting-view-base', s.viewBase);
  setSettingValue('setting-portal-base', s.portalBase); setSettingValue('setting-start-url', s.startURL);
  setSettingValue('setting-kiro-base', s.kiroBase); setSettingValue('setting-kiro-redirect', s.kiroRedirectURI);
  setSettingValue('setting-directory-id', s.directoryID);
  applyThemePreference(s.theme);
  syncEmailProxyField();
  syncWAFSettings();
  syncVolumeLabel();
  savedSettingsSnapshot = snapshotAppSettings(collectAppSettings());
  updateSettingsDirtyState();
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
  s.fingerprintOffsets = getFingerprintCurveValues();
  s.fingerprintCurvePositions = getFingerprintCurvePositions();
  s.telemetryEnabled = settingChecked('setting-telemetry', true);
  s.wafEnabled = settingChecked('setting-waf-enabled', false);
  s.twoCaptchaAPIKey = settingValue('setting-two-captcha-api-key', '').trim();
  s.wafWebsiteURL = settingValue('setting-waf-website-url', '').trim();
  s.wafWebsiteKey = settingValue('setting-waf-website-key', '').trim();
  s.wafIV = settingValue('setting-waf-iv', '').trim();
  s.wafContext = settingValue('setting-waf-context', '').trim();
  s.wafJSAPIScript = settingValue('setting-waf-jsapi-script', '').trim();
  s.wafChallengeScript = settingValue('setting-waf-challenge-script', '').trim();
  s.wafCaptchaScript = settingValue('setting-waf-captcha-script', '').trim();
  s.oidcBase = settingValue('setting-oidc-base', '').trim(); s.signinBase = settingValue('setting-signin-base', '').trim();
  s.profileBase = settingValue('setting-profile-base', '').trim(); s.viewBase = settingValue('setting-view-base', '').trim();
  s.portalBase = settingValue('setting-portal-base', '').trim(); s.startURL = settingValue('setting-start-url', '').trim();
  s.kiroBase = settingValue('setting-kiro-base', '').trim(); s.kiroRedirectURI = settingValue('setting-kiro-redirect', '').trim();
  s.directoryID = settingValue('setting-directory-id', '').trim();
  return s;
}

async function saveAppSettings() {
  if (settingsSaving || !settingsHaveChanges()) return;
  settingsSaving = true;
  updateSettingsDirtyState();
  try {
    var result = await window.go.main.App.SaveAppSettings(collectAppSettings());
    if (result.error) { showToast(result.error, 'error'); return; }
    renderAppSettings(result.settings);
    if (window.I18N) window.I18N.setLanguage(result.settings.language || 'zh');
    showToast(tr('settings.saved', '设置已保存'));
  } catch (e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
  } finally {
    settingsSaving = false;
    updateSettingsDirtyState();
  }
}

function syncEmailProxyField() {
  var field = document.getElementById('setting-email-proxy');
  if (field) field.disabled = settingValue('setting-email-proxy-mode', 'follow-task') !== 'custom';
}

function syncWAFSettings() {
  var fields = document.querySelectorAll('#setting-waf-fields input');
  var enabled = settingChecked('setting-waf-enabled', false);
  fields.forEach(function(field) { field.disabled = !enabled; });
}

function syncVolumeLabel() {
  var slider = document.getElementById('setting-sound-volume');
  var output = document.getElementById('setting-sound-volume-label');
  var value = Number(settingValue('setting-sound-volume', 70));
  if (slider) slider.style.setProperty('--range-progress', value + '%');
  if (output) output.textContent = value + '%';
}

function syncAWSRegionEndpoints() {
  var region = settingValue('setting-aws-region', 'us-east-1').trim() || 'us-east-1';
  setSettingValue('setting-oidc-base', 'https://oidc.' + region + '.amazonaws.com');
  setSettingValue('setting-signin-base', 'https://' + region + '.signin.aws');
  setSettingValue('setting-portal-base', 'https://portal.sso.' + region + '.amazonaws.com');
}

var fingerprintCurveDefaults = [0, 0, 0, 0, 0, 0, 0, 15, 15, 100];
var fingerprintCurveDefaultPositions = [0, 11, 22, 33, 44, 56, 67, 78, 89, 100];
var fingerprintCurveValues = fingerprintCurveDefaults.slice();
var fingerprintCurvePositions = fingerprintCurveDefaultPositions.slice();

function normalizeFingerprintCurve(values) {
  if (Array.isArray(values) && values.length === 5) {
    values = [values[0], values[1], values[0], values[1], values[1], values[2], values[2], values[3], values[3], values[4]];
  }
  if (!Array.isArray(values) || values.length !== fingerprintCurveDefaults.length) values = fingerprintCurveDefaults;
  return values.map(function(value) { return Math.max(0, Math.min(100, Math.round(Number(value) || 0))); });
}

function normalizeFingerprintCurvePositions(positions) {
  if (!Array.isArray(positions) || positions.length !== fingerprintCurveDefaultPositions.length) return fingerprintCurveDefaultPositions.slice();
  var normalized = [];
  positions.forEach(function(position, index) {
    var min = index === 0 ? 0 : normalized[index - 1] + 2;
    var max = 100 - (positions.length - 1 - index) * 2;
    normalized.push(Math.max(min, Math.min(max, Math.round(Number(position) || 0))));
  });
  return normalized;
}

function getFingerprintCurveValues() {
  return fingerprintCurveValues.slice();
}

function getFingerprintCurvePositions() {
  return fingerprintCurvePositions.slice();
}

function fingerprintCurvePoints(values) {
  return values.map(function(value, index) {
    return { x: 56 + fingerprintCurvePositions[index] * 9, y: 224 - value * 2 };
  });
}

function fingerprintEffectiveValues() {
  return fingerprintCurveDefaultPositions.map(function(position) {
    if (position <= fingerprintCurvePositions[0]) return fingerprintCurveValues[0];
    for (var i = 1; i < fingerprintCurvePositions.length; i++) {
      if (position <= fingerprintCurvePositions[i]) {
        var leftX = fingerprintCurvePositions[i - 1];
        var ratio = (position - leftX) / (fingerprintCurvePositions[i] - leftX);
        return Math.round(fingerprintCurveValues[i - 1] + (fingerprintCurveValues[i] - fingerprintCurveValues[i - 1]) * ratio);
      }
    }
    return fingerprintCurveValues[fingerprintCurveValues.length - 1];
  });
}

function fingerprintCurvePath(points) {
  if (!window.d3 || !points.length) return '';
  return d3.line().x(function(point) { return point.x; }).y(function(point) { return point.y; }).curve(d3.curveMonotoneX)(points);
}

function fingerprintCurveAreaPath(points) {
  if (!window.d3 || !points.length) return '';
  return d3.area().x(function(point) { return point.x; }).y0(224).y1(function(point) { return point.y; }).curve(d3.curveMonotoneX)(points);
}

function ensureFingerprintCurveHandles() {
  var handles = document.getElementById('fp-curve-handles');
  if (!handles || !window.d3) return;
  var entered = d3.select(handles).selectAll('.fp-curve-handle').data(fingerprintCurveDefaultPositions).enter().append('g')
    .attr('class', 'fp-curve-handle')
    .attr('data-fp-index', function(_, index) { return index; })
    .attr('tabindex', 0)
    .attr('role', 'slider')
    .attr('aria-valuemin', 0)
    .attr('aria-valuemax', 100);
  entered.append('line').attr('class', 'fp-curve-guide').attr('x1', 0).attr('y1', 0).attr('x2', 0).attr('y2', 0);
  entered.append('circle').attr('class', 'fp-curve-handle-hit').attr('r', 16);
  entered.append('circle').attr('class', 'fp-curve-handle-ring').attr('r', 6);
  entered.append('circle').attr('class', 'fp-curve-handle-core').attr('r', 2);
  entered.append('text').attr('class', 'fp-curve-handle-value').attr('x', 0).attr('y', -13);
}

function renderFingerprintCurve() {
  var line = document.getElementById('fp-curve-line');
  var area = document.getElementById('fp-curve-area');
  if (!line || !area) return;
  ensureFingerprintCurveHandles();
  var points = fingerprintCurvePoints(fingerprintCurveValues);
  d3.select(line).attr('d', fingerprintCurvePath(points));
  d3.select(area).attr('d', fingerprintCurveAreaPath(points));
  d3.selectAll('.fp-curve-handle').each(function(_, index) {
    var handle = this;
    var point = points[index];
    handle.setAttribute('transform', 'translate(' + point.x + ' ' + point.y + ')');
    handle.setAttribute('aria-label', tr('settings.fpControlPoint', '曲线控制点') + ' ' + (index + 1));
    handle.setAttribute('aria-valuenow', String(fingerprintCurveValues[index]));
    handle.setAttribute('aria-valuetext', 'X ' + fingerprintCurvePositions[index] + '%, Y ' + fingerprintCurveValues[index] + '%');
    handle.querySelector('.fp-curve-guide').setAttribute('y2', String(224 - point.y));
    handle.querySelector('.fp-curve-handle-value').textContent = fingerprintCurveValues[index] + '%';
  });
  var effectiveValues = fingerprintEffectiveValues();
  var average = Math.round(effectiveValues.reduce(function(total, value) { return total + value; }, 0) / effectiveValues.length);
  var meter = document.getElementById('fp-curve-average');
  if (meter) meter.textContent = average + '%';
  setSettingValue('setting-fingerprint-offsets', fingerprintCurveValues.join(','));
  updateSettingsDirtyState();
}

function setFingerprintCurveValues(values, positions) {
  fingerprintCurveValues = normalizeFingerprintCurve(values);
  fingerprintCurvePositions = normalizeFingerprintCurvePositions(positions);
  initFingerprintCurve();
  renderFingerprintCurve();
}

function initFingerprintCurve() {
  var svg = document.getElementById('fp-curve-svg');
  if (!svg || !window.d3) return;
  ensureFingerprintCurveHandles();
  if (svg.dataset.ready === 'true') return;
  svg.dataset.ready = 'true';
  d3.select(svg).selectAll('.fp-curve-handle')
    .call(d3.drag().container(svg)
      .on('start', function() { this.classList.add('dragging'); })
      .on('drag', function(event) {
        var index = Number(this.dataset.fpIndex);
        var minX = index === 0 ? 0 : fingerprintCurvePositions[index - 1] + 2;
        var maxX = index === fingerprintCurvePositions.length - 1 ? 100 : fingerprintCurvePositions[index + 1] - 2;
        fingerprintCurvePositions[index] = Math.round(Math.max(minX, Math.min(maxX, (event.x - 56) / 9)));
        fingerprintCurveValues[index] = Math.round(Math.max(0, Math.min(100, (224 - event.y) / 2)));
        renderFingerprintCurve();
      })
      .on('end', function() { this.classList.remove('dragging'); }))
    .on('keydown', function(event) {
    var index = Number(this.dataset.fpIndex);
    var step = event.shiftKey ? 5 : 1;
    if (event.key === 'ArrowUp') fingerprintCurveValues[index] += step;
    else if (event.key === 'ArrowDown') fingerprintCurveValues[index] -= step;
    else if (event.key === 'ArrowRight') {
      var rightMax = index === fingerprintCurvePositions.length - 1 ? 100 : fingerprintCurvePositions[index + 1] - 2;
      fingerprintCurvePositions[index] = Math.min(rightMax, fingerprintCurvePositions[index] + step);
    } else if (event.key === 'ArrowLeft') {
      var leftMin = index === 0 ? 0 : fingerprintCurvePositions[index - 1] + 2;
      fingerprintCurvePositions[index] = Math.max(leftMin, fingerprintCurvePositions[index] - step);
    }
    else return;
    event.preventDefault();
    fingerprintCurveValues[index] = Math.max(0, Math.min(100, fingerprintCurveValues[index]));
    renderFingerprintCurve();
  });
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
    var failedSkeleton = document.getElementById('skeleton-loader');
    if (failedSkeleton) failedSkeleton.style.display = 'none';
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
  initSettingsChangeTracking();
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
    var activeLanguage = window.I18N.getLanguage();
    setSettingValue('setting-language', activeLanguage);
    if (window.appSettings) window.appSettings.language = activeLanguage;
    if (savedSettingsSnapshot !== null) {
      var savedLanguageState = JSON.parse(savedSettingsSnapshot);
      savedLanguageState.language = activeLanguage;
      savedSettingsSnapshot = snapshotAppSettings(savedLanguageState);
    }
    renderInfoChangelogState();
    renderFingerprintCurve();
    renderProxyDetectCard(proxyDetectView.state, proxyDetectView.payload);
    updateSettingsDirtyState();
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
