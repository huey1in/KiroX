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

function selectedIpIds() {
  return Object.keys(ipSelected).filter(function(k) { return ipSelected[k]; });
}

function onIpSearch() {
  clearTimeout(_ipSearchTimer);
  _ipSearchTimer = setTimeout(renderIpList, 300);
}

// ===== 加载 =====
async function loadIpList() {
  var box = document.getElementById('ip-list');
  if (box) box.innerHTML = '<div style="text-align:center;color:var(--text-muted);padding:40px 0;font-size:13px;">加载中...</div>';
  try {
    ipPool = await window.go.main.App.ListProxyPool();
  } catch (e) {
    ipPool = [];
  }
  ipSelected = {};
  renderIpList();
  probeAllIp();
}

// 并行探测（限并发5），填充地理位置 / 延迟 / 可用性
async function probeAllIp() {
  var i = 0;
  async function worker() {
    while (i < ipPool.length) {
      var __i = i++;
      var p = ipPool[__i];
      if (!p || !p.url) continue;
      try {
        var t0 = performance.now();
        var info = await window.go.main.App.TestProxyEntry(p.url);
        var ms = Math.round(performance.now() - t0);
        p._probe = {
          ok: !!(info && info.ok),
          ms: ms,
          country: info && info.country,
          city: info && info.city,
          region: info && info.region,
          error: info && info.error
        };
      } catch (e) {
        p._probe = { ok: false, error: e.message };
      }
    }
  }
  var ws = [];
  for (var k = 0; k < 5; k++) ws.push(worker());
  await Promise.all(ws);
  renderIpList();
}

// ===== 渲染 =====
function _dv(id) { var el = document.getElementById(id); return (el && el.dataset) ? (el.dataset.value || '') : ''; }
function onIpFilterProtocol() { renderIpList(); }
function onIpFilterStatus() { renderIpList(); }

function renderIpList() {
  var box = document.getElementById('ip-list');
  var empty = document.getElementById('ip-empty');
  if (!box) return;
  var q = (document.getElementById('ip-search').value || '').toLowerCase().trim();
  var fproto = _dv('ip-filter-protocol');
  var fstatus = _dv('ip-filter-status');
  var rows = ipPool.filter(function(p) {
    var u = parseProxyUrl(p.url);
    if (q) {
      var hay = (((p.name || '') + ' ' + (p.url || '')) + ' ' + (p.id || '')).toLowerCase();
      if (hay.indexOf(q) < 0) return false;
    }
    if (fproto && u.proto !== fproto) return false;
    if (fstatus === 'enabled' && !p.enabled) return false;
    if (fstatus === 'disabled' && p.enabled) return false;
    return true;
  });
  if (empty) empty.style.display = rows.length ? 'none' : 'block';
  box.innerHTML = renderIpTable(rows);
}

