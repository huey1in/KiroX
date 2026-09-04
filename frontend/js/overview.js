// ===== 概览页面逻辑 =====

var overviewTimer = null;
var taskStatusTimer = null;

function _ovT(key, fallback) {
  if (window.I18N && typeof window.I18N.t === 'function') {
    var v = window.I18N.t(key);
    if (v && v !== key) return v;
  }
  return fallback;
}

// loadOverview 加载概览数据（含账号池统计，3秒刷新）
async function loadOverview() {
  if (!window.go || !window.go.main || !window.go.main.App) return;
  try {
    var data = await window.go.main.App.GetOverview();
    updateOverviewUI(data);
    // 加载 MoeMail 配置统计
    if (typeof loadMoeMailConfigs === 'function') {
      loadMoeMailConfigs();
    }
  } catch (e) {
    console.error('加载概览数据失败:', e);
  }
}

// loadTaskStatus 加载实时任务状态（纯内存，1秒刷新）
async function loadTaskStatus() {
  if (!window.go || !window.go.main || !window.go.main.App || !window.go.main.App.GetTaskStatus) return;
  try {
    await window.go.main.App.GetTaskStatus();
  } catch (e) {}
}

// updateOverviewUI 更新概览界面
function updateOverviewUI(data) {
  var kiro = data.kiro || {};

  // 本次任务成功数 + 成功率（来自实时任务状态）
  var taskSuccess = kiro.taskSuccess || 0;
  var taskFailed = kiro.taskFailed || 0;
  var taskTotal = taskSuccess + taskFailed;
  setText('ov-kiro-success', taskSuccess);
  var successRate = taskTotal > 0 ? Math.round(taskSuccess / taskTotal * 100) : 0;
  setText('ov-kiro-success-rate', successRate + '%');
}

// 辅助函数
function setText(id, text) {
  var el = document.getElementById(id);
  if (el) el.textContent = text;
}

// 启动概览定时刷新
function startOverviewTimer() {
  if (overviewTimer) clearInterval(overviewTimer);
  if (taskStatusTimer) clearInterval(taskStatusTimer);
  loadOverview();
  loadTaskStatus();
  overviewTimer = setInterval(loadOverview, 3000);
  taskStatusTimer = setInterval(loadTaskStatus, 1000);
}

// 停止概览定时刷新
function stopOverviewTimer() {
  if (overviewTimer) {
    clearInterval(overviewTimer);
    overviewTimer = null;
  }
  if (taskStatusTimer) {
    clearInterval(taskStatusTimer);
    taskStatusTimer = null;
  }
}
