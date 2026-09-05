// ===== IP 管理页逻辑 =====

var ipPool = [];
var ipSelected = {};
var ipFormMode = 'create'; // 'create' | 'edit'
var ipEditingId = null;
var _ipSearchTimer = null;

function _ipEscape(s) {
  if (s == null) return '';
  return String(s).replace(/[&<>"']/g, function(c) {
    return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
  });
}

function _ipT(key, fallback) {
  if (window.I18N && typeof window.I18N.t === 'function') {
    var v = window.I18N.t(key);
    if (v && v !== key) return v;
  }
  return fallback;
}

// ---- 代理 URL 解析 / 拼装 ----
// url 形如 scheme://[user:pass@]host[:port]（后端已归一化），兼容简写
function parseProxyUrl(u) {
  var r = { proto: 'http', host: '', port: '', user: '', pass: '' };
  if (!u) return r;
  var rest = u;
  var m = u.match(/^([a-zA-Z0-9]+):\/\//);
  if (m) { r.proto = m[1].toLowerCase(); rest = u.slice(m[0].length); }
  var at = rest.lastIndexOf('@');
  if (at >= 0) {
    var cred = rest.slice(0, at); rest = rest.slice(at + 1);
    var ci = cred.indexOf(':');
    if (ci >= 0) { r.user = cred.slice(0, ci); r.pass = cred.slice(ci + 1); }
    else r.user = cred;
  }
  if (rest[0] === '[') {
    var close = rest.indexOf(']');
    r.host = rest.slice(1, close);
    var after = rest.slice(close + 1);
    if (after[0] === ':') r.port = after.slice(1);
  } else {
    var colon = rest.lastIndexOf(':');
    if (colon >= 0) { r.host = rest.slice(0, colon); r.port = rest.slice(colon + 1); }
    else r.host = rest;
  }
  return r;
}

function buildProxyUrl(obj) {
  var proto = obj.protocol || 'http';
  var host = obj.host || '';
  var is6 = host && host.indexOf(':') >= 0 && host[0] !== '[';
  var hp = is6 ? '[' + host + ']' : host;
  if (obj.port) hp += ':' + obj.port;
  var cred = '';
  if (obj.user) cred = obj.user + (obj.pass ? ':' + obj.pass : '') + '@';
  return proto + '://' + cred + hp;
}

// 徽章辅助
function _badgeOk(t) { return '<span style="padding:2px 8px;border-radius:10px;font-size:11px;font-weight:600;background:rgba(16,185,129,0.15);color:#10b981;">' + t + '</span>'; }
function _badgeOff(t) { return '<span style="padding:2px 8px;border-radius:10px;font-size:11px;font-weight:600;background:rgba(107,114,128,0.15);color:var(--text-muted);">' + t + '</span>'; }
function _badgeErr(t) { return '<span style="padding:2px 8px;border-radius:10px;font-size:11px;font-weight:600;background:rgba(239,68,68,0.15);color:#ef4444;">' + t + '</span>'; }

// 代理质量色阶：家宽(最佳) / 移动(中等) / 机房(较差)
function _badgeType(type, label) {
  var knownTypes = { residential: true, mobile: true, datacenter: true };
  var typeClass = knownTypes[type] ? type : 'unknown';
  return '<span class="ip-type-badge ip-type-' + typeClass + '">' + _ipEscape(label || type) + '</span>';
}

// 国旗图标：用与侧边栏语言切换一致的 flagcdn 图标（如 CN -> https://flagcdn.com/w40/cn.png）
function flagImg(cc) {
  if (!cc || cc.length !== 2) return '';
  return '<img src="https://flagcdn.com/w40/' + cc.toLowerCase() + '.png" alt="' + _ipEscape(cc) + '" style="width:18px;height:13px;border-radius:2px;vertical-align:-2px;object-fit:cover;display:inline-block;">';
}

function selectedIpIds() {
  return Object.keys(ipSelected).filter(function(k) { return ipSelected[k]; });
}

function onIpSearch() {
  clearTimeout(_ipSearchTimer);
  _ipSearchTimer = setTimeout(renderIpList, 300);
}

// 搜索匹配串：包含出口 IP / 国家 / 类型 / 原始 URL / id，确保按看到的字段都能搜到
function ipSearchHay(p) {
  return [p.url, p.probeIp, p.probeCountry, p.probeType, p.id].filter(Boolean).join(' ').toLowerCase();
}

// ===== 加载 =====
var ipProbing = {}; // 正在测试中的代理 id -> true，用于骨架屏

async function loadIpList() {
  var box = document.getElementById('ip-list');
  if (box) box.innerHTML = '<div style="text-align:center;color:var(--text-muted);padding:40px 0;font-size:13px;">' + _ipT('ip.loading', '加载中...') + '</div>';
  try {
    ipPool = await window.go.main.App.ListProxyPool();
  } catch (e) {
    ipPool = [];
  }
  ipSelected = {};
  renderIpList();
  probeAllIp(false);
}

// 探测单个代理：显示骨架屏，结果写回持久化字段并重绘。返回完整上游结果。
async function probeOne(p) {
  if (!p || !p.url || ipProbing[p.id]) return null;
  ipProbing[p.id] = true;
  renderIpList();
  var t0 = performance.now();
  var res = null;
  try {
    res = await window.go.main.App.TestProxyByID(p.id);
    var ms = typeof res.ms === 'number' ? res.ms : Math.round(performance.now() - t0);
    if (res && res.error) {
      p.probeOk = false;
      p.probeError = res.error;
    } else {
      p.probeOk = !!res.ok;
      p.probeIp = res.ip || '';
      p.probeCountry = res.country || '';
      p.probeCountryCode = res.countryCode || '';
      p.probeRegion = res.region || '';
      p.probeCity = res.city || '';
      p.probeIsp = res.isp || '';
      p.probeType = res.proxyType || '';
      p.probeMs = ms;
      p.probeError = res.error || '';
      p.probeAt = Math.floor(Date.now() / 1000);
    }
  } catch (e) {
    p.probeOk = false;
    p.probeError = e.message;
  }
  delete ipProbing[p.id];
  renderIpList();
  return res;
}

// 并行探测（限并发5）。force=true 强制全部，否则只探测从未测过或缺国家代码的（旧数据补国旗）
async function probeAllIp(force) {
  var i = 0;
  async function worker() {
    while (i < ipPool.length) {
      var p = ipPool[i++];
      if (!p || !p.url) continue;
      if (!force && p.probeAt && p.probeCountryCode) continue; // 已有完整结果则不重复测
      await probeOne(p);
    }
  }
  var ws = [];
  for (var k = 0; k < 5; k++) ws.push(worker());
  await Promise.all(ws);
}

// ===== 渲染 =====
function _dv(id) { var el = document.getElementById(id); return (el && el.dataset) ? (el.dataset.value || '') : ''; }
function onIpFilterStatus() { renderIpList(); }

function renderIpList() {
  var box = document.getElementById('ip-list');
  var empty = document.getElementById('ip-empty');
  if (!box) return;
  var q = (document.getElementById('ip-search').value || '').toLowerCase().trim();
  var fstatus = _dv('ip-filter-status');
  var rows = ipPool.filter(function(p) {
    if (q) {
      if (ipSearchHay(p).indexOf(q) < 0) return false;
    }
    if (fstatus === 'enabled' && !p.enabled) return false;
    if (fstatus === 'disabled' && p.enabled) return false;
    return true;
  });
  if (empty) empty.style.display = rows.length ? 'none' : 'block';
  box.innerHTML = renderIpTable(rows);
}

function renderIpTable(rows) {
  var allSel = rows.length > 0 && rows.every(function(p) { return ipSelected[p.id]; });
  var h = '<table style="width:100%;table-layout:fixed;border-collapse:collapse;font-size:12.5px;">';
  h += '<thead><tr style="border-bottom:1px solid var(--border);">'
    + '<th style="width:30px;padding:8px;text-align:center;white-space:nowrap;"><input type="checkbox" ' + (allSel ? 'checked' : '') + ' onclick="toggleIpSelectAll(this)"></th>'
    + '<th style="text-align:left;padding:8px;color:var(--text-muted);font-weight:600;white-space:nowrap;">' + _ipT('ip.colAddress', '地址') + '</th>'
    + '<th style="text-align:center;padding:8px;color:var(--text-muted);font-weight:600;white-space:nowrap;width:70px;">' + _ipT('ip.colType', '类型') + '</th>'
    + '<th style="text-align:left;padding:8px;color:var(--text-muted);font-weight:600;white-space:nowrap;">' + _ipT('ip.colLocation', '位置') + '</th>'
    + '<th style="width:86px;text-align:center;padding:8px;color:var(--text-muted);font-weight:600;white-space:nowrap;">' + _ipT('ip.colLatency', '延迟') + '</th>'
    + '<th style="width:56px;text-align:center;padding:8px;color:var(--text-muted);font-weight:600;white-space:nowrap;">' + _ipT('ip.colStatus', '状态') + '</th>'
    + '<th style="width:172px;text-align:right;padding:8px;color:var(--text-muted);font-weight:600;white-space:nowrap;">' + _ipT('ip.colActions', '操作') + '</th>'
    + '</tr></thead><tbody>';
  rows.forEach(function(p) { h += renderIpRow(p); });
  h += '</tbody></table>';
  return h;
}

function renderIpRow(p) {
  var u = parseProxyUrl(p.url);
  var id = p.id;
  var chk = ipSelected[id] ? 'checked' : '';
  var addr = u.host + (u.port ? (':' + u.port) : '');
  var probing = !!ipProbing[id];
  var sk = '<span class="ip-skel"></span>';

  // 地址列：显示探测到的出口 IP（持久化），未探测时回退到原始 host:port；正在测试时骨架屏
  var addrCell;
  if (probing) {
    addrCell = '<span class="ip-skel wide"></span>';
  } else {
    var exitIp = (p.probeOk && p.probeIp) ? p.probeIp : addr;
    addrCell = '<span style="font-family:var(--font-mono);font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;flex:1;min-width:0;" title="' + _ipEscape(addr) + '">' + _ipEscape(exitIp) + '</span>';
  }

  // 类型列：机房 / 移动 / 家宽（探测中骨架屏）
  var typeCell = probing ? sk : (p.probeOk && p.probeType ? _badgeType(p.probeType, _ipT('ip.type.' + p.probeType, p.probeType)) : '—');

  // 位置列：国旗 + 国家（只显示国家，不显示城市等详细地址）
  var loc = probing ? sk : (p.probeOk ? (flagImg(p.probeCountryCode) + ' ' + _ipEscape(p.probeCountry || '')) : '—');

  // 延迟列
  var lat = '—';
  if (probing) lat = sk;
  else if (p.probeOk) lat = _badgeOk(p.probeMs ? (p.probeMs + 'ms') : _ipT('ip.available', '可用'));
  else if (p.probeAt) lat = _badgeErr(_ipT('ip.failure', '失败'));

  // 状态开关：添加后通过列表开关控制启用/停用
  var toggleHtml = '<label class="toggle-switch" title="' + (p.enabled ? _ipT('ip.enabled', '启用') : _ipT('ip.disabled', '停用')) + '">'
    + '<input type="checkbox" ' + (p.enabled ? 'checked' : '') + ' onchange="toggleIpEnabled(\'' + id + '\', this.checked)">'
    + '<span class="toggle-slider"></span></label>';

  var ops = '<button type="button" class="btn btn-secondary btn-sm" onclick="testIpEntry(\'' + id + '\')">' + _ipT('ip.test', '测试') + '</button>'
    + '<button type="button" class="btn btn-secondary btn-sm" onclick="editIpEntry(\'' + id + '\')">' + _ipT('ip.edit', '编辑') + '</button>'
    + '<button type="button" class="btn btn-secondary btn-sm" style="color:var(--danger);" onclick="deleteIpEntry(\'' + id + '\')">' + _ipT('ip.delete', '删除') + '</button>';

  return '<tr style="border-bottom:1px solid var(--border);">'
    + '<td style="padding:8px;text-align:center;"><input type="checkbox" ' + chk + ' onclick="toggleIpSelect(\'' + id + '\',this)"></td>'
    + '<td style="padding:8px;"><div style="display:flex;align-items:center;gap:6px;min-width:0;">'
      + addrCell + '</div></td>'
    + '<td style="padding:8px;text-align:center;white-space:nowrap;">' + typeCell + '</td>'
    + '<td style="padding:8px;white-space:nowrap;">' + loc + '</td>'
    + '<td style="padding:8px;text-align:center;white-space:nowrap;">' + lat + '</td>'
    + '<td style="padding:8px;text-align:center;">' + toggleHtml + '</td>'
    + '<td style="padding:8px;text-align:right;white-space:nowrap;">' + ops + '</td>'
    + '</tr>';
}

// 列表开关：启用/停用某个代理
async function toggleIpEnabled(id, enabled) {
  var p = ipPool.find(function(x) { return x.id === id; });
  if (!p) return;
  try {
    var res = await window.go.main.App.UpdateProxyEntry(id, '', p.url || '', p.weight || 1, enabled);
    if (res && res.error) { showToast(res.error, 'error'); await loadIpList(); return; }
    p.enabled = enabled;
    renderIpList();
  } catch (e) {
    showToast(_ipT('ip.saveFailed', '保存失败') + ': ' + e.message, 'error');
    await loadIpList();
  }
}

// 选择
function toggleIpSelect(id, el) { ipSelected[id] = el.checked; }
function toggleIpSelectAll(el) {
  var q = (document.getElementById('ip-search').value || '').toLowerCase().trim();
  var fstatus = _dv('ip-filter-status');
  ipPool.forEach(function(p) {
    var visible = true;
    if (q && (ipSearchHay(p).indexOf(q) < 0)) visible = false;
    if (fstatus === 'enabled' && !p.enabled) visible = false;
    if (fstatus === 'disabled' && p.enabled) visible = false;
    if (visible) ipSelected[p.id] = el.checked;
  });
  renderIpList();
}

// ===== 测试 =====
async function testIpEntry(id) {
  var p = ipPool.find(function(x) { return x.id === id; });
  if (!p) return;
  // 立即打开模态框显示加载骨架屏，探测完成后异步填充结果
  showIpResultModal(p, null);
  var res = await probeOne(p);
  showIpResultModal(p, res);
}

function closeIpResultModal() {
  var m = document.getElementById('ip-result-modal');
  if (m) m.classList.remove('show');
}

// 在模态框里展示完整上游检测结果；res 为 null 时显示加载骨架屏
function showIpResultModal(p, res) {
  var body = document.getElementById('ip-result-body');
  var m = document.getElementById('ip-result-modal');
  if (!body || !m) return;

  var row = function(label, valHtml) {
    return '<div style="display:flex;justify-content:space-between;align-items:center;gap:12px;padding:7px 0;border-bottom:1px solid var(--border);font-size:13px;">'
      + '<span style="color:var(--text-muted);flex-shrink:0;">' + label + '</span>'
      + '<span style="font-weight:600;text-align:right;word-break:break-all;">' + valHtml + '</span></div>';
  };

  // 加载态：已知信息（协议、上次出口 IP）直接硬编码，其余用骨架屏
  if (!res) {
    var u0 = parseProxyUrl(p.url);
    var known = [];
    known.push(row(_ipT('ip.resultScheme', '协议'), _ipEscape(u0.proto || '—')));
    known.push(row(_ipT('ip.resultIP', '出口 IP'), p.probeIp
      ? '<span style="font-family:var(--font-mono);">' + _ipEscape(p.probeIp) + '</span>'
      : '<span class="ip-skel" style="width:70%;"></span>'));
    known.push(row(_ipT('ip.colType', '类型'), p.probeType
      ? _badgeType(p.probeType, _ipT('ip.type.' + p.probeType, p.probeType))
      : '<span class="ip-skel" style="width:60px;"></span>'));
    known.push(row(_ipT('ip.resultCountry', '国家'), p.probeCountryCode
      ? (flagImg(p.probeCountryCode) + ' ' + _ipEscape(p.probeCountry || '—'))
      : '<span class="ip-skel" style="width:70%;"></span>'));
    body.innerHTML =
        '<div style="margin-bottom:12px;"><span class="ip-skel" style="width:110px;height:16px;"></span></div>'
      + known.join('')
      + '<div style="display:flex;gap:10px;justify-content:flex-end;margin-top:20px;">'
      + '<button onclick="closeIpResultModal()" class="btn btn-dark" data-i18n="common.close">' + _ipT('common.close', '关闭') + '</button>'
      + '</div>';
    m.classList.add('show');
    return;
  }

  var ok = !!(res && !res.error && res.ok);
  var ms = (res && typeof res.ms === 'number') ? res.ms : (p.probeMs || 0);
  var typeLabel = (p.probeType && _ipT('ip.type.' + p.probeType, p.probeType)) || '—';

  var html = '';
  html += '<div style="margin-bottom:12px;">'
    + (ok
        ? _badgeOk(_ipT('ip.available', '可用') + (ms ? ' · ' + ms + 'ms' : ''))
        : _badgeErr(_ipT('ip.failure', '失败') + ((res && res.error) ? ' · ' + _ipEscape(res.error) : '')))
    + '</div>';

  if (ok) {
    html += row(_ipT('ip.resultScheme', '协议'), _ipEscape(res.scheme || p._scheme || '—'));
    html += row(_ipT('ip.resultIP', '出口 IP'), '<span style="font-family:var(--font-mono);">' + _ipEscape(res.ip || p.probeIp || '—') + '</span>');
    html += row(_ipT('ip.colType', '类型'), _badgeType(p.probeType, typeLabel));
    html += row(_ipT('ip.resultCountry', '国家'), flagImg(res.countryCode || p.probeCountryCode) + ' ' + _ipEscape(res.country || p.probeCountry || '—'));
    html += row(_ipT('ip.resultRegion', '地区'), _ipEscape(res.region || p.probeRegion || '—'));
    html += row(_ipT('ip.resultCity', '城市'), _ipEscape(res.city || p.probeCity || '—'));
    html += row(_ipT('ip.resultISP', '运营商'), _ipEscape(res.isp || p.probeIsp || '—'));
  } else {
    html += row(_ipT('ip.resultError', '错误'), _ipEscape((res && res.error) || p.probeError || _ipT('ip.unknownError', '未知错误')));
  }

  html += '<div style="display:flex;gap:10px;justify-content:flex-end;margin-top:20px;">'
    + '<button onclick="closeIpResultModal()" class="btn btn-dark" data-i18n="common.close">' + _ipT('common.close', '关闭') + '</button>'
    + '</div>';

  body.innerHTML = html;
  m.classList.add('show');
}

async function batchTestIp() {
  var sel = selectedIpIds();
  var targets = ipPool.filter(function(p) { return sel.length ? sel.indexOf(p.id) >= 0 : !!p.url; });
  if (!targets.length) { showToast(_ipT('ip.noProxies', '没有可测试的代理')); return; }
  showToast(_ipT('ip.testingN', '测试 {n} 个代理…').replace('{n}', targets.length));
  var i = 0;
  async function w() {
    while (i < targets.length) {
      var t = targets[i++];
      await probeOne(t);
    }
  }
  var ws = [];
  for (var k = 0; k < 5; k++) ws.push(w());
  await Promise.all(ws);
}

// ===== 新增/编辑 =====
function openIpFormModal() {
  ipFormMode = 'create';
  ipEditingId = null;
  document.getElementById('ip-form-title').textContent = _ipT('ip.addTitle', '添加代理');
  document.getElementById('ip-form-protocol').value = 'http';
  document.getElementById('ip-form-host').value = '';
  document.getElementById('ip-form-port').value = '8080';
  setDropdownValue(document.getElementById('ip-form-protocol'), 'http');
  document.getElementById('ip-form-user').value = '';
  document.getElementById('ip-form-pass').value = '';
  document.getElementById('ip-form-batch-text').value = '';
  document.getElementById('ip-batch-status').textContent = '';
  switchIpFormTab('single');
  document.getElementById('ip-form-modal').classList.add('show');
}

function editIpEntry(id) {
  var p = ipPool.find(function(x) { return x.id === id; });
  if (!p) return;
  var u = parseProxyUrl(p.url);
  ipFormMode = 'edit';
  ipEditingId = id;
  document.getElementById('ip-form-title').textContent = _ipT('ip.editTitle', '编辑代理');
  document.getElementById('ip-form-protocol').value = u.proto || 'http';
  document.getElementById('ip-form-host').value = u.host || '';
  document.getElementById('ip-form-port').value = u.port || '';
  document.getElementById('ip-form-user').value = u.user || '';
  document.getElementById('ip-form-pass').value = u.pass || '';
  setDropdownValue(document.getElementById('ip-form-protocol'), u.proto || 'http');
  switchIpFormTab('single');
  document.getElementById('ip-form-modal').classList.add('show');
}

function closeIpFormModal() {
  document.getElementById('ip-form-modal').classList.remove('show');
}

function switchIpFormTab(tab) {
  var single = document.getElementById('ip-form-single');
  var batch = document.getElementById('ip-form-batch');
  var tS = document.getElementById('ip-tab-single');
  var tB = document.getElementById('ip-tab-batch');
  if (single) single.style.display = tab === 'single' ? 'grid' : 'none';
  if (batch) batch.style.display = tab === 'batch' ? 'block' : 'none';
  if (tS) tS.className = 'btn btn-' + (tab === 'single' ? 'dark' : 'secondary') + ' btn-sm';
  if (tB) tB.className = 'btn btn-' + (tab === 'batch' ? 'dark' : 'secondary') + ' btn-sm';
}

// 批量解析
function normalizeBatchLine(line) {
  var u = line.indexOf('://') >= 0 ? line : 'http://' + line;
  // 必须有 host[:port]
  if (!/^[a-zA-Z0-9]+:\/\/[^\/\s]+/.test(u)) return '';
  return u;
}
function parseBatchText() {
  var text = document.getElementById('ip-form-batch-text').value || '';
  var lines = text.split(/\r?\n/).map(function(s) { return s.trim(); }).filter(Boolean);
  var valid = [], invalid = 0, seen = {};
  lines.forEach(function(line) {
    var u = normalizeBatchLine(line);
    if (u && !seen[u]) { seen[u] = 1; valid.push(u); }
    else invalid++;
  });
  return { valid: valid, invalid: invalid };
}

async function submitIpForm() {
  if (ipFormMode === 'create' && document.getElementById('ip-form-batch').style.display !== 'none') {
    await submitBatch();
    return;
  }
  await submitSingle();
}

async function submitSingle() {
  var obj = {
    protocol: _dv('ip-form-protocol'),
    host: document.getElementById('ip-form-host').value.trim(),
    port: document.getElementById('ip-form-port').value,
    user: document.getElementById('ip-form-user').value,
    pass: document.getElementById('ip-form-pass').value
  };
  if (!obj.host) { showToast(_ipT('ip.hostRequired', '主机不能为空'), 'error'); return; }
  var url = buildProxyUrl(obj);
  if (ipFormMode === 'create') {
    try {
      var res = await window.go.main.App.AddProxyEntry('', url, 1);
      if (res && res.error) { showToast(res.error, 'error'); return; }
      showToast(_ipT('ip.added', '已添加'));
    } catch (e) { showToast(_ipT('ip.addFailed', '添加失败') + ': ' + e.message, 'error'); return; }
  } else {
    // 编辑时保持现有启用状态（列表开关负责切换）
    var cur = ipPool.find(function(x) { return x.id === ipEditingId; });
    var enabled = cur ? cur.enabled : true;
    try {
      var res2 = await window.go.main.App.UpdateProxyEntry(ipEditingId, '', url, 1, enabled);
      if (res2 && res2.error) { showToast(res2.error, 'error'); return; }
      showToast(_ipT('ip.saved', '已保存'));
    } catch (e) { showToast(_ipT('ip.saveFailed', '保存失败') + ': ' + e.message, 'error'); return; }
  }
  closeIpFormModal();
  await loadIpList();
}

async function submitBatch() {
  var r = parseBatchText();
  if (!r.valid.length) { showToast(_ipT('ip.invalidBatch', '没有有效的代理行'), 'error'); return; }
  var added = 0, dup = 0, fail = 0;
  for (var i = 0; i < r.valid.length; i++) {
    try {
      var res = await window.go.main.App.AddProxyEntry('', r.valid[i], 1);
      if (res && res.error) {
        if (/已存在/.test(res.error)) dup++; else fail++;
      } else added++;
    } catch (e) { fail++; }
  }
  var msg = _ipT('ip.batchDone', '完成：{added} 成功').replace('{added}', added);
  if (dup > 0) msg += _ipT('ip.batchDup', '，{n} 已存在').replace('{n}', dup);
  if (fail > 0) msg += _ipT('ip.batchFail', '，{n} 失败').replace('{n}', fail);
  showToast(msg);
  closeIpFormModal();
  await loadIpList();
}

// ===== 删除 =====
function deleteIpEntry(id) {
  showConfirmModal(_ipT('ip.deleteTitle', '删除代理'), _ipT('ip.deleteMsg', '确认从池中删除该代理？'), _ipT('ip.delete', '删除'), async function() {
    try {
      var res = await window.go.main.App.DeleteProxyEntry(id);
      if (res && res.error) { showToast(res.error, 'error'); return; }
      showToast(_ipT('ip.deleted', '已删除'));
      await loadIpList();
    } catch (e) { showToast(_ipT('ip.deleteFailed', '删除失败') + ': ' + e.message, 'error'); }
  });
}

async function batchDeleteIp() {
  var sel = selectedIpIds();
  if (!sel.length) { showToast(_ipT('ip.selectFirst', '请选择要删除的代理')); return; }
  showConfirmModal(_ipT('ip.batchDeleteTitle', '批量删除'), _ipT('ip.batchDeleteMsg', '确认删除选中的 {n} 个代理？').replace('{n}', sel.length), _ipT('ip.batchDelete', '批量删除'), async function() {
    var ok = 0, err = 0;
    for (var i = 0; i < sel.length; i++) {
      try {
        var res = await window.go.main.App.DeleteProxyEntry(sel[i]);
        if (res && res.error) err++; else ok++;
      } catch (e) { err++; }
    }
    showToast(_ipT('ip.batchDeleteDone', '删除完成：{ok} 成功').replace('{ok}', ok) + (err ? _ipT('ip.batchFail', '，{n} 失败').replace('{n}', err) : ''));
    await loadIpList();
  });
}

// ===== 填充新建任务模态框的代理下拉 =====
async function loadProxyOptions() {
  var wrap = document.getElementById('cfg-proxy-select');
  var box = document.getElementById('cfg-proxy-select-options');
  if (!wrap || !box) return;
  var list = [];
  try { list = await window.go.main.App.ListProxyPool(); } catch (e) {}
  var enabled = (list || []).filter(function(p) { return p.enabled; });
  var html = '<div class="dropdown-option" data-value="">' + _ipT('ip.direct', '直连') + '</div>';
  enabled.forEach(function(p) {
    var u = parseProxyUrl(p.url);
    var label = p.probeIp || (u.host + (u.port ? ':' + u.port : ''));
    html += '<div class="dropdown-option" data-value="' + _ipEscape(p.url) + '">' + _ipEscape(label) + '</div>';
  });
  box.innerHTML = html;
  setDropdownValue(wrap, '');
}
