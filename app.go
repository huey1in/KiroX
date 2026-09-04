package main

import (
	"context"
	"log"
	"os"
	"reg_go/internal/browser"
	"reg_go/internal/email"
	"reg_go/internal/proxy"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"reg_go/internal/storage"
	"reg_go/internal/task"
	"reg_go/internal/updater"
	"time"
)

type App struct {
	ctx context.Context
}

// NewApp 创建新的 App 实例
func NewApp() *App {
	return &App{}
}

// startup 在应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 重定向日志到内存
	log.SetOutput(&logWriter{app: a})
	log.SetFlags(log.Ltime)

	// 初始化代理池（按数据目录持久化）
	proxy.InitPool(storage.GetDataDir())

	// 居中显示窗口
	go func() {
		time.Sleep(200 * time.Millisecond)
		runtime.WindowCenter(ctx)
	}()

}

// shutdown 在应用关闭时调用
func (a *App) shutdown(ctx context.Context) {
	storage.FlushAccountsSync()
}

// OpenURL 在系统默认浏览器中打开 URL
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// logWriter 自定义日志写入器，根据运行状态路由日志
type logWriter struct {
	app *App
}

func (w *logWriter) Write(p []byte) (int, error) {
	msg := string(p)
	task.Manager.AppendLog(msg)
	return os.Stderr.Write(p)
}

// GetStatus 获取任务状态
func (a *App) GetStatus() map[string]interface{} {
	return task.Manager.GetStatus()
}

// GetLogs 获取日志
func (a *App) GetLogs() []string {
	return task.Manager.GetLogs()
}

// GetOverview 获取全局概览数据
func (a *App) GetOverview() map[string]interface{} {
	// Outlook 账号统计
	outlookTotal, outlookRegistered, outlookSuccess, outlookPending := countOutlookAccounts()

	// 当前任务状态
	taskStatus := task.Manager.GetStatus()

	return map[string]interface{}{
		"version": updater.GetCurrentVersion(),
		"kiro": map[string]interface{}{
			"taskRunning":   taskStatus["running"],
			"taskSuccess":   taskStatus["success"],
			"taskFailed":    taskStatus["failed"],
			"taskCompleted": taskStatus["completed"],
			"taskTotal":     taskStatus["total"],
		},
		"outlook": map[string]interface{}{
			"total":      outlookTotal,
			"registered": outlookRegistered,
			"success":    outlookSuccess,
			"pending":    outlookPending,
		},
	}
}

// GetTaskStatus 获取实时任务状态
func (a *App) GetTaskStatus() map[string]interface{} {
	taskStatus := task.Manager.GetStatus()
	return map[string]interface{}{
		"kiro": map[string]interface{}{
			"taskRunning":   taskStatus["running"],
			"taskSuccess":   taskStatus["success"],
			"taskFailed":    taskStatus["failed"],
			"taskCompleted": taskStatus["completed"],
			"taskTotal":     taskStatus["total"],
		},
	}
}

// countOutlookAccounts 统计 Outlook 账号
func countOutlookAccounts() (total, registered, success, pending int) {
	accounts := storage.GetAccountsCached()
	if len(accounts) == 0 {
		return
	}
	total = len(accounts)
	for _, acc := range accounts {
		reg, _ := acc["registered"].(bool)
		suc, _ := acc["success"].(bool)
		if reg {
			registered++
			if suc {
				success++
			}
		} else {
			pending++
		}
	}
	return
}

// ---- MoeMail ----

func (a *App) GetMoeMailConfigs() []email.MoeMailConfig {
	return email.GetMoeMailConfigs()
}

func (a *App) SaveMoeMailConfigs(configsJSON string) map[string]interface{} {
	return email.SaveMoeMailConfigs(configsJSON)
}

func (a *App) TestMoeMailConnection(configJSON string) map[string]interface{} {
	return email.TestMoeMailConnection(configJSON)
}

