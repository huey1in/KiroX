package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	keyDataDir         = "data_dir"
	keyResultOutputDir = "result_output_dir"
	keyProxy           = "proxy"
	keyLanguage        = "language"
)

var (
	_dataDir          string
	_dataDirOnce      sync.Once
	_resultOutputDir  string
	_resultOutputOnce sync.Once
	_proxy            string
	_proxyOnce        sync.Once
	_language         string
	_languageOnce     sync.Once
)

// GetDefaultDataDir 获取默认应用数据目录
func GetDefaultDataDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "kirox")
}

// getConfigFilePath 获取配置文件路径（始终在默认目录下）
func getConfigFilePath() string {
	return filepath.Join(GetDefaultDataDir(), "storage.conf")
}

// loadConfigMap 解析 storage.conf 为 KV；兼容旧版（整文件即 data_dir 路径）
func loadConfigMap() map[string]string {
	m := map[string]string{}
	data, err := os.ReadFile(getConfigFilePath())
	if err != nil {
		return m
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return m
	}
	if !strings.ContainsRune(text, '=') {
		m[keyDataDir] = text
		return m
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i < 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		if k != "" {
			m[k] = v
		}
	}
	return m
}

func saveConfigMap(m map[string]string) error {
	os.MkdirAll(GetDefaultDataDir(), 0755)
	var b strings.Builder
	for _, k := range []string{keyDataDir, keyResultOutputDir, keyProxy, keyLanguage} {
		if v := strings.TrimSpace(m[k]); v != "" {
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(v)
			b.WriteByte('\n')
		}
	}
	return os.WriteFile(getConfigFilePath(), []byte(b.String()), 0600)
}

// GetDataDir 获取应用数据目录（优先使用自定义目录）
func GetDataDir() string {
	_dataDirOnce.Do(func() {
		m := loadConfigMap()
		custom := strings.TrimSpace(m[keyDataDir])
		if custom != "" {
			if info, err := os.Stat(custom); err == nil && info.IsDir() {
				_dataDir = custom
			}
		}
		if _dataDir == "" {
			_dataDir = GetDefaultDataDir()
		}
		os.MkdirAll(_dataDir, 0755)
	})
	return _dataDir
}

// SetDataDirPath 设置自定义存储目录（自动迁移 accounts.dat）
func SetDataDirPath(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("目录不能为空")
	}
	oldDir := GetDataDir()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	if oldDir != "" && oldDir != dir {
		migrated, migErr := migrateData(oldDir, dir)
		if migErr != nil {
			return "", fmt.Errorf("数据迁移失败: %w", migErr)
		}
		if migrated > 0 {
			log.Printf("已迁移 %d 个数据文件: %s → %s", migrated, oldDir, dir)
		}
	}

	m := loadConfigMap()
	m[keyDataDir] = dir
	if err := saveConfigMap(m); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	_dataDir = dir
	_dataDirOnce = sync.Once{}
	_dataDirOnce.Do(func() {})

	return dir, nil
}

// ResetDataDirPath 重置为默认存储目录（自动迁移数据回默认目录）
func ResetDataDirPath() string {
	oldDir := GetDataDir()
	defaultDir := GetDefaultDataDir()

	if oldDir != "" && oldDir != defaultDir {
		migrated, _ := migrateData(oldDir, defaultDir)
		if migrated > 0 {
			log.Printf("已迁移 %d 个数据文件: %s → %s", migrated, oldDir, defaultDir)
		}
	}

	m := loadConfigMap()
	delete(m, keyDataDir)
	_ = saveConfigMap(m)

	os.MkdirAll(defaultDir, 0755)
	_dataDir = defaultDir
	_dataDirOnce = sync.Once{}
	_dataDirOnce.Do(func() {})

	return defaultDir
}

// getDefaultResultOutputDir 默认输出目录：用户文档目录下的 Kirox 文件夹。
// 若无法解析用户主目录，回落到可执行文件所在目录下的 output。
func getDefaultResultOutputDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "Documents", "Kirox")
	}
	base := "."
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		base = filepath.Dir(exe)
	} else if cwd, err := os.Getwd(); err == nil {
		base = cwd
	}
	return filepath.Join(base, "output")
}

// GetResultOutputDir 获取注册结果输出目录（默认为用户文档目录下的 Kirox）
func GetResultOutputDir() string {
	_resultOutputOnce.Do(func() {
		m := loadConfigMap()
		if custom := strings.TrimSpace(m[keyResultOutputDir]); custom != "" {
			_resultOutputDir = custom
		} else {
			_resultOutputDir = getDefaultResultOutputDir()
		}
		os.MkdirAll(_resultOutputDir, 0755)
	})
	return _resultOutputDir
}

