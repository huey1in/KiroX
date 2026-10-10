const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const initialSettings = {
  emailProxyMode: 'follow-task', emailProxy: '', otpTimeoutSeconds: 120,
  retryProfile: 'standard', stopOnRisk: true, soundEnabled: true,
  desktopNotifications: true, soundVolume: 70, autoCheckUpdates: true,
  theme: 'light', language: 'zh', persistentLogs: false, logRetentionDays: 7,
  autoProbeProxies: true, moeMailExpiryMinutes: 60
};

const dropdownValues = {
  'setting-email-proxy-mode': ['direct', 'follow-task', 'custom'],
  'setting-retry-profile': ['fast', 'standard', 'stable'],
  'setting-theme': ['system', 'light', 'dark'],
  'setting-language': ['zh', 'en', 'ja']
};

function makeElement(classes = []) {
  const classNames = new Set(classes);
  const attributes = {};
  return {
    value: '', checked: false, dataset: {}, textContent: '', style: { setProperty() {} },
    classList: {
      contains: name => classNames.has(name),
      toggle(name, enabled) { if (enabled) classNames.add(name); else classNames.delete(name); }
    },
    getAttribute: name => attributes[name] ?? null,
    setAttribute(name, value) { attributes[name] = String(value); },
    removeAttribute(name) { delete attributes[name]; },
    hasAttribute: name => Object.hasOwn(attributes, name),
    toggleAttribute(name, enabled) { if (enabled) attributes[name] = ''; else delete attributes[name]; },
    getBoundingClientRect: () => ({ top: 0, bottom: 30 }),
    addEventListener() {}, querySelector: () => null, querySelectorAll: () => [], remove() {}
  };
}

async function flushRequests() {
  await new Promise(resolve => setImmediate(resolve));
}

async function createApp(options = {}) {
  const elements = new Map();
  const listeners = new Map();
  const requests = [];
  let backend = { ...initialSettings, ...options.settings };
  const savedMailNest = { apiKey: 'saved-key', projectCode: 'saved-project' };

  function element(id) {
    if (!elements.has(id)) {
      const values = dropdownValues[id];
      const control = makeElement(values ? ['custom-dropdown'] : []);
      if (values) {
        const label = makeElement();
        const options = values.map(value => {
          const option = makeElement(['dropdown-option']);
          option.setAttribute('data-value', value);
          option.textContent = value;
          return option;
        });
        control.querySelector = selector => selector === '.dropdown-selected-text' ? label : null;
        control.querySelectorAll = selector => selector === '.dropdown-option' ? options : [];
      }
      elements.set(id, control);
    }
    return elements.get(id);
  }

  const context = {
    console, setTimeout() {}, setInterval() {},
    CustomEvent: function(type, options) { this.type = type; this.detail = options.detail; },
    localStorage: { getItem: () => null, setItem() {} },
    document: {
      documentElement: makeElement(), head: { appendChild() {} },
      getElementById: element, querySelector: () => element('scroll-root'),
      querySelectorAll: () => [], createElement: () => makeElement(), addEventListener() {}
    },
    addEventListener(type, callback) {
      if (!listeners.has(type)) listeners.set(type, []);
      listeners.get(type).push(callback);
    },
    dispatchEvent(event) {
      for (const callback of listeners.get(event.type) || []) callback(event);
    },
    go: { main: { App: {
      GetLanguage: async () => backend.language,
      GetOSLanguage: async () => options.osLanguage || 'en',
      GetMailNestConfig: async () => ({ ...savedMailNest }),
      SaveAppSettings(settings) {
        return new Promise(resolve => requests.push({
          payload: JSON.parse(JSON.stringify(settings)), resolve, finished: false
        }));
      }
    } } }
  };
  context.window = context;
  vm.createContext(context);
  for (const file of ['i18n.js', 'dropdown.js', 'app.js', 'ui.js', 'mailnest.js']) {
    vm.runInContext(fs.readFileSync(path.join(__dirname, '..', 'js', file), 'utf8'), context, { filename: file });
  }
  context.showToast = () => {};
  // Skip unrelated startup I/O while retaining the actual language event handlers.
  context.loadConfig = async () => {};
  context.initEmailProviderSelection = () => {};
  context.initSettingsChangeTracking = () => {};
  context.renderAppSettings({ ...backend });
  const startup = listeners.get('DOMContentLoaded')[0]();
  if (!options.deferStartup) await startup;
  await context.loadMailNestConfig();

  return {
    context, element, requests, startup,
    backend: () => backend,
    async finish(index, error) {
      const request = requests[index];
      assert.ok(request && !request.finished, 'a pending backend request must exist');
      request.finished = true;
      if (!error) backend = { ...request.payload };
      request.resolve(error ? { error } : { settings: { ...backend } });
      await flushRequests();
    }
  };
}