// ---- CloudMail ----

func (a *App) GetCloudMailConfigs() []email.CloudMailConfig {
	return email.GetCloudMailConfigs()
}

func (a *App) SaveCloudMailConfigs(configsJSON string) map[string]interface{} {
	return email.SaveCloudMailConfigs(configsJSON)
}

func (a *App) TestCloudMailConnection(configJSON string) map[string]interface{} {
	return email.TestCloudMailConnection(configJSON)
}

// ---- Outlook ----

func (a *App) AddOutlookAccounts(data string) map[string]interface{} {
	return email.AddOutlookAccounts(data)
}

func (a *App) GetOutlookAccounts() []map[string]interface{} {
	return email.GetOutlookAccounts()
}

func (a *App) DeleteOutlookAccount(em string) map[string]interface{} {
	return email.DeleteOutlookAccount(em)
}

func (a *App) ClearOutlookAccounts() map[string]interface{} {
	return email.ClearOutlookAccounts()
}

func (a *App) ClearRegisteredOutlookAccounts() map[string]interface{} {
	return email.ClearRegisteredOutlookAccounts()
}

func (a *App) ImportOutlookFile(filePath string) map[string]interface{} {
	return email.ImportOutlookFile(filePath)
}

// ---- MailNest ----

func (a *App) TestMailNestConnection(configJSON string) map[string]interface{} {
	return email.TestMailNestConnection(configJSON)
}

func (a *App) SaveMailNestConfig(configsJSON string) map[string]interface{} {
	return email.SaveMailNestConfig(configsJSON)
}

func (a *App) GetMailNestConfig() email.MailNestConfig {
	return email.GetMailNestConfig()
}

// SelectDirectory 选择目录 (Wails Dialog)
func (a *App) SelectDirectory() string {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择目录",
	})
	if err != nil {
		log.Printf("选择目录失败: %v", err)
		return ""
	}
	return path
}

// SelectOutlookFile 选择 Outlook 账号文件 (Wails Dialog)
func (a *App) SelectOutlookFile() string {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 Outlook 账号文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "文本文件 (*.txt)",
				Pattern:     "*.txt",
			},
			{
				DisplayName: "CSV 文件 (*.csv)",
				Pattern:     "*.csv",
			},
			{
				DisplayName: "所有文件 (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		log.Printf("选择文件失败: %v", err)
		return ""
	}
	return path
}

// GetDataDir 前端获取当前存储目录
func (a *App) GetDataDir() string {
	return storage.GetDataDir()
}

// SetDataDir 设置自定义存储目录（自动迁移旧数据）
func (a *App) SetDataDir(dir string) map[string]interface{} {
	path, err := storage.SetDataDirPath(dir)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"success": true, "path": path}
}

// ResetDataDir 重置为默认存储目录
func (a *App) ResetDataDir() map[string]interface{} {
	path := storage.ResetDataDirPath()
	return map[string]interface{}{"success": true, "path": path}
}

// GetResultOutputDir 获取注册结果输出目录（明文 accounts.json 的写入位置）
func (a *App) GetResultOutputDir() string {
	return storage.GetResultOutputDir()
}

// SetResultOutputDir 设置注册结果输出目录
func (a *App) SetResultOutputDir(dir string) map[string]interface{} {
	path, err := storage.SetResultOutputDir(dir)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"success": true, "path": path}
}

// ResetResultOutputDir 重置为默认输出目录
func (a *App) ResetResultOutputDir() map[string]interface{} {
	path := storage.ResetResultOutputDir()
	return map[string]interface{}{"success": true, "path": path}
}

// GetProxy 返回当前全局代理（空字符串=直连）
func (a *App) GetProxy() string {
	return storage.GetProxy()
}

