package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	settingsSchemaVersion = 3
	keyDataDir            = "data_dir"
	keyResultOutputDir    = "result_output_dir"
	keyProxy              = "proxy"
	keyLanguage           = "language"
)

type fileMigration struct {
	Destination string
	Sources     []string
}

var businessDataMigrations = []fileMigration{
	{Destination: "accounts.json", Sources: []string{"accounts.json", "accounts.dat"}},
	{Destination: "moemail.json", Sources: []string{"moemail.json", "moemail.dat"}},
	{Destination: "cloudmail.json", Sources: []string{"cloudmail.json", "cloudmail.dat"}},
	{Destination: "mailnest.json", Sources: []string{"mailnest.json", "mailnest.dat"}},
	{Destination: "proxy_pool.json", Sources: []string{"proxy_pool.json"}},
}

type settingsFile struct {
	SchemaVersion   int         `json:"schemaVersion"`
	DataDir         string      `json:"dataDir,omitempty"`
	ResultOutputDir string      `json:"resultOutputDir,omitempty"`
	Proxy           string      `json:"proxy,omitempty"`
	Language        string      `json:"language,omitempty"`
	Runtime         AppSettings `json:"runtime"`
}

// AppSettings contains user-facing runtime and advanced service overrides.
// Directory settings stay at the root of settings.json because they are needed before
// the rest of the application is initialized.
type AppSettings struct {
	EmailProxyMode            string `json:"emailProxyMode"`
	EmailProxy                string `json:"emailProxy"`
	OTPTimeoutSeconds         int    `json:"otpTimeoutSeconds"`
	RetryProfile              string `json:"retryProfile"`
	StopOnRisk                bool   `json:"stopOnRisk"`
	SoundEnabled              bool   `json:"soundEnabled"`
	DesktopNotifications      bool   `json:"desktopNotifications"`
	SoundVolume               int    `json:"soundVolume"`
	AutoCheckUpdates          bool   `json:"autoCheckUpdates"`
	Theme                     string `json:"theme"`
	Language                  string `json:"language"`
	PersistentLogs            bool   `json:"persistentLogs"`
	LogRetentionDays          int    `json:"logRetentionDays"`
	AutoProbeProxies          bool   `json:"autoProbeProxies"`
	MoeMailExpiryMinutes      int    `json:"moeMailExpiryMinutes"`
	AWSRegion                 string `json:"awsRegion"`
	RequestTimeoutSeconds     int    `json:"requestTimeoutSeconds"`
	FingerprintTTLHours       int    `json:"fingerprintTTLHours"`
	FingerprintOffsets        []int  `json:"fingerprintOffsets"`
	FingerprintCurvePositions []int  `json:"fingerprintCurvePositions"`
	TelemetryEnabled          bool   `json:"telemetryEnabled"`
	WAFEnabled                bool   `json:"wafEnabled"`
	TwoCaptchaAPIKey          string `json:"twoCaptchaAPIKey"`
	WAFWebsiteURL             string `json:"wafWebsiteURL"`
	WAFWebsiteKey             string `json:"wafWebsiteKey"`
	WAFIV                     string `json:"wafIV"`
	WAFContext                string `json:"wafContext"`
	WAFJSAPIScript            string `json:"wafJSAPIScript"`
	WAFChallengeScript        string `json:"wafChallengeScript"`
	WAFCaptchaScript          string `json:"wafCaptchaScript"`
	OIDCBase                  string `json:"oidcBase"`
	SigninBase                string `json:"signinBase"`
	ProfileBase               string `json:"profileBase"`
	ViewBase                  string `json:"viewBase"`
	PortalBase                string `json:"portalBase"`
	StartURL                  string `json:"startURL"`
	KiroBase                  string `json:"kiroBase"`
	KiroRedirectURI           string `json:"kiroRedirectURI"`
	DirectoryID               string `json:"directoryID"`
}