// SetResultOutputDir 设置自定义输出目录（不迁移已有 JSON 文件）
func SetResultOutputDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("目录不能为空")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}
	m := loadConfigMap()
	m[keyResultOutputDir] = dir
	if err := saveConfigMap(m); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}
	_resultOutputDir = dir
	_resultOutputOnce = sync.Once{}
	_resultOutputOnce.Do(func() {})
	return dir, nil
}

// ResetResultOutputDir 重置为默认输出目录（用户文档目录下的 Kirox）
func ResetResultOutputDir() string {
	m := loadConfigMap()
	delete(m, keyResultOutputDir)
	_ = saveConfigMap(m)

	defaultDir := getDefaultResultOutputDir()
	os.MkdirAll(defaultDir, 0755)
	_resultOutputDir = defaultDir
	_resultOutputOnce = sync.Once{}
	_resultOutputOnce.Do(func() {})
	return defaultDir
}

// GetProxy 返回当前全局代理 URL（空字符串表示直连）。
func GetProxy() string {
	_proxyOnce.Do(func() {
		m := loadConfigMap()
		_proxy = strings.TrimSpace(m[keyProxy])
	})
	return _proxy
}

// SetProxy 设置全局代理 URL（会自动归一化常见简写格式）。
func SetProxy(raw string) (string, error) {
	normalized := NormalizeProxyAddress(strings.TrimSpace(raw))
	m := loadConfigMap()
	if normalized == "" {
		delete(m, keyProxy)
	} else {
		m[keyProxy] = normalized
	}
	if err := saveConfigMap(m); err != nil {
		return "", err
	}
	_proxy = normalized
	_proxyOnce = sync.Once{}
	_proxyOnce.Do(func() {})
	return normalized, nil
}

// ResetProxy 清空代理配置，恢复直连。
func ResetProxy() {
	m := loadConfigMap()
	delete(m, keyProxy)
	_ = saveConfigMap(m)
	_proxy = ""
	_proxyOnce = sync.Once{}
	_proxyOnce.Do(func() {})
}

// GetLanguage 返回当前界面语言代码（"zh"/"en"/"ja"），未设置时返回空字符串。
func GetLanguage() string {
	_languageOnce.Do(func() {
		m := loadConfigMap()
		_language = strings.TrimSpace(m[keyLanguage])
	})
	return _language
}

// SetLanguage 持久化界面语言；仅接受 "zh"/"en"/"ja"，其他值返回错误。
func SetLanguage(lang string) error {
	lang = strings.TrimSpace(lang)
	if lang != "zh" && lang != "en" && lang != "ja" {
		return fmt.Errorf("不支持的语言: %s", lang)
	}
	m := loadConfigMap()
	m[keyLanguage] = lang
	if err := saveConfigMap(m); err != nil {
		return err
	}
	_language = lang
	_languageOnce = sync.Once{}
	_languageOnce.Do(func() {})
	return nil
}

// NormalizeProxyAddress 归一化常见代理写法为完整 URL:
//   - 已带 scheme 的 URL 原样返回
//   - host:port:user:pass -> http://user:pass@host:port (cliproxy 等导出格式)
//   - host:port -> socks5://host:port
//   - user:pass@host:port -> http://user:pass@host:port
func NormalizeProxyAddress(s string) string {
	if s == "" {
		return ""
	}
	if strings.Contains(s, "://") {
		return s
	}
	if strings.Contains(s, "@") {
		return "http://" + s
	}
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 4:
		host, port, user, pass := parts[0], parts[1], parts[2], parts[3]
		if host != "" && port != "" {
			return fmt.Sprintf("http://%s:%s@%s:%s", user, pass, host, port)
		}
	case 2:
		return "socks5://" + s
	}
	return s
}

// migrateData 将旧目录中的数据文件迁移到新目录
func migrateData(oldDir, newDir string) (int, error) {
	migrated := 0
	items := []string{"accounts.json", "accounts.dat"}

	for _, item := range items {
		src := filepath.Join(oldDir, item)
		dst := filepath.Join(newDir, "accounts.json")

		if _, err := os.Stat(src); err != nil {
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return migrated, err
		}
		os.MkdirAll(filepath.Dir(dst), 0755)
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return migrated, err
		}
		migrated++
	}
	return migrated, nil
}

// GetAccountsPath 获取微软邮箱账号文件路径
func GetAccountsPath() string {
	return filepath.Join(GetDataDir(), "accounts.json")
}