// SetProxy 保存全局代理；输入的简写（host:port:user:pass 等）会被自动归一化；
// 保存后会探测代理出口 IP 与归属信息并一并返回。
func (a *App) SetProxy(raw string) map[string]interface{} {
	normalized, err := storage.SetProxy(raw)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	resp := map[string]interface{}{"success": true, "proxy": normalized}
	if normalized != "" {
		resp["detect"] = proxy.Detect(normalized)
	}
	return resp
}

// DetectProxy 单独探测一个代理（不保存），用于"测试连接"
func (a *App) DetectProxy(raw string) proxy.Info {
	normalized := storage.NormalizeProxyAddress(raw)
	return proxy.Detect(normalized)
}

// ResetProxy 清空代理，恢复直连
func (a *App) ResetProxy() map[string]interface{} {
	storage.ResetProxy()
	return map[string]interface{}{"success": true}
}

// GetLanguage 获取当前界面语言代码，空字符串表示未设置（前端应回落到 OS 语言）
func (a *App) GetLanguage() string {
	return storage.GetLanguage()
}

// SetLanguage 保存界面语言；仅接受 "zh"/"en"/"ja"
func (a *App) SetLanguage(lang string) map[string]interface{} {
	if err := storage.SetLanguage(lang); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"success": true, "language": lang}
}

// GetOSLanguage 返回操作系统语言代码 "zh"/"en"/"ja"，用于首次启动自动选语言
func (a *App) GetOSLanguage() string {
	return detectOSLanguage()
}

// StartTask 启动注册任务
func (a *App) StartTask(req task.StartTaskRequest) map[string]interface{} {
	return task.StartTask(req)
}

// StopTask 停止注册任务
func (a *App) StopTask() map[string]interface{} {
	return task.StopTask(true)
}

// CheckUpdate 手动检查更新
func (a *App) CheckUpdate() map[string]interface{} {
	return updater.CheckUpdate()
}

// ResetFingerprintCache 清空所有按代理缓存的浏览器指纹，下一次注册重新生成
func (a *App) ResetFingerprintCache() map[string]interface{} {
	browser.ResetIdentityCache()
	return map[string]interface{}{"success": true}
}

// ---- 多代理池 ----

// ListProxyPool 返回当前代理池
func (a *App) ListProxyPool() []proxy.PoolEntry {
	return proxy.List()
}

// AddProxyEntry 新增一条代理（url 会被归一化），weight 1-100
func (a *App) AddProxyEntry(name, rawURL string, weight int) map[string]interface{} {
	normalized := storage.NormalizeProxyAddress(rawURL)
	entry, err := proxy.Add(proxy.PoolEntry{Name: name, URL: normalized, Weight: weight})
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"success": true, "entry": entry}
}

// UpdateProxyEntry 更新代理（按 id）
func (a *App) UpdateProxyEntry(id, name, rawURL string, weight int, enabled bool) map[string]interface{} {
	var u string
	if rawURL != "" {
		u = storage.NormalizeProxyAddress(rawURL)
	}
	entry, err := proxy.Update(id, proxy.PoolEntry{Name: name, URL: u, Weight: weight, Enabled: enabled})
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"success": true, "entry": entry}
}

// DeleteProxyEntry 删除（按 id）
func (a *App) DeleteProxyEntry(id string) map[string]interface{} {
	if err := proxy.Delete(id); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"success": true}
}

// TestProxyByID 测试某条代理（按 id），并把探测结果持久化到代理池
func (a *App) TestProxyByID(id string) map[string]interface{} {
	entry, err := proxy.Get(id)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	start := time.Now()
	info := proxy.Detect(entry.URL)
	ms := int(time.Since(start) / time.Millisecond)
	_ = proxy.SetProbe(id, info, ms)
	return map[string]interface{}{
		"ok": info.OK, "scheme": info.Scheme, "ip": info.IP,
		"country": info.Country, "countryCode": info.CountryCode,
		"region": info.Region, "city": info.City,
		"isp": info.ISP, "proxyType": info.ProxyType,
		"error": info.Error, "ms": ms,
	}
}