function renderIpTable(rows) {
  var allSel = rows.length > 0 && rows.every(function(p) { return ipSelected[p.id]; });
  var h = '<table style="width:100%;border-collapse:collapse;font-size:12.5px;">';
  h += '<thead><tr style="border-bottom:1px solid var(--border);">'
    + '<th style="width:32px;padding:8px;text-align:center;"><input type="checkbox" ' + (allSel ? 'checked' : '') + ' onclick="toggleIpSelectAll(this)"></th>'
    + '<th style="text-align:left;padding:8px;color:var(--text-muted);font-weight:600;">名称</th>'
    + '<th style="text-align:center;padding:8px;color:var(--text-muted);font-weight:600;">协议</th>'
    + '<th style="text-align:left;padding:8px;color:var(--text-muted);font-weight:600;">地址</th>'
    + '<th style="text-align:left;padding:8px;color:var(--text-muted);font-weight:600;">认证</th>'
    + '<th style="text-align:left;padding:8px;color:var(--text-muted);font-weight:600;">位置</th>'
    + '<th style="text-align:center;padding:8px;color:var(--text-muted);font-weight:600;">延迟</th>'
    + '<th style="text-align:center;padding:8px;color:var(--text-muted);font-weight:600;">状态</th>'
    + '<th style="padding:8px;color:var(--text-muted);font-weight:600;">操作</th>'
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

  var copyBtn = '<button type="button" class="btn btn-secondary btn-sm" title="复制" onclick="copyIpAddress(\'' + _ipEscape(p.url) + '\')">复制</button>';

  var auth = '—';
  if (u.user) {
    var show = p._showPass ? _ipEscape(u.pass) : '••••••';
    auth = _ipEscape(u.user) + ' : ' + show + ' <button type="button" class="btn btn-secondary btn-sm" onclick="toggleIpPass(\'' + id + '\')">' + (p._showPass ? '隐藏' : '👁') + '</button>';
  }

  var loc = '—';
  if (p._probe && p._probe.ok) {
    loc = _ipEscape(p._probe.country || '') + (p._probe.city ? (' · ' + _ipEscape(p._probe.city)) : '');
  }

  var lat = '—';
  if (p._probe) {
    lat = p._probe.ok ? _badgeOk(p._probe.ms ? (p._probe.ms + 'ms') : '可用') : _badgeErr('失败');
  }

  var protoBadge = _badgeOff((u.proto || 'http').toUpperCase());
  var statusBadge = p.enabled ? _badgeOk('启用') : _badgeOff('停用');

  var ops = '<button type="button" class="btn btn-secondary btn-sm" onclick="testIpEntry(\'' + id + '\')">测试</button>'
    + '<button type="button" class="btn btn-secondary btn-sm" onclick="editIpEntry(\'' + id + '\')">编辑</button>'
    + '<button type="button" class="btn btn-secondary btn-sm" style="color:var(--danger);" onclick="deleteIpEntry(\'' + id + '\')">删除</button>';

  return '<tr style="border-bottom:1px solid var(--border);">'
    + '<td style="padding:8px;text-align:center;"><input type="checkbox" ' + chk + ' onclick="toggleIpSelect(\'' + id + '\',this)"></td>'
    + '<td style="padding:8px;font-weight:600;">' + _ipEscape(p.name || u.host) + '</td>'
    + '<td style="padding:8px;text-align:center;">' + protoBadge + '</td>'
    + '<td style="padding:8px;font-family:var(--font-mono);font-size:12px;">' + _ipEscape(addr) + ' ' + copyBtn + '</td>'
    + '<td style="padding:8px;">' + auth + '</td>'
    + '<td style="padding:8px;">' + loc + '</td>'
    + '<td style="padding:8px;text-align:center;">' + lat + '</td>'
    + '<td style="padding:8px;text-align:center;">' + statusBadge + '</td>'
    + '<td style="padding:8px;white-space:nowrap;">' + ops + '</td>'
    + '</tr>';
}

// 选择
function toggleIpSelect(id, el) { ipSelected[id] = el.checked; }
function toggleIpSelectAll(el) {
  var q = (document.getElementById('ip-search').value || '').toLowerCase().trim();
  var fproto = _dv('ip-filter-protocol');
  var fstatus = _dv('ip-filter-status');
  ipPool.forEach(function(p) {
    var u = parseProxyUrl(p.url);
    var visible = true;
    if (q && (((p.name || '') + ' ' + (p.url || '') + ' ' + (p.id || '')).toLowerCase().indexOf(q) < 0)) visible = false;
    if (fproto && u.proto !== fproto) visible = false;
    if (fstatus === 'enabled' && !p.enabled) visible = false;
    if (fstatus === 'disabled' && p.enabled) visible = false;
    if (visible) ipSelected[p.id] = el.checked;
  });
  renderIpList();
}

// 复制 / 密码显隐
function copyIpAddress(url) {
  if (navigator.clipboard) {
    navigator.clipboard.writeText(url).then(function() { showToast('已复制'); }, function() { showToast('复制失败', 'error'); });
  } else {
    showToast('复制失败', 'error');
  }
}
function toggleIpPass(id) {
  var p = ipPool.find(function(x) { return x.id === id; });
  if (!p) return;
  p._showPass = !p._showPass;
  renderIpList();
}

// ===== 测试 =====
async function testIpEntry(id) {
  var p = ipPool.find(function(x) { return x.id === id; });
  if (!p) return;
  showToast('正在测试…');
  try {
    var t0 = performance.now();
    var info = await window.go.main.App.TestProxyEntry(p.url);
    var ms = Math.round(performance.now() - t0);
    p._probe = { ok: info && info.ok, ms: ms, country: info && info.country, city: info && info.city, region: info && info.region, error: info && info.error };
    renderIpList();
    if (info && info.ok) showToast((info.scheme || '').toUpperCase() + ' · ' + (info.ip || '') + (info.country ? (' (' + info.country + ')') : '') + ' · ' + ms + 'ms');
    else showToast('不可用: ' + ((info && info.error) || '未知错误'), 'error');
  } catch (e) {
    showToast('测试失败: ' + e.message, 'error');
  }
}

async function batchTestIp() {
  var sel = selectedIpIds();
  var targets = ipPool.filter(function(p) { return sel.length ? sel.indexOf(p.id) >= 0 : !!p.url; });
  if (!targets.length) { showToast('没有可测试的代理'); return; }
  showToast('测试 ' + targets.length + ' 个代理…');
  var i = 0;
  async function w() {
    while (i < targets.length) {
      var t = targets[i++];
      try {
        var info = await window.go.main.App.TestProxyEntry(t.url);
        t._probe = { ok: info && info.ok, ms: 0, country: info && info.country, city: info && info.city, region: info && info.region, error: info && info.error };
      } catch (e) {}
    }
  }
  var ws = [];
  for (var k = 0; k < 5; k++) ws.push(w());
  await Promise.all(ws);
  renderIpList();
}