// ===== Accounts 内存缓存（消除并发文件 I/O 瓶颈）=====

var (
	_accountsCache   []map[string]interface{}
	_accountsMu      sync.RWMutex
	_accountsLoadMu  sync.Mutex
	_accountsFlushMu sync.Mutex
	_accountsLoaded  bool
	_accountsDirty   bool
	_accountsVersion uint64
	_flushTimer      *time.Timer
)

func loadAccountsCache() {
	_accountsMu.RLock()
	loaded := _accountsLoaded
	_accountsMu.RUnlock()
	if loaded {
		return
	}
	_accountsLoadMu.Lock()
	defer _accountsLoadMu.Unlock()
	_accountsMu.RLock()
	loaded = _accountsLoaded
	_accountsMu.RUnlock()
	if loaded {
		return
	}

	data, err := LoadJSON(GetAccountsPath())
	if err != nil {
		data = []map[string]interface{}{}
	}
	_accountsMu.Lock()
	// A replacement made while disk I/O was running already owns the cache.
	if !_accountsLoaded {
		_accountsCache = data
		_accountsLoaded = true
	}
	_accountsMu.Unlock()
}

func cloneAccounts(accounts []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, len(accounts))
	for i, account := range accounts {
		// Account fields are scalar values; copy each map to isolate readers.
		result[i] = maps.Clone(account)
	}
	return result
}

// GetAccountsCached 获取账号列表（从内存缓存）
func GetAccountsCached() []map[string]interface{} {
	loadAccountsCache()
	_accountsMu.RLock()
	defer _accountsMu.RUnlock()
	return cloneAccounts(_accountsCache)
}

// SetAccountsCached 替换账号列表并触发异步刷盘
func SetAccountsCached(accounts []map[string]interface{}) {
	_accountsMu.Lock()
	_accountsCache = cloneAccounts(accounts)
	_accountsLoaded = true
	_accountsDirty = true
	_accountsVersion++
	scheduleFlush()
	_accountsMu.Unlock()
}

// ModifyAccountsCached 原子修改账号列表（回调在锁内执行，高效无文件 I/O）
func ModifyAccountsCached(fn func([]map[string]interface{}) []map[string]interface{}) {
	loadAccountsCache()
	_accountsMu.Lock()
	_accountsCache = fn(_accountsCache)
	_accountsDirty = true
	_accountsVersion++
	scheduleFlush()
	_accountsMu.Unlock()
}

func scheduleFlush() {
	if _flushTimer != nil {
		_flushTimer.Stop()
	}
	_flushTimer = time.AfterFunc(500*time.Millisecond, flushAccountsToDisk)
}

func flushAccountsToDisk() {
	// Serialize snapshots as well as writes so an older flush cannot win last.
	_accountsFlushMu.Lock()
	defer _accountsFlushMu.Unlock()
	_accountsMu.RLock()
	if !_accountsDirty {
		_accountsMu.RUnlock()
		return
	}
	data := cloneAccounts(_accountsCache)
	version := _accountsVersion
	_accountsMu.RUnlock()

	err := SaveJSON(GetAccountsPath(), data)

	_accountsMu.Lock()
	if err == nil && _accountsVersion == version {
		_accountsDirty = false
	}
	_accountsMu.Unlock()
}

// FlushAccountsSync 同步刷盘（程序退出前调用）
func FlushAccountsSync() {
	_accountsMu.Lock()
	if _flushTimer != nil {
		_flushTimer.Stop()
		_flushTimer = nil
	}
	_accountsMu.Unlock()
	flushAccountsToDisk()
}

// ===== JSON 存储读写 =====

var fileMutexes sync.Map

func getFileMutex(filePath string) *sync.Mutex {
	val, _ := fileMutexes.LoadOrStore(filePath, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// LoadJSON 从文件读取 JSON 数组（线程安全）
func LoadJSON(filePath string) ([]map[string]interface{}, error) {
	mu := getFileMutex(filePath)
	mu.Lock()
	defer mu.Unlock()
	return loadJSON(filePath)
}

// SaveJSON 将 JSON 数组写入文件（线程安全，原子写入）
func SaveJSON(filePath string, items []map[string]interface{}) error {
	mu := getFileMutex(filePath)
	mu.Lock()
	defer mu.Unlock()
	return saveJSON(filePath, items)
}

func loadJSON(filePath string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func saveJSON(filePath string, items []map[string]interface{}) error {
	b, err := json.Marshal(items)
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(filePath), 0755)
	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmpFile, filePath)
}