func DefaultAppSettings() AppSettings {
	return AppSettings{
		EmailProxyMode:            "follow-task",
		OTPTimeoutSeconds:         120,
		RetryProfile:              "standard",
		StopOnRisk:                true,
		SoundEnabled:              true,
		DesktopNotifications:      true,
		SoundVolume:               70,
		AutoCheckUpdates:          true,
		Theme:                     "system",
		LogRetentionDays:          7,
		AutoProbeProxies:          true,
		MoeMailExpiryMinutes:      60,
		AWSRegion:                 "us-east-1",
		RequestTimeoutSeconds:     60,
		FingerprintTTLHours:       6,
		FingerprintOffsets:        []int{0, 0, 0, 0, 0, 0, 0, 15, 15, 100},
		FingerprintCurvePositions: []int{0, 11, 22, 33, 44, 56, 67, 78, 89, 100},
		TelemetryEnabled:          true,
		WAFWebsiteURL:             "https://us-east-1.signin.aws/platform/d-9067642ac7/signup",
		OIDCBase:                  "https://oidc.us-east-1.amazonaws.com",
		SigninBase:                "https://us-east-1.signin.aws",
		ProfileBase:               "https://profile.aws.amazon.com",
		ViewBase:                  "https://view.awsapps.com",
		PortalBase:                "https://portal.sso.us-east-1.amazonaws.com",
		StartURL:                  "https://view.awsapps.com/start",
		KiroBase:                  "https://app.kiro.dev",
		KiroRedirectURI:           "https://app.kiro.dev/signin/oauth",
		DirectoryID:               "d-9067642ac7",
	}
}

func normalizeAppSettings(s AppSettings) AppSettings {
	d := DefaultAppSettings()
	if !oneOf(s.EmailProxyMode, "direct", "follow-task", "custom") {
		s.EmailProxyMode = d.EmailProxyMode
	}
	s.EmailProxy = NormalizeProxyAddress(strings.TrimSpace(s.EmailProxy))
	if s.EmailProxyMode != "custom" {
		s.EmailProxy = ""
	}
	if !oneOf(fmt.Sprint(s.OTPTimeoutSeconds), "60", "120", "180", "300") {
		s.OTPTimeoutSeconds = d.OTPTimeoutSeconds
	}
	if !oneOf(s.RetryProfile, "fast", "standard", "stable") {
		s.RetryProfile = d.RetryProfile
	}
	s.SoundVolume = clampDefault(s.SoundVolume, 0, 100, d.SoundVolume)
	if !oneOf(s.Theme, "system", "light", "dark") {
		s.Theme = d.Theme
	}
	if s.Language != "" && !oneOf(s.Language, "zh", "en", "ja") {
		s.Language = ""
	}
	s.LogRetentionDays = clampDefault(s.LogRetentionDays, 1, 90, d.LogRetentionDays)
	s.MoeMailExpiryMinutes = clampDefault(s.MoeMailExpiryMinutes, 10, 1440, d.MoeMailExpiryMinutes)
	s.RequestTimeoutSeconds = clampDefault(s.RequestTimeoutSeconds, 10, 180, d.RequestTimeoutSeconds)
	s.FingerprintTTLHours = clampDefault(s.FingerprintTTLHours, 1, 168, d.FingerprintTTLHours)
	s.FingerprintOffsets = normalizeFingerprintOffsets(s.FingerprintOffsets)
	s.FingerprintCurvePositions = normalizeFingerprintCurvePositions(s.FingerprintCurvePositions)
	if strings.TrimSpace(s.AWSRegion) == "" {
		s.AWSRegion = d.AWSRegion
	}
	if strings.TrimSpace(s.OIDCBase) == "" {
		s.OIDCBase = d.OIDCBase
	}
	if strings.TrimSpace(s.SigninBase) == "" {
		s.SigninBase = d.SigninBase
	}
	if strings.TrimSpace(s.ProfileBase) == "" {
		s.ProfileBase = d.ProfileBase
	}
	if strings.TrimSpace(s.ViewBase) == "" {
		s.ViewBase = d.ViewBase
	}
	if strings.TrimSpace(s.PortalBase) == "" {
		s.PortalBase = d.PortalBase
	}
	if strings.TrimSpace(s.StartURL) == "" {
		s.StartURL = d.StartURL
	}
	if strings.TrimSpace(s.KiroBase) == "" {
		s.KiroBase = d.KiroBase
	}
	if strings.TrimSpace(s.KiroRedirectURI) == "" {
		s.KiroRedirectURI = d.KiroRedirectURI
	}
	if strings.TrimSpace(s.DirectoryID) == "" {
		s.DirectoryID = d.DirectoryID
	}
	s.TwoCaptchaAPIKey = strings.TrimSpace(s.TwoCaptchaAPIKey)
	s.WAFWebsiteURL = strings.TrimSpace(s.WAFWebsiteURL)
	s.WAFWebsiteKey = strings.TrimSpace(s.WAFWebsiteKey)
	s.WAFIV = strings.TrimSpace(s.WAFIV)
	s.WAFContext = strings.TrimSpace(s.WAFContext)
	s.WAFJSAPIScript = strings.TrimSpace(s.WAFJSAPIScript)
	s.WAFChallengeScript = strings.TrimSpace(s.WAFChallengeScript)
	s.WAFCaptchaScript = strings.TrimSpace(s.WAFCaptchaScript)
	if s.WAFJSAPIScript == "https://us-east-1.signin.aws/assets/js/app.js" {
		s.WAFJSAPIScript = ""
	}
	if s.WAFWebsiteURL == "" {
		s.WAFWebsiteURL = d.WAFWebsiteURL
	}
	return s
}