// ===== 新增/编辑 =====
function openIpFormModal() {
  ipFormMode = 'create';
  ipEditingId = null;
  document.getElementById('ip-form-title').textContent = '添加代理';
  document.getElementById('ip-form-name').value = '';
  document.getElementById('ip-form-protocol').value = 'http';
  document.getElementById('ip-form-host').value = '';
  document.getElementById('ip-form-port').value = '8080';
  setDropdownValue(document.getElementById('ip-form-protocol'), 'http');
  document.getElementById('ip-form-user').value = '';
  document.getElementById('ip-form-pass').value = '';
  document.getElementById('ip-form-enabled').checked = true;
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
  document.getElementById('ip-form-title').textContent = '编辑代理';
  document.getElementById('ip-form-name').value = p.name || u.host || '';
  document.getElementById('ip-form-protocol').value = u.proto || 'http';
  document.getElementById('ip-form-host').value = u.host || '';
  document.getElementById('ip-form-port').value = u.port || '';
  document.getElementById('ip-form-user').value = u.user || '';
  document.getElementById('ip-form-pass').value = u.pass || '';
  document.getElementById('ip-form-enabled').checked = p.enabled;
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
    pass: document.getElementById('ip-form-pass').value,
    name: document.getElementById('ip-form-name').value.trim()
  };
  if (!obj.host) { showToast('主机不能为空', 'error'); return; }
  var url = buildProxyUrl(obj);
  if (ipFormMode === 'create') {
    try {
      var res = await window.go.main.App.AddProxyEntry(obj.name, url, 1);
      if (res && res.error) { showToast(res.error, 'error'); return; }
      showToast('已添加');
    } catch (e) { showToast('添加失败: ' + e.message, 'error'); return; }
  } else {
    try {
      var res2 = await window.go.main.App.UpdateProxyEntry(ipEditingId, obj.name, url, 1, document.getElementById('ip-form-enabled').checked);
      if (res2 && res2.error) { showToast(res2.error, 'error'); return; }
      showToast('已保存');
    } catch (e) { showToast('保存失败: ' + e.message, 'error'); return; }
  }
  closeIpFormModal();
  await loadIpList();
}

async function submitBatch() {
  var r = parseBatchText();
  if (!r.valid.length) { showToast('没有有效的代理行', 'error'); return; }
  var added = 0, dup = 0, fail = 0;
  for (var i = 0; i < r.valid.length; i++) {
    try {
      var res = await window.go.main.App.AddProxyEntry('', r.valid[i], 1);
      if (res && res.error) {
        if (/已存在/.test(res.error)) dup++; else fail++;
      } else added++;
    } catch (e) { fail++; }
  }
  showToast('完成：' + added + ' 成功' + (dup ? '，' + dup + ' 已存在' : '') + (fail ? '，' + fail + ' 失败' : ''));
  closeIpFormModal();
  await loadIpList();
}

// ===== 删除 =====
function deleteIpEntry(id) {
  showConfirmModal('删除代理', '确认从池中删除该代理？', '删除', async function() {
    try {
      var res = await window.go.main.App.DeleteProxyEntry(id);
      if (res && res.error) { showToast(res.error, 'error'); return; }
      showToast('已删除');
      await loadIpList();
    } catch (e) { showToast('删除失败: ' + e.message, 'error'); }
  });
}

async function batchDeleteIp() {
  var sel = selectedIpIds();
  if (!sel.length) { showToast('请选择要删除的代理'); return; }
  showConfirmModal('批量删除', '确认删除选中的 ' + sel.length + ' 个代理？', '批量删除', async function() {
    var ok = 0, err = 0;
    for (var i = 0; i < sel.length; i++) {
      try {
        var res = await window.go.main.App.DeleteProxyEntry(sel[i]);
        if (res && res.error) err++; else ok++;
      } catch (e) { err++; }
    }
    showToast('删除完成：' + ok + ' 成功' + (err ? '，' + err + ' 失败' : ''));
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
  var html = '<div class="dropdown-option" data-value="">直连</div>';
  enabled.forEach(function(p) {
    var u = parseProxyUrl(p.url);
    var label = p.name || (u.host + (u.port ? ':' + u.port : ''));
    html += '<div class="dropdown-option" data-value="' + _ipEscape(p.url) + '">' + _ipEscape(label) + '</div>';
  });
  box.innerHTML = html;
  setDropdownValue(wrap, '');
}
