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
  document.querySelectorAll('.page').forEach(function(p) {
    p.classList.remove('active');
  });
  var target = document.getElementById('page-' + pageId);
  if (target) target.classList.add('active');
  document.querySelectorAll('.nav-item[data-page]').forEach(function(item) {
    item.classList.toggle('active', item.getAttribute('data-page') === pageId);
  });
  document.getElementById('titlebar-text').textContent = getPageTitle(pageId);
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
    var ver = await window.go.main.App.GetCurrentVersion();
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
var settingsWriteQueue = Promise.resolve();
var pendingSettingsWrites = 0;
var appSettingControls = {
  emailProxyMode: 'setting-email-proxy-mode', emailProxy: 'setting-email-proxy',
  otpTimeoutSeconds: 'setting-otp-timeout', retryProfile: 'setting-retry-profile',
  stopOnRisk: 'setting-stop-on-risk', soundEnabled: 'cfg-sound',
  desktopNotifications: 'setting-desktop-notification', soundVolume: 'setting-sound-volume',
  autoCheckUpdates: 'setting-auto-update', theme: 'setting-theme', language: 'setting-language',
  persistentLogs: 'setting-persistent-logs', logRetentionDays: 'setting-log-retention',
  autoProbeProxies: 'setting-auto-probe', moeMailExpiryMinutes: 'setting-moe-expiry'
};

function renderAppSetting(key, value) {
  if (typeof value === 'boolean') setSettingChecked(appSettingControls[key], value);
  else setSettingValue(appSettingControls[key], value);
}

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
  var ordered = {};
  Object.keys(settings || {}).sort().forEach(function(key) { ordered[key] = settings[key]; });
  return JSON.stringify(ordered);
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
  Object.keys(appSettingControls).forEach(function(key) { renderAppSetting(key, s[key]); });
  applyThemePreference(s.theme);
  syncEmailProxyField();
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
  var s = {};
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
  return s;
}

function commitAppSettings(settings, submitted, fields) {
  var current = collectAppSettings();
  (fields || Object.keys(appSettingControls)).forEach(function(key) {
    if (current[key] !== submitted[key]) return;
    renderAppSetting(key, settings[key]);
    if (key === 'theme') applyThemePreference(settings.theme);
    if (key === 'language' && settings.language && window.I18N) window.I18N.setLanguage(settings.language);
  });
  window.appSettings = settings;
  savedSettingsSnapshot = snapshotAppSettings(settings);
  syncEmailProxyField();
  syncVolumeLabel();
  updateSettingsDirtyState();
}

function queueAppSettingsWrite(submitted, fields) {
  pendingSettingsWrites++;
  settingsSaving = true;
  updateSettingsDirtyState();
  var request = settingsWriteQueue.then(async function() {
    var settings = submitted;
    if (fields) {
      settings = Object.assign({}, window.appSettings);
      fields.forEach(function(key) { settings[key] = submitted[key]; });
    }
    var result = await window.go.main.App.SaveAppSettings(settings);
    if (result.error) throw new Error(result.error);
    commitAppSettings(result.settings, submitted, fields);
    return result;
  }).finally(function() {
    pendingSettingsWrites--;
    settingsSaving = pendingSettingsWrites > 0;
    updateSettingsDirtyState();
  });
  settingsWriteQueue = request.catch(function() {});
  return request;
}

function persistAppSetting(key, value) {
  var submitted = collectAppSettings();
  submitted[key] = value;
  return queueAppSettingsWrite(submitted, [key]);
}

async function saveAppSettings() {
  if (settingsSaving || !settingsHaveChanges()) return;
  try {
    await queueAppSettingsWrite(collectAppSettings());
    showToast(tr('settings.saved', '设置已保存'));
  } catch (e) {
    showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
  }
}

function syncEmailProxyField() {
  var field = document.getElementById('setting-email-proxy');
  if (field) field.disabled = settingValue('setting-email-proxy-mode', 'follow-task') !== 'custom';
}

function syncVolumeLabel() {
  var slider = document.getElementById('setting-sound-volume');
  var output = document.getElementById('setting-sound-volume-label');
  var value = Number(settingValue('setting-sound-volume', 70));
  if (slider) slider.style.setProperty('--range-progress', value + '%');
  if (output) output.textContent = value + '%';
}

async function openLogsDirectory() { try { await window.go.main.App.OpenLogsDir(); } catch (e) {} }
function clearPersistentLogs() { showConfirmModal(tr('settings.clearLogs', '清理日志'), tr('settings.clearLogsConfirm', '确定删除全部持久化日志吗？'), tr('common.confirm', '确认'), async function() { var r = await window.go.main.App.ClearLogs(); showToast(r.error || tr('settings.logsCleared', '日志已清理'), r.error ? 'error' : 'success'); }); }
function clearFingerprintCache() { showConfirmModal(tr('settings.clearFingerprint', '清理指纹缓存'), tr('settings.clearFingerprintConfirm', '确定清理全部指纹缓存吗？'), tr('common.confirm', '确认'), async function() { var r = await window.go.main.App.ResetFingerprintCache(); showToast(r.error || tr('settings.fingerprintCleared', '指纹缓存已清理'), r.error ? 'error' : 'success'); }); }

// 初始化加载
async function loadConfig() {
  console.log('[启动] 开始初始化...');


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
  if (typeof loadProxyOptions === 'function') loadProxyOptions();
  console.log('[启动] 初始化完成');
}

// 页面加载时自动初始化
window.addEventListener('DOMContentLoaded', async function() {
  await loadConfig();
  initSettingsChangeTracking();
  initEmailProviderSelection();
  var languageBeforeInit = settingValue('setting-language', '');
  var initializingLanguage = true;
  // 初始化和后续语言切换均同步控件，持久化由设置队列处理。
  window.addEventListener('i18n:changed', function() {
    var tb = document.getElementById('titlebar-text');
    if (tb) tb.textContent = getPageTitle(_currentPageId);
    refreshLanguageNavLabel();
    var activeLanguage = window.I18N.getLanguage();
    if (!initializingLanguage || settingValue('setting-language', '') === languageBeforeInit) {
      setSettingValue('setting-language', activeLanguage);
    }
    renderInfoChangelogState();
    updateSettingsDirtyState();
  });
  // 初始化 i18n（在 Wails runtime 就绪后），失败时不阻塞主流程
  try {
    if (window.I18N && typeof window.I18N.init === 'function') {
      var selectedDefaultLanguage = await window.I18N.init();
      var initialLanguage = window.I18N.getLanguage();
      initializingLanguage = false;
      refreshLanguageNavLabel();
      // 重新渲染依赖 i18n 的动态文本
      var tb = document.getElementById('titlebar-text');
      if (tb) tb.textContent = getPageTitle(_currentPageId);
      if (selectedDefaultLanguage && window.appSettings && !window.appSettings.language) {
        await persistAppSetting('language', initialLanguage);
      }
    }
  } catch(e) {} finally { initializingLanguage = false; }
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
    setSettingValue('setting-language', next);
    window.I18N.setLanguage(next);
    if (window.appSettings) persistAppSetting('language', next).catch(function(e) {
      showToast(tr('toast.operationFailed', '操作失败') + ': ' + e.message, 'error');
    });
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