func clampDefault(value, minValue, maxValue, fallback int) int {
	if value < minValue || value > maxValue {
		return fallback
	}
	return value
}

func normalizeFingerprintOffsets(values []int) []int {
	if len(values) == 5 {
		values = []int{values[0], values[1], values[0], values[1], values[1], values[2], values[2], values[3], values[3], values[4]}
	}
	if len(values) != 10 {
		values = []int{0, 0, 0, 0, 0, 0, 0, 15, 15, 100}
	}
	normalized := make([]int, 10)
	for i, value := range values {
		if value < 0 {
			value = 0
		} else if value > 100 {
			value = 100
		}
		normalized[i] = value
	}
	return normalized
}

func normalizeFingerprintCurvePositions(values []int) []int {
	if len(values) != 10 {
		return []int{0, 11, 22, 33, 44, 56, 67, 78, 89, 100}
	}
	normalized := make([]int, 10)
	for i, value := range values {
		minValue := 0
		if i > 0 {
			minValue = normalized[i-1] + 2
		}
		maxValue := 100 - (len(values)-1-i)*2
		if value < minValue {
			value = minValue
		} else if value > maxValue {
			value = maxValue
		}
		normalized[i] = value
	}
	return normalized
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

var (
	_dataDir          string
	_dataDirOnce      sync.Once
	_resultOutputDir  string
	_resultOutputOnce sync.Once
	_proxy            string
	_proxyOnce        sync.Once
	_language         string
	_languageOnce     sync.Once

	layoutOnce sync.Once
	layoutErr  error
	settingsMu sync.Mutex

	localDataBaseResolver = func() (string, error) {
		if runtime.GOOS == "windows" {
			return os.UserCacheDir()
		}
		return os.UserConfigDir()
	}
	configBaseResolver = os.UserConfigDir
)

// GetAppRootDir 获取安装版的应用根目录。Windows 下为 %LOCALAPPDATA%\KiroX。
func GetAppRootDir() string {
	baseDir, err := localDataBaseResolver()
	if err != nil {
		baseDir = "."
	}
	return filepath.Join(baseDir, "KiroX")
}

// GetDefaultDataDir 获取默认业务数据目录。
func GetDefaultDataDir() string {
	return filepath.Join(GetAppRootDir(), "data")
}

// GetCacheDir 获取可安全重建的缓存目录。
func GetCacheDir() string {
	dir := filepath.Join(GetAppRootDir(), "cache")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// GetLogsDir 获取应用日志目录。
func GetLogsDir() string {
	dir := filepath.Join(GetAppRootDir(), "logs")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func getSettingsFilePath() string {
	return filepath.Join(GetAppRootDir(), "settings.json")
}

func getLegacyRootDir() string {
	configDir, err := configBaseResolver()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "kirox")
}

func getLegacyConfigFilePath() string {
	return filepath.Join(getLegacyRootDir(), "storage.conf")
}

// loadLegacyConfigMap 解析旧版 storage.conf；兼容整文件仅保存 data_dir 的格式。
func loadLegacyConfigMap() map[string]string {
	m := map[string]string{}
	data, err := os.ReadFile(getLegacyConfigFilePath())
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

// migrateLegacyLayout copies data from the portable/Roaming layout once. Source
// files are deliberately retained so an upgrade can be rolled back safely.
func migrateLegacyLayout() error {
	settingsPath := getSettingsFilePath()
	if _, err := os.Stat(settingsPath); err == nil {
		settings := readSettingsFile()
		return writeSettingsFile(settings)
	} else if !os.IsNotExist(err) {
		return err
	}

	legacy := loadLegacyConfigMap()
	legacyRoot := getLegacyRootDir()
	dataDir := GetDefaultDataDir()
	sourceDir := legacyRoot
	if custom := strings.TrimSpace(legacy[keyDataDir]); custom != "" {
		custom = filepath.Clean(custom)
		if info, err := os.Stat(custom); err == nil && info.IsDir() {
			dataDir = custom
			sourceDir = custom
		}
	}

	migrated, err := migrateData(sourceDir, dataDir)
	if err != nil {
		return fmt.Errorf("迁移旧业务数据失败: %w", err)
	}
	cacheMigrated, err := migrateFiles(sourceDir, GetCacheDir(), []fileMigration{
		{Destination: "identities.json", Sources: []string{"identities.json", "identities.dat"}},
	})
	if err != nil {
		return fmt.Errorf("迁移旧缓存失败: %w", err)
	}

	settings := settingsFile{
		SchemaVersion:   settingsSchemaVersion,
		ResultOutputDir: strings.TrimSpace(legacy[keyResultOutputDir]),
		Proxy:           strings.TrimSpace(legacy[keyProxy]),
		Language:        strings.TrimSpace(legacy[keyLanguage]),
		Runtime:         DefaultAppSettings(),
	}
	settings.Runtime.Language = settings.Language
	if dataDir != GetDefaultDataDir() {
		settings.DataDir = dataDir
	}
	if err := writeSettingsFile(settings); err != nil {
		return fmt.Errorf("写入新设置文件失败: %w", err)
	}
	if migrated+cacheMigrated > 0 {
		log.Printf("[存储] 已从旧版目录复制 %d 个文件", migrated+cacheMigrated)
	}
	return nil
}

func readSettingsFile() settingsFile {
	settings := settingsFile{SchemaVersion: settingsSchemaVersion, Runtime: DefaultAppSettings()}
	data, err := os.ReadFile(getSettingsFilePath())
	if err != nil {
		return settings
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("[存储] settings.json 格式无效: %v", err)
		return settingsFile{SchemaVersion: settingsSchemaVersion, Runtime: DefaultAppSettings()}
	}
	if settings.SchemaVersion < 3 {
		settings.Runtime = DefaultAppSettings()
		settings.Runtime.Language = settings.Language
	}
	settings.Runtime = normalizeAppSettings(settings.Runtime)
	settings.SchemaVersion = settingsSchemaVersion
	return settings
}

func writeSettingsFile(settings settingsFile) error {
	settings.SchemaVersion = settingsSchemaVersion
	settings.Runtime = normalizeAppSettings(settings.Runtime)
	settings.Language = settings.Runtime.Language
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(GetAppRootDir(), 0o755); err != nil {
		return err
	}
	tmp := getSettingsFilePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, getSettingsFilePath())
}

// GetAppSettings returns a normalized copy of all runtime settings.
func GetAppSettings() AppSettings {
	return loadSettings().Runtime
}

// SaveAppSettings validates and atomically persists runtime settings.
func SaveAppSettings(appSettings AppSettings) (AppSettings, error) {
	appSettings = normalizeAppSettings(appSettings)
	for name, value := range map[string]string{
		"OIDC Base": appSettings.OIDCBase, "Signin Base": appSettings.SigninBase,
		"Profile Base": appSettings.ProfileBase, "View Base": appSettings.ViewBase,
		"Portal Base": appSettings.PortalBase, "Start URL": appSettings.StartURL,
		"Kiro Base": appSettings.KiroBase, "Kiro Redirect URI": appSettings.KiroRedirectURI,
	} {
		parsed, err := url.ParseRequestURI(value)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return AppSettings{}, fmt.Errorf("%s 必须是有效的 HTTP(S) URL", name)
		}
	}
	if appSettings.WAFEnabled {
		if appSettings.TwoCaptchaAPIKey == "" {
			return AppSettings{}, fmt.Errorf("启用 AWS WAF 打码时必须填写 2Captcha API Key")
		}
		hasAnyStaticParam := appSettings.WAFWebsiteKey != "" || appSettings.WAFIV != "" || appSettings.WAFContext != "" || appSettings.WAFJSAPIScript != "" || appSettings.WAFChallengeScript != "" || appSettings.WAFCaptchaScript != ""
		hasChallengeParams := appSettings.WAFWebsiteKey != "" && appSettings.WAFIV != "" && appSettings.WAFContext != ""
		hasJSAPIParams := appSettings.WAFJSAPIScript != ""
		if hasAnyStaticParam && !hasChallengeParams && !hasJSAPIParams {
			return AppSettings{}, fmt.Errorf("静态 AWS WAF 参数需要 websiteKey + iv + context，或 jsapiScript；全部留空时将自动处理动态挑战")
		}
		for name, value := range map[string]string{
			"WAF Website URL":     appSettings.WAFWebsiteURL,
			"WAF jsapiScript":     appSettings.WAFJSAPIScript,
			"WAF challengeScript": appSettings.WAFChallengeScript,
			"WAF captchaScript":   appSettings.WAFCaptchaScript,
		} {
			if value == "" && name != "WAF Website URL" {
				continue
			}
			parsed, err := url.ParseRequestURI(value)
			if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return AppSettings{}, fmt.Errorf("%s 必须是有效的 HTTP(S) URL", name)
			}
		}
	}
	if err := updateSettings(func(settings *settingsFile) {
		settings.Runtime = appSettings
		settings.Language = appSettings.Language
	}); err != nil {
		return AppSettings{}, err
	}
	_language = appSettings.Language
	_languageOnce = sync.Once{}
	_languageOnce.Do(func() {})
	return appSettings, nil
}