test('a theme shortcut waits for a full save and retains its saved fields', async () => {
  const app = await createApp();
  app.element('setting-sound-volume').value = 25;
  const save = app.context.saveAppSettings();
  app.context.toggleTheme();
  await flushRequests();
  assert.equal(app.requests.length, 1);
  await app.finish(0);
  await save;
  assert.equal(app.requests.length, 2);
  assert.equal(app.requests[1].payload.soundVolume, 25);
  await app.finish(1);
  assert.equal(app.backend().soundVolume, 25);
  assert.equal(app.backend().theme, 'dark');
  assert.equal(app.element('setting-theme').dataset.value, 'dark');
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('edits made while a save is pending remain dirty and can be saved later', async () => {
  const app = await createApp();
  app.element('setting-sound-volume').value = 25;
  const save = app.context.saveAppSettings();
  await flushRequests();
  app.element('setting-sound-volume').value = 40;
  await app.finish(0);
  await save;
  assert.equal(app.context.collectAppSettings().soundVolume, 40);
  assert.equal(app.context.settingsHaveChanges(), true);
  const retry = app.context.saveAppSettings();
  await flushRequests();
  await app.finish(1);
  await retry;
  assert.equal(app.backend().soundVolume, 40);
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('rapid theme changes allow only one pending request and retain the latest choice', async () => {
  const app = await createApp();
  app.context.toggleTheme();
  app.context.toggleTheme();
  app.context.toggleTheme();
  await flushRequests();
  for (let index = 0; index < 3; index++) {
    assert.equal(app.requests.length, index + 1);
    assert.equal(app.requests.filter(request => !request.finished).length, 1);
    await app.finish(index);
  }
  assert.deepEqual(app.requests.map(request => request.payload.theme), ['dark', 'light', 'dark']);
  assert.equal(app.context.collectAppSettings().theme, 'dark');
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('a failed theme save remains dirty and a manual retry succeeds', async () => {
  const app = await createApp();
  app.context.toggleTheme();
  await flushRequests();
  await app.finish(0, 'save failed');
  assert.equal(app.context.settingsHaveChanges(), true);
  assert.equal(app.context.settingsSaving, false);
  const retry = app.context.saveAppSettings();
  await flushRequests();
  await app.finish(1);
  await retry;
  assert.equal(app.backend().theme, 'dark');
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('a failed queued write does not prevent a later preference write', async () => {
  const app = await createApp();
  app.context.toggleTheme();
  app.context.toggleTheme();
  await flushRequests();
  await app.finish(0, 'save failed');
  assert.equal(app.requests.length, 2);
  await app.finish(1);
  assert.equal(app.backend().theme, 'light');
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('language shortcuts share the queue and survive an older full-save response', async () => {
  const app = await createApp();
  app.element('setting-sound-volume').value = 25;
  const save = app.context.saveAppSettings();
  app.context.cycleLanguage();
  await flushRequests();
  assert.equal(app.context.I18N.getLanguage(), 'en');
  assert.equal(app.requests.length, 1);
  await app.finish(0);
  await save;
  assert.equal(app.context.I18N.getLanguage(), 'en');
  assert.equal(app.requests[1].payload.language, 'en');
  assert.equal(app.requests[1].payload.soundVolume, 25);
  await app.finish(1);
  assert.equal(app.backend().language, 'en');
  assert.equal(app.requests.length, 2);
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('language changes preserve unsaved MailNest credentials', async () => {
  const app = await createApp();
  app.element('mailnest-inline-apikey').value = 'draft-key';
  app.element('mailnest-inline-project-code').value = 'draft-project';
  app.context.I18N.setLanguage('en');
  assert.equal(app.element('mailnest-inline-apikey').value, 'draft-key');
  assert.equal(app.element('mailnest-inline-project-code').value, 'draft-project');
  assert.equal(app.element('settings-mailnest-summary').textContent, app.context.tr('mailnest.summaryActive'));
});

test('first launch persists the detected language before marking settings clean', async () => {
  const app = await createApp({ settings: { language: '' }, osLanguage: 'en', deferStartup: true });
  await flushRequests();
  assert.equal(app.requests.length, 1);
  assert.equal(app.requests[0].payload.language, 'en');
  assert.equal(app.backend().language, '');
  assert.equal(app.context.settingsHaveChanges(), true);
  await app.finish(0);
  await app.startup;
  assert.equal(app.backend().language, 'en');
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('failed first-launch language persistence remains dirty and can be retried', async () => {
  const app = await createApp({ settings: { language: '' }, deferStartup: true });
  await flushRequests();
  await app.finish(0, 'save failed');
  await app.startup;
  assert.equal(app.backend().language, '');
  assert.equal(app.context.settingsHaveChanges(), true);
  const retry = app.context.saveAppSettings();
  await flushRequests();
  await app.finish(1);
  await retry;
  assert.equal(app.backend().language, 'en');
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('first-launch language persistence preserves unrelated edits made while waiting', async () => {
  const app = await createApp({ settings: { language: '' }, deferStartup: true });
  await flushRequests();
  app.element('setting-sound-volume').value = 40;
  await app.finish(0);
  await app.startup;
  assert.equal(app.backend().soundVolume, 70);
  assert.equal(app.context.collectAppSettings().soundVolume, 40);
  assert.equal(app.context.settingsHaveChanges(), true);
});

test('a newer language choice survives a pending first-launch language save', async () => {
  const app = await createApp({ settings: { language: '' }, deferStartup: true });
  await flushRequests();
  app.context.cycleLanguage();
  assert.equal(app.context.I18N.getLanguage(), 'ja');
  await app.finish(0);
  await app.startup;
  assert.equal(app.context.I18N.getLanguage(), 'ja');
  assert.equal(app.context.collectAppSettings().language, 'ja');
  assert.equal(app.requests.length, 2);
  await app.finish(1);
  assert.equal(app.backend().language, 'ja');
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('delayed language detection preserves a dropdown draft without saving it', async () => {
  let finishDetection;
  const detectedLanguage = new Promise(resolve => { finishDetection = resolve; });
  const app = await createApp({ settings: { language: '' }, osLanguage: detectedLanguage, deferStartup: true });
  await flushRequests();
  app.context.setSettingValue('setting-language', 'ja');
  finishDetection('en');
  await flushRequests();
  assert.equal(app.context.collectAppSettings().language, 'ja');
  assert.equal(app.requests.length, 1);
  assert.equal(app.requests[0].payload.language, 'en');
  await app.finish(0);
  await app.startup;
  assert.equal(app.backend().language, 'en');
  assert.equal(app.context.collectAppSettings().language, 'ja');
  assert.equal(app.context.settingsHaveChanges(), true);
});

test('delayed language detection cannot override a newer sidebar selection', async () => {
  let finishDetection;
  const detectedLanguage = new Promise(resolve => { finishDetection = resolve; });
  const app = await createApp({ settings: { language: '' }, osLanguage: detectedLanguage, deferStartup: true });
  await flushRequests();
  app.context.cycleLanguage();
  await flushRequests();
  finishDetection('ja');
  await app.startup;
  await app.finish(0);
  assert.equal(app.context.I18N.getLanguage(), 'en');
  assert.equal(app.context.collectAppSettings().language, 'en');
  assert.equal(app.backend().language, 'en');
  assert.equal(app.requests.length, 1);
  assert.equal(app.context.settingsHaveChanges(), false);
});

test('saving unrelated settings during language detection does not suppress the detected default', async () => {
  let finishDetection;
  const detectedLanguage = new Promise(resolve => { finishDetection = resolve; });
  const app = await createApp({ settings: { language: '' }, osLanguage: detectedLanguage, deferStartup: true });
  await flushRequests();
  app.element('setting-sound-volume').value = 25;
  const save = app.context.saveAppSettings();
  await flushRequests();
  await app.finish(0);
  await save;
  assert.equal(app.backend().language, '');
  finishDetection('en');
  await flushRequests();
  assert.equal(app.requests.length, 2);
  assert.equal(app.requests[1].payload.language, 'en');
  assert.equal(app.requests[1].payload.soundVolume, 25);
  await app.finish(1);
  await app.startup;
  assert.equal(app.backend().language, 'en');
  assert.equal(app.context.I18N.getLanguage(), 'en');
  assert.equal(app.context.settingsHaveChanges(), false);
});