func ensureStorageLayout() error {
	layoutOnce.Do(func() {
		for _, dir := range []string{GetAppRootDir(), GetDefaultDataDir(), GetCacheDir(), GetLogsDir()} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				layoutErr = err
				return
			}
		}
		layoutErr = migrateLegacyLayout()
	})
	return layoutErr
}

func loadSettings() settingsFile {
	if err := ensureStorageLayout(); err != nil {
		log.Printf("[存储] 初始化目录失败: %v", err)
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	return readSettingsFile()
}

func updateSettings(update func(*settingsFile)) error {
	if err := ensureStorageLayout(); err != nil {
		return err
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settings := readSettingsFile()
	update(&settings)
	return writeSettingsFile(settings)
}

// GetDataDir 获取应用数据目录（优先使用自定义目录）
func GetDataDir() string {
	_dataDirOnce.Do(func() {
		settings := loadSettings()
		custom := strings.TrimSpace(settings.DataDir)
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

// SetDataDirPath 设置自定义存储目录并迁移全部业务数据文件。
func SetDataDirPath(dir string) (string, error) {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || dir == "." {
		return "", fmt.Errorf("目录不能为空")
	}
	_accountsLocationMu.Lock()
	defer _accountsLocationMu.Unlock()
	flushAccountsSyncLocked()
	oldDir := GetDataDir()

	if err := os.MkdirAll(dir, 0o755); err != nil {
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

	if err := updateSettings(func(settings *settingsFile) {
		settings.DataDir = dir
	}); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	_dataDir = dir
	_dataDirOnce = sync.Once{}
	_dataDirOnce.Do(func() {})
	resetAccountsCache()

	return dir, nil
}

// ResetDataDirPath 重置为默认存储目录（自动迁移数据回默认目录）
func ResetDataDirPath() (string, error) {
	_accountsLocationMu.Lock()
	defer _accountsLocationMu.Unlock()
	flushAccountsSyncLocked()
	oldDir := GetDataDir()
	defaultDir := GetDefaultDataDir()

	if oldDir != "" && oldDir != defaultDir {
		migrated, err := migrateData(oldDir, defaultDir)
		if err != nil {
			return "", fmt.Errorf("数据迁移失败: %w", err)
		}
		if migrated > 0 {
			log.Printf("已迁移 %d 个数据文件: %s → %s", migrated, oldDir, defaultDir)
		}
	}

	if err := updateSettings(func(settings *settingsFile) {
		settings.DataDir = ""
	}); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	os.MkdirAll(defaultDir, 0o755)
	_dataDir = defaultDir
	_dataDirOnce = sync.Once{}
	_dataDirOnce.Do(func() {})
	resetAccountsCache()

	return defaultDir, nil
}

// getDefaultResultOutputDir 默认输出目录：用户文档目录下的 KiroX 文件夹。
// 若无法解析用户主目录，回落到可执行文件所在目录下的 output。
func getDefaultResultOutputDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "Documents", "KiroX")
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

// GetResultOutputDir 获取注册结果输出目录（默认为用户文档目录下的 KiroX）
func GetResultOutputDir() string {
	_resultOutputOnce.Do(func() {
		settings := loadSettings()
		if custom := strings.TrimSpace(settings.ResultOutputDir); custom != "" {
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
	if err := updateSettings(func(settings *settingsFile) {
		settings.ResultOutputDir = dir
	}); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}
	_resultOutputDir = dir
	_resultOutputOnce = sync.Once{}
	_resultOutputOnce.Do(func() {})
	return dir, nil
}

// ResetResultOutputDir 重置为默认输出目录（用户文档目录下的 KiroX）
func ResetResultOutputDir() (string, error) {
	if err := updateSettings(func(settings *settingsFile) {
		settings.ResultOutputDir = ""
	}); err != nil {
		return "", err
	}

	defaultDir := getDefaultResultOutputDir()
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return "", fmt.Errorf("创建默认输出目录失败: %w", err)
	}
	_resultOutputDir = defaultDir
	_resultOutputOnce = sync.Once{}
	_resultOutputOnce.Do(func() {})
	return defaultDir, nil
}

// GetProxy 返回当前全局代理 URL（空字符串表示直连）。
func GetProxy() string {
	_proxyOnce.Do(func() {
		settings := loadSettings()
		_proxy = strings.TrimSpace(settings.Proxy)
	})
	return _proxy
}

// SetProxy 设置全局代理 URL（会自动归一化常见简写格式）。
func SetProxy(raw string) (string, error) {
	normalized := NormalizeProxyAddress(strings.TrimSpace(raw))
	if err := updateSettings(func(settings *settingsFile) {
		settings.Proxy = normalized
	}); err != nil {
		return "", err
	}
	_proxy = normalized
	_proxyOnce = sync.Once{}
	_proxyOnce.Do(func() {})
	return normalized, nil
}

// ResetProxy 清空代理配置，恢复直连。
func ResetProxy() {
	_ = updateSettings(func(settings *settingsFile) {
		settings.Proxy = ""
	})
	_proxy = ""
	_proxyOnce = sync.Once{}
	_proxyOnce.Do(func() {})
}

// GetLanguage 返回当前界面语言代码（"zh"/"en"/"ja"），未设置时返回空字符串。
func GetLanguage() string {
	_languageOnce.Do(func() {
		settings := loadSettings()
		_language = strings.TrimSpace(settings.Language)
	})
	return _language
}

// SetLanguage 持久化界面语言；仅接受 "zh"/"en"/"ja"，其他值返回错误。
func SetLanguage(lang string) error {
	lang = strings.TrimSpace(lang)
	if lang != "zh" && lang != "en" && lang != "ja" {
		return fmt.Errorf("不支持的语言: %s", lang)
	}
	if err := updateSettings(func(settings *settingsFile) {
		settings.Language = lang
		settings.Runtime.Language = lang
	}); err != nil {
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

// migrateData 将旧目录中的全部业务数据文件复制到新目录。
func migrateData(oldDir, newDir string) (int, error) {
	return migrateFiles(oldDir, newDir, businessDataMigrations)
}

func migrateFiles(oldDir, newDir string, migrations []fileMigration) (int, error) {
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return 0, err
	}
	migrated := 0
	for _, migration := range migrations {
		dst := filepath.Join(newDir, migration.Destination)
		if _, err := os.Stat(dst); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return migrated, err
		}

		for _, sourceName := range migration.Sources {
			src := filepath.Join(oldDir, sourceName)
			info, err := os.Stat(src)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return migrated, err
			}
			if !info.Mode().IsRegular() {
				continue
			}
			data, err := os.ReadFile(src)
			if err != nil {
				return migrated, err
			}
			if err := os.WriteFile(dst, data, 0o600); err != nil {
				return migrated, err
			}
			migrated++
			break
		}
	}
	return migrated, nil
}

// GetAccountsPath 获取微软邮箱账号文件路径
func GetAccountsPath() string {
	return filepath.Join(GetDataDir(), "accounts.json")
}

// ===== Accounts 内存缓存（消除并发文件 I/O 瓶颈）=====

var (
	_accountsCache      []map[string]interface{}
	_accountsLocationMu sync.RWMutex
	_accountsMu         sync.RWMutex
	_accountsLoadMu     sync.Mutex
	_accountsFlushMu    sync.Mutex
	_accountsLoaded     bool
	_accountsDirty      bool
	_accountsVersion    uint64
	_flushTimer         *time.Timer
)

func resetAccountsCache() {
	_accountsLoadMu.Lock()
	defer _accountsLoadMu.Unlock()
	_accountsFlushMu.Lock()
	defer _accountsFlushMu.Unlock()
	_accountsMu.Lock()
	defer _accountsMu.Unlock()
	if _flushTimer != nil {
		_flushTimer.Stop()
	}
	_accountsCache = nil
	_accountsLoaded = false
	_accountsDirty = false
	_accountsVersion = 0
	_flushTimer = nil
}

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
	_accountsLocationMu.RLock()
	defer _accountsLocationMu.RUnlock()
	loadAccountsCache()
	_accountsMu.RLock()
	defer _accountsMu.RUnlock()
	return cloneAccounts(_accountsCache)
}

// SetAccountsCached 替换账号列表并触发异步刷盘
func SetAccountsCached(accounts []map[string]interface{}) {
	_accountsLocationMu.RLock()
	defer _accountsLocationMu.RUnlock()
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
	_accountsLocationMu.RLock()
	defer _accountsLocationMu.RUnlock()
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
	_accountsLocationMu.RLock()
	defer _accountsLocationMu.RUnlock()
	flushAccountsToDiskLocked()
}

func flushAccountsToDiskLocked() {
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
	_accountsLocationMu.RLock()
	defer _accountsLocationMu.RUnlock()
	flushAccountsSyncLocked()
}

func flushAccountsSyncLocked() {
	_accountsMu.Lock()
	if _flushTimer != nil {
		_flushTimer.Stop()
		_flushTimer = nil
	}
	_accountsMu.Unlock()
	flushAccountsToDiskLocked()
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
